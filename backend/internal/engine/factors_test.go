package engine

import (
	"testing"
	"time"

	"capacitarr/internal/integrations"
)

func timePtr(t time.Time) *time.Time { return &t }

func TestWatchHistoryFactor(t *testing.T) {
	f := &WatchHistoryFactor{}
	if f.Name() != "Watch History" {
		t.Errorf("unexpected name: %s", f.Name())
	}
	if f.Key() != "watch_history" {
		t.Errorf("unexpected key: %s", f.Key())
	}

	// Unwatched → 1.0
	score := f.Calculate(integrations.MediaItem{Title: "Serenity", PlayCount: 0})
	if score != 1.0 {
		t.Errorf("expected 1.0 for unwatched, got %.2f", score)
	}

	// 1 play → 0.5
	score = f.Calculate(integrations.MediaItem{Title: "Serenity", PlayCount: 1})
	if score != 0.5 {
		t.Errorf("expected 0.5 for 1 play, got %.2f", score)
	}

	// 5 plays → 0.1
	score = f.Calculate(integrations.MediaItem{Title: "Serenity", PlayCount: 5})
	if score != 0.1 {
		t.Errorf("expected 0.1 for 5 plays, got %.2f", score)
	}
}

func TestRecencyFactor(t *testing.T) {
	f := &RecencyFactor{}

	// Never watched → 1.0
	score := f.Calculate(integrations.MediaItem{Title: "Serenity"})
	if score != 1.0 {
		t.Errorf("expected 1.0 for never watched, got %.2f", score)
	}

	// Recently watched → < 1.0
	recent := timePtr(time.Now().Add(-24 * time.Hour))
	score = f.Calculate(integrations.MediaItem{Title: "Serenity", LastPlayed: recent})
	if score >= 0.1 {
		t.Errorf("expected < 0.1 for yesterday, got %.4f", score)
	}
}

func TestFileSizeFactor(t *testing.T) {
	f := &FileSizeFactor{}

	// 0 bytes → 0.0
	score := f.Calculate(integrations.MediaItem{Title: "Serenity", SizeBytes: 0})
	if score != 0.0 {
		t.Errorf("expected 0.0 for 0 bytes, got %.2f", score)
	}

	// 25GB → 0.5
	score = f.Calculate(integrations.MediaItem{Title: "Serenity", SizeBytes: 25 * 1024 * 1024 * 1024})
	if score != 0.5 {
		t.Errorf("expected 0.5 for 25GB, got %.2f", score)
	}

	// 100GB → capped at 1.0
	score = f.Calculate(integrations.MediaItem{Title: "Serenity", SizeBytes: 100 * 1024 * 1024 * 1024})
	if score != 1.0 {
		t.Errorf("expected 1.0 for 100GB (capped), got %.2f", score)
	}
}

func TestRatingFactor(t *testing.T) {
	f := &RatingFactor{}

	// Unknown rating → 0.5
	score := f.Calculate(integrations.MediaItem{Title: "Serenity", Rating: 0})
	if score != 0.5 {
		t.Errorf("expected 0.5 for unknown rating, got %.2f", score)
	}

	// Rating 10.0 → 0.0 (highly rated = don't delete)
	score = f.Calculate(integrations.MediaItem{Title: "Serenity", Rating: 10.0})
	if score != 0.0 {
		t.Errorf("expected 0.0 for rating 10, got %.2f", score)
	}

	// Rating 5.0 → 0.5
	score = f.Calculate(integrations.MediaItem{Title: "Serenity", Rating: 5.0})
	if score != 0.5 {
		t.Errorf("expected 0.5 for rating 5, got %.2f", score)
	}
}

func TestSeriesStatusFactor(t *testing.T) {
	f := &SeriesStatusFactor{}

	// Ended show → 1.0
	score := f.Calculate(integrations.MediaItem{Title: "Firefly", Type: integrations.MediaTypeShow, SeriesStatus: "ended"})
	if score != 1.0 {
		t.Errorf("expected 1.0 for ended show, got %.2f", score)
	}

	// Continuing show → 0.2
	score = f.Calculate(integrations.MediaItem{Title: "Firefly", Type: integrations.MediaTypeShow, SeriesStatus: "continuing"})
	if score != 0.2 {
		t.Errorf("expected 0.2 for continuing show, got %.2f", score)
	}

	// Movie → 0.5 (neutral)
	score = f.Calculate(integrations.MediaItem{Title: "Serenity", Type: integrations.MediaTypeMovie})
	if score != 0.5 {
		t.Errorf("expected 0.5 for movie, got %.2f", score)
	}
}

func TestRequestPopularityFactor(t *testing.T) {
	f := &RequestPopularityFactor{}

	// Not requested → 0.5
	score := f.Calculate(integrations.MediaItem{Title: "Serenity"})
	if score != 0.5 {
		t.Errorf("expected 0.5 for unrequested, got %.2f", score)
	}

	// Requested, not watched → 0.1 (strongly protect)
	score = f.Calculate(integrations.MediaItem{Title: "Serenity", IsRequested: true})
	if score != 0.1 {
		t.Errorf("expected 0.1 for requested unwatched, got %.2f", score)
	}

	// Requested and watched by requestor → 0.3
	score = f.Calculate(integrations.MediaItem{Title: "Serenity", IsRequested: true, WatchedByRequestor: true})
	if score != 0.3 {
		t.Errorf("expected 0.3 for requested+watched, got %.2f", score)
	}
}

func TestDefaultFactors(t *testing.T) {
	factors := DefaultFactors()
	if len(factors) != 7 {
		t.Errorf("expected 8 default factors, got %d", len(factors))
	}

	// Verify all keys are unique
	keys := make(map[string]bool)
	for _, f := range factors {
		if keys[f.Key()] {
			t.Errorf("duplicate factor key: %s", f.Key())
		}
		keys[f.Key()] = true
	}
}
