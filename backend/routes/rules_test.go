package routes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/testutil"
)

// ---------- helpers ----------

// seedRule creates a single CustomRule in the database and returns it.
func seedRule(t *testing.T, database *gorm.DB, field, operator, value, effect string, sortOrder int) db.CustomRule {
	t.Helper()
	rule := db.CustomRule{
		Field:     field,
		Operator:  operator,
		Value:     value,
		Effect:    effect,
		Enabled:   true,
		SortOrder: sortOrder,
	}
	if err := database.Create(&rule).Error; err != nil {
		t.Fatalf("Failed to seed rule: %v", err)
	}
	return rule
}

// seedRules inserts n protection rules with sequential sort orders.
func seedRules(t *testing.T, database *gorm.DB, n int) []db.CustomRule {
	t.Helper()
	rules := make([]db.CustomRule, 0, n)
	for i := 0; i < n; i++ {
		r := seedRule(t, database, "title", "contains", fmt.Sprintf("value_%d", i), "always_keep", i)
		rules = append(rules, r)
	}
	return rules
}

// ---------- GET /api/custom-rules ----------

func TestGetProtections_Empty(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/custom-rules", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var rules []db.CustomRule
	if err := json.Unmarshal(rec.Body.Bytes(), &rules); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("Expected empty rules list, got %d items", len(rules))
	}
}

func TestGetProtections_WithSeededRules(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	seedRules(t, database, 3)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/custom-rules", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var rules []db.CustomRule
	if err := json.Unmarshal(rec.Body.Bytes(), &rules); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(rules) != 3 {
		t.Errorf("Expected 3 rules, got %d", len(rules))
	}
}

func TestGetProtections_OrderedBySortOrder(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Insert rules out of sort order
	seedRule(t, database, "rating", ">", "8", "always_keep", 2)
	seedRule(t, database, "title", "contains", "Star", "prefer_keep", 0)
	seedRule(t, database, "genre", "==", "Horror", "prefer_remove", 1)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/custom-rules", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var rules []db.CustomRule
	if err := json.Unmarshal(rec.Body.Bytes(), &rules); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("Expected 3 rules, got %d", len(rules))
	}

	// Verify ascending sort_order
	for i := 1; i < len(rules); i++ {
		if rules[i].SortOrder < rules[i-1].SortOrder {
			t.Errorf("Rules not ordered by sort_order: index %d has sortOrder %d, index %d has sortOrder %d",
				i-1, rules[i-1].SortOrder, i, rules[i].SortOrder)
		}
	}

	// First rule should be the one with sort_order 0
	if rules[0].Value != "Star" {
		t.Errorf("Expected first rule value 'Star', got %q", rules[0].Value)
	}
}

func TestGetProtections_Unauthenticated(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := httptest.NewRequest(http.MethodGet, "/api/custom-rules", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Error("Expected non-200 for unauthenticated request")
	}
}

// ---------- POST /api/custom-rules ----------

func TestCreateProtection_ValidWithEffect(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{"field":"title","operator":"contains","value":"Star Wars","effect":"always_keep"}`
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/custom-rules", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var rule db.CustomRule
	if err := json.Unmarshal(rec.Body.Bytes(), &rule); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if rule.ID == 0 {
		t.Error("Expected non-zero rule ID")
	}
	if rule.Field != "title" {
		t.Errorf("Expected field 'title', got %q", rule.Field)
	}
	if rule.Effect != "always_keep" {
		t.Errorf("Expected effect 'always_keep', got %q", rule.Effect)
	}
}

func TestCreateProtection_ValidWithLegacyTypeIntensity(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{"field":"genre","operator":"==","value":"Horror","type":"target","intensity":"strong"}`
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/custom-rules", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var rule db.CustomRule
	if err := json.Unmarshal(rec.Body.Bytes(), &rule); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	// Legacy type=target + intensity=strong should auto-populate effect=prefer_remove
	if rule.Effect != "prefer_remove" {
		t.Errorf("Expected auto-populated effect 'prefer_remove', got %q", rule.Effect)
	}
}

