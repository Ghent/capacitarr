package services

import (
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)

// mockPreviewSource provides test data for analytics services.
type mockPreviewSource struct {
	items []integrations.MediaItem
}

func (m *mockPreviewSource) GetCachedItems() []integrations.MediaItem {
	return m.items
}

// mockRulesSource provides test rules for analytics services.
type mockRulesSource struct {
	rules []db.CustomRule
	err   error
}

func (m *mockRulesSource) GetEnabledRules() ([]db.CustomRule, error) {
	return m.rules, m.err
}

func sampleItems() []integrations.MediaItem {
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
			OnWatchlist:  false, // Explicitly enriched — set to false by media server
			// To signal enrichment happened, we need at least one enrichment field set.
			// Use LastPlayed with zero time to indicate "checked but never played"
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

func TestAnalyticsService_GetSizeAnomalies(t *testing.T) {
	// Create items where one is clearly bloated for its quality+type group
	items := []integrations.MediaItem{
		{Title: "Normal 720p", QualityProfile: "HD-720p", SizeBytes: 5 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie},
		{Title: "Normal 720p 2", QualityProfile: "HD-720p", SizeBytes: 6 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie},
		{Title: "Normal 720p 3", QualityProfile: "HD-720p", SizeBytes: 4 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie},
		{Title: "Bloated 720p", QualityProfile: "HD-720p", SizeBytes: 30 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie}, // 6x median
	}

	svc := NewAnalyticsService(&mockPreviewSource{items: items})
	report := svc.GetSizeAnomalies(nil)

	if len(report.Items) == 0 {
		t.Error("expected at least one size anomaly")
	}
	if len(report.Items) > 0 && report.Items[0].Title != "Bloated 720p" {
		t.Errorf("expected 'Bloated 720p' as worst offender, got %q", report.Items[0].Title)
	}
	if len(report.Items) > 0 && report.Items[0].MediaType != "movie" {
		t.Errorf("expected MediaType 'movie', got %q", report.Items[0].MediaType)
	}
}

func TestAnalyticsService_GetSizeAnomaliesGroupsByType(t *testing.T) {
	// Shows are excluded entirely (double-counting prevention).
	// This test verifies type-grouped comparison for non-show types:
	// seasons with similar sizes should not flag each other.
	items := []integrations.MediaItem{
		// Movies: median ~5 GB
		{Title: "Serenity", QualityProfile: "HD-1080p", SizeBytes: 4 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie},
		{Title: "Movie B", QualityProfile: "HD-1080p", SizeBytes: 5 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie},
		{Title: "Movie C", QualityProfile: "HD-1080p", SizeBytes: 6 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie},
		// Seasons: median ~350 GB (large by design)
		{Title: "Firefly S01", QualityProfile: "HD-1080p", SizeBytes: 300 * 1024 * 1024 * 1024, Type: integrations.MediaTypeSeason},
		{Title: "Season B", QualityProfile: "HD-1080p", SizeBytes: 350 * 1024 * 1024 * 1024, Type: integrations.MediaTypeSeason},
		{Title: "Season C", QualityProfile: "HD-1080p", SizeBytes: 400 * 1024 * 1024 * 1024, Type: integrations.MediaTypeSeason},
	}

	svc := NewAnalyticsService(&mockPreviewSource{items: items})
	report := svc.GetSizeAnomalies(nil)

	// No anomalies expected: within each (profile, type) group, no item is > 2x median
	if len(report.Items) != 0 {
		t.Errorf("expected 0 anomalies with type-grouped comparison, got %d", len(report.Items))
		for _, a := range report.Items {
			t.Logf("  anomaly: %s (type=%s, ratio=%.2f)", a.Title, a.MediaType, a.Ratio)
		}
	}
}

func TestAnalyticsService_GetSizeAnomaliesEmpty(t *testing.T) {
	svc := NewAnalyticsService(&mockPreviewSource{items: nil})
	report := svc.GetSizeAnomalies(nil)
	if len(report.Items) != 0 {
		t.Errorf("expected 0 anomalies for empty cache, got %d", len(report.Items))
	}
}

func TestAnalyticsService_GetSizeAnomaliesExcludesProtected(t *testing.T) {
	items := []integrations.MediaItem{
		{Title: "Normal Movie", QualityProfile: "HD-720p", SizeBytes: 5 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie, Genre: "Sci-Fi"},
		{Title: "Normal Movie 2", QualityProfile: "HD-720p", SizeBytes: 6 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie, Genre: "Sci-Fi"},
		{Title: "Normal Movie 3", QualityProfile: "HD-720p", SizeBytes: 4 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie, Genre: "Sci-Fi"},
		{Title: "Protected Bloat", QualityProfile: "HD-720p", SizeBytes: 30 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie, Genre: "Sci-Fi"},
	}

	rules := []db.CustomRule{
		{ID: 1, Field: "title", Operator: "==", Value: "protected bloat", Effect: "always_keep", Enabled: true},
	}

	svc := NewAnalyticsService(&mockPreviewSource{items: items})
	svc.SetRulesSource(&mockRulesSource{rules: rules})

	report := svc.GetSizeAnomalies(nil)
	if report.ProtectedCount != 1 {
		t.Errorf("expected 1 protected item, got %d", report.ProtectedCount)
	}
	// The bloated item is protected, so no anomalies should be reported
	// (after excluding it, the remaining 3 items have similar sizes)
	for _, a := range report.Items {
		if a.Title == "Protected Bloat" {
			t.Error("protected item should not appear in anomalies")
		}
	}
}

func TestAnalyticsService_GetSizeAnomaliesIncludesNonAbsoluteProtection(t *testing.T) {
	// prefer_keep and lean_keep items should still appear as anomalies —
	// only always_keep triggers absolute protection.
	items := []integrations.MediaItem{
		{Title: "Normal Movie", QualityProfile: "HD-720p", SizeBytes: 5 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie, Genre: "Sci-Fi"},
		{Title: "Normal Movie 2", QualityProfile: "HD-720p", SizeBytes: 6 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie, Genre: "Sci-Fi"},
		{Title: "Normal Movie 3", QualityProfile: "HD-720p", SizeBytes: 4 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie, Genre: "Sci-Fi"},
		{Title: "Serenity", QualityProfile: "HD-720p", SizeBytes: 30 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie, Genre: "Sci-Fi"},
	}

	rules := []db.CustomRule{
		{ID: 1, Field: "title", Operator: "==", Value: "serenity", Effect: "prefer_keep", Enabled: true},
	}

	svc := NewAnalyticsService(&mockPreviewSource{items: items})
	svc.SetRulesSource(&mockRulesSource{rules: rules})

	report := svc.GetSizeAnomalies(nil)
	if report.ProtectedCount != 0 {
		t.Errorf("prefer_keep should not increment protectedCount, got %d", report.ProtectedCount)
	}

	found := false
	for _, a := range report.Items {
		if a.Title == "Serenity" {
			found = true
			break
		}
	}
	if !found {
		t.Error("prefer_keep item Serenity should still appear in anomalies")
	}

	// Also verify lean_keep
	rules[0].Effect = "lean_keep"
	report = svc.GetSizeAnomalies(nil)
	if report.ProtectedCount != 0 {
		t.Errorf("lean_keep should not increment protectedCount, got %d", report.ProtectedCount)
	}

	found = false
	for _, a := range report.Items {
		if a.Title == "Serenity" {
			found = true
			break
		}
	}
	if !found {
		t.Error("lean_keep item Serenity should still appear in anomalies")
	}
}

func TestAnalyticsService_GetSizeAnomaliesExcludesShows(t *testing.T) {
	// Create show items that would be anomalous if included alongside movies,
	// plus movies that are normal within their group.
	items := []integrations.MediaItem{
		{Title: "Serenity", QualityProfile: "HD-1080p", SizeBytes: 5 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie},
		{Title: "Movie B", QualityProfile: "HD-1080p", SizeBytes: 6 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie},
		{Title: "Movie C", QualityProfile: "HD-1080p", SizeBytes: 4 * 1024 * 1024 * 1024, Type: integrations.MediaTypeMovie},
		// Shows should be completely excluded (not grouped, not compared)
		{Title: "Firefly", QualityProfile: "HD-1080p", SizeBytes: 300 * 1024 * 1024 * 1024, Type: integrations.MediaTypeShow},
		{Title: "Show B", QualityProfile: "HD-1080p", SizeBytes: 350 * 1024 * 1024 * 1024, Type: integrations.MediaTypeShow},
		{Title: "Show C", QualityProfile: "HD-1080p", SizeBytes: 400 * 1024 * 1024 * 1024, Type: integrations.MediaTypeShow},
	}

	svc := NewAnalyticsService(&mockPreviewSource{items: items})
	report := svc.GetSizeAnomalies(nil)

	for _, a := range report.Items {
		if a.MediaType == "show" {
			t.Errorf("show type item %q should be excluded from size anomalies", a.Title)
		}
	}
}

// ─── Watch analytics tests ──────────────────────────────────────────────────

func TestWatchAnalyticsService_GetDeadContent(t *testing.T) {
	svc := NewWatchAnalyticsService(&mockPreviewSource{items: sampleItems()})

	// Firefly: PlayCount=0, not on watchlist, added 1 year ago — should be "dead"
	report := svc.GetDeadContent(90, nil)

	if report.TotalCount == 0 {
		t.Error("expected at least one dead content item")
	}

	found := false
	for _, item := range report.Items {
		if item.Title == "Firefly" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Firefly to appear in dead content report")
	}
}

func TestWatchAnalyticsService_GetDeadContentExcludesProtected(t *testing.T) {
	rules := []db.CustomRule{
		{ID: 1, Field: "title", Operator: "==", Value: "firefly", Effect: "always_keep", Enabled: true},
	}

	svc := NewWatchAnalyticsService(&mockPreviewSource{items: sampleItems()})
	svc.SetRulesSource(&mockRulesSource{rules: rules})

	report := svc.GetDeadContent(90, nil)

	// Firefly is protected — should be excluded
	for _, item := range report.Items {
		if item.Title == "Firefly" {
			t.Error("protected item Firefly should not appear in dead content")
		}
	}
	if report.ProtectedCount != 1 {
		t.Errorf("expected 1 protected item, got %d", report.ProtectedCount)
	}
}

func TestWatchAnalyticsService_GetDeadContentIncludesNonAbsoluteProtection(t *testing.T) {
	// prefer_keep and lean_keep items should still appear in dead content —
	// only always_keep triggers absolute exclusion.
	now := time.Now()
	oneYearAgo := now.Add(-365 * 24 * time.Hour)

	items := []integrations.MediaItem{
		{
			Title: "Firefly", Type: integrations.MediaTypeShow,
			SizeBytes: 40 * 1024 * 1024 * 1024,
			PlayCount: 0, AddedAt: &oneYearAgo,
			OnWatchlist: false,
			LastPlayed:  func() *time.Time { t := time.Time{}; return &t }(),
		},
	}

	rules := []db.CustomRule{
		{ID: 1, Field: "title", Operator: "==", Value: "firefly", Effect: "prefer_keep", Enabled: true},
	}

	svc := NewWatchAnalyticsService(&mockPreviewSource{items: items})
	svc.SetRulesSource(&mockRulesSource{rules: rules})

	report := svc.GetDeadContent(90, nil)
	if report.ProtectedCount != 0 {
		t.Errorf("prefer_keep should not increment protectedCount, got %d", report.ProtectedCount)
	}

	found := false
	for _, item := range report.Items {
		if item.Title == "Firefly" {
			found = true
			break
		}
	}
	if !found {
		t.Error("prefer_keep item Firefly should still appear in dead content")
	}

	// Also verify lean_keep
	rules[0].Effect = "lean_keep"
	report = svc.GetDeadContent(90, nil)
	if report.ProtectedCount != 0 {
		t.Errorf("lean_keep should not increment protectedCount, got %d", report.ProtectedCount)
	}

	found = false
	for _, item := range report.Items {
		if item.Title == "Firefly" {
			found = true
			break
		}
	}
	if !found {
		t.Error("lean_keep item Firefly should still appear in dead content")
	}
}

func TestWatchAnalyticsService_GetStaleContent(t *testing.T) {
	svc := NewWatchAnalyticsService(&mockPreviewSource{items: sampleItems()})

	report := svc.GetStaleContent(90, nil)

	// Serenity and The Expanse were watched 180 days ago — stale if threshold is 90 days
	if report.TotalCount == 0 {
		t.Error("expected at least one stale content item")
	}
}

func TestWatchAnalyticsService_GetStaleContentExcludesProtected(t *testing.T) {
	rules := []db.CustomRule{
		{ID: 1, Field: "title", Operator: "==", Value: "serenity", Effect: "always_keep", Enabled: true},
	}

	svc := NewWatchAnalyticsService(&mockPreviewSource{items: sampleItems()})
	svc.SetRulesSource(&mockRulesSource{rules: rules})

	report := svc.GetStaleContent(90, nil)

	for _, item := range report.Items {
		if item.Title == "Serenity" {
			t.Error("protected item Serenity should not appear in stale content")
		}
	}
	if report.ProtectedCount != 1 {
		t.Errorf("expected 1 protected item, got %d", report.ProtectedCount)
	}
}

func TestWatchAnalyticsService_GetStaleContentIncludesNonAbsoluteProtection(t *testing.T) {
	// prefer_keep and lean_keep items should still appear in stale content —
	// only always_keep triggers absolute exclusion.
	now := time.Now()
	sixMonthsAgo := now.Add(-180 * 24 * time.Hour)
	oneYearAgo := now.Add(-365 * 24 * time.Hour)

	items := []integrations.MediaItem{
		{
			Title: "Serenity", Type: integrations.MediaTypeMovie,
			SizeBytes: 15 * 1024 * 1024 * 1024,
			PlayCount: 5, LastPlayed: &sixMonthsAgo,
			AddedAt: &oneYearAgo,
		},
	}

	rules := []db.CustomRule{
		{ID: 1, Field: "title", Operator: "==", Value: "serenity", Effect: "prefer_keep", Enabled: true},
	}

	svc := NewWatchAnalyticsService(&mockPreviewSource{items: items})
	svc.SetRulesSource(&mockRulesSource{rules: rules})

	report := svc.GetStaleContent(90, nil)
	if report.ProtectedCount != 0 {
		t.Errorf("prefer_keep should not increment protectedCount, got %d", report.ProtectedCount)
	}

	found := false
	for _, item := range report.Items {
		if item.Title == "Serenity" {
			found = true
			break
		}
	}
	if !found {
		t.Error("prefer_keep item Serenity should still appear in stale content")
	}

	// Also verify lean_keep
	rules[0].Effect = "lean_keep"
	report = svc.GetStaleContent(90, nil)
	if report.ProtectedCount != 0 {
		t.Errorf("lean_keep should not increment protectedCount, got %d", report.ProtectedCount)
	}

	found = false
	for _, item := range report.Items {
		if item.Title == "Serenity" {
			found = true
			break
		}
	}
	if !found {
		t.Error("lean_keep item Serenity should still appear in stale content")
	}
}

func TestWatchAnalyticsService_GetDeadContentEmpty(t *testing.T) {
	svc := NewWatchAnalyticsService(&mockPreviewSource{items: nil})
	report := svc.GetDeadContent(90, nil)
	if report.TotalCount != 0 {
		t.Errorf("expected 0 dead items for empty cache, got %d", report.TotalCount)
	}
}
