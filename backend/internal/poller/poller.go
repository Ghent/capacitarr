package poller

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
	"capacitarr/internal/notifications"
	"capacitarr/internal/services"
)

// RunAccumulator collects per-cycle metrics across multiple disk group
// evaluations within a single engine run. Each disk group gets its own
// GroupAccumulator. Not shared across goroutines — the poller runs
// single-threaded.
type RunAccumulator struct {
	Groups map[uint]*GroupAccumulator
}

// NewRunAccumulator creates a RunAccumulator with an initialized map.
func NewRunAccumulator() *RunAccumulator {
	return &RunAccumulator{Groups: make(map[uint]*GroupAccumulator)}
}

// GetOrCreate returns the accumulator for a disk group, creating it if needed.
func (a *RunAccumulator) GetOrCreate(groupID uint, mountPath, mode string) *GroupAccumulator {
	if ga, ok := a.Groups[groupID]; ok {
		return ga
	}
	ga := &GroupAccumulator{MountPath: mountPath, Mode: mode}
	a.Groups[groupID] = ga
	return ga
}

// Totals returns aggregate counts across all groups for engine stats.
func (a *RunAccumulator) Totals() (evaluated, candidates, protected, collections int64, freedBytes int64) {
	for _, ga := range a.Groups {
		evaluated += ga.Evaluated
		candidates += ga.Candidates
		protected += ga.Protected
		collections += ga.Collections
		freedBytes += ga.FreedBytes
	}
	return
}

// GroupAccumulator collects per-group metrics for a single disk group evaluation.
type GroupAccumulator struct {
	MountPath     string
	Mode          string
	Evaluated     int64
	Candidates    int64
	Protected     int64
	FreedBytes    int64
	Collections   int64
	DiskUsagePct  float64
	DiskThreshold float64
	DiskTargetPct float64
	// Sunset-mode counters (zero for other modes)
	SunsetQueued int
}

// Poller orchestrates periodic media library polling and capacity evaluation.
// All state is on the struct — no package-level globals.
type Poller struct {
	reg      *services.Registry
	done     chan struct{}
	stopped  chan struct{}
	started  atomic.Bool
	stopOnce sync.Once
}

// New creates a new Poller bound to the given service registry.
func New(reg *services.Registry) *Poller {
	return &Poller{
		reg:     reg,
		done:    make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

// Start begins the continuous polling loop. Call Stop() to terminate.
//
// The poller subscribes to the EventBus to receive:
//   - ManualRunTriggeredEvent: immediate engine run (replaces the old RunNowCh)
//   - SettingsChangedEvent: reset the poll timer to pick up interval changes
func (p *Poller) Start() {
	if !p.started.CompareAndSwap(false, true) {
		return
	}

	busCh := p.reg.Bus.Subscribe()

	go func() {
		defer close(p.stopped)
		defer p.reg.Bus.Unsubscribe(busCh)

		// Run immediately on startup so users see results without waiting
		// for the first poll interval to elapse.
		p.safePoll()

		timer := time.NewTimer(p.getPollInterval())
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				p.safePoll()
				timer.Reset(p.getPollInterval())
			case evt := <-busCh:
				switch evt.(type) {
				case events.ManualRunTriggeredEvent:
					slog.Info("Manual run triggered via API", "component", "poller")
					p.safePoll()
					// Don't reset the timer — let the next scheduled tick proceed normally
				case events.SettingsChangedEvent:
					slog.Info("Settings changed, resetting poll timer", "component", "poller")
					if !timer.Stop() {
						select {
						case <-timer.C:
						default:
						}
					}
					timer.Reset(p.getPollInterval())
				}
			case <-p.done:
				return
			}
		}
	}()
}

// Stop signals the poller goroutine to exit and waits for it to finish.
// Safe to call if Start was never invoked.
func (p *Poller) Stop() {
	p.stopOnce.Do(func() {
		close(p.done)
	})
	if p.started.Load() {
		<-p.stopped
	}
}

// getPollInterval reads PollIntervalSeconds from the database preference set.
// Falls back to 300s (5 min) if not set, and enforces a 30s minimum.
func (p *Poller) getPollInterval() time.Duration {
	prefs, err := p.reg.Settings.GetPreferences()
	if err != nil {
		return 5 * time.Minute
	}
	secs := prefs.PollIntervalSeconds
	if secs < 60 {
		secs = 300
	}
	return time.Duration(secs) * time.Second
}

