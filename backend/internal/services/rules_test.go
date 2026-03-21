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
	intID := seedTestIntegration(t, database)

	// Seed rules with different sort orders
	database.Create(&db.CustomRule{Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep", Enabled: true, SortOrder: 2, IntegrationID: intID})
	database.Create(&db.CustomRule{Field: "tag", Operator: "contains", Value: "anime", Effect: "prefer_keep", Enabled: true, SortOrder: 1, IntegrationID: intID})
	database.Create(&db.CustomRule{Field: "rating", Operator: ">", Value: "7.5", Effect: "lean_remove", Enabled: true, SortOrder: 0, IntegrationID: intID})

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
	intID := seedTestIntegration(t, database)

	// Two rules with the same sort_order — should tiebreak by ID ASC
	database.Create(&db.CustomRule{Field: "first", Operator: "==", Value: "a", Effect: "always_keep", Enabled: true, SortOrder: 0, IntegrationID: intID})
	database.Create(&db.CustomRule{Field: "second", Operator: "==", Value: "b", Effect: "always_keep", Enabled: true, SortOrder: 0, IntegrationID: intID})

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
	intID := seedTestIntegration(t, database)

	rule := db.CustomRule{
		Field:         "quality",
		Operator:      "==",
		Value:         "4K",
		Effect:        "always_keep",
		IntegrationID: intID,
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
	intID := seedTestIntegration(t, database)

	tests := []struct {
		name string
		rule db.CustomRule
	}{
		{"missing field", db.CustomRule{Operator: "==", Value: "4K", Effect: "always_keep", IntegrationID: intID}},
		{"missing operator", db.CustomRule{Field: "quality", Value: "4K", Effect: "always_keep", IntegrationID: intID}},
		{"missing value", db.CustomRule{Field: "quality", Operator: "==", Effect: "always_keep", IntegrationID: intID}},
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
	intID := seedTestIntegration(t, database)

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
				Field:         "quality",
				Operator:      "==",
				Value:         "4K",
				Effect:        tt.effect,
				IntegrationID: intID,
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
	intID := seedTestIntegration(t, database)

	// Create a rule first
	original := db.CustomRule{Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep", Enabled: true, IntegrationID: intID}
	database.Create(&original)

	// Update it
	updated := db.CustomRule{
		Field:         "quality",
		Operator:      "==",
		Value:         "1080p",
		Effect:        "prefer_keep",
		Enabled:       true,
		IntegrationID: intID,
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
	intID := seedTestIntegration(t, database)

	rule := db.CustomRule{Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep", IntegrationID: intID}
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
	intID := seedTestIntegration(t, database)

	// Create a rule first
	rule := db.CustomRule{Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep", Enabled: true, IntegrationID: intID}
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
	intID := seedTestIntegration(t, database)

	// Create rules in default order
	r1 := db.CustomRule{Field: "first", Operator: "==", Value: "a", Effect: "always_keep", Enabled: true, SortOrder: 0, IntegrationID: intID}
	r2 := db.CustomRule{Field: "second", Operator: "==", Value: "b", Effect: "always_keep", Enabled: true, SortOrder: 1, IntegrationID: intID}
	r3 := db.CustomRule{Field: "third", Operator: "==", Value: "c", Effect: "always_keep", Enabled: true, SortOrder: 2, IntegrationID: intID}
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
	intID := seedTestIntegration(t, database)

	// Seed enabled rules via Create
	database.Create(&db.CustomRule{Field: "title", Operator: "equals", Value: "Firefly", Effect: "always_keep", Enabled: true, IntegrationID: intID})
	database.Create(&db.CustomRule{Field: "genre", Operator: "equals", Value: "Sci-Fi", Effect: "prefer_keep", Enabled: true, IntegrationID: intID})

	// Create a rule then disable it via raw SQL: GORM's default:true prevents
	// inserting false directly, and Model().Update() may also skip zero-value bools.
	disabledRule := db.CustomRule{Field: "title", Operator: "equals", Value: "Serenity", Effect: "always_keep", Enabled: true, IntegrationID: intID}
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
	intID := seedTestIntegration(t, database)

	// Create a valid rule first
	original := db.CustomRule{Field: "title", Operator: "contains", Value: "Firefly", Effect: "always_keep", Enabled: true, SortOrder: 5, IntegrationID: intID}
	database.Create(&original)

	tests := []struct {
		name string
		rule db.CustomRule
	}{
		{"empty field", db.CustomRule{Field: "", Operator: "==", Value: "Serenity", Effect: "always_keep", Enabled: true, IntegrationID: intID}},
		{"empty operator", db.CustomRule{Field: "title", Operator: "", Value: "Serenity", Effect: "always_keep", Enabled: true, IntegrationID: intID}},
		{"empty value", db.CustomRule{Field: "title", Operator: "==", Value: "", Effect: "always_keep", Enabled: true, IntegrationID: intID}},
		{"empty effect", db.CustomRule{Field: "title", Operator: "==", Value: "Serenity", Effect: "", Enabled: true, IntegrationID: intID}},
		{"invalid effect", db.CustomRule{Field: "title", Operator: "==", Value: "Serenity", Effect: "banana", Enabled: true, IntegrationID: intID}},
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
	intID := seedTestIntegration(t, database)

	// Create a rule with an explicit sort order
	original := db.CustomRule{Field: "title", Operator: "contains", Value: "Firefly", Effect: "always_keep", Enabled: true, SortOrder: 7, IntegrationID: intID}
	database.Create(&original)

	// Update without providing sort order (simulates the edit form which doesn't set it)
	updated := db.CustomRule{
		Field:         "title",
		Operator:      "contains",
		Value:         "Serenity",
		Effect:        "prefer_keep",
		Enabled:       true,
		IntegrationID: intID,
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

// ---------- GetFieldDefinitions ----------

func TestRulesService_GetFieldDefinitions_BaseFields(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	fields := svc.GetFieldDefinitions("", EnrichmentPresence{})
	if len(fields) == 0 {
		t.Fatal("Expected non-empty field list")
	}

	// Verify title field is present
	found := false
	for _, f := range fields {
		if f.Field == "title" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'title' field in base fields")
	}

	// Verify Media Type field is always last
	last := fields[len(fields)-1]
	if last.Field != "type" {
		t.Errorf("Expected last field to be 'type', got %q", last.Field)
	}
}

func TestRulesService_GetFieldDefinitions_SonarrSpecific(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	// When serviceType == "sonarr", Sonarr fields should be included
	fields := svc.GetFieldDefinitions("sonarr", EnrichmentPresence{})
	hasSeries := false
	for _, f := range fields {
		if f.Field == "seriesstatus" {
			hasSeries = true
			break
		}
	}
	if !hasSeries {
		t.Error("Expected 'seriesstatus' field for Sonarr service type")
	}
}

func TestRulesService_GetFieldDefinitions_NoSonarrForRadarr(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	// When serviceType == "radarr", Sonarr fields should NOT be included
	fields := svc.GetFieldDefinitions("radarr", EnrichmentPresence{})
	for _, f := range fields {
		if f.Field == "seriesstatus" {
			t.Error("Did NOT expect 'seriesstatus' field for Radarr service type")
		}
	}
}

func TestRulesService_GetFieldDefinitions_EnrichmentFields(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	enrichment := EnrichmentPresence{HasTautulli: true, HasSeerr: true, HasMedia: true}
	fields := svc.GetFieldDefinitions("sonarr", enrichment)

	expected := map[string]bool{
		"playcount":    false,
		"lastplayed":   false,
		"requested":    false,
		"incollection": false,
		"watchedbyreq": false,
	}
	for _, f := range fields {
		if _, ok := expected[f.Field]; ok {
			expected[f.Field] = true
		}
	}
	for field, found := range expected {
		if !found {
			t.Errorf("Expected enrichment field %q to be present", field)
		}
	}
}

func TestRulesService_GetFieldDefinitions_UnfilteredWithSonarr(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	// When unfiltered (serviceType == "") and HasSonarr is true, Sonarr fields should appear
	fields := svc.GetFieldDefinitions("", EnrichmentPresence{HasSonarr: true})
	hasSeries := false
	for _, f := range fields {
		if f.Field == "seriesstatus" {
			hasSeries = true
			break
		}
	}
	if !hasSeries {
		t.Error("Expected 'seriesstatus' in unfiltered mode when HasSonarr is true")
	}
}

func TestRulesService_GetFieldDefinitions_UnfilteredWithoutSonarr(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	// When unfiltered but HasSonarr is false, Sonarr fields should not appear
	fields := svc.GetFieldDefinitions("", EnrichmentPresence{HasSonarr: false})
	for _, f := range fields {
		if f.Field == "seriesstatus" {
			t.Error("Did NOT expect 'seriesstatus' in unfiltered mode when HasSonarr is false")
		}
	}
}

// ---------- GetRuleContext ----------

func TestRulesService_GetRuleContext_NotFound(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	_, err := svc.GetRuleContext(99999)
	if err == nil {
		t.Error("Expected error for non-existent rule")
	}
	if !errors.Is(err, ErrRuleNotFound) {
		t.Errorf("Expected ErrRuleNotFound, got: %v", err)
	}
}

func TestRulesService_GetRuleContext_NoIntegrationProvider(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)
	// Don't call SetIntegrationProvider — it's nil

	// Create a rule without an integration ID (nil) so no FK constraint issues
	rule := db.CustomRule{Field: "title", Operator: "contains", Value: "Firefly", Effect: "always_keep", Enabled: true}
	database.Create(&rule)

	ctx, err := svc.GetRuleContext(rule.ID)
	if err != nil {
		t.Fatalf("GetRuleContext returned error: %v", err)
	}
	if ctx.Rule.ID != rule.ID {
		t.Errorf("Expected rule ID %d, got %d", rule.ID, ctx.Rule.ID)
	}
	// Fields and values should be empty since integration is nil
	if len(ctx.Fields) != 0 {
		t.Errorf("Expected 0 fields without integration, got %d", len(ctx.Fields))
	}
}

func TestRulesService_GetRuleContext_NilIntegrationID(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewRulesService(database, bus)

	// Rule with nil integration ID
	rule := db.CustomRule{Field: "title", Operator: "contains", Value: "Firefly", Effect: "always_keep", Enabled: true}
	database.Create(&rule)

	ctx, err := svc.GetRuleContext(rule.ID)
	if err != nil {
		t.Fatalf("GetRuleContext returned error: %v", err)
	}
	if ctx.Rule.Value != "Firefly" {
		t.Errorf("Expected value 'Firefly', got %q", ctx.Rule.Value)
	}
}
