// Package migration provides one-way data import from Capacitarr 1.x to 2.0.
// The migration reads configuration data from a 1.x SQLite database and imports
// it into the 2.0 schema. Transient data (approval queue, audit log, engine stats,
// activity events) is NOT migrated since it has no value in the new system.
package migration

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "github.com/ncruces/go-sqlite3/embed" // embed: SQLite WASM binary required for gormlite
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"capacitarr/internal/db"
)

// Result holds the outcome of a migration.
type Result struct {
	IntegrationsImported  int
	RulesImported         int
	PreferencesImported   bool
	NotificationsImported int
	AuthImported          bool
}

// MigrateFrom imports configuration data from a 1.x database into the current 2.0 database.
// The source database is opened read-only. Only configuration data is imported:
// - IntegrationConfig (with overseerr → seerr type transformation)
// - PreferenceSet (with default values for new 2.0 fields)
// - CustomRule
// - NotificationConfig
// - AuthConfig
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

	result := &Result{}

	// Import auth
	if err := importAuth(sourceDB, destDB); err != nil {
		slog.Warn("Failed to import auth config", "component", "migration", "error", err)
	} else {
		result.AuthImported = true
	}

	// Import integrations
	count, err := importIntegrations(sourceDB, destDB)
	if err != nil {
		slog.Warn("Failed to import integrations", "component", "migration", "error", err)
	}
	result.IntegrationsImported = count

	// Import preferences
	if err := importPreferences(sourceDB, destDB); err != nil {
		slog.Warn("Failed to import preferences", "component", "migration", "error", err)
	} else {
		result.PreferencesImported = true
	}

	// Import rules
	rulesCount, err := importRules(sourceDB, destDB)
	if err != nil {
		slog.Warn("Failed to import rules", "component", "migration", "error", err)
	}
	result.RulesImported = rulesCount

	// Import notifications
	notifCount, err := importNotifications(sourceDB, destDB)
	if err != nil {
		slog.Warn("Failed to import notifications", "component", "migration", "error", err)
	}
	result.NotificationsImported = notifCount

	slog.Info("Migration complete", "component", "migration",
		"integrations", result.IntegrationsImported,
		"rules", result.RulesImported,
		"notifications", result.NotificationsImported,
		"auth", result.AuthImported,
		"preferences", result.PreferencesImported)

	return result, nil
}

func importAuth(src, dest *gorm.DB) error {
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

func importIntegrations(src, dest *gorm.DB) (int, error) {
	var sources []db.IntegrationConfig
	if err := src.Find(&sources).Error; err != nil {
		return 0, fmt.Errorf("failed to read integrations: %w", err)
	}

	count := 0
	for _, s := range sources {
		target := db.IntegrationConfig{
			Type:    transformIntegrationType(s.Type),
			Name:    s.Name,
			URL:     s.URL,
			APIKey:  s.APIKey,
			Enabled: s.Enabled,
		}
		if err := dest.Create(&target).Error; err != nil {
			slog.Warn("Failed to import integration", "component", "migration",
				"name", s.Name, "type", s.Type, "error", err)
			continue
		}
		count++
	}
	return count, nil
}

// transformIntegrationType handles the overseerr → seerr rename.
func transformIntegrationType(t string) string {
	if t == "overseerr" {
		return "seerr"
	}
	return t
}

func importPreferences(src, dest *gorm.DB) error {
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

	row := src.Raw("SELECT log_level, audit_log_retention_days, poll_interval_seconds, " +
		"watch_history_weight, last_watched_weight, file_size_weight, rating_weight, " +
		"time_in_library_weight, series_status_weight, execution_mode, tiebreaker_method, " +
		"deletions_enabled, snooze_duration_hours, check_for_updates FROM preference_sets LIMIT 1").Row()
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

	// Update the 2.0 preferences (row already exists from db.Init seed).
	// Note: scoring factor weights are stored in the separate scoring_factor_weights
	// table in 2.0, so they are NOT included here — they are persisted below.
	if err := dest.Model(&db.PreferenceSet{}).Where("id = 1").Updates(map[string]interface{}{
		"log_level":                source.LogLevel,
		"audit_log_retention_days": source.AuditLogRetentionDays,
		"poll_interval_seconds":    source.PollIntervalSeconds,
		"execution_mode":           source.ExecutionMode,
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

func importRules(src, dest *gorm.DB) (int, error) {
	var sources []db.CustomRule
	if err := src.Find(&sources).Error; err != nil {
		return 0, fmt.Errorf("failed to read rules: %w", err)
	}

	count := 0
	for _, s := range sources {
		target := db.CustomRule{
			Field:     s.Field,
			Operator:  s.Operator,
			Value:     s.Value,
			Effect:    s.Effect,
			Enabled:   s.Enabled,
			SortOrder: s.SortOrder,
			// IntegrationID and LibraryID left nil — user re-assigns after migration
		}
		if err := dest.Create(&target).Error; err != nil {
			slog.Warn("Failed to import rule", "component", "migration",
				"field", s.Field, "error", err)
			continue
		}
		count++
	}
	return count, nil
}

func importNotifications(src, dest *gorm.DB) (int, error) {
	var sources []db.NotificationConfig
	if err := src.Find(&sources).Error; err != nil {
		return 0, fmt.Errorf("failed to read notifications: %w", err)
	}

	count := 0
	for _, s := range sources {
		target := db.NotificationConfig{
			Type:               s.Type,
			Name:               s.Name,
			WebhookURL:         s.WebhookURL,
			AppriseTags:        s.AppriseTags,
			Enabled:            s.Enabled,
			OnCycleDigest:      s.OnCycleDigest,
			OnError:            s.OnError,
			OnModeChanged:      s.OnModeChanged,
			OnServerStarted:    s.OnServerStarted,
			OnThresholdBreach:  s.OnThresholdBreach,
			OnUpdateAvailable:  s.OnUpdateAvailable,
			OnApprovalActivity: s.OnApprovalActivity,
		}
		if err := dest.Create(&target).Error; err != nil {
			slog.Warn("Failed to import notification", "component", "migration",
				"name", s.Name, "error", err)
			continue
		}
		count++
	}
	return count, nil
}

// Detect1xBackup returns true if a 1.x backup file (.v1.bak) exists in the
// config directory. This is checked after the startup pre-init phase has
// already renamed the 1.x database — the presence of .v1.bak means the
// user has not yet completed the migration workflow (import or dismiss).
func Detect1xBackup(configDir string) bool {
	path := configDir + "/capacitarr.db.v1.bak"
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// BackupPath returns the full path to the 1.x backup file in the config directory.
func BackupPath(configDir string) string {
	return configDir + "/capacitarr.db.v1.bak"
}

// BackupSourceDatabase renames the source 1.x database to .v1.bak.
func BackupSourceDatabase(configDir string) error {
	src := configDir + "/capacitarr.db"
	dst := configDir + "/capacitarr.db.v1.bak"
	return os.Rename(src, dst)
}

// RemoveBackup deletes the .v1.bak file. Used by "Start Fresh" when the user
// declines to import 1.x settings.
func RemoveBackup(configDir string) error {
	path := configDir + "/capacitarr.db.v1.bak"
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