// safePoll wraps poll() with panic recovery so a single failing cycle
// doesn't crash the entire poller goroutine.
func (p *Poller) safePoll() {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic recovered in poll cycle", "component", "poller", "panic", r)
			p.reg.Engine.SetRunning(false) // ensure the lock is released

			// Publish EngineErrorEvent so subscribers (notifications, SSE) can
			// report the failure to the user.
			p.reg.Bus.Publish(events.EngineErrorEvent{
				Error: fmt.Sprintf("panic: %v", r),
			})

			// Clear potentially stale preview cache so the next successful
			// cycle rebuilds it from scratch.
			p.reg.Preview.InvalidatePreviewCache("panic recovery")
		}
	}()
	p.poll()
}

// pollContext bundles the per-cycle state loaded during prepareContext()
// and shared across the poll sub-functions.
type pollContext struct {
	bus            *events.EventBus
	pollStart      time.Time
	acc            *RunAccumulator
	configs        []db.IntegrationConfig
	prefs          db.PreferenceSet
	weights        map[string]int
	rules          []db.CustomRule
	runStatsID     uint
	fetched        fetchResult
	evalCtx        *engine.EvaluationContext
	diskGroupModes map[uint]string // per-group modes collected during processMediaMounts
}

func (p *Poller) poll() {
	if p.reg.Engine.IsRunning() {
		slog.Info("Skipping poll — previous run still in progress", "component", "poller")
		return
	}
	p.reg.Engine.SetRunning(true)
	defer p.reg.Engine.SetRunning(false)

	pctx, ok := p.prepareContext()
	if !ok {
		return
	}

	totalDeletionsQueued, mediaMounts := p.processMediaMounts(pctx)
	p.finalizeCycle(pctx, totalDeletionsQueued, mediaMounts)
}

