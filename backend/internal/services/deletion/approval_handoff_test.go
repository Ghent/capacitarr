package deletion

import (
	"errors"
	"fmt"
	"testing"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
	"capacitarr/internal/services/approval"
)

// TestApprovalService_ExecuteApproval_UsesPerDiskGroupMode verifies that
// ExecuteApproval resolves the per-disk-group mode (not the global
// DefaultDiskGroupMode) when determining ForceDryRun. This is a regression
// test for a bug where items in an "approval"-mode disk group were dry-deleted
// and returned to pending because the code checked prefs.DefaultDiskGroupMode
// (which defaults to "dry-run") instead of the actual disk group's mode.
func TestApprovalService_ExecuteApproval_UsesPerDiskGroupMode(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)

	// Register integration factories so CreateClient works
	integrations.RegisterAllFactories()

	// Create services
	approvalSvc := approval.NewApprovalService(database, bus)
	auditLogSvc := NewAuditLogService(database)
	deletionSvc := NewDeletionService(bus, auditLogSvc)

	// Wire deletion service dependencies (don't Start() — we just inspect the queue)
	// The DiskGroups mock returns approval mode, and the Clients mock provides a deleter.
	deletionSvc.SetDependencies(DeletionDeps{
		Settings:      &mockSettingsReader{deletionsEnabled: true, executionMode: db.ModeDryRun, deletionQueueDelaySeconds: 300},
		Engine:        &mockEngineStatsWriter{},
		Metrics:       &mockDeletionStatsWriter{},
		Approval:      &mockApprovalReturner{},
		Snoozer:       &mockApprovalSnoozer{},
		DiskGroups:    &mockDiskGroupModeReader{mode: db.ModeApproval},
		Clients:       &mockClientResolver{deleter: &mockIntegration{}},
		SunsetCleaner: &mockSunsetQueueCleaner{},
	})

	// Seed integration
	ic := db.IntegrationConfig{
		Type: "sonarr", Name: "Test Sonarr", URL: "http://localhost:8989", APIKey: "key",
	}
	database.Create(&ic)

	// Create a disk group in "approval" mode
	dg := db.DiskGroup{
		MountPath:    "/media",
		TotalBytes:   1000000000,
		UsedBytes:    900000000,
		ThresholdPct: 85,
		TargetPct:    75,
		Mode:         db.ModeApproval, // disk group is in approval mode
	}
	database.Create(&dg)

	// Create a pending approval queue item linked to the approval-mode disk group
	dgID := dg.ID
	item := db.ApprovalQueueItem{
		MediaName:       "Firefly",
		MediaType:       "show",
		SizeBytes:       5000000000,
		Score:           0.85,
		IntegrationID:   ic.ID,
		ExternalID:      "42",
		DiskGroupID:     &dgID,
		CollectionGroup: "Firefly Collection",
		Status:          db.StatusPending,
	}
	database.Create(&item)

	// Enable preferences with DeletionsEnabled=true but DefaultDiskGroupMode="dry-run"
	database.Model(&db.PreferenceSet{}).Where("id = 1").Updates(map[string]any{
		"deletions_enabled":       true,
		"default_disk_group_mode": db.ModeDryRun, // global default is dry-run
	})

	// Execute the approval
	_, err := approvalSvc.ExecuteApproval(item.ID, approval.ExecuteApprovalDeps{
		Deletion: deletionSvc,
	})
	if err != nil {
		t.Fatalf("ExecuteApproval failed: %v", err)
	}

	// Inspect the queued deletion job — ForceDryRun should be false because
	// the item's disk group is in "approval" mode (not "dry-run")
	deletionSvc.queuedMu.Lock()
	defer deletionSvc.queuedMu.Unlock()

	if len(deletionSvc.queuedItems) != 1 {
		t.Fatalf("Expected 1 queued item, got %d", len(deletionSvc.queuedItems))
	}

	job := deletionSvc.queuedItems[0]
	if job.ForceDryRun {
		t.Errorf("ForceDryRun should be false (disk group is in approval mode), but got true — " +
			"this means the code is checking DefaultDiskGroupMode instead of the actual disk group mode")
	}
	if job.EnqueuedMode != db.ModeApproval {
		t.Errorf("Expected EnqueuedMode=%q, got %q", db.ModeApproval, job.EnqueuedMode)
	}
	// DiskGroupID must be propagated so processJob's resolveCurrentMode checks the
	// correct disk group — without it, the fallback to DefaultDiskGroupMode causes
	// false cancellation when the default differs from the item's disk group mode.
	if job.DiskGroupID == nil || *job.DiskGroupID != dg.ID {
		t.Errorf("Expected DiskGroupID=%d, got %v — processJob will use wrong fallback mode", dg.ID, job.DiskGroupID)
	}
	if job.CollectionGroup != "Firefly Collection" {
		t.Errorf("Expected CollectionGroup=%q, got %q", "Firefly Collection", job.CollectionGroup)
	}
}

