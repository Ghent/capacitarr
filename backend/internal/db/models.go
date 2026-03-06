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
	Password   string `gorm:"not null"` // bcrypt hash
	APIKey     string `gorm:"index"`    // SHA-256 hash (sha256:<hex>) or legacy plaintext
	APIKeyHint string // Last 4 characters of the plaintext key for identification
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// LibraryHistory stores historical capacity logs
type LibraryHistory struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	Timestamp     time.Time `gorm:"index;not null" json:"timestamp"`
	TotalCapacity int64     `gorm:"not null" json:"totalCapacity"`
	UsedCapacity  int64     `gorm:"not null" json:"usedCapacity"`
	Resolution    string    `gorm:"index;not null" json:"resolution"`   // "raw", "hourly", "daily", "weekly"
	DiskGroupID   *uint     `gorm:"index" json:"diskGroupId,omitempty"` // FK to DiskGroup (ON DELETE CASCADE)
	CreatedAt     time.Time `json:"createdAt"`
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
	Type           string     `gorm:"not null;index" json:"type"` // plex, sonarr, radarr
	Name           string     `gorm:"not null" json:"name"`       // User-friendly name
	URL            string     `gorm:"not null" json:"url"`
	APIKey         string     `gorm:"not null" json:"apiKey"` // API key or Plex token
	Enabled        bool       `gorm:"default:true" json:"enabled"`
	MediaSizeBytes int64      `json:"mediaSizeBytes"` // Total media file size
	MediaCount     int        `json:"mediaCount"`     // Number of media items
	LastSync       *time.Time `json:"lastSync,omitempty"`
	LastError      string     `json:"lastError,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// DiskGroup represents a physical disk/mount point shared by multiple services
type DiskGroup struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	MountPath    string    `gorm:"uniqueIndex;not null" json:"mountPath"`
	TotalBytes   int64     `gorm:"not null" json:"totalBytes"`
	UsedBytes    int64     `gorm:"not null" json:"usedBytes"`
	ThresholdPct float64   `gorm:"default:85" json:"thresholdPct"` // Clean up at this %
	TargetPct    float64   `gorm:"default:75" json:"targetPct"`    // Free down to this %
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// PreferenceSet stores the global weights for the scoring engine (0-10 scale)
type PreferenceSet struct {
	ID                    uint      `gorm:"primarykey" json:"id"`
	LogLevel              string    `gorm:"default:'info';not null" json:"logLevel"`          // "debug", "info", "warn", "error"
	AuditLogRetentionDays int       `gorm:"default:30;not null" json:"auditLogRetentionDays"` // 0 = forever, else days
	PollIntervalSeconds   int       `gorm:"default:300;not null" json:"pollIntervalSeconds"`  // minimum 30, default 300 (5 min)
	WatchHistoryWeight    int       `gorm:"default:10" json:"watchHistoryWeight"`             // High default
	LastWatchedWeight     int       `gorm:"default:8" json:"lastWatchedWeight"`
	FileSizeWeight        int       `gorm:"default:6" json:"fileSizeWeight"`
	RatingWeight          int       `gorm:"default:5" json:"ratingWeight"`
	TimeInLibraryWeight   int       `gorm:"default:4" json:"timeInLibraryWeight"`
	SeriesStatusWeight    int       `gorm:"default:3" json:"seriesStatusWeight"`
	ExecutionMode         string    `gorm:"default:'dry-run';not null" json:"executionMode"`      // "dry-run", "approval", "auto"
	TiebreakerMethod      string    `gorm:"default:'size_desc';not null" json:"tiebreakerMethod"` // "size_desc", "size_asc", "name_asc", "oldest_first", "newest_first"
	DeletionsEnabled      bool      `gorm:"default:true;not null" json:"deletionsEnabled"`        // Safety guard: actual deletions only when true
	SnoozeDurationHours   int       `gorm:"default:24;not null" json:"snoozeDurationHours"`       // Hours to snooze rejected items before re-evaluation
	CheckForUpdates       bool      `gorm:"default:true;not null" json:"checkForUpdates"`         // Enable outbound update checks
	UpdatedAt             time.Time `json:"updatedAt"`
}

// CustomRule stores custom rules that influence media scoring via keep/remove effects.
// The deprecated Type and Intensity fields have been removed in the schema refactor.
type CustomRule struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	IntegrationID *uint     `gorm:"index" json:"integrationId"` // FK to IntegrationConfig; nil = legacy global rule
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

// ApprovalQueueItem represents an item in the approval queue (state machine).
// Items flow through: pending → approved → (deletion) OR pending → rejected (snoozed).
type ApprovalQueueItem struct {
	ID            uint       `gorm:"primarykey" json:"id"`
	MediaName     string     `gorm:"index;not null" json:"mediaName"`
	MediaType     string     `gorm:"not null" json:"mediaType"`                          // movie, show, season, episode, artist, album, book
	Reason        string     `gorm:"not null" json:"reason"`                             // e.g. "Score: 0.85 (WatchHistory: 1.0, Size: 0.5)"
	ScoreDetails  string     `gorm:"type:text" json:"scoreDetails"`                      // JSON-encoded []ScoreFactor
	SizeBytes     int64      `gorm:"not null;default:0" json:"sizeBytes"`                // File size in bytes
	PosterURL     string     `gorm:"not null;default:''" json:"posterUrl"`               // Poster image URL from *arr
	IntegrationID uint       `gorm:"not null" json:"integrationId"`                      // FK to IntegrationConfig (required)
	ExternalID    string     `gorm:"not null;default:''" json:"externalId"`              // External ID in the integration
	Status        string     `gorm:"not null;default:'pending'" json:"status"`           // pending, approved, rejected
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
	ActionDryRun    = "dry_run"
	ActionDryDelete = "dry_delete"
)

// AuditLogEntry stores a permanent record of deletions and dry-runs.
// This table is append-only — entries are never modified after creation.
type AuditLogEntry struct {
	ID            uint      `gorm:"primarykey" json:"id"`
	MediaName     string    `gorm:"index;not null" json:"mediaName"`
	MediaType     string    `gorm:"not null" json:"mediaType"`
	Reason        string    `gorm:"not null" json:"reason"`        // e.g. "Score: 0.85 (WatchHistory: 1.0, Size: 0.5)"
	ScoreDetails  string    `gorm:"type:text" json:"scoreDetails"` // JSON-encoded []ScoreFactor
	Action        string    `gorm:"not null" json:"action"`        // "deleted", "dry_run", "dry_delete"
	SizeBytes     int64     `gorm:"not null;default:0" json:"sizeBytes"`
	IntegrationID *uint     `json:"integrationId,omitempty" gorm:"column:integration_id"` // FK to IntegrationConfig (nullable — preserved on integration delete)
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

// NotificationConfig stores a configured notification channel (Discord, Slack, or in-app).
type NotificationConfig struct {
	ID         uint   `gorm:"primarykey" json:"id"`
	Type       string `gorm:"not null" json:"type"` // "discord", "slack", "inapp"
	Name       string `gorm:"not null" json:"name"` // User-friendly name
	WebhookURL string `json:"webhookUrl,omitempty"` // Discord/Slack webhook URL
	Enabled    bool   `gorm:"default:true" json:"enabled"`
	// Event subscriptions — which events trigger this channel
	OnThresholdBreach  bool      `gorm:"default:true" json:"onThresholdBreach"`
	OnDeletionExecuted bool      `gorm:"default:true" json:"onDeletionExecuted"`
	OnEngineError      bool      `gorm:"default:true" json:"onEngineError"`
	OnEngineComplete   bool      `gorm:"default:false" json:"onEngineComplete"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// InAppNotification stores a notification displayed in the UI notification panel.
type InAppNotification struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	Title     string    `gorm:"not null" json:"title"`
	Message   string    `gorm:"not null" json:"message"`
	Severity  string    `gorm:"not null;default:'info'" json:"severity"` // "info", "warning", "error", "success"
	Read      bool      `gorm:"default:false" json:"read"`
	EventType string    `gorm:"not null" json:"eventType"` // "threshold_breach", "deletion_executed", etc.
	CreatedAt time.Time `json:"createdAt"`
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
