// Package orchestrator is the product loop: given fetched, enriched media and
// a disk group, it scores, filters, expands collections, and dispatches by
// mode. It does not own the poll timer, fetch, or cycle finalize.
//
// engine.Evaluate stays pure — this type is the I/O-adjacent dispatcher that
// sits between scoring and queues. Dependencies are small interfaces, not
// *services.Registry.
package orchestrator

import (
	"encoding/json"
	"errors"
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

// Approval is the approval-queue surface used during a disk-group evaluation.
// Satisfied by *services.ApprovalService.
type Approval interface {
	ClearQueueForDiskGroup(diskGroupID uint) (int, error)
	ListSnoozedKeys(diskGroupID uint) (map[string]bool, error)
	BulkUpsertPending(items []db.ApprovalQueueItem) (created int, updated int, err error)
	ReconcileQueue(diskGroupID uint, neededKeys map[string]bool) (int, error)
}

// Deletion is the live / dry-run enqueue surface used by dispatchByMode.
// Satisfied by *services.DeletionService.
type Deletion interface {
	QueueFromEngine(req services.EngineDeleteRequest) error
}

// IntegrationConfigs looks up per-integration flags (collection deletion,
// import exclusion). Satisfied by *services.IntegrationService.
type IntegrationConfigs interface {
	GetByID(id uint) (*db.IntegrationConfig, error)
}

// Sunset is the sunset-queue surface used by the sunset dispatch arm and
// post-admit escalate. Satisfied by *services.SunsetService.
type Sunset interface {
	ListSunsettedKeys(diskGroupID uint) (map[string]bool, error)
	BulkQueueSunset(items []db.SunsetQueueItem, deps services.SunsetDeps) (int, error)
	Escalate(diskGroupID uint, targetBytes int64, deps services.SunsetDeps) (int64, int, error)
}

// Publisher is the event-bus surface for ThresholdBreached / EngineError /
// SunsetMisconfigured. Satisfied by *events.EventBus.
type Publisher interface {
	Publish(event events.Event)
}

// Deps is the narrow constructor surface. Poller wires concrete services
// once; the disk group and per-cycle registry are arguments, not lookups.
type Deps struct {
	Approval     Approval
	Deletion     Deletion
	Integrations IntegrationConfigs
	Sunset       Sunset
	Bus          Publisher
	// SunsetDeps is the label/escalation handoff bag already used by
	// SunsetService. Registry is filled per evaluation from the fetch
	// result — do not set it here.
	SunsetDeps services.SunsetDeps
}

// Orchestrator is the named product loop: threshold → score → filter →
// expand collections → dispatch by mode (auto / approval / dry-run / sunset).
type Orchestrator struct {
	approval     Approval
	deletion     Deletion
	integrations IntegrationConfigs
	sunset       Sunset
	bus          Publisher
	sunsetDeps   services.SunsetDeps
}

// New constructs an Orchestrator from injected interfaces.
func New(deps Deps) *Orchestrator {
	return &Orchestrator{
		approval:     deps.Approval,
		deletion:     deps.Deletion,
		integrations: deps.Integrations,
		sunset:       deps.Sunset,
		bus:          deps.Bus,
		sunsetDeps:   deps.SunsetDeps,
	}
}

func (o *Orchestrator) sunsetDepsFor(registry *integrations.IntegrationRegistry) services.SunsetDeps {
	deps := o.sunsetDeps
	deps.Registry = registry
	return deps
}

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
	sunsettedKeys          map[string]bool
	expandedCollections    map[string]bool
	integrationConfigCache map[uint]*db.IntegrationConfig

	// Set when QueueFromEngine hits the in-memory cap. dispatchFiltered
	// stops sending more items for this disk group.
	queueFull bool
	// skipSunsetAdmit is set when ListSunsettedKeys fails. New holds are
	// skipped; escalate still runs. Step 3 also skips (held set unknown).
	skipSunsetAdmit bool
	// scoredCandidates is the full engine candidate list (not truncated
	// to the hold/action budget). Sunset escalate step 3 walks this set
	// so already-held prefix items cannot starve live extras.
	scoredCandidates []engine.EvaluatedItem
}

