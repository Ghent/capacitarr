package integrations

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SonarrClient implements Connectable, MediaSource, DiskReporter, MediaDeleter, and RuleValueFetcher for Sonarr v3 API.
// Shared *arr methods (TestConnection, GetDiskSpace, GetRootFolders, GetQualityProfiles, GetTags, GetLanguages)
// are provided by the embedded arrBaseClient.
type SonarrClient struct {
	arrBaseClient
}

// NewSonarrClient creates a new Sonarr TV series management API client.
func NewSonarrClient(url, apiKey string) *SonarrClient {
	return &SonarrClient{
		arrBaseClient: newArrBaseClient(url, apiKey, "/api/v3"),
	}
}

// sonarrSeries maps the Sonarr series API response
type sonarrSeries struct {
	ID               int         `json:"id"`
	Title            string      `json:"title"`
	Year             int         `json:"year"`
	TmdbID           int         `json:"tmdbId"`
	Path             string      `json:"path"`
	Monitored        bool        `json:"monitored"`
	Status           string      `json:"status"` // continuing, ended
	Genres           []string    `json:"genres"`
	Tags             []int       `json:"tags"`
	QualityProfileID int         `json:"qualityProfileId"`
	Added            string      `json:"added"`
	Images           []arrImage  `json:"images"`
	OriginalLanguage arrLanguage `json:"originalLanguage"`
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

// sonarrEpisodeFile maps the relevant fields from Sonarr's /api/v3/episodefile endpoint.
// The dateAdded field represents when the episode file was actually imported/downloaded.
type sonarrEpisodeFile struct {
	ID           int    `json:"id"`
	SeriesID     int    `json:"seriesId"`
	SeasonNumber int    `json:"seasonNumber"`
	DateAdded    string `json:"dateAdded"`
}

// sonarrSeasonDateIndex maps seriesID → seasonNumber → latest episodefile.dateAdded.
type sonarrSeasonDateIndex map[int]map[int]time.Time

func sonarrRecordFileDate(dst map[int]time.Time, f sonarrEpisodeFile) {
	if f.DateAdded == "" {
		return
	}
	t, err := time.Parse(time.RFC3339, f.DateAdded)
	if err != nil {
		return
	}
	if existing, ok := dst[f.SeasonNumber]; !ok || t.After(existing) {
		dst[f.SeasonNumber] = t
	}
}

func sonarrIndexEpisodeFiles(files []sonarrEpisodeFile) sonarrSeasonDateIndex {
	index := make(sonarrSeasonDateIndex)
	for _, f := range files {
		seasons := index[f.SeriesID]
		if seasons == nil {
			seasons = make(map[int]time.Time)
			index[f.SeriesID] = seasons
		}
		sonarrRecordFileDate(seasons, f)
	}
	return index
}

func sonarrParseEpisodeFiles(body []byte) ([]sonarrEpisodeFile, error) {
	var files []sonarrEpisodeFile
	if err := json.Unmarshal(body, &files); err != nil {
		return nil, err
	}
	return files, nil
}

// sonarrFetchAllEpisodeFileDates fetches every episode file in one request
// (GET /api/v3/episodefile with no seriesId) and groups max(dateAdded) by
// seriesId + season. Some Sonarr builds reject the unfiltered list (400 unless
// seriesId or episodeFileIds is set); callers must fall back per series.
func sonarrFetchAllEpisodeFileDates(doRequest func(string) ([]byte, error)) (sonarrSeasonDateIndex, error) {
	body, err := doRequest("/api/v3/episodefile")
	if err != nil {
		return nil, err
	}
	files, err := sonarrParseEpisodeFiles(body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse episode files: %w", err)
	}
	return sonarrIndexEpisodeFiles(files), nil
}

// sonarrFetchEpisodeFileDates fetches episode files for a series and returns
// a map of seasonNumber → max(dateAdded) for that season. This gives accurate
// per-season "time in library" dates based on when files were actually imported,
// rather than when the show was added to Sonarr.
func sonarrFetchEpisodeFileDates(doRequest func(string) ([]byte, error), seriesID int) map[int]time.Time {
	endpoint := fmt.Sprintf("/api/v3/episodefile?seriesId=%d", seriesID)
	body, err := doRequest(endpoint)
	if err != nil {
		return nil // Non-fatal: fall back to show-level added date
	}

	files, err := sonarrParseEpisodeFiles(body)
	if err != nil {
		return nil
	}

	seasonDates := make(map[int]time.Time)
	for _, f := range files {
		sonarrRecordFileDate(seasonDates, f)
	}
	return seasonDates
}

// sonarrResolveAddedAt determines the best "added" timestamp for a season or show.
// It prefers the file-level date (from episodefile API) over the show-level added date.
func sonarrResolveAddedAt(seasonDates map[int]time.Time, seasonNumber int, showAdded string) *time.Time {
	// Prefer file-level date for the specific season
	if seasonDates != nil {
		if t, ok := seasonDates[seasonNumber]; ok {
			return &t
		}
	}
	// Fall back to show-level added date
	if showAdded != "" {
		if t, err := time.Parse(time.RFC3339, showAdded); err == nil {
			return &t
		}
	}
	return nil
}

// sonarrResolveShowAddedAt determines the best "added" timestamp for a show-level item.
// It uses the latest file date across all seasons, falling back to show.added.
func sonarrResolveShowAddedAt(seasonDates map[int]time.Time, showAdded string) *time.Time {
	if len(seasonDates) > 0 {
		var latest time.Time
		for _, t := range seasonDates {
			if t.After(latest) {
				latest = t
			}
		}
		return &latest
	}
	if showAdded != "" {
		if t, err := time.Parse(time.RFC3339, showAdded); err == nil {
			return &t
		}
	}
	return nil
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

	// Prefer one GET /api/v3/episodefile for the whole library (O(1) HTTP)
	// instead of one request per series. Fall back to per-series only if the
	// bulk call fails — some Sonarr versions require seriesId.
	var (
		bulkDates    sonarrSeasonDateIndex
		useBulkDates bool
	)
	needsFileDates := false
	for _, show := range seriesList {
		if show.Statistics.SizeOnDisk > 0 {
			needsFileDates = true
			break
		}
	}
	if needsFileDates {
		var bulkErr error
		bulkDates, bulkErr = sonarrFetchAllEpisodeFileDates(s.doRequest)
		if bulkErr != nil {
			slog.Warn("Sonarr bulk episodefile fetch failed; falling back to per-series requests",
				"component", "sonarr", "error", bulkErr)
		} else {
			useBulkDates = true
		}
	}

	for _, show := range seriesList {
		if show.Statistics.SizeOnDisk == 0 {
			continue
		}

		tagNames := arrResolveTagNames(show.Tags, tagMap)

		// Season AddedAt uses episodefile.dateAdded (file import time), not
		// series.added (entry creation time). Falls back to series.added when
		// a season has no file dates.
		var seasonDates map[int]time.Time
		if useBulkDates {
			seasonDates = bulkDates[show.ID]
		} else {
			seasonDates = sonarrFetchEpisodeFileDates(s.doRequest, show.ID)
		}

		posterURL := arrExtractPosterURL(show.Images, s.URL)

		// Emit each season as a separate scoreable item
		for _, season := range show.Seasons {
			if season.SeasonNumber == 0 || season.Statistics.SizeOnDisk == 0 {
				continue // Skip specials and empty seasons
			}

			addedAt := sonarrResolveAddedAt(seasonDates, season.SeasonNumber, show.Added)

			items = append(items, MediaItem{
				ExternalID:     fmt.Sprintf("%d-s%d", show.ID, season.SeasonNumber),
				Type:           MediaTypeSeason,
				Title:          fmt.Sprintf("%s - Season %d", show.Title, season.SeasonNumber),
				ShowTitle:      show.Title,
				Year:           show.Year,
				TMDbID:         show.TmdbID,
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
				Language:       show.OriginalLanguage.Name,
				Tags:           tagNames,
				AddedAt:        addedAt,
			})
		}

		// Also emit the show-level item for "all or nothing" strategy
		showAddedAt := sonarrResolveShowAddedAt(seasonDates, show.Added)

		items = append(items, MediaItem{
			ExternalID:     strconv.Itoa(show.ID),
			Type:           MediaTypeShow,
			Title:          show.Title,
			Year:           show.Year,
			TMDbID:         show.TmdbID,
			SizeBytes:      show.Statistics.SizeOnDisk,
			Path:           show.Path,
			PosterURL:      posterURL,
			SeriesStatus:   show.Status,
			EpisodeCount:   show.Statistics.EpisodeCount,
			QualityProfile: profileMap[show.QualityProfileID],
			Rating:         show.Ratings.Value,
			Genre:          strings.Join(show.Genres, ", "),
			Monitored:      show.Monitored,
			Language:       show.OriginalLanguage.Name,
			Tags:           tagNames,
			AddedAt:        showAddedAt,
		})
	}

	return items, nil
}

// GetQualityProfiles, GetTags, GetLanguages are provided by arrBaseClient.

// DeleteMediaItem removes a series or season and its files from disk via the Sonarr API.
// When opts.AddImportExclusion is true and the item is a show (not a season),
// the series is added to Sonarr's import list exclusion to prevent automatic
// re-addition by import lists. Season-level deletes use the bulk episode file
// endpoint which does not support import exclusion.
func (s *SonarrClient) DeleteMediaItem(item MediaItem, opts DeleteOptions) error {
	var endpoint string
	switch item.Type { //nolint:exhaustive // Sonarr only handles shows and seasons
	case MediaTypeShow:
		// Delete the entire series and its files
		endpoint = fmt.Sprintf("/api/v3/series/%s?deleteFiles=true&addImportListExclusion=%t", item.ExternalID, opts.AddImportExclusion)
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

		payload, _ := json.Marshal(map[string]any{
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
