package integrations

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const (
	testEmbyPathSystemInfo = "/System/Info"
	testEmbyPathUsers      = "/Users"
)

func TestEmbyClient_TestConnection_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testEmbyPathSystemInfo {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Emby-Token") != testTautulliAPIKey {
			t.Errorf("Missing or wrong API key header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ServerName":"My Emby","Version":"4.7.0"}`))
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should succeed: %v", err)
	}
}

func TestEmbyClient_TestConnection_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, "bad-key")
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 401")
	}
}

func TestEmbyClient_TestConnection_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 500")
	}
}

func TestEmbyClient_TestConnection_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ServerName":"","Version":""}`))
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail when both ServerName and Version are empty")
	}
}

func TestEmbyClient_TestConnection_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with malformed JSON")
	}
}

func TestEmbyClient_GetBulkWatchDataForUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// The endpoint should be /Users/{userID}/Items with query params
		if r.URL.Path != "/Users/admin1/Items" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"Items": [
				{
					"Name": "Serenity",
					"ProviderIds": {"Tmdb": "16320"},
					"UserData": {
						"PlayCount": 5,
						"LastPlayedDate": "2024-11-01T20:00:00Z",
						"Played": true
					}
				},
				{
					"Name": "Serenity 2",
					"ProviderIds": {"Tmdb": "99999"},
					"UserData": {
						"PlayCount": 2,
						"LastPlayedDate": "2024-10-15T14:00:00Z",
						"Played": true
					}
				},
				{
					"Name": "Unwatched Movie",
					"ProviderIds": {"Tmdb": "55555"},
					"UserData": {
						"PlayCount": 0,
						"LastPlayedDate": "",
						"Played": false
					}
				}
			],
			"TotalRecordCount": 3
		}`))
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	data, err := client.GetBulkWatchDataForUser("admin1", "AdminUser")
	if err != nil {
		t.Fatalf("GetBulkWatchDataForUser should succeed: %v", err)
	}
	if len(data) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(data))
	}

	// Check Serenity (keyed by TMDb ID)
	movie, ok := data[16320]
	if !ok {
		t.Fatal("Expected TMDb ID 16320 in result")
	}
	if movie.PlayCount != 5 {
		t.Errorf("Expected PlayCount 5 for Serenity, got %d", movie.PlayCount)
	}
	if movie.LastPlayed == nil {
		t.Error("Expected non-nil LastPlayed for Serenity")
	}
	// Watched item should track the user
	if len(movie.Users) != 1 || movie.Users[0] != "AdminUser" {
		t.Errorf("Expected Users=[AdminUser], got %v", movie.Users)
	}

	// Check unwatched movie
	unwatched, ok := data[55555]
	if !ok {
		t.Fatal("Expected TMDb ID 55555 in result")
	}
	if unwatched.PlayCount != 0 {
		t.Errorf("Expected PlayCount 0 for unwatched, got %d", unwatched.PlayCount)
	}
	if unwatched.LastPlayed != nil {
		t.Error("Expected nil LastPlayed for unwatched movie")
	}
}

func TestEmbyClient_GetBulkWatchDataForUser_Pagination(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount == 1 {
			// First page: return 2 items with TotalRecordCount=3 to trigger pagination
			_, _ = w.Write([]byte(`{
				"Items": [
					{"Name": "Movie A", "ProviderIds": {"Tmdb": "111"}, "UserData": {"PlayCount": 1, "Played": true}},
					{"Name": "Movie B", "ProviderIds": {"Tmdb": "222"}, "UserData": {"PlayCount": 2, "Played": true}}
				],
				"TotalRecordCount": 3
			}`))
		} else {
			// Second page: final item
			_, _ = w.Write([]byte(`{
				"Items": [
					{"Name": "Movie C", "ProviderIds": {"Tmdb": "333"}, "UserData": {"PlayCount": 0, "Played": false}}
				],
				"TotalRecordCount": 3
			}`))
		}
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	data, err := client.GetBulkWatchDataForUser("admin1", "AdminUser")
	if err != nil {
		t.Fatalf("GetBulkWatchDataForUser should succeed: %v", err)
	}
	if len(data) != 3 {
		t.Fatalf("Expected 3 entries after pagination, got %d", len(data))
	}
	if _, ok := data[111]; !ok {
		t.Error("Expected TMDb ID 111 key")
	}
	if _, ok := data[333]; !ok {
		t.Error("Expected TMDb ID 333 key")
	}
}

func TestEmbyClient_GetBulkWatchDataForUser_DuplicateTMDbKeepsHigherPlayCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"Items": [
				{"Name": "Serenity", "ProviderIds": {"Tmdb": "16320"}, "UserData": {"PlayCount": 1, "Played": true}},
				{"Name": "Serenity (Special Edition)", "ProviderIds": {"Tmdb": "16320"}, "UserData": {"PlayCount": 5, "Played": true}}
			],
			"TotalRecordCount": 2
		}`))
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	data, err := client.GetBulkWatchDataForUser("admin1", "AdminUser")
	if err != nil {
		t.Fatalf("GetBulkWatchDataForUser should succeed: %v", err)
	}
	movie, ok := data[16320]
	if !ok {
		t.Fatal("Expected TMDb ID 16320 key")
	}
	if movie.PlayCount != 5 {
		t.Errorf("Expected higher PlayCount 5 to be kept, got %d", movie.PlayCount)
	}
}

