package services

import (
	"sync"
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/events"

	"gorm.io/gorm"
)

// setupSunsetTest creates a test DB, event bus, sunset service, and a seed disk group.
func setupSunsetTest(t *testing.T) (*gorm.DB, *events.EventBus, *SunsetService) {
	t.Helper()
	database := setupTestDB(t)
	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })
	svc := NewSunsetService(database, bus)
	// Seed FK targets for sunset_queue items
	database.Create(&db.DiskGroup{MountPath: "/data", TotalBytes: 100000, ThresholdPct: 85, TargetPct: 75, Mode: db.ModeSunset})
	database.Create(&db.IntegrationConfig{Type: "sonarr", Name: "Test Sonarr", URL: "http://localhost:8989", APIKey: "test"})
	return database, bus, svc
}

func sunsetDeps(database *gorm.DB, bus *events.EventBus) SunsetDeps {
	return SunsetDeps{Settings: NewSettingsService(database, bus)}
}

func TestQueueSunset(t *testing.T) {
	database, bus, svc := setupSunsetTest(t)

	item := db.SunsetQueueItem{
		MediaName: "Firefly", MediaType: "show", IntegrationID: 1, ExternalID: "1",
		SizeBytes: 5000000000, Score: 0.85, DiskGroupID: 1, Trigger: db.TriggerEngine,
		DeletionDate: time.Now().UTC().AddDate(0, 0, 30),
	}

	if err := svc.QueueSunset(item, sunsetDeps(database, bus)); err != nil {
		t.Fatalf("QueueSunset returned error: %v", err)
	}

	items, err := svc.ListAll()
	if err != nil {
		t.Fatalf("ListAll returned error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}
	if items[0].MediaName != "Firefly" {
		t.Errorf("Expected media name 'Firefly', got %q", items[0].MediaName)
	}
}

func TestBulkQueueSunset(t *testing.T) {
	database, bus, svc := setupSunsetTest(t)

	items := []db.SunsetQueueItem{
		{MediaName: "Firefly", MediaType: "show", IntegrationID: 1, SizeBytes: 5000000000, Score: 0.85, DiskGroupID: 1, Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, 30)},
		{MediaName: "Serenity", MediaType: "movie", IntegrationID: 1, SizeBytes: 3000000000, Score: 0.70, DiskGroupID: 1, Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, 30)},
	}

	created, err := svc.BulkQueueSunset(items, sunsetDeps(database, bus))
	if err != nil {
		t.Fatalf("BulkQueueSunset returned error: %v", err)
	}
	if created != 2 {
		t.Errorf("Expected 2 created, got %d", created)
	}

	all, _ := svc.ListAll()
	if len(all) != 2 {
		t.Errorf("Expected 2 items in queue, got %d", len(all))
	}
}

func TestCancel(t *testing.T) {
	database, bus, svc := setupSunsetTest(t)

	item := db.SunsetQueueItem{
		MediaName: "Firefly", MediaType: "show", IntegrationID: 1, SizeBytes: 5000000000,
		DiskGroupID: 1, Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, 30),
	}
	if err := svc.QueueSunset(item, sunsetDeps(database, bus)); err != nil {
		t.Fatalf("QueueSunset setup failed: %v", err)
	}

	items, _ := svc.ListAll()
	if len(items) != 1 {
		t.Fatalf("Expected 1 item before cancel, got %d", len(items))
	}

	if err := svc.Cancel(items[0].ID, sunsetDeps(database, bus)); err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}

	remaining, _ := svc.ListAll()
	if len(remaining) != 0 {
		t.Errorf("Expected 0 items after cancel, got %d", len(remaining))
	}
}

func TestReschedule(t *testing.T) {
	database, bus, svc := setupSunsetTest(t)

	item := db.SunsetQueueItem{
		MediaName: "Firefly", MediaType: "show", IntegrationID: 1, SizeBytes: 5000000000,
		DiskGroupID: 1, Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, 30),
	}
	if err := svc.QueueSunset(item, sunsetDeps(database, bus)); err != nil {
		t.Fatalf("QueueSunset setup failed: %v", err)
	}

	items, _ := svc.ListAll()
	if len(items) == 0 {
		t.Fatal("Expected at least 1 item for rescheduling")
	}

	newDate := time.Now().UTC().AddDate(0, 0, 60)
	updated, err := svc.Reschedule(items[0].ID, newDate)
	if err != nil {
		t.Fatalf("Reschedule returned error: %v", err)
	}
	if updated.DeletionDate.Format("2006-01-02") != newDate.Format("2006-01-02") {
		t.Errorf("Expected deletion date %s, got %s", newDate.Format("2006-01-02"), updated.DeletionDate.Format("2006-01-02"))
	}
}

