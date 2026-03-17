package integrations

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadarrClient_TestConnection_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/system/status" {
			t.Errorf("Unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-Api-Key") != testTautulliAPIKey {
			t.Errorf("Missing or wrong API key header")
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"0.3.0"}`))
	}))
	defer srv.Close()

	client := NewReadarrClient(srv.URL, testTautulliAPIKey)
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should succeed: %v", err)
	}
}

func TestReadarrClient_TestConnection_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewReadarrClient(srv.URL, "bad-key")
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 401")
	}
}

func TestReadarrClient_TestConnection_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewReadarrClient(srv.URL, testTautulliAPIKey)
	err := client.TestConnection()
	if err == nil {
		t.Fatal("TestConnection should fail with 500")
	}
}

func TestReadarrClient_GetDiskSpace(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/diskspace" {
			resp := []arrDiskSpace{
				{Path: "/media/books", TotalSpace: 500000000000, FreeSpace: 200000000000},
				{Path: "/media/audiobooks", TotalSpace: 1000000000000, FreeSpace: 750000000000},
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

	client := NewReadarrClient(srv.URL, testTautulliAPIKey)
	disks, err := client.GetDiskSpace()
	if err != nil {
		t.Fatalf("GetDiskSpace should succeed: %v", err)
	}
	if len(disks) != 2 {
		t.Fatalf("Expected 2 disks, got %d", len(disks))
	}
	if disks[0].Path != "/media/books" {
		t.Errorf("Expected path '/media/books', got %q", disks[0].Path)
	}
	if disks[0].TotalBytes != 500000000000 {
		t.Errorf("Expected TotalBytes 500000000000, got %d", disks[0].TotalBytes)
	}
	if disks[1].Path != "/media/audiobooks" {
		t.Errorf("Expected second path '/media/audiobooks', got %q", disks[1].Path)
	}
}

func TestReadarrClient_GetMediaItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testLidarrPathQuality:
			_, _ = w.Write([]byte(`[{"id":1,"name":"eBook"},{"id":2,"name":"Audiobook"}]`))
		case testLidarrPathTag:
			_, _ = w.Write([]byte(`[{"id":1,"label":"sci-fi"},{"id":2,"label":"classic"}]`))
		case "/api/v1/book":
			resp := []readarrBook{
				{
					ID:               1,
					Title:            "Serenity",
					AuthorID:         10,
					SizeOnDisk:       2000000,
					ReleaseDate:      "1965-08-01T00:00:00Z",
					Added:            "2024-03-01T10:30:00Z",
					Monitored:        true,
					Path:             "/media/books/Serenity",
					QualityProfileID: 1,
					Tags:             []int{1, 2},
					Genres:           []string{"Science Fiction", "Adventure"},
					Author: struct {
						AuthorName string `json:"authorName"`
					}{AuthorName: "Frank Herbert"},
					Ratings: struct {
						Value float64 `json:"value"`
					}{Value: 4.25}, // GoodReads 0–5 scale
				},
				{
					// Book with no file on disk — should be skipped
					ID:         2,
					Title:      "Serenity 2",
					SizeOnDisk: 0,
				},
				{
					ID:               3,
					Title:            "Firefly",
					AuthorID:         20,
					SizeOnDisk:       1500000,
					ReleaseDate:      "1984-07-01T00:00:00Z",
					Added:            "2024-04-15T08:00:00Z",
					Monitored:        false,
					Path:             "/media/books/Firefly",
					QualityProfileID: 2,
					Tags:             []int{1},
					Genres:           []string{"Cyberpunk"},
					Author: struct {
						AuthorName string `json:"authorName"`
					}{AuthorName: "William Gibson"},
					Ratings: struct {
						Value float64 `json:"value"`
					}{Value: 3.95}, // GoodReads 0–5 scale
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

	client := NewReadarrClient(srv.URL, testTautulliAPIKey)
	items, err := client.GetMediaItems()
	if err != nil {
		t.Fatalf("GetMediaItems should succeed: %v", err)
	}

	// Expect 2 books (Serenity 2 has SizeOnDisk=0)
	if len(items) != 2 {
		t.Fatalf("Expected 2 items, got %d", len(items))
	}

	// First book
	book1 := items[0]
	if book1.Type != MediaTypeBook {
		t.Errorf("Expected MediaTypeBook, got %v", book1.Type)
	}
	if book1.Title != "Serenity" {
		t.Errorf("Expected 'Serenity', got %q", book1.Title)
	}
	if book1.ExternalID != "1" {
		t.Errorf("Expected ExternalID '1', got %q", book1.ExternalID)
	}
	if book1.SizeBytes != 2000000 {
		t.Errorf("Expected SizeBytes 2000000, got %d", book1.SizeBytes)
	}
	if book1.Path != "/media/books/Serenity" {
		t.Errorf("Expected path '/media/books/Serenity', got %q", book1.Path)
	}
	if !book1.Monitored {
		t.Error("Expected Serenity to be monitored")
	}
	if book1.AddedAt == nil {
		t.Error("Expected non-nil AddedAt for Serenity")
	}
	if book1.QualityProfile != "eBook" {
		t.Errorf("Expected quality profile 'eBook', got %q", book1.QualityProfile)
	}
	// Rating is GoodReads 4.25 × 2.0 = 8.5 (normalized to 0–10)
	if book1.Rating != 8.5 {
		t.Errorf("Expected normalized rating 8.5 (4.25 × 2), got %f", book1.Rating)
	}
	if len(book1.Tags) != 2 || book1.Tags[0] != "sci-fi" || book1.Tags[1] != "classic" {
		t.Errorf("Expected tags [sci-fi, classic], got %v", book1.Tags)
	}
	if book1.Genre != "Science Fiction" {
		t.Errorf("Expected genre 'Science Fiction', got %q", book1.Genre)
	}
	if book1.Year != 1965 {
		t.Errorf("Expected Year 1965 from releaseDate, got %d", book1.Year)
	}

	// Second book
	book2 := items[1]
	if book2.Title != "Firefly" {
		t.Errorf("Expected 'Firefly', got %q", book2.Title)
	}
	if book2.Monitored {
		t.Error("Expected Firefly to be unmonitored")
	}
	if book2.QualityProfile != "Audiobook" {
		t.Errorf("Expected quality profile 'Audiobook', got %q", book2.QualityProfile)
	}
	if len(book2.Tags) != 1 || book2.Tags[0] != "sci-fi" {
		t.Errorf("Expected tags [sci-fi], got %v", book2.Tags)
	}
	if book2.Year != 1984 {
		t.Errorf("Expected Year 1984 from releaseDate, got %d", book2.Year)
	}
}

func TestReadarrClient_GetMediaItems_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testLidarrPathQuality:
			_, _ = w.Write([]byte(`[]`))
		case testLidarrPathTag:
			_, _ = w.Write([]byte(`[]`))
		case "/api/v1/book":
			_, _ = w.Write([]byte(`{not valid json}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewReadarrClient(srv.URL, testTautulliAPIKey)
	_, err := client.GetMediaItems()
	if err == nil {
		t.Fatal("Expected error for malformed JSON")
	}
}

