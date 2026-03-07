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

// seedIntegration creates an IntegrationConfig in the test database and returns it.
func seedIntegration(t *testing.T, database *gorm.DB, intType, name string) db.IntegrationConfig {
	t.Helper()
	ic := db.IntegrationConfig{
		Type:    intType,
		Name:    name,
		URL:     "http://localhost:8989",
		APIKey:  "test-key",
		Enabled: true,
	}
	if err := database.Create(&ic).Error; err != nil {
		t.Fatalf("Failed to seed integration: %v", err)
	}
	return ic
}

// seedRuleWithIntegration creates a CustomRule linked to a specific integration.
func seedRuleWithIntegration(t *testing.T, database *gorm.DB, field, operator, value, effect string, sortOrder int, integrationID uint) {
	t.Helper()
	id := integrationID
	rule := db.CustomRule{
		IntegrationID: &id,
		Field:         field,
		Operator:      operator,
		Value:         value,
		Effect:        effect,
		Enabled:       true,
		SortOrder:     sortOrder,
	}
	if err := database.Create(&rule).Error; err != nil {
		t.Fatalf("Failed to seed rule with integration: %v", err)
	}
}

// ---------- export response types ----------

type exportResponse struct {
	Version    int            `json:"version"`
	ExportedAt string         `json:"exportedAt"`
	Rules      []exportedRule `json:"rules"`
}

type exportedRule struct {
	Field           string  `json:"field"`
	Operator        string  `json:"operator"`
	Value           string  `json:"value"`
	Effect          string  `json:"effect"`
	Enabled         bool    `json:"enabled"`
	IntegrationName *string `json:"integrationName"`
	IntegrationType *string `json:"integrationType"`
}

// ---------- import response types ----------

type importSuccessResponse struct {
	Imported int `json:"imported"`
	Skipped  int `json:"skipped"`
}

type importErrorResponse struct {
	Error    string   `json:"error"`
	Unmapped []string `json:"unmapped"`
}

// ---------- GET /api/custom-rules/export ----------

func TestExportRules_Empty(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/custom-rules/export", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp exportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Version != 1 {
		t.Errorf("Expected version 1, got %d", resp.Version)
	}
	if resp.ExportedAt == "" {
		t.Error("Expected non-empty exportedAt")
	}
	if len(resp.Rules) != 0 {
		t.Errorf("Expected 0 rules, got %d", len(resp.Rules))
	}

	// Verify Content-Disposition header
	cd := rec.Header().Get("Content-Disposition")
	if cd == "" {
		t.Error("Expected Content-Disposition header")
	}
	if !strings.Contains(cd, "capacitarr-rules-") {
		t.Errorf("Content-Disposition should contain 'capacitarr-rules-', got %q", cd)
	}
}

func TestExportRules_WithRulesNoIntegration(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed rules without integration (global rules)
	seedRule(t, database, "title", "contains", "Star Trek", "always_keep", 0)
	seedRule(t, database, "quality", "==", "4K", "prefer_keep", 1)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/custom-rules/export", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp exportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(resp.Rules) != 2 {
		t.Fatalf("Expected 2 rules, got %d", len(resp.Rules))
	}

	// First rule (sort_order 0)
	if resp.Rules[0].Field != "title" {
		t.Errorf("Expected field 'title', got %q", resp.Rules[0].Field)
	}
	if resp.Rules[0].Value != "Star Trek" {
		t.Errorf("Expected value 'Star Trek', got %q", resp.Rules[0].Value)
	}
	if resp.Rules[0].IntegrationName != nil {
		t.Errorf("Expected nil integrationName, got %v", resp.Rules[0].IntegrationName)
	}
	if resp.Rules[0].IntegrationType != nil {
		t.Errorf("Expected nil integrationType, got %v", resp.Rules[0].IntegrationType)
	}
}

func TestExportRules_WithIntegration(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	ic := seedIntegration(t, database, "sonarr", "Main Sonarr")
	seedRuleWithIntegration(t, database, "title", "contains", "Star Trek", "always_keep", 0, ic.ID)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/custom-rules/export", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp exportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(resp.Rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(resp.Rules))
	}

	r := resp.Rules[0]
	if r.IntegrationName == nil || *r.IntegrationName != "Main Sonarr" {
		t.Errorf("Expected integrationName 'Main Sonarr', got %v", r.IntegrationName)
	}
	if r.IntegrationType == nil || *r.IntegrationType != "sonarr" {
		t.Errorf("Expected integrationType 'sonarr', got %v", r.IntegrationType)
	}
}

