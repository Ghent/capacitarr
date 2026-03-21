package services

import (
	"fmt"
	"testing"
	"time"

	"capacitarr/internal/integrations"
)

// ─── Performance benchmarks ─────────────────────────────────────────────────
// Phase 8 acceptance criteria: analytics aggregation with 10K+ items must
// complete within 1 second. These benchmarks verify that threshold.

// generateLargeDataset creates N media items with deterministic diversity of
// quality profiles, genres, years, and enrichment data. Uses index-based
// selection (not random) for reproducible benchmarks and lint compliance.
func generateLargeDataset(n int) []integrations.MediaItem {
	qualityProfiles := []string{"HD-1080p", "HD-720p", "Ultra-HD", "SD-480p", "WEBDL-1080p"}
	genres := []string{"Sci-Fi", "Drama", "Comedy", "Action", "Horror", "Thriller", "Documentary", "Animation"}
	mediaTypes := []integrations.MediaType{integrations.MediaTypeMovie, integrations.MediaTypeShow}
	seriesStatuses := []string{"continuing", "ended", ""}

	now := time.Now()
	items := make([]integrations.MediaItem, n)

	for i := range n {
		addedDaysAgo := (i%730 + 1) // 1-730 days, deterministic
		addedAt := now.Add(-time.Duration(addedDaysAgo) * 24 * time.Hour)

		item := integrations.MediaItem{
			Title:          fmt.Sprintf("Media Item %d", i),
			Type:           mediaTypes[i%len(mediaTypes)],
			SizeBytes:      int64(i%50+1) * 1024 * 1024 * 1024, // 1-50 GB
			Year:           i%50 + 1975,                        // 1975-2024
			QualityProfile: qualityProfiles[i%len(qualityProfiles)],
			Genre:          genres[i%len(genres)],
			Rating:         float64(i%90+10) / 10.0, // 1.0-10.0
			IntegrationID:  uint(i%3 + 1),
			AddedAt:        &addedAt,
			SeriesStatus:   seriesStatuses[i%len(seriesStatuses)],
		}

		// ~60% have watch data (enrichment)
		if i%5 < 3 {
			playCount := i % 20
			item.PlayCount = playCount
			if playCount > 0 {
				lastPlayed := now.Add(-time.Duration(i%365) * 24 * time.Hour)
				item.LastPlayed = &lastPlayed
			} else {
				// Enriched but never played
				zeroTime := time.Time{}
				item.LastPlayed = &zeroTime
			}
		}

		// ~15% are requested
		if i%7 == 0 {
			item.IsRequested = true
			item.RequestedBy = fmt.Sprintf("user%d", i%10)
			item.WatchedByRequestor = i%2 == 0
		}

		// ~10% are on a watchlist
		if i%10 == 0 {
			item.OnWatchlist = true
		}

		items[i] = item
	}
	return items
}

// BenchmarkWatchAnalytics_GetDeadContent_10K benchmarks dead content detection
// with 10,000 media items.
func BenchmarkWatchAnalytics_GetDeadContent_10K(b *testing.B) {
	items := generateLargeDataset(10_000)
	svc := NewWatchAnalyticsService(&mockPreviewSource{items: items})

	b.ResetTimer()
	for range b.N {
		svc.GetDeadContent(90, nil)
	}
}

// BenchmarkWatchAnalytics_GetStaleContent_10K benchmarks stale content detection
// with 10,000 media items.
func BenchmarkWatchAnalytics_GetStaleContent_10K(b *testing.B) {
	items := generateLargeDataset(10_000)
	svc := NewWatchAnalyticsService(&mockPreviewSource{items: items})

	b.ResetTimer()
	for range b.N {
		svc.GetStaleContent(180, nil)
	}
}

// BenchmarkAnalytics_AllEndpoints_10K benchmarks the full analytics suite
// sequentially (simulates a dashboard page load).
func BenchmarkAnalytics_AllEndpoints_10K(b *testing.B) {
	items := generateLargeDataset(10_000)
	watchAnalytics := NewWatchAnalyticsService(&mockPreviewSource{items: items})

	b.ResetTimer()
	for range b.N {
		watchAnalytics.GetDeadContent(90, nil)
		watchAnalytics.GetStaleContent(180, nil)
	}
}

// TestAnalytics_Performance_10K verifies the acceptance criteria:
// all analytics aggregations complete within 1 second for 10K items.
func TestAnalytics_Performance_10K(t *testing.T) {
	items := generateLargeDataset(10_000)
	watchAnalytics := NewWatchAnalyticsService(&mockPreviewSource{items: items})

	start := time.Now()

	watchAnalytics.GetDeadContent(90, nil)
	watchAnalytics.GetStaleContent(180, nil)

	elapsed := time.Since(start)
	if elapsed > 1*time.Second {
		t.Errorf("analytics suite took %v on 10K items — exceeds 1s target", elapsed)
	}
	t.Logf("analytics suite completed in %v for 10K items", elapsed)
}

// TestAnalytics_Performance_1K verifies analytics at smaller scale.
func TestAnalytics_Performance_1K(t *testing.T) {
	items := generateLargeDataset(1_000)
	watchAnalytics := NewWatchAnalyticsService(&mockPreviewSource{items: items})

	start := time.Now()

	watchAnalytics.GetDeadContent(90, nil)
	watchAnalytics.GetStaleContent(180, nil)

	elapsed := time.Since(start)
	if elapsed > 500*time.Millisecond {
		t.Errorf("analytics suite took %v on 1K items — exceeds 500ms target", elapsed)
	}
	t.Logf("analytics suite completed in %v for 1K items", elapsed)
}
