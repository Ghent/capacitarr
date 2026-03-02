package integrations

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OverseerrClient provides access to the Overseerr/Jellyseerr API for media request data.
// Overseerr tracks user-requested content, which is valuable for scoring — requested
// content should be protected from deletion since users specifically asked for it.
type OverseerrClient struct {
	URL    string
	APIKey string
}

// NewOverseerrClient creates a new Overseerr/Jellyseerr API client.
func NewOverseerrClient(url, apiKey string) *OverseerrClient {
	return &OverseerrClient{
		URL:    strings.TrimRight(url, "/"),
		APIKey: apiKey,
	}
}

// OverseerrMediaRequest contains a media request from Overseerr.
type OverseerrMediaRequest struct {
	MediaType   string `json:"mediaType"` // "movie" or "tv"
	TMDbID      int    `json:"tmdbId"`
	Status      int    `json:"status"` // 1=pending, 2=approved, 3=declined, 4=available
	RequestedBy string `json:"requestedBy"`
}

// doRequest executes an Overseerr API call using the X-Api-Key header.
func (o *OverseerrClient) doRequest(endpoint string) ([]byte, error) {
	fullURL := fmt.Sprintf("%s/api/v1%s", o.URL, endpoint)
	return DoAPIRequest(fullURL, "X-Api-Key", o.APIKey)
}

// overseerrStatusResponse maps the /api/v1/status endpoint response.
type overseerrStatusResponse struct {
	Version string `json:"version"`
}

// TestConnection verifies the Overseerr URL and API key are valid
// by calling the /api/v1/status endpoint.
func (o *OverseerrClient) TestConnection() error {
	body, err := o.doRequest("/status")
	if err != nil {
		return err
	}

	var resp overseerrStatusResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return fmt.Errorf("failed to parse Overseerr status response: %w", err)
	}

	if resp.Version == "" {
		return fmt.Errorf("overseerr returned empty version, unexpected response")
	}

	return nil
}

// overseerrRequestResults maps the paginated request list response.
type overseerrRequestResults struct {
	PageInfo struct {
		Pages   int `json:"pages"`
		Page    int `json:"page"`
		Results int `json:"results"`
	} `json:"pageInfo"`
	Results []overseerrRequest `json:"results"`
}

// overseerrRequest maps a single request object from Overseerr.
type overseerrRequest struct {
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

// GetRequestedMedia fetches all media requests from Overseerr to identify
// user-requested content. This data can be used to protect requested items
// from automatic deletion.
func (o *OverseerrClient) GetRequestedMedia() ([]OverseerrMediaRequest, error) {
	var allRequests []OverseerrMediaRequest
	skip := 0
	take := 100

	for {
		endpoint := fmt.Sprintf("/request?take=%d&skip=%d&filter=all", take, skip)
		body, err := o.doRequest(endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch requests: %w", err)
		}

		var results overseerrRequestResults
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

			allRequests = append(allRequests, OverseerrMediaRequest{
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
