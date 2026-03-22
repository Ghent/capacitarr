package integrations

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

const testJellyfinPathItems = "/Users/admin-1/Items"

func TestJellyfinClient_TestConnection_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/System/Info" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Emby-Token") != testTautulliAPIKey {
			t.Errorf("Missing or wrong API key header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ServerName":"My Jellyfin","Version":"10.8.0"}`))
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should succeed: %v", err)
	}
}

func TestJellyfinClient_TestConnection_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, "bad-key")
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 401")
	}
}

func TestJellyfinClient_TestConnection_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 500")
	}
}

func TestJellyfinClient_TestConnection_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Response with empty server info — both ServerName and Version are ""
		_, _ = w.Write([]byte(`{"ServerName":"","Version":""}`))
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail when both ServerName and Version are empty")
	}
}

func TestJellyfinClient_TestConnection_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with malformed JSON")
	}
}

func TestJellyfinClient_GetAdminUserID_Admin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Users" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[
				{"Id":"user-1","Name":"regular","Policy":{"IsAdministrator":false}},
				{"Id":"admin-1","Name":"admin","Policy":{"IsAdministrator":true}},
				{"Id":"user-2","Name":"kid","Policy":{"IsAdministrator":false}}
			]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	userID, err := client.GetAdminUserID()
	if err != nil {
		t.Fatalf("GetAdminUserID should succeed: %v", err)
	}
	if userID != "admin-1" {
		t.Errorf("Expected admin user ID 'admin-1', got %q", userID)
	}
}

func TestJellyfinClient_GetAdminUserID_FallbackToFirstUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Users" {
			w.Header().Set("Content-Type", "application/json")
			// No admin users — should fall back to first user
			_, _ = w.Write([]byte(`[
				{"Id":"user-1","Name":"regular","Policy":{"IsAdministrator":false}},
				{"Id":"user-2","Name":"kid","Policy":{"IsAdministrator":false}}
			]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	userID, err := client.GetAdminUserID()
	if err != nil {
		t.Fatalf("GetAdminUserID should fall back to first user: %v", err)
	}
	if userID != "user-1" {
		t.Errorf("Expected fallback to first user 'user-1', got %q", userID)
	}
}

func TestJellyfinClient_GetAdminUserID_NoUsers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Users" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	_, err := client.GetAdminUserID()
	if err == nil {
		t.Fatal("GetAdminUserID should fail with no users")
	}
}

func TestJellyfinClient_GetBulkWatchDataForUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testJellyfinPathItems {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"Items": [
					{
						"Id":"item-1",
						"Name":"Serenity",
						"Type":"Movie",
						"ProviderIds":{"Tmdb":"16320"},
						"UserData":{
							"PlayCount":3,
							"LastPlayedDate":"2024-01-15T20:30:00Z",
							"Played":true
						}
					},
					{
						"Id":"item-2",
						"Name":"Serenity 2",
						"Type":"Movie",
						"ProviderIds":{"Tmdb":"99999"},
						"UserData":{
							"PlayCount":1,
							"LastPlayedDate":"2024-02-20T15:00:00Z",
							"Played":true
						}
					},
					{
						"Id":"item-3",
						"Name":"New Movie",
						"Type":"Movie",
						"ProviderIds":{"Tmdb":"55555"},
						"UserData":{
							"PlayCount":0,
							"LastPlayedDate":"",
							"Played":false
						}
					}
				],
				"TotalRecordCount": 3
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	data, err := client.GetBulkWatchDataForUser("admin-1", "admin")
	if err != nil {
		t.Fatalf("GetBulkWatchDataForUser should succeed: %v", err)
	}

	if len(data) != 3 {
		t.Fatalf("Expected 3 items, got %d", len(data))
	}

	// Items are keyed by TMDb ID
	movie, ok := data[16320]
	if !ok {
		t.Fatal("Expected TMDb ID 16320 key in data map")
	}
	if movie.PlayCount != 3 {
		t.Errorf("Expected PlayCount 3, got %d", movie.PlayCount)
	}
	if movie.LastPlayed == nil {
		t.Error("Expected LastPlayed to be set")
	}
	// Watched item should track the user
	if len(movie.Users) != 1 || movie.Users[0] != "admin" {
		t.Errorf("Expected Users=[admin], got %v", movie.Users)
	}

	// Unwatched item
	newMovie, ok := data[55555]
	if !ok {
		t.Fatal("Expected TMDb ID 55555 key in data map")
	}
	if newMovie.PlayCount != 0 {
		t.Errorf("Expected PlayCount 0, got %d", newMovie.PlayCount)
	}
	if newMovie.LastPlayed != nil {
		t.Error("Expected nil LastPlayed for unwatched item")
	}
	// Unwatched items should not track the user
	if len(newMovie.Users) != 0 {
		t.Errorf("Expected no Users for unwatched item, got %v", newMovie.Users)
	}
}

