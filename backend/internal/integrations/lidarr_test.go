package integrations

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	testLidarrPathStatus  = "/api/v1/system/status"
	testLidarrPathQuality = "/api/v1/qualityprofile"
	testLidarrPathTag     = "/api/v1/tag"
)

func TestLidarrClient_TestConnection_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testLidarrPathStatus {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != testTautulliAPIKey {
			t.Errorf("Missing or wrong API key header")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"1.0.0"}`))
	}))
	defer srv.Close()

	client := NewLidarrClient(srv.URL, testTautulliAPIKey)
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should succeed: %v", err)
	}
}

func TestLidarrClient_TestConnection_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewLidarrClient(srv.URL, "bad-key")
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 401")
	}
}

func TestLidarrClient_TestConnection_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewLidarrClient(srv.URL, testTautulliAPIKey)
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 500")
	}
}

func TestLidarrClient_GetDiskSpace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/diskspace" {
			resp := []arrDiskSpace{
				{Path: "/media/music", TotalSpace: 1000000000000, FreeSpace: 300000000000},
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

	client := NewLidarrClient(srv.URL, testTautulliAPIKey)
	disks, err := client.GetDiskSpace()
	if err != nil {
		t.Fatalf("GetDiskSpace should succeed: %v", err)
	}
	if len(disks) != 1 {
		t.Fatalf("Expected 1 disk, got %d", len(disks))
	}
	if disks[0].Path != "/media/music" {
		t.Errorf("Expected path '/media/music', got %q", disks[0].Path)
	}
	if disks[0].TotalBytes != 1000000000000 {
		t.Errorf("Expected TotalBytes 1000000000000, got %d", disks[0].TotalBytes)
	}
	if disks[0].FreeBytes != 300000000000 {
		t.Errorf("Expected FreeBytes 300000000000, got %d", disks[0].FreeBytes)
	}
}

func TestLidarrClient_GetMediaItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testLidarrPathQuality:
			resp := []arrQualityProfile{
				{ID: 1, Name: "Lossless"},
				{ID: 2, Name: "Standard"},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode response: %v", err)
			}
		case testLidarrPathTag:
			resp := []arrTag{
				{ID: 1, Label: "favorite"},
				{ID: 2, Label: "jazz"},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode response: %v", err)
			}
		case "/api/v1/artist":
			resp := []lidarrArtist{
				{
					ID:               1,
					ArtistName:       "Pink Floyd",
					Path:             "/media/music/Pink Floyd",
					Monitored:        true,
					Genres:           []string{"rock", "progressive"},
					Tags:             []int{2},
					QualityProfileID: 1,
					Added:            "2024-01-15T00:00:00Z",
					Ratings: struct {
						Value float64 `json:"value"`
					}{Value: 9.2},
					Statistics: struct {
						SizeOnDisk int64 `json:"sizeOnDisk"`
						AlbumCount int   `json:"albumCount"`
						TrackCount int   `json:"trackCount"`
					}{SizeOnDisk: 15000000000, AlbumCount: 15, TrackCount: 164},
				},
				{
					// Artist with no files on disk — should be skipped
					ID:         2,
					ArtistName: "Empty Artist",
					Statistics: struct {
						SizeOnDisk int64 `json:"sizeOnDisk"`
						AlbumCount int   `json:"albumCount"`
						TrackCount int   `json:"trackCount"`
					}{SizeOnDisk: 0},
				},
				{
					ID:               3,
					ArtistName:       "Miles Davis",
					Path:             "/media/music/Miles Davis",
					Monitored:        false,
					QualityProfileID: 2,
					Tags:             []int{1, 2},
					Added:            "2024-02-20T12:00:00Z",
					Ratings: struct {
						Value float64 `json:"value"`
					}{Value: 8.5},
					Statistics: struct {
						SizeOnDisk int64 `json:"sizeOnDisk"`
						AlbumCount int   `json:"albumCount"`
						TrackCount int   `json:"trackCount"`
					}{SizeOnDisk: 5000000000},
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

	client := NewLidarrClient(srv.URL, testTautulliAPIKey)
	items, err := client.GetMediaItems()
	if err != nil {
		t.Fatalf("GetMediaItems should succeed: %v", err)
	}

	// Expect 2 artists (Empty Artist has SizeOnDisk=0)
	if len(items) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(items))
	}

	// First artist
	pinkFloyd := items[0]
	if pinkFloyd.Type != MediaTypeArtist {
		t.Errorf("Expected MediaTypeArtist, got %v", pinkFloyd.Type)
	}
	if pinkFloyd.Title != "Pink Floyd" {
		t.Errorf("Expected 'Pink Floyd', got %q", pinkFloyd.Title)
	}
	// Rating is passed through as-is (already 0–10 from MusicBrainz)
	expectedRating := 9.2
	if math.Abs(pinkFloyd.Rating-expectedRating) > 0.001 {
		t.Errorf("Expected rating %v, got %v", expectedRating, pinkFloyd.Rating)
	}
	if pinkFloyd.QualityProfile != "Lossless" {
		t.Errorf("Expected quality 'Lossless', got %q", pinkFloyd.QualityProfile)
	}
	if len(pinkFloyd.Tags) != 1 || pinkFloyd.Tags[0] != "jazz" {
		t.Errorf("Expected tags [jazz], got %v", pinkFloyd.Tags)
	}
	if pinkFloyd.Genre != "rock, progressive" {
		t.Errorf("Expected genre 'rock, progressive', got %q", pinkFloyd.Genre)
	}
	if pinkFloyd.SizeBytes != 15000000000 {
		t.Errorf("Expected SizeBytes 15000000000, got %d", pinkFloyd.SizeBytes)
	}
	if pinkFloyd.AddedAt == nil {
		t.Error("Expected non-nil AddedAt")
	}

	// Second artist — Miles Davis with both tags
	miles := items[1]
	if miles.Title != "Miles Davis" {
		t.Errorf("Expected 'Miles Davis', got %q", miles.Title)
	}
	if miles.QualityProfile != "Standard" {
		t.Errorf("Expected quality 'Standard', got %q", miles.QualityProfile)
	}
	if len(miles.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(miles.Tags))
	}
	if miles.Monitored {
		t.Error("Expected Miles Davis to be unmonitored")
	}
}

func TestLidarrClient_GetMediaItems_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testLidarrPathQuality:
			_, _ = w.Write([]byte(`[]`))
		case testLidarrPathTag:
			_, _ = w.Write([]byte(`[]`))
		case "/api/v1/artist":
			_, _ = w.Write([]byte(`not json at all`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewLidarrClient(srv.URL, testTautulliAPIKey)
	_, err := client.GetMediaItems()
	if err == nil {
		t.Fatal("Expected error for malformed JSON")
	}
}

func TestLidarrClient_GetMediaItems_EmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testLidarrPathQuality:
			_, _ = w.Write([]byte(`[]`))
		case testLidarrPathTag:
			_, _ = w.Write([]byte(`[]`))
		case "/api/v1/artist":
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewLidarrClient(srv.URL, testTautulliAPIKey)
	items, err := client.GetMediaItems()
	if err != nil {
		t.Fatalf("GetMediaItems should succeed with empty: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(items))
	}
}

func TestLidarrClient_GetRootFolders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/rootfolder" {
			resp := []arrRootFolder{
				{Path: "/media/music"},
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

	client := NewLidarrClient(srv.URL, testTautulliAPIKey)
	folders, err := client.GetRootFolders()
	if err != nil {
		t.Fatalf("GetRootFolders should succeed: %v", err)
	}
	if len(folders) != 1 || folders[0] != "/media/music" {
		t.Errorf("Expected [/media/music], got %v", folders)
	}
}

func TestLidarrClient_GetQualityProfiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testLidarrPathQuality {
			resp := []arrQualityProfile{{ID: 1, Name: "Lossless"}, {ID: 2, Name: "Standard"}}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewLidarrClient(srv.URL, testTautulliAPIKey)
	profiles, err := client.GetQualityProfiles()
	if err != nil {
		t.Fatalf("GetQualityProfiles should succeed: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("Expected 2 profiles, got %d", len(profiles))
	}
	if profiles[0].Value != "Lossless" {
		t.Errorf("Expected 'Lossless', got %q", profiles[0].Value)
	}
}

func TestLidarrClient_GetTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testLidarrPathTag {
			resp := []arrTag{{ID: 1, Label: "rock"}}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode: %v", err)
			}
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewLidarrClient(srv.URL, testTautulliAPIKey)
	tags, err := client.GetTags()
	if err != nil {
		t.Fatalf("GetTags should succeed: %v", err)
	}
	if len(tags) != 1 || tags[0].Value != "rock" {
		t.Errorf("Expected [rock], got %v", tags)
	}
}

func TestLidarrClient_GetLanguages(t *testing.T) {
	// Lidarr does not have a language endpoint — should return nil, nil
	client := NewLidarrClient("http://unused", "unused")
	langs, err := client.GetLanguages()
	if err != nil {
		t.Fatalf("GetLanguages should return nil error: %v", err)
	}
	if langs != nil {
		t.Errorf("Expected nil languages, got %v", langs)
	}
}

func TestLidarrClient_URLTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != testLidarrPathStatus {
			t.Errorf("Expected /api/v1/system/status, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewLidarrClient(srv.URL+"/", testTautulliAPIKey)
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should succeed: %v", err)
	}
}
