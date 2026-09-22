package integrations

import (
	"encoding/json"
	"fmt"
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
					TmdbID:           1437,
					Path:             "/media/tv/Firefly",
					Monitored:        true,
					Status:           "ended",
					Genres:           []string{"drama", "thriller"},
					Tags:             []int{1},
					QualityProfileID: 1,
					Added:            "2023-01-15T00:00:00Z",
					OriginalLanguage: arrLanguage{ID: 1, Name: "English"},
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
		case "/api/v3/episodefile":
			// Bulk (no seriesId) and per-series both return series 1 files so
			// AddedAt fixtures stay the same on either fetch path.
			seriesID := r.URL.Query().Get("seriesId")
			if seriesID == "" || seriesID == "1" {
				resp := []sonarrEpisodeFile{
					{ID: 101, SeriesID: 1, SeasonNumber: 1, DateAdded: "2023-06-15T12:00:00Z"},
					{ID: 102, SeriesID: 1, SeasonNumber: 1, DateAdded: "2023-06-20T12:00:00Z"}, // Latest for S1
					{ID: 103, SeriesID: 1, SeasonNumber: 0, DateAdded: "2023-03-01T00:00:00Z"}, // Specials
				}
				if err := json.NewEncoder(w).Encode(resp); err != nil {
					t.Fatalf("Failed to encode response: %v", err)
				}
			} else {
				_, _ = w.Write([]byte(`[]`))
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

	// First item: Season 1 — should use episode file dateAdded
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
	if season.TMDbID != 1437 {
		t.Errorf("Expected TMDbID 1437, got %d", season.TMDbID)
	}
	if season.Language != "English" {
		t.Errorf("Expected Language 'English', got %q", season.Language)
	}
	// AddedAt should be from episodefile (Jun 20), NOT series.added (Jan 15)
	if season.AddedAt == nil {
		t.Fatal("Expected AddedAt to be set for Season 1")
	}
	if season.AddedAt.Month() != 6 || season.AddedAt.Day() != 20 {
		t.Errorf("Expected AddedAt from episodefile (Jun 20), got %v", season.AddedAt)
	}

	// Second item: Show-level — should use latest file date across all seasons
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
	if show.TMDbID != 1437 {
		t.Errorf("Expected TMDbID 1437, got %d", show.TMDbID)
	}
	if show.Language != "English" {
		t.Errorf("Expected Language 'English', got %q", show.Language)
	}
	// Show-level AddedAt should use latest file date (Jun 20)
	if show.AddedAt == nil {
		t.Fatal("Expected AddedAt to be set for show")
	}
	if show.AddedAt.Month() != 6 || show.AddedAt.Day() != 20 {
		t.Errorf("Expected show AddedAt from latest episode file (Jun 20), got %v", show.AddedAt)
	}
}

func TestSonarrResolveAddedAt(t *testing.T) {
	jun15 := time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC)
	jun20 := time.Date(2023, 6, 20, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name         string
		seasonDates  map[int]time.Time
		seasonNumber int
		showAdded    string
		wantNil      bool
		wantMonth    int
		wantDay      int
	}{
		{
			name:         "prefers file date over show.added",
			seasonDates:  map[int]time.Time{1: jun20},
			seasonNumber: 1,
			showAdded:    "2023-01-15T00:00:00Z",
			wantMonth:    6,
			wantDay:      20,
		},
		{
			name:         "falls back to show.added when no file date for season",
			seasonDates:  map[int]time.Time{2: jun15}, // Only season 2 has dates
			seasonNumber: 1,                           // Season 1 has no dates
			showAdded:    "2023-01-15T00:00:00Z",
			wantMonth:    1,
			wantDay:      15,
		},
		{
			name:         "falls back to show.added when seasonDates is nil",
			seasonDates:  nil,
			seasonNumber: 1,
			showAdded:    "2023-03-20T00:00:00Z",
			wantMonth:    3,
			wantDay:      20,
		},
		{
			name:         "returns nil when both are empty",
			seasonDates:  nil,
			seasonNumber: 1,
			showAdded:    "",
			wantNil:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := sonarrResolveAddedAt(tc.seasonDates, tc.seasonNumber, tc.showAdded)
			if tc.wantNil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
				return
			}
			if result == nil {
				t.Fatal("Expected non-nil result")
			}
			if int(result.Month()) != tc.wantMonth || result.Day() != tc.wantDay {
				t.Errorf("Expected month=%d day=%d, got %v", tc.wantMonth, tc.wantDay, result)
			}
		})
	}
}

