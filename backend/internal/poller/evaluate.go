package poller

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/integrations"
	"capacitarr/internal/notifications"
)

// evaluateAndCleanDisk scores all media items on a disk group and, when the
// threshold is breached, queues the highest-scoring candidates for deletion.
func evaluateAndCleanDisk(group db.DiskGroup, allItems []integrations.MediaItem, serviceClients map[uint]integrations.Integration) {
	var prefs db.PreferenceSet
	if err := db.DB.FirstOrCreate(&prefs, db.PreferenceSet{ID: 1}).Error; err != nil {
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
		return
	}

	slog.Info("Disk threshold breached, evaluating media for deletion", "component", "poller",
		"mount", group.MountPath, "currentPct", fmt.Sprintf("%.1f", currentPct), "threshold", group.ThresholdPct)

	// Notify: threshold breached
	notifications.Dispatch(notifications.NotificationEvent{
		Type:    notifications.EventThresholdBreach,
		Title:   "Threshold Breached",
		Message: fmt.Sprintf("Disk %s is at %.1f%% capacity (threshold: %.0f%%)", group.MountPath, currentPct, group.ThresholdPct),
		Fields: map[string]string{
			"Disk Group": group.MountPath,
			"Usage":      fmt.Sprintf("%.1f%%", currentPct),
			"Threshold":  fmt.Sprintf("%.0f%%", group.ThresholdPct),
		},
	})

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
	if err := db.DB.Order("sort_order ASC, id ASC").Find(&rules).Error; err != nil {
		slog.Error("Failed to load custom rules", "component", "poller", "operation", "load_rules", "error", err)
		return
	}

	// Evaluate
	evaluated := engine.EvaluateMedia(diskItems, prefs, rules)
	atomic.AddInt64(&lastRunEvaluated, int64(len(evaluated)))

	// Count protected items for dashboard stats
	protectedCount := 0
	for _, ev := range evaluated {
		if ev.IsProtected {
			atomic.AddInt64(&lastRunProtected, 1)
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

	// Pre-build set of shows that have show-level entries in the evaluation results.
	// When a show-level item exists, its size includes all seasons, so logging
	// individual seasons would create duplicates.
	showsInResults := make(map[string]bool)
	for _, ev := range evaluated {
		if ev.Item.Type == integrations.MediaTypeShow {
			showsInResults[ev.Item.Title] = true
		}
	}

	for _, ev := range evaluated {
		if bytesFreed >= targetBytesToFree {
			break
		}
		if ev.IsProtected || ev.Score <= 0 {
			continue
		}

		// Dedup: skip season entries when a show-level entry exists for the same parent.
		// The show entry already covers all seasons in its size total.
		if ev.Item.Type == integrations.MediaTypeSeason && ev.Item.ShowTitle != "" {
			if showsInResults[ev.Item.ShowTitle] {
				continue
			}
		}

		slog.Debug("Deletion candidate", "component", "poller",
			"media", ev.Item.Title, "score", fmt.Sprintf("%.4f", ev.Score),
			"size", ev.Item.SizeBytes, "reason", ev.Reason)

		actionName := "Dry-Run"
		if prefs.ExecutionMode == "auto" {
			client, ok := serviceClients[ev.Item.IntegrationID]
			if ok && client != nil {
				// Queue for background deletion so we don't block the poller
				select {
				case deleteQueue <- deleteJob{
					client:  client,
					item:    ev.Item,
					reason:  ev.Reason,
					score:   ev.Score,
					factors: ev.Factors,
				}:
					bytesFreed += ev.Item.SizeBytes
					continue // Skip the synchronous DB insert below, worker handles it
				default:
					slog.Warn("Deletion queue full, skipping item", "component", "poller", "item", ev.Item.Title)
					continue
				}
			} else {
				slog.Error("Integration client not found for deletion", "component", "poller",
					"operation", "resolve_client", "integrationId", ev.Item.IntegrationID)
				continue
			}
		} else if prefs.ExecutionMode == "approval" {
			actionName = "Queued for Approval"
		}

		factorsJSON, _ := json.Marshal(ev.Factors) //nolint:errcheck // marshal of known-safe struct
		integrationID := ev.Item.IntegrationID
		logEntry := db.AuditLog{
			MediaName:     ev.Item.Title,
			MediaType:     string(ev.Item.Type),
			Reason:        fmt.Sprintf("Score: %.2f (%s)", ev.Score, ev.Reason),
			ScoreDetails:  string(factorsJSON),
			Action:        actionName,
			SizeBytes:     ev.Item.SizeBytes,
			IntegrationID: &integrationID,
			ExternalID:    ev.Item.ExternalID,
			CreatedAt:     time.Now(),
		}

		// Dry-run dedup: upsert instead of creating duplicates. Each media item
		// appears only once in the audit log; timestamp reflects the most recent evaluation.
		if actionName == "Dry-Run" {
			var existing db.AuditLog
			result := db.DB.Where(
				"media_name = ? AND media_type = ? AND action = ?",
				ev.Item.Title, string(ev.Item.Type), "Dry-Run",
			).First(&existing)
			if result.Error == nil {
				db.DB.Model(&existing).Updates(map[string]interface{}{
					"reason":        logEntry.Reason,
					"score_details": logEntry.ScoreDetails,
					"size_bytes":    logEntry.SizeBytes,
					"created_at":    logEntry.CreatedAt,
				})
			} else {
				db.DB.Create(&logEntry)
			}
		} else {
			// Auto/approval modes always create new entries (real actions)
			db.DB.Create(&logEntry)
		}

		bytesFreed += ev.Item.SizeBytes
		atomic.AddInt64(&lastRunFlagged, 1)
		atomic.AddInt64(&lastRunFreedBytes, ev.Item.SizeBytes)
		slog.Info("Engine action taken", "component", "poller",
			"media", ev.Item.Title, "action", actionName, "score", ev.Score, "freed", ev.Item.SizeBytes)
	}
}
