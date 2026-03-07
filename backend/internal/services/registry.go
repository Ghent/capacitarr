package services

import (
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
	Deletion             *DeletionService
	AuditLog             *AuditLogService
	Engine               *EngineService
	Settings             *SettingsService
	Integration          *IntegrationService
	Auth                 *AuthService
	NotificationChannel  *NotificationChannelService
	NotificationDispatch *NotificationDispatchService
	Data                 *DataService
	Rules                *RulesService
	Metrics              *MetricsService
	Version              *VersionService
}

// NewRegistry creates a fully wired Registry with all services.
func NewRegistry(database *gorm.DB, bus *events.EventBus, cfg *config.Config) *Registry {
	auditLog := NewAuditLogService(database)
	engineSvc := NewEngineService(database, bus)
	deletionSvc := NewDeletionService(bus, auditLog)
	settingsSvc := NewSettingsService(database, bus)
	metricsSvc := NewMetricsService(database, engineSvc, deletionSvc)

	// Wire cross-service dependencies that cannot be injected at construction
	// time due to circular initialization (DeletionService needs Settings,
	// Engine, and Metrics but they are constructed in the same function).
	deletionSvc.SetDependencies(settingsSvc, engineSvc, metricsSvc)

	notifChannelSvc := NewNotificationChannelService(database, bus)
	notifDispatch := NewNotificationDispatchService(bus, notifChannelSvc, nil, "")

	reg := &Registry{
		DB:                   database,
		Bus:                  bus,
		Cfg:                  cfg,
		Approval:             NewApprovalService(database, bus),
		Deletion:             deletionSvc,
		AuditLog:             auditLog,
		Engine:               engineSvc,
		Settings:             settingsSvc,
		Integration:          NewIntegrationService(database, bus),
		Auth:                 NewAuthService(database, bus, cfg),
		NotificationChannel:  notifChannelSvc,
		NotificationDispatch: notifDispatch,
		Data:                 NewDataService(database, bus),
		Rules:                NewRulesService(database, bus),
		Metrics:              metricsSvc,
	}

	// Wire IntegrationService's cross-service dependency on SettingsService
	reg.Integration.SetSettingsService(settingsSvc)

	// Wire MetricsService's cross-service dependency on SettingsService
	metricsSvc.SetSettingsService(settingsSvc)

	return reg
}

// InitVersion creates and registers the VersionService. Called by main.go
// after Registry construction, when the application version string is known.
// It also wires the dispatch service's version checker and version string.
func (r *Registry) InitVersion(appVersion string) {
	r.Version = NewVersionService(r.Settings, r.Bus, appVersion, DefaultGitLabReleasesURL)
	r.NotificationDispatch.SetVersionChecker(r.Version)
	r.NotificationDispatch.SetVersion(appVersion)
}
