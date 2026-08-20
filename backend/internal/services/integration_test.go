package services

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
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

// TestIntegrationService_TestConnection_NoNotificationEvents verifies that the
// manual UI test button does NOT publish IntegrationTestFailedEvent or modify
// DB health state. The user is looking at the result in the UI.
func TestIntegrationService_TestConnection_NoNotificationEvents(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	// Test with an unreachable URL — should NOT publish any events
	result := svc.TestConnection("sonarr", "http://192.0.2.1:9999", "fake-key", nil)
	if result.Success {
		t.Fatal("expected failure for unreachable URL")
	}

	// Drain channel — no notification events should be present
	timeout := time.After(100 * time.Millisecond)
	for {
		select {
		case evt := <-ch:
			if evt.EventType() == "integration_test_failed" {
				t.Error("manual TestConnection MUST NOT publish integration_test_failed events")
			}
			if evt.EventType() == "integration_test" {
				t.Error("manual TestConnection MUST NOT publish integration_test events")
			}
		case <-timeout:
			return // good — no notification events
		}
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

func TestIntegrationService_FetchCollectionValues_NoMediaServers(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	// No media server integrations → empty result
	result, err := svc.FetchCollectionValues()
	if err != nil {
		t.Fatalf("FetchCollectionValues returned error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 collection values with no media servers, got %d", len(result))
	}
}

func TestIntegrationService_FetchCollectionValues_SkipsNonMediaServers(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	// Create a non-media-server integration — should be ignored by the switch
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

	// Use mock servers that return 401 Unauthorized immediately, so the
	// connection test fails fast without depending on network timeouts.
	tautulliSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer tautulliSrv.Close()

	seerrSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer seerrSrv.Close()

	// Tautulli and Seerr don't implement the full Integration interface,
	// but SyncAll should still test their connections and return results.
	database.Create(&db.IntegrationConfig{Type: "tautulli", Name: "Firefly Tautulli", URL: tautulliSrv.URL, APIKey: "key1", Enabled: true})
	database.Create(&db.IntegrationConfig{Type: "seerr", Name: "Serenity Seerr", URL: seerrSrv.URL, APIKey: "key2", Enabled: true})

	results, err := svc.SyncAll()
	if err != nil {
		t.Fatalf("SyncAll returned error: %v", err)
	}
	// Enrichment services now get tested — they'll fail (mock returns 401) but return results
	if len(results) != 2 {
		t.Errorf("expected 2 sync results (tautulli + seerr tested), got %d", len(results))
	}
	for _, r := range results {
		if r.Status != "error" {
			t.Errorf("expected error status for %s (mock 401), got %q", r.Type, r.Status)
		}
	}
}

func TestIntegrationService_SyncAll_ReportsHealthOnConnectionFailure(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)
	healthSvc := NewIntegrationHealthService(svc, bus)

	// Wire health reporter so SyncAll reports failures
	svc.SetHealthReporter(healthSvc)

	// Use a mock server that returns 401 so the connection test fails fast
	mockSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer mockSrv.Close()

	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: mockSrv.URL, APIKey: "key1", Enabled: true,
	})

	// Seed health service so it tracks the integration
	healthSvc.seed()

	// Integration starts healthy
	if !healthSvc.IsHealthy(1) {
		t.Fatal("expected integration to start healthy")
	}

	// SyncAll detects connection failure and reports to health service
	results, err := svc.SyncAll()
	if err != nil {
		t.Fatalf("SyncAll returned error: %v", err)
	}
	if len(results) != 1 || results[0].Status != "error" {
		t.Fatalf("expected 1 error result, got %d results with status %q", len(results), results[0].Status)
	}

	// Health service should now mark the integration as unhealthy
	if healthSvc.IsHealthy(1) {
		t.Error("expected integration to be unhealthy after SyncAll connection failure")
	}

	// ConsecutiveFailures should have been incremented
	svc2 := healthSvc
	svc2.mu.Lock()
	state := svc2.states[1]
	svc2.mu.Unlock()
	if state == nil {
		t.Fatal("expected health state for integration 1")
	}
	if state.ConsecutiveFailures != 1 {
		t.Errorf("expected 1 consecutive failure, got %d", state.ConsecutiveFailures)
	}
}

