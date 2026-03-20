package services

import (
	"errors"
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
)

// errMockDelete is a sentinel error for simulating deletion failures in tests.
var errMockDelete = errors.New("mock delete error")

// mockSettingsReader implements SettingsReader for deletion tests.
type mockSettingsReader struct {
	deletionsEnabled bool
}

func (m *mockSettingsReader) GetPreferences() (db.PreferenceSet, error) {
	return db.PreferenceSet{DeletionsEnabled: m.deletionsEnabled}, nil
}

// mockEngineStatsWriter implements EngineStatsWriter for deletion tests.
type mockEngineStatsWriter struct{}

func (m *mockEngineStatsWriter) IncrementDeletedStats(_ uint, _ int64) error { return nil }

// mockDeletionStatsWriter implements DeletionStatsWriter for deletion tests.
type mockDeletionStatsWriter struct{}

func (m *mockDeletionStatsWriter) IncrementDeletionStats(_ int64) error { return nil }

// mockIntegration implements integrations.Integration for deletion tests.
type mockIntegration struct {
	deleteErr error
}

func (m *mockIntegration) TestConnection() error {
	return nil
}

func (m *mockIntegration) GetDiskSpace() ([]integrations.DiskSpace, error) {
	return nil, nil
}

func (m *mockIntegration) GetRootFolders() ([]string, error) {
	return nil, nil
}

func (m *mockIntegration) GetMediaItems() ([]integrations.MediaItem, error) {
	return nil, nil
}

func (m *mockIntegration) DeleteMediaItem(_ integrations.MediaItem) error {
	return m.deleteErr
}

// drainProgressEvent reads from the bus subscription channel until a
// DeletionProgressEvent arrives or the timeout expires.
func drainProgressEvent(t *testing.T, ch chan events.Event, timeout time.Duration) *events.DeletionProgressEvent {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case evt := <-ch:
			if pe, ok := evt.(events.DeletionProgressEvent); ok {
				return &pe
			}
			// Ignore other events
		case <-deadline:
			t.Fatal("timeout waiting for DeletionProgressEvent")
			return nil
		}
	}
}

// drainBatchEvent reads from the bus subscription channel until a
// DeletionBatchCompleteEvent arrives or the timeout expires.
func drainBatchEvent(t *testing.T, ch chan events.Event, timeout time.Duration) *events.DeletionBatchCompleteEvent {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case evt := <-ch:
			if bce, ok := evt.(events.DeletionBatchCompleteEvent); ok {
				return &bce
			}
			// Ignore other events (DeletionSuccessEvent, DeletionDryRunEvent, etc.)
		case <-deadline:
			t.Fatal("timeout waiting for DeletionBatchCompleteEvent")
			return nil
		}
	}
}

func TestDeletionService_SignalBatchSize_Zero(t *testing.T) {
	bus := newTestBus(t)
	auditLog := NewAuditLogService(setupTestDB(t))
	svc := NewDeletionService(bus, auditLog)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	// SignalBatchSize(0) should immediately publish DeletionBatchCompleteEvent
	svc.SignalBatchSize(0)

	bce := drainBatchEvent(t, ch, 2*time.Second)
	if bce.Succeeded != 0 {
		t.Errorf("expected Succeeded=0, got %d", bce.Succeeded)
	}
	if bce.Failed != 0 {
		t.Errorf("expected Failed=0, got %d", bce.Failed)
	}
}

func TestDeletionService_BatchTracking_AllSuccess(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)
	svc.SetDependencies(
		&mockSettingsReader{deletionsEnabled: false}, // dry-run mode
		&mockEngineStatsWriter{},
		&mockDeletionStatsWriter{},
	)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	svc.Start()
	defer svc.Stop()

	// Signal 3 items in this batch
	svc.SignalBatchSize(3)

	// Queue 3 jobs (dry-run mode — deletionsEnabled is false)
	for i := 0; i < 3; i++ {
		job := DeleteJob{
			Client: &mockIntegration{},
			Item: integrations.MediaItem{
				Title:     "Serenity",
				Type:      "movie",
				SizeBytes: 1024 * 1024 * 100,
			},
			Reason: "test",
		}
		if err := svc.QueueDeletion(job); err != nil {
			t.Fatalf("QueueDeletion returned error: %v", err)
		}
	}

	bce := drainBatchEvent(t, ch, 15*time.Second)
	if bce.Succeeded != 3 {
		t.Errorf("expected Succeeded=3, got %d", bce.Succeeded)
	}
	if bce.Failed != 0 {
		t.Errorf("expected Failed=0, got %d", bce.Failed)
	}
}

