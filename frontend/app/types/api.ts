/**
 * Shared TypeScript interfaces for Capacitarr API responses.
 * These mirror the backend Go structs and API response shapes.
 */

// ---------------------------------------------------------------------------
// Integration
// ---------------------------------------------------------------------------

export interface IntegrationConfig {
  id: number
  type: string
  name: string
  url: string
  apiKey: string
  enabled: boolean
  mediaSizeBytes: number
  mediaCount: number
  lastSync?: string | null
  lastError?: string
  createdAt: string
  updatedAt: string
}

// ---------------------------------------------------------------------------
// Disk Group
// ---------------------------------------------------------------------------

export interface DiskGroup {
  id: number
  mountPath: string
  totalBytes: number
  usedBytes: number
  thresholdPct: number
  targetPct: number
  createdAt: string
  updatedAt: string
}

// ---------------------------------------------------------------------------
// Preferences
// ---------------------------------------------------------------------------

export interface PreferenceSet {
  id: number
  logLevel: string
  auditLogRetentionDays: number
  pollIntervalSeconds: number
  watchHistoryWeight: number
  lastWatchedWeight: number
  fileSizeWeight: number
  ratingWeight: number
  timeInLibraryWeight: number
  seriesStatusWeight: number
  executionMode: string
  tiebreakerMethod: string
  deletionsEnabled: boolean
  updatedAt: string
}

// ---------------------------------------------------------------------------
// Custom Rule (API endpoint: /api/v1/custom-rules)
// ---------------------------------------------------------------------------

export interface CustomRule {
  id: number
  integrationId?: number | null
  field: string
  operator: string
  value: string
  effect: string
  enabled: boolean
  sortOrder: number
  /** @deprecated Legacy field — kept for migration compatibility */
  type?: string
  /** @deprecated Legacy field — kept for migration compatibility */
  intensity?: string
  createdAt: string
  updatedAt: string
}

// ---------------------------------------------------------------------------
// Audit Log
// ---------------------------------------------------------------------------

export interface AuditLog {
  id: number
  mediaName: string
  mediaType: string
  reason: string
  scoreDetails: string
  action: string
  sizeBytes: number
  createdAt: string
}

export interface AuditResponse {
  data: AuditLog[]
  total: number
  limit: number
  offset: number
}

// ---------------------------------------------------------------------------
// Engine / Worker Stats
// ---------------------------------------------------------------------------

export interface WorkerStats {
  executionMode: string
  isRunning: boolean
  pollIntervalSeconds: number
  queueDepth: number
  lastRunEvaluated: number
  lastRunFlagged: number
  lastRunFreedBytes: number
  lastRunEpoch: number
  currentlyDeleting: string
  protectedCount: number
  evaluated: number
  actioned: number
  freedBytes: number
  processed: number
  failed: number
}

// ---------------------------------------------------------------------------
// Dashboard Stats
// ---------------------------------------------------------------------------

export interface DashboardStats {
  totalBytesReclaimed: number
  totalItemsRemoved: number
  totalEngineRuns: number
  protectedCount: number
  growthBytesPerWeek: number
  hasGrowthData: boolean
}

// ---------------------------------------------------------------------------
// Media / Scoring (Preview)
// ---------------------------------------------------------------------------

export interface MediaItem {
  externalId: string
  integrationId: number
  type: string
  title: string
  year?: number
  sizeBytes: number
  path: string
  seasonNumber?: number
  episodeCount?: number
  showTitle?: string
  seriesStatus?: string
  qualityProfile?: string
  rating?: number
  genre?: string
  monitored: boolean
  playCount?: number
  lastPlayed?: string | null
  addedAt?: string | null
  tags?: string[]
  isRequested?: boolean
  requestedBy?: string
  requestCount?: number
  tmdbId?: number
  language?: string
}

export interface ScoreFactor {
  name: string
  rawScore: number
  weight: number
  contribution: number
  type: string
}

export interface EvaluatedItem {
  item: MediaItem
  score: number
  isProtected: boolean
  reason: string
  factors: ScoreFactor[]
}

export interface PreviewResponse {
  items: EvaluatedItem[]
  diskContext: DiskContext | null
}

export interface DiskContext {
  totalBytes: number
  usedBytes: number
  targetPct: number
  thresholdPct: number
  bytesToFree: number
}

// ---------------------------------------------------------------------------
// Metrics History
// ---------------------------------------------------------------------------

export interface LibraryHistoryRow {
  ID: number
  Timestamp: string
  TotalCapacity: number
  UsedCapacity: number
  Resolution: string
  DiskGroupID?: number | null
  CreatedAt: string
}

export interface MetricsHistoryResponse {
  status: string
  data: LibraryHistoryRow[]
}

// ---------------------------------------------------------------------------
// Connection Test
// ---------------------------------------------------------------------------

export interface ConnectionTestResult {
  success: boolean
  error?: string
}

// ---------------------------------------------------------------------------
// API Key
// ---------------------------------------------------------------------------

export interface ApiKeyResponse {
  api_key: string
}

// ---------------------------------------------------------------------------
// Data Reset
// ---------------------------------------------------------------------------

export interface DataResetResponse {
  status: string
  message?: string
}

// ---------------------------------------------------------------------------
// Auth Error (from catch blocks)
// ---------------------------------------------------------------------------

export interface ApiError {
  data?: {
    error?: string
  }
  message?: string
}

// ---------------------------------------------------------------------------
// Sparkline tooltip opts shape (from ApexCharts)
// ---------------------------------------------------------------------------

export interface SparklineTooltipOpts {
  seriesIndex: number
  dataPointIndex: number
  w: unknown
}

// ---------------------------------------------------------------------------
// Selected audit/preview detail item (used by ScoreDetailModal)
// ---------------------------------------------------------------------------

export interface SelectedDetailItem {
  mediaName: string
  mediaType: string
  _score: number
  scoreDetails: string
  sizeBytes: number
  action: string
  createdAt: string
}

// ---------------------------------------------------------------------------
// Notification Channel
// ---------------------------------------------------------------------------

export interface NotificationChannel {
  id: number
  type: 'discord' | 'slack' | 'inapp'
  name: string
  webhookUrl?: string
  enabled: boolean
  onThresholdBreach: boolean
  onDeletionExecuted: boolean
  onEngineError: boolean
  onEngineComplete: boolean
  createdAt: string
  updatedAt: string
}

// ---------------------------------------------------------------------------
// In-App Notification
// ---------------------------------------------------------------------------

export interface InAppNotification {
  id: number
  title: string
  message: string
  severity: 'info' | 'warning' | 'error' | 'success'
  read: boolean
  eventType: string
  createdAt: string
}
