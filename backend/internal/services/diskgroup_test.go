package services

import (
	"testing"
	"time"

	"gorm.io/gorm"

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

func TestDiskGroupService_Upsert_Create_InheritsDefaultMode(t *testing.T) {
	tests := []struct {
		name         string
		prefMode     string
		expectedMode string
	}{
		{"inherits auto mode", db.ModeAuto, db.ModeAuto},
		{"inherits approval mode", db.ModeApproval, db.ModeApproval},
		{"inherits sunset mode", db.ModeSunset, db.ModeSunset},
		{"inherits dry-run mode", db.ModeDryRun, db.ModeDryRun},
		{"empty preference falls back to GORM default", "", db.ModeDryRun},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			database := setupTestDB(t)
			bus := newTestBus(t)
			svc := NewDiskGroupService(database, bus)
			svc.SetSettingsReader(&mockSettingsReaderForDiskGroup{mode: tc.prefMode})

			disk := integrations.DiskSpace{Path: "/mnt/new", TotalBytes: 1000, FreeBytes: 400}
			group, err := svc.Upsert(disk)
			if err != nil {
				t.Fatalf("Upsert error: %v", err)
			}
			if group.Mode != tc.expectedMode {
				t.Errorf("expected mode %q, got %q", tc.expectedMode, group.Mode)
			}
		})
	}
}

