package integrations

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testJellyfinPathItems = "/Users/admin-1/Items"

func assertJellyfinAuthHeaders(t *testing.T, r *http.Request, apiKey string) {
	t.Helper()
	if got := r.Header.Get("X-Emby-Token"); got != apiKey {
		t.Errorf("X-Emby-Token = %q, want %q", got, apiKey)
	}
	wantAuth := jellyfinAuthorization(apiKey)
	if got := r.Header.Get("Authorization"); got != wantAuth {
		t.Errorf("Authorization = %q, want %q", got, wantAuth)
	}
}

func TestJellyfinClient_TestConnection_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/System/Info" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		assertJellyfinAuthHeaders(t, r, testTautulliAPIKey)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ServerName":"My Jellyfin","Version":"10.8.0"}`))
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should succeed: %v", err)
	}
}

func TestJellyfinClient_AuthHeaders_Jellyfin12Authorization(t *testing.T) {
	const apiKey = "jf12-api-key"
	var sawAuth, sawLegacy bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Jellyfin 12 disables legacy X-Emby-Token; only Authorization is accepted.
		if r.Header.Get("Authorization") != jellyfinAuthorization(apiKey) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		sawAuth = true
		if r.Header.Get("X-Emby-Token") == apiKey {
			sawLegacy = true
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ServerName":"JF12","Version":"12.0.0"}`))
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, apiKey)
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should succeed with MediaBrowser Authorization: %v", err)
	}
	if !sawAuth {
		t.Fatal("expected Authorization MediaBrowser header")
	}
	if !sawLegacy {
		t.Fatal("expected legacy X-Emby-Token to still be sent for older Jellyfin")
	}
}

