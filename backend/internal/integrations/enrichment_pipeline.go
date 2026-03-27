package integrations

import (
	"log/slog"
	"sort"
)

// ─── Enrichment capability constants ────────────────────────────────────────

const (
	// EnrichCapWatchData identifies enrichers that provide play count / last played data.
	EnrichCapWatchData = "watch_data"
	// EnrichCapRequestData identifies enrichers that provide media request data.
	EnrichCapRequestData = "request_data"
	// EnrichCapWatchlist identifies enrichers that provide watchlist membership data.
	EnrichCapWatchlist = "watchlist_data"
)

// ─── Enricher interfaces ────────────────────────────────────────────────────

// Enricher is a composable enrichment step that augments media items with data
// from external services. Each enricher wraps one or more integration clients.
// Adding a new enrichment source = one file implementing Enricher.
//
// New enrichers should also implement EnrichmentCapabilityProvider to declare
// which enrichment capability they contribute to. This enables the pipeline
// to detect when all enrichers for a capability have failed.
type Enricher interface {
	// Name returns the human-readable name for logging.
	Name() string
	// Priority returns the execution order (lower = earlier). Enrichers with
	// the same priority run in registration order.
	Priority() int
	// Enrich augments items in-place with data from the enricher's source.
	// Non-fatal errors are logged and do not stop the pipeline.
	Enrich(items []MediaItem) error
}

// EnrichmentCapabilityProvider is optionally implemented by enrichers to
// declare which enrichment capability they contribute to. Used by the
// pipeline to detect when all enrichers for a capability have failed or
// produced zero matches. Use the EnrichCap* constants.
type EnrichmentCapabilityProvider interface {
	EnrichmentCapability() string
}

// ZeroMatchExempt is optionally implemented by enrichers that should not
// be flagged when they produce zero new matches. This is appropriate for
// enrichers that reconcile or cross-reference data from other enrichers
// rather than contributing new field values (e.g., CrossReferenceEnricher).
type ZeroMatchExempt interface {
	SkipZeroMatchTracking() bool
}

// EnrichmentPipeline runs a sequence of enrichers in priority order.
type EnrichmentPipeline struct {
	enrichers []Enricher
}

// NewEnrichmentPipeline creates an empty pipeline.
func NewEnrichmentPipeline() *EnrichmentPipeline {
	return &EnrichmentPipeline{}
}

// Add registers an enricher in the pipeline.
func (p *EnrichmentPipeline) Add(e Enricher) {
	p.enrichers = append(p.enrichers, e)
}

// EnrichmentStats holds summary statistics from a pipeline run.
type EnrichmentStats struct {
	EnrichersRun       int      // Number of enrichers that executed successfully
	ItemsProcessed     int      // Number of items passed to the pipeline
	TotalMatches       int      // Estimated total matches (sum of per-item enrichment hits)
	ZeroMatchers       []string // Enricher names that ran but produced zero matches
	FailedEnrichers    []string // Enricher names that returned a non-nil error
	FailedCapabilities []string // Capabilities where ALL enrichers failed or zero-matched
}

// Run executes all enrichers in priority order. Failures are logged but do not
// stop the pipeline — subsequent enrichers still run. Returns enrichment stats
// including capability-level failure detection.
func (p *EnrichmentPipeline) Run(items []MediaItem) EnrichmentStats {
	stats := EnrichmentStats{ItemsProcessed: len(items)}

	if len(items) == 0 || len(p.enrichers) == 0 {
		return stats
	}

	// Sort by priority (stable sort preserves registration order for same priority)
	sorted := make([]Enricher, len(p.enrichers))
	copy(sorted, p.enrichers)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority() < sorted[j].Priority()
	})

	// Track per-capability enricher outcomes for failure detection.
	// capTotal: number of enrichers registered for each capability.
	// capErrored: number that returned a non-nil error from Enrich().
	// Only actual errors count toward capability failure — zero-matchers
	// are NOT counted because zero matches is a legitimate state (e.g.
	// no items requested via Seerr, fresh Tautulli with no history).
	capTotal := make(map[string]int)
	capErrored := make(map[string]int)

	for _, e := range sorted {
		// Determine this enricher's capability (if declared)
		var enrichCap string
		if ecp, ok := e.(EnrichmentCapabilityProvider); ok {
			enrichCap = ecp.EnrichmentCapability()
			capTotal[enrichCap]++
		}

		// Snapshot enrichment state before this enricher runs to count its contributions
		beforePlayCount := countItemsWithPlayCount(items)
		beforeRequested := countItemsRequested(items)
		beforeWatchlist := countItemsOnWatchlist(items)

		slog.Info("Running enricher", "component", "enrichment", "enricher", e.Name(),
			"priority", e.Priority(), "itemCount", len(items))
		if err := e.Enrich(items); err != nil {
			slog.Warn("Enrichment failed", "component", "enrichment",
				"enricher", e.Name(), "error", err)
			stats.FailedEnrichers = append(stats.FailedEnrichers, e.Name())
			if enrichCap != "" {
				capErrored[enrichCap]++
			}
			continue
		}
		stats.EnrichersRun++

		// Measure the delta this enricher added
		afterPlayCount := countItemsWithPlayCount(items)
		afterRequested := countItemsRequested(items)
		afterWatchlist := countItemsOnWatchlist(items)
		delta := (afterPlayCount - beforePlayCount) + (afterRequested - beforeRequested) + (afterWatchlist - beforeWatchlist)
		stats.TotalMatches += delta

		// Enrichers that implement ZeroMatchExempt (e.g., CrossReferenceEnricher)
		// reconcile data from other enrichers rather than contributing new field
		// values, so they are excluded from zero-match detection.
		skipZeroTrack := false
		if exempt, ok := e.(ZeroMatchExempt); ok {
			skipZeroTrack = exempt.SkipZeroMatchTracking()
		}
		if delta == 0 && !skipZeroTrack {
			stats.ZeroMatchers = append(stats.ZeroMatchers, e.Name())
		}
	}

	// Detect capabilities where ALL enrichers errored (returned non-nil from Enrich).
	// Only flag capabilities that had at least one enricher registered.
	// Zero-matchers are NOT counted — zero matches is legitimate (no requests,
	// fresh history, etc.) and should not disable scoring factors.
	for capability, total := range capTotal {
		if total > 0 && capErrored[capability] >= total {
			stats.FailedCapabilities = append(stats.FailedCapabilities, capability)
		}
	}

	slog.Info("Enrichment pipeline complete", "component", "enrichment",
		"enrichersRun", stats.EnrichersRun, "itemsProcessed", stats.ItemsProcessed,
		"totalMatches", stats.TotalMatches, "zeroMatchers", len(stats.ZeroMatchers),
		"failedEnrichers", len(stats.FailedEnrichers),
		"failedCapabilities", len(stats.FailedCapabilities))

	return stats
}

