package services

import (
	"fmt"
	"sync/atomic"
	"time"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
)

// IntegrationLister provides read access to enabled integrations and
// enrichment client construction.
// Defined here to avoid import cycles between EngineService and IntegrationService.
type IntegrationLister interface {
	ListEnabled() ([]db.IntegrationConfig, error)
	BuildEnrichmentClients() (*EnrichmentBuildResult, error)
}

// RulesProvider provides read access to custom rules.
// Defined here to avoid import cycles between EngineService and RulesService.
type RulesProvider interface {
	List() ([]db.CustomRule, error)
}

// DiskGroupLister provides read access to disk groups.
// Defined here to avoid import cycles between EngineService and DiskGroupService.
type DiskGroupLister interface {
	List() ([]db.DiskGroup, error)
}

// EngineService manages engine run triggers and stats.
type EngineService struct {
	db       *gorm.DB
	bus      *events.EventBus
	RunNowCh chan struct{} // Signals the poller to run immediately

	// Cross-service dependencies (set via SetDependencies)
	integrations IntegrationLister
	preferences  SettingsReader
	rules        RulesProvider
	diskGroups   DiskGroupLister

	// Observable state
	lastEvaluated atomic.Int64
	lastFlagged   atomic.Int64
	lastProtected atomic.Int64
	pollRunning   atomic.Bool
}

// EngineStatusStarted is returned by TriggerRun when a new run is initiated.
const EngineStatusStarted = "started"

// EngineStatusAlreadyRunning is returned by TriggerRun when a run is already in progress.
const EngineStatusAlreadyRunning = "already_running"

// NewEngineService creates a new EngineService.
func NewEngineService(database *gorm.DB, bus *events.EventBus) *EngineService {
	return &EngineService{
		db:       database,
		bus:      bus,
		RunNowCh: make(chan struct{}, 1),
	}
}

// SetDependencies wires cross-service dependencies that cannot be injected
// at construction time due to circular initialization in the registry.
func (s *EngineService) SetDependencies(integ IntegrationLister, settings SettingsReader, rules RulesProvider, diskGroups DiskGroupLister) {
	s.integrations = integ
	s.preferences = settings
	s.rules = rules
	s.diskGroups = diskGroups
}

// TriggerRun sends a signal to run the engine immediately.
// Returns EngineStatusStarted if the signal was sent, EngineStatusAlreadyRunning
// if a run is already in progress.
func (s *EngineService) TriggerRun() string {
	if s.pollRunning.Load() {
		return EngineStatusAlreadyRunning
	}

	select {
	case s.RunNowCh <- struct{}{}:
		s.bus.Publish(events.ManualRunTriggeredEvent{})
		return EngineStatusStarted
	default:
		return EngineStatusAlreadyRunning
	}
}

// SetRunning marks the engine as running or not running.
func (s *EngineService) SetRunning(running bool) {
	s.pollRunning.Store(running)
}

// IsRunning returns whether the engine is currently running.
func (s *EngineService) IsRunning() bool {
	return s.pollRunning.Load()
}

// SetLastRunStats updates the last run statistics.
func (s *EngineService) SetLastRunStats(evaluated, flagged, protected int) {
	s.lastEvaluated.Store(int64(evaluated))
	s.lastFlagged.Store(int64(flagged))
	s.lastProtected.Store(int64(protected))
}

// PreviewResult holds the full result of a score preview computation.
type PreviewResult struct {
	Items       []engine.EvaluatedItem `json:"items"`
	DiskContext *DiskContext           `json:"diskContext"`
}

// DiskContext provides disk usage information for the deletion line in the UI.
type DiskContext struct {
	TotalBytes   int64   `json:"totalBytes"`
	UsedBytes    int64   `json:"usedBytes"`
	TargetPct    float64 `json:"targetPct"`
	ThresholdPct float64 `json:"thresholdPct"`
	BytesToFree  int64   `json:"bytesToFree"`
}

// GetPreview fetches all media items from enabled integrations, enriches them
// with watch/request data, scores them against current rules and preferences,
// and returns the full evaluated result for the preview UI.
func (s *EngineService) GetPreview() (*PreviewResult, error) {
	buildResult, err := s.integrations.BuildEnrichmentClients()
	if err != nil {
		return nil, err
	}

	var allItems []integrations.MediaItem
	for _, cfg := range buildResult.ArrConfigs {
		client := integrations.NewClient(cfg.Type, cfg.URL, cfg.APIKey)
		if client == nil {
			continue
		}
		items, err := client.GetMediaItems()
		if err != nil {
			continue
		}
		for i := range items {
			items[i].IntegrationID = cfg.ID
		}
		allItems = append(allItems, items...)
	}

	// Apply enrichment (Plex, Tautulli, Jellyfin, Emby, Overseerr)
	integrations.EnrichItems(allItems, buildResult.Clients)

	prefs, err := s.preferences.GetPreferences()
	if err != nil {
		return nil, err
	}

	rules, err := s.rules.List()
	if err != nil {
		return nil, err
	}

	evaluated := engine.EvaluateMedia(allItems, prefs, rules)
	engine.SortEvaluated(evaluated, prefs.TiebreakerMethod)

	// Build disk context
	diskGroups, err := s.diskGroups.List()
	if err != nil {
		return nil, err
	}

	var diskCtx *DiskContext
	if len(diskGroups) > 0 {
		var bestGroup *db.DiskGroup
		var bestBytesToFree int64

		for i := range diskGroups {
			dg := &diskGroups[i]
			if dg.TotalBytes == 0 {
				continue
			}
			usedPct := float64(dg.UsedBytes) / float64(dg.TotalBytes) * 100
			var btf int64
			if usedPct >= dg.ThresholdPct {
				btf = dg.UsedBytes - int64(float64(dg.TotalBytes)*dg.TargetPct/100)
				if btf < 0 {
					btf = 0
				}
			}
			if bestGroup == nil || btf > bestBytesToFree {
				bestGroup = dg
				bestBytesToFree = btf
			}
		}

		if bestGroup != nil {
			diskCtx = &DiskContext{
				TotalBytes:   bestGroup.TotalBytes,
				UsedBytes:    bestGroup.UsedBytes,
				TargetPct:    bestGroup.TargetPct,
				ThresholdPct: bestGroup.ThresholdPct,
				BytesToFree:  bestBytesToFree,
			}
		}
	}

	return &PreviewResult{
		Items:       evaluated,
		DiskContext: diskCtx,
	}, nil
}

