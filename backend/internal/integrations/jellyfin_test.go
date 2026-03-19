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

func TestJellyfinClient_GetBulkWatchData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testJellyfinPathItems {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"Items": [
					{
						"Id":"item-1",
						"Name":"Serenity",
						"Type":"Movie",
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
	data, err := client.GetBulkWatchDataForUser("admin-1")
	if err != nil {
		t.Fatalf("GetBulkWatchData should succeed: %v", err)
	}

	if len(data) != 3 {
		t.Fatalf("Expected 3 items, got %d", len(data))
	}

	// Items are keyed by lowercase title
	movie, ok := data["serenity"]
	if !ok {
		t.Fatal("Expected 'serenity' key in data map")
	}
	if movie.PlayCount != 3 {
		t.Errorf("Expected PlayCount 3, got %d", movie.PlayCount)
	}
	if movie.LastPlayed == nil {
		t.Error("Expected LastPlayed to be set")
	}

	// Unwatched item
	newMovie, ok := data["new movie"]
	if !ok {
		t.Fatal("Expected 'new movie' key in data map")
	}
	if newMovie.PlayCount != 0 {
		t.Errorf("Expected PlayCount 0, got %d", newMovie.PlayCount)
	}
	if newMovie.LastPlayed != nil {
		t.Error("Expected nil LastPlayed for unwatched item")
	}
}

func TestJellyfinClient_GetBulkWatchData_DuplicateKeepsHigherPlayCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testJellyfinPathItems {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"Items": [
					{
						"Id":"item-1",
						"Name":"Serenity",
						"Type":"Movie",
						"UserData":{"PlayCount":2,"LastPlayedDate":"","Played":true}
					},
					{
						"Id":"item-2",
						"Name":"Serenity",
						"Type":"Series",
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
	data, err := client.GetBulkWatchDataForUser("admin-1")
	if err != nil {
		t.Fatalf("GetBulkWatchData should succeed: %v", err)
	}

	// Should keep the entry with higher play count
	movie, ok := data["serenity"]
	if !ok {
		t.Fatal("Expected 'serenity' key in data map")
	}
	if movie.PlayCount != 5 {
		t.Errorf("Expected higher PlayCount 5 for duplicate, got %d", movie.PlayCount)
	}
}

func TestJellyfinClient_GetBulkWatchData_Empty(t *testing.T) {
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
	data, err := client.GetBulkWatchDataForUser("admin-1")
	if err != nil {
		t.Fatalf("GetBulkWatchData should succeed with empty: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("Expected 0 items, got %d", len(data))
	}
}

func TestJellyfinClient_GetBulkWatchData_MalformedJSON(t *testing.T) {
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
	_, err := client.GetBulkWatchDataForUser("admin-1")
	if err == nil {
		t.Fatal("Expected error for malformed JSON response")
	}
}

func TestJellyfinClient_GetBulkWatchData_SkipsEmptyNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testJellyfinPathItems {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"Items": [
					{"Id":"1","Name":"","Type":"Movie","UserData":{"PlayCount":1,"LastPlayedDate":"","Played":true}},
					{"Id":"2","Name":"  ","Type":"Movie","UserData":{"PlayCount":2,"LastPlayedDate":"","Played":true}},
					{"Id":"3","Name":"Valid Movie","Type":"Movie","UserData":{"PlayCount":3,"LastPlayedDate":"","Played":true}}
				],
				"TotalRecordCount": 3
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewJellyfinClient(srv.URL, testTautulliAPIKey)
	data, err := client.GetBulkWatchDataForUser("admin-1")
	if err != nil {
		t.Fatalf("GetBulkWatchData should succeed: %v", err)
	}

	// Only "Valid Movie" should be in the result (empty and whitespace-only names are skipped)
	if len(data) != 1 {
		t.Errorf("Expected 1 valid item, got %d", len(data))
	}
	if _, ok := data["valid movie"]; !ok {
		t.Error("Expected 'valid movie' key in data map")
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
					{"Id":"item-1","Name":"Serenity","Type":"Movie"},
					{"Id":"item-2","Name":"Firefly","Type":"Series"}
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
	if !favs["serenity"] {
		t.Error("Expected 'serenity' in favorites map")
	}
	if !favs["firefly"] {
		t.Error("Expected 'firefly' in favorites map")
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

func TestJellyfinClient_GetFavoritedItems_SkipsEmptyNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testJellyfinPathItems {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"Items": [
					{"Id":"1","Name":"","Type":"Movie"},
					{"Id":"2","Name":"  ","Type":"Movie"},
					{"Id":"3","Name":"Serenity","Type":"Movie"}
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
		t.Fatalf("Expected 1 favorite (empty names skipped), got %d", len(favs))
	}
	if !favs["serenity"] {
		t.Error("Expected 'serenity' in favorites map")
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