func TestIntegrationService_ShowLevelOnly_CreateAndUpdate(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	// Create with ShowLevelOnly enabled
	created, err := svc.Create(db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989",
		APIKey: "key1", Enabled: true, ShowLevelOnly: true,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if !created.ShowLevelOnly {
		t.Error("expected ShowLevelOnly=true after create")
	}

	// GetByID should preserve the field
	fetched, err := svc.GetByID(created.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if !fetched.ShowLevelOnly {
		t.Error("expected ShowLevelOnly=true from GetByID")
	}

	// Update to disable ShowLevelOnly
	updated, err := svc.Update(created.ID, db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989",
		APIKey: "key1", Enabled: true, ShowLevelOnly: false,
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.ShowLevelOnly {
		t.Error("expected ShowLevelOnly=false after update")
	}

	// Verify via GetByID
	fetched2, err := svc.GetByID(created.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if fetched2.ShowLevelOnly {
		t.Error("expected ShowLevelOnly=false from GetByID after update")
	}
}

func TestIntegrationService_ListEnabled_ShowLevelOnlyFilter(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	// Create two integrations — one with ShowLevelOnly, one without
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989",
		APIKey: "key1", Enabled: true, ShowLevelOnly: true,
	})
	database.Create(&db.IntegrationConfig{
		Type: "radarr", Name: "Serenity Radarr", URL: "http://localhost:7878",
		APIKey: "key2", Enabled: true, ShowLevelOnly: false,
	})

	enabled, err := svc.ListEnabled()
	if err != nil {
		t.Fatalf("ListEnabled returned error: %v", err)
	}

	if len(enabled) != 2 {
		t.Fatalf("expected 2 enabled integrations, got %d", len(enabled))
	}

	// Verify ShowLevelOnly is correctly read from DB
	showLevelCount := 0
	for _, cfg := range enabled {
		if cfg.ShowLevelOnly {
			showLevelCount++
			if cfg.Name != "Firefly Sonarr" {
				t.Errorf("expected ShowLevelOnly on 'Firefly Sonarr', got %q", cfg.Name)
			}
		}
	}
	if showLevelCount != 1 {
		t.Errorf("expected exactly 1 integration with ShowLevelOnly, got %d", showLevelCount)
	}
}

// TestMediaTypeOptions_MatchesConstants verifies that mediaTypeOptions stays
// in sync with the MediaType* constants in integrations/types.go. A drift
// means the rule editor autocomplete will be missing a valid media type
// (like the episode bug that was caught during this audit).
func TestMediaTypeOptions_MatchesConstants(t *testing.T) {
	// All known MediaType constants
	allTypes := []integrations.MediaType{
		integrations.MediaTypeMovie,
		integrations.MediaTypeShow,
		integrations.MediaTypeSeason,
		integrations.MediaTypeEpisode,
		integrations.MediaTypeArtist,
		integrations.MediaTypeBook,
	}

	// Build a set from mediaTypeOptions
	optionValues := make(map[string]bool, len(mediaTypeOptions))
	for _, opt := range mediaTypeOptions {
		optionValues[opt.Value] = true
	}

	// Every constant must be in the options
	for _, mt := range allTypes {
		if !optionValues[string(mt)] {
			t.Errorf("MediaType constant %q is missing from mediaTypeOptions", mt)
		}
	}

	// Every option must correspond to a known constant
	constantSet := make(map[string]bool, len(allTypes))
	for _, mt := range allTypes {
		constantSet[string(mt)] = true
	}
	for _, opt := range mediaTypeOptions {
		if !constantSet[opt.Value] {
			t.Errorf("mediaTypeOptions contains %q which is not a known MediaType constant", opt.Value)
		}
	}

	// Counts must match
	if len(mediaTypeOptions) != len(allTypes) {
		t.Errorf("mediaTypeOptions has %d entries but there are %d MediaType constants",
			len(mediaTypeOptions), len(allTypes))
	}
}

func TestIntegrationService_PartialUpdate(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	original := db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989", APIKey: "key1", Enabled: true,
	}
	database.Create(&original)

	// Seed a LastError so we can verify it gets cleared
	database.Model(&original).Updates(map[string]any{"last_error": "connection refused", "last_sync": time.Now()})

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	// Partial update: change name only, leave URL and APIKey empty
	result, err := svc.PartialUpdate(original.ID, IntegrationUpdate{
		Name: "Serenity Sonarr",
	})
	if err != nil {
		t.Fatalf("PartialUpdate returned error: %v", err)
	}

	if result.Name != "Serenity Sonarr" {
		t.Errorf("expected name 'Serenity Sonarr', got %q", result.Name)
	}
	// URL should be preserved
	if result.URL != "http://localhost:8989" {
		t.Errorf("expected URL preserved, got %q", result.URL)
	}

	// Verify LastError was cleared
	var reloaded db.IntegrationConfig
	database.First(&reloaded, original.ID)
	if reloaded.LastError != "" {
		t.Errorf("expected LastError to be cleared, got %q", reloaded.LastError)
	}
	if reloaded.LastSync != nil {
		t.Error("expected LastSync to be cleared")
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

func TestIntegrationService_PartialUpdate_BooleanPointers(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	original := db.IntegrationConfig{
		Type: "radarr", Name: "Firefly Radarr", URL: "http://localhost:7878", APIKey: "key1",
		Enabled: true, CollectionDeletion: false, ShowLevelOnly: false,
	}
	database.Create(&original)

	// Enable CollectionDeletion and ShowLevelOnly via pointer booleans
	trueVal := true
	result, err := svc.PartialUpdate(original.ID, IntegrationUpdate{
		CollectionDeletion: &trueVal,
		ShowLevelOnly:      &trueVal,
	})
	if err != nil {
		t.Fatalf("PartialUpdate returned error: %v", err)
	}

	if !result.CollectionDeletion {
		t.Error("expected CollectionDeletion to be true")
	}
	if !result.ShowLevelOnly {
		t.Error("expected ShowLevelOnly to be true")
	}

	// Now explicitly set to false
	falseVal := false
	result, err = svc.PartialUpdate(original.ID, IntegrationUpdate{
		CollectionDeletion: &falseVal,
	})
	if err != nil {
		t.Fatalf("PartialUpdate returned error: %v", err)
	}
	if result.CollectionDeletion {
		t.Error("expected CollectionDeletion to be false after explicit disable")
	}
	// ShowLevelOnly should remain true (nil pointer = no change)
	var reloaded db.IntegrationConfig
	database.First(&reloaded, original.ID)
	if !reloaded.ShowLevelOnly {
		t.Error("expected ShowLevelOnly to remain true when not in update")
	}
}

func TestIntegrationService_PartialUpdate_MaskedAPIKeyIgnored(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	originalKey := "key1"
	original := db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989", APIKey: originalKey, Enabled: true,
	}
	database.Create(&original)

	// Send a masked key (what the UI sends back) — should be ignored
	maskedKey := db.MaskAPIKey(originalKey)
	_, err := svc.PartialUpdate(original.ID, IntegrationUpdate{
		APIKey: maskedKey,
	})
	if err != nil {
		t.Fatalf("PartialUpdate returned error: %v", err)
	}

	// Verify the real key was NOT overwritten with the masked version
	var reloaded db.IntegrationConfig
	database.First(&reloaded, original.ID)
	if reloaded.APIKey != originalKey {
		t.Errorf("expected original API key to be preserved, got %q", reloaded.APIKey)
	}
}

func TestIntegrationService_PartialUpdate_NotFound(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	_, err := svc.PartialUpdate(99999, IntegrationUpdate{Name: "ghost"})
	if err == nil {
		t.Fatal("expected error for non-existent integration")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestIntegrationService_IsShowLevelOnlyEffective_StoredTrue(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)
	dgSvc := NewDiskGroupService(database, bus)
	svc.SetDiskGroupService(dgSvc)

	database.Create(&db.IntegrationConfig{
		Name: "Firefly Sonarr", Type: "sonarr", URL: "http://localhost:8989",
		APIKey: "key1", ShowLevelOnly: true,
	})

	effective, err := svc.IsShowLevelOnlyEffective(1)
	if err != nil {
		t.Fatalf("IsShowLevelOnlyEffective error: %v", err)
	}
	if !effective {
		t.Error("expected true when ShowLevelOnly is stored as true")
	}
}

func TestIntegrationService_IsShowLevelOnlyEffective_SunsetOverride(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)
	dgSvc := NewDiskGroupService(database, bus)
	svc.SetDiskGroupService(dgSvc)

	database.Create(&db.IntegrationConfig{
		Name: "Firefly Sonarr", Type: "sonarr", URL: "http://localhost:8989",
		APIKey: "key1", ShowLevelOnly: false,
	})
	sunsetPct := 60.0
	database.Create(&db.DiskGroup{
		MountPath: "/mnt/media", TotalBytes: 1000, UsedBytes: 500,
		Mode: db.ModeSunset, SunsetPct: &sunsetPct,
	})
	_ = dgSvc.SyncIntegrationLinks(1, []uint{1})

	effective, err := svc.IsShowLevelOnlyEffective(1)
	if err != nil {
		t.Fatalf("IsShowLevelOnlyEffective error: %v", err)
	}
	if !effective {
		t.Error("expected true when ShowLevelOnly=false but linked to sunset-mode disk group")
	}
}

func TestIntegrationService_IsShowLevelOnlyEffective_NoOverride(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)
	dgSvc := NewDiskGroupService(database, bus)
	svc.SetDiskGroupService(dgSvc)

	database.Create(&db.IntegrationConfig{
		Name: "Firefly Sonarr", Type: "sonarr", URL: "http://localhost:8989",
		APIKey: "key1", ShowLevelOnly: false,
	})
	database.Create(&db.DiskGroup{
		MountPath: "/mnt/media", TotalBytes: 1000, UsedBytes: 500,
		Mode: db.ModeDryRun,
	})
	_ = dgSvc.SyncIntegrationLinks(1, []uint{1})

	effective, err := svc.IsShowLevelOnlyEffective(1)
	if err != nil {
		t.Fatalf("IsShowLevelOnlyEffective error: %v", err)
	}
	if effective {
		t.Error("expected false when ShowLevelOnly=false and no sunset-mode disk groups")
	}
}

func TestIntegrationService_GetWithOverrideState_NoOverride(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)
	dgSvc := NewDiskGroupService(database, bus)
	svc.SetDiskGroupService(dgSvc)

	database.Create(&db.IntegrationConfig{
		Name: "Firefly Sonarr", Type: "sonarr", URL: "http://localhost:8989",
		APIKey: "key1", ShowLevelOnly: false,
	})

	resp, err := svc.GetWithOverrideState(1)
	if err != nil {
		t.Fatalf("GetWithOverrideState error: %v", err)
	}
	if resp.ShowLevelOnlyOverride {
		t.Error("expected ShowLevelOnlyOverride=false when not linked to sunset group")
	}
	if resp.ShowLevelOnlyOverrideReason != "" {
		t.Errorf("expected empty override reason, got %q", resp.ShowLevelOnlyOverrideReason)
	}
}

