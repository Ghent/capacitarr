package services

import (
	"testing"
	"time"

	"capacitarr/internal/db"
)

func TestDataService_Reset(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDataService(database, bus)

	// Seed data across multiple tables
	intID := seedIntegration(t, database)

	// Audit log entries
	database.Create(&db.AuditLogEntry{
		MediaName: "Serenity", MediaType: "movie",
		Action: db.ActionDeleted, SizeBytes: 1000,
	})

	// Approval queue items
	database.Create(&db.ApprovalQueueItem{
		MediaName: "Firefly", MediaType: "show",
		SizeBytes: 2000, IntegrationID: intID, ExternalID: "1",
		Status: db.StatusPending,
	})

	// Library histories
	database.Create(&db.LibraryHistory{
		Timestamp: time.Now(), TotalCapacity: 100000, UsedCapacity: 80000, Resolution: "raw",
	})

	// Engine run stats
	database.Create(&db.EngineRunStats{
		RunAt: time.Now(), Evaluated: 10, Candidates: 3, DurationMs: 100,
	})

	// Disk group (should have transient fields reset)
	database.Create(&db.DiskGroup{
		MountPath: "/mnt/test", TotalBytes: 1000000, UsedBytes: 500000,
		ThresholdPct: 85, TargetPct: 75,
	})

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	summary, err := svc.Reset()
	if err != nil {
		t.Fatalf("Reset returned error: %v", err)
	}

	// Verify tables were cleared
	if summary["auditLog"] != 1 {
		t.Errorf("expected auditLog=1, got %d", summary["auditLog"])
	}
	if summary["approvalQueue"] != 1 {
		t.Errorf("expected approvalQueue=1, got %d", summary["approvalQueue"])
	}
	if summary["libraryHistories"] != 1 {
		t.Errorf("expected libraryHistories=1, got %d", summary["libraryHistories"])
	}
	if summary["engineRunStats"] != 1 {
		t.Errorf("expected engineRunStats=1, got %d", summary["engineRunStats"])
	}

	// Verify disk group transient fields reset
	var dg db.DiskGroup
	database.Where("mount_path = ?", "/mnt/test").First(&dg)
	if dg.TotalBytes != 0 {
		t.Errorf("expected disk group total_bytes=0, got %d", dg.TotalBytes)
	}
	if dg.UsedBytes != 0 {
		t.Errorf("expected disk group used_bytes=0, got %d", dg.UsedBytes)
	}
	// Thresholds should be preserved
	if dg.ThresholdPct != 85 {
		t.Errorf("expected threshold_pct=85, got %f", dg.ThresholdPct)
	}

	// Verify event
	select {
	case evt := <-ch:
		if evt.EventType() != "data_reset" {
			t.Errorf("expected event type 'data_reset', got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for data_reset event")
	}
}

func TestDataService_Reset_EmptyDB(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDataService(database, bus)

	// Reset with no data should succeed without error
	summary, err := svc.Reset()
	if err != nil {
		t.Fatalf("Reset on empty DB returned error: %v", err)
	}

	if summary["auditLog"] != 0 {
		t.Errorf("expected auditLog=0 on empty DB, got %d", summary["auditLog"])
	}
}

func TestDataService_Reset_RollsBackOnMidwayFailure(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewDataService(database, bus)

	intID := seedIntegration(t, database)
	if err := database.Create(&db.AuditLogEntry{
		MediaName: "Serenity", MediaType: "movie",
		Action: db.ActionDeleted, SizeBytes: 1000,
	}).Error; err != nil {
		t.Fatalf("failed to seed audit log: %v", err)
	}
	if err := database.Create(&db.ApprovalQueueItem{
		MediaName: "Firefly", MediaType: "show",
		SizeBytes: 2000, IntegrationID: intID, ExternalID: "1",
		Status: db.StatusPending,
	}).Error; err != nil {
		t.Fatalf("failed to seed approval queue: %v", err)
	}
	if err := database.Model(&db.LifetimeStats{}).Where("id = ?", 1).Updates(map[string]any{
		"total_bytes_reclaimed": 42,
		"total_items_removed":   3,
		"total_engine_runs":     7,
	}).Error; err != nil {
		t.Fatalf("failed to seed lifetime stats: %v", err)
	}

	var prefBefore db.PreferenceSet
	if err := database.First(&prefBefore, 1).Error; err != nil {
		t.Fatalf("failed to load preferences: %v", err)
	}

	if err := database.Exec(`
		CREATE TRIGGER fail_reset_after_audit
		BEFORE DELETE ON approval_queue
		BEGIN
			SELECT RAISE(ABORT, 'injected reset failure');
		END;
	`).Error; err != nil {
		t.Fatalf("failed to install reset failure trigger: %v", err)
	}

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	if _, err := svc.Reset(); err == nil {
		t.Fatal("expected Reset to fail after step 1")
	}

	var auditCount int64
	database.Model(&db.AuditLogEntry{}).Count(&auditCount)
	if auditCount != 1 {
		t.Errorf("expected audit log to be unchanged, got count %d", auditCount)
	}

	var approvalCount int64
	database.Model(&db.ApprovalQueueItem{}).Count(&approvalCount)
	if approvalCount != 1 {
		t.Errorf("expected approval queue to be unchanged, got count %d", approvalCount)
	}

	var lifetime db.LifetimeStats
	if err := database.First(&lifetime, 1).Error; err != nil {
		t.Fatalf("failed to reload lifetime stats: %v", err)
	}
	if lifetime.TotalBytesReclaimed != 42 || lifetime.TotalItemsRemoved != 3 || lifetime.TotalEngineRuns != 7 {
		t.Errorf("lifetime stats changed: %+v", lifetime)
	}

	var prefAfter db.PreferenceSet
	if err := database.First(&prefAfter, 1).Error; err != nil {
		t.Fatalf("failed to reload preferences: %v", err)
	}
	if prefAfter.LogLevel != prefBefore.LogLevel || prefAfter.DefaultDiskGroupMode != prefBefore.DefaultDiskGroupMode {
		t.Errorf("preferences changed: %+v", prefAfter)
	}

	select {
	case evt := <-ch:
		t.Errorf("expected no data_reset event after rollback, got %q", evt.EventType())
	default:
	}
}
