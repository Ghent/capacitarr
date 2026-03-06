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

// NewSonarrClient creates a new Sonarr TV series management API client.
func NewSonarrClient(url, apiKey string) *SonarrClient {
	return &SonarrClient{
		URL:    strings.TrimRight(url, "/"),
		APIKey: apiKey,
	}
}

func (s *SonarrClient) doRequest(endpoint string) ([]byte, error) {
	return DoAPIRequest(s.URL+endpoint, "X-Api-Key", s.APIKey)
}

// TestConnection verifies the Sonarr server is reachable and the API key is valid.
func (s *SonarrClient) TestConnection() error {
	_, err := s.doRequest("/api/v3/system/status")
	return err
}

// GetDiskSpace returns disk usage information reported by Sonarr.
func (s *SonarrClient) GetDiskSpace() ([]DiskSpace, error) {
	return arrFetchDiskSpace(s.doRequest, "/api/v3")
}

// GetRootFolders returns the configured root folder paths from Sonarr.
func (s *SonarrClient) GetRootFolders() ([]string, error) {
	return arrFetchRootFolders(s.doRequest, "/api/v3")
}

// sonarrSeries maps the Sonarr series API response
type sonarrSeries struct {
	ID               int        `json:"id"`
	Title            string     `json:"title"`
	Year             int        `json:"year"`
	Path             string     `json:"path"`
	Monitored        bool       `json:"monitored"`
	Status           string     `json:"status"` // continuing, ended
	Genres           []string   `json:"genres"`
	Tags             []int      `json:"tags"`
	QualityProfileID int        `json:"qualityProfileId"`
	Added            string     `json:"added"`
	Images           []arrImage `json:"images"`
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

// GetMediaItems fetches all series and seasons from Sonarr with quality and tag metadata.
func (s *SonarrClient) GetMediaItems() ([]MediaItem, error) {
	// Fetch quality profiles
	profileMap, err := arrFetchQualityProfileMap(s.doRequest, "/api/v3")
	if err != nil {
		return nil, err
	}

	// Fetch tags
	tagMap, err := arrFetchTagMap(s.doRequest, "/api/v3")
	if err != nil {
		return nil, err
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

		tagNames := arrResolveTagNames(show.Tags, tagMap)

		var addedAt *time.Time
		if show.Added != "" {
			if t, err := time.Parse(time.RFC3339, show.Added); err == nil {
				addedAt = &t
			}
		}

		posterURL := arrExtractPosterURL(show.Images)

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
				PosterURL:      posterURL,
				SeriesStatus:   show.Status,
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
			PosterURL:      posterURL,
			SeriesStatus:   show.Status,
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

// GetQualityProfiles returns available quality profiles from Sonarr.
func (s *SonarrClient) GetQualityProfiles() ([]NameValue, error) {
	return arrFetchQualityProfiles(s.doRequest, "/api/v3")
}

// GetTags returns all tags configured in Sonarr.
func (s *SonarrClient) GetTags() ([]NameValue, error) {
	return arrFetchTags(s.doRequest, "/api/v3")
}

// GetLanguages returns all languages configured in Sonarr.
func (s *SonarrClient) GetLanguages() ([]NameValue, error) {
	return arrFetchLanguages(s.doRequest, "/api/v3")
}

// DeleteMediaItem removes a series or season and its files from disk via the Sonarr API.
func (s *SonarrClient) DeleteMediaItem(item MediaItem) error {
	var endpoint string
	switch item.Type { //nolint:exhaustive // Sonarr only handles shows and seasons
	case MediaTypeShow:
		// Delete the entire series and its files
		endpoint = fmt.Sprintf("/api/v3/series/%s?deleteFiles=true", item.ExternalID)
	case MediaTypeSeason:
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

		resp, err := sharedHTTPClient.Do(req) //nolint:gosec // G704: URL is from admin-configured integration settings
		if err != nil {
			return fmt.Errorf("connection failed: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == 401 {
			return fmt.Errorf("unauthorized: invalid API key")
		}
		if resp.StatusCode != 200 {
			return fmt.Errorf("unexpected status: %d", resp.StatusCode)
		}

		return nil
	default:
		return fmt.Errorf("unsupported media type for sonarr deletion: %s", item.Type)
	}

	// Show-level deletion uses arrSimpleDelete
	return arrSimpleDelete(s.URL, s.APIKey, endpoint)
}
