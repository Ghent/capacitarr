package integrations

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PlexClient implements Connectable, WatchDataProvider, and WatchlistProvider for Plex Media Server.
// PlexClient intentionally does NOT implement MediaSource — only *arr integrations (which also
// implement MediaDeleter and DiskReporter) should provide media items to the evaluation pool.
//
// Per-cycle caching: getMediaItems() fetches the full Plex library once per client
// instance and caches the result. Since BuildIntegrationRegistry() creates new client
// instances each poll cycle, the cache is naturally cycle-scoped and requires no
// explicit reset. This eliminates ~16 redundant Plex API calls per cycle where
// multiple enrichers (watch data, collections, labels, TMDb mapping) each independently
// fetched the same library data.
type PlexClient struct {
	URL   string
	Token string `json:"-"` // X-Plex-Token

	// Per-cycle library cache. Populated on first call to getMediaItems(),
	// reused by all subsequent callers within the same poll cycle.
	cachedItems    []MediaItem
	cachedItemsErr error
	cacheOnce      sync.Once
}

// NewPlexClient creates a new Plex media server API client.
func NewPlexClient(url, token string) *PlexClient {
	return &PlexClient{
		URL:   strings.TrimRight(url, "/"),
		Token: token,
	}
}

func (p *PlexClient) doRequest(endpoint string) ([]byte, error) {
	sep := "?"
	if strings.Contains(endpoint, "?") {
		sep = "&"
	}
	fullURL := p.URL + endpoint + sep + "X-Plex-Token=" + p.Token
	return DoAPIRequest(fullURL, "Accept", "application/json")
}

// TestConnection verifies the Plex server is reachable and the token is valid.
func (p *PlexClient) TestConnection() error {
	_, err := p.doRequest("/identity")
	return err
}

// plexLibraryResponse maps /library/sections response
type plexLibraryResponse struct {
	MediaContainer struct {
		Directory []struct {
			Key   string `json:"key"`
			Title string `json:"title"`
			Type  string `json:"type"` // movie, show, artist
		} `json:"Directory"`
	} `json:"MediaContainer"`
}

// plexMediaResponse maps /library/sections/{key}/all response
type plexMediaResponse struct {
	MediaContainer struct {
		Metadata []plexMetadata `json:"Metadata"`
	} `json:"MediaContainer"`
}

type plexMetadata struct {
	RatingKey        string       `json:"ratingKey"`
	Title            string       `json:"title"`
	ParentTitle      string       `json:"parentTitle,omitempty"`
	GrandparentTitle string       `json:"grandparentTitle,omitempty"`
	Year             int          `json:"year"`
	Type             string       `json:"type"` // movie, show, season, episode
	AudienceRating   float64      `json:"audienceRating"`
	Rating           float64      `json:"rating"`
	Ratings          []plexRating `json:"Rating,omitempty"` // Plex rating array; exact tag so it does not collide with rating
	ViewCount        int          `json:"viewCount"`
	LastViewedAt     int64        `json:"lastViewedAt"`
	AddedAt          int64        `json:"addedAt"`
	Duration         int64        `json:"duration"`
	GUID             string       `json:"guid"`           // Primary GUID (e.g. "plex://movie/...")
	GUIDs            []plexGUID   `json:"Guid,omitempty"` // Additional GUIDs including TMDb references
	Genre            []struct {
		Tag string `json:"tag"`
	} `json:"Genre"`
	Collection []struct {
		Tag string `json:"tag"`
	} `json:"Collection"`
	Label []struct {
		Tag string `json:"tag"`
	} `json:"Label"`
	Media []struct {
		Part []struct {
			File string `json:"file"`
			Size int64  `json:"size"`
		} `json:"Part"`
	} `json:"Media"`
	Index     int `json:"index,omitempty"`     // season/episode number
	LeafCount int `json:"leafCount,omitempty"` // episode count (for shows/seasons)
}

// plexGUID represents a GUID entry from the Plex API.
type plexGUID struct {
	ID string `json:"id"` // e.g. "tmdb://12345", "imdb://tt1234567", "tvdb://54321"
}

// plexRating is the PascalCase Rating array Plex emits next to the scalar
// rating field. encoding/json matches keys case-insensitively, so this type
// must be tagged `Rating` or the array decodes into Rating float64 and the
// whole MediaContainer fails.
type plexRating struct {
	Image string  `json:"image"`
	Value float64 `json:"value"`
	Type  string  `json:"type"`
}

