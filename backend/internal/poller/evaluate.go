// Package poller orchestrates periodic media library polling and capacity evaluation.
package poller

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
	"capacitarr/internal/services"
)

// processItem groups a media item with its evaluation metadata for dispatch.
// Used by expandCollections and dispatchByMode to carry context through the pipeline.
type processItem struct {
	item            integrations.MediaItem
	score           float64
	factors         []engine.ScoreFactor
	collectionGroup string // non-empty if part of a collection
}

// evaluationContext bundles all per-cycle state needed by the sub-functions
// extracted from the original god function. Passed by pointer to avoid copying.
type evaluationContext struct {
	group      db.DiskGroup
	groupAcc   *GroupAccumulator
	allItems   []integrations.MediaItem
	registry   *integrations.IntegrationRegistry
	runStatsID uint
	prefs      db.PreferenceSet
	weights    map[string]int
	rules      []db.CustomRule
	evalCtx    *engine.EvaluationContext

	// Lazy-initialized caches shared across sub-functions.
	snoozedKeys            map[string]bool
	expandedCollections    map[string]bool
	integrationConfigCache map[uint]*db.IntegrationConfig
}

// evaluateDiskGroup scores all media items on a disk group and, when the
// disk is above threshold, queues candidates for deletion or approval.
// Returns the number of items queued for deletion. The acc accumulator
// collects per-run metrics across multiple disk group evaluations.
func (p *Poller) evaluateDiskGroup(acc *RunAccumulator, group db.DiskGroup, allItems []integrations.MediaItem, registry *integrations.IntegrationRegistry, runStatsID uint, prefs db.PreferenceSet, weights map[string]int, rules []db.CustomRule, evalCtx *engine.EvaluationContext) int {
	effectiveTotal := group.EffectiveTotalBytes()
	if effectiveTotal == 0 {
		slog.Warn("Disk group effective total is 0, skipping evaluation",
			"component", "poller", "mount", group.MountPath,
			"totalBytes", group.TotalBytes, "override", group.TotalBytesOverride)
		return 0
	}
	currentPct := float64(group.UsedBytes) / float64(effectiveTotal) * 100

	// Get or create a per-group accumulator and record disk usage info.
	groupAcc := acc.GetOrCreate(group.ID, group.MountPath, group.Mode)
	groupAcc.DiskUsagePct = currentPct
	groupAcc.DiskThreshold = group.ThresholdPct
	groupAcc.DiskTargetPct = group.TargetPct

	// ── Sunset mode: special threshold handling ─────────────────────────
	if group.Mode == db.ModeSunset {
		return p.evaluateSunsetMode(acc, group, allItems, registry, runStatsID, prefs, weights, rules, evalCtx, effectiveTotal, currentPct)
	}

	// ── Standard modes: dry-run, approval, auto ─────────────────────────
	if currentPct < group.ThresholdPct {
		slog.Debug("Disk within threshold, no action needed", "component", "poller",
			"mount", group.MountPath, "usedPct", fmt.Sprintf("%.1f", currentPct),
			"threshold", group.ThresholdPct, "mode", group.Mode)

		// Clear stale approval queue items for this specific disk group.
		if cleared, err := p.reg.Approval.ClearQueueForDiskGroup(group.ID); err != nil {
			slog.Error("Failed to clear approval queue for disk group",
				"component", "poller", "diskGroupID", group.ID, "error", err)
		} else if cleared > 0 {
			slog.Info("Approval queue cleared for disk group (below threshold)",
				"component", "poller", "mount", group.MountPath, "cleared", cleared)
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

	// Build the shared evaluation context for sub-functions.
	ectx := &evaluationContext{
		group:                  group,
		groupAcc:               groupAcc,
		allItems:               allItems,
		registry:               registry,
		runStatsID:             runStatsID,
		prefs:                  prefs,
		weights:                weights,
		rules:                  rules,
		evalCtx:                evalCtx,
		expandedCollections:    make(map[string]bool),
		integrationConfigCache: make(map[uint]*db.IntegrationConfig),
	}

	// Pre-fetch snoozed keys for O(1) lookup. Fail closed: a lookup error
	// must not queue deletions that the user has snoozed.
	snoozedKeys, snoozedErr := p.reg.Approval.ListSnoozedKeys(group.ID)
	if snoozedErr != nil {
		slog.Error("Failed to load snoozed keys, aborting disk-group evaluation",
			"component", "poller", "mount", group.MountPath, "error", snoozedErr)
		p.reg.Bus.Publish(events.EngineErrorEvent{
			Error: fmt.Sprintf("failed to load snoozed keys for %s: %v", group.MountPath, snoozedErr),
		})
		return 0
	}
	ectx.snoozedKeys = snoozedKeys

	// Score and select candidates within the byte budget.
	candidates, targetBytesToFree := p.scoreCandidates(ectx, currentPct, effectiveTotal)
	if targetBytesToFree <= 0 {
		return 0
	}

	// Filter candidates: dedup shows vs seasons, skip snoozed, skip zero-score.
	filtered, skipStats := filterCandidates(candidates, ectx.snoozedKeys)

	// Expand collections, dispatch by mode, and reconcile the queue.
	return p.dispatchFiltered(ectx, filtered, skipStats, targetBytesToFree)
}

// scoreCandidates filters items to the mount path, runs engine evaluation,
// and selects candidates within the byte budget. Returns the selected
// candidates and the target bytes to free.
func (p *Poller) scoreCandidates(ectx *evaluationContext, currentPct float64, effectiveTotal int64) ([]engine.EvaluatedItem, int64) {
	// Filter items on this mount
	normalizedMount := normalizePath(ectx.group.MountPath)
	var diskItems []integrations.MediaItem
	for _, item := range ectx.allItems {
		if strings.HasPrefix(normalizePath(item.Path), normalizedMount) {
			diskItems = append(diskItems, item)
		}
	}

	if len(diskItems) == 0 {
		slog.Warn("No items matched disk mount path — approval queue cannot be populated",
			"component", "poller", "mount", ectx.group.MountPath,
			"normalizedMount", normalizedMount, "totalItems", len(ectx.allItems))
		if len(ectx.allItems) > 0 {
			sampleCount := 3
			if len(ectx.allItems) < sampleCount {
				sampleCount = len(ectx.allItems)
			}
			for i := 0; i < sampleCount; i++ {
				slog.Debug("Sample item path for mount mismatch diagnosis",
					"component", "poller", "itemPath", normalizePath(ectx.allItems[i].Path),
					"mount", normalizedMount)
			}
		}
	}
	slog.Debug("Items on disk mount", "component", "poller",
		"mount", ectx.group.MountPath, "itemCount", len(diskItems))

	// Use the extracted Evaluator for scoring + categorization
	evaluator := engine.NewEvaluator()
	evalResult := evaluator.Evaluate(diskItems, ectx.weights, ectx.rules, ectx.prefs.TiebreakerMethod, ectx.evalCtx)
	ectx.groupAcc.Evaluated += int64(evalResult.TotalCount)
	ectx.groupAcc.Protected += int64(len(evalResult.Protected))

	slog.Debug("Evaluation summary", "component", "poller",
		"mount", ectx.group.MountPath,
		"evaluated", evalResult.TotalCount,
		"protected", len(evalResult.Protected),
		"candidates", len(evalResult.Candidates))

	targetBytesToFree := int64((currentPct - ectx.group.TargetPct) / 100.0 * float64(effectiveTotal))
	if targetBytesToFree <= 0 {
		slog.Warn("Target bytes to free is zero or negative, skipping evaluation",
			"component", "poller", "mount", ectx.group.MountPath,
			"currentPct", fmt.Sprintf("%.1f", currentPct),
			"targetPct", ectx.group.TargetPct,
			"targetBytesToFree", targetBytesToFree)
		return nil, 0
	}

	candidates := evalResult.CandidatesForDeletion(targetBytesToFree)

	slog.Info("Candidate selection for approval/deletion", "component", "poller",
		"mount", ectx.group.MountPath,
		"mode", ectx.group.Mode,
		"totalCandidates", len(evalResult.Candidates),
		"selectedCandidates", len(candidates),
		"targetBytesToFree", targetBytesToFree)

	return candidates, targetBytesToFree
}

// skipStats tracks how many candidates were skipped for each reason.
type skipStats struct {
	zeroScore           int
	dedup               int
	snoozed             int
	collectionProtected int
}

// filterCandidates applies show/season dedup, snooze-skip, and zero-score
// removal. Returns the filtered list and skip statistics.
func filterCandidates(candidates []engine.EvaluatedItem, snoozedKeys map[string]bool) ([]engine.EvaluatedItem, skipStats) {
	var stats skipStats

	// Pre-build set of shows that have season-level entries.
	showsWithSeasons := make(map[string]bool)
	for _, ev := range candidates {
		if ev.Item.Type == integrations.MediaTypeSeason && ev.Item.ShowTitle != "" {
			showsWithSeasons[ev.Item.ShowTitle] = true
		}
	}

	filtered := make([]engine.EvaluatedItem, 0, len(candidates))
	for _, ev := range candidates {
		if ev.IsProtected || ev.Score <= 0 {
			stats.zeroScore++
			continue
		}

		// Dedup: skip show-level entries when season entries exist.
		if ev.Item.Type == integrations.MediaTypeShow && showsWithSeasons[ev.Item.Title] {
			stats.dedup++
			continue
		}

		// Skip snoozed items.
		snoozedKey := db.MediaKey(ev.Item.Title, string(ev.Item.Type))
		if snoozedKeys[snoozedKey] {
			stats.snoozed++
			slog.Debug("Skipping snoozed item", "component", "poller", "media", ev.Item.Title)
			continue
		}

		filtered = append(filtered, ev)
	}

	return filtered, stats
}

// expandCollections resolves collection membership for a single candidate.
// When collection deletion is enabled, the trigger item is expanded into all
// collection members. Returns the items to process and whether the candidate
// was skipped (due to protection or snooze on a member).
func (p *Poller) expandCollections(ectx *evaluationContext, ev engine.EvaluatedItem, stats *skipStats) ([]processItem, bool) {
	items := []processItem{{item: ev.Item, score: ev.Score, factors: ev.Factors}}

	// Find collections where the source integration has collectionDeletion ON.
	var enabledCollections []string
	for _, colName := range ev.Item.Collections {
		sourceID, ok := ev.Item.CollectionSources[colName]
		if !ok {
			sourceID = ev.Item.IntegrationID
		}
		sourceCfg := p.getIntegrationConfig(ectx, sourceID)
		if sourceCfg != nil && sourceCfg.CollectionDeletion {
			enabledCollections = append(enabledCollections, colName)
		}
	}

	if len(enabledCollections) == 0 {
		return items, false
	}

	// Check if ALL enabled collections were already expanded
	allExpanded := true
	for _, colName := range enabledCollections {
		if !ectx.expandedCollections[colName] {
			allExpanded = false
			break
		}
	}
	if allExpanded {
		slog.Debug("Skipping already-expanded collection member", "component", "poller",
			"media", ev.Item.Title, "collections", strings.Join(enabledCollections, ", "))
		return nil, true // skip this candidate entirely
	}

	// Resolve members for each enabled collection, merging results.
	memberDedup := make(map[string]bool)
	var allMembers []integrations.MediaItem
	var resolvedCollections []string

	for _, colName := range enabledCollections {
		if ectx.expandedCollections[colName] {
			continue
		}

		var members []integrations.MediaItem

		// Strategy 1: Use CollectionResolver if available
		if resolver, ok := ectx.registry.CollectionResolver(ev.Item.IntegrationID); ok {
			resolved, resolveErr := resolver.ResolveCollectionMembers(ev.Item)
			if resolveErr != nil {
				slog.Error("Collection resolution failed, falling back to allItems scan", "component", "poller",
					"media", ev.Item.Title, "collection", colName, "error", resolveErr)
			} else if len(resolved) > 1 {
				members = resolved
			}
		}

		// Strategy 2: Scan allItems for siblings with the same collection name.
		if len(members) == 0 {
			for _, ai := range ectx.allItems {
				for _, c := range ai.Collections {
					if c == colName {
						members = append(members, ai)
						break
					}
				}
			}
		}

		if len(members) <= 1 {
			ectx.expandedCollections[colName] = true
			continue
		}

		for _, member := range members {
			key := member.ExternalID + "|" + fmt.Sprintf("%d", member.IntegrationID)
			if !memberDedup[key] {
				memberDedup[key] = true
				allMembers = append(allMembers, member)
			}
		}
		resolvedCollections = append(resolvedCollections, colName)
		ectx.expandedCollections[colName] = true
	}

	if len(allMembers) <= 1 {
		return items, false
	}

	collectionGroupName := strings.Join(resolvedCollections, ", ")

	// Check if any member is protected by always_keep rules
	for _, member := range allMembers {
		isProtected, _, _, _ := engine.ApplyRulesExported(member, ectx.rules)
		if isProtected {
			slog.Info("Collection skipped — member has always_keep rule", "component", "poller",
				"trigger", ev.Item.Title, "collection", collectionGroupName, "protectedMember", member.Title)
			stats.collectionProtected++
			return nil, true
		}
	}

	// Check if any member is snoozed
	for _, member := range allMembers {
		memberSnoozedKey := db.MediaKey(member.Title, string(member.Type))
		if ectx.snoozedKeys[memberSnoozedKey] {
			slog.Info("Collection skipped — member is snoozed", "component", "poller",
				"trigger", ev.Item.Title, "collection", collectionGroupName, "snoozedMember", member.Title)
			stats.snoozed++
			return nil, true
		}
	}

	// Expand: replace the single trigger item with all collection members
	expanded := make([]processItem, 0, len(allMembers))
	for _, member := range allMembers {
		expanded = append(expanded, processItem{
			item:            member,
			score:           ev.Score,
			factors:         ev.Factors,
			collectionGroup: collectionGroupName,
		})
	}

	ectx.groupAcc.Collections++
	slog.Info("Collection expanded for deletion", "component", "poller",
		"trigger", ev.Item.Title, "collections", collectionGroupName, "memberCount", len(allMembers))

	return expanded, false
}

// dispatchByMode routes a single processItem through the appropriate execution
// mode (auto, approval, dry-run). Returns (deletionsQueued, bytesFreed).
func (p *Poller) dispatchByMode(ectx *evaluationContext, pi processItem, pendingBatch *[]db.ApprovalQueueItem, neededKeys map[string]bool) (int, int64) {
	switch ectx.group.Mode {
	case db.ModeAuto:
		deleter, err := ectx.registry.Deleter(pi.item.IntegrationID)
		if err != nil {
			slog.Error("Integration not registered as MediaDeleter", "component", "poller",
				"integrationId", pi.item.IntegrationID, "error", err)
			return 0, 0
		}

		// Look up integration config for AddImportExclusion setting
		addImportExclusion := true // safe default
		if sourceCfg := p.getIntegrationConfig(ectx, pi.item.IntegrationID); sourceCfg != nil {
			addImportExclusion = sourceCfg.AddImportExclusion
		}

		if err := p.reg.Deletion.QueueFromEngine(services.EngineDeleteRequest{
			Client:             deleter,
			Item:               pi.item,
			Score:              pi.score,
			Factors:            pi.factors,
			RunStatsID:         ectx.runStatsID,
			DiskGroupID:        ectx.group.ID,
			CollectionGroup:    pi.collectionGroup,
			AddImportExclusion: addImportExclusion,
		}); err != nil {
			slog.Warn("Deletion queue full, skipping item", "component", "poller", "item", pi.item.Title)
			return 0, 0
		}
		ectx.groupAcc.Candidates++
		ectx.groupAcc.FreedBytes += pi.item.SizeBytes
		return 1, pi.item.SizeBytes

	case db.ModeApproval:
		factorsJSON, marshalErr := json.Marshal(pi.factors)
		if marshalErr != nil {
			slog.Error("Failed to marshal score factors", "component", "poller", "error", marshalErr)
			factorsJSON = []byte("[]")
		}
		diskGroupID := ectx.group.ID
		*pendingBatch = append(*pendingBatch, db.ApprovalQueueItem{
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

		neededKeys[db.MediaKey(pi.item.Title, string(pi.item.Type))] = true
		ectx.groupAcc.Candidates++
		ectx.groupAcc.FreedBytes += pi.item.SizeBytes

		slog.Info("Engine action taken", "component", "poller",
			"media", pi.item.Title, "action", "queued_for_approval", "score", pi.score, "freed", pi.item.SizeBytes,
			"collectionGroup", pi.collectionGroup)
		return 0, pi.item.SizeBytes

	default:
		// Dry-run mode
		if err := p.reg.Deletion.QueueFromEngine(services.EngineDeleteRequest{
			Client:          nil,
			Item:            pi.item,
			Score:           pi.score,
			Factors:         pi.factors,
			RunStatsID:      ectx.runStatsID,
			DiskGroupID:     ectx.group.ID,
			CollectionGroup: pi.collectionGroup,
			ForceDryRun:     true,
			UpsertAudit:     true,
		}); err != nil {
			slog.Warn("Deletion queue full, skipping dry-run item", "component", "poller", "item", pi.item.Title)
			return 0, 0
		}
		ectx.groupAcc.Candidates++
		ectx.groupAcc.FreedBytes += pi.item.SizeBytes

		slog.Info("Engine action taken", "component", "poller",
			"media", pi.item.Title, "action", db.ActionDryDelete, "score", pi.score, "freed", pi.item.SizeBytes,
			"collectionGroup", pi.collectionGroup)
		return 1, pi.item.SizeBytes
	}
}

// reconcileQueue flushes the pending approval batch and reconciles stale items.
func (p *Poller) reconcileQueue(ectx *evaluationContext, pendingBatch []db.ApprovalQueueItem, neededKeys map[string]bool) {
	if len(pendingBatch) > 0 {
		created, updated, batchErr := p.reg.Approval.BulkUpsertPending(pendingBatch)
		if batchErr != nil {
			slog.Error("Failed to batch upsert approval queue items", "component", "poller",
				"mount", ectx.group.MountPath, "batchSize", len(pendingBatch), "error", batchErr)
		} else {
			slog.Info("Batch upserted approval queue items", "component", "poller",
				"mount", ectx.group.MountPath, "created", created, "updated", updated)
		}
	}

	// Per-cycle queue reconciliation: dismiss stale pending items.
	if ectx.group.Mode == db.ModeApproval {
		if dismissed, reconcileErr := p.reg.Approval.ReconcileQueue(ectx.group.ID, neededKeys); reconcileErr != nil {
			slog.Error("Failed to reconcile approval queue", "component", "poller",
				"mount", ectx.group.MountPath, "error", reconcileErr)
		} else if dismissed > 0 {
			slog.Info("Approval queue reconciled", "component", "poller",
				"mount", ectx.group.MountPath, "dismissed", dismissed)
		}
	}
}

// dispatchFiltered runs collection expansion, mode dispatch, and queue
// reconciliation for all filtered candidates. Returns deletions queued.
func (p *Poller) dispatchFiltered(ectx *evaluationContext, filtered []engine.EvaluatedItem, stats skipStats, targetBytesToFree int64) int {
	var bytesFreed int64
	var deletionsQueued int
	var pendingBatch []db.ApprovalQueueItem
	neededKeys := make(map[string]bool)

	for _, ev := range filtered {
		if bytesFreed >= targetBytesToFree {
			break
		}

		slog.Debug("Deletion candidate", "component", "poller",
			"media", ev.Item.Title, "score", fmt.Sprintf("%.4f", ev.Score),
			"size", ev.Item.SizeBytes, "reason", ev.Reason)

		// Expand collections if applicable
		itemsToProcess, skipped := p.expandCollections(ectx, ev, &stats)
		if skipped {
			continue
		}

		// Dispatch each item through the appropriate mode
		for _, pi := range itemsToProcess {
			queued, freed := p.dispatchByMode(ectx, pi, &pendingBatch, neededKeys)
			deletionsQueued += queued
			bytesFreed += freed
		}
	}

	// Flush approval batch and reconcile queue
	p.reconcileQueue(ectx, pendingBatch, neededKeys)

	// Diagnostic summary
	if len(filtered) > 0 && deletionsQueued == 0 && ectx.groupAcc.Candidates == 0 {
		slog.Warn("All candidates were skipped — nothing queued for approval/deletion",
			"component", "poller", "mount", ectx.group.MountPath,
			"mode", ectx.group.Mode,
			"candidates", len(filtered),
			"skippedZeroScore", stats.zeroScore,
			"skippedDedup", stats.dedup,
			"skippedSnoozed", stats.snoozed,
			"skippedCollectionProtected", stats.collectionProtected,
			"bytesFreedSoFar", bytesFreed)
	}

	return deletionsQueued
}

// getIntegrationConfig returns the cached integration config, fetching from
// the service if not yet cached.
func (p *Poller) getIntegrationConfig(ectx *evaluationContext, id uint) *db.IntegrationConfig {
	if cfg, ok := ectx.integrationConfigCache[id]; ok {
		return cfg
	}
	cfg, err := p.reg.Integration.GetByID(id)
	if err != nil {
		return nil
	}
	ectx.integrationConfigCache[id] = cfg
	return cfg
}

// evaluateSunsetMode handles the sunset-mode disk group evaluation.
// Both steps run independently each cycle:
//  1. Queue: if currentPct >= sunsetPct, score items and add to sunset_queue
//  2. Escalate: if currentPct >= criticalPct, force-expire from the queue to free space
//
// This ensures items are always marked for sunset once past the sunset threshold,
// even when the disk is simultaneously past critical. Escalation then processes
// items that were just queued (or already existed) from the sunset queue.
func (p *Poller) evaluateSunsetMode(acc *RunAccumulator, group db.DiskGroup, allItems []integrations.MediaItem, registry *integrations.IntegrationRegistry, _ uint, prefs db.PreferenceSet, weights map[string]int, rules []db.CustomRule, evalCtx *engine.EvaluationContext, effectiveTotal int64, currentPct float64) int {
	// Get the per-group accumulator (already created by evaluateDiskGroup
	// before dispatching to sunset mode).
	groupAcc := acc.GetOrCreate(group.ID, group.MountPath, group.Mode)

	// Validation: sunsetPct must be configured
	if group.SunsetPct == nil {
		slog.Warn("Sunset mode skipped — sunset threshold not configured",
			"component", "poller", "mount", group.MountPath, "diskGroupID", group.ID)
		p.reg.Bus.Publish(events.SunsetMisconfiguredEvent{
			DiskGroupID: group.ID,
			MountPath:   group.MountPath,
		})
		return 0
	}

	sunsetPct := *group.SunsetPct

	sunsetDeps := services.SunsetDeps{
		Registry:      registry,
		Deletion:      p.reg.Deletion,
		Engine:        p.reg.Engine,
		Settings:      p.reg.Settings,
		Preview:       p.reg.Preview,
		PosterOverlay: p.reg.PosterOverlay,
		Mapping:       p.reg.Mapping,
	}

	// Fail closed: without a reliable snooze set we must not queue new
	// sunset items or escalate existing ones (escalation would delete
	// media the user may have snoozed after it entered the sunset queue).
	snoozedKeys, snoozedErr := p.reg.Approval.ListSnoozedKeys(group.ID)
	if snoozedErr != nil {
		slog.Error("Failed to load snoozed keys, aborting sunset evaluation",
			"component", "poller", "mount", group.MountPath, "error", snoozedErr)
		p.reg.Bus.Publish(events.EngineErrorEvent{
			Error: fmt.Sprintf("failed to load snoozed keys for %s: %v", group.MountPath, snoozedErr),
		})
		return 0
	}

	// Step 1: Queue items to sunset if sunsetPct is breached
	if currentPct >= sunsetPct {
		slog.Info("Sunset threshold breached, evaluating media for sunset queue", "component", "poller",
			"mount", group.MountPath, "currentPct", fmt.Sprintf("%.1f", currentPct),
			"sunsetPct", sunsetPct)

		// Filter items on this mount
		normalizedMount := normalizePath(group.MountPath)
		var diskItems []integrations.MediaItem
		for _, item := range allItems {
			if strings.HasPrefix(normalizePath(item.Path), normalizedMount) {
				diskItems = append(diskItems, item)
			}
		}

		// Score items
		evaluator := engine.NewEvaluator()
		evalResult := evaluator.Evaluate(diskItems, weights, rules, prefs.TiebreakerMethod, evalCtx)
		groupAcc.Evaluated += int64(evalResult.TotalCount)
		groupAcc.Protected += int64(len(evalResult.Protected))

		// Calculate how much to sunset (based on currentPct → sunsetPct range)
		targetBytesToFree := int64((currentPct - sunsetPct) / 100.0 * float64(effectiveTotal))
		if targetBytesToFree > 0 {
			candidates := evalResult.CandidatesForDeletion(targetBytesToFree)

			// Pre-build set of already-sunsetted items for dedup. On lookup
			// failure we skip new queueing (cannot dedup) but still escalate.
			sunsettedKeys, keysErr := p.reg.Sunset.ListSunsettedKeys(group.ID)
			if keysErr != nil {
				slog.Error("Failed to load sunsetted keys, skipping new sunset queueing",
					"component", "poller", "mount", group.MountPath, "error", keysErr)
			} else {
				// Calculate sunset deletion date
				deletionDate := time.Now().UTC().AddDate(0, 0, prefs.SunsetDays)

				var sunsetItems []db.SunsetQueueItem
				for _, candidate := range candidates {
					key := db.MediaKey(candidate.Item.Title, string(candidate.Item.Type))
					if sunsettedKeys[key] {
						continue // Already in sunset queue
					}
					if snoozedKeys[key] {
						continue // User snoozed this item
					}

					factorsJSON, marshalErr := json.Marshal(candidate.Factors)
					if marshalErr != nil {
						slog.Error("Failed to marshal sunset candidate factors", "component", "poller",
							"mediaName", candidate.Item.Title, "error", marshalErr)
						continue
					}
					sunsetItems = append(sunsetItems, db.SunsetQueueItem{
						MediaName:       candidate.Item.Title,
						MediaType:       string(candidate.Item.Type),
						TmdbID:          &candidate.Item.TMDbID,
						IntegrationID:   candidate.Item.IntegrationID,
						ExternalID:      candidate.Item.ExternalID,
						SizeBytes:       candidate.Item.SizeBytes,
						Score:           candidate.Score,
						ScoreDetails:    string(factorsJSON),
						PosterURL:       candidate.Item.PosterURL,
						DiskGroupID:     group.ID,
						CollectionGroup: "", // Collection groups are handled by approval/auto mode expansion; sunset evaluates items individually
						Trigger:         db.TriggerEngine,
						DeletionDate:    deletionDate,
					})
				}

				if len(sunsetItems) > 0 {
					created, err := p.reg.Sunset.BulkQueueSunset(sunsetItems, sunsetDeps)
					if err != nil {
						slog.Error("Failed to queue sunset items", "component", "poller",
							"mount", group.MountPath, "error", err)
					} else {
						slog.Info("Sunset items queued", "component", "poller",
							"mount", group.MountPath, "count", created,
							"deletionDate", deletionDate.Format("2006-01-02"))
						groupAcc.Candidates += int64(created)
						groupAcc.SunsetQueued += created
					}
				}
			}
		}
	} else {
		slog.Debug("Disk within sunset threshold, no action needed", "component", "poller",
			"mount", group.MountPath, "usedPct", fmt.Sprintf("%.1f", currentPct),
			"sunsetPct", sunsetPct)
	}

	// Step 2: Escalate if criticalPct is also breached (independent of step 1)
	if currentPct >= group.ThresholdPct {
		slog.Warn("Sunset escalation triggered — disk exceeds critical",
			"component", "poller", "mount", group.MountPath,
			"currentPct", fmt.Sprintf("%.1f", currentPct),
			"criticalPct", group.ThresholdPct,
			"targetPct", group.TargetPct)

		// Calculate bytes to free down to targetPct (NOT sunsetPct — preserves queue)
		targetBytes := int64((currentPct - group.TargetPct) / 100.0 * float64(effectiveTotal))
		freed, err := p.reg.Sunset.Escalate(group.ID, targetBytes, sunsetDeps)
		if err != nil {
			slog.Error("Sunset escalation failed", "component", "poller",
				"mount", group.MountPath, "error", err)
		}
		groupAcc.FreedBytes += freed
	}

	return 0 // Sunset mode doesn't queue immediate deletions
}
