package integrations

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ReadarrClient implements Integration for Readarr v1 API (books/audiobooks).
// Follows the same API pattern as Sonarr/Radarr/Lidarr.
type ReadarrClient struct {
	URL    string
	APIKey string `json:"-"`
}

// NewReadarrClient creates a new Readarr book management API client.
func NewReadarrClient(url, apiKey string) *ReadarrClient {
	return &ReadarrClient{
		URL:    strings.TrimRight(url, "/"),
		APIKey: apiKey,
	}
}

func (r *ReadarrClient) doRequest(endpoint string) ([]byte, error) {
	return DoAPIRequest(r.URL+endpoint, "X-Api-Key", r.APIKey)
}

// TestConnection verifies the Readarr server is reachable and the API key is valid.
func (r *ReadarrClient) TestConnection() error {
	_, err := r.doRequest("/api/v1/system/status")
	return err
}

// GetDiskSpace returns disk space info from Readarr
func (r *ReadarrClient) GetDiskSpace() ([]DiskSpace, error) {
	return arrFetchDiskSpace(r.doRequest, "/api/v1")
}

// GetRootFolders returns root folder paths configured in Readarr
func (r *ReadarrClient) GetRootFolders() ([]string, error) {
	return arrFetchRootFolders(r.doRequest, "/api/v1")
}

// readarrBook maps a Readarr book API response (relevant fields)
type readarrBook struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	AuthorID int    `json:"authorId"`
	Author   struct {
		AuthorName string `json:"authorName"`
	} `json:"author"`
	SizeOnDisk int64      `json:"sizeOnDisk"`
	Added      string     `json:"added"`
	Monitored  bool       `json:"monitored"`
	Path       string     `json:"path"`
	Images     []arrImage `json:"images"`
	Ratings    struct {
		Value float64 `json:"value"`
	} `json:"ratings"`
	Genres           []string `json:"genres"`
	Tags             []int    `json:"tags"`
	QualityProfileID int      `json:"qualityProfileId"`
}

// GetMediaItems fetches all books from Readarr with quality, tag, and rating metadata.
func (r *ReadarrClient) GetMediaItems() ([]MediaItem, error) {
	// Fetch quality profiles for name lookup
	profileMap, err := arrFetchQualityProfileMap(r.doRequest, "/api/v1")
	if err != nil {
		return nil, err
	}

	// Fetch tags for name lookup
	tagMap, err := arrFetchTagMap(r.doRequest, "/api/v1")
	if err != nil {
		return nil, err
	}

	// Fetch all books
	body, err := r.doRequest("/api/v1/book")
	if err != nil {
		return nil, err
	}
	var books []readarrBook
	if err := json.Unmarshal(body, &books); err != nil {
		return nil, fmt.Errorf("failed to parse Readarr books: %w", err)
	}

	items := make([]MediaItem, 0, len(books))
	for _, b := range books {
		if b.SizeOnDisk == 0 {
			continue
		}

		var addedAt *time.Time
		if b.Added != "" {
			t, err := time.Parse(time.RFC3339, b.Added)
			if err == nil {
				addedAt = &t
			}
		}

		tagNames := arrResolveTagNames(b.Tags, tagMap)

		// Pick genre string from first genre if available
		genre := ""
		if len(b.Genres) > 0 {
			genre = b.Genres[0]
		}

		items = append(items, MediaItem{
			ExternalID:     fmt.Sprintf("%d", b.ID),
			Title:          b.Title,
			Type:           MediaTypeBook,
			SizeBytes:      b.SizeOnDisk,
			AddedAt:        addedAt,
			Monitored:      b.Monitored,
			Path:           b.Path,
			PosterURL:      arrExtractPosterURL(b.Images),
			Rating:         b.Ratings.Value,
			Genre:          genre,
			Tags:           tagNames,
			QualityProfile: profileMap[b.QualityProfileID],
		})
	}
	return items, nil
}

// --- RuleValueFetcher implementation ---

// GetQualityProfiles returns available quality profiles from Readarr.
func (r *ReadarrClient) GetQualityProfiles() ([]NameValue, error) {
	return arrFetchQualityProfiles(r.doRequest, "/api/v1")
}

// GetTags returns all tags configured in Readarr.
func (r *ReadarrClient) GetTags() ([]NameValue, error) {
	return arrFetchTags(r.doRequest, "/api/v1")
}

// GetLanguages returns all languages configured in Readarr.
func (r *ReadarrClient) GetLanguages() ([]NameValue, error) {
	return arrFetchLanguages(r.doRequest, "/api/v1")
}

// DeleteMediaItem removes a book from Readarr and optionally deletes files
func (r *ReadarrClient) DeleteMediaItem(item MediaItem) error {
	endpoint := fmt.Sprintf("/api/v1/book/%s?deleteFiles=true&addImportExclusion=false", item.ExternalID)
	return arrSimpleDelete(r.URL, r.APIKey, endpoint)
}
