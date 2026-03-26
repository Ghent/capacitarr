package integrations

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testTracearrAPIKey = "test-key"

func TestTracearrClient_TestConnection_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/public/health" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		// Verify Bearer token auth
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+testTracearrAPIKey {
			t.Errorf("Expected Bearer auth, got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","version":"1.5.0"}`))
	}))
	defer srv.Close()

	client := NewTracearrClient(srv.URL, testTracearrAPIKey)
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should succeed: %v", err)
	}
}

func TestTracearrClient_TestConnection_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewTracearrClient(srv.URL, "bad-key")
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 401")
	}
	if !strings.Contains(err.Error(), "trr_pub_") {
		t.Errorf("Error should mention trr_pub_ prefix, got: %v", err)
	}
}

func TestTracearrClient_TestConnection_InvalidURL(t *testing.T) {
	client := NewTracearrClient("http://127.0.0.1:1", testTracearrAPIKey)
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with connection refused")
	}
}

func TestTracearrClient_GetWatchHistory_Success(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/public/history" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		callCount++
		w.Header().Set("Content-Type", "application/json")

		switch mediaType := r.URL.Query().Get("mediaType"); mediaType {
		case "movie":
			_, _ = w.Write([]byte(`{
				"data": [
					{
						"mediaTitle": "Serenity",
						"showTitle": "",
						"mediaType": "movie",
						"year": 2005,
						"watched": true,
						"durationMs": 7200000,
						"user": {"username": "mal"}
					},
					{
						"mediaTitle": "Serenity",
						"showTitle": "",
						"mediaType": "movie",
						"year": 2005,
						"watched": true,
						"durationMs": 7200000,
						"user": {"username": "wash"}
					}
				],
				"pagination": {"page": 1, "pageSize": 100, "total": 2}
			}`))
		case "episode":
			_, _ = w.Write([]byte(`{
				"data": [
					{
						"mediaTitle": "Out of Gas",
						"showTitle": "Firefly",
						"mediaType": "episode",
						"year": 2002,
						"watched": true,
						"durationMs": 2700000,
						"user": {"username": "kaylee"}
					}
				],
				"pagination": {"page": 1, "pageSize": 100, "total": 1}
			}`))
		}
	}))
	defer srv.Close()

	client := NewTracearrClient(srv.URL, testTracearrAPIKey)
	history, err := client.GetWatchHistory()
	if err != nil {
		t.Fatalf("GetWatchHistory should succeed: %v", err)
	}

	if len(history) != 3 {
		t.Fatalf("Expected 3 history items (2 movies + 1 episode), got %d", len(history))
	}

	// Verify movie items
	movieCount := 0
	for _, item := range history {
		if item.MediaType == "movie" && item.MediaTitle == "Serenity" {
			movieCount++
		}
	}
	if movieCount != 2 {
		t.Errorf("Expected 2 Serenity movie sessions, got %d", movieCount)
	}

	// Verify episode item
	episodeFound := false
	for _, item := range history {
		if item.MediaType == "episode" && item.ShowTitle == "Firefly" {
			episodeFound = true
		}
	}
	if !episodeFound {
		t.Error("Expected Firefly episode session")
	}
}

func TestTracearrClient_GetWatchHistory_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": [], "pagination": {"page": 1, "pageSize": 100, "total": 0}}`))
	}))
	defer srv.Close()

	client := NewTracearrClient(srv.URL, testTracearrAPIKey)
	history, err := client.GetWatchHistory()
	if err != nil {
		t.Fatalf("GetWatchHistory should succeed: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("Expected 0 history items, got %d", len(history))
	}
}

func TestTracearrClient_GetWatchHistory_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()

	client := NewTracearrClient(srv.URL, testTracearrAPIKey)
	_, err := client.GetWatchHistory()
	if err == nil {
		t.Fatal("GetWatchHistory should fail with malformed JSON")
	}
}