func TestDeletionService_BatchTracking_MixedSuccessFailure(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)
	svc.SetDependencies(
		&mockSettingsReader{deletionsEnabled: true}, // actual deletions
		&mockEngineStatsWriter{},
		&mockDeletionStatsWriter{},
	)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	svc.Start()
	defer svc.Stop()

	// Signal 3 items
	svc.SignalBatchSize(3)

	// Queue 2 success + 1 failure
	for i := 0; i < 2; i++ {
		job := DeleteJob{
			Client: &mockIntegration{deleteErr: nil},
			Item: integrations.MediaItem{
				Title:     "Serenity",
				Type:      "movie",
				SizeBytes: 1024 * 1024 * 50,
			},
			Reason: "test",
		}
		if err := svc.QueueDeletion(job); err != nil {
			t.Fatalf("QueueDeletion returned error: %v", err)
		}
	}

	// Queue 1 failure
	failJob := DeleteJob{
		Client: &mockIntegration{deleteErr: errMockDelete},
		Item: integrations.MediaItem{
			Title:     "Firefly",
			Type:      "show",
			SizeBytes: 1024 * 1024 * 200,
		},
		Reason: "test",
	}
	if err := svc.QueueDeletion(failJob); err != nil {
		t.Fatalf("QueueDeletion returned error: %v", err)
	}

	bce := drainBatchEvent(t, ch, 15*time.Second)
	if bce.Succeeded != 2 {
		t.Errorf("expected Succeeded=2, got %d", bce.Succeeded)
	}
	if bce.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", bce.Failed)
	}
}

func TestDeletionService_BatchTracking_CorrectCounts(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)
	svc.SetDependencies(
		&mockSettingsReader{deletionsEnabled: true},
		&mockEngineStatsWriter{},
		&mockDeletionStatsWriter{},
	)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	svc.Start()
	defer svc.Stop()

	// Signal 5 items: 3 succeed, 2 fail
	svc.SignalBatchSize(5)

	for i := 0; i < 3; i++ {
		_ = svc.QueueDeletion(DeleteJob{
			Client: &mockIntegration{deleteErr: nil},
			Item: integrations.MediaItem{
				Title:     "Serenity",
				Type:      "movie",
				SizeBytes: int64(i+1) * 1024 * 1024 * 10,
			},
			Reason: "test",
		})
	}
	for i := 0; i < 2; i++ {
		_ = svc.QueueDeletion(DeleteJob{
			Client: &mockIntegration{deleteErr: errMockDelete},
			Item: integrations.MediaItem{
				Title:     "Firefly",
				Type:      "show",
				SizeBytes: 1024 * 1024 * 5,
			},
			Reason: "test",
		})
	}

	bce := drainBatchEvent(t, ch, 20*time.Second)
	if bce.Succeeded != 3 {
		t.Errorf("expected Succeeded=3, got %d", bce.Succeeded)
	}
	if bce.Failed != 2 {
		t.Errorf("expected Failed=2, got %d", bce.Failed)
	}
}

func TestDeletionService_GracefulShutdown_DrainsQueue(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)
	svc.SetDependencies(
		&mockSettingsReader{deletionsEnabled: false}, // dry-run mode for safety
		&mockEngineStatsWriter{},
		&mockDeletionStatsWriter{},
	)

	svc.Start()

	// Queue a job, then immediately stop — the worker should drain the queue
	// before Stop() returns.
	job := DeleteJob{
		Client: &mockIntegration{deleteErr: nil},
		Item: integrations.MediaItem{
			Title:     "Serenity",
			Type:      "movie",
			SizeBytes: 1024 * 1024 * 100,
		},
		Reason: "shutdown-test",
	}
	if err := svc.QueueDeletion(job); err != nil {
		t.Fatalf("QueueDeletion returned error: %v", err)
	}

	// Stop should block until all queued jobs are processed
	svc.Stop()

	// Verify the job was processed (counter should be 1)
	if svc.Processed() != 1 {
		t.Errorf("expected 1 processed job after graceful shutdown, got %d", svc.Processed())
	}
}

