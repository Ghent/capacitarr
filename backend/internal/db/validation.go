package db

import (
	"sort"
	"strconv"
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
// Used for validating both DiskGroup.Mode and PreferenceSet.DefaultDiskGroupMode.
var ValidExecutionModes = map[string]bool{
	ModeDryRun: true, ModeApproval: true, ModeAuto: true, ModeSunset: true,
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
// "overseerr" was renamed to "seerr" in 2.0.
var ValidIntegrationTypes = map[string]bool{
	"plex": true, "sonarr": true, "radarr": true, "lidarr": true,
	"readarr": true, "tautulli": true, "seerr": true,
	"jellyfin": true, "emby": true, "jellystat": true, "tracearr": true,
}

// ValidNotificationChannelTypes defines the allowed notification channel types.
var ValidNotificationChannelTypes = map[string]bool{
	"discord": true, "apprise": true,
}

// ValidBackupRetentionDays defines the allowed backup retention day values.
var ValidBackupRetentionDays = map[int]bool{
	3: true, 7: true, 14: true, 30: true,
}

// ValidNotificationLevels defines the allowed notification level values.
var ValidNotificationLevels = map[string]bool{
	"off": true, "critical": true, "important": true,
	"normal": true, "verbose": true,
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

// ItemKey is the hold identity: (integration_id, external_id), scoped by
// disk_group_id at the call site. Never title + type (spec §2 / §4.2).
func ItemKey(integrationID uint, externalID string) string {
	return strconv.FormatUint(uint64(integrationID), 10) + "\x00" + externalID
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
