package services

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

// ErrUnsupportedVersion is returned when an import envelope has an unsupported version.
var ErrUnsupportedVersion = errors.New("unsupported export version")

// =============================================================================
// Export envelope types
// =============================================================================

// SettingsExportEnvelope is the top-level structure for settings export files.
type SettingsExportEnvelope struct {
	Version              int                  `json:"version"`
	ExportedAt           string               `json:"exportedAt"`
	AppVersion           string               `json:"appVersion"`
	Preferences          *PreferencesExport   `json:"preferences,omitempty"`
	Rules                []RuleExport         `json:"rules,omitempty"`
	Integrations         []IntegrationExport  `json:"integrations,omitempty"`
	DiskGroups           []DiskGroupExport    `json:"diskGroups,omitempty"`
	NotificationChannels []NotificationExport `json:"notificationChannels,omitempty"`
}

// ExportSections controls which sections to include in the export.
type ExportSections struct {
	Preferences          bool `json:"preferences"`
	Rules                bool `json:"rules"`
	Integrations         bool `json:"integrations"`
	DiskGroups           bool `json:"diskGroups"`
	NotificationChannels bool `json:"notificationChannels"`
}

// ImportSections controls which sections to import from an envelope.
type ImportSections struct {
	Preferences          bool `json:"preferences"`
	Rules                bool `json:"rules"`
	Integrations         bool `json:"integrations"`
	DiskGroups           bool `json:"diskGroups"`
	NotificationChannels bool `json:"notificationChannels"`
}

// ImportResult reports what was imported.
type ImportResult struct {
	PreferencesImported          bool `json:"preferencesImported"`
	RulesImported                int  `json:"rulesImported"`
	IntegrationsImported         int  `json:"integrationsImported"`
	DiskGroupsImported           int  `json:"diskGroupsImported"`
	NotificationChannelsImported int  `json:"notificationChannelsImported"`
}

// PreferencesExport contains all PreferenceSet fields except ID and UpdatedAt.
type PreferencesExport struct {
	LogLevel              string `json:"logLevel"`
	AuditLogRetentionDays int    `json:"auditLogRetentionDays"`
	PollIntervalSeconds   int    `json:"pollIntervalSeconds"`
	WatchHistoryWeight    int    `json:"watchHistoryWeight"`
	LastWatchedWeight     int    `json:"lastWatchedWeight"`
	FileSizeWeight        int    `json:"fileSizeWeight"`
	RatingWeight          int    `json:"ratingWeight"`
	TimeInLibraryWeight   int    `json:"timeInLibraryWeight"`
	SeriesStatusWeight    int    `json:"seriesStatusWeight"`
	ExecutionMode         string `json:"executionMode"`
	TiebreakerMethod      string `json:"tiebreakerMethod"`
	DeletionsEnabled      bool   `json:"deletionsEnabled"`
	SnoozeDurationHours   int    `json:"snoozeDurationHours"`
	CheckForUpdates       bool   `json:"checkForUpdates"`
}

// RuleExport is a single rule in the portable export format.
type RuleExport struct {
	Field           string  `json:"field"`
	Operator        string  `json:"operator"`
	Value           string  `json:"value"`
	Effect          string  `json:"effect"`
	Enabled         bool    `json:"enabled"`
	IntegrationName *string `json:"integrationName"`
	IntegrationType *string `json:"integrationType"`
}

// IntegrationExport contains non-sensitive integration fields.
type IntegrationExport struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

// DiskGroupExport contains configuration-only disk group fields.
type DiskGroupExport struct {
	MountPath          string  `json:"mountPath"`
	ThresholdPct       float64 `json:"thresholdPct"`
	TargetPct          float64 `json:"targetPct"`
	TotalBytesOverride *int64  `json:"totalBytesOverride,omitempty"`
}