func TestDiskGroupService_Upsert_Create_NoSettingsReader(t *testing.T) {
	// When no SettingsReader is wired, new groups should get the GORM column default (dry-run)
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	disk := integrations.DiskSpace{Path: "/mnt/new", TotalBytes: 1000, FreeBytes: 400}
	group, err := svc.Upsert(disk)
	if err != nil {
		t.Fatalf("Upsert error: %v", err)
	}
	if group.Mode != db.ModeDryRun {
		t.Errorf("expected default mode %q, got %q", db.ModeDryRun, group.Mode)
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

	updated, err := svc.UpdateThresholds(group.ID, 90.0, 80.0, nil, "", nil)
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
	updated, err := svc.UpdateThresholds(group.ID, 85.0, 75.0, &override, "", nil)
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
	cleared, err := svc.UpdateThresholds(group.ID, 85.0, 75.0, nil, "", nil)
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

	_, err := svc.UpdateThresholds(99999, 90.0, 80.0, nil, "", nil)
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

func TestDiskGroupService_ReconcileActiveMounts_MarksStaleInsteadOfDeleting(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	database.Create(&db.DiskGroup{MountPath: "/mnt/a", TotalBytes: 100, UsedBytes: 50})
	database.Create(&db.DiskGroup{MountPath: "/mnt/b", TotalBytes: 200, UsedBytes: 100})
	database.Create(&db.DiskGroup{MountPath: "/mnt/c", TotalBytes: 300, UsedBytes: 150})

	// Only /mnt/a is still active
	activeMounts := map[string]bool{"/mnt/a": true}
	stale, err := svc.ReconcileActiveMounts(activeMounts)
	if err != nil {
		t.Fatalf("ReconcileActiveMounts error: %v", err)
	}
	if stale != 2 {
		t.Errorf("expected 2 newly stale, got %d", stale)
	}

	// All 3 groups should still exist — stale ones are not deleted
	groups, _ := svc.List()
	if len(groups) != 3 {
		t.Fatalf("expected 3 disk groups (stale not deleted), got %d", len(groups))
	}

	// Verify stale_since is set on orphaned groups
	staleGroups, _ := svc.ListStale()
	if len(staleGroups) != 2 {
		t.Fatalf("expected 2 stale groups, got %d", len(staleGroups))
	}
}

func TestDiskGroupService_ReconcileActiveMounts_EmptyMapMarksAllStale(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	// Create groups with custom thresholds (simulating user configuration)
	database.Create(&db.DiskGroup{MountPath: "/mnt/a", TotalBytes: 100, UsedBytes: 50, ThresholdPct: 90, TargetPct: 80})
	database.Create(&db.DiskGroup{MountPath: "/mnt/b", TotalBytes: 200, UsedBytes: 100, ThresholdPct: 92, TargetPct: 82})

	// Empty active mounts marks groups stale but doesn't delete them.
	stale, err := svc.ReconcileActiveMounts(map[string]bool{})
	if err != nil {
		t.Fatalf("ReconcileActiveMounts error: %v", err)
	}
	if stale != 2 {
		t.Errorf("expected 2 stale, got %d", stale)
	}

	// Groups still exist — thresholds are preserved
	groups, _ := svc.List()
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups after reconcile (stale, not deleted), got %d", len(groups))
	}

	// Resurrect via Upsert — thresholds should be preserved (not reverted to defaults)
	g, err := svc.Upsert(integrations.DiskSpace{Path: "/mnt/a", TotalBytes: 100, FreeBytes: 50})
	if err != nil {
		t.Fatalf("Upsert error: %v", err)
	}
	// Reload to get the preserved threshold values
	reloaded, _ := svc.GetByID(g.ID)
	if reloaded.ThresholdPct != 90 {
		t.Errorf("expected preserved threshold 90, got %f", reloaded.ThresholdPct)
	}
	if reloaded.TargetPct != 80 {
		t.Errorf("expected preserved target 80, got %f", reloaded.TargetPct)
	}
	if reloaded.StaleSince != nil {
		t.Error("expected stale_since to be nil after resurrection")
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

	group, err = svc.GetByID(1)
	if err != nil {
		t.Fatal("GetByID after update:", err)
	}
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
	_, err := svc.UpdateThresholds(group.ID, 90.0, 80.0, nil, "", nil)
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
	_, err := svc.UpdateThresholds(group.ID, 90.0, 80.0, nil, "", nil)
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

func TestDiskGroupService_HasSunsetModeForIntegration_NotLinked(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	database.Create(&db.IntegrationConfig{Name: "Firefly Sonarr", Type: "sonarr", URL: "http://localhost:8989", APIKey: "key1"})

	has, err := svc.HasSunsetModeForIntegration(1)
	if err != nil {
		t.Fatalf("HasSunsetModeForIntegration error: %v", err)
	}
	if has {
		t.Error("expected false when integration is not linked to any disk group")
	}
}

func TestDiskGroupService_HasSunsetModeForIntegration_DryRunMode(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	database.Create(&db.DiskGroup{MountPath: "/mnt/media", TotalBytes: 1000, UsedBytes: 500, Mode: db.ModeDryRun})
	database.Create(&db.IntegrationConfig{Name: "Firefly Sonarr", Type: "sonarr", URL: "http://localhost:8989", APIKey: "key1"})
	_ = svc.SyncIntegrationLinks(1, []uint{1})

	has, err := svc.HasSunsetModeForIntegration(1)
	if err != nil {
		t.Fatalf("HasSunsetModeForIntegration error: %v", err)
	}
	if has {
		t.Error("expected false when linked disk group is in dry-run mode")
	}
}

func TestDiskGroupService_HasSunsetModeForIntegration_SunsetMode(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	sunsetPct := 60.0
	database.Create(&db.DiskGroup{MountPath: "/mnt/media", TotalBytes: 1000, UsedBytes: 500, Mode: db.ModeSunset, SunsetPct: &sunsetPct})
	database.Create(&db.IntegrationConfig{Name: "Firefly Sonarr", Type: "sonarr", URL: "http://localhost:8989", APIKey: "key1"})
	_ = svc.SyncIntegrationLinks(1, []uint{1})

	has, err := svc.HasSunsetModeForIntegration(1)
	if err != nil {
		t.Fatalf("HasSunsetModeForIntegration error: %v", err)
	}
	if !has {
		t.Error("expected true when linked disk group is in sunset mode")
	}
}

func TestDiskGroupService_HasSunsetModeForIntegration_MultipleGroups_OneSunset(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	sunsetPct := 60.0
	database.Create(&db.DiskGroup{MountPath: "/mnt/media1", TotalBytes: 1000, UsedBytes: 500, Mode: db.ModeDryRun})
	database.Create(&db.DiskGroup{MountPath: "/mnt/media2", TotalBytes: 2000, UsedBytes: 1000, Mode: db.ModeSunset, SunsetPct: &sunsetPct})
	database.Create(&db.IntegrationConfig{Name: "Firefly Sonarr", Type: "sonarr", URL: "http://localhost:8989", APIKey: "key1"})

	// Link integration to both disk groups
	_ = svc.SyncIntegrationLinks(1, []uint{1})
	_ = svc.SyncIntegrationLinks(2, []uint{1})

	has, err := svc.HasSunsetModeForIntegration(1)
	if err != nil {
		t.Fatalf("HasSunsetModeForIntegration error: %v", err)
	}
	if !has {
		t.Error("expected true when at least one linked disk group is in sunset mode")
	}
}

// =============================================================================
// Stale lifecycle tests
// =============================================================================

func TestDiskGroupService_MarkStale_SetsTimestamp(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	database.Create(&db.DiskGroup{MountPath: "/mnt/media", TotalBytes: 1000, UsedBytes: 500})

	if err := svc.MarkStale(1); err != nil {
		t.Fatalf("MarkStale error: %v", err)
	}

	group, _ := svc.GetByID(1)
	if group.StaleSince == nil {
		t.Fatal("expected stale_since to be set")
	}
}

func TestDiskGroupService_MarkStale_Idempotent(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	earlier := time.Now().Add(-24 * time.Hour)
	database.Create(&db.DiskGroup{MountPath: "/mnt/media", TotalBytes: 1000, UsedBytes: 500, StaleSince: &earlier})

	// Calling MarkStale on an already-stale group should not reset the clock
	if err := svc.MarkStale(1); err != nil {
		t.Fatalf("MarkStale error: %v", err)
	}

	group, _ := svc.GetByID(1)
	if group.StaleSince == nil {
		t.Fatal("expected stale_since to remain set")
	}
	// The stale_since should still be approximately 24 hours ago, not now
	if time.Since(*group.StaleSince) < 23*time.Hour {
		t.Error("expected stale_since to remain at the original timestamp, not reset to now")
	}
}

func TestDiskGroupService_MarkAllStale_MarksOnlyActive(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	earlier := time.Now().Add(-48 * time.Hour)
	database.Create(&db.DiskGroup{MountPath: "/mnt/a", TotalBytes: 100, UsedBytes: 50})                        // active
	database.Create(&db.DiskGroup{MountPath: "/mnt/b", TotalBytes: 200, UsedBytes: 100, StaleSince: &earlier}) // already stale
	database.Create(&db.DiskGroup{MountPath: "/mnt/c", TotalBytes: 300, UsedBytes: 150})                       // active

	marked, err := svc.MarkAllStale()
	if err != nil {
		t.Fatalf("MarkAllStale error: %v", err)
	}
	if marked != 2 {
		t.Errorf("expected 2 newly stale, got %d", marked)
	}

	// All 3 should now be stale
	stale, _ := svc.ListStale()
	if len(stale) != 3 {
		t.Errorf("expected 3 stale groups, got %d", len(stale))
	}

	// The already-stale group's timestamp should NOT have been reset
	for _, g := range stale {
		if g.MountPath == "/mnt/b" {
			if time.Since(*g.StaleSince) < 47*time.Hour {
				t.Error("expected /mnt/b stale_since to remain at the original timestamp")
			}
		}
	}
}

func TestDiskGroupService_Upsert_ResurrectsStaleGroup(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	earlier := time.Now().Add(-72 * time.Hour)
	database.Create(&db.DiskGroup{
		MountPath:    "/mnt/media",
		TotalBytes:   1000,
		UsedBytes:    500,
		ThresholdPct: 90,
		TargetPct:    80,
		StaleSince:   &earlier,
	})

	// Upsert should resurrect the stale group
	group, err := svc.Upsert(integrations.DiskSpace{Path: "/mnt/media", TotalBytes: 2000, FreeBytes: 800})
	if err != nil {
		t.Fatalf("Upsert error: %v", err)
	}
	if group.StaleSince != nil {
		t.Error("expected stale_since to be nil after resurrection")
	}

	// Reload and verify thresholds are preserved
	reloaded, _ := svc.GetByID(group.ID)
	if reloaded.ThresholdPct != 90 {
		t.Errorf("expected preserved threshold 90, got %f", reloaded.ThresholdPct)
	}
	if reloaded.TargetPct != 80 {
		t.Errorf("expected preserved target 80, got %f", reloaded.TargetPct)
	}
}

func TestDiskGroupService_ReapStale_DeletesExpiredGroups(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	old := time.Now().Add(-10 * 24 * time.Hour) // 10 days ago
	database.Create(&db.DiskGroup{MountPath: "/mnt/old", TotalBytes: 100, UsedBytes: 50, StaleSince: &old})
	database.Create(&db.DiskGroup{MountPath: "/mnt/active", TotalBytes: 200, UsedBytes: 100}) // active

	reaped, err := svc.ReapStale(7) // Grace period: 7 days
	if err != nil {
		t.Fatalf("ReapStale error: %v", err)
	}
	if reaped != 1 {
		t.Errorf("expected 1 reaped, got %d", reaped)
	}

	groups, _ := svc.List()
	if len(groups) != 1 {
		t.Fatalf("expected 1 group remaining, got %d", len(groups))
	}
	if groups[0].MountPath != "/mnt/active" {
		t.Errorf("expected remaining group '/mnt/active', got %q", groups[0].MountPath)
	}
}

func TestDiskGroupService_ReapStale_PreservesRecentlyStaleGroups(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	recent := time.Now().Add(-2 * 24 * time.Hour) // 2 days ago
	database.Create(&db.DiskGroup{MountPath: "/mnt/recent", TotalBytes: 100, UsedBytes: 50, StaleSince: &recent})

	reaped, err := svc.ReapStale(7) // Grace period: 7 days — should NOT reap 2-day-old stale group
	if err != nil {
		t.Fatalf("ReapStale error: %v", err)
	}
	if reaped != 0 {
		t.Errorf("expected 0 reaped (within grace period), got %d", reaped)
	}
}

func TestDiskGroupService_ReapStale_ZeroGracePeriod(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	recent := time.Now().Add(-1 * time.Hour) // 1 hour ago
	database.Create(&db.DiskGroup{MountPath: "/mnt/media", TotalBytes: 100, UsedBytes: 50, StaleSince: &recent})

	reaped, err := svc.ReapStale(0) // Grace period: 0 = reap all stale immediately
	if err != nil {
		t.Fatalf("ReapStale error: %v", err)
	}
	if reaped != 1 {
		t.Errorf("expected 1 reaped with zero grace period, got %d", reaped)
	}
}

func TestDiskGroupService_ReapStale_ClearsJunctionTable(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	old := time.Now().Add(-10 * 24 * time.Hour)
	database.Create(&db.DiskGroup{MountPath: "/mnt/media", TotalBytes: 1000, UsedBytes: 500, StaleSince: &old})
	database.Create(&db.IntegrationConfig{Name: "Firefly Sonarr", Type: "sonarr", URL: "http://localhost:8989", APIKey: "key1"})
	_ = svc.SyncIntegrationLinks(1, []uint{1})

	_, err := svc.ReapStale(7)
	if err != nil {
		t.Fatalf("ReapStale error: %v", err)
	}

	var count int64
	database.Model(&db.DiskGroupIntegration{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 junction rows after reap, got %d", count)
	}
}

func TestDiskGroupService_ListStale_ReturnsOnlyStale(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	earlier := time.Now().Add(-24 * time.Hour)
	database.Create(&db.DiskGroup{MountPath: "/mnt/active", TotalBytes: 100, UsedBytes: 50})
	database.Create(&db.DiskGroup{MountPath: "/mnt/stale", TotalBytes: 200, UsedBytes: 100, StaleSince: &earlier})

	stale, err := svc.ListStale()
	if err != nil {
		t.Fatalf("ListStale error: %v", err)
	}
	if len(stale) != 1 {
		t.Fatalf("expected 1 stale group, got %d", len(stale))
	}
	if stale[0].MountPath != "/mnt/stale" {
		t.Errorf("expected stale mount '/mnt/stale', got %q", stale[0].MountPath)
	}
}

func TestDiskGroupService_ImportUpsert_ResurrectsStale(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	earlier := time.Now().Add(-48 * time.Hour)
	database.Create(&db.DiskGroup{
		MountPath:    "/mnt/media",
		TotalBytes:   1000,
		UsedBytes:    500,
		ThresholdPct: 85,
		TargetPct:    75,
		StaleSince:   &earlier,
	})

	// Import should resurrect the stale group and update thresholds
	err := svc.ImportUpsert("/mnt/media", 92, 82, nil)
	if err != nil {
		t.Fatalf("ImportUpsert error: %v", err)
	}

	group, _ := svc.GetByID(1)
	if group.StaleSince != nil {
		t.Error("expected stale_since to be nil after import resurrection")
	}
	if group.ThresholdPct != 92 {
		t.Errorf("expected threshold 92, got %f", group.ThresholdPct)
	}
}

// mockSettingsReaderForDiskGroup implements SettingsReader for disk group tests.
type mockSettingsReaderForDiskGroup struct {
	mode string
}

func (m *mockSettingsReaderForDiskGroup) GetPreferences() (db.PreferenceSet, error) {
	return db.PreferenceSet{DefaultDiskGroupMode: m.mode}, nil
}

func (m *mockSettingsReaderForDiskGroup) GetWeightMap() (map[string]int, error) {
	return map[string]int{}, nil
}

// createDiskGroupWithMode creates a disk group and sets its mode via a separate
// UPDATE statement. This is necessary because GORM's Create() does not reliably
// include fields that have a `default` tag — the mode column has
// `gorm:"not null;default:'dry-run'"` which causes GORM to omit the mode value
// from INSERT even when explicitly set on the struct.
func createDiskGroupWithMode(t *testing.T, database *gorm.DB, mountPath string, mode string) db.DiskGroup {
	t.Helper()
	group := db.DiskGroup{MountPath: mountPath, TotalBytes: 1000, UsedBytes: 500, ThresholdPct: 80, TargetPct: 70}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("createDiskGroupWithMode: Create failed: %v", err)
	}
	if mode != "" && mode != db.ModeDryRun {
		if err := database.Model(&group).Update("mode", mode).Error; err != nil {
			t.Fatalf("createDiskGroupWithMode: Update mode failed: %v", err)
		}
	}
	return group
}

// mockDeletionQueueGroupClearer implements DeletionQueueGroupClearer for tests.
type mockDeletionQueueGroupClearer struct {
	clearedGroupIDs []uint
}

func (m *mockDeletionQueueGroupClearer) ClearQueueForDiskGroup(diskGroupID uint) int {
	m.clearedGroupIDs = append(m.clearedGroupIDs, diskGroupID)
	return 1
}

func TestDiskGroupService_UpdateThresholds_ClearsDeletionQueueOnModeChange(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)
	clearer := &mockDeletionQueueGroupClearer{}
	svc.SetDeletionClearer(clearer)
	svc.SetEngineService(&mockEngineRunTrigger{})

	group := createDiskGroupWithMode(t, database, "/mnt/media", db.ModeAuto)

	// Change mode from auto → dry-run
	_, err := svc.UpdateThresholds(group.ID, 80, 70, nil, db.ModeDryRun, nil)
	if err != nil {
		t.Fatalf("UpdateThresholds error: %v", err)
	}

	if len(clearer.clearedGroupIDs) != 1 {
		t.Fatalf("expected 1 ClearQueueForDiskGroup call, got %d", len(clearer.clearedGroupIDs))
	}
	if clearer.clearedGroupIDs[0] != group.ID {
		t.Errorf("expected cleared group ID %d, got %d", group.ID, clearer.clearedGroupIDs[0])
	}
}

func TestDiskGroupService_UpdateThresholds_ClearsDeletionQueueOnModeAdvance(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)
	clearer := &mockDeletionQueueGroupClearer{}
	svc.SetDeletionClearer(clearer)
	svc.SetEngineService(&mockEngineRunTrigger{})

	// Create a disk group (default mode is dry-run from column default)
	database.Create(&db.DiskGroup{MountPath: "/mnt/media", TotalBytes: 1000, UsedBytes: 500, ThresholdPct: 80, TargetPct: 70})

	// Change mode from dry-run → auto (advance)
	_, err := svc.UpdateThresholds(1, 80, 70, nil, db.ModeAuto, nil)
	if err != nil {
		t.Fatalf("UpdateThresholds error: %v", err)
	}

	// Queue should also be cleared on mode advance (any mode change clears)
	if len(clearer.clearedGroupIDs) != 1 {
		t.Fatalf("expected 1 ClearQueueForDiskGroup call on mode advance, got %d", len(clearer.clearedGroupIDs))
	}
}

func TestDiskGroupService_UpdateThresholds_NoClearWhenModeUnchanged(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)
	clearer := &mockDeletionQueueGroupClearer{}
	svc.SetDeletionClearer(clearer)
	svc.SetEngineService(&mockEngineRunTrigger{})

	group := createDiskGroupWithMode(t, database, "/mnt/media", db.ModeAuto)

	// Update thresholds without changing mode (pass same mode)
	_, err := svc.UpdateThresholds(group.ID, 90, 75, nil, db.ModeAuto, nil)
	if err != nil {
		t.Fatalf("UpdateThresholds error: %v", err)
	}

	if len(clearer.clearedGroupIDs) != 0 {
		t.Errorf("expected no ClearQueueForDiskGroup calls when mode unchanged, got %d", len(clearer.clearedGroupIDs))
	}
}

