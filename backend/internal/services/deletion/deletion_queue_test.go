package deletion

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
)

func fillDeletionQueue(svc *DeletionService, n int) {
	svc.queuedMu.Lock()
	defer svc.queuedMu.Unlock()
	svc.queuedItems = make([]deleteJob, n)
	for i := range svc.queuedItems {
		svc.queuedItems[i] = deleteJob{
			Item: integrations.MediaItem{
				Title: fmt.Sprintf("queued-%d", i),
				Type:  integrations.MediaTypeMovie,
			},
		}
	}
}

func drainQueueFullEvent(t *testing.T, ch chan events.Event, wait time.Duration) *events.DeletionQueueFullEvent {
	t.Helper()
	deadline := time.After(wait)
	for {
		select {
		case evt := <-ch:
			if qe, ok := evt.(events.DeletionQueueFullEvent); ok {
				return &qe
			}
		case <-deadline:
			return nil
		}
	}
}

func TestEnqueue_QueueFullPublishesOnceAndCounts(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := newTestDeletionService(bus, NewAuditLogService(database))
	svc.SetDependencies(DeletionDeps{
		Settings:      &mockSettingsReader{deletionsEnabled: true, deletionQueueDelaySeconds: 300},
		Engine:        &mockEngineStatsWriter{},
		Metrics:       &mockDeletionStatsWriter{},
		Approval:      &mockApprovalReturner{},
		Snoozer:       &mockApprovalSnoozer{},
		DiskGroups:    &mockDiskGroupModeReader{mode: "auto"},
		Clients:       &mockClientResolver{},
		SunsetCleaner: &mockSunsetQueueCleaner{},
	})

	fillDeletionQueue(svc, 500)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	err := svc.enqueue(deleteJob{
		Item: integrations.MediaItem{Title: "overflow-1", Type: integrations.MediaTypeMovie},
	})
	if !errors.Is(err, ErrDeletionQueueFull) {
		t.Fatalf("expected ErrDeletionQueueFull, got %v", err)
	}
	if got := svc.QueueFullRejections(); got != 1 {
		t.Errorf("QueueFullRejections = %d, want 1", got)
	}

	first := drainQueueFullEvent(t, ch, testEventTimeout)
	if first == nil {
		t.Fatal("expected DeletionQueueFullEvent on first overflow")
	}
	if first.QueueSize != 500 {
		t.Errorf("QueueSize = %d, want 500", first.QueueSize)
	}
	if first.MediaName != "overflow-1" {
		t.Errorf("MediaName = %q, want overflow-1", first.MediaName)
	}

	err = svc.enqueue(deleteJob{
		Item: integrations.MediaItem{Title: "overflow-2", Type: integrations.MediaTypeMovie},
	})
	if !errors.Is(err, ErrDeletionQueueFull) {
		t.Fatalf("expected second ErrDeletionQueueFull, got %v", err)
	}
	if got := svc.QueueFullRejections(); got != 2 {
		t.Errorf("QueueFullRejections = %d, want 2", got)
	}
	if second := drainQueueFullEvent(t, ch, 50*time.Millisecond); second != nil {
		t.Fatal("did not expect a second DeletionQueueFullEvent in the same burst")
	}

	svc.queuedMu.Lock()
	svc.queuedItems = svc.queuedItems[:499]
	svc.queuedMu.Unlock()

	if err := svc.enqueue(deleteJob{
		Item: integrations.MediaItem{Title: "after-space", Type: integrations.MediaTypeMovie},
	}); err != nil {
		t.Fatalf("enqueue after space: %v", err)
	}
	if svc.queueFullSignaled.Load() {
		t.Error("queueFullSignaled should reset after a successful enqueue")
	}

	fillDeletionQueue(svc, 500)
	err = svc.enqueue(deleteJob{
		Item: integrations.MediaItem{Title: "overflow-3", Type: integrations.MediaTypeMovie},
	})
	if !errors.Is(err, ErrDeletionQueueFull) {
		t.Fatalf("expected overflow after reset, got %v", err)
	}
	replay := drainQueueFullEvent(t, ch, testEventTimeout)
	if replay == nil {
		t.Fatal("expected DeletionQueueFullEvent after a successful enqueue reset the burst")
	}
	if replay.MediaName != "overflow-3" {
		t.Errorf("MediaName = %q, want overflow-3", replay.MediaName)
	}
}
