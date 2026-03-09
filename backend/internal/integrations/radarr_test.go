package integrations

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	testRadarrPathStatus  = "/api/v3/system/status"
	testRadarrPathQuality = "/api/v3/qualityprofile"
	testRadarrPathTag     = "/api/v3/tag"
)

func TestRadarrClient_TestConnection_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testRadarrPathStatus {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != testTautulliAPIKey {
			t.Errorf("Missing or wrong API key header")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"4.0.0"}`))
	}))
	defer srv.Close()

	client := NewRadarrClient(srv.URL, testTautulliAPIKey)
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should succeed: %v", err)
	}
}

func TestRadarrClient_TestConnection_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewRadarrClient(srv.URL, "bad-key")
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 401")
	}
}

func TestRadarrClient_TestConnection_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewRadarrClient(srv.URL, testTautulliAPIKey)
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 500")
	}
}

func TestRadarrClient_GetDiskSpace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/diskspace" {
			resp := []arrDiskSpace{
				{Path: "/media/movies", TotalSpace: 2000000000000, FreeSpace: 500000000000},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode response: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewRadarrClient(srv.URL, testTautulliAPIKey)
	disks, err := client.GetDiskSpace()
	if err != nil {
		t.Fatalf("GetDiskSpace should succeed: %v", err)
	}
	if len(disks) != 1 {
		t.Fatalf("Expected 1 disk, got %d", len(disks))
	}
	if disks[0].Path != "/media/movies" {
		t.Errorf("Expected path '/media/movies', got %q", disks[0].Path)
	}
	if disks[0].TotalBytes != 2000000000000 {
		t.Errorf("Expected TotalBytes 2000000000000, got %d", disks[0].TotalBytes)
	}
}

func TestRadarrClient_GetMediaItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testRadarrPathQuality:
			resp := []arrQualityProfile{
				{ID: 1, Name: "HD-1080p"},
				{ID: 2, Name: "Ultra-HD"},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode response: %v", err)
			}
		case testRadarrPathTag:
			resp := []arrTag{
				{ID: 1, Label: "christmas"},
				{ID: 2, Label: "classic"},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode response: %v", err)
			}
		case "/api/v3/movie":
			resp := []radarrMovie{
				{
					ID:               1,
					Title:            "Serenity",
					Year:             2010,
					Path:             "/media/movies/Serenity (2010)",
					Monitored:        true,
					HasFile:          true,
					SizeOnDisk:       8000000000,
					Genres:           []string{"action", "sci-fi"},
					Tags:             []int{2},
					QualityProfileID: 1,
					Added:            "2023-06-01T00:00:00Z",
					Ratings: struct {
						IMDB struct {
							Value float64 `json:"value"`
						} `json:"imdb"`
						TMDB struct {
							Value float64 `json:"value"`
						} `json:"tmdb"`
					}{
						IMDB: struct {
							Value float64 `json:"value"`
						}{Value: 8.8},
					},
				},
				{
					// Movie with TMDB rating only (IMDB = 0)
					ID:         2,
					Title:      "Serenity 2",
					HasFile:    true,
					SizeOnDisk: 5000000000,
					Ratings: struct {
						IMDB struct {
							Value float64 `json:"value"`
						} `json:"imdb"`
						TMDB struct {
							Value float64 `json:"value"`
						} `json:"tmdb"`
					}{
						TMDB: struct {
							Value float64 `json:"value"`
						}{Value: 7.2},
					},
				},
				{
					// Movie without file — should be skipped
					ID:      3,
					Title:   "Serenity 3",
					HasFile: false,
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode response: %v", err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewRadarrClient(srv.URL, testTautulliAPIKey)
	items, err := client.GetMediaItems()
	if err != nil {
		t.Fatalf("GetMediaItems should succeed: %v", err)
	}

	// Expect 2 movies (No File has HasFile=false)
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
	if movie.Rating != 8.8 {
		t.Errorf("Expected IMDB rating 8.8, got %v", movie.Rating)
	}
	if movie.QualityProfile != "HD-1080p" {
		t.Errorf("Expected quality 'HD-1080p', got %q", movie.QualityProfile)
	}
	if len(movie.Tags) != 1 || movie.Tags[0] != "classic" {
		t.Errorf("Expected tags [classic], got %v", movie.Tags)
	}
	if movie.Year != 2010 {
		t.Errorf("Expected year 2010, got %d", movie.Year)
	}

	// Second movie — TMDB fallback rating
	tmdbOnly := items[1]
	if tmdbOnly.Rating != 7.2 {
		t.Errorf("Expected TMDB fallback rating 7.2, got %v", tmdbOnly.Rating)
	}
}

func TestRadarrClient_GetMediaItems_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testRadarrPathQuality:
			_, _ = w.Write([]byte(`[]`))
		case testRadarrPathTag:
			_, _ = w.Write([]byte(`[]`))
		case "/api/v3/movie":
			_, _ = w.Write([]byte(`not json at all`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewRadarrClient(srv.URL, testTautulliAPIKey)
	_, err := client.GetMediaItems()
	if err == nil {
		t.Fatal("Expected error for malformed JSON")
	}
}

func TestRadarrClient_GetMediaItems_EmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testRadarrPathQuality:
			_, _ = w.Write([]byte(`[]`))
		case testRadarrPathTag:
			_, _ = w.Write([]byte(`[]`))
		case "/api/v3/movie":
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewRadarrClient(srv.URL, testTautulliAPIKey)
	items, err := client.GetMediaItems()
	if err != nil {
		t.Fatalf("GetMediaItems should succeed with empty: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(items))
	}
}

func TestRadarrClient_GetRootFolders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/rootfolder" {
			resp := []arrRootFolder{
				{Path: "/media/movies"},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewRadarrClient(srv.URL, testTautulliAPIKey)
	folders, err := client.GetRootFolders()
	if err != nil {
		t.Fatalf("GetRootFolders should succeed: %v", err)
	}
	if len(folders) != 1 || folders[0] != "/media/movies" {
		t.Errorf("Expected [/media/movies], got %v", folders)
	}
}

func TestRadarrClient_GetQualityProfiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testRadarrPathQuality {
			resp := []arrQualityProfile{{ID: 1, Name: "Any"}, {ID: 2, Name: "Ultra-HD"}}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewRadarrClient(srv.URL, testTautulliAPIKey)
	profiles, err := client.GetQualityProfiles()
	if err != nil {
		t.Fatalf("GetQualityProfiles should succeed: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("Expected 2 profiles, got %d", len(profiles))
	}
}

func TestRadarrClient_GetTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testRadarrPathTag {
			resp := []arrTag{{ID: 1, Label: "action"}}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewRadarrClient(srv.URL, testTautulliAPIKey)
	tags, err := client.GetTags()
	if err != nil {
		t.Fatalf("GetTags should succeed: %v", err)
	}
	if len(tags) != 1 || tags[0].Value != "action" {
		t.Errorf("Expected [action], got %v", tags)
	}
}

func TestRadarrClient_URLTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testRadarrPathStatus {
			t.Errorf("Expected /api/v3/system/status, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewRadarrClient(srv.URL+"/", testTautulliAPIKey)
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should succeed: %v", err)
	}
}
