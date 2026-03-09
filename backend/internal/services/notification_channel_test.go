package services

import (
	"errors"
	"testing"
	"time"

	"capacitarr/internal/db"
)

func TestNotificationChannelService_Create(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewNotificationChannelService(database, bus)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	config := db.NotificationConfig{
		Type:       "discord",
		Name:       "Dev Alerts",
		WebhookURL: "https://discord.com/api/webhooks/test",
		Enabled:    true,
	}

	result, err := svc.Create(config)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if result.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if result.Name != "Dev Alerts" {
		t.Errorf("expected name 'Dev Alerts', got %q", result.Name)
	}

	// Verify event
	select {
	case evt := <-ch:
		if evt.EventType() != "notification_channel_added" {
			t.Errorf("expected event type 'notification_channel_added', got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for notification_channel_added event")
	}
}

func TestNotificationChannelService_Update(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewNotificationChannelService(database, bus)

	original := db.NotificationConfig{
		Type: "apprise", Name: "Original Apprise", WebhookURL: "http://apprise:8000/api/notify/key1/",
		AppriseTags: "admin",
	}
	database.Create(&original)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	updated := db.NotificationConfig{
		Type: "apprise", Name: "Updated Apprise", WebhookURL: "http://apprise:8000/api/notify/key2/",
		AppriseTags: "ops,alerts",
	}

	result, err := svc.Update(original.ID, updated)
	if err != nil {
		t.Fatalf("Update returned error: %v", err)
	}

	if result.Name != "Updated Apprise" {
		t.Errorf("expected name 'Updated Apprise', got %q", result.Name)
	}

	// Verify event
	select {
	case evt := <-ch:
		if evt.EventType() != "notification_channel_updated" {
			t.Errorf("expected event type 'notification_channel_updated', got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for notification_channel_updated event")
	}
}

func TestNotificationChannelService_Update_NotFound(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewNotificationChannelService(database, bus)

	_, err := svc.Update(99999, db.NotificationConfig{Name: "ghost"})
	if err == nil {
		t.Fatal("expected error for non-existent channel")
	}
}

func TestNotificationChannelService_Delete(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewNotificationChannelService(database, bus)

	config := db.NotificationConfig{
		Type: "discord", Name: "Firefly Alerts", WebhookURL: "https://discord.com/api/webhooks/test",
	}
	database.Create(&config)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	if err := svc.Delete(config.ID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	// Verify deleted from DB
	var count int64
	database.Model(&db.NotificationConfig{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 channels after delete, got %d", count)
	}

	// Verify event
	select {
	case evt := <-ch:
		if evt.EventType() != "notification_channel_removed" {
			t.Errorf("expected event type 'notification_channel_removed', got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for notification_channel_removed event")
	}
}

func TestNotificationChannelService_Delete_NotFound(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewNotificationChannelService(database, bus)

	err := svc.Delete(99999)
	if err == nil {
		t.Fatal("expected error for non-existent channel")
	}
}

func TestNotificationChannelService_List(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewNotificationChannelService(database, bus)

	// Empty list initially
	configs, err := svc.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(configs) != 0 {
		t.Errorf("expected 0 channels, got %d", len(configs))
	}

	// Insert two channels
	database.Create(&db.NotificationConfig{Type: "discord", Name: "Firefly Alerts", WebhookURL: "https://discord.com/api/webhooks/1", Enabled: true})
	database.Create(&db.NotificationConfig{Type: "apprise", Name: "Serenity Alerts", WebhookURL: "http://apprise:8000/api/notify/key/", Enabled: false})

	configs, err = svc.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(configs))
	}
	// Ordered by id ASC
	if configs[0].Name != "Firefly Alerts" {
		t.Errorf("expected first channel 'Firefly Alerts', got %q", configs[0].Name)
	}
}

func TestNotificationChannelService_GetByID(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewNotificationChannelService(database, bus)

	nc := db.NotificationConfig{Type: "discord", Name: "Firefly Alerts", WebhookURL: "https://discord.com/api/webhooks/1"}
	database.Create(&nc)

	config, err := svc.GetByID(nc.ID)
	if err != nil {
		t.Fatalf("GetByID returned error: %v", err)
	}
	if config.Name != "Firefly Alerts" {
		t.Errorf("expected name 'Firefly Alerts', got %q", config.Name)
	}
}

func TestNotificationChannelService_GetByID_NotFound(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewNotificationChannelService(database, bus)

	_, err := svc.GetByID(99999)
	if err == nil {
		t.Fatal("expected error for non-existent channel")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestNotificationChannelService_ListEnabled(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewNotificationChannelService(database, bus)

	database.Create(&db.NotificationConfig{Type: "discord", Name: "Firefly Alerts", Enabled: true})
	disabled := db.NotificationConfig{Type: "apprise", Name: "Serenity Alerts", Enabled: true}
	database.Create(&disabled)
	// Explicitly disable — GORM default:true ignores zero-value false on Create
	database.Model(&disabled).Update("enabled", false)

	configs, err := svc.ListEnabled()
	if err != nil {
		t.Fatalf("ListEnabled returned error: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 enabled channel, got %d", len(configs))
	}
	if configs[0].Name != "Firefly Alerts" {
		t.Errorf("expected 'Firefly Alerts', got %q", configs[0].Name)
	}
}
