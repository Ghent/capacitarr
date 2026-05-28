package services

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

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
	WatchAnalytics       *WatchAnalyticsService
	Migration            *MigrationService
	Sunset               *SunsetService
	PosterOverlay        *PosterOverlayService
	Mapping              *MappingService
	Health               *IntegrationHealthService
	DatabaseBackup       *DatabaseBackupService
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

	sunsetSvc := NewSunsetService(database, bus)

	// Wire cross-service dependencies that cannot be injected at construction
	// time due to circular initialization (DeletionService needs Settings,
	// Engine, Metrics, ApprovalReturner, ApprovalSnoozer, DiskGroups,
	// ClientResolver, and SunsetCleaner but they are constructed in the same function).
	integrationSvc := NewIntegrationService(database, bus)
	deletionSvc.SetDependencies(DeletionDeps{
		Settings:      settingsSvc,
		Engine:        engineSvc,
		Metrics:       metricsSvc,
		Approval:      approvalSvc,
		Snoozer:       approvalSvc,
		DiskGroups:    diskGroupSvc,
		Clients:       NewClientResolver(integrationSvc),
		SunsetCleaner: sunsetSvc,
	})

	notifChannelSvc := NewNotificationChannelService(database, bus)
	notifDispatch := NewNotificationDispatchService(bus, notifChannelSvc, nil, "")
	backupSvc := NewBackupService(database, bus)
	dbBackupSvc := NewDatabaseBackupService(database, bus, cfg.Database)
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
		Integration:          integrationSvc,
		Auth:                 NewAuthService(database, bus, cfg),
		NotificationChannel:  notifChannelSvc,
		NotificationDispatch: notifDispatch,
		Data:                 NewDataService(database, bus),
		Rules:                NewRulesService(database, bus),
		Metrics:              metricsSvc,
		WatchAnalytics:       NewWatchAnalyticsService(previewSvc),
		Migration:            NewMigrationService(database, bus, filepath.Dir(cfg.Database)),
		Sunset:               sunsetSvc,
		Mapping:              NewMappingService(database, bus),
		DatabaseBackup:       dbBackupSvc,
	}

	// Initialize PosterOverlayService — cache dir is alongside the database file
	posterCacheDir := filepath.Join(filepath.Dir(cfg.Database), "posters", "originals")
	if posterSvc, err := NewPosterOverlayService(database, bus, posterCacheDir); err != nil {
		slog.Error("Failed to initialize poster overlay service — poster overlays disabled",
			"component", "registry", "error", err)
	} else {
		reg.PosterOverlay = posterSvc
		// Startup validation: warn about items with active poster overlays
		posterSvc.ValidateCache()
	}

	// Wire SettingsService's cross-service dependency on DeletionService
	// so execution mode changes clear the deletion queue (prevents stale
	// jobs from executing under the wrong mode).
	settingsSvc.SetDeletionClearer(deletionSvc)

	// Wire SettingsService's cross-service dependency on SunsetService
	// so changing the sunset label preference migrates labels on media servers.
	settingsSvc.SetLabelMigrator(NewSunsetLabelMigrator(sunsetSvc, reg.Integration, reg.Mapping))

	// Wire IntegrationService's cross-service dependency on DiskGroupService
	reg.Integration.SetDiskGroupService(diskGroupSvc)

	// Create IntegrationHealthService — replaces RecoveryService.
	// Health service depends on IntegrationService for DB operations.
	// IntegrationService depends on health service (via HealthReporter interface)
	// only for SyncAll connection result reporting.
	reg.Health = NewIntegrationHealthService(reg.Integration, bus)
	reg.Integration.SetHealthReporter(reg.Health)

	// Wire DiskGroupService's cross-service dependencies
	diskGroupSvc.SetEngineService(engineSvc)
	diskGroupSvc.SetSettingsReader(settingsSvc)
	diskGroupSvc.SetDeletionClearer(deletionSvc)

	// Wire BackupService's cross-service dependency on DiskGroupService
	backupSvc.SetDiskGroupService(diskGroupSvc)

	// Wire DatabaseBackupService's cross-service dependency on SettingsService
	dbBackupSvc.SetSettingsService(settingsSvc)

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

	// Validate all lazy wiring before returning — panics with a descriptive
	// message if any service has unwired dependencies.
	reg.Validate()

	return reg
}

// wireable is implemented by services that have lazily-injected dependencies
// which must be verified at startup.
type wireable interface {
	Wired() bool
}

// Validate checks that all services with lazy dependencies have been fully
// wired. Panics with a descriptive message listing all unwired services.
// Called at the end of NewRegistry() to catch wiring mistakes at startup
// rather than at runtime (e.g., during a 3 AM deletion cycle).
//
// Note: VersionService is intentionally excluded because it is created via
// InitVersion() after the registry is constructed (it depends on the app
// version string). InitVersion() must be called before starting any
// background workers that use Version — enforced by the startup sequence
// in main.go.
func (r *Registry) Validate() {
	checks := []struct {
		name    string
		service wireable
	}{
		{"Deletion", r.Deletion},
		{"Settings", r.Settings},
		{"Integration", r.Integration},
		{"DiskGroup", r.DiskGroup},
		{"Backup", r.Backup},
		{"Metrics", r.Metrics},
		{"Preview", r.Preview},
		{"Rules", r.Rules},
		{"WatchAnalytics", r.WatchAnalytics},
		{"Data", r.Data},
		{"Migration", r.Migration},
		{"DatabaseBackup", r.DatabaseBackup},
	}

	var unwired []string
	for _, c := range checks {
		if !c.service.Wired() {
			unwired = append(unwired, c.name)
		}
	}

	if len(unwired) > 0 {
		panic(fmt.Sprintf("service registry validation failed: unwired services: %s", strings.Join(unwired, ", ")))
	}
}

// InitVersion creates and registers the VersionService. Called by main.go
// after Registry construction, when the application version string is known.
// It also wires the dispatch service's version checker and version string.
func (r *Registry) InitVersion(appVersion string) {
	r.Version = NewVersionService(r.Settings, r.Bus, appVersion, DefaultGitHubReleasesURL)
	r.NotificationDispatch.SetVersionChecker(r.Version)
	r.NotificationDispatch.SetVersion(appVersion)
}
