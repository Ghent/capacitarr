package engine

import (
	"strings"
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)

// allActiveCtx returns an EvaluationContext with all integration types active.
// Used by tests where applicability filtering should not exclude any factors.
func allActiveCtx() *EvaluationContext {
	return &EvaluationContext{
		ActiveIntegrationTypes: map[integrations.IntegrationType]bool{
			integrations.IntegrationTypeSonarr:  true,
			integrations.IntegrationTypeRadarr:  true,
			integrations.IntegrationTypeSeerr:   true,
			integrations.IntegrationTypePlex:    true,
			integrations.IntegrationTypeLidarr:  true,
			integrations.IntegrationTypeReadarr: true,
		},
	}
}

// isolatedWeights returns a weight map with all weights zeroed except the
// named one, allowing a single scoring factor to be tested in isolation.
func isolatedWeights(key string, value int) map[string]int { //nolint:unparam // value is always 10 in tests but the param documents intent
	weights := map[string]int{}
	weights[key] = value
	return weights
}

func TestCalculateScore_AllZeroWeights(t *testing.T) {
	item := integrations.MediaItem{PlayCount: 5, SizeBytes: 10 * 1024 * 1024 * 1024}
	weights := map[string]int{} // all zero

	score, reason, factors := calculateScore(item, DefaultFactors(), weights, allActiveCtx())
	if score != 0.0 {
		t.Errorf("Expected 0.0 with zero weights, got %v", score)
	}
	if !strings.Contains(reason, "All preference weights are zero") {
		t.Errorf("Expected zero-weight reason, got: %s", reason)
	}
	if factors != nil {
		t.Errorf("Expected nil factors with zero weights, got %d factors", len(factors))
	}
}

func TestCalculateScore_WatchHistory(t *testing.T) {
	tests := []struct {
		name      string
		playCount int
		minScore  float64
		maxScore  float64
	}{
		{"0 plays = max deletion score", 0, 1.0, 1.0},
		{"1 play = moderate score", 1, 0.49, 0.51},
		{"2 plays = lower score", 2, 0.24, 0.26},
		{"10 plays = very low score", 10, 0.04, 0.06},
		{"100 plays = near-zero score", 100, 0.0, 0.01},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := integrations.MediaItem{PlayCount: tc.playCount}
			weights := isolatedWeights("watch_history", 10)

			score, _, factors := calculateScore(item, DefaultFactors(), weights, allActiveCtx())
			if score < tc.minScore || score > tc.maxScore {
				t.Errorf("Expected score in [%v, %v], got %v", tc.minScore, tc.maxScore, score)
			}
			// Verify raw score bounds (0.0–1.0)
			for _, f := range factors {
				if f.Name == "Play History" && (f.RawScore < 0.0 || f.RawScore > 1.0) {
					t.Errorf("Watch History raw score out of bounds: %v", f.RawScore)
				}
			}
		})
	}
}

func TestCalculateScore_LastWatched(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	sixMonthsAgo := now.Add(-183 * 24 * time.Hour)
	twoYearsAgo := now.Add(-2 * 365 * 24 * time.Hour)

	tests := []struct {
		name       string
		lastPlayed *time.Time
		minScore   float64
		maxScore   float64
	}{
		{"nil lastPlayed = max score", nil, 1.0, 1.0},
		{"yesterday = very low score", &yesterday, 0.0, 0.01},
		{"6 months ago = moderate score", &sixMonthsAgo, 0.49, 0.52},
		{"2 years ago = capped at 1.0", &twoYearsAgo, 1.0, 1.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := integrations.MediaItem{LastPlayed: tc.lastPlayed}
			weights := isolatedWeights("last_watched", 10)

			score, _, factors := calculateScore(item, DefaultFactors(), weights, allActiveCtx())
			if score < tc.minScore || score > tc.maxScore {
				t.Errorf("Expected score in [%v, %v], got %v", tc.minScore, tc.maxScore, score)
			}
			for _, f := range factors {
				if f.Name == "Last Played" && (f.RawScore < 0.0 || f.RawScore > 1.0) {
					t.Errorf("Last Watched raw score out of bounds: %v", f.RawScore)
				}
			}
		})
	}
}