// prepareContext loads all configuration, fetches integration data, runs
// enrichment, and builds the evaluation context. Returns false if the cycle
// should be aborted (e.g., missing config).
func (p *Poller) prepareContext() (*pollContext, bool) {
	bus := p.reg.Bus
	pollStart := time.Now()

	// Clean expired snoozes at the start of each cycle
	if count, err := p.reg.Approval.CleanExpiredSnoozes(); err != nil {
		slog.Error("Failed to clean expired snoozes", "component", "poller", "error", err)
	} else if count > 0 {
		slog.Info("Cleaned expired snoozes at cycle start", "component", "poller", "count", count)
	}

	// Increment lifetime engine runs counter via service
	if err := p.reg.Metrics.IncrementEngineRuns(); err != nil {
		slog.Error("Failed to increment engine runs", "component", "poller", "error", err)
	}

	acc := NewRunAccumulator()

	configs, err := p.reg.Integration.ListEnabled()
	if err != nil {
		slog.Error("Failed to load integrations", "component", "poller", "operation", "load_integrations", "error", err)
		bus.Publish(events.EngineErrorEvent{Error: fmt.Sprintf("failed to load integrations: %v", err)})
		return nil, false
	}

	prefs, err := p.reg.Settings.GetPreferences()
	if err != nil {
		slog.Error("Failed to load preferences", "component", "poller", "operation", "load_preferences", "error", err)
		return nil, false
	}

	weights, err := p.reg.Settings.GetWeightMap()
	if err != nil {
		slog.Error("Failed to load scoring factor weights", "component", "poller", "operation", "load_weights", "error", err)
		return nil, false
	}

	// Create engine run stats row via service
	runStats, err := p.reg.Engine.CreateRunStats()
	if err != nil {
		slog.Error("Failed to create engine run stats", "component", "poller", "operation", "create_stats", "error", err)
	}
	var runStatsID uint
	if runStats != nil {
		runStatsID = runStats.ID
	}

	// Publish engine start event (disk group modes are populated during processMediaMounts)
	bus.Publish(events.EngineStartEvent{})

	slog.Debug("Poll cycle starting", "component", "poller",
		"enabledIntegrations", len(configs),
		"pollInterval", prefs.PollIntervalSeconds)

	if len(configs) == 0 {
		slog.Debug("No enabled integrations, marking all disk groups stale", "component", "poller")
		if marked, markErr := p.reg.DiskGroup.MarkAllStale(); markErr != nil {
			slog.Error("Failed to mark disk groups stale", "component", "poller", "error", markErr)
		} else if marked > 0 {
			slog.Info("Marked all disk groups stale (no enabled integrations)", "component", "poller", "count", marked)
		}
		if reaped, reapErr := p.reg.DiskGroup.ReapStale(prefs.DiskGroupGracePeriodDays); reapErr != nil {
			slog.Error("Failed to reap stale disk groups", "component", "poller", "error", reapErr)
		} else if reaped > 0 {
			slog.Info("Reaped expired stale disk groups", "component", "poller", "count", reaped)
		}
		return nil, false
	}

	rules, err := p.reg.Rules.List()
	if err != nil {
		slog.Error("Failed to load custom rules", "component", "poller", "operation", "load_rules", "error", err)
		return nil, false
	}

	// Fetch media items, disk space, and build registry+pipeline from healthy integrations.
	// The health service determines which integrations are reachable; the poller
	// reports data-fetch failures/successes back to keep health state current.
	fetched := fetchAllIntegrations(p.reg.Integration, p.reg.Health)

	// Enrich items using the pluggable enrichment pipeline
	var enrichStats integrations.EnrichmentStats
	if fetched.pipeline != nil {
		enrichStats = fetched.pipeline.Run(fetched.allItems)

		bus.Publish(events.EnrichmentCompleteEvent{
			EnrichersRun:   enrichStats.EnrichersRun,
			ItemsProcessed: enrichStats.ItemsProcessed,
			TotalMatches:   enrichStats.TotalMatches,
			ZeroMatchers:   enrichStats.ZeroMatchers,
			Timestamp:      time.Now().UTC(),
		})
	}

	// Populate persistent media server ID mapping table
	if fetched.registry != nil && p.reg.Mapping != nil {
		p.populateMediaServerMappings(fetched.registry, fetched.allItems)
	}

	// Build EvaluationContext AFTER fetch + enrichment
	configTypes := make([]string, len(configs))
	for i, cfg := range configs {
		configTypes[i] = cfg.Type
	}
	evalCtx := engine.NewEvaluationContext(configTypes, p.reg.Health.UnhealthyTypes())
	if len(enrichStats.FailedCapabilities) > 0 {
		failedCaps := make(map[string]bool, len(enrichStats.FailedCapabilities))
		for _, cap := range enrichStats.FailedCapabilities {
			failedCaps[cap] = true
		}
		evalCtx.FailedEnrichmentCapabilities = failedCaps
	}

	return &pollContext{
		bus:        bus,
		pollStart:  pollStart,
		acc:        acc,
		configs:    configs,
		prefs:      prefs,
		weights:    weights,
		rules:      rules,
		runStatsID: runStatsID,
		fetched:    fetched,
		evalCtx:    evalCtx,
	}, true
}