func TestJellyfinClient_UploadPosterImage_SendsAuthHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/Items/item-1/Images/Primary" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		assertJellyfinAuthHeaders(t, r, testTautulliAPIKey)
		if r.Header.Get("Content-Type") != "image/jpeg" {
			t.Errorf("Content-Type = %q, want image/jpeg", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	if err := client.UploadPosterImage("item-1", []byte("poster-bytes"), "image/jpeg"); err != nil {
		t.Fatalf("UploadPosterImage should succeed: %v", err)
	}
}

func TestJellyfinClient_RestorePosterImage_SendsAuthHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		assertJellyfinAuthHeaders(t, r, testTautulliAPIKey)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	if err := client.RestorePosterImage("item-1"); err != nil {
		t.Fatalf("RestorePosterImage should succeed: %v", err)
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
						"DateCreated":"2024-01-10T12:00:00Z",
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
						"DateCreated":"2024-03-05T09:00:00Z",
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
	// AddedAt should be populated from DateCreated
	if movie.AddedAt == nil {
		t.Fatal("Expected AddedAt to be set from DateCreated")
	}
	if movie.AddedAt.Month() != 1 || movie.AddedAt.Day() != 10 {
		t.Errorf("Expected AddedAt Jan 10, got %v", movie.AddedAt)
	}

	// Item without DateCreated
	movie2, ok := data[99999]
	if !ok {
		t.Fatal("Expected TMDb ID 99999 key in data map")
	}
	if movie2.AddedAt != nil {
		t.Error("Expected AddedAt to be nil for item without DateCreated")
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
	// Unwatched item with DateCreated should still have AddedAt
	if newMovie.AddedAt == nil {
		t.Fatal("Expected AddedAt to be set from DateCreated for unwatched item")
	}
	if newMovie.AddedAt.Month() != 3 || newMovie.AddedAt.Day() != 5 {
		t.Errorf("Expected AddedAt Mar 5, got %v", newMovie.AddedAt)
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

func TestJellyfinClient_GetBulkWatchDataForUser_EpisodeAggregation(t *testing.T) {
	// Series has PlayCount=0 at the series level, but episodes have been watched.
	// The episode pass should promote that watch data to the parent series.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testJellyfinPathItems {
			w.WriteHeader(http.StatusNotFound)
			return
		}
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

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	data, err := client.GetBulkWatchDataForUser("admin-1", "admin")
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
	if len(series.Users) != 1 || series.Users[0] != "admin" {
		t.Errorf("Expected Users=[admin], got %v", series.Users)
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

func TestJellyfinClient_GetCollectionNames_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Users":
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

	client := NewJellyfinClient(srv.URL, "test-key")
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

func TestJellyfinClient_GetCollectionNames_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Users":
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

	client := NewJellyfinClient(srv.URL, "test-key")
	names, err := client.GetCollectionNames()
	if err != nil {
		t.Fatalf("GetCollectionNames should succeed with empty result: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("Expected 0 collection names, got %d", len(names))
	}
}

func TestJellyfinClient_GetCollectionNames_NoUsers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Users" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, "test-key")
	_, err := client.GetCollectionNames()
	if err == nil {
		t.Fatal("GetCollectionNames should fail when no users exist")
	}
}

func TestJellyfinClient_GetCollectionNames_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Users":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"Id":"admin-1","Name":"admin","Policy":{"IsAdministrator":true}}]`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, "test-key")
	_, err := client.GetCollectionNames()
	if err == nil {
		t.Fatal("GetCollectionNames should fail on API error")
	}
}

func TestJellyfinClient_GetLabelMemberships_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/Users":
			_, _ = w.Write([]byte(`[{"Id":"admin-1","Name":"admin","Policy":{"IsAdministrator":true}}]`))
		case "/Users/admin-1/Items":
			_, _ = w.Write([]byte(`{"Items":[
				{"Id":"1","Name":"Serenity","Type":"Movie","ProviderIds":{"Tmdb":"16320"},"Tags":["4K DV","Keep"]},
				{"Id":"2","Name":"Firefly","Type":"Series","ProviderIds":{"Tmdb":"1437"},"Tags":["Award Winner"]},
				{"Id":"3","Name":"No Tags","Type":"Movie","ProviderIds":{"Tmdb":"55555"},"Tags":[]}
			],"TotalRecordCount":3}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, "test-key")
	labelMap, err := client.GetLabelMemberships()
	if err != nil {
		t.Fatalf("GetLabelMemberships should succeed: %v", err)
	}
	if len(labelMap) != 2 {
		t.Fatalf("Expected 2 entries (items with tags), got %d", len(labelMap))
	}
	if labels := labelMap[16320]; len(labels) != 2 || labels[0] != "4K DV" || labels[1] != "Keep" {
		t.Errorf("Expected tags [4K DV, Keep] for TMDb 16320, got %v", labels)
	}
	if labels := labelMap[1437]; len(labels) != 1 || labels[0] != "Award Winner" {
		t.Errorf("Expected tags [Award Winner] for TMDb 1437, got %v", labels)
	}
}

func TestJellyfinClient_GetLabelMemberships_NoTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/Users":
			_, _ = w.Write([]byte(`[{"Id":"admin-1","Name":"admin","Policy":{"IsAdministrator":true}}]`))
		case "/Users/admin-1/Items":
			_, _ = w.Write([]byte(`{"Items":[
				{"Id":"1","Name":"Serenity","Type":"Movie","ProviderIds":{"Tmdb":"16320"},"Tags":[]}
			],"TotalRecordCount":1}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, "test-key")
	labelMap, err := client.GetLabelMemberships()
	if err != nil {
		t.Fatalf("GetLabelMemberships should succeed: %v", err)
	}
	if len(labelMap) != 0 {
		t.Errorf("Expected 0 entries (no tags), got %d", len(labelMap))
	}
}

func TestJellyfinClient_GetLabelNames_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/Users":
			_, _ = w.Write([]byte(`[{"Id":"admin-1","Name":"admin","Policy":{"IsAdministrator":true}}]`))
		case "/Users/admin-1/Items":
			_, _ = w.Write([]byte(`{"Items":[
				{"Tags":["4K DV","Keep"]},
				{"Tags":["4K DV","Award Winner"]},
				{"Tags":[]}
			],"TotalRecordCount":3}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, "test-key")
	names, err := client.GetLabelNames()
	if err != nil {
		t.Fatalf("GetLabelNames should succeed: %v", err)
	}
	expected := []string{"4K DV", "Award Winner", "Keep"}
	if len(names) != len(expected) {
		t.Fatalf("Expected %d label names, got %d: %v", len(expected), len(names), names)
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("Expected names[%d]=%q, got %q", i, expected[i], name)
		}
	}
}

func TestJellyfinClient_GetLabelNames_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/Users":
			_, _ = w.Write([]byte(`[{"Id":"admin-1","Name":"admin","Policy":{"IsAdministrator":true}}]`))
		case "/Users/admin-1/Items":
			_, _ = w.Write([]byte(`{"Items":[],"TotalRecordCount":0}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, "test-key")
	names, err := client.GetLabelNames()
	if err != nil {
		t.Fatalf("GetLabelNames should succeed: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("Expected 0 label names, got %d", len(names))
	}
}

func TestJellyfinClient_GetCollectionNames_SkipsBlankNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Users":
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

	client := NewJellyfinClient(srv.URL, "test-key")
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