func TestDeletionProgressEvent_EventType(t *testing.T) {
	evt := events.DeletionProgressEvent{
		CurrentItem: "Serenity",
		QueueDepth:  3,
		Processed:   2,
		Succeeded:   1,
		Failed:      1,
		BatchTotal:  5,
	}

	if got := evt.EventType(); got != "deletion_progress" {
		t.Errorf("expected EventType() = %q, got %q", "deletion_progress", got)
	}
}

func TestDeletionProgressEvent_EventMessage(t *testing.T) {
	evt := events.DeletionProgressEvent{
		CurrentItem: "Serenity",
		QueueDepth:  3,
		Processed:   2,
		Succeeded:   1,
		Failed:      1,
		BatchTotal:  5,
	}

	expected := "Deletion progress: 2/5 completed (1 succeeded, 1 failed)"
	if got := evt.EventMessage(); got != expected {
		t.Errorf("expected EventMessage() = %q, got %q", expected, got)
	}
}

func TestDeletionService_ProgressEvent_DryRun(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)
	svc.SetDependencies(
		&mockSettingsReader{deletionsEnabled: false}, // dry-run mode
		&mockEngineStatsWriter{},
		&mockDeletionStatsWriter{},
	)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	svc.Start()
	defer svc.Stop()

	svc.SignalBatchSize(1)

	job := DeleteJob{
		Client: &mockIntegration{},
		Item: integrations.MediaItem{
			Title:     "Serenity",
			Type:      "movie",
			SizeBytes: 1024 * 1024 * 100,
		},
		Reason: "test-progress",
		Score:  0.72,
	}
	if err := svc.QueueDeletion(job); err != nil {
		t.Fatalf("QueueDeletion returned error: %v", err)
	}

	pe := drainProgressEvent(t, ch, 15*time.Second)
	if pe.Succeeded != 1 {
		t.Errorf("expected Succeeded=1, got %d", pe.Succeeded)
	}
	if pe.Failed != 0 {
		t.Errorf("expected Failed=0, got %d", pe.Failed)
	}
	if pe.Processed != 1 {
		t.Errorf("expected Processed=1, got %d", pe.Processed)
	}
	if pe.BatchTotal != 1 {
		t.Errorf("expected BatchTotal=1, got %d", pe.BatchTotal)
	}

	// Verify audit log entry contains the score from the DeleteJob
	var entry db.AuditLogEntry
	if err := database.First(&entry).Error; err != nil {
		t.Fatalf("Expected audit log entry: %v", err)
	}
	if entry.Score != 0.72 {
		t.Errorf("expected audit log score 0.72, got %f", entry.Score)
	}
}

func TestDeletionService_ProgressEvent_ActualDeletion(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)
	svc.SetDependencies(
		&mockSettingsReader{deletionsEnabled: true},
		&mockEngineStatsWriter{},
		&mockDeletionStatsWriter{},
	)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	svc.Start()
	defer svc.Stop()

	svc.SignalBatchSize(1)

	job := DeleteJob{
		Client: &mockIntegration{deleteErr: nil},
		Item: integrations.MediaItem{
			Title:     "Serenity",
			Type:      "movie",
			SizeBytes: 1024 * 1024 * 50,
		},
		Reason: "test-progress",
		Score:  0.91,
	}
	if err := svc.QueueDeletion(job); err != nil {
		t.Fatalf("QueueDeletion returned error: %v", err)
	}

	pe := drainProgressEvent(t, ch, 15*time.Second)
	if pe.Succeeded != 1 {
		t.Errorf("expected Succeeded=1, got %d", pe.Succeeded)
	}
	if pe.Failed != 0 {
		t.Errorf("expected Failed=0, got %d", pe.Failed)
	}
	if pe.Processed != 1 {
		t.Errorf("expected Processed=1, got %d", pe.Processed)
	}
	if pe.BatchTotal != 1 {
		t.Errorf("expected BatchTotal=1, got %d", pe.BatchTotal)
	}

	// Verify audit log entry contains the score from the DeleteJob
	var entry db.AuditLogEntry
	if err := database.First(&entry).Error; err != nil {
		t.Fatalf("Expected audit log entry: %v", err)
	}
	if entry.Score != 0.91 {
		t.Errorf("expected audit log score 0.91, got %f", entry.Score)
	}
}