func TestExportRules_MixedIntegrations(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	ic := seedIntegration(t, database, "radarr", "Movies")
	seedRule(t, database, "title", "contains", "Global Rule", "always_keep", 0)
	seedRuleWithIntegration(t, database, "quality", "==", "4K", "prefer_remove", 1, ic.ID)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/custom-rules/export", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp exportResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(resp.Rules) != 2 {
		t.Fatalf("Expected 2 rules, got %d", len(resp.Rules))
	}

	// Global rule — nil integration
	if resp.Rules[0].IntegrationName != nil {
		t.Errorf("Expected nil integrationName for global rule")
	}
	// Integration-bound rule
	if resp.Rules[1].IntegrationName == nil || *resp.Rules[1].IntegrationName != "Movies" {
		t.Errorf("Expected integrationName 'Movies' for second rule")
	}
}

// ---------- POST /api/custom-rules/import ----------

func TestImportRules_BasicSuccess(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{
		"payload": {
			"version": 1,
			"exportedAt": "2026-03-06T22:00:00Z",
			"rules": [
				{
					"field": "title",
					"operator": "contains",
					"value": "Star Trek",
					"effect": "always_keep",
					"enabled": true
				}
			]
		},
		"integrationMapping": {}
	}`

	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/custom-rules/import", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp importSuccessResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp.Imported != 1 {
		t.Errorf("Expected imported=1, got %d", resp.Imported)
	}
	if resp.Skipped != 0 {
		t.Errorf("Expected skipped=0, got %d", resp.Skipped)
	}

	// Verify rule was actually created in DB
	var rules []db.CustomRule
	database.Find(&rules)
	if len(rules) != 1 {
		t.Fatalf("Expected 1 rule in DB, got %d", len(rules))
	}
	if rules[0].Field != "title" {
		t.Errorf("Expected field 'title', got %q", rules[0].Field)
	}
	if rules[0].IntegrationID != nil {
		t.Errorf("Expected nil integrationID, got %v", rules[0].IntegrationID)
	}
}

func TestImportRules_InvalidVersion(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{
		"payload": {
			"version": 99,
			"exportedAt": "2026-03-06T22:00:00Z",
			"rules": []
		},
		"integrationMapping": {}
	}`

	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/custom-rules/import", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestImportRules_MissingRequiredFields(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	tests := []struct {
		name string
		body string
	}{
		{
			"missing field",
			`{"payload":{"version":1,"rules":[{"operator":"contains","value":"x","effect":"always_keep"}]}}`,
		},
		{
			"missing operator",
			`{"payload":{"version":1,"rules":[{"field":"title","value":"x","effect":"always_keep"}]}}`,
		},
		{
			"missing value",
			`{"payload":{"version":1,"rules":[{"field":"title","operator":"contains","effect":"always_keep"}]}}`,
		},
		{
			"missing effect",
			`{"payload":{"version":1,"rules":[{"field":"title","operator":"contains","value":"x"}]}}`,
		},
		{
			"invalid effect",
			`{"payload":{"version":1,"rules":[{"field":"title","operator":"contains","value":"x","effect":"bogus_effect"}]}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/custom-rules/import", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("Expected 400 for %s, got %d: %s", tc.name, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestImportRules_AutoMatchIntegration(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	ic := seedIntegration(t, database, "sonarr", "Main")

	body := `{
		"payload": {
			"version": 1,
			"exportedAt": "2026-03-06T22:00:00Z",
			"rules": [
				{
					"field": "title",
					"operator": "contains",
					"value": "Star Trek",
					"effect": "always_keep",
					"enabled": true,
					"integrationName": "Main",
					"integrationType": "sonarr"
				}
			]
		}
	}`

	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/custom-rules/import", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify the rule was linked to the integration
	var rules []db.CustomRule
	database.Find(&rules)
	if len(rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(rules))
	}
	if rules[0].IntegrationID == nil {
		t.Fatal("Expected non-nil integrationID")
	}
	if *rules[0].IntegrationID != ic.ID {
		t.Errorf("Expected integrationID=%d, got %d", ic.ID, *rules[0].IntegrationID)
	}
}

func TestImportRules_ExplicitMapping(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	ic := seedIntegration(t, database, "sonarr", "New Server")

	body := fmt.Sprintf(`{
		"payload": {
			"version": 1,
			"exportedAt": "2026-03-06T22:00:00Z",
			"rules": [
				{
					"field": "title",
					"operator": "contains",
					"value": "Star Trek",
					"effect": "always_keep",
					"enabled": true,
					"integrationName": "Old Server",
					"integrationType": "sonarr"
				}
			]
		},
		"integrationMapping": {
			"sonarr:Old Server": %d
		}
	}`, ic.ID)

	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/custom-rules/import", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var rules []db.CustomRule
	database.Find(&rules)
	if len(rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(rules))
	}
	if rules[0].IntegrationID == nil || *rules[0].IntegrationID != ic.ID {
		t.Errorf("Expected integrationID=%d, got %v", ic.ID, rules[0].IntegrationID)
	}
}

func TestImportRules_UnmappedIntegration(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{
		"payload": {
			"version": 1,
			"exportedAt": "2026-03-06T22:00:00Z",
			"rules": [
				{
					"field": "title",
					"operator": "contains",
					"value": "Star Trek",
					"effect": "always_keep",
					"enabled": true,
					"integrationName": "OldServer",
					"integrationType": "sonarr"
				}
			]
		}
	}`

	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/custom-rules/import", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp importErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp.Error != "unmapped integrations" {
		t.Errorf("Expected 'unmapped integrations' error, got %q", resp.Error)
	}
	if len(resp.Unmapped) != 1 || resp.Unmapped[0] != "sonarr:OldServer" {
		t.Errorf("Expected unmapped=['sonarr:OldServer'], got %v", resp.Unmapped)
	}

	// Verify no rules were created
	var count int64
	database.Model(&db.CustomRule{}).Count(&count)
	if count != 0 {
		t.Errorf("Expected 0 rules in DB after unmapped error, got %d", count)
	}
}

