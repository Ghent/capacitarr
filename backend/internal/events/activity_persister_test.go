package events

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"capacitarr/internal/db"
)

// mockActivityWriter records all CreateActivity calls for testing.
type mockActivityWriter struct {
	mu      sync.Mutex
	entries []activityEntry
}

type activityEntry struct {
	EventType string
	Message   string
	Metadata  string
}

func (m *mockActivityWriter) CreateActivity(eventType, message, metadata string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, activityEntry{
		EventType: eventType,
		Message:   message,
		Metadata:  metadata,
	})
	return nil
}

func (m *mockActivityWriter) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.entries)
}

func (m *mockActivityWriter) getAll() []activityEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]activityEntry, len(m.entries))
	copy(result, m.entries)
	return result
}

func TestActivityPersister_PersistsSingleEvent(t *testing.T) {
	writer := &mockActivityWriter{}
	bus := NewEventBus()

	persister := NewActivityPersister(writer, bus)
	persister.Start()

	bus.Publish(EngineStartEvent{ExecutionMode: db.ModeDryRun})

	// Give the persister time to write
	time.Sleep(100 * time.Millisecond)

	persister.Stop()

	entries := writer.getAll()
	if len(entries) != 1 {
		t.Fatalf("expected 1 activity event, got %d", len(entries))
	}

	evt := entries[0]
	if evt.EventType != "engine_start" {
		t.Errorf("expected event type 'engine_start', got %q", evt.EventType)
	}
	if evt.Message != "Engine run started in dry-run mode" {
		t.Errorf("unexpected message: %q", evt.Message)
	}

	// Verify metadata contains JSON-encoded event
	var meta map[string]any
	if err := json.Unmarshal([]byte(evt.Metadata), &meta); err != nil {
		t.Fatalf("failed to parse metadata JSON: %v", err)
	}
	if meta["executionMode"] != db.ModeDryRun {
		t.Errorf("expected executionMode 'dry-run' in metadata, got %v", meta["executionMode"])
	}
}

func TestActivityPersister_PersistsMultipleEvents(t *testing.T) {
	writer := &mockActivityWriter{}
	bus := NewEventBus()

	persister := NewActivityPersister(writer, bus)
	persister.Start()

	bus.Publish(EngineStartEvent{ExecutionMode: db.ModeApproval})
	bus.Publish(EngineCompleteEvent{Evaluated: 50, Flagged: 5})
	bus.Publish(LoginEvent{Username: "admin"})

	time.Sleep(100 * time.Millisecond)
	persister.Stop()

	if writer.count() != 3 {
		t.Fatalf("expected 3 activity events, got %d", writer.count())
	}

	// Verify ordering
	entries := writer.getAll()
	expectedTypes := []string{"engine_start", "engine_complete", "login"}
	for i, expected := range expectedTypes {
		if entries[i].EventType != expected {
			t.Errorf("event %d: expected type %q, got %q", i, expected, entries[i].EventType)
		}
	}
}

func TestActivityPersister_StopDrainsRemaining(t *testing.T) {
	writer := &mockActivityWriter{}
	bus := NewEventBus()

	persister := NewActivityPersister(writer, bus)
	persister.Start()

	// Publish events and immediately stop — Stop should drain remaining events
	bus.Publish(ServerStartedEvent{Version: "1.0.0"})

	time.Sleep(50 * time.Millisecond)
	persister.Stop()

	if writer.count() != 1 {
		t.Fatalf("expected 1 activity event after stop, got %d", writer.count())
	}
}

func TestActivityPersister_MetadataContainsFullEvent(t *testing.T) {
	writer := &mockActivityWriter{}
	bus := NewEventBus()

	persister := NewActivityPersister(writer, bus)
	persister.Start()

	bus.Publish(DeletionSuccessEvent{
		MediaName:     "Serenity",
		MediaType:     "movie",
		SizeBytes:     5069636198,
		IntegrationID: 42,
	})

	time.Sleep(100 * time.Millisecond)
	persister.Stop()

	entries := writer.getAll()
	if len(entries) != 1 {
		t.Fatalf("expected 1 activity event, got %d", len(entries))
	}

	var meta DeletionSuccessEvent
	if err := json.Unmarshal([]byte(entries[0].Metadata), &meta); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	if meta.MediaName != "Serenity" {
		t.Errorf("expected mediaName 'Serenity', got %q", meta.MediaName)
	}
	if meta.SizeBytes != 5069636198 {
		t.Errorf("expected sizeBytes 5069636198, got %d", meta.SizeBytes)
	}
	if meta.IntegrationID != 42 {
		t.Errorf("expected integrationId 42, got %d", meta.IntegrationID)
	}
}
