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

// Import mode constants.
const (
	// ImportModeAppend adds imported items alongside existing data (default).
	ImportModeAppend = "append"
	// ImportModeReplace deletes existing items before importing.
	ImportModeReplace = "replace"
)

// ImportSections controls which sections to import from an envelope.
type ImportSections struct {
	Preferences          bool   `json:"preferences"`
	Rules                bool   `json:"rules"`
	Integrations         bool   `json:"integrations"`
	DiskGroups           bool   `json:"diskGroups"`
	NotificationChannels bool   `json:"notificationChannels"`
	Mode                 string `json:"mode"` // "append" (default) or "replace"
}

// ImportResult reports what was imported.
type ImportResult struct {
	PreferencesImported          bool `json:"preferencesImported"`
	RulesImported                int  `json:"rulesImported"`
	RulesUnmatched               int  `json:"rulesUnmatched"`
	IntegrationsImported         int  `json:"integrationsImported"`
	DiskGroupsImported           int  `json:"diskGroupsImported"`
	NotificationChannelsImported int  `json:"notificationChannelsImported"`
}

// PreferencesExport contains all PreferenceSet fields except ID and UpdatedAt,
// plus scoring factor weights as a dynamic map.
type PreferencesExport struct {
	LogLevel              string         `json:"logLevel"`
	AuditLogRetentionDays int            `json:"auditLogRetentionDays"`
	PollIntervalSeconds   int            `json:"pollIntervalSeconds"`
	ExecutionMode         string         `json:"executionMode"`
	TiebreakerMethod      string         `json:"tiebreakerMethod"`
	DeletionsEnabled      bool           `json:"deletionsEnabled"`
	SnoozeDurationHours   int            `json:"snoozeDurationHours"`
	CheckForUpdates       bool           `json:"checkForUpdates"`
	FactorWeights         map[string]int `json:"factorWeights,omitempty"` // factor_key → weight (0-10)
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
		// Export scoring factor weights as a dynamic map
		var factorWeights []db.ScoringFactorWeight
		s.db.Find(&factorWeights)
		weightsMap := make(map[string]int, len(factorWeights))
		for _, fw := range factorWeights {
			weightsMap[fw.FactorKey] = fw.Weight
		}

		envelope.Preferences = &PreferencesExport{
			LogLevel:              pref.LogLevel,
			AuditLogRetentionDays: pref.AuditLogRetentionDays,
			PollIntervalSeconds:   pref.PollIntervalSeconds,
			ExecutionMode:         pref.ExecutionMode,
			TiebreakerMethod:      pref.TiebreakerMethod,
			DeletionsEnabled:      pref.DeletionsEnabled,
			SnoozeDurationHours:   pref.SnoozeDurationHours,
			CheckForUpdates:       pref.CheckForUpdates,
			FactorWeights:         weightsMap,
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
// All sections are imported within a single database transaction — if any section
// fails, all changes are rolled back.
func (s *BackupService) Import(envelope SettingsExportEnvelope, sections ImportSections) (*ImportResult, error) {
	if envelope.Version != 1 {
		return nil, fmt.Errorf("%w: got %d, expected 1", ErrUnsupportedVersion, envelope.Version)
	}

	// Default to append mode if not specified
	replaceMode := sections.Mode == ImportModeReplace

	// Begin wrapping transaction for the entire import
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin import transaction: %w", tx.Error)
	}

	result := &ImportResult{}

	// In replace mode, delete existing data for selected sections BEFORE importing.
	// Order matters: delete rules before integrations to avoid FK issues.
	if replaceMode {
		if err := s.deleteForReplace(tx, sections); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to clear existing data for replace: %w", err)
		}
	}

	if sections.Preferences && envelope.Preferences != nil {
		if err := s.importPreferences(tx, envelope.Preferences); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to import preferences: %w", err)
		}
		result.PreferencesImported = true
	}

	// Import integrations BEFORE rules so that rule auto-match can find
	// freshly-imported integrations by type+name.
	if sections.Integrations && len(envelope.Integrations) > 0 {
		count, err := s.importIntegrations(tx, envelope.Integrations)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to import integrations: %w", err)
		}
		result.IntegrationsImported = count
	}

	if sections.Rules && len(envelope.Rules) > 0 {
		count, unmatched, err := s.importRules(tx, envelope.Rules)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to import rules: %w", err)
		}
		result.RulesImported = count
		result.RulesUnmatched = unmatched
	}

	if sections.NotificationChannels && len(envelope.NotificationChannels) > 0 {
		count, err := s.importNotificationChannels(tx, envelope.NotificationChannels)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to import notification channels: %w", err)
		}
		result.NotificationChannelsImported = count
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit import transaction: %w", err)
	}

	// Disk groups are imported outside the main transaction because the
	// DiskGroupService uses its own *gorm.DB handle, which would deadlock
	// with SQLite's single-writer constraint if called inside a transaction.
	// Disk groups use upsert-by-mount-path semantics so they're idempotent.
	if sections.DiskGroups && len(envelope.DiskGroups) > 0 {
		count, err := s.importDiskGroups(envelope.DiskGroups)
		if err != nil {
			return nil, fmt.Errorf("failed to import disk groups: %w", err)
		}
		result.DiskGroupsImported = count
	}

	// Build list of imported section names for the event
	sectionNames := importedSectionNames(sections)
	s.bus.Publish(events.SettingsImportedEvent{
		Sections: sectionNames,
		Result: map[string]any{
			"preferencesImported":          result.PreferencesImported,
			"rulesImported":                result.RulesImported,
			"rulesUnmatched":               result.RulesUnmatched,
			"integrationsImported":         result.IntegrationsImported,
			"diskGroupsImported":           result.DiskGroupsImported,
			"notificationChannelsImported": result.NotificationChannelsImported,
		},
	})

	slog.Info("Settings imported", "component", "services", "sections", sectionNames, "mode", sections.Mode)

	return result, nil
}

