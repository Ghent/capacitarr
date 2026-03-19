package services

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"gorm.io/gorm"

	"capacitarr/internal/events"
	"capacitarr/internal/migration"
)

// MigrationService provides the web layer with access to the 1.x → 2.0
// migration logic. It wraps the migration package functions and enforces
// the service layer architecture (route handlers never access migration
// functions or the filesystem directly).
type MigrationService struct {
	db        *gorm.DB
	bus       *events.EventBus
	configDir string
}

// NewMigrationService creates a MigrationService.
// configDir is the directory containing the database files (derived from the DB_PATH config).
func NewMigrationService(db *gorm.DB, bus *events.EventBus, configDir string) *MigrationService {
	return &MigrationService{db: db, bus: bus, configDir: configDir}
}

// MigrationStatus holds the detection result for a 1.x database backup.
type MigrationStatus struct {
	Available bool   `json:"available"`
	SourceDB  string `json:"sourceDb,omitempty"`
}

// Status checks whether a 1.x database backup (.v1.bak) exists in the config
// directory. The backup is created during startup when a legacy schema is
// detected — its presence means the user has not yet completed the migration
// workflow (import settings or dismiss).
func (s *MigrationService) Status() MigrationStatus {
	available := migration.Detect1xBackup(s.configDir)
	status := MigrationStatus{Available: available}
	if available {
		status.SourceDB = filepath.Join(s.configDir, "capacitarr.db.v1.bak")
	}
	return status
}

// MigrationResult wraps the migration outcome for the web layer.
type MigrationResult struct {
	Success               bool   `json:"success"`
	IntegrationsImported  int    `json:"integrationsImported"`
	RulesImported         int    `json:"rulesImported"`
	PreferencesImported   bool   `json:"preferencesImported"`
	NotificationsImported int    `json:"notificationsImported"`
	AuthImported          bool   `json:"authImported"`
	Error                 string `json:"error,omitempty"`
}

// Execute runs the 1.x → 2.0 migration from the .v1.bak backup file.
// Auth has already been auto-imported at startup, so this imports the
// remaining configuration: integrations, rules, preferences, and notifications.
// After successful import, the .v1.bak file is removed so the migration
// page does not reappear.
func (s *MigrationService) Execute() MigrationResult {
	sourcePath := migration.BackupPath(s.configDir)

	result, err := migration.MigrateFrom(sourcePath, s.db)
	if err != nil {
		return MigrationResult{
			Success: false,
			Error:   fmt.Sprintf("Migration failed: %v", err),
		}
	}

	// Remove the backup file so the migration page doesn't re-appear
	if removeErr := migration.RemoveBackup(s.configDir); removeErr != nil {
		slog.Warn("Migration succeeded but failed to remove .v1.bak",
			"component", "migration", "error", removeErr)
	}

	// Publish a settings-imported event for the activity log
	if s.bus != nil {
		s.bus.Publish(events.SettingsImportedEvent{
			Sections: []string{"migration_1x_to_2x"},
			Result: map[string]any{
				"integrations":  result.IntegrationsImported,
				"rules":         result.RulesImported,
				"notifications": result.NotificationsImported,
				"preferences":   result.PreferencesImported,
				"auth":          result.AuthImported,
			},
		})
	}

	return MigrationResult{
		Success:               true,
		IntegrationsImported:  result.IntegrationsImported,
		RulesImported:         result.RulesImported,
		PreferencesImported:   result.PreferencesImported,
		NotificationsImported: result.NotificationsImported,
		AuthImported:          result.AuthImported,
	}
}

// Dismiss removes the 1.x backup file without importing any settings.
// Used by the "Start Fresh" flow when the user declines to import their
// 1.x configuration into the 2.0 database.
func (s *MigrationService) Dismiss() error {
	return migration.RemoveBackup(s.configDir)
}
