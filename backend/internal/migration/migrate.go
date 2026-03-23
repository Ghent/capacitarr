// Package migration provides one-way data import from Capacitarr 1.x to 2.0.
// The migration reads configuration data from a 1.x SQLite database and imports
// it into the 2.0 schema. All reads from the 1.x database use raw SQL to avoid
// schema mismatches between the 1.x and 2.0 GORM models. Transient data
// (approval queue, audit log, engine stats, activity events) is NOT migrated
// since it has no value in the new system.
package migration

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"os"
	"time"

	_ "github.com/ncruces/go-sqlite3/embed" // embed: SQLite WASM binary required for gormlite
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"capacitarr/internal/db"
)

// backupFilename is the name of the 1.x database backup file created during
// the startup pre-init phase. The descriptive name makes it clear to users
// browsing their config directory that this is a pre-migration safety copy
// of their 1.x database and can be safely deleted after migration.
const backupFilename = "capacitarr.db.v1-pre-migration-backup"

// Result holds the outcome of a migration.
type Result struct {
	IntegrationsImported  int
	DiskGroupsImported    int
	RulesImported         int
	PreferencesImported   bool
	NotificationsImported int
}

// MigrateFrom imports configuration data from a 1.x database into the current 2.0 database.
// The source database is opened read-only. Only configuration data is imported:
//   - IntegrationConfig (with overseerr → seerr type transformation)
//   - DiskGroup (thresholds, target percentages, and overrides)
//   - PreferenceSet (with execution_mode forced to dry-run for safety)
//   - CustomRule (re-linked to new integration IDs via old→new mapping)
//   - NotificationConfig (with OnIntegrationStatus defaulting to true)
//
// Auth is NOT imported here — it was already auto-imported at startup by
// ImportAuthOnly() so the user could log in to trigger this migration.
//
// The migration is idempotent: if integrations already exist in the destination
// database (from a previous import), the function returns early to prevent
// duplicate records.
//
// Transient data is NOT imported: approval queue, audit log, engine stats, activity events.
func MigrateFrom(sourcePath string, destDB *gorm.DB) (*Result, error) {
	slog.Info("Starting 1.x → 2.0 migration", "component", "migration", "source", sourcePath)

	// Verify source exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("source database not found: %s", sourcePath)
	}

	// Open source database. Note: gormlite does not support URI parameters like
	// ?mode=ro — using them creates a rogue file with the literal "?mode=ro" in
	// the filename. We open in default mode and only perform read operations.
	sourceDB, err := gorm.Open(gormlite.Open(sourcePath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open source database: %w", err)
	}
	sourceSQLDB, _ := sourceDB.DB()
	defer func() {
		if err := sourceSQLDB.Close(); err != nil {
			slog.Warn("Failed to close source database", "error", err)
		}
	}()

	// Idempotency guard: if integrations already exist in the destination
	// database, this migration was already run. Return early to prevent
	// duplicate records from double-clicks or race conditions.
	var existingCount int64
	if err := destDB.Model(&db.IntegrationConfig{}).Count(&existingCount).Error; err == nil && existingCount > 0 {
		slog.Warn("Migration skipped — destination already has integrations (idempotency guard)",
			"component", "migration", "existingIntegrations", existingCount)
		return &Result{}, nil
	}

	ctx := context.Background()
	result := &Result{}

	// Import integrations first — we need the old→new ID mapping for rules.
	// Auth is NOT imported here; it was already auto-imported at startup.
	idMap, count, err := importIntegrations(ctx, sourceSQLDB, destDB)
	if err != nil {
		slog.Warn("Failed to import integrations", "component", "migration", "error", err)
	}
	result.IntegrationsImported = count

	// Import disk groups (thresholds, targets, overrides)
	dgCount, err := importDiskGroups(ctx, sourceSQLDB, destDB)
	if err != nil {
		slog.Warn("Failed to import disk groups", "component", "migration", "error", err)
	}
	result.DiskGroupsImported = dgCount

	// Import preferences (execution_mode forced to dry-run for safety)
	if err := importPreferences(ctx, sourceSQLDB, destDB); err != nil {
		slog.Warn("Failed to import preferences", "component", "migration", "error", err)
	} else {
		result.PreferencesImported = true
	}

	// Import rules (re-linked to new integration IDs via old→new mapping)
	rulesCount, err := importRules(ctx, sourceSQLDB, destDB, idMap)
	if err != nil {
		slog.Warn("Failed to import rules", "component", "migration", "error", err)
	}
	result.RulesImported = rulesCount

	// Import notifications (with OnIntegrationStatus defaulting to true)
	notifCount, err := importNotifications(ctx, sourceSQLDB, destDB)
	if err != nil {
		slog.Warn("Failed to import notifications", "component", "migration", "error", err)
	}
	result.NotificationsImported = notifCount

	slog.Info("Migration complete", "component", "migration",
		"integrations", result.IntegrationsImported,
		"diskGroups", result.DiskGroupsImported,
		"rules", result.RulesImported,
		"notifications", result.NotificationsImported,
		"preferences", result.PreferencesImported)

	return result, nil
}

