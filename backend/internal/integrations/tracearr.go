package integrations

import (
	"encoding/json"
	"fmt"
	"strings"
)

// TracearrClient provides access to the Tracearr Public API for unified
// media server analytics. Tracearr is a self-hosted monitoring platform that
// supports Plex, Jellyfin, and Emby from a single instance — effectively
// replacing both Tautulli (Plex-only) and Jellystat (Jellyfin-only).
//
// Authentication uses Bearer tokens: `Authorization: Bearer trr_pub_<token>`.
// Tokens are generated in Tracearr Settings and require the `owner` role.
//
// The Public API lives under /api/v1/public/ (distinct from the internal
// session-authenticated API at /api/v1/stats/). All Capacitarr calls use
// the public API exclusively.
type TracearrClient struct {
	URL    string
	APIKey string `json:"-"` // Tracearr Public API key (must start with trr_pub_)
}

// NewTracearrClient creates a new Tracearr API client.
func NewTracearrClient(url, apiKey string) *TracearrClient {
	return &TracearrClient{
		URL:    strings.TrimRight(url, "/"),
		APIKey: apiKey,
	}
}

// doRequest executes a Tracearr API call using Bearer token authentication.
func (t *TracearrClient) doRequest(endpoint string) ([]byte, error) {
	fullURL := t.URL + endpoint
	return DoAPIRequest(fullURL, "Authorization", "Bearer "+t.APIKey)
}

// TestConnection verifies the Tracearr URL and API key are valid by calling
// the public health endpoint. On 401, returns a descriptive error about the
// API key format requirement.
func (t *TracearrClient) TestConnection() error {
	body, err := t.doRequest("/api/v1/public/health")
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") {
			return fmt.Errorf("tracearr auth failed (check your API key — generate a Public API key in Tracearr Settings, it must start with trr_pub_)")
		}
		return err
	}

	// Verify we got a valid JSON response with status field
	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse Tracearr response: %w", err)
	}

	return nil
}

// tracearrHistoryResponse represents the paginated response from the
// Tracearr Public API /history endpoint.
type tracearrHistoryResponse struct {
	Data       []TracearrHistoryItem `json:"data"`
	Pagination struct {
		Page     int `json:"page"`
		PageSize int `json:"pageSize"`
		Total    int `json:"total"`
	} `json:"pagination"`
}

// TracearrHistoryItem represents a single session from Tracearr's
// public /history endpoint. Each item is a play session — multiple
// sessions for the same media title must be aggregated by the enricher.
type TracearrHistoryItem struct {
	MediaTitle string `json:"mediaTitle"`
	ShowTitle  string `json:"showTitle"`  // For episodes: the series name (grandparent_title)
	MediaType  string `json:"mediaType"`  // "movie", "episode", "track", "live"
	Year       int    `json:"year"`       // Release year
	Watched    bool   `json:"watched"`    // Whether session completed
	DurationMs int64  `json:"durationMs"` // Watch duration in ms
	User       struct {
		Username string `json:"username"`
	} `json:"user"`
}

// GetWatchHistory fetches session history from the Tracearr Public API,
// paginating through all results. Returns the raw session list — aggregation
// into per-title play counts is done by the TracearrEnricher.
func (t *TracearrClient) GetWatchHistory() ([]TracearrHistoryItem, error) {
	var allItems []TracearrHistoryItem
	page := 1
	pageSize := 100

	for {
		endpoint := fmt.Sprintf("/api/v1/public/history?page=%d&pageSize=%d&mediaType=movie", page, pageSize)
		body, err := t.doRequest(endpoint)
		if err != nil {
			return nil, fmt.Errorf("tracearr history (movies page %d): %w", page, err)
		}

		var resp tracearrHistoryResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("failed to parse Tracearr history: %w", err)
		}

		allItems = append(allItems, resp.Data...)

		if page*pageSize >= resp.Pagination.Total || len(resp.Data) == 0 {
			break
		}
		page++
	}

	// Also fetch episode history (for show-level aggregation)
	page = 1
	for {
		endpoint := fmt.Sprintf("/api/v1/public/history?page=%d&pageSize=%d&mediaType=episode", page, pageSize)
		body, err := t.doRequest(endpoint)
		if err != nil {
			return nil, fmt.Errorf("tracearr history (episodes page %d): %w", page, err)
		}

		var resp tracearrHistoryResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, fmt.Errorf("failed to parse Tracearr episode history: %w", err)
		}

		allItems = append(allItems, resp.Data...)

		if page*pageSize >= resp.Pagination.Total || len(resp.Data) == 0 {
			break
		}
		page++
	}

	return allItems, nil
}

// Verify TracearrClient satisfies capability interfaces at compile time.
// Note: Tracearr uses the TracearrEnricher with a unified rating key → TMDb ID
// map, not the WatchDataProvider interface directly.
var _ Connectable = (*TracearrClient)(nil)
