package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RadarrClient implements Integration for Radarr v3 API
type RadarrClient struct {
	URL    string
	APIKey string
}

func NewRadarrClient(url, apiKey string) *RadarrClient {
	return &RadarrClient{
		URL:    strings.TrimRight(url, "/"),
		APIKey: apiKey,
	}
}

func (r *RadarrClient) doRequest(endpoint string) ([]byte, error) {
	return DoAPIRequest(r.URL+endpoint, "X-Api-Key", r.APIKey)
}

func (r *RadarrClient) TestConnection() error {
	_, err := r.doRequest("/api/v3/system/status")
	return err
}

// radarrDiskSpace maps the Radarr diskspace API response
type radarrDiskSpace struct {
	Path       string `json:"path"`
	TotalSpace int64  `json:"totalSpace"`
	FreeSpace  int64  `json:"freeSpace"`
}

func (r *RadarrClient) GetDiskSpace() ([]DiskSpace, error) {
	body, err := r.doRequest("/api/v3/diskspace")
	if err != nil {
		return nil, err
	}

	var result []radarrDiskSpace
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse diskspace: %w", err)
	}

	disks := make([]DiskSpace, len(result))
	for i, d := range result {
		disks[i] = DiskSpace{
			Path:       d.Path,
			TotalBytes: d.TotalSpace,
			FreeBytes:  d.FreeSpace,
		}
	}
	return disks, nil
}

// radarrRootFolder maps the root folder API response
type radarrRootFolder struct {
	Path string `json:"path"`
}

func (r *RadarrClient) GetRootFolders() ([]string, error) {
	body, err := r.doRequest("/api/v3/rootfolder")
	if err != nil {
		return nil, err
	}

	var folders []radarrRootFolder
	if err := json.Unmarshal(body, &folders); err != nil {
		return nil, fmt.Errorf("failed to parse root folders: %w", err)
	}

	paths := make([]string, len(folders))
	for i, f := range folders {
		paths[i] = f.Path
	}
	return paths, nil
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
	Genres           []string `json:"genres"`
	Tags             []int    `json:"tags"`
	QualityProfileID int      `json:"qualityProfileId"`
	Added            string   `json:"added"`
}

// radarrQualityProfile maps quality profile names
type radarrQualityProfile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// radarrTag maps tag names
type radarrTag struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

func (r *RadarrClient) GetMediaItems() ([]MediaItem, error) {
	// Fetch quality profiles for name lookup
	profileBody, err := r.doRequest("/api/v3/qualityprofile")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quality profiles: %w", err)
	}
	var profiles []radarrQualityProfile
	if err := json.Unmarshal(profileBody, &profiles); err != nil {
		return nil, fmt.Errorf("failed to parse quality profiles: %w", err)
	}
	profileMap := make(map[int]string)
	for _, p := range profiles {
		profileMap[p.ID] = p.Name
	}

	// Fetch tags for name lookup
	tagBody, err := r.doRequest("/api/v3/tag")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}
	var tags []radarrTag
	if err := json.Unmarshal(tagBody, &tags); err != nil {
		return nil, fmt.Errorf("failed to parse tags: %w", err)
	}
	tagMap := make(map[int]string)
	for _, t := range tags {
		tagMap[t.ID] = t.Label
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

		// Map tag IDs to names
		tagNames := make([]string, 0, len(m.Tags))
		for _, tid := range m.Tags {
			if name, ok := tagMap[tid]; ok {
				tagNames = append(tagNames, name)
			}
		}

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

func (r *RadarrClient) GetQualityProfiles() ([]NameValue, error) {
	body, err := r.doRequest("/api/v3/qualityprofile")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quality profiles: %w", err)
	}
	var profiles []radarrQualityProfile
	if err := json.Unmarshal(body, &profiles); err != nil {
		return nil, fmt.Errorf("failed to parse quality profiles: %w", err)
	}
	result := make([]NameValue, len(profiles))
	for i, p := range profiles {
		result[i] = NameValue{Value: p.Name, Label: p.Name}
	}
	return result, nil
}

func (r *RadarrClient) GetTags() ([]NameValue, error) {
	body, err := r.doRequest("/api/v3/tag")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}
	var tags []radarrTag
	if err := json.Unmarshal(body, &tags); err != nil {
		return nil, fmt.Errorf("failed to parse tags: %w", err)
	}
	result := make([]NameValue, len(tags))
	for i, t := range tags {
		result[i] = NameValue{Value: t.Label, Label: t.Label}
	}
	return result, nil
}

func (r *RadarrClient) GetLanguages() ([]NameValue, error) {
	body, err := r.doRequest("/api/v3/language")
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

func (r *RadarrClient) DeleteMediaItem(item MediaItem) error {
	endpoint := fmt.Sprintf("/api/v3/movie/%s?deleteFiles=true", item.ExternalID)
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