func TestEmbyClient_GetBulkWatchDataForUser_EmptyItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Items": [], "TotalRecordCount": 0}`))
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	data, err := client.GetBulkWatchDataForUser("admin1", "AdminUser")
	if err != nil {
		t.Fatalf("GetBulkWatchDataForUser should succeed: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(data))
	}
}

func TestEmbyClient_GetBulkWatchDataForUser_SkipsMissingTMDbID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"Items": [
				{"Name": "No Provider IDs", "UserData": {"PlayCount": 1, "Played": true}},
				{"Name": "Only IMDB", "ProviderIds": {"Imdb": "tt1234567"}, "UserData": {"PlayCount": 2, "Played": true}},
				{"Name": "Serenity", "ProviderIds": {"Tmdb": "16320"}, "UserData": {"PlayCount": 3, "Played": true}}
			],
			"TotalRecordCount": 3
		}`))
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	data, err := client.GetBulkWatchDataForUser("admin1", "AdminUser")
	if err != nil {
		t.Fatalf("GetBulkWatchDataForUser should succeed: %v", err)
	}
	// Items without TMDb IDs should be skipped
	if len(data) != 1 {
		t.Fatalf("Expected 1 entry (missing TMDb IDs skipped), got %d", len(data))
	}
	if _, ok := data[16320]; !ok {
		t.Error("Expected TMDb ID 16320 key")
	}
}

func TestEmbyClient_GetBulkWatchDataForUser_EpisodeAggregation(t *testing.T) {
	// Series has PlayCount=0 at the series level, but episodes have been watched.
	// The episode pass should promote that watch data to the parent series.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		includeTypes := r.URL.Query().Get("IncludeItemTypes")
		switch includeTypes {
		case "Movie,Series":
			_, _ = w.Write([]byte(`{
				"Items": [
					{
						"Id":"series-ff",
						"Name":"Firefly",
						"Type":"Series",
						"ProviderIds":{"Tmdb":"1437"},
						"UserData":{"PlayCount":0,"LastPlayedDate":"","Played":false}
					},
					{
						"Id":"movie-1",
						"Name":"Serenity",
						"Type":"Movie",
						"ProviderIds":{"Tmdb":"16320"},
						"UserData":{"PlayCount":2,"LastPlayedDate":"2024-06-01T10:00:00Z","Played":true}
					}
				],
				"TotalRecordCount": 2
			}`))
		case "Episode":
			_, _ = w.Write([]byte(`{
				"Items": [
					{
						"Id":"ep-1",
						"Name":"Serenity (Pilot)",
						"SeriesId":"series-ff",
						"Type":"Episode",
						"UserData":{"PlayCount":3,"LastPlayedDate":"2024-08-15T20:00:00Z","Played":true}
					},
					{
						"Id":"ep-2",
						"Name":"The Train Job",
						"SeriesId":"series-ff",
						"Type":"Episode",
						"UserData":{"PlayCount":1,"LastPlayedDate":"2024-07-10T18:00:00Z","Played":true}
					}
				],
				"TotalRecordCount": 2
			}`))
		default:
			_, _ = w.Write([]byte(`{"Items":[],"TotalRecordCount":0}`))
		}
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	data, err := client.GetBulkWatchDataForUser("admin1", "AdminUser")
	if err != nil {
		t.Fatalf("GetBulkWatchDataForUser should succeed: %v", err)
	}

	// Series should have episode-level watch data promoted
	series, ok := data[1437]
	if !ok {
		t.Fatal("Expected TMDb ID 1437 (Firefly) in data map")
	}
	if series.PlayCount != 3 {
		t.Errorf("Expected PlayCount 3 (from best episode), got %d", series.PlayCount)
	}
	if series.LastPlayed == nil {
		t.Fatal("Expected LastPlayed to be set from episode data")
	}
	expectedTime, _ := time.Parse(time.RFC3339, "2024-08-15T20:00:00Z")
	if !series.LastPlayed.Equal(expectedTime) {
		t.Errorf("Expected LastPlayed %v, got %v", expectedTime, *series.LastPlayed)
	}
	if len(series.Users) != 1 || series.Users[0] != "AdminUser" {
		t.Errorf("Expected Users=[AdminUser], got %v", series.Users)
	}

	// Movie should be unaffected by episode pass
	movie, ok := data[16320]
	if !ok {
		t.Fatal("Expected TMDb ID 16320 (Serenity) in data map")
	}
	if movie.PlayCount != 2 {
		t.Errorf("Expected PlayCount 2 for movie, got %d", movie.PlayCount)
	}
}

func TestEmbyClient_GetAdminUserID_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testEmbyPathUsers {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"Id": "user-regular-1",
				"Name": "RegularUser",
				"Policy": {"IsAdministrator": false}
			},
			{
				"Id": "user-admin-1",
				"Name": "AdminUser",
				"Policy": {"IsAdministrator": true}
			}
		]`))
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	userID, err := client.GetAdminUserID()
	if err != nil {
		t.Fatalf("GetAdminUserID should succeed: %v", err)
	}
	if userID != "user-admin-1" {
		t.Errorf("Expected admin user ID 'user-admin-1', got %q", userID)
	}
}

