// Package poller orchestrates periodic media library polling and capacity evaluation.
package poller

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync/atomic"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/integrations"
	"capacitarr/internal/services"
)

// evaluateAndCleanDisk scores all media items on a disk group and, when the
// threshold is breached, queues the highest-scoring candidates for deletion.
func (p *Poller) evaluateAndCleanDisk(group db.DiskGroup, allItems []integrations.MediaItem, serviceClients map[uint]integrations.Integration, runStatsID uint) {
	database := p.reg.DB

	var prefs db.PreferenceSet
	if err := database.FirstOrCreate(&prefs, db.PreferenceSet{ID: 1}).Error; err != nil {
		slog.Error("Failed to load preferences", "component", "poller", "operation", "load_preferences", "error", err)
		return
	}

	if group.TotalBytes == 0 {
		return
	}
	currentPct := float64(group.UsedBytes) / float64(group.TotalBytes) * 100
	if currentPct < group.ThresholdPct {
		slog.Debug("Disk within threshold, no action needed", "component", "poller",
			"mount", group.MountPath, "usedPct", fmt.Sprintf("%.1f", currentPct),
			"threshold", group.ThresholdPct)

		// Auto-clear all active snoozes when below threshold — gives a clean slate
		// for the next cleanup cycle. Resets rejected items back to pending.
		if _, err := p.reg.Approval.BulkUnsnooze(); err != nil {
			slog.Error("Failed to clear active snoozes", "component", "poller", "error", err)
		}

		return
	}

	slog.Info("Disk threshold breached, evaluating media for deletion", "component", "poller",
		"mount", group.MountPath, "currentPct", fmt.Sprintf("%.1f", currentPct), "threshold", group.ThresholdPct)

	// Filter items on this mount
	var diskItems []integrations.MediaItem
	for _, item := range allItems {
		if strings.HasPrefix(item.Path, group.MountPath) {
			diskItems = append(diskItems, item)
		}
	}

	slog.Debug("Items on disk mount", "component", "poller",
		"mount", group.MountPath, "itemCount", len(diskItems))

	var rules []db.CustomRule
	if err := database.Order("sort_order ASC, id ASC").Find(&rules).Error; err != nil {
		slog.Error("Failed to load custom rules", "component", "poller", "operation", "load_rules", "error", err)
		return
	}

	// Evaluate
	evaluated := engine.EvaluateMedia(diskItems, prefs, rules)
	atomic.AddInt64(&p.lastRunEvaluated, int64(len(evaluated)))

	// Count protected items for dashboard stats
	protectedCount := 0
	for _, ev := range evaluated {
		if ev.IsProtected {
			atomic.AddInt64(&p.lastRunProtected, 1)
			protectedCount++
		}
	}

	// Sort by score descending
	sort.Slice(evaluated, func(i, j int) bool {
		return evaluated[i].Score > evaluated[j].Score // highest score first
	})

	targetBytesToFree := int64((currentPct - group.TargetPct) / 100.0 * float64(group.TotalBytes))
	if targetBytesToFree <= 0 {
		return
	}

	slog.Debug("Evaluation summary", "component", "poller",
		"mount", group.MountPath,
		"evaluated", len(evaluated),
		"protected", protectedCount,
		"targetBytesToFree", targetBytesToFree)

	var bytesFreed int64

	// Pre-build set of shows that have season-level entries in the evaluation results.
	// When season entries exist, prefer them over show-level entries so each season
	// can be individually approved/snoozed/deleted in the approval queue.
	showsWithSeasons := make(map[string]bool)
	for _, ev := range evaluated {
		if ev.Item.Type == integrations.MediaTypeSeason && ev.Item.ShowTitle != "" {
			showsWithSeasons[ev.Item.ShowTitle] = true
		}
	}

	for _, ev := range evaluated {
		if bytesFreed >= targetBytesToFree {
			break
		}
		if ev.IsProtected || ev.Score <= 0 {
			continue
		}

		// Dedup: skip show-level entries when season entries exist for the same show.
		// Season entries allow granular per-season approval and deletion.
		if ev.Item.Type == integrations.MediaTypeShow {
			if showsWithSeasons[ev.Item.Title] {
				continue
			}
		}

		slog.Debug("Deletion candidate", "component", "poller",
			"media", ev.Item.Title, "score", fmt.Sprintf("%.4f", ev.Score),
			"size", ev.Item.SizeBytes, "reason", ev.Reason)

		if prefs.ExecutionMode == "auto" {
			client, ok := serviceClients[ev.Item.IntegrationID]
			if ok && client != nil {
				// Queue for background deletion via DeletionService
				if err := p.reg.Deletion.QueueDeletion(services.DeleteJob{
					Client:     client,
					Item:       ev.Item,
					Reason:     ev.Reason,
					Score:      ev.Score,
					Factors:    ev.Factors,
					RunStatsID: runStatsID,
				}); err != nil {
					slog.Warn("Deletion queue full, skipping item", "component", "poller", "item", ev.Item.Title)
					continue
				}
				bytesFreed += ev.Item.SizeBytes
				continue // Skip the synchronous DB insert below, worker handles it
			}
			slog.Error("Integration client not found for deletion", "component", "poller",
				"operation", "resolve_client", "integrationId", ev.Item.IntegrationID)
			continue
		} else if prefs.ExecutionMode == "approval" {
			// Skip items that are currently snoozed (rejected with an active snooze window)
			if p.reg.Approval.IsSnoozed(ev.Item.Title, string(ev.Item.Type)) {
				slog.Debug("Skipping snoozed item", "component", "poller", "media", ev.Item.Title)
				continue
			}

			// Upsert into approval_queue via ApprovalService
			factorsJSON, _ := json.Marshal(ev.Factors) //nolint:errcheck
			if _, err := p.reg.Approval.UpsertPending(db.ApprovalQueueItem{
				MediaName:     ev.Item.Title,
				MediaType:     string(ev.Item.Type),
				Reason:        fmt.Sprintf("Score: %.2f (%s)", ev.Score, ev.Reason),
				ScoreDetails:  string(factorsJSON),
				SizeBytes:     ev.Item.SizeBytes,
				PosterURL:     ev.Item.PosterURL,
				IntegrationID: ev.Item.IntegrationID,
				ExternalID:    ev.Item.ExternalID,
			}); err != nil {
				slog.Error("Failed to upsert approval queue item", "component", "poller", "media", ev.Item.Title, "error", err)
				continue
			}

			bytesFreed += ev.Item.SizeBytes
			atomic.AddInt64(&p.lastRunFlagged, 1)
			slog.Info("Engine action taken", "component", "poller",
				"media", ev.Item.Title, "action", "queued_for_approval", "score", ev.Score, "freed", ev.Item.SizeBytes)
			continue
		}

		// Dry-run mode: write to audit_log via AuditLogService
		factorsJSON, _ := json.Marshal(ev.Factors) //nolint:errcheck
		integrationID := ev.Item.IntegrationID
		logEntry := db.AuditLogEntry{
			MediaName:     ev.Item.Title,
			MediaType:     string(ev.Item.Type),
			Reason:        fmt.Sprintf("Score: %.2f (%s)", ev.Score, ev.Reason),
			ScoreDetails:  string(factorsJSON),
			Action:        db.ActionDryRun,
			SizeBytes:     ev.Item.SizeBytes,
			IntegrationID: &integrationID,
		}

		if err := p.reg.AuditLog.UpsertDryRun(logEntry); err != nil {
			slog.Error("Failed to upsert dry-run audit entry", "component", "poller", "media", ev.Item.Title, "error", err)
		}

		bytesFreed += ev.Item.SizeBytes
		atomic.AddInt64(&p.lastRunFlagged, 1)
		slog.Info("Engine action taken", "component", "poller",
			"media", ev.Item.Title, "action", db.ActionDryRun, "score", ev.Score, "freed", ev.Item.SizeBytes)
	}
}
