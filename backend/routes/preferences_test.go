package routes_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"capacitarr/internal/db"
	"capacitarr/internal/testutil"
)

func TestGetPreferences(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/preferences", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var pref db.PreferenceSet
	if err := json.Unmarshal(rec.Body.Bytes(), &pref); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Verify default values from seed
	if pref.WatchHistoryWeight != 10 {
		t.Errorf("Expected WatchHistoryWeight 10, got %d", pref.WatchHistoryWeight)
	}
	if pref.ExecutionMode != "dry-run" {
		t.Errorf("Expected ExecutionMode 'dry-run', got %q", pref.ExecutionMode)
	}
	if pref.LogLevel != "info" {
		t.Errorf("Expected LogLevel 'info', got %q", pref.LogLevel)
	}
}

func TestSavePreferences(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{
		"watchHistoryWeight": 5,
		"lastWatchedWeight": 3,
		"fileSizeWeight": 2,
		"ratingWeight": 4,
		"timeInLibraryWeight": 1,
		"availabilityWeight": 7,
		"executionMode": "approval",
		"tiebreakerMethod": "name_asc",
		"logLevel": "debug",
		"pollIntervalSeconds": 60,
		"auditLogRetentionDays": 7
	}`

	req := testutil.AuthenticatedRequest(t, http.MethodPut, "/api/preferences", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify persisted
	req = testutil.AuthenticatedRequest(t, http.MethodGet, "/api/preferences", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var pref db.PreferenceSet
	if err := json.Unmarshal(rec.Body.Bytes(), &pref); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if pref.WatchHistoryWeight != 5 {
		t.Errorf("Expected WatchHistoryWeight 5, got %d", pref.WatchHistoryWeight)
	}
	if pref.AvailabilityWeight != 7 {
		t.Errorf("Expected AvailabilityWeight 7, got %d", pref.AvailabilityWeight)
	}
	if pref.ExecutionMode != "approval" {
		t.Errorf("Expected ExecutionMode 'approval', got %q", pref.ExecutionMode)
	}
	if pref.TiebreakerMethod != "name_asc" {
		t.Errorf("Expected TiebreakerMethod 'name_asc', got %q", pref.TiebreakerMethod)
	}
}

func TestSavePreferences_InvalidWeight(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{
		"watchHistoryWeight": 15,
		"executionMode": "dry-run",
		"tiebreakerMethod": "size_desc",
		"logLevel": "info"
	}`
	req := testutil.AuthenticatedRequest(t, http.MethodPut, "/api/preferences", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for weight > 10, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSavePreferences_InvalidExecutionMode(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{
		"executionMode": "invalid-mode",
		"tiebreakerMethod": "size_desc",
		"logLevel": "info"
	}`
	req := testutil.AuthenticatedRequest(t, http.MethodPut, "/api/preferences", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid execution mode, got %d", rec.Code)
	}
}

func TestSavePreferences_InvalidLogLevel(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{
		"executionMode": "dry-run",
		"tiebreakerMethod": "size_desc",
		"logLevel": "verbose"
	}`
	req := testutil.AuthenticatedRequest(t, http.MethodPut, "/api/preferences", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid log level, got %d", rec.Code)
	}
}

func TestSavePreferences_InvalidTiebreaker(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{
		"executionMode": "dry-run",
		"tiebreakerMethod": "random_order",
		"logLevel": "info"
	}`
	req := testutil.AuthenticatedRequest(t, http.MethodPut, "/api/preferences", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid tiebreaker, got %d", rec.Code)
	}
}

func TestGetPreferences_Unauthenticated(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := httptest.NewRequest(http.MethodGet, "/api/preferences", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rec.Code)
	}
}
