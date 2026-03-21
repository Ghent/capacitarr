package engine

import (
	"strings"
	"time"

	"capacitarr/internal/integrations"
)

// ScoringFactor calculates a single dimension of the deletion score.
// Each factor returns a raw score from 0.0 (least deletable) to 1.0 (most deletable).
// The engine multiplies by the user-configured weight and normalizes across all factors.
type ScoringFactor interface {
	// Name returns the human-readable name shown in UI breakdowns.
	Name() string
	// Key returns the machine-readable key for serialization and preferences.
	Key() string
	// Calculate returns a raw score from 0.0 to 1.0 for the given item.
	Calculate(item integrations.MediaItem) float64
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
func (f *WatchHistoryFactor) Name() string { return "Watch History" }

// Key returns the preference key.
func (f *WatchHistoryFactor) Key() string { return "watch_history" }

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
func (f *RecencyFactor) Name() string { return "Last Watched" }

// Key returns the preference key.
func (f *RecencyFactor) Key() string { return "last_watched" }

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
type SeriesStatusFactor struct{}

// Name returns the display name.
func (f *SeriesStatusFactor) Name() string { return "Series Status" }

// Key returns the preference key.
func (f *SeriesStatusFactor) Key() string { return "series_status" }

// Calculate returns 1.0 for ended shows, 0.2 for continuing, 0.5 for non-TV or unknown.
func (f *SeriesStatusFactor) Calculate(item integrations.MediaItem) float64 {
	if item.Type == integrations.MediaTypeShow || item.Type == integrations.MediaTypeSeason {
		switch strings.ToLower(item.SeriesStatus) {
		case "ended":
			return 1.0
		case "continuing":
			return 0.2
		}
	}
	return 0.5
}

// ─── RequestPopularityFactor (NEW in 2.0) ───────────────────────────────────

// RequestPopularityFactor scores items by whether they were user-requested.
// Requested items get a lower deletion score (more protected).
type RequestPopularityFactor struct{}

// Name returns the display name.
func (f *RequestPopularityFactor) Name() string { return "Request Popularity" }

// Key returns the preference key.
func (f *RequestPopularityFactor) Key() string { return "request_popularity" }

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