func TestEmbyClient_GetAdminUserID_FallsBackToFirstUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"Id": "user-1",
				"Name": "NonAdmin",
				"Policy": {"IsAdministrator": false}
			}
		]`))
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	userID, err := client.GetAdminUserID()
	if err != nil {
		t.Fatalf("GetAdminUserID should succeed: %v", err)
	}
	// Falls back to first user when no admin found
	if userID != "user-1" {
		t.Errorf("Expected fallback user ID 'user-1', got %q", userID)
	}
}

func TestEmbyClient_GetAdminUserID_NoUsers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	_, err := client.GetAdminUserID()
	if err == nil {
		t.Fatal("GetAdminUserID should fail when no users exist")
	}
}

func TestEmbyClient_GetAdminUserID_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	_, err := client.GetAdminUserID()
	if err == nil {
		t.Fatal("GetAdminUserID should fail with malformed JSON")
	}
}

func TestEmbyClient_URLTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testEmbyPathSystemInfo {
			t.Errorf("Expected /System/Info, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ServerName":"Test","Version":"1.0"}`))
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL+"/", testTautulliAPIKey)
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should succeed: %v", err)
	}
}

func TestEmbyClient_GetFavoritedItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Users/admin1/Items" {
			// Verify the IsFavorite=true query param is present
			if r.URL.Query().Get("IsFavorite") != "true" {
				t.Errorf("Expected IsFavorite=true query param, got %q", r.URL.Query().Get("IsFavorite"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"Items": [
					{"Name": "Serenity", "ProviderIds": {"Tmdb": "16320"}},
					{"Name": "Firefly", "ProviderIds": {"Tmdb": "1437"}}
				],
				"TotalRecordCount": 2
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	favs, err := client.GetFavoritedItems("admin1")
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

func TestEmbyClient_GetFavoritedItems_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Users/admin1/Items" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Items":[],"TotalRecordCount":0}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	favs, err := client.GetFavoritedItems("admin1")
	if err != nil {
		t.Fatalf("GetFavoritedItems should succeed with empty: %v", err)
	}
	if len(favs) != 0 {
		t.Errorf("Expected 0 favorites, got %d", len(favs))
	}
}

func TestEmbyClient_GetFavoritedItems_SkipsMissingTMDbID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Users/admin1/Items" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"Items": [
					{"Name": "No IDs"},
					{"Name": "Only IMDB", "ProviderIds": {"Imdb": "tt1234567"}},
					{"Name": "Serenity", "ProviderIds": {"Tmdb": "16320"}}
				],
				"TotalRecordCount": 3
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	favs, err := client.GetFavoritedItems("admin1")
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

