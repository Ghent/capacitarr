package services

import (
	"errors"
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
)

// mockApprovalUpserter implements ApprovalReturnerUpserter for intake tests.
type mockApprovalUpserter struct {
	items []db.ApprovalQueueItem
}

func (m *mockApprovalUpserter) UpsertPending(item db.ApprovalQueueItem) (bool, error) {
	m.items = append(m.items, item)
	return true, nil
}

// ---------------------------------------------------------------------------
// QueueFromEngine tests
// ---------------------------------------------------------------------------

func TestQueueFromEngine_ConvertsRequest(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)
	svc.SetDependencies(DeletionDeps{
		Settings:      &mockSettingsReader{deletionsEnabled: false, deletionQueueDelaySeconds: 1},
		Engine:        &mockEngineStatsWriter{},
		Metrics:       &mockDeletionStatsWriter{},
		Approval:      &mockApprovalReturner{},
		Snoozer:       &mockApprovalSnoozer{},
		DiskGroups:    &mockDiskGroupModeReader{},
		Clients:       &mockClientResolver{},
		SunsetCleaner: &mockSunsetQueueCleaner{},
	})

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	err := svc.QueueFromEngine(EngineDeleteRequest{
		Client:             &mockIntegration{},
		Item:               integrations.MediaItem{Title: "Firefly", Type: "show", SizeBytes: 500},
		Score:              0.85,
		Factors:            []engine.ScoreFactor{{Name: "size", RawScore: 0.5}},
		RunStatsID:         42,
		DiskGroupID:        7,
		CollectionGroup:    "SciFi Collection",
		AddImportExclusion: true,
		UpsertAudit:        true,
	})
	if err != nil {
		t.Fatalf("QueueFromEngine returned error: %v", err)
	}

	// Verify it landed in the queue with correct fields
	if svc.QueueLen() != 1 {
		t.Fatalf("expected queue length 1, got %d", svc.QueueLen())
	}

	// Verify the event was published
	deadline := drainQueuedEvent(t, ch)
	if deadline == nil {
		t.Fatal("expected DeletionQueuedEvent, got timeout")
	}
	if deadline.MediaName != "Firefly" {
		t.Errorf("expected media name 'Firefly', got %q", deadline.MediaName)
	}
}

func TestQueueFromEngine_SetsTriggerAndMode(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)
	svc.SetDependencies(DeletionDeps{
		Settings:      &mockSettingsReader{deletionsEnabled: false, deletionQueueDelaySeconds: 300},
		Engine:        &mockEngineStatsWriter{},
		Metrics:       &mockDeletionStatsWriter{},
		Approval:      &mockApprovalReturner{},
		Snoozer:       &mockApprovalSnoozer{},
		DiskGroups:    &mockDiskGroupModeReader{},
		Clients:       &mockClientResolver{},
		SunsetCleaner: &mockSunsetQueueCleaner{},
	})

	_ = svc.QueueFromEngine(EngineDeleteRequest{
		Client:      &mockIntegration{},
		Item:        integrations.MediaItem{Title: "Serenity", Type: "movie", SizeBytes: 100},
		DiskGroupID: 3,
	})

	// Peek at the internal queue to verify trigger and mode
	svc.queuedMu.Lock()
	if len(svc.queuedItems) != 1 {
		svc.queuedMu.Unlock()
		t.Fatal("expected 1 queued item")
	}
	job := svc.queuedItems[0]
	svc.queuedMu.Unlock()

	if job.Trigger != db.TriggerEngine {
		t.Errorf("expected trigger %q, got %q", db.TriggerEngine, job.Trigger)
	}
	if job.EnqueuedMode != db.ModeAuto {
		t.Errorf("expected enqueued mode %q, got %q", db.ModeAuto, job.EnqueuedMode)
	}
	if job.DiskGroupID == nil || *job.DiskGroupID != 3 {
		t.Error("expected DiskGroupID to be pointer to 3")
	}
}

// ---------------------------------------------------------------------------
// QueueFromApproval tests
// ---------------------------------------------------------------------------