func TestSonarrResolveShowAddedAt(t *testing.T) {
	jun15 := time.Date(2023, 6, 15, 0, 0, 0, 0, time.UTC)
	sep01 := time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		dates     map[int]time.Time
		showAdded string
		wantNil   bool
		wantMonth int
	}{
		{
			name:      "uses latest file date across seasons",
			dates:     map[int]time.Time{1: jun15, 2: sep01},
			showAdded: "2023-01-01T00:00:00Z",
			wantMonth: 9, // Sep is latest
		},
		{
			name:      "falls back to show.added when no file dates",
			dates:     map[int]time.Time{},
			showAdded: "2023-02-10T00:00:00Z",
			wantMonth: 2,
		},
		{
			name:      "returns nil when both empty",
			dates:     nil,
			showAdded: "",
			wantNil:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := sonarrResolveShowAddedAt(tc.dates, tc.showAdded)
			if tc.wantNil {
				if result != nil {
					t.Errorf("Expected nil, got %v", result)
				}
				return
			}
			if result == nil {
				t.Fatal("Expected non-nil result")
			}
			if int(result.Month()) != tc.wantMonth {
				t.Errorf("Expected month %d, got %d", tc.wantMonth, result.Month())
			}
		})
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
		case "/api/v3/episodefile":
			_, _ = w.Write([]byte(`[]`))
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

