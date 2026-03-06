package integrations

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RadarrClient implements Integration for Radarr v3 API
type RadarrClient struct {
	URL    string
	APIKey string
}

// NewRadarrClient creates a new Radarr movie management API client.
func NewRadarrClient(url, apiKey string) *RadarrClient {
	return &RadarrClient{
		URL:    strings.TrimRight(url, "/"),
		APIKey: apiKey,
	}
}

func (r *RadarrClient) doRequest(endpoint string) ([]byte, error) {
	return DoAPIRequest(r.URL+endpoint, "X-Api-Key", r.APIKey)
}

// TestConnection verifies the Radarr server is reachable and the API key is valid.
func (r *RadarrClient) TestConnection() error {
	_, err := r.doRequest("/api/v3/system/status")
	return err
}

// GetDiskSpace returns disk usage information reported by Radarr.
func (r *RadarrClient) GetDiskSpace() ([]DiskSpace, error) {
	return arrFetchDiskSpace(r.doRequest, "/api/v3")
}

// GetRootFolders returns the configured root folder paths from Radarr.
func (r *RadarrClient) GetRootFolders() ([]string, error) {
	return arrFetchRootFolders(r.doRequest, "/api/v3")
}

// radarrMovie maps the Radarr movie API response (relevant fields)
type radarrMovie struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	Year       int    `json:"year"`
	Path       string `json:"path"`
	Monitored  bool   `json:"monitored"`
	HasFile    bool   `json:"hasFile"`
	SizeOnDisk int64  `json:"sizeOnDisk"`
	Ratings    struct {
		IMDB struct {
			Value float64 `json:"value"`
		} `json:"imdb"`
		TMDB struct {
			Value float64 `json:"value"`
		} `json:"tmdb"`
	} `json:"ratings"`
	Genres           []string   `json:"genres"`
	Tags             []int      `json:"tags"`
	QualityProfileID int        `json:"qualityProfileId"`
	Added            string     `json:"added"`
	Images           []arrImage `json:"images"`
}

// GetMediaItems fetches all movies from Radarr with quality and tag metadata.
func (r *RadarrClient) GetMediaItems() ([]MediaItem, error) {
	// Fetch quality profiles for name lookup
	profileMap, err := arrFetchQualityProfileMap(r.doRequest, "/api/v3")
	if err != nil {
		return nil, err
	}

	// Fetch tags for name lookup
	tagMap, err := arrFetchTagMap(r.doRequest, "/api/v3")
	if err != nil {
		return nil, err
	}

	// Fetch all movies
	body, err := r.doRequest("/api/v3/movie")
	if err != nil {
		return nil, err
	}

	var movies []radarrMovie
	if err := json.Unmarshal(body, &movies); err != nil {
		return nil, fmt.Errorf("failed to parse movies: %w", err)
	}

	items := make([]MediaItem, 0, len(movies))
	for _, m := range movies {
		if !m.HasFile {
			continue // Skip movies without files
		}

		// Pick best available rating
		rating := m.Ratings.IMDB.Value
		if rating == 0 {
			rating = m.Ratings.TMDB.Value
		}

		tagNames := arrResolveTagNames(m.Tags, tagMap)

		var addedAt *time.Time
		if m.Added != "" {
			if t, err := time.Parse(time.RFC3339, m.Added); err == nil {
				addedAt = &t
			}
		}

		items = append(items, MediaItem{
			ExternalID:     strconv.Itoa(m.ID),
			Type:           MediaTypeMovie,
			Title:          m.Title,
			Year:           m.Year,
			SizeBytes:      m.SizeOnDisk,
			Path:           m.Path,
			PosterURL:      arrExtractPosterURL(m.Images),
			QualityProfile: profileMap[m.QualityProfileID],
			Rating:         rating,
			Genre:          strings.Join(m.Genres, ", "),
			Monitored:      m.Monitored,
			Tags:           tagNames,
			AddedAt:        addedAt,
		})
	}

	return items, nil
}

// --- RuleValueFetcher implementation ---

// GetQualityProfiles returns available quality profiles from Radarr.
func (r *RadarrClient) GetQualityProfiles() ([]NameValue, error) {
	return arrFetchQualityProfiles(r.doRequest, "/api/v3")
}

// GetTags returns all tags configured in Radarr.
func (r *RadarrClient) GetTags() ([]NameValue, error) {
	return arrFetchTags(r.doRequest, "/api/v3")
}

// GetLanguages returns all languages configured in Radarr.
func (r *RadarrClient) GetLanguages() ([]NameValue, error) {
	return arrFetchLanguages(r.doRequest, "/api/v3")
}

// DeleteMediaItem removes a movie and its files from disk via the Radarr API.
func (r *RadarrClient) DeleteMediaItem(item MediaItem) error {
	endpoint := fmt.Sprintf("/api/v3/movie/%s?deleteFiles=true", item.ExternalID)
	return arrSimpleDelete(r.URL, r.APIKey, endpoint)
}