// processMediaMounts iterates over discovered media mounts, upserts disk
// groups, records history snapshots, and runs per-group evaluation.
// Returns the total deletions queued and the set of active media mounts.
func (p *Poller) processMediaMounts(pctx *pollContext) (int, map[string]bool) {
	mediaMounts := findMediaMounts(pctx.fetched.diskMap, pctx.fetched.rootFolders)

	slog.Info("Processing disk groups", "component", "poller",
		"mediaMounts", len(mediaMounts))

	var totalDeletionsQueued int
	anyThresholdBreached := false
	for mountPath := range mediaMounts {
		disk := pctx.fetched.diskMap[mountPath]
		usedBytes := disk.TotalBytes - disk.FreeBytes

		effectiveTotal := disk.TotalBytes
		usedPct := float64(0)
		if effectiveTotal > 0 {
			usedPct = float64(usedBytes) / float64(effectiveTotal) * 100
		}

		slog.Info("Evaluating disk group", "component", "poller",
			"mount", mountPath,
			"totalBytes", disk.TotalBytes,
			"usedBytes", usedBytes,
			"usedPct", fmt.Sprintf("%.1f%%", usedPct),
			"freeBytes", disk.FreeBytes)

		// Upsert DiskGroup via service
		group, upsertErr := p.reg.DiskGroup.Upsert(disk)
		if upsertErr != nil {
			slog.Error("Failed to upsert disk group", "component", "poller", "mount", mountPath, "error", upsertErr)
			continue
		}
		group.TotalBytes = disk.TotalBytes
		group.UsedBytes = usedBytes

		// Record per-group mode for events and stats
		if pctx.diskGroupModes == nil {
			pctx.diskGroupModes = make(map[uint]string)
		}
		pctx.diskGroupModes[group.ID] = group.Mode

		// Track threshold breaches across all disk groups
		groupEffective := group.EffectiveTotalBytes()
		if groupEffective > 0 {
			groupPct := float64(group.UsedBytes) / float64(groupEffective) * 100
			if groupPct >= group.ThresholdPct {
				anyThresholdBreached = true
			}
		}

		// Sync integration links for this disk group
		if intIDs, ok := pctx.fetched.mountIntegrations[mountPath]; ok {
			if linkErr := p.reg.DiskGroup.SyncIntegrationLinks(group.ID, intIDs); linkErr != nil {
				slog.Error("Failed to sync integration links", "component", "poller",
					"mount", mountPath, "error", linkErr)
			}
		}

		// Record LibraryHistory snapshot via service
		if err := p.reg.Metrics.RecordLibraryHistory(group.ID, disk.TotalBytes, usedBytes); err != nil {
			slog.Error("Failed to save capacity record", "component", "poller", "operation", "save_capacity",
				"mount", mountPath, "error", err)
		}

		// Evaluate and trigger cleanup if threshold breached
		totalDeletionsQueued += p.evaluateDiskGroup(pctx.acc, *group, pctx.fetched.allItems, pctx.fetched.registry, pctx.runStatsID, pctx.prefs, pctx.weights, pctx.rules, pctx.evalCtx)
	}

	// Clear approval queue when ALL disk groups are below threshold.
	if !anyThresholdBreached && len(mediaMounts) > 0 {
		if cleared, err := p.reg.Approval.ClearQueue(); err != nil {
			slog.Error("Failed to clear approval queue", "component", "poller", "error", err)
		} else if cleared > 0 {
			slog.Info("Approval queue cleared (all disks below threshold)",
				"component", "poller", "clearedCount", cleared)
		}
	}

	// Clean up orphaned disk groups that are no longer media mounts.
	if len(mediaMounts) == 0 && !pctx.fetched.anyDiskSuccess {
		slog.Warn("Skipping disk group reconciliation — no disk reporters returned data",
			"component", "poller")
	} else if stale, cleanErr := p.reg.DiskGroup.ReconcileActiveMounts(mediaMounts); cleanErr != nil {
		slog.Error("Failed to reconcile disk groups", "component", "poller", "error", cleanErr)
	} else if stale > 0 {
		slog.Info("Marked orphaned disk groups stale", "component", "poller", "count", stale)
	}

	// Reap stale disk groups whose grace period has expired
	if reaped, reapErr := p.reg.DiskGroup.ReapStale(pctx.prefs.DiskGroupGracePeriodDays); reapErr != nil {
		slog.Error("Failed to reap stale disk groups", "component", "poller", "error", reapErr)
	} else if reaped > 0 {
		slog.Info("Reaped expired stale disk groups", "component", "poller", "count", reaped)
	}

	return totalDeletionsQueued, mediaMounts
}

