package integrations

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testPlexPathSections = "/library/sections"
const testPlexPathMoviesAll = "/library/sections/1/all"

func TestPlexClient_TestConnection_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/identity" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		// Plex sends token as query param
		if r.URL.Query().Get("X-Plex-Token") != "test-token" {
			t.Errorf("Missing or wrong Plex token in query params")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"MediaContainer":{"machineIdentifier":"abc123","version":"1.32.0"}}`))
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should succeed: %v", err)
	}
}

func TestPlexClient_TestConnection_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "bad-token")
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 401")
	}
}

func TestPlexClient_TestConnection_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 500")
	}
}

func TestPlexClient_GetDiskSpace_Empty(t *testing.T) {
	// Plex doesn't report disk space — should always return empty slice
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	disks, err := client.GetDiskSpace()
	if err != nil {
		t.Fatalf("GetDiskSpace should succeed: %v", err)
	}
	if len(disks) != 0 {
		t.Errorf("Expected empty disk space from Plex, got %d", len(disks))
	}
}

func TestPlexClient_GetRootFolders_Empty(t *testing.T) {
	// Plex doesn't have root folders in the *arr sense
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	folders, err := client.GetRootFolders()
	if err != nil {
		t.Fatalf("GetRootFolders should succeed: %v", err)
	}
	if len(folders) != 0 {
		t.Errorf("Expected empty root folders from Plex, got %d", len(folders))
	}
}

func TestPlexClient_GetMediaItems_Movies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testPlexPathSections:
			resp := plexLibraryResponse{}
			resp.MediaContainer.Directory = []struct {
				Key   string `json:"key"`
				Title string `json:"title"`
				Type  string `json:"type"`
			}{
				{Key: "1", Title: "Movies", Type: "movie"},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
		case testPlexPathMoviesAll:
			resp := plexMediaResponse{}
			resp.MediaContainer.Metadata = []plexMetadata{
				{
					RatingKey:      "101",
					Title:          "Serenity",
					Year:           2010,
					Type:           "movie",
					AudienceRating: 8.8,
					ViewCount:      3,
					LastViewedAt:   1700000000,
					AddedAt:        1680000000,
					Genre: []struct {
						Tag string `json:"tag"`
					}{{Tag: "Action"}, {Tag: "Sci-Fi"}},
					Media: []struct {
						Part []struct {
							File string `json:"file"`
							Size int64  `json:"size"`
						} `json:"Part"`
					}{
						{Part: []struct {
							File string `json:"file"`
							Size int64  `json:"size"`
						}{{File: "/media/movies/Serenity.mkv", Size: 8000000000}}},
					},
				},
				{
					RatingKey: "102",
					Title:     "Serenity 2",
					Year:      2014,
					Type:      "movie",
					Rating:    9.0, // Only critic rating, no audience
					Media: []struct {
						Part []struct {
							File string `json:"file"`
							Size int64  `json:"size"`
						} `json:"Part"`
					}{
						{Part: []struct {
							File string `json:"file"`
							Size int64  `json:"size"`
						}{{File: "/media/movies/Serenity2.mkv", Size: 12000000000}}},
					},
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	items, err := client.GetMediaItems()
	if err != nil {
		t.Fatalf("GetMediaItems should succeed: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(items))
	}

	// First movie
	movie := items[0]
	if movie.Type != MediaTypeMovie {
		t.Errorf("Expected MediaTypeMovie, got %v", movie.Type)
	}
	if movie.Title != "Serenity" {
		t.Errorf("Expected 'Serenity', got %q", movie.Title)
	}
	if movie.Year != 2010 {
		t.Errorf("Expected year 2010, got %d", movie.Year)
	}
	if movie.ExternalID != "101" {
		t.Errorf("Expected ExternalID '101', got %q", movie.ExternalID)
	}
	if movie.SizeBytes != 8000000000 {
		t.Errorf("Expected SizeBytes 8000000000, got %d", movie.SizeBytes)
	}
	if movie.Path != "/media/movies/Serenity.mkv" {
		t.Errorf("Expected path '/media/movies/Serenity.mkv', got %q", movie.Path)
	}
	if movie.Rating != 8.8 {
		t.Errorf("Expected audience rating 8.8, got %v", movie.Rating)
	}
	if movie.PlayCount != 3 {
		t.Errorf("Expected PlayCount 3, got %d", movie.PlayCount)
	}
	if movie.Genre != "Action, Sci-Fi" {
		t.Errorf("Expected genre 'Action, Sci-Fi', got %q", movie.Genre)
	}
	if movie.LastPlayed == nil {
		t.Error("Expected LastPlayed to be set")
	}
	if movie.AddedAt == nil {
		t.Error("Expected AddedAt to be set")
	}

	// Second movie — falls back to Rating since AudienceRating=0
	movie2 := items[1]
	if movie2.Rating != 9.0 {
		t.Errorf("Expected critic rating fallback 9.0, got %v", movie2.Rating)
	}
}

func TestPlexClient_GetMediaItems_ShowLibrary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testPlexPathSections:
			resp := plexLibraryResponse{}
			resp.MediaContainer.Directory = []struct {
				Key   string `json:"key"`
				Title string `json:"title"`
				Type  string `json:"type"`
			}{
				{Key: "2", Title: "TV Shows", Type: "show"},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
		case "/library/sections/2/all":
			resp := plexMediaResponse{}
			resp.MediaContainer.Metadata = []plexMetadata{
				{
					RatingKey: "200",
					Title:     "Firefly",
					Year:      2008,
					Type:      "show",
					Rating:    9.5,
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	items, err := client.GetMediaItems()
	if err != nil {
		t.Fatalf("GetMediaItems should succeed: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("Expected 1 show, got %d", len(items))
	}

	if items[0].Type != MediaTypeShow {
		t.Errorf("Expected MediaTypeShow, got %v", items[0].Type)
	}
	if items[0].Title != "Firefly" {
		t.Errorf("Expected 'Firefly', got %q", items[0].Title)
	}
}

func TestPlexClient_GetMediaItems_SkipsNonMediaLibraries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testPlexPathSections:
			resp := plexLibraryResponse{}
			resp.MediaContainer.Directory = []struct {
				Key   string `json:"key"`
				Title string `json:"title"`
				Type  string `json:"type"`
			}{
				{Key: "3", Title: "Music", Type: "artist"},
				{Key: "4", Title: "Photos", Type: "photo"},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	items, err := client.GetMediaItems()
	if err != nil {
		t.Fatalf("GetMediaItems should succeed with non-media libraries: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("Expected 0 items from non-movie/show libraries, got %d", len(items))
	}
}

func TestPlexClient_GetMediaItems_EmptyLibrary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testPlexPathSections:
			_, _ = w.Write([]byte(`{"MediaContainer":{"Directory":[]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	items, err := client.GetMediaItems()
	if err != nil {
		t.Fatalf("GetMediaItems should succeed with empty library: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("Expected 0 items from empty library, got %d", len(items))
	}
}

func TestPlexClient_GetMediaItems_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == testPlexPathSections {
			_, _ = w.Write([]byte(`not json at all`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	_, err := client.GetMediaItems()
	if err == nil {
		t.Fatal("Expected error for malformed JSON")
	}
}

func TestPlexClient_GetLibrarySections(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testPlexPathSections {
			w.Header().Set("Content-Type", "application/json")
			resp := plexLibraryResponse{}
			resp.MediaContainer.Directory = []struct {
				Key   string `json:"key"`
				Title string `json:"title"`
				Type  string `json:"type"`
			}{
				{Key: "1", Title: "Movies", Type: "movie"},
				{Key: "2", Title: "TV Shows", Type: "show"},
				{Key: "3", Title: "Music", Type: "artist"},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	sections, err := client.GetLibrarySections()
	if err != nil {
		t.Fatalf("GetLibrarySections should succeed: %v", err)
	}

	if len(sections) != 3 {
		t.Fatalf("Expected 3 sections, got %d", len(sections))
	}

	if sections[0].Title != "Movies" || sections[0].Type != "movie" || sections[0].Key != "1" {
		t.Errorf("Unexpected first section: %+v", sections[0])
	}
}

func TestPlexClient_DeleteMediaItem_Noop(t *testing.T) {
	// Plex's DeleteMediaItem is a no-op (read-only for watch history)
	client := NewPlexClient("http://localhost", "test-token")
	err := client.DeleteMediaItem(MediaItem{
		ExternalID: "101",
		Title:      "Test Movie",
		Type:       MediaTypeMovie,
	})
	if err != nil {
		t.Errorf("DeleteMediaItem should be a no-op, got error: %v", err)
	}
}

func TestPlexClient_URLTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/identity" {
			t.Errorf("Expected /identity, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"MediaContainer":{"machineIdentifier":"test","version":"1.0"}}`))
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL+"/", "test-token")
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should handle trailing slash: %v", err)
	}
}

func TestPlexClient_SeasonMetadata(t *testing.T) {
	// Test plexMetadataToMediaItem with season type
	m := plexMetadata{
		RatingKey:   "300",
		Title:       "Season 2",
		ParentTitle: "Firefly",
		Type:        "season",
		Index:       2,
		LeafCount:   13,
		Media: []struct {
			Part []struct {
				File string `json:"file"`
				Size int64  `json:"size"`
			} `json:"Part"`
		}{
			{Part: []struct {
				File string `json:"file"`
				Size int64  `json:"size"`
			}{{File: "/media/tv/Firefly/Season 2", Size: 15000000000}}},
		},
	}

	item := plexMetadataToMediaItem(m)
	if item == nil {
		t.Fatal("Expected non-nil MediaItem for season")
	}
	if item.Type != MediaTypeSeason {
		t.Errorf("Expected MediaTypeSeason, got %v", item.Type)
	}
	if item.SeasonNumber != 2 {
		t.Errorf("Expected SeasonNumber 2, got %d", item.SeasonNumber)
	}
	if item.EpisodeCount != 13 {
		t.Errorf("Expected EpisodeCount 13, got %d", item.EpisodeCount)
	}
	if item.ShowTitle != "Firefly" {
		t.Errorf("Expected ShowTitle 'Firefly', got %q", item.ShowTitle)
	}
}

func TestPlexClient_UnknownMediaType(t *testing.T) {
	// Unknown media types should return nil
	m := plexMetadata{
		RatingKey: "400",
		Title:     "Unknown",
		Type:      "photo",
	}

	item := plexMetadataToMediaItem(m)
	if item != nil {
		t.Errorf("Expected nil for unknown media type 'photo', got %+v", item)
	}
}

func TestPlexClient_GetBulkWatchData_Movies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testPlexPathSections:
			resp := plexLibraryResponse{}
			resp.MediaContainer.Directory = []struct {
				Key   string `json:"key"`
				Title string `json:"title"`
				Type  string `json:"type"`
			}{
				{Key: "1", Title: "Movies", Type: "movie"},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
		case testPlexPathMoviesAll:
			resp := plexMediaResponse{}
			resp.MediaContainer.Metadata = []plexMetadata{
				{
					RatingKey:    "101",
					Title:        "Serenity",
					Year:         2010,
					Type:         "movie",
					ViewCount:    5,
					LastViewedAt: 1700000000,
				},
				{
					RatingKey: "102",
					Title:     "Serenity 2",
					Year:      2014,
					Type:      "movie",
					ViewCount: 0,
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	watchMap, err := client.GetBulkWatchData()
	if err != nil {
		t.Fatalf("GetBulkWatchData should succeed: %v", err)
	}

	if len(watchMap) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(watchMap))
	}

	// Verify title normalization (lowercase)
	movie1, ok := watchMap["serenity"]
	if !ok {
		t.Fatal("Expected 'serenity' key in watch map")
	}
	if movie1.PlayCount != 5 {
		t.Errorf("Expected PlayCount 5, got %d", movie1.PlayCount)
	}
	if !movie1.Played {
		t.Error("Expected Played=true for Serenity")
	}
	if movie1.LastPlayed == nil {
		t.Error("Expected LastPlayed to be set for Serenity")
	}

	// Unwatched movie should still be in map with PlayCount=0
	movie2, ok := watchMap["serenity 2"]
	if !ok {
		t.Fatal("Expected 'serenity 2' key in watch map")
	}
	if movie2.PlayCount != 0 {
		t.Errorf("Expected PlayCount 0, got %d", movie2.PlayCount)
	}
	if movie2.Played {
		t.Error("Expected Played=false for Serenity 2")
	}
	if movie2.LastPlayed != nil {
		t.Error("Expected LastPlayed to be nil for Serenity 2")
	}
}

func TestPlexClient_GetBulkWatchData_Shows(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testPlexPathSections:
			resp := plexLibraryResponse{}
			resp.MediaContainer.Directory = []struct {
				Key   string `json:"key"`
				Title string `json:"title"`
				Type  string `json:"type"`
			}{
				{Key: "2", Title: "TV Shows", Type: "show"},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
		case "/library/sections/2/all":
			resp := plexMediaResponse{}
			resp.MediaContainer.Metadata = []plexMetadata{
				{
					RatingKey:    "200",
					Title:        "Firefly",
					Year:         2008,
					Type:         "show",
					ViewCount:    10,
					LastViewedAt: 1700000000,
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	watchMap, err := client.GetBulkWatchData()
	if err != nil {
		t.Fatalf("GetBulkWatchData should succeed: %v", err)
	}

	show, ok := watchMap["firefly"]
	if !ok {
		t.Fatal("Expected 'firefly' key in watch map")
	}
	if show.PlayCount != 10 {
		t.Errorf("Expected PlayCount 10, got %d", show.PlayCount)
	}
}

func TestPlexClient_GetBulkWatchData_DuplicateTitle(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testPlexPathSections:
			resp := plexLibraryResponse{}
			resp.MediaContainer.Directory = []struct {
				Key   string `json:"key"`
				Title string `json:"title"`
				Type  string `json:"type"`
			}{
				{Key: "1", Title: "Movies", Type: "movie"},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
		case testPlexPathMoviesAll:
			resp := plexMediaResponse{}
			resp.MediaContainer.Metadata = []plexMetadata{
				{
					RatingKey: "101",
					Title:     "Dune",
					Year:      1984,
					Type:      "movie",
					ViewCount: 2,
				},
				{
					RatingKey: "102",
					Title:     "Dune",
					Year:      2021,
					Type:      "movie",
					ViewCount: 7,
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	watchMap, err := client.GetBulkWatchData()
	if err != nil {
		t.Fatalf("GetBulkWatchData should succeed: %v", err)
	}

	// Should keep the entry with the highest play count
	dune, ok := watchMap["dune"]
	if !ok {
		t.Fatal("Expected 'dune' key in watch map")
	}
	if dune.PlayCount != 7 {
		t.Errorf("Expected highest PlayCount 7, got %d", dune.PlayCount)
	}
}

func TestPlexClient_GetBulkWatchData_EmptyLibrary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testPlexPathSections:
			_, _ = w.Write([]byte(`{"MediaContainer":{"Directory":[]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	watchMap, err := client.GetBulkWatchData()
	if err != nil {
		t.Fatalf("GetBulkWatchData should succeed with empty library: %v", err)
	}
	if len(watchMap) != 0 {
		t.Errorf("Expected empty watch map, got %d entries", len(watchMap))
	}
}

func TestPlexClient_GetBulkWatchData_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	_, err := client.GetBulkWatchData()
	if err == nil {
		t.Fatal("Expected error for API failure")
	}
}

func TestPlexClient_MultiPartMedia(t *testing.T) {
	// Test that file sizes from multiple parts are summed
	m := plexMetadata{
		RatingKey: "500",
		Title:     "Extended Movie",
		Type:      "movie",
		Media: []struct {
			Part []struct {
				File string `json:"file"`
				Size int64  `json:"size"`
			} `json:"Part"`
		}{
			{Part: []struct {
				File string `json:"file"`
				Size int64  `json:"size"`
			}{
				{File: "/media/movies/part1.mkv", Size: 4000000000},
				{File: "/media/movies/part2.mkv", Size: 3000000000},
			}},
		},
	}

	item := plexMetadataToMediaItem(m)
	if item == nil {
		t.Fatal("Expected non-nil MediaItem")
	}
	if item.SizeBytes != 7000000000 {
		t.Errorf("Expected total size 7000000000, got %d", item.SizeBytes)
	}
	// Path should be from the first part
	if item.Path != "/media/movies/part1.mkv" {
		t.Errorf("Expected path from first part, got %q", item.Path)
	}
}
