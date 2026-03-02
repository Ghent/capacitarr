package db

import (
	"time"
)

// AuthConfig stores credentials for web UI sessions.
// The Password field stores a bcrypt hash. The APIKey field stores a SHA-256
// hash (prefixed with "sha256:") — plaintext keys are shown once on generation
// and never stored. Legacy plaintext keys are transparently upgraded on first use.
type AuthConfig struct {
	ID        uint   `gorm:"primarykey"`
	Username  string `gorm:"uniqueIndex;not null"`
	Password  string `gorm:"not null"` // bcrypt hash
	APIKey    string `gorm:"index"`    // SHA-256 hash (sha256:<hex>) or legacy plaintext
	CreatedAt time.Time
	UpdatedAt time.Time
}

// LibraryHistory stores historical capacity logs
type LibraryHistory struct {
	ID            uint      `gorm:"primarykey"`
	Timestamp     time.Time `gorm:"index;not null"`
	TotalCapacity int64     `gorm:"not null"`
	UsedCapacity  int64     `gorm:"not null"`
	Resolution    string    `gorm:"index;not null"` // "raw", "hourly", "daily", "weekly"
	DiskGroupID   *uint     `gorm:"index"`          // Optional FK to DiskGroup
	CreatedAt     time.Time
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
	AvailabilityWeight    int       `gorm:"default:3" json:"availabilityWeight"`
	ExecutionMode         string    `gorm:"default:'dry-run';not null" json:"executionMode"`      // "dry-run", "approval", "auto"
	TiebreakerMethod      string    `gorm:"default:'size_desc';not null" json:"tiebreakerMethod"` // "size_desc", "size_asc", "name_asc", "oldest_first", "newest_first"
	UpdatedAt             time.Time `json:"updatedAt"`
}

// ProtectionRule stores custom rules that influence media scoring via keep/remove effects
type ProtectionRule struct {
	ID            uint   `gorm:"primarykey" json:"id"`
	IntegrationID *uint  `gorm:"index" json:"integrationId"` // FK to IntegrationConfig; nil = legacy global rule
	Field         string `gorm:"not null" json:"field"`      // e.g. "quality", "tag", "rating"
	Operator      string `gorm:"not null" json:"operator"`   // e.g. "==", "contains", ">"
	Value         string `gorm:"not null" json:"value"`      // e.g. "4K", "anime", "7.5"
	Effect        string `gorm:"not null" json:"effect"`     // e.g. "always_keep", "prefer_remove"
	// Deprecated — kept for migration compatibility
	Type      string    `json:"type,omitempty"`
	Intensity string    `json:"intensity,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AuditLog stores a history of what was deleted, when, and why
type AuditLog struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	MediaName    string    `gorm:"index;not null" json:"mediaName"`
	MediaType    string    `gorm:"not null" json:"mediaType"`
	Reason       string    `gorm:"not null" json:"reason"`        // e.g. "Score: 0.85 (WatchHistory: 1.0, Size: 0.5)" — backward compat
	ScoreDetails string    `gorm:"type:text" json:"scoreDetails"` // JSON-encoded []ScoreFactor
	Action       string    `gorm:"not null" json:"action"`        // "Deleted", "Dry-Run"
	SizeBytes    int64     `json:"sizeBytes"`
	CreatedAt    time.Time `json:"createdAt"`
}

// EngineRunStats stores one row per engine evaluation cycle, persisting metrics
// across container restarts so the UI always shows the latest run's stats.
type EngineRunStats struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	RunAt         time.Time `gorm:"index;not null" json:"runAt"`
	Evaluated     int       `gorm:"not null;default:0" json:"evaluated"`
	Flagged       int       `gorm:"not null;default:0" json:"flagged"`
	FreedBytes    int64     `gorm:"not null;default:0" json:"freedBytes"`
	ExecutionMode string    `gorm:"not null;default:'dry-run'" json:"executionMode"`
	DurationMs    int64     `gorm:"not null;default:0" json:"durationMs"`
	ErrorMessage  string    `json:"errorMessage,omitempty"`
}

// LifetimeStats stores cumulative counters that persist across container restarts
// and are NOT cleared by the "Clear All Scraped Data" action. Singleton row (id=1).
type LifetimeStats struct {
	ID                  uint  `gorm:"primarykey" json:"id"`
	TotalBytesReclaimed int64 `gorm:"not null;default:0" json:"totalBytesReclaimed"`
	TotalItemsRemoved   int   `gorm:"not null;default:0" json:"totalItemsRemoved"`
	TotalEngineRuns     int   `gorm:"not null;default:0" json:"totalEngineRuns"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}
