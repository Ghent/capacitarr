// Package integrations provides clients for external media management services.
package integrations

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// EmbyClient provides access to the Emby API for watch history data.
// Emby's API is structurally similar to Jellyfin (Jellyfin forked from Emby),
// using the same X-Emby-Token auth header and similar endpoint patterns.
//
// Per-cycle caching: getAllUsers() and GetAdminUserID() cache their results
// for the lifetime of this client instance (one poll cycle). Since
// BuildIntegrationRegistry() creates new client instances each cycle, caches
// are naturally cycle-scoped. This eliminates ~5 redundant /Users API calls
// per cycle where multiple methods independently fetched the same user list.
type EmbyClient struct {
	URL    string
	APIKey string `json:"-"`

	// Per-cycle user list cache. Populated on first call to getAllUsers().
	cachedUsers    []embyUser
	cachedUsersErr error
	usersOnce      sync.Once

	// Per-cycle admin user ID cache. Populated on first call to GetAdminUserID().
	cachedAdminID    string
	cachedAdminIDErr error
	adminOnce        sync.Once
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
	ID          string            `json:"Id"`
	Name        string            `json:"Name"`
	SeriesID    string            `json:"SeriesId,omitempty"` // Parent series ID (Episode items only)
	Type        string            `json:"Type"`               // "Movie", "Series", "Episode", "Audio"
	DateCreated string            `json:"DateCreated"`        // When the item was added to the Emby library (RFC3339)
	ProviderIDs map[string]string `json:"ProviderIds"`        // e.g. {"Tmdb": "12345", "Imdb": "tt1234567"}
	Tags        []string          `json:"Tags"`               // User-defined tags on items (equivalent to Plex Labels)
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

// getAllUsers returns all Emby users, caching the result for the lifetime
// of this client instance (one poll cycle). The /Users endpoint is hit by
// GetBulkWatchData, GetWatchlistItems, GetCollectionMemberships, and
// GetAdminUserID — caching avoids ~5 redundant API calls per cycle.
func (e *EmbyClient) getAllUsers() ([]embyUser, error) {
	e.usersOnce.Do(func() {
		e.cachedUsers, e.cachedUsersErr = e.fetchAllUsers()
	})
	return e.cachedUsers, e.cachedUsersErr
}

// fetchAllUsers performs the actual HTTP call to fetch all Emby users.
func (e *EmbyClient) fetchAllUsers() ([]embyUser, error) {
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
// watch data (PlayCount, LastPlayedDate) in a single paginated API call, then supplements
// with episode-level watch data that may not be reflected on the parent Series item.
// Returns a map from TMDb ID to WatchData.
func (e *EmbyClient) GetBulkWatchDataForUser(userID, userName string) (map[int]*WatchData, error) {
	result := make(map[int]*WatchData)
	// seriesIndex maps Emby item IDs for Series items to their TMDb ID.
	// Used in the episode pass to find the parent series TMDb ID.
	seriesIndex := make(map[string]int)
	startIndex := 0
	pageSize := 500

	// Pass 1: Fetch Movie and Series items.
	for {
		endpoint := fmt.Sprintf(
			"/Users/%s/Items?IncludeItemTypes=Movie,Series&Recursive=true&Fields=UserData,ProviderIds,DateCreated&StartIndex=%d&Limit=%d",
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

			// Build series index for episode lookups
			if item.Type == "Series" && item.ID != "" {
				seriesIndex[item.ID] = tmdbID
			}

			data := &WatchData{
				PlayCount: item.UserData.PlayCount,
			}
			if item.UserData.LastPlayedDate != "" {
				if t, err := time.Parse(time.RFC3339, item.UserData.LastPlayedDate); err == nil {
					data.LastPlayed = &t
				}
			}
			// Bridge the media server's library date to WatchData for enrichment
			if item.DateCreated != "" {
				if t, err := time.Parse(time.RFC3339, item.DateCreated); err == nil {
					data.AddedAt = &t
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

	// Pass 2: Fetch played Episode items and aggregate into parent Series.
	// Emby sometimes tracks play data on episodes but not on the parent
	// Series item, so a Series may show PlayCount=0 even when episodes have
	// been watched. This pass fixes that by rolling episode data up.
	if len(seriesIndex) > 0 {
		startIndex = 0
		for {
			endpoint := fmt.Sprintf(
				"/Users/%s/Items?IncludeItemTypes=Episode&IsPlayed=true&Recursive=true&Fields=UserData&StartIndex=%d&Limit=%d",
				userID, startIndex, pageSize,
			)
			body, err := e.doRequest(endpoint)
			if err != nil {
				slog.Error("Failed to fetch Emby episode watch data",
					"component", "emby", "user", userName, "error", err)
				break
			}

			var resp struct {
				Items            []embyItem `json:"Items"`
				TotalRecordCount int        `json:"TotalRecordCount"`
			}
			if err := json.Unmarshal(body, &resp); err != nil {
				slog.Error("Failed to parse Emby episode response",
					"component", "emby", "error", err)
				break
			}

			for _, ep := range resp.Items {
				if ep.SeriesID == "" || ep.UserData.PlayCount == 0 {
					continue
				}
				seriesTMDbID, ok := seriesIndex[ep.SeriesID]
				if !ok || seriesTMDbID == 0 {
					continue
				}

				var epLastPlayed *time.Time
				if ep.UserData.LastPlayedDate != "" {
					if t, err := time.Parse(time.RFC3339, ep.UserData.LastPlayedDate); err == nil {
						epLastPlayed = &t
					}
				}

				if existing, ok := result[seriesTMDbID]; ok {
					// Series already in result — supplement with episode data.
					if ep.UserData.PlayCount > existing.PlayCount {
						existing.PlayCount = ep.UserData.PlayCount
					}
					if epLastPlayed != nil && (existing.LastPlayed == nil || epLastPlayed.After(*existing.LastPlayed)) {
						existing.LastPlayed = epLastPlayed
					}
					if userName != "" && len(existing.Users) == 0 {
						existing.Users = []string{userName}
					}
				} else {
					wd := &WatchData{
						PlayCount: ep.UserData.PlayCount,
					}
					if epLastPlayed != nil {
						wd.LastPlayed = epLastPlayed
					}
					if userName != "" {
						wd.Users = []string{userName}
					}
					result[seriesTMDbID] = wd
				}
			}

			startIndex += len(resp.Items)
			if startIndex >= resp.TotalRecordCount || len(resp.Items) == 0 {
				break
			}
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
// Result is cached for the lifetime of this client instance (one poll cycle).
func (e *EmbyClient) GetAdminUserID() (string, error) {
	e.adminOnce.Do(func() {
		e.cachedAdminID, e.cachedAdminIDErr = e.resolveAdminUserID()
	})
	return e.cachedAdminID, e.cachedAdminIDErr
}

// resolveAdminUserID performs the actual admin user lookup from the cached user list.
func (e *EmbyClient) resolveAdminUserID() (string, error) {
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
			slog.Error("Failed to fetch Emby watch data for user",
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
			slog.Error("Failed to fetch Emby favorites for user",
				"component", "emby", "user", user.Name, "error", err)
			continue
		}
		for tmdbID := range favs {
			merged[tmdbID] = true
		}
	}

	return merged, nil
}

// GetItemIDToTMDbIDMap builds a map from Emby Item ID to TMDb ID for all
// movies and series in the library. Used by the Tracearr enricher to resolve
// Emby rating keys to TMDb IDs.
func (e *EmbyClient) GetItemIDToTMDbIDMap() (map[string]int, error) {
	adminID, err := e.GetAdminUserID()
	if err != nil {
		return nil, fmt.Errorf("emby item ID map: %w", err)
	}

	result := make(map[string]int)
	startIndex := 0
	pageSize := 500

	for {
		endpoint := fmt.Sprintf(
			"/Users/%s/Items?IncludeItemTypes=Movie,Series&Recursive=true&Fields=ProviderIds&StartIndex=%d&Limit=%d",
			adminID, startIndex, pageSize,
		)
		body, err := e.doRequest(endpoint)
		if err != nil {
			return result, fmt.Errorf("failed to fetch Emby items for ID map: %w", err)
		}

		var resp struct {
			Items            []embyItem `json:"Items"`
			TotalRecordCount int        `json:"TotalRecordCount"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return result, fmt.Errorf("failed to parse Emby items for ID map: %w", err)
		}

		for _, item := range resp.Items {
			tmdbID := extractTMDbID(item.ProviderIDs)
			if tmdbID > 0 {
				result[item.ID] = tmdbID
			}
		}

		startIndex += len(resp.Items)
		if startIndex >= resp.TotalRecordCount || len(resp.Items) == 0 {
			break
		}
	}

	slog.Debug("Built Emby Item ID → TMDb ID map", "component", "emby",
		"mappings", len(result))

	return result, nil
}

// GetCollectionNames returns a sorted, deduplicated list of Box Set names from
// Emby. This is used by FetchCollectionValues() to provide autocomplete
// options for collection-based rules. Emby's API is structurally identical to
// Jellyfin (forked codebase).
func (e *EmbyClient) GetCollectionNames() ([]string, error) {
	adminID, err := e.GetAdminUserID()
	if err != nil {
		return nil, fmt.Errorf("failed to get Emby admin user for collection names: %w", err)
	}

	endpoint := fmt.Sprintf("/Users/%s/Items?IncludeItemTypes=BoxSet&Recursive=true", adminID)
	body, err := e.doRequest(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Emby box sets: %w", err)
	}

	var resp struct {
		Items []struct {
			Name string `json:"Name"`
		} `json:"Items"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse Emby box sets: %w", err)
	}

	seen := make(map[string]bool)
	for _, item := range resp.Items {
		name := strings.TrimSpace(item.Name)
		if name != "" {
			seen[name] = true
		}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// GetCollectionMemberships fetches all Box Sets from Emby, then fetches their
// child items to build a TMDb ID → box set name mapping. Emby's API is
// structurally identical to Jellyfin (forked codebase). Implements CollectionDataProvider.
func (e *EmbyClient) GetCollectionMemberships() (map[int][]string, error) {
	// Find an admin user for API queries
	users, err := e.getAllUsers()
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Emby users for collection memberships: %w", err)
	}
	var adminUserID string
	for _, u := range users {
		if u.Policy.IsAdministrator {
			adminUserID = u.ID
			break
		}
	}
	if adminUserID == "" && len(users) > 0 {
		adminUserID = users[0].ID
	}
	if adminUserID == "" {
		return nil, fmt.Errorf("no Emby users found for collection memberships")
	}

	// Fetch all Box Sets
	endpoint := fmt.Sprintf("/Users/%s/Items?IncludeItemTypes=BoxSet&Recursive=true&Fields=ProviderIds", adminUserID)
	body, err := e.doRequest(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Emby box sets: %w", err)
	}

	var boxSetResp struct {
		Items []struct {
			ID   string `json:"Id"`
			Name string `json:"Name"`
		} `json:"Items"`
	}
	if err := json.Unmarshal(body, &boxSetResp); err != nil {
		return nil, fmt.Errorf("failed to parse Emby box sets: %w", err)
	}

	result := make(map[int][]string)

	// For each Box Set, fetch its children and map their TMDb IDs
	for _, boxSet := range boxSetResp.Items {
		childEndpoint := fmt.Sprintf("/Users/%s/Items?ParentId=%s&Fields=ProviderIds", adminUserID, boxSet.ID)
		childBody, childErr := e.doRequest(childEndpoint)
		if childErr != nil {
			slog.Error("Failed to fetch Emby box set children", "component", "integrations",
				"boxSet", boxSet.Name, "error", childErr)
			continue
		}

		var childResp struct {
			Items []embyItem `json:"Items"`
		}
		if childErr := json.Unmarshal(childBody, &childResp); childErr != nil {
			continue
		}

		for _, child := range childResp.Items {
			tmdbID := extractTMDbID(child.ProviderIDs)
			if tmdbID > 0 {
				result[tmdbID] = append(result[tmdbID], boxSet.Name)
			}
		}
	}

	return result, nil
}

// GetLabelMemberships implements LabelDataProvider by fetching all items with
// their Tags field from Emby. Emby "Tags" on items serve the same purpose as
// Plex "Labels" — user-defined freeform metadata.
func (e *EmbyClient) GetLabelMemberships() (map[int][]string, error) {
	adminID, err := e.GetAdminUserID()
	if err != nil {
		return nil, fmt.Errorf("failed to get Emby admin user for label memberships: %w", err)
	}

	result := make(map[int][]string)
	startIndex := 0
	pageSize := 500

	for {
		endpoint := fmt.Sprintf(
			"/Users/%s/Items?IncludeItemTypes=Movie,Series&Recursive=true&Fields=Tags,ProviderIds&StartIndex=%d&Limit=%d",
			adminID, startIndex, pageSize,
		)
		body, fetchErr := e.doRequest(endpoint)
		if fetchErr != nil {
			return result, fmt.Errorf("failed to fetch Emby items for labels: %w", fetchErr)
		}

		var resp struct {
			Items            []embyItem `json:"Items"`
			TotalRecordCount int        `json:"TotalRecordCount"`
		}
		if parseErr := json.Unmarshal(body, &resp); parseErr != nil {
			return result, fmt.Errorf("failed to parse Emby items for labels: %w", parseErr)
		}

		for _, item := range resp.Items {
			if len(item.Tags) == 0 {
				continue
			}
			tmdbID := extractTMDbID(item.ProviderIDs)
			if tmdbID > 0 {
				result[tmdbID] = item.Tags
			}
		}

		startIndex += len(resp.Items)
		if startIndex >= resp.TotalRecordCount || len(resp.Items) == 0 {
			break
		}
	}

	return result, nil
}

// GetLabelNames returns a sorted, deduplicated list of item tag names from
// Emby. Used by FetchLabelValues() for rule value autocomplete.
func (e *EmbyClient) GetLabelNames() ([]string, error) {
	adminID, err := e.GetAdminUserID()
	if err != nil {
		return nil, fmt.Errorf("failed to get Emby admin user for label names: %w", err)
	}

	seen := make(map[string]bool)
	startIndex := 0
	pageSize := 500

	for {
		endpoint := fmt.Sprintf(
			"/Users/%s/Items?IncludeItemTypes=Movie,Series&Recursive=true&Fields=Tags&StartIndex=%d&Limit=%d",
			adminID, startIndex, pageSize,
		)
		body, fetchErr := e.doRequest(endpoint)
		if fetchErr != nil {
			return nil, fmt.Errorf("failed to fetch Emby items for label names: %w", fetchErr)
		}

		var resp struct {
			Items []struct {
				Tags []string `json:"Tags"`
			} `json:"Items"`
			TotalRecordCount int `json:"TotalRecordCount"`
		}
		if parseErr := json.Unmarshal(body, &resp); parseErr != nil {
			return nil, fmt.Errorf("failed to parse Emby items for label names: %w", parseErr)
		}

		for _, item := range resp.Items {
			for _, tag := range item.Tags {
				name := strings.TrimSpace(tag)
				if name != "" {
					seen[name] = true
				}
			}
		}

		startIndex += len(resp.Items)
		if startIndex >= resp.TotalRecordCount || len(resp.Items) == 0 {
			break
		}
	}

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// embyTagPayload is the request body for Emby's dedicated tag endpoints
// (POST /Items/{Id}/Tags/Add and POST /Items/{Id}/Tags/Delete).
// Tags are NameIdPair objects; only Name is required for add/remove operations.
type embyTagPayload struct {
	Tags []embyNameIDPair `json:"Tags"`
}

// embyNameIDPair represents Emby's NameIdPair type used in the TagService API.
type embyNameIDPair struct {
	Name string `json:"Name"`
	ID   string `json:"Id,omitempty"`
}

// AddLabel adds a tag to an Emby item using the dedicated TagService endpoint.
// Uses POST /Items/{Id}/Tags/Add which properly updates Emby's internal TagItems
// storage. This is idempotent — Emby ignores duplicate tag names.
func (e *EmbyClient) AddLabel(itemID string, label string) error {
	payload := embyTagPayload{
		Tags: []embyNameIDPair{{Name: label}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal tag payload: %w", err)
	}

	fullURL := e.URL + "/Items/" + itemID + "/Tags/Add"
	return DoAPIRequestWithBody("POST", fullURL, body, "X-Emby-Token", e.APIKey)
}

// RemoveLabel removes a tag from an Emby item using the dedicated TagService endpoint.
// Uses POST /Items/{Id}/Tags/Delete which properly updates Emby's internal TagItems
// storage. The previous read-modify-write approach on the generic item update endpoint
// (POST /Items/{Id}) only modified the Tags string array but not the authoritative
// TagItems field, causing tags to persist after removal.
func (e *EmbyClient) RemoveLabel(itemID string, label string) error {
	payload := embyTagPayload{
		Tags: []embyNameIDPair{{Name: label}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal tag payload: %w", err)
	}

	fullURL := e.URL + "/Items/" + itemID + "/Tags/Delete"
	return DoAPIRequestWithBody("POST", fullURL, body, "X-Emby-Token", e.APIKey)
}

// GetPosterImage downloads the current primary poster for an Emby item.
func (e *EmbyClient) GetPosterImage(itemID string) ([]byte, string, error) {
	endpoint := fmt.Sprintf("/Items/%s/Images/Primary", itemID)
	data, err := e.doRequest(endpoint)
	if err != nil {
		return nil, "", fmt.Errorf("fetch poster: %w", err)
	}
	return data, "image/jpeg", nil
}

// UploadPosterImage uploads a new primary poster to an Emby item.
// Emby expects the image body as a base64-encoded string (same as Jellyfin)
// and requires the X-Emby-Token auth header alongside it.
func (e *EmbyClient) UploadPosterImage(itemID string, imageData []byte, contentType string) error {
	fullURL := e.URL + "/Items/" + itemID + "/Images/Primary"
	encoded := []byte(base64.StdEncoding.EncodeToString(imageData))
	return DoAPIRequestWithHeaders("POST", fullURL, encoded, map[string]string{
		"Content-Type": contentType,
		"X-Emby-Token": e.APIKey,
	})
}

// RestorePosterImage removes the custom primary poster from an Emby item,
// reverting to the default provider-sourced poster.
func (e *EmbyClient) RestorePosterImage(itemID string) error {
	fullURL := e.URL + "/Items/" + itemID + "/Images/Primary"
	return DoAPIRequestWithBody("DELETE", fullURL, nil, "X-Emby-Token", e.APIKey)
}

// GetTMDbToItemIDMap builds a mapping from TMDb ID to Emby item ID by
// scanning all movies and series. Used by SunsetService to translate TMDb IDs
// from *arr items into Emby native IDs for label operations.
func (e *EmbyClient) GetTMDbToItemIDMap() (map[int]string, error) {
	adminID, err := e.GetAdminUserID()
	if err != nil {
		return nil, fmt.Errorf("failed to get Emby admin user for TMDb mapping: %w", err)
	}

	result := make(map[int]string)
	startIndex := 0
	pageSize := 500

	for {
		endpoint := fmt.Sprintf(
			"/Users/%s/Items?IncludeItemTypes=Movie,Series&Recursive=true&Fields=ProviderIds&StartIndex=%d&Limit=%d",
			adminID, startIndex, pageSize,
		)
		body, fetchErr := e.doRequest(endpoint)
		if fetchErr != nil {
			return result, fmt.Errorf("failed to fetch Emby items for TMDb mapping: %w", fetchErr)
		}

		var resp struct {
			Items            []embyItem `json:"Items"`
			TotalRecordCount int        `json:"TotalRecordCount"`
		}
		if parseErr := json.Unmarshal(body, &resp); parseErr != nil {
			return result, fmt.Errorf("failed to parse Emby items for TMDb mapping: %w", parseErr)
		}

		for _, item := range resp.Items {
			tmdbID := extractTMDbID(item.ProviderIDs)
			if tmdbID > 0 {
				result[tmdbID] = item.ID
			}
		}

		startIndex += len(resp.Items)
		if startIndex >= resp.TotalRecordCount || len(resp.Items) == 0 {
			break
		}
	}

	return result, nil
}

// SearchByTMDbID searches Emby for an item matching the given TMDb ID.
// Uses title to narrow the search space, then verifies the TMDb ID in
// ProviderIds. Returns the Emby item ID.
func (e *EmbyClient) SearchByTMDbID(title string, tmdbID int) (string, error) {
	if title == "" || tmdbID <= 0 {
		return "", fmt.Errorf("title and tmdbID are required for Emby search")
	}

	adminID, err := e.GetAdminUserID()
	if err != nil {
		return "", fmt.Errorf("emby search: get admin user: %w", err)
	}

	endpoint := fmt.Sprintf(
		"/Users/%s/Items?SearchTerm=%s&Fields=ProviderIds&IncludeItemTypes=Movie,Series&Recursive=true&Limit=25",
		adminID, url.QueryEscape(title),
	)
	body, err := e.doRequest(endpoint)
	if err != nil {
		return "", fmt.Errorf("emby search: %w", err)
	}

	var resp struct {
		Items []embyItem `json:"Items"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("emby search unmarshal: %w", err)
	}

	for _, item := range resp.Items {
		if extractTMDbID(item.ProviderIDs) == tmdbID {
			return item.ID, nil
		}
	}

	return "", fmt.Errorf("emby search: no item found with TMDb ID %d", tmdbID)
}

var _ Connectable = (*EmbyClient)(nil)
var _ WatchDataProvider = (*EmbyClient)(nil)
var _ WatchlistProvider = (*EmbyClient)(nil)
var _ CollectionDataProvider = (*EmbyClient)(nil)
var _ LabelDataProvider = (*EmbyClient)(nil)
var _ LabelManager = (*EmbyClient)(nil)
var _ PosterManager = (*EmbyClient)(nil)
var _ NativeIDSearcher = (*EmbyClient)(nil)
