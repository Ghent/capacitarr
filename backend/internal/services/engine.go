package services

import (
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

// EngineService manages engine run triggers and stats.
type EngineService struct {
	db  *gorm.DB
	bus *events.EventBus

	// Observable state
	lastEvaluated  atomic.Int64
	lastCandidates atomic.Int64
	lastProtected  atomic.Int64
	pollRunning    atomic.Bool
}

// EngineStatusStarted is returned by TriggerRun when a new run is initiated.
const EngineStatusStarted = "started"

// EngineStatusAlreadyRunning is returned by TriggerRun when a run is already in progress.
const EngineStatusAlreadyRunning = "already_running"

// NewEngineService creates a new EngineService.
func NewEngineService(database *gorm.DB, bus *events.EventBus) *EngineService {
	return &EngineService{
		db:  database,
		bus: bus,
	}
}

// TriggerRun publishes a ManualRunTriggeredEvent on the EventBus to signal the
// poller to run immediately. Returns EngineStatusStarted if the event was
// published, EngineStatusAlreadyRunning if a run is already in progress.
func (s *EngineService) TriggerRun() string {
	if s.pollRunning.Load() {
		return EngineStatusAlreadyRunning
	}

	s.bus.Publish(events.ManualRunTriggeredEvent{})
	return EngineStatusStarted
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
func (s *EngineService) SetLastRunStats(evaluated, candidates, protected int) {
	s.lastEvaluated.Store(int64(evaluated))
	s.lastCandidates.Store(int64(candidates))
	s.lastProtected.Store(int64(protected))
}

// EngineHistoryPoint holds a single data point for the engine history sparklines.
type EngineHistoryPoint struct {
	Timestamp     time.Time `json:"timestamp"`
	Evaluated     int       `json:"evaluated"`
	Candidates    int       `json:"candidates"`
	Queued        int       `json:"queued"`
	Deleted       int       `json:"deleted"`
	FreedBytes    int64     `json:"freedBytes"`
	DurationMs    int64     `json:"durationMs"`
	ExecutionMode string    `json:"executionMode"`
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

// UpdateRunStats updates a run stats entry with the final evaluation results
// and sets completed_at to the current time. The freedBytes parameter records
// the total bytes that would be freed (dry-run/approval) or were queued for
// deletion (auto). In auto mode, callers should pass 0 here because
// IncrementDeletedStats() accumulates actual freed bytes per-item to avoid
// double-counting.
func (s *EngineService) UpdateRunStats(id uint, evaluated, candidates, queued int, freedBytes, durationMs int64) error {
	now := time.Now().UTC()
	result := s.db.Model(&db.EngineRunStats{}).Where("id = ?", id).Updates(map[string]any{
		"evaluated":    evaluated,
		"candidates":   candidates,
		"queued":       queued,
		"freed_bytes":  freedBytes,
		"duration_ms":  durationMs,
		"completed_at": now,
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
			Timestamp:     st.RunAt,
			Evaluated:     st.Evaluated,
			Candidates:    st.Candidates,
			Queued:        st.Queued,
			Deleted:       st.Deleted,
			FreedBytes:    st.FreedBytes,
			DurationMs:    st.DurationMs,
			ExecutionMode: st.ExecutionMode,
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

// RestoreLastRunStats initializes the in-memory atomic counters from the most
// recent EngineRunStats DB row. Called once on startup so the worker stats
// panel shows the last run's results instead of zeros.
//
// Note: The Queued field is intentionally not restored here. It is consumed
// exclusively via the history sparkline API (GetHistory), not the dashboard
// header stat counters (GetStats).
func (s *EngineService) RestoreLastRunStats() {
	var latest db.EngineRunStats
	if err := s.db.Order("run_at desc").First(&latest).Error; err != nil {
		return // No previous runs — leave at zero
	}

	s.lastEvaluated.Store(int64(latest.Evaluated))
	s.lastCandidates.Store(int64(latest.Candidates))

	slog.Info("Restored engine stats from last run",
		"component", "engine",
		"evaluated", latest.Evaluated,
		"candidates", latest.Candidates,
		"runAt", latest.RunAt.Format(time.RFC3339))
}

// GetStats returns the current engine statistics as a map.
// Keys match the frontend TypeScript WorkerStats interface.
func (s *EngineService) GetStats() map[string]any {
	stats := map[string]any{
		"isRunning":         s.pollRunning.Load(),
		"lastRunEvaluated":  s.lastEvaluated.Load(),
		"lastRunCandidates": s.lastCandidates.Load(),
		"protectedCount":    s.lastProtected.Load(),
	}

	// Get the latest completed run from the database.
	// Use completed_at for lastRunEpoch so the UI shows when the run finished,
	// not when it started. Fall back to run_at for rows that predate the
	// completed_at column (they will have NULL completed_at).
	var latest db.EngineRunStats
	if err := s.db.Order("run_at desc").First(&latest).Error; err == nil {
		stats["executionMode"] = latest.ExecutionMode
		stats["lastRunFreedBytes"] = latest.FreedBytes
		if latest.CompletedAt != nil {
			stats["lastRunEpoch"] = latest.CompletedAt.Unix()
		} else {
			stats["lastRunEpoch"] = latest.RunAt.Unix()
		}
	}

	return stats
}
