package integrations

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newPlexMockServer creates a mock Plex server that returns the given movies and shows.
func newPlexMockServer(t *testing.T, movies []plexMockItem, shows []plexMockItem) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/identity":
			_, _ = w.Write([]byte(`{"MediaContainer":{"machineIdentifier":"test","version":"1.0"}}`))
		case "/library/sections":
			dirs := []map[string]string{}
			if len(movies) > 0 {
				dirs = append(dirs, map[string]string{"key": "1", "title": "Movies", "type": "movie"})
			}
			if len(shows) > 0 {
				dirs = append(dirs, map[string]string{"key": "2", "title": "TV Shows", "type": "show"})
			}
			resp := map[string]interface{}{
				"MediaContainer": map[string]interface{}{
					"Directory": dirs,
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode sections: %v", err)
			}
		case testPlexPathMoviesAll:
			metadata := make([]map[string]interface{}, len(movies))
			for i, m := range movies {
				metadata[i] = map[string]interface{}{
					"ratingKey":    m.RatingKey,
					"title":        m.Title,
					"year":         m.Year,
					"type":         "movie",
					"viewCount":    m.ViewCount,
					"lastViewedAt": m.LastViewedAt,
				}
			}
			resp := map[string]interface{}{
				"MediaContainer": map[string]interface{}{
					"Metadata": metadata,
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode movies: %v", err)
			}
		case "/library/sections/2/all":
			metadata := make([]map[string]interface{}, len(shows))
			for i, s := range shows {
				metadata[i] = map[string]interface{}{
					"ratingKey":    s.RatingKey,
					"title":        s.Title,
					"year":         s.Year,
					"type":         "show",
					"viewCount":    s.ViewCount,
					"lastViewedAt": s.LastViewedAt,
				}
			}
			resp := map[string]interface{}{
				"MediaContainer": map[string]interface{}{
					"Metadata": metadata,
				},
			}
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("Failed to encode shows: %v", err)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

type plexMockItem struct {
	RatingKey    string
	Title        string
	Year         int
	ViewCount    int
	LastViewedAt int64
}

func TestEnrichItems_PlexEnrichment(t *testing.T) {
	srv := newPlexMockServer(t,
		[]plexMockItem{
			{RatingKey: "101", Title: "Serenity", Year: 2010, ViewCount: 5, LastViewedAt: 1700000000},
			{RatingKey: "102", Title: "Serenity 2", Year: 2014, ViewCount: 3, LastViewedAt: 1699000000},
		},
		[]plexMockItem{
			{RatingKey: "200", Title: "Firefly", Year: 2008, ViewCount: 10, LastViewedAt: 1700000000},
		},
	)
	defer srv.Close()

	plexClient := NewPlexClient(srv.URL, "test-token")

	// Simulate *arr items with no watch data
	items := []MediaItem{
		{Title: "Serenity", Type: MediaTypeMovie, ExternalID: "1"},
		{Title: "Serenity 2", Type: MediaTypeMovie, ExternalID: "2"},
		{Title: "Firefly", Type: MediaTypeShow, ExternalID: "3"},
		{Title: "Firefly 2", Type: MediaTypeShow, ExternalID: "4"}, // not in Plex
	}

	ec := EnrichmentClients{Plex: plexClient}
	EnrichItems(items, ec)

	// Serenity should be enriched
	if items[0].PlayCount != 5 {
		t.Errorf("Serenity: expected PlayCount 5, got %d", items[0].PlayCount)
	}
	if items[0].LastPlayed == nil {
		t.Error("Serenity: expected LastPlayed to be set")
	}

	// Serenity 2 should be enriched
	if items[1].PlayCount != 3 {
		t.Errorf("Serenity 2: expected PlayCount 3, got %d", items[1].PlayCount)
	}

	// Firefly should be enriched
	if items[2].PlayCount != 10 {
		t.Errorf("Firefly: expected PlayCount 10, got %d", items[2].PlayCount)
	}

	// Firefly 2 should NOT be enriched (not in Plex)
	if items[3].PlayCount != 0 {
		t.Errorf("Firefly 2: expected PlayCount 0, got %d", items[3].PlayCount)
	}
	if items[3].LastPlayed != nil {
		t.Error("Firefly 2: expected LastPlayed to be nil")
	}
}

func TestEnrichItems_PlexEnrichment_SeasonMatchesByShowTitle(t *testing.T) {
	srv := newPlexMockServer(t,
		nil,
		[]plexMockItem{
			{RatingKey: "200", Title: "Firefly", Year: 2008, ViewCount: 10, LastViewedAt: 1700000000},
		},
	)
	defer srv.Close()

	plexClient := NewPlexClient(srv.URL, "test-token")

	// Simulate a season item from Sonarr — Season should match via ShowTitle
	items := []MediaItem{
		{
			Title:     "Season 2",
			ShowTitle: "Firefly",
			Type:      MediaTypeSeason,
		},
	}

	ec := EnrichmentClients{Plex: plexClient}
	EnrichItems(items, ec)

	if items[0].PlayCount != 10 {
		t.Errorf("Season 2: expected PlayCount 10 via ShowTitle match, got %d", items[0].PlayCount)
	}
}

func TestEnrichItems_TautulliTakesPriorityOverPlex(t *testing.T) {
	srv := newPlexMockServer(t,
		[]plexMockItem{
			{RatingKey: "101", Title: "Serenity", Year: 2010, ViewCount: 5, LastViewedAt: 1700000000},
		},
		nil,
	)
	defer srv.Close()

	plexClient := NewPlexClient(srv.URL, "test-token")

	// Simulate an item already enriched by Tautulli (PlayCount > 0)
	tautulliTime := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	items := []MediaItem{
		{
			Title:      "Serenity",
			Type:       MediaTypeMovie,
			ExternalID: "1",
			PlayCount:  20,            // Already set by Tautulli
			LastPlayed: &tautulliTime, // Already set by Tautulli
		},
	}

	ec := EnrichmentClients{Plex: plexClient}
	EnrichItems(items, ec)

	// Tautulli data should NOT be overwritten by Plex (PlayCount > 0 guard)
	if items[0].PlayCount != 20 {
		t.Errorf("Expected Tautulli PlayCount 20 to be preserved, got %d", items[0].PlayCount)
	}
	if !items[0].LastPlayed.Equal(tautulliTime) {
		t.Errorf("Expected Tautulli LastPlayed to be preserved, got %v", items[0].LastPlayed)
	}
}

func TestEnrichItems_PlexDoesNotOverwriteExistingData(t *testing.T) {
	srv := newPlexMockServer(t,
		[]plexMockItem{
			{RatingKey: "101", Title: "Serenity", Year: 2010, ViewCount: 5, LastViewedAt: 1700000000},
		},
		nil,
	)
	defer srv.Close()

	plexClient := NewPlexClient(srv.URL, "test-token")

	// Simulate an item with existing play data from another enrichment source
	existingTime := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
	items := []MediaItem{
		{
			Title:      "Serenity",
			Type:       MediaTypeMovie,
			ExternalID: "1",
			PlayCount:  3,
			LastPlayed: &existingTime,
		},
	}

	ec := EnrichmentClients{Plex: plexClient}
	EnrichItems(items, ec)

	// Existing data should be preserved (PlayCount != 0 guard)
	if items[0].PlayCount != 3 {
		t.Errorf("Expected existing PlayCount 3 to be preserved, got %d", items[0].PlayCount)
	}
	if !items[0].LastPlayed.Equal(existingTime) {
		t.Errorf("Expected existing LastPlayed to be preserved, got %v", items[0].LastPlayed)
	}
}
