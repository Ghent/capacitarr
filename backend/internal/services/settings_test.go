package services

import (
	"testing"
	"time"

	"capacitarr/internal/db"
)

func TestSettingsService_GetPreferences(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewSettingsService(database, bus)

	prefs, err := svc.GetPreferences()
	if err != nil {
		t.Fatalf("GetPreferences returned error: %v", err)
	}

	if prefs.ID != 1 {
		t.Errorf("expected preference ID 1, got %d", prefs.ID)
	}
	if prefs.ExecutionMode != db.ModeDryRun {
		t.Errorf("expected execution mode 'dry-run', got %q", prefs.ExecutionMode)
	}
}

func TestSettingsService_UpdatePreferences(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewSettingsService(database, bus)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	// Get the current preferences so we have all seeded values
	current, _ := svc.GetPreferences()
	current.PollIntervalSeconds = 600

	updated, err := svc.UpdatePreferences(current)
	if err != nil {
		t.Fatalf("UpdatePreferences returned error: %v", err)
	}

	if updated.PollIntervalSeconds != 600 {
		t.Errorf("expected poll interval 600, got %d", updated.PollIntervalSeconds)
	}

	// Should publish settings_changed event
	select {
	case evt := <-ch:
		if evt.EventType() != "settings_changed" {
			t.Errorf("expected event type 'settings_changed', got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for settings_changed event")
	}
}

func TestSettingsService_UpdatePreferences_ModeChange(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewSettingsService(database, bus)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	// Get current and change mode
	current, _ := svc.GetPreferences()
	current.ExecutionMode = "approval"

	if _, err := svc.UpdatePreferences(current); err != nil {
		t.Fatalf("UpdatePreferences returned error: %v", err)
	}

	// Should publish two events: engine_mode_changed and settings_changed
	receivedTypes := map[string]bool{}
	for i := 0; i < 2; i++ {
		select {
		case evt := <-ch:
			receivedTypes[evt.EventType()] = true
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for events")
		}
	}

	if !receivedTypes["engine_mode_changed"] {
		t.Error("expected engine_mode_changed event")
	}
	if !receivedTypes["settings_changed"] {
		t.Error("expected settings_changed event")
	}
}

func TestSettingsService_ListRecentActivities(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewSettingsService(database, bus)

	database.Create(&db.ActivityEvent{EventType: "engine_complete", Message: "Done"})
	database.Create(&db.ActivityEvent{EventType: "login", Message: "User logged in"})

	activities, err := svc.ListRecentActivities(1)
	if err != nil {
		t.Fatalf("ListRecentActivities error: %v", err)
	}
	if len(activities) != 1 {
		t.Errorf("expected 1 activity, got %d", len(activities))
	}
}
