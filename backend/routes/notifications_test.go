package routes_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"capacitarr/internal/db"
	"capacitarr/internal/testutil"
)

// ---------- GET /api/notifications/channels ----------

func TestListNotificationChannels_Empty(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/notifications/channels", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var channels []db.NotificationConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &channels); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(channels) != 0 {
		t.Errorf("Expected empty channel list, got %d items", len(channels))
	}
}

func TestListNotificationChannels_WithData(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed two channels
	database.Create(&db.NotificationConfig{
		Type: "discord", Name: "Firefly Alerts", WebhookURL: "https://discord.com/api/webhooks/123/abc", Enabled: true,
	})
	database.Create(&db.NotificationConfig{
		Type: "apprise", Name: "Serenity Alerts", WebhookURL: "http://apprise:8000/api/notify/key/",
		AppriseTags: "admin", Enabled: true,
	})

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/notifications/channels", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var channels []db.NotificationConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &channels); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if len(channels) != 2 {
		t.Errorf("Expected 2 channels, got %d", len(channels))
	}
}

// ---------- POST /api/notifications/channels ----------

func TestCreateNotificationChannel_ValidDiscord(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{"type":"discord","name":"Firefly Alerts","webhookUrl":"https://discord.com/api/webhooks/123/abc","enabled":true}`
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/notifications/channels", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var channel db.NotificationConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &channel); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if channel.ID == 0 {
		t.Error("Expected non-zero channel ID")
	}
	if channel.Type != "discord" {
		t.Errorf("Expected type 'discord', got %q", channel.Type)
	}
	if channel.Name != "Firefly Alerts" {
		t.Errorf("Expected name 'Firefly Alerts', got %q", channel.Name)
	}
}

func TestCreateNotificationChannel_ValidApprise(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{"type":"apprise","name":"Serenity Alerts","webhookUrl":"http://apprise:8000/api/notify/mykey/","appriseTags":"admin,ops","enabled":true}`
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/notifications/channels", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var channel db.NotificationConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &channel); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if channel.Type != "apprise" {
		t.Errorf("Expected type 'apprise', got %q", channel.Type)
	}
	if channel.AppriseTags != "admin,ops" {
		t.Errorf("Expected appriseTags 'admin,ops', got %q", channel.AppriseTags)
	}
}

func TestCreateNotificationChannel_AppriseWithoutTags(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Tags are optional for Apprise
	body := `{"type":"apprise","name":"Serenity Alerts","webhookUrl":"http://apprise:8000/api/notify/mykey/","enabled":true}`
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/notifications/channels", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var channel db.NotificationConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &channel); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if channel.Type != "apprise" {
		t.Errorf("Expected type 'apprise', got %q", channel.Type)
	}
}