func TestDeletionService_ForceDryRun_OverridesDeletionsEnabled(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)
	svc.SetDependencies(
		&mockSettingsReader{deletionsEnabled: true}, // deletions enabled, but ForceDryRun overrides
		&mockEngineStatsWriter{},
		&mockDeletionStatsWriter{},
	)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	svc.Start()
	defer svc.Stop()

	svc.SignalBatchSize(1)

	// ForceDryRun=true should cause a dry-delete even though DeletionsEnabled=true
	job := DeleteJob{
		Client:      &mockIntegration{deleteErr: nil},
		Item:        integrations.MediaItem{Title: "Serenity", Type: "movie", SizeBytes: 1024 * 1024 * 100},
		Reason:      "force-dry-run-test",
		ForceDryRun: true,
	}
	if err := svc.QueueDeletion(job); err != nil {
		t.Fatalf("QueueDeletion returned error: %v", err)
	}

	// Should receive DeletionDryRunEvent, not DeletionSuccessEvent
	deadline := time.After(15 * time.Second)
	gotDryRun := false
	for {
		select {
		case evt := <-ch:
			switch evt.(type) {
			case events.DeletionDryRunEvent:
				gotDryRun = true
			case events.DeletionSuccessEvent:
				t.Fatal("Expected DeletionDryRunEvent but got DeletionSuccessEvent — ForceDryRun was not honoured")
			case events.DeletionBatchCompleteEvent:
				if !gotDryRun {
					t.Fatal("Batch completed without DeletionDryRunEvent")
				}
				return // test passed
			}
		case <-deadline:
			t.Fatal("timeout waiting for events")
		}
	}
}

func TestDeletionService_NoDryRun_WhenDeletionsDisabled(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)
	svc.SetDependencies(
		&mockSettingsReader{deletionsEnabled: false}, // deletions disabled
		&mockEngineStatsWriter{},
		&mockDeletionStatsWriter{},
	)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	svc.Start()
	defer svc.Stop()

	svc.SignalBatchSize(1)

	// ForceDryRun=false, but DeletionsEnabled=false → should dry-delete
	job := DeleteJob{
		Client:      &mockIntegration{deleteErr: nil},
		Item:        integrations.MediaItem{Title: "Firefly", Type: "show", SizeBytes: 1024 * 1024 * 200},
		Reason:      "deletions-disabled-test",
		ForceDryRun: false,
	}
	if err := svc.QueueDeletion(job); err != nil {
		t.Fatalf("QueueDeletion returned error: %v", err)
	}

	// Should receive DeletionDryRunEvent, not DeletionSuccessEvent
	deadline := time.After(15 * time.Second)
	gotDryRun := false
	for {
		select {
		case evt := <-ch:
			switch evt.(type) {
			case events.DeletionDryRunEvent:
				gotDryRun = true
			case events.DeletionSuccessEvent:
				t.Fatal("Expected DeletionDryRunEvent but got DeletionSuccessEvent")
			case events.DeletionBatchCompleteEvent:
				if !gotDryRun {
					t.Fatal("Batch completed without DeletionDryRunEvent")
				}
				return // test passed
			}
		case <-deadline:
			t.Fatal("timeout waiting for events")
		}
	}
}

// ---------------------------------------------------------------------------
// Cancellation tests
// ---------------------------------------------------------------------------

