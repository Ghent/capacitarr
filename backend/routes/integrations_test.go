package routes_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"capacitarr/internal/db"
	"capacitarr/internal/testutil"
)

func TestCreateIntegration(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{
		"type": "sonarr",
		"name": "My Sonarr",
		"url": "http://localhost:8989",
		"apiKey": "abc123"
	}`
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/integrations", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var config db.IntegrationConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &config); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	if config.Name != "My Sonarr" {
		t.Errorf("Expected name 'My Sonarr', got %q", config.Name)
	}
	if config.Type != "sonarr" {
		t.Errorf("Expected type 'sonarr', got %q", config.Type)
	}
	if !config.Enabled {
		t.Error("Expected Enabled=true for new integration")
	}
}

func TestCreateIntegration_MissingFields(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	tests := []struct {
		name string
		body string
	}{
		{"missing type", `{"name":"test","url":"http://x","apiKey":"k"}`},
		{"missing name", `{"type":"sonarr","url":"http://x","apiKey":"k"}`},
		{"missing url", `{"type":"sonarr","name":"test","apiKey":"k"}`},
		{"missing apiKey", `{"type":"sonarr","name":"test","url":"http://x"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/integrations", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("Expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCreateIntegration_InvalidType(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{"type":"invalid","name":"test","url":"http://x","apiKey":"k"}`
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/integrations", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid type, got %d", rec.Code)
	}
}

func TestCreateIntegration_InvalidURL(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	tests := []struct {
		name string
		url  string
	}{
		{"ftp scheme", "ftp://example.com"},
		{"no scheme", "just-a-host"},
		{"file scheme", "file:///etc/passwd"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"type":"sonarr","name":"test","url":"%s","apiKey":"k"}`, tc.url)
			req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/integrations", strings.NewReader(body))
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("Expected 400 for invalid URL %q, got %d", tc.url, rec.Code)
			}
		})
	}
}

func TestTestIntegration_InvalidURL(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	tests := []struct {
		name string
		url  string
	}{
		{"ftp scheme", "ftp://example.com"},
		{"no scheme", "just-a-host"},
		{"file scheme", "file:///etc/passwd"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body := fmt.Sprintf(`{"type":"sonarr","url":"%s","apiKey":"k"}`, tc.url)
			req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/integrations/test", strings.NewReader(body))
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("Expected 400 for invalid URL %q, got %d: %s", tc.url, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestListIntegrations(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Create two integrations
	for _, name := range []string{"Sonarr", "Radarr"} {
		intType := "sonarr"
		if name == "Radarr" {
			intType = "radarr"
		}
		body := fmt.Sprintf(`{"type":"%s","name":"%s","url":"http://x","apiKey":"secret12345678"}`, intType, name)
		req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/integrations", strings.NewReader(body))
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("Failed to create %s: %d", name, rec.Code)
		}
	}

	// List
	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/integrations", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	var configs []db.IntegrationConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &configs); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if len(configs) != 2 {
		t.Fatalf("Expected 2 integrations, got %d", len(configs))
	}

	// API keys should be masked with bullet characters
	for _, c := range configs {
		if c.APIKey == "secret12345678" {
			t.Error("API key should be masked in list response")
		}
		if !strings.HasPrefix(c.APIKey, "•") {
			t.Errorf("Expected masked API key starting with '•', got %q", c.APIKey)
		}
		// Last 4 chars should be visible
		if !strings.HasSuffix(c.APIKey, "5678") {
			t.Errorf("Expected masked API key ending with '5678', got %q", c.APIKey)
		}
	}
}

func TestDeleteIntegration(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Create
	body := `{"type":"sonarr","name":"ToDelete","url":"http://x","apiKey":"key"}`
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/integrations", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var config db.IntegrationConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &config); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Delete
	req = testutil.AuthenticatedRequest(t, http.MethodDelete, fmt.Sprintf("/api/integrations/%d", config.ID), nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify deleted
	req = testutil.AuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/api/integrations/%d", config.ID), nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 after delete, got %d", rec.Code)
	}
}

func TestUpdateIntegration_ShowLevelOnlyAndCollectionDeletion(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Create a Sonarr integration
	body := `{"type":"sonarr","name":"Firefly Sonarr","url":"http://localhost:8989","apiKey":"abc123"}`
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/integrations", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created db.IntegrationConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("Failed to parse created integration: %v", err)
	}

	// Verify defaults are false
	if created.ShowLevelOnly {
		t.Error("Expected ShowLevelOnly=false on creation")
	}
	if created.CollectionDeletion {
		t.Error("Expected CollectionDeletion=false on creation")
	}

	// Update: enable both ShowLevelOnly and CollectionDeletion
	updateBody := `{"showLevelOnly":true,"collectionDeletion":true}`
	req = testutil.AuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/api/integrations/%d", created.ID), strings.NewReader(updateBody))
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated db.IntegrationConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("Failed to parse updated integration: %v", err)
	}

	if !updated.ShowLevelOnly {
		t.Error("Expected ShowLevelOnly=true after update")
	}
	if !updated.CollectionDeletion {
		t.Error("Expected CollectionDeletion=true after update")
	}
	// Ensure other fields were preserved
	if updated.Name != "Firefly Sonarr" {
		t.Errorf("Expected name preserved as 'Firefly Sonarr', got %q", updated.Name)
	}

	// Update: disable ShowLevelOnly via explicit false
	updateBody2 := `{"showLevelOnly":false}`
	req = testutil.AuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/api/integrations/%d", created.ID), strings.NewReader(updateBody2))
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated2 db.IntegrationConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &updated2); err != nil {
		t.Fatalf("Failed to parse second update: %v", err)
	}

	if updated2.ShowLevelOnly {
		t.Error("Expected ShowLevelOnly=false after second update")
	}
	// CollectionDeletion should be preserved (not sent in update)
	if !updated2.CollectionDeletion {
		t.Error("Expected CollectionDeletion=true to be preserved when not sent in update")
	}
}

func TestIntegrationsCRUD_Unauthenticated(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/integrations", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rec.Code)
	}
}