// deleteForReplace removes existing data for selected sections before import.
func (s *BackupService) deleteForReplace(tx *gorm.DB, sections ImportSections) error {
	// Delete rules before integrations to avoid FK constraint issues
	if sections.Rules {
		if err := tx.Where("1 = 1").Delete(&db.CustomRule{}).Error; err != nil {
			return fmt.Errorf("failed to delete existing rules: %w", err)
		}
	}
	if sections.Integrations {
		// Also clear rules that reference integrations to avoid orphans
		if !sections.Rules {
			if err := tx.Where("integration_id IS NOT NULL").Delete(&db.CustomRule{}).Error; err != nil {
				return fmt.Errorf("failed to delete integration-scoped rules: %w", err)
			}
		}
		if err := tx.Where("1 = 1").Delete(&db.IntegrationConfig{}).Error; err != nil {
			return fmt.Errorf("failed to delete existing integrations: %w", err)
		}
	}
	if sections.NotificationChannels {
		if err := tx.Where("1 = 1").Delete(&db.NotificationConfig{}).Error; err != nil {
			return fmt.Errorf("failed to delete existing notification channels: %w", err)
		}
	}
	// DiskGroups and Preferences use upsert semantics, no delete needed
	return nil
}

// importPreferences updates the singleton PreferenceSet row and scoring factor weights.
func (s *BackupService) importPreferences(tx *gorm.DB, p *PreferencesExport) error {
	var pref db.PreferenceSet
	if err := tx.FirstOrCreate(&pref, db.PreferenceSet{ID: 1}).Error; err != nil {
		return err
	}

	pref.LogLevel = p.LogLevel
	pref.AuditLogRetentionDays = p.AuditLogRetentionDays
	pref.PollIntervalSeconds = p.PollIntervalSeconds
	pref.ExecutionMode = p.ExecutionMode
	pref.TiebreakerMethod = p.TiebreakerMethod
	pref.DeletionsEnabled = p.DeletionsEnabled
	pref.SnoozeDurationHours = p.SnoozeDurationHours
	pref.CheckForUpdates = p.CheckForUpdates

	if err := tx.Save(&pref).Error; err != nil {
		return err
	}

	// Import scoring factor weights (only update existing keys)
	for key, weight := range p.FactorWeights {
		if weight < 0 {
			weight = 0
		}
		if weight > 10 {
			weight = 10
		}
		tx.Model(&db.ScoringFactorWeight{}).
			Where("factor_key = ?", key).
			Updates(map[string]any{"weight": weight})
	}

	return nil
}

