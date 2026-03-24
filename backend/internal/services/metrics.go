package services

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"capacitarr/internal/db"
)

// MetricsService consolidates inline DB queries for metrics, history, and
// dashboard statistics. It delegates worker-specific stats to EngineService
// and DeletionService.
type MetricsService struct {
	db       *gorm.DB
	engine   *EngineService
	deletion *DeletionService
	settings SettingsReader
}

// SetSettingsService wires the SettingsService dependency for preference reads.
// Called by Registry after construction to avoid circular initialization.
func (s *MetricsService) SetSettingsService(settings SettingsReader) {
	s.settings = settings
}

// NewMetricsService creates a new MetricsService.
func NewMetricsService(database *gorm.DB, engine *EngineService, deletion *DeletionService) *MetricsService {
	return &MetricsService{db: database, engine: engine, deletion: deletion}
}

// GetHistory returns library history entries filtered by resolution, disk group, and time range.
// The since parameter supports: "1h", "24h", "7d", "30d".
func (s *MetricsService) GetHistory(resolution, diskGroupID, since string) ([]db.LibraryHistory, error) {
	if resolution == "" {
		resolution = "raw"
	}

	query := s.db.Where("resolution = ?", resolution)
	if diskGroupID != "" {
		query = query.Where("disk_group_id = ?", diskGroupID)
	}

	// Apply time range filter
	if since != "" {
		var duration time.Duration
		switch since {
		case "1h":
			duration = 1 * time.Hour
		case "24h":
			duration = 24 * time.Hour
		case "7d":
			duration = 7 * 24 * time.Hour
		case "30d":
			duration = 30 * 24 * time.Hour
		}
		if duration > 0 {
			cutoff := time.Now().Add(-duration)
			query = query.Where("timestamp >= ?", cutoff)
		}
	}

	history := make([]db.LibraryHistory, 0)
	if err := query.Order("timestamp asc").Limit(1000).Find(&history).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch metrics history: %w", err)
	}

	return history, nil
}

// GetLifetimeStats returns the singleton lifetime stats row, creating it if it doesn't exist.
func (s *MetricsService) GetLifetimeStats() (db.LifetimeStats, error) {
	var stats db.LifetimeStats
	if err := s.db.FirstOrCreate(&stats, db.LifetimeStats{ID: 1}).Error; err != nil {
		return stats, fmt.Errorf("failed to fetch lifetime stats: %w", err)
	}
	return stats, nil
}

// GetDashboardStats aggregates lifetime stats, protected count, and library
// growth rate into a single response for the dashboard.
func (s *MetricsService) GetDashboardStats() (map[string]any, error) {
	// 1. Lifetime stats
	var lifetime db.LifetimeStats
	if err := s.db.FirstOrCreate(&lifetime, db.LifetimeStats{ID: 1}).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch lifetime stats: %w", err)
	}

	// 2. Protected count from engine service
	engineStats := s.engine.GetStats()
	protectedCount, _ := engineStats["protectedCount"].(int64)

	// 3. Library growth rate: compare most recent entry to 7 days ago
	growthBytes := int64(0)
	hasGrowthData := false

	cutoff := time.Now().Add(-7 * 24 * time.Hour)
	// Use Find+Limit instead of First to avoid GORM logging "record not found" —
	// having no library history is expected on fresh installs or after data resets.
	var recentRows []db.LibraryHistory
	s.db.Where("resolution = ?", "raw").
		Order("timestamp DESC").Limit(1).Find(&recentRows)
	if len(recentRows) > 0 {
		recent := recentRows[0]
		var weekAgoRows []db.LibraryHistory
		s.db.Where("resolution = ? AND timestamp <= ?", "raw", cutoff).
			Order("timestamp DESC").Limit(1).Find(&weekAgoRows)
		if len(weekAgoRows) > 0 {
			weekAgo := weekAgoRows[0]
			growthBytes = recent.UsedCapacity - weekAgo.UsedCapacity
			hasGrowthData = true
		}
	}

	return map[string]any{
		"totalBytesReclaimed": lifetime.TotalBytesReclaimed,
		"totalItemsRemoved":   lifetime.TotalItemsRemoved,
		"totalEngineRuns":     lifetime.TotalEngineRuns,
		"protectedCount":      protectedCount,
		"growthBytesPerWeek":  growthBytes,
		"hasGrowthData":       hasGrowthData,
	}, nil
}