// plexHistoryResponse maps /status/sessions/history/all response.
// The Plex history API returns individual play events for all users (admin token required).
// Pagination is controlled via X-Plex-Container-Start and X-Plex-Container-Size query params.
type plexHistoryResponse struct {
	MediaContainer struct {
		Size      int                `json:"size"`      // Number of entries in this page
		TotalSize int                `json:"totalSize"` // Total number of history entries
		Metadata  []plexHistoryEntry `json:"Metadata"`
	} `json:"MediaContainer"`
}

// plexHistoryEntry represents a single play event from Plex session history.
type plexHistoryEntry struct {
	RatingKey            string `json:"ratingKey"`
	ParentRatingKey      string `json:"parentRatingKey"`
	GrandparentRatingKey string `json:"grandparentRatingKey"`
	Type                 string `json:"type"`     // movie, episode, track
	ViewedAt             int64  `json:"viewedAt"` // Unix epoch timestamp
	AccountID            int    `json:"accountID"`
}

// plexAccountsResponse maps /accounts response for resolving account IDs to usernames.
type plexAccountsResponse struct {
	MediaContainer struct {
		Account []plexAccount `json:"Account"`
	} `json:"MediaContainer"`
}

// plexAccount represents a Plex managed/shared user account.
type plexAccount struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// plexHistoryPageSize controls the number of history entries per API request in
// fetchAllHistory(). 1000 is a good balance — small enough to keep individual
// response sizes manageable, large enough to minimize round trips. A typical
// server has 1,000–50,000 total history entries = 1–50 API calls.
const plexHistoryPageSize = 1000

// plexTMDbIDRegex matches TMDb IDs in Plex GUID strings like "tmdb://12345".
var plexTMDbIDRegex = regexp.MustCompile(`^tmdb://(\d+)$`)

// plexExtractTMDbID extracts the TMDb ID from a Plex item's GUIDs array.
// Plex stores GUIDs as "tmdb://12345", "imdb://tt1234567", etc.
// Returns 0 if no TMDb GUID is found.
func plexExtractTMDbID(guids []plexGUID) int {
	for _, g := range guids {
		matches := plexTMDbIDRegex.FindStringSubmatch(g.ID)
		if len(matches) == 2 {
			id, err := strconv.Atoi(matches[1])
			if err == nil {
				return id
			}
		}
	}
	return 0
}

// getMediaItems returns all movies, shows, and seasons from all Plex libraries.
// Results are cached for the lifetime of this PlexClient instance (one poll cycle).
// This method is unexported to prevent PlexClient from satisfying the MediaSource interface.
// Only *arr integrations should implement MediaSource. Internal callers (GetBulkWatchData,
// GetCollectionMemberships, GetLabelMemberships, GetTMDbToRatingKeyMap, GetCollectionNames,
// GetLabelNames) all share the same cached result.
func (p *PlexClient) getMediaItems() ([]MediaItem, error) {
	p.cacheOnce.Do(func() {
		p.cachedItems, p.cachedItemsErr = p.fetchMediaItems()
	})
	return p.cachedItems, p.cachedItemsErr
}

// fetchMediaItems performs the actual HTTP calls to fetch all movies, shows, and
// seasons from all Plex libraries. Called once per cycle via getMediaItems().
func (p *PlexClient) fetchMediaItems() ([]MediaItem, error) {
	// 1. Get all library sections
	body, err := p.doRequest("/library/sections")
	if err != nil {
		return nil, err
	}

	var libs plexLibraryResponse
	if err := json.Unmarshal(body, &libs); err != nil {
		return nil, fmt.Errorf("failed to parse library sections: %w", err)
	}

	var items []MediaItem

	for _, lib := range libs.MediaContainer.Directory {
		// Only process movie and show libraries
		if lib.Type != string(MediaTypeMovie) && lib.Type != string(MediaTypeShow) {
			continue
		}

		// 2. Get all items in this library (includeGuids=1 is required for TMDb ID extraction)
		itemBody, err := p.doRequest(fmt.Sprintf("/library/sections/%s/all?includeGuids=1", lib.Key))
		if err != nil {
			continue // Skip failed libraries
		}

		var media plexMediaResponse
		if err := json.Unmarshal(itemBody, &media); err != nil {
			continue
		}

		for _, m := range media.MediaContainer.Metadata {
			item := plexMetadataToMediaItem(m)
			if item != nil {
				items = append(items, *item)
			}
		}
	}

	return items, nil
}

