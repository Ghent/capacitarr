package integrations

import (
	"log/slog"
	"strings"
)

// ─── BulkWatchEnricher ──────────────────────────────────────────────────────

// BulkWatchEnricher enriches media items with bulk watch data from any
// WatchDataProvider (Plex, Jellyfin, Emby). Only populates items that don't
// already have watch data (PlayCount == 0) so higher-priority enrichers win.
// Matches items by TMDb ID for deterministic, unambiguous matching.
type BulkWatchEnricher struct {
	name     string
	priority int
	provider WatchDataProvider
}

// NewBulkWatchEnricher creates an enricher wrapping a WatchDataProvider.
func NewBulkWatchEnricher(name string, priority int, provider WatchDataProvider) *BulkWatchEnricher {
	return &BulkWatchEnricher{name: name, priority: priority, provider: provider}
}

// Name implements Enricher.
func (e *BulkWatchEnricher) Name() string { return e.name }

// Priority implements Enricher.
func (e *BulkWatchEnricher) Priority() int { return e.priority }

// Enrich implements Enricher by fetching bulk watch data and merging by TMDb ID.
func (e *BulkWatchEnricher) Enrich(items []MediaItem) error {
	watchMap, err := e.provider.GetBulkWatchData()
	if err != nil {
		return err
	}
	matched := 0
	for i := range items {
		item := &items[i]
		if item.TMDbID == 0 {
			continue // Skip items without TMDb ID
		}
		if wd, ok := watchMap[item.TMDbID]; ok {
			// Only enrich if no higher-priority enricher already set watch data
			if item.PlayCount == 0 {
				item.PlayCount = wd.PlayCount
				item.LastPlayed = wd.LastPlayed
				if len(wd.Users) > 0 {
					item.WatchedByUsers = wd.Users
				}
				matched++
			}
		}
	}
	slog.Info("Bulk watch enrichment complete", "component", "enrichment",
		"enricher", e.name, "libraryItems", len(watchMap), "matched", matched)
	return nil
}

// Verify BulkWatchEnricher satisfies Enricher at compile time.
var _ Enricher = (*BulkWatchEnricher)(nil)

// ─── TautulliEnricher ───────────────────────────────────────────────────────

// TautulliEnricher enriches items one-by-one using Tautulli's per-rating-key
// history API. Higher priority than bulk providers since Tautulli provides
// richer data (play counts, per-user history, watch durations).
type TautulliEnricher struct {
	client *TautulliClient
}

// NewTautulliEnricher creates an enricher wrapping a TautulliClient.
func NewTautulliEnricher(client *TautulliClient) *TautulliEnricher {
	return &TautulliEnricher{client: client}
}

// Name implements Enricher.
func (e *TautulliEnricher) Name() string { return "Tautulli Watch History" }

// Priority implements Enricher. Priority 10 is highest for watch data.
func (e *TautulliEnricher) Priority() int { return 10 }

// Enrich implements Enricher by querying Tautulli per item.
func (e *TautulliEnricher) Enrich(items []MediaItem) error {
	for i := range items {
		item := &items[i]
		if item.ExternalID == "" {
			continue
		}
		var watchData *TautulliWatchData
		var err error
		if item.Type == MediaTypeShow {
			watchData, err = e.client.GetShowWatchHistory(item.ExternalID)
		} else {
			watchData, err = e.client.GetWatchHistory(item.ExternalID)
		}
		if err != nil {
			slog.Debug("Tautulli enrichment failed", "component", "enrichment",
				"title", item.Title, "error", err)
			continue
		}
		if watchData != nil {
			item.PlayCount = watchData.PlayCount
			item.LastPlayed = watchData.LastPlayed
			if len(watchData.Users) > 0 {
				item.WatchedByUsers = watchData.Users
			}
		}
	}
	return nil
}

// Verify TautulliEnricher satisfies Enricher at compile time.
var _ Enricher = (*TautulliEnricher)(nil)

// ─── RequestEnricher ────────────────────────────────────────────────────────

// RequestEnricher enriches items with media request data from a RequestProvider
// (Seerr/Overseerr/Jellyseerr). Matches by TMDb ID.
type RequestEnricher struct {
	provider RequestProvider
}

// NewRequestEnricher creates an enricher wrapping a RequestProvider.
func NewRequestEnricher(provider RequestProvider) *RequestEnricher {
	return &RequestEnricher{provider: provider}
}

// Name implements Enricher.
func (e *RequestEnricher) Name() string { return "Seerr Request Data" }

// Priority implements Enricher.
func (e *RequestEnricher) Priority() int { return 30 }

// Enrich implements Enricher by matching items to media requests via TMDb ID.
func (e *RequestEnricher) Enrich(items []MediaItem) error {
	requests, err := e.provider.GetRequestedMedia()
	if err != nil {
		return err
	}
	// Build lookup by TMDb ID
	requestMap := make(map[int]MediaRequest)
	for _, req := range requests {
		requestMap[req.TMDbID] = req
	}
	matched := 0
	for i := range items {
		item := &items[i]
		if item.TMDbID > 0 {
			if req, ok := requestMap[item.TMDbID]; ok {
				item.IsRequested = true
				item.RequestedBy = req.RequestedBy
				item.RequestCount = 1
				matched++
			}
		}
	}
	slog.Debug("Request enrichment complete", "component", "enrichment",
		"requests", len(requests), "matched", matched)
	return nil
}

