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

	sonarrType := "sonarr"
	sonarrName := "Firefly Sonarr"

	envelope := SettingsExportEnvelope{
		Version:    1,
		ExportedAt: "2026-03-09T12:00:00Z",
		AppVersion: "v1.0.0",
		Preferences: &PreferencesExport{
			LogLevel:              "debug",
			AuditLogRetentionDays: 60,
			PollIntervalSeconds:   600,
			ExecutionMode:         "approval",
			TiebreakerMethod:      "name_asc",
			DeletionsEnabled:      false,
			SnoozeDurationHours:   48,
			CheckForUpdates:       false,
			FactorWeights: map[string]int{
				"watch_history":      8,
				"last_watched":       6,
				"file_size":          4,
				"rating":             3,
				"time_in_library":    2,
				"series_status":      1,
				"request_popularity": 0,
			},
		},
		Rules: []RuleExport{
			{Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep", Enabled: true, IntegrationType: &sonarrType, IntegrationName: &sonarrName},
			{Field: "tag", Operator: "contains", Value: "anime", Effect: "prefer_keep", Enabled: false, IntegrationType: &sonarrType, IntegrationName: &sonarrName},
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

	// Verify integration was created with placeholder API key and disabled
	var configs []db.IntegrationConfig
	database.Find(&configs)
	if len(configs) != 1 {
		t.Fatalf("expected 1 integration in DB, got %d", len(configs))
	}
	if configs[0].APIKey != "PLACEHOLDER_REPLACE_ME" {
		t.Errorf("expected placeholder API key, got %q", configs[0].APIKey)
	}
	if configs[0].Enabled {
		t.Error("expected new placeholder integration to be disabled")
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

	// Verify notification channel was created with placeholder webhook and disabled
	var channels []db.NotificationConfig
	database.Find(&channels)
	if len(channels) != 1 {
		t.Fatalf("expected 1 notification channel in DB, got %d", len(channels))
	}
	if channels[0].WebhookURL != "https://placeholder.example.com/replace-me" {
		t.Errorf("expected placeholder webhook URL, got %q", channels[0].WebhookURL)
	}
	if channels[0].Enabled {
		t.Error("expected new placeholder notification channel to be disabled")
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
			ExecutionMode:         "auto",
			TiebreakerMethod:      "size_asc",
			DeletionsEnabled:      true,
			SnoozeDurationHours:   12,
			CheckForUpdates:       true,
			FactorWeights: map[string]int{
				"watch_history":      5,
				"last_watched":       5,
				"file_size":          5,
				"rating":             5,
				"time_in_library":    5,
				"series_status":      5,
				"request_popularity": 0,
			},
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

// ---------- Integration upsert ----------

func TestBackupService_Import_IntegrationUpsert(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Seed existing integration with real API key
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://old-url:8989",
		APIKey: "real-api-key-123", Enabled: true,
	})

	envelope := SettingsExportEnvelope{
		Version: 1,
		Integrations: []IntegrationExport{
			{Name: "Firefly Sonarr", Type: "sonarr", URL: "http://new-url:8989", Enabled: true},
		},
	}

	sections := ImportSections{Integrations: true}

	result, err := svc.Import(envelope, sections)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	if result.IntegrationsImported != 1 {
		t.Errorf("expected 1 integration imported, got %d", result.IntegrationsImported)
	}

	// Should have updated, not duplicated
	var configs []db.IntegrationConfig
	database.Find(&configs)
	if len(configs) != 1 {
		t.Fatalf("expected 1 integration in DB (upsert), got %d", len(configs))
	}
	if configs[0].URL != "http://new-url:8989" {
		t.Errorf("expected updated URL, got %q", configs[0].URL)
	}
	// API key must be preserved (not overwritten with placeholder)
	if configs[0].APIKey != "real-api-key-123" {
		t.Errorf("expected API key to be preserved, got %q", configs[0].APIKey)
	}
	// Existing integration should remain enabled
	if !configs[0].Enabled {
		t.Error("expected existing integration to remain enabled after upsert")
	}
}

// ---------- Notification channel upsert ----------

func TestBackupService_Import_NotificationChannelUpsert(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Seed existing channel with real webhook
	database.Create(&db.NotificationConfig{
		Type: "discord", Name: "Serenity Discord",
		WebhookURL: "https://discord.com/api/webhooks/real-webhook",
		Enabled:    true, OnCycleDigest: false, OnError: false,
	})

	envelope := SettingsExportEnvelope{
		Version: 1,
		NotificationChannels: []NotificationExport{
			{Name: "Serenity Discord", Type: "discord", Enabled: true, OnCycleDigest: true, OnError: true},
		},
	}

	sections := ImportSections{NotificationChannels: true}

	result, err := svc.Import(envelope, sections)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	if result.NotificationChannelsImported != 1 {
		t.Errorf("expected 1 channel imported, got %d", result.NotificationChannelsImported)
	}

	// Should have updated, not duplicated
	var channels []db.NotificationConfig
	database.Find(&channels)
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel in DB (upsert), got %d", len(channels))
	}
	// Webhook URL must be preserved
	if channels[0].WebhookURL != "https://discord.com/api/webhooks/real-webhook" {
		t.Errorf("expected webhook URL to be preserved, got %q", channels[0].WebhookURL)
	}
	// Subscription flags should be updated
	if !channels[0].OnCycleDigest {
		t.Error("expected OnCycleDigest to be updated to true")
	}
	if !channels[0].OnError {
		t.Error("expected OnError to be updated to true")
	}
}

// ---------- Type-only fallback matching ----------

func TestBackupService_Import_RulesTypeOnlyFallback(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Seed one sonarr integration with a DIFFERENT name than what the export has
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "My Sonarr", URL: "http://sonarr:8989",
		APIKey: "test-key", Enabled: true,
	})

	sonarrType := "sonarr"
	exportedName := "Old Sonarr Name"

	envelope := SettingsExportEnvelope{
		Version: 1,
		Rules: []RuleExport{
			{
				Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep",
				Enabled: true, IntegrationType: &sonarrType, IntegrationName: &exportedName,
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
	if result.RulesUnmatched != 0 {
		t.Errorf("expected 0 unmatched (type-only fallback), got %d", result.RulesUnmatched)
	}

	// Verify the rule was linked to the only sonarr integration
	var rules []db.CustomRule
	database.Find(&rules)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule in DB, got %d", len(rules))
	}
	if rules[0].IntegrationID == nil {
		t.Error("expected rule to have an integration ID from type-only fallback")
	}
}

func TestBackupService_Import_RulesTypeOnlyFallback_AmbiguousSkips(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Seed TWO sonarr integrations — type-only fallback should NOT match
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Sonarr A", URL: "http://sonarr-a:8989",
		APIKey: "test-key-a", Enabled: true,
	})
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Sonarr B", URL: "http://sonarr-b:8989",
		APIKey: "test-key-b", Enabled: true,
	})

	sonarrType := "sonarr"
	exportedName := "Non-existent Sonarr"

	envelope := SettingsExportEnvelope{
		Version: 1,
		Rules: []RuleExport{
			{
				Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep",
				Enabled: true, IntegrationType: &sonarrType, IntegrationName: &exportedName,
			},
		},
	}

	sections := ImportSections{Rules: true}

	result, err := svc.Import(envelope, sections)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	if result.RulesUnmatched != 1 {
		t.Errorf("expected 1 unmatched rule (ambiguous), got %d", result.RulesUnmatched)
	}

	// Unmatched rules are skipped — every rule must belong to an integration
	var rules []db.CustomRule
	database.Find(&rules)
	if len(rules) != 0 {
		t.Fatalf("expected 0 rules in DB (ambiguous match skipped), got %d", len(rules))
	}
}