func TestSonarrClient_GetMediaItems_EpisodeFileAPIError(t *testing.T) {
	// When the episodefile endpoint fails, GetMediaItems should still succeed
	// and fall back to series.added for AddedAt dates.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testRadarrPathQuality:
			_, _ = w.Write([]byte(`[{"id":1,"name":"HD-1080p"}]`))
		case testRadarrPathTag:
			_, _ = w.Write([]byte(`[]`))
		case "/api/v3/series":
			resp := []sonarrSeries{
				{
					ID:     1,
					Title:  "Firefly",
					Year:   2002,
					TmdbID: 1437,
					Path:   "/media/tv/Firefly",
					Added:  "2023-01-15T00:00:00Z",
					Status: "ended",
					Ratings: struct {
						Value float64 `json:"value"`
					}{Value: 9.0},
					Statistics: struct {
						SizeOnDisk   int64 `json:"sizeOnDisk"`
						SeasonCount  int   `json:"seasonCount"`
						EpisodeCount int   `json:"episodeCount"`
					}{SizeOnDisk: 50000000000, EpisodeCount: 14},
					Seasons: []sonarrSeason{
						{
							SeasonNumber: 1,
							Monitored:    true,
							Statistics: struct {
								SizeOnDisk        int64 `json:"sizeOnDisk"`
								EpisodeFileCount  int   `json:"episodeFileCount"`
								TotalEpisodeCount int   `json:"totalEpisodeCount"`
							}{SizeOnDisk: 50000000000, EpisodeFileCount: 14},
						},
					},
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode response: %v", err)
			}
		case "/api/v3/episodefile":
			// Simulate API error
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewSonarrClient(srv.URL, testTautulliAPIKey)
	items, err := client.GetMediaItems()
	if err != nil {
		t.Fatalf("GetMediaItems should succeed even when episodefile API fails: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("Expected 2 items (1 season + 1 show), got %d", len(items))
	}

	// Both should fall back to series.added (Jan 15) since episodefile failed
	for _, item := range items {
		if item.AddedAt == nil {
			t.Errorf("Expected AddedAt to be set (fallback to series.added) for %q", item.Title)
			continue
		}
		if item.AddedAt.Month() != 1 || item.AddedAt.Day() != 15 {
			t.Errorf("Expected AddedAt Jan 15 (series.added fallback), got %v for %q", item.AddedAt, item.Title)
		}
	}
}

func TestSonarrFetchEpisodeFileDates(t *testing.T) {
	t.Run("returns nil on API error", func(t *testing.T) {
		doRequest := func(_ string) ([]byte, error) {
			return nil, fmt.Errorf("connection refused")
		}
		result := sonarrFetchEpisodeFileDates(doRequest, 1)
		if result != nil {
			t.Errorf("Expected nil on API error, got %v", result)
		}
	})

	t.Run("returns nil on malformed JSON", func(t *testing.T) {
		doRequest := func(_ string) ([]byte, error) {
			return []byte(`{broken`), nil
		}
		result := sonarrFetchEpisodeFileDates(doRequest, 1)
		if result != nil {
			t.Errorf("Expected nil on malformed JSON, got %v", result)
		}
	})

	t.Run("returns empty map for empty file list", func(t *testing.T) {
		doRequest := func(_ string) ([]byte, error) {
			return []byte(`[]`), nil
		}
		result := sonarrFetchEpisodeFileDates(doRequest, 1)
		if result == nil {
			t.Fatal("Expected non-nil map for empty file list")
		}
		if len(result) != 0 {
			t.Errorf("Expected empty map, got %v", result)
		}
	})

	t.Run("skips files with empty dateAdded", func(t *testing.T) {
		doRequest := func(_ string) ([]byte, error) {
			return []byte(`[{"id":1,"seasonNumber":1,"dateAdded":""},{"id":2,"seasonNumber":1,"dateAdded":"2024-06-15T12:00:00Z"}]`), nil
		}
		result := sonarrFetchEpisodeFileDates(doRequest, 1)
		if result == nil {
			t.Fatal("Expected non-nil map")
		}
		if len(result) != 1 {
			t.Fatalf("Expected 1 season entry, got %d", len(result))
		}
		if d, ok := result[1]; !ok {
			t.Error("Expected season 1 entry")
		} else if d.Month() != 6 || d.Day() != 15 {
			t.Errorf("Expected Jun 15, got %v", d)
		}
	})

	t.Run("picks latest dateAdded per season", func(t *testing.T) {
		doRequest := func(_ string) ([]byte, error) {
			return []byte(`[
				{"id":1,"seasonNumber":1,"dateAdded":"2024-01-10T00:00:00Z"},
				{"id":2,"seasonNumber":1,"dateAdded":"2024-06-20T00:00:00Z"},
				{"id":3,"seasonNumber":2,"dateAdded":"2024-03-05T00:00:00Z"}
			]`), nil
		}
		result := sonarrFetchEpisodeFileDates(doRequest, 1)
		if result == nil {
			t.Fatal("Expected non-nil map")
		}
		if len(result) != 2 {
			t.Fatalf("Expected 2 season entries, got %d", len(result))
		}
		// Season 1 should have Jun 20 (latest)
		if d := result[1]; d.Month() != 6 || d.Day() != 20 {
			t.Errorf("Expected season 1 latest Jun 20, got %v", d)
		}
		// Season 2 should have Mar 5
		if d := result[2]; d.Month() != 3 || d.Day() != 5 {
			t.Errorf("Expected season 2 Mar 5, got %v", d)
		}
	})
}

func TestSonarrIndexEpisodeFiles(t *testing.T) {
	files := []sonarrEpisodeFile{
		{ID: 1, SeriesID: 10, SeasonNumber: 1, DateAdded: "2024-01-10T00:00:00Z"},
		{ID: 2, SeriesID: 10, SeasonNumber: 1, DateAdded: "2024-06-20T00:00:00Z"},
		{ID: 3, SeriesID: 10, SeasonNumber: 2, DateAdded: "2024-03-05T00:00:00Z"},
		{ID: 4, SeriesID: 20, SeasonNumber: 1, DateAdded: "2024-07-01T00:00:00Z"},
		{ID: 5, SeriesID: 20, SeasonNumber: 1, DateAdded: ""},
		{ID: 6, SeriesID: 20, SeasonNumber: 2, DateAdded: "not-a-date"},
	}

	index := sonarrIndexEpisodeFiles(files)
	if len(index) != 2 {
		t.Fatalf("Expected 2 series entries, got %d", len(index))
	}
	if d := index[10][1]; d.Month() != 6 || d.Day() != 20 {
		t.Errorf("Expected series 10 season 1 Jun 20, got %v", d)
	}
	if d := index[10][2]; d.Month() != 3 || d.Day() != 5 {
		t.Errorf("Expected series 10 season 2 Mar 5, got %v", d)
	}
	if d := index[20][1]; d.Month() != 7 || d.Day() != 1 {
		t.Errorf("Expected series 20 season 1 Jul 1, got %v", d)
	}
	if _, ok := index[20][2]; ok {
		t.Error("Expected unparseable dateAdded to be skipped")
	}
}

func TestSonarrFetchAllEpisodeFileDates(t *testing.T) {
	t.Run("returns error on API failure", func(t *testing.T) {
		doRequest := func(endpoint string) ([]byte, error) {
			if endpoint != "/api/v3/episodefile" {
				t.Errorf("Expected bulk /api/v3/episodefile, got %s", endpoint)
			}
			return nil, fmt.Errorf("unexpected status: 400")
		}
		_, err := sonarrFetchAllEpisodeFileDates(doRequest)
		if err == nil {
			t.Fatal("Expected error on API failure")
		}
	})

	t.Run("returns error on malformed JSON", func(t *testing.T) {
		doRequest := func(_ string) ([]byte, error) {
			return []byte(`{broken`), nil
		}
		_, err := sonarrFetchAllEpisodeFileDates(doRequest)
		if err == nil {
			t.Fatal("Expected error on malformed JSON")
		}
	})

	t.Run("groups dateAdded by seriesId and season", func(t *testing.T) {
		doRequest := func(endpoint string) ([]byte, error) {
			if endpoint != "/api/v3/episodefile" {
				t.Errorf("Expected bulk /api/v3/episodefile, got %s", endpoint)
			}
			return []byte(`[
				{"id":1,"seriesId":10,"seasonNumber":1,"dateAdded":"2024-01-10T00:00:00Z"},
				{"id":2,"seriesId":10,"seasonNumber":1,"dateAdded":"2024-06-20T00:00:00Z"},
				{"id":3,"seriesId":11,"seasonNumber":1,"dateAdded":"2024-03-05T00:00:00Z"}
			]`), nil
		}
		index, err := sonarrFetchAllEpisodeFileDates(doRequest)
		if err != nil {
			t.Fatalf("Expected success, got %v", err)
		}
		if d := index[10][1]; d.Month() != 6 || d.Day() != 20 {
			t.Errorf("Expected series 10 season 1 Jun 20, got %v", d)
		}
		if d := index[11][1]; d.Month() != 3 || d.Day() != 5 {
			t.Errorf("Expected series 11 season 1 Mar 5, got %v", d)
		}
	})
}

func testSonarrOnDiskSeries(id int, title string, added string) sonarrSeries {
	return sonarrSeries{
		ID:               id,
		Title:            title,
		Year:             2000 + id,
		TmdbID:           1000 + id,
		Path:             "/media/tv/" + title,
		Monitored:        true,
		Status:           "ended",
		QualityProfileID: 1,
		Added:            added,
		Statistics: struct {
			SizeOnDisk   int64 `json:"sizeOnDisk"`
			SeasonCount  int   `json:"seasonCount"`
			EpisodeCount int   `json:"episodeCount"`
		}{SizeOnDisk: 10_000_000_000, SeasonCount: 1, EpisodeCount: 8},
		Seasons: []sonarrSeason{
			{
				SeasonNumber: 1,
				Monitored:    true,
				Statistics: struct {
					SizeOnDisk        int64 `json:"sizeOnDisk"`
					EpisodeFileCount  int   `json:"episodeFileCount"`
					TotalEpisodeCount int   `json:"totalEpisodeCount"`
				}{SizeOnDisk: 10_000_000_000, EpisodeFileCount: 8, TotalEpisodeCount: 8},
			},
		},
	}
}

func TestSonarrClient_GetMediaItems_EpisodeFileCallCountIsO1(t *testing.T) {
	const nSeries = 20
	var episodeFileCalls int
	var episodeFileWithSeriesID int

	series := make([]sonarrSeries, 0, nSeries+1)
	files := make([]sonarrEpisodeFile, 0, nSeries)
	for i := 1; i <= nSeries; i++ {
		series = append(series, testSonarrOnDiskSeries(i, fmt.Sprintf("Show %02d", i), "2023-01-15T00:00:00Z"))
		files = append(files, sonarrEpisodeFile{
			ID:           1000 + i,
			SeriesID:     i,
			SeasonNumber: 1,
			DateAdded:    "2023-06-20T12:00:00Z",
		})
	}
	// Size-0 series must not force extra episodefile traffic.
	series = append(series, sonarrSeries{
		ID:    99,
		Title: "Empty Show",
		Statistics: struct {
			SizeOnDisk   int64 `json:"sizeOnDisk"`
			SeasonCount  int   `json:"seasonCount"`
			EpisodeCount int   `json:"episodeCount"`
		}{SizeOnDisk: 0},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testRadarrPathQuality:
			_, _ = w.Write([]byte(`[{"id":1,"name":"HD-1080p"}]`))
		case testRadarrPathTag:
			_, _ = w.Write([]byte(`[]`))
		case "/api/v3/series":
			if err := json.NewEncoder(w).Encode(series); err != nil {
				t.Fatalf("Failed to encode series: %v", err)
			}
		case "/api/v3/episodefile":
			episodeFileCalls++
			if r.URL.Query().Get("seriesId") != "" {
				episodeFileWithSeriesID++
			}
			if err := json.NewEncoder(w).Encode(files); err != nil {
				t.Fatalf("Failed to encode episode files: %v", err)
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

	if episodeFileCalls != 1 {
		t.Errorf("Expected 1 bulk episodefile request (O(1) in series), got %d", episodeFileCalls)
	}
	if episodeFileWithSeriesID != 0 {
		t.Errorf("Expected 0 per-series episodefile requests, got %d", episodeFileWithSeriesID)
	}

	// 20 seasons + 20 show-level items; empty show skipped
	if len(items) != nSeries*2 {
		t.Fatalf("Expected %d items, got %d", nSeries*2, len(items))
	}

	for _, item := range items {
		if item.AddedAt == nil {
			t.Fatalf("Expected AddedAt from episodefile for %q", item.Title)
		}
		if item.AddedAt.Month() != 6 || item.AddedAt.Day() != 20 {
			t.Errorf("Expected AddedAt from episodefile (Jun 20), not series.added, got %v for %q", item.AddedAt, item.Title)
		}
	}
}

func TestSonarrClient_GetMediaItems_BulkEpisodeFileFallback(t *testing.T) {
	var bulkCalls int
	var perSeriesCalls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testRadarrPathQuality:
			_, _ = w.Write([]byte(`[{"id":1,"name":"HD-1080p"}]`))
		case testRadarrPathTag:
			_, _ = w.Write([]byte(`[]`))
		case "/api/v3/series":
			resp := []sonarrSeries{
				testSonarrOnDiskSeries(1, "Firefly", "2023-01-15T00:00:00Z"),
				testSonarrOnDiskSeries(2, "Serenity", "2023-02-01T00:00:00Z"),
				{
					ID:    3,
					Title: "Empty Show",
					Statistics: struct {
						SizeOnDisk   int64 `json:"sizeOnDisk"`
						SeasonCount  int   `json:"seasonCount"`
						EpisodeCount int   `json:"episodeCount"`
					}{SizeOnDisk: 0},
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode series: %v", err)
			}
		case "/api/v3/episodefile":
			if r.URL.Query().Get("seriesId") == "" {
				bulkCalls++
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"message":"seriesId or episodeFileIds must be provided"}`))
				return
			}
			perSeriesCalls++
			seriesID := r.URL.Query().Get("seriesId")
			var resp []sonarrEpisodeFile
			switch seriesID {
			case "1":
				resp = []sonarrEpisodeFile{{ID: 101, SeriesID: 1, SeasonNumber: 1, DateAdded: "2023-06-20T12:00:00Z"}}
			case "2":
				resp = []sonarrEpisodeFile{{ID: 201, SeriesID: 2, SeasonNumber: 1, DateAdded: "2023-07-04T12:00:00Z"}}
			default:
				resp = []sonarrEpisodeFile{}
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode episode files: %v", err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewSonarrClient(srv.URL, testTautulliAPIKey)
	items, err := client.GetMediaItems()
	if err != nil {
		t.Fatalf("GetMediaItems should succeed after bulk failure: %v", err)
	}

	if bulkCalls != 1 {
		t.Errorf("Expected 1 bulk episodefile probe, got %d", bulkCalls)
	}
	if perSeriesCalls != 2 {
		t.Errorf("Expected per-series fallback for 2 on-disk series, got %d calls", perSeriesCalls)
	}
	if len(items) != 4 {
		t.Fatalf("Expected 4 items (2 seasons + 2 shows), got %d", len(items))
	}

	want := map[string][2]int{
		"Firefly - Season 1":  {6, 20},
		"Firefly":             {6, 20},
		"Serenity - Season 1": {7, 4},
		"Serenity":            {7, 4},
	}
	for _, item := range items {
		expect, ok := want[item.Title]
		if !ok {
			t.Errorf("Unexpected item %q", item.Title)
			continue
		}
		if item.AddedAt == nil {
			t.Errorf("Expected AddedAt from fallback episodefile for %q", item.Title)
			continue
		}
		if int(item.AddedAt.Month()) != expect[0] || item.AddedAt.Day() != expect[1] {
			t.Errorf("Expected AddedAt %d/%d from episodefile for %q, got %v", expect[0], expect[1], item.Title, item.AddedAt)
		}
	}
}
