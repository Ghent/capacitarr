// Package jobs manages scheduled background tasks and cron execution.
package jobs

import (
	"log/slog"
	"time"

	"capacitarr/internal/services"

	"github.com/robfig/cron/v3"
)

// Start creates and starts the background cron scheduler for time-series rollup and pruning jobs.
func Start(reg *services.Registry) *cron.Cron {
	c := cron.New()

	// 1. Rollup "raw" to "hourly" every hour at minute 0.
	// Uses persisted checkpoints so rollups are idempotent and delay-tolerant.
	_, err := c.AddFunc("@hourly", func() {
		slog.Info("Running hourly rollup", "component", "jobs")
		start := reg.Metrics.GetRollupCheckpoint("hourly")
		end := time.Now().UTC().Truncate(time.Hour)
		if !start.IsZero() && end.After(start) {
			if rollupErr := reg.Metrics.RollupHistory("raw", "hourly", start, end); rollupErr != nil {
				slog.Error("Hourly rollup failed", "component", "jobs", "error", rollupErr)
			} else if cpErr := reg.Metrics.SetRollupCheckpoint("hourly", end); cpErr != nil {
				slog.Error("Failed to persist hourly rollup checkpoint — next tick may reprocess data",
					"component", "jobs", "error", cpErr)
			}
		}
		// Keep raw data for 2 hours (enough for real-time zooming)
		if _, pruneErr := reg.Metrics.PruneHistory("raw", time.Now().Add(-2*time.Hour)); pruneErr != nil {
			slog.Error("Failed to prune raw history", "component", "jobs", "error", pruneErr)
		}
	})
	if err != nil {
		slog.Error("Failed to add hourly cron", "component", "jobs", "operation", "add_cron", "error", err)
	}

	// 2. Rollup "hourly" to "daily" every day at midnight.
	_, err = c.AddFunc("@daily", func() {
		slog.Info("Running daily rollup", "component", "jobs")
		start := reg.Metrics.GetRollupCheckpoint("daily")
		end := time.Now().UTC().Truncate(24 * time.Hour)
		if !start.IsZero() && end.After(start) {
			if rollupErr := reg.Metrics.RollupHistory("hourly", "daily", start, end); rollupErr != nil {
				slog.Error("Daily rollup failed", "component", "jobs", "error", rollupErr)
			} else if cpErr := reg.Metrics.SetRollupCheckpoint("daily", end); cpErr != nil {
				slog.Error("Failed to persist daily rollup checkpoint — next tick may reprocess data",
					"component", "jobs", "error", cpErr)
			}
		}
		// Keep hourly snapshots for 7 days
		if _, pruneErr := reg.Metrics.PruneHistory("hourly", time.Now().Add(-7*24*time.Hour)); pruneErr != nil {
			slog.Error("Failed to prune hourly history", "component", "jobs", "error", pruneErr)
		}
	})
	if err != nil {
		slog.Error("Failed to add daily cron", "component", "jobs", "operation", "add_cron", "error", err)
	}

	// 3. Rollup "daily" to "weekly" every week on Sunday at midnight.
	_, err = c.AddFunc("@weekly", func() {
		slog.Info("Running weekly rollup", "component", "jobs")
		start := reg.Metrics.GetRollupCheckpoint("weekly")
		end := time.Now().UTC().Truncate(24 * time.Hour)
		if !start.IsZero() && end.After(start) {
			if rollupErr := reg.Metrics.RollupHistory("daily", "weekly", start, end); rollupErr != nil {
				slog.Error("Weekly rollup failed", "component", "jobs", "error", rollupErr)
			} else if cpErr := reg.Metrics.SetRollupCheckpoint("weekly", end); cpErr != nil {
				slog.Error("Failed to persist weekly rollup checkpoint — next tick may reprocess data",
					"component", "jobs", "error", cpErr)
			}
		}
		// Keep daily snapshots for 30 days
		if _, pruneErr := reg.Metrics.PruneHistory("daily", time.Now().Add(-30*24*time.Hour)); pruneErr != nil {
			slog.Error("Failed to prune daily history", "component", "jobs", "error", pruneErr)
		}
	})
	if err != nil {
		slog.Error("Failed to add weekly cron", "component", "jobs", "operation", "add_cron", "error", err)
	}

	// 4. Prune "weekly" data older than 1 year
	_, err = c.AddFunc("@monthly", func() {
		slog.Info("Running pruning of old data", "component", "jobs")
		if _, pruneErr := reg.Metrics.PruneHistory("weekly", time.Now().Add(-365*24*time.Hour)); pruneErr != nil {
			slog.Error("Failed to prune weekly history", "component", "jobs", "error", pruneErr)
		}
	})
	if err != nil {
		slog.Error("Failed to add monthly cron", "component", "jobs", "operation", "add_cron", "error", err)
	}

	// 5. Prune old engine run stats — keep the last 1000 rows
	_, err = c.AddFunc("@daily", func() {
		if deleted, pruneErr := reg.Engine.PruneOldStats(1000); pruneErr != nil {
			slog.Error("Failed to prune engine run stats", "component", "jobs", "error", pruneErr)
		} else if deleted > 0 {
			slog.Info("Pruned old engine run stats", "component", "jobs", "deleted", deleted, "kept", 1000)
		}
	})
	if err != nil {
		slog.Error("Failed to add engine stats cleanup cron", "component", "jobs", "operation", "add_cron", "error", err)
	}

	// 6. Prune old activity events — fixed 7-day retention
	_, err = c.AddFunc("@daily", func() {
		if deleted, pruneErr := reg.Settings.PruneOldActivities(7); pruneErr != nil {
			slog.Error("Failed to prune activity events", "component", "jobs", "error", pruneErr)
		} else if deleted > 0 {
			slog.Info("Pruned old activity events", "component", "jobs", "deleted", deleted, "retention", "7 days")
		}
	})
	if err != nil {
		slog.Error("Failed to add activity events cleanup cron", "component", "jobs", "operation", "add_cron", "error", err)
	}

	// 7. Prune old audit log entries — uses audit log retention setting
	_, err = c.AddFunc("@daily", func() {
		prefs, prefsErr := reg.Settings.GetPreferences()
		if prefsErr != nil {
			slog.Error("Failed to fetch preferences for audit log cleanup", "component", "jobs", "error", prefsErr)
			return
		}
		if deleted, pruneErr := reg.AuditLog.PruneOlderThan(prefs.AuditLogRetentionDays); pruneErr != nil {
			slog.Error("Failed to prune audit log", "component", "jobs", "error", pruneErr)
		} else if deleted > 0 {
			slog.Info("Pruned old audit log entries", "component", "jobs", "deleted", deleted, "retentionDays", prefs.AuditLogRetentionDays)
		}
	})
	if err != nil {
		slog.Error("Failed to add audit log cleanup cron", "component", "jobs", "operation", "add_cron", "error", err)
	}

	// Job 8: Daily database backup — VACUUM INTO with rotation
	_, err = c.AddFunc("@daily", func() {
		if backupErr := reg.DatabaseBackup.RunScheduledBackup(); backupErr != nil {
			slog.Error("Scheduled database backup failed", "component", "jobs", "error", backupErr)
		}
	})
	if err != nil {
		slog.Error("Failed to add database backup cron", "component", "jobs", "operation", "add_cron", "error", err)
	}

	// Job 9: Daily sunset processing — expire countdowns, rescore, cleanup saved, update poster overlays.
	_, err = c.AddFunc("@daily", func() {
		// Build integration registry for label/poster operations
		registry, registryErr := reg.Integration.BuildIntegrationRegistry()
		if registryErr != nil {
			slog.Error("Failed to build integration registry for sunset cron", "component", "jobs", "error", registryErr)
		}

		// Load preferences and weights early — needed by rescore + poster overlay steps
		prefs, prefsErr := reg.Settings.GetPreferences()
		if prefsErr != nil {
			slog.Error("Failed to load preferences for sunset cron", "component", "jobs", "error", prefsErr)
		}
		weights, weightsErr := reg.Settings.GetWeightMap()
		if weightsErr != nil {
			slog.Error("Failed to load scoring weights for sunset cron", "component", "jobs", "error", weightsErr)
		}

		sunsetDeps := services.SunsetDeps{
			Registry:      registry,
			Deletion:      reg.Deletion,
			Engine:        reg.Engine,
			Settings:      reg.Settings,
			Preview:       reg.Preview,
			PosterOverlay: reg.PosterOverlay,
			Mapping:       reg.Mapping,
		}

		// 1. Process expired sunset items → DeletionService
		processed, sunsetErr := reg.Sunset.ProcessExpired(sunsetDeps)
		if sunsetErr != nil {
			slog.Error("Failed to process expired sunset items", "component", "jobs", "error", sunsetErr)
		} else if processed > 0 {
			slog.Info("Processed expired sunset items", "component", "jobs", "count", processed)
		}

		// 2. Rescore pending items and save those with dropped scores (if enabled)
		if prefsErr == nil && prefs.SunsetRescoreEnabled {
			rescored, rescoreErr := reg.Sunset.RescoreAndSave(sunsetDeps, prefs, weights)
			if rescoreErr != nil {
				slog.Error("Failed to rescore sunset items", "component", "jobs", "error", rescoreErr)
			} else if rescored > 0 {
				slog.Info("Sunset items saved by popular demand", "component", "jobs", "count", rescored)
			}
		}

		// 3. Cleanup saved items whose marker duration has expired
		cleaned, cleanErr := reg.Sunset.CleanupSaved(sunsetDeps)
		if cleanErr != nil {
			slog.Error("Failed to cleanup saved sunset items", "component", "jobs", "error", cleanErr)
		} else if cleaned > 0 {
			slog.Info("Cleaned up expired saved sunset markers", "component", "jobs", "count", cleaned)
		}

		// 4. Update poster overlays (if enabled and service is available)
		if reg.PosterOverlay != nil {
			if prefsErr == nil && prefs.PosterOverlayStyle != "off" {
				if _, overlayErr := reg.PosterOverlay.UpdateAll(reg.Sunset, prefs.PosterOverlayStyle, services.PosterDeps{
					Registry: registry,
					Mapping:  reg.Mapping,
				}); overlayErr != nil {
					slog.Error("Failed to update poster overlays", "component", "jobs", "error", overlayErr)
				}
			}
		}

		// 5. Garbage collect stale media server ID mappings (Layer 3)
		if reg.Mapping != nil {
			if cleaned, gcErr := reg.Mapping.GarbageCollect(7 * 24 * time.Hour); gcErr != nil {
				slog.Error("Failed to garbage collect media server mappings",
					"component", "jobs", "error", gcErr)
			} else if cleaned > 0 {
				slog.Info("Garbage collected stale media server mappings",
					"component", "jobs", "removed", cleaned)
			}
		}
	})
	if err != nil {
		slog.Error("Failed to add sunset expiry cron", "component", "jobs", "operation", "add_cron", "error", err)
	}

	c.Start()
	slog.Info("Cron jobs started successfully", "component", "jobs")
	return c
}