func TestReadarrClient_GetMediaItems_EmptyResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case testLidarrPathQuality:
			_, _ = w.Write([]byte(`[]`))
		case testLidarrPathTag:
			_, _ = w.Write([]byte(`[]`))
		case "/api/v1/book":
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewReadarrClient(srv.URL, testTautulliAPIKey)
	items, err := client.GetMediaItems()
	if err != nil {
		t.Fatalf("GetMediaItems should succeed with empty: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(items))
	}
}

func TestReadarrClient_GetRootFolders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/rootfolder" {
			resp := []struct {
				Path string `json:"path"`
			}{
				{Path: "/media/books"},
				{Path: "/media/audiobooks"},
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

	client := NewReadarrClient(srv.URL, testTautulliAPIKey)
	folders, err := client.GetRootFolders()
	if err != nil {
		t.Fatalf("GetRootFolders should succeed: %v", err)
	}
	if len(folders) != 2 {
		t.Fatalf("Expected 2 folders, got %d", len(folders))
	}
	if folders[0] != "/media/books" {
		t.Errorf("Expected '/media/books', got %q", folders[0])
	}
	if folders[1] != "/media/audiobooks" {
		t.Errorf("Expected '/media/audiobooks', got %q", folders[1])
	}
}

func TestReadarrClient_GetQualityProfiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testLidarrPathQuality {
			resp := []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			}{
				{ID: 1, Name: "eBook"},
				{ID: 2, Name: "Audiobook"},
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

	client := NewReadarrClient(srv.URL, testTautulliAPIKey)
	profiles, err := client.GetQualityProfiles()
	if err != nil {
		t.Fatalf("GetQualityProfiles should succeed: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("Expected 2 profiles, got %d", len(profiles))
	}
	if profiles[0].Value != "eBook" {
		t.Errorf("Expected 'eBook', got %q", profiles[0].Value)
	}
}

func TestReadarrClient_GetTags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == testLidarrPathTag {
			resp := []struct {
				ID    int    `json:"id"`
				Label string `json:"label"`
			}{
				{ID: 1, Label: "sci-fi"},
				{ID: 2, Label: "fantasy"},
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

	client := NewReadarrClient(srv.URL, testTautulliAPIKey)
	tags, err := client.GetTags()
	if err != nil {
		t.Fatalf("GetTags should succeed: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("Expected 2 tags, got %d", len(tags))
	}
	if tags[0].Value != "sci-fi" {
		t.Errorf("Expected 'sci-fi', got %q", tags[0].Value)
	}
}

func TestReadarrClient_GetLanguages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/language" {
			resp := []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			}{
				{ID: 1, Name: "English"},
				{ID: 2, Name: "French"},
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

	client := NewReadarrClient(srv.URL, testTautulliAPIKey)
	langs, err := client.GetLanguages()
	if err != nil {
		t.Fatalf("GetLanguages should succeed: %v", err)
	}
	if len(langs) != 2 {
		t.Fatalf("Expected 2 languages, got %d", len(langs))
	}
	if langs[0].Value != "English" {
		t.Errorf("Expected 'English', got %q", langs[0].Value)
	}
	if langs[1].Value != "French" {
		t.Errorf("Expected 'French', got %q", langs[1].Value)
	}
}

func TestReadarrClient_URLTrailingSlash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/system/status" {
			t.Errorf("Expected /api/v1/system/status, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	client := NewReadarrClient(srv.URL+"/", testTautulliAPIKey)
	if err := client.TestConnection(); err != nil {
		t.Fatalf("TestConnection should succeed: %v", err)
	}
}