func TestJellyfinClient_GetBulkWatchDataForUser_DuplicateTMDbKeepsHigherPlayCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testJellyfinPathItems {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"Items": [
					{
						"Id":"item-1",
						"Name":"Serenity",
						"Type":"Movie",
						"ProviderIds":{"Tmdb":"16320"},
						"UserData":{"PlayCount":2,"LastPlayedDate":"","Played":true}
					},
					{
						"Id":"item-2",
						"Name":"Serenity (Special Edition)",
						"Type":"Movie",
						"ProviderIds":{"Tmdb":"16320"},
						"UserData":{"PlayCount":5,"LastPlayedDate":"","Played":true}
					}
				],
				"TotalRecordCount": 2
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	data, err := client.GetBulkWatchDataForUser("admin-1", "admin")
	if err != nil {
		t.Fatalf("GetBulkWatchDataForUser should succeed: %v", err)
	}

	// Should keep the entry with higher play count
	movie, ok := data[16320]
	if !ok {
		t.Fatal("Expected TMDb ID 16320 key in data map")
	}
	if movie.PlayCount != 5 {
		t.Errorf("Expected higher PlayCount 5 for duplicate, got %d", movie.PlayCount)
	}
}

func TestJellyfinClient_GetBulkWatchDataForUser_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testJellyfinPathItems {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Items":[],"TotalRecordCount":0}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	data, err := client.GetBulkWatchDataForUser("admin-1", "admin")
	if err != nil {
		t.Fatalf("GetBulkWatchDataForUser should succeed with empty: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("Expected 0 items, got %d", len(data))
	}
}

func TestJellyfinClient_GetBulkWatchDataForUser_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testJellyfinPathItems {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{broken`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	_, err := client.GetBulkWatchDataForUser("admin-1", "admin")
	if err == nil {
		t.Fatal("Expected error for malformed JSON response")
	}
}

func TestJellyfinClient_GetBulkWatchDataForUser_SkipsMissingTMDbID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testJellyfinPathItems {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"Items": [
					{"Id":"1","Name":"No Provider IDs","Type":"Movie","UserData":{"PlayCount":1,"LastPlayedDate":"","Played":true}},
					{"Id":"2","Name":"Empty TMDb","Type":"Movie","ProviderIds":{"Imdb":"tt1234567"},"UserData":{"PlayCount":2,"LastPlayedDate":"","Played":true}},
					{"Id":"3","Name":"Serenity","Type":"Movie","ProviderIds":{"Tmdb":"16320"},"UserData":{"PlayCount":3,"LastPlayedDate":"","Played":true}}
				],
				"TotalRecordCount": 3
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	data, err := client.GetBulkWatchDataForUser("admin-1", "admin")
	if err != nil {
		t.Fatalf("GetBulkWatchDataForUser should succeed: %v", err)
	}

	// Only "Serenity" with a valid TMDb ID should be in the result
	if len(data) != 1 {
		t.Errorf("Expected 1 valid item, got %d", len(data))
	}
	if _, ok := data[16320]; !ok {
		t.Error("Expected TMDb ID 16320 key in data map")
	}
}

func TestJellyfinClient_URLTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/System/Info" {
			t.Errorf("Expected /System/Info, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ServerName":"Test","Version":"10.8.0"}`))
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL+"/", testTautulliAPIKey)
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should handle trailing slash: %v", err)
	}
}