// importRules creates rules from the export payload, resolving integration
// names to IDs via auto-match. Returns (imported count, unmatched count, error).
//
// Match strategy (in order):
//  1. Exact match: type + name
//  2. Type-only fallback: type alone, only if exactly one integration of that type exists
//  3. No match: skip the rule and count as unmatched
func (s *BackupService) importRules(tx *gorm.DB, rules []RuleExport) (int, int, error) {
	// Validate all rules before importing any
	for i, r := range rules {
		if r.Field == "" || r.Operator == "" || r.Value == "" {
			return 0, 0, fmt.Errorf("rule %d: field, operator, and value are required", i)
		}
		if r.Effect == "" {
			return 0, 0, fmt.Errorf("rule %d: effect is required", i)
		}
		if !db.ValidEffects[r.Effect] {
			return 0, 0, fmt.Errorf("rule %d: invalid effect %q", i, r.Effect)
		}
	}

	// Build auto-match cache for integration lookups
	autoMatchCache := make(map[string]*uint)

	type resolvedRule struct {
		rule          RuleExport
		integrationID *uint
	}
	resolved := make([]resolvedRule, 0, len(rules))
	unmatched := 0

	for _, r := range rules {
		// Rule has no integration reference — skip it (every rule must belong to an integration)
		if (r.IntegrationName == nil || *r.IntegrationName == "") &&
			(r.IntegrationType == nil || *r.IntegrationType == "") {
			unmatched++
			slog.Warn("Rule has no integration reference, skipping",
				"component", "services",
				"field", r.Field,
				"operator", r.Operator,
				"value", r.Value,
			)
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
			if cachedID == nil {
				unmatched++
			}
			resolved = append(resolved, resolvedRule{rule: r, integrationID: cachedID})
			continue
		}

		// Strategy 1: Exact match by type and name
		var ic db.IntegrationConfig
		err := tx.Where("type = ? AND name = ?", intType, intName).First(&ic).Error
		if err == nil {
			id := ic.ID
			autoMatchCache[lookupKey] = &id
			resolved = append(resolved, resolvedRule{rule: r, integrationID: &id})
			continue
		}

		// Strategy 2: Type-only fallback — use if exactly one integration of this type exists
		if intType != "" {
			typeKey := intType + ":*"
			if cachedID, ok := autoMatchCache[typeKey]; ok {
				if cachedID == nil {
					unmatched++
				}
				autoMatchCache[lookupKey] = cachedID
				resolved = append(resolved, resolvedRule{rule: r, integrationID: cachedID})
				continue
			}

			var typeMatches []db.IntegrationConfig
			if dbErr := tx.Where("type = ?", intType).Find(&typeMatches).Error; dbErr == nil && len(typeMatches) == 1 {
				id := typeMatches[0].ID
				autoMatchCache[typeKey] = &id
				autoMatchCache[lookupKey] = &id
				slog.Warn("Rule integration matched by type-only fallback",
					"component", "services",
					"exportedName", intName,
					"matchedName", typeMatches[0].Name,
					"type", intType,
				)
				resolved = append(resolved, resolvedRule{rule: r, integrationID: &id})
				continue
			}
			// Ambiguous or empty — cache nil for the type key
			autoMatchCache[typeKey] = nil
		}

		// No match found — skip this rule (every rule must belong to an integration)
		autoMatchCache[lookupKey] = nil
		unmatched++
		slog.Warn("Rule integration match failed, skipping rule",
			"component", "services",
			"integrationName", intName,
			"integrationType", intType,
			"field", r.Field,
			"operator", r.Operator,
			"value", r.Value,
		)
	}

	// Determine the starting sort_order
	var maxOrder int
	row := tx.Model(&db.CustomRule{}).Select("COALESCE(MAX(sort_order), -1)").Row()
	if err := row.Scan(&maxOrder); err != nil {
		return 0, 0, fmt.Errorf("failed to determine rule ordering: %w", err)
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
			return 0, 0, fmt.Errorf("failed to insert imported rule: %w", err)
		}
		// GORM default:true tag ignores false on Create
		if !rr.rule.Enabled {
			if err := tx.Model(&newRule).Update("enabled", false).Error; err != nil {
				return 0, 0, fmt.Errorf("failed to disable imported rule: %w", err)
			}
		}
		nextOrder++
	}

	return len(resolved), unmatched, nil
}