// NotificationExport contains non-sensitive notification channel fields.
type NotificationExport struct {
	Name               string `json:"name"`
	Type               string `json:"type"`
	Enabled            bool   `json:"enabled"`
	AppriseTags        string `json:"appriseTags,omitempty"`
	OnCycleDigest      bool   `json:"onCycleDigest"`
	OnError            bool   `json:"onError"`
	OnModeChanged      bool   `json:"onModeChanged"`
	OnServerStarted    bool   `json:"onServerStarted"`
	OnThresholdBreach  bool   `json:"onThresholdBreach"`
	OnUpdateAvailable  bool   `json:"onUpdateAvailable"`
	OnApprovalActivity bool   `json:"onApprovalActivity"`
}

// =============================================================================
// BackupService
// =============================================================================

// BackupService handles settings export and import operations.
type BackupService struct {
	db         *gorm.DB
	bus        *events.EventBus
	diskGroups *DiskGroupService
}

// NewBackupService creates a new BackupService.
func NewBackupService(database *gorm.DB, bus *events.EventBus) *BackupService {
	return &BackupService{db: database, bus: bus}
}

// SetDiskGroupService wires the DiskGroupService dependency for disk group
// export and import. Called by Registry after construction.
func (s *BackupService) SetDiskGroupService(dg *DiskGroupService) {
	s.diskGroups = dg
}

// Export produces a SettingsExportEnvelope containing the requested sections.
func (s *BackupService) Export(sections ExportSections, appVersion string) (*SettingsExportEnvelope, error) {
	envelope := &SettingsExportEnvelope{
		Version:    1,
		ExportedAt: time.Now().UTC().Format(time.RFC3339),
		AppVersion: appVersion,
	}

	if sections.Preferences {
		var pref db.PreferenceSet
		if err := s.db.FirstOrCreate(&pref, db.PreferenceSet{ID: 1}).Error; err != nil {
			return nil, fmt.Errorf("failed to fetch preferences for export: %w", err)
		}
		envelope.Preferences = &PreferencesExport{
			LogLevel:              pref.LogLevel,
			AuditLogRetentionDays: pref.AuditLogRetentionDays,
			PollIntervalSeconds:   pref.PollIntervalSeconds,
			WatchHistoryWeight:    pref.WatchHistoryWeight,
			LastWatchedWeight:     pref.LastWatchedWeight,
			FileSizeWeight:        pref.FileSizeWeight,
			RatingWeight:          pref.RatingWeight,
			TimeInLibraryWeight:   pref.TimeInLibraryWeight,
			SeriesStatusWeight:    pref.SeriesStatusWeight,
			ExecutionMode:         pref.ExecutionMode,
			TiebreakerMethod:      pref.TiebreakerMethod,
			DeletionsEnabled:      pref.DeletionsEnabled,
			SnoozeDurationHours:   pref.SnoozeDurationHours,
			CheckForUpdates:       pref.CheckForUpdates,
		}
	}

	if sections.Rules {
		var rules []db.CustomRule
		if err := s.db.Order("sort_order ASC, id ASC").Find(&rules).Error; err != nil {
			return nil, fmt.Errorf("failed to fetch rules for export: %w", err)
		}

		// Collect all referenced integration IDs
		integrationIDs := make([]uint, 0)
		for _, r := range rules {
			if r.IntegrationID != nil {
				integrationIDs = append(integrationIDs, *r.IntegrationID)
			}
		}

		// Batch-load referenced integrations
		integrationMap := make(map[uint]db.IntegrationConfig)
		if len(integrationIDs) > 0 {
			var configs []db.IntegrationConfig
			if err := s.db.Where("id IN ?", integrationIDs).Find(&configs).Error; err != nil {
				return nil, fmt.Errorf("failed to fetch integrations for rule export: %w", err)
			}
			for _, ic := range configs {
				integrationMap[ic.ID] = ic
			}
		}

		exported := make([]RuleExport, 0, len(rules))
		for _, r := range rules {
			re := RuleExport{
				Field:    r.Field,
				Operator: r.Operator,
				Value:    r.Value,
				Effect:   r.Effect,
				Enabled:  r.Enabled,
			}
			if r.IntegrationID != nil {
				if ic, ok := integrationMap[*r.IntegrationID]; ok {
					re.IntegrationName = &ic.Name
					re.IntegrationType = &ic.Type
				}
			}
			exported = append(exported, re)
		}
		envelope.Rules = exported
	}

	if sections.Integrations {
		var configs []db.IntegrationConfig
		if err := s.db.Find(&configs).Error; err != nil {
			return nil, fmt.Errorf("failed to fetch integrations for export: %w", err)
		}
		exported := make([]IntegrationExport, 0, len(configs))
		for _, ic := range configs {
			exported = append(exported, IntegrationExport{
				Name:    ic.Name,
				Type:    ic.Type,
				URL:     ic.URL,
				Enabled: ic.Enabled,
			})
		}
		envelope.Integrations = exported
	}

	if sections.DiskGroups && s.diskGroups != nil {
		groups, dgErr := s.diskGroups.List()
		if dgErr != nil {
			return nil, fmt.Errorf("failed to fetch disk groups for export: %w", dgErr)
		}
		exported := make([]DiskGroupExport, 0, len(groups))
		for _, dg := range groups {
			exported = append(exported, DiskGroupExport{
				MountPath:          dg.MountPath,
				ThresholdPct:       dg.ThresholdPct,
				TargetPct:          dg.TargetPct,
				TotalBytesOverride: dg.TotalBytesOverride,
			})
		}
		envelope.DiskGroups = exported
	}

	if sections.NotificationChannels {
		var channels []db.NotificationConfig
		if err := s.db.Find(&channels).Error; err != nil {
			return nil, fmt.Errorf("failed to fetch notification channels for export: %w", err)
		}
		exported := make([]NotificationExport, 0, len(channels))
		for _, nc := range channels {
			exported = append(exported, NotificationExport{
				Name:               nc.Name,
				Type:               nc.Type,
				Enabled:            nc.Enabled,
				AppriseTags:        nc.AppriseTags,
				OnCycleDigest:      nc.OnCycleDigest,
				OnError:            nc.OnError,
				OnModeChanged:      nc.OnModeChanged,
				OnServerStarted:    nc.OnServerStarted,
				OnThresholdBreach:  nc.OnThresholdBreach,
				OnUpdateAvailable:  nc.OnUpdateAvailable,
				OnApprovalActivity: nc.OnApprovalActivity,
			})
		}
		envelope.NotificationChannels = exported
	}

	// Build list of exported section names for the event
	sectionNames := exportedSectionNames(sections)
	s.bus.Publish(events.SettingsExportedEvent{Sections: sectionNames})

	slog.Info("Settings exported", "component", "services", "sections", sectionNames)

	return envelope, nil
}

