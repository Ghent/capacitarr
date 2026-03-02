package engine

import (
	"strings"
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)

func TestCalculateScore(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	twoYearsAgo := now.Add(-2 * 365 * 24 * time.Hour)

	tests := []struct {
		name          string
		item          integrations.MediaItem
		prefs         db.PreferenceSet
		validateScore func(t *testing.T, score float64)
	}{
		{
			name: "All zero weights should return 0",
			item: integrations.MediaItem{PlayCount: 0},
			prefs: db.PreferenceSet{
				WatchHistoryWeight:  0,
				LastWatchedWeight:   0,
				FileSizeWeight:      0,
				RatingWeight:        0,
				TimeInLibraryWeight: 0,
				AvailabilityWeight:  0,
			},
			validateScore: func(t *testing.T, score float64) {
				if score != 0.0 {
					t.Errorf("Expected score 0.0 with zero weights, got %v", score)
				}
			},
		},
		{
			name: "More plays should result in lower score",
			// Comparing two scenarios is harder in a single struct, but we can verify the score is very low
			item: integrations.MediaItem{PlayCount: 10},
			prefs: db.PreferenceSet{
				WatchHistoryWeight: 10,
				// Zero out others to isolate
				LastWatchedWeight:   0,
				FileSizeWeight:      0,
				RatingWeight:        0,
				TimeInLibraryWeight: 0,
				AvailabilityWeight:  0,
			},
			validateScore: func(t *testing.T, score float64) {
				// With 10 plays, watchHistoryScore = 0.5 / 10 = 0.05
				if score > 0.1 {
					t.Errorf("Expected low score for highly watched item, got %v", score)
				}
			},
		},
		{
			name: "0 plays should result in maximum watch history score",
			item: integrations.MediaItem{PlayCount: 0},
			prefs: db.PreferenceSet{
				WatchHistoryWeight: 10,
				// Zero out others
				LastWatchedWeight:   0,
				FileSizeWeight:      0,
				RatingWeight:        0,
				TimeInLibraryWeight: 0,
				AvailabilityWeight:  0,
			},
			validateScore: func(t *testing.T, score float64) {
				// 0 plays = watchHistoryScore 1.0
				if score != 1.0 {
					t.Errorf("Expected score 1.0 for completely unwatched item, got %v", score)
				}
			},
		},
		{
			name: "Recently watched should result in lower score",
			item: integrations.MediaItem{LastPlayed: &yesterday},
			prefs: db.PreferenceSet{
				WatchHistoryWeight:  0,
				LastWatchedWeight:   10,
				FileSizeWeight:      0,
				RatingWeight:        0,
				TimeInLibraryWeight: 0,
				AvailabilityWeight:  0,
			},
			validateScore: func(t *testing.T, score float64) {
				// 1 day out of 365 => ~0.0027
				if score > 0.01 {
					t.Errorf("Expected very low score for recently watched item, got %v", score)
				}
			},
		},
		{
			name: "Watched long ago should max out score",
			item: integrations.MediaItem{LastPlayed: &twoYearsAgo},
			prefs: db.PreferenceSet{
				WatchHistoryWeight:  0,
				LastWatchedWeight:   10,
				FileSizeWeight:      0,
				RatingWeight:        0,
				TimeInLibraryWeight: 0,
				AvailabilityWeight:  0,
			},
			validateScore: func(t *testing.T, score float64) {
				if score != 1.0 {
					t.Errorf("Expected score 1.0 for item watched over a year ago, got %v", score)
				}
			},
		},
		{
			name: "Large file should have higher score",
			item: integrations.MediaItem{SizeBytes: 40 * 1024 * 1024 * 1024}, // 40GB
			prefs: db.PreferenceSet{
				WatchHistoryWeight:  0,
				LastWatchedWeight:   0,
				FileSizeWeight:      10,
				RatingWeight:        0,
				TimeInLibraryWeight: 0,
				AvailabilityWeight:  0,
			},
			validateScore: func(t *testing.T, score float64) {
				// 40 / 50 = 0.8
				if score < 0.79 || score > 0.81 {
					t.Errorf("Expected ~0.8 score for 40GB file, got %v", score)
				}
			},
		},
		{
			name: "Poor rating should have higher score",
			item: integrations.MediaItem{Rating: 3.0}, // Out of 10
			prefs: db.PreferenceSet{
				WatchHistoryWeight:  0,
				LastWatchedWeight:   0,
				FileSizeWeight:      0,
				RatingWeight:        10,
				TimeInLibraryWeight: 0,
				AvailabilityWeight:  0,
			},
			validateScore: func(t *testing.T, score float64) {
				// 1.0 - (3.0/10) = 0.7
				if score < 0.69 || score > 0.71 {
					t.Errorf("Expected ~0.7 score for 3/10 rating, got %v", score)
				}
			},
		},
		{
			name: "Ended show availability should equal 1.0",
			item: integrations.MediaItem{Type: integrations.MediaTypeShow, ShowStatus: "ended"},
			prefs: db.PreferenceSet{
				WatchHistoryWeight:  0,
				LastWatchedWeight:   0,
				FileSizeWeight:      0,
				RatingWeight:        0,
				TimeInLibraryWeight: 0,
				AvailabilityWeight:  10,
			},
			validateScore: func(t *testing.T, score float64) {
				if score != 1.0 {
					t.Errorf("Expected 1.0 score for ended show, got %v", score)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			score, _, _ := calculateScore(tc.item, tc.prefs)
			tc.validateScore(t, score)
		})
	}
}

func TestCalculateScoreReasonFormat(t *testing.T) {
	item := integrations.MediaItem{
		PlayCount:  0,
		Type:       integrations.MediaTypeShow,
		ShowStatus: "ended",
	}
	prefs := db.PreferenceSet{
		WatchHistoryWeight:  5,
		LastWatchedWeight:   3,
		FileSizeWeight:      2,
		RatingWeight:        4,
		TimeInLibraryWeight: 1,
		AvailabilityWeight:  5,
	}

	_, reason, _ := calculateScore(item, prefs)

	// Reason should contain all six factor labels
	for _, label := range []string{"Watch:", "Recency:", "Size:", "Rating:", "Age:", "Status:"} {
		if !strings.Contains(reason, label) {
			t.Errorf("Expected reason to contain %q, got: %s", label, reason)
		}
	}

	// Should not contain the old opaque reason
	if strings.Contains(reason, "Composite relative score") {
		t.Errorf("Reason should no longer contain 'Composite relative score', got: %s", reason)
	}
}
