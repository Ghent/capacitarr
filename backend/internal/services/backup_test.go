package services

import (
	"testing"

	"capacitarr/internal/db"
)

// ---------- Export ----------

func TestBackupService_Export_AllSections(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Seed test data
	intID := seedIntegration(t, database)
	database.Create(&db.CustomRule{
		Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep",
		Enabled: true, IntegrationID: &intID,
	})
	database.Create(&db.DiskGroup{
		MountPath: "/mnt/media", TotalBytes: 1000000, UsedBytes: 500000,
		ThresholdPct: 85, TargetPct: 75,
	})
	database.Create(&db.NotificationConfig{
		Type: "discord", Name: "Firefly Alerts",
		WebhookURL: "https://discord.com/api/webhooks/secret",
		Enabled:    true, OnCycleDigest: true, OnError: true,
	})

	sections := ExportSections{
		Preferences:          true,
		Rules:                true,
		Integrations:         true,
		DiskGroups:           true,
		NotificationChannels: true,
	}

	envelope, err := svc.Export(sections, "v1.0.0-test")
	if err != nil {
		t.Fatalf("Export returned error: %v", err)
	}

	if envelope.Version != 1 {
		t.Errorf("expected version 1, got %d", envelope.Version)
	}
	if envelope.AppVersion != "v1.0.0-test" {
		t.Errorf("expected appVersion 'v1.0.0-test', got %q", envelope.AppVersion)
	}
	if envelope.ExportedAt == "" {
		t.Error("expected non-empty exportedAt")
	}

	// Preferences
	if envelope.Preferences == nil {
		t.Fatal("expected preferences to be non-nil")
	}
	if envelope.Preferences.ExecutionMode != "dry-run" {
		t.Errorf("expected execution mode 'dry-run', got %q", envelope.Preferences.ExecutionMode)
	}

	// Rules
	if len(envelope.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(envelope.Rules))
	}
	if envelope.Rules[0].Field != "quality" {
		t.Errorf("expected rule field 'quality', got %q", envelope.Rules[0].Field)
	}
	if envelope.Rules[0].IntegrationName == nil || *envelope.Rules[0].IntegrationName != "Test Sonarr" {
		t.Error("expected integration name 'Test Sonarr' on exported rule")
	}

	// Integrations
	if len(envelope.Integrations) != 1 {
		t.Fatalf("expected 1 integration, got %d", len(envelope.Integrations))
	}
	if envelope.Integrations[0].Name != "Test Sonarr" {
		t.Errorf("expected integration name 'Test Sonarr', got %q", envelope.Integrations[0].Name)
	}

	// DiskGroups
	if len(envelope.DiskGroups) != 1 {
		t.Fatalf("expected 1 disk group, got %d", len(envelope.DiskGroups))
	}
	if envelope.DiskGroups[0].MountPath != "/mnt/media" {
		t.Errorf("expected mount path '/mnt/media', got %q", envelope.DiskGroups[0].MountPath)
	}

	// NotificationChannels
	if len(envelope.NotificationChannels) != 1 {
		t.Fatalf("expected 1 notification channel, got %d", len(envelope.NotificationChannels))
	}
	if envelope.NotificationChannels[0].Name != "Firefly Alerts" {
		t.Errorf("expected channel name 'Firefly Alerts', got %q", envelope.NotificationChannels[0].Name)
	}
}

func TestBackupService_Export_OnlyRules(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	database.Create(&db.CustomRule{
		Field: "tag", Operator: "contains", Value: "anime", Effect: "prefer_keep", Enabled: true,
	})

	sections := ExportSections{Rules: true}

	envelope, err := svc.Export(sections, "v1.0.0-test")
	if err != nil {
		t.Fatalf("Export returned error: %v", err)
	}

	if envelope.Preferences != nil {
		t.Error("expected preferences to be nil when not requested")
	}
	if len(envelope.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(envelope.Rules))
	}
	if len(envelope.Integrations) != 0 {
		t.Errorf("expected 0 integrations, got %d", len(envelope.Integrations))
	}
	if len(envelope.DiskGroups) != 0 {
		t.Errorf("expected 0 disk groups, got %d", len(envelope.DiskGroups))
	}
	if len(envelope.NotificationChannels) != 0 {
		t.Errorf("expected 0 notification channels, got %d", len(envelope.NotificationChannels))
	}
}

