package engine

import (
	"strings"
	"time"

	"capacitarr/internal/integrations"
)

// ScoringFactor calculates a single dimension of the deletion score.
// Each factor returns a raw score from 0.0 (least deletable) to 1.0 (most deletable).
// The engine multiplies by the user-configured weight and normalizes across all factors.
//
// Factors are self-describing: they declare their own key, display name, description,
// and default weight. This allows the system to auto-seed the scoring_factor_weights
// table and auto-populate the UI without hardcoding factor metadata anywhere else.
// Adding a new scoring dimension requires only implementing this interface and
// appending it to DefaultFactors() — no schema migration, no route changes, no UI changes.
type ScoringFactor interface {
	// Name returns the human-readable name shown in UI breakdowns.
	Name() string
	// Key returns the machine-readable key for serialization and DB storage.
	Key() string
	// Description returns a short explanation of what this factor measures,
	// shown below the weight slider in the UI.
	Description() string
	// DefaultWeight returns the initial weight (0-10) used when seeding the
	// scoring_factor_weights table for the first time.
	DefaultWeight() int
	// Calculate returns a raw score from 0.0 to 1.0 for the given item.
	Calculate(item integrations.MediaItem) float64
}

// ─── Optional capability interfaces ─────────────────────────────────────────

// RequiresIntegration is optionally implemented by scoring factors that
// depend on a specific enrichment integration being connected. When the
// required integration type is absent, the factor is excluded from scoring
// and hidden from the factor weights API.
type RequiresIntegration interface {
	RequiredIntegrationType() integrations.IntegrationType
}

// MediaTypeScoped is optionally implemented by scoring factors that are
// only meaningful for certain media types. For items of non-applicable
// types, the factor is skipped per-item and its weight excluded from
// that item's normalization.
type MediaTypeScoped interface {
	ApplicableMediaTypes() []integrations.MediaType
}

// ─── EvaluationContext ──────────────────────────────────────────────────────

// EvaluationContext carries the set of active integration types through the
// evaluation pipeline. Built from the enabled integrations at the start of
// each poll cycle and passed to the evaluator.
type EvaluationContext struct {
	ActiveIntegrationTypes map[integrations.IntegrationType]bool
}

// HasIntegrationType returns true if the given integration type is active.
func (ctx *EvaluationContext) HasIntegrationType(t integrations.IntegrationType) bool {
	return ctx.ActiveIntegrationTypes[t]
}

// NewEvaluationContext builds an EvaluationContext from a list of integration
// type strings (as stored in db.IntegrationConfig.Type). Duplicate types are
// deduplicated automatically via the map.
func NewEvaluationContext(typeStrings []string) *EvaluationContext {
	active := make(map[integrations.IntegrationType]bool, len(typeStrings))
	for _, t := range typeStrings {
		active[integrations.IntegrationType(t)] = true
	}
	return &EvaluationContext{ActiveIntegrationTypes: active}
}

// isFactorApplicable checks whether a factor should participate in scoring
// for the given item and evaluation context. Factors that don't implement
// RequiresIntegration or MediaTypeScoped are always applicable.
func isFactorApplicable(f ScoringFactor, item integrations.MediaItem, ctx *EvaluationContext) bool {
	if ri, ok := f.(RequiresIntegration); ok {
		if !ctx.HasIntegrationType(ri.RequiredIntegrationType()) {
			return false
		}
	}
	if mts, ok := f.(MediaTypeScoped); ok {
		typeMatch := false
		for _, mt := range mts.ApplicableMediaTypes() {
			if item.Type == mt {
				typeMatch = true
				break
			}
		}
		if !typeMatch {
			return false
		}
	}
	return true
}

// ─── Default scoring factors ────────────────────────────────────────────────

// DefaultFactors returns the standard set of scoring factors in priority order.
func DefaultFactors() []ScoringFactor {
	return []ScoringFactor{
		&WatchHistoryFactor{},
		&RecencyFactor{},
		&FileSizeFactor{},
		&RatingFactor{},
		&LibraryAgeFactor{},
		&SeriesStatusFactor{},
		&RequestPopularityFactor{},
	}
}