func TestProcessExpired_WithoutDeletion(t *testing.T) {
	database, bus, svc := setupSunsetTest(t)

	// Create an already-expired item
	database.Create(&db.SunsetQueueItem{
		MediaName: "Firefly", MediaType: "show", IntegrationID: 1, SizeBytes: 5000000000,
		DiskGroupID: 1, Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, -1),
	})

	// Create a future item that should NOT be processed
	database.Create(&db.SunsetQueueItem{
		MediaName: "Serenity", MediaType: "movie", IntegrationID: 1, SizeBytes: 3000000000,
		DiskGroupID: 1, Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, 30),
	})

	// Without a DeletionService or Registry, expired items should NOT be processed
	// (they remain in the queue for retry when deletion becomes available)
	processed, err := svc.ProcessExpired(sunsetDeps(database, bus))
	if err != nil {
		t.Fatalf("ProcessExpired returned error: %v", err)
	}
	if processed != 0 {
		t.Errorf("Expected 0 processed (no deletion service), got %d", processed)
	}

	// Both items should still be in the queue (expired + future)
	remaining, _ := svc.ListAll()
	if len(remaining) != 2 {
		t.Errorf("Expected 2 remaining items (no deletions without service), got %d", len(remaining))
	}
}

func TestProcessExpired_WithDeletion(t *testing.T) {
	database, bus, svc := setupSunsetTest(t)
	auditSvc := NewAuditLogService(database)
	deletionSvc := NewDeletionService(bus, auditSvc)

	// Create an already-expired item
	database.Create(&db.SunsetQueueItem{
		MediaName: "Firefly", MediaType: "show", IntegrationID: 1, SizeBytes: 5000000000,
		DiskGroupID: 1, Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, -1),
	})

	// Create a future item
	database.Create(&db.SunsetQueueItem{
		MediaName: "Serenity", MediaType: "movie", IntegrationID: 1, SizeBytes: 3000000000,
		DiskGroupID: 1, Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, 30),
	})

	// With DeletionService but no Registry, items still can't be processed
	// (Registry is needed to look up the integration's deleter client)
	deps := SunsetDeps{
		Settings: NewSettingsService(database, bus),
		Deletion: deletionSvc,
	}
	processed, err := svc.ProcessExpired(deps)
	if err != nil {
		t.Fatalf("ProcessExpired returned error: %v", err)
	}
	if processed != 0 {
		t.Errorf("Expected 0 processed (no registry), got %d", processed)
	}

	// Future item should still be in queue
	remaining, _ := svc.ListAll()
	if len(remaining) != 2 {
		t.Errorf("Expected 2 remaining items, got %d", len(remaining))
	}
}