// importIntegrations reads integrations from the 1.x database using raw SQL
// and creates them in the 2.0 database. Returns a mapping of old IDs to new
// IDs so that rules can be re-linked to the correct integration.
func importIntegrations(ctx context.Context, srcDB *sql.DB, dest *gorm.DB) (map[uint]uint, int, error) {
	idMap := make(map[uint]uint) // old ID → new ID

	rows, err := srcDB.QueryContext(ctx, "SELECT id, type, name, url, api_key, enabled FROM integration_configs")
	if err != nil {
		return idMap, 0, fmt.Errorf("failed to read integrations: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Warn("Failed to close integration rows", "component", "migration", "error", closeErr)
		}
	}()

	count := 0
	for rows.Next() {
		var (
			oldID   int64
			iType   string
			name    string
			url     string
			apiKey  string
			enabled bool
		)
		if err := rows.Scan(&oldID, &iType, &name, &url, &apiKey, &enabled); err != nil {
			slog.Warn("Failed to scan integration row", "component", "migration", "error", err)
			continue
		}

		target := db.IntegrationConfig{
			Type:    transformIntegrationType(iType),
			Name:    name,
			URL:     url,
			APIKey:  apiKey,
			Enabled: enabled,
		}
		if err := dest.Create(&target).Error; err != nil {
			slog.Warn("Failed to import integration", "component", "migration",
				"name", name, "type", iType, "error", err)
			continue
		}
		if oldID >= 0 && oldID <= math.MaxUint32 {
			idMap[uint(oldID)] = target.ID
		}
		count++
	}
	return idMap, count, rows.Err()
}

// transformIntegrationType handles the overseerr → seerr rename.
func transformIntegrationType(t string) string {
	if t == "overseerr" {
		return "seerr"
	}
	return t
}

// importDiskGroups reads disk group configuration (thresholds, targets, overrides)
// from the 1.x database and creates them in the 2.0 database. Only configuration
// fields are imported — discovery fields (total_bytes, used_bytes) are left at zero
// since they will be populated by the next poll cycle.
//
// This prevents the dangerous scenario where user-customized thresholds (e.g. 95%/90%)
// are silently reset to defaults (85%/75%), which could trigger mass unsupervised
// deletions on the first poll cycle.
func importDiskGroups(ctx context.Context, srcDB *sql.DB, dest *gorm.DB) (int, error) {
	rows, err := srcDB.QueryContext(ctx, "SELECT mount_path, threshold_pct, target_pct FROM disk_groups")
	if err != nil {
		// The 1.x database might not have a disk_groups table at all — this is
		// not an error, just means there are no disk groups to import.
		slog.Debug("No disk_groups table in source (may not exist in this 1.x version)",
			"component", "migration", "error", err)
		return 0, nil
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Warn("Failed to close disk group rows", "component", "migration", "error", closeErr)
		}
	}()

	count := 0
	for rows.Next() {
		var (
			mountPath    string
			thresholdPct float64
			targetPct    float64
		)
		if err := rows.Scan(&mountPath, &thresholdPct, &targetPct); err != nil {
			slog.Warn("Failed to scan disk group row", "component", "migration", "error", err)
			continue
		}

		// Try to also read total_bytes_override if the column exists.
		// Not all 1.x versions may have this column.
		var totalOverride *int64
		overrideRow := srcDB.QueryRowContext(ctx,
			"SELECT total_bytes_override FROM disk_groups WHERE mount_path = ?", mountPath)
		var override sql.NullInt64
		if scanErr := overrideRow.Scan(&override); scanErr == nil && override.Valid {
			v := override.Int64
			totalOverride = &v
		}

		group := db.DiskGroup{
			MountPath:          mountPath,
			ThresholdPct:       thresholdPct,
			TargetPct:          targetPct,
			TotalBytesOverride: totalOverride,
		}
		if err := dest.Create(&group).Error; err != nil {
			slog.Warn("Failed to import disk group", "component", "migration",
				"mount", mountPath, "error", err)
			continue
		}
		count++
		slog.Info("Imported disk group thresholds", "component", "migration",
			"mount", mountPath, "threshold", thresholdPct, "target", targetPct)
	}
	return count, rows.Err()
}

