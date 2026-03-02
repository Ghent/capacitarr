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
	APIKey string
}

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
	ID                string `json:"Id"`
	Name              string `json:"Name"`
	Type              string `json:"Type"` // "Movie", "Series", "Episode", "Audio"
	Path              string `json:"Path"`
	RunTimeTicks      int64  `json:"RunTimeTicks"`
	UserData          struct {
		PlayCount       int    `json:"PlayCount"`
		LastPlayedDate  string `json:"LastPlayedDate"`
		Played          bool   `json:"Played"`
	} `json:"UserData"`
}

// JellyfinWatchData contains watch history for a media item
type JellyfinWatchData struct {
	PlayCount      int
	LastPlayedDate time.Time
	Played         bool
}

// GetWatchHistory fetches play history for a specific item by its Jellyfin ID.
// The userId parameter is required — use GetAdminUserID to find it.
func (j *JellyfinClient) GetWatchHistory(jellyfinID, userID string) (*JellyfinWatchData, error) {
	endpoint := fmt.Sprintf("/Users/%s/Items/%s", userID, jellyfinID)
	body, err := j.doRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var item jellyfinItem
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, fmt.Errorf("failed to parse Jellyfin item: %w", err)
	}

	data := &JellyfinWatchData{
		PlayCount: item.UserData.PlayCount,
		Played:    item.UserData.Played,
	}

	if item.UserData.LastPlayedDate != "" {
		data.LastPlayedDate, _ = time.Parse(time.RFC3339, item.UserData.LastPlayedDate)
	}

	return data, nil
}

// MediaServerWatchData contains aggregated watch data from a media server (Jellyfin or Emby).
// Used for bulk enrichment — keyed by normalized title in the lookup map.
type MediaServerWatchData struct {
	PlayCount  int
	LastPlayed *time.Time
	Played     bool
}

// GetBulkWatchData fetches all movies and series from Jellyfin's library with their
// watch data (PlayCount, LastPlayedDate) in a single paginated API call.
// Returns a map from normalized (lowercase) title to watch data.
func (j *JellyfinClient) GetBulkWatchData(userID string) (map[string]*MediaServerWatchData, error) {
	result := make(map[string]*MediaServerWatchData)
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
			data := &MediaServerWatchData{
				PlayCount: item.UserData.PlayCount,
				Played:    item.UserData.Played,
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
