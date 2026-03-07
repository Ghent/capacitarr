package routes

// Integration type constants used across route handlers.
// These MUST match the keys in db.ValidIntegrationTypes and the
// IntegrationType constants in integrations/types.go.
const (
	intTypeSonarr    = "sonarr"
	intTypeRadarr    = "radarr"
	intTypeLidarr    = "lidarr"
	intTypeReadarr   = "readarr"
	intTypePlex      = "plex"
	intTypeTautulli  = "tautulli"
	intTypeOverseerr = "overseerr"
	intTypeJellyfin  = "jellyfin"
	intTypeEmby      = "emby"
)

// URL scheme constants for webhook/URL validation.
const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)
