package engine

import (
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)

// intIDPtr returns a pointer to a uint for use in test rule IntegrationID fields.
func intIDPtr(id uint) *uint { return &id }

func TestStringMatch(t *testing.T) {
	tests := []struct {
		name     string
		actual   string
		cond     string
		expected string
		result   bool
	}{
		{"exact equal", "serenity", "==", "serenity", true},
		{"exact equal case sensitive", "Serenity", "==", "serenity", false},
		{"not equal true", "serenity", "!=", "other", true},
		{"not equal false", "serenity", "!=", "serenity", false},
		{"contains match", "serenity", "contains", "seren", true},
		{"contains no match", "serenity", "contains", "other", false},
		{"not contains match", "serenity", "!contains", "other", true},
		{"not contains no match", "serenity", "!contains", "seren", false},
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
		Title:          "Serenity",
		QualityProfile: "HD-1080p",
		SeriesStatus:   "Ended",
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
		Collections:    []string{"Sci-Fi Classics", "Serenity Saga"},
		IntegrationID:  1,
	}

	tests := []struct {
		name    string
		rule    db.CustomRule
		matched bool
	}{
		// Title field
		{"title == match", db.CustomRule{Enabled: true, Field: "title", Operator: "==", Value: "serenity"}, true},
		{"title == no match", db.CustomRule{Enabled: true, Field: "title", Operator: "==", Value: "other"}, false},
		{"title contains match", db.CustomRule{Enabled: true, Field: "title", Operator: "contains", Value: "seren"}, true},
		{"title !contains match", db.CustomRule{Enabled: true, Field: "title", Operator: "!contains", Value: "other"}, true},

		// Quality field
		{"quality == match", db.CustomRule{Enabled: true, Field: "quality", Operator: "==", Value: "hd-1080p"}, true},
		{"quality != match", db.CustomRule{Enabled: true, Field: "quality", Operator: "!=", Value: "4k"}, true},

		// SeriesStatus
		{"seriesstatus == match", db.CustomRule{Enabled: true, Field: "seriesstatus", Operator: "==", Value: "ended"}, true},
		{"seriesstatus == no match", db.CustomRule{Enabled: true, Field: "seriesstatus", Operator: "==", Value: "continuing"}, false},

		// Tag field (matches any tag in the slice)
		{"tag contains match", db.CustomRule{Enabled: true, Field: "tag", Operator: "contains", Value: "anime"}, true},
		{"tag contains no match", db.CustomRule{Enabled: true, Field: "tag", Operator: "contains", Value: "horror"}, false},
		{"tag !contains match", db.CustomRule{Enabled: true, Field: "tag", Operator: "!contains", Value: "horror"}, true},

		// Genre field
		{"genre contains match", db.CustomRule{Enabled: true, Field: "genre", Operator: "contains", Value: "sci-fi"}, true},
		{"genre == exact", db.CustomRule{Enabled: true, Field: "genre", Operator: "==", Value: "action, sci-fi"}, true},

		// Rating field (numeric)
		{"rating > match", db.CustomRule{Enabled: true, Field: "rating", Operator: ">", Value: "8.0"}, true},
		{"rating < no match", db.CustomRule{Enabled: true, Field: "rating", Operator: "<", Value: "8.0"}, false},
		{"rating >= boundary", db.CustomRule{Enabled: true, Field: "rating", Operator: ">=", Value: "8.5"}, true},
		{"rating invalid value", db.CustomRule{Enabled: true, Field: "rating", Operator: ">", Value: "notanumber"}, false},

		// SizeBytes field (numeric)
		{"sizebytes > match", db.CustomRule{Enabled: true, Field: "sizebytes", Operator: ">", Value: "5000000000"}, true},
		{"sizebytes < no match", db.CustomRule{Enabled: true, Field: "sizebytes", Operator: "<", Value: "5000000000"}, false},

		// TimeInLibrary (computed from AddedAt)
		{"timeinlibrary > match", db.CustomRule{Enabled: true, Field: "timeinlibrary", Operator: ">", Value: "300"}, true},
		{"timeinlibrary < no match", db.CustomRule{Enabled: true, Field: "timeinlibrary", Operator: "<", Value: "100"}, false},

		// SeasonCount
		{"seasoncount == match", db.CustomRule{Enabled: true, Field: "seasoncount", Operator: "==", Value: "3"}, true},
		{"seasoncount > match", db.CustomRule{Enabled: true, Field: "seasoncount", Operator: ">", Value: "2"}, true},

		// EpisodeCount
		{"episodecount >= match", db.CustomRule{Enabled: true, Field: "episodecount", Operator: ">=", Value: "24"}, true},
		{"episodecount < no match", db.CustomRule{Enabled: true, Field: "episodecount", Operator: "<", Value: "10"}, false},

		// Monitored (boolean)
		{"monitored == true", db.CustomRule{Enabled: true, Field: "monitored", Operator: "==", Value: "true"}, true},
		{"monitored == false no match", db.CustomRule{Enabled: true, Field: "monitored", Operator: "==", Value: "false"}, false},

		// PlayCount
		{"playcount > match", db.CustomRule{Enabled: true, Field: "playcount", Operator: ">", Value: "3"}, true},
		{"playcount == match", db.CustomRule{Enabled: true, Field: "playcount", Operator: "==", Value: "5"}, true},

		// Requested (boolean)
		{"requested == true", db.CustomRule{Enabled: true, Field: "requested", Operator: "==", Value: "true"}, true},
		{"requested == false no match", db.CustomRule{Enabled: true, Field: "requested", Operator: "==", Value: "false"}, false},

		// RequestCount
		{"requestcount >= match", db.CustomRule{Enabled: true, Field: "requestcount", Operator: ">=", Value: "2"}, true},

		// Language
		{"language == match", db.CustomRule{Enabled: true, Field: "language", Operator: "==", Value: "english"}, true},
		{"language != match", db.CustomRule{Enabled: true, Field: "language", Operator: "!=", Value: "japanese"}, true},

		// Collection (string field matching against Collections slice)
		{"collection == match", db.CustomRule{Enabled: true, Field: "collection", Operator: "==", Value: "serenity saga"}, true},
		{"collection == no match", db.CustomRule{Enabled: true, Field: "collection", Operator: "==", Value: "marvel"}, false},
		{"collection contains match", db.CustomRule{Enabled: true, Field: "collection", Operator: "contains", Value: "sci-fi"}, true},
		{"collection contains no match", db.CustomRule{Enabled: true, Field: "collection", Operator: "contains", Value: "marvel"}, false},
		{"collection != match", db.CustomRule{Enabled: true, Field: "collection", Operator: "!=", Value: "marvel"}, true},
		{"collection !contains match", db.CustomRule{Enabled: true, Field: "collection", Operator: "!contains", Value: "marvel"}, true},

		// Type
		{"type == match", db.CustomRule{Enabled: true, Field: "type", Operator: "==", Value: "show"}, true},
		{"type != match", db.CustomRule{Enabled: true, Field: "type", Operator: "!=", Value: "movie"}, true},

		// Year
		{"year == match", db.CustomRule{Enabled: true, Field: "year", Operator: "==", Value: "1999"}, true},
		{"year > match", db.CustomRule{Enabled: true, Field: "year", Operator: ">", Value: "1990"}, true},
		{"year < no match", db.CustomRule{Enabled: true, Field: "year", Operator: "<", Value: "1990"}, false},

		// Unknown field
		{"unknown field returns false", db.CustomRule{Enabled: true, Field: "nonexistent", Operator: "==", Value: "anything"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, _ := matchesRuleWithValue(baseItem, tc.rule)
			if result != tc.matched {
				t.Errorf("matchesRuleWithValue for %s %s %s = %v, want %v",
					tc.rule.Field, tc.rule.Operator, tc.rule.Value, result, tc.matched)
			}
		})
	}
}

