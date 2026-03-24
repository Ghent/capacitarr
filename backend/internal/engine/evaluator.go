package engine

import (
	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)

// Evaluator performs reusable scoring and filtering of media items.
// Decoupled from the poller so it can be called from:
//   - The poller (orchestration)
//   - Library page (preview evaluations)
//   - Analytics APIs (composition analysis)
//   - Dry-run / audit reporting
type Evaluator struct {
	factors []ScoringFactor
}

// NewEvaluator creates an Evaluator with the default scoring factors.
func NewEvaluator() *Evaluator {
	return &Evaluator{factors: DefaultFactors()}
}

// NewEvaluatorWithFactors creates an Evaluator with custom scoring factors.
func NewEvaluatorWithFactors(factors []ScoringFactor) *Evaluator {
	return &Evaluator{factors: factors}
}

// EvaluationResult holds the output of an evaluation run.
type EvaluationResult struct {
	// Items is the full list of evaluated items, sorted by score descending.
	Items []EvaluatedItem
	// Protected is items matched by always_keep rules.
	Protected []EvaluatedItem
	// Candidates is non-protected items sorted by deletion priority.
	Candidates []EvaluatedItem
	// TotalCount is the number of items evaluated.
	TotalCount int
}

// Evaluate scores all items against the given weight map and rules,
// and returns a categorized result. This is the pure evaluation logic —
// no side effects, no DB writes, no queue operations.
//
// The weights map provides the user-configured weight (0-10) for each factor key.
// If a factor's key is missing from the map, it defaults to 0 (disabled).
//
// The EvaluationContext carries active integration types so that factors
// implementing RequiresIntegration or MediaTypeScoped are excluded when
// their prerequisites are not met.
func (e *Evaluator) Evaluate(items []integrations.MediaItem, weights map[string]int, rules []db.CustomRule, tiebreakerMethod string, ctx *EvaluationContext) *EvaluationResult {
	evaluated := EvaluateMedia(items, e.factors, weights, rules, ctx)
	SortEvaluated(evaluated, tiebreakerMethod)

	result := &EvaluationResult{
		Items:      evaluated,
		TotalCount: len(evaluated),
	}

	for _, item := range evaluated {
		if item.IsProtected {
			result.Protected = append(result.Protected, item)
		} else {
			result.Candidates = append(result.Candidates, item)
		}
	}

	return result
}

// CandidatesForDeletion returns items that should be considered for deletion
// to free the specified number of bytes. Items are returned in deletion
// priority order (highest score first).
func (r *EvaluationResult) CandidatesForDeletion(bytesToFree int64) []EvaluatedItem {
	if bytesToFree <= 0 {
		return nil
	}

	var candidates []EvaluatedItem
	var totalSize int64
	for _, item := range r.Candidates {
		candidates = append(candidates, item)
		totalSize += item.Item.SizeBytes
		if totalSize >= bytesToFree {
			break
		}
	}
	return candidates
}

// Factors returns the scoring factors used by this evaluator.
func (e *Evaluator) Factors() []ScoringFactor {
	return e.factors
}
