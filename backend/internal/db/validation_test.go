package db_test

import (
	"strings"
	"testing"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)

// TestValidIntegrationTypes_MatchesFactoryRegistry verifies that the
// ValidIntegrationTypes validation map stays in sync with the integration
// factory registry. Every factory-registered type must be in the validation
// map and vice versa — a drift means users can't create a valid integration
// type through the API (like the jellystat bug) or validation accepts a type
// that has no factory.
func TestValidIntegrationTypes_MatchesFactoryRegistry(t *testing.T) {
	// Ensure all factories are registered (idempotent; safe to call multiple times)
	integrations.RegisterAllFactories()

	registeredTypes := integrations.RegisteredTypes()

	// Every factory-registered type must be in ValidIntegrationTypes
	for _, intType := range registeredTypes {
		if !db.ValidIntegrationTypes[intType] {
			t.Errorf("integration type %q is registered in the factory but missing from db.ValidIntegrationTypes", intType)
		}
	}

	// Every ValidIntegrationTypes entry must have a factory
	for intType := range db.ValidIntegrationTypes {
		if !integrations.HasFactory(intType) {
			t.Errorf("integration type %q is in db.ValidIntegrationTypes but has no registered factory", intType)
		}
	}

	// Counts must match (catches duplicates or other anomalies)
	if len(db.ValidIntegrationTypes) != len(registeredTypes) {
		t.Errorf("ValidIntegrationTypes has %d entries but factory registry has %d", len(db.ValidIntegrationTypes), len(registeredTypes))
	}
}

// TestFormatValidKeys verifies that FormatValidKeys produces a sorted,
// comma-separated string of map keys. This utility is used in error messages
// to avoid hardcoding lists of valid values.
func TestFormatValidKeys(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]bool
		want string
	}{
		{
			name: "empty map",
			m:    map[string]bool{},
			want: "",
		},
		{
			name: "single key",
			m:    map[string]bool{"alpha": true},
			want: "alpha",
		},
		{
			name: "multiple keys sorted",
			m:    map[string]bool{"charlie": true, "alpha": true, "bravo": true},
			want: "alpha, bravo, charlie",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := db.FormatValidKeys(tc.m)
			if got != tc.want {
				t.Errorf("FormatValidKeys() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestMediaKey verifies the canonical media key function used for deduplication,
// snooze checks, approval queue matching, and audit log upserts throughout the
// codebase.
func TestMediaKey(t *testing.T) {
	tests := []struct {
		name      string
		mediaName string
		mediaType string
	}{
		{name: "basic", mediaName: "Firefly", mediaType: "show"},
		{name: "movie", mediaName: "Serenity", mediaType: "movie"},
		{name: "pipe in title", mediaName: "Foo|Bar", mediaType: "show"},
		{name: "colon in title", mediaName: "Star Trek: Picard", mediaType: "show"},
		{name: "empty name", mediaName: "", mediaType: "movie"},
		{name: "unicode", mediaName: "日本語タイトル", mediaType: "movie"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key := db.MediaKey(tc.mediaName, tc.mediaType)

			// Key must contain the NUL separator
			if !strings.Contains(key, "\x00") {
				t.Errorf("MediaKey(%q, %q) = %q — missing NUL separator", tc.mediaName, tc.mediaType, key)
			}

			// Same inputs must produce the same key (deterministic)
			if db.MediaKey(tc.mediaName, tc.mediaType) != key {
				t.Errorf("MediaKey is not deterministic for (%q, %q)", tc.mediaName, tc.mediaType)
			}
		})
	}

	// Different (name, type) pairs must produce different keys
	if db.MediaKey("Firefly", "show") == db.MediaKey("Firefly", "movie") {
		t.Error("MediaKey should produce different keys for different media types")
	}
	if db.MediaKey("Firefly", "show") == db.MediaKey("Serenity", "show") {
		t.Error("MediaKey should produce different keys for different media names")
	}
}

// TestFormatValidKeys_RealMaps verifies that FormatValidKeys produces
// non-empty, reasonable output for each real validation map. This catches
// accidentally emptying a map.
func TestFormatValidKeys_RealMaps(t *testing.T) {
	maps := map[string]map[string]bool{
		"ValidEffects":                  db.ValidEffects,
		"ValidExecutionModes":           db.ValidExecutionModes,
		"ValidTiebreakerMethods":        db.ValidTiebreakerMethods,
		"ValidLogLevels":                db.ValidLogLevels,
		"ValidIntegrationTypes":         db.ValidIntegrationTypes,
		"ValidNotificationChannelTypes": db.ValidNotificationChannelTypes,
	}

	for name, m := range maps {
		t.Run(name, func(t *testing.T) {
			result := db.FormatValidKeys(m)
			if result == "" {
				t.Errorf("FormatValidKeys(%s) returned empty string — map has %d entries", name, len(m))
			}
			// Result should contain at least one key (no trailing/leading commas)
			if strings.HasPrefix(result, ",") || strings.HasSuffix(result, ",") {
				t.Errorf("FormatValidKeys(%s) has leading/trailing comma: %q", name, result)
			}
		})
	}
}
