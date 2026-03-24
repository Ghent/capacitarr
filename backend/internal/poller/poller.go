package poller

import (
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
	"capacitarr/internal/services"
)

// Poller orchestrates periodic media library polling and capacity evaluation.
// All state is on the struct — no package-level globals.
type Poller struct {
	reg  *services.Registry
	done chan struct{}

	// Per-run metrics (reset each engine cycle, synced to EngineService at the end)
	lastRunEvaluated  int64
	lastRunCandidates int64
	lastRunProtected  int64
	lastRunFreedBytes int64
}

// New creates a new Poller bound to the given service registry.
func New(reg *services.Registry) *Poller {
	return &Poller{
		reg:  reg,
		done: make(chan struct{}),
	}
}

// Start begins the continuous polling loop. Call Stop() to terminate.
//
// The poller subscribes to the EventBus to receive:
//   - ManualRunTriggeredEvent: immediate engine run (replaces the old RunNowCh)
//   - SettingsChangedEvent: reset the poll timer to pick up interval changes
func (p *Poller) Start() {
	busCh := p.reg.Bus.Subscribe()

	go func() {
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

// Stop signals the poller goroutine to exit.
func (p *Poller) Stop() {
	close(p.done)
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
		}
	}()
	p.poll()
}

func (p *Poller) poll() {
	if p.reg.Engine.IsRunning() {
		slog.Info("Skipping poll — previous run still in progress", "component", "poller")
		return
	}
	p.reg.Engine.SetRunning(true)
	defer p.reg.Engine.SetRunning(false)

	bus := p.reg.Bus
	pollStart := time.Now()

	// Clean expired snoozes at the start of each cycle — resets rejected items
	// with expired snoozed_until back to pending so they're re-evaluated.
	if count, err := p.reg.Approval.CleanExpiredSnoozes(); err != nil {
		slog.Error("Failed to clean expired snoozes", "component", "poller", "error", err)
	} else if count > 0 {
		slog.Info("Cleaned expired snoozes at cycle start", "component", "poller", "count", count)
	}

	// Increment lifetime engine runs counter via service
	if err := p.reg.Metrics.IncrementEngineRuns(); err != nil {
		slog.Error("Failed to increment engine runs", "component", "poller", "error", err)
	}

	// Reset per-run counters at the start of each poll cycle
	atomic.StoreInt64(&p.lastRunEvaluated, 0)
	atomic.StoreInt64(&p.lastRunCandidates, 0)
	atomic.StoreInt64(&p.lastRunProtected, 0)
	atomic.StoreInt64(&p.lastRunFreedBytes, 0)

	configs, err := p.reg.Integration.ListEnabled()
	if err != nil {
		slog.Error("Failed to load integrations", "component", "poller", "operation", "load_integrations", "error", err)
		bus.Publish(events.EngineErrorEvent{Error: fmt.Sprintf("failed to load integrations: %v", err)})
		return
	}

	prefs, err := p.reg.Settings.GetPreferences()
	if err != nil {
		slog.Error("Failed to load preferences", "component", "poller", "operation", "load_preferences", "error", err)
		return
	}

	weights, err := p.reg.Settings.GetWeightMap()
	if err != nil {
		slog.Error("Failed to load scoring factor weights", "component", "poller", "operation", "load_weights", "error", err)
		return
	}

	// Create engine run stats row via service
	runStats, err := p.reg.Engine.CreateRunStats(prefs.ExecutionMode)
	if err != nil {
		slog.Error("Failed to create engine run stats", "component", "poller", "operation", "create_stats", "error", err)
	}
	var runStatsID uint
	if runStats != nil {
		runStatsID = runStats.ID
	}

	// Publish engine start event
	bus.Publish(events.EngineStartEvent{ExecutionMode: prefs.ExecutionMode})

	slog.Debug("Poll cycle starting", "component", "poller",
		"enabledIntegrations", len(configs),
		"pollInterval", prefs.PollIntervalSeconds,
		"executionMode", prefs.ExecutionMode)

	if len(configs) == 0 {
		slog.Debug("No enabled integrations, cleaning all disk groups", "component", "poller")
		if removed, rmErr := p.reg.DiskGroup.RemoveAll(); rmErr != nil {
			slog.Error("Failed to remove disk groups", "component", "poller", "error", rmErr)
		} else if removed > 0 {
			slog.Info("Removed all disk groups (no enabled integrations)", "component", "poller", "count", removed)
		}
		return
	}

	rules, err := p.reg.Rules.List()
	if err != nil {
		slog.Error("Failed to load custom rules", "component", "poller", "operation", "load_rules", "error", err)
		return
	}

	// Build EvaluationContext from enabled integration types so the scoring
	// engine can exclude factors whose prerequisites are not met (e.g.
	// RequestPopularityFactor without Seerr, SeriesStatusFactor for movies).
	configTypes := make([]string, len(configs))
	for i, cfg := range configs {
		configTypes[i] = cfg.Type
	}
	evalCtx := engine.NewEvaluationContext(configTypes)

	// Fetch media items, disk space, and build registry+pipeline from all integrations
	fetched := fetchAllIntegrations(p.reg.Integration)

	// Enrich items using the pluggable enrichment pipeline
	if fetched.pipeline != nil {
		enrichStats := fetched.pipeline.Run(fetched.allItems)

		// Publish enrichment summary event
		bus.Publish(events.EnrichmentCompleteEvent{
			EnrichersRun:   enrichStats.EnrichersRun,
			ItemsProcessed: enrichStats.ItemsProcessed,
			TotalMatches:   enrichStats.TotalMatches,
			ZeroMatchers:   enrichStats.ZeroMatchers,
			Timestamp:      time.Now().UTC(),
		})
	}

	// Find the most specific mount for each root folder
	mediaMounts := findMediaMounts(fetched.diskMap, fetched.rootFolders)

	// Update DiskGroups and record history only for media mounts
	slog.Info("Processing disk groups", "component", "poller",
		"mediaMounts", len(mediaMounts), "executionMode", prefs.ExecutionMode)

	var totalDeletionsQueued int
	anyThresholdBreached := false
	for mountPath := range mediaMounts {
		disk := fetched.diskMap[mountPath]
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
		// Ensure local struct has latest values for threshold check
		group.TotalBytes = disk.TotalBytes
		group.UsedBytes = usedBytes

		// Track threshold breaches across all disk groups
		groupEffective := group.EffectiveTotalBytes()
		if groupEffective > 0 {
			groupPct := float64(group.UsedBytes) / float64(groupEffective) * 100
			if groupPct >= group.ThresholdPct {
				anyThresholdBreached = true
			}
		}

		// Sync integration links for this disk group
		if intIDs, ok := fetched.mountIntegrations[mountPath]; ok {
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
		totalDeletionsQueued += p.evaluateAndCleanDisk(*group, fetched.allItems, fetched.registry, runStatsID, prefs, weights, rules, evalCtx)
	}

	// Clear the approval queue only when ALL disk groups are below threshold.
	// This uses global ClearQueue() to wipe all pending/rejected items since
	// no disk group needs action. Items are scoped to disk groups via DiskGroupID,
	// so per-group clearing happens automatically during next evaluation when
	// individual groups drop below threshold.
	if !anyThresholdBreached && len(mediaMounts) > 0 {
		if cleared, err := p.reg.Approval.ClearQueue(); err != nil {
			slog.Error("Failed to clear approval queue", "component", "poller", "error", err)
		} else if cleared > 0 {
			slog.Info("Approval queue cleared (all disks below threshold)",
				"component", "poller", "clearedCount", cleared)
		}
	}

	// Clean up orphaned disk groups that are no longer media mounts.
	// This also handles the case where all integrations failed to return
	// disk space — mediaMounts is empty, so all disk groups are removed.
	// When integrations recover, the next successful poll recreates them.
	if deleted, cleanErr := p.reg.DiskGroup.ReconcileActiveMounts(mediaMounts); cleanErr != nil {
		slog.Error("Failed to clean orphaned disk groups", "component", "poller", "error", cleanErr)
	} else if deleted > 0 {
		slog.Info("Removed orphaned disk groups", "component", "poller", "count", deleted)
	}

	// Signal the deletion service with the batch size so it can publish
	// DeletionBatchCompleteEvent when all items are processed. If zero items
	// were queued, SignalBatchSize(0) publishes the event immediately.
	p.reg.Deletion.SignalBatchSize(totalDeletionsQueued)

	// Update engine run stats via service
	evaluated := atomic.LoadInt64(&p.lastRunEvaluated)
	candidates := atomic.LoadInt64(&p.lastRunCandidates)
	protected := atomic.LoadInt64(&p.lastRunProtected)
	freedBytes := atomic.LoadInt64(&p.lastRunFreedBytes)

	// In auto mode, IncrementDeletedStats() accumulates actual freed bytes
	// per-item as deletions complete. Writing freedBytes here would double-count.
	// For dry-run and approval modes, the poller's accumulated freedBytes is the
	// only source of truth — persist it now.
	writeFreedBytes := freedBytes
	if prefs.ExecutionMode == db.ModeAuto {
		writeFreedBytes = 0
	}

	if err := p.reg.Engine.UpdateRunStats(runStatsID, int(evaluated), int(candidates), totalDeletionsQueued, writeFreedBytes, time.Since(pollStart).Milliseconds()); err != nil {
		slog.Error("Failed to update engine run stats", "component", "poller", "error", err)
	}

	// Sync per-run stats to EngineService for API consumers
	p.reg.Engine.SetLastRunStats(int(evaluated), int(candidates), int(protected))

	// Populate preview cache with already-fetched and enriched items
	p.reg.Preview.SetPreviewCache(fetched.allItems, prefs, weights, rules, evalCtx)

	// Publish engine complete event
	bus.Publish(events.EngineCompleteEvent{
		Evaluated:        int(evaluated),
		Candidates:       int(candidates),
		DurationMs:       time.Since(pollStart).Milliseconds(),
		ExecutionMode:    prefs.ExecutionMode,
		FreedBytes:       freedBytes,
		CompletedAtEpoch: time.Now().UTC().Unix(),
	})

	slog.Debug("Poll cycle complete", "component", "poller",
		"duration", time.Since(pollStart).String(),
		"totalItems", len(fetched.allItems),
		"evaluated", evaluated,
		"candidates", candidates,
		"protected", protected)
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
