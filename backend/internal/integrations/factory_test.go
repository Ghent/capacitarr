package integrations

import (
	"testing"
)

func TestRegisterAllFactories(t *testing.T) {
	// Clear any previous registrations (tests run in same process)
	factoryRegistry = make(map[string]IntegrationFactory)

	RegisterAllFactories()

	expectedTypes := []string{
		"sonarr", "radarr", "lidarr", "readarr",
		"plex", "tautulli", "seerr", "jellyfin", "emby", "jellystat", "tracearr",
	}

	if len(factoryRegistry) != len(expectedTypes) {
		t.Errorf("expected %d factories, got %d", len(expectedTypes), len(factoryRegistry))
	}

	for _, intType := range expectedTypes {
		if !HasFactory(intType) {
			t.Errorf("expected factory for %q", intType)
		}
	}
}

func TestCreateClient_KnownType(t *testing.T) {
	factoryRegistry = make(map[string]IntegrationFactory)
	RegisterAllFactories()

	client := CreateClient("sonarr", "http://localhost:8989", "test-key")
	if client == nil {
		t.Fatal("expected non-nil client for sonarr")
	}

	// Verify it implements expected capabilities
	if _, ok := client.(Connectable); !ok {
		t.Error("sonarr client should implement Connectable")
	}
	if _, ok := client.(MediaSource); !ok {
		t.Error("sonarr client should implement MediaSource")
	}
	if _, ok := client.(DiskReporter); !ok {
		t.Error("sonarr client should implement DiskReporter")
	}
	if _, ok := client.(MediaDeleter); !ok {
		t.Error("sonarr client should implement MediaDeleter")
	}
}

func TestCreateClient_UnknownType(t *testing.T) {
	factoryRegistry = make(map[string]IntegrationFactory)
	RegisterAllFactories()

	client := CreateClient("unknown", "http://localhost", "key")
	if client != nil {
		t.Error("expected nil client for unknown type")
	}
}

func TestCreateClient_SeerrCapabilities(t *testing.T) {
	factoryRegistry = make(map[string]IntegrationFactory)
	RegisterAllFactories()

	client := CreateClient("seerr", "http://localhost:5055", "test-key")
	if client == nil {
		t.Fatal("expected non-nil client for seerr")
	}
	if _, ok := client.(Connectable); !ok {
		t.Error("seerr client should implement Connectable")
	}
	if _, ok := client.(RequestProvider); !ok {
		t.Error("seerr client should implement RequestProvider")
	}
	// Should NOT implement MediaSource
	if _, ok := client.(MediaSource); ok {
		t.Error("seerr client should NOT implement MediaSource")
	}
}

func TestCreateClient_PlexCapabilities(t *testing.T) {
	factoryRegistry = make(map[string]IntegrationFactory)
	RegisterAllFactories()

	client := CreateClient("plex", "http://localhost:32400", "test-token")
	if client == nil {
		t.Fatal("expected non-nil client for plex")
	}
	if _, ok := client.(Connectable); !ok {
		t.Error("plex client should implement Connectable")
	}
	if _, ok := client.(WatchlistProvider); !ok {
		t.Error("plex client should implement WatchlistProvider")
	}
	if _, ok := client.(MediaSource); ok {
		t.Error("plex client must NOT implement MediaSource — only *arr integrations should")
	}
}

func TestRegisteredTypes(t *testing.T) {
	factoryRegistry = make(map[string]IntegrationFactory)
	RegisterAllFactories()

	types := RegisteredTypes()
	if len(types) != 11 {
		t.Errorf("expected 11 registered types, got %d", len(types))
	}
}