func plexMetadataToMediaItem(m plexMetadata) *MediaItem {
	// Calculate total file size from all media parts
	var totalSize int64
	var filePath string
	for _, media := range m.Media {
		for _, part := range media.Part {
			totalSize += part.Size
			if filePath == "" {
				filePath = part.File
			}
		}
	}

	// Build genre string
	genres := make([]string, 0, len(m.Genre))
	for _, g := range m.Genre {
		genres = append(genres, g.Tag)
	}

	// Build collections list
	collections := make([]string, 0, len(m.Collection))
	for _, c := range m.Collection {
		collections = append(collections, c.Tag)
	}

	// Build labels list
	labels := make([]string, 0, len(m.Label))
	for _, l := range m.Label {
		labels = append(labels, l.Tag)
	}

	// Pick best rating
	rating := m.AudienceRating
	if rating == 0 {
		rating = m.Rating
	}

	// Convert timestamps
	var lastPlayed *time.Time
	if m.LastViewedAt > 0 {
		t := time.Unix(m.LastViewedAt, 0)
		lastPlayed = &t
	}

	var addedAt *time.Time
	if m.AddedAt > 0 {
		t := time.Unix(m.AddedAt, 0)
		addedAt = &t
	}

	var mediaType MediaType
	switch MediaType(m.Type) { //nolint:exhaustive // Plex only returns movie, show, season, and episode types
	case MediaTypeMovie:
		mediaType = MediaTypeMovie
	case MediaTypeShow:
		mediaType = MediaTypeShow
	case MediaTypeSeason:
		mediaType = MediaTypeSeason
	case MediaTypeEpisode:
		mediaType = MediaTypeEpisode
	default:
		return nil
	}

	item := &MediaItem{
		ExternalID:  m.RatingKey,
		Type:        mediaType,
		Title:       m.Title,
		Year:        m.Year,
		TMDbID:      plexExtractTMDbID(m.GUIDs),
		SizeBytes:   totalSize,
		Path:        filePath,
		Rating:      rating,
		Genre:       strings.Join(genres, ", "),
		PlayCount:   m.ViewCount,
		LastPlayed:  lastPlayed,
		AddedAt:     addedAt,
		Collections: collections,
		Labels:      labels,
	}

	// Show/season specifics
	if m.Type == "season" {
		item.SeasonNumber = m.Index
		item.EpisodeCount = m.LeafCount
		item.ShowTitle = m.ParentTitle
	}

	return item
}

// GetLibrarySections returns the library sections for display purposes
func (p *PlexClient) GetLibrarySections() ([]PlexLibrarySection, error) {
	body, err := p.doRequest("/library/sections")
	if err != nil {
		return nil, err
	}

	var libs plexLibraryResponse
	if err := json.Unmarshal(body, &libs); err != nil {
		return nil, fmt.Errorf("failed to parse library sections: %w", err)
	}

	sections := make([]PlexLibrarySection, len(libs.MediaContainer.Directory))
	for i, d := range libs.MediaContainer.Directory {
		sections[i] = PlexLibrarySection{
			Key:   d.Key,
			Title: d.Title,
			Type:  d.Type,
		}
	}
	return sections, nil
}

// PlexLibrarySection represents a Plex library section
type PlexLibrarySection struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// fetchAccounts retrieves the account ID → username mapping from the Plex server.
// Returns an empty map (not nil) on error — this is non-fatal since callers fall
// back to using numeric account IDs in WatchedByUsers.
func (p *PlexClient) fetchAccounts() map[int]string {
	body, err := p.doRequest("/accounts")
	if err != nil {
		slog.Debug("Failed to fetch Plex accounts — will use account IDs instead of names",
			"component", "plex", "error", err)
		return make(map[int]string)
	}

	var resp plexAccountsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		slog.Debug("Failed to parse Plex accounts response",
			"component", "plex", "error", err)
		return make(map[int]string)
	}

	accounts := make(map[int]string, len(resp.MediaContainer.Account))
	for _, a := range resp.MediaContainer.Account {
		if a.Name != "" {
			accounts[a.ID] = a.Name
		}
	}
	return accounts
}

