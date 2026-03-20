package engine

import (
	"fmt"
	"log/slog"
	"math"
	"sort"
	"strings"
	"time"

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

// EvaluateMedia calculates deletion scores for a list of items based on preferences and protections.
// Higher score = More likely to be deleted.
func EvaluateMedia(items []integrations.MediaItem, prefs db.PreferenceSet, rules []db.CustomRule) []EvaluatedItem {
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

		score, scoreReason, scoreFactors := calculateScore(item, prefs)
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

func calculateScore(item integrations.MediaItem, prefs db.PreferenceSet) (float64, string, []ScoreFactor) {
	var totalScore float64

	// Max total weight to normalize the score
	totalWeight := float64(prefs.WatchHistoryWeight + prefs.LastWatchedWeight + prefs.FileSizeWeight + prefs.RatingWeight + prefs.TimeInLibraryWeight + prefs.SeriesStatusWeight)
	if totalWeight == 0 {
		return 0.0, "All preference weights are zero", nil
	}

	// Watch History (More plays = lower deletion score. Unwatched = higher deletion score)
	// Example: 0 plays = 1.0 multiplier. 1 play = 0.5. >2 plays = 0.1
	watchHistoryScore := 1.0
	if item.PlayCount > 0 {
		watchHistoryScore = 0.5 / float64(item.PlayCount) // Simplistic decay
	}
	watchHistoryContrib := watchHistoryScore * float64(prefs.WatchHistoryWeight)
	totalScore += watchHistoryContrib

	// Last Watched (More recently watched = lower deletion score)
	// Example: 1 year ago = 1.0, 1 month ago = 0.2, unwatched = 1.0
	lastWatchedScore := 1.0
	if item.LastPlayed != nil && !item.LastPlayed.IsZero() {
		daysSincePlayed := time.Since(*item.LastPlayed).Hours() / 24.0
		if daysSincePlayed < 365 {
			lastWatchedScore = daysSincePlayed / 365.0
		}
	}
	recencyContrib := lastWatchedScore * float64(prefs.LastWatchedWeight)
	totalScore += recencyContrib

	// File Size (Bigger file = higher deletion score to free space)
	// Normalize to say, maximum 50GB for a season/movie?
	// Or simplistic log-based sizing
	sizeGB := float64(item.SizeBytes) / (1024 * 1024 * 1024)
	fileSizeScore := sizeGB / 50.0 // Normalize against 50GB
	if fileSizeScore > 1.0 {
		fileSizeScore = 1.0
	}
	sizeContrib := fileSizeScore * float64(prefs.FileSizeWeight)
	totalScore += sizeContrib

	// Rating (Higher rating = lower deletion score)
	// Rating is usually 0-10 or 0-100. Assume 0-10 for Sonarr/Radarr mostly? Normalize to 1.0 = bad, 0.0 = good
	ratingScore := 0.5 // default if unknown
	if item.Rating > 0 && item.Rating <= 10 {
		ratingScore = 1.0 - (item.Rating / 10.0)
	} else if item.Rating > 10 && item.Rating <= 100 { // Just in case it's percentage
		ratingScore = 1.0 - (item.Rating / 100.0)
	}
	ratingContrib := ratingScore * float64(prefs.RatingWeight)
	totalScore += ratingContrib

	// Time In Library (Older items = slightly higher chance to delete, depending on user prefs)
	timeInLibraryScore := 0.5
	if item.AddedAt != nil && !item.AddedAt.IsZero() {
		daysSinceAdded := time.Since(*item.AddedAt).Hours() / 24.0
		timeInLibraryScore = daysSinceAdded / 365.0 // Normalize against 1 year
		if timeInLibraryScore > 1.0 {
			timeInLibraryScore = 1.0
		}
	}
	ageContrib := timeInLibraryScore * float64(prefs.TimeInLibraryWeight)
	totalScore += ageContrib

	// Series Status (Ended shows = higher deletion score vs continuing shows)
	seriesStatusScore := 0.5
	if item.Type == integrations.MediaTypeShow || item.Type == integrations.MediaTypeSeason {
		if strings.ToLower(item.SeriesStatus) == "ended" {
			seriesStatusScore = 1.0
		} else if strings.ToLower(item.SeriesStatus) == "continuing" {
			seriesStatusScore = 0.2
		}
	}
	statusContrib := seriesStatusScore * float64(prefs.SeriesStatusWeight)
	totalScore += statusContrib

	// Normalize to 0.0 - 1.0
	finalScore := totalScore / totalWeight
	if finalScore > 1.0 {
		finalScore = 1.0
	}

	// Build structured factors
	factors := []ScoreFactor{
		{Name: "Watch History", RawScore: watchHistoryScore, Weight: prefs.WatchHistoryWeight, Contribution: watchHistoryContrib / totalWeight, Type: "weight"},
		{Name: "Last Watched", RawScore: lastWatchedScore, Weight: prefs.LastWatchedWeight, Contribution: recencyContrib / totalWeight, Type: "weight"},
		{Name: "File Size", RawScore: fileSizeScore, Weight: prefs.FileSizeWeight, Contribution: sizeContrib / totalWeight, Type: "weight"},
		{Name: "Rating", RawScore: ratingScore, Weight: prefs.RatingWeight, Contribution: ratingContrib / totalWeight, Type: "weight"},
		{Name: "Time in Library", RawScore: timeInLibraryScore, Weight: prefs.TimeInLibraryWeight, Contribution: ageContrib / totalWeight, Type: "weight"},
		{Name: "Series Status", RawScore: seriesStatusScore, Weight: prefs.SeriesStatusWeight, Contribution: statusContrib / totalWeight, Type: "weight"},
	}

	// Build per-factor breakdown showing each factor's normalized contribution (backward compat)
	reason := fmt.Sprintf("Watch:%.2f, Recency:%.2f, Size:%.2f, Rating:%.2f, Age:%.2f, Status:%.2f",
		watchHistoryContrib/totalWeight,
		recencyContrib/totalWeight,
		sizeContrib/totalWeight,
		ratingContrib/totalWeight,
		ageContrib/totalWeight,
		statusContrib/totalWeight,
	)
	slog.Debug("Score calculated", "component", "engine",
		"title", item.Title, "finalScore", fmt.Sprintf("%.4f", finalScore),
		"watchHistory", fmt.Sprintf("%.2f", watchHistoryContrib/totalWeight),
		"recency", fmt.Sprintf("%.2f", recencyContrib/totalWeight),
		"size", fmt.Sprintf("%.2f", sizeContrib/totalWeight),
		"rating", fmt.Sprintf("%.2f", ratingContrib/totalWeight),
		"age", fmt.Sprintf("%.2f", ageContrib/totalWeight),
		"status", fmt.Sprintf("%.2f", statusContrib/totalWeight))

	return finalScore, reason, factors
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
		case "size_asc":
			return a.SizeBytes < b.SizeBytes
		case "name_asc":
			return strings.ToLower(a.Title) < strings.ToLower(b.Title)
		case "oldest_first":
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
		case "newest_first":
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
		default: // "size_desc" — largest first
			return a.SizeBytes > b.SizeBytes
		}
	})
}