func TestMatchesRule_NilAddedAt(t *testing.T) {
	item := integrations.MediaItem{AddedAt: nil}
	rule := db.CustomRule{Enabled: true, Field: "timeinlibrary", Operator: ">", Value: "30"}

	result, _ := matchesRuleWithValue(item, rule)
	if result {
		t.Error("Expected false for timeinlibrary with nil AddedAt")
	}
}

func TestMatchesRule_TagNoTags(t *testing.T) {
	item := integrations.MediaItem{Tags: nil}
	rule := db.CustomRule{Enabled: true, Field: "tag", Operator: "contains", Value: "anime"}

	result, _ := matchesRuleWithValue(item, rule)
	if result {
		t.Error("Expected false for tag match with no tags")
	}
}

func TestMatchesRule_TagNotContainsWithNoTags(t *testing.T) {
	item := integrations.MediaItem{Tags: nil}
	rule := db.CustomRule{Enabled: true, Field: "tag", Operator: "!contains", Value: "anime"}

	// With nil tags, !contains is vacuously true (no tag violates the condition)
	result, _ := matchesRuleWithValue(item, rule)
	if !result {
		t.Error("Expected true for tag !contains with no tags (vacuous truth)")
	}
}

func TestApplyRules(t *testing.T) {
	now := time.Now()

	baseItem := integrations.MediaItem{
		Title:         "Serenity",
		SeriesStatus:  "Ended",
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
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "title", Operator: "==", Value: "serenity", Effect: "always_keep"},
			},
			isAbs:    true,
			modifier: 0.0,
		},
		{
			name: "Prefer keep by rating",
			item: baseItem,
			rules: []db.CustomRule{
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "rating", Operator: ">", Value: "8.0", Effect: "prefer_keep"},
			},
			isAbs:    false,
			modifier: 0.2,
		},
		{
			name: "Lean keep modifier",
			item: baseItem,
			rules: []db.CustomRule{
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "rating", Operator: ">", Value: "8.0", Effect: "lean_keep"},
			},
			isAbs:    false,
			modifier: 0.5,
		},
		{
			name: "Lean remove modifier",
			item: baseItem,
			rules: []db.CustomRule{
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "seriesstatus", Operator: "==", Value: "ended", Effect: "lean_remove"},
			},
			isAbs:    false,
			modifier: 1.5,
		},
		{
			name: "Prefer remove modifier",
			item: baseItem,
			rules: []db.CustomRule{
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "seriesstatus", Operator: "==", Value: "ended", Effect: "prefer_remove"},
			},
			isAbs:    false,
			modifier: 3.0,
		},
		{
			name: "Always remove by seriesstatus",
			item: baseItem,
			rules: []db.CustomRule{
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "seriesstatus", Operator: "==", Value: "ended", Effect: "always_remove"},
			},
			isAbs:    false,
			modifier: 100.0,
		},
		{
			name: "Multiple cascading modifiers multiply",
			item: baseItem,
			rules: []db.CustomRule{
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "rating", Operator: ">", Value: "8.0", Effect: "prefer_keep"},       // ×0.2
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "title", Operator: "contains", Value: "seren", Effect: "lean_keep"}, // ×0.5
			},
			isAbs:    false,
			modifier: 0.1, // 0.2 × 0.5
		},
		{
			name: "Lean keep + lean remove partially cancel",
			item: baseItem,
			rules: []db.CustomRule{
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "rating", Operator: ">", Value: "8.0", Effect: "lean_keep"},            // ×0.5
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "seriesstatus", Operator: "==", Value: "ended", Effect: "lean_remove"}, // ×1.5
			},
			isAbs:    false,
			modifier: 0.75, // 0.5 × 1.5
		},
		{
			name: "Always keep wins over always remove",
			item: baseItem,
			rules: []db.CustomRule{
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "title", Operator: "==", Value: "serenity", Effect: "always_keep"},
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "seriesstatus", Operator: "==", Value: "ended", Effect: "always_remove"},
			},
			isAbs:    true,
			modifier: 0.0,
		},
		{
			name: "Always keep wins over prefer remove",
			item: baseItem,
			rules: []db.CustomRule{
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "title", Operator: "==", Value: "serenity", Effect: "always_keep"},
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "rating", Operator: ">", Value: "5.0", Effect: "prefer_remove"},
			},
			isAbs:    true,
			modifier: 0.0,
		},
		{
			name: "Prefer keep + prefer remove = net protection",
			item: baseItem,
			rules: []db.CustomRule{
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "rating", Operator: ">", Value: "8.0", Effect: "prefer_keep"},            // ×0.2
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "seriesstatus", Operator: "==", Value: "ended", Effect: "prefer_remove"}, // ×3.0
			},
			isAbs:    false,
			modifier: 0.6, // 0.2 × 3.0
		},
		{
			name: "Stacked prefer remove",
			item: baseItem,
			rules: []db.CustomRule{
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "rating", Operator: ">", Value: "5.0", Effect: "prefer_remove"},          // ×3.0
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "seriesstatus", Operator: "==", Value: "ended", Effect: "prefer_remove"}, // ×3.0
			},
			isAbs:    false,
			modifier: 9.0, // 3.0 × 3.0
		},
		{
			name: "Non-matching rule has no effect",
			item: baseItem,
			rules: []db.CustomRule{
				{Enabled: true, IntegrationID: intIDPtr(1), Field: "title", Operator: "==", Value: "other", Effect: "always_keep"},
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
		Title:         "Serenity",
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
				{Enabled: true, IntegrationID: &integrationID1, Field: "title", Operator: "==", Value: "serenity", Effect: "always_keep"},
			},
			isAbs:    true,
			modifier: 0.0,
		},
		{
			name: "Rule scoped to different integration is skipped",
			rules: []db.CustomRule{
				{Enabled: true, IntegrationID: &integrationID2, Field: "title", Operator: "==", Value: "serenity", Effect: "always_keep"},
			},
			isAbs:    false,
			modifier: 1.0,
		},
		{
			name: "Nil integration_id rule is skipped (no global rules)",
			rules: []db.CustomRule{
				{Enabled: true, IntegrationID: nil, Field: "title", Operator: "==", Value: "serenity", Effect: "prefer_keep"},
			},
			isAbs:    false,
			modifier: 1.0,
		},
		{
			name: "Mixed: nil integration rule skipped, scoped rule skipped",
			rules: []db.CustomRule{
				{Enabled: true, IntegrationID: nil, Field: "rating", Operator: ">", Value: "8.0", Effect: "lean_keep"},                    // skipped (nil)
				{Enabled: true, IntegrationID: &integrationID2, Field: "title", Operator: "==", Value: "serenity", Effect: "always_keep"}, // skipped (wrong integration)
			},
			isAbs:    false,
			modifier: 1.0,
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
		Title:         "Serenity",
		Rating:        8.5,
		AddedAt:       &now,
		IntegrationID: 1,
	}

	rules := []db.CustomRule{
		{Enabled: true, IntegrationID: intIDPtr(1), Field: "rating", Operator: ">", Value: "8.0", Effect: "prefer_keep"},
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
		Title:         "Serenity",
		Rating:        8.5,
		AddedAt:       &now,
		IntegrationID: 1,
	}

	// always_keep is first followed by modifiers that would change things
	rules := []db.CustomRule{
		{Enabled: true, IntegrationID: intIDPtr(1), Field: "title", Operator: "==", Value: "serenity", Effect: "always_keep"},
		{Enabled: true, IntegrationID: intIDPtr(1), Field: "rating", Operator: ">", Value: "5.0", Effect: "prefer_remove"},
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

func TestApplyRules_RuleIDPropagation(t *testing.T) {
	now := time.Now()
	item := integrations.MediaItem{
		Title:         "Serenity",
		Rating:        3.0,
		AddedAt:       &now,
		IntegrationID: 1,
	}

	rules := []db.CustomRule{
		{ID: 42, Enabled: true, IntegrationID: intIDPtr(1), Field: "rating", Operator: "<", Value: "5.0", Effect: "prefer_remove"},
		{ID: 99, Enabled: true, IntegrationID: intIDPtr(1), Field: "title", Operator: "contains", Value: "seren", Effect: "lean_remove"},
	}

	_, _, _, factors := applyRules(item, rules)
	if len(factors) != 2 {
		t.Fatalf("Expected 2 rule factors, got %d", len(factors))
	}

	// Verify first factor has RuleID 42
	if factors[0].RuleID == nil {
		t.Fatal("Expected RuleID to be set on first factor")
	}
	if *factors[0].RuleID != 42 {
		t.Errorf("Expected RuleID 42, got %d", *factors[0].RuleID)
	}

	// Verify second factor has RuleID 99
	if factors[1].RuleID == nil {
		t.Fatal("Expected RuleID to be set on second factor")
	}
	if *factors[1].RuleID != 99 {
		t.Errorf("Expected RuleID 99, got %d", *factors[1].RuleID)
	}
}

func TestApplyRules_AlwaysKeepRuleID(t *testing.T) {
	now := time.Now()
	item := integrations.MediaItem{
		Title:         "Serenity",
		Rating:        8.5,
		AddedAt:       &now,
		IntegrationID: 1,
	}

	rules := []db.CustomRule{
		{ID: 7, Enabled: true, IntegrationID: intIDPtr(1), Field: "title", Operator: "==", Value: "serenity", Effect: "always_keep"},
	}

	isAbs, _, _, factors := applyRules(item, rules)
	if !isAbs {
		t.Fatal("Expected absolute protection")
	}
	if len(factors) != 1 {
		t.Fatalf("Expected 1 factor, got %d", len(factors))
	}
	if factors[0].RuleID == nil {
		t.Fatal("Expected RuleID to be set on always_keep factor")
	}
	if *factors[0].RuleID != 7 {
		t.Errorf("Expected RuleID 7, got %d", *factors[0].RuleID)
	}
}

func TestApplyRules_WeightFactorsHaveNilRuleID(t *testing.T) {
	now := time.Now()
	item := integrations.MediaItem{
		Title:         "Serenity",
		Rating:        5.0,
		AddedAt:       &now,
		IntegrationID: 1,
	}

	weights := map[string]int{
		"watch_history":      10,
		"last_watched":       8,
		"file_size":          6,
		"rating":             5,
		"time_in_library":    4,
		"series_status":      3,
		"request_popularity": 0,
	}

	evaluated := EvaluateMedia([]integrations.MediaItem{item}, DefaultFactors(), weights, nil, allActiveCtx())
	if len(evaluated) != 1 {
		t.Fatalf("Expected 1 evaluated item, got %d", len(evaluated))
	}

	for _, f := range evaluated[0].Factors {
		if f.Type == "weight" && f.RuleID != nil {
			t.Errorf("Weight factor %q should have nil RuleID, got %d", f.Name, *f.RuleID)
		}
	}
}

// TestLegacyEffect removed — legacyEffect() and the deprecated Type/Intensity
// fields were removed in the service-layer-event-bus refactor.

func TestMatchesRule_LastPlayed(t *testing.T) {
	recentPlay := time.Now().Add(-5 * 24 * time.Hour) // 5 days ago
	oldPlay := time.Now().Add(-90 * 24 * time.Hour)   // 90 days ago

	tests := []struct {
		name       string
		lastPlayed *time.Time
		operator   string
		value      string
		matched    bool
	}{
		{"in_last match (recent)", &recentPlay, "in_last", "30", true},
		{"in_last no match (old)", &oldPlay, "in_last", "30", false},
		{"over_ago match (old)", &oldPlay, "over_ago", "30", true},
		{"over_ago no match (recent)", &recentPlay, "over_ago", "30", false},
		{"never with nil", nil, "never", "", true},
		{"never with play", &recentPlay, "never", "", false},
		{"over_ago with nil (counts as over)", nil, "over_ago", "30", true},
		{"in_last with nil (not in last)", nil, "in_last", "30", false},
		{"invalid value", &recentPlay, "in_last", "notanumber", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := integrations.MediaItem{LastPlayed: tc.lastPlayed}
			rule := db.CustomRule{Enabled: true, Field: "lastplayed", Operator: tc.operator, Value: tc.value}
			result, _ := matchesRuleWithValue(item, rule)
			if result != tc.matched {
				t.Errorf("lastplayed %s %s = %v, want %v", tc.operator, tc.value, result, tc.matched)
			}
		})
	}
}