func TestEmbyClient_GetFavoritedItems_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	_, err := client.GetFavoritedItems("admin1")
	if err == nil {
		t.Fatal("Expected error for API failure")
	}
}

func TestEmbyClient_GetBulkWatchData_MultiUserAggregation(t *testing.T) {
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
						"Name":"Serenity",
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
						"Name":"Serenity",
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

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
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

func TestEmbyClient_GetWatchlistItems_MultiUserUnion(t *testing.T) {
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
					{"Name":"Serenity","ProviderIds":{"Tmdb":"16320"}}
				],
				"TotalRecordCount": 1
			}`))
		case "/Users/user-2/Items":
			_, _ = w.Write([]byte(`{
				"Items": [
					{"Name":"Firefly","ProviderIds":{"Tmdb":"1437"}}
				],
				"TotalRecordCount": 1
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
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

func TestEmbyClient_GetCollectionNames_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case testEmbyPathUsers:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"Id":"admin-1","Name":"admin","Policy":{"IsAdministrator":true}}]`))
		case "/Users/admin-1/Items":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Items":[
				{"Name":"Sci-Fi Classics"},
				{"Name":"Joss Whedon Collection"},
				{"Name":"Sci-Fi Classics"},
				{"Name":"  Space Westerns  "}
			]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, "test-key")
	names, err := client.GetCollectionNames()
	if err != nil {
		t.Fatalf("GetCollectionNames should succeed: %v", err)
	}

	// Should be deduplicated and sorted
	expected := []string{"Joss Whedon Collection", "Sci-Fi Classics", "Space Westerns"}
	if len(names) != len(expected) {
		t.Fatalf("Expected %d collection names, got %d: %v", len(expected), len(names), names)
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("Expected names[%d] = %q, got %q", i, expected[i], name)
		}
	}
}

func TestEmbyClient_GetCollectionNames_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case testEmbyPathUsers:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"Id":"admin-1","Name":"admin","Policy":{"IsAdministrator":true}}]`))
		case "/Users/admin-1/Items":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Items":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, "test-key")
	names, err := client.GetCollectionNames()
	if err != nil {
		t.Fatalf("GetCollectionNames should succeed with empty result: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("Expected 0 collection names, got %d", len(names))
	}
}