func importPreferences(ctx context.Context, srcDB *sql.DB, dest *gorm.DB) error {
	// Read 1.x preferences using raw SQL to handle missing columns gracefully
	var source struct {
		LogLevel              string
		AuditLogRetentionDays int
		PollIntervalSeconds   int
		WatchHistoryWeight    int
		LastWatchedWeight     int
		FileSizeWeight        int
		RatingWeight          int
		TimeInLibraryWeight   int
		SeriesStatusWeight    int
		ExecutionMode         string
		TiebreakerMethod      string
		DeletionsEnabled      bool
		SnoozeDurationHours   int
		CheckForUpdates       bool
	}

	row := srcDB.QueryRowContext(ctx, "SELECT log_level, audit_log_retention_days, poll_interval_seconds, "+
		"watch_history_weight, last_watched_weight, file_size_weight, rating_weight, "+
		"time_in_library_weight, series_status_weight, execution_mode, tiebreaker_method, "+
		"deletions_enabled, snooze_duration_hours, check_for_updates FROM preference_sets LIMIT 1")
	if err := row.Scan(
		&source.LogLevel, &source.AuditLogRetentionDays, &source.PollIntervalSeconds,
		&source.WatchHistoryWeight, &source.LastWatchedWeight, &source.FileSizeWeight,
		&source.RatingWeight, &source.TimeInLibraryWeight, &source.SeriesStatusWeight,
		&source.ExecutionMode, &source.TiebreakerMethod,
		&source.DeletionsEnabled, &source.SnoozeDurationHours, &source.CheckForUpdates,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil // No preferences to import, using defaults
		}
		return fmt.Errorf("failed to read preferences: %w", err)
	}

	// Log if the user's execution mode is being overridden for safety
	if source.ExecutionMode != db.ModeDryRun {
		slog.Info("Overriding execution mode to dry-run for safety after migration",
			"component", "migration", "original_mode", source.ExecutionMode)
	}

	// Update the 2.0 preferences (row already exists from db.Init seed).
	// Safety: always start in dry-run after migration so the user can verify
	// that disk group thresholds, integration links, and rules are correct
	// before enabling approval or auto mode. This prevents mass unsupervised
	// deletions against potentially incorrect settings.
	// Note: scoring factor weights are stored in the separate scoring_factor_weights
	// table in 2.0, so they are NOT included here — they are persisted below.
	if err := dest.Model(&db.PreferenceSet{}).Where("id = 1").Updates(map[string]interface{}{
		"log_level":                source.LogLevel,
		"audit_log_retention_days": source.AuditLogRetentionDays,
		"poll_interval_seconds":    source.PollIntervalSeconds,
		"execution_mode":           db.ModeDryRun, // Safety: force dry-run after migration
		"tiebreaker_method":        source.TiebreakerMethod,
		"deletions_enabled":        source.DeletionsEnabled,
		"snooze_duration_hours":    source.SnoozeDurationHours,
		"check_for_updates":        source.CheckForUpdates,
		// New 2.0 fields keep their defaults (already seeded)
	}).Error; err != nil {
		return fmt.Errorf("failed to update preferences: %w", err)
	}

	// Persist 1.x scoring factor weights into the 2.0 scoring_factor_weights table.
	// The rows are already seeded by db.Init with default weight=5; we update them
	// to carry over the user's 1.x customizations.
	weightMap := map[string]int{
		"watch_history":   source.WatchHistoryWeight,
		"last_watched":    source.LastWatchedWeight,
		"file_size":       source.FileSizeWeight,
		"rating":          source.RatingWeight,
		"time_in_library": source.TimeInLibraryWeight,
		"series_status":   source.SeriesStatusWeight,
	}
	for factorKey, weight := range weightMap {
		if err := dest.Model(&db.ScoringFactorWeight{}).
			Where("factor_key = ?", factorKey).
			Update("weight", weight).Error; err != nil {
			slog.Warn("Failed to import scoring factor weight",
				"component", "migration", "factor_key", factorKey, "error", err)
		}
	}

	return nil
}