func TestBackupService_Export_SensitiveFieldsExcluded(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Seed data with sensitive fields
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Arr", URL: "http://localhost:8989",
		APIKey: "super-secret-api-key", Enabled: true,
	})
	database.Create(&db.NotificationConfig{
		Type: "discord", Name: "Serenity Alerts",
		WebhookURL: "https://discord.com/api/webhooks/secret-webhook",
		Enabled:    true,
	})
	database.Create(&db.DiskGroup{
		MountPath: "/mnt/data", TotalBytes: 2000000, UsedBytes: 1000000,
		ThresholdPct: 90, TargetPct: 80,
	})

	sections := ExportSections{
		Integrations:         true,
		NotificationChannels: true,
		DiskGroups:           true,
	}

	envelope, err := svc.Export(sections, "v1.0.0-test")
	if err != nil {
		t.Fatalf("Export returned error: %v", err)
	}

	// IntegrationExport should NOT contain API key
	if len(envelope.Integrations) != 1 {
		t.Fatalf("expected 1 integration, got %d", len(envelope.Integrations))
	}
	// The IntegrationExport struct simply doesn't have an APIKey field,
	// so there's no way to leak it. Verify the exported fields are correct.
	ie := envelope.Integrations[0]
	if ie.URL != "http://localhost:8989" {
		t.Errorf("expected URL 'http://localhost:8989', got %q", ie.URL)
	}

	// NotificationExport should NOT contain webhook URL
	if len(envelope.NotificationChannels) != 1 {
		t.Fatalf("expected 1 notification channel, got %d", len(envelope.NotificationChannels))
	}

	// DiskGroupExport should NOT contain TotalBytes/UsedBytes
	if len(envelope.DiskGroups) != 1 {
		t.Fatalf("expected 1 disk group, got %d", len(envelope.DiskGroups))
	}
	dge := envelope.DiskGroups[0]
	if dge.ThresholdPct != 90 {
		t.Errorf("expected threshold 90, got %f", dge.ThresholdPct)
	}
}

// ---------- Import ----------

func TestBackupService_Import_AllSections(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	envelope := SettingsExportEnvelope{
		Version:    1,
		ExportedAt: "2026-03-09T12:00:00Z",
		AppVersion: "v1.0.0",
		Preferences: &PreferencesExport{
			LogLevel:              "debug",
			AuditLogRetentionDays: 60,
			PollIntervalSeconds:   600,
			WatchHistoryWeight:    8,
			LastWatchedWeight:     6,
			FileSizeWeight:        4,
			RatingWeight:          3,
			TimeInLibraryWeight:   2,
			SeriesStatusWeight:    1,
			ExecutionMode:         "approval",
			TiebreakerMethod:      "name_asc",
			DeletionsEnabled:      false,
			SnoozeDurationHours:   48,
			CheckForUpdates:       false,
		},
		Rules: []RuleExport{
			{Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep", Enabled: true},
			{Field: "tag", Operator: "contains", Value: "anime", Effect: "prefer_keep", Enabled: false},
		},
		Integrations: []IntegrationExport{
			{Name: "Firefly Sonarr", Type: "sonarr", URL: "http://sonarr:8989", Enabled: true},
		},
		DiskGroups: []DiskGroupExport{
			{MountPath: "/mnt/media", ThresholdPct: 90, TargetPct: 80},
		},
		NotificationChannels: []NotificationExport{
			{Name: "Serenity Discord", Type: "discord", Enabled: true, OnCycleDigest: true, OnError: true},
		},
	}

	sections := ImportSections{
		Preferences:          true,
		Rules:                true,
		Integrations:         true,
		DiskGroups:           true,
		NotificationChannels: true,
	}

	result, err := svc.Import(envelope, sections)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	if !result.PreferencesImported {
		t.Error("expected preferencesImported to be true")
	}
	if result.RulesImported != 2 {
		t.Errorf("expected 2 rules imported, got %d", result.RulesImported)
	}
	if result.IntegrationsImported != 1 {
		t.Errorf("expected 1 integration imported, got %d", result.IntegrationsImported)
	}
	if result.DiskGroupsImported != 1 {
		t.Errorf("expected 1 disk group imported, got %d", result.DiskGroupsImported)
	}
	if result.NotificationChannelsImported != 1 {
		t.Errorf("expected 1 notification channel imported, got %d", result.NotificationChannelsImported)
	}

	// Verify preferences were updated
	var pref db.PreferenceSet
	database.First(&pref)
	if pref.ExecutionMode != "approval" {
		t.Errorf("expected execution mode 'approval', got %q", pref.ExecutionMode)
	}
	if pref.LogLevel != "debug" {
		t.Errorf("expected log level 'debug', got %q", pref.LogLevel)
	}
	if pref.PollIntervalSeconds != 600 {
		t.Errorf("expected poll interval 600, got %d", pref.PollIntervalSeconds)
	}

	// Verify rules were created
	var rules []db.CustomRule
	database.Order("sort_order ASC").Find(&rules)
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules in DB, got %d", len(rules))
	}
	if rules[0].Field != "quality" {
		t.Errorf("expected first rule field 'quality', got %q", rules[0].Field)
	}

	// Verify integration was created with placeholder API key
	var configs []db.IntegrationConfig
	database.Find(&configs)
	if len(configs) != 1 {
		t.Fatalf("expected 1 integration in DB, got %d", len(configs))
	}
	if configs[0].APIKey != "PLACEHOLDER_REPLACE_ME" {
		t.Errorf("expected placeholder API key, got %q", configs[0].APIKey)
	}

	// Verify disk group was created
	var groups []db.DiskGroup
	database.Find(&groups)
	if len(groups) != 1 {
		t.Fatalf("expected 1 disk group in DB, got %d", len(groups))
	}
	if groups[0].MountPath != "/mnt/media" {
		t.Errorf("expected mount path '/mnt/media', got %q", groups[0].MountPath)
	}

	// Verify notification channel was created with placeholder webhook
	var channels []db.NotificationConfig
	database.Find(&channels)
	if len(channels) != 1 {
		t.Fatalf("expected 1 notification channel in DB, got %d", len(channels))
	}
	if channels[0].WebhookURL != "https://placeholder.example.com/replace-me" {
		t.Errorf("expected placeholder webhook URL, got %q", channels[0].WebhookURL)
	}
}

