package services

import (
	"errors"
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

func TestIntegrationService_Create(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	config := db.IntegrationConfig{
		Type:   "sonarr",
		Name:   "My Sonarr",
		URL:    "http://localhost:8989",
		APIKey: "abc123",
	}

	result, err := svc.Create(config)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if result.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if result.Name != "My Sonarr" {
		t.Errorf("expected name 'My Sonarr', got %q", result.Name)
	}

	// Verify event
	select {
	case evt := <-ch:
		if evt.EventType() != "integration_added" {
			t.Errorf("expected event type 'integration_added', got %q", evt.EventType())
		}
		e, ok := evt.(events.IntegrationAddedEvent)
		if !ok {
			t.Fatal("event is not IntegrationAddedEvent")
		}
		if e.Name != "My Sonarr" {
			t.Errorf("expected event name 'My Sonarr', got %q", e.Name)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for integration_added event")
	}
}

func TestIntegrationService_Update(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	// Create first
	original := db.IntegrationConfig{
		Type: "sonarr", Name: "Original", URL: "http://localhost:8989", APIKey: "key1",
	}
	database.Create(&original)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	updated := db.IntegrationConfig{
		Type: "sonarr", Name: "Updated", URL: "http://localhost:8990", APIKey: "key2",
	}

	result, err := svc.Update(original.ID, updated)
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	if result.Name != "Updated" {
		t.Errorf("expected name 'Updated', got %q", result.Name)
	}
	if result.URL != "http://localhost:8990" {
		t.Errorf("expected URL 'http://localhost:8990', got %q", result.URL)
	}

	// Verify event
	select {
	case evt := <-ch:
		if evt.EventType() != "integration_updated" {
			t.Errorf("expected event type 'integration_updated', got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for integration_updated event")
	}
}

func TestIntegrationService_Update_NotFound(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	_, err := svc.Update(99999, db.IntegrationConfig{Name: "ghost"})
	if err == nil {
		t.Fatal("expected error for non-existent integration")
	}
}

func TestIntegrationService_Delete(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	config := db.IntegrationConfig{
		Type: "radarr", Name: "My Radarr", URL: "http://localhost:7878", APIKey: "key1",
	}
	database.Create(&config)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	if err := svc.Delete(config.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	// Verify deleted from DB
	var count int64
	database.Model(&db.IntegrationConfig{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 integrations after delete, got %d", count)
	}

	// Verify event
	select {
	case evt := <-ch:
		if evt.EventType() != "integration_removed" {
			t.Errorf("expected event type 'integration_removed', got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for integration_removed event")
	}
}

func TestIntegrationService_Delete_NotFound(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	err := svc.Delete(99999)
	if err == nil {
		t.Fatal("expected error for non-existent integration")
	}
}

func TestIntegrationService_Delete_RemovesDiskGroupsWhenLastDeleted(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	// Wire up a DiskGroupService
	dgSvc := NewDiskGroupService(database, bus)
	svc.SetDiskGroupService(dgSvc)

	// Create an integration and some disk groups
	config := db.IntegrationConfig{
		Type: "radarr", Name: "Serenity Radarr", URL: "http://localhost:7878", APIKey: "key1", Enabled: true,
	}
	database.Create(&config)
	database.Create(&db.DiskGroup{MountPath: "/mnt/media", TotalBytes: 1000, UsedBytes: 500})

	// Delete the only integration — disk groups should be removed
	if err := svc.Delete(config.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	var dgCount int64
	database.Model(&db.DiskGroup{}).Count(&dgCount)
	if dgCount != 0 {
		t.Errorf("expected 0 disk groups after last integration deleted, got %d", dgCount)
	}
}

func TestIntegrationService_Delete_KeepsDiskGroupsWhenOthersExist(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	dgSvc := NewDiskGroupService(database, bus)
	svc.SetDiskGroupService(dgSvc)

	// Create two integrations and a disk group
	config1 := db.IntegrationConfig{
		Type: "radarr", Name: "Serenity Radarr", URL: "http://localhost:7878", APIKey: "key1", Enabled: true,
	}
	config2 := db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989", APIKey: "key2", Enabled: true,
	}
	database.Create(&config1)
	database.Create(&config2)
	database.Create(&db.DiskGroup{MountPath: "/mnt/media", TotalBytes: 1000, UsedBytes: 500})

	// Delete one — disk groups should remain
	if err := svc.Delete(config1.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	var dgCount int64
	database.Model(&db.DiskGroup{}).Count(&dgCount)
	if dgCount != 1 {
		t.Errorf("expected 1 disk group when other integrations remain, got %d", dgCount)
	}
}

func TestIntegrationService_PublishTestSuccess(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	svc.PublishTestSuccess("sonarr", "My Sonarr", "http://localhost:8989")

	select {
	case evt := <-ch:
		if evt.EventType() != "integration_test" {
			t.Errorf("expected event type 'integration_test', got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for integration_test event")
	}
}

func TestIntegrationService_PublishTestFailure(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	svc.PublishTestFailure("sonarr", "My Sonarr", "http://localhost:8989", "connection refused")

	select {
	case evt := <-ch:
		if evt.EventType() != "integration_test_failed" {
			t.Errorf("expected event type 'integration_test_failed', got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for integration_test_failed event")
	}
}

func TestIntegrationService_List(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	// Empty list initially
	configs, err := svc.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(configs) != 0 {
		t.Errorf("expected 0 integrations, got %d", len(configs))
	}

	// Insert two integrations
	database.Create(&db.IntegrationConfig{Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989", APIKey: "key1"})
	database.Create(&db.IntegrationConfig{Type: "radarr", Name: "Serenity Radarr", URL: "http://localhost:7878", APIKey: "key2"})

	configs, err = svc.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 integrations, got %d", len(configs))
	}
	if configs[0].Name != "Firefly Sonarr" {
		t.Errorf("expected first integration 'Firefly Sonarr', got %q", configs[0].Name)
	}
}

func TestIntegrationService_GetByID(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	ic := db.IntegrationConfig{Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989", APIKey: "key1"}
	database.Create(&ic)

	config, err := svc.GetByID(ic.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if config.Name != "Firefly Sonarr" {
		t.Errorf("expected name 'Firefly Sonarr', got %q", config.Name)
	}
}

func TestIntegrationService_GetByID_NotFound(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	_, err := svc.GetByID(99999)
	if err == nil {
		t.Fatal("expected error for non-existent integration")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegrationService_ListEnabled(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	database.Create(&db.IntegrationConfig{Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989", APIKey: "key1", Enabled: true})
	disabled := db.IntegrationConfig{Type: "radarr", Name: "Serenity Radarr", URL: "http://localhost:7878", APIKey: "key2", Enabled: true}
	database.Create(&disabled)
	// Explicitly disable — GORM default:true ignores zero-value false on Create
	database.Model(&disabled).Update("enabled", false)

	configs, err := svc.ListEnabled()
	if err != nil {
		t.Fatalf("ListEnabled returned error: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 enabled integration, got %d", len(configs))
	}
	if configs[0].Name != "Firefly Sonarr" {
		t.Errorf("expected 'Firefly Sonarr', got %q", configs[0].Name)
	}
}

func TestIntegrationService_UpdateSyncStatus(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	ic := db.IntegrationConfig{Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989", APIKey: "key1"}
	database.Create(&ic)

	now := time.Now().UTC().Truncate(time.Second)
	err := svc.UpdateSyncStatus(ic.ID, &now, "")
	if err != nil {
		t.Fatalf("UpdateSyncStatus returned error: %v", err)
	}

	// Verify the update
	var updated db.IntegrationConfig
	database.First(&updated, ic.ID)
	if updated.LastSync == nil {
		t.Fatal("expected LastSync to be set")
	}
	if updated.LastError != "" {
		t.Errorf("expected empty LastError, got %q", updated.LastError)
	}

	// Update with an error
	err = svc.UpdateSyncStatus(ic.ID, &now, "connection timeout")
	if err != nil {
		t.Fatalf("UpdateSyncStatus returned error: %v", err)
	}

	database.First(&updated, ic.ID)
	if updated.LastError != "connection timeout" {
		t.Errorf("expected LastError 'connection timeout', got %q", updated.LastError)
	}
}

func TestIntegrationService_UpdateSyncStatus_NotFound(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	now := time.Now()
	err := svc.UpdateSyncStatus(99999, &now, "some error")
	if err == nil {
		t.Fatal("expected error for non-existent integration")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegrationService_SyncAll_NoEnabled(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	// No integrations → empty results
	results, err := svc.SyncAll()
	if err != nil {
		t.Fatalf("SyncAll returned error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 sync results, got %d", len(results))
	}
}

func TestIntegrationService_FetchCollectionValues_NoPlex(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	// No Plex integrations → empty result
	result, err := svc.FetchCollectionValues()
	if err != nil {
		t.Fatalf("FetchCollectionValues returned error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 collection values with no Plex, got %d", len(result))
	}
}

func TestIntegrationService_FetchCollectionValues_SkipsNonPlex(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	// Create a non-Plex integration — should be ignored
	database.Create(&db.IntegrationConfig{Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989", APIKey: "key1", Enabled: true})

	result, err := svc.FetchCollectionValues()
	if err != nil {
		t.Fatalf("FetchCollectionValues returned error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 collection values with only Sonarr, got %d", len(result))
	}
}

func TestIntegrationService_FetchRuleValues_Collection(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	// With no Plex integrations, collection action should still work (empty suggestions)
	result, err := svc.FetchRuleValues(0, "collection")
	if err != nil {
		t.Fatalf("FetchRuleValues(collection) returned error: %v", err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatal("expected map[string]any result")
	}
	if m["type"] != "combobox" {
		t.Errorf("expected type 'combobox', got %v", m["type"])
	}
}

func TestIntegrationService_SyncAll_TestsEnrichmentTypes(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	// Tautulli and Overseerr don't implement the full Integration interface,
	// but SyncAll should still test their connections and return results.
	database.Create(&db.IntegrationConfig{Type: "tautulli", Name: "Firefly Tautulli", URL: "http://localhost:8181", APIKey: "key1", Enabled: true})
	database.Create(&db.IntegrationConfig{Type: "overseerr", Name: "Serenity Overseerr", URL: "http://localhost:5055", APIKey: "key2", Enabled: true})

	results, err := svc.SyncAll()
	if err != nil {
		t.Fatalf("SyncAll returned error: %v", err)
	}
	// Enrichment services now get tested — they'll fail (unreachable) but return results
	if len(results) != 2 {
		t.Errorf("expected 2 sync results (tautulli + overseerr tested), got %d", len(results))
	}
	for _, r := range results {
		if r.Status != "error" {
			t.Errorf("expected error status for %s (unreachable), got %q", r.Type, r.Status)
		}
	}
}
