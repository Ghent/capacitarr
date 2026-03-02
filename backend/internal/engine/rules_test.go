package engine

import (
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)

func TestStringMatch(t *testing.T) {
	tests := []struct {
		name     string
		actual   string
		cond     string
		expected string
		result   bool
	}{
		{"exact equal", "the matrix", "==", "the matrix", true},
		{"exact equal case sensitive", "The Matrix", "==", "the matrix", false},
		{"not equal true", "the matrix", "!=", "avatar", true},
		{"not equal false", "the matrix", "!=", "the matrix", false},
		{"contains match", "the matrix", "contains", "matrix", true},
		{"contains no match", "the matrix", "contains", "avatar", false},
		{"not contains match", "the matrix", "!contains", "avatar", true},
		{"not contains no match", "the matrix", "!contains", "matrix", false},
		{"contains empty string", "anything", "contains", "", true},
		{"not contains empty string", "anything", "!contains", "", false},
		{"equal empty strings", "", "==", "", true},
		{"unknown operator returns false", "test", "regex", "test", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := stringMatch(tc.actual, tc.cond, tc.expected)
			if result != tc.result {
				t.Errorf("stringMatch(%q, %q, %q) = %v, want %v",
					tc.actual, tc.cond, tc.expected, result, tc.result)
			}
		})
	}
}

func TestNumberMatch(t *testing.T) {
	tests := []struct {
		name     string
		actual   float64
		cond     string
		expected float64
		result   bool
	}{
		{"equal true", 5.0, "==", 5.0, true},
		{"equal false", 5.0, "==", 4.0, false},
		{"not equal true", 5.0, "!=", 4.0, true},
		{"not equal false", 5.0, "!=", 5.0, false},
		{"greater than true", 5.0, ">", 4.0, true},
		{"greater than false at boundary", 5.0, ">", 5.0, false},
		{"greater than or equal true at boundary", 5.0, ">=", 5.0, true},
		{"greater than or equal false", 4.0, ">=", 5.0, false},
		{"less than true", 4.0, "<", 5.0, true},
		{"less than false at boundary", 5.0, "<", 5.0, false},
		{"less than or equal true at boundary", 5.0, "<=", 5.0, true},
		{"less than or equal false", 6.0, "<=", 5.0, false},
		{"zero values equal", 0.0, "==", 0.0, true},
		{"negative values", -5.0, "<", 0.0, true},
		{"float precision", 7.5, ">", 7.0, true},
		{"unknown operator returns false", 5.0, "~", 5.0, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := numberMatch(tc.actual, tc.cond, tc.expected)
			if result != tc.result {
				t.Errorf("numberMatch(%v, %q, %v) = %v, want %v",
					tc.actual, tc.cond, tc.expected, result, tc.result)
			}
		})
	}
}

