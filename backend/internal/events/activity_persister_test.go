package events

import (
	"encoding/json"
	"testing"
	"time"

	"capacitarr/internal/db"

	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupTestDB creates an in-memory SQLite database with migrations applied.
// This is a local helper to avoid importing testutil (which pulls in routes).
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
	return database
}

func TestActivityPersister_PersistsSingleEvent(t *testing.T) {
	database := setupTestDB(t)
	bus := NewEventBus()

	persister := NewActivityPersister(database, bus)
	persister.Start()

	bus.Publish(EngineStartEvent{ExecutionMode: "dry-run"})

	// Give the persister time to write
	time.Sleep(100 * time.Millisecond)

	persister.Stop()

	var events []db.ActivityEvent
	if err := database.Find(&events).Error; err != nil {
		t.Fatalf("Failed to query activity events: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 activity event, got %d", len(events))
	}

	evt := events[0]
	if evt.EventType != "engine_start" {
		t.Errorf("expected event type 'engine_start', got %q", evt.EventType)
	}
	if evt.Message != "Engine run started in dry-run mode" {
		t.Errorf("unexpected message: %q", evt.Message)
	}

	// Verify metadata contains JSON-encoded event
	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(evt.Metadata), &meta); err != nil {
		t.Fatalf("failed to parse metadata JSON: %v", err)
	}
	if meta["executionMode"] != "dry-run" {
		t.Errorf("expected executionMode 'dry-run' in metadata, got %v", meta["executionMode"])
	}
}

func TestActivityPersister_PersistsMultipleEvents(t *testing.T) {
	database := setupTestDB(t)
	bus := NewEventBus()

	persister := NewActivityPersister(database, bus)
	persister.Start()

	bus.Publish(EngineStartEvent{ExecutionMode: "approval"})
	bus.Publish(EngineCompleteEvent{Evaluated: 50, Flagged: 5})
	bus.Publish(LoginEvent{Username: "admin"})

	time.Sleep(100 * time.Millisecond)
	persister.Stop()

	var count int64
	database.Model(&db.ActivityEvent{}).Count(&count)

	if count != 3 {
		t.Fatalf("expected 3 activity events, got %d", count)
	}

	// Verify ordering
	var events []db.ActivityEvent
	database.Order("id asc").Find(&events)

	expectedTypes := []string{"engine_start", "engine_complete", "login"}
	for i, expected := range expectedTypes {
		if events[i].EventType != expected {
			t.Errorf("event %d: expected type %q, got %q", i, expected, events[i].EventType)
		}
	}
}

func TestActivityPersister_StopDrainsRemaining(t *testing.T) {
	database := setupTestDB(t)
	bus := NewEventBus()

	persister := NewActivityPersister(database, bus)
	persister.Start()

	// Publish events and immediately stop — Stop should drain remaining events
	bus.Publish(ServerStartedEvent{Version: "1.0.0"})

	time.Sleep(50 * time.Millisecond)
	persister.Stop()

	var count int64
	database.Model(&db.ActivityEvent{}).Count(&count)

	if count != 1 {
		t.Fatalf("expected 1 activity event after stop, got %d", count)
	}
}

func TestActivityPersister_MetadataContainsFullEvent(t *testing.T) {
	database := setupTestDB(t)
	bus := NewEventBus()

	persister := NewActivityPersister(database, bus)
	persister.Start()

	bus.Publish(DeletionSuccessEvent{
		MediaName:     "Breaking Bad",
		MediaType:     "show",
		SizeBytes:     5069636198,
		IntegrationID: 42,
	})

	time.Sleep(100 * time.Millisecond)
	persister.Stop()

	var evt db.ActivityEvent
	database.First(&evt)

	var meta DeletionSuccessEvent
	if err := json.Unmarshal([]byte(evt.Metadata), &meta); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	if meta.MediaName != "Breaking Bad" {
		t.Errorf("expected mediaName 'Breaking Bad', got %q", meta.MediaName)
	}
	if meta.SizeBytes != 5069636198 {
		t.Errorf("expected sizeBytes 5069636198, got %d", meta.SizeBytes)
	}
	if meta.IntegrationID != 42 {
		t.Errorf("expected integrationId 42, got %d", meta.IntegrationID)
	}
}