// ─── WatchHistoryFactor ─────────────────────────────────────────────────────

// WatchHistoryFactor scores items by play count. More plays = lower deletion score.
// Unwatched items (PlayCount == 0) get the maximum score of 1.0.
type WatchHistoryFactor struct{}

// Name returns the display name.
func (f *WatchHistoryFactor) Name() string { return "Play History" }

// Key returns the preference key.
func (f *WatchHistoryFactor) Key() string { return "watch_history" }

// Description returns the UI description.
func (f *WatchHistoryFactor) Description() string {
	return "Unplayed items score higher for deletion. More plays = more protected."
}

// DefaultWeight returns the initial weight.
func (f *WatchHistoryFactor) DefaultWeight() int { return 10 }

// Calculate returns 1.0 for unwatched, decaying for more plays.
func (f *WatchHistoryFactor) Calculate(item integrations.MediaItem) float64 {
	if item.PlayCount > 0 {
		return 0.5 / float64(item.PlayCount)
	}
	return 1.0
}

// ─── RecencyFactor ──────────────────────────────────────────────────────────

// RecencyFactor scores items by how recently they were watched.
// More recently watched = lower deletion score. Never watched = 1.0.
type RecencyFactor struct{}

// Name returns the display name.
func (f *RecencyFactor) Name() string { return "Last Played" }

// Key returns the preference key.
func (f *RecencyFactor) Key() string { return "last_watched" }

// Description returns the UI description.
func (f *RecencyFactor) Description() string {
	return "Media not played in a long time scores higher for deletion."
}

// DefaultWeight returns the initial weight.
func (f *RecencyFactor) DefaultWeight() int { return 8 }

// Calculate returns 1.0 for never-watched or > 365 days, proportional for recent.
func (f *RecencyFactor) Calculate(item integrations.MediaItem) float64 {
	if item.LastPlayed != nil && !item.LastPlayed.IsZero() {
		daysSincePlayed := time.Since(*item.LastPlayed).Hours() / 24.0
		if daysSincePlayed < 365 {
			return daysSincePlayed / 365.0
		}
	}
	return 1.0
}

// ─── FileSizeFactor ─────────────────────────────────────────────────────────

// FileSizeFactor scores items by file size. Bigger = higher deletion score.
// Normalized against 50GB (scores > 50GB cap at 1.0).
type FileSizeFactor struct{}

// Name returns the display name.
func (f *FileSizeFactor) Name() string { return "File Size" }

// Key returns the preference key.
func (f *FileSizeFactor) Key() string { return "file_size" }

// Description returns the UI description.
func (f *FileSizeFactor) Description() string {
	return "Larger files score higher to free more space per deletion."
}

// DefaultWeight returns the initial weight.
func (f *FileSizeFactor) DefaultWeight() int { return 6 }

// Calculate returns 0.0-1.0 proportional to size, capped at 50GB.
func (f *FileSizeFactor) Calculate(item integrations.MediaItem) float64 {
	sizeGB := float64(item.SizeBytes) / (1024 * 1024 * 1024)
	score := sizeGB / 50.0
	if score > 1.0 {
		return 1.0
	}
	return score
}

// ─── RatingFactor ───────────────────────────────────────────────────────────

// RatingFactor scores items by rating. Higher rating = lower deletion score.
type RatingFactor struct{}

// Name returns the display name.
func (f *RatingFactor) Name() string { return "Rating" }

// Key returns the preference key.
func (f *RatingFactor) Key() string { return "rating" }

// Description returns the UI description.
func (f *RatingFactor) Description() string {
	return "Low-rated content scores higher for deletion."
}

// DefaultWeight returns the initial weight.
func (f *RatingFactor) DefaultWeight() int { return 5 }

// Calculate returns 0.5 for unknown, inverted scale for rated items.
func (f *RatingFactor) Calculate(item integrations.MediaItem) float64 {
	if item.Rating > 0 && item.Rating <= 10 {
		return 1.0 - (item.Rating / 10.0)
	}
	if item.Rating > 10 && item.Rating <= 100 {
		return 1.0 - (item.Rating / 100.0)
	}
	return 0.5
}

