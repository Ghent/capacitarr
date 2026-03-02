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
	Token string // X-Plex-Token
}

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

func (p *PlexClient) TestConnection() error {
	_, err := p.doRequest("/identity")
	return err
}

// Plex doesn't report disk space directly through its API,
// so we return an empty slice. Disk info comes from *arr services.
func (p *PlexClient) GetDiskSpace() ([]DiskSpace, error) {
	return []DiskSpace{}, nil
}

// Plex doesn't have root folders in the *arr sense
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
	Media []struct {
		Part []struct {
			File string `json:"file"`
			Size int64  `json:"size"`
		} `json:"Part"`
	} `json:"Media"`
	Index     int `json:"index,omitempty"`     // season/episode number
	LeafCount int `json:"leafCount,omitempty"` // episode count (for shows/seasons)
}

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
		if lib.Type != "movie" && lib.Type != "show" {
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
	switch m.Type {
	case "movie":
		mediaType = MediaTypeMovie
	case "show":
		mediaType = MediaTypeShow
	case "season":
		mediaType = MediaTypeSeason
	case "episode":
		mediaType = MediaTypeEpisode
	default:
		return nil
	}

	item := &MediaItem{
		ExternalID: m.RatingKey,
		Type:       mediaType,
		Title:      m.Title,
		Year:       m.Year,
		SizeBytes:  totalSize,
		Path:       filePath,
		Rating:     rating,
		Genre:      strings.Join(genres, ", "),
		PlayCount:  m.ViewCount,
		LastPlayed: lastPlayed,
		AddedAt:    addedAt,
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

// GetWatchHistory enriches existing media items with Plex watch data.
// Call this to overlay play counts and last watched dates onto items from *arr services.
func (p *PlexClient) GetWatchHistory(ratingKey string) (*WatchData, error) {
	body, err := p.doRequest(fmt.Sprintf("/library/metadata/%s", ratingKey))
	if err != nil {
		return nil, err
	}

	var resp plexMediaResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	if len(resp.MediaContainer.Metadata) == 0 {
		return nil, fmt.Errorf("no metadata found for key %s", ratingKey)
	}

	m := resp.MediaContainer.Metadata[0]
	data := &WatchData{
		PlayCount: m.ViewCount,
	}
	if m.LastViewedAt > 0 {
		t := time.Unix(m.LastViewedAt, 0)
		data.LastPlayed = &t
	}
	if m.AddedAt > 0 {
		t := time.Unix(m.AddedAt, 0)
		data.AddedAt = &t
	}

	return data, nil
}

// WatchData represents watch history from Plex
type WatchData struct {
	PlayCount  int        `json:"playCount"`
	LastPlayed *time.Time `json:"lastPlayed,omitempty"`
	AddedAt    *time.Time `json:"addedAt,omitempty"`
	RatingKey  string     `json:"ratingKey,omitempty"`
}

// GetServerIdentity returns basic server info for display
func (p *PlexClient) GetServerIdentity() (*PlexServerInfo, error) {
	body, err := p.doRequest("/identity")
	if err != nil {
		return nil, err
	}

	var resp struct {
		MediaContainer struct {
			MachineIdentifier string `json:"machineIdentifier"`
			Version           string `json:"version"`
		} `json:"MediaContainer"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	return &PlexServerInfo{
		MachineID: resp.MediaContainer.MachineIdentifier,
		Version:   resp.MediaContainer.Version,
	}, nil
}

// PlexServerInfo contains basic Plex server identity
type PlexServerInfo struct {
	MachineID string `json:"machineId"`
	Version   string `json:"version"`
}

// Ensure PlexClient implements Integration
var _ Integration = (*PlexClient)(nil)

func (p *PlexClient) DeleteMediaItem(_ MediaItem) error {
	// Plex is read-only for watch history in this architecture.
	// Actual deletion happens via Radarr/Sonarr.
	return nil
}
