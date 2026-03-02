package integrations

import "time"

// IntegrationType represents the type of service integration
type IntegrationType string

const (
	IntegrationTypePlex      IntegrationType = "plex"
	IntegrationTypeSonarr    IntegrationType = "sonarr"
	IntegrationTypeRadarr    IntegrationType = "radarr"
	IntegrationTypeTautulli  IntegrationType = "tautulli"
	IntegrationTypeOverseerr IntegrationType = "overseerr"
	IntegrationTypeLidarr    IntegrationType = "lidarr"
)

// Integration defines the common interface all service integrations implement
type Integration interface {
	// TestConnection verifies the URL + API key are valid
	TestConnection() error
	// GetDiskSpace returns disk usage info from the service
	GetDiskSpace() ([]DiskSpace, error)
	// GetRootFolders returns the configured media root folder paths
	GetRootFolders() ([]string, error)
	// GetMediaItems returns all media items managed by the service
	GetMediaItems() ([]MediaItem, error)
	// DeleteMediaItem removes the item from the service and disk
	DeleteMediaItem(item MediaItem) error
}

// DiskSpace represents disk usage reported by a service
type DiskSpace struct {
	Path       string `json:"path"`
	TotalBytes int64  `json:"totalBytes"`
	FreeBytes  int64  `json:"freeBytes"`
}

// MediaItem represents a single media item from any service
type MediaItem struct {
	// Core identity
	ExternalID    string    `json:"externalId"`    // ID from the source service
	IntegrationID uint      `json:"integrationId"` // FK to IntegrationConfig
	Type          MediaType `json:"type"`          // movie, show, season, episode, album
	Title         string    `json:"title"`
	Year          int       `json:"year,omitempty"`
	SizeBytes     int64     `json:"sizeBytes"`
	Path          string    `json:"path"` // File path on disk

	// TV-specific
	SeasonNumber int    `json:"seasonNumber,omitempty"`
	EpisodeCount int    `json:"episodeCount,omitempty"`
	ShowTitle    string `json:"showTitle,omitempty"`
	ShowStatus   string `json:"showStatus,omitempty"` // continuing, ended

	// Quality / metadata
	QualityProfile string  `json:"qualityProfile,omitempty"`
	Rating         float64 `json:"rating,omitempty"`
	Genre          string  `json:"genre,omitempty"`
	Monitored      bool    `json:"monitored"`

	// Watch data (from Plex)
	PlayCount  int        `json:"playCount,omitempty"`
	LastPlayed *time.Time `json:"lastPlayed,omitempty"`
	AddedAt    *time.Time `json:"addedAt,omitempty"`

	// Tags
	Tags []string `json:"tags,omitempty"`

	// Enrichment data (from Tautulli, Overseerr, etc.)
	IsRequested  bool   `json:"isRequested,omitempty"`  // Overseerr: was this item user-requested?
	RequestedBy  string `json:"requestedBy,omitempty"`  // Overseerr: who requested it
	RequestCount int    `json:"requestCount,omitempty"` // Overseerr: number of requests
	TMDbID       int    `json:"tmdbId,omitempty"`       // TMDb ID for cross-referencing Overseerr
	Language     string `json:"language,omitempty"`     // Original language from *arr
}

// MediaType represents different forms of media content
type MediaType string

const (
	MediaTypeMovie   MediaType = "movie"
	MediaTypeShow    MediaType = "show"
	MediaTypeSeason  MediaType = "season"
	MediaTypeEpisode MediaType = "episode"
	MediaTypeArtist  MediaType = "artist"
	MediaTypeBook    MediaType = "book"
)

const (
	IntegrationTypeReadarr  IntegrationType = "readarr"
	IntegrationTypeJellyfin IntegrationType = "jellyfin"
	IntegrationTypeEmby     IntegrationType = "emby"
)

// NameValue is a simple label/value pair used for rule value options.
type NameValue struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// RuleValueFetcher defines methods for fetching autocomplete values
// from *arr services (quality profiles, tags, languages).
// Not all *arr clients support all methods — callers should check errors.
type RuleValueFetcher interface {
	// GetQualityProfiles returns all quality profiles from the service.
	GetQualityProfiles() ([]NameValue, error)
	// GetTags returns all tags from the service.
	GetTags() ([]NameValue, error)
	// GetLanguages returns all languages from the service.
	// Returns nil, nil if the service does not support language lookup.
	GetLanguages() ([]NameValue, error)
}
