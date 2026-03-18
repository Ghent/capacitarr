package services

import (
	"testing"

	"capacitarr/internal/db"
	"capacitarr/internal/events"

	"gorm.io/gorm"
)

func setupLibraryTest(t *testing.T) (*LibraryService, *gorm.DB) {
	t.Helper()
	database := setupTestDB(t)
	bus := events.NewEventBus()
	svc := NewLibraryService(database, bus)
	return svc, database
}

func TestLibraryService_CRUD(t *testing.T) {
	svc, _ := setupLibraryTest(t)

	// Create
	lib := &db.Library{Name: "Firefly Library"}
	if err := svc.Create(lib); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if lib.ID == 0 {
		t.Fatal("expected non-zero ID after create")
	}

	// GetByID
	found, err := svc.GetByID(lib.ID)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if found.Name != "Firefly Library" {
		t.Errorf("expected name 'Firefly Library', got %q", found.Name)
	}

	// List
	libs, err := svc.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(libs) != 1 {
		t.Errorf("expected 1 library, got %d", len(libs))
	}

	// Update
	found.Name = "Serenity Library"
	if err := svc.Update(found); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	updated, _ := svc.GetByID(found.ID)
	if updated.Name != "Serenity Library" {
		t.Errorf("expected name 'Serenity Library', got %q", updated.Name)
	}

	// Delete
	if err := svc.Delete(lib.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	libs, _ = svc.List()
	if len(libs) != 0 {
		t.Errorf("expected 0 libraries after delete, got %d", len(libs))
	}
}

func TestLibraryService_CreateValidation(t *testing.T) {
	svc, _ := setupLibraryTest(t)

	// Empty name should fail
	err := svc.Create(&db.Library{Name: ""})
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestLibraryService_DeleteNotFound(t *testing.T) {
	svc, _ := setupLibraryTest(t)

	err := svc.Delete(999)
	if err == nil {
		t.Error("expected error deleting non-existent library")
	}
}

func TestLibraryService_EffectiveThreshold(t *testing.T) {
	svc, database := setupLibraryTest(t)

	// Create a disk group
	dg := db.DiskGroup{MountPath: "/data", TotalBytes: 1000, UsedBytes: 800, ThresholdPct: 90, TargetPct: 80}
	database.Create(&dg)

	// Create a library with custom thresholds
	thresh := 95.0
	target := 85.0
	lib := &db.Library{Name: "Firefly Library", DiskGroupID: &dg.ID, ThresholdPct: &thresh, TargetPct: &target}
	if err := svc.Create(lib); err != nil {
		t.Fatalf("Create library failed: %v", err)
	}

	// Create an integration with no overrides, assigned to library
	integ := db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989",
		APIKey: "key1", Enabled: true, LibraryID: &lib.ID,
	}
	database.Create(&integ)

	// Should get library threshold (95, 85) since integration has no override
	threshold, targetPct, err := svc.EffectiveThresholdForIntegration(integ.ID)
	if err != nil {
		t.Fatalf("EffectiveThresholdForIntegration failed: %v", err)
	}
	if threshold != 95.0 || targetPct != 85.0 {
		t.Errorf("expected (95, 85), got (%.1f, %.1f)", threshold, targetPct)
	}

	// Now set integration-level override
	integThresh := 98.0
	integTarget := 90.0
	integ.ThresholdPct = &integThresh
	integ.TargetPct = &integTarget
	database.Save(&integ)

	// Should get integration threshold (98, 90) — integration override takes priority
	threshold, targetPct, err = svc.EffectiveThresholdForIntegration(integ.ID)
	if err != nil {
		t.Fatalf("EffectiveThresholdForIntegration failed: %v", err)
	}
	if threshold != 98.0 || targetPct != 90.0 {
		t.Errorf("expected (98, 90), got (%.1f, %.1f)", threshold, targetPct)
	}
}

func TestLibraryService_EffectiveThresholdFallback(t *testing.T) {
	svc, database := setupLibraryTest(t)

	// Create an integration with no library and no disk group
	integ := db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989",
		APIKey: "key1", Enabled: true,
	}
	database.Create(&integ)

	// Should get ultimate default (85, 75)
	threshold, targetPct, err := svc.EffectiveThresholdForIntegration(integ.ID)
	if err != nil {
		t.Fatalf("EffectiveThresholdForIntegration failed: %v", err)
	}
	if threshold != 85.0 || targetPct != 75.0 {
		t.Errorf("expected (85, 75), got (%.1f, %.1f)", threshold, targetPct)
	}
}

func TestLibraryService_GetIntegrationsForLibrary(t *testing.T) {
	svc, database := setupLibraryTest(t)

	lib := &db.Library{Name: "Firefly Library"}
	if err := svc.Create(lib); err != nil {
		t.Fatalf("Create library failed: %v", err)
	}

	// Create integrations
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989",
		APIKey: "key1", Enabled: true, LibraryID: &lib.ID,
	})
	database.Create(&db.IntegrationConfig{
		Type: "radarr", Name: "Serenity Radarr", URL: "http://localhost:7878",
		APIKey: "key2", Enabled: true, LibraryID: &lib.ID,
	})
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Other Sonarr", URL: "http://localhost:8990",
		APIKey: "key3", Enabled: true, // no library
	})

	integrations, err := svc.GetIntegrationsForLibrary(lib.ID)
	if err != nil {
		t.Fatalf("GetIntegrationsForLibrary failed: %v", err)
	}
	if len(integrations) != 2 {
		t.Errorf("expected 2 integrations, got %d", len(integrations))
	}
}