// placeholderAPIKey is the sentinel value used for imported integrations
// that don't have a real API key yet.
const placeholderAPIKey = "PLACEHOLDER_REPLACE_ME"

// importIntegrations upserts integration configs by type+name.
// Existing integrations have their URL and Enabled state updated but API keys
// are preserved. New integrations are created with a placeholder API key and
// disabled until the user configures real credentials.
func (s *BackupService) importIntegrations(tx *gorm.DB, integrations []IntegrationExport) (int, error) {
	// Validate all integrations before importing any
	for i, ie := range integrations {
		if ie.Name == "" {
			return 0, fmt.Errorf("integration %d: name is required", i)
		}
		if ie.URL == "" {
			return 0, fmt.Errorf("integration %d (%s): url is required", i, ie.Name)
		}
		if !db.ValidIntegrationTypes[ie.Type] {
			return 0, fmt.Errorf("integration %d (%s): invalid type %q", i, ie.Name, ie.Type)
		}
	}

	count := 0
	for _, ie := range integrations {
		// Upsert: look up existing by type + name
		var existing db.IntegrationConfig
		err := tx.Where("type = ? AND name = ?", ie.Type, ie.Name).First(&existing).Error
		if err == nil {
			// Found — update URL and Enabled but preserve API key
			existing.URL = ie.URL
			existing.Enabled = ie.Enabled
			if dbErr := tx.Save(&existing).Error; dbErr != nil {
				return count, fmt.Errorf("failed to update integration %q: %w", ie.Name, dbErr)
			}
			count++
			continue
		}

		// Not found — create new with placeholder API key, forced disabled
		ic := db.IntegrationConfig{
			Name:    ie.Name,
			Type:    ie.Type,
			URL:     ie.URL,
			APIKey:  placeholderAPIKey,
			Enabled: true, // GORM default:true workaround — disable below
		}
		if dbErr := tx.Create(&ic).Error; dbErr != nil {
			return count, fmt.Errorf("failed to create integration %q: %w", ie.Name, dbErr)
		}
		// Force disable new imports with placeholder credentials
		if dbErr := tx.Model(&ic).Update("enabled", false).Error; dbErr != nil {
			return count, fmt.Errorf("failed to disable placeholder integration %q: %w", ie.Name, dbErr)
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

// placeholderWebhookURL is the sentinel value used for imported notification
// channels that don't have a real webhook URL yet.
const placeholderWebhookURL = "https://placeholder.example.com/replace-me"

// importNotificationChannels upserts notification channels by type+name.
// Existing channels have their subscription flags updated but webhook URLs
// are preserved. New channels are created with a placeholder webhook URL and
// disabled until the user configures real credentials.
func (s *BackupService) importNotificationChannels(tx *gorm.DB, channels []NotificationExport) (int, error) {
	// Validate all channels before importing any
	for i, ne := range channels {
		if ne.Name == "" {
			return 0, fmt.Errorf("notification channel %d: name is required", i)
		}
		if !db.ValidNotificationChannelTypes[ne.Type] {
			return 0, fmt.Errorf("notification channel %d (%s): invalid type %q", i, ne.Name, ne.Type)
		}
	}

	count := 0
	for _, ne := range channels {
		// Upsert: look up existing by type + name
		var existing db.NotificationConfig
		err := tx.Where("type = ? AND name = ?", ne.Type, ne.Name).First(&existing).Error
		if err == nil {
			// Found — update subscription flags but preserve webhook URL
			existing.Enabled = ne.Enabled
			existing.AppriseTags = ne.AppriseTags
			existing.OnCycleDigest = ne.OnCycleDigest
			existing.OnError = ne.OnError
			existing.OnModeChanged = ne.OnModeChanged
			existing.OnServerStarted = ne.OnServerStarted
			existing.OnThresholdBreach = ne.OnThresholdBreach
			existing.OnUpdateAvailable = ne.OnUpdateAvailable
			existing.OnApprovalActivity = ne.OnApprovalActivity
			if dbErr := tx.Save(&existing).Error; dbErr != nil {
				return count, fmt.Errorf("failed to update notification channel %q: %w", ne.Name, dbErr)
			}
			count++
			continue
		}

		// Not found — create new with placeholder webhook URL, forced disabled
		nc := db.NotificationConfig{
			Name:               ne.Name,
			Type:               ne.Type,
			WebhookURL:         placeholderWebhookURL,
			Enabled:            true, // GORM default:true workaround — disable below
			AppriseTags:        ne.AppriseTags,
			OnCycleDigest:      ne.OnCycleDigest,
			OnError:            ne.OnError,
			OnModeChanged:      ne.OnModeChanged,
			OnServerStarted:    ne.OnServerStarted,
			OnThresholdBreach:  ne.OnThresholdBreach,
			OnUpdateAvailable:  ne.OnUpdateAvailable,
			OnApprovalActivity: ne.OnApprovalActivity,
		}
		if dbErr := tx.Create(&nc).Error; dbErr != nil {
			return count, fmt.Errorf("failed to create notification channel %q: %w", ne.Name, dbErr)
		}
		// Force disable new imports with placeholder credentials
		if dbErr := tx.Model(&nc).Update("enabled", false).Error; dbErr != nil {
			return count, fmt.Errorf("failed to disable placeholder notification channel %q: %w", ne.Name, dbErr)
		}
		count++
	}
	return count, nil
}

// =============================================================================
// Import Preview (Phase 3)
// =============================================================================

// RuleResolution describes the match result for a single rule during import preview.
type RuleResolution struct {
	Index          int            `json:"index"`
	Rule           RuleExport     `json:"rule"`
	Resolution     string         `json:"resolution"` // "matched", "type_fallback", "unmatched"
	MatchedIntID   *uint          `json:"matchedIntegrationId"`
	MatchedIntName string         `json:"matchedIntegrationName,omitempty"`
	Candidates     []IntCandidate `json:"candidates"`
}

// IntCandidate represents an available integration for manual rule assignment.
type IntCandidate struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// ImportPreview reports what would happen if the import were executed.
type ImportPreview struct {
	Rules []RuleResolution `json:"rules"`
}

// RuleOverride allows the user to manually assign an integration to a specific rule.
type RuleOverride struct {
	Index         int   `json:"index"`         // position in the rules array
	IntegrationID *uint `json:"integrationId"` // user-chosen integration (nil = global)
	Skip          bool  `json:"skip"`          // true = don't import this rule
}

// PreviewImport analyzes the export envelope against the current database and
// reports how each rule would be resolved without committing any changes.
func (s *BackupService) PreviewImport(envelope SettingsExportEnvelope) (*ImportPreview, error) {
	preview := &ImportPreview{
		Rules: make([]RuleResolution, 0, len(envelope.Rules)),
	}

	// Load all integrations once for candidate lookup
	var allIntegrations []db.IntegrationConfig
	if err := s.db.Find(&allIntegrations).Error; err != nil {
		return nil, fmt.Errorf("failed to load integrations for preview: %w", err)
	}

	autoMatchCache := make(map[string]*matchResult)

	for i, r := range envelope.Rules {
		res := RuleResolution{
			Index: i,
			Rule:  r,
		}

		// Rule has no integration reference — mark as unmatched (every rule must belong to an integration)
		if (r.IntegrationName == nil || *r.IntegrationName == "") &&
			(r.IntegrationType == nil || *r.IntegrationType == "") {
			res.Resolution = "unmatched"
			preview.Rules = append(preview.Rules, res)
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

		// Check cache
		if cached, ok := autoMatchCache[lookupKey]; ok {
			res.Resolution = cached.resolution
			res.MatchedIntID = cached.id
			res.MatchedIntName = cached.name
			res.Candidates = candidatesForType(allIntegrations, intType)
			preview.Rules = append(preview.Rules, res)
			continue
		}

		// Strategy 1: Exact match by type and name
		matched := false
		for idx := range allIntegrations {
			ic := &allIntegrations[idx]
			if ic.Type == intType && ic.Name == intName {
				id := ic.ID
				res.Resolution = "matched"
				res.MatchedIntID = &id
				res.MatchedIntName = ic.Name
				autoMatchCache[lookupKey] = &matchResult{resolution: "matched", id: &id, name: ic.Name}
				matched = true
				break
			}
		}
		if matched {
			res.Candidates = candidatesForType(allIntegrations, intType)
			preview.Rules = append(preview.Rules, res)
			continue
		}

		// Strategy 2: Type-only fallback
		typeMatches := candidatesForType(allIntegrations, intType)
		if len(typeMatches) == 1 {
			id := typeMatches[0].ID
			res.Resolution = "type_fallback"
			res.MatchedIntID = &id
			res.MatchedIntName = typeMatches[0].Name
			autoMatchCache[lookupKey] = &matchResult{resolution: "type_fallback", id: &id, name: typeMatches[0].Name}
		} else {
			res.Resolution = "unmatched"
			autoMatchCache[lookupKey] = &matchResult{resolution: "unmatched"}
		}
		res.Candidates = typeMatches
		preview.Rules = append(preview.Rules, res)
	}

	return preview, nil
}

// CommitImport executes the import using user-provided overrides for rule
// integration assignments. Rules with overrides use the user-chosen integration
// ID instead of auto-match. Rules marked as Skip are excluded.
func (s *BackupService) CommitImport(envelope SettingsExportEnvelope, sections ImportSections, overrides []RuleOverride) (*ImportResult, error) {
	if envelope.Version != 1 {
		return nil, fmt.Errorf("%w: got %d, expected 1", ErrUnsupportedVersion, envelope.Version)
	}

	replaceMode := sections.Mode == ImportModeReplace

	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin import transaction: %w", tx.Error)
	}

	result := &ImportResult{}

	if replaceMode {
		if err := s.deleteForReplace(tx, sections); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to clear existing data for replace: %w", err)
		}
	}

	if sections.Preferences && envelope.Preferences != nil {
		if err := s.importPreferences(tx, envelope.Preferences); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to import preferences: %w", err)
		}
		result.PreferencesImported = true
	}

	if sections.Integrations && len(envelope.Integrations) > 0 {
		count, err := s.importIntegrations(tx, envelope.Integrations)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to import integrations: %w", err)
		}
		result.IntegrationsImported = count
	}

	if sections.Rules && len(envelope.Rules) > 0 {
		count, unmatched, err := s.importRulesWithOverrides(tx, envelope.Rules, overrides)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to import rules: %w", err)
		}
		result.RulesImported = count
		result.RulesUnmatched = unmatched
	}

	if sections.NotificationChannels && len(envelope.NotificationChannels) > 0 {
		count, err := s.importNotificationChannels(tx, envelope.NotificationChannels)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to import notification channels: %w", err)
		}
		result.NotificationChannelsImported = count
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit import transaction: %w", err)
	}

	// Disk groups imported outside transaction (see Import() comment)
	if sections.DiskGroups && len(envelope.DiskGroups) > 0 {
		count, err := s.importDiskGroups(envelope.DiskGroups)
		if err != nil {
			return nil, fmt.Errorf("failed to import disk groups: %w", err)
		}
		result.DiskGroupsImported = count
	}

	sectionNames := importedSectionNames(sections)
	s.bus.Publish(events.SettingsImportedEvent{
		Sections: sectionNames,
		Result: map[string]any{
			"preferencesImported":          result.PreferencesImported,
			"rulesImported":                result.RulesImported,
			"rulesUnmatched":               result.RulesUnmatched,
			"integrationsImported":         result.IntegrationsImported,
			"diskGroupsImported":           result.DiskGroupsImported,
			"notificationChannelsImported": result.NotificationChannelsImported,
		},
	})

	return result, nil
}