func TestCreateNotificationChannel_MissingRequiredFields(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	tests := []struct {
		name string
		body string
	}{
		{"missing type", `{"name":"Firefly Alerts","webhookUrl":"https://discord.com/api/webhooks/123/abc"}`},
		{"missing name", `{"type":"discord","webhookUrl":"https://discord.com/api/webhooks/123/abc"}`},
		{"empty type", `{"type":"","name":"Firefly Alerts","webhookUrl":"https://discord.com/api/webhooks/123/abc"}`},
		{"empty name", `{"type":"discord","name":"","webhookUrl":"https://discord.com/api/webhooks/123/abc"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/notifications/channels", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("Expected 400 for %s, got %d: %s", tc.name, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestCreateNotificationChannel_InvalidTypeInApp(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{"type":"inapp","name":"In-App","webhookUrl":""}`
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/notifications/channels", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for inapp type, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateNotificationChannel_InvalidTypeSlack(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Slack is no longer a valid type — use Apprise with Slack plugin instead
	body := `{"type":"slack","name":"Serenity Alerts","webhookUrl":"https://hooks.slack.com/services/T00/B00/xxx"}`
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/notifications/channels", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for slack type, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateNotificationChannel_InvalidTypeTelegram(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{"type":"telegram","name":"Firefly Channel","webhookUrl":"https://api.telegram.org/bot123/sendMessage"}`
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/notifications/channels", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for telegram type, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateNotificationChannel_MissingWebhookURL(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{"type":"discord","name":"Firefly Alerts"}`
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/notifications/channels", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for missing webhook URL, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateNotificationChannel_MissingWebhookURL_Apprise(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{"type":"apprise","name":"Firefly Alerts"}`
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/notifications/channels", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for missing webhook URL on apprise, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateNotificationChannel_InvalidWebhookURLScheme(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{"type":"discord","name":"Firefly Alerts","webhookUrl":"ftp://evil.example.com/hook"}`
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/notifications/channels", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for invalid webhook URL scheme, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------- PUT /api/notifications/channels/:id ----------

func TestUpdateNotificationChannel_Valid(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed a channel
	channel := db.NotificationConfig{
		Type: "discord", Name: "Firefly Alerts", WebhookURL: "https://discord.com/api/webhooks/123/abc", Enabled: true,
	}
	database.Create(&channel)

	body := `{"name":"Serenity Alerts","webhookUrl":"https://discord.com/api/webhooks/456/def","enabled":false}`
	req := testutil.AuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/api/notifications/channels/%d", channel.ID), strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated db.NotificationConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if updated.Name != "Serenity Alerts" {
		t.Errorf("Expected name 'Serenity Alerts', got %q", updated.Name)
	}
	if updated.Enabled {
		t.Error("Expected enabled to be false after update")
	}
}

func TestUpdateNotificationChannel_NotFound(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{"name":"Firefly Alerts","webhookUrl":"https://discord.com/api/webhooks/123/abc"}`
	req := testutil.AuthenticatedRequest(t, http.MethodPut, "/api/notifications/channels/99999", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateNotificationChannel_InvalidWebhookURL(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed a channel
	channel := db.NotificationConfig{
		Type: "discord", Name: "Firefly Alerts", WebhookURL: "https://discord.com/api/webhooks/123/abc", Enabled: true,
	}
	database.Create(&channel)

	body := `{"webhookUrl":"ftp://evil.example.com/hook"}`
	req := testutil.AuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/api/notifications/channels/%d", channel.ID), strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400 for invalid webhook URL, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateNotificationChannel_AppriseTags(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed an Apprise channel
	channel := db.NotificationConfig{
		Type: "apprise", Name: "Firefly Alerts", WebhookURL: "http://apprise:8000/api/notify/mykey/",
		AppriseTags: "admin", Enabled: true,
	}
	database.Create(&channel)

	body := `{"name":"Firefly Alerts","webhookUrl":"http://apprise:8000/api/notify/mykey/","appriseTags":"admin,ops","enabled":true}`
	req := testutil.AuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/api/notifications/channels/%d", channel.ID), strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated db.NotificationConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if updated.AppriseTags != "admin,ops" {
		t.Errorf("Expected appriseTags 'admin,ops', got %q", updated.AppriseTags)
	}
}

// ---------- DELETE /api/notifications/channels/:id ----------

func TestDeleteNotificationChannel_Valid(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed a channel
	channel := db.NotificationConfig{
		Type: "discord", Name: "Firefly Alerts", WebhookURL: "https://discord.com/api/webhooks/123/abc", Enabled: true,
	}
	database.Create(&channel)

	req := testutil.AuthenticatedRequest(t, http.MethodDelete, fmt.Sprintf("/api/notifications/channels/%d", channel.ID), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify it's actually deleted
	var count int64
	database.Model(&db.NotificationConfig{}).Count(&count)
	if count != 0 {
		t.Errorf("Expected 0 channels after delete, got %d", count)
	}
}

func TestDeleteNotificationChannel_NotFound(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodDelete, "/api/notifications/channels/99999", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("Expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}
