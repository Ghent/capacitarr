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
		if r.URL.Path != "/api/stats/dashboard" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		// Verify Bearer token auth
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+testTracearrAPIKey {
			t.Errorf("Expected Bearer auth, got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total_plays": 42, "total_watch_time_ms": 123456}`))
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

func TestTracearrClient_GetTopContent_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/stats/content/top-content" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		period := r.URL.Query().Get("period")
		if period != "all" {
			t.Errorf("Expected period 'all', got %q", period)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"movies": [
				{
					"media_title": "Serenity",
					"year": 2005,
					"play_count": 15,
					"total_watch_ms": 7200000,
					"server_id": "srv-1",
					"rating_key": "12345"
				}
			],
			"shows": [
				{
					"grandparent_title": "Firefly",
					"year": 2002,
					"play_count": 42,
					"total_watch_ms": 36000000,
					"server_id": "srv-1",
					"rating_key": "67890"
				}
			]
		}`))
	}))
	defer srv.Close()

	client := NewTracearrClient(srv.URL, testTracearrAPIKey)
	content, err := client.GetTopContent("all")
	if err != nil {
		t.Fatalf("GetTopContent should succeed: %v", err)
	}

	if len(content.Movies) != 1 {
		t.Fatalf("Expected 1 movie, got %d", len(content.Movies))
	}
	if content.Movies[0].MediaTitle != "Serenity" {
		t.Errorf("Expected movie 'Serenity', got %q", content.Movies[0].MediaTitle)
	}
	if content.Movies[0].PlayCount != 15 {
		t.Errorf("Expected play count 15, got %d", content.Movies[0].PlayCount)
	}
	if content.Movies[0].RatingKey != "12345" {
		t.Errorf("Expected rating key '12345', got %q", content.Movies[0].RatingKey)
	}

	if len(content.Shows) != 1 {
		t.Fatalf("Expected 1 show, got %d", len(content.Shows))
	}
	if content.Shows[0].GrandparentTitle != "Firefly" {
		t.Errorf("Expected show 'Firefly', got %q", content.Shows[0].GrandparentTitle)
	}
	if content.Shows[0].PlayCount != 42 {
		t.Errorf("Expected play count 42, got %d", content.Shows[0].PlayCount)
	}
	if content.Shows[0].RatingKey != "67890" {
		t.Errorf("Expected rating key '67890', got %q", content.Shows[0].RatingKey)
	}
}

func TestTracearrClient_GetTopContent_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"movies": [], "shows": []}`))
	}))
	defer srv.Close()

	client := NewTracearrClient(srv.URL, testTracearrAPIKey)
	content, err := client.GetTopContent("all")
	if err != nil {
		t.Fatalf("GetTopContent should succeed: %v", err)
	}
	if len(content.Movies) != 0 {
		t.Errorf("Expected 0 movies, got %d", len(content.Movies))
	}
	if len(content.Shows) != 0 {
		t.Errorf("Expected 0 shows, got %d", len(content.Shows))
	}
}

func TestTracearrClient_GetTopContent_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not valid json`))
	}))
	defer srv.Close()

	client := NewTracearrClient(srv.URL, testTracearrAPIKey)
	_, err := client.GetTopContent("all")
	if err == nil {
		t.Fatal("GetTopContent should fail with malformed JSON")
	}
}
