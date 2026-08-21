package db

import (
	"time"
)

// AuthConfig stores credentials for web UI sessions.
// The Password field stores a bcrypt hash. The APIKey field stores a SHA-256
// hash (prefixed with "sha256:") — plaintext keys are shown once on generation
// and never stored. Legacy plaintext keys are transparently upgraded on first use.
type AuthConfig struct {
	ID         uint   `gorm:"primarykey"`
	Username   string `gorm:"uniqueIndex;not null"`
	Password   string `gorm:"not null" json:"-"` // bcrypt hash — never serialized to JSON
	APIKey     string `gorm:"index" json:"-"`    // SHA-256 hash (sha256:<hex>) or legacy plaintext — never serialized to JSON
	APIKeyHint string // Last 4 characters of the plaintext key for identification
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// DiskGroup represents a physical disk/mount point shared by multiple services.
type DiskGroup struct {
	ID                 uint       `gorm:"primarykey" json:"id"`
	MountPath          string     `gorm:"uniqueIndex;not null" json:"mountPath"`
	TotalBytes         int64      `gorm:"not null" json:"totalBytes"`
	UsedBytes          int64      `gorm:"not null" json:"usedBytes"`
	TotalBytesOverride *int64     `json:"totalBytesOverride,omitempty"`            // User-defined total; nil = use detected
	ThresholdPct       float64    `gorm:"not null;default:85" json:"thresholdPct"` // Clean up at this % (escalation trigger in sunset mode)
	TargetPct          float64    `gorm:"not null;default:75" json:"targetPct"`    // Free down to this %
	Mode               string     `gorm:"not null;default:'dry-run'" json:"mode"`  // "dry-run", "approval", "auto", "sunset" — per-group execution mode
	SunsetPct          *float64   `json:"sunsetPct,omitempty"`                     // Sunset countdown starts at this %; NULL until explicitly configured
	StaleSince         *time.Time `json:"staleSince,omitempty"`                    // Non-nil = stale (not reported by any integration); nil = active
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

// EffectiveTotalBytes returns the user override if set, otherwise the API-detected total.
func (g DiskGroup) EffectiveTotalBytes() int64 {
	if g.TotalBytesOverride != nil && *g.TotalBytesOverride > 0 {
		return *g.TotalBytesOverride
	}
	return g.TotalBytes
}

// IntegrationConfig stores a configured service connection.
//
// SECURITY NOTE: Integration API keys (e.g. Sonarr, Radarr, Plex tokens) are
// stored in plaintext in SQLite. This is an accepted trade-off for a
// self-hosted home-lab tool: full encryption-at-rest would require a master
// key, which adds significant complexity and key-management burden with
// minimal practical benefit when the SQLite file is already on a user-owned
// machine. If the database file is compromised, the attacker already has
// access to the host. Ensure the DB file permissions are restrictive (0600).
type IntegrationConfig struct {
	ID                  uint       `gorm:"primarykey" json:"id"`
	Type                string     `gorm:"not null;index" json:"type"` // See ValidIntegrationTypes in validation.go for allowed values
	Name                string     `gorm:"not null" json:"name"`       // User-friendly name
	URL                 string     `gorm:"not null" json:"url"`
	APIKey              string     `gorm:"not null" json:"apiKey"` // API key or Plex token
	Enabled             bool       `gorm:"default:true" json:"enabled"`
	MediaSizeBytes      int64      `json:"mediaSizeBytes"` // Total media file size
	MediaCount          int        `json:"mediaCount"`     // Number of media items
	LastSync            *time.Time `json:"lastSync,omitempty"`
	LastError           string     `json:"lastError,omitempty"`
	CollectionDeletion  bool       `gorm:"default:false" json:"collectionDeletion"`       // When enabled, deleting one collection member deletes all
	ShowLevelOnly       bool       `gorm:"default:false" json:"showLevelOnly"`            // Sonarr only: evaluate entire shows instead of individual seasons
	AddImportExclusion  bool       `gorm:"default:true" json:"addImportExclusion"`        // *arr only: add to import exclusion list on delete (prevents re-add by import lists)
	ConsecutiveFailures int        `gorm:"default:0;not null" json:"consecutiveFailures"` // Incremented on connection test failure, reset on success
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

// DiskGroupIntegration tracks which integrations reported each disk group.
// The junction table is cleared and repopulated on each poll cycle.
type DiskGroupIntegration struct {
	DiskGroupID   uint `gorm:"primaryKey" json:"diskGroupId"`
	IntegrationID uint `gorm:"primaryKey" json:"integrationId"`
}

// LibraryHistory stores historical capacity logs per disk group.
// Named "library_histories" in the database for historical reasons (v2.0 baseline).
type LibraryHistory struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	Timestamp     time.Time `gorm:"index;not null" json:"timestamp"`
	TotalCapacity int64     `gorm:"not null" json:"totalCapacity"`
	UsedCapacity  int64     `gorm:"not null" json:"usedCapacity"`
	Resolution    string    `gorm:"index;not null" json:"resolution"`   // "raw", "hourly", "daily", "weekly"
	DiskGroupID   *uint     `gorm:"index" json:"diskGroupId,omitempty"` // FK to DiskGroup (ON DELETE CASCADE)
	CreatedAt     time.Time `json:"createdAt"`
}

// PreferenceSet stores global application settings (engine modes, thresholds,
// analytics config). Scoring factor weights are stored separately in the
// scoring_factor_weights table — see ScoringFactorWeight.
//
// Note: execution mode was a global setting in 2.x (ExecutionMode). In 3.0 it
// moved to per-disk-group (DiskGroup.Mode). The global field is now
// DefaultDiskGroupMode — used only as the default for newly auto-discovered
// disk groups.
type PreferenceSet struct {
	ID                        uint      `gorm:"primarykey" json:"id"`
	LogLevel                  string    `gorm:"default:'info';not null" json:"logLevel"`                                               // "debug", "info", "warn", "error"
	AuditLogRetentionDays     int       `gorm:"default:30;not null" json:"auditLogRetentionDays"`                                      // 0 = forever, else days
	PollIntervalSeconds       int       `gorm:"default:300;not null" json:"pollIntervalSeconds"`                                       // minimum 60, default 300 (5 min)
	DefaultDiskGroupMode      string    `gorm:"column:default_disk_group_mode;default:'dry-run';not null" json:"defaultDiskGroupMode"` // Default mode for new disk groups: "dry-run", "approval", "auto", "sunset"
	TiebreakerMethod          string    `gorm:"default:'size_desc';not null" json:"tiebreakerMethod"`                                  // "size_desc", "size_asc", "name_asc", "oldest_first", "newest_first"
	DeletionsEnabled          bool      `gorm:"default:true;not null" json:"deletionsEnabled"`                                         // Safety guard: actual deletions only when true
	SnoozeDurationHours       int       `gorm:"default:24;not null" json:"snoozeDurationHours"`                                        // Hours to snooze rejected items before re-evaluation
	CheckForUpdates           bool      `gorm:"default:true;not null" json:"checkForUpdates"`                                          // Enable outbound update checks
	DeletionQueueDelaySeconds int       `gorm:"default:30;not null" json:"deletionQueueDelaySeconds"`                                  // Grace period before processing queued deletions (10-300)
	DeadContentMinDays        int       `gorm:"default:90;not null" json:"deadContentMinDays"`                                         // Minimum days in library for "dead content" report
	StaleContentDays          int       `gorm:"default:180;not null" json:"staleContentDays"`                                          // Days since last watch for "stale content" report
	SunsetDays                int       `gorm:"default:30;not null" json:"sunsetDays"`                                                 // Default countdown period in days for sunset mode
	SunsetLabel               string    `gorm:"default:'capacitarr-sunset';not null" json:"sunsetLabel"`                               // Label/tag applied to media server items in sunset queue
	PosterOverlayStyle        string    `gorm:"default:'countdown';not null" json:"posterOverlayStyle"`                                // "off" (disabled), "countdown" (exact days), or "simple" ("Leaving soon")
	SunsetRescoreEnabled      bool      `gorm:"default:true;not null" json:"sunsetRescoreEnabled"`                                     // Enable daily re-scoring of sunset queue items; if score drops, item is saved instead of deleted
	SavedDurationDays         int       `gorm:"default:7;not null" json:"savedDurationDays"`                                           // How long the "Saved" marker/overlay persists before auto-cleanup (days)
	SavedLabel                string    `gorm:"default:'capacitarr-saved';not null" json:"savedLabel"`                                 // Label/tag applied to media server items that were saved by activity
	BackupRetentionDays       int       `gorm:"default:7;not null" json:"backupRetentionDays"`                                         // How many days of automatic database backups to keep (3, 7, 14, 30)
	DiskGroupGracePeriodDays  int       `gorm:"default:7;not null" json:"diskGroupGracePeriodDays"`                                    // Days before stale disk groups are reaped (0 = immediate)
	UpdatedAt                 time.Time `json:"updatedAt"`
}

// ScoringFactorWeight stores the user-configured weight for a single scoring
// factor. The factor_key matches ScoringFactor.Key() from engine/factors.go.
// Rows are auto-seeded from DefaultFactors() on startup — adding a new factor
// implementation is all that's needed to register a new weight.
type ScoringFactorWeight struct {
	FactorKey string    `gorm:"primaryKey;column:factor_key" json:"key"`
	Weight    int       `gorm:"not null;default:5" json:"weight"` // 0-10 scale
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// TableName returns the database table name for ScoringFactorWeight.
func (ScoringFactorWeight) TableName() string {
	return "scoring_factor_weights"
}

// CustomRule stores custom rules that influence media scoring via keep/remove effects.
type CustomRule struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	IntegrationID *uint     `gorm:"index" json:"integrationId"` // FK to IntegrationConfig; required — every rule must belong to an integration
	Field         string    `gorm:"not null" json:"field"`      // e.g. "quality", "tag", "rating"
	Operator      string    `gorm:"not null" json:"operator"`   // e.g. "==", "contains", ">"
	Value         string    `gorm:"not null" json:"value"`      // e.g. "4K", "anime", "7.5"
	Effect        string    `gorm:"not null" json:"effect"`     // e.g. "always_keep", "prefer_remove"
	Enabled       bool      `gorm:"default:true;not null" json:"enabled"`
	SortOrder     int       `gorm:"default:0;not null" json:"sortOrder"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// Approval queue status constants — used in ApprovalQueueItem.Status field.
const (
	StatusPending  = "pending"
	StatusApproved = "approved"
	StatusRejected = "rejected"
)

// Execution mode constants — used in DiskGroup.Mode and PreferenceSet.DefaultDiskGroupMode.
const (
	ModeAuto     = "auto"
	ModeDryRun   = "dry-run"
	ModeApproval = "approval"
	ModeSunset   = "sunset"
)

// Tiebreaker method constants — used in PreferenceSet.TiebreakerMethod field.
const (
	TiebreakerSizeDesc    = "size_desc"
	TiebreakerSizeAsc     = "size_asc"
	TiebreakerNameAsc     = "name_asc"
	TiebreakerOldestFirst = "oldest_first"
	TiebreakerNewestFirst = "newest_first"
)

// Log level constants — used in PreferenceSet.LogLevel field.
const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// ApprovalQueueItem represents an item in the approval queue (state machine).
// Items flow through: pending → approved → (deletion) OR pending → rejected (snoozed).
// Items with UserInitiated=true were queued by a user (via POST /delete) rather than
// the engine poller, and are preserved when the queue is cleared on below-threshold cycles.
type ApprovalQueueItem struct {
	ID              uint       `gorm:"primarykey" json:"id"`
	MediaName       string     `gorm:"index;not null" json:"mediaName"`
	MediaType       string     `gorm:"not null" json:"mediaType"`                            // movie, show, season, episode, artist, album, book
	ScoreDetails    string     `gorm:"type:text" json:"scoreDetails"`                        // JSON-encoded []ScoreFactor
	SizeBytes       int64      `gorm:"not null;default:0" json:"sizeBytes"`                  // File size in bytes
	Score           float64    `gorm:"not null;default:0" json:"score"`                      // Numeric score from engine evaluation
	PosterURL       string     `gorm:"not null;default:''" json:"posterUrl"`                 // Poster image URL from *arr
	IntegrationID   uint       `gorm:"not null" json:"integrationId"`                        // FK to IntegrationConfig (required)
	ExternalID      string     `gorm:"not null;default:''" json:"externalId"`                // External ID in the integration
	DiskGroupID     *uint      `gorm:"index" json:"diskGroupId,omitempty"`                   // FK to DiskGroup (nullable — set by poller to scope queue per disk group)
	Status          string     `gorm:"not null;default:'pending'" json:"status"`             // pending, approved, rejected
	Trigger         string     `gorm:"not null;default:'engine'" json:"trigger"`             // "engine", "user"
	UserInitiated   bool       `gorm:"not null;default:false" json:"userInitiated"`          // True when queued by user via POST /delete (preserved on queue clear)
	CollectionGroup string     `gorm:"not null;default:''" json:"collectionGroup,omitempty"` // Groups collection members (e.g., "Sonic the Hedgehog Collection")
	SnoozedUntil    *time.Time `gorm:"column:snoozed_until" json:"snoozedUntil,omitempty"`   // When snooze expires (rejected items)
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// TableName returns the database table name for ApprovalQueueItem.
func (ApprovalQueueItem) TableName() string {
	return "approval_queue"
}

// SunsetQueueItem represents an item in the sunset countdown queue.
// Items enter when disk usage exceeds sunsetPct on a sunset-mode disk group.
// After the countdown expires (deletion_date <= now), the item is handed to
// DeletionService for actual removal. Unlike approval queue items, sunset items
// are time-driven (daily cron), not reconciled per engine cycle, and persist
// across cycles regardless of threshold changes.
type SunsetQueueItem struct {
	ID                  uint       `gorm:"primarykey" json:"id"`
	MediaName           string     `gorm:"index;not null" json:"mediaName"`
	MediaType           string     `gorm:"not null" json:"mediaType"`                            // movie, show, season, episode, artist, book
	TmdbID              *int       `gorm:"index" json:"tmdbId,omitempty"`                        // TMDb ID for media server label/poster targeting; nil if not matched
	IntegrationID       uint       `gorm:"not null" json:"integrationId"`                        // FK to IntegrationConfig
	ExternalID          string     `gorm:"not null;default:''" json:"externalId"`                // *arr external ID
	SizeBytes           int64      `gorm:"not null;default:0" json:"sizeBytes"`                  // File size in bytes
	Score               float64    `gorm:"not null;default:0" json:"score"`                      // Score at time of scheduling
	ScoreDetails        string     `gorm:"type:text" json:"scoreDetails"`                        // JSON-encoded score factors
	PosterURL           string     `json:"posterUrl,omitempty"`                                  // Original poster URL from *arr
	DiskGroupID         uint       `gorm:"index;not null" json:"diskGroupId"`                    // FK to DiskGroup
	CollectionGroup     string     `gorm:"not null;default:''" json:"collectionGroup,omitempty"` // Collection deletion group
	Trigger             string     `gorm:"not null;default:'engine'" json:"trigger"`             // "engine", "user"
	DeletionDate        time.Time  `gorm:"index;not null" json:"deletionDate"`                   // When to hand to DeletionService
	LabelApplied        bool       `gorm:"not null;default:false" json:"labelApplied"`           // Whether sunset label has been applied to media server
	PosterOverlayActive bool       `gorm:"not null;default:false" json:"posterOverlayActive"`    // Whether an overlay poster is currently uploaded
	Status              string     `gorm:"not null;default:'pending'" json:"status"`             // "pending" (in countdown), "saved" (score dropped, saved by activity), "expired" (handed to DeletionService)
	SavedAt             *time.Time `json:"savedAt,omitempty"`                                    // Non-nil when item was saved due to score drop; cleared on cleanup
	SavedScore          float64    `gorm:"not null;default:0" json:"savedScore"`                 // Score at the time the item was saved (for display)
	SavedReason         string     `gorm:"type:text" json:"savedReason,omitempty"`               // Human-readable explanation of why the item was saved
	ExpiredAt           *time.Time `json:"expiredAt,omitempty"`                                  // Non-nil when countdown expired and item was handed to DeletionService; item remains in queue for visibility
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

// TableName returns the database table name for SunsetQueueItem.
func (SunsetQueueItem) TableName() string {
	return "sunset_queue"
}

// Sunset queue status constants — used in SunsetQueueItem.Status field.
const (
	SunsetStatusPending = "pending" // In countdown
	SunsetStatusSaved   = "saved"   // Score dropped, saved by activity
	SunsetStatusExpired = "expired" // Handed to DeletionService
)

// Audit log action constants — used in AuditLogEntry.Action field.
const (
	ActionDeleted       = "deleted"
	ActionDryDelete     = "dry_delete"
	ActionCancelled     = "cancelled"
	ActionPendingDelete = "pending_delete" // intent written before a live *arr delete
)

// Trigger constants — used in AuditLogEntry.Trigger and ApprovalQueueItem.Trigger fields.
const (
	TriggerEngine   = "engine"
	TriggerUser     = "user"
	TriggerApproval = "approval"
)

// DryRunReason constants — used in AuditLogEntry.DryRunReason field.
const (
	DryRunReasonDeletionsDisabled = "deletions_disabled"
	DryRunReasonExecutionMode     = "execution_mode"
	DryRunReasonNone              = "" // Not a dry-run
)

// AuditLogEntry stores a permanent record of deletions and dry-runs.
// Rows are append-only except for two documented exceptions:
//   - dry-run upserts (same media_name/media_type, action=dry_delete)
//   - pending_delete → deleted after a successful live *arr API call
type AuditLogEntry struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	MediaName       string    `gorm:"index;not null" json:"mediaName"`
	MediaType       string    `gorm:"not null" json:"mediaType"`
	ScoreDetails    string    `gorm:"type:text" json:"scoreDetails"` // JSON-encoded []ScoreFactor
	Action          string    `gorm:"not null" json:"action"`        // "deleted", "dry_delete", "cancelled", "pending_delete"
	SizeBytes       int64     `gorm:"not null;default:0" json:"sizeBytes"`
	Score           float64   `gorm:"not null;default:0" json:"score"`                      // Numeric score from engine evaluation
	Trigger         string    `gorm:"not null;default:'engine'" json:"trigger"`             // "engine", "user", "approval"
	DryRunReason    string    `gorm:"not null;default:''" json:"dryRunReason"`              // "deletions_disabled", "execution_mode", "" (empty if not dry-run)
	IntegrationID   *uint     `json:"integrationId,omitempty" gorm:"column:integration_id"` // FK to IntegrationConfig (nullable — preserved on integration delete)
	DiskGroupID     *uint     `gorm:"index" json:"diskGroupId,omitempty"`                   // FK to DiskGroup (nullable — set by poller to scope audit entries per disk group)
	CollectionGroup string    `gorm:"not null;default:''" json:"collectionGroup,omitempty"` // Groups collection deletions (e.g., "Sonic the Hedgehog Collection")
	CreatedAt       time.Time `json:"createdAt"`
}

// TableName returns the database table name for AuditLogEntry.
func (AuditLogEntry) TableName() string {
	return "audit_log"
}

// EngineRunStats stores one row per engine evaluation cycle, persisting metrics
// across container restarts so the UI always shows the latest run's stats.
type EngineRunStats struct {
	ID             uint       `gorm:"primarykey" json:"id"`
	RunAt          time.Time  `gorm:"index;not null" json:"runAt"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
	Evaluated      int        `gorm:"not null;default:0" json:"evaluated"`
	Candidates     int        `gorm:"not null;default:0" json:"candidates"`
	Queued         int        `gorm:"not null;default:0" json:"queued"`
	Deleted        int        `gorm:"not null;default:0" json:"deleted"`
	FreedBytes     int64      `gorm:"not null;default:0" json:"freedBytes"`
	DiskGroupModes string     `gorm:"type:text" json:"diskGroupModes,omitempty"` // JSON map of diskGroupID → mode (e.g. {"1":"auto","2":"sunset"})
	DurationMs     int64      `gorm:"not null;default:0" json:"durationMs"`
	ErrorMessage   string     `json:"errorMessage,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"` // SQLite DEFAULT CURRENT_TIMESTAMP; mirrors RunAt
}

// LifetimeStats stores cumulative counters that persist across container restarts
// and are NOT cleared by the "Clear All Scraped Data" action. Singleton row (id=1).
type LifetimeStats struct {
	ID                  uint      `gorm:"primarykey" json:"id"`
	TotalBytesReclaimed int64     `gorm:"not null;default:0" json:"totalBytesReclaimed"`
	TotalItemsRemoved   int       `gorm:"not null;default:0" json:"totalItemsRemoved"`
	TotalEngineRuns     int       `gorm:"not null;default:0" json:"totalEngineRuns"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

// NotificationConfig stores a configured notification channel (Discord or Apprise).
type NotificationConfig struct {
	ID          uint   `gorm:"primarykey" json:"id"`
	Type        string `gorm:"not null" json:"type"`                    // "discord", "apprise"
	Name        string `gorm:"not null" json:"name"`                    // User-friendly name
	WebhookURL  string `json:"webhookUrl,omitempty"`                    // Discord webhook or Apprise API endpoint URL
	AppriseTags string `gorm:"default:''" json:"appriseTags,omitempty"` // Comma-separated Apprise tags for routing
	Enabled     bool   `gorm:"default:true" json:"enabled"`
	// Notification level controls which events trigger this channel.
	// Override fields (nullable) force individual events on/off regardless of level.
	NotificationLevel         string    `gorm:"default:'normal';not null" json:"notificationLevel"`
	OverrideCycleDigest       *bool     `json:"overrideCycleDigest,omitempty"`
	OverrideError             *bool     `json:"overrideError,omitempty"`
	OverrideModeChanged       *bool     `json:"overrideModeChanged,omitempty"`
	OverrideServerStarted     *bool     `json:"overrideServerStarted,omitempty"`
	OverrideThresholdBreach   *bool     `json:"overrideThresholdBreach,omitempty"`
	OverrideUpdateAvailable   *bool     `json:"overrideUpdateAvailable,omitempty"`
	OverrideApprovalActivity  *bool     `json:"overrideApprovalActivity,omitempty"`
	OverrideIntegrationStatus *bool     `json:"overrideIntegrationStatus,omitempty"`
	CreatedAt                 time.Time `json:"createdAt"`
	UpdatedAt                 time.Time `json:"updatedAt"`
}

// ActivityEvent stores system-level activity events for the dashboard feed.
// These complement audit logs (which track media actions) by recording
// operational events such as engine runs, settings changes, and logins.
// Retention is fixed at 7 days, auto-pruned by the daily cron job.
type ActivityEvent struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	EventType string    `gorm:"not null;index" json:"eventType"` // "engine_start", "engine_complete", etc.
	Message   string    `gorm:"not null" json:"message"`         // Human-readable: "Engine run completed: evaluated 97, candidates 12"
	Metadata  string    `gorm:"type:text" json:"metadata"`       // Optional JSON for extra data
	CreatedAt time.Time `json:"createdAt"`
}

// RollupState persists the last successful rollup timestamp per resolution
// tier (hourly, daily, weekly). Used by cron jobs to compute rollup windows
// from the last checkpoint instead of from time.Now(), making rollups
// idempotent and tolerant of scheduling delays.
type RollupState struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	Resolution    string    `gorm:"uniqueIndex;not null" json:"resolution"` // "hourly", "daily", "weekly"
	LastCompleted time.Time `gorm:"not null" json:"lastCompleted"`
}

// MediaCache is a singleton row (id=1) storing a JSON snapshot of the scored
// preview result. This allows the dashboard and analytics to have data
// immediately on startup without waiting for the first engine run.
// The row is fully replaced at the end of each engine cycle.
type MediaCache struct {
	ID          uint   `gorm:"primarykey"`
	PreviewJSON string `gorm:"column:preview_json;type:text;not null"`
	ItemCount   int    `gorm:"column:item_count;not null"`
	UpdatedAt   time.Time
}

// TableName returns the database table name for MediaCache.
func (MediaCache) TableName() string {
	return "media_cache"
}

// MediaServerMapping stores a resolved TMDb ID → media server native ID mapping.
// Populated during engine poll cycles from media server library scans.
// Used by PosterOverlayService and SunsetService to translate TMDb IDs into
// per-server identifiers (Plex ratingKey, Jellyfin/Emby item ID) for label
// and poster operations. Survives media server downtime (stale data is better
// than no data).
type MediaServerMapping struct {
	TmdbID        int       `gorm:"primaryKey;column:tmdb_id" json:"tmdbId"`
	IntegrationID uint      `gorm:"primaryKey;column:integration_id" json:"integrationId"`
	NativeID      string    `gorm:"not null;column:native_id" json:"nativeId"`
	MediaType     string    `gorm:"not null;default:'movie';column:media_type" json:"mediaType"`
	Title         string    `gorm:"not null;default:'';column:title" json:"title"`
	UpdatedAt     time.Time `gorm:"not null;column:updated_at" json:"updatedAt"`
}

// TableName returns the database table name for MediaServerMapping.
func (MediaServerMapping) TableName() string {
	return "media_server_mappings"
}