func TestJellyfinClient_GetFavoritedItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testJellyfinPathItems {
			// Verify the IsFavorite=true query param is present
			if r.URL.Query().Get("IsFavorite") != "true" {
				t.Errorf("Expected IsFavorite=true query param, got %q", r.URL.Query().Get("IsFavorite"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"Items": [
					{"Id":"item-1","Name":"Serenity","Type":"Movie","ProviderIds":{"Tmdb":"16320"}},
					{"Id":"item-2","Name":"Firefly","Type":"Series","ProviderIds":{"Tmdb":"1437"}}
				],
				"TotalRecordCount": 2
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	favs, err := client.GetFavoritedItems("admin-1")
	if err != nil {
		t.Fatalf("GetFavoritedItems should succeed: %v", err)
	}
	if len(favs) != 2 {
		t.Fatalf("Expected 2 favorited items, got %d", len(favs))
	}
	if !favs[16320] {
		t.Error("Expected TMDb ID 16320 (Serenity) in favorites map")
	}
	if !favs[1437] {
		t.Error("Expected TMDb ID 1437 (Firefly) in favorites map")
	}
}

func TestJellyfinClient_GetFavoritedItems_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testJellyfinPathItems {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Items":[],"TotalRecordCount":0}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	favs, err := client.GetFavoritedItems("admin-1")
	if err != nil {
		t.Fatalf("GetFavoritedItems should succeed with empty: %v", err)
	}
	if len(favs) != 0 {
		t.Errorf("Expected 0 favorites, got %d", len(favs))
	}
}

func TestJellyfinClient_GetFavoritedItems_SkipsMissingTMDbID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testJellyfinPathItems {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"Items": [
					{"Id":"1","Name":"No IDs","Type":"Movie"},
					{"Id":"2","Name":"Only IMDB","Type":"Movie","ProviderIds":{"Imdb":"tt1234567"}},
					{"Id":"3","Name":"Serenity","Type":"Movie","ProviderIds":{"Tmdb":"16320"}}
				],
				"TotalRecordCount": 3
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	favs, err := client.GetFavoritedItems("admin-1")
	if err != nil {
		t.Fatalf("GetFavoritedItems should succeed: %v", err)
	}
	if len(favs) != 1 {
		t.Fatalf("Expected 1 favorite (missing TMDb IDs skipped), got %d", len(favs))
	}
	if !favs[16320] {
		t.Error("Expected TMDb ID 16320 in favorites map")
	}
}

func TestJellyfinClient_GetFavoritedItems_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	_, err := client.GetFavoritedItems("admin-1")
	if err == nil {
		t.Fatal("Expected error for API failure")
	}
}

func TestExtractTMDbID(t *testing.T) {
	tests := []struct {
		name        string
		providerIDs map[string]string
		want        int
	}{
		{"valid TMDb ID", map[string]string{"Tmdb": "16320"}, 16320},
		{"missing TMDb key", map[string]string{"Imdb": "tt1234567"}, 0},
		{"empty TMDb value", map[string]string{"Tmdb": ""}, 0},
		{"invalid TMDb value", map[string]string{"Tmdb": "notanumber"}, 0},
		{"nil map", nil, 0},
		{"empty map", map[string]string{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTMDbID(tt.providerIDs)
			if got != tt.want {
				t.Errorf("extractTMDbID(%v) = %d, want %d", tt.providerIDs, got, tt.want)
			}
		})
	}
}

func TestJellyfinClient_GetBulkWatchData_MultiUserAggregation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/Users":
			_, _ = w.Write([]byte(`[
				{"Id":"user-1","Name":"Alice","Policy":{"IsAdministrator":true}},
				{"Id":"user-2","Name":"Bob","Policy":{"IsAdministrator":false}}
			]`))
		case "/Users/user-1/Items":
			_, _ = w.Write([]byte(`{
				"Items": [
					{
						"Id":"item-1","Name":"Serenity","Type":"Movie",
						"ProviderIds":{"Tmdb":"16320"},
						"UserData":{"PlayCount":3,"LastPlayedDate":"2024-01-15T20:30:00Z","Played":true}
					}
				],
				"TotalRecordCount": 1
			}`))
		case "/Users/user-2/Items":
			_, _ = w.Write([]byte(`{
				"Items": [
					{
						"Id":"item-1","Name":"Serenity","Type":"Movie",
						"ProviderIds":{"Tmdb":"16320"},
						"UserData":{"PlayCount":2,"LastPlayedDate":"2024-02-20T15:00:00Z","Played":true}
					}
				],
				"TotalRecordCount": 1
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	data, err := client.GetBulkWatchData()
	if err != nil {
		t.Fatalf("GetBulkWatchData should succeed: %v", err)
	}

	movie, ok := data[16320]
	if !ok {
		t.Fatal("Expected TMDb ID 16320 in aggregated data")
	}
	// Play counts are summed across users: 3 + 2 = 5
	if movie.PlayCount != 5 {
		t.Errorf("Expected aggregated PlayCount 5, got %d", movie.PlayCount)
	}
	// LastPlayed should be the most recent (Bob's 2024-02-20)
	if movie.LastPlayed == nil {
		t.Fatal("Expected LastPlayed to be set")
	}
	if movie.LastPlayed.Month() != 2 || movie.LastPlayed.Day() != 20 {
		t.Errorf("Expected most recent LastPlayed (Feb 20), got %v", movie.LastPlayed)
	}
	// Users should contain both
	if len(movie.Users) != 2 {
		t.Fatalf("Expected 2 users, got %d: %v", len(movie.Users), movie.Users)
	}
}

func TestJellyfinClient_GetWatchlistItems_MultiUserUnion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/Users":
			_, _ = w.Write([]byte(`[
				{"Id":"user-1","Name":"Alice","Policy":{"IsAdministrator":true}},
				{"Id":"user-2","Name":"Bob","Policy":{"IsAdministrator":false}}
			]`))
		case "/Users/user-1/Items":
			_, _ = w.Write([]byte(`{
				"Items": [
					{"Id":"item-1","Name":"Serenity","Type":"Movie","ProviderIds":{"Tmdb":"16320"}}
				],
				"TotalRecordCount": 1
			}`))
		case "/Users/user-2/Items":
			_, _ = w.Write([]byte(`{
				"Items": [
					{"Id":"item-2","Name":"Firefly","Type":"Series","ProviderIds":{"Tmdb":"1437"}}
				],
				"TotalRecordCount": 1
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	favs, err := client.GetWatchlistItems()
	if err != nil {
		t.Fatalf("GetWatchlistItems should succeed: %v", err)
	}
	// Union of both users' favorites
	if len(favs) != 2 {
		t.Fatalf("Expected 2 watchlist items (union), got %d", len(favs))
	}
	if !favs[16320] {
		t.Error("Expected TMDb ID 16320 (Serenity) in watchlist")
	}
	if !favs[1437] {
		t.Error("Expected TMDb ID 1437 (Firefly) in watchlist")
	}
}
