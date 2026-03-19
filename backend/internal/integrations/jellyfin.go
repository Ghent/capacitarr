package integrations

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// JellyfinClient provides access to the Jellyfin API for watch history data.
// Jellyfin is a free, open-source media server (Plex alternative).
// Its API is used to enrich media items with last-played and play-count
// data for scoring — recently watched content should be protected.
type JellyfinClient struct {
	URL    string
	APIKey string `json:"-"`
}

// NewJellyfinClient creates a new Jellyfin media server API client.
func NewJellyfinClient(url, apiKey string) *JellyfinClient {
	return &JellyfinClient{
		URL:    strings.TrimRight(url, "/"),
		APIKey: apiKey,
	}
}

func (j *JellyfinClient) doRequest(endpoint string) ([]byte, error) {
	fullURL := j.URL + endpoint
	return DoAPIRequest(fullURL, "X-Emby-Token", j.APIKey)
}

// TestConnection verifies the Jellyfin URL and API key by calling /System/Info
func (j *JellyfinClient) TestConnection() error {
	body, err := j.doRequest("/System/Info")
	if err != nil {
		return err
	}
	var info struct {
		ServerName string `json:"ServerName"`
		Version    string `json:"Version"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return fmt.Errorf("failed to parse Jellyfin system info: %w", err)
	}
	if info.ServerName == "" && info.Version == "" {
		return fmt.Errorf("unexpected Jellyfin response — no server name or version")
	}
	return nil
}

// jellyfinItem represents a media item from the Jellyfin API
type jellyfinItem struct {
	ID           string `json:"Id"`
	Name         string `json:"Name"`
	Type         string `json:"Type"` // "Movie", "Series", "Episode", "Audio"
	Path         string `json:"Path"`
	RunTimeTicks int64  `json:"RunTimeTicks"`
	UserData     struct {
		PlayCount      int    `json:"PlayCount"`
		LastPlayedDate string `json:"LastPlayedDate"`
		Played         bool   `json:"Played"`
	} `json:"UserData"`
}

// GetBulkWatchDataForUser fetches all movies and series from Jellyfin's library with their
// watch data (PlayCount, LastPlayedDate) in a single paginated API call.
// Returns a map from normalized (lowercase) title to watch data.
func (j *JellyfinClient) GetBulkWatchDataForUser(userID string) (map[string]*WatchData, error) {
	result := make(map[string]*WatchData)
	startIndex := 0
	pageSize := 500

	for {
		endpoint := fmt.Sprintf(
			"/Users/%s/Items?IncludeItemTypes=Movie,Series&Recursive=true&Fields=UserData&StartIndex=%d&Limit=%d",
			userID, startIndex, pageSize,
		)
		body, err := j.doRequest(endpoint)
		if err != nil {
			return result, fmt.Errorf("failed to fetch Jellyfin items: %w", err)
		}

		var resp struct {
			Items            []jellyfinItem `json:"Items"`
			TotalRecordCount int            `json:"TotalRecordCount"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return result, fmt.Errorf("failed to parse Jellyfin items: %w", err)
		}

		for _, item := range resp.Items {
			key := strings.ToLower(strings.TrimSpace(item.Name))
			if key == "" {
				continue
			}
			data := &WatchData{
				PlayCount: item.UserData.PlayCount,
			}
			if item.UserData.LastPlayedDate != "" {
				if t, err := time.Parse(time.RFC3339, item.UserData.LastPlayedDate); err == nil {
					data.LastPlayed = &t
				}
			}
			// Keep the entry with the highest play count if there are duplicates
			if existing, ok := result[key]; ok {
				if data.PlayCount > existing.PlayCount {
					result[key] = data
				}
			} else {
				result[key] = data
			}
		}

		startIndex += len(resp.Items)
		if startIndex >= resp.TotalRecordCount || len(resp.Items) == 0 {
			break
		}
	}

	return result, nil
}

// GetFavoritedItems returns a set of normalized title keys for items marked as
// favorites by the user. Jellyfin's Items API supports IsFavorite=true as a
// filter. The returned map is keyed by lowercase title for matching against
// *arr items.
func (j *JellyfinClient) GetFavoritedItems(userID string) (map[string]bool, error) {
	result := make(map[string]bool)
	startIndex := 0
	pageSize := 500

	for {
		endpoint := fmt.Sprintf(
			"/Users/%s/Items?IsFavorite=true&IncludeItemTypes=Movie,Series&Recursive=true&StartIndex=%d&Limit=%d",
			userID, startIndex, pageSize,
		)
		body, err := j.doRequest(endpoint)
		if err != nil {
			return result, fmt.Errorf("failed to fetch Jellyfin favorites: %w", err)
		}

		var resp struct {
			Items            []jellyfinItem `json:"Items"`
			TotalRecordCount int            `json:"TotalRecordCount"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return result, fmt.Errorf("failed to parse Jellyfin favorites: %w", err)
		}

		for _, item := range resp.Items {
			key := strings.ToLower(strings.TrimSpace(item.Name))
			if key != "" {
				result[key] = true
			}
		}

		startIndex += len(resp.Items)
		if startIndex >= resp.TotalRecordCount || len(resp.Items) == 0 {
			break
		}
	}

	return result, nil
}

// GetAdminUserID returns the first admin user's ID for making user-specific queries.
func (j *JellyfinClient) GetAdminUserID() (string, error) {
	body, err := j.doRequest("/Users")
	if err != nil {
		return "", err
	}

	var users []struct {
		ID     string `json:"Id"`
		Name   string `json:"Name"`
		Policy struct {
			IsAdministrator bool `json:"IsAdministrator"`
		} `json:"Policy"`
	}

	if err := json.Unmarshal(body, &users); err != nil {
		return "", fmt.Errorf("failed to parse Jellyfin users: %w", err)
	}

	for _, u := range users {
		if u.Policy.IsAdministrator {
			return u.ID, nil
		}
	}

	if len(users) > 0 {
		return users[0].ID, nil
	}

	return "", fmt.Errorf("no Jellyfin users found")
}

// ─── Capability interface implementations ───────────────────────────────────

// GetBulkWatchData implements WatchDataProvider by resolving the admin user
// internally and delegating to the userID-based method.
func (j *JellyfinClient) GetBulkWatchData() (map[string]*WatchData, error) {
	userID, err := j.GetAdminUserID()
	if err != nil {
		return nil, fmt.Errorf("jellyfin watch data: %w", err)
	}
	return j.GetBulkWatchDataForUser(userID)
}

// GetWatchlistItems implements WatchlistProvider by resolving the admin user
// and returning favorited items.
func (j *JellyfinClient) GetWatchlistItems() (map[string]bool, error) {
	userID, err := j.GetAdminUserID()
	if err != nil {
		return nil, fmt.Errorf("jellyfin watchlist: %w", err)
	}
	return j.GetFavoritedItems(userID)
}

// Verify JellyfinClient satisfies capability interfaces at compile time.
var _ Connectable = (*JellyfinClient)(nil)
var _ WatchDataProvider = (*JellyfinClient)(nil)
var _ WatchlistProvider = (*JellyfinClient)(nil)