// IncrementEngineRuns atomically increments the total_engine_runs counter.
func (s *MetricsService) IncrementEngineRuns() error {
	result := s.db.Model(&db.LifetimeStats{}).Where("id = 1").
		UpdateColumn("total_engine_runs", gorm.Expr("total_engine_runs + ?", 1))
	if result.Error != nil {
		return fmt.Errorf("failed to increment engine runs: %w", result.Error)
	}
	// Ensure the row exists (first run)
	if result.RowsAffected == 0 {
		s.db.FirstOrCreate(&db.LifetimeStats{}, db.LifetimeStats{ID: 1})
		s.db.Model(&db.LifetimeStats{}).Where("id = 1").
			UpdateColumn("total_engine_runs", gorm.Expr("total_engine_runs + ?", 1))
	}
	return nil
}

// IncrementDeletionStats atomically increments the lifetime stats counters
// for total bytes reclaimed and total items removed. Used by the DeletionService
// after a successful deletion.
func (s *MetricsService) IncrementDeletionStats(sizeBytes int64) error {
	result := s.db.Model(&db.LifetimeStats{}).Where("id = 1").
		UpdateColumns(map[string]any{
			"total_bytes_reclaimed": gorm.Expr("total_bytes_reclaimed + ?", sizeBytes),
			"total_items_removed":   gorm.Expr("total_items_removed + ?", 1),
		})
	if result.Error != nil {
		return fmt.Errorf("failed to increment deletion stats: %w", result.Error)
	}
	// Ensure the row exists (first run)
	if result.RowsAffected == 0 {
		s.db.FirstOrCreate(&db.LifetimeStats{}, db.LifetimeStats{ID: 1})
		s.db.Model(&db.LifetimeStats{}).Where("id = 1").
			UpdateColumns(map[string]any{
				"total_bytes_reclaimed": gorm.Expr("total_bytes_reclaimed + ?", sizeBytes),
				"total_items_removed":   gorm.Expr("total_items_removed + ?", 1),
			})
	}
	return nil
}

// RecordLibraryHistory records a library capacity snapshot for a disk group.
func (s *MetricsService) RecordLibraryHistory(diskGroupID uint, totalBytes, usedBytes int64) error {
	record := db.LibraryHistory{
		Timestamp:     time.Now().UTC(),
		TotalCapacity: totalBytes,
		UsedCapacity:  usedBytes,
		Resolution:    "raw",
		DiskGroupID:   &diskGroupID,
	}
	if err := s.db.Create(&record).Error; err != nil {
		return fmt.Errorf("failed to record library history: %w", err)
	}
	return nil
}

// RollupHistory aggregates raw library history entries into a coarser resolution.
// For each (disk_group_id, resolution) bucket, it averages the total and used capacity
// within the time window and creates a single summary row.
func (s *MetricsService) RollupHistory(fromRes, toRes string, start, end time.Time) error {
	// Get distinct disk group IDs with data in the range
	type groupRow struct {
		DiskGroupID *uint
	}
	var groups []groupRow
	s.db.Model(&db.LibraryHistory{}).
		Select("DISTINCT disk_group_id").
		Where("resolution = ? AND timestamp >= ? AND timestamp < ?", fromRes, start, end).
		Find(&groups)

	for _, g := range groups {
		var avgTotal, avgUsed float64
		row := s.db.Model(&db.LibraryHistory{}).
			Select("AVG(total_capacity) as avg_total, AVG(used_capacity) as avg_used").
			Where("resolution = ? AND disk_group_id = ? AND timestamp >= ? AND timestamp < ?",
				fromRes, g.DiskGroupID, start, end)
		sqlRow := row.Row()
		if sqlRow != nil {
			if err := sqlRow.Scan(&avgTotal, &avgUsed); err != nil {
				continue // skip this group if scan fails
			}
		}

		summary := db.LibraryHistory{
			Timestamp:     start,
			TotalCapacity: int64(avgTotal),
			UsedCapacity:  int64(avgUsed),
			Resolution:    toRes,
			DiskGroupID:   g.DiskGroupID,
		}
		s.db.Create(&summary)
	}

	return nil
}