// fetchAllHistory fetches the complete play history from Plex in paginated chunks.
// Uses GET /status/sessions/history/all which returns play events for ALL users
// when called with an admin token. Pagination uses X-Plex-Container-Start and
// X-Plex-Container-Size query parameters.
//
// On mid-pagination failure, returns accumulated entries with a nil error (partial
// data is better than no data for large libraries). On total failure (first page
// errors), returns nil entries with the error.
func (p *PlexClient) fetchAllHistory() ([]plexHistoryEntry, error) {
	var allEntries []plexHistoryEntry
	start := 0

	for {
		endpoint := fmt.Sprintf(
			"/status/sessions/history/all?X-Plex-Container-Start=%d&X-Plex-Container-Size=%d",
			start, plexHistoryPageSize,
		)
		body, err := p.doRequest(endpoint)
		if err != nil {
			if len(allEntries) > 0 {
				slog.Warn("Plex history fetch failed mid-pagination — returning partial results",
					"component", "plex", "accumulated", len(allEntries), "error", err)
				return allEntries, nil
			}
			return nil, fmt.Errorf("plex history: %w", err)
		}

		var resp plexHistoryResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			if len(allEntries) > 0 {
				slog.Warn("Plex history parse failed mid-pagination — returning partial results",
					"component", "plex", "accumulated", len(allEntries), "error", err)
				return allEntries, nil
			}
			return nil, fmt.Errorf("plex history parse: %w", err)
		}

		allEntries = append(allEntries, resp.MediaContainer.Metadata...)

		// Stop when all records have been fetched or the page was empty
		if resp.MediaContainer.TotalSize > 0 && len(allEntries) >= resp.MediaContainer.TotalSize {
			break
		}
		if len(resp.MediaContainer.Metadata) == 0 {
			break
		}
		start += len(resp.MediaContainer.Metadata)
	}

	return allEntries, nil
}

// GetBulkWatchData returns a map of TMDb ID to watch data aggregated across ALL
// Plex users. Uses /status/sessions/history/all (admin-only) as the primary source,
// falling back to per-token viewCount from library metadata if the history endpoint
// is unavailable (e.g., non-admin token or older Plex server).
//
// The history-based path provides correct multi-user play counts, per-user tracking
// (WatchedByUsers), and accurate LastPlayed timestamps. The fallback path only returns
// the token owner's watch data (the pre-fix behavior).
func (p *PlexClient) GetBulkWatchData() (map[int]*WatchData, error) {
	items, err := p.getMediaItems()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Plex items for watch data: %w", err)
	}

	// Build ratingKey → TMDb ID reverse map and TMDb ID → AddedAt map
	// from cached library metadata. The ratingKey map translates Plex-native
	// ratingKeys in history entries to TMDb IDs for matching. The AddedAt map
	// bridges the media server's library date to WatchData for the enrichment pipeline.
	ratingKeyToTMDb := make(map[string]int)
	tmdbAddedAt := make(map[int]*time.Time)
	for _, item := range items {
		if item.TMDbID > 0 && item.ExternalID != "" {
			ratingKeyToTMDb[item.ExternalID] = item.TMDbID
		}
		if item.TMDbID > 0 && item.AddedAt != nil {
			tmdbAddedAt[item.TMDbID] = item.AddedAt
		}
	}

	// Primary path: fetch all-user history from /status/sessions/history/all
	history, err := p.fetchAllHistory()
	if err != nil {
		slog.Warn("Plex history API unavailable — falling back to per-token viewCount",
			"component", "plex", "error", err)
		return p.getBulkWatchDataFallback(items)
	}
	if len(history) == 0 {
		// Empty history is valid (new server, no plays yet) — don't fall back,
		// return zero-count entries for all library items.
		return p.buildEmptyWatchData(items), nil
	}

	// Fetch account ID → username mapping for WatchedByUsers
	accounts := p.fetchAccounts()

	// Aggregate history entries by effective ratingKey.
	// Movies: use ratingKey directly.
	// Episodes: use grandparentRatingKey (the show's key) to aggregate all
	//           episode watches under the parent show.
	type historyAgg struct {
		playCount  int
		lastPlayed int64 // Unix epoch of most recent play
		accountIDs map[int]bool
	}
	aggByKey := make(map[string]*historyAgg)

	for _, entry := range history {
		var key string
		switch MediaType(entry.Type) { //nolint:exhaustive // History only contains movie, episode, track types
		case MediaTypeEpisode:
			key = entry.GrandparentRatingKey
			if key == "" {
				key = entry.RatingKey // fallback for missing grandparent
			}
		default:
			key = entry.RatingKey
		}
		if key == "" {
			continue
		}

		agg, ok := aggByKey[key]
		if !ok {
			agg = &historyAgg{accountIDs: make(map[int]bool)}
			aggByKey[key] = agg
		}
		agg.playCount++
		agg.accountIDs[entry.AccountID] = true
		if entry.ViewedAt > agg.lastPlayed {
			agg.lastPlayed = entry.ViewedAt
		}
	}

	// Map aggregated data to TMDb IDs
	result := make(map[int]*WatchData)
	for ratingKey, agg := range aggByKey {
		tmdbID, ok := ratingKeyToTMDb[ratingKey]
		if !ok || tmdbID == 0 {
			continue
		}

		var lastPlayed *time.Time
		if agg.lastPlayed > 0 {
			t := time.Unix(agg.lastPlayed, 0)
			lastPlayed = &t
		}

		// Resolve account IDs to usernames
		users := make([]string, 0, len(agg.accountIDs))
		for id := range agg.accountIDs {
			if name, ok := accounts[id]; ok {
				users = append(users, name)
			} else {
				users = append(users, fmt.Sprintf("account:%d", id))
			}
		}
		sort.Strings(users)

		data := &WatchData{
			PlayCount:  agg.playCount,
			LastPlayed: lastPlayed,
			Users:      users,
			AddedAt:    tmdbAddedAt[tmdbID],
		}
		// Keep entry with highest play count if duplicates (e.g., same TMDb ID
		// mapped from multiple ratingKeys)
		if existing, ok := result[tmdbID]; ok {
			if data.PlayCount > existing.PlayCount {
				result[tmdbID] = data
			}
		} else {
			result[tmdbID] = data
		}
	}

	// Include zero-count entries for library items with no history
	for _, item := range items {
		if item.TMDbID == 0 {
			continue
		}
		if _, ok := result[item.TMDbID]; !ok {
			result[item.TMDbID] = &WatchData{AddedAt: item.AddedAt}
		}
	}

	slog.Info("Plex multi-user watch data aggregated",
		"component", "plex",
		"historyEntries", len(history),
		"uniqueItems", len(aggByKey),
		"tmdbMatched", len(result))

	return result, nil
}

