package poller

import (
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
	"capacitarr/internal/notifications"

	"gorm.io/gorm"
)

// RunNowCh allows triggering an immediate engine evaluation cycle from the API.
var RunNowCh = make(chan struct{}, 1)

// pollRunning prevents concurrent poll() executions when a manual "Run Now"
// overlaps with a ticker-triggered poll.
var pollRunning atomic.Bool

// Start begins the continuous polling loop and returns a stop function.
// It queries all enabled integrations, fetches disk space for media root folders only,
// updates DiskGroups, and records a LibraryHistory snapshot per disk group.
// The poll interval is read from the database on each cycle, allowing dynamic
// reconfiguration without restart.
func Start() func() {
	done := make(chan struct{})
	go func() {
		timer := time.NewTimer(getPollInterval())
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				poll()
				timer.Reset(getPollInterval())
			case <-RunNowCh:
				slog.Info("Manual run triggered via API", "component", "poller")
				poll()
				// Don't reset the timer — let the next scheduled tick proceed normally
			case <-done:
				return
			}
		}
	}()
	return func() {
		close(done)
	}
}

// getPollInterval reads PollIntervalSeconds from the database preference set.
// Falls back to 300s (5 min) if not set, and enforces a 30s minimum.
func getPollInterval() time.Duration {
	var prefs db.PreferenceSet
	if err := db.DB.First(&prefs, 1).Error; err != nil {
		return 5 * time.Minute
	}
	secs := prefs.PollIntervalSeconds
	if secs < 30 {
		secs = 300
	}
	return time.Duration(secs) * time.Second
}

// StopWorker closes the delete queue channel so the deletion worker can drain and exit.
func StopWorker() {
	close(deleteQueue)
}

func poll() {
	if !pollRunning.CompareAndSwap(false, true) {
		slog.Info("Skipping poll — previous run still in progress", "component", "poller")
		return
	}
	defer pollRunning.Store(false)

	pollStart := time.Now()

	// Increment lifetime engine runs counter (atomic DB update)
	db.DB.Model(&db.LifetimeStats{}).Where("id = 1").
		UpdateColumn("total_engine_runs", gorm.Expr("total_engine_runs + ?", 1))

	// Reset per-run counters at the start of each poll cycle
	atomic.StoreInt64(&lastRunEvaluated, 0)
	atomic.StoreInt64(&lastRunFlagged, 0)
	atomic.StoreInt64(&lastRunFreedBytes, 0)
	atomic.StoreInt64(&lastRunProtected, 0)

	var configs []db.IntegrationConfig
	if err := db.DB.Where("enabled = ?", true).Find(&configs).Error; err != nil {
		slog.Error("Failed to load integrations", "component", "poller", "operation", "load_integrations", "error", err)
		notifications.Dispatch(notifications.NotificationEvent{
			Type:    notifications.EventEngineError,
			Title:   "Engine Error",
			Message: fmt.Sprintf("Failed to load integrations: %v", err),
		})
		return
	}

	var prefs db.PreferenceSet
	if err := db.DB.FirstOrCreate(&prefs, db.PreferenceSet{ID: 1}).Error; err != nil {
		slog.Error("Failed to load preferences", "component", "poller", "operation", "load_preferences", "error", err)
		return
	}

	slog.Debug("Poll cycle starting", "component", "poller",
		"enabledIntegrations", len(configs),
		"pollInterval", prefs.PollIntervalSeconds,
		"executionMode", prefs.ExecutionMode)

	// Prune old audit logs
	if prefs.AuditLogRetentionDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -prefs.AuditLogRetentionDays)
		if err := db.DB.Where("created_at < ?", cutoff).Delete(&db.AuditLog{}).Error; err != nil {
			slog.Error("Failed to prune old audit logs", "component", "poller", "operation", "prune_audit_logs", "error", err)
		}
	}

	if len(configs) == 0 {
		slog.Debug("No enabled integrations, skipping poll", "component", "poller")
		return
	}

	// Fetch media items, disk space, and enrichment clients from all integrations
	fetched := fetchAllIntegrations(configs)

	// Enrich items with watch history and request data
	enrichItems(fetched.allItems, fetched.enrichment)

	// Find the most specific mount for each root folder
	mediaMounts := findMediaMounts(fetched.diskMap, fetched.rootFolders)

	// Update DiskGroups and record history only for media mounts
	for mountPath := range mediaMounts {
		disk := fetched.diskMap[mountPath]
		usedBytes := disk.TotalBytes - disk.FreeBytes

		// Upsert DiskGroup
		var group db.DiskGroup
		result := db.DB.Where("mount_path = ?", mountPath).First(&group)
		if result.Error != nil {
			group = db.DiskGroup{
				MountPath:  mountPath,
				TotalBytes: disk.TotalBytes,
				UsedBytes:  usedBytes,
			}
			db.DB.Create(&group)
		} else {
			db.DB.Model(&group).Updates(map[string]interface{}{
				"total_bytes": disk.TotalBytes,
				"used_bytes":  usedBytes,
			})
			// Update the local struct values for threshold check
			group.TotalBytes = disk.TotalBytes
			group.UsedBytes = usedBytes
		}

		// Record LibraryHistory snapshot
		record := db.LibraryHistory{
			Timestamp:     time.Now(),
			TotalCapacity: disk.TotalBytes,
			UsedCapacity:  usedBytes,
			Resolution:    "raw",
			DiskGroupID:   &group.ID,
		}
		if err := db.DB.Create(&record).Error; err != nil {
			slog.Error("Failed to save capacity record", "component", "poller", "operation", "save_capacity",
				"mount", mountPath, "error", err)
		}

		// Evaluate and trigger cleanup if threshold breached
		evaluateAndCleanDisk(group, fetched.allItems, fetched.serviceClients)
	}

	// Clean up orphaned disk groups that are no longer media mounts
	if len(mediaMounts) > 0 {
		var allGroups []db.DiskGroup
		db.DB.Find(&allGroups)
		for _, g := range allGroups {
			if !mediaMounts[g.MountPath] {
				slog.Info("Removing orphaned disk group", "component", "poller",
					"mount", g.MountPath, "id", g.ID)
				db.DB.Where("disk_group_id = ?", g.ID).Delete(&db.LibraryHistory{})
				db.DB.Delete(&g)
			}
		}
	}

	// Persist engine run stats to DB so they survive container restarts
	runStats := db.EngineRunStats{
		RunAt:         pollStart,
		Evaluated:     int(atomic.LoadInt64(&lastRunEvaluated)),
		Flagged:       int(atomic.LoadInt64(&lastRunFlagged)),
		FreedBytes:    atomic.LoadInt64(&lastRunFreedBytes),
		ExecutionMode: prefs.ExecutionMode,
		DurationMs:    time.Since(pollStart).Milliseconds(),
	}
	if err := db.DB.Create(&runStats).Error; err != nil {
		slog.Error("Failed to persist engine run stats", "component", "poller", "operation", "persist_stats", "error", err)
	}

	// Notify: engine cycle complete
	evaluated := atomic.LoadInt64(&lastRunEvaluated)
	flagged := atomic.LoadInt64(&lastRunFlagged)
	notifications.Dispatch(notifications.NotificationEvent{
		Type:    notifications.EventEngineComplete,
		Title:   "Engine Cycle Complete",
		Message: fmt.Sprintf("Evaluated %d items, flagged %d for removal in %s", evaluated, flagged, time.Since(pollStart).String()),
		Fields: map[string]string{
			"Evaluated": fmt.Sprintf("%d", evaluated),
			"Flagged":   fmt.Sprintf("%d", flagged),
			"Duration":  time.Since(pollStart).String(),
			"Mode":      prefs.ExecutionMode,
		},
	})

	slog.Debug("Poll cycle complete", "component", "poller",
		"duration", time.Since(pollStart).String(),
		"totalItems", len(fetched.allItems),
		"evaluated", evaluated,
		"flagged", flagged,
		"protected", atomic.LoadInt64(&lastRunProtected))
}

