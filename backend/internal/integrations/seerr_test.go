package integrations

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSeerrClient_TestConnection_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/me" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != testTautulliAPIKey {
			t.Errorf("Missing or wrong API key header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1,"displayName":"Admin","email":"admin@example.com"}`))
	}))
	defer srv.Close()

	client := NewSeerrClient(srv.URL, testTautulliAPIKey)
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should succeed: %v", err)
	}
}

func TestSeerrClient_TestConnection_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewSeerrClient(srv.URL, "bad-key")
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 401")
	}
}

func TestSeerrClient_TestConnection_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewSeerrClient(srv.URL, testTautulliAPIKey)
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 500")
	}
}

func TestSeerrClient_TestConnection_InvalidUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":0}`))
	}))
	defer srv.Close()

	client := NewSeerrClient(srv.URL, testTautulliAPIKey)
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail when user ID is zero")
	}
}

func TestSeerrClient_TestConnection_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	client := NewSeerrClient(srv.URL, testTautulliAPIKey)
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with malformed JSON")
	}
}

func TestSeerrClient_GetRequestedMedia(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/request" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"pageInfo": {
				"pages": 1,
				"page": 1,
				"results": 3
			},
			"results": [
				{
					"id": 1,
					"status": 2,
					"type": "movie",
					"media": {
						"tmdbId": 550,
						"mediaType": "movie"
					},
					"requestedBy": {
						"displayName": "Alice",
						"username": "alice123"
					}
				},
				{
					"id": 2,
					"status": 4,
					"type": "tv",
					"media": {
						"tmdbId": 1399,
						"mediaType": "tv"
					},
					"requestedBy": {
						"displayName": "",
						"username": "bob456"
					}
				},
				{
					"id": 3,
					"status": 1,
					"type": "movie",
					"media": {
						"tmdbId": 680,
						"mediaType": ""
					},
					"requestedBy": {
						"displayName": "Carol",
						"username": "carol789"
					}
				}
			]
		}`))
	}))
	defer srv.Close()

	client := NewSeerrClient(srv.URL, testTautulliAPIKey)
	requests, err := client.GetRequestedMedia()
	if err != nil {
		t.Fatalf("GetRequestedMedia should succeed: %v", err)
	}
	if len(requests) != 3 {
		t.Fatalf("Expected 3 requests, got %d", len(requests))
	}

	// First request: movie with displayName
	if requests[0].MediaType != "movie" {
		t.Errorf("Expected media type 'movie', got %q", requests[0].MediaType)
	}
	if requests[0].TMDbID != 550 {
		t.Errorf("Expected TMDbID 550, got %d", requests[0].TMDbID)
	}
	if requests[0].Status != 2 {
		t.Errorf("Expected status 2 (approved), got %d", requests[0].Status)
	}
	if requests[0].RequestedBy != "Alice" {
		t.Errorf("Expected requestedBy 'Alice', got %q", requests[0].RequestedBy)
	}

	// Second request: TV with empty displayName — should fall back to username
	if requests[1].MediaType != "tv" {
		t.Errorf("Expected media type 'tv', got %q", requests[1].MediaType)
	}
	if requests[1].RequestedBy != "bob456" {
		t.Errorf("Expected requestedBy 'bob456' (fallback to username), got %q", requests[1].RequestedBy)
	}
	if requests[1].TMDbID != 1399 {
		t.Errorf("Expected TMDbID 1399, got %d", requests[1].TMDbID)
	}

	// Third request: empty media.mediaType — should fall back to request type
	if requests[2].MediaType != "movie" {
		t.Errorf("Expected media type 'movie' (fallback from request type), got %q", requests[2].MediaType)
	}
	if requests[2].Status != 1 {
		t.Errorf("Expected status 1 (pending), got %d", requests[2].Status)
	}
}

func TestSeerrClient_GetRequestedMedia_Pagination(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount == 1 {
			// First page: return exactly 100 results (take=100) to trigger pagination
			results := `"results": [`
			for i := range 100 {
				if i > 0 {
					results += ","
				}
				results += `{
					"id": ` + itoa(i+1) + `,
					"status": 2,
					"type": "movie",
					"media": {"tmdbId": ` + itoa(1000+i) + `, "mediaType": "movie"},
					"requestedBy": {"displayName": "User", "username": "user"}
				}`
			}
			results += `]`
			page1 := `{"pageInfo": {"pages": 2, "page": 1, "results": 150}, ` + results + `}`
			_ = json.NewEncoder(w).Encode(json.RawMessage(page1))
		} else {
			// Second page: return remaining 50 results
			results := `"results": [`
			for i := range 50 {
				if i > 0 {
					results += ","
				}
				results += `{
					"id": ` + itoa(101+i) + `,
					"status": 4,
					"type": "tv",
					"media": {"tmdbId": ` + itoa(2000+i) + `, "mediaType": "tv"},
					"requestedBy": {"displayName": "User2", "username": "user2"}
				}`
			}
			results += `]`
			page2 := `{"pageInfo": {"pages": 2, "page": 2, "results": 150}, ` + results + `}`
			_ = json.NewEncoder(w).Encode(json.RawMessage(page2))
		}
	}))
	defer srv.Close()

	client := NewSeerrClient(srv.URL, testTautulliAPIKey)
	requests, err := client.GetRequestedMedia()
	if err != nil {
		t.Fatalf("GetRequestedMedia should succeed: %v", err)
	}
	if len(requests) != 150 {
		t.Errorf("Expected 150 total requests after pagination, got %d", len(requests))
	}
	if callCount != 2 {
		t.Errorf("Expected 2 API calls for pagination, got %d", callCount)
	}
}

func TestSeerrClient_GetRequestedMedia_EmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"pageInfo": {"pages": 0, "page": 1, "results": 0},
			"results": []
		}`))
	}))
	defer srv.Close()

	client := NewSeerrClient(srv.URL, testTautulliAPIKey)
	requests, err := client.GetRequestedMedia()
	if err != nil {
		t.Fatalf("GetRequestedMedia should succeed with empty: %v", err)
	}
	if len(requests) != 0 {
		t.Errorf("Expected 0 requests, got %d", len(requests))
	}
}

func TestSeerrClient_GetRequestedMedia_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{broken`))
	}))
	defer srv.Close()

	client := NewSeerrClient(srv.URL, testTautulliAPIKey)
	_, err := client.GetRequestedMedia()
	if err == nil {
		t.Fatal("Expected error for malformed JSON")
	}
}

func TestSeerrClient_URLTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/me" {
			t.Errorf("Expected /api/v1/auth/me, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1,"displayName":"Admin"}`))
	}))
	defer srv.Close()

	client := NewSeerrClient(srv.URL+"/", testTautulliAPIKey)
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should succeed: %v", err)
	}
}

// itoa is a simple int-to-string helper for test JSON building.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
