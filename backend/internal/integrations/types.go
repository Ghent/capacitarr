package integrations

import "time"

// IntegrationType represents the type of service integration
type IntegrationType string

const (
	// IntegrationTypePlex identifies a Plex Media Server integration.
	IntegrationTypePlex IntegrationType = "plex"
	// IntegrationTypeSonarr identifies a Sonarr (TV series) integration.
	IntegrationTypeSonarr IntegrationType = "sonarr"
	// IntegrationTypeRadarr identifies a Radarr (movies) integration.
	IntegrationTypeRadarr IntegrationType = "radarr"
	// IntegrationTypeTautulli identifies a Tautulli (Plex analytics) integration.
	IntegrationTypeTautulli IntegrationType = "tautulli"
	// IntegrationTypeSeerr identifies a Seerr (media requests) integration.
	// Compatible with Overseerr, Jellyseerr, and Seerr instances.
	IntegrationTypeSeerr IntegrationType = "seerr"
	// IntegrationTypeLidarr identifies a Lidarr (music) integration.
	IntegrationTypeLidarr IntegrationType = "lidarr"
	// IntegrationTypeReadarr identifies a Readarr (books) integration.
	IntegrationTypeReadarr IntegrationType = "readarr"
	// IntegrationTypeJellyfin identifies a Jellyfin media server integration.
	IntegrationTypeJellyfin IntegrationType = "jellyfin"
	// IntegrationTypeEmby identifies an Emby media server integration.
	IntegrationTypeEmby IntegrationType = "emby"
	// IntegrationTypeJellystat identifies a Jellystat (Jellyfin analytics) integration.
	// Jellystat is to Jellyfin what Tautulli is to Plex — an analytics supplement
	// that provides richer watch history, per-user stats, and activity tracking.
	IntegrationTypeJellystat IntegrationType = "jellystat"
)

// ============================================================================
// Capability Interfaces (2.0)
//
// Each integration implements only the interfaces it supports. Client creation
// uses CreateClient() via the factory registry — no switch-statement wiring.
//
// IMPORTANT: Only *arr integrations implement MediaSource — enrichment sources
// (media servers, analytics, request managers) must NEVER provide media items
// into the evaluation pool. MediaSource implies the integration also implements
// MediaDeleter and DiskReporter (i.e., it authoritatively manages content).
//
// Sonarr, Radarr, Lidarr, Readarr: Connectable + MediaSource + DiskReporter + MediaDeleter + RuleValueFetcher
// Plex:                             Connectable + WatchDataProvider + WatchlistProvider
// Tautulli:                         Connectable (enriches via TautulliEnricher, not WatchDataProvider)
// Seerr:                            Connectable + RequestProvider
// Jellyfin, Emby:                   Connectable + WatchDataProvider + WatchlistProvider
// Jellystat:                        Connectable (enriches via JellystatEnricher, not WatchDataProvider)
// ============================================================================

// Connectable is implemented by any integration that can verify its connection.
type Connectable interface {
	TestConnection() error
}

// MediaSource is implemented by integrations that can list managed media items.
type MediaSource interface {
	GetMediaItems() ([]MediaItem, error)
}

// DiskReporter is implemented by integrations that can report disk usage.
type DiskReporter interface {
	GetDiskSpace() ([]DiskSpace, error)
	GetRootFolders() ([]string, error)
}

// MediaDeleter is implemented by integrations that can delete media items.
type MediaDeleter interface {
	DeleteMediaItem(item MediaItem) error
}

// WatchDataProvider is implemented by integrations that can supply bulk watch data.
// Returns a map keyed by TMDb ID for deterministic matching against *arr items.
// Implementations resolve any internal setup (e.g. admin user ID, TMDb ID
// extraction from ProviderIds/GUIDs) internally.
type WatchDataProvider interface {
	GetBulkWatchData() (map[int]*WatchData, error)
}

// WatchData holds watch statistics for a single media item.
type WatchData struct {
	PlayCount  int
	LastPlayed *time.Time
	Users      []string // Users who watched this item
}

// RequestProvider is implemented by integrations that track media requests.
type RequestProvider interface {
	GetRequestedMedia() ([]MediaRequest, error)
}

// MediaRequest represents a user request for media content.
type MediaRequest struct {
	MediaType   string // "movie" or "tv"
	TMDbID      int
	Status      int // 1=pending, 2=approved, 3=declined, 4=available
	RequestedBy string
}

// WatchlistProvider is implemented by integrations that can report watchlist/favorites.
// Returns a map keyed by TMDb ID for deterministic matching against *arr items.
type WatchlistProvider interface {
	GetWatchlistItems() (map[int]bool, error)
}

// CollectionResolver is implemented by integrations that can resolve which
// other items share a collection with a given item. When collection deletion
// is enabled on an integration, the poller calls this to expand a single
// deletion candidate into all members of its collection.
type CollectionResolver interface {
	// ResolveCollectionMembers returns all items in the same collection(s) as
	// the given item, INCLUDING the item itself. Returns nil/empty if the item
	// has no collection membership or collection deletion is not applicable.
	ResolveCollectionMembers(item MediaItem) ([]MediaItem, error)
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
	Path          string    `json:"path"`                // File path on disk
	PosterURL     string    `json:"posterUrl,omitempty"` // Poster image URL (from *arr's images array, coverType=poster)

	// TV-specific
	SeasonNumber int    `json:"seasonNumber,omitempty"`
	EpisodeCount int    `json:"episodeCount,omitempty"`
	ShowTitle    string `json:"showTitle,omitempty"`
	SeriesStatus string `json:"seriesStatus,omitempty"` // continuing, ended

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
	IsRequested        bool     `json:"isRequested,omitempty"`    // Seerr: was this item user-requested?
	RequestedBy        string   `json:"requestedBy,omitempty"`    // Seerr: who requested it
	RequestCount       int      `json:"requestCount,omitempty"`   // Seerr: number of requests
	TMDbID             int      `json:"tmdbId,omitempty"`         // TMDb ID for cross-referencing Seerr
	Language           string   `json:"language,omitempty"`       // Original language from *arr
	Collections        []string `json:"collections,omitempty"`    // Plex collection membership
	WatchedByUsers     []string `json:"watchedByUsers,omitempty"` // Users who watched (from Tautulli)
	WatchedByRequestor bool     `json:"watchedByRequestor"`       // Cross-ref: requestor watched it
	OnWatchlist        bool     `json:"onWatchlist,omitempty"`    // Item is on a user's watchlist or favorited
}

// MediaType represents different forms of media content
type MediaType string

const (
	// MediaTypeMovie represents a film or movie entry.
	MediaTypeMovie MediaType = "movie"
	// MediaTypeShow represents a TV series.
	MediaTypeShow MediaType = "show"
	// MediaTypeSeason represents a single season of a TV series.
	MediaTypeSeason MediaType = "season"
	// MediaTypeEpisode represents a single episode of a TV series.
	MediaTypeEpisode MediaType = "episode"
	// MediaTypeArtist represents a music artist.
	MediaTypeArtist MediaType = "artist"
	// MediaTypeBook represents a book or audiobook.
	MediaTypeBook MediaType = "book"
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
