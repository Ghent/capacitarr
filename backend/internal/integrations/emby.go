package integrations

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// EmbyClient provides access to the Emby API for watch history data.
// Emby's API is structurally similar to Jellyfin (Jellyfin forked from Emby),
// using the same X-Emby-Token auth header and similar endpoint patterns.
type EmbyClient struct {
	URL    string
	APIKey string
}

// NewEmbyClient creates a new Emby media server API client.
func NewEmbyClient(url, apiKey string) *EmbyClient {
	return &EmbyClient{
		URL:    strings.TrimRight(url, "/"),
		APIKey: apiKey,
	}
}

func (e *EmbyClient) doRequest(endpoint string) ([]byte, error) {
	fullURL := e.URL + endpoint
	return DoAPIRequest(fullURL, "X-Emby-Token", e.APIKey)
}

// TestConnection verifies the Emby URL and API key by calling /System/Info
func (e *EmbyClient) TestConnection() error {
	body, err := e.doRequest("/System/Info")
	if err != nil {
		return err
	}
	var info struct {
		ServerName string `json:"ServerName"`
		Version    string `json:"Version"`
	}
	if err := json.Unmarshal(body, &info); err != nil {
		return fmt.Errorf("failed to parse Emby system info: %w", err)
	}
	if info.ServerName == "" && info.Version == "" {
		return fmt.Errorf("unexpected Emby response — no server name or version")
	}
	return nil
}

// EmbyWatchData contains watch history for a media item from Emby
type EmbyWatchData struct {
	PlayCount      int
	LastPlayedDate time.Time
	Played         bool
}

// GetWatchHistory fetches play history for a specific item by its Emby ID.
func (e *EmbyClient) GetWatchHistory(embyID, userID string) (*EmbyWatchData, error) {
	endpoint := fmt.Sprintf("/Users/%s/Items/%s", userID, embyID)
	body, err := e.doRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var item struct {
		UserData struct {
			PlayCount      int    `json:"PlayCount"`
			LastPlayedDate string `json:"LastPlayedDate"`
			Played         bool   `json:"Played"`
		} `json:"UserData"`
	}

	if err := json.Unmarshal(body, &item); err != nil {
		return nil, fmt.Errorf("failed to parse Emby item: %w", err)
	}

	data := &EmbyWatchData{
		PlayCount: item.UserData.PlayCount,
		Played:    item.UserData.Played,
	}

	if item.UserData.LastPlayedDate != "" {
		data.LastPlayedDate, _ = time.Parse(time.RFC3339, item.UserData.LastPlayedDate)
	}

	return data, nil
}

// GetBulkWatchData fetches all movies and series from Emby's library with their
// watch data (PlayCount, LastPlayedDate) in a single paginated API call.
// Returns a map from normalized (lowercase) title to MediaServerWatchData.
func (e *EmbyClient) GetBulkWatchData(userID string) (map[string]*MediaServerWatchData, error) {
	result := make(map[string]*MediaServerWatchData)
	startIndex := 0
	pageSize := 500

	for {
		endpoint := fmt.Sprintf(
			"/Users/%s/Items?IncludeItemTypes=Movie,Series&Recursive=true&Fields=UserData&StartIndex=%d&Limit=%d",
			userID, startIndex, pageSize,
		)
		body, err := e.doRequest(endpoint)
		if err != nil {
			return result, fmt.Errorf("failed to fetch Emby items: %w", err)
		}

		var resp struct {
			Items []struct {
				Name     string `json:"Name"`
				UserData struct {
					PlayCount      int    `json:"PlayCount"`
					LastPlayedDate string `json:"LastPlayedDate"`
					Played         bool   `json:"Played"`
				} `json:"UserData"`
			} `json:"Items"`
			TotalRecordCount int `json:"TotalRecordCount"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return result, fmt.Errorf("failed to parse Emby items: %w", err)
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
func (e *EmbyClient) GetAdminUserID() (string, error) {
	body, err := e.doRequest("/Users")
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
		return "", fmt.Errorf("failed to parse Emby users: %w", err)
	}

	for _, u := range users {
		if u.Policy.IsAdministrator {
			return u.ID, nil
		}
	}

	if len(users) > 0 {
		return users[0].ID, nil
	}

	return "", fmt.Errorf("no Emby users found")
}