func TestQueueFromApproval_FullResolution(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)

	dgID := uint(5)
	svc.SetDependencies(DeletionDeps{
		Settings:      &mockSettingsReader{deletionsEnabled: true, executionMode: db.ModeAuto, deletionQueueDelaySeconds: 300},
		Engine:        &mockEngineStatsWriter{},
		Metrics:       &mockDeletionStatsWriter{},
		Approval:      &mockApprovalReturner{},
		Snoozer:       &mockApprovalSnoozer{},
		DiskGroups:    &mockDiskGroupModeReader{mode: db.ModeAuto},
		Clients:       &mockClientResolver{deleter: &mockIntegration{}, config: &db.IntegrationConfig{AddImportExclusion: true}},
		SunsetCleaner: &mockSunsetQueueCleaner{},
	})

	item := &db.ApprovalQueueItem{
		ID:              10,
		MediaName:       "Firefly",
		MediaType:       "show",
		ScoreDetails:    `[{"name":"size","score":0.5}]`,
		SizeBytes:       1024,
		Score:           0.75,
		IntegrationID:   1,
		ExternalID:      "ext-1",
		DiskGroupID:     &dgID,
		CollectionGroup: "SciFi",
	}

	err := svc.QueueFromApproval(item)
	if err != nil {
		t.Fatalf("QueueFromApproval returned error: %v", err)
	}

	// Verify the job was queued correctly
	svc.queuedMu.Lock()
	if len(svc.queuedItems) != 1 {
		svc.queuedMu.Unlock()
		t.Fatal("expected 1 queued item")
	}
	job := svc.queuedItems[0]
	svc.queuedMu.Unlock()

	if job.Trigger != db.TriggerApproval {
		t.Errorf("expected trigger %q, got %q", db.TriggerApproval, job.Trigger)
	}
	if job.ApprovalEntryID != 10 {
		t.Errorf("expected ApprovalEntryID 10, got %d", job.ApprovalEntryID)
	}
	if job.CollectionGroup != "SciFi" {
		t.Errorf("expected CollectionGroup 'SciFi', got %q", job.CollectionGroup)
	}
	if job.DiskGroupID == nil || *job.DiskGroupID != 5 {
		t.Error("expected DiskGroupID pointer to 5")
	}
	if job.AddImportExclusion != true {
		t.Error("expected AddImportExclusion true")
	}
	if job.ForceDryRun {
		t.Error("expected ForceDryRun false (deletions enabled, mode auto)")
	}
}

func TestQueueFromApproval_ClientResolutionFailure(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)

	svc.SetDependencies(DeletionDeps{
		Settings:      &mockSettingsReader{deletionsEnabled: true, deletionQueueDelaySeconds: 300},
		Engine:        &mockEngineStatsWriter{},
		Metrics:       &mockDeletionStatsWriter{},
		Approval:      &mockApprovalReturner{},
		Snoozer:       &mockApprovalSnoozer{},
		DiskGroups:    &mockDiskGroupModeReader{},
		Clients:       &mockClientResolver{err: errors.New("integration not found")},
		SunsetCleaner: &mockSunsetQueueCleaner{},
	})

	item := &db.ApprovalQueueItem{ID: 1, IntegrationID: 99}
	err := svc.QueueFromApproval(item)
	if err == nil {
		t.Fatal("expected error when client resolution fails")
	}
	if svc.QueueLen() != 0 {
		t.Error("expected empty queue after failed resolution")
	}
}

// ---------------------------------------------------------------------------
// QueueFromSunset tests
// ---------------------------------------------------------------------------

