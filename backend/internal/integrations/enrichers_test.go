package integrations

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ─── Mock CollectionDataProvider ────────────────────────────────────────────

type mockCollectionDataProvider struct {
	memberships map[int][]string
	err         error
}

func (m *mockCollectionDataProvider) GetCollectionMemberships() (map[int][]string, error) {
	return m.memberships, m.err
}

// ─── CollectionEnricher tests ───────────────────────────────────────────────

func TestCollectionEnricher_EnrichesItemsWithNoExistingCollections(t *testing.T) {
	provider := &mockCollectionDataProvider{
		memberships: map[int][]string{
			100: {"Firefly Collection"},
			200: {"Serenity Saga"},
		},
	}
	enricher := NewCollectionEnricher("test", 50, 42, provider)

	items := []MediaItem{
		{Title: "Serenity", TMDbID: 100},
		{Title: "Firefly Movie", TMDbID: 200},
		{Title: "Unrelated", TMDbID: 300},
	}

	if err := enricher.Enrich(items); err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// Item 0: should have collection from provider
	if len(items[0].Collections) != 1 || items[0].Collections[0] != "Firefly Collection" {
		t.Errorf("Expected [Firefly Collection], got %v", items[0].Collections)
	}
	if items[0].CollectionSources["Firefly Collection"] != 42 {
		t.Errorf("Expected source 42 for Firefly Collection, got %d", items[0].CollectionSources["Firefly Collection"])
	}

	// Item 1: should have collection from provider
	if len(items[1].Collections) != 1 || items[1].Collections[0] != "Serenity Saga" {
		t.Errorf("Expected [Serenity Saga], got %v", items[1].Collections)
	}

	// Item 2: no TMDb match — no collections
	if len(items[2].Collections) != 0 {
		t.Errorf("Expected no collections for unmatched item, got %v", items[2].Collections)
	}
}

func TestCollectionEnricher_MergesWithExistingCollections(t *testing.T) {
	provider := &mockCollectionDataProvider{
		memberships: map[int][]string{
			100: {"Plex Sci-Fi", "Plex Classics"},
		},
	}
	enricher := NewCollectionEnricher("test", 50, 99, provider)

	items := []MediaItem{
		{
			Title:             "Serenity",
			TMDbID:            100,
			Collections:       []string{"Firefly Collection"},
			CollectionSources: map[string]uint{"Firefly Collection": 1},
		},
	}

	if err := enricher.Enrich(items); err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// Should have all 3 collections: original + 2 from provider
	if len(items[0].Collections) != 3 {
		t.Fatalf("Expected 3 collections, got %d: %v", len(items[0].Collections), items[0].Collections)
	}

	// Original source preserved
	if items[0].CollectionSources["Firefly Collection"] != 1 {
		t.Errorf("Expected original source 1 for Firefly Collection, got %d", items[0].CollectionSources["Firefly Collection"])
	}

	// New sources attributed to provider integration
	if items[0].CollectionSources["Plex Sci-Fi"] != 99 {
		t.Errorf("Expected source 99 for Plex Sci-Fi, got %d", items[0].CollectionSources["Plex Sci-Fi"])
	}
	if items[0].CollectionSources["Plex Classics"] != 99 {
		t.Errorf("Expected source 99 for Plex Classics, got %d", items[0].CollectionSources["Plex Classics"])
	}
}

func TestCollectionEnricher_DeduplicatesCollectionNames(t *testing.T) {
	provider := &mockCollectionDataProvider{
		memberships: map[int][]string{
			100: {"Firefly Collection", "Plex Sci-Fi"},
		},
	}
	enricher := NewCollectionEnricher("test", 50, 99, provider)

	items := []MediaItem{
		{
			Title:             "Serenity",
			TMDbID:            100,
			Collections:       []string{"Firefly Collection"},
			CollectionSources: map[string]uint{"Firefly Collection": 1},
		},
	}

	if err := enricher.Enrich(items); err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// Should have 2 collections: original "Firefly Collection" + new "Plex Sci-Fi"
	// "Firefly Collection" should NOT be duplicated
	if len(items[0].Collections) != 2 {
		t.Fatalf("Expected 2 collections (deduped), got %d: %v", len(items[0].Collections), items[0].Collections)
	}

	// Source for shared name should be overwritten by enricher (last writer wins)
	if items[0].CollectionSources["Firefly Collection"] != 99 {
		t.Errorf("Expected source 99 for shared Firefly Collection (last writer wins), got %d",
			items[0].CollectionSources["Firefly Collection"])
	}
}

