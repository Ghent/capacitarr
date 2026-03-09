package routes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"capacitarr/internal/testutil"
)

// ---------- GET /api/preview ----------

func TestGetPreview_EmptyState(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/preview", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify response structure has the expected top-level keys
	if _, ok := resp["items"]; !ok {
		t.Error("Expected 'items' key in response")
	}

	// items should be an empty array (or null) when no integrations exist
	items, ok := resp["items"]
	if ok && items != nil {
		itemList, isList := items.([]any)
		if isList && len(itemList) != 0 {
			t.Errorf("Expected empty items with no integrations, got %d items", len(itemList))
		}
	}

	// diskContext should be null when no disk groups exist
	dc := resp["diskContext"]
	if dc != nil {
		t.Errorf("Expected null diskContext with no disk groups, got %v", dc)
	}
}

func TestGetPreview_ResponseStructure(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/preview", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify the JSON can be decoded and has the expected shape
	var resp struct {
		Items       any `json:"items"`
		DiskContext any `json:"diskContext"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response into expected structure: %v", err)
	}

	// Both keys must be present (even if null)
	raw := make(map[string]json.RawMessage)
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("Failed to parse raw response: %v", err)
	}
	if _, ok := raw["items"]; !ok {
		t.Error("Response missing 'items' key")
	}
	if _, ok := raw["diskContext"]; !ok {
		t.Error("Response missing 'diskContext' key")
	}
}

func TestGetPreview_Unauthenticated(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/preview", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Error("Expected non-200 for unauthenticated request to preview endpoint")
	}
}

func TestGetPreview_SkipsNonArrIntegrations(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed non-arr integrations that should be skipped by the preview handler
	nonArrTypes := []string{"plex", "tautulli", "overseerr", "jellyfin", "emby"}
	for _, intType := range nonArrTypes {
		cfg := struct {
			Type    string
			Name    string
			URL     string
			APIKey  string `json:"-"`
			Enabled bool
		}{
			Type:    intType,
			Name:    "Test " + intType,
			URL:     "http://localhost:1234",
			APIKey:  "testkey-" + intType,
			Enabled: true,
		}
		if err := database.Exec(
			"INSERT INTO integration_configs (type, name, url, api_key, enabled) VALUES (?, ?, ?, ?, ?)",
			cfg.Type, cfg.Name, cfg.URL, cfg.APIKey, cfg.Enabled,
		).Error; err != nil {
			t.Fatalf("Failed to seed %s integration: %v", intType, err)
		}
	}

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/preview", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Even with enabled non-arr integrations, items should still be empty
	// because non-arr integrations are skipped during media fetch
	items := resp["items"]
	if items != nil {
		if itemList, ok := items.([]any); ok && len(itemList) != 0 {
			t.Errorf("Expected no items from non-arr integrations, got %d", len(itemList))
		}
	}
}
