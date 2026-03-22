package integrations

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
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
	ID           string            `json:"Id"`
	Name         string            `json:"Name"`
	Type         string            `json:"Type"` // "Movie", "Series", "Episode", "Audio"
	Path         string            `json:"Path"`
	RunTimeTicks int64             `json:"RunTimeTicks"`
	ProviderIDs  map[string]string `json:"ProviderIds"` // e.g. {"Tmdb": "12345", "Imdb": "tt1234567"}
	UserData     struct {
		PlayCount      int    `json:"PlayCount"`
		LastPlayedDate string `json:"LastPlayedDate"`
		Played         bool   `json:"Played"`
	} `json:"UserData"`
}

// jellyfinUser represents a user from the Jellyfin /Users endpoint.
type jellyfinUser struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Policy struct {
		IsAdministrator bool `json:"IsAdministrator"`
	} `json:"Policy"`
}

// getAllUsers fetches all users from Jellyfin.
func (j *JellyfinClient) getAllUsers() ([]jellyfinUser, error) {
	body, err := j.doRequest("/Users")
	if err != nil {
		return nil, err
	}
	var users []jellyfinUser
	if err := json.Unmarshal(body, &users); err != nil {
		return nil, fmt.Errorf("failed to parse Jellyfin users: %w", err)
	}
	return users, nil
}

// extractTMDbID extracts the TMDb ID from a Jellyfin/Emby item's ProviderIDs map.
// Returns 0 if the TMDb ID is not present or invalid.
func extractTMDbID(providerIDs map[string]string) int {
	tmdbStr, ok := providerIDs["Tmdb"]
	if !ok || tmdbStr == "" {
		return 0
	}
	id, err := strconv.Atoi(tmdbStr)
	if err != nil {
		return 0
	}
	return id
}

// GetBulkWatchDataForUser fetches all movies and series from Jellyfin's library with their
// watch data (PlayCount, LastPlayedDate) in a single paginated API call.
// Returns a map from TMDb ID to watch data, along with the username for user tracking.
func (j *JellyfinClient) GetBulkWatchDataForUser(userID, userName string) (map[int]*WatchData, error) {
	result := make(map[int]*WatchData)
	startIndex := 0
	pageSize := 500

	for {
		endpoint := fmt.Sprintf(
			"/Users/%s/Items?IncludeItemTypes=Movie,Series&Recursive=true&Fields=UserData,ProviderIds&StartIndex=%d&Limit=%d",
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

// GetFavoritedItems returns a set of TMDb IDs for items marked as
// favorites by the user. Jellyfin's Items API supports IsFavorite=true as a
// filter. The returned map is keyed by TMDb ID for matching against *arr items.
func (j *JellyfinClient) GetFavoritedItems(userID string) (map[int]bool, error) {
	result := make(map[int]bool)
	startIndex := 0
	pageSize := 500

	for {
		endpoint := fmt.Sprintf(
			"/Users/%s/Items?IsFavorite=true&IncludeItemTypes=Movie,Series&Recursive=true&Fields=ProviderIds&StartIndex=%d&Limit=%d",
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
func (j *JellyfinClient) GetAdminUserID() (string, error) {
	users, err := j.getAllUsers()
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

	return "", fmt.Errorf("no Jellyfin users found")
}

// ─── Capability interface implementations ───────────────────────────────────

// GetBulkWatchData implements WatchDataProvider by iterating all users and
// aggregating watch data across all of them. Play counts are summed, the most
// recent LastPlayed is kept, and usernames are unioned into Users.
func (j *JellyfinClient) GetBulkWatchData() (map[int]*WatchData, error) {
	users, err := j.getAllUsers()
	if err != nil {
		return nil, fmt.Errorf("jellyfin watch data: %w", err)
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("jellyfin watch data: no users found")
	}

	merged := make(map[int]*WatchData)

	for _, user := range users {
		userData, err := j.GetBulkWatchDataForUser(user.ID, user.Name)
		if err != nil {
			slog.Warn("Failed to fetch Jellyfin watch data for user",
				"component", "jellyfin", "user", user.Name, "error", err)
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
func (j *JellyfinClient) GetWatchlistItems() (map[int]bool, error) {
	users, err := j.getAllUsers()
	if err != nil {
		return nil, fmt.Errorf("jellyfin watchlist: %w", err)
	}
	if len(users) == 0 {
		return nil, fmt.Errorf("jellyfin watchlist: no users found")
	}

	merged := make(map[int]bool)

	for _, user := range users {
		favs, err := j.GetFavoritedItems(user.ID)
		if err != nil {
			slog.Warn("Failed to fetch Jellyfin favorites for user",
				"component", "jellyfin", "user", user.Name, "error", err)
			continue
		}
		for tmdbID := range favs {
			merged[tmdbID] = true
		}
	}

	return merged, nil
}

// Verify JellyfinClient satisfies capability interfaces at compile time.
var _ Connectable = (*JellyfinClient)(nil)
var _ WatchDataProvider = (*JellyfinClient)(nil)
var _ WatchlistProvider = (*JellyfinClient)(nil)