func TestCollectionEnricher_SkipsItemsWithoutTMDbID(t *testing.T) {
	provider := &mockCollectionDataProvider{
		memberships: map[int][]string{
			100: {"Firefly Collection"},
		},
	}
	enricher := NewCollectionEnricher("test", 50, 42, provider)

	items := []MediaItem{
		{Title: "Serenity", TMDbID: 0}, // No TMDb ID
	}

	if err := enricher.Enrich(items); err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if len(items[0].Collections) != 0 {
		t.Errorf("Expected no collections for item without TMDb ID, got %v", items[0].Collections)
	}
}

func TestCollectionEnricher_HandlesEmptyMemberships(t *testing.T) {
	provider := &mockCollectionDataProvider{
		memberships: map[int][]string{},
	}
	enricher := NewCollectionEnricher("test", 50, 42, provider)

	items := []MediaItem{
		{Title: "Serenity", TMDbID: 100, Collections: []string{"Existing"}},
	}

	if err := enricher.Enrich(items); err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// Original collections should be unchanged
	if len(items[0].Collections) != 1 || items[0].Collections[0] != "Existing" {
		t.Errorf("Expected [Existing] unchanged, got %v", items[0].Collections)
	}
}

func TestCollectionEnricher_PropagatesProviderError(t *testing.T) {
	provider := &mockCollectionDataProvider{
		err: errConnectionFailed,
	}
	enricher := NewCollectionEnricher("test", 50, 42, provider)

	items := []MediaItem{{Title: "Serenity", TMDbID: 100}}

	if err := enricher.Enrich(items); err == nil {
		t.Fatal("Expected error from provider, got nil")
	}
}

