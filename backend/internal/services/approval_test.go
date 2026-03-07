package services

import (
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/events"

	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupTestDB creates an in-memory SQLite database with migrations applied.
// Local helper to avoid importing testutil (which pulls in routes → services
// circular dependency).
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	database, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open in-memory SQLite: %v", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("Failed to get underlying sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Seed default preferences
	pref := db.PreferenceSet{ID: 1, ExecutionMode: "dry-run", LogLevel: "info", AuditLogRetentionDays: 30}
	if err := database.FirstOrCreate(&pref, db.PreferenceSet{ID: 1}).Error; err != nil {
		t.Fatalf("Failed to seed preferences: %v", err)
	}

	return database
}

// newTestBus creates a new EventBus and registers a cleanup to close it.
func newTestBus(t *testing.T) *events.EventBus {
	t.Helper()
	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })
	return bus
}

// seedIntegration creates a minimal integration config for FK references.
func seedIntegration(t *testing.T, database *gorm.DB) uint {
	t.Helper()
	ic := db.IntegrationConfig{
		Type:   "sonarr",
		Name:   "Test Sonarr",
		URL:    "http://localhost:8989",
		APIKey: "test-key",
	}
	if err := database.Create(&ic).Error; err != nil {
		t.Fatalf("Failed to seed integration: %v", err)
	}
	return ic.ID
}

// seedPendingItem creates a pending approval queue item.
func seedPendingItem(t *testing.T, database *gorm.DB, integrationID uint) db.ApprovalQueueItem {
	t.Helper()
	item := db.ApprovalQueueItem{
		MediaName:     "Firefly",
		MediaType:     "show",
		Reason:        "Score: 0.85",
		SizeBytes:     5069636198,
		IntegrationID: integrationID,
		ExternalID:    "1",
		Status:        db.StatusPending,
	}
	if err := database.Create(&item).Error; err != nil {
		t.Fatalf("Failed to seed approval queue item: %v", err)
	}
	return item
}

