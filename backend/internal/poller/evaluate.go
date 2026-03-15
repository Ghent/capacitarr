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
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
	"capacitarr/internal/services"
)

// evaluateAndCleanDisk scores all media items on a disk group and, when the
// threshold is breached, queues the highest-scoring candidates for deletion.
// Returns the number of items queued to the DeletionService worker (auto mode only).
func (p *Poller) evaluateAndCleanDisk(group db.DiskGroup, allItems []integrations.MediaItem, serviceClients map[uint]integrations.Integration, runStatsID uint) int {
	prefs, err := p.reg.Settings.GetPreferences()
	if err != nil {
		slog.Error("Failed to load preferences", "component", "poller", "operation", "load_preferences", "error", err)
		return 0
	}

	if group.TotalBytes == 0 {
		return 0
	}
	currentPct := float64(group.UsedBytes) / float64(group.TotalBytes) * 100
	if currentPct < group.ThresholdPct {
		slog.Debug("Disk within threshold, no action needed", "component", "poller",
			"mount", group.MountPath, "usedPct", fmt.Sprintf("%.1f", currentPct),
			"threshold", group.ThresholdPct)

		// Clear all pending and rejected items when below threshold — the queue
		// should only contain current, actionable candidates. Approved items
		// (mid-deletion) are preserved.
		if _, err := p.reg.Approval.ClearQueue(); err != nil {
			slog.Error("Failed to clear approval queue", "component", "poller", "error", err)
		}

		return 0
	}

	slog.Info("Disk threshold breached, evaluating media for deletion", "component", "poller",
		"mount", group.MountPath, "currentPct", fmt.Sprintf("%.1f", currentPct), "threshold", group.ThresholdPct)

	p.reg.Bus.Publish(events.ThresholdBreachedEvent{
		MountPath:    group.MountPath,
		CurrentPct:   currentPct,
		ThresholdPct: group.ThresholdPct,
		TargetPct:    group.TargetPct,
	})

	// Filter items on this mount — normalize paths for cross-platform
	// compatibility (Windows *arr instances return backslash paths).
	normalizedMount := normalizePath(group.MountPath)
	var diskItems []integrations.MediaItem
	for _, item := range allItems {
		if strings.HasPrefix(normalizePath(item.Path), normalizedMount) {
			diskItems = append(diskItems, item)
		}
	}

	slog.Debug("Items on disk mount", "component", "poller",
		"mount", group.MountPath, "itemCount", len(diskItems))

	rules, err := p.reg.Rules.List()
	if err != nil {
		slog.Error("Failed to load custom rules", "component", "poller", "operation", "load_rules", "error", err)
		return 0
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
		return 0
	}

	slog.Debug("Evaluation summary", "component", "poller",
		"mount", group.MountPath,
		"evaluated", len(evaluated),
		"protected", protectedCount,
		"targetBytesToFree", targetBytesToFree)

	var bytesFreed int64
	var deletionsQueued int

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
				deletionsQueued++
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
			factorsJSON, marshalErr := json.Marshal(ev.Factors)
			if marshalErr != nil {
				slog.Error("Failed to marshal score factors", "component", "poller", "error", marshalErr)
				factorsJSON = []byte("[]")
			}
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
			atomic.AddInt64(&p.lastRunFreedBytes, ev.Item.SizeBytes)
			slog.Info("Engine action taken", "component", "poller",
				"media", ev.Item.Title, "action", "queued_for_approval", "score", ev.Score, "freed", ev.Item.SizeBytes)
			continue
		}

		// Dry-run mode: write to audit_log via AuditLogService
		factorsJSON, marshalErr := json.Marshal(ev.Factors)
		if marshalErr != nil {
			slog.Error("Failed to marshal score factors", "component", "poller", "error", marshalErr)
			factorsJSON = []byte("[]")
		}
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
		atomic.AddInt64(&p.lastRunFreedBytes, ev.Item.SizeBytes)
		slog.Info("Engine action taken", "component", "poller",
			"media", ev.Item.Title, "action", db.ActionDryRun, "score", ev.Score, "freed", ev.Item.SizeBytes)
	}

	return deletionsQueued
}
