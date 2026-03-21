package services

import (
	"errors"
	"testing"

	"capacitarr/internal/db"
)

// ---------- List ----------

func TestRulesService_List_Empty(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	rules, err := svc.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("Expected 0 rules, got %d", len(rules))
	}
}

func TestRulesService_List_WithSeededRules(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	// Seed rules with different sort orders
	database.Create(&db.CustomRule{Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep", Enabled: true, SortOrder: 2})
	database.Create(&db.CustomRule{Field: "tag", Operator: "contains", Value: "anime", Effect: "prefer_keep", Enabled: true, SortOrder: 1})
	database.Create(&db.CustomRule{Field: "rating", Operator: ">", Value: "7.5", Effect: "lean_remove", Enabled: true, SortOrder: 0})

	rules, err := svc.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("Expected 3 rules, got %d", len(rules))
	}

	// Verify ordering: sort_order 0, 1, 2
	if rules[0].Field != "rating" {
		t.Errorf("Expected first rule to be 'rating' (sort_order=0), got %q", rules[0].Field)
	}
	if rules[1].Field != "tag" {
		t.Errorf("Expected second rule to be 'tag' (sort_order=1), got %q", rules[1].Field)
	}
	if rules[2].Field != "quality" {
		t.Errorf("Expected third rule to be 'quality' (sort_order=2), got %q", rules[2].Field)
	}
}

func TestRulesService_List_OrderingTiebreakByID(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	// Two rules with the same sort_order — should tiebreak by ID ASC
	database.Create(&db.CustomRule{Field: "first", Operator: "==", Value: "a", Effect: "always_keep", Enabled: true, SortOrder: 0})
	database.Create(&db.CustomRule{Field: "second", Operator: "==", Value: "b", Effect: "always_keep", Enabled: true, SortOrder: 0})

	rules, err := svc.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("Expected 2 rules, got %d", len(rules))
	}
	if rules[0].Field != "first" {
		t.Errorf("Expected first rule (lower ID) to come first, got %q", rules[0].Field)
	}
}

// ---------- Create ----------

func TestRulesService_Create_Valid(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	rule := db.CustomRule{
		Field:    "quality",
		Operator: "==",
		Value:    "4K",
		Effect:   "always_keep",
	}

	created, err := svc.Create(rule)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID == 0 {
		t.Error("Expected created rule to have a non-zero ID")
	}
	if !created.Enabled {
		t.Error("Expected created rule to be enabled by default")
	}
	if created.Field != "quality" {
		t.Errorf("Expected field 'quality', got %q", created.Field)
	}

	// Verify it was persisted
	rules, _ := svc.List()
	if len(rules) != 1 {
		t.Errorf("Expected 1 rule in DB, got %d", len(rules))
	}
}

func TestRulesService_Create_MissingFields(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	tests := []struct {
		name string
		rule db.CustomRule
	}{
		{"missing field", db.CustomRule{Operator: "==", Value: "4K", Effect: "always_keep"}},
		{"missing operator", db.CustomRule{Field: "quality", Value: "4K", Effect: "always_keep"}},
		{"missing value", db.CustomRule{Field: "quality", Operator: "==", Effect: "always_keep"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(tt.rule)
			if err == nil {
				t.Error("Expected error for missing required fields")
			}
		})
	}
}

func TestRulesService_Create_InvalidEffect(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	tests := []struct {
		name   string
		effect string
	}{
		{"empty effect", ""},
		{"invalid effect", "super_keep"},
		{"typo effect", "always_keeps"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := db.CustomRule{
				Field:    "quality",
				Operator: "==",
				Value:    "4K",
				Effect:   tt.effect,
			}
			_, err := svc.Create(rule)
			if err == nil {
				t.Errorf("Expected error for effect %q", tt.effect)
			}
		})
	}
}

// ---------- Update ----------