// ---------- Rules resolve to imported integrations ----------

func TestBackupService_Import_RulesResolveToImportedIntegrations(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	sonarrType := "sonarr"
	sonarrName := "Firefly Sonarr"

	envelope := SettingsExportEnvelope{
		Version: 1,
		Integrations: []IntegrationExport{
			{Name: "Firefly Sonarr", Type: "sonarr", URL: "http://sonarr:8989", Enabled: true},
		},
		Rules: []RuleExport{
			{
				Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep",
				Enabled: true, IntegrationType: &sonarrType, IntegrationName: &sonarrName,
			},
		},
	}

	// Import both integrations AND rules in one shot
	sections := ImportSections{Integrations: true, Rules: true}

	result, err := svc.Import(envelope, sections)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	if result.IntegrationsImported != 1 {
		t.Errorf("expected 1 integration imported, got %d", result.IntegrationsImported)
	}
	if result.RulesImported != 1 {
		t.Errorf("expected 1 rule imported, got %d", result.RulesImported)
	}
	if result.RulesUnmatched != 0 {
		t.Errorf("expected 0 unmatched rules, got %d", result.RulesUnmatched)
	}

	// Verify the rule was linked to the imported integration
	var rules []db.CustomRule
	database.Find(&rules)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule in DB, got %d", len(rules))
	}
	if rules[0].IntegrationID == nil {
		t.Fatal("expected rule to have an integration ID")
	}

	var configs []db.IntegrationConfig
	database.Find(&configs)
	if len(configs) != 1 {
		t.Fatalf("expected 1 integration in DB, got %d", len(configs))
	}
	if *rules[0].IntegrationID != configs[0].ID {
		t.Errorf("expected rule integration ID %d to match imported integration ID %d",
			*rules[0].IntegrationID, configs[0].ID)
	}
}

