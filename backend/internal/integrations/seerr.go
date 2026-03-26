package integrations

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SeerrClient provides access to the Seerr (compatible with Overseerr and Jellyseerr) API for media request data.
// Seerr tracks user-requested content, which is valuable for scoring — requested
// content should be protected from deletion since users specifically asked for it.
type SeerrClient struct {
	URL    string
	APIKey string `json:"-"`
}

// NewSeerrClient creates a new Seerr (compatible with Overseerr and Jellyseerr) API client.
func NewSeerrClient(url, apiKey string) *SeerrClient {
	return &SeerrClient{
		URL:    strings.TrimRight(url, "/"),
		APIKey: apiKey,
	}
}

// doRequest executes an Seerr API call using the X-Api-Key header.
func (o *SeerrClient) doRequest(endpoint string) ([]byte, error) {
	fullURL := fmt.Sprintf("%s/api/v1%s", o.URL, endpoint)
	return DoAPIRequest(fullURL, "X-Api-Key", o.APIKey)
}

// seerrAuthMeResponse maps the /api/v1/auth/me endpoint response.
// This endpoint requires a valid API key, unlike /api/v1/status which is public.
type seerrAuthMeResponse struct {
	ID int `json:"id"`
}

// TestConnection verifies the Seerr URL and API key are valid
// by calling the /api/v1/auth/me endpoint. This endpoint requires
// authentication, unlike /api/v1/status which is public and returns
// 200 for any API key (including invalid ones).
func (o *SeerrClient) TestConnection() error {
	body, err := o.doRequest("/auth/me")
	if err != nil {
		return err
	}

	var resp seerrAuthMeResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse Seerr auth response: %w", err)
	}

	if resp.ID == 0 {
		return fmt.Errorf("seerr returned invalid user, unexpected response")
	}

	return nil
}

// seerrRequestResults maps the paginated request list response.
type seerrRequestResults struct {
	PageInfo struct {
		Pages   int `json:"pages"`
		Page    int `json:"page"`
		Results int `json:"results"`
	} `json:"pageInfo"`
	Results []seerrRequest `json:"results"`
}

// seerrRequest maps a single request object from Seerr.
type seerrRequest struct {
	ID        int    `json:"id"`
	Status    int    `json:"status"` // 1=pending, 2=approved, 3=declined, 4=available
	MediaType string `json:"type"`   // "movie" or "tv"
	Media     struct {
		TmdbID    int    `json:"tmdbId"`
		MediaType string `json:"mediaType"`
	} `json:"media"`
	RequestedBy struct {
		DisplayName string `json:"displayName"`
		Username    string `json:"username"`
	} `json:"requestedBy"`
}

// GetRequestedMedia fetches all media requests from Seerr to identify
// user-requested content. This data can be used to protect requested items
// from automatic deletion.
func (o *SeerrClient) GetRequestedMedia() ([]MediaRequest, error) {
	var allRequests []MediaRequest
	skip := 0
	take := 100

	for {
		endpoint := fmt.Sprintf("/request?take=%d&skip=%d&filter=all", take, skip)
		body, err := o.doRequest(endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch requests: %w", err)
		}

		var results seerrRequestResults
		if err := json.Unmarshal(body, &results); err != nil {
			return nil, fmt.Errorf("failed to parse request results: %w", err)
		}

		for _, req := range results.Results {
			username := req.RequestedBy.DisplayName
			if username == "" {
				username = req.RequestedBy.Username
			}

			mediaType := req.Media.MediaType
			if mediaType == "" {
				mediaType = req.MediaType
			}

			allRequests = append(allRequests, MediaRequest{
				MediaType:   mediaType,
				TMDbID:      req.Media.TmdbID,
				Status:      req.Status,
				RequestedBy: username,
			})
		}

		// Check if we've fetched all pages
		if len(results.Results) < take {
			break
		}
		skip += take
	}

	return allRequests, nil
}

// Verify SeerrClient satisfies capability interfaces at compile time.
var _ Connectable = (*SeerrClient)(nil)
var _ RequestProvider = (*SeerrClient)(nil)
