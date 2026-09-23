/**
 * Frontend API types. Resource shapes are generated from
 * docs/reference/api/openapi.yaml — run `make api:generate`.
 * Do not add hand-written structs here for generated schemas.
 */
import type { components } from './generated/openapi';

export type { ApiError, SelectedDetailItem } from './api-extra';

type Schemas = components['schemas'];

export type IntegrationConfig = Schemas['IntegrationConfig'];
export type DiskGroupIntegration = Schemas['DiskGroupIntegration'];
export type DiskGroup = Schemas['DiskGroup'];
export type PreferenceSet = Schemas['PreferenceSet'];
export type ScoringFactorWeight = Schemas['ScoringFactorWeight'];
export type CustomRule = Schemas['CustomRule'];
export type AuditLogEntry = Schemas['AuditLogEntry'];
export type AuditAction = NonNullable<AuditLogEntry['action']>;
export type AuditResponse = Schemas['AuditLogResponse'];
export type ApprovalQueueItem = Schemas['ApprovalQueueItem'];
export type ActivityEvent = Schemas['ActivityEvent'];
export type WorkerStats = Schemas['WorkerStats'];
export type DeletionProgress = Schemas['DeletionProgress'];
export type MediaItem = Schemas['MediaItem'];
export type ScoreFactor = Schemas['ScoreFactor'];
export type EvaluatedItem = Schemas['EvaluatedItem'];
export type PreviewResponse = Schemas['PreviewResponse'];
export type DiskContext = Schemas['DiskContext'];
export type ConnectionTestResult = Schemas['ConnectionTestResult'];
export type ApiKeyResponse = Schemas['ApiKeyResponse'];
export type PreferencesExport = Schemas['PreferencesExport'];
export type SunsetQueueItem = Schemas['SunsetQueueItem'];
export type RuleExport = Schemas['PortableRule'];
export type IntegrationExport = Schemas['IntegrationExport'];
export type DiskGroupExport = Schemas['DiskGroupExport'];
export type NotificationExport = Schemas['NotificationExport'];
export type SettingsExportEnvelope = Schemas['SettingsExportEnvelope'];
export type ExportSections = Schemas['ExportSections'];
export type ImportSections = Schemas['ImportSections'];
export type ImportResult = Schemas['SettingsImportResponse'];
export type IntCandidate = Schemas['IntCandidate'];
export type RuleResolution = Schemas['RuleResolution'];
export type ItemResolution = Schemas['ItemResolution'];
export type FieldChange = Schemas['FieldChange'];
export type PreferencesResolution = Schemas['PreferencesResolution'];
export type DeletionPreview = Schemas['DeletionPreview'];
export type ImportPreview = Schemas['ImportPreview'];
export type RuleOverride = Schemas['RuleOverride'];
export type NotificationChannel = Schemas['NotificationChannel'];
export type DeletionQueueItem = Schemas['DeletionQueueItem'];
export type DeletionCompletedItem = Schemas['DeletionCompletedItem'];
