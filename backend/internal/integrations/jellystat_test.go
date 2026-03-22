package integrations

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testJellystatToken = "test-jellystat-jwt-token" //nolint:gosec // G101: test fixture, not a real credential

// ─── JellystatClient.TestConnection ─────────────────────────────────────────

func TestJellystatClient_TestConnection_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/getLibraries" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		// Verify Bearer token auth
		auth := r.Header.Get("Authorization")
		if auth != "Bearer "+testJellystatToken {
			t.Errorf("Expected Bearer auth, got %q", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"Id":"lib1","Name":"Movies"},{"Id":"lib2","Name":"Shows"}]`))
	}))
	defer srv.Close()

	client := NewJellystatClient(srv.URL, testJellystatToken)
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should succeed: %v", err)
	}
}

func TestJellystatClient_TestConnection_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewJellystatClient(srv.URL, "expired-token")
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 401")
	}
	if !strings.Contains(err.Error(), "JWT token may be expired") {
		t.Errorf("Error should mention JWT expiry, got: %v", err)
	}
}

func TestJellystatClient_TestConnection_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewJellystatClient(srv.URL, testJellystatToken)
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 500")
	}
}

func TestJellystatClient_TestConnection_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	client := NewJellystatClient(srv.URL, testJellystatToken)
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with malformed JSON")
	}
}

// ─── JellystatClient.GetBulkWatchStats ──────────────────────────────────────

func TestJellystatClient_GetBulkWatchStats_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/getItemsWithStats" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"Id": "jf-item-1",
				"Name": "Serenity",
				"total_play_count": 5,
				"total_played": "2026-01-15T20:00:00Z",
				"Users": [
					{"UserName": "alice", "play_count": 3},
					{"UserName": "bob", "play_count": 2}
				]
			},
			{
				"Id": "jf-item-2",
				"Name": "Firefly",
				"total_play_count": 12,
				"total_played": "2026-03-01T12:00:00Z",
				"Users": [
					{"UserName": "alice", "play_count": 7},
					{"UserName": "charlie", "play_count": 5}
				]
			},
			{
				"Id": "jf-item-3",
				"Name": "Unknown Item",
				"total_play_count": 1,
				"total_played": "2026-02-01T10:00:00Z",
				"Users": [
					{"UserName": "alice", "play_count": 1}
				]
			}
		]`))
	}))
	defer srv.Close()

	client := NewJellystatClient(srv.URL, testJellystatToken)

	// Build Jellyfin ID → TMDb ID map (jf-item-3 has no mapping)
	jellyfinIDToTMDb := map[string]int{
		"jf-item-1": 16320, // Serenity TMDb ID
		"jf-item-2": 1437,  // Firefly TMDb ID
	}

	result, err := client.GetBulkWatchStats(jellyfinIDToTMDb)
	if err != nil {
		t.Fatalf("GetBulkWatchStats should succeed: %v", err)
	}

	// Should have 2 items (jf-item-3 skipped — no mapping)
	if len(result) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(result))
	}

	// Check Serenity
	serenity, ok := result[16320]
	if !ok {
		t.Fatal("Expected Serenity (TMDb 16320) in results")
	}
	if serenity.PlayCount != 5 {
		t.Errorf("Serenity play count: expected 5, got %d", serenity.PlayCount)
	}
	if serenity.LastPlayed == nil {
		t.Fatal("Serenity LastPlayed should not be nil")
	}
	expectedTime, _ := time.Parse(time.RFC3339, "2026-01-15T20:00:00Z")
	if !serenity.LastPlayed.Equal(expectedTime) {
		t.Errorf("Serenity LastPlayed: expected %v, got %v", expectedTime, *serenity.LastPlayed)
	}
	if len(serenity.Users) != 2 {
		t.Errorf("Serenity users: expected 2, got %d", len(serenity.Users))
	}

	// Check Firefly
	firefly, ok := result[1437]
	if !ok {
		t.Fatal("Expected Firefly (TMDb 1437) in results")
	}
	if firefly.PlayCount != 12 {
		t.Errorf("Firefly play count: expected 12, got %d", firefly.PlayCount)
	}
	if len(firefly.Users) != 2 {
		t.Errorf("Firefly users: expected 2, got %d", len(firefly.Users))
	}
}

func TestJellystatClient_GetBulkWatchStats_EmptyLibrary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	client := NewJellystatClient(srv.URL, testJellystatToken)
	result, err := client.GetBulkWatchStats(map[string]int{"jf-1": 12345})
	if err != nil {
		t.Fatalf("GetBulkWatchStats should succeed with empty library: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("Expected 0 items, got %d", len(result))
	}
}

