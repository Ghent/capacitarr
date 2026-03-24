package services

import (
	"sync"
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
)

// mockApprovalQueueReader implements ApprovalQueueReader for testing.
type mockApprovalQueueReader struct {
	items map[string][]db.ApprovalQueueItem // status → items
}

func (m *mockApprovalQueueReader) ListQueue(status string, _ int, _ *uint) ([]db.ApprovalQueueItem, error) {
	return m.items[status], nil
}

// mockDeletionStateReader implements DeletionStateReader for testing.
type mockDeletionStateReader struct {
	current string
}

func (m *mockDeletionStateReader) CurrentlyDeleting() string {
	return m.current
}

func TestPreviewService_GetPreview_NoIntegrations(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewPreviewService(database, bus)

	// Wire real services as dependencies
	integrationSvc := NewIntegrationService(database, bus)
	settingsSvc := NewSettingsService(database, bus)
	rulesSvc := NewRulesService(database, bus)
	diskGroupSvc := NewDiskGroupService(database, bus)
	svc.SetDependencies(integrationSvc, settingsSvc, rulesSvc, diskGroupSvc)

	result, err := svc.GetPreview(false)
	if err != nil {
		t.Fatalf("GetPreview error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Items) != 0 {
		t.Errorf("expected 0 items with no integrations, got %d", len(result.Items))
	}
	if result.DiskContext != nil {
		t.Error("expected nil disk context with no disk groups")
	}
}

func TestPreviewService_GetPreview_WithDiskGroups(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewPreviewService(database, bus)

	// Wire real services as dependencies
	integrationSvc := NewIntegrationService(database, bus)
	settingsSvc := NewSettingsService(database, bus)
	rulesSvc := NewRulesService(database, bus)
	diskGroupSvc := NewDiskGroupService(database, bus)
	svc.SetDependencies(integrationSvc, settingsSvc, rulesSvc, diskGroupSvc)

	// Seed a disk group that is over threshold
	dg := db.DiskGroup{
		MountPath:    "/data",
		TotalBytes:   1000000000,
		UsedBytes:    900000000,
		TargetPct:    80.0,
		ThresholdPct: 85.0,
	}
	if err := database.Create(&dg).Error; err != nil {
		t.Fatalf("failed to create disk group: %v", err)
	}

	result, err := svc.GetPreview(false)
	if err != nil {
		t.Fatalf("GetPreview error: %v", err)
	}
	if result.DiskContext == nil {
		t.Fatal("expected non-nil disk context with disk groups")
	}
	if result.DiskContext.TotalBytes != 1000000000 {
		t.Errorf("expected totalBytes 1000000000, got %d", result.DiskContext.TotalBytes)
	}
	if result.DiskContext.BytesToFree <= 0 {
		t.Error("expected positive bytesToFree for over-threshold disk group")
	}
}

func TestPreviewService_SetDependencies(t *testing.T) {
	bus := newTestBus(t)
	database := setupTestDB(t)
	svc := NewPreviewService(database, bus)

	// Before wiring, fields should be nil
	if svc.integrations != nil {
		t.Error("expected nil integrations before SetDependencies")
	}

	integrationSvc := NewIntegrationService(database, bus)
	settingsSvc := NewSettingsService(database, bus)
	rulesSvc := NewRulesService(database, bus)
	diskGroupSvc := NewDiskGroupService(database, bus)
	svc.SetDependencies(integrationSvc, settingsSvc, rulesSvc, diskGroupSvc)

	if svc.integrations == nil {
		t.Error("expected non-nil integrations after SetDependencies")
	}
	if svc.preferences == nil {
		t.Error("expected non-nil preferences after SetDependencies")
	}
	if svc.rules == nil {
		t.Error("expected non-nil rules after SetDependencies")
	}
	if svc.diskGroups == nil {
		t.Error("expected non-nil diskGroups after SetDependencies")
	}
}

func TestPreviewService_SetPreviewCache(t *testing.T) {
	bus := newTestBus(t)
	database := setupTestDB(t)
	svc := NewPreviewService(database, bus)

	diskGroupSvc := NewDiskGroupService(database, bus)
	settingsSvc := NewSettingsService(database, bus)
	svc.SetDependencies(nil, settingsSvc, nil, diskGroupSvc)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	items := []integrations.MediaItem{
		{Title: "Firefly", SizeBytes: 1000},
		{Title: "Serenity", SizeBytes: 2000},
	}
	prefs := db.PreferenceSet{}
	var rules []db.CustomRule

	svc.SetPreviewCache(items, prefs, map[string]int{}, rules, &engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}})

	// Verify cache is populated
	result, err := svc.GetPreview(false)
	if err != nil {
		t.Fatalf("GetPreview error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Errorf("expected 2 cached items, got %d", len(result.Items))
	}

	// Verify PreviewUpdatedEvent was published
	select {
	case evt := <-ch:
		if evt.EventType() != "preview_updated" {
			t.Errorf("expected preview_updated event, got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for preview_updated event")
	}
}

func TestPreviewService_CacheHit(t *testing.T) {
	bus := newTestBus(t)
	database := setupTestDB(t)
	svc := NewPreviewService(database, bus)

	diskGroupSvc := NewDiskGroupService(database, bus)
	settingsSvc := NewSettingsService(database, bus)
	svc.SetDependencies(nil, settingsSvc, nil, diskGroupSvc)

	// Populate cache
	items := []integrations.MediaItem{{Title: "Firefly"}}
	svc.SetPreviewCache(items, db.PreferenceSet{}, map[string]int{}, nil, &engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}})

	// Should return cached result without error
	result, err := svc.GetPreview(false)
	if err != nil {
		t.Fatalf("GetPreview error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Errorf("expected 1 cached item, got %d", len(result.Items))
	}
}

