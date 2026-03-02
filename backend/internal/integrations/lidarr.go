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

// LidarrClient implements Integration for Lidarr v1 API (Servarr framework).
// Lidarr manages music libraries and follows the same API patterns as Sonarr/Radarr.
type LidarrClient struct {
	URL    string
	APIKey string
}

func NewLidarrClient(url, apiKey string) *LidarrClient {
	return &LidarrClient{
		URL:    strings.TrimRight(url, "/"),
		APIKey: apiKey,
	}
}

func (l *LidarrClient) doRequest(endpoint string) ([]byte, error) {
	return DoAPIRequest(l.URL+endpoint, "X-Api-Key", l.APIKey)
}

func (l *LidarrClient) TestConnection() error {
	_, err := l.doRequest("/api/v1/system/status")
	return err
}

// lidarrDiskSpace maps the Lidarr diskspace API response
type lidarrDiskSpace struct {
	Path       string `json:"path"`
	TotalSpace int64  `json:"totalSpace"`
	FreeSpace  int64  `json:"freeSpace"`
}

func (l *LidarrClient) GetDiskSpace() ([]DiskSpace, error) {
	body, err := l.doRequest("/api/v1/diskspace")
	if err != nil {
		return nil, err
	}

	var result []lidarrDiskSpace
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

// lidarrRootFolder maps the root folder API response
type lidarrRootFolder struct {
	Path string `json:"path"`
}

func (l *LidarrClient) GetRootFolders() ([]string, error) {
	body, err := l.doRequest("/api/v1/rootfolder")
	if err != nil {
		return nil, err
	}

	var folders []lidarrRootFolder
	if err := json.Unmarshal(body, &folders); err != nil {
		return nil, fmt.Errorf("failed to parse root folders: %w", err)
	}

	paths := make([]string, len(folders))
	for i, f := range folders {
		paths[i] = f.Path
	}
	return paths, nil
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
	Genres           []string `json:"genres"`
	Tags             []int    `json:"tags"`
	QualityProfileID int      `json:"qualityProfileId"`
	Added            string   `json:"added"`
	Statistics       struct {
		SizeOnDisk int64 `json:"sizeOnDisk"`
		AlbumCount int   `json:"albumCount"`
		TrackCount int   `json:"trackCount"`
	} `json:"statistics"`
}

// lidarrQualityProfile maps quality profile names
type lidarrQualityProfile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// lidarrTag maps tag names
type lidarrTag struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

func (l *LidarrClient) GetMediaItems() ([]MediaItem, error) {
	// Fetch quality profiles for name lookup
	profileBody, err := l.doRequest("/api/v1/qualityprofile")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quality profiles: %w", err)
	}
	var profiles []lidarrQualityProfile
	if err := json.Unmarshal(profileBody, &profiles); err != nil {
		return nil, fmt.Errorf("failed to parse quality profiles: %w", err)
	}
	profileMap := make(map[int]string)
	for _, p := range profiles {
		profileMap[p.ID] = p.Name
	}

	// Fetch tags for name lookup
	tagBody, err := l.doRequest("/api/v1/tag")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}
	var tags []lidarrTag
	if err := json.Unmarshal(tagBody, &tags); err != nil {
		return nil, fmt.Errorf("failed to parse tags: %w", err)
	}
	tagMap := make(map[int]string)
	for _, t := range tags {
		tagMap[t.ID] = t.Label
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

		// Lidarr ratings are 0-10 scale, normalize to 0-1
		rating := a.Ratings.Value / 10.0

		// Map tag IDs to names
		tagNames := make([]string, 0, len(a.Tags))
		for _, tid := range a.Tags {
			if name, ok := tagMap[tid]; ok {
				tagNames = append(tagNames, name)
			}
		}

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

func (l *LidarrClient) GetQualityProfiles() ([]NameValue, error) {
	body, err := l.doRequest("/api/v1/qualityprofile")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quality profiles: %w", err)
	}
	var profiles []lidarrQualityProfile
	if err := json.Unmarshal(body, &profiles); err != nil {
		return nil, fmt.Errorf("failed to parse quality profiles: %w", err)
	}
	result := make([]NameValue, len(profiles))
	for i, p := range profiles {
		result[i] = NameValue{Value: p.Name, Label: p.Name}
	}
	return result, nil
}

func (l *LidarrClient) GetTags() ([]NameValue, error) {
	body, err := l.doRequest("/api/v1/tag")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}
	var tags []lidarrTag
	if err := json.Unmarshal(body, &tags); err != nil {
		return nil, fmt.Errorf("failed to parse tags: %w", err)
	}
	result := make([]NameValue, len(tags))
	for i, t := range tags {
		result[i] = NameValue{Value: t.Label, Label: t.Label}
	}
	return result, nil
}

func (l *LidarrClient) GetLanguages() ([]NameValue, error) {
	// Lidarr does not have a language endpoint
	return nil, nil
}

func (l *LidarrClient) DeleteMediaItem(item MediaItem) error {
	endpoint := fmt.Sprintf("/api/v1/artist/%s?deleteFiles=true", item.ExternalID)
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", l.URL+endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", l.APIKey)

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
