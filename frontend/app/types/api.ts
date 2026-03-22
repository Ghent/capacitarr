/**
 * Shared TypeScript interfaces for Capacitarr API responses.
 * These mirror the backend Go structs and API response shapes.
 */

// ---------------------------------------------------------------------------
// Integration
// ---------------------------------------------------------------------------

export interface IntegrationConfig {
  id: number;
  type: string;
  name: string;
  url: string;
  apiKey: string;
  enabled: boolean;
  mediaSizeBytes: number;
  mediaCount: number;
  lastSync?: string | null;
  lastError?: string;
  createdAt: string;
  updatedAt: string;
}

// ---------------------------------------------------------------------------
// Disk Group
// ---------------------------------------------------------------------------

export interface DiskGroupIntegration {
  id: number;
  name: string;
  type: string;
}

export interface DiskGroup {
  id: number;
  mountPath: string;
  totalBytes: number;
  usedBytes: number;
  totalBytesOverride?: number | null;
  thresholdPct: number;
  targetPct: number;
  integrations?: DiskGroupIntegration[];
  createdAt: string;
  updatedAt: string;
}

// ---------------------------------------------------------------------------
// Preferences
// ---------------------------------------------------------------------------

export interface PreferenceSet {
  id: number;
  logLevel: string;
  auditLogRetentionDays: number;
  pollIntervalSeconds: number;
  executionMode: string;
  tiebreakerMethod: string;
  deletionsEnabled: boolean;
  snoozeDurationHours: number;
  deletionQueueDelaySeconds: number;
  checkForUpdates: boolean;
  updatedAt: string;
}

// ScoringFactorWeight represents a single scoring factor with its current weight
// and metadata from the engine's factor registry. Returned by GET /api/v1/scoring-factor-weights.
export interface ScoringFactorWeight {
  key: string;
  name: string;
  description: string;
  weight: number;
  defaultWeight: number;
}

// ---------------------------------------------------------------------------
// Custom Rule (API endpoint: /api/v1/custom-rules)
// ---------------------------------------------------------------------------

export interface CustomRule {
  id: number;
  integrationId: number;
  field: string;
  operator: string;
  value: string;
  effect: string;
  enabled: boolean;
  sortOrder: number;
  /** @deprecated Legacy field preserved for backward-compatible migration display. */
  type?: string;
  /** @deprecated Legacy field preserved for backward-compatible migration display. */
  intensity?: string;
  createdAt: string;
  updatedAt: string;
}

// ---------------------------------------------------------------------------
// Audit Log
// ---------------------------------------------------------------------------

/** Action values match backend db.Action* constants (deleted, dry_delete, cancelled). */
export type AuditAction = 'deleted' | 'dry_delete' | 'cancelled';

export interface AuditLogEntry {
  id: number;
  mediaName: string;
  mediaType: string;
  scoreDetails: string;
  action: AuditAction;
  sizeBytes: number;
  score: number;
  trigger: string;
  dryRunReason: string;
  integrationId?: number;
  createdAt: string;
}

export interface AuditResponse {
  data: AuditLogEntry[];
  total: number;
  limit: number;
  offset: number;
}

export interface ApprovalQueueItem {
  id: number;
  mediaName: string;
  mediaType: string;
  scoreDetails: string;
  sizeBytes: number;
  score: number;
  posterUrl?: string;
  integrationId: number;
  externalId: string;
  status: 'pending' | 'approved' | 'rejected';
  trigger: string;
  userInitiated?: boolean;
  snoozedUntil?: string;
  createdAt: string;
  updatedAt: string;
}

// ---------------------------------------------------------------------------
// Activity Event (system events from /api/v1/activity/recent)
// ---------------------------------------------------------------------------

export interface ActivityEvent {
  id: number;
  eventType: string;
  message: string;
  metadata: string;
  createdAt: string;
}

// ---------------------------------------------------------------------------
// Engine / Worker Stats
// ---------------------------------------------------------------------------

export interface WorkerStats {
  executionMode: string;
  isRunning: boolean;
  pollIntervalSeconds: number;
  queueDepth: number;
  lastRunEvaluated: number;
  lastRunFlagged: number;
  lastRunFreedBytes: number;
  lastRunEpoch: number;
  currentlyDeleting: string;
  protectedCount: number;
  processed: number;
  failed: number;
}