func TestCalculateScore_FileSize(t *testing.T) {
	tests := []struct {
		name      string
		sizeBytes int64
		minScore  float64
		maxScore  float64
	}{
		{"0 bytes = score 0.0", 0, 0.0, 0.0},
		{"1 GB", 1 * 1024 * 1024 * 1024, 0.01, 0.03},
		{"25 GB = score ~0.5", 25 * 1024 * 1024 * 1024, 0.49, 0.51},
		{"40 GB = score ~0.8", 40 * 1024 * 1024 * 1024, 0.79, 0.81},
		{"50 GB = score 1.0", 50 * 1024 * 1024 * 1024, 1.0, 1.0},
		{"100 GB = capped at 1.0", 100 * 1024 * 1024 * 1024, 1.0, 1.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := integrations.MediaItem{SizeBytes: tc.sizeBytes}
			weights := isolatedWeights("file_size", 10)

			score, _, factors := calculateScore(item, DefaultFactors(), weights, allActiveCtx())
			if score < tc.minScore || score > tc.maxScore {
				t.Errorf("Expected score in [%v, %v], got %v", tc.minScore, tc.maxScore, score)
			}
			for _, f := range factors {
				if f.Name == "File Size" && (f.RawScore < 0.0 || f.RawScore > 1.0) {
					t.Errorf("File Size raw score out of bounds: %v", f.RawScore)
				}
			}
		})
	}
}

func TestCalculateScore_Rating(t *testing.T) {
	tests := []struct {
		name     string
		rating   float64
		minScore float64
		maxScore float64
	}{
		{"rating 0 (unknown) = default 0.5", 0, 0.5, 0.5},
		{"rating 10 = score 0.0 (excellent)", 10, 0.0, 0.0},
		{"rating 5 = score 0.5 (average)", 5, 0.5, 0.5},
		{"rating 1 = score 0.9 (poor)", 1, 0.9, 0.9},
		{"rating 3 = score 0.7", 3, 0.7, 0.7},
		{"rating 75 (100-scale) = score 0.25", 75, 0.25, 0.25},
		{"rating -1 (invalid) = default 0.5", -1, 0.5, 0.5},
		{"rating 150 (out of both scales) = default 0.5", 150, 0.5, 0.5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := integrations.MediaItem{Rating: tc.rating}
			weights := isolatedWeights("rating", 10)

			score, _, factors := calculateScore(item, DefaultFactors(), weights, allActiveCtx())
			if score < tc.minScore-0.001 || score > tc.maxScore+0.001 {
				t.Errorf("Expected score in [%v, %v], got %v", tc.minScore, tc.maxScore, score)
			}
			for _, f := range factors {
				if f.Name == "Rating" && (f.RawScore < 0.0 || f.RawScore > 1.0) {
					t.Errorf("Rating raw score out of bounds: %v", f.RawScore)
				}
			}
		})
	}
}

func TestCalculateScore_TimeInLibrary(t *testing.T) {
	now := time.Now()
	oneWeekAgo := now.Add(-7 * 24 * time.Hour)
	sixMonthsAgo := now.Add(-183 * 24 * time.Hour)
	twoYearsAgo := now.Add(-2 * 365 * 24 * time.Hour)

	tests := []struct {
		name     string
		addedAt  *time.Time
		minScore float64
		maxScore float64
	}{
		{"nil addedAt = default 0.5", nil, 0.5, 0.5},
		{"1 week ago = low score", &oneWeekAgo, 0.01, 0.03},
		{"6 months ago = moderate score", &sixMonthsAgo, 0.49, 0.52},
		{"2 years ago = capped at 1.0", &twoYearsAgo, 1.0, 1.0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := integrations.MediaItem{AddedAt: tc.addedAt}
			weights := isolatedWeights("time_in_library", 10)

			score, _, factors := calculateScore(item, DefaultFactors(), weights, allActiveCtx())
			if score < tc.minScore || score > tc.maxScore {
				t.Errorf("Expected score in [%v, %v], got %v", tc.minScore, tc.maxScore, score)
			}
			for _, f := range factors {
				if f.Name == "Time in Library" && (f.RawScore < 0.0 || f.RawScore > 1.0) {
					t.Errorf("Time in Library raw score out of bounds: %v", f.RawScore)
				}
			}
		})
	}
}

