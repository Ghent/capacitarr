package db

import (
	"sort"
	"strings"
)

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

// MediaKey builds a composite lookup key from a media name and type. This is
// the canonical key format used throughout the codebase for deduplication,
// snooze checks, approval queue matching, and audit log upserts. Using a
// single function ensures all call sites produce identical keys.
//
// The separator (\x00) is a NUL byte that cannot appear in user-visible media
// titles or type strings, eliminating collision risk from titles that contain
// common delimiter characters like "|" or ":".
func MediaKey(name, mediaType string) string {
	return name + "\x00" + mediaType
}

// FormatValidKeys returns a sorted, comma-separated string of keys from a
// validation map. Use this in error messages instead of hardcoding the list
// of valid values — when the map is updated, the error messages update too.
func FormatValidKeys(m map[string]bool) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