// Import restores settings from a SettingsExportEnvelope for the requested sections.
func (s *BackupService) Import(envelope SettingsExportEnvelope, sections ImportSections) (*ImportResult, error) {
	if envelope.Version != 1 {
		return nil, fmt.Errorf("%w: got %d, expected 1", ErrUnsupportedVersion, envelope.Version)
	}

	result := &ImportResult{}

	if sections.Preferences && envelope.Preferences != nil {
		if err := s.importPreferences(envelope.Preferences); err != nil {
			return nil, fmt.Errorf("failed to import preferences: %w", err)
		}
		result.PreferencesImported = true
	}

	if sections.Rules && len(envelope.Rules) > 0 {
		count, err := s.importRules(envelope.Rules)
		if err != nil {
			return nil, fmt.Errorf("failed to import rules: %w", err)
		}
		result.RulesImported = count
	}

	if sections.Integrations && len(envelope.Integrations) > 0 {
		count, err := s.importIntegrations(envelope.Integrations)
		if err != nil {
			return nil, fmt.Errorf("failed to import integrations: %w", err)
		}
		result.IntegrationsImported = count
	}

	if sections.DiskGroups && len(envelope.DiskGroups) > 0 {
		count, err := s.importDiskGroups(envelope.DiskGroups)
		if err != nil {
			return nil, fmt.Errorf("failed to import disk groups: %w", err)
		}
		result.DiskGroupsImported = count
	}

	if sections.NotificationChannels && len(envelope.NotificationChannels) > 0 {
		count, err := s.importNotificationChannels(envelope.NotificationChannels)
		if err != nil {
			return nil, fmt.Errorf("failed to import notification channels: %w", err)
		}
		result.NotificationChannelsImported = count
	}

	// Build list of imported section names for the event
	sectionNames := importedSectionNames(sections)
	s.bus.Publish(events.SettingsImportedEvent{
		Sections: sectionNames,
		Result: map[string]any{
			"preferencesImported":          result.PreferencesImported,
			"rulesImported":                result.RulesImported,
			"integrationsImported":         result.IntegrationsImported,
			"diskGroupsImported":           result.DiskGroupsImported,
			"notificationChannelsImported": result.NotificationChannelsImported,
		},
	})

	slog.Info("Settings imported", "component", "services", "sections", sectionNames)

	return result, nil
}

