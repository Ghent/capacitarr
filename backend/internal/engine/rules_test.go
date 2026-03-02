package engine

import (
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)

func TestStringMatch(t *testing.T) {
	tests := []struct {
		actual   string
		cond     string
		expected string
		result   bool
	}{
		{"The Matrix", "==", "The Matrix", true},
		{"The Matrix", "==", "the matrix", false}, // strict equal
		{"The Matrix", "!=", "Avatar", true},
		{"The Matrix", "contains", "Matrix", true},
		{"The Matrix", "contains", "Avatar", false},
		{"The Matrix", "!contains", "Avatar", true},
		{"The Matrix", "!contains", "Matrix", false},
	}

	for _, tc := range tests {
		t.Run(tc.actual+" "+tc.cond+" "+tc.expected, func(t *testing.T) {
			result := stringMatch(tc.actual, tc.cond, tc.expected)
			if result != tc.result {
				t.Errorf("Expected %v, got %v", tc.result, result)
			}
		})
	}
}

func TestNumberMatch(t *testing.T) {
	tests := []struct {
		actual   float64
		cond     string
		expected float64
		result   bool
	}{
		{5.0, "==", 5.0, true},
		{5.0, "!=", 4.0, true},
		{5.0, ">", 4.0, true},
		{5.0, ">", 5.0, false},
		{5.0, ">=", 5.0, true},
		{5.0, "<", 6.0, true},
		{5.0, "<=", 5.0, true},
	}

	for _, tc := range tests {
		t.Run("NumberMatch", func(t *testing.T) {
			result := numberMatch(tc.actual, tc.cond, tc.expected)
			if result != tc.result {
				t.Errorf("Expected %v, got %v for %v %v %v", tc.result, result, tc.actual, tc.cond, tc.expected)
			}
		})
	}
}