// ─── LibraryAgeFactor ───────────────────────────────────────────────────────

// LibraryAgeFactor scores items by how long they've been in the library.
// Older items get higher scores. Normalized against 1 year.
type LibraryAgeFactor struct{}

// Name returns the display name.
func (f *LibraryAgeFactor) Name() string { return "Time in Library" }

// Key returns the preference key.
func (f *LibraryAgeFactor) Key() string { return "time_in_library" }

// Description returns the UI description.
func (f *LibraryAgeFactor) Description() string {
	return "Older content may be less valuable. Normalized against one year."
}

// DefaultWeight returns the initial weight.
func (f *LibraryAgeFactor) DefaultWeight() int { return 4 }

// Calculate returns 0.5 for unknown, proportional to age capped at 1.0.
func (f *LibraryAgeFactor) Calculate(item integrations.MediaItem) float64 {
	if item.AddedAt != nil && !item.AddedAt.IsZero() {
		daysSinceAdded := time.Since(*item.AddedAt).Hours() / 24.0
		score := daysSinceAdded / 365.0
		if score > 1.0 {
			return 1.0
		}
		return score
	}
	return 0.5
}

// ─── SeriesStatusFactor ─────────────────────────────────────────────────────

// SeriesStatusFactor scores TV shows by series status. Ended = more deletable.
// Implements MediaTypeScoped — only applies to show and season items.
type SeriesStatusFactor struct{}

// Name returns the display name.
func (f *SeriesStatusFactor) Name() string { return "Show Status" }

// Key returns the preference key.
func (f *SeriesStatusFactor) Key() string { return "series_status" }

// Description returns the UI description.
func (f *SeriesStatusFactor) Description() string {
	return "Ended or canceled shows score higher since no new episodes are expected."
}

// DefaultWeight returns the initial weight.
func (f *SeriesStatusFactor) DefaultWeight() int { return 3 }

// ApplicableMediaTypes returns the media types this factor applies to.
// Only TV shows and seasons have a meaningful series status.
func (f *SeriesStatusFactor) ApplicableMediaTypes() []integrations.MediaType {
	return []integrations.MediaType{integrations.MediaTypeShow, integrations.MediaTypeSeason}
}

// Calculate returns 1.0 for ended shows, 0.2 for continuing, 0.5 for unknown.
// Items of non-applicable types are excluded by the engine via MediaTypeScoped,
// so this method only receives show/season items.
func (f *SeriesStatusFactor) Calculate(item integrations.MediaItem) float64 {
	switch strings.ToLower(item.SeriesStatus) {
	case "ended":
		return 1.0
	case "continuing":
		return 0.2
	}
	return 0.5
}

// ─── RequestPopularityFactor ────────────────────────────────────────────────

// RequestPopularityFactor scores items by whether they were user-requested.
// Requested items get a lower deletion score (more protected).
// Implements RequiresIntegration — requires Seerr to be connected.
type RequestPopularityFactor struct{}

// Name returns the display name.
func (f *RequestPopularityFactor) Name() string { return "Request Popularity" }

// Key returns the preference key.
func (f *RequestPopularityFactor) Key() string { return "request_popularity" }

// Description returns the UI description.
func (f *RequestPopularityFactor) Description() string {
	return "Requested content is protected. Unfulfilled requests are strongly protected."
}

// DefaultWeight returns the initial weight.
func (f *RequestPopularityFactor) DefaultWeight() int { return 2 }

// RequiredIntegrationType returns the integration type that must be active
// for this factor to participate in scoring.
func (f *RequestPopularityFactor) RequiredIntegrationType() integrations.IntegrationType {
	return integrations.IntegrationTypeSeerr
}

// Calculate returns 0.1 for requested items (protect), 0.5 for unrequested.
func (f *RequestPopularityFactor) Calculate(item integrations.MediaItem) float64 {
	if item.IsRequested {
		// Requested and watched by requestor = slightly more deletable than unwatched request
		if item.WatchedByRequestor {
			return 0.3
		}
		return 0.1 // Strongly protect unfulfilled requests
	}
	return 0.5
}
