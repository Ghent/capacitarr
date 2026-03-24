package engine

import (
	"testing"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)

func defaultWeights() map[string]int {
	return map[string]int{
		"watch_history":      10,
		"last_watched":       8,
		"file_size":          6,
		"rating":             5,
		"time_in_library":    4,
		"series_status":      3,
		"request_popularity": 2,
	}
}

func TestEvaluator_Evaluate(t *testing.T) {
	eval := NewEvaluator()

	items := []integrations.MediaItem{
		{Title: "Serenity", SizeBytes: 10 * 1024 * 1024 * 1024, PlayCount: 0},
		{Title: "Firefly", Type: integrations.MediaTypeShow, SizeBytes: 5 * 1024 * 1024 * 1024, PlayCount: 3},
	}

	weights := defaultWeights()
	rules := []db.CustomRule{}

	result := eval.Evaluate(items, weights, rules, db.TiebreakerSizeDesc, allActiveCtx())

	if result.TotalCount != 2 {
		t.Errorf("expected TotalCount 2, got %d", result.TotalCount)
	}
	if len(result.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(result.Items))
	}
	if len(result.Protected) != 0 {
		t.Errorf("expected 0 protected items, got %d", len(result.Protected))
	}
	if len(result.Candidates) != 2 {
		t.Errorf("expected 2 candidates, got %d", len(result.Candidates))
	}

	// Unwatched item should score higher (more deletable)
	if result.Items[0].Item.Title != "Serenity" {
		t.Errorf("expected Serenity (unwatched) first, got %s", result.Items[0].Item.Title)
	}
}

func TestEvaluator_EvaluateWithProtection(t *testing.T) {
	eval := NewEvaluator()

	intID := uint(1)
	items := []integrations.MediaItem{
		{Title: "Serenity", SizeBytes: 10 * 1024 * 1024 * 1024, PlayCount: 0, Rating: 5.0, IntegrationID: 1},
		{Title: "Firefly", Type: integrations.MediaTypeShow, SizeBytes: 5 * 1024 * 1024 * 1024, PlayCount: 3, Rating: 9.0, IntegrationID: 1},
	}

	weights := defaultWeights()
	rules := []db.CustomRule{
		{ID: 1, IntegrationID: &intID, Field: "title", Operator: "==", Value: "Firefly", Effect: "always_keep", Enabled: true},
	}

	result := eval.Evaluate(items, weights, rules, db.TiebreakerSizeDesc, allActiveCtx())

	if len(result.Protected) != 1 {
		t.Fatalf("expected 1 protected item, got %d", len(result.Protected))
	}
	if result.Protected[0].Item.Title != "Firefly" {
		t.Errorf("expected Firefly to be protected, got %s", result.Protected[0].Item.Title)
	}
	if len(result.Candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(result.Candidates))
	}
}

func TestEvaluationResult_CandidatesForDeletion(t *testing.T) {
	eval := NewEvaluator()

	items := []integrations.MediaItem{
		{Title: "Serenity", SizeBytes: 10 * 1024 * 1024 * 1024, PlayCount: 0},
		{Title: "Firefly S1", Type: integrations.MediaTypeShow, SizeBytes: 20 * 1024 * 1024 * 1024, PlayCount: 0},
		{Title: "Firefly S2", Type: integrations.MediaTypeShow, SizeBytes: 15 * 1024 * 1024 * 1024, PlayCount: 5},
	}

	weights := defaultWeights()
	result := eval.Evaluate(items, weights, []db.CustomRule{}, db.TiebreakerSizeDesc, allActiveCtx())

	// Request 15GB freed
	candidates := result.CandidatesForDeletion(15 * 1024 * 1024 * 1024)
	if len(candidates) < 1 {
		t.Error("expected at least 1 candidate to free 15GB")
	}

	// Zero bytes needed → no candidates
	candidates = result.CandidatesForDeletion(0)
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates for 0 bytes, got %d", len(candidates))
	}

	// Negative → no candidates
	candidates = result.CandidatesForDeletion(-1)
	if len(candidates) != 0 {
		t.Errorf("expected 0 candidates for negative bytes, got %d", len(candidates))
	}
}

func TestEvaluator_Factors(t *testing.T) {
	eval := NewEvaluator()
	factors := eval.Factors()
	if len(factors) != 7 {
		t.Errorf("expected 7 factors, got %d", len(factors))
	}
}