func TestDaysRemaining(t *testing.T) {
	_, _, svc := setupSunsetTest(t)

	tests := []struct {
		name    string
		date    time.Time
		wantMin int
		wantMax int
	}{
		{"30 days future", time.Now().UTC().AddDate(0, 0, 30), 28, 30},
		{"1 day future", time.Now().UTC().AddDate(0, 0, 1), 0, 1},
		{"past date", time.Now().UTC().AddDate(0, 0, -1), 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := db.SunsetQueueItem{DeletionDate: tt.date}
			got := svc.DaysRemaining(item)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("DaysRemaining() = %d, want %d-%d", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestListSunsettedKeys(t *testing.T) {
	database, _, svc := setupSunsetTest(t)

	database.Create(&db.SunsetQueueItem{
		MediaName: "Firefly", MediaType: "show", IntegrationID: 1, SizeBytes: 5000000000,
		DiskGroupID: 1, Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, 30),
	})
	database.Create(&db.SunsetQueueItem{
		MediaName: "Serenity", MediaType: "movie", IntegrationID: 1, SizeBytes: 3000000000,
		DiskGroupID: 1, Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, 30),
	})

	keys, err := svc.ListSunsettedKeys(1)
	if err != nil {
		t.Fatalf("ListSunsettedKeys returned error: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("Expected 2 keys, got %d", len(keys))
	}
	if !keys[db.MediaKey("Firefly", "show")] {
		t.Error("Expected Firefly/show key to be present")
	}
	if !keys[db.MediaKey("Serenity", "movie")] {
		t.Error("Expected Serenity/movie key to be present")
	}
	if keys["Firefly|show"] {
		t.Error("pipe-delimited key must not be used; ListSunsettedKeys should use db.MediaKey")
	}
}

func TestIsSunsetted(t *testing.T) {
	database, _, svc := setupSunsetTest(t)

	database.Create(&db.SunsetQueueItem{
		MediaName: "Firefly", MediaType: "show", IntegrationID: 1, SizeBytes: 5000000000,
		DiskGroupID: 1, Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, 30),
	})

	if !svc.IsSunsetted("Firefly", "show", 1) {
		t.Error("Expected IsSunsetted=true for queued item")
	}
	if svc.IsSunsetted("Serenity", "movie", 1) {
		t.Error("Expected IsSunsetted=false for non-queued item")
	}
}

func TestCancelAll(t *testing.T) {
	database, bus, svc := setupSunsetTest(t)

	for i := 0; i < 5; i++ {
		database.Create(&db.SunsetQueueItem{
			MediaName: "Firefly", MediaType: "show", IntegrationID: 1, SizeBytes: 1000000,
			DiskGroupID: 1, Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, 30),
		})
	}

	count, err := svc.CancelAll(sunsetDeps(database, bus))
	if err != nil {
		t.Fatalf("CancelAll returned error: %v", err)
	}
	if count != 5 {
		t.Errorf("Expected 5 cancelled, got %d", count)
	}

	remaining, _ := svc.ListAll()
	if len(remaining) != 0 {
		t.Errorf("Expected 0 remaining, got %d", len(remaining))
	}
}

func TestEscalate_OrderAndTargetBytes(t *testing.T) {
	database, bus, svc := setupSunsetTest(t)

	// Seed 3 items with varying scores:
	// 1. Expired item, low score — should be targeted after higher-scored items
	// 2. Future item, high score — targeted first by escalation step 2
	// 3. Future item, medium score — targeted second by escalation step 2

	database.Create(&db.SunsetQueueItem{
		MediaName: "Firefly", MediaType: "show", IntegrationID: 1,
		SizeBytes: 2000000000, Score: 0.60, DiskGroupID: 1, Trigger: db.TriggerEngine,
		DeletionDate: time.Now().UTC().AddDate(0, 0, -1), // Expired
	})

	database.Create(&db.SunsetQueueItem{
		MediaName: "Serenity", MediaType: "movie", IntegrationID: 1,
		SizeBytes: 3000000000, Score: 0.95, DiskGroupID: 1, Trigger: db.TriggerEngine,
		DeletionDate: time.Now().UTC().AddDate(0, 0, 20), // Future, highest score
	})

	database.Create(&db.SunsetQueueItem{
		MediaName: "Firefly Movie", MediaType: "movie", IntegrationID: 1,
		SizeBytes: 1000000000, Score: 0.75, DiskGroupID: 1, Trigger: db.TriggerEngine,
		DeletionDate: time.Now().UTC().AddDate(0, 0, 25), // Future, medium score
	})

	// Escalate without a registry/deletion service — items can't actually be
	// processed (same pattern as TestProcessExpired_WithDeletion), but we verify:
	// 1. No panic on escalation
	// 2. All items remain (since processExpiredItem returns false without deps)
	// 3. Zero bytes freed
	freed, err := svc.Escalate(1, 5000000000, sunsetDeps(database, bus))
	if err != nil {
		t.Fatalf("Escalate returned error: %v", err)
	}
	if freed != 0 {
		t.Errorf("Expected 0 bytes freed (no registry), got %d", freed)
	}

	// All 3 items should still be in the queue (no deletions without registry)
	remaining, _ := svc.ListAll()
	if len(remaining) != 3 {
		t.Errorf("Expected 3 remaining items (no deletions without deps), got %d", len(remaining))
	}
}

func TestEscalate_PreservesQueueBelowTarget(t *testing.T) {
	database, bus, svc := setupSunsetTest(t)

	// Seed items: one expired (small), two future (larger)
	// targetBytes is set low enough that only the expired item would suffice
	database.Create(&db.SunsetQueueItem{
		MediaName: "Firefly", MediaType: "show", IntegrationID: 1,
		SizeBytes: 1000000000, DiskGroupID: 1, Trigger: db.TriggerEngine,
		DeletionDate: time.Now().UTC().AddDate(0, 0, -1), // Expired
	})
	database.Create(&db.SunsetQueueItem{
		MediaName: "Serenity", MediaType: "movie", IntegrationID: 1,
		SizeBytes: 5000000000, DiskGroupID: 1, Trigger: db.TriggerEngine,
		DeletionDate: time.Now().UTC().AddDate(0, 0, 20),
	})
	database.Create(&db.SunsetQueueItem{
		MediaName: "Firefly", MediaType: "show", IntegrationID: 1,
		SizeBytes: 4000000000, DiskGroupID: 1, Trigger: db.TriggerEngine,
		DeletionDate: time.Now().UTC().AddDate(0, 0, 25),
	})

	// Without registry, no items can be processed — all remain preserved.
	// This verifies the escalation loop exits gracefully when processExpiredItem
	// returns false, leaving the queue intact for retry on next cron run.
	freed, err := svc.Escalate(1, 1000000000, sunsetDeps(database, bus))
	if err != nil {
		t.Fatalf("Escalate returned error: %v", err)
	}
	if freed != 0 {
		t.Errorf("Expected 0 bytes freed, got %d", freed)
	}

	remaining, _ := svc.ListAll()
	if len(remaining) != 3 {
		t.Errorf("Expected 3 remaining (preserved below target), got %d", len(remaining))
	}
}

func TestValidation_SunsetPctOrdering(t *testing.T) {
	tests := []struct {
		name      string
		sunsetPct float64
		targetPct float64
		threshold float64
		wantErr   bool
	}{
		{"valid ordering", 60.0, 75.0, 85.0, false},
		{"sunsetPct equals targetPct", 75.0, 75.0, 85.0, true},
		{"sunsetPct above targetPct", 80.0, 75.0, 85.0, true},
		{"sunsetPct equals thresholdPct", 85.0, 75.0, 85.0, true},
		{"sunsetPct above thresholdPct", 90.0, 75.0, 85.0, true},
		{"tight valid ordering", 74.9, 75.0, 85.0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pct := tt.sunsetPct
			err := ValidateSunsetConfig(db.ModeSunset, &pct, tt.targetPct, tt.threshold)
			if tt.wantErr && err == nil {
				t.Errorf("Expected error for sunsetPct=%.1f targetPct=%.1f thresholdPct=%.1f, got nil",
					tt.sunsetPct, tt.targetPct, tt.threshold)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error for sunsetPct=%.1f targetPct=%.1f thresholdPct=%.1f, got: %v",
					tt.sunsetPct, tt.targetPct, tt.threshold, err)
			}
		})
	}
}