func TestApplyRules(t *testing.T) {
	now := time.Now()

	baseItem := integrations.MediaItem{
		Title:         "The Matrix",
		ShowStatus:    "Ended",
		Rating:        8.5,
		AddedAt:       &now,
		IntegrationID: 1,
	}

	tests := []struct {
		name     string
		item     integrations.MediaItem
		rules    []db.ProtectionRule
		isAbs    bool
		modifier float64
	}{
		{
			name:     "No rules",
			item:     baseItem,
			rules:    []db.ProtectionRule{},
			isAbs:    false,
			modifier: 1.0,
		},
		{
			name: "Always keep by title (new effect field)",
			item: baseItem,
			rules: []db.ProtectionRule{
				{Field: "title", Operator: "==", Value: "the matrix", Effect: "always_keep"},
			},
			isAbs:    true,
			modifier: 0.0,
		},
		{
			name: "Prefer keep by rating",
			item: baseItem,
			rules: []db.ProtectionRule{
				{Field: "rating", Operator: ">", Value: "8.0", Effect: "prefer_keep"},
			},
			isAbs:    false,
			modifier: 0.2,
		},
		{
			name: "Always remove by availability",
			item: baseItem,
			rules: []db.ProtectionRule{
				{Field: "availability", Operator: "==", Value: "ended", Effect: "always_remove"},
			},
			isAbs:    false,
			modifier: 100.0,
		},
		{
			name: "Multiple cascading modifiers",
			item: baseItem,
			rules: []db.ProtectionRule{
				{Field: "rating", Operator: ">", Value: "8.0", Effect: "prefer_keep"},        // ×0.2
				{Field: "title", Operator: "contains", Value: "matrix", Effect: "lean_keep"}, // ×0.5
			},
			isAbs:    false,
			modifier: 0.1, // 0.2 × 0.5
		},
		{
			name: "Lean keep + lean remove partially cancel",
			item: baseItem,
			rules: []db.ProtectionRule{
				{Field: "rating", Operator: ">", Value: "8.0", Effect: "lean_keep"},            // ×0.5
				{Field: "availability", Operator: "==", Value: "ended", Effect: "lean_remove"}, // ×1.2
			},
			isAbs:    false,
			modifier: 0.6, // 0.5 × 1.2
		},
		{
			name: "Always keep wins over always remove",
			item: baseItem,
			rules: []db.ProtectionRule{
				{Field: "title", Operator: "==", Value: "the matrix", Effect: "always_keep"},
				{Field: "availability", Operator: "==", Value: "ended", Effect: "always_remove"},
			},
			isAbs:    true,
			modifier: 0.0,
		},
		{
			name: "Always keep wins over prefer remove",
			item: baseItem,
			rules: []db.ProtectionRule{
				{Field: "title", Operator: "==", Value: "the matrix", Effect: "always_keep"},
				{Field: "rating", Operator: ">", Value: "5.0", Effect: "prefer_remove"},
			},
			isAbs:    true,
			modifier: 0.0,
		},
		{
			name: "Prefer keep + prefer remove = net protection",
			item: baseItem,
			rules: []db.ProtectionRule{
				{Field: "rating", Operator: ">", Value: "8.0", Effect: "prefer_keep"},            // ×0.2
				{Field: "availability", Operator: "==", Value: "ended", Effect: "prefer_remove"}, // ×2.0
			},
			isAbs:    false,
			modifier: 0.4, // 0.2 × 2.0
		},
		{
			name: "Stacked prefer remove",
			item: baseItem,
			rules: []db.ProtectionRule{
				{Field: "rating", Operator: ">", Value: "5.0", Effect: "prefer_remove"},          // ×2.0
				{Field: "availability", Operator: "==", Value: "ended", Effect: "prefer_remove"}, // ×2.0
			},
			isAbs:    false,
			modifier: 4.0, // 2.0 × 2.0
		},
		{
			name: "Legacy type+intensity fallback: absolute protect",
			item: baseItem,
			rules: []db.ProtectionRule{
				{Type: "protect", Field: "title", Operator: "==", Value: "the matrix", Intensity: "absolute"},
			},
			isAbs:    true,
			modifier: 0.0,
		},
		{
			name: "Legacy type+intensity fallback: strong target",
			item: baseItem,
			rules: []db.ProtectionRule{
				{Type: "target", Field: "rating", Operator: ">", Value: "8.0", Intensity: "strong"},
			},
			isAbs:    false,
			modifier: 2.0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			isAbs, modifier, _, _ := applyRules(tc.item, tc.rules)
			if isAbs != tc.isAbs {
				t.Errorf("Expected absolute protect %v, got %v", tc.isAbs, isAbs)
			}
			// Use small delta for float comparison
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
		rules    []db.ProtectionRule
		isAbs    bool
		modifier float64
	}{
		{
			name: "Rule scoped to matching integration applies",
			rules: []db.ProtectionRule{
				{IntegrationID: &integrationID1, Field: "title", Operator: "==", Value: "the matrix", Effect: "always_keep"},
			},
			isAbs:    true,
			modifier: 0.0,
		},
		{
			name: "Rule scoped to different integration is skipped",
			rules: []db.ProtectionRule{
				{IntegrationID: &integrationID2, Field: "title", Operator: "==", Value: "the matrix", Effect: "always_keep"},
			},
			isAbs:    false,
			modifier: 1.0,
		},
		{
			name: "Global rule (nil integration_id) applies to all items",
			rules: []db.ProtectionRule{
				{IntegrationID: nil, Field: "title", Operator: "==", Value: "the matrix", Effect: "prefer_keep"},
			},
			isAbs:    false,
			modifier: 0.2,
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

func TestLegacyEffect(t *testing.T) {
	tests := []struct {
		ruleType  string
		intensity string
		expected  string
	}{
		{"protect", "absolute", "always_keep"},
		{"protect", "strong", "prefer_keep"},
		{"protect", "slight", "lean_keep"},
		{"target", "absolute", "always_remove"},
		{"target", "strong", "prefer_remove"},
		{"target", "slight", "lean_remove"},
		{"", "", "lean_keep"},
	}

	for _, tc := range tests {
		t.Run(tc.ruleType+"_"+tc.intensity, func(t *testing.T) {
			result := legacyEffect(tc.ruleType, tc.intensity)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}
