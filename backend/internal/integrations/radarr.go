package integrations

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RadarrClient implements Connectable, MediaSource, DiskReporter, MediaDeleter, and RuleValueFetcher for Radarr v3 API.
type RadarrClient struct {
	URL    string
	APIKey string `json:"-"`
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
	ID               int         `json:"id"`
	Title            string      `json:"title"`
	Year             int         `json:"year"`
	TmdbID           int         `json:"tmdbId"`
	Path             string      `json:"path"`
	Monitored        bool        `json:"monitored"`
	HasFile          bool        `json:"hasFile"`
	SizeOnDisk       int64       `json:"sizeOnDisk"`
	OriginalLanguage arrLanguage `json:"originalLanguage"`
	Ratings          struct {
		IMDB struct {
			Value float64 `json:"value"`
		} `json:"imdb"`
		TMDB struct {
			Value float64 `json:"value"`
		} `json:"tmdb"`
	} `json:"ratings"`
	Genres           []string          `json:"genres"`
	Tags             []int             `json:"tags"`
	QualityProfileID int               `json:"qualityProfileId"`
	Added            string            `json:"added"`
	Images           []arrImage        `json:"images"`
	Collection       *radarrCollection `json:"collection,omitempty"` // TMDb collection (e.g., "Sonic the Hedgehog Collection")
}

// radarrCollection maps the Radarr API collection object.
type radarrCollection struct {
	Name   string `json:"name"`
	TmdbID int    `json:"tmdbId"`
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

		// Extract collection name if present
		var collections []string
		if m.Collection != nil && m.Collection.Name != "" {
			collections = []string{m.Collection.Name}
		}

		items = append(items, MediaItem{
			ExternalID:     strconv.Itoa(m.ID),
			Type:           MediaTypeMovie,
			Title:          m.Title,
			Year:           m.Year,
			TMDbID:         m.TmdbID,
			SizeBytes:      m.SizeOnDisk,
			Path:           m.Path,
			PosterURL:      arrExtractPosterURL(m.Images, r.URL),
			QualityProfile: profileMap[m.QualityProfileID],
			Rating:         rating,
			Genre:          strings.Join(m.Genres, ", "),
			Monitored:      m.Monitored,
			Language:       m.OriginalLanguage.Name,
			Tags:           tagNames,
			AddedAt:        addedAt,
			Collections:    collections,
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

// --- CollectionResolver implementation ---

// ResolveCollectionMembers returns all movies in the same TMDb collection as the
// given item. Fetches the full movie list from Radarr and groups by collection
// TMDb ID. Returns nil if the item has no collection membership.
func (r *RadarrClient) ResolveCollectionMembers(item MediaItem) ([]MediaItem, error) {
	if len(item.Collections) == 0 {
		return nil, nil
	}

	// Fetch all movies to find collection siblings
	body, err := r.doRequest("/api/v3/movie")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch movies for collection resolution: %w", err)
	}

	var movies []radarrMovie
	if err := json.Unmarshal(body, &movies); err != nil {
		return nil, fmt.Errorf("failed to parse movies for collection resolution: %w", err)
	}

	// Find the target collection TMDb ID by matching the item's collection name
	targetCollectionName := item.Collections[0]
	var targetCollectionTmdbID int
	for _, m := range movies {
		if m.Collection != nil && m.Collection.Name == targetCollectionName {
			targetCollectionTmdbID = m.Collection.TmdbID
			break
		}
	}
	if targetCollectionTmdbID == 0 {
		return nil, nil // Collection not found in Radarr library
	}

	// Fetch quality profiles and tags for full MediaItem construction
	profileMap, err := arrFetchQualityProfileMap(r.doRequest, "/api/v3")
	if err != nil {
		profileMap = make(map[int]string)
	}
	tagMap, err := arrFetchTagMap(r.doRequest, "/api/v3")
	if err != nil {
		tagMap = make(map[int]string)
	}

	// Collect all movies in the same collection
	var members []MediaItem
	for _, m := range movies {
		if !m.HasFile || m.Collection == nil || m.Collection.TmdbID != targetCollectionTmdbID {
			continue
		}

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

		members = append(members, MediaItem{
			ExternalID:     strconv.Itoa(m.ID),
			IntegrationID:  item.IntegrationID,
			Type:           MediaTypeMovie,
			Title:          m.Title,
			Year:           m.Year,
			TMDbID:         m.TmdbID,
			SizeBytes:      m.SizeOnDisk,
			Path:           m.Path,
			PosterURL:      arrExtractPosterURL(m.Images, r.URL),
			QualityProfile: profileMap[m.QualityProfileID],
			Rating:         rating,
			Genre:          strings.Join(m.Genres, ", "),
			Monitored:      m.Monitored,
			Language:       m.OriginalLanguage.Name,
			Tags:           tagNames,
			AddedAt:        addedAt,
			Collections:    []string{m.Collection.Name},
		})
	}

	if len(members) <= 1 {
		return nil, nil // Only the trigger item itself — no expansion needed
	}

	return members, nil
}

// Verify RadarrClient satisfies CollectionResolver at compile time.
var _ CollectionResolver = (*RadarrClient)(nil)
