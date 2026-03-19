package integrations

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// TautulliClient provides access to the Tautulli API for enriched watch history.
// Tautulli supplements Plex's binary watched/unwatched signal with detailed
// play counts, last played timestamps, watch durations, and per-user history.
type TautulliClient struct {
	URL    string
	APIKey string `json:"-"`
}

// NewTautulliClient creates a new Tautulli API client.
func NewTautulliClient(url, apiKey string) *TautulliClient {
	return &TautulliClient{
		URL:    strings.TrimRight(url, "/"),
		APIKey: apiKey,
	}
}

// TautulliWatchData contains enriched watch history from Tautulli.
type TautulliWatchData struct {
	PlayCount     int        `json:"playCount"`
	LastPlayed    *time.Time `json:"lastPlayed,omitempty"`
	TotalDuration int64      `json:"totalDuration"` // total seconds watched
	Users         []string   `json:"users"`         // which users watched
}

// doRequest executes a Tautulli API call using the ?apikey=XXX&cmd=CMD pattern.
func (t *TautulliClient) doRequest(cmd string, extraParams string) ([]byte, error) {
	fullURL := fmt.Sprintf("%s/api/v2?apikey=%s&cmd=%s", t.URL, t.APIKey, cmd)
	if extraParams != "" {
		fullURL += "&" + extraParams
	}
	return DoAPIRequest(fullURL, "", "")
}

// tautulliResponse wraps the standard Tautulli API response envelope.
type tautulliResponse struct {
	Response struct {
		Result  string          `json:"result"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	} `json:"response"`
}

// tautulliHistoryData maps the get_history response data.
type tautulliHistoryData struct {
	RecordsFiltered int                    `json:"recordsFiltered"`
	RecordsTotal    int                    `json:"recordsTotal"`
	Data            []tautulliHistoryEntry `json:"data"`
}

// tautulliHistoryEntry represents one play record from Tautulli history.
type tautulliHistoryEntry struct {
	Date                 int64  `json:"date"`           // Unix epoch of play start
	Duration             int64  `json:"duration"`       // Duration of item (seconds)
	PlayDuration         int64  `json:"play_duration"`  // Actual time played (seconds)
	PausedCounter        int64  `json:"paused_counter"` // Time spent paused (seconds)
	WatchedStatus        int    `json:"watched_status"` // 0=unwatched, 0.5=partial, 1=watched
	User                 string `json:"user"`           // Username
	RatingKey            string `json:"rating_key"`     // Plex rating key
	ParentRatingKey      string `json:"parent_rating_key"`
	GrandparentRatingKey string `json:"grandparent_rating_key"`
	Title                string `json:"title"`
	MediaType            string `json:"media_type"` // movie, episode, track
}

// TestConnection verifies the Tautulli URL and API key are valid
// by calling the get_tautulli_info command.
func (t *TautulliClient) TestConnection() error {
	body, err := t.doRequest("get_tautulli_info", "")
	if err != nil {
		return err
	}

	var resp tautulliResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse Tautulli response: %w", err)
	}

	if resp.Response.Result != "success" {
		return fmt.Errorf("tautulli error: %s", resp.Response.Message)
	}

	return nil
}

// GetWatchHistory fetches aggregated watch data for a specific Plex rating key.
// It queries Tautulli's get_history endpoint filtered by the rating key, then
// aggregates play count, last played time, total duration, and unique users.
func (t *TautulliClient) GetWatchHistory(ratingKey string) (*TautulliWatchData, error) {
	params := fmt.Sprintf("rating_key=%s&length=100", ratingKey)
	body, err := t.doRequest("get_history", params)
	if err != nil {
		return nil, err
	}

	var resp tautulliResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse Tautulli response: %w", err)
	}

	if resp.Response.Result != "success" {
		return nil, fmt.Errorf("tautulli error: %s", resp.Response.Message)
	}

	var history tautulliHistoryData
	if err := json.Unmarshal(resp.Response.Data, &history); err != nil {
		return nil, fmt.Errorf("failed to parse history data: %w", err)
	}

	data := &TautulliWatchData{}
	data.PlayCount = len(history.Data)

	userSet := make(map[string]bool)
	var latestPlay int64

	for _, entry := range history.Data {
		// Track total actual watch duration
		data.TotalDuration += entry.PlayDuration

		// Track unique users
		if entry.User != "" {
			userSet[entry.User] = true
		}

		// Track most recent play
		if entry.Date > latestPlay {
			latestPlay = entry.Date
		}
	}

	if latestPlay > 0 {
		t := time.Unix(latestPlay, 0)
		data.LastPlayed = &t
	}

	for user := range userSet {
		data.Users = append(data.Users, user)
	}

	return data, nil
}

// GetShowWatchHistory fetches watch data for a show by querying with grandparent_rating_key.
// This aggregates across all episodes of the show for a holistic view.
func (t *TautulliClient) GetShowWatchHistory(ratingKey string) (*TautulliWatchData, error) {
	params := fmt.Sprintf("grandparent_rating_key=%s&length=500", ratingKey)
	body, err := t.doRequest("get_history", params)
	if err != nil {
		return nil, err
	}

	var resp tautulliResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse Tautulli response: %w", err)
	}

	if resp.Response.Result != "success" {
		return nil, fmt.Errorf("tautulli error: %s", resp.Response.Message)
	}

	var history tautulliHistoryData
	if err := json.Unmarshal(resp.Response.Data, &history); err != nil {
		return nil, fmt.Errorf("failed to parse history data: %w", err)
	}

	data := &TautulliWatchData{}
	data.PlayCount = len(history.Data)

	userSet := make(map[string]bool)
	var latestPlay int64

	for _, entry := range history.Data {
		data.TotalDuration += entry.PlayDuration

		if entry.User != "" {
			userSet[entry.User] = true
		}

		if entry.Date > latestPlay {
			latestPlay = entry.Date
		}
	}

	if latestPlay > 0 {
		t := time.Unix(latestPlay, 0)
		data.LastPlayed = &t
	}

	for user := range userSet {
		data.Users = append(data.Users, user)
	}

	return data, nil
}

// Verify TautulliClient satisfies capability interfaces at compile time.
// Note: Tautulli uses per-item watch history queries rather than a bulk fetch,
// so it does not implement WatchDataProvider. The TautulliEnricher handles
// the per-item enrichment pattern directly.
var _ Connectable = (*TautulliClient)(nil)