// PruneHistory deletes library history entries matching the given resolution
// that are older than the given time.
func (s *MetricsService) PruneHistory(resolution string, before time.Time) (int64, error) {
	result := s.db.Where("resolution = ? AND timestamp < ?", resolution, before).
		Delete(&db.LibraryHistory{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to prune history: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// ─── Capacity forecast ──────────────────────────────────────────────────────

// DiskGroupForForecast holds the disk group parameters needed for forecast computation.
// Extracted from the route handler to avoid passing the full db.DiskGroup model.
type DiskGroupForForecast struct {
	ID            uint
	ThresholdPct  float64
	TotalCapacity int64
	UsedCapacity  int64
}

// CapacityForecast holds projected capacity data based on linear regression of
// recent usage history.
type CapacityForecast struct {
	CurrentUsedPct     float64 `json:"currentUsedPct"`
	GrowthRatePerDay   int64   `json:"growthRatePerDay"`   // bytes/day
	DaysUntilThreshold int     `json:"daysUntilThreshold"` // -1 if shrinking
	DaysUntilFull      int     `json:"daysUntilFull"`      // -1 if shrinking
	TotalCapacity      int64   `json:"totalCapacity"`
	UsedCapacity       int64   `json:"usedCapacity"`
}

// GetCapacityForecast computes a linear regression on the last 30 days of
// capacity history and projects when the given threshold (and 100%) will be
// reached. Returns nil, nil if there is insufficient data (< 2 data points).
func (s *MetricsService) GetCapacityForecast(thresholdPct float64, totalCapacity, usedCapacity int64) (*CapacityForecast, error) {
	history, err := s.GetHistory("raw", "", "30d")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch history for forecast: %w", err)
	}

	if len(history) < 2 {
		// Not enough data for regression — return current state with no projections
		currentPct := 0.0
		if totalCapacity > 0 {
			currentPct = float64(usedCapacity) / float64(totalCapacity) * 100
		}
		return &CapacityForecast{
			CurrentUsedPct:     currentPct,
			GrowthRatePerDay:   0,
			DaysUntilThreshold: -1,
			DaysUntilFull:      -1,
			TotalCapacity:      totalCapacity,
			UsedCapacity:       usedCapacity,
		}, nil
	}

	// Perform least-squares linear regression on usedCapacity over time.
	// x = days since first data point, y = usedCapacity bytes.
	baseTime := history[0].Timestamp
	n := float64(len(history))
	var sumX, sumY, sumXY, sumX2 float64

	for _, h := range history {
		x := h.Timestamp.Sub(baseTime).Hours() / 24.0 // days
		y := float64(h.UsedCapacity)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// slope = (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	denominator := n*sumX2 - sumX*sumX
	var slope float64
	if denominator != 0 {
		slope = (n*sumXY - sumX*sumY) / denominator
	}

	growthRatePerDay := int64(slope) // bytes/day

	currentPct := 0.0
	if totalCapacity > 0 {
		currentPct = float64(usedCapacity) / float64(totalCapacity) * 100
	}

	daysUntilThreshold := -1
	daysUntilFull := -1

	if growthRatePerDay > 0 && totalCapacity > 0 {
		thresholdBytes := int64(float64(totalCapacity) * thresholdPct / 100.0)
		remaining := thresholdBytes - usedCapacity
		if remaining > 0 {
			daysUntilThreshold = int(remaining / growthRatePerDay)
		} else {
			daysUntilThreshold = 0 // Already past threshold
		}

		fullRemaining := totalCapacity - usedCapacity
		if fullRemaining > 0 {
			daysUntilFull = int(fullRemaining / growthRatePerDay)
		} else {
			daysUntilFull = 0 // Already full
		}
	}

	return &CapacityForecast{
		CurrentUsedPct:     currentPct,
		GrowthRatePerDay:   growthRatePerDay,
		DaysUntilThreshold: daysUntilThreshold,
		DaysUntilFull:      daysUntilFull,
		TotalCapacity:      totalCapacity,
		UsedCapacity:       usedCapacity,
	}, nil
}

// GetWorkerMetrics assembles worker metrics from the EngineService and DeletionService.
// Keys match the frontend TypeScript WorkerStats interface.
func (s *MetricsService) GetWorkerMetrics() map[string]any {
	stats := s.engine.GetStats()

	// Add poll interval and execution mode from preferences via SettingsService.
	// The execution mode MUST come from the preferences table (source of truth),
	// not from the last EngineRunStats record (which reflects the mode at the time
	// of the last run, not the current configured mode).
	if prefs, err := s.settings.GetPreferences(); err == nil {
		stats["pollIntervalSeconds"] = prefs.PollIntervalSeconds
		stats["executionMode"] = prefs.ExecutionMode
	}

	// Add deletion worker state
	stats["queueDepth"] = s.deletion.QueueLen()
	stats["currentlyDeleting"] = s.deletion.CurrentlyDeleting()
	stats["processed"] = s.deletion.Processed()
	stats["failed"] = s.deletion.Failed()

	return stats
}