// EngineHistoryPoint holds a single data point for the engine history sparklines.
type EngineHistoryPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	Evaluated  int       `json:"evaluated"`
	Flagged    int       `json:"flagged"`
	Deleted    int       `json:"deleted"`
	FreedBytes int64     `json:"freedBytes"`
	DurationMs int64     `json:"durationMs"`
}

// CreateRunStats creates a new engine run stats entry and returns it.
func (s *EngineService) CreateRunStats(mode string) (*db.EngineRunStats, error) {
	stats := db.EngineRunStats{
		RunAt:         time.Now().UTC(),
		ExecutionMode: mode,
	}
	if err := s.db.Create(&stats).Error; err != nil {
		return nil, fmt.Errorf("failed to create engine run stats: %w", err)
	}
	return &stats, nil
}

// UpdateRunStats updates a run stats entry with the final evaluation results.
func (s *EngineService) UpdateRunStats(id uint, evaluated, flagged int, durationMs int64) error {
	result := s.db.Model(&db.EngineRunStats{}).Where("id = ?", id).Updates(map[string]any{
		"evaluated":   evaluated,
		"flagged":     flagged,
		"duration_ms": durationMs,
	})
	if result.Error != nil {
		return fmt.Errorf("failed to update engine run stats: %w", result.Error)
	}
	return nil
}

// GetHistory returns engine run history points within the given duration.
func (s *EngineService) GetHistory(since time.Duration) ([]EngineHistoryPoint, error) {
	cutoff := time.Now().UTC().Add(-since)

	var stats []db.EngineRunStats
	if err := s.db.Where("run_at >= ?", cutoff).
		Order("run_at ASC").
		Find(&stats).Error; err != nil {
		return nil, fmt.Errorf("failed to query engine history: %w", err)
	}

	points := make([]EngineHistoryPoint, len(stats))
	for i, st := range stats {
		points[i] = EngineHistoryPoint{
			Timestamp:  st.RunAt,
			Evaluated:  st.Evaluated,
			Flagged:    st.Flagged,
			Deleted:    st.Deleted,
			FreedBytes: st.FreedBytes,
			DurationMs: st.DurationMs,
		}
	}

	return points, nil
}

// PruneOldStats keeps only the most recent N engine run stats entries.
func (s *EngineService) PruneOldStats(keep int) (int64, error) {
	// Get the Nth newest run_at timestamp
	var cutoffRows []db.EngineRunStats
	s.db.Order("run_at desc").Offset(keep).Limit(1).Find(&cutoffRows)
	if len(cutoffRows) == 0 {
		return 0, nil // fewer than `keep` entries exist
	}

	result := s.db.Where("run_at <= ?", cutoffRows[0].RunAt).Delete(&db.EngineRunStats{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to prune engine run stats: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// LatestRunStatsID returns the ID of the most recent EngineRunStats row, or 0
// if no rows exist. Used by the approval flow to attribute deletions to the
// engine run that originally flagged the item.
func (s *EngineService) LatestRunStatsID() uint {
	var row db.EngineRunStats
	if err := s.db.Order("run_at desc").Select("id").First(&row).Error; err != nil {
		return 0
	}
	return row.ID
}

// IncrementDeletedStats atomically increments the deleted counter and freed bytes
// on an engine run stats row. Used by the DeletionService after a successful deletion.
func (s *EngineService) IncrementDeletedStats(runStatsID uint, sizeBytes int64) error {
	if runStatsID == 0 {
		return nil
	}
	result := s.db.Model(&db.EngineRunStats{}).Where("id = ?", runStatsID).
		UpdateColumns(map[string]any{
			"deleted":     gorm.Expr("deleted + ?", 1),
			"freed_bytes": gorm.Expr("freed_bytes + ?", sizeBytes),
		})
	if result.Error != nil {
		return fmt.Errorf("failed to increment deleted stats: %w", result.Error)
	}
	return nil
}

// GetStats returns the current engine statistics as a map.
// Keys match the frontend TypeScript WorkerStats interface.
func (s *EngineService) GetStats() map[string]any {
	stats := map[string]any{
		"isRunning":        s.pollRunning.Load(),
		"lastRunEvaluated": s.lastEvaluated.Load(),
		"lastRunFlagged":   s.lastFlagged.Load(),
		"protectedCount":   s.lastProtected.Load(),
	}

	// Get the latest run from the database
	var latest db.EngineRunStats
	if err := s.db.Order("run_at desc").First(&latest).Error; err == nil {
		stats["executionMode"] = latest.ExecutionMode
		stats["lastRunFreedBytes"] = latest.FreedBytes
		stats["lastRunEpoch"] = latest.RunAt.Unix()
	}

	return stats
}
