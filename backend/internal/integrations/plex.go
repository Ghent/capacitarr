package integrations

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PlexClient implements Integration for Plex Media Server
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

// GetDiskSpace returns an empty slice because Plex doesn't report disk space
// directly through its API. Disk info comes from *arr services.
func (p *PlexClient) GetDiskSpace() ([]DiskSpace, error) {
	return []DiskSpace{}, nil
}

// GetRootFolders returns an empty slice because Plex doesn't have root folders in the *arr sense.
func (p *PlexClient) GetRootFolders() ([]string, error) {
	return []string{}, nil
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
	RatingKey        string  `json:"ratingKey"`
	Title            string  `json:"title"`
	ParentTitle      string  `json:"parentTitle,omitempty"`
	GrandparentTitle string  `json:"grandparentTitle,omitempty"`
	Year             int     `json:"year"`
	Type             string  `json:"type"` // movie, show, season, episode
	AudienceRating   float64 `json:"audienceRating"`
	Rating           float64 `json:"rating"`
	ViewCount        int     `json:"viewCount"`
	LastViewedAt     int64   `json:"lastViewedAt"`
	AddedAt          int64   `json:"addedAt"`
	Duration         int64   `json:"duration"`
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

// GetMediaItems fetches all movies, shows, and seasons from all Plex libraries.
func (p *PlexClient) GetMediaItems() ([]MediaItem, error) {
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
// a map from normalized (lowercase) title to watch data. This allows enriching
// *arr items with Plex watch history by title matching.
func (p *PlexClient) GetBulkWatchData() (map[string]*WatchData, error) {
	items, err := p.GetMediaItems()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Plex items: %w", err)
	}

	result := make(map[string]*WatchData)
	for _, item := range items {
		key := strings.ToLower(strings.TrimSpace(item.Title))
		if key == "" {
			continue
		}
		data := &WatchData{
			PlayCount:  item.PlayCount,
			LastPlayed: item.LastPlayed,
		}
		// Keep the entry with the highest play count if duplicates
		if existing, ok := result[key]; ok {
			if data.PlayCount > existing.PlayCount {
				result[key] = data
			}
		} else {
			result[key] = data
		}
	}
	return result, nil
}

// GetOnDeckItems returns a set of normalized title keys for items on the Plex
// "On Deck" list. On-deck items are those a user has started watching or that
// are next in a series they are watching — a strong signal of active interest.
// The returned map is keyed by lowercase title for matching against *arr items.
func (p *PlexClient) GetOnDeckItems() (map[string]bool, error) {
	body, err := p.doRequest("/library/onDeck")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Plex on-deck items: %w", err)
	}

	var resp plexMediaResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse Plex on-deck response: %w", err)
	}

	result := make(map[string]bool)
	for _, m := range resp.MediaContainer.Metadata {
		// For episodes, use the show title (grandparentTitle) so the show-level
		// item from *arr will match. For movies, use the title directly.
		key := strings.ToLower(strings.TrimSpace(m.Title))
		if m.GrandparentTitle != "" {
			key = strings.ToLower(strings.TrimSpace(m.GrandparentTitle))
		}
		if key != "" {
			result[key] = true
		}
	}
	return result, nil
}

// GetWatchlistItems implements WatchlistProvider by returning Plex on-deck items.
func (p *PlexClient) GetWatchlistItems() (map[string]bool, error) {
	return p.GetOnDeckItems()
}

// Ensure PlexClient implements Integration + WatchlistProvider
// Verify PlexClient satisfies capability interfaces at compile time.
var _ Connectable = (*PlexClient)(nil)
var _ MediaSource = (*PlexClient)(nil)
var _ WatchDataProvider = (*PlexClient)(nil)
var _ WatchlistProvider = (*PlexClient)(nil)

// DeleteMediaItem is a no-op for Plex; actual deletion is performed via *arr services.
func (p *PlexClient) DeleteMediaItem(_ MediaItem) error {
	// Plex is read-only for watch history in this architecture.
	// Actual deletion happens via Radarr/Sonarr.
	return nil
}