func TestCalculateScore_SeriesStatus(t *testing.T) {
	tests := []struct {
		name         string
		mediaType    integrations.MediaType
		seriesStatus string
		expected     float64
	}{
		{"ended show = 1.0", integrations.MediaTypeShow, "ended", 1.0},
		{"Ended show (mixed case) = 1.0", integrations.MediaTypeShow, "Ended", 1.0},
		{"continuing show = 0.2", integrations.MediaTypeShow, "continuing", 0.2},
		{"Continuing show (mixed case) = 0.2", integrations.MediaTypeShow, "Continuing", 0.2},
		{"unknown status show = default 0.5", integrations.MediaTypeShow, "unknown", 0.5},
		{"empty status show = default 0.5", integrations.MediaTypeShow, "", 0.5},
		{"movie = excluded (inapplicable)", integrations.MediaTypeMovie, "", 0.0},
		{"season (ended) = 1.0", integrations.MediaTypeSeason, "ended", 1.0},
		{"season (continuing) = 0.2", integrations.MediaTypeSeason, "continuing", 0.2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			item := integrations.MediaItem{
				Type:         tc.mediaType,
				SeriesStatus: tc.seriesStatus,
			}
			weights := isolatedWeights("series_status", 10)

			score, _, factors := calculateScore(item, DefaultFactors(), weights, allActiveCtx())
			if score < tc.expected-0.001 || score > tc.expected+0.001 {
				t.Errorf("Expected score ~%v, got %v", tc.expected, score)
			}
			for _, f := range factors {
				if f.Name == "Show Status" && (f.RawScore < 0.0 || f.RawScore > 1.0) {
					t.Errorf("SeriesStatus raw score out of bounds: %v", f.RawScore)
				}
			}
		})
	}
}

func TestCalculateScore_CombinedWeights(t *testing.T) {
	item := integrations.MediaItem{
		PlayCount:    0,                       // worst watch score = 1.0
		SizeBytes:    50 * 1024 * 1024 * 1024, // max file size score = 1.0
		Rating:       1.0,                     // worst rating score = 0.9
		Type:         integrations.MediaTypeShow,
		SeriesStatus: "ended", // seriesstatus = 1.0
	}

	// All weights equal at 5
	weights := map[string]int{
		"watch_history":   5,
		"last_watched":    5,
		"file_size":       5,
		"rating":          5,
		"time_in_library": 5,
		"series_status":   5,
	}

	score, _, factors := calculateScore(item, DefaultFactors(), weights, allActiveCtx())

	// Score should be high (most factors push toward deletion)
	if score < 0.7 {
		t.Errorf("Expected high combined score, got %v", score)
	}
	if score > 1.0 {
		t.Errorf("Score should be capped at 1.0, got %v", score)
	}

	// Should have exactly 7 factors (includes request_popularity with weight 0)
	if len(factors) != 7 {
		t.Errorf("Expected 7 factors, got %d", len(factors))
	}

	// All factors should have Type "weight"
	for _, f := range factors {
		if f.Type != "weight" {
			t.Errorf("Expected factor type 'weight', got %q for %s", f.Type, f.Name)
		}
	}
}

func TestCalculateScoreReasonFormat(t *testing.T) {
	item := integrations.MediaItem{
		PlayCount:    0,
		Type:         integrations.MediaTypeShow,
		SeriesStatus: "ended",
	}
	weights := map[string]int{
		"watch_history":      5,
		"last_watched":       3,
		"file_size":          2,
		"rating":             4,
		"time_in_library":    1,
		"series_status":      5,
		"request_popularity": 0,
	}

	_, reason, _ := calculateScore(item, DefaultFactors(), weights, allActiveCtx())

	// Reason should contain all seven factor keys
	for _, label := range []string{"watch_history:", "last_watched:", "file_size:", "rating:", "time_in_library:", "series_status:", "request_popularity:"} {
		if !strings.Contains(reason, label) {
			t.Errorf("Expected reason to contain %q, got: %s", label, reason)
		}
	}

	// Should not contain the old opaque reason
	if strings.Contains(reason, "Composite relative score") {
		t.Errorf("Reason should no longer contain 'Composite relative score', got: %s", reason)
	}
}

