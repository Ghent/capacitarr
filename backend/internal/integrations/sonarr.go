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

// SonarrClient implements Integration for Sonarr v3 API
type SonarrClient struct {
	URL    string
	APIKey string
}

func NewSonarrClient(url, apiKey string) *SonarrClient {
	return &SonarrClient{
		URL:    strings.TrimRight(url, "/"),
		APIKey: apiKey,
	}
}

func (s *SonarrClient) doRequest(endpoint string) ([]byte, error) {
	return DoAPIRequest(s.URL+endpoint, "X-Api-Key", s.APIKey)
}

func (s *SonarrClient) TestConnection() error {
	_, err := s.doRequest("/api/v3/system/status")
	return err
}

// sonarrDiskSpace maps the Sonarr diskspace API response
type sonarrDiskSpace struct {
	Path       string `json:"path"`
	TotalSpace int64  `json:"totalSpace"`
	FreeSpace  int64  `json:"freeSpace"`
}

func (s *SonarrClient) GetDiskSpace() ([]DiskSpace, error) {
	body, err := s.doRequest("/api/v3/diskspace")
	if err != nil {
		return nil, err
	}

	var result []sonarrDiskSpace
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

// sonarrRootFolder maps the root folder API response
type sonarrRootFolder struct {
	Path string `json:"path"`
}

func (s *SonarrClient) GetRootFolders() ([]string, error) {
	body, err := s.doRequest("/api/v3/rootfolder")
	if err != nil {
		return nil, err
	}

	var folders []sonarrRootFolder
	if err := json.Unmarshal(body, &folders); err != nil {
		return nil, fmt.Errorf("failed to parse root folders: %w", err)
	}

	paths := make([]string, len(folders))
	for i, f := range folders {
		paths[i] = f.Path
	}
	return paths, nil
}

// sonarrSeries maps the Sonarr series API response
type sonarrSeries struct {
	ID               int      `json:"id"`
	Title            string   `json:"title"`
	Year             int      `json:"year"`
	Path             string   `json:"path"`
	Monitored        bool     `json:"monitored"`
	Status           string   `json:"status"` // continuing, ended
	Genres           []string `json:"genres"`
	Tags             []int    `json:"tags"`
	QualityProfileID int      `json:"qualityProfileId"`
	Added            string   `json:"added"`
	Ratings          struct {
		Value float64 `json:"value"`
	} `json:"ratings"`
	Statistics struct {
		SizeOnDisk   int64 `json:"sizeOnDisk"`
		SeasonCount  int   `json:"seasonCount"`
		EpisodeCount int   `json:"episodeCount"`
	} `json:"statistics"`
	Seasons []sonarrSeason `json:"seasons"`
}

type sonarrSeason struct {
	SeasonNumber int  `json:"seasonNumber"`
	Monitored    bool `json:"monitored"`
	Statistics   struct {
		SizeOnDisk        int64 `json:"sizeOnDisk"`
		EpisodeFileCount  int   `json:"episodeFileCount"`
		TotalEpisodeCount int   `json:"totalEpisodeCount"`
	} `json:"statistics"`
}

// sonarrQualityProfile maps quality profile names
type sonarrQualityProfile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// sonarrTag maps tag names
type sonarrTag struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

func (s *SonarrClient) GetMediaItems() ([]MediaItem, error) {
	// Fetch quality profiles
	profileBody, err := s.doRequest("/api/v3/qualityprofile")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quality profiles: %w", err)
	}
	var profiles []sonarrQualityProfile
	if err := json.Unmarshal(profileBody, &profiles); err != nil {
		return nil, fmt.Errorf("failed to parse quality profiles: %w", err)
	}
	profileMap := make(map[int]string)
	for _, p := range profiles {
		profileMap[p.ID] = p.Name
	}

	// Fetch tags
	tagBody, err := s.doRequest("/api/v3/tag")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}
	var tags []sonarrTag
	if err := json.Unmarshal(tagBody, &tags); err != nil {
		return nil, fmt.Errorf("failed to parse tags: %w", err)
	}
	tagMap := make(map[int]string)
	for _, t := range tags {
		tagMap[t.ID] = t.Label
	}

	// Fetch all series
	body, err := s.doRequest("/api/v3/series")
	if err != nil {
		return nil, err
	}

	var seriesList []sonarrSeries
	if err := json.Unmarshal(body, &seriesList); err != nil {
		return nil, fmt.Errorf("failed to parse series: %w", err)
	}

	items := make([]MediaItem, 0, len(seriesList)*2)
	for _, show := range seriesList {
		if show.Statistics.SizeOnDisk == 0 {
			continue
		}

		tagNames := make([]string, 0, len(show.Tags))
		for _, tid := range show.Tags {
			if name, ok := tagMap[tid]; ok {
				tagNames = append(tagNames, name)
			}
		}

		var addedAt *time.Time
		if show.Added != "" {
			if t, err := time.Parse(time.RFC3339, show.Added); err == nil {
				addedAt = &t
			}
		}

		// Emit each season as a separate scoreable item
		for _, season := range show.Seasons {
			if season.SeasonNumber == 0 || season.Statistics.SizeOnDisk == 0 {
				continue // Skip specials and empty seasons
			}

			items = append(items, MediaItem{
				ExternalID:     fmt.Sprintf("%d-s%d", show.ID, season.SeasonNumber),
				Type:           MediaTypeSeason,
				Title:          fmt.Sprintf("%s - Season %d", show.Title, season.SeasonNumber),
				ShowTitle:      show.Title,
				Year:           show.Year,
				SeasonNumber:   season.SeasonNumber,
				EpisodeCount:   season.Statistics.EpisodeFileCount,
				SizeBytes:      season.Statistics.SizeOnDisk,
				Path:           show.Path,
				ShowStatus:     show.Status,
				QualityProfile: profileMap[show.QualityProfileID],
				Rating:         show.Ratings.Value,
				Genre:          strings.Join(show.Genres, ", "),
				Monitored:      show.Monitored && season.Monitored,
				Tags:           tagNames,
				AddedAt:        addedAt,
			})
		}

		// Also emit the show-level item for "all or nothing" strategy
		items = append(items, MediaItem{
			ExternalID:     strconv.Itoa(show.ID),
			Type:           MediaTypeShow,
			Title:          show.Title,
			Year:           show.Year,
			SizeBytes:      show.Statistics.SizeOnDisk,
			Path:           show.Path,
			ShowStatus:     show.Status,
			EpisodeCount:   show.Statistics.EpisodeCount,
			QualityProfile: profileMap[show.QualityProfileID],
			Rating:         show.Ratings.Value,
			Genre:          strings.Join(show.Genres, ", "),
			Monitored:      show.Monitored,
			Tags:           tagNames,
			AddedAt:        addedAt,
		})
	}

	return items, nil
}