func TestApprovalService_Approve(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)
	item := seedPendingItem(t, database, intID)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	result, err := svc.Approve(item.ID)
	if err != nil {
		t.Fatalf("Approve returned error: %v", err)
	}

	if result.Status != db.StatusApproved {
		t.Errorf("expected status %q, got %q", db.StatusApproved, result.Status)
	}

	// Verify event was published
	select {
	case evt := <-ch:
		if evt.EventType() != "approval_approved" {
			t.Errorf("expected event type 'approval_approved', got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestApprovalService_Approve_NotPending(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)
	item := seedPendingItem(t, database, intID)

	// Approve once
	if _, err := svc.Approve(item.ID); err != nil {
		t.Fatalf("First approve failed: %v", err)
	}

	// Approve again should fail
	_, err := svc.Approve(item.ID)
	if err == nil {
		t.Fatal("expected error when approving non-pending item")
	}
}

func TestApprovalService_Approve_NotFound(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	_, err := svc.Approve(99999)
	if err == nil {
		t.Fatal("expected error for non-existent entry")
	}
}

func TestApprovalService_Reject(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)
	item := seedPendingItem(t, database, intID)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	result, err := svc.Reject(item.ID, 24)
	if err != nil {
		t.Fatalf("Reject returned error: %v", err)
	}

	if result.Status != db.StatusRejected {
		t.Errorf("expected status %q, got %q", db.StatusRejected, result.Status)
	}
	if result.SnoozedUntil == nil {
		t.Fatal("expected SnoozedUntil to be set")
	}

	// Should be approximately 24 hours from now
	expected := time.Now().UTC().Add(24 * time.Hour)
	diff := result.SnoozedUntil.Sub(expected)
	if diff < -5*time.Second || diff > 5*time.Second {
		t.Errorf("SnoozedUntil is not ~24h from now: %v (diff: %v)", result.SnoozedUntil, diff)
	}

	// Verify event was published
	select {
	case evt := <-ch:
		if evt.EventType() != "approval_rejected" {
			t.Errorf("expected event type 'approval_rejected', got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestApprovalService_Reject_NotPending(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)
	item := seedPendingItem(t, database, intID)

	// Reject once
	if _, err := svc.Reject(item.ID, 24); err != nil {
		t.Fatalf("First reject failed: %v", err)
	}

	// Reject again should fail (status is now rejected)
	_, err := svc.Reject(item.ID, 24)
	if err == nil {
		t.Fatal("expected error when rejecting non-pending item")
	}
}

func TestApprovalService_Unsnooze(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)
	item := seedPendingItem(t, database, intID)

	// Reject first (reject → sets snoozed_until)
	if _, err := svc.Reject(item.ID, 24); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	result, err := svc.Unsnooze(item.ID)
	if err != nil {
		t.Fatalf("Unsnooze returned error: %v", err)
	}

	if result.Status != db.StatusPending {
		t.Errorf("expected status %q, got %q", db.StatusPending, result.Status)
	}
	if result.SnoozedUntil != nil {
		t.Error("expected SnoozedUntil to be cleared")
	}

	// Verify event was published
	select {
	case evt := <-ch:
		if evt.EventType() != "approval_unsnoozed" {
			t.Errorf("expected event type 'approval_unsnoozed', got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestApprovalService_Unsnooze_NotRejected(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)
	item := seedPendingItem(t, database, intID)

	// Try to unsnooze a pending item (should fail)
	_, err := svc.Unsnooze(item.ID)
	if err == nil {
		t.Fatal("expected error when unsnoozing non-rejected item")
	}
}

func TestApprovalService_UpsertPending_Create(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)

	created, err := svc.UpsertPending(db.ApprovalQueueItem{
		MediaName:     "New Movie",
		MediaType:     "movie",
		Reason:        "Score: 0.90",
		SizeBytes:     1000000,
		IntegrationID: intID,
		ExternalID:    "42",
	})
	if err != nil {
		t.Fatalf("UpsertPending returned error: %v", err)
	}
	if !created {
		t.Error("expected created=true for new item")
	}

	// Verify in DB
	var count int64
	database.Model(&db.ApprovalQueueItem{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 item in queue, got %d", count)
	}
}

func TestApprovalService_UpsertPending_Update(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)

	// Create first
	_, err := svc.UpsertPending(db.ApprovalQueueItem{
		MediaName:     "Existing Movie",
		MediaType:     "movie",
		Reason:        "Score: 0.80",
		SizeBytes:     1000000,
		IntegrationID: intID,
		ExternalID:    "42",
	})
	if err != nil {
		t.Fatalf("First UpsertPending failed: %v", err)
	}

	// Upsert again with updated reason
	created, err := svc.UpsertPending(db.ApprovalQueueItem{
		MediaName:     "Existing Movie",
		MediaType:     "movie",
		Reason:        "Score: 0.95",
		SizeBytes:     2000000,
		IntegrationID: intID,
		ExternalID:    "42",
	})
	if err != nil {
		t.Fatalf("Second UpsertPending failed: %v", err)
	}
	if created {
		t.Error("expected created=false for upsert of existing item")
	}

	// Verify only 1 item in queue
	var count int64
	database.Model(&db.ApprovalQueueItem{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 item in queue after upsert, got %d", count)
	}

	// Verify updated values
	var item db.ApprovalQueueItem
	database.First(&item)
	if item.Reason != "Score: 0.95" {
		t.Errorf("expected updated reason, got %q", item.Reason)
	}
	if item.SizeBytes != 2000000 {
		t.Errorf("expected updated size, got %d", item.SizeBytes)
	}
}

func TestApprovalService_IsSnoozed(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)
	item := seedPendingItem(t, database, intID)

	// Not snoozed initially
	if svc.IsSnoozed("Firefly", "show") {
		t.Error("expected IsSnoozed=false for pending item")
	}

	// Reject (snooze) it
	if _, err := svc.Reject(item.ID, 24); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	// Now snoozed
	if !svc.IsSnoozed("Firefly", "show") {
		t.Error("expected IsSnoozed=true for rejected item with active snooze")
	}

	// Different media name should not be snoozed
	if svc.IsSnoozed("Other Show", "show") {
		t.Error("expected IsSnoozed=false for different media")
	}
}

func TestApprovalService_BulkUnsnooze(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)

	// Create 3 items, reject 2 of them
	for i, name := range []string{"Movie A", "Movie B", "Movie C"} {
		item := db.ApprovalQueueItem{
			MediaName: name, MediaType: "movie", Reason: "Score: 0.50",
			SizeBytes: 1000, IntegrationID: intID, ExternalID: string(rune('1' + i)),
			Status: db.StatusPending,
		}
		if err := database.Create(&item).Error; err != nil {
			t.Fatalf("Failed to create item: %v", err)
		}
	}

	// Reject first two
	var items []db.ApprovalQueueItem
	database.Find(&items)
	for i := 0; i < 2; i++ {
		if _, err := svc.Reject(items[i].ID, 24); err != nil {
			t.Fatalf("Reject failed: %v", err)
		}
	}

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	count, err := svc.BulkUnsnooze()
	if err != nil {
		t.Fatalf("BulkUnsnooze returned error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 unsnoozed, got %d", count)
	}

	// All items should be pending now
	var rejected int64
	database.Model(&db.ApprovalQueueItem{}).Where("status = ?", db.StatusRejected).Count(&rejected)
	if rejected != 0 {
		t.Errorf("expected 0 rejected items after bulk unsnooze, got %d", rejected)
	}

	// Verify event published
	select {
	case evt := <-ch:
		if evt.EventType() != "approval_bulk_unsnoozed" {
			t.Errorf("expected event type 'approval_bulk_unsnoozed', got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for bulk unsnooze event")
	}
}

func TestApprovalService_BulkUnsnooze_NoSnoozed(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	count, err := svc.BulkUnsnooze()
	if err != nil {
		t.Fatalf("BulkUnsnooze returned error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 unsnoozed when no items, got %d", count)
	}
}

func TestApprovalService_CleanExpiredSnoozes(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)

	// Create items with expired and active snoozes
	expired := time.Now().UTC().Add(-1 * time.Hour) // Expired
	active := time.Now().UTC().Add(24 * time.Hour)  // Still active

	for _, tc := range []struct {
		name string
		snz  *time.Time
	}{
		{"Expired Movie", &expired},
		{"Active Movie", &active},
	} {
		item := db.ApprovalQueueItem{
			MediaName: tc.name, MediaType: "movie", Reason: "Score: 0.50",
			SizeBytes: 1000, IntegrationID: intID, ExternalID: "x",
			Status: db.StatusRejected, SnoozedUntil: tc.snz,
		}
		if err := database.Create(&item).Error; err != nil {
			t.Fatalf("Failed to create item: %v", err)
		}
	}

	count, err := svc.CleanExpiredSnoozes()
	if err != nil {
		t.Fatalf("CleanExpiredSnoozes returned error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 expired snooze cleaned, got %d", count)
	}

	// Verify: expired item is now pending, active item is still rejected
	var expiredItem db.ApprovalQueueItem
	database.Where("media_name = ?", "Expired Movie").First(&expiredItem)
	if expiredItem.Status != db.StatusPending {
		t.Errorf("expected expired item to be pending, got %q", expiredItem.Status)
	}

	var activeItem db.ApprovalQueueItem
	database.Where("media_name = ?", "Active Movie").First(&activeItem)
	if activeItem.Status != db.StatusRejected {
		t.Errorf("expected active item to still be rejected, got %q", activeItem.Status)
	}
}

func TestApprovalService_RecoverOrphans(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)

	// Create an approved item (orphaned — no deletion in progress)
	item := db.ApprovalQueueItem{
		MediaName: "Orphaned Movie", MediaType: "movie", Reason: "Score: 0.70",
		SizeBytes: 1000, IntegrationID: intID, ExternalID: "orphan",
		Status: db.StatusApproved,
	}
	if err := database.Create(&item).Error; err != nil {
		t.Fatalf("Failed to create item: %v", err)
	}

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	count, err := svc.RecoverOrphans()
	if err != nil {
		t.Fatalf("RecoverOrphans returned error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 orphan recovered, got %d", count)
	}

	// Verify item is now pending
	var recovered db.ApprovalQueueItem
	database.First(&recovered, item.ID)
	if recovered.Status != db.StatusPending {
		t.Errorf("expected recovered item to be pending, got %q", recovered.Status)
	}

	// Verify event published
	select {
	case evt := <-ch:
		if evt.EventType() != "approval_orphans_recovered" {
			t.Errorf("expected event type 'approval_orphans_recovered', got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for orphan recovery event")
	}
}

func TestApprovalService_ListQueue(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)

	// Create items with different statuses
	_, _ = svc.UpsertPending(db.ApprovalQueueItem{
		MediaName: "Firefly", MediaType: "show", Reason: "Score: 0.80",
		IntegrationID: intID, ExternalID: "1",
	})
	_, _ = svc.UpsertPending(db.ApprovalQueueItem{
		MediaName: "Serenity", MediaType: "movie", Reason: "Score: 0.90",
		IntegrationID: intID, ExternalID: "2",
	})

	// All items
	items, err := svc.ListQueue("", 100)
	if err != nil {
		t.Fatalf("ListQueue returned error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	// Filter by status
	pendingItems, err := svc.ListQueue("pending", 100)
	if err != nil {
		t.Fatalf("ListQueue(pending) returned error: %v", err)
	}
	if len(pendingItems) != 2 {
		t.Errorf("expected 2 pending items, got %d", len(pendingItems))
	}
}

func TestApprovalService_ListQueue_Limit(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)

	for i := 0; i < 5; i++ {
		_, _ = svc.UpsertPending(db.ApprovalQueueItem{
			MediaName: "Firefly " + string(rune('A'+i)), MediaType: "show", Reason: "Score",
			IntegrationID: intID, ExternalID: string(rune('1' + i)),
		})
	}

	items, err := svc.ListQueue("", 3)
	if err != nil {
		t.Fatalf("ListQueue returned error: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items (limit), got %d", len(items))
	}
}