// EvaluateDiskGroup scores all media items on a disk group and, when the
// disk is above evaluateAt, admits candidates through the shared pipeline
// (score → filter → expand → dispatchByMode). Returns the number of items
// handed to the executor this cycle (live / dry-run jobs, plus sunset
// escalate releases). The acc accumulator collects per-run metrics across
// multiple disk group evaluations.
func (o *Orchestrator) EvaluateDiskGroup(acc *RunAccumulator, group db.DiskGroup, allItems []integrations.MediaItem, registry *integrations.IntegrationRegistry, runStatsID uint, prefs db.PreferenceSet, weights map[string]int, rules []db.CustomRule, evalCtx *engine.EvaluationContext) int {
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

	policy := PolicyFor(group)
	if !policy.OK {
		slog.Warn("Sunset mode skipped — sunset threshold not configured",
			"component", "poller", "mount", group.MountPath, "diskGroupID", group.ID)
		o.bus.Publish(events.SunsetMisconfiguredEvent{
			DiskGroupID: group.ID,
			MountPath:   group.MountPath,
		})
		return 0
	}

	if !policy.ShouldEvaluate(currentPct) {
		slog.Debug("Disk within evaluateAt, no action needed", "component", "poller",
			"mount", group.MountPath, "usedPct", fmt.Sprintf("%.1f", currentPct),
			"evaluateAt", policy.EvaluateAt, "mode", group.Mode)

		// Below evaluateAt: approval clears engine-queued holds. Sunset
		// keeps existing holds (household promise). Dry-run / auto no-op.
		if group.Mode != db.ModeSunset {
			if cleared, err := o.approval.ClearQueueForDiskGroup(group.ID); err != nil {
				slog.Error("Failed to clear approval queue for disk group",
					"component", "poller", "diskGroupID", group.ID, "error", err)
			} else if cleared > 0 {
				slog.Info("Approval queue cleared for disk group (below threshold)",
					"component", "poller", "mount", group.MountPath, "cleared", cleared)
			}
		}

		return 0
	}

	slog.Info("Disk evaluateAt breached, evaluating media", "component", "poller",
		"mount", group.MountPath, "currentPct", fmt.Sprintf("%.1f", currentPct),
		"evaluateAt", policy.EvaluateAt, "mode", group.Mode)

	if group.Mode != db.ModeSunset {
		o.bus.Publish(events.ThresholdBreachedEvent{
			MountPath:    group.MountPath,
			CurrentPct:   currentPct,
			ThresholdPct: group.ThresholdPct,
			TargetPct:    group.TargetPct,
		})
	}

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
		sunsettedKeys:          make(map[string]bool),
	}

	// Pre-fetch snoozed keys for O(1) lookup. Fail closed: a lookup error
	// must not queue deletions that the user has snoozed.
	snoozedKeys, snoozedErr := o.approval.ListSnoozedKeys(group.ID)
	if snoozedErr != nil {
		slog.Error("Failed to load snoozed keys, aborting disk-group evaluation",
			"component", "poller", "mount", group.MountPath, "error", snoozedErr)
		o.bus.Publish(events.EngineErrorEvent{
			Error: fmt.Sprintf("failed to load snoozed keys for %s: %v", group.MountPath, snoozedErr),
		})
		return 0
	}
	ectx.snoozedKeys = snoozedKeys

	if group.Mode == db.ModeSunset {
		sunsettedKeys, keysErr := o.sunset.ListSunsettedKeys(group.ID)
		if keysErr != nil {
			slog.Error("Failed to load sunsetted keys, skipping new sunset queueing",
				"component", "poller", "mount", group.MountPath, "error", keysErr)
			ectx.skipSunsetAdmit = true
		} else {
			ectx.sunsettedKeys = sunsettedKeys
		}
	}

	// Score and select candidates within the byte budget (action budget
	// for dry-run/approval/auto, hold budget for sunset).
	candidates, targetBytesToFree := o.scoreCandidates(ectx, currentPct, effectiveTotal, policy.BudgetFromPct())

	var deletionsQueued int
	if targetBytesToFree > 0 {
		filtered, skipStats := filterCandidates(candidates, ectx.snoozedKeys)
		deletionsQueued = o.dispatchFiltered(ectx, filtered, skipStats, targetBytesToFree)
	}

	// Duration-hold escalate: after Admit, free down to target (not
	// evaluateAt). Steps 1–2 release holds; step 3 live-admits unheld
	// candidates from the same score → filter → expand set.
	if policy.ShouldEscalate(currentPct) {
		slog.Warn("Sunset escalation triggered — disk exceeds critical",
			"component", "poller", "mount", group.MountPath,
			"currentPct", fmt.Sprintf("%.1f", currentPct),
			"criticalPct", policy.EscalateAt,
			"targetPct", policy.Target)

		targetBytes := policy.ActionBudgetBytes(currentPct, effectiveTotal)
		sunsetDeps := o.sunsetDepsFor(registry)
		sunsetDeps.SnoozedKeys = ectx.snoozedKeys
		freed, released, err := o.sunset.Escalate(group.ID, targetBytes, sunsetDeps)
		if err != nil {
			slog.Error("Sunset escalation failed", "component", "poller",
				"mount", group.MountPath, "error", err)
		}
		groupAcc.FreedBytes += freed

		remaining := targetBytes - freed
		var liveQueued int
		if remaining > 0 && !ectx.skipSunsetAdmit {
			liveQueued = o.dispatchSunsetEscalateLive(ectx, remaining)
		}
		return deletionsQueued + released + liveQueued
	}

	return deletionsQueued
}

