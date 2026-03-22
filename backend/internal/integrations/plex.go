package integrations

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// PlexClient implements Connectable, WatchDataProvider, and WatchlistProvider for Plex Media Server.
// PlexClient intentionally does NOT implement MediaSource — only *arr integrations (which also
// implement MediaDeleter and DiskReporter) should provide media items to the evaluation pool.
type PlexClient struct {
	URL   string
	Token string `json:"-"` // X-Plex-Token
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
	RatingKey        string     `json:"ratingKey"`
	Title            string     `json:"title"`
	ParentTitle      string     `json:"parentTitle,omitempty"`
	GrandparentTitle string     `json:"grandparentTitle,omitempty"`
	Year             int        `json:"year"`
	Type             string     `json:"type"` // movie, show, season, episode
	AudienceRating   float64    `json:"audienceRating"`
	Rating           float64    `json:"rating"`
	ViewCount        int        `json:"viewCount"`
	LastViewedAt     int64      `json:"lastViewedAt"`
	AddedAt          int64      `json:"addedAt"`
	Duration         int64      `json:"duration"`
	GUID             string     `json:"guid"`           // Primary GUID (e.g. "plex://movie/...")
	GUIDs            []plexGUID `json:"Guid,omitempty"` // Additional GUIDs including TMDb references
	Genre            []struct {
		Tag string `json:"tag"`
	} `json:"Genre"`
	Collection []struct {
		Tag string `json:"tag"`
	} `json:"Collection"`
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

// getMediaItems fetches all movies, shows, and seasons from all Plex libraries.
// This method is unexported to prevent PlexClient from satisfying the MediaSource interface.
// Only *arr integrations should implement MediaSource. Internal callers (GetBulkWatchData,
// GetCollectionNames) use this method for enrichment data extraction.
func (p *PlexClient) getMediaItems() ([]MediaItem, error) {
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

		// 2. Get all items in this library
		itemBody, err := p.doRequest(fmt.Sprintf("/library/sections/%s/all", lib.Key))
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

// GetBulkWatchData fetches all movies and shows from Plex libraries and returns
// a map keyed by TMDb ID to watch data. TMDb IDs are extracted from Plex GUIDs.
// Items without a parseable TMDb GUID are skipped.
func (p *PlexClient) GetBulkWatchData() (map[int]*WatchData, error) {
	body, err := p.doRequest("/library/sections")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Plex library sections: %w", err)
	}

	var libs plexLibraryResponse
	if err := json.Unmarshal(body, &libs); err != nil {
		return nil, fmt.Errorf("failed to parse library sections: %w", err)
	}

	result := make(map[int]*WatchData)
	for _, lib := range libs.MediaContainer.Directory {
		if lib.Type != string(MediaTypeMovie) && lib.Type != string(MediaTypeShow) {
			continue
		}

		itemBody, err := p.doRequest(fmt.Sprintf("/library/sections/%s/all", lib.Key))
		if err != nil {
			continue
		}

		var media plexMediaResponse
		if err := json.Unmarshal(itemBody, &media); err != nil {
			continue
		}

		for _, m := range media.MediaContainer.Metadata {
			tmdbID := plexExtractTMDbID(m.GUIDs)
			if tmdbID == 0 {
				continue
			}

			var lastPlayed *time.Time
			if m.LastViewedAt > 0 {
				t := time.Unix(m.LastViewedAt, 0)
				lastPlayed = &t
			}

			data := &WatchData{
				PlayCount:  m.ViewCount,
				LastPlayed: lastPlayed,
			}
			// Keep the entry with the highest play count if duplicates
			if existing, ok := result[tmdbID]; ok {
				if data.PlayCount > existing.PlayCount {
					result[tmdbID] = data
				}
			} else {
				result[tmdbID] = data
			}
		}
	}
	return result, nil
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
	body, err := p.doRequest("/library/onDeck")
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

// Verify PlexClient satisfies capability interfaces at compile time.
// Note: PlexClient intentionally does NOT implement MediaSource — only *arr integrations should.
var _ Connectable = (*PlexClient)(nil)
var _ WatchDataProvider = (*PlexClient)(nil)
var _ WatchlistProvider = (*PlexClient)(nil)