func TestPreviewService_InvalidatePreviewCache(t *testing.T) {
	bus := newTestBus(t)
	database := setupTestDB(t)
	svc := NewPreviewService(database, bus)

	diskGroupSvc := NewDiskGroupService(database, bus)
	settingsSvc := NewSettingsService(database, bus)
	integrationSvc := NewIntegrationService(database, bus)
	rulesSvc := NewRulesService(database, bus)
	svc.SetDependencies(integrationSvc, settingsSvc, rulesSvc, diskGroupSvc)

	// Populate cache
	items := []integrations.MediaItem{{Title: "Firefly"}}
	svc.SetPreviewCache(items, db.PreferenceSet{}, map[string]int{}, nil, &engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}})

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	// Invalidate
	svc.InvalidatePreviewCache("test_reason")

	// Cache should be cleared — GetPreview should do fresh computation
	result, err := svc.GetPreview(false)
	if err != nil {
		t.Fatalf("GetPreview after invalidation error: %v", err)
	}
	// Fresh computation with no integrations returns 0 items
	if len(result.Items) != 0 {
		t.Errorf("expected 0 items after invalidation (fresh compute), got %d", len(result.Items))
	}

	// Verify PreviewInvalidatedEvent was published
	select {
	case evt := <-ch:
		if evt.EventType() != "preview_invalidated" {
			t.Errorf("expected preview_invalidated event, got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for preview_invalidated event")
	}
}

func TestPreviewService_ForceBypassesCache(t *testing.T) {
	bus := newTestBus(t)
	database := setupTestDB(t)
	svc := NewPreviewService(database, bus)

	diskGroupSvc := NewDiskGroupService(database, bus)
	settingsSvc := NewSettingsService(database, bus)
	integrationSvc := NewIntegrationService(database, bus)
	rulesSvc := NewRulesService(database, bus)
	svc.SetDependencies(integrationSvc, settingsSvc, rulesSvc, diskGroupSvc)

	// Populate cache with items
	items := []integrations.MediaItem{{Title: "Firefly"}, {Title: "Serenity"}}
	svc.SetPreviewCache(items, db.PreferenceSet{}, map[string]int{}, nil, &engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}})

	// Force=true should bypass cache and recompute from scratch (0 items — no integrations)
	result, err := svc.GetPreview(true)
	if err != nil {
		t.Fatalf("GetPreview force error: %v", err)
	}
	if len(result.Items) != 0 {
		t.Errorf("expected 0 items with force=true (no integrations), got %d", len(result.Items))
	}
}