func TestIntegrationService_GetWithOverrideState_MasksAPIKey(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)
	dgSvc := NewDiskGroupService(database, bus)
	svc.SetDiskGroupService(dgSvc)

	const rawKey = "abcdef1234567890abcdef1234567890"
	database.Create(&db.IntegrationConfig{
		Name: "Firefly Sonarr", Type: "sonarr", URL: "http://localhost:8989",
		APIKey: rawKey, ShowLevelOnly: false,
	})

	resp, err := svc.GetWithOverrideState(1)
	if err != nil {
		t.Fatalf("GetWithOverrideState error: %v", err)
	}
	if resp.APIKey == rawKey {
		t.Error("GetWithOverrideState must mask the API key")
	}
	if !db.IsMaskedKey(resp.APIKey) {
		t.Errorf("expected masked API key, got %q", resp.APIKey)
	}

	stored, err := svc.GetByID(1)
	if err != nil {
		t.Fatalf("GetByID error: %v", err)
	}
	if stored.APIKey != rawKey {
		t.Errorf("GetByID must return the plaintext key for internal use, got %q", stored.APIKey)
	}
}

func TestIntegrationService_GetWithOverrideState_SunsetOverride(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)
	dgSvc := NewDiskGroupService(database, bus)
	svc.SetDiskGroupService(dgSvc)

	database.Create(&db.IntegrationConfig{
		Name: "Firefly Sonarr", Type: "sonarr", URL: "http://localhost:8989",
		APIKey: "key1", ShowLevelOnly: false,
	})
	sunsetPct := 60.0
	database.Create(&db.DiskGroup{
		MountPath: "/mnt/media", TotalBytes: 1000, UsedBytes: 500,
		Mode: db.ModeSunset, SunsetPct: &sunsetPct,
	})
	_ = dgSvc.SyncIntegrationLinks(1, []uint{1})

	resp, err := svc.GetWithOverrideState(1)
	if err != nil {
		t.Fatalf("GetWithOverrideState error: %v", err)
	}
	if !resp.ShowLevelOnlyOverride {
		t.Error("expected ShowLevelOnlyOverride=true when linked to sunset group")
	}
	if resp.ShowLevelOnlyOverrideReason == "" {
		t.Error("expected non-empty override reason")
	}
	// Stored value must remain unchanged
	if resp.ShowLevelOnly {
		t.Error("expected stored ShowLevelOnly to remain false")
	}
}

