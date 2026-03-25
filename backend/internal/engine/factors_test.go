package engine

import (
	"strings"
	"testing"
	"time"

	"capacitarr/internal/integrations"
)

func timePtr(t time.Time) *time.Time { return &t }

func TestWatchHistoryFactor(t *testing.T) {
	f := &WatchHistoryFactor{}
	if f.Name() != "Play History" {
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
		t.Errorf("expected 7 default factors, got %d", len(factors))
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

// ─── Label rename tests ─────────────────────────────────────────────────────

func TestFactorLabelRenames(t *testing.T) {
	tests := []struct {
		factor      ScoringFactor
		wantName    string
		wantKey     string
		descContain string
	}{
		{&WatchHistoryFactor{}, "Play History", "watch_history", "Unplayed"},
		{&RecencyFactor{}, "Last Played", "last_watched", "not played"},
		{&SeriesStatusFactor{}, "Show Status", "series_status", "Ended or canceled"},
	}

	for _, tc := range tests {
		t.Run(tc.wantName, func(t *testing.T) {
			if tc.factor.Name() != tc.wantName {
				t.Errorf("Name() = %q, want %q", tc.factor.Name(), tc.wantName)
			}
			if tc.factor.Key() != tc.wantKey {
				t.Errorf("Key() = %q, want %q (DB key must not change)", tc.factor.Key(), tc.wantKey)
			}
			if !strings.Contains(tc.factor.Description(), tc.descContain) {
				t.Errorf("Description() = %q, expected it to contain %q", tc.factor.Description(), tc.descContain)
			}
		})
	}
}

// ─── RequiresIntegration / MediaTypeScoped interface tests ──────────────────

func TestRequestPopularityFactor_RequiresIntegration(t *testing.T) {
	var f ScoringFactor = &RequestPopularityFactor{}
	ri, ok := f.(RequiresIntegration)
	if !ok {
		t.Fatal("RequestPopularityFactor must implement RequiresIntegration")
	}
	if ri.RequiredIntegrationType() != integrations.IntegrationTypeSeerr {
		t.Errorf("RequiredIntegrationType() = %q, want %q", ri.RequiredIntegrationType(), integrations.IntegrationTypeSeerr)
	}
}

func TestSeriesStatusFactor_MediaTypeScoped(t *testing.T) {
	var f ScoringFactor = &SeriesStatusFactor{}
	mts, ok := f.(MediaTypeScoped)
	if !ok {
		t.Fatal("SeriesStatusFactor must implement MediaTypeScoped")
	}
	types := mts.ApplicableMediaTypes()
	if len(types) != 2 {
		t.Fatalf("expected 2 applicable media types, got %d", len(types))
	}
	hasShow, hasSeason := false, false
	for _, mt := range types {
		if mt == integrations.MediaTypeShow {
			hasShow = true
		}
		if mt == integrations.MediaTypeSeason {
			hasSeason = true
		}
	}
	if !hasShow || !hasSeason {
		t.Errorf("expected show and season types, got %v", types)
	}
}

func TestUniversalFactors_DoNotImplementOptionalInterfaces(t *testing.T) {
	// Factors with no integration dependency — no RequiresIntegration,
	// RequiresAnyIntegration, or MediaTypeScoped.
	universalFactors := []ScoringFactor{
		&FileSizeFactor{},
		&RatingFactor{},
		&LibraryAgeFactor{},
	}

	for _, f := range universalFactors {
		if _, ok := f.(RequiresIntegration); ok {
			t.Errorf("%s should not implement RequiresIntegration", f.Name())
		}
		if _, ok := f.(RequiresAnyIntegration); ok {
			t.Errorf("%s should not implement RequiresAnyIntegration", f.Name())
		}
		if _, ok := f.(MediaTypeScoped); ok {
			t.Errorf("%s should not implement MediaTypeScoped", f.Name())
		}
	}
}

func TestWatchDataFactors_ImplementRequiresAnyIntegration(t *testing.T) {
	watchDataFactors := []ScoringFactor{
		&WatchHistoryFactor{},
		&RecencyFactor{},
	}

	for _, f := range watchDataFactors {
		// Must implement RequiresAnyIntegration, NOT RequiresIntegration
		if _, ok := f.(RequiresIntegration); ok {
			t.Errorf("%s should not implement RequiresIntegration (uses RequiresAnyIntegration)", f.Name())
		}
		rai, ok := f.(RequiresAnyIntegration)
		if !ok {
			t.Fatalf("%s must implement RequiresAnyIntegration", f.Name())
		}
		types := rai.RequiredIntegrationTypes()
		if len(types) < 3 {
			t.Errorf("%s.RequiredIntegrationTypes() returned %d types, want >= 3", f.Name(), len(types))
		}
		// Must include Plex and Tautulli
		hasPlex, hasTautulli := false, false
		for _, tt := range types {
			if tt == integrations.IntegrationTypePlex {
				hasPlex = true
			}
			if tt == integrations.IntegrationTypeTautulli {
				hasTautulli = true
			}
		}
		if !hasPlex || !hasTautulli {
			t.Errorf("%s.RequiredIntegrationTypes() must include plex and tautulli, got %v", f.Name(), types)
		}

		// Must implement RequiresEnrichmentCapability
		rec, ok := f.(RequiresEnrichmentCapability)
		if !ok {
			t.Fatalf("%s must implement RequiresEnrichmentCapability", f.Name())
		}
		if rec.RequiredEnrichmentCapability() != integrations.EnrichCapWatchData {
			t.Errorf("%s.RequiredEnrichmentCapability() = %q, want %q",
				f.Name(), rec.RequiredEnrichmentCapability(), integrations.EnrichCapWatchData)
		}
	}
}

func TestIsFactorApplicable(t *testing.T) {
	allActive := &EvaluationContext{
		ActiveIntegrationTypes: map[integrations.IntegrationType]bool{
			integrations.IntegrationTypeSeerr:    true,
			integrations.IntegrationTypeSonarr:   true,
			integrations.IntegrationTypeRadarr:   true,
			integrations.IntegrationTypePlex:     true,
			integrations.IntegrationTypeTautulli: true,
		},
	}
	noSeerr := &EvaluationContext{
		ActiveIntegrationTypes: map[integrations.IntegrationType]bool{
			integrations.IntegrationTypeSonarr: true,
			integrations.IntegrationTypeRadarr: true,
			integrations.IntegrationTypePlex:   true,
		},
	}

	movieItem := integrations.MediaItem{Title: "Serenity", Type: integrations.MediaTypeMovie}
	showItem := integrations.MediaItem{Title: "Firefly", Type: integrations.MediaTypeShow}

	// RequestPopularityFactor: applicable with Seerr, not without
	rpf := &RequestPopularityFactor{}
	if ok, _ := isFactorApplicable(rpf, movieItem, allActive); !ok {
		t.Error("RequestPopularityFactor should be applicable when Seerr is active")
	}
	if ok, _ := isFactorApplicable(rpf, movieItem, noSeerr); ok {
		t.Error("RequestPopularityFactor should not be applicable when Seerr is absent")
	}

	// SeriesStatusFactor: applicable for shows, not for movies
	ssf := &SeriesStatusFactor{}
	if ok, _ := isFactorApplicable(ssf, showItem, allActive); !ok {
		t.Error("SeriesStatusFactor should be applicable for show items")
	}
	if ok, _ := isFactorApplicable(ssf, movieItem, allActive); ok {
		t.Error("SeriesStatusFactor should not be applicable for movie items")
	}

	// WatchHistoryFactor: applicable when at least one watch-data integration active
	whf := &WatchHistoryFactor{}
	if ok, _ := isFactorApplicable(whf, movieItem, allActive); !ok {
		t.Error("WatchHistoryFactor should be applicable with active watch-data integrations")
	}
	// Still applicable without Seerr (Plex is active)
	if ok, _ := isFactorApplicable(whf, movieItem, noSeerr); !ok {
		t.Error("WatchHistoryFactor should be applicable when Plex is active")
	}
}

func TestIsFactorApplicable_BrokenIntegrations(t *testing.T) {
	// All watch-data integrations broken → skip WatchHistory/Recency
	allWatchBroken := &EvaluationContext{
		ActiveIntegrationTypes: map[integrations.IntegrationType]bool{
			integrations.IntegrationTypePlex:     true,
			integrations.IntegrationTypeTautulli: true,
		},
		BrokenIntegrationTypes: map[integrations.IntegrationType]bool{
			integrations.IntegrationTypePlex:     true,
			integrations.IntegrationTypeTautulli: true,
		},
	}

	movieItem := integrations.MediaItem{Title: "Serenity", Type: integrations.MediaTypeMovie}

	whf := &WatchHistoryFactor{}
	ok, reason := isFactorApplicable(whf, movieItem, allWatchBroken)
	if ok {
		t.Error("WatchHistoryFactor should be skipped when all watch-data integrations are broken")
	}
	if reason != skipReasonIntegrationError {
		t.Errorf("skip reason = %q, want %q", reason, skipReasonIntegrationError)
	}

	// One healthy → applicable
	oneHealthy := &EvaluationContext{
		ActiveIntegrationTypes: map[integrations.IntegrationType]bool{
			integrations.IntegrationTypePlex:     true,
			integrations.IntegrationTypeTautulli: true,
		},
		BrokenIntegrationTypes: map[integrations.IntegrationType]bool{
			integrations.IntegrationTypeTautulli: true,
		},
	}
	ok, reason = isFactorApplicable(whf, movieItem, oneHealthy)
	if !ok {
		t.Error("WatchHistoryFactor should be applicable when at least one watch-data integration is healthy")
	}
	if reason != "" {
		t.Errorf("skip reason should be empty when applicable, got %q", reason)
	}

	// Single RequiresIntegration broken (Seerr)
	seerrBroken := &EvaluationContext{
		ActiveIntegrationTypes: map[integrations.IntegrationType]bool{
			integrations.IntegrationTypeSeerr: true,
			integrations.IntegrationTypePlex:  true,
		},
		BrokenIntegrationTypes: map[integrations.IntegrationType]bool{
			integrations.IntegrationTypeSeerr: true,
		},
	}
	rpf := &RequestPopularityFactor{}
	ok, reason = isFactorApplicable(rpf, movieItem, seerrBroken)
	if ok {
		t.Error("RequestPopularityFactor should be skipped when Seerr is broken")
	}
	if reason != skipReasonIntegrationError {
		t.Errorf("skip reason = %q, want %q", reason, skipReasonIntegrationError)
	}
}

func TestIsFactorApplicable_FailedEnrichmentCapabilities(t *testing.T) {
	ctx := &EvaluationContext{
		ActiveIntegrationTypes: map[integrations.IntegrationType]bool{
			integrations.IntegrationTypePlex: true,
		},
		FailedEnrichmentCapabilities: map[string]bool{
			integrations.EnrichCapWatchData: true,
		},
	}

	movieItem := integrations.MediaItem{Title: "Serenity", Type: integrations.MediaTypeMovie}

	whf := &WatchHistoryFactor{}
	ok, reason := isFactorApplicable(whf, movieItem, ctx)
	if ok {
		t.Error("WatchHistoryFactor should be skipped when watch_data capability failed")
	}
	if reason != skipReasonNoEnrichmentData {
		t.Errorf("skip reason = %q, want %q", reason, skipReasonNoEnrichmentData)
	}

	// FileSizeFactor has no enrichment requirement, should still be applicable
	fsf := &FileSizeFactor{}
	ok, reason = isFactorApplicable(fsf, movieItem, ctx)
	if !ok {
		t.Error("FileSizeFactor should be applicable regardless of enrichment failures")
	}
	if reason != "" {
		t.Errorf("skip reason should be empty, got %q", reason)
	}
}

func TestNewEvaluationContext(t *testing.T) {
	ctx := NewEvaluationContext([]string{"sonarr", "radarr", "seerr"}, nil)
	if !ctx.HasIntegrationType(integrations.IntegrationTypeSonarr) {
		t.Error("expected sonarr to be active")
	}
	if !ctx.HasIntegrationType(integrations.IntegrationTypeSeerr) {
		t.Error("expected seerr to be active")
	}
	if ctx.HasIntegrationType(integrations.IntegrationTypePlex) {
		t.Error("expected plex to not be active")
	}

	// Test with broken types
	ctx2 := NewEvaluationContext([]string{"plex", "tautulli"}, []string{"tautulli"})
	if !ctx2.HasIntegrationType(integrations.IntegrationTypePlex) {
		t.Error("expected plex to be active")
	}
	if !ctx2.IsIntegrationBroken(integrations.IntegrationTypeTautulli) {
		t.Error("expected tautulli to be broken")
	}
	if ctx2.IsIntegrationBroken(integrations.IntegrationTypePlex) {
		t.Error("expected plex to not be broken")
	}
}