func TestMatchesRule_AllFieldTypes(t *testing.T) {
	now := time.Now()
	oneYearAgo := now.Add(-365 * 24 * time.Hour)

	baseItem := integrations.MediaItem{
		Title:          "The Matrix",
		QualityProfile: "HD-1080p",
		SeriesStatus:     "Ended",
		Genre:          "action, sci-fi",
		Rating:         8.5,
		SizeBytes:      10 * 1024 * 1024 * 1024, // 10 GB
		AddedAt:        &oneYearAgo,
		SeasonNumber:   3,
		EpisodeCount:   24,
		Monitored:      true,
		PlayCount:      5,
		IsRequested:    true,
		RequestCount:   2,
		Language:       "english",
		Type:           integrations.MediaTypeShow,
		Year:           1999,
		Tags:           []string{"anime", "classic"},
		IntegrationID:  1,
	}

	tests := []struct {
		name    string
		rule    db.CustomRule
		matched bool
	}{
		// Title field
		{"title == match", db.CustomRule{Field: "title", Operator: "==", Value: "the matrix"}, true},
		{"title == no match", db.CustomRule{Field: "title", Operator: "==", Value: "avatar"}, false},
		{"title contains match", db.CustomRule{Field: "title", Operator: "contains", Value: "matrix"}, true},
		{"title !contains match", db.CustomRule{Field: "title", Operator: "!contains", Value: "avatar"}, true},

		// Quality field
		{"quality == match", db.CustomRule{Field: "quality", Operator: "==", Value: "hd-1080p"}, true},
		{"quality != match", db.CustomRule{Field: "quality", Operator: "!=", Value: "4k"}, true},

		// SeriesStatus
		{"seriesstatus == match", db.CustomRule{Field: "seriesstatus", Operator: "==", Value: "ended"}, true},
		{"seriesstatus == no match", db.CustomRule{Field: "seriesstatus", Operator: "==", Value: "continuing"}, false},

		// Tag field (matches any tag in the slice)
		{"tag contains match", db.CustomRule{Field: "tag", Operator: "contains", Value: "anime"}, true},
		{"tag contains no match", db.CustomRule{Field: "tag", Operator: "contains", Value: "horror"}, false},
		{"tag !contains match", db.CustomRule{Field: "tag", Operator: "!contains", Value: "horror"}, true},

		// Genre field
		{"genre contains match", db.CustomRule{Field: "genre", Operator: "contains", Value: "sci-fi"}, true},
		{"genre == exact", db.CustomRule{Field: "genre", Operator: "==", Value: "action, sci-fi"}, true},

		// Rating field (numeric)
		{"rating > match", db.CustomRule{Field: "rating", Operator: ">", Value: "8.0"}, true},
		{"rating < no match", db.CustomRule{Field: "rating", Operator: "<", Value: "8.0"}, false},
		{"rating >= boundary", db.CustomRule{Field: "rating", Operator: ">=", Value: "8.5"}, true},
		{"rating invalid value", db.CustomRule{Field: "rating", Operator: ">", Value: "notanumber"}, false},

		// SizeBytes field (numeric)
		{"sizebytes > match", db.CustomRule{Field: "sizebytes", Operator: ">", Value: "5000000000"}, true},
		{"sizebytes < no match", db.CustomRule{Field: "sizebytes", Operator: "<", Value: "5000000000"}, false},

		// TimeInLibrary (computed from AddedAt)
		{"timeinlibrary > match", db.CustomRule{Field: "timeinlibrary", Operator: ">", Value: "300"}, true},
		{"timeinlibrary < no match", db.CustomRule{Field: "timeinlibrary", Operator: "<", Value: "100"}, false},

		// SeasonCount
		{"seasoncount == match", db.CustomRule{Field: "seasoncount", Operator: "==", Value: "3"}, true},
		{"seasoncount > match", db.CustomRule{Field: "seasoncount", Operator: ">", Value: "2"}, true},

		// EpisodeCount
		{"episodecount >= match", db.CustomRule{Field: "episodecount", Operator: ">=", Value: "24"}, true},
		{"episodecount < no match", db.CustomRule{Field: "episodecount", Operator: "<", Value: "10"}, false},

		// Monitored (boolean)
		{"monitored == true", db.CustomRule{Field: "monitored", Operator: "==", Value: "true"}, true},
		{"monitored == false no match", db.CustomRule{Field: "monitored", Operator: "==", Value: "false"}, false},

		// PlayCount
		{"playcount > match", db.CustomRule{Field: "playcount", Operator: ">", Value: "3"}, true},
		{"playcount == match", db.CustomRule{Field: "playcount", Operator: "==", Value: "5"}, true},

		// Requested (boolean)
		{"requested == true", db.CustomRule{Field: "requested", Operator: "==", Value: "true"}, true},
		{"requested == false no match", db.CustomRule{Field: "requested", Operator: "==", Value: "false"}, false},

		// RequestCount
		{"requestcount >= match", db.CustomRule{Field: "requestcount", Operator: ">=", Value: "2"}, true},

		// Language
		{"language == match", db.CustomRule{Field: "language", Operator: "==", Value: "english"}, true},
		{"language != match", db.CustomRule{Field: "language", Operator: "!=", Value: "japanese"}, true},

		// Type
		{"type == match", db.CustomRule{Field: "type", Operator: "==", Value: "show"}, true},
		{"type != match", db.CustomRule{Field: "type", Operator: "!=", Value: "movie"}, true},

		// Year
		{"year == match", db.CustomRule{Field: "year", Operator: "==", Value: "1999"}, true},
		{"year > match", db.CustomRule{Field: "year", Operator: ">", Value: "1990"}, true},
		{"year < no match", db.CustomRule{Field: "year", Operator: "<", Value: "1990"}, false},

		// Unknown field
		{"unknown field returns false", db.CustomRule{Field: "nonexistent", Operator: "==", Value: "anything"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := matchesRule(baseItem, tc.rule)
			if result != tc.matched {
				t.Errorf("matchesRule for %s %s %s = %v, want %v",
					tc.rule.Field, tc.rule.Operator, tc.rule.Value, result, tc.matched)
			}
		})
	}
}