func TestValidation_NullSunsetPct(t *testing.T) {
	// Sunset mode with nil sunsetPct should be rejected
	err := ValidateSunsetConfig(db.ModeSunset, nil, 75.0, 85.0)
	if err == nil {
		t.Error("Expected error for sunset mode with nil sunsetPct, got nil")
	}

	// Non-sunset modes with nil sunsetPct should be accepted
	for _, mode := range []string{db.ModeDryRun, db.ModeApproval, db.ModeAuto} {
		err := ValidateSunsetConfig(mode, nil, 75.0, 85.0)
		if err != nil {
			t.Errorf("Expected no error for %s mode with nil sunsetPct, got: %v", mode, err)
		}
	}
}

func TestGetExpired_OrdersByScoreDescending(t *testing.T) {
	database, _, svc := setupSunsetTest(t)

	// Seed 3 expired items with different scores, inserted in ascending score
	// order. GetExpired() must return them highest-score-first.
	database.Create(&db.SunsetQueueItem{
		MediaName: "Firefly", MediaType: "show", IntegrationID: 1,
		SizeBytes: 5000000000, Score: 0.45, DiskGroupID: 1, Trigger: db.TriggerEngine,
		DeletionDate: time.Now().UTC().AddDate(0, 0, -1), // expired
	})
	database.Create(&db.SunsetQueueItem{
		MediaName: "Serenity", MediaType: "movie", IntegrationID: 1,
		SizeBytes: 3000000000, Score: 0.85, DiskGroupID: 1, Trigger: db.TriggerEngine,
		DeletionDate: time.Now().UTC().AddDate(0, 0, -2), // expired earlier
	})
	database.Create(&db.SunsetQueueItem{
		MediaName: "Firefly Movie", MediaType: "movie", IntegrationID: 1,
		SizeBytes: 2000000000, Score: 1.20, DiskGroupID: 1, Trigger: db.TriggerEngine,
		DeletionDate: time.Now().UTC().AddDate(0, 0, -3), // expired earliest
	})

	expired, err := svc.GetExpired()
	if err != nil {
		t.Fatalf("GetExpired returned error: %v", err)
	}
	if len(expired) != 3 {
		t.Fatalf("Expected 3 expired items, got %d", len(expired))
	}

	// Verify score-descending order
	if expired[0].MediaName != "Firefly Movie" || expired[0].Score != 1.20 {
		t.Errorf("Expected first item 'Firefly Movie' (score 1.20), got %q (score %.2f)",
			expired[0].MediaName, expired[0].Score)
	}
	if expired[1].MediaName != "Serenity" || expired[1].Score != 0.85 {
		t.Errorf("Expected second item 'Serenity' (score 0.85), got %q (score %.2f)",
			expired[1].MediaName, expired[1].Score)
	}
	if expired[2].MediaName != "Firefly" || expired[2].Score != 0.45 {
		t.Errorf("Expected third item 'Firefly' (score 0.45), got %q (score %.2f)",
			expired[2].MediaName, expired[2].Score)
	}
}

