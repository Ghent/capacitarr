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

export interface DiskGroup {
  id: number;
  mountPath: string;
  totalBytes: number;
  usedBytes: number;
  thresholdPct: number;
  targetPct: number;
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
  updatedAt: string;
}

// ---------------------------------------------------------------------------
// Custom Rule (API endpoint: /api/v1/custom-rules)
// ---------------------------------------------------------------------------

export interface CustomRule {
  id: number;
  integrationId?: number | null;
  field: string;
  operator: string;
  value: string;
  effect: string;
  enabled: boolean;
  sortOrder: number;
  createdAt: string;
  updatedAt: string;
}

// ---------------------------------------------------------------------------
// Audit Log
// ---------------------------------------------------------------------------

/** Action values match backend db.Action* constants (deleted, dry_run, dry_delete). */
export type AuditAction = 'deleted' | 'dry_run' | 'dry_delete';

export interface AuditLogEntry {
  id: number;
  mediaName: string;
  mediaType: string;
  reason: string;
  scoreDetails: string;
  action: AuditAction;
  sizeBytes: number;
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
  reason: string;
  scoreDetails: string;
  sizeBytes: number;
  posterUrl?: string;
  integrationId: number;
  externalId: string;
  status: 'pending' | 'approved' | 'rejected';
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
}

export interface EvaluatedItem {
  item: MediaItem;
  score: number;
  isProtected: boolean;
  reason: string;
  factors: ScoreFactor[];
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
// Sparkline tooltip opts shape (from ApexCharts)
// ---------------------------------------------------------------------------

export interface SparklineTooltipOpts {
  seriesIndex: number;
  dataPointIndex: number;
  w: unknown;
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
// Custom Rule Import/Export (API endpoints: /api/v1/custom-rules/export, /import)
// ---------------------------------------------------------------------------

export interface ExportedRule {
  field: string;
  operator: string;
  value: string;
  effect: string;
  enabled: boolean;
  integrationName: string | null;
  integrationType: string | null;
}

export interface RuleExportEnvelope {
  version: number;
  exportedAt: string;
  rules: ExportedRule[];
}

export interface RuleImportRequest {
  payload: RuleExportEnvelope;
  integrationMapping?: Record<string, number>;
}

export interface RuleImportResponse {
  imported: number;
  skipped: number;
}

// ---------------------------------------------------------------------------
// Notification Channel
// ---------------------------------------------------------------------------

export interface NotificationChannel {
  id: number;
  type: 'discord' | 'slack' | 'inapp';
  name: string;
  webhookUrl?: string;
  enabled: boolean;
  onThresholdBreach: boolean;
  onDeletionExecuted: boolean;
  onEngineError: boolean;
  onEngineComplete: boolean;
  createdAt: string;
  updatedAt: string;
}

// ---------------------------------------------------------------------------
// In-App Notification
// ---------------------------------------------------------------------------

export interface InAppNotification {
  id: number;
  title: string;
  message: string;
  severity: 'info' | 'warning' | 'error' | 'success';
  read: boolean;
  eventType: string;
  createdAt: string;
}
