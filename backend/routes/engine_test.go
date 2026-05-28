package routes_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/testutil"
)

func TestEngineHistory_ReturnsStats(t *testing.T) {
	database := testutil.SetupTestDB(t)

	// Insert a few engine run stats (completed_at must be set — GetHistory filters out incomplete runs)
	now := time.Now().UTC()
	completed1 := now.Add(-6*time.Hour + time.Second)
	completed2 := now.Add(-3*time.Hour + time.Second)
	completed3 := now.Add(-1*time.Hour + time.Second)
	stats := []db.EngineRunStats{
		{RunAt: now.Add(-6 * time.Hour), CompletedAt: &completed1, Evaluated: 100, Candidates: 5, Deleted: 3, FreedBytes: 1000000, DurationMs: 250},
		{RunAt: now.Add(-3 * time.Hour), CompletedAt: &completed2, Evaluated: 80, Candidates: 2, Deleted: 1, FreedBytes: 500000, DurationMs: 180},
		{RunAt: now.Add(-1 * time.Hour), CompletedAt: &completed3, Evaluated: 120, Candidates: 8, Deleted: 6, FreedBytes: 2000000, DurationMs: 320},
	}
	for _, s := range stats {
		database.Create(&s)
	}

	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/engine/history?range=24h", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var points []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &points); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(points))
	}

	// Verify first point has correct data
	first := points[0]
	if first["evaluated"].(float64) != 100 {
		t.Errorf("expected evaluated=100, got %v", first["evaluated"])
	}
	if first["candidates"].(float64) != 5 {
		t.Errorf("expected flagged=5, got %v", first["candidates"])
	}
	if first["deleted"].(float64) != 3 {
		t.Errorf("expected deleted=3, got %v", first["deleted"])
	}
}

func TestEngineHistory_DefaultRange(t *testing.T) {
	database := testutil.SetupTestDB(t)

	// Insert one old stat (10 days ago) and one recent (1 day ago)
	// completed_at must be set — GetHistory filters out incomplete runs
	now := time.Now().UTC()
	completedOld := now.Add(-10*24*time.Hour + time.Second)
	completedRecent := now.Add(-1*24*time.Hour + time.Second)
	database.Create(&db.EngineRunStats{RunAt: now.Add(-10 * 24 * time.Hour), CompletedAt: &completedOld, Evaluated: 50})
	database.Create(&db.EngineRunStats{RunAt: now.Add(-1 * 24 * time.Hour), CompletedAt: &completedRecent, Evaluated: 100})

	e := testutil.SetupTestServer(t, database)

	// Default range is 7d
	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/engine/history", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var points []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &points); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	// Only the recent one should be returned (10 days ago is outside 7d range)
	if len(points) != 1 {
		t.Fatalf("expected 1 point (7d default range), got %d", len(points))
	}
}

func TestEngineHistory_InvalidRange(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/engine/history?range=invalid", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
