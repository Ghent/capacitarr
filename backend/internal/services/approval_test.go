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
		Score:         0.85,
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
	if result.Score != 0.85 {
		t.Errorf("expected score 0.85 preserved after approve, got %f", result.Score)
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

func TestApprovalService_Dismiss_Pending(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)
	item := seedPendingItem(t, database, intID)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	err := svc.Dismiss(item.ID)
	if err != nil {
		t.Fatalf("Dismiss returned error: %v", err)
	}

	// Verify item was deleted from DB
	var count int64
	database.Model(&db.ApprovalQueueItem{}).Where("id = ?", item.ID).Count(&count)
	if count != 0 {
		t.Error("expected item to be deleted from DB")
	}

	// Verify event was published
	select {
	case evt := <-ch:
		if evt.EventType() != "approval_dismissed" {
			t.Errorf("expected event type 'approval_dismissed', got %q", evt.EventType())
		}
		dismissed, ok := evt.(events.ApprovalDismissedEvent)
		if !ok {
			t.Fatalf("expected ApprovalDismissedEvent, got %T", evt)
		}
		if dismissed.EntryID != item.ID {
			t.Errorf("expected entry ID %d, got %d", item.ID, dismissed.EntryID)
		}
		if dismissed.MediaName != "Firefly" {
			t.Errorf("expected media name 'Firefly', got %q", dismissed.MediaName)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dismissed event")
	}
}

func TestApprovalService_Dismiss_Rejected(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)
	item := seedPendingItem(t, database, intID)

	// Reject the item first so it's in rejected status
	if _, err := svc.Reject(item.ID, 24); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	err := svc.Dismiss(item.ID)
	if err != nil {
		t.Fatalf("Dismiss returned error: %v", err)
	}

	// Verify item was deleted from DB
	var count int64
	database.Model(&db.ApprovalQueueItem{}).Where("id = ?", item.ID).Count(&count)
	if count != 0 {
		t.Error("expected rejected item to be deleted from DB")
	}

	// Verify event was published
	select {
	case evt := <-ch:
		if evt.EventType() != "approval_dismissed" {
			t.Errorf("expected event type 'approval_dismissed', got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for dismissed event")
	}
}

func TestApprovalService_Dismiss_Approved(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)
	item := seedPendingItem(t, database, intID)

	// Approve the item
	if _, err := svc.Approve(item.ID); err != nil {
		t.Fatalf("Approve failed: %v", err)
	}

	// Dismissing an approved item should fail
	err := svc.Dismiss(item.ID)
	if err == nil {
		t.Fatal("expected error when dismissing approved item")
	}

	// Verify item still exists in DB
	var count int64
	database.Model(&db.ApprovalQueueItem{}).Where("id = ?", item.ID).Count(&count)
	if count != 1 {
		t.Error("expected approved item to still exist in DB")
	}
}

