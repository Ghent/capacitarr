package services

import (
	"capacitarr/internal/services/approval"
	"capacitarr/internal/services/backup"
	"capacitarr/internal/services/deletion"
	"capacitarr/internal/services/settings"
)

// Compatibility aliases so leftover services, routes, and the orchestrator
// keep compiling against capacitarr/internal/services after the
// implementations moved into subpackages.
//
//nolint:revive // re-exports of existing types; docs live on the definitions
type (
	DeletionService     = deletion.DeletionService
	DeletionDeps        = deletion.DeletionDeps
	SettingsReader      = deletion.SettingsReader
	EngineStatsWriter   = deletion.EngineStatsWriter
	DeletionStatsWriter = deletion.DeletionStatsWriter
	ApprovalReturner    = deletion.ApprovalReturner
	ApprovalSnoozer     = deletion.ApprovalSnoozer
	DiskGroupModeReader = deletion.DiskGroupModeReader
	DiskGroupResolver   = deletion.DiskGroupResolver
	ClientResolver      = deletion.ClientResolver
	SunsetQueueCleaner  = deletion.SunsetQueueCleaner
	DeleteJobSummary    = deletion.DeleteJobSummary
	EngineDeleteRequest = deletion.EngineDeleteRequest

	ApprovalService          = approval.ApprovalService
	ExecuteApprovalDeps      = approval.ExecuteApprovalDeps
	ManualDeleteDeps         = approval.ManualDeleteDeps
	ManualDeleteRequest      = approval.ManualDeleteRequest
	ManualDeleteResult       = approval.ManualDeleteResult
	ApprovalReturnerUpserter = approval.ApprovalReturnerUpserter

	BackupService          = backup.BackupService
	SettingsExportEnvelope = backup.SettingsExportEnvelope
	ExportSections         = backup.ExportSections
	ImportSections         = backup.ImportSections
	ImportResult           = backup.ImportResult
	ImportPreview          = backup.ImportPreview
	RuleOverride           = backup.RuleOverride
	PreferencesExport      = backup.PreferencesExport
	RuleExport             = backup.RuleExport
	IntegrationExport      = backup.IntegrationExport
	DiskGroupExport        = backup.DiskGroupExport
	NotificationExport     = backup.NotificationExport

	SettingsService         = settings.SettingsService
	DeletionQueueClearer    = settings.DeletionQueueClearer
	SunsetLabelMigrator     = settings.SunsetLabelMigrator
	EnginePreferencePatch   = settings.EnginePreferencePatch
	SunsetPreferencePatch   = settings.SunsetPreferencePatch
	ContentPreferencePatch  = settings.ContentPreferencePatch
	AdvancedPreferencePatch = settings.AdvancedPreferencePatch
	FactorWeightResponse    = settings.FactorWeightResponse
)

// Constructor and sentinel aliases preserve existing call sites in this package
// and in routes that import capacitarr/internal/services.
var (
	NewDeletionService   = deletion.NewDeletionService
	ErrDeletionQueueFull = deletion.ErrDeletionQueueFull

	NewApprovalService        = approval.NewApprovalService
	ErrApprovalNotFound       = approval.ErrApprovalNotFound
	ErrApprovalNotPending     = approval.ErrApprovalNotPending
	ErrApprovalNotDismissable = approval.ErrApprovalNotDismissable
	ErrApprovalGroupEmpty     = approval.ErrApprovalGroupEmpty

	NewBackupService      = backup.NewBackupService
	ErrUnsupportedVersion = backup.ErrUnsupportedVersion

	NewSettingsService = settings.NewSettingsService
)