func TestMatchesRule_RequestedBy(t *testing.T) {
	tests := []struct {
		name        string
		requestedBy string
		operator    string
		value       string
		matched     bool
	}{
		{"== match case insensitive", "John", "==", "john", true},
		{"== no match", "John", "==", "jane", false},
		{"!= match", "John", "!=", "jane", true},
		{"!= no match", "John", "!=", "john", false},
		{"contains match", "JohnDoe", "contains", "john", true},
		{"contains no match", "JohnDoe", "contains", "jane", false},
		{"!contains match", "JohnDoe", "!contains", "jane", true},
		{"!contains no match", "JohnDoe", "!contains", "john", false},
		{"== empty requestedby vs empty value", "", "==", "", true},
		{"contains empty value always matches", "anything", "contains", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := integrations.MediaItem{RequestedBy: tc.requestedBy}
			rule := db.CustomRule{Enabled: true, Field: "requestedby", Operator: tc.operator, Value: tc.value}
			result, _ := matchesRuleWithValue(item, rule)
			if result != tc.matched {
				t.Errorf("requestedby %s %s = %v, want %v", tc.operator, tc.value, result, tc.matched)
			}
		})
	}
}

func TestMatchesRule_InCollection(t *testing.T) {
	tests := []struct {
		name        string
		collections []string
		value       string
		matched     bool
	}{
		{"true with collections", []string{"Marvel", "MCU"}, "true", true},
		{"true with no collections", nil, "true", false},
		{"false with no collections", nil, "false", true},
		{"false with collections", []string{"Marvel"}, "false", false},
		{"true with empty slice", []string{}, "true", false},
		{"false with empty slice", []string{}, "false", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := integrations.MediaItem{Collections: tc.collections}
			rule := db.CustomRule{Enabled: true, Field: "incollection", Operator: "==", Value: tc.value}
			result, _ := matchesRuleWithValue(item, rule)
			if result != tc.matched {
				t.Errorf("incollection == %s = %v, want %v", tc.value, result, tc.matched)
			}
		})
	}
}

