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
	ID                 uint      `gorm:"primarykey" json:"id"`
	MountPath          string    `gorm:"uniqueIndex;not null" json:"mountPath"`
	TotalBytes         int64     `gorm:"not null" json:"totalBytes"`
	UsedBytes          int64     `gorm:"not null" json:"usedBytes"`
	TotalBytesOverride *int64    `json:"totalBytesOverride,omitempty"`   // User-defined total; nil = use detected
	ThresholdPct       float64   `gorm:"default:85" json:"thresholdPct"` // Clean up at this %
	TargetPct          float64   `gorm:"default:75" json:"targetPct"`    // Free down to this %
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// EffectiveTotalBytes returns the user override if set, otherwise the API-detected total.
func (g DiskGroup) EffectiveTotalBytes() int64 {
	if g.TotalBytesOverride != nil && *g.TotalBytesOverride > 0 {
		return *g.TotalBytesOverride
	}
	return g.TotalBytes
}

// Library groups integrations into a logical library with optional threshold overrides.
// Threshold hierarchy: integration override → library override → disk group default.
type Library struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	Name         string    `gorm:"not null" json:"name"`
	DiskGroupID  *uint     `gorm:"index" json:"diskGroupId,omitempty"` // FK to DiskGroup (ON DELETE SET NULL)
	ThresholdPct *float64  `json:"thresholdPct,omitempty"`             // Override disk group threshold; nil = inherit
	TargetPct    *float64  `json:"targetPct,omitempty"`                // Override disk group target; nil = inherit
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
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
	ID             uint       `gorm:"primarykey" json:"id"`
	Type           string     `gorm:"not null;index" json:"type"` // plex, sonarr, radarr, lidarr, readarr, tautulli, seerr, jellyfin, emby
	Name           string     `gorm:"not null" json:"name"`       // User-friendly name
	URL            string     `gorm:"not null" json:"url"`
	APIKey         string     `gorm:"not null" json:"apiKey"` // API key or Plex token
	Enabled        bool       `gorm:"default:true" json:"enabled"`
	LibraryID      *uint      `gorm:"index" json:"libraryId,omitempty"` // FK to Library (ON DELETE SET NULL)
	ThresholdPct   *float64   `json:"thresholdPct,omitempty"`           // Per-integration threshold override; nil = inherit
	TargetPct      *float64   `json:"targetPct,omitempty"`              // Per-integration target override; nil = inherit
	MediaSizeBytes int64      `json:"mediaSizeBytes"`                   // Total media file size
	MediaCount     int        `json:"mediaCount"`                       // Number of media items
	LastSync       *time.Time `json:"lastSync,omitempty"`
	LastError      string     `json:"lastError,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// DiskGroupIntegration tracks which integrations reported each disk group.
// The junction table is cleared and repopulated on each poll cycle.
type DiskGroupIntegration struct {
	DiskGroupID   uint `gorm:"primaryKey" json:"diskGroupId"`
	IntegrationID uint `gorm:"primaryKey" json:"integrationId"`
}

// LibraryHistory stores historical capacity logs.
type LibraryHistory struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	Timestamp     time.Time `gorm:"index;not null" json:"timestamp"`
	TotalCapacity int64     `gorm:"not null" json:"totalCapacity"`
	UsedCapacity  int64     `gorm:"not null" json:"usedCapacity"`
	Resolution    string    `gorm:"index;not null" json:"resolution"`   // "raw", "hourly", "daily", "weekly"
	DiskGroupID   *uint     `gorm:"index" json:"diskGroupId,omitempty"` // FK to DiskGroup (ON DELETE CASCADE)
	LibraryID     *uint     `gorm:"index" json:"libraryId,omitempty"`   // FK to Library (ON DELETE CASCADE)
	CreatedAt     time.Time `json:"createdAt"`
}

// PreferenceSet stores the global weights for the scoring engine (0-10 scale)
// and analytics threshold settings.
type PreferenceSet struct {
	ID                        uint      `gorm:"primarykey" json:"id"`
	LogLevel                  string    `gorm:"default:'info';not null" json:"logLevel"`          // "debug", "info", "warn", "error"
	AuditLogRetentionDays     int       `gorm:"default:30;not null" json:"auditLogRetentionDays"` // 0 = forever, else days
	PollIntervalSeconds       int       `gorm:"default:300;not null" json:"pollIntervalSeconds"`  // minimum 60, default 300 (5 min)
	WatchHistoryWeight        int       `gorm:"default:10" json:"watchHistoryWeight"`             // High default
	LastWatchedWeight         int       `gorm:"default:8" json:"lastWatchedWeight"`
	FileSizeWeight            int       `gorm:"default:6" json:"fileSizeWeight"`
	RatingWeight              int       `gorm:"default:5" json:"ratingWeight"`
	TimeInLibraryWeight       int       `gorm:"default:4" json:"timeInLibraryWeight"`
	SeriesStatusWeight        int       `gorm:"default:3" json:"seriesStatusWeight"`
	RequestPopularityWeight   int       `gorm:"default:2" json:"requestPopularityWeight"`             // NEW in 2.0: RequestPopularityFactor
	ExecutionMode             string    `gorm:"default:'dry-run';not null" json:"executionMode"`      // "dry-run", "approval", "auto"
	TiebreakerMethod          string    `gorm:"default:'size_desc';not null" json:"tiebreakerMethod"` // "size_desc", "size_asc", "name_asc", "oldest_first", "newest_first"
	DeletionsEnabled          bool      `gorm:"default:true;not null" json:"deletionsEnabled"`        // Safety guard: actual deletions only when true
	SnoozeDurationHours       int       `gorm:"default:24;not null" json:"snoozeDurationHours"`       // Hours to snooze rejected items before re-evaluation
	CheckForUpdates           bool      `gorm:"default:true;not null" json:"checkForUpdates"`         // Enable outbound update checks
	DeletionQueueDelaySeconds int       `gorm:"default:30;not null" json:"deletionQueueDelaySeconds"` // Grace period before processing queued deletions (10-300)
	DeadContentMinDays        int       `gorm:"default:90;not null" json:"deadContentMinDays"`        // NEW in 2.0: Minimum days in library for "dead content" report
	StaleContentDays          int       `gorm:"default:180;not null" json:"staleContentDays"`         // NEW in 2.0: Days since last watch for "stale content" report
	UpdatedAt                 time.Time `json:"updatedAt"`
}

// GetFactorWeight returns the configured weight for a scoring factor by key.
// Keys map to the ScoringFactor.Key() values defined in engine/factors.go.
func (p PreferenceSet) GetFactorWeight(key string) int {
	switch key {
	case "watch_history":
		return p.WatchHistoryWeight
	case "last_watched":
		return p.LastWatchedWeight
	case "file_size":
		return p.FileSizeWeight
	case "rating":
		return p.RatingWeight
	case "time_in_library":
		return p.TimeInLibraryWeight
	case "series_status":
		return p.SeriesStatusWeight
	case "request_popularity":
		return p.RequestPopularityWeight
	default:
		return 0
	}
}

// CustomRule stores custom rules that influence media scoring via keep/remove effects.
type CustomRule struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	IntegrationID *uint     `gorm:"index" json:"integrationId"` // FK to IntegrationConfig; nil = global rule
	LibraryID     *uint     `gorm:"index" json:"libraryId"`     // FK to Library; nil = global rule (NEW in 2.0: per-library scoping)
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

// Execution mode constants — used in PreferenceSet.ExecutionMode field.
const (
	ModeAuto     = "auto"
	ModeDryRun   = "dry-run"
	ModeApproval = "approval"
)

// ApprovalQueueItem represents an item in the approval queue (state machine).
// Items flow through: pending → approved → (deletion) OR pending → rejected (snoozed).
// Items with UserInitiated=true were queued by a user (via POST /delete) rather than
// the engine poller, and are preserved when the queue is cleared on below-threshold cycles.
type ApprovalQueueItem struct {
	ID            uint       `gorm:"primarykey" json:"id"`
	MediaName     string     `gorm:"index;not null" json:"mediaName"`
	MediaType     string     `gorm:"not null" json:"mediaType"`                          // movie, show, season, episode, artist, album, book
	ScoreDetails  string     `gorm:"type:text" json:"scoreDetails"`                      // JSON-encoded []ScoreFactor
	SizeBytes     int64      `gorm:"not null;default:0" json:"sizeBytes"`                // File size in bytes
	Score         float64    `gorm:"not null;default:0" json:"score"`                    // Numeric score from engine evaluation
	PosterURL     string     `gorm:"not null;default:''" json:"posterUrl"`               // Poster image URL from *arr
	IntegrationID uint       `gorm:"not null" json:"integrationId"`                      // FK to IntegrationConfig (required)
	ExternalID    string     `gorm:"not null;default:''" json:"externalId"`              // External ID in the integration
	DiskGroupID   *uint      `gorm:"index" json:"diskGroupId,omitempty"`                 // FK to DiskGroup (nullable — set by poller to scope queue per disk group)
	Status        string     `gorm:"not null;default:'pending'" json:"status"`           // pending, approved, rejected
	Trigger       string     `gorm:"not null;default:'engine'" json:"trigger"`           // "engine", "user"
	UserInitiated bool       `gorm:"not null;default:false" json:"userInitiated"`        // True when queued by user via POST /delete (preserved on queue clear)
	SnoozedUntil  *time.Time `gorm:"column:snoozed_until" json:"snoozedUntil,omitempty"` // When snooze expires (rejected items)
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// TableName returns the database table name for ApprovalQueueItem.
func (ApprovalQueueItem) TableName() string {
	return "approval_queue"
}

// Audit log action constants — used in AuditLogEntry.Action field.
const (
	ActionDeleted   = "deleted"
	ActionDryDelete = "dry_delete"
	ActionCancelled = "cancelled"
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
// This table is append-only — entries are never modified after creation.
type AuditLogEntry struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	MediaName     string    `gorm:"index;not null" json:"mediaName"`
	MediaType     string    `gorm:"not null" json:"mediaType"`
	ScoreDetails  string    `gorm:"type:text" json:"scoreDetails"` // JSON-encoded []ScoreFactor
	Action        string    `gorm:"not null" json:"action"`        // "deleted", "dry_delete", "cancelled"
	SizeBytes     int64     `gorm:"not null;default:0" json:"sizeBytes"`
	Score         float64   `gorm:"not null;default:0" json:"score"`                      // Numeric score from engine evaluation
	Trigger       string    `gorm:"not null;default:'engine'" json:"trigger"`             // "engine", "user", "approval"
	DryRunReason  string    `gorm:"not null;default:''" json:"dryRunReason"`              // "deletions_disabled", "execution_mode", "" (empty if not dry-run)
	IntegrationID *uint     `json:"integrationId,omitempty" gorm:"column:integration_id"` // FK to IntegrationConfig (nullable — preserved on integration delete)
	DiskGroupID   *uint     `gorm:"index" json:"diskGroupId,omitempty"`                   // FK to DiskGroup (nullable — set by poller to scope audit entries per disk group)
	CreatedAt     time.Time `json:"createdAt"`
}

// TableName returns the database table name for AuditLogEntry.
func (AuditLogEntry) TableName() string {
	return "audit_log"
}

// EngineRunStats stores one row per engine evaluation cycle, persisting metrics
// across container restarts so the UI always shows the latest run's stats.
type EngineRunStats struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	RunAt         time.Time `gorm:"index;not null" json:"runAt"`
	Evaluated     int       `gorm:"not null;default:0" json:"evaluated"`
	Flagged       int       `gorm:"not null;default:0" json:"flagged"`
	Deleted       int       `gorm:"not null;default:0" json:"deleted"`
	FreedBytes    int64     `gorm:"not null;default:0" json:"freedBytes"`
	ExecutionMode string    `gorm:"not null;default:'dry-run'" json:"executionMode"`
	DurationMs    int64     `gorm:"not null;default:0" json:"durationMs"`
	ErrorMessage  string    `json:"errorMessage,omitempty"`
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
	// Event subscriptions — which notification types trigger this channel
	OnCycleDigest      bool      `gorm:"default:true" json:"onCycleDigest"`
	OnError            bool      `gorm:"default:true" json:"onError"`
	OnModeChanged      bool      `gorm:"default:true" json:"onModeChanged"`
	OnServerStarted    bool      `gorm:"default:true" json:"onServerStarted"`
	OnThresholdBreach  bool      `gorm:"default:true" json:"onThresholdBreach"`
	OnUpdateAvailable  bool      `gorm:"default:true" json:"onUpdateAvailable"`
	OnApprovalActivity bool      `gorm:"default:true" json:"onApprovalActivity"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// ActivityEvent stores system-level activity events for the dashboard feed.
// These complement audit logs (which track media actions) by recording
// operational events such as engine runs, settings changes, and logins.
// Retention is fixed at 7 days, auto-pruned by the daily cron job.
type ActivityEvent struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	EventType string    `gorm:"not null;index" json:"eventType"` // "engine_start", "engine_complete", etc.
	Message   string    `gorm:"not null" json:"message"`         // Human-readable: "Engine run completed: evaluated 97, flagged 12"
	Metadata  string    `gorm:"type:text" json:"metadata"`       // Optional JSON for extra data
	CreatedAt time.Time `json:"createdAt"`
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