func TestDeletionService_CancelDeletion_ReturnsTrue_WhenItemInQueue(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)
	svc.SetDependencies(
		&mockSettingsReader{deletionsEnabled: true},
		&mockEngineStatsWriter{},
		&mockDeletionStatsWriter{},
	)

	// Queue an item (service NOT started — item stays in channel and tracking slice)
	job := DeleteJob{
		Client: &mockIntegration{},
		Item: integrations.MediaItem{
			Title:         "Firefly",
			Type:          "show",
			SizeBytes:     1024 * 1024 * 200,
			IntegrationID: 1,
		},
		Reason: "test-cancel",
	}
	if err := svc.QueueDeletion(job); err != nil {
		t.Fatalf("QueueDeletion returned error: %v", err)
	}

	if !svc.CancelDeletion("Firefly", "show") {
		t.Error("CancelDeletion returned false; expected true when item is in queue")
	}

	if !svc.IsCancelled("Firefly", "show") {
		t.Error("IsCancelled returned false after CancelDeletion")
	}
}

func TestDeletionService_CancelDeletion_ReturnsFalse_WhenNotInQueue(t *testing.T) {
	bus := newTestBus(t)
	auditLog := NewAuditLogService(setupTestDB(t))
	svc := NewDeletionService(bus, auditLog)

	if svc.CancelDeletion("Serenity", "movie") {
		t.Error("CancelDeletion returned true; expected false when item is not in queue")
	}
}

func TestDeletionService_ProcessJob_SkipsCancelledItem(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)
	svc.SetDependencies(
		&mockSettingsReader{deletionsEnabled: true},
		&mockEngineStatsWriter{},
		&mockDeletionStatsWriter{},
	)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	svc.Start()
	defer svc.Stop()

	svc.SignalBatchSize(1)

	// Queue a job, then cancel before the worker processes it.
	// We rely on the rate limiter (3s) giving us time to cancel.
	job := DeleteJob{
		Client: &mockIntegration{deleteErr: nil},
		Item: integrations.MediaItem{
			Title:         "Firefly",
			Type:          "show",
			SizeBytes:     1024 * 1024 * 200,
			IntegrationID: 1,
		},
		Reason: "test-cancel-process",
	}
	if err := svc.QueueDeletion(job); err != nil {
		t.Fatalf("QueueDeletion returned error: %v", err)
	}

	// Mark as cancelled
	svc.cancelled.Store(cancelKey("Firefly", "show"), true)

	// Wait for events — should get DeletionCancelledEvent, NOT DeletionSuccessEvent
	deadline := time.After(15 * time.Second)
	gotCancelled := false
	for {
		select {
		case evt := <-ch:
			switch evt.(type) {
			case events.DeletionCancelledEvent:
				gotCancelled = true
			case events.DeletionSuccessEvent:
				t.Fatal("Expected DeletionCancelledEvent but got DeletionSuccessEvent")
			case events.DeletionBatchCompleteEvent:
				if !gotCancelled {
					t.Fatal("Batch completed without DeletionCancelledEvent")
				}
				// Verify audit log entry was created
				var entry db.AuditLogEntry
				if err := database.Where("action = ?", db.ActionCancelled).First(&entry).Error; err != nil {
					t.Fatalf("Expected cancelled audit log entry: %v", err)
				}
				if entry.MediaName != "Firefly" {
					t.Errorf("Expected media name 'Firefly', got %q", entry.MediaName)
				}
				return // test passed
			}
		case <-deadline:
			t.Fatal("timeout waiting for events")
		}
	}
}

func TestDeletionService_ListQueuedItems_ReturnsSnapshot(t *testing.T) {
	bus := newTestBus(t)
	auditLog := NewAuditLogService(setupTestDB(t))
	svc := NewDeletionService(bus, auditLog)

	// Empty initially
	items := svc.ListQueuedItems()
	if len(items) != 0 {
		t.Errorf("expected 0 queued items initially, got %d", len(items))
	}

	// Queue two items (service NOT started — items stay in tracking slice)
	_ = svc.QueueDeletion(DeleteJob{
		Client: &mockIntegration{},
		Item:   integrations.MediaItem{Title: "Firefly", Type: "show", SizeBytes: 100, IntegrationID: 1},
		Reason: "reason-1",
	})
	_ = svc.QueueDeletion(DeleteJob{
		Client: &mockIntegration{},
		Item:   integrations.MediaItem{Title: "Serenity", Type: "movie", SizeBytes: 200, IntegrationID: 2},
		Reason: "reason-2",
	})

	items = svc.ListQueuedItems()
	if len(items) != 2 {
		t.Fatalf("expected 2 queued items, got %d", len(items))
	}
	if items[0].MediaName != "Firefly" {
		t.Errorf("expected first item 'Firefly', got %q", items[0].MediaName)
	}
	if items[1].MediaName != "Serenity" {
		t.Errorf("expected second item 'Serenity', got %q", items[1].MediaName)
	}

	// Verify snapshot isolation — mutating returned slice doesn't affect internal state
	items[0].MediaName = "modified"
	fresh := svc.ListQueuedItems()
	if fresh[0].MediaName != "Firefly" {
		t.Error("ListQueuedItems did not return a copy; internal state was mutated")
	}
}

