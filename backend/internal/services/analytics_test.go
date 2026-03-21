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