// --- RuleValueFetcher implementation ---

func (s *SonarrClient) GetQualityProfiles() ([]NameValue, error) {
	body, err := s.doRequest("/api/v3/qualityprofile")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quality profiles: %w", err)
	}
	var profiles []sonarrQualityProfile
	if err := json.Unmarshal(body, &profiles); err != nil {
		return nil, fmt.Errorf("failed to parse quality profiles: %w", err)
	}
	result := make([]NameValue, len(profiles))
	for i, p := range profiles {
		result[i] = NameValue{Value: p.Name, Label: p.Name}
	}
	return result, nil
}

func (s *SonarrClient) GetTags() ([]NameValue, error) {
	body, err := s.doRequest("/api/v3/tag")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}
	var tags []sonarrTag
	if err := json.Unmarshal(body, &tags); err != nil {
		return nil, fmt.Errorf("failed to parse tags: %w", err)
	}
	result := make([]NameValue, len(tags))
	for i, t := range tags {
		result[i] = NameValue{Value: t.Label, Label: t.Label}
	}
	return result, nil
}

func (s *SonarrClient) GetLanguages() ([]NameValue, error) {
	body, err := s.doRequest("/api/v3/language")
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

func (s *SonarrClient) DeleteMediaItem(item MediaItem) error {
	var endpoint string
	if item.Type == MediaTypeShow { //nolint:gocritic // conditions test different fields, not a single value
		// Delete the entire series and its files
		endpoint = fmt.Sprintf("/api/v3/series/%s?deleteFiles=true", item.ExternalID)
	} else if item.Type == MediaTypeSeason {
		// ExternalID for season is formatted as "seriesId-seasonNum" (e.g., "12-s1")
		parts := strings.Split(item.ExternalID, "-s")
		if len(parts) != 2 {
			return fmt.Errorf("invalid season external ID format: %s", item.ExternalID)
		}

		seriesIDStr := parts[0]
		seasonNumStr := parts[1]

		// To delete a season, we fetch all episode files for the season...
		filesBody, err := s.doRequest(fmt.Sprintf("/api/v3/episodefile?seriesId=%s&seasonNumber=%s", seriesIDStr, seasonNumStr))
		if err != nil {
			return fmt.Errorf("failed to fetch episode files for season: %w", err)
		}

		var files []struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal(filesBody, &files); err != nil {
			return fmt.Errorf("failed to parse episode files: %w", err)
		}

		// ...and delete them in bulk
		fileIDs := make([]int, len(files))
		for i, f := range files {
			fileIDs[i] = f.ID
		}

		if len(fileIDs) == 0 {
			return nil // Nothing to delete
		}

		payload, _ := json.Marshal(map[string]interface{}{
			"episodeFileIds": fileIDs,
		})

		req, err := http.NewRequestWithContext(context.Background(), "DELETE", s.URL+"/api/v3/episodefile/bulk", strings.NewReader(string(payload)))
		if err != nil {
			return err
		}
		req.Header.Set("X-Api-Key", s.APIKey)
		req.Header.Set("Content-Type", "application/json")

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
	} else {
		return fmt.Errorf("unsupported media type for sonarr deletion: %s", item.Type)
	}

	req, err := http.NewRequestWithContext(context.Background(), "DELETE", s.URL+endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", s.APIKey)

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
