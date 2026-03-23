package routes_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"capacitarr/internal/db"
	"capacitarr/internal/testutil"
)

// ─── Library CRUD E2E tests ─────────────────────────────────────────────────

func TestLibraryE2E_FullCRUDCycle(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// 1. List — should be empty initially
	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/libraries", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /libraries: expected 200, got %d", rec.Code)
	}

	var libraries []db.Library
	if err := json.NewDecoder(rec.Body).Decode(&libraries); err != nil {
		t.Fatalf("failed to decode libraries list: %v", err)
	}
	if len(libraries) != 0 {
		t.Errorf("expected 0 libraries initially, got %d", len(libraries))
	}

	// 2. Create a library
	createBody := `{"name":"Firefly Collection"}`
	req = testutil.AuthenticatedRequest(t, http.MethodPost, "/api/libraries", bytes.NewBufferString(createBody))
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /libraries: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var created db.Library
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("failed to decode created library: %v", err)
	}
	if created.Name != "Firefly Collection" {
		t.Errorf("expected name 'Firefly Collection', got %q", created.Name)
	}
	if created.ID == 0 {
		t.Error("expected non-zero library ID")
	}

	// 3. Get by ID
	req = testutil.AuthenticatedRequest(t, http.MethodGet, "/api/libraries/1", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /libraries/1: expected 200, got %d", rec.Code)
	}

	var fetched db.Library
	if err := json.NewDecoder(rec.Body).Decode(&fetched); err != nil {
		t.Fatalf("failed to decode library: %v", err)
	}
	if fetched.Name != "Firefly Collection" {
		t.Errorf("expected name 'Firefly Collection', got %q", fetched.Name)
	}

	// 4. Update
	threshold := 85.0
	updateBody, _ := json.Marshal(map[string]interface{}{
		"name":         "Serenity Vault",
		"thresholdPct": threshold,
	})
	req = testutil.AuthenticatedRequest(t, http.MethodPut, "/api/libraries/1", bytes.NewBuffer(updateBody))
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /libraries/1: expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated db.Library
	if err := json.NewDecoder(rec.Body).Decode(&updated); err != nil {
		t.Fatalf("failed to decode updated library: %v", err)
	}
	if updated.Name != "Serenity Vault" {
		t.Errorf("expected name 'Serenity Vault', got %q", updated.Name)
	}

	// 5. Delete
	req = testutil.AuthenticatedRequest(t, http.MethodDelete, "/api/libraries/1", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /libraries/1: expected 204, got %d", rec.Code)
	}

	// 6. Verify deletion — get should 404
	req = testutil.AuthenticatedRequest(t, http.MethodGet, "/api/libraries/1", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("GET after DELETE: expected 404, got %d", rec.Code)
	}
}

func TestLibraryE2E_CreateNoName(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{"name":""}`
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/libraries", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty name, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLibraryE2E_DeleteNonExistent(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodDelete, "/api/libraries/999", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent library, got %d", rec.Code)
	}
}

func TestLibraryE2E_UnauthenticatedReturns401(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/api/libraries", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated, got %d", rec.Code)
	}
}