func TestApprovalService_Dismiss_NotFound(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	err := svc.Dismiss(99999)
	if err == nil {
		t.Fatal("expected error for non-existent entry")
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
		Score:         0.90,
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

	// Verify score is stored
	var item db.ApprovalQueueItem
	database.First(&item)
	if item.Score != 0.90 {
		t.Errorf("expected score 0.90, got %f", item.Score)
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
		Score:         0.80,
		IntegrationID: intID,
		ExternalID:    "42",
	})
	if err != nil {
		t.Fatalf("First UpsertPending failed: %v", err)
	}

	// Upsert again with updated reason and score
	created, err := svc.UpsertPending(db.ApprovalQueueItem{
		MediaName:     "Existing Movie",
		MediaType:     "movie",
		Reason:        "Score: 0.95",
		SizeBytes:     2000000,
		Score:         0.95,
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
	if item.Score != 0.95 {
		t.Errorf("expected updated score 0.95, got %f", item.Score)
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

	count, err := svc.BulkUnsnooze(nil)
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

	count, err := svc.BulkUnsnooze(nil)
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
	items, err := svc.ListQueue("", 100, nil)
	if err != nil {
		t.Fatalf("ListQueue returned error: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	// Filter by status
	pendingItems, err := svc.ListQueue("pending", 100, nil)
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

	items, err := svc.ListQueue("", 3, nil)
	if err != nil {
		t.Fatalf("ListQueue returned error: %v", err)
	}
	if len(items) != 3 {
		t.Errorf("expected 3 items (limit), got %d", len(items))
	}
}

func TestApprovalService_ClearQueue(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)

	// Create 3 items: 1 pending, 1 rejected, 1 approved
	pending := db.ApprovalQueueItem{
		MediaName: "Firefly", MediaType: "show", Reason: "Score: 0.85",
		SizeBytes: 5000, IntegrationID: intID, ExternalID: "1",
		Status: db.StatusPending,
	}
	if err := database.Create(&pending).Error; err != nil {
		t.Fatalf("Failed to create pending item: %v", err)
	}

	snoozedUntil := time.Now().UTC().Add(24 * time.Hour)
	rejected := db.ApprovalQueueItem{
		MediaName: "Serenity", MediaType: "movie", Reason: "Score: 0.70",
		SizeBytes: 3000, IntegrationID: intID, ExternalID: "2",
		Status: db.StatusRejected, SnoozedUntil: &snoozedUntil,
	}
	if err := database.Create(&rejected).Error; err != nil {
		t.Fatalf("Failed to create rejected item: %v", err)
	}

	approved := db.ApprovalQueueItem{
		MediaName: "Firefly - Season 1", MediaType: "season", Reason: "Score: 0.90",
		SizeBytes: 8000, IntegrationID: intID, ExternalID: "3",
		Status: db.StatusApproved,
	}
	if err := database.Create(&approved).Error; err != nil {
		t.Fatalf("Failed to create approved item: %v", err)
	}

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	count, err := svc.ClearQueue()
	if err != nil {
		t.Fatalf("ClearQueue returned error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 items cleared, got %d", count)
	}

	// Verify only the approved item remains
	var remaining []db.ApprovalQueueItem
	database.Find(&remaining)
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining item, got %d", len(remaining))
	}
	if remaining[0].Status != db.StatusApproved {
		t.Errorf("expected remaining item to be approved, got %q", remaining[0].Status)
	}
	if remaining[0].MediaName != "Firefly - Season 1" {
		t.Errorf("expected remaining item to be 'Firefly - Season 1', got %q", remaining[0].MediaName)
	}

	// Verify event published
	select {
	case evt := <-ch:
		if evt.EventType() != "approval_queue_cleared" {
			t.Errorf("expected event type 'approval_queue_cleared', got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for queue cleared event")
	}
}

func TestApprovalService_ClearQueue_PreservesApproved(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)

	// Create only an approved item
	approved := db.ApprovalQueueItem{
		MediaName: "Firefly", MediaType: "show", Reason: "Score: 0.85",
		SizeBytes: 5000, IntegrationID: intID, ExternalID: "1",
		Status: db.StatusApproved,
	}
	if err := database.Create(&approved).Error; err != nil {
		t.Fatalf("Failed to create approved item: %v", err)
	}

	count, err := svc.ClearQueue()
	if err != nil {
		t.Fatalf("ClearQueue returned error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 items cleared (only approved present), got %d", count)
	}

	// Verify approved item still exists
	var remaining int64
	database.Model(&db.ApprovalQueueItem{}).Count(&remaining)
	if remaining != 1 {
		t.Errorf("expected 1 item remaining, got %d", remaining)
	}
}

func TestApprovalService_ClearQueue_Empty(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	count, err := svc.ClearQueue()
	if err != nil {
		t.Fatalf("ClearQueue returned error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 items cleared on empty queue, got %d", count)
	}

	// No event should be published for empty queue
	select {
	case evt := <-ch:
		t.Errorf("unexpected event published: %q", evt.EventType())
	case <-time.After(100 * time.Millisecond):
		// Expected: no event
	}
}

func TestApprovalService_ClearQueue_PublishesEvent(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)

	// Create multiple pending items
	for i, name := range []string{"Firefly", "Serenity"} {
		item := db.ApprovalQueueItem{
			MediaName: name, MediaType: "show", Reason: "Score: 0.50",
			SizeBytes: 1000, IntegrationID: intID, ExternalID: string(rune('1' + i)),
			Status: db.StatusPending,
		}
		if err := database.Create(&item).Error; err != nil {
			t.Fatalf("Failed to create item: %v", err)
		}
	}

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	count, err := svc.ClearQueue()
	if err != nil {
		t.Fatalf("ClearQueue returned error: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 items cleared, got %d", count)
	}

	// Verify event with correct count
	select {
	case evt := <-ch:
		cleared, ok := evt.(events.ApprovalQueueClearedEvent)
		if !ok {
			t.Fatalf("expected ApprovalQueueClearedEvent, got %T", evt)
		}
		if cleared.Count != 2 {
			t.Errorf("expected event count 2, got %d", cleared.Count)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for queue cleared event")
	}
}

// seedDiskGroup creates a minimal disk group for FK references.
func seedDiskGroup(t *testing.T, database *gorm.DB) uint {
	t.Helper()
	dg := db.DiskGroup{
		MountPath:    "/mnt/media",
		TotalBytes:   1000000000,
		UsedBytes:    900000000,
		ThresholdPct: 80.0,
		TargetPct:    70.0,
	}
	if err := database.Create(&dg).Error; err != nil {
		t.Fatalf("Failed to seed disk group: %v", err)
	}
	return dg.ID
}

func TestApprovalService_ListPendingForDiskGroup(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)
	dgID := seedDiskGroup(t, database)

	// Create a second disk group for cross-group filtering
	otherDG := db.DiskGroup{MountPath: "/mnt/other", TotalBytes: 500000000, UsedBytes: 400000000, ThresholdPct: 80.0, TargetPct: 70.0}
	if err := database.Create(&otherDG).Error; err != nil {
		t.Fatalf("Failed to create other disk group: %v", err)
	}
	otherDGID := otherDG.ID

	// Create pending items: two for our disk group, one for another
	for _, item := range []db.ApprovalQueueItem{
		{MediaName: "Firefly", MediaType: "show", Status: db.StatusPending, IntegrationID: intID, ExternalID: "1", DiskGroupID: &dgID},
		{MediaName: "Serenity", MediaType: "movie", Status: db.StatusPending, IntegrationID: intID, ExternalID: "2", DiskGroupID: &dgID},
		{MediaName: "Other Show", MediaType: "show", Status: db.StatusPending, IntegrationID: intID, ExternalID: "3", DiskGroupID: &otherDGID},
		{MediaName: "Rejected Show", MediaType: "show", Status: db.StatusRejected, IntegrationID: intID, ExternalID: "4", DiskGroupID: &dgID},
	} {
		if err := database.Create(&item).Error; err != nil {
			t.Fatalf("Failed to create item: %v", err)
		}
	}

	items, err := svc.ListPendingForDiskGroup(dgID)
	if err != nil {
		t.Fatalf("ListPendingForDiskGroup returned error: %v", err)
	}

	if len(items) != 2 {
		t.Errorf("expected 2 pending items for disk group, got %d", len(items))
	}

	// Verify the items are the correct ones
	names := make(map[string]bool)
	for _, item := range items {
		names[item.MediaName] = true
	}
	if !names["Firefly"] || !names["Serenity"] {
		t.Errorf("expected Firefly and Serenity, got %v", names)
	}
}

func TestApprovalService_ReconcileQueue_DismissesStaleItems(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)
	dgID := seedDiskGroup(t, database)

	// Create 3 pending items for the disk group
	for _, item := range []db.ApprovalQueueItem{
		{MediaName: "Firefly", MediaType: "show", Status: db.StatusPending, IntegrationID: intID, ExternalID: "1", DiskGroupID: &dgID},
		{MediaName: "Serenity", MediaType: "movie", Status: db.StatusPending, IntegrationID: intID, ExternalID: "2", DiskGroupID: &dgID},
		{MediaName: "Firefly - Season 1", MediaType: "season", Status: db.StatusPending, IntegrationID: intID, ExternalID: "3", DiskGroupID: &dgID},
	} {
		if err := database.Create(&item).Error; err != nil {
			t.Fatalf("Failed to create item: %v", err)
		}
	}

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	// Only "Firefly|show" is still needed — the other two should be dismissed
	neededKeys := map[string]bool{
		"Firefly|show": true,
	}

	dismissed, err := svc.ReconcileQueue(dgID, neededKeys)
	if err != nil {
		t.Fatalf("ReconcileQueue returned error: %v", err)
	}

	if dismissed != 2 {
		t.Errorf("expected 2 items dismissed, got %d", dismissed)
	}

	// Verify only the needed item remains
	var remaining []db.ApprovalQueueItem
	database.Where("disk_group_id = ?", dgID).Find(&remaining)
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining item, got %d", len(remaining))
	}
	if remaining[0].MediaName != "Firefly" {
		t.Errorf("expected remaining item to be 'Firefly', got %q", remaining[0].MediaName)
	}

	// Verify reconciliation event was published
	select {
	case evt := <-ch:
		reconciled, ok := evt.(events.ApprovalQueueReconciledEvent)
		if !ok {
			t.Fatalf("expected ApprovalQueueReconciledEvent, got %T", evt)
		}
		if reconciled.Dismissed != 2 {
			t.Errorf("expected event dismissed count 2, got %d", reconciled.Dismissed)
		}
		if reconciled.DiskGroupID != dgID {
			t.Errorf("expected event disk group ID %d, got %d", dgID, reconciled.DiskGroupID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for reconciliation event")
	}
}

func TestApprovalService_ReconcileQueue_LeavesRejectedUntouched(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)
	dgID := seedDiskGroup(t, database)

	snoozedUntil := time.Now().UTC().Add(24 * time.Hour)

	// Create a pending and a rejected item
	for _, item := range []db.ApprovalQueueItem{
		{MediaName: "Firefly", MediaType: "show", Status: db.StatusPending, IntegrationID: intID, ExternalID: "1", DiskGroupID: &dgID},
		{MediaName: "Serenity", MediaType: "movie", Status: db.StatusRejected, SnoozedUntil: &snoozedUntil, IntegrationID: intID, ExternalID: "2", DiskGroupID: &dgID},
	} {
		if err := database.Create(&item).Error; err != nil {
			t.Fatalf("Failed to create item: %v", err)
		}
	}

	// No items are still needed — pending should be dismissed, rejected should stay
	dismissed, err := svc.ReconcileQueue(dgID, map[string]bool{})
	if err != nil {
		t.Fatalf("ReconcileQueue returned error: %v", err)
	}

	if dismissed != 1 {
		t.Errorf("expected 1 item dismissed (pending only), got %d", dismissed)
	}

	// Verify rejected item is preserved
	var remaining []db.ApprovalQueueItem
	database.Where("disk_group_id = ?", dgID).Find(&remaining)
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining item (rejected), got %d", len(remaining))
	}
	if remaining[0].Status != db.StatusRejected {
		t.Errorf("expected remaining item to be rejected, got %q", remaining[0].Status)
	}
}