func TestQueueFromSunset_FullResolution(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)

	svc.SetDependencies(DeletionDeps{
		Settings:      &mockSettingsReader{deletionsEnabled: true, deletionQueueDelaySeconds: 300},
		Engine:        &mockEngineStatsWriter{},
		Metrics:       &mockDeletionStatsWriter{},
		Approval:      &mockApprovalReturner{},
		Snoozer:       &mockApprovalSnoozer{},
		DiskGroups:    &mockDiskGroupModeReader{},
		Clients:       &mockClientResolver{deleter: &mockIntegration{}, config: &db.IntegrationConfig{AddImportExclusion: false}},
		SunsetCleaner: &mockSunsetQueueCleaner{},
	})

	item := &db.SunsetQueueItem{
		ID:              20,
		MediaName:       "Serenity",
		MediaType:       "movie",
		ScoreDetails:    `[{"name":"age","score":0.9}]`,
		SizeBytes:       2048,
		Score:           0.9,
		IntegrationID:   2,
		ExternalID:      "ext-2",
		DiskGroupID:     8,
		CollectionGroup: "Action",
	}

	err := svc.QueueFromSunset(item)
	if err != nil {
		t.Fatalf("QueueFromSunset returned error: %v", err)
	}

	svc.queuedMu.Lock()
	if len(svc.queuedItems) != 1 {
		svc.queuedMu.Unlock()
		t.Fatal("expected 1 queued item")
	}
	job := svc.queuedItems[0]
	svc.queuedMu.Unlock()

	if job.Trigger != db.TriggerEngine {
		t.Errorf("expected trigger %q, got %q", db.TriggerEngine, job.Trigger)
	}
	if job.EnqueuedMode != db.ModeSunset {
		t.Errorf("expected enqueued mode %q, got %q", db.ModeSunset, job.EnqueuedMode)
	}
	if job.SunsetQueueItemID != 20 {
		t.Errorf("expected SunsetQueueItemID 20, got %d", job.SunsetQueueItemID)
	}
	if job.DiskGroupID == nil || *job.DiskGroupID != 8 {
		t.Error("expected DiskGroupID pointer to 8")
	}
	if job.AddImportExclusion != false {
		t.Error("expected AddImportExclusion false (from integration config)")
	}
	if job.CollectionGroup != "Action" {
		t.Errorf("expected CollectionGroup 'Action', got %q", job.CollectionGroup)
	}
}

func TestQueueFromSunset_ClientResolutionFailure(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)

	svc.SetDependencies(DeletionDeps{
		Settings:      &mockSettingsReader{deletionsEnabled: true, deletionQueueDelaySeconds: 300},
		Engine:        &mockEngineStatsWriter{},
		Metrics:       &mockDeletionStatsWriter{},
		Approval:      &mockApprovalReturner{},
		Snoozer:       &mockApprovalSnoozer{},
		DiskGroups:    &mockDiskGroupModeReader{},
		Clients:       &mockClientResolver{err: errors.New("integration gone")},
		SunsetCleaner: &mockSunsetQueueCleaner{},
	})

	item := &db.SunsetQueueItem{ID: 30, IntegrationID: 99}
	err := svc.QueueFromSunset(item)
	if err == nil {
		t.Fatal("expected error when client resolution fails")
	}
}

// ---------------------------------------------------------------------------
// QueueManual tests
// ---------------------------------------------------------------------------

func TestQueueManual_QueuesInAutoMode(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)

	dgID := uint(3)
	svc.SetDependencies(DeletionDeps{
		Settings:      &mockSettingsReader{deletionsEnabled: true, executionMode: db.ModeAuto, deletionQueueDelaySeconds: 300},
		Engine:        &mockEngineStatsWriter{},
		Metrics:       &mockDeletionStatsWriter{},
		Approval:      &mockApprovalReturner{},
		Snoozer:       &mockApprovalSnoozer{},
		DiskGroups:    &mockDiskGroupModeReader{mode: db.ModeAuto, diskGroupID: &dgID},
		Clients:       &mockClientResolver{deleter: &mockIntegration{}},
		SunsetCleaner: &mockSunsetQueueCleaner{},
	})

	upserter := &mockApprovalUpserter{}
	result, err := svc.QueueManual([]ManualDeleteRequest{
		{MediaName: "Firefly", MediaType: "show", IntegrationID: 1, SizeBytes: 500, Score: 0.8, ScoreDetails: `[]`},
		{MediaName: "Serenity", MediaType: "movie", IntegrationID: 1, SizeBytes: 1000, Score: 0.9, ScoreDetails: `[]`},
	}, upserter)
	if err != nil {
		t.Fatalf("QueueManual returned error: %v", err)
	}

	if result.Queued != 2 {
		t.Errorf("expected 2 queued, got %d", result.Queued)
	}
	if result.Total != 2 {
		t.Errorf("expected 2 total, got %d", result.Total)
	}
	if svc.QueueLen() != 2 {
		t.Errorf("expected queue length 2, got %d", svc.QueueLen())
	}
	if len(upserter.items) != 0 {
		t.Errorf("expected 0 approval upserts, got %d", len(upserter.items))
	}
}