func TestIntegrationService_GetWithOverrideState_Radarr(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)
	dgSvc := NewDiskGroupService(database, bus)
	svc.SetDiskGroupService(dgSvc)

	database.Create(&db.IntegrationConfig{
		Name: "Serenity Radarr", Type: "radarr", URL: "http://localhost:7878",
		APIKey: "key1", ShowLevelOnly: false,
	})
	sunsetPct := 60.0
	database.Create(&db.DiskGroup{
		MountPath: "/mnt/media", TotalBytes: 1000, UsedBytes: 500,
		Mode: db.ModeSunset, SunsetPct: &sunsetPct,
	})
	_ = dgSvc.SyncIntegrationLinks(1, []uint{1})

	resp, err := svc.GetWithOverrideState(1)
	if err != nil {
		t.Fatalf("GetWithOverrideState error: %v", err)
	}
	if resp.ShowLevelOnlyOverride {
		t.Error("expected ShowLevelOnlyOverride=false for Radarr regardless of disk group mode")
	}
}

func TestIntegrationService_ListWithOverrideState(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)
	dgSvc := NewDiskGroupService(database, bus)
	svc.SetDiskGroupService(dgSvc)

	database.Create(&db.IntegrationConfig{
		Name: "Firefly Sonarr", Type: "sonarr", URL: "http://localhost:8989",
		APIKey: "key1", ShowLevelOnly: false,
	})
	database.Create(&db.IntegrationConfig{
		Name: "Serenity Radarr", Type: "radarr", URL: "http://localhost:7878",
		APIKey: "key2", ShowLevelOnly: false,
	})
	sunsetPct := 60.0
	database.Create(&db.DiskGroup{
		MountPath: "/mnt/media", TotalBytes: 1000, UsedBytes: 500,
		Mode: db.ModeSunset, SunsetPct: &sunsetPct,
	})
	_ = dgSvc.SyncIntegrationLinks(1, []uint{1, 2})

	results, err := svc.ListWithOverrideState()
	if err != nil {
		t.Fatalf("ListWithOverrideState error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Sonarr integration should have override
	if !results[0].ShowLevelOnlyOverride {
		t.Error("expected Sonarr integration to have ShowLevelOnlyOverride=true")
	}
	if results[0].ShowLevelOnlyOverrideReason == "" {
		t.Error("expected Sonarr integration to have non-empty override reason")
	}

	// Radarr integration should NOT have override
	if results[1].ShowLevelOnlyOverride {
		t.Error("expected Radarr integration to have ShowLevelOnlyOverride=false")
	}
}

func TestIntegrationService_GetWithOverrideState_AfterUpdate(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)
	dgSvc := NewDiskGroupService(database, bus)
	svc.SetDiskGroupService(dgSvc)

	database.Create(&db.IntegrationConfig{
		Name: "Firefly Sonarr", Type: "sonarr", URL: "http://localhost:8989",
		APIKey: "key1", ShowLevelOnly: false,
	})
	sunsetPct := 60.0
	database.Create(&db.DiskGroup{
		MountPath: "/mnt/media", TotalBytes: 1000, UsedBytes: 500,
		Mode: db.ModeSunset, SunsetPct: &sunsetPct,
	})
	_ = dgSvc.SyncIntegrationLinks(1, []uint{1})

	// Update integration name (non-ShowLevelOnly field)
	_, updateErr := svc.PartialUpdate(1, IntegrationUpdate{Name: "Firefly Sonarr Updated"})
	if updateErr != nil {
		t.Fatalf("PartialUpdate error: %v", updateErr)
	}

	// Override state should still be present after the update
	resp, err := svc.GetWithOverrideState(1)
	if err != nil {
		t.Fatalf("GetWithOverrideState after update error: %v", err)
	}
	if resp.Name != "Firefly Sonarr Updated" {
		t.Errorf("expected updated name, got %q", resp.Name)
	}
	if !resp.ShowLevelOnlyOverride {
		t.Error("expected ShowLevelOnlyOverride=true after update")
	}
	if resp.ShowLevelOnly {
		t.Error("expected stored ShowLevelOnly to remain false after update")
	}
}