// errConnectionFailed is a sentinel error for testing.
var errConnectionFailed = &testError{"connection failed"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

func TestCollectionEnricher_InitializesCollectionSourcesMap(t *testing.T) {
	provider := &mockCollectionDataProvider{
		memberships: map[int][]string{
			100: {"Firefly Collection"},
		},
	}
	enricher := NewCollectionEnricher("test", 50, 42, provider)

	// Item with nil CollectionSources
	items := []MediaItem{
		{Title: "Serenity", TMDbID: 100, CollectionSources: nil},
	}

	if err := enricher.Enrich(items); err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if items[0].CollectionSources == nil {
		t.Fatal("Expected CollectionSources to be initialized")
	}
	if items[0].CollectionSources["Firefly Collection"] != 42 {
		t.Errorf("Expected source 42, got %d", items[0].CollectionSources["Firefly Collection"])
	}
}

// ─── Mock RequestProvider ───────────────────────────────────────────────────

type mockRequestProvider struct {
	requests []MediaRequest
	err      error
}

func (m *mockRequestProvider) GetRequestedMedia() ([]MediaRequest, error) {
	return m.requests, m.err
}

// ─── RequestEnricher tests ──────────────────────────────────────────────────

func TestRequestEnricher_BasicMatch(t *testing.T) {
	provider := &mockRequestProvider{
		requests: []MediaRequest{
			{MediaType: "movie", TMDbID: 16320, RequestedBy: "mal"},
		},
	}
	enricher := NewRequestEnricher(provider)

	items := []MediaItem{
		{Title: "Serenity", TMDbID: 16320},
		{Title: "Firefly", TMDbID: 1437, Type: MediaTypeShow},
	}

	if err := enricher.Enrich(items); err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	// Item 0: should be marked as requested
	if !items[0].IsRequested {
		t.Error("Expected Serenity to be marked as requested")
	}
	if items[0].RequestedBy != "mal" {
		t.Errorf("Expected RequestedBy 'mal', got %q", items[0].RequestedBy)
	}
	if items[0].RequestCount != 1 {
		t.Errorf("Expected RequestCount 1, got %d", items[0].RequestCount)
	}

	// Item 1: no matching request — should not be requested
	if items[1].IsRequested {
		t.Error("Expected Firefly to not be marked as requested")
	}
	if items[1].RequestCount != 0 {
		t.Errorf("Expected RequestCount 0 for unmatched, got %d", items[1].RequestCount)
	}
}

func TestRequestEnricher_AggregatesMultipleRequests(t *testing.T) {
	provider := &mockRequestProvider{
		requests: []MediaRequest{
			{MediaType: "movie", TMDbID: 16320, RequestedBy: "mal"},
			{MediaType: "movie", TMDbID: 16320, RequestedBy: "wash"},
			{MediaType: "movie", TMDbID: 16320, RequestedBy: "zoe"},
		},
	}
	enricher := NewRequestEnricher(provider)

	items := []MediaItem{
		{Title: "Serenity", TMDbID: 16320},
	}

	if err := enricher.Enrich(items); err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if !items[0].IsRequested {
		t.Error("Expected Serenity to be marked as requested")
	}
	if items[0].RequestCount != 3 {
		t.Errorf("Expected RequestCount 3 (aggregated), got %d", items[0].RequestCount)
	}
	// First requestor is preserved
	if items[0].RequestedBy != "mal" {
		t.Errorf("Expected RequestedBy 'mal' (first requestor), got %q", items[0].RequestedBy)
	}
}

func TestRequestEnricher_SkipsItemsWithoutTMDbID(t *testing.T) {
	provider := &mockRequestProvider{
		requests: []MediaRequest{
			{MediaType: "movie", TMDbID: 16320, RequestedBy: "mal"},
		},
	}
	enricher := NewRequestEnricher(provider)

	items := []MediaItem{
		{Title: "Serenity", TMDbID: 0}, // No TMDb ID
	}

	if err := enricher.Enrich(items); err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if items[0].IsRequested {
		t.Error("Expected item without TMDb ID to not be marked as requested")
	}
	if items[0].RequestCount != 0 {
		t.Errorf("Expected RequestCount 0, got %d", items[0].RequestCount)
	}
}

func TestRequestEnricher_NoMatchingRequests(t *testing.T) {
	provider := &mockRequestProvider{
		requests: []MediaRequest{
			{MediaType: "movie", TMDbID: 99999, RequestedBy: "mal"},
		},
	}
	enricher := NewRequestEnricher(provider)

	items := []MediaItem{
		{Title: "Serenity", TMDbID: 16320},
	}

	if err := enricher.Enrich(items); err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if items[0].IsRequested {
		t.Error("Expected item with non-matching TMDb ID to not be requested")
	}
	if items[0].RequestedBy != "" {
		t.Errorf("Expected empty RequestedBy, got %q", items[0].RequestedBy)
	}
	if items[0].RequestCount != 0 {
		t.Errorf("Expected RequestCount 0, got %d", items[0].RequestCount)
	}
}

func TestRequestEnricher_PropagatesProviderError(t *testing.T) {
	provider := &mockRequestProvider{
		err: errConnectionFailed,
	}
	enricher := NewRequestEnricher(provider)

	items := []MediaItem{{Title: "Serenity", TMDbID: 16320}}

	if err := enricher.Enrich(items); err == nil {
		t.Fatal("Expected error from provider, got nil")
	}
}

func TestRequestEnricher_EmptyRequestList(t *testing.T) {
	provider := &mockRequestProvider{
		requests: []MediaRequest{},
	}
	enricher := NewRequestEnricher(provider)

	items := []MediaItem{
		{Title: "Serenity", TMDbID: 16320},
		{Title: "Firefly", TMDbID: 1437, Type: MediaTypeShow},
	}

	if err := enricher.Enrich(items); err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	for i, item := range items {
		if item.IsRequested {
			t.Errorf("Item %d (%s): expected not requested with empty request list", i, item.Title)
		}
		if item.RequestCount != 0 {
			t.Errorf("Item %d (%s): expected RequestCount 0, got %d", i, item.Title, item.RequestCount)
		}
	}
}

// ─── TracearrEnricher tests ─────────────────────────────────────────────────

func newTracearrHistoryServer(t *testing.T, movieResp, episodeResp string) *TracearrClient {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mediaType := r.URL.Query().Get("mediaType")
		var resp json.RawMessage
		if mediaType == "episode" {
			resp = json.RawMessage(episodeResp)
		} else {
			resp = json.RawMessage(movieResp)
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return NewTracearrClient(srv.URL, "test-key")
}

func TestTracearrEnricher_EnrichMovies(t *testing.T) {
	movieResp := `{"data": [
		{"mediaTitle": "Serenity", "showTitle": "", "mediaType": "movie", "year": 2005, "watched": true, "durationMs": 7200000, "user": {"username": "mal"}},
		{"mediaTitle": "Serenity", "showTitle": "", "mediaType": "movie", "year": 2005, "watched": true, "durationMs": 7200000, "user": {"username": "wash"}}
	], "pagination": {"page": 1, "pageSize": 100, "total": 2}}`
	episodeResp := `{"data": [], "pagination": {"page": 1, "pageSize": 100, "total": 0}}`

	client := newTracearrHistoryServer(t, movieResp, episodeResp)
	enricher := NewTracearrEnricher(client)

	items := []MediaItem{
		{Title: "Serenity", TMDbID: 16320, Year: 2005},
		{Title: "Firefly", TMDbID: 1437, Type: MediaTypeShow},
	}

	if err := enricher.Enrich(items); err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if items[0].PlayCount != 2 {
		t.Errorf("Expected PlayCount 2 for Serenity (2 sessions), got %d", items[0].PlayCount)
	}
	if len(items[0].WatchedByUsers) != 2 {
		t.Errorf("Expected 2 users for Serenity, got %d", len(items[0].WatchedByUsers))
	}
	if items[1].PlayCount != 0 {
		t.Errorf("Expected PlayCount 0 for Firefly (no match — no episodes), got %d", items[1].PlayCount)
	}
}

func TestTracearrEnricher_EnrichShows(t *testing.T) {
	movieResp := `{"data": [], "pagination": {"page": 1, "pageSize": 100, "total": 0}}`
	episodeResp := `{"data": [
		{"mediaTitle": "Out of Gas", "showTitle": "Firefly", "mediaType": "episode", "year": 2002, "watched": true, "durationMs": 2700000, "user": {"username": "kaylee"}},
		{"mediaTitle": "Shindig", "showTitle": "Firefly", "mediaType": "episode", "year": 2002, "watched": true, "durationMs": 2700000, "user": {"username": "inara"}}
	], "pagination": {"page": 1, "pageSize": 100, "total": 2}}`

	client := newTracearrHistoryServer(t, movieResp, episodeResp)
	enricher := NewTracearrEnricher(client)

	items := []MediaItem{
		{Title: "Firefly", TMDbID: 1437, Type: MediaTypeShow},
	}

	if err := enricher.Enrich(items); err != nil {
		t.Fatalf("Enrich failed: %v", err)
	}

	if items[0].PlayCount != 2 {
		t.Errorf("Expected PlayCount 2 for Firefly (2 episode sessions), got %d", items[0].PlayCount)
	}
	if len(items[0].WatchedByUsers) != 2 {
		t.Errorf("Expected 2 users for Firefly, got %d", len(items[0].WatchedByUsers))
	}
}

func TestTracearrEnricher_EmptyHistory(t *testing.T) {
	emptyResp := `{"data": [], "pagination": {"page": 1, "pageSize": 100, "total": 0}}`
	client := newTracearrHistoryServer(t, emptyResp, emptyResp)
	enricher := NewTracearrEnricher(client)

	items := []MediaItem{
		{Title: "Serenity", TMDbID: 16320, Year: 2005},
	}

	if err := enricher.Enrich(items); err != nil {
		t.Fatalf("Enrich should not fail with empty history: %v", err)
	}

	if items[0].PlayCount != 0 {
		t.Errorf("Expected PlayCount 0 (no history), got %d", items[0].PlayCount)
	}
}