// finalizeCycle persists run stats, flushes notifications, populates the
// preview cache, and publishes the engine complete event.
func (p *Poller) finalizeCycle(pctx *pollContext, totalDeletionsQueued int, mediaMounts map[string]bool) {
	_ = mediaMounts // used for context in future extensions; silences unused warning

	p.reg.Deletion.SignalBatchSize(totalDeletionsQueued)

	// Read per-run stats from the accumulator
	evaluated, candidates, protected, _, freedBytes := pctx.acc.Totals()

	// Store per-disk-group modes on the engine run stats row
	if pctx.runStatsID > 0 {
		dgModes := make(map[uint]string, len(pctx.acc.Groups))
		for groupID, ga := range pctx.acc.Groups {
			dgModes[groupID] = ga.Mode
		}
		if modesErr := p.reg.Engine.SetDiskGroupModes(pctx.runStatsID, dgModes); modesErr != nil {
			slog.Error("Failed to set disk group modes on run stats",
				"component", "poller", "error", modesErr)
		}
	}

	// Build per-group digest data from the accumulator.
	groups := make([]notifications.GroupDigest, 0, len(pctx.acc.Groups))
	for _, ga := range pctx.acc.Groups {
		groups = append(groups, notifications.GroupDigest{
			MountPath:          ga.MountPath,
			Mode:               ga.Mode,
			Evaluated:          int(ga.Evaluated),
			Candidates:         int(ga.Candidates),
			FreedBytes:         ga.FreedBytes,
			DiskUsagePct:       ga.DiskUsagePct,
			DiskThreshold:      ga.DiskThreshold,
			DiskTargetPct:      ga.DiskTargetPct,
			CollectionsDeleted: int(ga.Collections),
			SunsetQueued:       ga.SunsetQueued,
		})
	}

	// Flush cycle digest notification
	p.reg.NotificationDispatch.FlushCycleDigest(notifications.CycleDigest{
		Groups:     groups,
		DurationMs: time.Since(pctx.pollStart).Milliseconds(),
	})

	// Persist run stats — avoid double-counting freed bytes when any group is in auto mode.
	// In auto mode, IncrementDeletedStats() accumulates actual freed bytes per-item,
	// so we zero the engine-level estimate to prevent double-counting.
	writeFreedBytes := freedBytes
	for _, mode := range pctx.diskGroupModes {
		if mode == db.ModeAuto {
			writeFreedBytes = 0
			break
		}
	}

	if pctx.runStatsID == 0 {
		slog.Warn("Skipping engine run stats update — stats row was not created", "component", "poller")
	} else if err := p.reg.Engine.UpdateRunStats(pctx.runStatsID, int(evaluated), int(candidates), totalDeletionsQueued, writeFreedBytes, time.Since(pctx.pollStart).Milliseconds()); err != nil {
		slog.Error("Failed to update engine run stats", "component", "poller", "error", err)
	}

	// Sync per-run stats to EngineService for API consumers
	p.reg.Engine.SetLastRunStats(int(evaluated), int(candidates), int(protected))

	// Populate preview cache with already-fetched and enriched items
	p.reg.Preview.SetPreviewCache(pctx.fetched.allItems, pctx.prefs, pctx.weights, pctx.rules, pctx.evalCtx)

	// Persist per-disk-group modes on the run stats row
	if pctx.runStatsID != 0 {
		if err := p.reg.Engine.SetDiskGroupModes(pctx.runStatsID, pctx.diskGroupModes); err != nil {
			slog.Error("Failed to persist disk group modes", "component", "poller", "error", err)
		}
	}

	// Publish engine complete event
	pctx.bus.Publish(events.EngineCompleteEvent{
		Evaluated:        int(evaluated),
		Candidates:       int(candidates),
		DurationMs:       time.Since(pctx.pollStart).Milliseconds(),
		DiskGroupModes:   pctx.diskGroupModes,
		FreedBytes:       freedBytes,
		CompletedAtEpoch: time.Now().UTC().Unix(),
	})

	slog.Debug("Poll cycle complete", "component", "poller",
		"duration", time.Since(pctx.pollStart).String(),
		"totalItems", len(pctx.fetched.allItems),
		"evaluated", evaluated,
		"candidates", candidates,
		"protected", protected)
}

// populateMediaServerMappings extracts TMDb→NativeID mappings from all media
// server integrations and persists them via MappingService.BulkUpsert(). Title
// and media type are cross-referenced from the *arr-sourced allItems to enrich
// the mapping records for targeted search fallback (Phase 2).
//
// This runs once per poll cycle, replacing the per-request full-library-scan
// pattern that previously existed in 8 separate call sites.
func (p *Poller) populateMediaServerMappings(registry *integrations.IntegrationRegistry, allItems []integrations.MediaItem) {
	// Build title/type index from *arr items for enriching mapping records
	type itemInfo struct {
		Title     string
		MediaType string
	}
	infoByTMDbID := make(map[int]itemInfo, len(allItems))
	for _, item := range allItems {
		if item.TMDbID > 0 {
			infoByTMDbID[item.TMDbID] = itemInfo{
				Title:     item.Title,
				MediaType: string(item.Type),
			}
		}
	}

	// Get TMDb→NativeID maps from all media server integrations.
	// This calls the existing per-client map builders (GetTMDbToRatingKeyMap,
	// GetTMDbToItemIDMap) which will be removed in Phase 4 when the mapping
	// table becomes the sole source of truth.
	rawMaps := registry.BuildTMDbToNativeIDMaps()

	cycleStart := time.Now().UTC()

	for integrationID, idMap := range rawMaps {
		mappings := make([]db.MediaServerMapping, 0, len(idMap))
		for tmdbID, nativeID := range idMap {
			info := infoByTMDbID[tmdbID]
			mediaType := info.MediaType
			if mediaType == "" {
				mediaType = string(integrations.MediaTypeMovie)
			}
			mappings = append(mappings, db.MediaServerMapping{
				TmdbID:        tmdbID,
				IntegrationID: integrationID,
				NativeID:      nativeID,
				MediaType:     mediaType,
				Title:         info.Title,
			})
		}

		if err := p.reg.Mapping.BulkUpsert(mappings); err != nil {
			slog.Error("Failed to populate media server mappings",
				"component", "poller", "integrationID", integrationID, "error", err)
			continue
		}

		// Layer 2: Log stale mappings not seen in this poll cycle.
		// Mappings whose updated_at was not touched by BulkUpsert represent
		// items no longer present on this media server.
		if stale, err := p.reg.Mapping.TouchedBefore(integrationID, cycleStart); err == nil && stale > 0 {
			slog.Debug("Stale media server mappings detected (not seen in this poll cycle)",
				"component", "poller", "integrationID", integrationID, "staleCount", stale)
		}
	}
}

