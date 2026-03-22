package integrations

import (
	"log/slog"
	"sort"
)

// Enricher is a composable enrichment step that augments media items with data
// from external services. Each enricher wraps one or more integration clients.
// Adding a new enrichment source = one file implementing Enricher.
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
	EnrichersRun   int      // Number of enrichers that executed
	ItemsProcessed int      // Number of items passed to the pipeline
	TotalMatches   int      // Estimated total matches (sum of per-item enrichment hits)
	ZeroMatchers   []string // Enricher names that ran but produced zero matches
}

// Run executes all enrichers in priority order. Failures are logged but do not
// stop the pipeline — subsequent enrichers still run. Returns enrichment stats.
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

	for _, e := range sorted {
		// Snapshot enrichment state before this enricher runs to count its contributions
		beforePlayCount := countItemsWithPlayCount(items)
		beforeRequested := countItemsRequested(items)
		beforeWatchlist := countItemsOnWatchlist(items)

		slog.Info("Running enricher", "component", "enrichment", "enricher", e.Name(),
			"priority", e.Priority(), "itemCount", len(items))
		if err := e.Enrich(items); err != nil {
			slog.Warn("Enrichment failed", "component", "enrichment",
				"enricher", e.Name(), "error", err)
			continue
		}
		stats.EnrichersRun++

		// Measure the delta this enricher added
		afterPlayCount := countItemsWithPlayCount(items)
		afterRequested := countItemsRequested(items)
		afterWatchlist := countItemsOnWatchlist(items)
		delta := (afterPlayCount - beforePlayCount) + (afterRequested - beforeRequested) + (afterWatchlist - beforeWatchlist)
		stats.TotalMatches += delta

		// CrossReferenceEnricher always produces zero new matches (it just reconciles)
		// so exclude it from zero-match detection
		if delta == 0 && e.Priority() < 100 {
			stats.ZeroMatchers = append(stats.ZeroMatchers, e.Name())
		}
	}

	slog.Info("Enrichment pipeline complete", "component", "enrichment",
		"enrichersRun", stats.EnrichersRun, "itemsProcessed", stats.ItemsProcessed,
		"totalMatches", stats.TotalMatches, "zeroMatchers", len(stats.ZeroMatchers))

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
