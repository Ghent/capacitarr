package routes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/testutil"
)

// seedActivityEvents inserts n activity event records into the database for testing.
func seedActivityEvents(t *testing.T, database *gorm.DB, n int) {
	t.Helper()
	types := []string{db.EventEngineStart, db.EventEngineComplete, db.EventLogin, db.EventSettingsChanged}
	for i := 0; i < n; i++ {
		event := db.ActivityEvent{
			EventType: types[i%len(types)],
			Message:   fmt.Sprintf("Test event %d", i),
			CreatedAt: time.Now().UTC().Add(-time.Duration(i) * time.Minute),
		}
		if err := database.Create(&event).Error; err != nil {
			t.Fatalf("Failed to seed activity event %d: %v", i, err)
		}
	}
}

// ---------------------------------------------------------------------------
// GET /api/activity/recent
// ---------------------------------------------------------------------------

func TestGetActivityRecent_Empty(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/activity/recent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var events []db.ActivityEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("Expected empty slice, got %d items", len(events))
	}
}

func TestGetActivityRecent_WithData(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	seedActivityEvents(t, database, 10)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/activity/recent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var events []db.ActivityEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(events) != 5 {
		t.Errorf("Expected default limit of 5, got %d items", len(events))
	}
}

func TestGetActivityRecent_CustomLimit(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	seedActivityEvents(t, database, 10)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/activity/recent?limit=3", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var events []db.ActivityEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if len(events) != 3 {
		t.Errorf("Expected 3 items, got %d", len(events))
	}
}

func TestGetActivityRecent_MaxLimit(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	seedActivityEvents(t, database, 10)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/activity/recent?limit=999", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var events []db.ActivityEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Should be capped at 50 max, but we only seeded 10
	if len(events) != 10 {
		t.Errorf("Expected 10 items (all seeded), got %d", len(events))
	}
}

func TestGetActivityRecent_RequiresAuth(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := httptest.NewRequest(http.MethodGet, "/api/activity/recent", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rec.Code)
	}
}

func TestGetDashboardFeed_Removed(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/dashboard/feed", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// The dashboard/feed endpoint has been removed; expect 404
	if rec.Code != http.StatusNotFound {
		t.Errorf("Expected 404 for removed dashboard/feed endpoint, got %d", rec.Code)
	}
}
