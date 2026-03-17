package integrations

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// LidarrClient implements Integration for Lidarr v1 API (Servarr framework).
// Lidarr manages music libraries and follows the same API patterns as Sonarr/Radarr.
type LidarrClient struct {
	URL    string
	APIKey string `json:"-"`
}

// NewLidarrClient creates a new Lidarr music management API client.
func NewLidarrClient(url, apiKey string) *LidarrClient {
	return &LidarrClient{
		URL:    strings.TrimRight(url, "/"),
		APIKey: apiKey,
	}
}

func (l *LidarrClient) doRequest(endpoint string) ([]byte, error) {
	return DoAPIRequest(l.URL+endpoint, "X-Api-Key", l.APIKey)
}

// TestConnection verifies the Lidarr server is reachable and the API key is valid.
func (l *LidarrClient) TestConnection() error {
	_, err := l.doRequest("/api/v1/system/status")
	return err
}

// GetDiskSpace returns disk usage information reported by Lidarr.
func (l *LidarrClient) GetDiskSpace() ([]DiskSpace, error) {
	return arrFetchDiskSpace(l.doRequest, "/api/v1")
}

// GetRootFolders returns the configured root folder paths from Lidarr.
func (l *LidarrClient) GetRootFolders() ([]string, error) {
	return arrFetchRootFolders(l.doRequest, "/api/v1")
}

// lidarrArtist maps the Lidarr artist API response (relevant fields)
type lidarrArtist struct {
	ID         int    `json:"id"`
	ArtistName string `json:"artistName"`
	Path       string `json:"path"`
	Monitored  bool   `json:"monitored"`
	Ratings    struct {
		Value float64 `json:"value"`
	} `json:"ratings"`
	Genres           []string   `json:"genres"`
	Tags             []int      `json:"tags"`
	QualityProfileID int        `json:"qualityProfileId"`
	Added            string     `json:"added"`
	Images           []arrImage `json:"images"`
	Statistics       struct {
		SizeOnDisk int64 `json:"sizeOnDisk"`
		AlbumCount int   `json:"albumCount"`
		TrackCount int   `json:"trackCount"`
	} `json:"statistics"`
}

// GetMediaItems fetches all artists and their albums from Lidarr.
func (l *LidarrClient) GetMediaItems() ([]MediaItem, error) {
	// Fetch quality profiles for name lookup
	profileMap, err := arrFetchQualityProfileMap(l.doRequest, "/api/v1")
	if err != nil {
		return nil, err
	}

	// Fetch tags for name lookup
	tagMap, err := arrFetchTagMap(l.doRequest, "/api/v1")
	if err != nil {
		return nil, err
	}

	// Fetch all artists
	body, err := l.doRequest("/api/v1/artist")
	if err != nil {
		return nil, err
	}

	var artists []lidarrArtist
	if err := json.Unmarshal(body, &artists); err != nil {
		return nil, fmt.Errorf("failed to parse artists: %w", err)
	}

	items := make([]MediaItem, 0, len(artists))
	for _, a := range artists {
		if a.Statistics.SizeOnDisk == 0 {
			continue // Skip artists without files on disk
		}

		// Lidarr ratings.value is already on a 0–10 scale (MusicBrainz).
		// Pass through directly — the scoring engine normalizes 0–10 at score time.
		rating := a.Ratings.Value

		tagNames := arrResolveTagNames(a.Tags, tagMap)

		var addedAt *time.Time
		if a.Added != "" {
			if t, err := time.Parse(time.RFC3339, a.Added); err == nil {
				addedAt = &t
			}
		}

		items = append(items, MediaItem{
			ExternalID:     strconv.Itoa(a.ID),
			Type:           MediaTypeArtist,
			Title:          a.ArtistName,
			SizeBytes:      a.Statistics.SizeOnDisk,
			Path:           a.Path,
			PosterURL:      arrExtractPosterURL(a.Images),
			QualityProfile: profileMap[a.QualityProfileID],
			Rating:         rating,
			Genre:          strings.Join(a.Genres, ", "),
			Monitored:      a.Monitored,
			Tags:           tagNames,
			AddedAt:        addedAt,
		})
	}

	return items, nil
}

// --- RuleValueFetcher implementation ---

// GetQualityProfiles returns available quality profiles from Lidarr.
func (l *LidarrClient) GetQualityProfiles() ([]NameValue, error) {
	return arrFetchQualityProfiles(l.doRequest, "/api/v1")
}

// GetTags returns all tags configured in Lidarr.
func (l *LidarrClient) GetTags() ([]NameValue, error) {
	return arrFetchTags(l.doRequest, "/api/v1")
}

// GetLanguages returns nil because Lidarr does not support language lookup.
func (l *LidarrClient) GetLanguages() ([]NameValue, error) {
	// Lidarr does not have a language endpoint
	return nil, nil
}

// DeleteMediaItem removes an artist and its files from disk via the Lidarr API.
func (l *LidarrClient) DeleteMediaItem(item MediaItem) error {
	endpoint := fmt.Sprintf("/api/v1/artist/%s?deleteFiles=true", item.ExternalID)
	return arrSimpleDelete(l.URL, l.APIKey, endpoint)
}