// importRules reads rules from the 1.x database using raw SQL and creates them
// in the 2.0 database. The idMap maps old integration IDs to their new 2.0 IDs
// so that rules can be re-linked to the correct integration. Rules whose old
// integration ID has no mapping (e.g. the integration failed to import) are
// imported with IntegrationID=nil and a warning is logged.
func importRules(ctx context.Context, srcDB *sql.DB, dest *gorm.DB, idMap map[uint]uint) (int, error) {
	rows, err := srcDB.QueryContext(ctx,
		"SELECT field, operator, value, effect, enabled, sort_order, integration_id "+
			"FROM custom_rules")
	if err != nil {
		return 0, fmt.Errorf("failed to read rules: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Warn("Failed to close rule rows", "component", "migration", "error", closeErr)
		}
	}()

	count := 0
	for rows.Next() {
		var (
			field         string
			operator      string
			value         string
			effect        string
			enabled       bool
			sortOrder     int
			integrationID sql.NullInt64
		)
		if err := rows.Scan(&field, &operator, &value, &effect, &enabled, &sortOrder, &integrationID); err != nil {
			slog.Warn("Failed to scan rule row", "component", "migration", "error", err)
			continue
		}

		target := db.CustomRule{
			Field:     field,
			Operator:  operator,
			Value:     value,
			Effect:    effect,
			Enabled:   enabled,
			SortOrder: sortOrder,
		}

		// Re-link to the new integration ID using the old→new mapping
		if integrationID.Valid && integrationID.Int64 >= 0 && integrationID.Int64 <= math.MaxUint32 {
			oldID := uint(integrationID.Int64)
			if newID, ok := idMap[oldID]; ok {
				target.IntegrationID = &newID
			} else {
				slog.Warn("Rule references unknown integration — importing without integration link",
					"component", "migration", "field", field, "oldIntegrationID", oldID)
			}
		}

		if err := dest.Create(&target).Error; err != nil {
			slog.Warn("Failed to import rule", "component", "migration",
				"field", field, "error", err)
			continue
		}
		count++
	}
	return count, rows.Err()
}