func TestMatchesRule_Watchlist(t *testing.T) {
	tests := []struct {
		name        string
		onWatchlist bool
		value       string
		matched     bool
	}{
		{"true match", true, "true", true},
		{"true no match", false, "true", false},
		{"false match", false, "false", true},
		{"false no match", true, "false", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := integrations.MediaItem{OnWatchlist: tc.onWatchlist}
			rule := db.CustomRule{Enabled: true, Field: "watchlist", Operator: "==", Value: tc.value}
			result, _ := matchesRuleWithValue(item, rule)
			if result != tc.matched {
				t.Errorf("watchlist == %s = %v, want %v", tc.value, result, tc.matched)
			}
		})
	}
}

func TestMatchesRule_WatchedByReq(t *testing.T) {
	tests := []struct {
		name               string
		watchedByRequestor bool
		value              string
		matched            bool
	}{
		{"true match", true, "true", true},
		{"true no match", false, "true", false},
		{"false match", false, "false", true},
		{"false no match", true, "false", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := integrations.MediaItem{WatchedByRequestor: tc.watchedByRequestor}
			rule := db.CustomRule{Enabled: true, Field: "watchedbyreq", Operator: "==", Value: tc.value}
			result, _ := matchesRuleWithValue(item, rule)
			if result != tc.matched {
				t.Errorf("watchedbyreq == %s = %v, want %v", tc.value, result, tc.matched)
			}
		})
	}
}

