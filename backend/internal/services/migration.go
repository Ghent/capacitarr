package services

import (
	"fmt"
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

// MigrationStatus holds the detection result for a 1.x database.
type MigrationStatus struct {
	Available bool   `json:"available"`
	SourceDB  string `json:"sourceDb,omitempty"`
}

// Status checks whether a 1.x database file exists in the config directory.
func (s *MigrationService) Status() MigrationStatus {
	available := migration.Detect1xDatabase(s.configDir)
	status := MigrationStatus{Available: available}
	if available {
		status.SourceDB = filepath.Join(s.configDir, "capacitarr.db")
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

// Execute runs the 1.x → 2.0 migration and backs up the source database.
func (s *MigrationService) Execute() MigrationResult {
	sourcePath := filepath.Join(s.configDir, "capacitarr.db")

	result, err := migration.MigrateFrom(sourcePath, s.db)
	if err != nil {
		return MigrationResult{
			Success: false,
			Error:   fmt.Sprintf("Migration failed: %v", err),
		}
	}

	// Rename the source database to .v1.bak so the migration page doesn't re-appear
	if backupErr := migration.BackupSourceDatabase(s.configDir); backupErr != nil {
		return MigrationResult{
			Success:               true,
			IntegrationsImported:  result.IntegrationsImported,
			RulesImported:         result.RulesImported,
			PreferencesImported:   result.PreferencesImported,
			NotificationsImported: result.NotificationsImported,
			AuthImported:          result.AuthImported,
			Error:                 fmt.Sprintf("Migration succeeded but failed to backup source: %v", backupErr),
		}
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
