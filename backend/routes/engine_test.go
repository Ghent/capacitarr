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

	// Insert a few engine run stats
	now := time.Now().UTC()
	stats := []db.EngineRunStats{
		{RunAt: now.Add(-6 * time.Hour), Evaluated: 100, Flagged: 5, Deleted: 3, FreedBytes: 1000000, DurationMs: 250, ExecutionMode: "auto"},
		{RunAt: now.Add(-3 * time.Hour), Evaluated: 80, Flagged: 2, Deleted: 1, FreedBytes: 500000, DurationMs: 180, ExecutionMode: "auto"},
		{RunAt: now.Add(-1 * time.Hour), Evaluated: 120, Flagged: 8, Deleted: 6, FreedBytes: 2000000, DurationMs: 320, ExecutionMode: "dry-run"},
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

	var points []map[string]interface{}
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
	if first["flagged"].(float64) != 5 {
		t.Errorf("expected flagged=5, got %v", first["flagged"])
	}
	if first["deleted"].(float64) != 3 {
		t.Errorf("expected deleted=3, got %v", first["deleted"])
	}
}

func TestEngineHistory_DefaultRange(t *testing.T) {
	database := testutil.SetupTestDB(t)

	// Insert one old stat (10 days ago) and one recent (1 day ago)
	now := time.Now().UTC()
	database.Create(&db.EngineRunStats{RunAt: now.Add(-10 * 24 * time.Hour), Evaluated: 50, ExecutionMode: "auto"})
	database.Create(&db.EngineRunStats{RunAt: now.Add(-1 * 24 * time.Hour), Evaluated: 100, ExecutionMode: "auto"})

	e := testutil.SetupTestServer(t, database)

	// Default range is 7d
	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/engine/history", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var points []map[string]interface{}
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
