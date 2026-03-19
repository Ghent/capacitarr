package poller

import (
	"testing"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
	"capacitarr/internal/services"
	"capacitarr/internal/testutil"
)

func TestFetchAllIntegrations_EmptyConfigs(t *testing.T) {
	database := testutil.SetupTestDB(t)
	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })

	cfg := testutil.TestConfig()
	reg := services.NewRegistry(database, bus, cfg)

	// Ensure factories are registered before fetch
	integrations.RegisterAllFactories()

	result := fetchAllIntegrations(reg.Integration)

	if len(result.allItems) != 0 {
		t.Errorf("expected 0 items, got %d", len(result.allItems))
	}
	if result.registry == nil {
		t.Error("expected non-nil registry")
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

	// Ensure factories are registered before fetch
	integrations.RegisterAllFactories()

	// Create an unknown-type integration in the DB so the registry
	// gets no factory match — the integration should be silently skipped.
	database.Create(&db.IntegrationConfig{
		Type: "unknown_type", Name: "Firefly Tracker", URL: "http://localhost:9999", APIKey: "test-key", Enabled: true,
	})

	result := fetchAllIntegrations(reg.Integration)

	// Unknown type has no factory, so it won't appear in any registry map.
	if len(result.allItems) != 0 {
		t.Errorf("expected 0 items for unknown type, got %d", len(result.allItems))
	}
}

func TestFetchAllIntegrations_RegistryAndPipeline(t *testing.T) {
	database := testutil.SetupTestDB(t)
	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })

	cfg := testutil.TestConfig()
	reg := services.NewRegistry(database, bus, cfg)

	// Ensure factories are registered before fetch
	integrations.RegisterAllFactories()

	result := fetchAllIntegrations(reg.Integration)

	if result.registry == nil {
		t.Fatal("expected non-nil IntegrationRegistry")
	}
	if result.pipeline == nil {
		t.Fatal("expected non-nil EnrichmentPipeline")
	}
	// With no integrations, pipeline should still have the cross-reference enricher
	if result.pipeline.Count() < 1 {
		t.Errorf("expected at least 1 enricher (cross-reference), got %d", result.pipeline.Count())
	}
}
