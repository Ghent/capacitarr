package db

import "strings"

// IsMaskedKey checks if an API key string is a masked version (starts with "•").
func IsMaskedKey(key string) bool {
	return len(key) > 0 && strings.HasPrefix(key, "•")
}

// MaskAPIKey returns a masked version of the key, showing only the last 4 characters.
func MaskAPIKey(key string) string {
	if len(key) <= 4 {
		return "••••"
	}
	return strings.Repeat("•", len(key)-4) + key[len(key)-4:]
}

// ValidEffects defines the allowed rule effect values.
var ValidEffects = map[string]bool{
	"always_keep": true, "prefer_keep": true, "lean_keep": true,
	"lean_remove": true, "prefer_remove": true, "always_remove": true,
}

// ValidExecutionModes defines the allowed engine execution modes.
var ValidExecutionModes = map[string]bool{
	ModeDryRun: true, ModeApproval: true, ModeAuto: true,
}

// ValidTiebreakerMethods defines the allowed tiebreaker sort methods.
var ValidTiebreakerMethods = map[string]bool{
	TiebreakerSizeDesc: true, TiebreakerSizeAsc: true, TiebreakerNameAsc: true,
	TiebreakerOldestFirst: true, TiebreakerNewestFirst: true,
}

// ValidLogLevels defines the allowed log level values.
var ValidLogLevels = map[string]bool{
	LogLevelDebug: true, LogLevelInfo: true, LogLevelWarn: true, LogLevelError: true,
}

// ValidIntegrationTypes defines the allowed integration type values.
// NOTE: "overseerr" replaced by "seerr" in 2.0. See docs/plans/01-architecture/20260318T2119Z-capacitarr-2.0-plan.md.
var ValidIntegrationTypes = map[string]bool{
	"plex": true, "sonarr": true, "radarr": true, "lidarr": true,
	"readarr": true, "tautulli": true, "seerr": true,
	"jellyfin": true, "emby": true, "jellystat": true, "tracearr": true,
}

// ValidNotificationChannelTypes defines the allowed notification channel types.
var ValidNotificationChannelTypes = map[string]bool{
	"discord": true, "apprise": true,
}