func TestMatchesRule_NilAddedAt(t *testing.T) {
	item := integrations.MediaItem{AddedAt: nil}
	rule := db.CustomRule{Field: "timeinlibrary", Operator: ">", Value: "30"}

	result := matchesRule(item, rule)
	if result {
		t.Error("Expected false for timeinlibrary with nil AddedAt")
	}
}

func TestMatchesRule_TagNoTags(t *testing.T) {
	item := integrations.MediaItem{Tags: nil}
	rule := db.CustomRule{Field: "tag", Operator: "contains", Value: "anime"}

	result := matchesRule(item, rule)
	if result {
		t.Error("Expected false for tag match with no tags")
	}
}

func TestMatchesRule_TagNotContainsWithNoTags(t *testing.T) {
	item := integrations.MediaItem{Tags: nil}
	rule := db.CustomRule{Field: "tag", Operator: "!contains", Value: "anime"}

	// With nil tags, the loop doesn't execute, so returns false
	result := matchesRule(item, rule)
	if result {
		t.Error("Expected false for tag !contains with no tags (empty loop)")
	}
}

func TestApplyRules(t *testing.T) {
	now := time.Now()

	baseItem := integrations.MediaItem{
		Title:         "The Matrix",
		SeriesStatus:    "Ended",
		Rating:        8.5,
		AddedAt:       &now,
		IntegrationID: 1,
	}

	tests := []struct {
		name     string
		item     integrations.MediaItem
		rules    []db.CustomRule
		isAbs    bool
		modifier float64
	}{
		{
			name:     "No rules",
			item:     baseItem,
			rules:    []db.CustomRule{},
			isAbs:    false,
			modifier: 1.0,
		},
		{
			name: "Always keep by title (new effect field)",
			item: baseItem,
			rules: []db.CustomRule{
				{Field: "title", Operator: "==", Value: "the matrix", Effect: "always_keep"},
			},
			isAbs:    true,
			modifier: 0.0,
		},
		{
			name: "Prefer keep by rating",
			item: baseItem,
			rules: []db.CustomRule{
				{Field: "rating", Operator: ">", Value: "8.0", Effect: "prefer_keep"},
			},
			isAbs:    false,
			modifier: 0.2,
		},
		{
			name: "Lean keep modifier",
			item: baseItem,
			rules: []db.CustomRule{
				{Field: "rating", Operator: ">", Value: "8.0", Effect: "lean_keep"},
			},
			isAbs:    false,
			modifier: 0.5,
		},
		{
			name: "Lean remove modifier",
			item: baseItem,
			rules: []db.CustomRule{
				{Field: "seriesstatus", Operator: "==", Value: "ended", Effect: "lean_remove"},
			},
			isAbs:    false,
			modifier: 1.2,
		},
		{
			name: "Prefer remove modifier",
			item: baseItem,
			rules: []db.CustomRule{
				{Field: "seriesstatus", Operator: "==", Value: "ended", Effect: "prefer_remove"},
			},
			isAbs:    false,
			modifier: 2.0,
		},
		{
			name: "Always remove by seriesstatus",
			item: baseItem,
			rules: []db.CustomRule{
				{Field: "seriesstatus", Operator: "==", Value: "ended", Effect: "always_remove"},
			},
			isAbs:    false,
			modifier: 100.0,
		},
		{
			name: "Multiple cascading modifiers multiply",
			item: baseItem,
			rules: []db.CustomRule{
				{Field: "rating", Operator: ">", Value: "8.0", Effect: "prefer_keep"},        // ×0.2
				{Field: "title", Operator: "contains", Value: "matrix", Effect: "lean_keep"}, // ×0.5
			},
			isAbs:    false,
			modifier: 0.1, // 0.2 × 0.5
		},
		{
			name: "Lean keep + lean remove partially cancel",
			item: baseItem,
			rules: []db.CustomRule{
				{Field: "rating", Operator: ">", Value: "8.0", Effect: "lean_keep"},            // ×0.5
				{Field: "seriesstatus", Operator: "==", Value: "ended", Effect: "lean_remove"}, // ×1.2
			},
			isAbs:    false,
			modifier: 0.6, // 0.5 × 1.2
		},
		{
			name: "Always keep wins over always remove",
			item: baseItem,
			rules: []db.CustomRule{
				{Field: "title", Operator: "==", Value: "the matrix", Effect: "always_keep"},
				{Field: "seriesstatus", Operator: "==", Value: "ended", Effect: "always_remove"},
			},
			isAbs:    true,
			modifier: 0.0,
		},
		{
			name: "Always keep wins over prefer remove",
			item: baseItem,
			rules: []db.CustomRule{
				{Field: "title", Operator: "==", Value: "the matrix", Effect: "always_keep"},
				{Field: "rating", Operator: ">", Value: "5.0", Effect: "prefer_remove"},
			},
			isAbs:    true,
			modifier: 0.0,
		},
		{
			name: "Prefer keep + prefer remove = net protection",
			item: baseItem,
			rules: []db.CustomRule{
				{Field: "rating", Operator: ">", Value: "8.0", Effect: "prefer_keep"},            // ×0.2
				{Field: "seriesstatus", Operator: "==", Value: "ended", Effect: "prefer_remove"}, // ×2.0
			},
			isAbs:    false,
			modifier: 0.4, // 0.2 × 2.0
		},
		{
			name: "Stacked prefer remove",
			item: baseItem,
			rules: []db.CustomRule{
				{Field: "rating", Operator: ">", Value: "5.0", Effect: "prefer_remove"},          // ×2.0
				{Field: "seriesstatus", Operator: "==", Value: "ended", Effect: "prefer_remove"}, // ×2.0
			},
			isAbs:    false,
			modifier: 4.0, // 2.0 × 2.0
		},
		{
			name: "Non-matching rule has no effect",
			item: baseItem,
			rules: []db.CustomRule{
				{Field: "title", Operator: "==", Value: "avatar", Effect: "always_keep"},
			},
			isAbs:    false,
			modifier: 1.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isAbs, modifier, _, _ := applyRules(tc.item, tc.rules)
			if isAbs != tc.isAbs {
				t.Errorf("Expected absolute protect %v, got %v", tc.isAbs, isAbs)
			}
			if modifier < tc.modifier-0.01 || modifier > tc.modifier+0.01 {
				t.Errorf("Expected modifier %v, got %v", tc.modifier, modifier)
			}
		})
	}
}