func TestDiskGroupService_GetModeForIntegration(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)
	svc.SetEngineService(&mockEngineRunTrigger{})
	svc.SetDeletionClearer(&mockDeletionQueueGroupClearer{})

	// Seed a valid integration (FK constraint requires it)
	integrationID := seedTestIntegration(t, database)

	// Create a disk group and set its mode to approval via UpdateThresholds
	group, err := svc.Upsert(integrations.DiskSpace{Path: "/mnt/media", TotalBytes: 1000, FreeBytes: 500})
	if err != nil {
		t.Fatalf("Upsert error: %v", err)
	}
	if _, err := svc.UpdateThresholds(group.ID, 85, 75, nil, db.ModeApproval, nil); err != nil {
		t.Fatalf("UpdateThresholds error: %v", err)
	}

	// Link integration to disk group via junction table
	if err := database.Create(&db.DiskGroupIntegration{DiskGroupID: group.ID, IntegrationID: *integrationID}).Error; err != nil {
		t.Fatalf("Failed to create junction row: %v", err)
	}

	mode, err := svc.GetModeForIntegration(*integrationID)
	if err != nil {
		t.Fatalf("GetModeForIntegration error: %v", err)
	}
	if mode != db.ModeApproval {
		t.Errorf("expected mode %q, got %q", db.ModeApproval, mode)
	}
}

func TestDiskGroupService_GetModeForIntegration_NoLink(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDiskGroupService(database, bus)

	// No disk group link for integration 99
	mode, err := svc.GetModeForIntegration(99)
	if err != nil {
		t.Fatalf("GetModeForIntegration error: %v", err)
	}
	if mode != "" {
		t.Errorf("expected empty mode for unlinked integration, got %q", mode)
	}
}
