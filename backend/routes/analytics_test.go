package routes_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
	"capacitarr/internal/services"
	"capacitarr/internal/testutil"
)

// ─── E2E Integration tests ─────────────────────────────────────────────────
// These tests verify the full analytics cycle:
// populate preview cache → query analytics endpoints → verify responses.

// sampleMediaItems returns a set of representative media items for analytics testing.
// Uses canonical test names: "Serenity" (movie), "Firefly" (TV show).
func sampleMediaItems() []integrations.MediaItem {
	now := time.Now()
	sixMonthsAgo := now.Add(-180 * 24 * time.Hour)
	oneYearAgo := now.Add(-365 * 24 * time.Hour)

	return []integrations.MediaItem{
		{
			Title: "Serenity", Type: integrations.MediaTypeMovie,
			SizeBytes: 15 * 1024 * 1024 * 1024, Year: 2005,
			QualityProfile: "HD-1080p", Genre: "Sci-Fi", Rating: 7.4,
			PlayCount: 5, LastPlayed: &sixMonthsAgo,
			AddedAt: &oneYearAgo, IntegrationID: 1,
		},
		{
			Title: "Firefly", Type: integrations.MediaTypeShow,
			SizeBytes: 40 * 1024 * 1024 * 1024, Year: 2002,
			QualityProfile: "HD-1080p", Genre: "Sci-Fi", Rating: 9.0,
			PlayCount: 0, AddedAt: &oneYearAgo, IntegrationID: 1,
			SeriesStatus: "ended",
			OnWatchlist:  false,
			// Enrichment marker: zero-time LastPlayed signals "checked but never played"
			LastPlayed: func() *time.Time { t := time.Time{}; return &t }(),
		},
		{
			Title: "The Expanse", Type: integrations.MediaTypeShow,
			SizeBytes: 60 * 1024 * 1024 * 1024, Year: 2015,
			QualityProfile: "HD-720p", Genre: "Sci-Fi", Rating: 8.5,
			PlayCount: 3, LastPlayed: &sixMonthsAgo,
			AddedAt: &sixMonthsAgo, IntegrationID: 2,
			IsRequested: true, RequestedBy: "mal", WatchedByRequestor: true,
		},
		{
			Title: "Unknown Movie", Type: integrations.MediaTypeMovie,
			SizeBytes: 2 * 1024 * 1024 * 1024, Year: 0,
			IntegrationID: 1,
		},
	}
}

// seedPreviewCache populates the preview cache with sample items for E2E testing.
func seedPreviewCache(t *testing.T, reg *services.Registry, items []integrations.MediaItem) {
	t.Helper()
	prefs := db.PreferenceSet{
		WatchHistoryWeight:  10,
		LastWatchedWeight:   8,
		FileSizeWeight:      6,
		RatingWeight:        5,
		TimeInLibraryWeight: 4,
		SeriesStatusWeight:  3,
		TiebreakerMethod:    "size_desc",
	}
	reg.Preview.SetPreviewCache(items, prefs, nil)
}

func TestAnalyticsE2E_BloatEndpoint(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e, reg := testutil.SetupTestServerWithRegistry(t, database)

	// Create items with a clear bloat outlier
	items := []integrations.MediaItem{
		{Title: "Normal 720p A", QualityProfile: "HD-720p", SizeBytes: 5 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie},
		{Title: "Normal 720p B", QualityProfile: "HD-720p", SizeBytes: 6 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie},
		{Title: "Normal 720p C", QualityProfile: "HD-720p", SizeBytes: 4 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie},
		{Title: "Bloated 720p", QualityProfile: "HD-720p", SizeBytes: 30 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie},
	}
	seedPreviewCache(t, reg, items)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/analytics/bloat", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var report services.SizeAnomalyReport
	if err := json.NewDecoder(rec.Body).Decode(&report); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(report.Items) == 0 {
		t.Error("expected at least one anomaly")
	}
}

