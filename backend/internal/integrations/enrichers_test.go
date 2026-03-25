package integrations

import (
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