func TestCalculateScore_FactorContributionsNormalized(t *testing.T) {
	item := integrations.MediaItem{
		PlayCount:    3,
		SizeBytes:    20 * 1024 * 1024 * 1024,
		Rating:       7.0,
		Type:         integrations.MediaTypeShow,
		SeriesStatus: "ended",
	}
	weights := map[string]int{
		"watch_history":      10,
		"last_watched":       8,
		"file_size":          6,
		"rating":             5,
		"time_in_library":    4,
		"series_status":      3,
		"request_popularity": 0,
	}

	_, _, factors := calculateScore(item, DefaultFactors(), weights, allActiveCtx())

	var totalContribution float64
	for _, f := range factors {
		if f.Contribution < 0 {
			t.Errorf("Factor %q has negative contribution: %v", f.Name, f.Contribution)
		}
		totalContribution += f.Contribution
	}

	// Total contributions should approximately equal the final score
	score, _, _ := calculateScore(item, DefaultFactors(), weights, allActiveCtx())
	if totalContribution < score-0.01 || totalContribution > score+0.01 {
		t.Errorf("Sum of contributions (%v) doesn't match final score (%v)", totalContribution, score)
	}
}

func TestEvaluateMedia_EmptyItemList(t *testing.T) {
	weights := map[string]int{"watch_history": 5}
	result := EvaluateMedia(nil, DefaultFactors(), weights, nil, allActiveCtx())
	if len(result) != 0 {
		t.Errorf("Expected empty result for nil items, got %d", len(result))
	}

	result = EvaluateMedia([]integrations.MediaItem{}, DefaultFactors(), weights, nil, allActiveCtx())
	if len(result) != 0 {
		t.Errorf("Expected empty result for empty items, got %d", len(result))
	}
}

func TestEvaluateMedia_ProtectedItemHasZeroScore(t *testing.T) {
	items := []integrations.MediaItem{
		{Title: "Protected Movie", IntegrationID: 1},
	}
	weights := map[string]int{"watch_history": 10}
	intID := uint(1)
	rules := []db.CustomRule{
		{Enabled: true, IntegrationID: &intID, Field: "title", Operator: "==", Value: "protected movie", Effect: "always_keep"},
	}

	result := EvaluateMedia(items, DefaultFactors(), weights, rules, allActiveCtx())
	if len(result) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(result))
	}
	if result[0].Score != 0.0 {
		t.Errorf("Protected item should have score 0.0, got %v", result[0].Score)
	}
	if !result[0].IsProtected {
		t.Error("Protected item should have IsProtected=true")
	}
}

func TestEvaluateMedia_RuleModifiersApplied(t *testing.T) {
	items := []integrations.MediaItem{
		{Title: "Target Movie", IntegrationID: 1, Rating: 3.0},
	}
	weights := map[string]int{"rating": 10}
	intID := uint(1)
	rules := []db.CustomRule{
		{Enabled: true, IntegrationID: &intID, Field: "rating", Operator: "<", Value: "5", Effect: "prefer_remove"}, // ×3.0
	}

	result := EvaluateMedia(items, DefaultFactors(), weights, rules, allActiveCtx())
	if len(result) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(result))
	}

	// Without the rule, score = 0.7 (rating 3/10 → 1.0 - 0.3 = 0.7)
	// With prefer_remove (×3.0), score = 0.7 × 3.0 = 2.1
	// The score can exceed 1.0 when modifiers are applied
	if result[0].Score < 1.0 {
		t.Errorf("Expected score > 1.0 with prefer_remove modifier, got %v", result[0].Score)
	}
}

