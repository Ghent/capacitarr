package integrations

import (
	"net/http"
	"net/http/httptest"
	"testing"
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

func TestEmbyClient_GetBulkWatchData(t *testing.T) {
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
					"UserData": {
						"PlayCount": 5,
						"LastPlayedDate": "2024-11-01T20:00:00Z",
						"Played": true
					}
				},
				{
					"Name": "Serenity 2",
					"UserData": {
						"PlayCount": 2,
						"LastPlayedDate": "2024-10-15T14:00:00Z",
						"Played": true
					}
				},
				{
					"Name": "Unwatched Movie",
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
	data, err := client.GetBulkWatchDataForUser("admin1")
	if err != nil {
		t.Fatalf("GetBulkWatchData should succeed: %v", err)
	}
	if len(data) != 3 {
		t.Fatalf("Expected 3 entries, got %d", len(data))
	}

	// Check Serenity (normalized to lowercase)
	movie, ok := data["serenity"]
	if !ok {
		t.Fatal("Expected 'serenity' key in result")
	}
	if movie.PlayCount != 5 {
		t.Errorf("Expected PlayCount 5 for Serenity, got %d", movie.PlayCount)
	}
	if movie.PlayCount == 0 {
		t.Error("Expected Serenity to be marked as Played")
	}
	if movie.LastPlayed == nil {
		t.Error("Expected non-nil LastPlayed for Serenity")
	}

	// Check unwatched movie
	unwatched, ok := data["unwatched movie"]
	if !ok {
		t.Fatal("Expected 'unwatched movie' key in result")
	}
	if unwatched.PlayCount != 0 {
		t.Errorf("Expected PlayCount 0 for unwatched, got %d", unwatched.PlayCount)
	}
	if unwatched.LastPlayed != nil {
		t.Error("Expected nil LastPlayed for unwatched movie")
	}
}

func TestEmbyClient_GetBulkWatchData_Pagination(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount == 1 {
			// First page: return 2 items with TotalRecordCount=3 to trigger pagination
			_, _ = w.Write([]byte(`{
				"Items": [
					{"Name": "Movie A", "UserData": {"PlayCount": 1, "Played": true}},
					{"Name": "Movie B", "UserData": {"PlayCount": 2, "Played": true}}
				],
				"TotalRecordCount": 3
			}`))
		} else {
			// Second page: final item
			_, _ = w.Write([]byte(`{
				"Items": [
					{"Name": "Movie C", "UserData": {"PlayCount": 0, "Played": false}}
				],
				"TotalRecordCount": 3
			}`))
		}
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	data, err := client.GetBulkWatchDataForUser("admin1")
	if err != nil {
		t.Fatalf("GetBulkWatchData should succeed: %v", err)
	}
	if len(data) != 3 {
		t.Fatalf("Expected 3 entries after pagination, got %d", len(data))
	}
	if _, ok := data["movie a"]; !ok {
		t.Error("Expected 'movie a' key")
	}
	if _, ok := data["movie c"]; !ok {
		t.Error("Expected 'movie c' key")
	}
}

func TestEmbyClient_GetBulkWatchData_DuplicateKeepsHigherPlayCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"Items": [
				{"Name": "Serenity", "UserData": {"PlayCount": 1, "Played": true}},
				{"Name": "Serenity", "UserData": {"PlayCount": 5, "Played": true}}
			],
			"TotalRecordCount": 2
		}`))
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	data, err := client.GetBulkWatchDataForUser("admin1")
	if err != nil {
		t.Fatalf("GetBulkWatchData should succeed: %v", err)
	}
	movie, ok := data["serenity"]
	if !ok {
		t.Fatal("Expected 'serenity' key")
	}
	if movie.PlayCount != 5 {
		t.Errorf("Expected higher PlayCount 5 to be kept, got %d", movie.PlayCount)
	}
}

func TestEmbyClient_GetBulkWatchData_EmptyItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Items": [], "TotalRecordCount": 0}`))
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	data, err := client.GetBulkWatchDataForUser("admin1")
	if err != nil {
		t.Fatalf("GetBulkWatchData should succeed: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(data))
	}
}

func TestEmbyClient_GetBulkWatchData_SkipsEmptyNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"Items": [
				{"Name": "", "UserData": {"PlayCount": 1, "Played": true}},
				{"Name": "  ", "UserData": {"PlayCount": 2, "Played": true}},
				{"Name": "Valid Movie", "UserData": {"PlayCount": 3, "Played": true}}
			],
			"TotalRecordCount": 3
		}`))
	}))
	defer srv.Close()

	client := NewEmbyClient(srv.URL, testTautulliAPIKey)
	data, err := client.GetBulkWatchDataForUser("admin1")
	if err != nil {
		t.Fatalf("GetBulkWatchData should succeed: %v", err)
	}
	// Empty and whitespace-only names should be skipped
	if len(data) != 1 {
		t.Fatalf("Expected 1 entry (empty names skipped), got %d", len(data))
	}
	if _, ok := data["valid movie"]; !ok {
		t.Error("Expected 'valid movie' key")
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
					{"Name": "Serenity"},
					{"Name": "Firefly"}
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
	if !favs["serenity"] {
		t.Error("Expected 'serenity' in favorites map")
	}
	if !favs["firefly"] {
		t.Error("Expected 'firefly' in favorites map")
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

func TestEmbyClient_GetFavoritedItems_SkipsEmptyNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/Users/admin1/Items" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"Items": [
					{"Name": ""},
					{"Name": "  "},
					{"Name": "Serenity"}
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
		t.Fatalf("Expected 1 favorite (empty names skipped), got %d", len(favs))
	}
	if !favs["serenity"] {
		t.Error("Expected 'serenity' in favorites map")
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
