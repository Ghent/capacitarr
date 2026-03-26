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
// the dashboard stats endpoint. On 401, returns a descriptive error about the
// API key format requirement.
func (t *TracearrClient) TestConnection() error {
	body, err := t.doRequest("/api/stats/dashboard")
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") {
			return fmt.Errorf("tracearr auth failed (check your API key — generate a Public API key in Tracearr Settings, it must start with trr_pub_)")
		}
		return err
	}

	// Verify we got a valid JSON response
	var result map[string]json.RawMessage
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse Tracearr response: %w", err)
	}

	return nil
}

// TracearrTopContent holds the top movies and shows from Tracearr's
// top-content endpoint.
type TracearrTopContent struct {
	Movies []TracearrContentItem `json:"movies"`
	Shows  []TracearrContentItem `json:"shows"`
}

// TracearrContentItem represents a single movie or show from Tracearr's
// top-content response. Movies use MediaTitle; shows use GrandparentTitle.
type TracearrContentItem struct {
	MediaTitle       string `json:"media_title"`
	GrandparentTitle string `json:"grandparent_title"` // Shows use this instead of media_title
	Year             int    `json:"year"`
	PlayCount        int    `json:"play_count"`
	TotalWatchMs     int64  `json:"total_watch_ms"`
	ServerID         string `json:"server_id"`
	RatingKey        string `json:"rating_key"`
}

// GetTopContent fetches the top movies and shows from Tracearr for the given
// period. Use "all" for all-time data.
func (t *TracearrClient) GetTopContent(period string) (*TracearrTopContent, error) {
	body, err := t.doRequest("/api/stats/content/top-content?period=" + period)
	if err != nil {
		return nil, fmt.Errorf("tracearr top content: %w", err)
	}

	var content TracearrTopContent
	if err := json.Unmarshal(body, &content); err != nil {
		return nil, fmt.Errorf("failed to parse Tracearr top content: %w", err)
	}

	return &content, nil
}

// Verify TracearrClient satisfies capability interfaces at compile time.
// Note: Tracearr uses the TracearrEnricher with a unified rating key → TMDb ID
// map, not the WatchDataProvider interface directly.
var _ Connectable = (*TracearrClient)(nil)