// TestApprovalService_ExecuteApproval_FallsBackToDefaultMode verifies that
// ExecuteApproval falls back to DefaultDiskGroupMode when the approval item
// has no DiskGroupID (e.g. user-initiated items).
func TestApprovalService_ExecuteApproval_FallsBackToDefaultMode(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)

	integrations.RegisterAllFactories()

	approvalSvc := approval.NewApprovalService(database, bus)
	auditLogSvc := NewAuditLogService(database)
	deletionSvc := NewDeletionService(bus, auditLogSvc)

	// Global default is dry-run, deletions enabled. DiskGroups mock returns error
	// (no group found) so QueueFromApproval falls back to DefaultDiskGroupMode.
	deletionSvc.SetDependencies(DeletionDeps{
		Settings:      &mockSettingsReader{deletionsEnabled: true, executionMode: db.ModeDryRun, deletionQueueDelaySeconds: 300},
		Engine:        &mockEngineStatsWriter{},
		Metrics:       &mockDeletionStatsWriter{},
		Approval:      &mockApprovalReturner{},
		Snoozer:       &mockApprovalSnoozer{},
		DiskGroups:    &mockDiskGroupModeReader{},
		Clients:       &mockClientResolver{deleter: &mockIntegration{}},
		SunsetCleaner: &mockSunsetQueueCleaner{},
	})

	// Seed integration
	ic := db.IntegrationConfig{
		Type: "sonarr", Name: "Test Sonarr", URL: "http://localhost:8989", APIKey: "key",
	}
	database.Create(&ic)

	// Create a pending approval queue item WITHOUT a DiskGroupID
	item := db.ApprovalQueueItem{
		MediaName:     "Serenity",
		MediaType:     "movie",
		SizeBytes:     3000000000,
		Score:         0.70,
		IntegrationID: ic.ID,
		ExternalID:    "99",
		DiskGroupID:   nil, // no disk group — user-initiated
		Status:        db.StatusPending,
	}
	database.Create(&item)

	// Execute the approval — should fall back to DefaultDiskGroupMode ("dry-run")
	_, err := approvalSvc.ExecuteApproval(item.ID, approval.ExecuteApprovalDeps{
		Deletion: deletionSvc,
	})
	if err != nil {
		t.Fatalf("ExecuteApproval failed: %v", err)
	}

	// ForceDryRun should be true because the fallback uses DefaultDiskGroupMode="dry-run"
	deletionSvc.queuedMu.Lock()
	defer deletionSvc.queuedMu.Unlock()

	if len(deletionSvc.queuedItems) != 1 {
		t.Fatalf("Expected 1 queued item, got %d", len(deletionSvc.queuedItems))
	}

	job := deletionSvc.queuedItems[0]
	if !job.ForceDryRun {
		t.Errorf("ForceDryRun should be true (no disk group, fallback to dry-run default), but got false")
	}
	if job.EnqueuedMode != db.ModeDryRun {
		t.Errorf("Expected EnqueuedMode=%q, got %q", db.ModeDryRun, job.EnqueuedMode)
	}
}

func TestApprovalService_ExecuteApproval_QueueFullLeavesPending(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	approvalSvc := approval.NewApprovalService(database, bus)
	deletionSvc := NewDeletionService(bus, NewAuditLogService(database))
	deletionSvc.SetDependencies(DeletionDeps{
		Settings:      &mockSettingsReader{deletionsEnabled: true, executionMode: db.ModeApproval, deletionQueueDelaySeconds: 300},
		Engine:        &mockEngineStatsWriter{},
		Metrics:       &mockDeletionStatsWriter{},
		Approval:      &mockApprovalReturner{},
		Snoozer:       &mockApprovalSnoozer{},
		DiskGroups:    &mockDiskGroupModeReader{mode: db.ModeApproval},
		Clients:       &mockClientResolver{deleter: &mockIntegration{}},
		SunsetCleaner: &mockSunsetQueueCleaner{},
	})

	ic := db.IntegrationConfig{
		Type: "sonarr", Name: "Test Sonarr", URL: "http://localhost:8989", APIKey: "key",
	}
	database.Create(&ic)

	item := db.ApprovalQueueItem{
		MediaName:     "Firefly",
		MediaType:     "show",
		SizeBytes:     1000,
		IntegrationID: ic.ID,
		ExternalID:    "42",
		Status:        db.StatusPending,
	}
	database.Create(&item)

	for i := 0; i < 500; i++ {
		if err := deletionSvc.enqueue(deleteJob{
			Item: integrations.MediaItem{Title: fmt.Sprintf("Filler-%d", i), Type: "movie"},
		}); err != nil {
			t.Fatalf("failed to fill queue: %v", err)
		}
	}

	_, err := approvalSvc.ExecuteApproval(item.ID, approval.ExecuteApprovalDeps{Deletion: deletionSvc})
	if err == nil {
		t.Fatal("expected queue-full error")
	}
	if !errors.Is(err, ErrDeletionQueueFull) {
		t.Errorf("expected ErrDeletionQueueFull, got %v", err)
	}

	var got db.ApprovalQueueItem
	if dbErr := database.First(&got, item.ID).Error; dbErr != nil {
		t.Fatalf("failed to reload approval: %v", dbErr)
	}
	if got.Status != db.StatusPending {
		t.Errorf("expected status pending after queue-full, got %s", got.Status)
	}
}