// getBulkWatchDataFallback returns watch data using the per-token-user viewCount
// from library metadata. Used when /status/sessions/history/all is unavailable
// (e.g., non-admin token, older Plex server). This only returns the token owner's
// play counts — not aggregated across all users.
func (p *PlexClient) getBulkWatchDataFallback(items []MediaItem) (map[int]*WatchData, error) {
	result := make(map[int]*WatchData)
	for _, item := range items {
		if item.TMDbID == 0 {
			continue
		}
		data := &WatchData{
			PlayCount:  item.PlayCount,
			LastPlayed: item.LastPlayed,
			AddedAt:    item.AddedAt,
		}
		// Keep the entry with the highest play count if duplicates
		if existing, ok := result[item.TMDbID]; ok {
			if data.PlayCount > existing.PlayCount {
				result[item.TMDbID] = data
			}
		} else {
			result[item.TMDbID] = data
		}
	}
	return result, nil
}

// buildEmptyWatchData returns zero-count WatchData entries for all library items
// with TMDb IDs. Used when the history endpoint returns zero entries (new server,
// no plays yet) to distinguish "checked and empty" from "didn't check".
func (p *PlexClient) buildEmptyWatchData(items []MediaItem) map[int]*WatchData {
	result := make(map[int]*WatchData)
	for _, item := range items {
		if item.TMDbID == 0 {
			continue
		}
		if _, ok := result[item.TMDbID]; !ok {
			result[item.TMDbID] = &WatchData{AddedAt: item.AddedAt}
		}
	}
	return result
}

// GetTMDbToRatingKeyMap builds a mapping from TMDb ID to Plex ratingKey by
// scanning all movie and show libraries. This is used by the Tautulli enricher
// to translate TMDb IDs from *arr items into Plex rating keys for per-item
// watch history queries. Built and consumed within a single poll cycle — not cached.
func (p *PlexClient) GetTMDbToRatingKeyMap() (map[int]string, error) {
	items, err := p.getMediaItems()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Plex items for TMDb mapping: %w", err)
	}

	result := make(map[int]string)
	for _, item := range items {
		if item.TMDbID > 0 && item.ExternalID != "" {
			result[item.TMDbID] = item.ExternalID
		}
	}
	return result, nil
}

