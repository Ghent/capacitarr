package integrations

import (
	"fmt"
	"log/slog"
	"strings"
)

// logEnrichmentResult logs enrichment match statistics at Info level with match
// rate, and issues a Warn if the enricher processed items but produced zero
// matches (likely a configuration error or expired credential).
func logEnrichmentResult(enricherName string, itemCount, libraryItems, matched int, extraAttrs ...any) {
	unmatched := itemCount - matched
	var matchRate string
	if itemCount > 0 {
		matchRate = fmt.Sprintf("%.1f%%", float64(matched)/float64(itemCount)*100)
	} else {
		matchRate = "N/A"
	}

	attrs := make([]any, 0, 14+len(extraAttrs))
	attrs = append(attrs,
		"component", "enrichment",
		"enricher", enricherName,
		"itemCount", itemCount,
		"libraryItems", libraryItems,
		"matched", matched,
		"unmatched", unmatched,
		"matchRate", matchRate,
	)
	attrs = append(attrs, extraAttrs...)

	if matched == 0 && libraryItems > 0 {
		slog.Warn("Enrichment produced zero matches — check integration configuration", attrs...)
	} else {
		slog.Info("Enrichment complete", attrs...)
	}
}

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
	logEnrichmentResult(e.name, len(items), len(watchMap), matched)
	return nil
}

// Verify BulkWatchEnricher satisfies Enricher at compile time.
var _ Enricher = (*BulkWatchEnricher)(nil)

// ─── TautulliEnricher ───────────────────────────────────────────────────────

// TautulliEnricher enriches items one-by-one using Tautulli's per-rating-key
// history API. Higher priority than bulk providers since Tautulli provides
// richer data (play counts, per-user history, watch durations).
//
// Tautulli queries by Plex ratingKey, but *arr items have TMDb IDs as their
// primary identifier. The tmdbToRatingKey map bridges this gap — it maps
// TMDb IDs to Plex ratingKeys, built from PlexClient during the same poll cycle.
type TautulliEnricher struct {
	client          *TautulliClient
	tmdbToRatingKey map[int]string // TMDb ID → Plex ratingKey
}

// NewTautulliEnricher creates an enricher wrapping a TautulliClient with a
// TMDb→RatingKey lookup map for translating *arr item IDs to Plex rating keys.
func NewTautulliEnricher(client *TautulliClient, tmdbToRatingKey map[int]string) *TautulliEnricher {
	return &TautulliEnricher{client: client, tmdbToRatingKey: tmdbToRatingKey}
}

// Name implements Enricher.
func (e *TautulliEnricher) Name() string { return "Tautulli Watch History" }

// Priority implements Enricher. Priority 10 is highest for watch data.
func (e *TautulliEnricher) Priority() int { return 10 }

// Enrich implements Enricher by querying Tautulli per item.
// Uses the TMDb→RatingKey map to translate *arr TMDb IDs into Plex rating keys.
func (e *TautulliEnricher) Enrich(items []MediaItem) error {
	if len(e.tmdbToRatingKey) == 0 {
		slog.Debug("Tautulli enricher skipped — no TMDb→RatingKey mappings available",
			"component", "enrichment")
		return nil
	}

	matched := 0
	for i := range items {
		item := &items[i]
		if item.TMDbID == 0 {
			continue
		}
		ratingKey, ok := e.tmdbToRatingKey[item.TMDbID]
		if !ok {
			continue // No Plex ratingKey for this TMDb ID
		}

		var watchData *TautulliWatchData
		var err error
		if item.Type == MediaTypeShow {
			watchData, err = e.client.GetShowWatchHistory(ratingKey)
		} else {
			watchData, err = e.client.GetWatchHistory(ratingKey)
		}
		if err != nil {
			slog.Debug("Tautulli enrichment failed", "component", "enrichment",
				"title", item.Title, "tmdbID", item.TMDbID, "ratingKey", ratingKey, "error", err)
			continue
		}
		if watchData != nil {
			item.PlayCount = watchData.PlayCount
			item.LastPlayed = watchData.LastPlayed
			if len(watchData.Users) > 0 {
				item.WatchedByUsers = watchData.Users
			}
			matched++
		}
	}
	logEnrichmentResult("Tautulli Watch History", len(items), len(e.tmdbToRatingKey), matched,
		"ratingKeyMappings", len(e.tmdbToRatingKey))
	return nil
}

// Verify TautulliEnricher satisfies Enricher at compile time.
var _ Enricher = (*TautulliEnricher)(nil)