// importPreferences updates the singleton PreferenceSet row.
func (s *BackupService) importPreferences(p *PreferencesExport) error {
	var pref db.PreferenceSet
	if err := s.db.FirstOrCreate(&pref, db.PreferenceSet{ID: 1}).Error; err != nil {
		return err
	}

	pref.LogLevel = p.LogLevel
	pref.AuditLogRetentionDays = p.AuditLogRetentionDays
	pref.PollIntervalSeconds = p.PollIntervalSeconds
	pref.WatchHistoryWeight = p.WatchHistoryWeight
	pref.LastWatchedWeight = p.LastWatchedWeight
	pref.FileSizeWeight = p.FileSizeWeight
	pref.RatingWeight = p.RatingWeight
	pref.TimeInLibraryWeight = p.TimeInLibraryWeight
	pref.SeriesStatusWeight = p.SeriesStatusWeight
	pref.ExecutionMode = p.ExecutionMode
	pref.TiebreakerMethod = p.TiebreakerMethod
	pref.DeletionsEnabled = p.DeletionsEnabled
	pref.SnoozeDurationHours = p.SnoozeDurationHours
	pref.CheckForUpdates = p.CheckForUpdates

	return s.db.Save(&pref).Error
}

// importRules creates rules from the export payload, resolving integration
// names to IDs via auto-match by type and name.
func (s *BackupService) importRules(rules []RuleExport) (int, error) {
	// Build auto-match cache for integration lookups
	autoMatchCache := make(map[string]*uint)

	type resolvedRule struct {
		rule          RuleExport
		integrationID *uint
	}
	resolved := make([]resolvedRule, 0, len(rules))

	for _, r := range rules {
		// Rule has no integration reference
		if (r.IntegrationName == nil || *r.IntegrationName == "") &&
			(r.IntegrationType == nil || *r.IntegrationType == "") {
			resolved = append(resolved, resolvedRule{rule: r, integrationID: nil})
			continue
		}

		intName := ""
		intType := ""
		if r.IntegrationName != nil {
			intName = *r.IntegrationName
		}
		if r.IntegrationType != nil {
			intType = *r.IntegrationType
		}
		lookupKey := intType + ":" + intName

		// Check auto-match cache
		if cachedID, ok := autoMatchCache[lookupKey]; ok {
			resolved = append(resolved, resolvedRule{rule: r, integrationID: cachedID})
			continue
		}

		// Auto-match by type and name
		var ic db.IntegrationConfig
		err := s.db.Where("type = ? AND name = ?", intType, intName).First(&ic).Error
		if err != nil {
			// No match found — import rule without integration binding
			autoMatchCache[lookupKey] = nil
			resolved = append(resolved, resolvedRule{rule: r, integrationID: nil})
			continue
		}
		id := ic.ID
		autoMatchCache[lookupKey] = &id
		resolved = append(resolved, resolvedRule{rule: r, integrationID: &id})
	}

	// Transactional insert
	tx := s.db.Begin()
	if tx.Error != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	// Determine the starting sort_order
	var maxOrder int
	row := tx.Model(&db.CustomRule{}).Select("COALESCE(MAX(sort_order), -1)").Row()
	if err := row.Scan(&maxOrder); err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("failed to determine rule ordering: %w", err)
	}
	nextOrder := maxOrder + 1

	for _, rr := range resolved {
		newRule := db.CustomRule{
			IntegrationID: rr.integrationID,
			Field:         rr.rule.Field,
			Operator:      rr.rule.Operator,
			Value:         rr.rule.Value,
			Effect:        rr.rule.Effect,
			Enabled:       true,
			SortOrder:     nextOrder,
		}
		if err := tx.Create(&newRule).Error; err != nil {
			tx.Rollback()
			return 0, fmt.Errorf("failed to insert imported rule: %w", err)
		}
		// GORM default:true tag ignores false on Create
		if !rr.rule.Enabled {
			if err := tx.Model(&newRule).Update("enabled", false).Error; err != nil {
				tx.Rollback()
				return 0, fmt.Errorf("failed to disable imported rule: %w", err)
			}
		}
		nextOrder++
	}

	if err := tx.Commit().Error; err != nil {
		return 0, fmt.Errorf("failed to commit imported rules: %w", err)
	}

	return len(resolved), nil
}