func TestDeletionService_SignalBatchSize_ClearsCancelledSet(t *testing.T) {
	bus := newTestBus(t)
	auditLog := NewAuditLogService(setupTestDB(t))
	svc := NewDeletionService(bus, auditLog)

	// Add an item to queue and cancel it
	_ = svc.QueueDeletion(DeleteJob{
		Client: &mockIntegration{},
		Item:   integrations.MediaItem{Title: "Firefly", Type: "show", SizeBytes: 100},
		Reason: "test",
	})
	svc.CancelDeletion("Firefly", "show")

	if !svc.IsCancelled("Firefly", "show") {
		t.Fatal("expected IsCancelled=true before SignalBatchSize")
	}

	// Drain the batch complete event from SignalBatchSize(0)
	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	svc.SignalBatchSize(0)

	if svc.IsCancelled("Firefly", "show") {
		t.Error("expected IsCancelled=false after SignalBatchSize cleared the set")
	}
}

func TestDeletionCancelledEvent_EventType(t *testing.T) {
	evt := events.DeletionCancelledEvent{
		MediaName: "Firefly",
		MediaType: "show",
		SizeBytes: 1024,
	}

	if got := evt.EventType(); got != "deletion_cancelled" {
		t.Errorf("expected EventType() = %q, got %q", "deletion_cancelled", got)
	}
}

func TestDeletionCancelledEvent_EventMessage(t *testing.T) {
	evt := events.DeletionCancelledEvent{
		MediaName: "Firefly",
		MediaType: "show",
		SizeBytes: 1024,
	}

	expected := "Deletion cancelled: Firefly"
	if got := evt.EventMessage(); got != expected {
		t.Errorf("expected EventMessage() = %q, got %q", expected, got)
	}
}

// ---------------------------------------------------------------------------
// Existing tests continued
// ---------------------------------------------------------------------------

func TestDeletionService_ProgressEvent_Failure(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)
	svc.SetDependencies(
		&mockSettingsReader{deletionsEnabled: true},
		&mockEngineStatsWriter{},
		&mockDeletionStatsWriter{},
	)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	svc.Start()
	defer svc.Stop()

	svc.SignalBatchSize(1)

	job := DeleteJob{
		Client: &mockIntegration{deleteErr: errMockDelete},
		Item: integrations.MediaItem{
			Title:     "Firefly",
			Type:      "show",
			SizeBytes: 1024 * 1024 * 200,
		},
		Reason: "test-progress",
	}
	if err := svc.QueueDeletion(job); err != nil {
		t.Fatalf("QueueDeletion returned error: %v", err)
	}

	pe := drainProgressEvent(t, ch, 15*time.Second)
	if pe.Succeeded != 0 {
		t.Errorf("expected Succeeded=0, got %d", pe.Succeeded)
	}
	if pe.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", pe.Failed)
	}
	if pe.Processed != 1 {
		t.Errorf("expected Processed=1, got %d", pe.Processed)
	}
	if pe.BatchTotal != 1 {
		t.Errorf("expected BatchTotal=1, got %d", pe.BatchTotal)
	}
}

func TestDeletionQueuedEvent_EventType(t *testing.T) {
	evt := events.DeletionQueuedEvent{
		MediaName:     "Serenity",
		MediaType:     "movie",
		SizeBytes:     1024 * 1024 * 100,
		IntegrationID: 1,
	}

	if got := evt.EventType(); got != "deletion_queued" {
		t.Errorf("expected EventType() = %q, got %q", "deletion_queued", got)
	}
}