// Verify RequestEnricher satisfies Enricher at compile time.
var _ Enricher = (*RequestEnricher)(nil)

// ─── WatchlistEnricher ──────────────────────────────────────────────────────

// WatchlistEnricher enriches items with watchlist/favorites data from
// WatchlistProvider implementations (Plex on-deck, Jellyfin/Emby favorites).
// Multiple providers can be combined — only the first hit per item wins.
type WatchlistEnricher struct {
	name     string
	priority int
	provider WatchlistProvider
}

// NewWatchlistEnricher creates an enricher wrapping a WatchlistProvider.
func NewWatchlistEnricher(name string, priority int, provider WatchlistProvider) *WatchlistEnricher {
	return &WatchlistEnricher{name: name, priority: priority, provider: provider}
}

// Name implements Enricher.
func (e *WatchlistEnricher) Name() string { return e.name }

// Priority implements Enricher.
func (e *WatchlistEnricher) Priority() int { return e.priority }

// Enrich implements Enricher by applying watchlist flags to items via TMDb ID matching.
func (e *WatchlistEnricher) Enrich(items []MediaItem) error {
	watchlistSet, err := e.provider.GetWatchlistItems()
	if err != nil {
		return err
	}
	if len(watchlistSet) == 0 {
		return nil
	}
	matched := 0
	for i := range items {
		item := &items[i]
		if item.OnWatchlist {
			continue // Already set by a higher-priority enricher
		}
		if item.TMDbID == 0 {
			continue // Skip items without TMDb ID
		}
		if watchlistSet[item.TMDbID] {
			item.OnWatchlist = true
			matched++
		}
	}
	slog.Info("Watchlist enrichment complete", "component", "enrichment",
		"enricher", e.name, "watchlistItems", len(watchlistSet), "matched", matched)
	return nil
}

// Verify WatchlistEnricher satisfies Enricher at compile time.
var _ Enricher = (*WatchlistEnricher)(nil)

// ─── CrossReferenceEnricher ─────────────────────────────────────────────────

// CrossReferenceEnricher runs after all other enrichers to reconcile
// cross-references: did the user who requested an item also watch it?
type CrossReferenceEnricher struct{}

// NewCrossReferenceEnricher creates the cross-reference enricher.
func NewCrossReferenceEnricher() *CrossReferenceEnricher {
	return &CrossReferenceEnricher{}
}

// Name implements Enricher.
func (e *CrossReferenceEnricher) Name() string { return "Cross-Reference" }

// Priority implements Enricher. Always runs last at priority 100.
func (e *CrossReferenceEnricher) Priority() int { return 100 }

// Enrich implements Enricher by reconciling requestor vs watched-by.
func (e *CrossReferenceEnricher) Enrich(items []MediaItem) error {
	for i := range items {
		item := &items[i]
		if item.IsRequested && item.RequestedBy != "" && len(item.WatchedByUsers) > 0 {
			for _, user := range item.WatchedByUsers {
				if strings.EqualFold(user, item.RequestedBy) {
					item.WatchedByRequestor = true
					break
				}
			}
		}
	}
	return nil
}

// Verify CrossReferenceEnricher satisfies Enricher at compile time.
var _ Enricher = (*CrossReferenceEnricher)(nil)

// ─── Pipeline builder ───────────────────────────────────────────────────────

// BuildEnrichmentPipeline constructs an EnrichmentPipeline from an
// IntegrationRegistry, automatically discovering which enrichers to add
// based on registered capabilities.
func BuildEnrichmentPipeline(registry *IntegrationRegistry) *EnrichmentPipeline {
	pipeline := NewEnrichmentPipeline()

	// Tautulli enrichers (priority 10 — highest for watch data)
	// Tautulli doesn't implement WatchDataProvider, so we check for the
	// concrete type via the Connectable+type assertion pattern. Actually,
	// Tautulli is registered via factory; we need to detect it.
	// Since the registry stores clients by interface, we iterate connectors
	// and check for *TautulliClient.
	for _, client := range registry.WatchProviders() {
		// WatchDataProviders get bulk enrichers at priority 20 (after Tautulli)
		pipeline.Add(NewBulkWatchEnricher("Bulk Watch Data", 20, client))
	}

	// Request enrichers (priority 30)
	for _, provider := range registry.RequestProviders() {
		pipeline.Add(NewRequestEnricher(provider))
	}

	// Watchlist enrichers (priority 40)
	for _, provider := range registry.WatchlistProviders() {
		pipeline.Add(NewWatchlistEnricher("Watchlist/Favorites", 40, provider))
	}

	// Cross-reference enricher always added last (priority 100)
	pipeline.Add(NewCrossReferenceEnricher())

	return pipeline
}

// RegisterTautulliEnrichers scans connectors for TautulliClient instances and
// adds TautulliEnricher to the pipeline. Called separately because Tautulli
// doesn't implement WatchDataProvider (it uses per-item queries, not bulk).
func RegisterTautulliEnrichers(pipeline *EnrichmentPipeline, registry *IntegrationRegistry) {
	for id := range registry.Connectors() {
		if tautulli, ok := registry.TautulliClient(id); ok {
			pipeline.Add(NewTautulliEnricher(tautulli))
			slog.Debug("Added TautulliEnricher to pipeline", "component", "enrichment",
				"integrationID", id)
		}
	}
}