// normalizePath converts backslash path separators to forward slashes for
// consistent cross-platform path comparison. This is necessary because *arr
// services running on Windows return backslash paths (e.g. H:\Movies), but
// Capacitarr runs in Docker (Linux). We use strings.ReplaceAll instead of
// filepath.ToSlash because the latter only converts the OS-native separator,
// and on Linux backslash is not treated as a path separator.
func normalizePath(p string) string {
	return strings.ReplaceAll(p, `\`, "/")
}

// findMediaMounts returns only the mount paths that are the most specific match
// for at least one root folder. For example, if mounts are ["/", "/media"] and
// root folder is "/media/movies", only "/media" is returned (not "/").
//
// Paths are normalized to forward slashes before comparison to handle Windows
// *arr instances that return backslash paths (e.g. H:\Movies).
func findMediaMounts(diskMap map[string]integrations.DiskSpace, rootFolders map[string]bool) map[string]bool {
	mediaMounts := make(map[string]bool)

	// Collect available mount paths for diagnostic logging on failure
	availableMounts := make([]string, 0, len(diskMap))
	for mountPath := range diskMap {
		availableMounts = append(availableMounts, mountPath)
	}

	for rf := range rootFolders {
		cleanRF := strings.TrimRight(normalizePath(rf), "/")
		bestMount := ""
		bestLen := 0

		for mountPath := range diskMap {
			cleanMount := strings.TrimRight(normalizePath(mountPath), "/")
			// Special case: root "/" matches everything
			if cleanMount == "" {
				if bestLen == 0 {
					bestMount = mountPath
				}
				continue
			}
			// Check if root folder lives under this mount
			if strings.HasPrefix(cleanRF, cleanMount+"/") || cleanRF == cleanMount {
				if len(cleanMount) > bestLen {
					bestLen = len(cleanMount)
					bestMount = mountPath
				}
			}
		}

		if bestMount != "" {
			mediaMounts[bestMount] = true
			slog.Debug("Matched root folder to mount", "component", "poller",
				"rootFolder", rf, "mount", bestMount)
		} else {
			slog.Warn("No disk mount matched root folder", "component", "poller",
				"rootFolder", rf, "availableMounts", availableMounts)
		}
	}

	// If we have both "/" and other more specific mounts, drop "/"
	// This handles Docker/container scenarios where different services
	// see different mount namespaces for the same underlying storage
	if len(mediaMounts) > 1 {
		for m := range mediaMounts {
			if strings.TrimRight(normalizePath(m), "/") == "" {
				slog.Debug("Dropping root mount '/' since more specific mounts exist", "component", "poller")
				delete(mediaMounts, m)
			}
		}
	}

	// Log summary for diagnostic purposes
	if len(mediaMounts) > 0 {
		slog.Debug("Mount matching complete", "component", "poller",
			"rootFolders", len(rootFolders), "diskEntries", len(diskMap),
			"matchedMounts", len(mediaMounts))
	} else if len(rootFolders) > 0 {
		slog.Warn("Mount matching complete with no matches", "component", "poller",
			"rootFolders", len(rootFolders), "diskEntries", len(diskMap),
			"matchedMounts", 0)
	}

	return mediaMounts
}