func TestRulesService_Update_Existing(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	// Create a rule first
	original := db.CustomRule{Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep", Enabled: true}
	database.Create(&original)

	// Update it
	updated := db.CustomRule{
		Field:    "quality",
		Operator: "==",
		Value:    "1080p",
		Effect:   "prefer_keep",
		Enabled:  true,
	}
	result, err := svc.Update(original.ID, updated)
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if result.ID != original.ID {
		t.Errorf("Expected ID %d, got %d", original.ID, result.ID)
	}
	if result.Value != "1080p" {
		t.Errorf("Expected value '1080p', got %q", result.Value)
	}
	if result.Effect != "prefer_keep" {
		t.Errorf("Expected effect 'prefer_keep', got %q", result.Effect)
	}
}

func TestRulesService_Update_NotFound(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	rule := db.CustomRule{Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep"}
	_, err := svc.Update(99999, rule)
	if err == nil {
		t.Error("Expected error when updating non-existent rule")
	}
}

// ---------- Delete ----------

func TestRulesService_Delete_Existing(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	// Create a rule first
	rule := db.CustomRule{Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep", Enabled: true}
	database.Create(&rule)

	err := svc.Delete(rule.ID)
	if err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	// Verify it was deleted
	rules, _ := svc.List()
	if len(rules) != 0 {
		t.Errorf("Expected 0 rules after deletion, got %d", len(rules))
	}
}

func TestRulesService_Delete_NotFound(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	err := svc.Delete(99999)
	if err == nil {
		t.Error("Expected error when deleting non-existent rule")
	}
}

// ---------- Reorder ----------

func TestRulesService_Reorder_Valid(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	// Create rules in default order
	r1 := db.CustomRule{Field: "first", Operator: "==", Value: "a", Effect: "always_keep", Enabled: true, SortOrder: 0}
	r2 := db.CustomRule{Field: "second", Operator: "==", Value: "b", Effect: "always_keep", Enabled: true, SortOrder: 1}
	r3 := db.CustomRule{Field: "third", Operator: "==", Value: "c", Effect: "always_keep", Enabled: true, SortOrder: 2}
	database.Create(&r1)
	database.Create(&r2)
	database.Create(&r3)

	// Reverse the order
	err := svc.Reorder([]uint{r3.ID, r2.ID, r1.ID})
	if err != nil {
		t.Fatalf("Reorder returned error: %v", err)
	}

	// Verify new order
	rules, _ := svc.List()
	if len(rules) != 3 {
		t.Fatalf("Expected 3 rules, got %d", len(rules))
	}
	if rules[0].Field != "third" {
		t.Errorf("Expected first rule to be 'third', got %q", rules[0].Field)
	}
	if rules[1].Field != "second" {
		t.Errorf("Expected second rule to be 'second', got %q", rules[1].Field)
	}
	if rules[2].Field != "first" {
		t.Errorf("Expected third rule to be 'first', got %q", rules[2].Field)
	}
}

func TestRulesService_Reorder_EmptySlice(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	// Empty slice should succeed (no-op)
	err := svc.Reorder([]uint{})
	if err != nil {
		t.Fatalf("Reorder with empty slice returned error: %v", err)
	}
}

// ---------- GetEnabledRules ----------

func TestRulesService_GetEnabledRules_Empty(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	rules, err := svc.GetEnabledRules()
	if err != nil {
		t.Fatalf("GetEnabledRules returned error: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("Expected 0 rules, got %d", len(rules))
	}
}

func TestRulesService_GetEnabledRules_FiltersDisabled(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	// Seed enabled rules via Create
	database.Create(&db.CustomRule{Field: "title", Operator: "equals", Value: "Firefly", Effect: "always_keep", Enabled: true})
	database.Create(&db.CustomRule{Field: "genre", Operator: "equals", Value: "Sci-Fi", Effect: "prefer_keep", Enabled: true})

	// Create a rule then disable it via raw SQL: GORM's default:true prevents
	// inserting false directly, and Model().Update() may also skip zero-value bools.
	disabledRule := db.CustomRule{Field: "title", Operator: "equals", Value: "Serenity", Effect: "always_keep", Enabled: true}
	database.Create(&disabledRule)
	database.Exec("UPDATE custom_rules SET enabled = 0 WHERE id = ?", disabledRule.ID)

	rules, err := svc.GetEnabledRules()
	if err != nil {
		t.Fatalf("GetEnabledRules returned error: %v", err)
	}
	if len(rules) != 2 {
		t.Fatalf("Expected 2 enabled rules, got %d", len(rules))
	}

	// Verify both returned rules are enabled
	for _, rule := range rules {
		if !rule.Enabled {
			t.Errorf("GetEnabledRules returned disabled rule: %v", rule)
		}
	}
}

// ---------- Update Validation ----------

func TestRulesService_Update_ValidationErrors(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	// Create a valid rule first
	original := db.CustomRule{Field: "title", Operator: "contains", Value: "Firefly", Effect: "always_keep", Enabled: true, SortOrder: 5}
	database.Create(&original)

	tests := []struct {
		name string
		rule db.CustomRule
	}{
		{"empty field", db.CustomRule{Field: "", Operator: "==", Value: "Serenity", Effect: "always_keep", Enabled: true}},
		{"empty operator", db.CustomRule{Field: "title", Operator: "", Value: "Serenity", Effect: "always_keep", Enabled: true}},
		{"empty value", db.CustomRule{Field: "title", Operator: "==", Value: "", Effect: "always_keep", Enabled: true}},
		{"empty effect", db.CustomRule{Field: "title", Operator: "==", Value: "Serenity", Effect: "", Enabled: true}},
		{"invalid effect", db.CustomRule{Field: "title", Operator: "==", Value: "Serenity", Effect: "banana", Enabled: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Update(original.ID, tt.rule)
			if err == nil {
				t.Errorf("Expected validation error for %s", tt.name)
			}
			if !errors.Is(err, ErrRuleValidation) {
				t.Errorf("Expected ErrRuleValidation, got: %v", err)
			}
		})
	}

	// Verify the original rule was not modified
	rules, _ := svc.List()
	if len(rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(rules))
	}
	if rules[0].Value != "Firefly" {
		t.Errorf("Expected original value 'Firefly' preserved, got %q", rules[0].Value)
	}
}

func TestRulesService_Update_PreservesSortOrder(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	// Create a rule with an explicit sort order
	original := db.CustomRule{Field: "title", Operator: "contains", Value: "Firefly", Effect: "always_keep", Enabled: true, SortOrder: 7}
	database.Create(&original)

	// Update without providing sort order (simulates the edit form which doesn't set it)
	updated := db.CustomRule{
		Field:    "title",
		Operator: "contains",
		Value:    "Serenity",
		Effect:   "prefer_keep",
		Enabled:  true,
		// SortOrder intentionally 0 (zero value)
	}
	result, err := svc.Update(original.ID, updated)
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if result.SortOrder != 7 {
		t.Errorf("Expected sort order 7 preserved, got %d", result.SortOrder)
	}
	if result.Value != "Serenity" {
		t.Errorf("Expected value 'Serenity', got %q", result.Value)
	}
}