// ---------------------------------------------------------------------------
// Deletion Progress (SSE: deletion_progress event)
// ---------------------------------------------------------------------------

export interface DeletionProgress {
  currentItem: string;
  queueDepth: number;
  processed: number;
  succeeded: number;
  failed: number;
  batchTotal: number;
}

// ---------------------------------------------------------------------------
// Dashboard Stats
// ---------------------------------------------------------------------------

export interface DashboardStats {
  totalBytesReclaimed: number;
  totalItemsRemoved: number;
  totalEngineRuns: number;
  protectedCount: number;
  growthBytesPerWeek: number;
  hasGrowthData: boolean;
}

// ---------------------------------------------------------------------------
// Media / Scoring (Preview)
// ---------------------------------------------------------------------------

export interface MediaItem {
  externalId: string;
  integrationId: number;
  type: string;
  title: string;
  year?: number;
  sizeBytes: number;
  path: string;
  posterUrl?: string;
  seasonNumber?: number;
  episodeCount?: number;
  showTitle?: string;
  seriesStatus?: string;
  qualityProfile?: string;
  rating?: number;
  genre?: string;
  monitored: boolean;
  playCount?: number;
  lastPlayed?: string | null;
  addedAt?: string | null;
  tags?: string[];
  isRequested?: boolean;
  requestedBy?: string;
  requestCount?: number;
  tmdbId?: number;
  language?: string;
}

export interface ScoreFactor {
  name: string;
  rawScore: number;
  weight: number;
  contribution: number;
  type: string;
  matchedValue?: string;
  ruleId?: number;
}

export interface EvaluatedItem {
  item: MediaItem;
  score: number;
  isProtected: boolean;
  reason: string;
  factors: ScoreFactor[];
  /** Queue state indicator: pending approval, approved, user-initiated, or actively deleting. */
  queueStatus?: 'pending' | 'approved' | 'user_initiated' | 'deleting';
  /** Links to the approval queue entry for action buttons. */
  approvalQueueId?: number;
  /** Present on legacy responses that embed score details as a JSON string. */
  scoreDetails?: string;
}

export interface PreviewResponse {
  items: EvaluatedItem[];
  diskContext: DiskContext | null;
}

export interface DiskContext {
  totalBytes: number;
  usedBytes: number;
  targetPct: number;
  thresholdPct: number;
  bytesToFree: number;
}

// ---------------------------------------------------------------------------
// Metrics History
// ---------------------------------------------------------------------------

export interface LibraryHistoryRow {
  id: number;
  timestamp: string;
  totalCapacity: number;
  usedCapacity: number;
  resolution: string;
  diskGroupId?: number | null;
  createdAt: string;
}

export interface MetricsHistoryResponse {
  status: string;
  data: LibraryHistoryRow[];
}

// ---------------------------------------------------------------------------
// Connection Test
// ---------------------------------------------------------------------------

export interface ConnectionTestResult {
  success: boolean;
  error?: string;
}

// ---------------------------------------------------------------------------
// API Key
// ---------------------------------------------------------------------------

export interface ApiKeyResponse {
  api_key: string;
}

// ---------------------------------------------------------------------------
// Auth Error (from catch blocks)
// ---------------------------------------------------------------------------

export interface ApiError {
  data?: {
    error?: string;
  };
  message?: string;
}

// ---------------------------------------------------------------------------
// Selected audit/preview detail item (used by ScoreDetailModal)
// ---------------------------------------------------------------------------

export interface SelectedDetailItem {
  mediaName: string;
  mediaType: string;
  _score: number;
  scoreDetails: string;
  sizeBytes: number;
  action: string;
  createdAt: string;
}

// ---------------------------------------------------------------------------
// Settings Backup & Restore (API endpoints: /api/v1/settings/export, /import)
// ---------------------------------------------------------------------------

export interface PreferencesExport {
  logLevel: string;
  auditLogRetentionDays: number;
  pollIntervalSeconds: number;
  watchHistoryWeight: number;
  lastWatchedWeight: number;
  fileSizeWeight: number;
  ratingWeight: number;
  timeInLibraryWeight: number;
  seriesStatusWeight: number;
  executionMode: string;
  tiebreakerMethod: string;
  deletionsEnabled: boolean;
  snoozeDurationHours: number;
  checkForUpdates: boolean;
}