// ---------- Validation ----------

func TestBackupService_Import_RejectsInvalidRuleEffect(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	envelope := SettingsExportEnvelope{
		Version: 1,
		Rules: []RuleExport{
			{Field: "quality", Operator: "==", Value: "4K", Effect: "bogus_effect", Enabled: true},
		},
	}

	sections := ImportSections{Rules: true}

	_, err := svc.Import(envelope, sections)
	if err == nil {
		t.Fatal("expected error for invalid rule effect")
	}
}

func TestBackupService_Import_RejectsInvalidIntegrationType(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	envelope := SettingsExportEnvelope{
		Version: 1,
		Integrations: []IntegrationExport{
			{Name: "Bad Integration", Type: "not_a_real_type", URL: "http://example.com", Enabled: true},
		},
	}

	sections := ImportSections{Integrations: true}

	_, err := svc.Import(envelope, sections)
	if err == nil {
		t.Fatal("expected error for invalid integration type")
	}
}

// ---------- Replace mode ----------

func TestBackupService_Import_ReplaceMode_Rules(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Seed an integration for rule matching
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://sonarr:8989",
		APIKey: "test-key", Enabled: true,
	})

	// Seed existing rules
	database.Create(&db.CustomRule{
		Field: "tag", Operator: "contains", Value: "anime", Effect: "prefer_keep", Enabled: true,
	})
	database.Create(&db.CustomRule{
		Field: "rating", Operator: ">", Value: "8.0", Effect: "always_keep", Enabled: true,
	})

	sonarrType := "sonarr"
	sonarrName := "Firefly Sonarr"

	envelope := SettingsExportEnvelope{
		Version: 1,
		Rules: []RuleExport{
			{Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep", Enabled: true, IntegrationType: &sonarrType, IntegrationName: &sonarrName},
		},
	}

	sections := ImportSections{Rules: true, Mode: ImportModeReplace}

	result, err := svc.Import(envelope, sections)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	if result.RulesImported != 1 {
		t.Errorf("expected 1 rule imported, got %d", result.RulesImported)
	}

	// Old rules should be gone, only the imported one remains
	var rules []db.CustomRule
	database.Find(&rules)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule in DB after replace, got %d", len(rules))
	}
	if rules[0].Field != "quality" {
		t.Errorf("expected imported rule field 'quality', got %q", rules[0].Field)
	}
}

// ---------- Preview ----------

func TestBackupService_PreviewImport_MatchedAndUnmatched(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Seed one sonarr integration
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://sonarr:8989",
		APIKey: "test-key", Enabled: true,
	})

	sonarrType := "sonarr"
	sonarrName := "Firefly Sonarr"
	radarrType := "radarr"
	radarrName := "Non-existent Radarr"

	envelope := SettingsExportEnvelope{
		Version: 1,
		Rules: []RuleExport{
			{
				Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep",
				Enabled: true, IntegrationType: &sonarrType, IntegrationName: &sonarrName,
			},
			{
				Field: "tag", Operator: "contains", Value: "anime", Effect: "prefer_keep",
				Enabled: true, IntegrationType: &radarrType, IntegrationName: &radarrName,
			},
			{
				Field: "monitored", Operator: "==", Value: "false", Effect: "always_remove",
				Enabled: true, // Global rule, no integration
			},
		},
	}

	sections := ImportSections{Rules: true}
	preview, err := svc.PreviewImport(envelope, sections)
	if err != nil {
		t.Fatalf("PreviewImport returned error: %v", err)
	}

	if len(preview.Rules) != 3 {
		t.Fatalf("expected 3 rule resolutions, got %d", len(preview.Rules))
	}

	// Rule 0: sonarr exact match
	if preview.Rules[0].Resolution != "matched" {
		t.Errorf("rule 0: expected resolution 'matched', got %q", preview.Rules[0].Resolution)
	}
	if preview.Rules[0].MatchedIntID == nil {
		t.Error("rule 0: expected matched integration ID")
	}

	// Rule 1: radarr unmatched (no radarr integration exists)
	if preview.Rules[1].Resolution != "unmatched" {
		t.Errorf("rule 1: expected resolution 'unmatched', got %q", preview.Rules[1].Resolution)
	}
	if preview.Rules[1].MatchedIntID != nil {
		t.Error("rule 1: expected nil matched integration ID")
	}

	// Rule 2: global (no integration reference) — unmatched since every rule must belong to an integration
	if preview.Rules[2].Resolution != "unmatched" {
		t.Errorf("rule 2: expected resolution 'unmatched' (global), got %q", preview.Rules[2].Resolution)
	}
}