// GetOnDeckItems returns a set of TMDb IDs for items on the Plex "On Deck" list.
// On-deck items are those a user has started watching or that are next in a
// series they are watching — a strong signal of active interest.
// The returned map is keyed by TMDb ID for matching against *arr items.
func (p *PlexClient) GetOnDeckItems() (map[int]bool, error) {
	// includeGuids=1 is required for TMDb ID extraction, same as library fetch.
	body, err := p.doRequest("/library/onDeck?includeGuids=1")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Plex on-deck items: %w", err)
	}

	var resp plexMediaResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse Plex on-deck response: %w", err)
	}

	result := make(map[int]bool)
	for _, m := range resp.MediaContainer.Metadata {
		tmdbID := plexExtractTMDbID(m.GUIDs)
		if tmdbID > 0 {
			result[tmdbID] = true
		}
	}
	return result, nil
}

// GetCollectionNames returns a sorted, deduplicated list of collection names
// from all Plex libraries. This is used by FetchCollectionValues() to provide
// autocomplete options for collection-based rules without exposing GetMediaItems
// (which would make PlexClient satisfy MediaSource).
func (p *PlexClient) GetCollectionNames() ([]string, error) {
	items, err := p.getMediaItems()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Plex items for collections: %w", err)
	}

	seen := make(map[string]bool)
	for _, item := range items {
		for _, col := range item.Collections {
			name := strings.TrimSpace(col)
			if name != "" {
				seen[name] = true
			}
		}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// GetWatchlistItems implements WatchlistProvider by returning Plex on-deck items
// keyed by TMDb ID.
func (p *PlexClient) GetWatchlistItems() (map[int]bool, error) {
	return p.GetOnDeckItems()
}

// GetCollectionMemberships implements CollectionDataProvider by scanning all
// Plex libraries and building a TMDb ID → collection names map from metadata.
// This bridges Plex collection data onto *arr items via the CollectionEnricher.
func (p *PlexClient) GetCollectionMemberships() (map[int][]string, error) {
	items, err := p.getMediaItems()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Plex items for collection memberships: %w", err)
	}

	result := make(map[int][]string)
	for _, item := range items {
		if item.TMDbID == 0 || len(item.Collections) == 0 {
			continue
		}
		result[item.TMDbID] = item.Collections
	}
	return result, nil
}

// GetLabelMemberships implements LabelDataProvider by scanning all Plex
// libraries and building a TMDb ID → label names map from metadata.
// This bridges Plex label data onto *arr items via the LabelEnricher.
func (p *PlexClient) GetLabelMemberships() (map[int][]string, error) {
	items, err := p.getMediaItems()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Plex items for label memberships: %w", err)
	}

	result := make(map[int][]string)
	for _, item := range items {
		if item.TMDbID == 0 || len(item.Labels) == 0 {
			continue
		}
		result[item.TMDbID] = item.Labels
	}
	return result, nil
}