func TestBackupService_Import_OnlyPreferences(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	envelope := SettingsExportEnvelope{
		Version:    1,
		ExportedAt: "2026-03-09T12:00:00Z",
		AppVersion: "v1.0.0",
		Preferences: &PreferencesExport{
			LogLevel:              "warn",
			AuditLogRetentionDays: 7,
			PollIntervalSeconds:   120,
			WatchHistoryWeight:    5,
			LastWatchedWeight:     5,
			FileSizeWeight:        5,
			RatingWeight:          5,
			TimeInLibraryWeight:   5,
			SeriesStatusWeight:    5,
			ExecutionMode:         "auto",
			TiebreakerMethod:      "size_asc",
			DeletionsEnabled:      true,
			SnoozeDurationHours:   12,
			CheckForUpdates:       true,
		},
		Rules: []RuleExport{
			{Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep", Enabled: true},
		},
	}

	// Only import preferences, not rules
	sections := ImportSections{Preferences: true}

	result, err := svc.Import(envelope, sections)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	if !result.PreferencesImported {
		t.Error("expected preferencesImported to be true")
	}
	if result.RulesImported != 0 {
		t.Errorf("expected 0 rules imported, got %d", result.RulesImported)
	}

	// Verify preferences were updated
	var pref db.PreferenceSet
	database.First(&pref)
	if pref.ExecutionMode != "auto" {
		t.Errorf("expected execution mode 'auto', got %q", pref.ExecutionMode)
	}

	// Verify no rules were created
	var rules []db.CustomRule
	database.Find(&rules)
	if len(rules) != 0 {
		t.Errorf("expected 0 rules in DB, got %d", len(rules))
	}
}

func TestBackupService_Import_RejectsUnsupportedVersion(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	envelope := SettingsExportEnvelope{
		Version:    99,
		ExportedAt: "2026-03-09T12:00:00Z",
		AppVersion: "v1.0.0",
	}

	sections := ImportSections{Preferences: true}

	_, err := svc.Import(envelope, sections)
	if err == nil {
		t.Fatal("expected error for unsupported version")
	}
}

// ---------- Import disk groups upsert ----------

func TestBackupService_Import_DiskGroupUpsert(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Seed an existing disk group
	database.Create(&db.DiskGroup{
		MountPath: "/mnt/media", TotalBytes: 1000000, UsedBytes: 500000,
		ThresholdPct: 85, TargetPct: 75,
	})

	envelope := SettingsExportEnvelope{
		Version: 1,
		DiskGroups: []DiskGroupExport{
			{MountPath: "/mnt/media", ThresholdPct: 95, TargetPct: 85},
		},
	}

	sections := ImportSections{DiskGroups: true}

	result, err := svc.Import(envelope, sections)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	if result.DiskGroupsImported != 1 {
		t.Errorf("expected 1 disk group imported, got %d", result.DiskGroupsImported)
	}

	// Should have updated, not duplicated
	var groups []db.DiskGroup
	database.Find(&groups)
	if len(groups) != 1 {
		t.Fatalf("expected 1 disk group in DB (upsert), got %d", len(groups))
	}
	if groups[0].ThresholdPct != 95 {
		t.Errorf("expected threshold 95, got %f", groups[0].ThresholdPct)
	}
	if groups[0].TargetPct != 85 {
		t.Errorf("expected target 85, got %f", groups[0].TargetPct)
	}
}

// ---------- Import rules with integration resolution ----------

func TestBackupService_Import_RulesWithIntegrationResolution(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Seed an integration for auto-match
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://sonarr:8989",
		APIKey: "test-key", Enabled: true,
	})

	sonarrType := "sonarr"
	sonarrName := "Firefly Sonarr"

	envelope := SettingsExportEnvelope{
		Version: 1,
		Rules: []RuleExport{
			{
				Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep",
				Enabled: true, IntegrationType: &sonarrType, IntegrationName: &sonarrName,
			},
		},
	}

	sections := ImportSections{Rules: true}

	result, err := svc.Import(envelope, sections)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	if result.RulesImported != 1 {
		t.Errorf("expected 1 rule imported, got %d", result.RulesImported)
	}

	// Verify the rule was linked to the integration
	var rules []db.CustomRule
	database.Find(&rules)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule in DB, got %d", len(rules))
	}
	if rules[0].IntegrationID == nil {
		t.Error("expected rule to have an integration ID")
	}
}
