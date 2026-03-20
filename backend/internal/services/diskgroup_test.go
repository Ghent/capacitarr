package services

import (
	"testing"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)

func TestDiskGroupService_List(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	database.Create(&db.DiskGroup{MountPath: "/mnt/a", TotalBytes: 100, UsedBytes: 50})
	database.Create(&db.DiskGroup{MountPath: "/mnt/b", TotalBytes: 200, UsedBytes: 100})

	groups, err := svc.List()
	if err != nil {
		t.Fatalf("List error: %v", err)
	}
	if len(groups) != 2 {
		t.Errorf("expected 2 disk groups, got %d", len(groups))
	}
}

func TestDiskGroupService_GetByID(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	database.Create(&db.DiskGroup{MountPath: "/mnt/media", TotalBytes: 1000, UsedBytes: 500})

	group, err := svc.GetByID(1)
	if err != nil {
		t.Fatalf("GetByID error: %v", err)
	}
	if group.MountPath != "/mnt/media" {
		t.Errorf("expected mount path '/mnt/media', got %q", group.MountPath)
	}
}

func TestDiskGroupService_Upsert_Create(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	disk := integrations.DiskSpace{Path: "/mnt/new", TotalBytes: 1000, FreeBytes: 400}
	group, err := svc.Upsert(disk)
	if err != nil {
		t.Fatalf("Upsert error: %v", err)
	}
	if group.MountPath != "/mnt/new" {
		t.Errorf("expected mount path '/mnt/new', got %q", group.MountPath)
	}
	if group.UsedBytes != 600 {
		t.Errorf("expected used bytes 600, got %d", group.UsedBytes)
	}
}