// scoreCandidates filters items to the mount path, runs engine evaluation,
// and selects candidates within the byte budget. budgetFromPct is the
// preset binding (targetPct for action budget, sunsetPct for hold budget).
// Returns the selected candidates and the target bytes to free.
func (o *Orchestrator) scoreCandidates(ectx *evaluationContext, currentPct float64, effectiveTotal int64, budgetFromPct float64) ([]engine.EvaluatedItem, int64) {
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

	ectx.scoredCandidates = evalResult.Candidates

	targetBytesToFree := int64((currentPct - budgetFromPct) / 100.0 * float64(effectiveTotal))
	if targetBytesToFree <= 0 {
		slog.Warn("Target bytes to free is zero or negative, skipping evaluation",
			"component", "poller", "mount", ectx.group.MountPath,
			"currentPct", fmt.Sprintf("%.1f", currentPct),
			"budgetFromPct", budgetFromPct,
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
func (o *Orchestrator) expandCollections(ectx *evaluationContext, ev engine.EvaluatedItem, stats *skipStats) ([]processItem, bool) {
	items := []processItem{{item: ev.Item, score: ev.Score, factors: ev.Factors}}

	// Find collections where the source integration has collectionDeletion ON.
	var enabledCollections []string
	for _, colName := range ev.Item.Collections {
		sourceID, ok := ev.Item.CollectionSources[colName]
		if !ok {
			sourceID = ev.Item.IntegrationID
		}
		sourceCfg := o.getIntegrationConfig(ectx, sourceID)
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
// mode (auto, approval, dry-run, sunset). Returns (deletionsQueued, bytesFreed).
// Auto / approval / dry-run arms are unchanged; sunset is a new hold sink.
func (o *Orchestrator) dispatchByMode(ectx *evaluationContext, pi processItem, pendingBatch *[]db.ApprovalQueueItem, neededKeys map[string]bool, sunsetBatch *[]db.SunsetQueueItem) (int, int64) {
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
		if sourceCfg := o.getIntegrationConfig(ectx, pi.item.IntegrationID); sourceCfg != nil {
			addImportExclusion = sourceCfg.AddImportExclusion
		}

		if err := o.deletion.QueueFromEngine(services.EngineDeleteRequest{
			Client:             deleter,
			Item:               pi.item,
			Score:              pi.score,
			Factors:            pi.factors,
			RunStatsID:         ectx.runStatsID,
			DiskGroupID:        ectx.group.ID,
			CollectionGroup:    pi.collectionGroup,
			AddImportExclusion: addImportExclusion,
		}); err != nil {
			if errors.Is(err, services.ErrDeletionQueueFull) {
				ectx.queueFull = true
				ectx.groupAcc.QueueFullSkipped++
			}
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

		neededKeys[db.ItemKey(pi.item.IntegrationID, pi.item.ExternalID)] = true
		ectx.groupAcc.Candidates++
		ectx.groupAcc.FreedBytes += pi.item.SizeBytes

		slog.Info("Engine action taken", "component", "poller",
			"media", pi.item.Title, "action", "queued_for_approval", "score", pi.score, "freed", pi.item.SizeBytes,
			"collectionGroup", pi.collectionGroup)
		return 0, pi.item.SizeBytes

	case db.ModeSunset:
		if ectx.skipSunsetAdmit {
			return 0, 0
		}
		key := db.ItemKey(pi.item.IntegrationID, pi.item.ExternalID)
		if ectx.sunsettedKeys[key] {
			return 0, 0
		}

		factorsJSON, marshalErr := json.Marshal(pi.factors)
		if marshalErr != nil {
			slog.Error("Failed to marshal sunset candidate factors", "component", "poller",
				"mediaName", pi.item.Title, "error", marshalErr)
			return 0, 0
		}
		tmdbID := pi.item.TMDbID
		*sunsetBatch = append(*sunsetBatch, db.SunsetQueueItem{
			MediaName:       pi.item.Title,
			MediaType:       string(pi.item.Type),
			TmdbID:          &tmdbID,
			IntegrationID:   pi.item.IntegrationID,
			ExternalID:      pi.item.ExternalID,
			SizeBytes:       pi.item.SizeBytes,
			Score:           pi.score,
			ScoreDetails:    string(factorsJSON),
			PosterURL:       pi.item.PosterURL,
			DiskGroupID:     ectx.group.ID,
			CollectionGroup: pi.collectionGroup,
			Trigger:         db.TriggerEngine,
			DeletionDate:    time.Now().UTC().AddDate(0, 0, ectx.prefs.SunsetDays),
		})
		ectx.groupAcc.Candidates++

		slog.Info("Engine action taken", "component", "poller",
			"media", pi.item.Title, "action", "queued_for_sunset", "score", pi.score,
			"collectionGroup", pi.collectionGroup)
		return 0, pi.item.SizeBytes

	default:
		// Dry-run mode
		if err := o.deletion.QueueFromEngine(services.EngineDeleteRequest{
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
			if errors.Is(err, services.ErrDeletionQueueFull) {
				ectx.queueFull = true
				ectx.groupAcc.QueueFullSkipped++
			}
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

// flushSunsetBatch writes the sunset Admit batch. No-op when empty.
func (o *Orchestrator) flushSunsetBatch(ectx *evaluationContext, sunsetBatch []db.SunsetQueueItem) {
	if len(sunsetBatch) == 0 {
		return
	}
	created, err := o.sunset.BulkQueueSunset(sunsetBatch, o.sunsetDepsFor(ectx.registry))
	if err != nil {
		slog.Error("Failed to queue sunset items", "component", "poller",
			"mount", ectx.group.MountPath, "error", err)
		return
	}
	for _, item := range sunsetBatch {
		ectx.sunsettedKeys[db.ItemKey(item.IntegrationID, item.ExternalID)] = true
	}
	ectx.groupAcc.SunsetQueued += created
	slog.Info("Sunset items queued", "component", "poller",
		"mount", ectx.group.MountPath, "count", created)
}

// reconcileQueue flushes the pending approval batch and reconciles stale items.
func (o *Orchestrator) reconcileQueue(ectx *evaluationContext, pendingBatch []db.ApprovalQueueItem, neededKeys map[string]bool) {
	if len(pendingBatch) > 0 {
		created, updated, batchErr := o.approval.BulkUpsertPending(pendingBatch)
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
		if dismissed, reconcileErr := o.approval.ReconcileQueue(ectx.group.ID, neededKeys); reconcileErr != nil {
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
func (o *Orchestrator) dispatchFiltered(ectx *evaluationContext, filtered []engine.EvaluatedItem, stats skipStats, targetBytesToFree int64) int {
	var bytesFreed int64
	var deletionsQueued int
	var pendingBatch []db.ApprovalQueueItem
	var sunsetBatch []db.SunsetQueueItem
	neededKeys := make(map[string]bool)

	for i, ev := range filtered {
		if ectx.queueFull {
			remaining := len(filtered) - i
			ectx.groupAcc.QueueFullSkipped += remaining
			slog.Warn("Deletion queue full — pausing further dispatch for this disk group",
				"component", "poller", "mount", ectx.group.MountPath,
				"skipped", remaining, "queueFullSkipped", ectx.groupAcc.QueueFullSkipped)
			break
		}
		if bytesFreed >= targetBytesToFree {
			break
		}

		slog.Debug("Deletion candidate", "component", "poller",
			"media", ev.Item.Title, "score", fmt.Sprintf("%.4f", ev.Score),
			"size", ev.Item.SizeBytes, "reason", ev.Reason)

		// Expand collections if applicable
		itemsToProcess, skipped := o.expandCollections(ectx, ev, &stats)
		if skipped {
			continue
		}

		// Dispatch each item through the appropriate mode
		for j, pi := range itemsToProcess {
			if ectx.queueFull {
				ectx.groupAcc.QueueFullSkipped += len(itemsToProcess) - j
				break
			}
			queued, freed := o.dispatchByMode(ectx, pi, &pendingBatch, neededKeys, &sunsetBatch)
			deletionsQueued += queued
			bytesFreed += freed
		}
	}

	// Flush sunset holds, then approval batch / reconcile.
	o.flushSunsetBatch(ectx, sunsetBatch)
	o.reconcileQueue(ectx, pendingBatch, neededKeys)

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

// dispatchSunsetEscalateLive is escalate ladder step 3: if releasing holds
// did not meet the action budget, admit additional candidates from the same
// scored set that are not already held, as immediate live deletes. Same
// filter/expand as Admit. EnqueuedMode stays sunset so a later mode change
// still cancels these jobs.
func (o *Orchestrator) dispatchSunsetEscalateLive(ectx *evaluationContext, remainingBytes int64) int {
	if remainingBytes <= 0 || ectx.skipSunsetAdmit {
		return 0
	}

	filtered, stats := filterCandidates(ectx.scoredCandidates, ectx.snoozedKeys)
	ectx.expandedCollections = make(map[string]bool)

	var bytesFreed int64
	var queued int

	for i, ev := range filtered {
		if ectx.queueFull {
			ectx.groupAcc.QueueFullSkipped += len(filtered) - i
			break
		}
		if bytesFreed >= remainingBytes {
			break
		}

		itemsToProcess, skipped := o.expandCollections(ectx, ev, &stats)
		if skipped {
			continue
		}

		for j, pi := range itemsToProcess {
			if ectx.queueFull {
				ectx.groupAcc.QueueFullSkipped += len(itemsToProcess) - j
				break
			}
			if bytesFreed >= remainingBytes {
				break
			}
			key := db.ItemKey(pi.item.IntegrationID, pi.item.ExternalID)
			if ectx.sunsettedKeys[key] {
				continue
			}
			n, freed := o.queueSunsetEscalateLive(ectx, pi)
			queued += n
			bytesFreed += freed
		}
	}

	if queued > 0 {
		o.bus.Publish(events.SunsetEscalatedEvent{
			DiskGroupID:  ectx.group.ID,
			ItemsExpired: queued,
			BytesFreed:   bytesFreed,
		})
		slog.Warn("Sunset escalation step 3 — live-admitted unheld candidates",
			"component", "poller", "mount", ectx.group.MountPath,
			"queued", queued, "bytes", bytesFreed)
	}

	return queued
}

// queueSunsetEscalateLive enqueues one unheld candidate as a live engine
// delete without flipping the group to auto. Auto / approval / dry-run
// dispatch arms are unchanged.
func (o *Orchestrator) queueSunsetEscalateLive(ectx *evaluationContext, pi processItem) (int, int64) {
	if ectx.registry == nil {
		return 0, 0
	}
	deleter, err := ectx.registry.Deleter(pi.item.IntegrationID)
	if err != nil {
		slog.Error("Integration not registered as MediaDeleter", "component", "poller",
			"integrationId", pi.item.IntegrationID, "error", err)
		return 0, 0
	}

	addImportExclusion := true
	if sourceCfg := o.getIntegrationConfig(ectx, pi.item.IntegrationID); sourceCfg != nil {
		addImportExclusion = sourceCfg.AddImportExclusion
	}

	if err := o.deletion.QueueFromEngine(services.EngineDeleteRequest{
		Client:             deleter,
		Item:               pi.item,
		Score:              pi.score,
		Factors:            pi.factors,
		RunStatsID:         ectx.runStatsID,
		DiskGroupID:        ectx.group.ID,
		CollectionGroup:    pi.collectionGroup,
		AddImportExclusion: addImportExclusion,
		EnqueuedMode:       db.ModeSunset,
	}); err != nil {
		if errors.Is(err, services.ErrDeletionQueueFull) {
			ectx.queueFull = true
			ectx.groupAcc.QueueFullSkipped++
		}
		return 0, 0
	}
	ectx.groupAcc.Candidates++
	ectx.groupAcc.FreedBytes += pi.item.SizeBytes

	slog.Info("Engine action taken", "component", "poller",
		"media", pi.item.Title, "action", "sunset_escalate_live", "score", pi.score,
		"freed", pi.item.SizeBytes, "collectionGroup", pi.collectionGroup)
	return 1, pi.item.SizeBytes
}

// getIntegrationConfig returns the cached integration config, fetching from
// the service if not yet cached.
func (o *Orchestrator) getIntegrationConfig(ectx *evaluationContext, id uint) *db.IntegrationConfig {
	if cfg, ok := ectx.integrationConfigCache[id]; ok {
		return cfg
	}
	cfg, err := o.integrations.GetByID(id)
	if err != nil {
		return nil
	}
	ectx.integrationConfigCache[id] = cfg
	return cfg
}

// normalizePath converts backslash path separators to forward slashes for
// consistent cross-platform path comparison. Duplicated from poller (fetch /
// mount matching still live there) so this package does not import poller.
func normalizePath(p string) string {
	return strings.ReplaceAll(p, `\`, "/")
}