func TestSortEvaluated_ScoreDescending(t *testing.T) {
	items := []EvaluatedItem{
		{Item: integrations.MediaItem{Title: "Low"}, Score: 0.2},
		{Item: integrations.MediaItem{Title: "High"}, Score: 0.9},
		{Item: integrations.MediaItem{Title: "Mid"}, Score: 0.5},
	}

	SortEvaluated(items, db.TiebreakerSizeDesc)

	if items[0].Item.Title != "High" || items[1].Item.Title != "Mid" || items[2].Item.Title != "Low" {
		t.Errorf("Expected High, Mid, Low order; got %s, %s, %s",
			items[0].Item.Title, items[1].Item.Title, items[2].Item.Title)
	}
}

func TestSortEvaluated_TiebreakerSizeDesc(t *testing.T) {
	items := []EvaluatedItem{
		{Item: integrations.MediaItem{Title: "Small", SizeBytes: 100}, Score: 0.5},
		{Item: integrations.MediaItem{Title: "Large", SizeBytes: 1000}, Score: 0.5},
	}

	SortEvaluated(items, db.TiebreakerSizeDesc)

	if items[0].Item.Title != "Large" {
		t.Errorf("Expected 'Large' first with size_desc tiebreaker, got %s", items[0].Item.Title)
	}
}

func TestSortEvaluated_TiebreakerSizeAsc(t *testing.T) {
	items := []EvaluatedItem{
		{Item: integrations.MediaItem{Title: "Large", SizeBytes: 1000}, Score: 0.5},
		{Item: integrations.MediaItem{Title: "Small", SizeBytes: 100}, Score: 0.5},
	}

	SortEvaluated(items, db.TiebreakerSizeAsc)

	if items[0].Item.Title != "Small" {
		t.Errorf("Expected 'Small' first with size_asc tiebreaker, got %s", items[0].Item.Title)
	}
}

func TestSortEvaluated_TiebreakerNameAsc(t *testing.T) {
	items := []EvaluatedItem{
		{Item: integrations.MediaItem{Title: "Zebra"}, Score: 0.5},
		{Item: integrations.MediaItem{Title: "Alpha"}, Score: 0.5},
	}

	SortEvaluated(items, db.TiebreakerNameAsc)

	if items[0].Item.Title != "Alpha" {
		t.Errorf("Expected 'Alpha' first with name_asc tiebreaker, got %s", items[0].Item.Title)
	}
}

func TestSortEvaluated_TiebreakerOldestFirst(t *testing.T) {
	old := time.Now().Add(-365 * 24 * time.Hour)
	recent := time.Now().Add(-24 * time.Hour)

	items := []EvaluatedItem{
		{Item: integrations.MediaItem{Title: "Recent", AddedAt: &recent}, Score: 0.5},
		{Item: integrations.MediaItem{Title: "Old", AddedAt: &old}, Score: 0.5},
	}

	SortEvaluated(items, db.TiebreakerOldestFirst)

	if items[0].Item.Title != "Old" {
		t.Errorf("Expected 'Old' first with oldest_first tiebreaker, got %s", items[0].Item.Title)
	}
}

func TestSortEvaluated_TiebreakerNewestFirst(t *testing.T) {
	old := time.Now().Add(-365 * 24 * time.Hour)
	recent := time.Now().Add(-24 * time.Hour)

	items := []EvaluatedItem{
		{Item: integrations.MediaItem{Title: "Old", AddedAt: &old}, Score: 0.5},
		{Item: integrations.MediaItem{Title: "Recent", AddedAt: &recent}, Score: 0.5},
	}

	SortEvaluated(items, db.TiebreakerNewestFirst)

	if items[0].Item.Title != "Recent" {
		t.Errorf("Expected 'Recent' first with newest_first tiebreaker, got %s", items[0].Item.Title)
	}
}

func TestSortEvaluated_NilAddedAt(t *testing.T) {
	old := time.Now().Add(-365 * 24 * time.Hour)

	items := []EvaluatedItem{
		{Item: integrations.MediaItem{Title: "NoDate"}, Score: 0.5},
		{Item: integrations.MediaItem{Title: "HasDate", AddedAt: &old}, Score: 0.5},
	}

	// Should not panic with nil dates
	SortEvaluated(items, db.TiebreakerOldestFirst)
	SortEvaluated(items, db.TiebreakerNewestFirst)

	if len(items) != 2 {
		t.Errorf("Expected 2 items after sorting with nil dates, got %d", len(items))
	}
}