func TestApplyRules_IntegrationIDFiltering(t *testing.T) {
	now := time.Now()
	integrationID1 := uint(1)
	integrationID2 := uint(2)

	item := integrations.MediaItem{
		Title:         "The Matrix",
		Rating:        8.5,
		AddedAt:       &now,
		IntegrationID: 1,
	}

	tests := []struct {
		name     string
		rules    []db.CustomRule
		isAbs    bool
		modifier float64
	}{
		{
			name: "Rule scoped to matching integration applies",
			rules: []db.CustomRule{
				{IntegrationID: &integrationID1, Field: "title", Operator: "==", Value: "the matrix", Effect: "always_keep"},
			},
			isAbs:    true,
			modifier: 0.0,
		},
		{
			name: "Rule scoped to different integration is skipped",
			rules: []db.CustomRule{
				{IntegrationID: &integrationID2, Field: "title", Operator: "==", Value: "the matrix", Effect: "always_keep"},
			},
			isAbs:    false,
			modifier: 1.0,
		},
		{
			name: "Global rule (nil integration_id) applies to all items",
			rules: []db.CustomRule{
				{IntegrationID: nil, Field: "title", Operator: "==", Value: "the matrix", Effect: "prefer_keep"},
			},
			isAbs:    false,
			modifier: 0.2,
		},
		{
			name: "Mixed: global rule applies, scoped rule skipped",
			rules: []db.CustomRule{
				{IntegrationID: nil, Field: "rating", Operator: ">", Value: "8.0", Effect: "lean_keep"},                      // ×0.5 (applies)
				{IntegrationID: &integrationID2, Field: "title", Operator: "==", Value: "the matrix", Effect: "always_keep"}, // skipped
			},
			isAbs:    false,
			modifier: 0.5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isAbs, modifier, _, _ := applyRules(item, tc.rules)
			if isAbs != tc.isAbs {
				t.Errorf("Expected absolute protect %v, got %v", tc.isAbs, isAbs)
			}
			if modifier < tc.modifier-0.01 || modifier > tc.modifier+0.01 {
				t.Errorf("Expected modifier %v, got %v", tc.modifier, modifier)
			}
		})
	}
}