// ---------- Commit with overrides ----------

func TestBackupService_CommitImport_WithOverrides(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Seed a radarr integration for manual assignment
	database.Create(&db.IntegrationConfig{
		Type: "radarr", Name: "Serenity Radarr", URL: "http://radarr:7878",
		APIKey: "test-key", Enabled: true,
	})

	var radarr db.IntegrationConfig
	database.First(&radarr)

	radarrType := "radarr"
	radarrName := "Non-existent Radarr"

	envelope := SettingsExportEnvelope{
		Version: 1,
		Rules: []RuleExport{
			{
				Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep",
				Enabled: true, IntegrationType: &radarrType, IntegrationName: &radarrName,
			},
			{
				Field: "tag", Operator: "contains", Value: "anime", Effect: "prefer_keep",
				Enabled: true,
			},
		},
	}

	sections := ImportSections{Rules: true}
	overrides := []RuleOverride{
		{Index: 0, IntegrationID: &radarr.ID, Skip: false},
		{Index: 1, Skip: true}, // skip the second rule
	}

	result, err := svc.CommitImport(envelope, sections, overrides)
	if err != nil {
		t.Fatalf("CommitImport returned error: %v", err)
	}

	if result.RulesImported != 1 {
		t.Errorf("expected 1 rule imported (1 skipped), got %d", result.RulesImported)
	}

	// Verify the rule was linked to the user-chosen integration
	var rules []db.CustomRule
	database.Find(&rules)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule in DB, got %d", len(rules))
	}
	if rules[0].IntegrationID == nil {
		t.Fatal("expected rule to have integration ID from override")
	}
	if *rules[0].IntegrationID != radarr.ID {
		t.Errorf("expected rule integration ID %d, got %d", radarr.ID, *rules[0].IntegrationID)
	}
}

// ---------- Sync mode per-item tests ----------

func TestBackupService_Import_SyncMode_DeletesOrphanIntegrations(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Seed 3 integrations
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://sonarr:8989",
		APIKey: "key-1", Enabled: true,
	})
	database.Create(&db.IntegrationConfig{
		Type: "radarr", Name: "Serenity Radarr", URL: "http://radarr:7878",
		APIKey: "key-2", Enabled: true,
	})
	database.Create(&db.IntegrationConfig{
		Type: "lidarr", Name: "Firefly Lidarr", URL: "http://lidarr:8686",
		APIKey: "key-3", Enabled: true,
	})

	// Import only 2 integrations in sync mode
	envelope := SettingsExportEnvelope{
		Version: 1,
		Integrations: []IntegrationExport{
			{Name: "Firefly Sonarr", Type: "sonarr", URL: "http://sonarr:8989", Enabled: true},
			{Name: "Serenity Radarr", Type: "radarr", URL: "http://radarr-new:7878", Enabled: true},
		},
	}

	sections := ImportSections{Integrations: true, Mode: ImportModeSync}
	result, err := svc.Import(envelope, sections)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	if result.IntegrationsImported != 2 {
		t.Errorf("expected 2 integrations imported, got %d", result.IntegrationsImported)
	}
	if result.ItemsDeleted != 1 {
		t.Errorf("expected 1 item deleted (orphan lidarr), got %d", result.ItemsDeleted)
	}

	// Verify only 2 remain
	var integrations []db.IntegrationConfig
	database.Find(&integrations)
	if len(integrations) != 2 {
		t.Fatalf("expected 2 integrations in DB after sync, got %d", len(integrations))
	}
}

