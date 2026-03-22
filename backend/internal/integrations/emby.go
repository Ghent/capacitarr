// Package integrations provides clients for external media management services.
package integrations

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// EmbyClient provides access to the Emby API for watch history data.
// Emby's API is structurally similar to Jellyfin (Jellyfin forked from Emby),
// using the same X-Emby-Token auth header and similar endpoint patterns.
type EmbyClient struct {
	URL    string
	APIKey string `json:"-"`
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

// embyItem represents a media item from the Emby API
type embyItem struct {
	Name        string            `json:"Name"`
	ProviderIDs map[string]string `json:"ProviderIds"` // e.g. {"Tmdb": "12345", "Imdb": "tt1234567"}
	UserData    struct {
		PlayCount      int    `json:"PlayCount"`
		LastPlayedDate string `json:"LastPlayedDate"`
		Played         bool   `json:"Played"`
	} `json:"UserData"`
}

// embyUser represents a user from the Emby /Users endpoint.
type embyUser struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Policy struct {
		IsAdministrator bool `json:"IsAdministrator"`
	} `json:"Policy"`
}

// getAllUsers fetches all users from Emby.
func (e *EmbyClient) getAllUsers() ([]embyUser, error) {
	body, err := e.doRequest("/Users")
	if err != nil {
		return nil, err
	}
	var users []embyUser
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, fmt.Errorf("failed to parse Emby users: %w", err)
	}
	return users, nil
}

// GetBulkWatchDataForUser fetches all movies and series from Emby's library with their
// watch data (PlayCount, LastPlayedDate) in a single paginated API call.
// Returns a map from TMDb ID to WatchData.
func (e *EmbyClient) GetBulkWatchDataForUser(userID, userName string) (map[int]*WatchData, error) {
	result := make(map[int]*WatchData)
	startIndex := 0
	pageSize := 500

	for {
		endpoint := fmt.Sprintf(
			"/Users/%s/Items?IncludeItemTypes=Movie,Series&Recursive=true&Fields=UserData,ProviderIds&StartIndex=%d&Limit=%d",
			userID, startIndex, pageSize,
		)
		body, err := e.doRequest(endpoint)
		if err != nil {
			return result, fmt.Errorf("failed to fetch Emby items: %w", err)
		}

		var resp struct {
			Items            []embyItem `json:"Items"`
			TotalRecordCount int        `json:"TotalRecordCount"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return result, fmt.Errorf("failed to parse Emby items: %w", err)
		}

		for _, item := range resp.Items {
			tmdbID := extractTMDbID(item.ProviderIDs)
			if tmdbID == 0 {
				continue // Skip items without TMDb ID
			}

			data := &WatchData{
				PlayCount: item.UserData.PlayCount,
			}
			if item.UserData.LastPlayedDate != "" {
				if t, err := time.Parse(time.RFC3339, item.UserData.LastPlayedDate); err == nil {
					data.LastPlayed = &t
				}
			}
			// Track which user watched this item
			if item.UserData.PlayCount > 0 && userName != "" {
				data.Users = []string{userName}
			}

			// Keep the entry with the highest play count if there are duplicates
			if existing, ok := result[tmdbID]; ok {
				if data.PlayCount > existing.PlayCount {
					result[tmdbID] = data
				}
			} else {
				result[tmdbID] = data
			}
		}

		startIndex += len(resp.Items)
		if startIndex >= resp.TotalRecordCount || len(resp.Items) == 0 {
			break
		}
	}

	return result, nil
}

// GetFavoritedItems returns a set of TMDb IDs for items marked as favorites
// by the user. Emby's Items API supports IsFavorite=true as a filter (same
// pattern as Jellyfin). The returned map is keyed by TMDb ID for matching
// against *arr items.
func (e *EmbyClient) GetFavoritedItems(userID string) (map[int]bool, error) {
	result := make(map[int]bool)
	startIndex := 0
	pageSize := 500

	for {
		endpoint := fmt.Sprintf(
			"/Users/%s/Items?IsFavorite=true&IncludeItemTypes=Movie,Series&Recursive=true&Fields=ProviderIds&StartIndex=%d&Limit=%d",
			userID, startIndex, pageSize,
		)
		body, err := e.doRequest(endpoint)
		if err != nil {
			return result, fmt.Errorf("failed to fetch Emby favorites: %w", err)
		}

		var resp struct {
			Items []struct {
				ProviderIDs map[string]string `json:"ProviderIds"`
			} `json:"Items"`
			TotalRecordCount int `json:"TotalRecordCount"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return result, fmt.Errorf("failed to parse Emby favorites: %w", err)
		}

		for _, item := range resp.Items {
			tmdbID := extractTMDbID(item.ProviderIDs)
			if tmdbID > 0 {
				result[tmdbID] = true
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
	users, err := e.getAllUsers()
	if err != nil {
		return "", err
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

// ─── Capability interface implementations ───────────────────────────────────

// GetBulkWatchData implements WatchDataProvider by iterating all users and
// aggregating watch data across all of them. Play counts are summed, the most
// recent LastPlayed is kept, and usernames are unioned into Users.
func (e *EmbyClient) GetBulkWatchData() (map[int]*WatchData, error) {
	users, err := e.getAllUsers()
	if err != nil {
		return nil, fmt.Errorf("emby watch data: %w", err)
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("emby watch data: no users found")
	}

	merged := make(map[int]*WatchData)

	for _, user := range users {
		userData, err := e.GetBulkWatchDataForUser(user.ID, user.Name)
		if err != nil {
			slog.Warn("Failed to fetch Emby watch data for user",
				"component", "emby", "user", user.Name, "error", err)
			continue
		}

		for tmdbID, wd := range userData {
			if existing, ok := merged[tmdbID]; ok {
				// Sum play counts across users
				existing.PlayCount += wd.PlayCount
				// Keep the most recent LastPlayed
				if wd.LastPlayed != nil && (existing.LastPlayed == nil || wd.LastPlayed.After(*existing.LastPlayed)) {
					existing.LastPlayed = wd.LastPlayed
				}
				// Union usernames
				if len(wd.Users) > 0 {
					existing.Users = append(existing.Users, wd.Users...)
				}
			} else {
				merged[tmdbID] = wd
			}
		}
	}

	return merged, nil
}

// GetWatchlistItems implements WatchlistProvider by iterating all users
// and returning the union of favorited items keyed by TMDb ID.
func (e *EmbyClient) GetWatchlistItems() (map[int]bool, error) {
	users, err := e.getAllUsers()
	if err != nil {
		return nil, fmt.Errorf("emby watchlist: %w", err)
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("emby watchlist: no users found")
	}

	merged := make(map[int]bool)

	for _, user := range users {
		favs, err := e.GetFavoritedItems(user.ID)
		if err != nil {
			slog.Warn("Failed to fetch Emby favorites for user",
				"component", "emby", "user", user.Name, "error", err)
			continue
		}
		for tmdbID := range favs {
			merged[tmdbID] = true
		}
	}

	return merged, nil
}

// Verify EmbyClient satisfies capability interfaces at compile time.
var _ Connectable = (*EmbyClient)(nil)
var _ WatchDataProvider = (*EmbyClient)(nil)
var _ WatchlistProvider = (*EmbyClient)(nil)