func TestPreviewService_SingleflightCoalesces(t *testing.T) {
	bus := newTestBus(t)
	database := setupTestDB(t)
	svc := NewPreviewService(database, bus)

	diskGroupSvc := NewDiskGroupService(database, bus)
	settingsSvc := NewSettingsService(database, bus)
	integrationSvc := NewIntegrationService(database, bus)
	rulesSvc := NewRulesService(database, bus)
	svc.SetDependencies(integrationSvc, settingsSvc, rulesSvc, diskGroupSvc)

	// Launch multiple concurrent GetPreview calls — singleflight should coalesce
	const goroutines = 5
	var wg sync.WaitGroup
	results := make([]*PreviewResult, goroutines)
	errs := make([]error, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = svc.GetPreview(false)
		}(i)
	}
	wg.Wait()

	for i := 0; i < goroutines; i++ {
		if errs[i] != nil {
			t.Errorf("goroutine %d error: %v", i, errs[i])
		}
		if results[i] == nil {
			t.Errorf("goroutine %d got nil result", i)
		}
	}
}

func TestPreviewService_CacheInvalidationOnEvents(t *testing.T) {
	bus := newTestBus(t)
	database := setupTestDB(t)
	svc := NewPreviewService(database, bus)

	diskGroupSvc := NewDiskGroupService(database, bus)
	settingsSvc := NewSettingsService(database, bus)
	integrationSvc := NewIntegrationService(database, bus)
	rulesSvc := NewRulesService(database, bus)
	svc.SetDependencies(integrationSvc, settingsSvc, rulesSvc, diskGroupSvc)

	svc.StartCacheInvalidation()
	defer svc.Stop()

	// Populate cache
	items := []integrations.MediaItem{{Title: "Firefly"}}
	svc.SetPreviewCache(items, db.PreferenceSet{}, map[string]int{}, nil, &engine.EvaluationContext{ActiveIntegrationTypes: map[integrations.IntegrationType]bool{}})

	// Verify cache is populated
	svc.previewMu.RLock()
	hasCacheBefore := svc.previewCache != nil
	svc.previewMu.RUnlock()
	if !hasCacheBefore {
		t.Fatal("expected cache to be populated before event")
	}

	// Publish a settings changed event
	bus.Publish(events.SettingsChangedEvent{})

	// Wait briefly for the invalidation goroutine to process the event
	time.Sleep(100 * time.Millisecond)

	// Cache should be cleared
	svc.previewMu.RLock()
	hasCacheAfter := svc.previewCache != nil
	svc.previewMu.RUnlock()
	if hasCacheAfter {
		t.Error("expected cache to be cleared after SettingsChangedEvent")
	}
}

func TestPreviewService_EnrichWithQueueStatus_Pending(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewPreviewService(database, bus)

	approvalMock := &mockApprovalQueueReader{
		items: map[string][]db.ApprovalQueueItem{
			db.StatusPending: {
				{ID: 1, MediaName: "Firefly", MediaType: "show", Status: db.StatusPending},
			},
			db.StatusApproved: {},
		},
	}
	deletionMock := &mockDeletionStateReader{current: ""}
	svc.SetQueueDependencies(approvalMock, deletionMock)

	items := []engine.EvaluatedItem{
		{Item: integrations.MediaItem{Title: "Firefly", Type: integrations.MediaTypeShow}},
		{Item: integrations.MediaItem{Title: "Serenity", Type: integrations.MediaTypeMovie}},
	}

	svc.EnrichWithQueueStatus(items)

	if items[0].QueueStatus != "pending" {
		t.Errorf("expected 'pending' for Firefly, got %q", items[0].QueueStatus)
	}
	if items[0].ApprovalQueueID == nil || *items[0].ApprovalQueueID != 1 {
		t.Errorf("expected ApprovalQueueID=1 for Firefly, got %v", items[0].ApprovalQueueID)
	}
	if items[1].QueueStatus != "" {
		t.Errorf("expected empty queueStatus for Serenity, got %q", items[1].QueueStatus)
	}
	if items[1].ApprovalQueueID != nil {
		t.Errorf("expected nil ApprovalQueueID for Serenity, got %v", items[1].ApprovalQueueID)
	}
}