func TestMatchesRule_TimeInLibrary_DateOperators(t *testing.T) {
	recentAdd := time.Now().Add(-10 * 24 * time.Hour) // 10 days ago
	oldAdd := time.Now().Add(-90 * 24 * time.Hour)    // 90 days ago

	tests := []struct {
		name     string
		addedAt  *time.Time
		operator string
		value    string
		matched  bool
	}{
		{"in_last match (recent)", &recentAdd, "in_last", "30", true},
		{"in_last no match (old)", &oldAdd, "in_last", "30", false},
		{"over_ago match (old)", &oldAdd, "over_ago", "30", true},
		{"over_ago no match (recent)", &recentAdd, "over_ago", "30", false},
		{"nil addedAt in_last", nil, "in_last", "30", false},
		{"nil addedAt over_ago", nil, "over_ago", "30", false},
		// Backward compat: old operators still work
		{"old > operator still works", &oldAdd, ">", "30", true},
		{"old < operator still works", &recentAdd, "<", "30", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := integrations.MediaItem{AddedAt: tc.addedAt}
			rule := db.CustomRule{Enabled: true, Field: "timeinlibrary", Operator: tc.operator, Value: tc.value}
			result, _ := matchesRuleWithValue(item, rule)
			if result != tc.matched {
				t.Errorf("timeinlibrary %s %s = %v, want %v", tc.operator, tc.value, result, tc.matched)
			}
		})
	}
}