// Count returns the number of registered enrichers.
func (p *EnrichmentPipeline) Count() int {
	return len(p.enrichers)
}

// countItemsWithPlayCount returns the number of items with PlayCount > 0.
func countItemsWithPlayCount(items []MediaItem) int {
	count := 0
	for i := range items {
		if items[i].PlayCount > 0 {
			count++
		}
	}
	return count
}

// countItemsRequested returns the number of items with IsRequested == true.
func countItemsRequested(items []MediaItem) int {
	count := 0
	for i := range items {
		if items[i].IsRequested {
			count++
		}
	}
	return count
}

// countItemsOnWatchlist returns the number of items with OnWatchlist == true.
func countItemsOnWatchlist(items []MediaItem) int {
	count := 0
	for i := range items {
		if items[i].OnWatchlist {
			count++
		}
	}
	return count
}

// BuildFullPipeline constructs an EnrichmentPipeline with all enrichers
// registered: capability-based (Plex, Seerr, Jellyfin, Emby), ID-mapped
// (Tautulli via TMDb→RatingKey, Jellystat via JellyfinID→TMDbID), and
// title-matched (Tracearr). This is the single source of truth for
// enrichment pipeline construction — both the poller and cold-start
// preview path call this function to avoid logic divergence.
func BuildFullPipeline(registry *IntegrationRegistry) *EnrichmentPipeline {
	pipeline := BuildEnrichmentPipeline(registry)

	// Build TMDb→RatingKey map from Plex for Tautulli enrichment.
	tmdbToRatingKey := make(map[int]string)
	for id := range registry.Connectors() {
		if plex, ok := registry.PlexClient(id); ok {
			plexMap, mapErr := plex.GetTMDbToRatingKeyMap()
			if mapErr != nil {
				slog.Warn("Failed to build TMDb→RatingKey map from Plex",
					"component", "enrichment", "integrationID", id, "error", mapErr)
				continue
			}
			for tmdbID, ratingKey := range plexMap {
				tmdbToRatingKey[tmdbID] = ratingKey
			}
			slog.Debug("Built TMDb→RatingKey map from Plex",
				"component", "enrichment", "integrationID", id, "mappings", len(plexMap))
		}
	}
	RegisterTautulliEnrichers(pipeline, registry, tmdbToRatingKey)

	// Build Jellyfin Item ID → TMDb ID map for Jellystat enrichment.
	jellyfinIDToTMDbID := make(map[string]int)
	for id := range registry.Connectors() {
		if jf, ok := registry.JellyfinClient(id); ok {
			jfMap, mapErr := jf.GetItemIDToTMDbIDMap()
			if mapErr != nil {
				slog.Warn("Failed to build Jellyfin ID→TMDb ID map",
					"component", "enrichment", "integrationID", id, "error", mapErr)
				continue
			}
			for itemID, tmdbID := range jfMap {
				jellyfinIDToTMDbID[itemID] = tmdbID
			}
			slog.Debug("Built Jellyfin ID→TMDb ID map",
				"component", "enrichment", "integrationID", id, "mappings", len(jfMap))
		}
	}
	RegisterJellystatEnrichers(pipeline, registry, jellyfinIDToTMDbID)

	// Register Tracearr enrichers (title-based matching, no ID maps needed).
	RegisterTracearrEnrichers(pipeline, registry)

	return pipeline
}