// ─── JellystatEnricher ──────────────────────────────────────────────────────

// JellystatEnricher enriches items with watch data from Jellystat (Jellyfin
// analytics). Like TautulliEnricher, it runs at priority 10 (highest for watch
// data) and provides richer per-user stats than Jellyfin's native bulk API.
//
// Jellystat stores items by Jellyfin Item ID, so resolution to TMDb IDs
// requires the jellyfinIDToTMDbID map — built from JellyfinClient's
// ProviderIDs during the same poll cycle.
type JellystatEnricher struct {
	client             *JellystatClient
	jellyfinIDToTMDbID map[string]int // Jellyfin Item ID → TMDb ID
}

// NewJellystatEnricher creates an enricher wrapping a JellystatClient with
// a Jellyfin Item ID → TMDb ID lookup map for resolving Jellystat items.
func NewJellystatEnricher(client *JellystatClient, jellyfinIDToTMDbID map[string]int) *JellystatEnricher {
	return &JellystatEnricher{client: client, jellyfinIDToTMDbID: jellyfinIDToTMDbID}
}

// Name implements Enricher.
func (e *JellystatEnricher) Name() string { return "Jellystat Watch History" }

// Priority implements Enricher. Priority 10 is highest for watch data.
func (e *JellystatEnricher) Priority() int { return 10 }

// Enrich implements Enricher by fetching bulk watch stats from Jellystat and
// matching items by TMDb ID.
func (e *JellystatEnricher) Enrich(items []MediaItem) error {
	if len(e.jellyfinIDToTMDbID) == 0 {
		slog.Debug("Jellystat enricher skipped — no Jellyfin ID→TMDb ID mappings available",
			"component", "enrichment")
		return nil
	}

	watchMap, err := e.client.GetBulkWatchStats(e.jellyfinIDToTMDbID)
	if err != nil {
		return err
	}

	matched := 0
	for i := range items {
		item := &items[i]
		if item.TMDbID == 0 {
			continue
		}
		if wd, ok := watchMap[item.TMDbID]; ok {
			item.PlayCount = wd.PlayCount
			item.LastPlayed = wd.LastPlayed
			if len(wd.Users) > 0 {
				item.WatchedByUsers = wd.Users
			}
			matched++
		}
	}
	logEnrichmentResult("Jellystat Watch History", len(items), len(watchMap), matched,
		"jellyfinMappings", len(e.jellyfinIDToTMDbID))
	return nil
}

// Verify JellystatEnricher satisfies Enricher at compile time.
var _ Enricher = (*JellystatEnricher)(nil)

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
	logEnrichmentResult("Seerr Request Data", len(items), len(requests), matched)
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
	logEnrichmentResult(e.name, len(items), len(watchlistSet), matched)
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
// The tmdbToRatingKey map translates TMDb IDs (from *arr items) to Plex rating
// keys (which Tautulli uses for history queries). This map is built from
// PlexClient during the same poll cycle.
func RegisterTautulliEnrichers(pipeline *EnrichmentPipeline, registry *IntegrationRegistry, tmdbToRatingKey map[int]string) {
	for id := range registry.Connectors() {
		if tautulli, ok := registry.TautulliClient(id); ok {
			pipeline.Add(NewTautulliEnricher(tautulli, tmdbToRatingKey))
			slog.Debug("Added TautulliEnricher to pipeline", "component", "enrichment",
				"integrationID", id, "tmdbMappings", len(tmdbToRatingKey))
		}
	}
}

// RegisterJellystatEnrichers scans connectors for JellystatClient instances and
// adds JellystatEnricher to the pipeline. Called separately because Jellystat
// doesn't implement WatchDataProvider (it requires a Jellyfin ID→TMDb ID map
// that must be injected externally). The jellyfinIDToTMDbID map translates
// Jellyfin Item IDs to TMDb IDs, built from JellyfinClient during the same
// poll cycle.
func RegisterJellystatEnrichers(pipeline *EnrichmentPipeline, registry *IntegrationRegistry, jellyfinIDToTMDbID map[string]int) {
	for id := range registry.Connectors() {
		if jellystat, ok := registry.JellystatClient(id); ok {
			pipeline.Add(NewJellystatEnricher(jellystat, jellyfinIDToTMDbID))
			slog.Debug("Added JellystatEnricher to pipeline", "component", "enrichment",
				"integrationID", id, "jellyfinMappings", len(jellyfinIDToTMDbID))
		}
	}
}