func TestJellystatClient_GetBulkWatchStats_EmptyMap(t *testing.T) {
	// Should short-circuit and not even call the API
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Fatal("API should not be called when map is empty")
	}))
	defer srv.Close()

	client := NewJellystatClient(srv.URL, testJellystatToken)
	result, err := client.GetBulkWatchStats(map[string]int{})
	if err != nil {
		t.Fatalf("GetBulkWatchStats should succeed with empty map: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("Expected 0 items, got %d", len(result))
	}
}

func TestJellystatClient_GetBulkWatchStats_ZeroPlayCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"Id": "jf-item-1",
				"Name": "Serenity",
				"total_play_count": 0,
				"total_played": "",
				"Users": []
			}
		]`))
	}))
	defer srv.Close()

	client := NewJellystatClient(srv.URL, testJellystatToken)
	result, err := client.GetBulkWatchStats(map[string]int{"jf-item-1": 16320})
	if err != nil {
		t.Fatalf("GetBulkWatchStats should succeed: %v", err)
	}
	// Items with zero play count should be skipped
	if len(result) != 0 {
		t.Errorf("Expected 0 items (zero play count skipped), got %d", len(result))
	}
}

func TestJellystatClient_BearerAuthHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("Expected Bearer auth prefix, got %q", auth)
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		if token != testJellystatToken {
			t.Errorf("Expected token %q, got %q", testJellystatToken, token)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	client := NewJellystatClient(srv.URL, testJellystatToken)
	_ = client.TestConnection()
}

// ─── JellystatEnricher ──────────────────────────────────────────────────────

func TestJellystatEnricher_Priority(t *testing.T) {
	enricher := NewJellystatEnricher(nil, nil)
	if enricher.Priority() != 10 {
		t.Errorf("Expected priority 10, got %d", enricher.Priority())
	}
}

func TestJellystatEnricher_Name(t *testing.T) {
	enricher := NewJellystatEnricher(nil, nil)
	if enricher.Name() != "Jellystat Watch History" {
		t.Errorf("Expected 'Jellystat Watch History', got %q", enricher.Name())
	}
}

func TestJellystatEnricher_MatchesByTMDbID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"Id": "jf-item-1",
				"Name": "Serenity",
				"total_play_count": 5,
				"total_played": "2026-01-15T20:00:00Z",
				"Users": [{"UserName": "alice", "play_count": 5}]
			}
		]`))
	}))
	defer srv.Close()

	client := NewJellystatClient(srv.URL, testJellystatToken)
	jellyfinMap := map[string]int{"jf-item-1": 16320}
	enricher := NewJellystatEnricher(client, jellyfinMap)

	items := []MediaItem{
		{Title: "Serenity", TMDbID: 16320},
		{Title: "Firefly", TMDbID: 1437}, // No match
		{Title: "No TMDb", TMDbID: 0},    // Skipped
	}

	if err := enricher.Enrich(items); err != nil {
		t.Fatalf("Enrich should succeed: %v", err)
	}

	// Serenity should be enriched
	if items[0].PlayCount != 5 {
		t.Errorf("Serenity play count: expected 5, got %d", items[0].PlayCount)
	}
	if len(items[0].WatchedByUsers) != 1 || items[0].WatchedByUsers[0] != "alice" {
		t.Errorf("Serenity watched by users: expected [alice], got %v", items[0].WatchedByUsers)
	}

	// Firefly should NOT be enriched
	if items[1].PlayCount != 0 {
		t.Errorf("Firefly play count: expected 0 (unmatched), got %d", items[1].PlayCount)
	}

	// No TMDb should NOT be enriched
	if items[2].PlayCount != 0 {
		t.Errorf("No TMDb play count: expected 0 (skipped), got %d", items[2].PlayCount)
	}
}

func TestJellystatEnricher_EmptyMap_SkipsGracefully(t *testing.T) {
	enricher := NewJellystatEnricher(nil, map[string]int{})
	items := []MediaItem{
		{Title: "Serenity", TMDbID: 16320},
	}
	if err := enricher.Enrich(items); err != nil {
		t.Fatalf("Enrich should succeed with empty map: %v", err)
	}
	if items[0].PlayCount != 0 {
		t.Errorf("Expected 0 play count with empty map, got %d", items[0].PlayCount)
	}
}

func TestJellystatEnricher_CompileTimeInterfaceCheck(_ *testing.T) {
	var _ Enricher = (*JellystatEnricher)(nil)
}

func TestJellystatClient_CompileTimeInterfaceCheck(_ *testing.T) {
	var _ Connectable = (*JellystatClient)(nil)
}