func TestDiskGroupService_Upsert_Update(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	// Create first
	_, _ = svc.Upsert(integrations.DiskSpace{Path: "/mnt/data", TotalBytes: 1000, FreeBytes: 500})

	// Update
	group, err := svc.Upsert(integrations.DiskSpace{Path: "/mnt/data", TotalBytes: 2000, FreeBytes: 800})
	if err != nil {
		t.Fatalf("Upsert update error: %v", err)
	}
	if group.TotalBytes != 2000 {
		t.Errorf("expected total bytes 2000, got %d", group.TotalBytes)
	}

	// Should still be 1 group
	var count int64
	database.Model(&db.DiskGroup{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 disk group, got %d", count)
	}
}

func TestDiskGroupService_UpdateThresholds(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	group := db.DiskGroup{
		MountPath:    "/mnt/media",
		TotalBytes:   1000000000,
		UsedBytes:    800000000,
		ThresholdPct: 85.0,
		TargetPct:    75.0,
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("Failed to create disk group: %v", err)
	}

	updated, err := svc.UpdateThresholds(group.ID, 90.0, 80.0, nil)
	if err != nil {
		t.Fatalf("UpdateThresholds returned error: %v", err)
	}

	if updated.ThresholdPct != 90.0 {
		t.Errorf("expected threshold 90.0, got %f", updated.ThresholdPct)
	}
	if updated.TargetPct != 80.0 {
		t.Errorf("expected target 80.0, got %f", updated.TargetPct)
	}
}

func TestDiskGroupService_UpdateThresholds_WithOverride(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	group := db.DiskGroup{
		MountPath:    "/mnt/media",
		TotalBytes:   1000000000000,
		UsedBytes:    800000000000,
		ThresholdPct: 85.0,
		TargetPct:    75.0,
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("Failed to create disk group: %v", err)
	}

	// Set override
	override := int64(500000000000)
	updated, err := svc.UpdateThresholds(group.ID, 85.0, 75.0, &override)
	if err != nil {
		t.Fatalf("UpdateThresholds with override returned error: %v", err)
	}
	if updated.TotalBytesOverride == nil || *updated.TotalBytesOverride != 500000000000 {
		t.Errorf("expected override 500000000000, got %v", updated.TotalBytesOverride)
	}
	if updated.EffectiveTotalBytes() != 500000000000 {
		t.Errorf("expected effective total 500000000000, got %d", updated.EffectiveTotalBytes())
	}

	// Clear override by passing nil
	cleared, err := svc.UpdateThresholds(group.ID, 85.0, 75.0, nil)
	if err != nil {
		t.Fatalf("UpdateThresholds clear override returned error: %v", err)
	}
	if cleared.TotalBytesOverride != nil {
		t.Errorf("expected override nil after clear, got %v", cleared.TotalBytesOverride)
	}
	if cleared.EffectiveTotalBytes() != 1000000000000 {
		t.Errorf("expected effective total to revert to detected, got %d", cleared.EffectiveTotalBytes())
	}
}

func TestDiskGroupService_UpdateThresholds_NotFound(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	_, err := svc.UpdateThresholds(99999, 90.0, 80.0, nil)
	if err == nil {
		t.Fatal("expected error for non-existent disk group")
	}
}

func TestDiskGroupService_RemoveAll(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	database.Create(&db.DiskGroup{MountPath: "/mnt/a", TotalBytes: 100, UsedBytes: 50})
	database.Create(&db.DiskGroup{MountPath: "/mnt/b", TotalBytes: 200, UsedBytes: 100})

	removed, err := svc.RemoveAll()
	if err != nil {
		t.Fatalf("RemoveAll error: %v", err)
	}
	if removed != 2 {
		t.Errorf("expected 2 removed, got %d", removed)
	}

	// Verify all gone
	groups, _ := svc.List()
	if len(groups) != 0 {
		t.Errorf("expected 0 disk groups after RemoveAll, got %d", len(groups))
	}
}

func TestDiskGroupService_RemoveAll_Empty(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	removed, err := svc.RemoveAll()
	if err != nil {
		t.Fatalf("RemoveAll on empty table error: %v", err)
	}
	if removed != 0 {
		t.Errorf("expected 0 removed from empty table, got %d", removed)
	}
}

func TestDiskGroupService_ReconcileActiveMounts(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	database.Create(&db.DiskGroup{MountPath: "/mnt/a", TotalBytes: 100, UsedBytes: 50})
	database.Create(&db.DiskGroup{MountPath: "/mnt/b", TotalBytes: 200, UsedBytes: 100})
	database.Create(&db.DiskGroup{MountPath: "/mnt/c", TotalBytes: 300, UsedBytes: 150})

	// Only /mnt/a is still active
	activeMounts := map[string]bool{"/mnt/a": true}
	deleted, err := svc.ReconcileActiveMounts(activeMounts)
	if err != nil {
		t.Fatalf("ReconcileActiveMounts error: %v", err)
	}
	if deleted != 2 {
		t.Errorf("expected 2 deleted, got %d", deleted)
	}

	// Verify only /mnt/a remains
	groups, _ := svc.List()
	if len(groups) != 1 {
		t.Fatalf("expected 1 disk group, got %d", len(groups))
	}
	if groups[0].MountPath != "/mnt/a" {
		t.Errorf("expected remaining mount '/mnt/a', got %q", groups[0].MountPath)
	}
}

func TestDiskGroupService_ImportUpsert(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	// Import creates new
	err := svc.ImportUpsert("/mnt/imported", 90.0, 80.0, nil)
	if err != nil {
		t.Fatalf("ImportUpsert create error: %v", err)
	}

	group, _ := svc.GetByID(1)
	if group.ThresholdPct != 90.0 {
		t.Errorf("expected threshold 90.0, got %f", group.ThresholdPct)
	}

	// Import updates existing
	err = svc.ImportUpsert("/mnt/imported", 85.0, 70.0, nil)
	if err != nil {
		t.Fatalf("ImportUpsert update error: %v", err)
	}

	group, _ = svc.GetByID(1)
	if group.ThresholdPct != 85.0 {
		t.Errorf("expected threshold 85.0 after update, got %f", group.ThresholdPct)
	}
}

func TestDiskGroupService_SyncIntegrationLinks(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	// Create disk group and integrations
	database.Create(&db.DiskGroup{MountPath: "/mnt/media", TotalBytes: 1000, UsedBytes: 500})
	database.Create(&db.IntegrationConfig{Name: "Firefly Sonarr", Type: "sonarr", URL: "http://localhost:8989", APIKey: "key1"})
	database.Create(&db.IntegrationConfig{Name: "Serenity Radarr", Type: "radarr", URL: "http://localhost:7878", APIKey: "key2"})

	// Sync links
	err := svc.SyncIntegrationLinks(1, []uint{1, 2})
	if err != nil {
		t.Fatalf("SyncIntegrationLinks error: %v", err)
	}

	// Verify via ListWithIntegrations
	groups, err := svc.ListWithIntegrations()
	if err != nil {
		t.Fatalf("ListWithIntegrations error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Integrations) != 2 {
		t.Errorf("expected 2 integrations, got %d", len(groups[0].Integrations))
	}

	// Re-sync with different set (simulating integration removal)
	err = svc.SyncIntegrationLinks(1, []uint{1})
	if err != nil {
		t.Fatalf("SyncIntegrationLinks re-sync error: %v", err)
	}

	groups, _ = svc.ListWithIntegrations()
	if len(groups[0].Integrations) != 1 {
		t.Errorf("expected 1 integration after re-sync, got %d", len(groups[0].Integrations))
	}
	if groups[0].Integrations[0].Type != "sonarr" {
		t.Errorf("expected integration type 'sonarr', got %q", groups[0].Integrations[0].Type)
	}
}

func TestDiskGroupService_ListWithIntegrations_Empty(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	groups, err := svc.ListWithIntegrations()
	if err != nil {
		t.Fatalf("ListWithIntegrations empty error: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
}

// mockEngineRunTrigger records whether TriggerRun was called.
type mockEngineRunTrigger struct {
	triggered bool
}

func (m *mockEngineRunTrigger) TriggerRun() string {
	m.triggered = true
	return "started"
}

func TestDiskGroupService_UpdateThresholds_TriggersEngineRun(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	mock := &mockEngineRunTrigger{}
	svc.SetEngineService(mock)

	group := db.DiskGroup{
		MountPath:    "/mnt/media",
		TotalBytes:   1000000000,
		UsedBytes:    800000000,
		ThresholdPct: 85.0,
		TargetPct:    75.0,
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("Failed to create disk group: %v", err)
	}

	// Update thresholds — should trigger an engine run
	_, err := svc.UpdateThresholds(group.ID, 90.0, 80.0, nil)
	if err != nil {
		t.Fatalf("UpdateThresholds returned error: %v", err)
	}

	if !mock.triggered {
		t.Error("expected engine run to be triggered after threshold change")
	}
}

func TestDiskGroupService_UpdateThresholds_NoEngineService(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)
	// Do NOT call SetEngineService — engine is nil

	group := db.DiskGroup{
		MountPath:    "/mnt/media",
		TotalBytes:   1000000000,
		UsedBytes:    800000000,
		ThresholdPct: 85.0,
		TargetPct:    75.0,
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("Failed to create disk group: %v", err)
	}

	// Should not panic when engine is nil
	_, err := svc.UpdateThresholds(group.ID, 90.0, 80.0, nil)
	if err != nil {
		t.Fatalf("UpdateThresholds returned error: %v", err)
	}
}

func TestDiskGroupService_RemoveAll_ClearsJunctionTable(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	database.Create(&db.DiskGroup{MountPath: "/mnt/media", TotalBytes: 1000, UsedBytes: 500})
	database.Create(&db.IntegrationConfig{Name: "Firefly Sonarr", Type: "sonarr", URL: "http://localhost:8989", APIKey: "key1"})
	_ = svc.SyncIntegrationLinks(1, []uint{1})

	// RemoveAll should also clear junction table
	_, err := svc.RemoveAll()
	if err != nil {
		t.Fatalf("RemoveAll error: %v", err)
	}

	var count int64
	database.Model(&db.DiskGroupIntegration{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 junction rows after RemoveAll, got %d", count)
	}
}