// importNotifications reads notification channels from the 1.x database using
// raw SQL and creates them in the 2.0 database. The new 2.0 field
// OnIntegrationStatus is set to true (matching the default for freshly-created
// channels) since it didn't exist in 1.x.
func importNotifications(ctx context.Context, srcDB *sql.DB, dest *gorm.DB) (int, error) {
	rows, err := srcDB.QueryContext(ctx,
		"SELECT type, name, webhook_url, apprise_tags, enabled, "+
			"on_cycle_digest, on_error, on_mode_changed, on_server_started, "+
			"on_threshold_breach, on_update_available, on_approval_activity "+
			"FROM notification_configs")
	if err != nil {
		return 0, fmt.Errorf("failed to read notifications: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			slog.Warn("Failed to close notification rows", "component", "migration", "error", closeErr)
		}
	}()

	count := 0
	for rows.Next() {
		var (
			nType              string
			name               string
			webhookURL         string
			appriseTags        string
			enabled            bool
			onCycleDigest      bool
			onError            bool
			onModeChanged      bool
			onServerStarted    bool
			onThresholdBreach  bool
			onUpdateAvailable  bool
			onApprovalActivity bool
		)
		if err := rows.Scan(&nType, &name, &webhookURL, &appriseTags, &enabled,
			&onCycleDigest, &onError, &onModeChanged, &onServerStarted,
			&onThresholdBreach, &onUpdateAvailable, &onApprovalActivity); err != nil {
			slog.Warn("Failed to scan notification row", "component", "migration", "error", err)
			continue
		}

		target := db.NotificationConfig{
			Type:                nType,
			Name:                name,
			WebhookURL:          webhookURL,
			AppriseTags:         appriseTags,
			Enabled:             enabled,
			OnCycleDigest:       onCycleDigest,
			OnError:             onError,
			OnModeChanged:       onModeChanged,
			OnServerStarted:     onServerStarted,
			OnThresholdBreach:   onThresholdBreach,
			OnUpdateAvailable:   onUpdateAvailable,
			OnApprovalActivity:  onApprovalActivity,
			OnIntegrationStatus: true, // New in 2.0 — default to true to match freshly-created channel behavior
		}
		if err := dest.Create(&target).Error; err != nil {
			slog.Warn("Failed to import notification", "component", "migration",
				"name", name, "error", err)
			continue
		}
		count++
	}
	return count, rows.Err()
}

// Detect1xBackup returns true if a 1.x backup file exists in the config
// directory. This is checked after the startup pre-init phase has already
// renamed the 1.x database — the presence of the backup means the user has
// not yet completed the migration workflow (import or dismiss).
func Detect1xBackup(configDir string) bool {
	path := configDir + "/" + backupFilename
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// BackupPath returns the full path to the 1.x backup file in the config directory.
func BackupPath(configDir string) string {
	return configDir + "/" + backupFilename
}

// BackupSourceDatabase renames the source 1.x database to the backup filename.
func BackupSourceDatabase(configDir string) error {
	src := configDir + "/capacitarr.db"
	dst := configDir + "/" + backupFilename
	return os.Rename(src, dst)
}

// RemoveBackup deletes the backup file. Used by "Start Fresh" when the user
// declines to import 1.x settings.
func RemoveBackup(configDir string) error {
	path := configDir + "/" + backupFilename
	return os.Remove(path)
}

// ImportAuthOnly imports only the auth_configs from a 1.x database backup into
// the 2.0 database. This is called during startup after a legacy database is
// detected and renamed, so the user can log in with their existing credentials
// before deciding whether to import the rest of their settings.
func ImportAuthOnly(sourcePath string, destDB *gorm.DB) error {
	slog.Info("Auto-importing auth config from 1.x backup",
		"component", "migration", "source", sourcePath)

	// Verify source exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("source database backup not found: %s", sourcePath)
	}

	// Open source database. Note: gormlite does not support URI parameters like
	// ?mode=ro — using them creates a rogue file with the literal "?mode=ro" in
	// the filename. We open in default mode and only perform read operations.
	sourceDB, err := gorm.Open(gormlite.Open(sourcePath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Warn),
	})
	if err != nil {
		return fmt.Errorf("failed to open source database backup: %w", err)
	}
	sourceSQLDB, _ := sourceDB.DB()
	defer func() {
		if closeErr := sourceSQLDB.Close(); closeErr != nil {
			slog.Warn("Failed to close source database backup", "error", closeErr)
		}
	}()

	return importAuth(sourceDB, destDB)
}

func importAuth(src *gorm.DB, dest *gorm.DB) error {
	var source db.AuthConfig
	if err := src.First(&source).Error; err != nil {
		return fmt.Errorf("no auth config in source: %w", err)
	}

	// Import into 2.0 (clear existing first)
	dest.Where("1 = 1").Delete(&db.AuthConfig{})
	target := db.AuthConfig{
		Username:   source.Username,
		Password:   source.Password,
		APIKey:     source.APIKey,
		APIKeyHint: source.APIKeyHint,
		CreatedAt:  source.CreatedAt,
		UpdatedAt:  time.Now(),
	}
	return dest.Create(&target).Error
}
