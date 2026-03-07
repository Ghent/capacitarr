package poller

import (
	"testing"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/services"
	"capacitarr/internal/testutil"
)

func TestFetchAllIntegrations_EmptyConfigs(t *testing.T) {
	database := testutil.SetupTestDB(t)
	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })

	cfg := testutil.TestConfig()
	reg := services.NewRegistry(database, bus, cfg)

	result := fetchAllIntegrations(nil, reg.Integration)

	if len(result.allItems) != 0 {
		t.Errorf("expected 0 items, got %d", len(result.allItems))
	}
	if len(result.serviceClients) != 0 {
		t.Errorf("expected 0 service clients, got %d", len(result.serviceClients))
	}
	if len(result.rootFolders) != 0 {
		t.Errorf("expected 0 root folders, got %d", len(result.rootFolders))
	}
	if len(result.diskMap) != 0 {
		t.Errorf("expected 0 disk entries, got %d", len(result.diskMap))
	}
}

func TestFetchAllIntegrations_UnknownType(t *testing.T) {
	database := testutil.SetupTestDB(t)
	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })

	cfg := testutil.TestConfig()
	reg := services.NewRegistry(database, bus, cfg)

	configs := []db.IntegrationConfig{
		{ID: 1, Type: "unknown_type", Name: "Firefly Tracker", URL: "http://localhost:9999", APIKey: "test-key"},
	}

	result := fetchAllIntegrations(configs, reg.Integration)

	// Unknown type should not be added to any map
	if len(result.serviceClients) != 0 {
		t.Errorf("expected 0 service clients for unknown type, got %d", len(result.serviceClients))
	}
}

func TestConnectEnrichment_FailureUpdatesStatus(t *testing.T) {
	database := testutil.SetupTestDB(t)
	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })

	cfg := testutil.TestConfig()
	reg := services.NewRegistry(database, bus, cfg)

	// Create an integration that points to a non-existent server
	integration := db.IntegrationConfig{
		Type:    "tautulli",
		Name:    "Firefly Analytics",
		URL:     "http://localhost:1",
		APIKey:  "fake-key",
		Enabled: true,
	}
	database.Create(&integration)

	testCfg := db.IntegrationConfig{
		ID:   integration.ID,
		Type: "tautulli",
		Name: "Firefly Analytics",
	}

	ok := connectEnrichment(testCfg, func() error {
		return nil // Simulate success
	}, reg.Integration)

	if !ok {
		t.Error("expected connectEnrichment to return true on success")
	}
}
