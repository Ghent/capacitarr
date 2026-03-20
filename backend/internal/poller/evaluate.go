// Package poller orchestrates periodic media library polling and capacity evaluation.
package poller

import (
	"encoding/json"
	"fmt"
	"log/slog"
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
func (p *Poller) evaluateAndCleanDisk(group db.DiskGroup, allItems []integrations.MediaItem, registry *integrations.IntegrationRegistry, runStatsID uint, prefs db.PreferenceSet, rules []db.CustomRule) int {
	effectiveTotal := group.EffectiveTotalBytes()
	if effectiveTotal == 0 {
		slog.Warn("Disk group effective total is 0, skipping evaluation",
			"component", "poller", "mount", group.MountPath,
			"totalBytes", group.TotalBytes, "override", group.TotalBytesOverride)
		return 0
	}
	currentPct := float64(group.UsedBytes) / float64(effectiveTotal) * 100
	if currentPct < group.ThresholdPct {
		slog.Debug("Disk within threshold, no action needed", "component", "poller",
			"mount", group.MountPath, "usedPct", fmt.Sprintf("%.1f", currentPct),
			"threshold", group.ThresholdPct)

		// Process any force-delete items even when below threshold
		forceQueued := p.processForceDeletes(registry, runStatsID)
		return forceQueued
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

	if len(diskItems) == 0 {
		slog.Warn("No items matched disk mount path — approval queue cannot be populated",
			"component", "poller", "mount", group.MountPath,
			"normalizedMount", normalizedMount, "totalItems", len(allItems))
		if len(allItems) > 0 {
			sampleCount := 3
			if len(allItems) < sampleCount {
				sampleCount = len(allItems)
			}
			for i := 0; i < sampleCount; i++ {
				slog.Debug("Sample item path for mount mismatch diagnosis",
					"component", "poller", "itemPath", normalizePath(allItems[i].Path),
					"mount", normalizedMount)
			}
		}
	}
	slog.Debug("Items on disk mount", "component", "poller",
		"mount", group.MountPath, "itemCount", len(diskItems))

	// Use the extracted Evaluator for scoring + categorization
	evaluator := engine.NewEvaluator()
	evalResult := evaluator.Evaluate(diskItems, prefs, rules, prefs.TiebreakerMethod)
	atomic.AddInt64(&p.lastRunEvaluated, int64(evalResult.TotalCount))
	atomic.AddInt64(&p.lastRunProtected, int64(len(evalResult.Protected)))

	slog.Debug("Evaluation summary", "component", "poller",
		"mount", group.MountPath,
		"evaluated", evalResult.TotalCount,
		"protected", len(evalResult.Protected),
		"candidates", len(evalResult.Candidates))

	targetBytesToFree := int64((currentPct - group.TargetPct) / 100.0 * float64(effectiveTotal))
	if targetBytesToFree <= 0 {
		slog.Warn("Target bytes to free is zero or negative, skipping evaluation",
			"component", "poller", "mount", group.MountPath,
			"currentPct", fmt.Sprintf("%.1f", currentPct),
			"targetPct", group.TargetPct,
			"targetBytesToFree", targetBytesToFree)
		return 0
	}

	// Get deletion candidates from the evaluator result
	candidates := evalResult.CandidatesForDeletion(targetBytesToFree)

	slog.Info("Candidate selection for approval/deletion", "component", "poller",
		"mount", group.MountPath,
		"executionMode", prefs.ExecutionMode,
		"totalCandidates", len(evalResult.Candidates),
		"selectedCandidates", len(candidates),
		"targetBytesToFree", targetBytesToFree)

	// Pre-build set of shows that have season-level entries in the candidates.
	// When season entries exist, prefer them over show-level entries so each season
	// can be individually approved/snoozed/deleted in the approval queue.
	showsWithSeasons := make(map[string]bool)
	for _, ev := range candidates {
		if ev.Item.Type == integrations.MediaTypeSeason && ev.Item.ShowTitle != "" {
			showsWithSeasons[ev.Item.ShowTitle] = true
		}
	}

	// Track which items are still needed this cycle (for queue reconciliation).
	// Keys are "MediaName|MediaType" strings matching the approval queue schema.
	neededKeys := make(map[string]bool)

	var bytesFreed int64
	var deletionsQueued int
	var skippedZeroScore int
	var skippedDedup int
	var skippedSnoozed int

	for _, ev := range candidates {
		if bytesFreed >= targetBytesToFree {
			break
		}
		if ev.IsProtected || ev.Score <= 0 {
			skippedZeroScore++
			continue
		}

		// Dedup: skip show-level entries when season entries exist for the same show.
		// Season entries allow granular per-season approval and deletion.
		if ev.Item.Type == integrations.MediaTypeShow {
			if showsWithSeasons[ev.Item.Title] {
				skippedDedup++
				continue
			}
		}

		slog.Debug("Deletion candidate", "component", "poller",
			"media", ev.Item.Title, "score", fmt.Sprintf("%.4f", ev.Score),
			"size", ev.Item.SizeBytes, "reason", ev.Reason)

		if prefs.ExecutionMode == "auto" {
			deleter, err := registry.Deleter(ev.Item.IntegrationID)
			if err != nil {
				slog.Error("Integration not registered as MediaDeleter", "component", "poller",
					"integrationId", ev.Item.IntegrationID, "error", err)
				continue
			}

			// Queue for background deletion via DeletionService
			diskGroupID := group.ID
			if err := p.reg.Deletion.QueueDeletion(services.DeleteJob{
				Client:      deleter,
				Item:        ev.Item,
				Reason:      ev.Reason,
				Score:       ev.Score,
				Factors:     ev.Factors,
				RunStatsID:  runStatsID,
				DiskGroupID: &diskGroupID,
			}); err != nil {
				slog.Warn("Deletion queue full, skipping item", "component", "poller", "item", ev.Item.Title)
				continue
			}
			bytesFreed += ev.Item.SizeBytes
			deletionsQueued++
			continue // Skip the synchronous DB insert below, worker handles it
		} else if prefs.ExecutionMode == "approval" {
			// Skip items that are currently snoozed (rejected with an active snooze window)
			if p.reg.Approval.IsSnoozed(ev.Item.Title, string(ev.Item.Type), group.ID) {
				skippedSnoozed++
				slog.Debug("Skipping snoozed item", "component", "poller", "media", ev.Item.Title)
				continue
			}

			// Upsert into approval_queue via ApprovalService
			factorsJSON, marshalErr := json.Marshal(ev.Factors)
			if marshalErr != nil {
				slog.Error("Failed to marshal score factors", "component", "poller", "error", marshalErr)
				factorsJSON = []byte("[]")
			}
			diskGroupID := group.ID
			if _, err := p.reg.Approval.UpsertPending(db.ApprovalQueueItem{
				MediaName:     ev.Item.Title,
				MediaType:     string(ev.Item.Type),
				Reason:        fmt.Sprintf("Score: %.2f (%s)", ev.Score, ev.Reason),
				ScoreDetails:  string(factorsJSON),
				SizeBytes:     ev.Item.SizeBytes,
				Score:         ev.Score,
				PosterURL:     ev.Item.PosterURL,
				IntegrationID: ev.Item.IntegrationID,
				ExternalID:    ev.Item.ExternalID,
				DiskGroupID:   &diskGroupID,
			}); err != nil {
				slog.Error("Failed to upsert approval queue item", "component", "poller", "media", ev.Item.Title, "error", err)
				continue
			}

			// Track this item as still-needed for post-loop reconciliation
			neededKeys[ev.Item.Title+"|"+string(ev.Item.Type)] = true

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
		diskGroupID := group.ID
		logEntry := db.AuditLogEntry{
			MediaName:     ev.Item.Title,
			MediaType:     string(ev.Item.Type),
			Reason:        fmt.Sprintf("Score: %.2f (%s)", ev.Score, ev.Reason),
			ScoreDetails:  string(factorsJSON),
			Action:        db.ActionDryRun,
			SizeBytes:     ev.Item.SizeBytes,
			Score:         ev.Score,
			IntegrationID: &integrationID,
			DiskGroupID:   &diskGroupID,
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

	// Per-cycle queue reconciliation: in approval mode, dismiss any pending items
	// for this disk group that are no longer in the "still-needed" set. This trims
	// stale entries that were added in previous cycles but are no longer candidates
	// (e.g., threshold was raised, scores changed, media was removed).
	if prefs.ExecutionMode == "approval" {
		if dismissed, reconcileErr := p.reg.Approval.ReconcileQueue(group.ID, neededKeys); reconcileErr != nil {
			slog.Error("Failed to reconcile approval queue", "component", "poller",
				"mount", group.MountPath, "error", reconcileErr)
		} else if dismissed > 0 {
			slog.Info("Approval queue reconciled", "component", "poller",
				"mount", group.MountPath, "dismissed", dismissed)
		}
	}

	// Diagnostic summary: log when candidates were found but all were skipped
	if len(candidates) > 0 && deletionsQueued == 0 && atomic.LoadInt64(&p.lastRunFlagged) == 0 {
		slog.Warn("All candidates were skipped — nothing flagged for approval/deletion",
			"component", "poller", "mount", group.MountPath,
			"executionMode", prefs.ExecutionMode,
			"candidates", len(candidates),
			"skippedZeroScore", skippedZeroScore,
			"skippedDedup", skippedDedup,
			"skippedSnoozed", skippedSnoozed,
			"bytesFreedSoFar", bytesFreed)
	}

	return deletionsQueued
}

// processForceDeletes queries the approval queue for force-delete items and
// queues them for deletion via the DeletionService, regardless of disk threshold.
// Returns the number of items queued.
func (p *Poller) processForceDeletes(registry *integrations.IntegrationRegistry, runStatsID uint) int {
	items, err := p.reg.Approval.ListForceDeletes()
	if err != nil {
		slog.Error("Failed to list force-delete items", "component", "poller", "error", err)
		return 0
	}
	if len(items) == 0 {
		return 0
	}

	slog.Info("Processing force-delete items", "component", "poller", "count", len(items))

	prefs, err := p.reg.Settings.GetPreferences()
	if err != nil {
		slog.Error("Failed to load preferences for force-delete", "component", "poller", "error", err)
		return 0
	}

	// Determine whether to simulate (dry-delete) instead of actually deleting
	forceDryRun := !prefs.DeletionsEnabled || prefs.ExecutionMode == "dry-run"

	var queued int
	for _, item := range items {
		deleter, err := registry.Deleter(item.IntegrationID)
		if err != nil {
			slog.Error("Integration not registered as MediaDeleter for force-delete", "component", "poller",
				"integrationId", item.IntegrationID, "media", item.MediaName, "error", err)
			continue
		}

		// Parse stored score details back into factors
		var factors []engine.ScoreFactor
		if item.ScoreDetails != "" {
			if jsonErr := json.Unmarshal([]byte(item.ScoreDetails), &factors); jsonErr != nil {
				slog.Warn("Failed to parse score details for force-delete", "id", item.ID, "error", jsonErr)
			}
		}

		mediaItem := integrations.MediaItem{
			ExternalID:    item.ExternalID,
			IntegrationID: item.IntegrationID,
			Type:          integrations.MediaType(item.MediaType),
			Title:         item.MediaName,
			SizeBytes:     item.SizeBytes,
		}

		if queueErr := p.reg.Deletion.QueueDeletion(services.DeleteJob{
			Client:      deleter,
			Item:        mediaItem,
			Reason:      item.Reason,
			Score:       0,
			Factors:     factors,
			RunStatsID:  runStatsID,
			ForceDryRun: forceDryRun,
		}); queueErr != nil {
			slog.Warn("Deletion queue full, skipping force-delete item", "component", "poller", "item", item.MediaName)
			continue
		}

		// Remove the force-delete entry from the queue after successful queueing
		if rmErr := p.reg.Approval.RemoveForceDelete(item.ID); rmErr != nil {
			slog.Error("Failed to remove force-delete entry", "component", "poller", "id", item.ID, "error", rmErr)
		}

		queued++
		slog.Info("Force-delete item queued for deletion", "component", "poller",
			"media", item.MediaName, "size", item.SizeBytes)
	}

	return queued
}