func TestBackupService_Import_MergeMode_PreservesUnmatchedIntegrations(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Seed 3 integrations
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://sonarr:8989",
		APIKey: "key-1", Enabled: true,
	})
	database.Create(&db.IntegrationConfig{
		Type: "radarr", Name: "Serenity Radarr", URL: "http://radarr:7878",
		APIKey: "key-2", Enabled: true,
	})
	database.Create(&db.IntegrationConfig{
		Type: "lidarr", Name: "Firefly Lidarr", URL: "http://lidarr:8686",
		APIKey: "key-3", Enabled: true,
	})

	// Import only 2 integrations in merge mode
	envelope := SettingsExportEnvelope{
		Version: 1,
		Integrations: []IntegrationExport{
			{Name: "Firefly Sonarr", Type: "sonarr", URL: "http://sonarr:8989", Enabled: true},
			{Name: "Serenity Radarr", Type: "radarr", URL: "http://radarr:7878", Enabled: true},
		},
	}

	sections := ImportSections{Integrations: true, Mode: ImportModeMerge}
	result, err := svc.Import(envelope, sections)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	if result.IntegrationsImported != 2 {
		t.Errorf("expected 2 integrations imported, got %d", result.IntegrationsImported)
	}
	if result.ItemsDeleted != 0 {
		t.Errorf("expected 0 items deleted in merge mode, got %d", result.ItemsDeleted)
	}

	// All 3 should remain
	var integrations []db.IntegrationConfig
	database.Find(&integrations)
	if len(integrations) != 3 {
		t.Fatalf("expected 3 integrations in DB after merge, got %d", len(integrations))
	}
}

func TestBackupService_Import_SyncMode_DeletesOrphanRules(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Seed integration
	intID := seedIntegration(t, database)

	// Seed 3 existing rules
	database.Create(&db.CustomRule{
		Field: "tag", Operator: "contains", Value: "anime", Effect: "prefer_keep",
		Enabled: true, IntegrationID: &intID,
	})
	database.Create(&db.CustomRule{
		Field: "rating", Operator: ">", Value: "8.0", Effect: "always_keep",
		Enabled: true, IntegrationID: &intID,
	})
	database.Create(&db.CustomRule{
		Field: "monitored", Operator: "==", Value: "false", Effect: "always_remove",
		Enabled: true, IntegrationID: &intID,
	})

	sonarrType := "sonarr"
	sonarrName := "Test Sonarr"

	// Import 1 rule in sync mode
	envelope := SettingsExportEnvelope{
		Version: 1,
		Rules: []RuleExport{
			{Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep", Enabled: true, IntegrationType: &sonarrType, IntegrationName: &sonarrName},
		},
	}

	sections := ImportSections{Rules: true, Mode: ImportModeSync}
	result, err := svc.Import(envelope, sections)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	if result.RulesImported != 1 {
		t.Errorf("expected 1 rule imported, got %d", result.RulesImported)
	}
	if result.ItemsDeleted != 3 {
		t.Errorf("expected 3 orphaned rules deleted, got %d", result.ItemsDeleted)
	}

	// Only the imported rule should remain
	var rules []db.CustomRule
	database.Find(&rules)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule in DB after sync, got %d", len(rules))
	}
	if rules[0].Field != "quality" {
		t.Errorf("expected imported rule field 'quality', got %q", rules[0].Field)
	}
}

func TestBackupService_Import_SyncMode_DeletesOrphanNotifications(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Seed 2 notification channels
	database.Create(&db.NotificationConfig{
		Type: "discord", Name: "Firefly Alerts",
		WebhookURL: "https://discord.com/api/webhooks/1", Enabled: true,
	})
	database.Create(&db.NotificationConfig{
		Type: "apprise", Name: "Serenity Apprise",
		WebhookURL: "https://apprise.example.com", Enabled: true,
	})

	// Import only 1 channel in sync mode
	envelope := SettingsExportEnvelope{
		Version: 1,
		NotificationChannels: []NotificationExport{
			{Name: "Firefly Alerts", Type: "discord", Enabled: true},
		},
	}

	sections := ImportSections{NotificationChannels: true, Mode: ImportModeSync}
	result, err := svc.Import(envelope, sections)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	if result.NotificationChannelsImported != 1 {
		t.Errorf("expected 1 channel imported, got %d", result.NotificationChannelsImported)
	}
	if result.ItemsDeleted != 1 {
		t.Errorf("expected 1 orphaned channel deleted, got %d", result.ItemsDeleted)
	}

	var channels []db.NotificationConfig
	database.Find(&channels)
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel in DB after sync, got %d", len(channels))
	}
	if channels[0].Name != "Firefly Alerts" {
		t.Errorf("expected remaining channel 'Firefly Alerts', got %q", channels[0].Name)
	}
}

