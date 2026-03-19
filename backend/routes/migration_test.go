package routes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"capacitarr/internal/testutil"
)

func TestMigrationStatus_NoDatabase(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Migration status is a public endpoint (no auth required)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/migration/status", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Available bool   `json:"available"`
		SourceDB  string `json:"sourceDb"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Available {
		t.Error("Expected available=false when no 1.x database exists")
	}
}

func TestMigrationExecute_NoDatabase(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/migration/execute", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("Expected 409 (no 1.x database), got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMigrationExecute_Unauthenticated(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/migration/execute", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Error("Expected non-200 for unauthenticated migration execute request")
	}
}