func TestDeletionQueuedEvent_EventMessage(t *testing.T) {
	evt := events.DeletionQueuedEvent{
		MediaName:     "Serenity",
		MediaType:     "movie",
		SizeBytes:     1024 * 1024 * 100,
		IntegrationID: 1,
	}

	expected := "Queued for deletion: Serenity"
	if got := evt.EventMessage(); got != expected {
		t.Errorf("expected EventMessage() = %q, got %q", expected, got)
	}
}

func TestDeletionService_UpsertAudit_UsesUpsertSemantics(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)
	svc.SetDependencies(
		&mockSettingsReader{deletionsEnabled: false}, // dry-run mode
		&mockEngineStatsWriter{},
		&mockDeletionStatsWriter{},
	)

	svc.Start()
	defer svc.Stop()

	// Queue the same item twice with UpsertAudit=true — should produce only 1 audit entry
	for i := 0; i < 2; i++ {
		svc.SignalBatchSize(1)
		job := DeleteJob{
			Client:      nil, // Dry-run with nil client
			Item:        integrations.MediaItem{Title: "Firefly", Type: "show", SizeBytes: 1024 * 1024 * 200},
			Reason:      "upsert-test",
			Score:       float64(i+1) * 0.5,
			ForceDryRun: true,
			UpsertAudit: true,
		}
		if err := svc.QueueDeletion(job); err != nil {
			t.Fatalf("QueueDeletion returned error: %v", err)
		}
		// Wait for processing
		ch := bus.Subscribe()
		drainBatchEvent(t, ch, 15*time.Second)
		bus.Unsubscribe(ch)
	}

	// Verify: only 1 audit entry (upsert semantics)
	var count int64
	database.Model(&db.AuditLogEntry{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 audit entry (upsert), got %d", count)
	}

	// Verify the entry has the latest score
	var entry db.AuditLogEntry
	database.First(&entry)
	if entry.Score != 1.0 {
		t.Errorf("expected score 1.0 (latest upsert), got %f", entry.Score)
	}
	if entry.Action != db.ActionDryDelete {
		t.Errorf("expected action %q, got %q", db.ActionDryDelete, entry.Action)
	}
}

func TestDeletionService_UpsertAudit_False_AppendsMultiple(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)
	svc.SetDependencies(
		&mockSettingsReader{deletionsEnabled: false}, // dry-run mode
		&mockEngineStatsWriter{},
		&mockDeletionStatsWriter{},
	)

	svc.Start()
	defer svc.Stop()

	// Queue the same item twice with UpsertAudit=false — should produce 2 audit entries
	for i := 0; i < 2; i++ {
		svc.SignalBatchSize(1)
		job := DeleteJob{
			Client:      nil,
			Item:        integrations.MediaItem{Title: "Serenity", Type: "movie", SizeBytes: 1024 * 1024 * 100},
			Reason:      "append-test",
			Score:       float64(i+1) * 0.3,
			ForceDryRun: true,
			UpsertAudit: false,
		}
		if err := svc.QueueDeletion(job); err != nil {
			t.Fatalf("QueueDeletion returned error: %v", err)
		}
		ch := bus.Subscribe()
		drainBatchEvent(t, ch, 15*time.Second)
		bus.Unsubscribe(ch)
	}

	// Verify: 2 audit entries (append-only semantics)
	var count int64
	database.Model(&db.AuditLogEntry{}).Count(&count)
	if count != 2 {
		t.Errorf("expected 2 audit entries (append), got %d", count)
	}
}