func TestIntegrationService_IsShowLevelOnlyEffective_NonSonarr(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)
	dgSvc := NewDiskGroupService(database, bus)
	svc.SetDiskGroupService(dgSvc)

	database.Create(&db.IntegrationConfig{
		Name: "Serenity Radarr", Type: "radarr", URL: "http://localhost:7878",
		APIKey: "key1", ShowLevelOnly: false,
	})
	sunsetPct := 60.0
	database.Create(&db.DiskGroup{
		MountPath: "/mnt/media", TotalBytes: 1000, UsedBytes: 500,
		Mode: db.ModeSunset, SunsetPct: &sunsetPct,
	})
	_ = dgSvc.SyncIntegrationLinks(1, []uint{1})

	effective, err := svc.IsShowLevelOnlyEffective(1)
	if err != nil {
		t.Fatalf("IsShowLevelOnlyEffective error: %v", err)
	}
	if effective {
		t.Error("expected false for non-Sonarr integration regardless of disk group mode")
	}
}

func TestIntegrationService_AddImportExclusion_CreateAndUpdate(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	// Create with AddImportExclusion enabled (default)
	created, err := svc.Create(db.IntegrationConfig{
		Type: "radarr", Name: "Serenity Radarr", URL: "http://localhost:7878",
		APIKey: "key1", Enabled: true, AddImportExclusion: true,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if !created.AddImportExclusion {
		t.Error("expected AddImportExclusion=true after create")
	}

	// GetByID should preserve the field
	fetched, err := svc.GetByID(created.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if !fetched.AddImportExclusion {
		t.Error("expected AddImportExclusion=true from GetByID")
	}

	// Update to disable AddImportExclusion
	updated, err := svc.Update(created.ID, db.IntegrationConfig{
		Type: "radarr", Name: "Serenity Radarr", URL: "http://localhost:7878",
		APIKey: "key1", Enabled: true, AddImportExclusion: false,
	})
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}
	if updated.AddImportExclusion {
		t.Error("expected AddImportExclusion=false after update")
	}

	// Verify via GetByID
	fetched2, err := svc.GetByID(created.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if fetched2.AddImportExclusion {
		t.Error("expected AddImportExclusion=false from GetByID after update")
	}
}

func TestIntegrationService_PartialUpdate_AddImportExclusion(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	original := db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989", APIKey: "key1",
		Enabled: true, AddImportExclusion: true,
	}
	database.Create(&original)

	// Disable via partial update
	falseVal := false
	result, err := svc.PartialUpdate(original.ID, IntegrationUpdate{
		AddImportExclusion: &falseVal,
	})
	if err != nil {
		t.Fatalf("PartialUpdate returned error: %v", err)
	}
	if result.AddImportExclusion {
		t.Error("expected AddImportExclusion=false after partial update")
	}

	// Re-enable via partial update
	trueVal := true
	result, err = svc.PartialUpdate(original.ID, IntegrationUpdate{
		AddImportExclusion: &trueVal,
	})
	if err != nil {
		t.Fatalf("PartialUpdate returned error: %v", err)
	}
	if !result.AddImportExclusion {
		t.Error("expected AddImportExclusion=true after re-enable")
	}

	// Nil pointer should not change the value
	_, err = svc.PartialUpdate(original.ID, IntegrationUpdate{
		Name: "Firefly Sonarr Renamed",
	})
	if err != nil {
		t.Fatalf("PartialUpdate returned error: %v", err)
	}
	var reloaded db.IntegrationConfig
	database.First(&reloaded, original.ID)
	if !reloaded.AddImportExclusion {
		t.Error("expected AddImportExclusion to remain true when not in update")
	}
}

func TestIntegrationService_AddImportExclusion_DefaultTrue(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationService(database, bus)

	// Create without explicitly setting AddImportExclusion — GORM default:true
	// should apply. Note: Go zero-value for bool is false, so we need to verify
	// the GORM default takes effect for DB-level inserts.
	created, err := svc.Create(db.IntegrationConfig{
		Type: "lidarr", Name: "Firefly Lidarr", URL: "http://localhost:8686",
		APIKey: "key1", Enabled: true,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	// Reload from DB to verify GORM default
	var reloaded db.IntegrationConfig
	database.First(&reloaded, created.ID)
	if !reloaded.AddImportExclusion {
		t.Error("expected AddImportExclusion to default to true for new integrations")
	}
}
