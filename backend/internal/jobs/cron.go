package jobs

import (
	"log/slog"
	"time"

	"capacitarr/internal/db"
	"github.com/robfig/cron/v3"
)

// Start creates and starts the background cron scheduler for time-series rollup and pruning jobs.
func Start() *cron.Cron {
	c := cron.New()

	// 1. Rollup "raw" to "hourly" every hour at minute 0
	_, err := c.AddFunc("@hourly", func() {
		slog.Info("Running hourly rollup", "component", "jobs")
		rollupData("raw", "hourly", time.Now().Add(-time.Hour).Truncate(time.Hour), time.Now().Truncate(time.Hour))
		// Keep raw data for 2 hours (enough for real-time zooming)
		pruneData("raw", time.Now().Add(-2*time.Hour))
	})
	if err != nil {
		slog.Error("Failed to add hourly cron", "component", "jobs", "operation", "add_cron", "error", err)
	}

	// 2. Rollup "hourly" to "daily" every day at midnight
	_, err = c.AddFunc("@daily", func() {
		slog.Info("Running daily rollup", "component", "jobs")
		rollupData("hourly", "daily", time.Now().Add(-24*time.Hour).Truncate(24*time.Hour), time.Now().Truncate(24*time.Hour))
		// Keep hourly snapshots for 7 days
		pruneData("hourly", time.Now().Add(-7*24*time.Hour))
	})
	if err != nil {
		slog.Error("Failed to add daily cron", "component", "jobs", "operation", "add_cron", "error", err)
	}

	// 3. Rollup "daily" to "weekly" every week on Sunday at midnight
	_, err = c.AddFunc("@weekly", func() {
		slog.Info("Running weekly rollup", "component", "jobs")
		rollupData("daily", "weekly", time.Now().Add(-7*24*time.Hour).Truncate(24*time.Hour), time.Now().Truncate(24*time.Hour))
		// Keep daily snapshots for 30 days
		pruneData("daily", time.Now().Add(-30*24*time.Hour))
	})
	if err != nil {
		slog.Error("Failed to add weekly cron", "component", "jobs", "operation", "add_cron", "error", err)
	}

	// 4. Prune "weekly" data older than 1 year
	_, err = c.AddFunc("@monthly", func() {
		slog.Info("Running pruning of old data", "component", "jobs")
		pruneData("weekly", time.Now().Add(-365*24*time.Hour))
	})
	if err != nil {
		slog.Error("Failed to add monthly cron", "component", "jobs", "operation", "add_cron", "error", err)
	}

	// 5. Prune old engine run stats — keep the last 1000 rows
	_, err = c.AddFunc("@daily", func() {
		pruneEngineRunStats(1000)
	})
	if err != nil {
		slog.Error("Failed to add engine stats cleanup cron", "component", "jobs", "operation", "add_cron", "error", err)
	}

	c.Start()
	slog.Info("Cron jobs started successfully", "component", "jobs")
	return c
}

func rollupData(fromRes, toRes string, start, end time.Time) {
	// Query distinct disk_group_id values from the source resolution in the time window
	var groupIDs []uint
	err := db.DB.Model(&db.LibraryHistory{}).
		Where("resolution = ? AND timestamp >= ? AND timestamp < ? AND disk_group_id IS NOT NULL", fromRes, start, end).
		Distinct("disk_group_id").
		Pluck("disk_group_id", &groupIDs).Error
	if err != nil {
		slog.Error("Failed to query distinct disk group IDs for rollup", "component", "jobs", "operation", "rollup_query", "from", fromRes, "error", err)
		return
	}

	// For each disk group, compute average capacity and create a rollup record
	for _, gid := range groupIDs {
		var avgResult struct {
			AvgTotal float64
			AvgUsed  float64
		}

		err := db.DB.Model(&db.LibraryHistory{}).
			Select("AVG(total_capacity) as avg_total, AVG(used_capacity) as avg_used").
			Where("resolution = ? AND timestamp >= ? AND timestamp < ? AND disk_group_id = ?", fromRes, start, end, gid).
			Scan(&avgResult).Error

		if err != nil {
			slog.Error("Failed to calculate average for rollup", "component", "jobs", "operation", "rollup_calculate", "from", fromRes, "diskGroupId", gid, "error", err)
			continue
		}

		if avgResult.AvgTotal > 0 {
			diskGroupID := gid
			record := db.LibraryHistory{
				Timestamp:     start,
				TotalCapacity: int64(avgResult.AvgTotal),
				UsedCapacity:  int64(avgResult.AvgUsed),
				Resolution:    toRes,
				DiskGroupID:   &diskGroupID,
			}
			if err := db.DB.Create(&record).Error; err != nil {
				slog.Error("Failed to save rollup record", "component", "jobs", "operation", "rollup_save", "to", toRes, "diskGroupId", gid, "error", err)
			}
		}
	}
}

func pruneData(resolution string, before time.Time) {
	if err := db.DB.Where("resolution = ? AND timestamp < ?", resolution, before).Delete(&db.LibraryHistory{}).Error; err != nil {
		slog.Error("Failed to prune data", "component", "jobs", "operation", "prune", "resolution", resolution, "error", err)
	}
}

// pruneEngineRunStats keeps only the most recent `keep` rows in engine_run_stats.
func pruneEngineRunStats(keep int) {
	var count int64
	db.DB.Model(&db.EngineRunStats{}).Count(&count)
	if count <= int64(keep) {
		return
	}

	// Find the ID threshold — delete everything below the Nth newest row
	var cutoffRow db.EngineRunStats
	result := db.DB.Order("run_at DESC").Offset(keep).Limit(1).First(&cutoffRow)
	if result.Error != nil {
		return
	}

	deleted := db.DB.Where("run_at <= ?", cutoffRow.RunAt).Delete(&db.EngineRunStats{})
	if deleted.Error != nil {
		slog.Error("Failed to prune engine run stats", "component", "jobs", "operation", "prune_engine_stats", "error", deleted.Error)
	} else if deleted.RowsAffected > 0 {
		slog.Info("Pruned old engine run stats", "component", "jobs", "deleted", deleted.RowsAffected, "kept", keep)
	}
}