func TestDeletionService_NilClient_DryRunSucceeds(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)
	svc.SetDependencies(
		&mockSettingsReader{deletionsEnabled: false}, // dry-run mode
		&mockEngineStatsWriter{},
		&mockDeletionStatsWriter{},
	)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	svc.Start()
	defer svc.Stop()

	svc.SignalBatchSize(1)

	// Queue a job with nil client in dry-run mode — should succeed
	job := DeleteJob{
		Client:      nil,
		Item:        integrations.MediaItem{Title: "Serenity", Type: "movie", SizeBytes: 1024 * 1024 * 100},
		Reason:      "nil-client-dry-run",
		Score:       0.65,
		ForceDryRun: true,
	}
	if err := svc.QueueDeletion(job); err != nil {
		t.Fatalf("QueueDeletion returned error: %v", err)
	}

	// Should get DeletionDryRunEvent (not a failure)
	deadline := time.After(15 * time.Second)
	gotDryRun := false
	for {
		select {
		case evt := <-ch:
			switch evt.(type) {
			case events.DeletionDryRunEvent:
				gotDryRun = true
			case events.DeletionFailedEvent:
				t.Fatal("Expected DeletionDryRunEvent but got DeletionFailedEvent — nil client should not fail dry-run")
			case events.DeletionBatchCompleteEvent:
				if !gotDryRun {
					t.Fatal("Batch completed without DeletionDryRunEvent")
				}
				// Verify audit entry was created
				var entry db.AuditLogEntry
				if err := database.First(&entry).Error; err != nil {
					t.Fatalf("Expected audit log entry: %v", err)
				}
				if entry.Action != db.ActionDryDelete {
					t.Errorf("expected action %q, got %q", db.ActionDryDelete, entry.Action)
				}
				return
			}
		case <-deadline:
			t.Fatal("timeout waiting for events")
		}
	}
}

func TestDeletionService_NilClient_ActualDeletion_Fails(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)
	svc.SetDependencies(
		&mockSettingsReader{deletionsEnabled: true}, // actual deletions enabled
		&mockEngineStatsWriter{},
		&mockDeletionStatsWriter{},
	)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	svc.Start()
	defer svc.Stop()

	svc.SignalBatchSize(1)

	// Queue a job with nil client AND deletions enabled, ForceDryRun=false
	// This should hit the nil-safety check and count as a failure
	job := DeleteJob{
		Client:      nil,
		Item:        integrations.MediaItem{Title: "Firefly", Type: "show", SizeBytes: 1024 * 1024 * 200},
		Reason:      "nil-client-actual",
		ForceDryRun: false,
	}
	if err := svc.QueueDeletion(job); err != nil {
		t.Fatalf("QueueDeletion returned error: %v", err)
	}

	bce := drainBatchEvent(t, ch, 15*time.Second)
	if bce.Succeeded != 0 {
		t.Errorf("expected Succeeded=0, got %d", bce.Succeeded)
	}
	if bce.Failed != 1 {
		t.Errorf("expected Failed=1, got %d", bce.Failed)
	}

	// Verify no audit entry was created (nil client fails before logging)
	var count int64
	database.Model(&db.AuditLogEntry{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 audit entries for nil client failure, got %d", count)
	}
}

func TestDeletionService_QueueDeletion_PublishesDeletionQueuedEvent(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)
	svc.SetDependencies(
		&mockSettingsReader{deletionsEnabled: false},
		&mockEngineStatsWriter{},
		&mockDeletionStatsWriter{},
	)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	// Don't start the worker — we only want to verify the enqueue event,
	// not the downstream processing events.
	job := DeleteJob{
		Client: &mockIntegration{},
		Item: integrations.MediaItem{
			Title:         "Serenity",
			Type:          "movie",
			SizeBytes:     1024 * 1024 * 100,
			IntegrationID: 7,
		},
		Reason: "test-queued-event",
	}
	if err := svc.QueueDeletion(job); err != nil {
		t.Fatalf("QueueDeletion returned error: %v", err)
	}

	// Read the first event — should be DeletionQueuedEvent
	deadline := time.After(2 * time.Second)
	select {
	case evt := <-ch:
		qe, ok := evt.(events.DeletionQueuedEvent)
		if !ok {
			t.Fatalf("expected DeletionQueuedEvent, got %T", evt)
		}
		if qe.MediaName != "Serenity" {
			t.Errorf("expected MediaName=%q, got %q", "Serenity", qe.MediaName)
		}
		if qe.MediaType != "movie" {
			t.Errorf("expected MediaType=%q, got %q", "movie", qe.MediaType)
		}
		if qe.SizeBytes != 1024*1024*100 {
			t.Errorf("expected SizeBytes=%d, got %d", 1024*1024*100, qe.SizeBytes)
		}
		if qe.IntegrationID != 7 {
			t.Errorf("expected IntegrationID=%d, got %d", 7, qe.IntegrationID)
		}
	case <-deadline:
		t.Fatal("timeout waiting for DeletionQueuedEvent")
	}
}