func TestCreateProtection_MissingRequiredFields(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	tests := []struct {
		name string
		body string
	}{
		{"missing field", `{"operator":"==","value":"test","effect":"always_keep"}`},
		{"missing operator", `{"field":"title","value":"test","effect":"always_keep"}`},
		{"missing value", `{"field":"title","operator":"==","effect":"always_keep"}`},
		{"empty field", `{"field":"","operator":"==","value":"test","effect":"always_keep"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/custom-rules", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("Expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCreateProtection_MissingEffectAndTypeIntensity(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Has field/operator/value but no effect and no type+intensity
	body := `{"field":"title","operator":"contains","value":"test"}`
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/custom-rules", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 without effect or type+intensity, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateProtection_InvalidEffect(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{"field":"title","operator":"==","value":"test","effect":"invalid_effect"}`
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/custom-rules", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid effect, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateProtection_AllValidEffects(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	effects := []string{"always_keep", "prefer_keep", "lean_keep", "lean_remove", "prefer_remove", "always_remove"}

	for _, effect := range effects {
		t.Run(effect, func(t *testing.T) {
			body := fmt.Sprintf(`{"field":"title","operator":"contains","value":"test-%s","effect":"%s"}`, effect, effect)
			req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/custom-rules", strings.NewReader(body))
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusCreated {
				t.Errorf("Expected 201 for effect %q, got %d: %s", effect, rec.Code, rec.Body.String())
			}
		})
	}
}

// ---------- PUT /api/custom-rules/:id ----------

func TestUpdateProtection_Existing(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	rule := seedRule(t, database, "title", "contains", "Old Value", "always_keep", 0)

	body := `{"field":"genre","operator":"==","value":"Comedy","effect":"prefer_remove","enabled":true}`
	path := fmt.Sprintf("/api/custom-rules/%d", rule.ID)
	req := testutil.AuthenticatedRequest(t, http.MethodPut, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated db.CustomRule
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if updated.ID != rule.ID {
		t.Errorf("Expected ID %d preserved, got %d", rule.ID, updated.ID)
	}
	if updated.Value != "Comedy" {
		t.Errorf("Expected value 'Comedy', got %q", updated.Value)
	}
	if updated.Effect != "prefer_remove" {
		t.Errorf("Expected effect 'prefer_remove', got %q", updated.Effect)
	}
}

func TestUpdateProtection_NotFound(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{"field":"title","operator":"==","value":"test","effect":"always_keep"}`
	req := testutil.AuthenticatedRequest(t, http.MethodPut, "/api/custom-rules/99999", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for non-existent rule, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------- DELETE /api/custom-rules/:id ----------

func TestDeleteProtection_Existing(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	rule := seedRule(t, database, "title", "contains", "Delete Me", "always_keep", 0)

	path := fmt.Sprintf("/api/custom-rules/%d", rule.ID)
	req := testutil.AuthenticatedRequest(t, http.MethodDelete, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("Expected 204, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify rule was deleted
	var count int64
	database.Model(&db.CustomRule{}).Where("id = ?", rule.ID).Count(&count)
	if count != 0 {
		t.Errorf("Expected rule to be deleted, but found %d matching rows", count)
	}
}

func TestDeleteProtection_NonExistentID(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// The handler now looks up the rule before deleting (to include details in
	// the activity event), so a non-existent ID returns 404.
	req := testutil.AuthenticatedRequest(t, http.MethodDelete, "/api/custom-rules/99999", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for non-existent ID, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------- PUT /api/custom-rules/reorder ----------

func TestReorderProtections_Valid(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	rules := seedRules(t, database, 3)

	// Reverse the order
	body := fmt.Sprintf(`{"order":[%d,%d,%d]}`, rules[2].ID, rules[1].ID, rules[0].ID)
	req := testutil.AuthenticatedRequest(t, http.MethodPut, "/api/custom-rules/reorder", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify sort_order was updated
	var reordered []db.CustomRule
	database.Order("sort_order ASC").Find(&reordered)

	if len(reordered) != 3 {
		t.Fatalf("Expected 3 rules, got %d", len(reordered))
	}
	// First rule should now be what was originally the last
	if reordered[0].ID != rules[2].ID {
		t.Errorf("Expected first rule to be ID %d, got %d", rules[2].ID, reordered[0].ID)
	}
	if reordered[2].ID != rules[0].ID {
		t.Errorf("Expected last rule to be ID %d, got %d", rules[0].ID, reordered[2].ID)
	}
}

func TestReorderProtections_EmptyOrder(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{"order":[]}`
	req := testutil.AuthenticatedRequest(t, http.MethodPut, "/api/custom-rules/reorder", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty order, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestReorderProtections_InvalidPayload(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `not json`
	req := testutil.AuthenticatedRequest(t, http.MethodPut, "/api/custom-rules/reorder", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid JSON, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------- GET /api/rule-fields ----------

func TestGetRuleFields_NoFilter(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/rule-fields", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var fields []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &fields); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Should return at least the base fields + type field
	if len(fields) < 10 {
		t.Errorf("Expected at least 10 base fields, got %d", len(fields))
	}

	// Verify the structure of returned fields
	for _, f := range fields {
		if f["field"] == nil {
			t.Error("Expected 'field' key in field definition")
		}
		if f["label"] == nil {
			t.Error("Expected 'label' key in field definition")
		}
		if f["type"] == nil {
			t.Error("Expected 'type' key in field definition")
		}
		if f["operators"] == nil {
			t.Error("Expected 'operators' key in field definition")
		}
	}

	// Verify "type" (Media Type) field is always present
	found := false
	for _, f := range fields {
		if f["field"] == "type" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected 'type' (Media Type) field to always be present")
	}
}

func TestGetRuleFields_SonarrFilter(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/rule-fields?service_type=sonarr", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var fields []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &fields); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// When filtering by sonarr, we should get sonarr-specific fields
	fieldNames := make(map[string]bool)
	for _, f := range fields {
		if name, ok := f["field"].(string); ok {
			fieldNames[name] = true
		}
	}

	// Sonarr-specific fields
	sonarrFields := []string{"seriesstatus", "seasoncount", "episodecount"}
	for _, sf := range sonarrFields {
		if !fieldNames[sf] {
			t.Errorf("Expected sonarr-specific field %q to be present with service_type=sonarr", sf)
		}
	}
}

func TestGetRuleFields_RadarrFilter(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/rule-fields?service_type=radarr", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var fields []map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &fields); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Radarr should NOT have sonarr-specific fields
	fieldNames := make(map[string]bool)
	for _, f := range fields {
		if name, ok := f["field"].(string); ok {
			fieldNames[name] = true
		}
	}

	sonarrOnly := []string{"seasoncount", "episodecount"}
	for _, sf := range sonarrOnly {
		if fieldNames[sf] {
			t.Errorf("Radarr filter should NOT include sonarr-specific field %q", sf)
		}
	}
}

// ---------- GET /api/rule-values ----------

func TestGetRuleValues_MissingParams(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	tests := []struct {
		name  string
		query string
	}{
		{"no params", ""},
		{"missing action", "?integration_id=1"},
		{"missing integration_id", "?action=quality"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/rule-values"+tc.query, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("Expected 400 for %s, got %d: %s", tc.name, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestGetRuleValues_InvalidIntegrationID(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/rule-values?integration_id=notanumber&action=quality", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid integration_id, got %d", rec.Code)
	}
}

func TestGetRuleValues_StaticActions(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	tests := []struct {
		name       string
		action     string
		expectType string // "closed" or "free"
	}{
		{"seriesstatus", "seriesstatus", "closed"},
		{"monitored", "monitored", "closed"},
		{"requested", "requested", "closed"},
		{"type", "type", "closed"},
		{"title", "title", "free"},
		{"rating", "rating", "free"},
		{"sizebytes", "sizebytes", "free"},
		{"timeinlibrary", "timeinlibrary", "free"},
		{"year", "year", "free"},
		{"seasoncount", "seasoncount", "free"},
		{"episodecount", "episodecount", "free"},
		{"playcount", "playcount", "free"},
		{"requestcount", "requestcount", "free"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := fmt.Sprintf("/api/rule-values?integration_id=1&action=%s", tc.action)
			req := testutil.AuthenticatedRequest(t, http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			respType, ok := resp["type"].(string)
			if !ok {
				t.Fatal("Expected 'type' field in response")
			}
			if respType != tc.expectType {
				t.Errorf("Expected type %q, got %q", tc.expectType, respType)
			}
		})
	}
}

func TestGetRuleValues_ClosedOptionsHaveValues(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Test that closed-type static actions return options
	closedActions := []string{"seriesstatus", "monitored", "type"}

	for _, action := range closedActions {
		t.Run(action, func(t *testing.T) {
			path := fmt.Sprintf("/api/rule-values?integration_id=1&action=%s", action)
			req := testutil.AuthenticatedRequest(t, http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
			}

			var resp map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			options, ok := resp["options"].([]interface{})
			if !ok {
				t.Fatal("Expected 'options' array in closed response")
			}
			if len(options) == 0 {
				t.Errorf("Expected non-empty options for %q", action)
			}
		})
	}
}

func TestGetRuleValues_DynamicAction_IntegrationNotFound(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Request a dynamic action (quality) for a non-existent integration
	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/rule-values?integration_id=99999&action=quality", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for non-existent integration, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetRuleValues_UnknownAction(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed an integration so we get past the "not found" check
	cfg := db.IntegrationConfig{
		Type:    "sonarr",
		Name:    "Test Sonarr",
		URL:     "http://localhost:8989",
		APIKey:  "test-key-12345",
		Enabled: true,
	}
	if err := database.Create(&cfg).Error; err != nil {
		t.Fatalf("Failed to seed integration: %v", err)
	}

	path := fmt.Sprintf("/api/rule-values?integration_id=%d&action=unknownfield", cfg.ID)
	req := testutil.AuthenticatedRequest(t, http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for unknown action, got %d: %s", rec.Code, rec.Body.String())
	}
}
