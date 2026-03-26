package services

import (
	"path/filepath"

	"gorm.io/gorm"

	"capacitarr/internal/config"
	"capacitarr/internal/events"
)

// Registry holds all service instances and shared dependencies. Created once
// in main.go and passed to route registration functions and the poller.
//
// DB and Bus are exposed on the struct for use by service constructors and
// internal wiring. Route handlers, middleware, orchestrators, and event
// subscribers must NOT access DB or Bus directly — all data access must go
// through the appropriate service method. See .kilocoderules for the full
// service layer architecture policy.
type Registry struct {
	DB  *gorm.DB
	Bus *events.EventBus
	Cfg *config.Config

	Approval             *ApprovalService
	Backup               *BackupService
	Deletion             *DeletionService
	AuditLog             *AuditLogService
	DiskGroup            *DiskGroupService
	Engine               *EngineService
	Preview              *PreviewService
	Settings             *SettingsService
	Integration          *IntegrationService
	Auth                 *AuthService
	NotificationChannel  *NotificationChannelService
	NotificationDispatch *NotificationDispatchService
	Data                 *DataService
	Rules                *RulesService
	Metrics              *MetricsService
	Version              *VersionService
	Library              *LibraryService
	WatchAnalytics       *WatchAnalyticsService
	Migration            *MigrationService
}

// NewRegistry creates a fully wired Registry with all services.
func NewRegistry(database *gorm.DB, bus *events.EventBus, cfg *config.Config) *Registry {
	auditLog := NewAuditLogService(database)
	engineSvc := NewEngineService(database, bus)
	deletionSvc := NewDeletionService(bus, auditLog)
	settingsSvc := NewSettingsService(database, bus)
	diskGroupSvc := NewDiskGroupService(database, bus)
	metricsSvc := NewMetricsService(database, engineSvc, deletionSvc)
	approvalSvc := NewApprovalService(database, bus)

	// Wire cross-service dependencies that cannot be injected at construction
	// time due to circular initialization (DeletionService needs Settings,
	// Engine, Metrics, and ApprovalReturner but they are constructed in the
	// same function).
	deletionSvc.SetDependencies(settingsSvc, engineSvc, metricsSvc, approvalSvc)

	notifChannelSvc := NewNotificationChannelService(database, bus)
	notifDispatch := NewNotificationDispatchService(bus, notifChannelSvc, nil, "")
	backupSvc := NewBackupService(database, bus)
	previewSvc := NewPreviewService(database, bus)

	reg := &Registry{
		DB:                   database,
		Bus:                  bus,
		Cfg:                  cfg,
		Approval:             approvalSvc,
		Backup:               backupSvc,
		Deletion:             deletionSvc,
		AuditLog:             auditLog,
		DiskGroup:            diskGroupSvc,
		Engine:               engineSvc,
		Preview:              previewSvc,
		Settings:             settingsSvc,
		Integration:          NewIntegrationService(database, bus),
		Auth:                 NewAuthService(database, bus, cfg),
		NotificationChannel:  notifChannelSvc,
		NotificationDispatch: notifDispatch,
		Data:                 NewDataService(database, bus),
		Rules:                NewRulesService(database, bus),
		Metrics:              metricsSvc,
		Library:              NewLibraryService(database, bus),
		WatchAnalytics:       NewWatchAnalyticsService(previewSvc),
		Migration:            NewMigrationService(database, bus, filepath.Dir(cfg.Database)),
	}

	// Wire SettingsService's cross-service dependency on DeletionService
	// so execution mode changes clear the deletion queue (prevents stale
	// jobs from executing under the wrong mode).
	settingsSvc.SetDeletionClearer(deletionSvc)

	// Wire IntegrationService's cross-service dependency on DiskGroupService
	reg.Integration.SetDiskGroupService(diskGroupSvc)

	// Wire DiskGroupService's cross-service dependency on EngineService
	// so threshold changes trigger an immediate engine run for queue reconciliation
	diskGroupSvc.SetEngineService(engineSvc)

	// Wire BackupService's cross-service dependency on DiskGroupService
	backupSvc.SetDiskGroupService(diskGroupSvc)

	// Wire MetricsService's cross-service dependency on SettingsService
	metricsSvc.SetSettingsService(settingsSvc)

	// Wire PreviewService's cross-service dependencies for preview computation
	previewSvc.SetDependencies(reg.Integration, settingsSvc, reg.Rules, diskGroupSvc)

	// Wire PreviewService's queue status enrichment dependencies
	previewSvc.SetQueueDependencies(reg.Approval, deletionSvc)

	// Wire RulesService's preview source for rule impact calculation
	reg.Rules.SetPreviewSource(previewSvc)

	// Wire RulesService's integration provider for rule context endpoint
	reg.Rules.SetIntegrationProvider(reg.Integration)

	// Wire analytics services' rules sources for protected-item filtering
	reg.WatchAnalytics.SetRulesSource(reg.Rules)

	// Wire analytics services' disk group lister for per-disk-group filtering

	reg.WatchAnalytics.SetDiskGroupLister(diskGroupSvc)

	// Wire DataService's preview dependency for cache clearing on data reset
	reg.Data.SetPreviewService(previewSvc)

	// Wire MigrationService's engine dependency so a successful 1.x → 2.0
	// migration triggers an immediate engine run to populate the dashboard
	reg.Migration.SetEngineService(engineSvc)

	return reg
}

// InitVersion creates and registers the VersionService. Called by main.go
// after Registry construction, when the application version string is known.
// It also wires the dispatch service's version checker and version string.
func (r *Registry) InitVersion(appVersion string) {
	r.Version = NewVersionService(r.Settings, r.Bus, appVersion, DefaultGitHubReleasesURL)
	r.NotificationDispatch.SetVersionChecker(r.Version)
	r.NotificationDispatch.SetVersion(appVersion)
}
