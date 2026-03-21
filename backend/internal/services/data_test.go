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
		MediaName: "Movie A", MediaType: "movie",
		Action: db.ActionDeleted, SizeBytes: 1000,
	})

	// Approval queue items
	database.Create(&db.ApprovalQueueItem{
		MediaName: "Show B", MediaType: "show",
		SizeBytes: 2000, IntegrationID: intID, ExternalID: "1",
		Status: db.StatusPending,
	})

	// Library histories
	database.Create(&db.LibraryHistory{
		Timestamp: time.Now(), TotalCapacity: 100000, UsedCapacity: 80000, Resolution: "raw",
	})

	// Engine run stats
	database.Create(&db.EngineRunStats{
		RunAt: time.Now(), Evaluated: 10, Flagged: 3, ExecutionMode: "dry-run", DurationMs: 100,
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