func TestEmbyClient_GetCollectionNames_NoUsers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testEmbyPathUsers {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, "test-key")
	_, err := client.GetCollectionNames()
	if err == nil {
		t.Fatal("GetCollectionNames should fail when no users exist")
	}
}

func TestEmbyClient_GetCollectionNames_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case testEmbyPathUsers:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"Id":"admin-1","Name":"admin","Policy":{"IsAdministrator":true}}]`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, "test-key")
	_, err := client.GetCollectionNames()
	if err == nil {
		t.Fatal("GetCollectionNames should fail on API error")
	}
}

func TestEmbyClient_GetCollectionNames_SkipsBlankNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case testEmbyPathUsers:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"Id":"admin-1","Name":"admin","Policy":{"IsAdministrator":true}}]`))
		case "/Users/admin-1/Items":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Items":[
				{"Name":"Serenity Collection"},
				{"Name":""},
				{"Name":"   "},
				{"Name":"Firefly Box Set"}
			]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, "test-key")
	names, err := client.GetCollectionNames()
	if err != nil {
		t.Fatalf("GetCollectionNames should succeed: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("Expected 2 collection names (blank skipped), got %d: %v", len(names), names)
	}
	if names[0] != "Firefly Box Set" {
		t.Errorf("Expected names[0] = %q, got %q", "Firefly Box Set", names[0])
	}
	if names[1] != "Serenity Collection" {
		t.Errorf("Expected names[1] = %q, got %q", "Serenity Collection", names[1])
	}
}

// ─── GetItemIDToTMDbIDMap tests ─────────────────────────────────────────────

func TestEmbyClient_GetItemIDToTMDbIDMap_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/Users":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"Id":"admin-1","Name":"admin","Policy":{"IsAdministrator":true}}]`))
		case strings.HasPrefix(r.URL.Path, "/Users/admin-1/Items"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"Items": [
					{"Id": "emby-001", "Name": "Serenity", "Type": "Movie", "ProviderIds": {"Tmdb": "16320"}},
					{"Id": "emby-002", "Name": "Firefly", "Type": "Series", "ProviderIds": {"Tmdb": "1437"}},
					{"Id": "emby-003", "Name": "No TMDb", "Type": "Movie", "ProviderIds": {"Imdb": "tt9999999"}}
				],
				"TotalRecordCount": 3
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, "test-key")
	idMap, err := client.GetItemIDToTMDbIDMap()
	if err != nil {
		t.Fatalf("GetItemIDToTMDbIDMap should succeed: %v", err)
	}

	if len(idMap) != 2 {
		t.Fatalf("Expected 2 mappings (item without TMDb skipped), got %d", len(idMap))
	}
	if idMap["emby-001"] != 16320 {
		t.Errorf("Expected emby-001 → 16320, got %d", idMap["emby-001"])
	}
	if idMap["emby-002"] != 1437 {
		t.Errorf("Expected emby-002 → 1437, got %d", idMap["emby-002"])
	}
}

func TestEmbyClient_GetItemIDToTMDbIDMap_MissingTMDbProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/Users":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"Id":"admin-1","Name":"admin","Policy":{"IsAdministrator":true}}]`))
		case strings.HasPrefix(r.URL.Path, "/Users/admin-1/Items"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"Items": [
					{"Id": "emby-100", "Name": "No Providers", "Type": "Movie", "ProviderIds": {}},
					{"Id": "emby-101", "Name": "Imdb Only", "Type": "Movie", "ProviderIds": {"Imdb": "tt1234567"}}
				],
				"TotalRecordCount": 2
			}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, "test-key")
	idMap, err := client.GetItemIDToTMDbIDMap()
	if err != nil {
		t.Fatalf("GetItemIDToTMDbIDMap should succeed: %v", err)
	}

	if len(idMap) != 0 {
		t.Errorf("Expected 0 mappings (no TMDb provider IDs), got %d", len(idMap))
	}
}