func TestImportRules_SortOrderContinuation(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed existing rules so sort_order is already at 2
	seedRule(t, database, "title", "contains", "existing_0", "always_keep", 0)
	seedRule(t, database, "title", "contains", "existing_1", "always_keep", 1)
	seedRule(t, database, "title", "contains", "existing_2", "prefer_keep", 2)

	body := `{
		"payload": {
			"version": 1,
			"exportedAt": "2026-03-06T22:00:00Z",
			"rules": [
				{
					"field": "genre",
					"operator": "==",
					"value": "Horror",
					"effect": "prefer_remove",
					"enabled": true
				},
				{
					"field": "rating",
					"operator": ">",
					"value": "8",
					"effect": "always_keep",
					"enabled": true
				}
			]
		}
	}`

	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/custom-rules/import", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify the new rules have sort_order 3 and 4
	var rules []db.CustomRule
	database.Where("value IN ?", []string{"Horror", "8"}).Order("sort_order ASC").Find(&rules)
	if len(rules) != 2 {
		t.Fatalf("Expected 2 imported rules, got %d", len(rules))
	}
	if rules[0].SortOrder != 3 {
		t.Errorf("Expected first imported rule sort_order=3, got %d", rules[0].SortOrder)
	}
	if rules[1].SortOrder != 4 {
		t.Errorf("Expected second imported rule sort_order=4, got %d", rules[1].SortOrder)
	}
}

func TestImportRules_MultipleRulesMultipleIntegrations(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	sonarr := seedIntegration(t, database, "sonarr", "TV Shows")
	radarr := seedIntegration(t, database, "radarr", "Movies")

	// Auto-match by type+name — no explicit mapping needed
	body := `{
		"payload": {
			"version": 1,
			"exportedAt": "2026-03-06T22:00:00Z",
			"rules": [
				{
					"field": "title",
					"operator": "contains",
					"value": "Star Trek",
					"effect": "always_keep",
					"enabled": true,
					"integrationName": "TV Shows",
					"integrationType": "sonarr"
				},
				{
					"field": "quality",
					"operator": "==",
					"value": "4K",
					"effect": "prefer_keep",
					"enabled": false,
					"integrationName": "Movies",
					"integrationType": "radarr"
				},
				{
					"field": "genre",
					"operator": "==",
					"value": "Horror",
					"effect": "prefer_remove",
					"enabled": true
				}
			]
		}
	}`

	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/custom-rules/import", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp importSuccessResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	if resp.Imported != 3 {
		t.Errorf("Expected imported=3, got %d", resp.Imported)
	}

	// Verify DB state
	var rules []db.CustomRule
	database.Order("sort_order ASC").Find(&rules)
	if len(rules) != 3 {
		t.Fatalf("Expected 3 rules, got %d", len(rules))
	}

	// Rule 0: sonarr integration
	if rules[0].IntegrationID == nil || *rules[0].IntegrationID != sonarr.ID {
		t.Errorf("First rule should be linked to sonarr integration")
	}
	// Rule 1: radarr integration
	if rules[1].IntegrationID == nil || *rules[1].IntegrationID != radarr.ID {
		t.Errorf("Second rule should be linked to radarr integration")
	}
	// Rule 2: no integration
	if rules[2].IntegrationID != nil {
		t.Errorf("Third rule should have nil integrationID")
	}
	// Verify enabled flag is preserved
	if rules[1].Enabled {
		t.Errorf("Second rule should have enabled=false")
	}
}