func TestAnalyticsE2E_DeadContentEndpoint(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e, reg := testutil.SetupTestServerWithRegistry(t, database)

	seedPreviewCache(t, reg, sampleMediaItems())

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/analytics/dead-content?minDays=90", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var report services.DeadContentReport
	if err := json.NewDecoder(rec.Body).Decode(&report); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Firefly has PlayCount==0, enrichment data set, added > 90 days ago
	if report.TotalCount == 0 {
		t.Error("expected at least one dead content item (Firefly)")
	}
}

func TestAnalyticsE2E_StaleContentEndpoint(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e, reg := testutil.SetupTestServerWithRegistry(t, database)

	seedPreviewCache(t, reg, sampleMediaItems())

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/analytics/stale-content?staleDays=90", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var report services.StaleContentReport
	if err := json.NewDecoder(rec.Body).Decode(&report); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Serenity and The Expanse were watched 180 days ago — stale with 90-day threshold
	if report.TotalCount == 0 {
		t.Error("expected at least one stale content item")
	}
}

func TestAnalyticsE2E_ForecastEndpoint(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e, _ := testutil.SetupTestServerWithRegistry(t, database)

	// No disk groups or history — should return empty forecast
	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/analytics/forecast", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var forecast services.CapacityForecast
	if err := json.NewDecoder(rec.Body).Decode(&forecast); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if forecast.DaysUntilThreshold != -1 {
		t.Errorf("expected -1 days until threshold (no data), got %d", forecast.DaysUntilThreshold)
	}
}

func TestAnalyticsE2E_ForecastEndpointWithDiskGroup(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e, reg := testutil.SetupTestServerWithRegistry(t, database)

	// Create a disk group
	dg := db.DiskGroup{
		MountPath:    "/data",
		TotalBytes:   1000 * 1024 * 1024 * 1024, // 1 TB
		UsedBytes:    500 * 1024 * 1024 * 1024,  // 500 GB
		ThresholdPct: 85,
		TargetPct:    75,
	}
	if err := database.Create(&dg).Error; err != nil {
		t.Fatalf("failed to create disk group: %v", err)
	}

	// Seed some history data for linear regression
	now := time.Now()
	for i := 30; i >= 0; i-- {
		h := db.LibraryHistory{
			Timestamp:     now.Add(-time.Duration(i) * 24 * time.Hour),
			TotalCapacity: 1000 * 1024 * 1024 * 1024,
			UsedCapacity:  int64(500+i) * 1024 * 1024 * 1024, // growing slightly
			Resolution:    "raw",
			DiskGroupID:   &dg.ID,
		}
		database.Create(&h)
	}
	_ = reg // registry used for future test deps

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/analytics/forecast", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var forecast services.CapacityForecast
	if err := json.NewDecoder(rec.Body).Decode(&forecast); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if forecast.TotalCapacity == 0 {
		t.Error("expected non-zero total capacity")
	}
	if forecast.UsedCapacity == 0 {
		t.Error("expected non-zero used capacity")
	}
}

// TestAnalyticsE2E_EmptyCacheReturnsDefaults verifies all analytics endpoints
// return valid (empty) responses when the preview cache has no items.
func TestAnalyticsE2E_EmptyCacheReturnsDefaults(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Preview cache is empty by default — no items seeded
	endpoints := []string{
		"/api/analytics/bloat",
		"/api/analytics/dead-content",
		"/api/analytics/stale-content",
		"/api/analytics/forecast",
	}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			req := testutil.AuthenticatedRequest(t, http.MethodGet, ep, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("expected 200 for %s, got %d: %s", ep, rec.Code, rec.Body.String())
			}
		})
	}
}

// TestAnalyticsE2E_UnauthenticatedReturns401 verifies that analytics endpoints
// require authentication.
func TestAnalyticsE2E_UnauthenticatedReturns401(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	endpoints := []string{
		"/api/analytics/bloat",
		"/api/analytics/dead-content",
		"/api/analytics/stale-content",
		"/api/analytics/forecast",
	}

	for _, ep := range endpoints {
		t.Run(ep, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, ep, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 for unauthenticated %s, got %d", ep, rec.Code)
			}
		})
	}
}