// findMediaMounts returns only the mount paths that are the most specific match
// for at least one root folder. For example, if mounts are ["/", "/media"] and
// root folder is "/media/movies", only "/media" is returned (not "/").
func findMediaMounts(diskMap map[string]integrations.DiskSpace, rootFolders map[string]bool) map[string]bool {
	mediaMounts := make(map[string]bool)

	for rf := range rootFolders {
		cleanRF := strings.TrimRight(rf, "/")
		bestMount := ""
		bestLen := 0

		for mountPath := range diskMap {
			cleanMount := strings.TrimRight(mountPath, "/")
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
		}
	}

	// If we have both "/" and other more specific mounts, drop "/"
	// This handles Docker/container scenarios where different services
	// see different mount namespaces for the same underlying storage
	if len(mediaMounts) > 1 {
		for m := range mediaMounts {
			if strings.TrimRight(m, "/") == "" {
				slog.Debug("Dropping root mount '/' since more specific mounts exist", "component", "poller")
				delete(mediaMounts, m)
			}
		}
	}

	return mediaMounts
}

// CreateClient constructs an integration client based on the integration type.
// Exported for use by the approval route handler to reconstruct clients from
// stored IntegrationConfig records.
func CreateClient(intType, url, apiKey string) integrations.Integration {
	switch intType {
	case "sonarr":
		return integrations.NewSonarrClient(url, apiKey)
	case "radarr":
		return integrations.NewRadarrClient(url, apiKey)
	case "lidarr":
		return integrations.NewLidarrClient(url, apiKey)
	case "readarr":
		return integrations.NewReadarrClient(url, apiKey)
	case "plex":
		return integrations.NewPlexClient(url, apiKey)
	default:
		return nil
	}
}
