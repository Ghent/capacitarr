package services

import (
	"fmt"
	"log/slog"

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
	engineSvc *EngineService
}

// SetEngineService wires the cross-service dependency on EngineService.
// Called after construction in NewRegistry to break the initialization cycle.
// The engine is triggered after a successful migration so the dashboard
// populates immediately instead of waiting for the next scheduled poll.
func (s *MigrationService) SetEngineService(engine *EngineService) {
	s.engineSvc = engine
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

// Status checks whether a 1.x database backup exists in the config directory.
// The backup is created during startup when a legacy schema is detected — its
// presence means the user has not yet completed the migration workflow (import
// settings or dismiss).
func (s *MigrationService) Status() MigrationStatus {
	available := migration.Detect1xBackup(s.configDir)
	status := MigrationStatus{Available: available}
	if available {
		status.SourceDB = migration.BackupPath(s.configDir)
	}
	return status
}

// MigrationResult wraps the migration outcome for the web layer.
type MigrationResult struct {
	Success               bool   `json:"success"`
	IntegrationsImported  int    `json:"integrationsImported"`
	DiskGroupsImported    int    `json:"diskGroupsImported"`
	RulesImported         int    `json:"rulesImported"`
	PreferencesImported   bool   `json:"preferencesImported"`
	NotificationsImported int    `json:"notificationsImported"`
	EngineRunTriggered    bool   `json:"engineRunTriggered"`
	Error                 string `json:"error,omitempty"`
}

// Execute runs the 1.x → 2.0 migration from the backup file.
// Auth has already been auto-imported at startup, so this imports the
// remaining configuration: integrations, disk groups, rules, preferences,
// and notifications. After successful import, the backup file is removed
// so the migration page does not reappear.
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
		slog.Warn("Migration succeeded but failed to remove backup",
			"component", "migration", "error", removeErr)
	}

	// Publish a settings-imported event for the activity log
	if s.bus != nil {
		s.bus.Publish(events.SettingsImportedEvent{
			Sections: []string{"migration_1x_to_2x"},
			Result: map[string]any{
				"integrations":  result.IntegrationsImported,
				"diskGroups":    result.DiskGroupsImported,
				"rules":         result.RulesImported,
				"notifications": result.NotificationsImported,
				"preferences":   result.PreferencesImported,
			},
		})
	}

	// Trigger an engine run so the dashboard populates immediately with
	// library data from the freshly imported integrations. Without this,
	// the user would see an empty dashboard until the next scheduled poll.
	engineTriggered := false
	if s.engineSvc != nil {
		status := s.engineSvc.TriggerRun()
		engineTriggered = status == EngineStatusStarted
		slog.Info("Triggered post-migration engine run",
			"component", "migration", "status", status)
	}

	return MigrationResult{
		Success:               true,
		IntegrationsImported:  result.IntegrationsImported,
		DiskGroupsImported:    result.DiskGroupsImported,
		RulesImported:         result.RulesImported,
		PreferencesImported:   result.PreferencesImported,
		NotificationsImported: result.NotificationsImported,
		EngineRunTriggered:    engineTriggered,
	}
}

// Dismiss removes the 1.x backup file without importing any settings.
// Used by the "Start Fresh" flow when the user declines to import their
// 1.x configuration into the 2.0 database.
func (s *MigrationService) Dismiss() error {
	return migration.RemoveBackup(s.configDir)
}