func TestPreviewService_EnrichWithQueueStatus_Approved(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewPreviewService(database, bus)

	approvalMock := &mockApprovalQueueReader{
		items: map[string][]db.ApprovalQueueItem{
			db.StatusPending: {},
			db.StatusApproved: {
				{ID: 2, MediaName: "Serenity", MediaType: "movie", Status: db.StatusApproved},
			},
		},
	}
	deletionMock := &mockDeletionStateReader{current: ""}
	svc.SetQueueDependencies(approvalMock, deletionMock)

	items := []engine.EvaluatedItem{
		{Item: integrations.MediaItem{Title: "Serenity", Type: integrations.MediaTypeMovie}},
	}

	svc.EnrichWithQueueStatus(items)

	if items[0].QueueStatus != "approved" {
		t.Errorf("expected 'approved' for Serenity, got %q", items[0].QueueStatus)
	}
	if items[0].ApprovalQueueID == nil || *items[0].ApprovalQueueID != 2 {
		t.Errorf("expected ApprovalQueueID=2 for Serenity, got %v", items[0].ApprovalQueueID)
	}
}

func TestPreviewService_EnrichWithQueueStatus_UserInitiated(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewPreviewService(database, bus)

	approvalMock := &mockApprovalQueueReader{
		items: map[string][]db.ApprovalQueueItem{
			db.StatusPending: {},
			db.StatusApproved: {
				{ID: 3, MediaName: "Firefly", MediaType: "show", Status: db.StatusApproved, UserInitiated: true},
			},
		},
	}
	deletionMock := &mockDeletionStateReader{current: ""}
	svc.SetQueueDependencies(approvalMock, deletionMock)

	items := []engine.EvaluatedItem{
		{Item: integrations.MediaItem{Title: "Firefly", Type: integrations.MediaTypeShow}},
	}

	svc.EnrichWithQueueStatus(items)

	if items[0].QueueStatus != "user_initiated" {
		t.Errorf("expected 'user_initiated' for Firefly, got %q", items[0].QueueStatus)
	}
}

func TestPreviewService_EnrichWithQueueStatus_Deleting(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewPreviewService(database, bus)

	approvalMock := &mockApprovalQueueReader{
		items: map[string][]db.ApprovalQueueItem{
			db.StatusPending: {},
			db.StatusApproved: {
				{ID: 4, MediaName: "Serenity", MediaType: "movie", Status: db.StatusApproved},
			},
		},
	}
	deletionMock := &mockDeletionStateReader{current: "Serenity"}
	svc.SetQueueDependencies(approvalMock, deletionMock)

	items := []engine.EvaluatedItem{
		{Item: integrations.MediaItem{Title: "Serenity", Type: integrations.MediaTypeMovie}},
	}

	svc.EnrichWithQueueStatus(items)

	if items[0].QueueStatus != "deleting" {
		t.Errorf("expected 'deleting' for Serenity, got %q", items[0].QueueStatus)
	}
	if items[0].ApprovalQueueID == nil || *items[0].ApprovalQueueID != 4 {
		t.Errorf("expected ApprovalQueueID=4 for Serenity, got %v", items[0].ApprovalQueueID)
	}
}

func TestPreviewService_EnrichWithQueueStatus_NotInQueue(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewPreviewService(database, bus)

	approvalMock := &mockApprovalQueueReader{
		items: map[string][]db.ApprovalQueueItem{
			db.StatusPending:  {},
			db.StatusApproved: {},
		},
	}
	deletionMock := &mockDeletionStateReader{current: ""}
	svc.SetQueueDependencies(approvalMock, deletionMock)

	items := []engine.EvaluatedItem{
		{Item: integrations.MediaItem{Title: "Firefly", Type: integrations.MediaTypeShow}},
	}

	svc.EnrichWithQueueStatus(items)

	if items[0].QueueStatus != "" {
		t.Errorf("expected empty queueStatus for Firefly, got %q", items[0].QueueStatus)
	}
	if items[0].ApprovalQueueID != nil {
		t.Errorf("expected nil ApprovalQueueID for Firefly, got %v", items[0].ApprovalQueueID)
	}
}

func TestPreviewService_EnrichWithQueueStatus_NilDependencies(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewPreviewService(database, bus)

	// Do not set queue dependencies — should be a no-op
	items := []engine.EvaluatedItem{
		{Item: integrations.MediaItem{Title: "Firefly", Type: integrations.MediaTypeShow}},
	}

	svc.EnrichWithQueueStatus(items)

	if items[0].QueueStatus != "" {
		t.Errorf("expected empty queueStatus with nil dependencies, got %q", items[0].QueueStatus)
	}
}