func TestImportRules_DuplicateUnmappedDeduplication(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Two rules reference the same non-existent integration — unmapped should de-duplicate
	body := `{
		"payload": {
			"version": 1,
			"exportedAt": "2026-03-06T22:00:00Z",
			"rules": [
				{
					"field": "title",
					"operator": "contains",
					"value": "A",
					"effect": "always_keep",
					"enabled": true,
					"integrationName": "Gone",
					"integrationType": "sonarr"
				},
				{
					"field": "title",
					"operator": "contains",
					"value": "B",
					"effect": "always_keep",
					"enabled": true,
					"integrationName": "Gone",
					"integrationType": "sonarr"
				}
			]
		}
	}`

	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/custom-rules/import", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp importErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	if len(resp.Unmapped) != 1 {
		t.Errorf("Expected 1 deduplicated unmapped entry, got %d: %v", len(resp.Unmapped), resp.Unmapped)
	}
}

// ---------- Round-trip: export then import ----------

func TestExportImportRoundTrip(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Setup: create an integration and some rules
	ic := seedIntegration(t, database, "sonarr", "Main")
	seedRuleWithIntegration(t, database, "title", "contains", "Star Trek", "always_keep", 0, ic.ID)
	seedRule(t, database, "quality", "==", "4K", "prefer_keep", 1)

	// Step 1: Export
	exportReq := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/custom-rules/export", nil)
	exportRec := httptest.NewRecorder()
	e.ServeHTTP(exportRec, exportReq)

	if exportRec.Code != http.StatusOK {
		t.Fatalf("Export: expected 200, got %d: %s", exportRec.Code, exportRec.Body.String())
	}

	exportBody := exportRec.Body.String()

	// Step 2: Clear existing rules
	database.Where("1 = 1").Delete(&db.CustomRule{})

	// Step 3: Import (using the exported payload)
	importBody := fmt.Sprintf(`{"payload": %s}`, exportBody)
	importReq := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/custom-rules/import", strings.NewReader(importBody))
	importRec := httptest.NewRecorder()
	e.ServeHTTP(importRec, importReq)

	if importRec.Code != http.StatusOK {
		t.Fatalf("Import: expected 200, got %d: %s", importRec.Code, importRec.Body.String())
	}

	var importResp importSuccessResponse
	if err := json.Unmarshal(importRec.Body.Bytes(), &importResp); err != nil {
		t.Fatalf("Failed to parse import response: %v", err)
	}
	if importResp.Imported != 2 {
		t.Errorf("Expected imported=2, got %d", importResp.Imported)
	}

	// Verify DB now has 2 rules with correct data
	var rules []db.CustomRule
	database.Order("sort_order ASC").Find(&rules)
	if len(rules) != 2 {
		t.Fatalf("Expected 2 rules after round-trip, got %d", len(rules))
	}
	if rules[0].Value != "Star Trek" {
		t.Errorf("Expected first rule value 'Star Trek', got %q", rules[0].Value)
	}
	if rules[0].IntegrationID == nil || *rules[0].IntegrationID != ic.ID {
		t.Errorf("Expected first rule linked to integration %d", ic.ID)
	}
	if rules[1].Value != "4K" {
		t.Errorf("Expected second rule value '4K', got %q", rules[1].Value)
	}
	if rules[1].IntegrationID != nil {
		t.Errorf("Expected second rule to have nil integrationID")
	}
}
