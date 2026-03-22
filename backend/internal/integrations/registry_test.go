package integrations

import (
	"testing"
	"time"
)

// ─── Test doubles ────────────────────────────────────────────────────────────

// mockFullClient implements all capability interfaces (like Sonarr/Radarr).
type mockFullClient struct{}

func (m *mockFullClient) TestConnection() error                    { return nil }
func (m *mockFullClient) GetMediaItems() ([]MediaItem, error)      { return nil, nil }
func (m *mockFullClient) GetDiskSpace() ([]DiskSpace, error)       { return nil, nil }
func (m *mockFullClient) GetRootFolders() ([]string, error)        { return nil, nil }
func (m *mockFullClient) DeleteMediaItem(_ MediaItem) error        { return nil }
func (m *mockFullClient) GetQualityProfiles() ([]NameValue, error) { return nil, nil }
func (m *mockFullClient) GetTags() ([]NameValue, error)            { return nil, nil }
func (m *mockFullClient) GetLanguages() ([]NameValue, error)       { return nil, nil }

// mockWatchClient implements Connectable + WatchDataProvider + WatchlistProvider (like Plex).
type mockWatchClient struct{}

func (m *mockWatchClient) TestConnection() error { return nil }
func (m *mockWatchClient) GetBulkWatchData() (map[int]*WatchData, error) {
	return map[int]*WatchData{
		16320: {PlayCount: 3, LastPlayed: timePtr(time.Now())},
	}, nil
}
func (m *mockWatchClient) GetWatchlistItems() (map[int]bool, error) {
	return map[int]bool{1437: true}, nil
}

// mockRequestClient implements Connectable + RequestProvider (like Seerr).
type mockRequestClient struct{}

func (m *mockRequestClient) TestConnection() error { return nil }
func (m *mockRequestClient) GetRequestedMedia() ([]MediaRequest, error) {
	return []MediaRequest{{MediaType: "movie", TMDbID: 16320, RequestedBy: "mal"}}, nil
}

func timePtr(t time.Time) *time.Time { return &t }

// ─── Tests ───────────────────────────────────────────────────────────────────

func TestRegistryRegisterAndDiscovery(t *testing.T) {
	reg := NewIntegrationRegistry()

	// Register a full client (like Sonarr)
	reg.Register(1, &mockFullClient{})
	// Register a watch client (like Plex)
	reg.Register(2, &mockWatchClient{})
	// Register a request client (like Seerr)
	reg.Register(3, &mockRequestClient{})

	// MediaSources: only the full client (mockFullClient implements MediaSource)
	sources := reg.MediaSources()
	if len(sources) != 1 {
		t.Errorf("expected 1 MediaSource, got %d", len(sources))
	}
	if _, ok := sources[1]; !ok {
		t.Error("expected integration 1 in MediaSources")
	}

	// DiskReporters: only the full client
	reporters := reg.DiskReporters()
	if len(reporters) != 1 {
		t.Errorf("expected 1 DiskReporter, got %d", len(reporters))
	}

	// WatchProviders: only the watch client
	watchProviders := reg.WatchProviders()
	if len(watchProviders) != 1 {
		t.Errorf("expected 1 WatchDataProvider, got %d", len(watchProviders))
	}
	if _, ok := watchProviders[2]; !ok {
		t.Error("expected integration 2 in WatchProviders")
	}

	// RequestProviders: only the request client
	requestProviders := reg.RequestProviders()
	if len(requestProviders) != 1 {
		t.Errorf("expected 1 RequestProvider, got %d", len(requestProviders))
	}
	if _, ok := requestProviders[3]; !ok {
		t.Error("expected integration 3 in RequestProviders")
	}

	// WatchlistProviders: only the watch client
	watchlistProviders := reg.WatchlistProviders()
	if len(watchlistProviders) != 1 {
		t.Errorf("expected 1 WatchlistProvider, got %d", len(watchlistProviders))
	}

	// Deleter: only the full client
	deleter, err := reg.Deleter(1)
	if err != nil {
		t.Errorf("expected deleter for integration 1, got error: %v", err)
	}
	if deleter == nil {
		t.Error("expected non-nil deleter")
	}

	// Deleter for non-deleting integration should error
	_, err = reg.Deleter(2)
	if err == nil {
		t.Error("expected error for deleter on integration 2 (watch-only)")
	}

	// RuleValueFetcher: only the full client
	fetcher, err := reg.RuleValueFetcherFor(1)
	if err != nil || fetcher == nil {
		t.Errorf("expected rule value fetcher for integration 1, got error: %v", err)
	}

	// Has-methods
	if !reg.HasWatchProviders() {
		t.Error("expected HasWatchProviders() to be true")
	}
	if !reg.HasRequestProviders() {
		t.Error("expected HasRequestProviders() to be true")
	}
}

func TestRegistryUnregister(t *testing.T) {
	reg := NewIntegrationRegistry()
	reg.Register(1, &mockFullClient{})
	reg.Register(2, &mockWatchClient{})

	reg.Unregister(1)

	sources := reg.MediaSources()
	if len(sources) != 0 {
		t.Errorf("expected 0 MediaSources after unregister, got %d", len(sources))
	}

	// Integration 2 should still be there
	if !reg.HasWatchProviders() {
		t.Error("expected HasWatchProviders() to be true after unregistering integration 1")
	}
}

func TestRegistryClear(t *testing.T) {
	reg := NewIntegrationRegistry()
	reg.Register(1, &mockFullClient{})
	reg.Register(2, &mockWatchClient{})
	reg.Register(3, &mockRequestClient{})

	reg.Clear()

	if len(reg.MediaSources()) != 0 {
		t.Error("expected 0 MediaSources after clear")
	}
	if reg.HasWatchProviders() {
		t.Error("expected HasWatchProviders() false after clear")
	}
	if reg.HasRequestProviders() {
		t.Error("expected HasRequestProviders() false after clear")
	}
}

func TestRegistryConnector(t *testing.T) {
	reg := NewIntegrationRegistry()
	reg.Register(1, &mockFullClient{})

	conn, err := reg.Connector(1)
	if err != nil || conn == nil {
		t.Errorf("expected connector for integration 1, got error: %v", err)
	}

	_, err = reg.Connector(999)
	if err == nil {
		t.Error("expected error for non-existent connector")
	}
}