func TestQueueManual_RoutesToApprovalQueue(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)

	dgID := uint(5)
	svc.SetDependencies(DeletionDeps{
		Settings:      &mockSettingsReader{deletionsEnabled: true, executionMode: db.ModeApproval, deletionQueueDelaySeconds: 300},
		Engine:        &mockEngineStatsWriter{},
		Metrics:       &mockDeletionStatsWriter{},
		Approval:      &mockApprovalReturner{},
		Snoozer:       &mockApprovalSnoozer{},
		DiskGroups:    &mockDiskGroupModeReader{mode: db.ModeApproval, diskGroupID: &dgID},
		Clients:       &mockClientResolver{deleter: &mockIntegration{}},
		SunsetCleaner: &mockSunsetQueueCleaner{},
	})

	upserter := &mockApprovalUpserter{}
	result, err := svc.QueueManual([]ManualDeleteRequest{
		{MediaName: "Firefly", MediaType: "show", IntegrationID: 1, SizeBytes: 500, Score: 0.8},
	}, upserter)
	if err != nil {
		t.Fatalf("QueueManual returned error: %v", err)
	}

	// Item should have been routed to approval, not deletion queue
	if svc.QueueLen() != 0 {
		t.Errorf("expected empty deletion queue, got %d items", svc.QueueLen())
	}
	if len(upserter.items) != 1 {
		t.Fatalf("expected 1 approval upsert, got %d", len(upserter.items))
	}
	if upserter.items[0].MediaName != "Firefly" {
		t.Errorf("expected approval item 'Firefly', got %q", upserter.items[0].MediaName)
	}
	if !upserter.items[0].UserInitiated {
		t.Error("expected UserInitiated=true on approval item")
	}
	if result.Queued != 1 {
		t.Errorf("expected Queued=1 (approval counts as queued), got %d", result.Queued)
	}
}

func TestQueueManual_ClientFailureSkipsItem(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	auditLog := NewAuditLogService(database)
	svc := NewDeletionService(bus, auditLog)

	dgID := uint(3)
	svc.SetDependencies(DeletionDeps{
		Settings:      &mockSettingsReader{deletionsEnabled: true, executionMode: db.ModeAuto, deletionQueueDelaySeconds: 300},
		Engine:        &mockEngineStatsWriter{},
		Metrics:       &mockDeletionStatsWriter{},
		Approval:      &mockApprovalReturner{},
		Snoozer:       &mockApprovalSnoozer{},
		DiskGroups:    &mockDiskGroupModeReader{mode: db.ModeAuto, diskGroupID: &dgID},
		Clients:       &mockClientResolver{err: errors.New("integration gone")},
		SunsetCleaner: &mockSunsetQueueCleaner{},
	})

	upserter := &mockApprovalUpserter{}
	result, err := svc.QueueManual([]ManualDeleteRequest{
		{MediaName: "Firefly", MediaType: "show", IntegrationID: 99},
	}, upserter)
	if err != nil {
		t.Fatalf("QueueManual returned error: %v", err)
	}

	if result.Queued != 0 {
		t.Errorf("expected 0 queued (client failed), got %d", result.Queued)
	}
	if svc.QueueLen() != 0 {
		t.Error("expected empty queue after client failure")
	}
}

// ---------------------------------------------------------------------------
// Helper: drainQueuedEvent
// ---------------------------------------------------------------------------

func drainQueuedEvent(t *testing.T, ch chan events.Event) *events.DeletionQueuedEvent {
	t.Helper()
	deadline := testEventTimeout
	timer := make(chan struct{})
	go func() {
		<-time.After(deadline)
		close(timer)
	}()
	for {
		select {
		case evt := <-ch:
			if qe, ok := evt.(events.DeletionQueuedEvent); ok {
				return &qe
			}
		case <-timer:
			return nil
		}
	}
}