func TestMatchesRule_Collection(t *testing.T) {
	tests := []struct {
		name        string
		collections []string
		operator    string
		value       string
		matched     bool
	}{
		// Exact match (==)
		{"== match exact", []string{"Favorites", "MCU"}, "==", "favorites", true},
		{"== no match", []string{"Favorites", "MCU"}, "==", "dceu", false},
		{"== match second", []string{"Sci-Fi", "Firefly Collection"}, "==", "firefly collection", true},

		// Not equal (!=) — all collections must differ
		{"!= match (none equal)", []string{"Favorites", "MCU"}, "!=", "dceu", true},
		{"!= no match (one equal)", []string{"Favorites", "MCU"}, "!=", "favorites", false},

		// Contains
		{"contains match", []string{"Firefly Collection", "Serenity Saga"}, "contains", "firefly", true},
		{"contains no match", []string{"Firefly Collection"}, "contains", "serenity", false},
		{"contains match partial", []string{"MCU Phase 1"}, "contains", "phase", true},

		// Not contains — all collections must not contain the value
		{"!contains match (none contain)", []string{"Favorites", "MCU"}, "!contains", "firefly", true},
		{"!contains no match (one contains)", []string{"Firefly Collection", "MCU"}, "!contains", "firefly", false},

		// Edge cases
		{"== empty collections", nil, "==", "anything", false},
		{"!= empty collections (vacuous truth)", nil, "!=", "anything", true},
		{"contains empty collections", []string{}, "contains", "anything", false},
		{"!contains empty collections (vacuous truth)", []string{}, "!contains", "anything", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := integrations.MediaItem{Collections: tc.collections}
			rule := db.CustomRule{Enabled: true, Field: "collection", Operator: tc.operator, Value: tc.value}
			result, _ := matchesRuleWithValue(item, rule)
			if result != tc.matched {
				t.Errorf("collection %s %q with %v = %v, want %v",
					tc.operator, tc.value, tc.collections, result, tc.matched)
			}
		})
	}
}

