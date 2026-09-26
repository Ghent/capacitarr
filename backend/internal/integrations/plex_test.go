package integrations

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

// TestPlexClient_NotMediaSource verifies that PlexClient does NOT implement MediaSource.
// This is a design invariant: only *arr integrations should provide media items
// to the evaluation pool. If this test fails, someone added GetMediaItems() back.
func TestPlexClient_NotMediaSource(t *testing.T) {
	client := NewPlexClient("http://localhost", "token")
	var iface interface{} = client
	if _, ok := iface.(MediaSource); ok {
		t.Fatal("PlexClient must NOT implement MediaSource — only *arr integrations should")
	}
}

func TestPlexClient_getMediaItems_Movies(t *testing.T) {
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
					GUIDs:          []plexGUID{{ID: "tmdb://16320"}, {ID: "imdb://tt0379786"}},
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
					GUIDs:     []plexGUID{{ID: "tmdb://99999"}},
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
	items, err := client.getMediaItems()
	if err != nil {
		t.Fatalf("getMediaItems should succeed: %v", err)
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
	if movie.TMDbID != 16320 {
		t.Errorf("Expected TMDbID 16320, got %d", movie.TMDbID)
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

func TestPlexClient_getMediaItems_ShowLibrary(t *testing.T) {
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
					GUIDs:     []plexGUID{{ID: "tmdb://1437"}},
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
	items, err := client.getMediaItems()
	if err != nil {
		t.Fatalf("getMediaItems should succeed: %v", err)
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

func TestPlexClient_getMediaItems_SkipsNonMediaLibraries(t *testing.T) {
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
	items, err := client.getMediaItems()
	if err != nil {
		t.Fatalf("getMediaItems should succeed with non-media libraries: %v", err)
	}

	if len(items) != 0 {
		t.Errorf("Expected 0 items from non-movie/show libraries, got %d", len(items))
	}
}

func TestPlexClient_getMediaItems_EmptyLibrary(t *testing.T) {
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
	items, err := client.getMediaItems()
	if err != nil {
		t.Fatalf("getMediaItems should succeed with empty library: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("Expected 0 items from empty library, got %d", len(items))
	}
}

func TestPlexClient_getMediaItems_MalformedJSON(t *testing.T) {
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
	_, err := client.getMediaItems()
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
		GUIDs:       []plexGUID{{ID: "tmdb://1437"}},
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
		Title:     "Serenity",
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
					AddedAt:      1680000000, // 2023-03-28T17:46:40Z
					GUIDs:        []plexGUID{{ID: "tmdb://16320"}},
				},
				{
					RatingKey: "102",
					Title:     "Serenity 2",
					Year:      2014,
					Type:      "movie",
					ViewCount: 0,
					GUIDs:     []plexGUID{{ID: "tmdb://99999"}},
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

	// Verify keyed by TMDb ID
	movie1, ok := watchMap[16320]
	if !ok {
		t.Fatal("Expected TMDb ID 16320 key in watch map")
	}
	if movie1.PlayCount != 5 {
		t.Errorf("Expected PlayCount 5, got %d", movie1.PlayCount)
	}
	if movie1.LastPlayed == nil {
		t.Error("Expected LastPlayed to be set for Serenity")
	}
	// AddedAt should be bridged from Plex library metadata
	if movie1.AddedAt == nil {
		t.Error("Expected AddedAt to be set for Serenity (from Plex addedAt)")
	}

	// Unwatched movie should still be in map with PlayCount=0
	movie2, ok := watchMap[99999]
	if !ok {
		t.Fatal("Expected TMDb ID 99999 key in watch map")
	}
	if movie2.PlayCount != 0 {
		t.Errorf("Expected PlayCount 0, got %d", movie2.PlayCount)
	}
	if movie2.LastPlayed != nil {
		t.Error("Expected LastPlayed to be nil for Serenity 2")
	}
	// No addedAt in metadata → AddedAt should be nil
	if movie2.AddedAt != nil {
		t.Error("Expected AddedAt to be nil for Serenity 2 (no addedAt in metadata)")
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
					GUIDs:        []plexGUID{{ID: "tmdb://1437"}},
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

	show, ok := watchMap[1437]
	if !ok {
		t.Fatal("Expected TMDb ID 1437 key in watch map")
	}
	if show.PlayCount != 10 {
		t.Errorf("Expected PlayCount 10, got %d", show.PlayCount)
	}
}

func TestPlexClient_GetBulkWatchData_DuplicateTMDbID(t *testing.T) {
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
					Title:     "Serenity",
					Year:      2005,
					Type:      "movie",
					ViewCount: 2,
					GUIDs:     []plexGUID{{ID: "tmdb://16320"}},
				},
				{
					RatingKey: "102",
					Title:     "Serenity (Special Edition)",
					Year:      2005,
					Type:      "movie",
					ViewCount: 7,
					GUIDs:     []plexGUID{{ID: "tmdb://16320"}},
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
	serenity, ok := watchMap[16320]
	if !ok {
		t.Fatal("Expected TMDb ID 16320 key in watch map")
	}
	if serenity.PlayCount != 7 {
		t.Errorf("Expected highest PlayCount 7, got %d", serenity.PlayCount)
	}
}

func TestPlexClient_GetBulkWatchData_SkipsMissingTMDbGUID(t *testing.T) {
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
					Title:     "No GUIDs Movie",
					Type:      "movie",
					ViewCount: 1,
					// No Guids — should be skipped
				},
				{
					RatingKey: "102",
					Title:     "Only IMDB GUID",
					Type:      "movie",
					ViewCount: 2,
					GUIDs:     []plexGUID{{ID: "imdb://tt1234567"}},
				},
				{
					RatingKey: "103",
					Title:     "Serenity",
					Type:      "movie",
					ViewCount: 3,
					GUIDs:     []plexGUID{{ID: "tmdb://16320"}},
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

	// Only item with TMDb GUID should be in result
	if len(watchMap) != 1 {
		t.Fatalf("Expected 1 entry (missing TMDb GUIDs skipped), got %d", len(watchMap))
	}
	if _, ok := watchMap[16320]; !ok {
		t.Error("Expected TMDb ID 16320 key in watch map")
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
		Title:     "Serenity",
		Type:      "movie",
		GUIDs:     []plexGUID{{ID: "tmdb://16320"}},
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

func TestPlexClient_GetOnDeckItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/library/onDeck" {
			resp := plexMediaResponse{}
			resp.MediaContainer.Metadata = []plexMetadata{
				{
					RatingKey: "101",
					Title:     "Serenity",
					Type:      "movie",
					GUIDs:     []plexGUID{{ID: "tmdb://16320"}},
				},
				{
					RatingKey: "102",
					Title:     "Serenity 2",
					Type:      "movie",
					GUIDs:     []plexGUID{{ID: "tmdb://99999"}},
				},
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
	onDeck, err := client.GetOnDeckItems()
	if err != nil {
		t.Fatalf("GetOnDeckItems should succeed: %v", err)
	}
	if len(onDeck) != 2 {
		t.Fatalf("Expected 2 on-deck items, got %d", len(onDeck))
	}
	if !onDeck[16320] {
		t.Error("Expected TMDb ID 16320 (Serenity) in on-deck map")
	}
	if !onDeck[99999] {
		t.Error("Expected TMDb ID 99999 (Serenity 2) in on-deck map")
	}
}

func TestPlexClient_GetOnDeckItems_Episodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/library/onDeck" {
			resp := plexMediaResponse{}
			resp.MediaContainer.Metadata = []plexMetadata{
				{
					RatingKey:        "301",
					Title:            "The Train Job",
					Type:             "episode",
					GrandparentTitle: "Firefly",
					GUIDs:            []plexGUID{{ID: "tmdb://1437"}},
				},
				{
					RatingKey:        "302",
					Title:            "Bushwhacked",
					Type:             "episode",
					GrandparentTitle: "Firefly",
					GUIDs:            []plexGUID{{ID: "tmdb://1437"}},
				},
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
	onDeck, err := client.GetOnDeckItems()
	if err != nil {
		t.Fatalf("GetOnDeckItems should succeed: %v", err)
	}
	// Both episodes from the same show share TMDb ID 1437, so result is deduplicated
	if len(onDeck) != 1 {
		t.Fatalf("Expected 1 on-deck item (deduplicated by TMDb ID), got %d", len(onDeck))
	}
	if !onDeck[1437] {
		t.Error("Expected TMDb ID 1437 (Firefly) in on-deck map")
	}
}

func TestPlexClient_GetOnDeckItems_SkipsMissingTMDbGUID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/library/onDeck" {
			resp := plexMediaResponse{}
			resp.MediaContainer.Metadata = []plexMetadata{
				{
					RatingKey: "101",
					Title:     "No GUID Movie",
					Type:      "movie",
					// No Guids — should be skipped
				},
				{
					RatingKey: "102",
					Title:     "Serenity",
					Type:      "movie",
					GUIDs:     []plexGUID{{ID: "tmdb://16320"}},
				},
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
	onDeck, err := client.GetOnDeckItems()
	if err != nil {
		t.Fatalf("GetOnDeckItems should succeed: %v", err)
	}
	if len(onDeck) != 1 {
		t.Fatalf("Expected 1 on-deck item (missing TMDb GUID skipped), got %d", len(onDeck))
	}
	if !onDeck[16320] {
		t.Error("Expected TMDb ID 16320 in on-deck map")
	}
}

func TestPlexClient_GetOnDeckItems_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/library/onDeck" {
			_, _ = w.Write([]byte(`{"MediaContainer":{"Metadata":[]}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	onDeck, err := client.GetOnDeckItems()
	if err != nil {
		t.Fatalf("GetOnDeckItems should succeed with empty deck: %v", err)
	}
	if len(onDeck) != 0 {
		t.Errorf("Expected empty on-deck map, got %d entries", len(onDeck))
	}
}

func TestPlexClient_GetOnDeckItems_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	_, err := client.GetOnDeckItems()
	if err == nil {
		t.Fatal("Expected error for API failure")
	}
}

// plexOnDeckRatingArrayJSON is a Plex on-deck payload with both the scalar
// rating field and the PascalCase Rating array on one item. encoding/json
// matches keys case-insensitively, so this is the shape that used to fail
// the entire on-deck parse (issue #67).
const plexOnDeckRatingArrayJSON = `{
  "MediaContainer": {
    "Metadata": [
      {
        "ratingKey": "101",
        "title": "Serenity",
        "type": "movie",
        "rating": 8.5,
        "audienceRating": 8.8,
        "Rating": [
          {"image": "imdb://image.rating", "value": 8.0, "type": "audience"},
          {"image": "rottentomatoes://image.rating.ripe", "value": 8.2, "type": "critic"}
        ],
        "Guid": [{"id": "tmdb://16320"}]
      }
    ]
  }
}`

func TestPlexMetadata_RatingArrayDoesNotCollideWithScalar(t *testing.T) {
	var resp plexMediaResponse
	if err := json.Unmarshal([]byte(plexOnDeckRatingArrayJSON), &resp); err != nil {
		t.Fatalf("expected Rating array and rating scalar to decode together: %v", err)
	}
	if len(resp.MediaContainer.Metadata) != 1 {
		t.Fatalf("expected 1 metadata item, got %d", len(resp.MediaContainer.Metadata))
	}
	m := resp.MediaContainer.Metadata[0]
	if m.Rating != 8.5 {
		t.Errorf("expected scalar rating 8.5, got %v", m.Rating)
	}
	if m.AudienceRating != 8.8 {
		t.Errorf("expected audienceRating 8.8, got %v", m.AudienceRating)
	}
	if len(m.Ratings) != 2 {
		t.Fatalf("expected 2 Rating array entries, got %d", len(m.Ratings))
	}
	if m.Ratings[0].Value != 8.0 || m.Ratings[0].Type != "audience" {
		t.Errorf("unexpected first Rating entry: %+v", m.Ratings[0])
	}
	if plexExtractTMDbID(m.GUIDs) != 16320 {
		t.Errorf("expected TMDb ID 16320, got %d", plexExtractTMDbID(m.GUIDs))
	}
}

func TestPlexClient_GetOnDeckItems_RatingArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/library/onDeck" {
			_, _ = w.Write([]byte(plexOnDeckRatingArrayJSON))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	onDeck, err := client.GetOnDeckItems()
	if err != nil {
		t.Fatalf("GetOnDeckItems should succeed with Rating array present: %v", err)
	}
	if !onDeck[16320] {
		t.Errorf("expected TMDb ID 16320 in on-deck map, got %v", onDeck)
	}
}

func TestPlexClient_GetOnDeckItems_RequestsIncludeGuids(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/library/onDeck" {
			gotQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"MediaContainer":{"Metadata":[]}}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	if _, err := client.GetOnDeckItems(); err != nil {
		t.Fatalf("GetOnDeckItems should succeed: %v", err)
	}
	if !strings.Contains(gotQuery, "includeGuids=1") {
		t.Errorf("on-deck request must include includeGuids=1, got %q", gotQuery)
	}
}

func TestPlexClient_GetCollectionNames(t *testing.T) {
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
					Title:     "Serenity",
					Type:      "movie",
					GUIDs:     []plexGUID{{ID: "tmdb://16320"}},
					Collection: []struct {
						Tag string `json:"tag"`
					}{{Tag: "Joss Whedon"}, {Tag: "Sci-Fi Classics"}},
				},
				{
					RatingKey: "102",
					Title:     "Firefly: The Movie",
					Type:      "movie",
					GUIDs:     []plexGUID{{ID: "tmdb://1437"}},
					Collection: []struct {
						Tag string `json:"tag"`
					}{{Tag: "Joss Whedon"}, {Tag: "Space Westerns"}},
				},
				{
					RatingKey: "103",
					Title:     "No Collections Movie",
					Type:      "movie",
					GUIDs:     []plexGUID{{ID: "tmdb://55555"}},
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
	names, err := client.GetCollectionNames()
	if err != nil {
		t.Fatalf("GetCollectionNames should succeed: %v", err)
	}

	// Should be sorted and deduplicated
	expected := []string{"Joss Whedon", "Sci-Fi Classics", "Space Westerns"}
	if len(names) != len(expected) {
		t.Fatalf("Expected %d collection names, got %d: %v", len(expected), len(names), names)
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("Expected names[%d]=%q, got %q", i, expected[i], name)
		}
	}
}

func TestPlexClient_GetCollectionNames_Empty(t *testing.T) {
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
	names, err := client.GetCollectionNames()
	if err != nil {
		t.Fatalf("GetCollectionNames should succeed with empty library: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("Expected 0 collection names, got %d", len(names))
	}
}

func TestPlexClient_GetCollectionNames_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	_, err := client.GetCollectionNames()
	if err == nil {
		t.Fatal("Expected error for API failure")
	}
}

func TestPlexClient_GetTMDbToRatingKeyMap(t *testing.T) {
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
					Title:     "Serenity",
					Type:      "movie",
					GUIDs:     []plexGUID{{ID: "tmdb://16320"}},
				},
				{
					RatingKey: "102",
					Title:     "Firefly: The Movie",
					Type:      "movie",
					GUIDs:     []plexGUID{{ID: "tmdb://1437"}},
				},
				{
					RatingKey: "103",
					Title:     "No GUID Movie",
					Type:      "movie",
					// No Guids — should not appear in map
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
	tmdbMap, err := client.GetTMDbToRatingKeyMap()
	if err != nil {
		t.Fatalf("GetTMDbToRatingKeyMap should succeed: %v", err)
	}

	// Only items with TMDb GUIDs should be in the map
	if len(tmdbMap) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(tmdbMap))
	}
	if tmdbMap[16320] != "101" {
		t.Errorf("Expected TMDb 16320 → ratingKey '101', got %q", tmdbMap[16320])
	}
	if tmdbMap[1437] != "102" {
		t.Errorf("Expected TMDb 1437 → ratingKey '102', got %q", tmdbMap[1437])
	}
}

func TestPlexClient_GetLabelMemberships_Success(t *testing.T) {
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
					Title:     "Serenity",
					Type:      "movie",
					GUIDs:     []plexGUID{{ID: "tmdb://16320"}},
					Label: []struct {
						Tag string `json:"tag"`
					}{{Tag: "4K DV"}, {Tag: "Keep"}},
				},
				{
					RatingKey: "102",
					Title:     "Firefly: The Movie",
					Type:      "movie",
					GUIDs:     []plexGUID{{ID: "tmdb://1437"}},
					Label: []struct {
						Tag string `json:"tag"`
					}{{Tag: "Award Winner"}},
				},
				{
					RatingKey: "103",
					Title:     "No Labels Movie",
					Type:      "movie",
					GUIDs:     []plexGUID{{ID: "tmdb://55555"}},
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
	labelMap, err := client.GetLabelMemberships()
	if err != nil {
		t.Fatalf("GetLabelMemberships should succeed: %v", err)
	}

	if len(labelMap) != 2 {
		t.Fatalf("Expected 2 entries (items with labels), got %d", len(labelMap))
	}
	if labels := labelMap[16320]; len(labels) != 2 || labels[0] != "4K DV" || labels[1] != "Keep" {
		t.Errorf("Expected labels [4K DV, Keep] for TMDb 16320, got %v", labels)
	}
	if labels := labelMap[1437]; len(labels) != 1 || labels[0] != "Award Winner" {
		t.Errorf("Expected labels [Award Winner] for TMDb 1437, got %v", labels)
	}
	if _, ok := labelMap[55555]; ok {
		t.Error("Item with no labels should not appear in label map")
	}
}

func TestPlexClient_GetLabelMemberships_SkipsNoTMDbID(t *testing.T) {
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
					Title:     "Serenity",
					Type:      "movie",
					// No GUIDs — TMDb ID will be 0
					Label: []struct {
						Tag string `json:"tag"`
					}{{Tag: "4K DV"}},
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
	labelMap, err := client.GetLabelMemberships()
	if err != nil {
		t.Fatalf("GetLabelMemberships should succeed: %v", err)
	}
	if len(labelMap) != 0 {
		t.Errorf("Expected 0 entries (no TMDb IDs), got %d", len(labelMap))
	}
}

func TestPlexClient_GetLabelMemberships_Empty(t *testing.T) {
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
	labelMap, err := client.GetLabelMemberships()
	if err != nil {
		t.Fatalf("GetLabelMemberships should succeed with empty library: %v", err)
	}
	if len(labelMap) != 0 {
		t.Errorf("Expected 0 label entries, got %d", len(labelMap))
	}
}

func TestPlexClient_GetLabelNames_Success(t *testing.T) {
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
					Title:     "Serenity",
					Type:      "movie",
					GUIDs:     []plexGUID{{ID: "tmdb://16320"}},
					Label: []struct {
						Tag string `json:"tag"`
					}{{Tag: "4K DV"}, {Tag: "Keep"}},
				},
				{
					RatingKey: "102",
					Title:     "Firefly: The Movie",
					Type:      "movie",
					GUIDs:     []plexGUID{{ID: "tmdb://1437"}},
					Label: []struct {
						Tag string `json:"tag"`
					}{{Tag: "4K DV"}, {Tag: "Award Winner"}},
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
	names, err := client.GetLabelNames()
	if err != nil {
		t.Fatalf("GetLabelNames should succeed: %v", err)
	}

	// Should be sorted and deduplicated
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

func TestPlexClient_GetLabelNames_SkipsBlanks(t *testing.T) {
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
					Title:     "Serenity",
					Type:      "movie",
					GUIDs:     []plexGUID{{ID: "tmdb://16320"}},
					Label: []struct {
						Tag string `json:"tag"`
					}{{Tag: "Keep"}, {Tag: ""}, {Tag: "   "}},
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
	names, err := client.GetLabelNames()
	if err != nil {
		t.Fatalf("GetLabelNames should succeed: %v", err)
	}
	if len(names) != 1 || names[0] != "Keep" {
		t.Errorf("Expected [Keep] (blanks excluded), got %v", names)
	}
}

func TestPlexExtractTMDbID(t *testing.T) {
	tests := []struct {
		name  string
		guids []plexGUID
		want  int
	}{
		{"valid TMDb GUID", []plexGUID{{ID: "tmdb://16320"}}, 16320},
		{"TMDb among others", []plexGUID{{ID: "imdb://tt0379786"}, {ID: "tmdb://16320"}, {ID: "tvdb://54321"}}, 16320},
		{"no TMDb GUID", []plexGUID{{ID: "imdb://tt0379786"}, {ID: "tvdb://54321"}}, 0},
		{"empty guids", []plexGUID{}, 0},
		{"nil guids", nil, 0},
		{"malformed TMDb GUID", []plexGUID{{ID: "tmdb://notanumber"}}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := plexExtractTMDbID(tt.guids)
			if got != tt.want {
				t.Errorf("plexExtractTMDbID(%v) = %d, want %d", tt.guids, got, tt.want)
			}
		})
	}
}

// ─── fetchAllHistory tests ──────────────────────────────────────────────────

const testPlexPathHistory = "/status/sessions/history/all"
const testPlexPathAccounts = "/accounts"

// encodePlexHistory writes a JSON history response to w using json.NewEncoder
// (matching the pattern used by all existing test mock handlers).
func encodePlexHistory(t *testing.T, w http.ResponseWriter, totalSize int, entries []plexHistoryEntry) {
	t.Helper()
	resp := plexHistoryResponse{}
	resp.MediaContainer.Size = len(entries)
	resp.MediaContainer.TotalSize = totalSize
	resp.MediaContainer.Metadata = entries
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		t.Fatalf("Failed to encode history response: %v", err)
	}
}

// encodePlexAccounts writes a JSON accounts response to w using json.NewEncoder.
func encodePlexAccounts(t *testing.T, w http.ResponseWriter, accounts []plexAccount) {
	t.Helper()
	resp := plexAccountsResponse{}
	resp.MediaContainer.Account = accounts
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		t.Fatalf("Failed to encode accounts response: %v", err)
	}
}

func TestPlexClient_fetchAllHistory_SinglePage(t *testing.T) {
	entries := []plexHistoryEntry{
		{RatingKey: "101", Type: "movie", ViewedAt: 1700000000, AccountID: 1},
		{RatingKey: "102", Type: "movie", ViewedAt: 1700001000, AccountID: 2},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == testPlexPathHistory {
			encodePlexHistory(t, w, 2, entries)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	history, err := client.fetchAllHistory()
	if err != nil {
		t.Fatalf("fetchAllHistory should succeed: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(history))
	}
	if history[0].RatingKey != "101" {
		t.Errorf("Expected ratingKey '101', got %q", history[0].RatingKey)
	}
	if history[1].AccountID != 2 {
		t.Errorf("Expected accountID 2, got %d", history[1].AccountID)
	}
}

func TestPlexClient_fetchAllHistory_MultiPage(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == testPlexPathHistory {
			start := r.URL.Query().Get("X-Plex-Container-Start")
			callCount++
			if start == "" || start == "0" {
				// First page: return 3 entries, totalSize=5
				entries := []plexHistoryEntry{
					{RatingKey: "101", Type: "movie", ViewedAt: 1700000001, AccountID: 1},
					{RatingKey: "102", Type: "movie", ViewedAt: 1700000002, AccountID: 1},
					{RatingKey: "103", Type: "movie", ViewedAt: 1700000003, AccountID: 2},
				}
				encodePlexHistory(t, w, 5, entries)
			} else {
				// Second page: return remaining 2 entries
				entries := []plexHistoryEntry{
					{RatingKey: "104", Type: "movie", ViewedAt: 1700000004, AccountID: 2},
					{RatingKey: "105", Type: "movie", ViewedAt: 1700000005, AccountID: 3},
				}
				encodePlexHistory(t, w, 5, entries)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	history, err := client.fetchAllHistory()
	if err != nil {
		t.Fatalf("fetchAllHistory should succeed: %v", err)
	}
	if len(history) != 5 {
		t.Fatalf("Expected 5 entries across 2 pages, got %d", len(history))
	}
	if callCount != 2 {
		t.Errorf("Expected 2 API calls for pagination, got %d", callCount)
	}
	// Verify last entry is from second page
	if history[4].RatingKey != "105" {
		t.Errorf("Expected last ratingKey '105', got %q", history[4].RatingKey)
	}
}

func TestPlexClient_fetchAllHistory_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == testPlexPathHistory {
			encodePlexHistory(t, w, 0, nil)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	history, err := client.fetchAllHistory()
	if err != nil {
		t.Fatalf("fetchAllHistory should succeed with empty history: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("Expected 0 entries, got %d", len(history))
	}
}

func TestPlexClient_fetchAllHistory_MidPaginationError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == testPlexPathHistory {
			start := r.URL.Query().Get("X-Plex-Container-Start")
			if start == "" || start == "0" {
				// First page succeeds
				entries := []plexHistoryEntry{
					{RatingKey: "101", Type: "movie", ViewedAt: 1700000001, AccountID: 1},
					{RatingKey: "102", Type: "movie", ViewedAt: 1700000002, AccountID: 2},
				}
				encodePlexHistory(t, w, 10, entries)
			} else {
				// Second page fails
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	history, err := client.fetchAllHistory()
	// Should return partial results without error
	if err != nil {
		t.Fatalf("fetchAllHistory should return partial results on mid-pagination error: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("Expected 2 partial entries, got %d", len(history))
	}
}

func TestPlexClient_fetchAllHistory_FirstPageError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	history, err := client.fetchAllHistory()
	if err == nil {
		t.Fatal("fetchAllHistory should return error when first page fails")
	}
	if len(history) != 0 {
		t.Errorf("Expected 0 entries on first-page error, got %d", len(history))
	}
}

func TestPlexClient_fetchAllHistory_EntryDeserialization(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == testPlexPathHistory {
			entries := []plexHistoryEntry{
				{
					RatingKey:            "301",
					ParentRatingKey:      "300",
					GrandparentRatingKey: "200",
					Type:                 "episode",
					ViewedAt:             1712700000,
					AccountID:            42,
				},
			}
			encodePlexHistory(t, w, 1, entries)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	history, err := client.fetchAllHistory()
	if err != nil {
		t.Fatalf("fetchAllHistory should succeed: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(history))
	}

	entry := history[0]
	if entry.RatingKey != "301" {
		t.Errorf("Expected ratingKey '301', got %q", entry.RatingKey)
	}
	if entry.ParentRatingKey != "300" {
		t.Errorf("Expected parentRatingKey '300', got %q", entry.ParentRatingKey)
	}
	if entry.GrandparentRatingKey != "200" {
		t.Errorf("Expected grandparentRatingKey '200', got %q", entry.GrandparentRatingKey)
	}
	if entry.Type != "episode" {
		t.Errorf("Expected type 'episode', got %q", entry.Type)
	}
	if entry.ViewedAt != 1712700000 {
		t.Errorf("Expected viewedAt 1712700000, got %d", entry.ViewedAt)
	}
	if entry.AccountID != 42 {
		t.Errorf("Expected accountID 42, got %d", entry.AccountID)
	}
}

// ─── GetBulkWatchData multi-user history tests ─────────────────────────────

// newPlexMultiUserMockServer creates an httptest.Server that serves all endpoints
// needed for a full GetBulkWatchData cycle: library sections, library items,
// session history, and accounts. Callers provide history entries and accounts.
func newPlexMultiUserMockServer(
	t *testing.T,
	libraryType string,
	libraryKey string,
	metadata []plexMetadata,
	history []plexHistoryEntry,
	accounts []plexAccount,
) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testPlexPathSections:
			resp := plexLibraryResponse{}
			resp.MediaContainer.Directory = []struct {
				Key   string `json:"key"`
				Title string `json:"title"`
				Type  string `json:"type"`
			}{
				{Key: libraryKey, Title: "Library", Type: libraryType},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
		case "/library/sections/" + libraryKey + "/all":
			resp := plexMediaResponse{}
			resp.MediaContainer.Metadata = metadata
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
		case testPlexPathHistory:
			encodePlexHistory(t, w, len(history), history)
		case testPlexPathAccounts:
			encodePlexAccounts(t, w, accounts)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestPlexClient_GetBulkWatchData_MultiUserAggregation(t *testing.T) {
	// Library has one movie: Serenity (ratingKey=101, TMDb=16320)
	metadata := []plexMetadata{
		{
			RatingKey: "101",
			Title:     "Serenity",
			Year:      2005,
			Type:      "movie",
			ViewCount: 1, // Admin's own view count (should be ignored)
			GUIDs:     []plexGUID{{ID: "tmdb://16320"}},
		},
	}
	// Three users' play events: user A watched 2x, user B 1x, user C 3x
	history := []plexHistoryEntry{
		{RatingKey: "101", Type: "movie", ViewedAt: 1700000001, AccountID: 1},
		{RatingKey: "101", Type: "movie", ViewedAt: 1700000002, AccountID: 1},
		{RatingKey: "101", Type: "movie", ViewedAt: 1700000003, AccountID: 2},
		{RatingKey: "101", Type: "movie", ViewedAt: 1700000004, AccountID: 3},
		{RatingKey: "101", Type: "movie", ViewedAt: 1700000005, AccountID: 3},
		{RatingKey: "101", Type: "movie", ViewedAt: 1700000006, AccountID: 3},
	}
	accounts := []plexAccount{
		{ID: 1, Name: "mal"},
		{ID: 2, Name: "wash"},
		{ID: 3, Name: "zoe"},
	}

	srv := newPlexMultiUserMockServer(t, "movie", "1", metadata, history, accounts)
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	watchMap, err := client.GetBulkWatchData()
	if err != nil {
		t.Fatalf("GetBulkWatchData should succeed: %v", err)
	}

	wd, ok := watchMap[16320]
	if !ok {
		t.Fatal("Expected TMDb ID 16320 in watch map")
	}

	// Total play count across all users: 2+1+3 = 6
	if wd.PlayCount != 6 {
		t.Errorf("Expected PlayCount 6 (aggregated across 3 users), got %d", wd.PlayCount)
	}

	// LastPlayed should be the most recent viewedAt
	if wd.LastPlayed == nil {
		t.Fatal("Expected LastPlayed to be set")
	}
	if wd.LastPlayed.Unix() != 1700000006 {
		t.Errorf("Expected LastPlayed at 1700000006, got %d", wd.LastPlayed.Unix())
	}

	// Users should contain all 3 unique users, sorted alphabetically
	if len(wd.Users) != 3 {
		t.Fatalf("Expected 3 users, got %d: %v", len(wd.Users), wd.Users)
	}
	expectedUsers := []string{"mal", "wash", "zoe"}
	for i, u := range wd.Users {
		if u != expectedUsers[i] {
			t.Errorf("Expected Users[%d]=%q, got %q", i, expectedUsers[i], u)
		}
	}
}

func TestPlexClient_GetBulkWatchData_EpisodeAggregation(t *testing.T) {
	// Library has one show: Firefly (ratingKey=200, TMDb=1437)
	metadata := []plexMetadata{
		{
			RatingKey: "200",
			Title:     "Firefly",
			Year:      2002,
			Type:      "show",
			GUIDs:     []plexGUID{{ID: "tmdb://1437"}},
		},
	}
	// Episode watches should aggregate under grandparentRatingKey (the show)
	history := []plexHistoryEntry{
		{RatingKey: "301", ParentRatingKey: "210", GrandparentRatingKey: "200", Type: "episode", ViewedAt: 1700000001, AccountID: 1},
		{RatingKey: "302", ParentRatingKey: "210", GrandparentRatingKey: "200", Type: "episode", ViewedAt: 1700000002, AccountID: 1},
		{RatingKey: "303", ParentRatingKey: "220", GrandparentRatingKey: "200", Type: "episode", ViewedAt: 1700000003, AccountID: 2},
	}
	accounts := []plexAccount{
		{ID: 1, Name: "mal"},
		{ID: 2, Name: "wash"},
	}

	srv := newPlexMultiUserMockServer(t, "show", "2", metadata, history, accounts)
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	watchMap, err := client.GetBulkWatchData()
	if err != nil {
		t.Fatalf("GetBulkWatchData should succeed: %v", err)
	}

	wd, ok := watchMap[1437]
	if !ok {
		t.Fatal("Expected TMDb ID 1437 (Firefly) in watch map")
	}

	// Total episode plays: 3 (all under grandparent show ratingKey 200)
	if wd.PlayCount != 3 {
		t.Errorf("Expected PlayCount 3 (episodes aggregated under show), got %d", wd.PlayCount)
	}

	// Two unique users watched episodes
	if len(wd.Users) != 2 {
		t.Fatalf("Expected 2 users, got %d: %v", len(wd.Users), wd.Users)
	}
}

func TestPlexClient_GetBulkWatchData_HistoryFallback(t *testing.T) {
	// History endpoint fails, should fall back to per-token viewCount
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
					Year:         2005,
					Type:         "movie",
					ViewCount:    5,
					LastViewedAt: 1700000000,
					GUIDs:        []plexGUID{{ID: "tmdb://16320"}},
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
		case testPlexPathHistory:
			// History endpoint returns 500 — triggers fallback
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	watchMap, err := client.GetBulkWatchData()
	if err != nil {
		t.Fatalf("GetBulkWatchData should succeed via fallback: %v", err)
	}

	wd, ok := watchMap[16320]
	if !ok {
		t.Fatal("Expected TMDb ID 16320 in watch map (fallback)")
	}

	// Fallback uses viewCount from library metadata
	if wd.PlayCount != 5 {
		t.Errorf("Expected fallback PlayCount 5, got %d", wd.PlayCount)
	}
	if wd.LastPlayed == nil {
		t.Fatal("Expected fallback LastPlayed to be set")
	}
	// Fallback does not populate Users
	if len(wd.Users) != 0 {
		t.Errorf("Expected no Users in fallback, got %v", wd.Users)
	}
}

func TestPlexClient_GetBulkWatchData_AccountsFallback(t *testing.T) {
	// Accounts endpoint fails, should use numeric account IDs ("account:N")
	metadata := []plexMetadata{
		{
			RatingKey: "101",
			Title:     "Serenity",
			Year:      2005,
			Type:      "movie",
			GUIDs:     []plexGUID{{ID: "tmdb://16320"}},
		},
	}
	history := []plexHistoryEntry{
		{RatingKey: "101", Type: "movie", ViewedAt: 1700000001, AccountID: 42},
	}

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
			resp.MediaContainer.Metadata = metadata
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
		case testPlexPathHistory:
			encodePlexHistory(t, w, len(history), history)
		case testPlexPathAccounts:
			// Accounts endpoint returns 500
			w.WriteHeader(http.StatusInternalServerError)
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

	wd, ok := watchMap[16320]
	if !ok {
		t.Fatal("Expected TMDb ID 16320 in watch map")
	}
	if wd.PlayCount != 1 {
		t.Errorf("Expected PlayCount 1, got %d", wd.PlayCount)
	}
	// Without account names, should use "account:N" format
	if len(wd.Users) != 1 || wd.Users[0] != "account:42" {
		t.Errorf("Expected Users=[account:42], got %v", wd.Users)
	}
}

func TestPlexClient_GetBulkWatchData_UnwatchedItemsIncluded(t *testing.T) {
	// Library has 2 movies, but only 1 has history entries
	metadata := []plexMetadata{
		{
			RatingKey: "101",
			Title:     "Serenity",
			Year:      2005,
			Type:      "movie",
			GUIDs:     []plexGUID{{ID: "tmdb://16320"}},
		},
		{
			RatingKey: "102",
			Title:     "Serenity 2",
			Year:      2014,
			Type:      "movie",
			GUIDs:     []plexGUID{{ID: "tmdb://99999"}},
		},
	}
	history := []plexHistoryEntry{
		{RatingKey: "101", Type: "movie", ViewedAt: 1700000001, AccountID: 1},
	}
	accounts := []plexAccount{{ID: 1, Name: "mal"}}

	srv := newPlexMultiUserMockServer(t, "movie", "1", metadata, history, accounts)
	defer srv.Close()

	client := NewPlexClient(srv.URL, "test-token")
	watchMap, err := client.GetBulkWatchData()
	if err != nil {
		t.Fatalf("GetBulkWatchData should succeed: %v", err)
	}

	// Both items should be in the map
	if len(watchMap) != 2 {
		t.Fatalf("Expected 2 entries (watched + unwatched), got %d", len(watchMap))
	}

	// Watched item
	watched, ok := watchMap[16320]
	if !ok {
		t.Fatal("Expected TMDb ID 16320 in watch map")
	}
	if watched.PlayCount != 1 {
		t.Errorf("Expected PlayCount 1, got %d", watched.PlayCount)
	}

	// Unwatched item should have zero play count
	unwatched, ok := watchMap[99999]
	if !ok {
		t.Fatal("Expected TMDb ID 99999 in watch map")
	}
	if unwatched.PlayCount != 0 {
		t.Errorf("Expected PlayCount 0 for unwatched item, got %d", unwatched.PlayCount)
	}
}
