package routes_test

import (
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

	// API keys should be masked
	for _, c := range configs {
		if c.APIKey == "secret12345678" {
			t.Error("API key should be masked in list response")
		}
		if !strings.Contains(c.APIKey, "...") {
			t.Errorf("Expected masked API key containing '...', got %q", c.APIKey)
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

func TestIntegrationsCRUD_Unauthenticated(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := httptest.NewRequest(http.MethodGet, "/api/integrations", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rec.Code)
	}
}