func TestBackupService_Import_SyncMode_CascadeDeletesOrphanRules(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Seed 2 integrations
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://sonarr:8989",
		APIKey: "key-1", Enabled: true,
	})
	database.Create(&db.IntegrationConfig{
		Type: "radarr", Name: "Serenity Radarr", URL: "http://radarr:7878",
		APIKey: "key-2", Enabled: true,
	})

	var radarr db.IntegrationConfig
	database.Where("type = ?", "radarr").First(&radarr)

	// Seed a rule scoped to the radarr integration that will be deleted
	database.Create(&db.CustomRule{
		Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep",
		Enabled: true, IntegrationID: &radarr.ID,
	})

	// Import only sonarr in sync mode — radarr and its rules should be deleted
	envelope := SettingsExportEnvelope{
		Version: 1,
		Integrations: []IntegrationExport{
			{Name: "Firefly Sonarr", Type: "sonarr", URL: "http://sonarr:8989", Enabled: true},
		},
	}

	sections := ImportSections{Integrations: true, Mode: ImportModeSync}
	result, err := svc.Import(envelope, sections)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	if result.IntegrationsImported != 1 {
		t.Errorf("expected 1 integration imported, got %d", result.IntegrationsImported)
	}

	// Radarr and its rule should be gone
	var integrations []db.IntegrationConfig
	database.Find(&integrations)
	if len(integrations) != 1 {
		t.Fatalf("expected 1 integration after sync, got %d", len(integrations))
	}

	var rules []db.CustomRule
	database.Find(&rules)
	if len(rules) != 0 {
		t.Errorf("expected 0 rules after integration cascade delete, got %d", len(rules))
	}
}

func TestBackupService_Import_SyncMode_IncludesPreImportSnapshot(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Seed integration
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://sonarr:8989",
		APIKey: "key-1", Enabled: true,
	})

	// Import in sync mode
	envelope := SettingsExportEnvelope{
		Version: 1,
		Integrations: []IntegrationExport{
			{Name: "Firefly Sonarr", Type: "sonarr", URL: "http://sonarr-new:8989", Enabled: true},
		},
	}

	sections := ImportSections{Integrations: true, Mode: ImportModeSync}
	result, err := svc.Import(envelope, sections)
	if err != nil {
		t.Fatalf("Import returned error: %v", err)
	}

	if result.PreImportSnapshot == nil {
		t.Fatal("expected pre-import snapshot in sync mode")
	}
	if len(result.PreImportSnapshot.Integrations) != 1 {
		t.Errorf("expected 1 integration in snapshot, got %d", len(result.PreImportSnapshot.Integrations))
	}
}