// GetLabelNames returns a sorted, deduplicated list of label names from all
// Plex libraries. Used by FetchLabelValues() for rule value autocomplete.
func (p *PlexClient) GetLabelNames() ([]string, error) {
	items, err := p.getMediaItems()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Plex items for labels: %w", err)
	}

	seen := make(map[string]bool)
	for _, item := range items {
		for _, lbl := range item.Labels {
			name := strings.TrimSpace(lbl)
			if name != "" {
				seen[name] = true
			}
		}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// doRequestWithMethod creates an HTTP request with a custom method (PUT, POST)
// and appends the Plex token as a query parameter. Used for label management.
func (p *PlexClient) doRequestWithMethod(method, endpoint string) error {
	sep := "?"
	if strings.Contains(endpoint, "?") {
		sep = "&"
	}
	fullURL := p.URL + endpoint + sep + "X-Plex-Token=" + p.Token
	return DoAPIRequestWithBody(method, fullURL, nil, "Accept", "application/json")
}

// AddLabel applies a label to a Plex item identified by ratingKey.
// Uses the Plex metadata endpoint: PUT /library/metadata/{ratingKey}?label[0].tag.tag={label}&label.locked=1
func (p *PlexClient) AddLabel(itemID string, label string) error {
	endpoint := fmt.Sprintf("/library/metadata/%s?label[0].tag.tag=%s&label.locked=1",
		url.PathEscape(itemID), url.QueryEscape(label))
	return p.doRequestWithMethod("PUT", endpoint)
}

// RemoveLabel removes a label from a Plex item identified by ratingKey.
// Uses the Plex metadata endpoint: PUT /library/metadata/{ratingKey}?label[].tag.tag-={label}&label.locked=1
func (p *PlexClient) RemoveLabel(itemID string, label string) error {
	endpoint := fmt.Sprintf("/library/metadata/%s?label[].tag.tag-=%s&label.locked=1",
		url.PathEscape(itemID), url.QueryEscape(label))
	return p.doRequestWithMethod("PUT", endpoint)
}

// GetPosterImage downloads the current primary poster for a Plex item.
// Uses /library/metadata/{ratingKey}/thumb to fetch the poster image.
func (p *PlexClient) GetPosterImage(itemID string) ([]byte, string, error) {
	endpoint := fmt.Sprintf("/library/metadata/%s/thumb", url.PathEscape(itemID))
	data, err := p.doRequest(endpoint)
	if err != nil {
		return nil, "", fmt.Errorf("fetch poster: %w", err)
	}
	return data, "image/jpeg", nil
}

// UploadPosterImage uploads a new primary poster to a Plex item.
// Plex requires multipart/form-data with a "file" field to both upload AND select
// the poster as active. A raw body POST only adds the poster to the selection list
// without making it the active poster.
func (p *PlexClient) UploadPosterImage(itemID string, imageData []byte, _ string) error {
	sep := "?"
	endpoint := fmt.Sprintf("/library/metadata/%s/posters", url.PathEscape(itemID))
	if strings.Contains(endpoint, "?") {
		sep = "&"
	}
	fullURL := p.URL + endpoint + sep + "X-Plex-Token=" + p.Token
	return DoMultipartUpload(fullURL, imageData, "file", "poster.jpg", nil)
}

// RestorePosterImage removes any custom poster from a Plex item, reverting to
// the default agent-sourced poster. Achieved by unlocking the poster field.
func (p *PlexClient) RestorePosterImage(itemID string) error {
	endpoint := fmt.Sprintf("/library/metadata/%s?thumb=&poster.locked=0",
		url.PathEscape(itemID))
	return p.doRequestWithMethod("PUT", endpoint)
}

// SearchByTMDbID searches Plex for an item matching the given TMDb ID.
// Uses title to narrow the search space via /hubs/search, then verifies
// the TMDb ID in the Guid array of each result. Returns the ratingKey
// of the matched item.
func (p *PlexClient) SearchByTMDbID(title string, tmdbID int) (string, error) {
	if title == "" || tmdbID <= 0 {
		return "", fmt.Errorf("title and tmdbID are required for Plex search")
	}

	endpoint := fmt.Sprintf("/hubs/search?query=%s&includeGuids=1&limit=25", url.QueryEscape(title))
	body, err := p.doRequest(endpoint)
	if err != nil {
		return "", fmt.Errorf("plex search: %w", err)
	}

	// Plex search returns hubs containing metadata arrays
	var resp struct {
		MediaContainer struct {
			Hub []struct {
				Metadata []plexMetadata `json:"Metadata"`
			} `json:"Hub"`
		} `json:"MediaContainer"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("plex search unmarshal: %w", err)
	}

	for _, hub := range resp.MediaContainer.Hub {
		for _, m := range hub.Metadata {
			if plexExtractTMDbID(m.GUIDs) == tmdbID {
				return m.RatingKey, nil
			}
		}
	}

	return "", fmt.Errorf("plex search: no item found with TMDb ID %d", tmdbID)
}

// Verify PlexClient satisfies capability interfaces at compile time.
// Note: PlexClient intentionally does NOT implement MediaSource — only *arr integrations should.
var _ Connectable = (*PlexClient)(nil)
var _ WatchDataProvider = (*PlexClient)(nil)
var _ WatchlistProvider = (*PlexClient)(nil)
var _ CollectionDataProvider = (*PlexClient)(nil)
var _ LabelDataProvider = (*PlexClient)(nil)
var _ LabelManager = (*PlexClient)(nil)
var _ PosterManager = (*PlexClient)(nil)
var _ NativeIDSearcher = (*PlexClient)(nil)
