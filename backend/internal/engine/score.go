package engine

import (
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)

// ScoreFactor represents a single dimension of the scoring breakdown
type ScoreFactor struct {
	Name         string  `json:"name"`                   // "Watch History", "File Size", etc.
	RawScore     float64 `json:"rawScore"`               // 0.0-1.0 before weighting
	Weight       int     `json:"weight"`                 // weight applied (0-10)
	Contribution float64 `json:"contribution"`           // normalized contribution to final score
	Type         string  `json:"type"`                   // "weight" or "rule"
	MatchedValue string  `json:"matchedValue,omitempty"` // actual item value that triggered a rule match
	RuleID       *uint   `json:"ruleId,omitempty"`       // database ID of the matched custom rule (rule factors only)
}

// EvaluatedItem contains the media item, its deletion score, and explanation
type EvaluatedItem struct {
	Item            integrations.MediaItem `json:"item"`
	Score           float64                `json:"score"`
	IsProtected     bool                   `json:"isProtected"`
	Reason          string                 `json:"reason"`
	Factors         []ScoreFactor          `json:"factors"`
	QueueStatus     string                 `json:"queueStatus,omitempty"`     // "", "pending", "approved", "user_initiated", "deleting"
	ApprovalQueueID *uint                  `json:"approvalQueueId,omitempty"` // for linking to approval actions
}

// EvaluateMedia calculates deletion scores for a list of items using the
// given scoring factors and weight map. Higher score = More likely to be deleted.
//
// The factors slice determines which scoring dimensions are active, and the
// weights map provides the user-configured weight (0-10) for each factor key.
// Factors whose key is missing from the weights map use 0 (disabled).
//
// The EvaluationContext carries the set of active integration types so that
// factors implementing RequiresIntegration or MediaTypeScoped can be excluded
// when their prerequisites are not met.
func EvaluateMedia(items []integrations.MediaItem, factors []ScoringFactor, weights map[string]int, rules []db.CustomRule, ctx *EvaluationContext) []EvaluatedItem {
	evaluated := make([]EvaluatedItem, 0, len(items))

	for _, item := range items {
		slog.Debug("Evaluating item", "component", "engine", "title", item.Title,
			"type", item.Type, "size", item.SizeBytes)
		isAbsProtected, modifier, ruleReasons, ruleFactors := applyRules(item, rules)
		if isAbsProtected {
			evaluated = append(evaluated, EvaluatedItem{
				Item:        item,
				Score:       0.0,
				IsProtected: true,
				Reason:      ruleReasons,
				Factors:     ruleFactors,
			})
			continue
		}

		score, scoreReason, scoreFactors := calculateScore(item, factors, weights, ctx)
		finalScore := score * modifier

		// Merge weight factors with rule factors
		allFactors := make([]ScoreFactor, 0, len(scoreFactors)+len(ruleFactors))
		allFactors = append(allFactors, scoreFactors...)
		allFactors = append(allFactors, ruleFactors...)

		var combinedReasons string
		if ruleReasons != "" {
			combinedReasons = scoreReason + ". " + ruleReasons
		} else {
			combinedReasons = scoreReason
		}

		evaluated = append(evaluated, EvaluatedItem{
			Item:        item,
			Score:       finalScore,
			IsProtected: false,
			Reason:      combinedReasons,
			Factors:     allFactors,
		})
	}

	return evaluated
}

// calculateScore iterates over the registered scoring factors, calculates each
// factor's raw score, applies the user-configured weight, and normalizes the
// result to 0.0–1.0. Inapplicable factors (per RequiresIntegration /
// MediaTypeScoped) are excluded from both weight normalization and scoring.
func calculateScore(item integrations.MediaItem, factors []ScoringFactor, weights map[string]int, ctx *EvaluationContext) (float64, string, []ScoreFactor) {
	// Sum total weight for normalization — only applicable factors
	var totalWeight float64
	for _, f := range factors {
		if !isFactorApplicable(f, item, ctx) {
			continue
		}
		totalWeight += float64(weights[f.Key()])
	}
	if totalWeight == 0 {
		return 0.0, "All preference weights are zero", nil
	}

	var totalScore float64
	resultFactors := make([]ScoreFactor, 0, len(factors))
	reasonParts := make([]string, 0, len(factors))

	for _, f := range factors {
		if !isFactorApplicable(f, item, ctx) {
			continue
		}
		w := weights[f.Key()]
		rawScore := f.Calculate(item)
		contribution := rawScore * float64(w)
		totalScore += contribution

		normalizedContrib := contribution / totalWeight
		resultFactors = append(resultFactors, ScoreFactor{
			Name:         f.Name(),
			RawScore:     rawScore,
			Weight:       w,
			Contribution: normalizedContrib,
			Type:         "weight",
		})
		reasonParts = append(reasonParts, fmt.Sprintf("%s:%.2f", f.Key(), normalizedContrib))
	}

	// Normalize to 0.0 - 1.0
	finalScore := totalScore / totalWeight
	if finalScore > 1.0 {
		finalScore = 1.0
	}

	reason := strings.Join(reasonParts, ", ")

	slog.Debug("Score calculated", "component", "engine",
		"title", item.Title, "finalScore", fmt.Sprintf("%.4f", finalScore),
		"breakdown", reason)

	return finalScore, reason, resultFactors
}

// SortEvaluated sorts evaluated items by score descending, using the configured tiebreaker
// when scores are within tolerance (0.0001).
func SortEvaluated(evaluated []EvaluatedItem, tiebreakerMethod string) {
	const tolerance = 0.0001

	sort.SliceStable(evaluated, func(i, j int) bool {
		// Primary sort: score descending
		if math.Abs(evaluated[i].Score-evaluated[j].Score) > tolerance {
			return evaluated[i].Score > evaluated[j].Score
		}

		// Tiebreaker for equal scores
		a, b := evaluated[i].Item, evaluated[j].Item
		switch tiebreakerMethod {
		case db.TiebreakerSizeAsc:
			return a.SizeBytes < b.SizeBytes
		case db.TiebreakerNameAsc:
			return strings.ToLower(a.Title) < strings.ToLower(b.Title)
		case db.TiebreakerOldestFirst:
			if a.AddedAt == nil && b.AddedAt == nil {
				return false
			}
			if a.AddedAt == nil {
				return false
			}
			if b.AddedAt == nil {
				return true
			}
			return a.AddedAt.Before(*b.AddedAt)
		case db.TiebreakerNewestFirst:
			if a.AddedAt == nil && b.AddedAt == nil {
				return false
			}
			if a.AddedAt == nil {
				return false
			}
			if b.AddedAt == nil {
				return true
			}
			return a.AddedAt.After(*b.AddedAt)
		default: // db.TiebreakerSizeDesc — largest first
			return a.SizeBytes > b.SizeBytes
		}
	})
}