func TestApplyRules_ReturnsFactors(t *testing.T) {
	now := time.Now()
	item := integrations.MediaItem{
		Title:         "The Matrix",
		Rating:        8.5,
		AddedAt:       &now,
		IntegrationID: 1,
	}

	rules := []db.CustomRule{
		{Field: "rating", Operator: ">", Value: "8.0", Effect: "prefer_keep"},
	}

	_, _, _, factors := applyRules(item, rules)
	if len(factors) != 1 {
		t.Fatalf("Expected 1 rule factor, got %d", len(factors))
	}
	if factors[0].Type != "rule" {
		t.Errorf("Expected factor type 'rule', got %q", factors[0].Type)
	}
}

func TestApplyRules_AlwaysKeepReturnsImmediately(t *testing.T) {
	now := time.Now()
	item := integrations.MediaItem{
		Title:         "The Matrix",
		Rating:        8.5,
		AddedAt:       &now,
		IntegrationID: 1,
	}

	// always_keep is first followed by modifiers that would change things
	rules := []db.CustomRule{
		{Field: "title", Operator: "==", Value: "the matrix", Effect: "always_keep"},
		{Field: "rating", Operator: ">", Value: "5.0", Effect: "prefer_remove"},
	}

	isAbs, modifier, reason, factors := applyRules(item, rules)
	if !isAbs {
		t.Error("Expected absolute protection")
	}
	if modifier != 0.0 {
		t.Errorf("Expected modifier 0.0, got %v", modifier)
	}
	if reason == "" {
		t.Error("Expected non-empty reason for always_keep")
	}
	if len(factors) != 1 {
		t.Errorf("Expected 1 factor for always_keep, got %d", len(factors))
	}
}

func TestLegacyEffect(t *testing.T) {
	tests := []struct {
		name      string
		ruleType  string
		intensity string
		expected  string
	}{
		{"protect absolute", "protect", "absolute", "always_keep"},
		{"protect strong", "protect", "strong", "prefer_keep"},
		{"protect slight", "protect", "slight", "lean_keep"},
		{"protect default", "protect", "", "lean_keep"},
		{"target absolute", "target", "absolute", "always_remove"},
		{"target strong", "target", "strong", "prefer_remove"},
		{"target slight", "target", "slight", "lean_remove"},
		{"target default", "target", "", "lean_remove"},
		{"empty type defaults to lean_keep", "", "", "lean_keep"},
		{"unknown type defaults to lean_keep", "unknown", "unknown", "lean_keep"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := legacyEffect(tc.ruleType, tc.intensity)
			if result != tc.expected {
				t.Errorf("legacyEffect(%q, %q) = %q, want %q",
					tc.ruleType, tc.intensity, result, tc.expected)
			}
		})
	}
}

func TestApplyRules_LegacyFallback(t *testing.T) {
	now := time.Now()
	item := integrations.MediaItem{
		Title:         "The Matrix",
		Rating:        8.5,
		AddedAt:       &now,
		IntegrationID: 1,
	}

	tests := []struct {
		name     string
		rule     db.CustomRule
		isAbs    bool
		modifier float64
	}{
		{
			name:     "Legacy absolute protect",
			rule:     db.CustomRule{Type: "protect", Field: "title", Operator: "==", Value: "the matrix", Intensity: "absolute"},
			isAbs:    true,
			modifier: 0.0,
		},
		{
			name:     "Legacy strong target",
			rule:     db.CustomRule{Type: "target", Field: "rating", Operator: ">", Value: "8.0", Intensity: "strong"},
			isAbs:    false,
			modifier: 2.0,
		},
		{
			name:     "Legacy default protect",
			rule:     db.CustomRule{Type: "protect", Field: "rating", Operator: ">", Value: "8.0"},
			isAbs:    false,
			modifier: 0.5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isAbs, modifier, _, _ := applyRules(item, []db.CustomRule{tc.rule})
			if isAbs != tc.isAbs {
				t.Errorf("Expected absolute protect %v, got %v", tc.isAbs, isAbs)
			}
			if modifier < tc.modifier-0.01 || modifier > tc.modifier+0.01 {
				t.Errorf("Expected modifier %v, got %v", tc.modifier, modifier)
			}
		})
	}
}