func TestCancelAllForDiskGroup(t *testing.T) {
	database, bus, svc := setupSunsetTest(t)

	// Create a second disk group
	database.Create(&db.DiskGroup{MountPath: "/data2", TotalBytes: 100000, ThresholdPct: 85, TargetPct: 75, Mode: db.ModeSunset})

	database.Create(&db.SunsetQueueItem{
		MediaName: "Firefly", MediaType: "show", IntegrationID: 1, SizeBytes: 5000000000,
		DiskGroupID: 1, Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, 30),
	})
	database.Create(&db.SunsetQueueItem{
		MediaName: "Serenity", MediaType: "movie", IntegrationID: 1, SizeBytes: 3000000000,
		DiskGroupID: 2, Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, 30),
	})

	count, err := svc.CancelAllForDiskGroup(1, sunsetDeps(database, bus))
	if err != nil {
		t.Fatalf("CancelAllForDiskGroup returned error: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 cancelled, got %d", count)
	}

	remaining, _ := svc.ListAll()
	if len(remaining) != 1 {
		t.Errorf("Expected 1 remaining, got %d", len(remaining))
	}
	if len(remaining) > 0 && remaining[0].DiskGroupID != 2 {
		t.Errorf("Expected remaining item in disk group 2, got %d", remaining[0].DiskGroupID)
	}
}

func TestProcessExpired_ConcurrentHandoffQueuesOnce(t *testing.T) {
	database, bus, svc := setupSunsetTest(t)
	deletionSvc := NewDeletionService(bus, NewAuditLogService(database))
	deletionSvc.SetDependencies(DeletionDeps{
		Settings:      &mockSettingsReader{deletionsEnabled: true, executionMode: db.ModeSunset},
		Engine:        &mockEngineStatsWriter{},
		Metrics:       &mockDeletionStatsWriter{},
		Approval:      &mockApprovalReturner{},
		Snoozer:       &mockApprovalSnoozer{},
		DiskGroups:    &mockDiskGroupModeReader{mode: db.ModeSunset},
		Clients:       &mockClientResolver{deleter: &mockIntegration{}},
		SunsetCleaner: &mockSunsetQueueCleaner{},
	})

	database.Create(&db.SunsetQueueItem{
		MediaName: "Firefly", MediaType: "show", IntegrationID: 1, SizeBytes: 5000000000,
		DiskGroupID: 1, Trigger: db.TriggerEngine, DeletionDate: time.Now().UTC().AddDate(0, 0, -1),
	})

	deps := SunsetDeps{
		Settings: NewSettingsService(database, bus),
		Deletion: deletionSvc,
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if _, err := svc.ProcessExpired(deps); err != nil {
			t.Errorf("ProcessExpired: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		if _, err := svc.Escalate(1, 1e15, deps); err != nil {
			t.Errorf("Escalate: %v", err)
		}
	}()
	wg.Wait()

	if deletionSvc.QueueLen() != 1 {
		t.Errorf("expected 1 queued deletion job, got %d", deletionSvc.QueueLen())
	}

	var items []db.SunsetQueueItem
	database.Find(&items)
	expired := 0
	for _, item := range items {
		if item.ExpiredAt != nil {
			expired++
		}
	}
	if expired != 1 {
		t.Errorf("expected 1 claimed sunset row, got %d", expired)
	}
}
