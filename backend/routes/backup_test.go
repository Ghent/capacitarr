package routes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"capacitarr/internal/db"
	"capacitarr/internal/services"
	"capacitarr/internal/testutil"
)

// ---------- GET /api/settings/export ----------

func TestExportSettings_AllSections(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Seed test data
	ic := db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://sonarr:8989",
		APIKey: "secret-key", Enabled: true,
	}
	database.Create(&ic)
	database.Create(&db.CustomRule{
		Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep", Enabled: true,
		IntegrationID: &ic.ID,
	})
	database.Create(&db.DiskGroup{
		MountPath: "/mnt/media", TotalBytes: 1000000, UsedBytes: 500000,
		ThresholdPct: 85, TargetPct: 75,
	})
	database.Create(&db.NotificationConfig{
		Type: "discord", Name: "Serenity Alerts",
		WebhookURL: "https://discord.com/api/webhooks/secret",
		Enabled:    true,
	})

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/settings/export", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify Content-Disposition header
	cd := rec.Header().Get("Content-Disposition")
	if cd == "" {
		t.Error("Expected Content-Disposition header to be set")
	}
	if !strings.Contains(cd, "capacitarr-settings-") {
		t.Errorf("Expected Content-Disposition to contain 'capacitarr-settings-', got %q", cd)
	}

	var envelope services.SettingsExportEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if envelope.Version != 1 {
		t.Errorf("Expected version 1, got %d", envelope.Version)
	}
	if envelope.Preferences == nil {
		t.Error("Expected preferences to be present")
	}
	if len(envelope.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(envelope.Rules))
	}
	if len(envelope.Integrations) != 1 {
		t.Errorf("Expected 1 integration, got %d", len(envelope.Integrations))
	}
	if len(envelope.DiskGroups) != 1 {
		t.Errorf("Expected 1 disk group, got %d", len(envelope.DiskGroups))
	}
	if len(envelope.NotificationChannels) != 1 {
		t.Errorf("Expected 1 notification channel, got %d", len(envelope.NotificationChannels))
	}
}

func TestExportSettings_SelectedSections(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	ic := db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://sonarr:8989",
		APIKey: "secret-key", Enabled: true,
	}
	database.Create(&ic)
	database.Create(&db.CustomRule{
		Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep", Enabled: true,
		IntegrationID: &ic.ID,
	})

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/settings/export?sections=rules", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var envelope services.SettingsExportEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if envelope.Preferences != nil {
		t.Error("Expected preferences to be nil when only rules requested")
	}
	if len(envelope.Rules) != 1 {
		t.Errorf("Expected 1 rule, got %d", len(envelope.Rules))
	}
	if len(envelope.Integrations) != 0 {
		t.Errorf("Expected 0 integrations, got %d", len(envelope.Integrations))
	}
}

func TestExportSettings_Unauthenticated(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/api/settings/export", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rec.Code)
	}
}

// ---------- POST /api/settings/import ----------

func TestImportSettings_AllSections(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{
		"payload": {
			"version": 1,
			"exportedAt": "2026-03-09T12:00:00Z",
			"appVersion": "v1.0.0",
			"preferences": {
				"logLevel": "warn",
				"auditLogRetentionDays": 60,
				"pollIntervalSeconds": 600,
				"watchHistoryWeight": 7,
				"lastWatchedWeight": 6,
				"fileSizeWeight": 5,
				"ratingWeight": 4,
				"timeInLibraryWeight": 3,
				"seriesStatusWeight": 2,
				"executionMode": "approval",
				"tiebreakerMethod": "name_asc",
				"deletionsEnabled": false,
				"snoozeDurationHours": 48,
				"checkForUpdates": false
			},
			"rules": [
				{"field": "quality", "operator": "==", "value": "4K", "effect": "always_keep", "enabled": true, "integrationName": "Firefly Sonarr", "integrationType": "sonarr"}
			],
			"integrations": [
				{"name": "Firefly Sonarr", "type": "sonarr", "url": "http://sonarr:8989", "enabled": true}
			],
			"diskGroups": [
				{"mountPath": "/mnt/data", "thresholdPct": 90, "targetPct": 80}
			],
			"notificationChannels": [
				{"name": "Serenity Discord", "type": "discord", "enabled": true, "onCycleDigest": true, "onError": true}
			]
		},
		"sections": {
			"preferences": true,
			"rules": true,
			"integrations": true,
			"diskGroups": true,
			"notificationChannels": true
		}
	}`

	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/settings/import", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result services.ImportResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !result.PreferencesImported {
		t.Error("Expected preferencesImported to be true")
	}
	if result.RulesImported != 1 {
		t.Errorf("Expected 1 rule imported, got %d", result.RulesImported)
	}
	if result.IntegrationsImported != 1 {
		t.Errorf("Expected 1 integration imported, got %d", result.IntegrationsImported)
	}
	if result.DiskGroupsImported != 1 {
		t.Errorf("Expected 1 disk group imported, got %d", result.DiskGroupsImported)
	}
	if result.NotificationChannelsImported != 1 {
		t.Errorf("Expected 1 notification channel imported, got %d", result.NotificationChannelsImported)
	}
}

func TestImportSettings_UnsupportedVersion(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{
		"payload": {
			"version": 99,
			"exportedAt": "2026-03-09T12:00:00Z",
			"appVersion": "v1.0.0"
		},
		"sections": { "preferences": true }
	}`

	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/settings/import", strings.NewReader(body))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestImportSettings_InvalidBody(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/settings/import", strings.NewReader("not json"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestImportSettings_Unauthenticated(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{"payload": {"version": 1}, "sections": {"preferences": true}}`
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "/api/settings/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", rec.Code)
	}
}
