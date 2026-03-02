package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ReadarrClient implements Integration for Readarr v1 API (books/audiobooks).
// Follows the same API pattern as Sonarr/Radarr/Lidarr.
type ReadarrClient struct {
	URL    string
	APIKey string
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

func (r *ReadarrClient) TestConnection() error {
	_, err := r.doRequest("/api/v1/system/status")
	return err
}

// readarrDiskSpace maps the Readarr diskspace API response
type readarrDiskSpace struct {
	Path       string `json:"path"`
	TotalSpace int64  `json:"totalSpace"`
	FreeSpace  int64  `json:"freeSpace"`
}

// GetDiskSpace returns disk space info from Readarr
func (r *ReadarrClient) GetDiskSpace() ([]DiskSpace, error) {
	body, err := r.doRequest("/api/v1/diskspace")
	if err != nil {
		return nil, err
	}
	var disks []readarrDiskSpace
	if err := json.Unmarshal(body, &disks); err != nil {
		return nil, fmt.Errorf("failed to parse Readarr diskspace: %w", err)
	}
	result := make([]DiskSpace, 0, len(disks))
	for _, d := range disks {
		result = append(result, DiskSpace{
			Path:       d.Path,
			TotalBytes: d.TotalSpace,
			FreeBytes:  d.FreeSpace,
		})
	}
	return result, nil
}

// GetRootFolders returns root folder paths configured in Readarr
func (r *ReadarrClient) GetRootFolders() ([]string, error) {
	body, err := r.doRequest("/api/v1/rootfolder")
	if err != nil {
		return nil, err
	}
	var folders []struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(body, &folders); err != nil {
		return nil, fmt.Errorf("failed to parse Readarr root folders: %w", err)
	}
	paths := make([]string, 0, len(folders))
	for _, f := range folders {
		paths = append(paths, f.Path)
	}
	return paths, nil
}

// readarrBook maps a Readarr book response
type readarrBook struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	AuthorID int    `json:"authorId"`
	Author   struct {
		AuthorName string `json:"authorName"`
	} `json:"author"`
	SizeOnDisk int64  `json:"sizeOnDisk"`
	Added      string `json:"added"`
	Monitored  bool   `json:"monitored"`
	Path       string `json:"path"`
}

// GetMediaItems returns all books as MediaItems for scoring
func (r *ReadarrClient) GetMediaItems() ([]MediaItem, error) {
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

		items = append(items, MediaItem{
			ExternalID: fmt.Sprintf("%d", b.ID),
			Title:      b.Title,
			Type:       MediaTypeBook,
			SizeBytes:  b.SizeOnDisk,
			AddedAt:    addedAt,
			Monitored:  b.Monitored,
			Path:       b.Path,
		})
	}
	return items, nil
}

// --- RuleValueFetcher implementation ---

func (r *ReadarrClient) GetQualityProfiles() ([]NameValue, error) {
	body, err := r.doRequest("/api/v1/qualityprofile")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quality profiles: %w", err)
	}
	var profiles []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &profiles); err != nil {
		return nil, fmt.Errorf("failed to parse quality profiles: %w", err)
	}
	result := make([]NameValue, len(profiles))
	for i, p := range profiles {
		result[i] = NameValue{Value: p.Name, Label: p.Name}
	}
	return result, nil
}

func (r *ReadarrClient) GetTags() ([]NameValue, error) {
	body, err := r.doRequest("/api/v1/tag")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}
	var tags []struct {
		ID    int    `json:"id"`
		Label string `json:"label"`
	}
	if err := json.Unmarshal(body, &tags); err != nil {
		return nil, fmt.Errorf("failed to parse tags: %w", err)
	}
	result := make([]NameValue, len(tags))
	for i, t := range tags {
		result[i] = NameValue{Value: t.Label, Label: t.Label}
	}
	return result, nil
}

func (r *ReadarrClient) GetLanguages() ([]NameValue, error) {
	body, err := r.doRequest("/api/v1/language")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch languages: %w", err)
	}
	var langs []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(body, &langs); err != nil {
		return nil, fmt.Errorf("failed to parse languages: %w", err)
	}
	result := make([]NameValue, len(langs))
	for i, l := range langs {
		result[i] = NameValue{Value: l.Name, Label: l.Name}
	}
	return result, nil
}

// DeleteMediaItem removes a book from Readarr and optionally deletes files
func (r *ReadarrClient) DeleteMediaItem(item MediaItem) error {
	endpoint := fmt.Sprintf("/api/v1/book/%s?deleteFiles=true&addImportExclusion=false", item.ExternalID)
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", r.URL+endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", r.APIKey)

	resp, err := sharedHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("unauthorized: invalid API key")
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}
