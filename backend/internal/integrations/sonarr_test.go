package integrations

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSonarrClient_TestConnection_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v3/system/status" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != testTautulliAPIKey {
			t.Errorf("Missing or wrong API key header")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"3.0.0"}`))
	}))
	defer srv.Close()

	client := NewSonarrClient(srv.URL, testTautulliAPIKey)
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should succeed: %v", err)
	}
}

func TestSonarrClient_TestConnection_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewSonarrClient(srv.URL, "bad-key")
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 401")
	}
}

func TestSonarrClient_TestConnection_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewSonarrClient(srv.URL, testTautulliAPIKey)
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 500")
	}
}

func TestSonarrClient_TestConnection_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Use a very short timeout client
	origClient := sharedHTTPClient
	sharedHTTPClient = &http.Client{Timeout: 10 * time.Millisecond}
	defer func() { sharedHTTPClient = origClient }()

	client := NewSonarrClient(srv.URL, testTautulliAPIKey)
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with timeout")
	}
}

func TestSonarrClient_GetDiskSpace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/diskspace" {
			resp := []arrDiskSpace{
				{Path: "/media/tv", TotalSpace: 1000000000000, FreeSpace: 300000000000},
				{Path: "/media/anime", TotalSpace: 500000000000, FreeSpace: 100000000000},
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

	client := NewSonarrClient(srv.URL, testTautulliAPIKey)
	disks, err := client.GetDiskSpace()
	if err != nil {
		t.Fatalf("GetDiskSpace should succeed: %v", err)
	}
	if len(disks) != 2 {
		t.Fatalf("Expected 2 disks, got %d", len(disks))
	}
	if disks[0].Path != "/media/tv" {
		t.Errorf("Expected path '/media/tv', got %q", disks[0].Path)
	}
	if disks[0].TotalBytes != 1000000000000 {
		t.Errorf("Expected TotalBytes 1000000000000, got %d", disks[0].TotalBytes)
	}
	if disks[0].FreeBytes != 300000000000 {
		t.Errorf("Expected FreeBytes 300000000000, got %d", disks[0].FreeBytes)
	}
}

func TestSonarrClient_GetMediaItems(t *testing.T) {
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
				{ID: 1, Label: "anime"},
				{ID: 2, Label: "classic"},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode response: %v", err)
			}
		case "/api/v3/series":
			resp := []sonarrSeries{
				{
					ID:               1,
					Title:            "Firefly",
					Year:             2008,
					Path:             "/media/tv/Firefly",
					Monitored:        true,
					Status:           "ended",
					Genres:           []string{"drama", "thriller"},
					Tags:             []int{1},
					QualityProfileID: 1,
					Added:            "2023-01-15T00:00:00Z",
					Ratings: struct {
						Value float64 `json:"value"`
					}{Value: 9.5},
					Statistics: struct {
						SizeOnDisk   int64 `json:"sizeOnDisk"`
						SeasonCount  int   `json:"seasonCount"`
						EpisodeCount int   `json:"episodeCount"`
					}{SizeOnDisk: 50000000000, SeasonCount: 5, EpisodeCount: 62},
					Seasons: []sonarrSeason{
						{
							SeasonNumber: 1,
							Monitored:    true,
							Statistics: struct {
								SizeOnDisk        int64 `json:"sizeOnDisk"`
								EpisodeFileCount  int   `json:"episodeFileCount"`
								TotalEpisodeCount int   `json:"totalEpisodeCount"`
							}{SizeOnDisk: 10000000000, EpisodeFileCount: 7, TotalEpisodeCount: 7},
						},
						{
							SeasonNumber: 0, // Specials — should be skipped
							Monitored:    false,
							Statistics: struct {
								SizeOnDisk        int64 `json:"sizeOnDisk"`
								EpisodeFileCount  int   `json:"episodeFileCount"`
								TotalEpisodeCount int   `json:"totalEpisodeCount"`
							}{SizeOnDisk: 500000000, EpisodeFileCount: 2, TotalEpisodeCount: 2},
						},
					},
				},
				{
					// Show with zero disk usage — should be skipped entirely
					ID:    2,
					Title: "Firefly 2",
					Statistics: struct {
						SizeOnDisk   int64 `json:"sizeOnDisk"`
						SeasonCount  int   `json:"seasonCount"`
						EpisodeCount int   `json:"episodeCount"`
					}{SizeOnDisk: 0},
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

	client := NewSonarrClient(srv.URL, testTautulliAPIKey)
	items, err := client.GetMediaItems()
	if err != nil {
		t.Fatalf("GetMediaItems should succeed: %v", err)
	}

	// Expect: 1 season + 1 show-level item (Firefly)
	// Specials (season 0) skipped, Empty Show skipped
	if len(items) != 2 {
		t.Fatalf("Expected 2 items (1 season + 1 show), got %d", len(items))
	}

	// First item: Season 1
	season := items[0]
	if season.Type != MediaTypeSeason {
		t.Errorf("Expected MediaTypeSeason, got %v", season.Type)
	}
	if season.Title != "Firefly - Season 1" {
		t.Errorf("Expected 'Firefly - Season 1', got %q", season.Title)
	}
	if season.QualityProfile != "HD-1080p" {
		t.Errorf("Expected quality profile 'HD-1080p', got %q", season.QualityProfile)
	}
	if len(season.Tags) != 1 || season.Tags[0] != "anime" {
		t.Errorf("Expected tags [anime], got %v", season.Tags)
	}
	if season.SizeBytes != 10000000000 {
		t.Errorf("Expected SizeBytes 10000000000, got %d", season.SizeBytes)
	}

	// Second item: Show-level
	show := items[1]
	if show.Type != MediaTypeShow {
		t.Errorf("Expected MediaTypeShow, got %v", show.Type)
	}
	if show.Title != "Firefly" {
		t.Errorf("Expected 'Firefly', got %q", show.Title)
	}
	if show.Rating != 9.5 {
		t.Errorf("Expected rating 9.5, got %v", show.Rating)
	}
}

func TestSonarrClient_GetMediaItems_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testRadarrPathQuality:
			_, _ = w.Write([]byte(`[{"id":1,"name":"HD"}]`))
		case testRadarrPathTag:
			_, _ = w.Write([]byte(`[]`))
		case "/api/v3/series":
			_, _ = w.Write([]byte(`{not valid json`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewSonarrClient(srv.URL, testTautulliAPIKey)
	_, err := client.GetMediaItems()
	if err == nil {
		t.Fatal("Expected error for malformed JSON")
	}
}

func TestSonarrClient_GetMediaItems_EmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testRadarrPathQuality:
			_, _ = w.Write([]byte(`[]`))
		case testRadarrPathTag:
			_, _ = w.Write([]byte(`[]`))
		case "/api/v3/series":
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewSonarrClient(srv.URL, testTautulliAPIKey)
	items, err := client.GetMediaItems()
	if err != nil {
		t.Fatalf("GetMediaItems should succeed with empty results: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(items))
	}
}

func TestSonarrClient_GetRootFolders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v3/rootfolder" {
			resp := []arrRootFolder{
				{Path: "/media/tv"},
				{Path: "/media/anime"},
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

	client := NewSonarrClient(srv.URL, testTautulliAPIKey)
	folders, err := client.GetRootFolders()
	if err != nil {
		t.Fatalf("GetRootFolders should succeed: %v", err)
	}
	if len(folders) != 2 {
		t.Fatalf("Expected 2 folders, got %d", len(folders))
	}
	if folders[0] != "/media/tv" {
		t.Errorf("Expected first folder '/media/tv', got %q", folders[0])
	}
}

func TestSonarrClient_GetQualityProfiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testRadarrPathQuality {
			resp := []arrQualityProfile{
				{ID: 1, Name: "HD-1080p"},
				{ID: 2, Name: "Ultra-HD"},
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

	client := NewSonarrClient(srv.URL, testTautulliAPIKey)
	profiles, err := client.GetQualityProfiles()
	if err != nil {
		t.Fatalf("GetQualityProfiles should succeed: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("Expected 2 profiles, got %d", len(profiles))
	}
	if profiles[0].Value != "HD-1080p" {
		t.Errorf("Expected first profile 'HD-1080p', got %q", profiles[0].Value)
	}
}

func TestSonarrClient_GetTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testRadarrPathTag {
			resp := []arrTag{
				{ID: 1, Label: "anime"},
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

	client := NewSonarrClient(srv.URL, testTautulliAPIKey)
	tags, err := client.GetTags()
	if err != nil {
		t.Fatalf("GetTags should succeed: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("Expected 1 tag, got %d", len(tags))
	}
	if tags[0].Value != "anime" {
		t.Errorf("Expected tag 'anime', got %q", tags[0].Value)
	}
}

func TestSonarrClient_HTMLResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><body>Login Page</body></html>`))
	}))
	defer srv.Close()

	client := NewSonarrClient(srv.URL, testTautulliAPIKey)
	err := client.TestConnection()
	if err == nil {
		t.Fatal("Expected error for HTML response (reverse proxy login page)")
	}
}

func TestSonarrClient_URLTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify no double slashes
		if r.URL.Path != "/api/v3/system/status" {
			t.Errorf("Expected /api/v3/system/status, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	// URL with trailing slash should be normalized
	client := NewSonarrClient(srv.URL+"/", testTautulliAPIKey)
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should succeed: %v", err)
	}
}
