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
// Returns the number of items queued to the DeletionService worker (auto and dry-run modes).
func (p *Poller) evaluateAndCleanDisk(group db.DiskGroup, allItems []integrations.MediaItem, registry *integrations.IntegrationRegistry, runStatsID uint, prefs db.PreferenceSet, weights map[string]int, rules []db.CustomRule) int {
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
	evalResult := evaluator.Evaluate(diskItems, weights, rules, prefs.TiebreakerMethod)
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

	// Pre-fetch all snoozed keys for this disk group in a single query.
	// This replaces per-item IsSnoozed() calls (N queries → 1).
	snoozedKeys, snoozedErr := p.reg.Approval.ListSnoozedKeys(group.ID)
	if snoozedErr != nil {
		slog.Error("Failed to pre-fetch snoozed keys, falling back to empty set",
			"component", "poller", "mount", group.MountPath, "error", snoozedErr)
		snoozedKeys = make(map[string]bool)
	}

	var bytesFreed int64
	var deletionsQueued int
	var skippedZeroScore int
	var skippedDedup int
	var skippedSnoozed int
	var skippedCollectionProtected int

	// Collect approval-mode items for batch upsert (Phase 2 optimization).
	var pendingBatch []db.ApprovalQueueItem

	// Track collections already expanded to avoid duplicate processing when
	// multiple items from the same collection appear in the candidate list.
	expandedCollections := make(map[string]bool)

	// Pre-fetch integration configs for collection deletion checks.
	// Uses a cache to avoid repeated DB lookups for the same integration.
	integrationConfigCache := make(map[uint]*db.IntegrationConfig)
	getIntegrationConfig := func(id uint) *db.IntegrationConfig {
		if cfg, ok := integrationConfigCache[id]; ok {
			return cfg
		}
		cfg, err := p.reg.Integration.GetByID(id)
		if err != nil {
			return nil
		}
		integrationConfigCache[id] = cfg
		return cfg
	}

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

		// Skip items that are currently snoozed (rejected with an active snooze window).
		// This check runs in ALL execution modes so items snoozed from the deletion queue
		// in auto/dry-run mode are also respected by the engine.
		// Uses pre-fetched snoozedKeys map for O(1) lookup instead of per-item DB query.
		snoozedKey := ev.Item.Title + "|" + string(ev.Item.Type)
		if snoozedKeys[snoozedKey] {
			skippedSnoozed++
			slog.Debug("Skipping snoozed item", "component", "poller", "media", ev.Item.Title)
			continue
		}

		slog.Debug("Deletion candidate", "component", "poller",
			"media", ev.Item.Title, "score", fmt.Sprintf("%.4f", ev.Score),
			"size", ev.Item.SizeBytes, "reason", ev.Reason)

		// ── Collection expansion ─────────────────────────────────────────
		// When collection deletion is enabled on the item's integration,
		// expand the single candidate into all collection members.
		// The engine is unchanged — this is a post-selection expansion step.
		type processItem struct {
			item            integrations.MediaItem
			score           float64
			factors         []engine.ScoreFactor
			collectionGroup string // non-empty if part of a collection
		}
		itemsToProcess := []processItem{{item: ev.Item, score: ev.Score, factors: ev.Factors}}

		cfg := getIntegrationConfig(ev.Item.IntegrationID)
		if cfg != nil && cfg.CollectionDeletion && len(ev.Item.Collections) > 0 {
			collectionName := ev.Item.Collections[0]

			// Skip if this collection was already expanded by a prior candidate
			if expandedCollections[collectionName] {
				slog.Debug("Skipping already-expanded collection member", "component", "poller",
					"media", ev.Item.Title, "collection", collectionName)
				continue
			}

			if resolver, ok := registry.CollectionResolver(ev.Item.IntegrationID); ok {
				members, resolveErr := resolver.ResolveCollectionMembers(ev.Item)
				if resolveErr != nil {
					slog.Warn("Collection resolution failed, proceeding with single item", "component", "poller",
						"media", ev.Item.Title, "collection", collectionName, "error", resolveErr)
				} else if len(members) > 1 {
					// Check if any collection member is protected by always_keep rules
					memberProtected := false
					for _, member := range members {
						isProtected, _, _, _ := engine.ApplyRulesExported(member, rules)
						if isProtected {
							slog.Info("Collection skipped — member has always_keep rule", "component", "poller",
								"trigger", ev.Item.Title, "collection", collectionName, "protectedMember", member.Title)
							memberProtected = true
							break
						}
					}
					if memberProtected {
						skippedCollectionProtected++
						expandedCollections[collectionName] = true
						continue
					}

					// Check if any collection member is snoozed
					memberSnoozed := false
					for _, member := range members {
						memberSnoozedKey := member.Title + "|" + string(member.Type)
						if snoozedKeys[memberSnoozedKey] {
							slog.Info("Collection skipped — member is snoozed", "component", "poller",
								"trigger", ev.Item.Title, "collection", collectionName, "snoozedMember", member.Title)
							memberSnoozed = true
							break
						}
					}
					if memberSnoozed {
						skippedSnoozed++
						expandedCollections[collectionName] = true
						continue
					}

					// Expand: replace the single trigger item with all collection members
					itemsToProcess = make([]processItem, 0, len(members))
					for _, member := range members {
						itemsToProcess = append(itemsToProcess, processItem{
							item:            member,
							score:           ev.Score, // Use the trigger item's score for all members
							factors:         ev.Factors,
							collectionGroup: collectionName,
						})
					}
					expandedCollections[collectionName] = true

					slog.Info("Collection expanded for deletion", "component", "poller",
						"trigger", ev.Item.Title, "collection", collectionName, "memberCount", len(members))
				}
			}
		}

		// ── Process all items (single item or expanded collection) ────────
		for _, pi := range itemsToProcess {
			if prefs.ExecutionMode == "auto" {
				deleter, err := registry.Deleter(pi.item.IntegrationID)
				if err != nil {
					slog.Error("Integration not registered as MediaDeleter", "component", "poller",
						"integrationId", pi.item.IntegrationID, "error", err)
					continue
				}

				// Queue for background deletion via DeletionService
				diskGroupID := group.ID
				if err := p.reg.Deletion.QueueDeletion(services.DeleteJob{
					Client:          deleter,
					Item:            pi.item,
					Score:           pi.score,
					Factors:         pi.factors,
					Trigger:         db.TriggerEngine,
					RunStatsID:      runStatsID,
					DiskGroupID:     &diskGroupID,
					CollectionGroup: pi.collectionGroup,
				}); err != nil {
					slog.Warn("Deletion queue full, skipping item", "component", "poller", "item", pi.item.Title)
					continue
				}
				bytesFreed += pi.item.SizeBytes
				deletionsQueued++
			} else if prefs.ExecutionMode == "approval" {
				// Collect for batch upsert after the loop.
				factorsJSON, marshalErr := json.Marshal(pi.factors)
				if marshalErr != nil {
					slog.Error("Failed to marshal score factors", "component", "poller", "error", marshalErr)
					factorsJSON = []byte("[]")
				}
				diskGroupID := group.ID
				pendingBatch = append(pendingBatch, db.ApprovalQueueItem{
					MediaName:       pi.item.Title,
					MediaType:       string(pi.item.Type),
					ScoreDetails:    string(factorsJSON),
					SizeBytes:       pi.item.SizeBytes,
					Score:           pi.score,
					PosterURL:       pi.item.PosterURL,
					IntegrationID:   pi.item.IntegrationID,
					ExternalID:      pi.item.ExternalID,
					DiskGroupID:     &diskGroupID,
					Trigger:         db.TriggerEngine,
					CollectionGroup: pi.collectionGroup,
				})

				// Track this item as still-needed for post-loop reconciliation
				neededKeys[pi.item.Title+"|"+string(pi.item.Type)] = true

				bytesFreed += pi.item.SizeBytes
				atomic.AddInt64(&p.lastRunFlagged, 1)
				atomic.AddInt64(&p.lastRunFreedBytes, pi.item.SizeBytes)
				slog.Info("Engine action taken", "component", "poller",
					"media", pi.item.Title, "action", "queued_for_approval", "score", pi.score, "freed", pi.item.SizeBytes,
					"collectionGroup", pi.collectionGroup)
			} else {
				// Dry-run mode: queue through DeletionService with ForceDryRun + UpsertAudit
				diskGroupID := group.ID
				if err := p.reg.Deletion.QueueDeletion(services.DeleteJob{
					Client:          nil, // Dry-run never calls DeleteMediaItem; nil-safe in processJob()
					Item:            pi.item,
					Score:           pi.score,
					Factors:         pi.factors,
					Trigger:         db.TriggerEngine,
					RunStatsID:      runStatsID,
					DiskGroupID:     &diskGroupID,
					ForceDryRun:     true,
					UpsertAudit:     true,
					CollectionGroup: pi.collectionGroup,
				}); err != nil {
					slog.Warn("Deletion queue full, skipping dry-run item", "component", "poller", "item", pi.item.Title)
					continue
				}
				bytesFreed += pi.item.SizeBytes
				deletionsQueued++
				atomic.AddInt64(&p.lastRunFlagged, 1)
				atomic.AddInt64(&p.lastRunFreedBytes, pi.item.SizeBytes)
				slog.Info("Engine action taken", "component", "poller",
					"media", pi.item.Title, "action", db.ActionDryDelete, "score", pi.score, "freed", pi.item.SizeBytes,
					"collectionGroup", pi.collectionGroup)
			}
		}
	}

	// Flush collected approval-mode items in a single batch transaction.
	// This replaces N individual UpsertPending() calls with one BulkUpsertPending().
	if len(pendingBatch) > 0 {
		created, updated, batchErr := p.reg.Approval.BulkUpsertPending(pendingBatch)
		if batchErr != nil {
			slog.Error("Failed to batch upsert approval queue items", "component", "poller",
				"mount", group.MountPath, "batchSize", len(pendingBatch), "error", batchErr)
		} else {
			slog.Info("Batch upserted approval queue items", "component", "poller",
				"mount", group.MountPath, "created", created, "updated", updated)
		}
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
			"skippedCollectionProtected", skippedCollectionProtected,
			"bytesFreedSoFar", bytesFreed)
	}

	return deletionsQueued
}