// importRulesWithOverrides creates rules using user-provided integration overrides
// instead of auto-match for overridden rules. Skipped rules are not imported.
func (s *BackupService) importRulesWithOverrides(tx *gorm.DB, rules []RuleExport, overrides []RuleOverride) (int, int, error) {
	// Validate all non-skipped rules before importing any
	overrideMap := make(map[int]RuleOverride, len(overrides))
	for _, o := range overrides {
		overrideMap[o.Index] = o
	}

	for i, r := range rules {
		if ov, ok := overrideMap[i]; ok && ov.Skip {
			continue
		}
		if r.Field == "" || r.Operator == "" || r.Value == "" {
			return 0, 0, fmt.Errorf("rule %d: field, operator, and value are required", i)
		}
		if r.Effect == "" {
			return 0, 0, fmt.Errorf("rule %d: effect is required", i)
		}
		if !db.ValidEffects[r.Effect] {
			return 0, 0, fmt.Errorf("rule %d: invalid effect %q", i, r.Effect)
		}
	}

	// Determine the starting sort_order
	var maxOrder int
	row := tx.Model(&db.CustomRule{}).Select("COALESCE(MAX(sort_order), -1)").Row()
	if err := row.Scan(&maxOrder); err != nil {
		return 0, 0, fmt.Errorf("failed to determine rule ordering: %w", err)
	}
	nextOrder := maxOrder + 1

	imported := 0
	unmatched := 0

	for i, r := range rules {
		// Check if this rule has a user override
		if ov, ok := overrideMap[i]; ok {
			if ov.Skip {
				continue
			}
			// Use user-chosen integration ID
			newRule := db.CustomRule{
				IntegrationID: ov.IntegrationID,
				Field:         r.Field,
				Operator:      r.Operator,
				Value:         r.Value,
				Effect:        r.Effect,
				Enabled:       true,
				SortOrder:     nextOrder,
			}
			if err := tx.Create(&newRule).Error; err != nil {
				return 0, 0, fmt.Errorf("failed to insert imported rule %d: %w", i, err)
			}
			if !r.Enabled {
				if err := tx.Model(&newRule).Update("enabled", false).Error; err != nil {
					return 0, 0, fmt.Errorf("failed to disable imported rule %d: %w", i, err)
				}
			}
			nextOrder++
			imported++
			continue
		}

		// No override — use auto-match (same logic as importRules)
		var integrationID *uint
		intName := ""
		intType := ""
		if r.IntegrationName != nil {
			intName = *r.IntegrationName
		}
		if r.IntegrationType != nil {
			intType = *r.IntegrationType
		}

		if intName != "" || intType != "" {
			// Try exact match
			var ic db.IntegrationConfig
			if err := tx.Where("type = ? AND name = ?", intType, intName).First(&ic).Error; err == nil {
				integrationID = &ic.ID
			} else if intType != "" {
				// Type-only fallback
				var typeMatches []db.IntegrationConfig
				if dbErr := tx.Where("type = ?", intType).Find(&typeMatches).Error; dbErr == nil && len(typeMatches) == 1 {
					integrationID = &typeMatches[0].ID
				}
			}
			if integrationID == nil {
				unmatched++
				slog.Warn("Rule integration match failed in override path, skipping rule",
					"component", "services",
					"field", r.Field,
					"operator", r.Operator,
					"value", r.Value,
				)
				continue
			}
		}

		// Every rule must have an integration — skip if still nil
		if integrationID == nil {
			unmatched++
			continue
		}

		newRule := db.CustomRule{
			IntegrationID: integrationID,
			Field:         r.Field,
			Operator:      r.Operator,
			Value:         r.Value,
			Effect:        r.Effect,
			Enabled:       true,
			SortOrder:     nextOrder,
		}
		if err := tx.Create(&newRule).Error; err != nil {
			return 0, 0, fmt.Errorf("failed to insert imported rule %d: %w", i, err)
		}
		if !r.Enabled {
			if err := tx.Model(&newRule).Update("enabled", false).Error; err != nil {
				return 0, 0, fmt.Errorf("failed to disable imported rule %d: %w", i, err)
			}
		}
		nextOrder++
		imported++
	}

	return imported, unmatched, nil
}

// matchResult caches the result of an integration lookup for preview.
type matchResult struct {
	resolution string
	id         *uint
	name       string
}

// candidatesForType returns all integrations matching the given type.
func candidatesForType(integrations []db.IntegrationConfig, intType string) []IntCandidate {
	candidates := make([]IntCandidate, 0)
	for _, ic := range integrations {
		if ic.Type == intType {
			candidates = append(candidates, IntCandidate{
				ID:   ic.ID,
				Name: ic.Name,
				Type: ic.Type,
			})
		}
	}
	return candidates
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