export interface RuleExport {
  field: string;
  operator: string;
  value: string;
  effect: string;
  enabled: boolean;
  integrationName: string | null;
  integrationType: string | null;
}

export interface IntegrationExport {
  name: string;
  type: string;
  url: string;
  enabled: boolean;
}

export interface DiskGroupExport {
  mountPath: string;
  thresholdPct: number;
  targetPct: number;
  totalBytesOverride?: number | null;
}

export interface NotificationExport {
  name: string;
  type: string;
  enabled: boolean;
  appriseTags?: string;
  onCycleDigest: boolean;
  onError: boolean;
  onModeChanged: boolean;
  onServerStarted: boolean;
  onThresholdBreach: boolean;
  onUpdateAvailable: boolean;
  onApprovalActivity: boolean;
  onIntegrationStatus: boolean;
}

export interface SettingsExportEnvelope {
  version: number;
  exportedAt: string;
  appVersion: string;
  preferences?: PreferencesExport;
  rules?: RuleExport[];
  integrations?: IntegrationExport[];
  diskGroups?: DiskGroupExport[];
  notificationChannels?: NotificationExport[];
}

export interface ExportSections {
  preferences: boolean;
  rules: boolean;
  integrations: boolean;
  diskGroups: boolean;
  notificationChannels: boolean;
}

export interface ImportSections {
  preferences: boolean;
  rules: boolean;
  integrations: boolean;
  diskGroups: boolean;
  notificationChannels: boolean;
  mode?: 'merge' | 'sync';
}

export interface ImportResult {
  preferencesImported: boolean;
  rulesImported: number;
  rulesUnmatched: number;
  integrationsImported: number;
  diskGroupsImported: number;
  notificationChannelsImported: number;
  itemsDeleted: number;
  preImportSnapshot?: SettingsExportEnvelope;
}

// ---------------------------------------------------------------------------
// Import Preview / Resolution
// ---------------------------------------------------------------------------

export interface IntCandidate {
  id: number;
  name: string;
  type: string;
}

export interface RuleResolution {
  index: number;
  rule: RuleExport;
  resolution: 'matched' | 'type_fallback' | 'unmatched';
  matchedIntegrationId: number | null;
  matchedIntegrationName?: string;
  candidates: IntCandidate[];
}

export interface ItemResolution {
  name: string;
  type?: string;
  action: 'create' | 'update' | 'unchanged';
  changes?: FieldChange[];
}

export interface FieldChange {
  field: string;
  oldValue: string;
  newValue: string;
}

export interface PreferencesResolution {
  action: 'update' | 'unchanged';
  changes?: FieldChange[];
}

export interface DeletionPreview {
  rules?: string[];
  integrations?: string[];
  notifications?: string[];
  diskGroups?: string[];
}

export interface ImportPreview {
  rules: RuleResolution[];
  integrations?: ItemResolution[];
  notifications?: ItemResolution[];
  diskGroups?: ItemResolution[];
  preferences?: PreferencesResolution;
  deletions?: DeletionPreview;
}

export interface RuleOverride {
  index: number;
  integrationId: number | null;
  skip: boolean;
}

// ---------------------------------------------------------------------------
// Notification Channel
// ---------------------------------------------------------------------------

export interface NotificationChannel {
  id: number;
  type: 'discord' | 'apprise';
  name: string;
  webhookUrl?: string;
  appriseTags?: string;
  enabled: boolean;
  onCycleDigest: boolean;
  onError: boolean;
  onModeChanged: boolean;
  onServerStarted: boolean;
  onThresholdBreach: boolean;
  onUpdateAvailable: boolean;
  onApprovalActivity: boolean;
  onIntegrationStatus: boolean;
  createdAt: string;
  updatedAt: string;
}

// ---------------------------------------------------------------------------
// Deletion Queue (API endpoint: /api/deletion-queue)
// ---------------------------------------------------------------------------

export interface DeletionQueueItem {
  mediaName: string;
  mediaType: string;
  sizeBytes: number;
  integrationId: number;
  score: number;
}

export interface DeletionCompletedItem {
  mediaName: string;
  mediaType: string;
  sizeBytes: number;
  status: 'success' | 'failed' | 'cancelled';
  timestamp: string;
}