func TestBackupService_PreviewImport_AllSections(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Seed existing data
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://sonarr:8989",
		APIKey: "key-1", Enabled: true,
	})
	database.Create(&db.IntegrationConfig{
		Type: "radarr", Name: "Serenity Radarr", URL: "http://radarr:7878",
		APIKey: "key-2", Enabled: true,
	})
	database.Create(&db.NotificationConfig{
		Type: "discord", Name: "Firefly Alerts",
		WebhookURL: "https://discord.com/api/webhooks/1", Enabled: true,
	})
	database.Create(&db.DiskGroup{
		MountPath: "/mnt/media", ThresholdPct: 85, TargetPct: 75,
	})

	sonarrType := "sonarr"
	sonarrName := "Firefly Sonarr"

	envelope := SettingsExportEnvelope{
		Version: 1,
		Preferences: &PreferencesExport{
			LogLevel: "debug", ExecutionMode: "live",
			PollIntervalSeconds: 300, AuditLogRetentionDays: 30,
			TiebreakerMethod: "newest", DeletionsEnabled: true,
			SnoozeDurationHours: 24, CheckForUpdates: true,
		},
		Integrations: []IntegrationExport{
			{Name: "Firefly Sonarr", Type: "sonarr", URL: "http://sonarr-new:8989", Enabled: true},
			{Name: "New Lidarr", Type: "lidarr", URL: "http://lidarr:8686", Enabled: true},
		},
		Rules: []RuleExport{
			{Field: "quality", Operator: "==", Value: "4K", Effect: "always_keep",
				Enabled: true, IntegrationType: &sonarrType, IntegrationName: &sonarrName},
		},
		NotificationChannels: []NotificationExport{
			{Name: "Firefly Alerts", Type: "discord", Enabled: true},
			{Name: "New Apprise", Type: "apprise", Enabled: true},
		},
		DiskGroups: []DiskGroupExport{
			{MountPath: "/mnt/media", ThresholdPct: 90, TargetPct: 80},
		},
	}

	sections := ImportSections{
		Preferences: true, Rules: true, Integrations: true,
		NotificationChannels: true, DiskGroups: true, Mode: ImportModeSync,
	}

	preview, err := svc.PreviewImport(envelope, sections)
	if err != nil {
		t.Fatalf("PreviewImport returned error: %v", err)
	}

	// Check preferences
	if preview.Preferences == nil {
		t.Fatal("expected preferences preview")
	}
	if preview.Preferences.Action != "update" {
		t.Errorf("expected preferences action 'update', got %q", preview.Preferences.Action)
	}

	// Check integrations
	if len(preview.Integrations) != 2 {
		t.Fatalf("expected 2 integration resolutions, got %d", len(preview.Integrations))
	}
	// Sonarr should be update (URL changed)
	if preview.Integrations[0].Action != "update" {
		t.Errorf("expected sonarr action 'update', got %q", preview.Integrations[0].Action)
	}
	// Lidarr should be create (new)
	if preview.Integrations[1].Action != "create" {
		t.Errorf("expected lidarr action 'create', got %q", preview.Integrations[1].Action)
	}

	// Check rules
	if len(preview.Rules) != 1 {
		t.Fatalf("expected 1 rule resolution, got %d", len(preview.Rules))
	}
	if preview.Rules[0].Resolution != "matched" {
		t.Errorf("expected rule resolution 'matched', got %q", preview.Rules[0].Resolution)
	}

	// Check notifications
	if len(preview.Notifications) != 2 {
		t.Fatalf("expected 2 notification resolutions, got %d", len(preview.Notifications))
	}

	// Check disk groups
	if len(preview.DiskGroups) != 1 {
		t.Fatalf("expected 1 disk group resolution, got %d", len(preview.DiskGroups))
	}
	if preview.DiskGroups[0].Action != "update" {
		t.Errorf("expected disk group action 'update', got %q", preview.DiskGroups[0].Action)
	}

	// Check sync-mode deletions
	if preview.Deletions == nil {
		t.Fatal("expected deletion preview in sync mode")
	}
	if len(preview.Deletions.Integrations) != 1 {
		t.Errorf("expected 1 integration to delete (radarr), got %d", len(preview.Deletions.Integrations))
	}
}

func TestBackupService_Import_LegacyModes(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewBackupService(database, bus)
	svc.SetDiskGroupService(NewDiskGroupService(database, bus))

	// Verify legacy "append" works like merge (no deletions)
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://sonarr:8989",
		APIKey: "key-1", Enabled: true,
	})
	database.Create(&db.IntegrationConfig{
		Type: "radarr", Name: "Serenity Radarr", URL: "http://radarr:7878",
		APIKey: "key-2", Enabled: true,
	})

	envelope := SettingsExportEnvelope{
		Version: 1,
		Integrations: []IntegrationExport{
			{Name: "Firefly Sonarr", Type: "sonarr", URL: "http://sonarr:8989", Enabled: true},
		},
	}

	// Legacy "append" mode
	sections := ImportSections{Integrations: true, Mode: ImportModeAppend}
	result, err := svc.Import(envelope, sections)
	if err != nil {
		t.Fatalf("Import with legacy append mode returned error: %v", err)
	}
	if result.ItemsDeleted != 0 {
		t.Errorf("legacy append mode should not delete items, got %d deleted", result.ItemsDeleted)
	}

	var integrations []db.IntegrationConfig
	database.Find(&integrations)
	if len(integrations) != 2 {
		t.Errorf("expected 2 integrations preserved in append mode, got %d", len(integrations))
	}
}