// importIntegrations creates integration configs with placeholder API keys.
func (s *BackupService) importIntegrations(integrations []IntegrationExport) (int, error) {
	count := 0
	for _, ie := range integrations {
		ic := db.IntegrationConfig{
			Name:    ie.Name,
			Type:    ie.Type,
			URL:     ie.URL,
			APIKey:  "PLACEHOLDER_REPLACE_ME",
			Enabled: ie.Enabled,
		}
		if err := s.db.Create(&ic).Error; err != nil {
			return count, fmt.Errorf("failed to create integration %q: %w", ie.Name, err)
		}
		count++
	}
	return count, nil
}

// importDiskGroups creates or updates disk groups by mount path via DiskGroupService.
func (s *BackupService) importDiskGroups(groups []DiskGroupExport) (int, error) {
	if s.diskGroups == nil {
		return 0, fmt.Errorf("disk group service not available")
	}
	count := 0
	for _, dge := range groups {
		if err := s.diskGroups.ImportUpsert(dge.MountPath, dge.ThresholdPct, dge.TargetPct, dge.TotalBytesOverride); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

// importNotificationChannels creates notification channels with placeholder webhook URLs.
func (s *BackupService) importNotificationChannels(channels []NotificationExport) (int, error) {
	count := 0
	for _, ne := range channels {
		nc := db.NotificationConfig{
			Name:               ne.Name,
			Type:               ne.Type,
			WebhookURL:         "https://placeholder.example.com/replace-me",
			Enabled:            ne.Enabled,
			AppriseTags:        ne.AppriseTags,
			OnCycleDigest:      ne.OnCycleDigest,
			OnError:            ne.OnError,
			OnModeChanged:      ne.OnModeChanged,
			OnServerStarted:    ne.OnServerStarted,
			OnThresholdBreach:  ne.OnThresholdBreach,
			OnUpdateAvailable:  ne.OnUpdateAvailable,
			OnApprovalActivity: ne.OnApprovalActivity,
		}
		if err := s.db.Create(&nc).Error; err != nil {
			return count, fmt.Errorf("failed to create notification channel %q: %w", ne.Name, err)
		}
		count++
	}
	return count, nil
}

// exportedSectionNames returns the names of sections included in an export.
func exportedSectionNames(s ExportSections) []string {
	names := make([]string, 0, 5)
	if s.Preferences {
		names = append(names, "preferences")
	}
	if s.Rules {
		names = append(names, "rules")
	}
	if s.Integrations {
		names = append(names, "integrations")
	}
	if s.DiskGroups {
		names = append(names, "diskGroups")
	}
	if s.NotificationChannels {
		names = append(names, "notificationChannels")
	}
	return names
}

// importedSectionNames returns the names of sections included in an import.
func importedSectionNames(s ImportSections) []string {
	names := make([]string, 0, 5)
	if s.Preferences {
		names = append(names, "preferences")
	}
	if s.Rules {
		names = append(names, "rules")
	}
	if s.Integrations {
		names = append(names, "integrations")
	}
	if s.DiskGroups {
		names = append(names, "diskGroups")
	}
	if s.NotificationChannels {
		names = append(names, "notificationChannels")
	}
	return names
}