func TestMatchesRule_CollectionMatchedValue(t *testing.T) {
	// Verify matched value returns the specific collection that matched
	item := integrations.MediaItem{Collections: []string{"Sci-Fi Classics", "Firefly Collection"}}

	// Positive match: should return the matching collection
	rule := db.CustomRule{Enabled: true, Field: "collection", Operator: "contains", Value: "firefly"}
	_, matchedValue := matchesRuleWithValue(item, rule)
	if matchedValue != "Firefly Collection" {
		t.Errorf("Expected matchedValue 'Firefly Collection', got %q", matchedValue)
	}

	// Negation match: should return all collections joined
	rule = db.CustomRule{Enabled: true, Field: "collection", Operator: "!contains", Value: "marvel"}
	matched, matchedValue := matchesRuleWithValue(item, rule)
	if !matched {
		t.Error("Expected !contains 'marvel' to match")
	}
	if matchedValue != "Sci-Fi Classics, Firefly Collection" {
		t.Errorf("Expected matchedValue 'Sci-Fi Classics, Firefly Collection', got %q", matchedValue)
	}

	// No collections: negation should return "(no collections)"
	item2 := integrations.MediaItem{Collections: nil}
	rule = db.CustomRule{Enabled: true, Field: "collection", Operator: "!=", Value: "anything"}
	matched, matchedValue = matchesRuleWithValue(item2, rule)
	if !matched {
		t.Error("Expected != with nil collections to match (vacuous truth)")
	}
	if matchedValue != "(no collections)" {
		t.Errorf("Expected matchedValue '(no collections)', got %q", matchedValue)
	}
}

// TestApplyRules_LegacyFallback was removed because the deprecated type/intensity
// fields were removed in the Phase 0 schema rewrite. Rules now require the
// effect field (always_keep, prefer_keep, lean_keep, lean_remove, prefer_remove,
// always_remove). Legacy type/intensity fallback logic no longer exists.