func TestApprovalService_ReconcileQueue_NoopWhenAllNeeded(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewApprovalService(database, bus)

	intID := seedIntegration(t, database)
	dgID := seedDiskGroup(t, database)

	// Create pending items
	for _, item := range []db.ApprovalQueueItem{
		{MediaName: "Firefly", MediaType: "show", Status: db.StatusPending, IntegrationID: intID, ExternalID: "1", DiskGroupID: &dgID},
		{MediaName: "Serenity", MediaType: "movie", Status: db.StatusPending, IntegrationID: intID, ExternalID: "2", DiskGroupID: &dgID},
	} {
		if err := database.Create(&item).Error; err != nil {
			t.Fatalf("Failed to create item: %v", err)
		}
	}

	// All items are still needed
	neededKeys := map[string]bool{
		"Firefly|show":   true,
		"Serenity|movie": true,
	}

	dismissed, err := svc.ReconcileQueue(dgID, neededKeys)
	if err != nil {
		t.Fatalf("ReconcileQueue returned error: %v", err)
	}

	if dismissed != 0 {
		t.Errorf("expected 0 items dismissed, got %d", dismissed)
	}

	// Verify all items remain
	var remaining []db.ApprovalQueueItem
	database.Where("disk_group_id = ?", dgID).Find(&remaining)
	if len(remaining) != 2 {
		t.Errorf("expected 2 remaining items, got %d", len(remaining))
	}
}
