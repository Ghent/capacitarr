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
	// ImportModeMerge upserts matching items and creates new ones, leaving
	// existing unmatched items untouched. This is the default mode.
	ImportModeMerge = "merge"
	// ImportModeSync upserts matching items, creates new ones, and deletes
	// existing items that are not present in the import file — making the
	// database match the file exactly for the selected sections.
	ImportModeSync = "sync"

	// Deprecated: Use ImportModeMerge instead. Kept for backward compatibility.
	ImportModeAppend = "append"
	// Deprecated: Use ImportModeSync instead. Kept for backward compatibility.
	ImportModeReplace = "replace"
)

// isSyncMode returns true if the mode string indicates sync/replace semantics.
func isSyncMode(mode string) bool {
	return mode == ImportModeSync || mode == ImportModeReplace
}

// ImportSections controls which sections to import from an envelope.
type ImportSections struct {
	Preferences          bool   `json:"preferences"`
	Rules                bool   `json:"rules"`
	Integrations         bool   `json:"integrations"`
	DiskGroups           bool   `json:"diskGroups"`
	NotificationChannels bool   `json:"notificationChannels"`
	Mode                 string `json:"mode"` // "merge" (default) or "sync"
}

// ImportResult reports what was imported.
type ImportResult struct {
	PreferencesImported          bool                    `json:"preferencesImported"`
	RulesImported                int                     `json:"rulesImported"`
	RulesUnmatched               int                     `json:"rulesUnmatched"`
	IntegrationsImported         int                     `json:"integrationsImported"`
	DiskGroupsImported           int                     `json:"diskGroupsImported"`
	NotificationChannelsImported int                     `json:"notificationChannelsImported"`
	ItemsDeleted                 int                     `json:"itemsDeleted"`
	PreImportSnapshot            *SettingsExportEnvelope `json:"preImportSnapshot,omitempty"`
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
//
// In merge mode (default), items are upserted alongside existing data.
// In sync mode, items are upserted and existing items NOT in the import file
// are deleted — making the DB match the file exactly for selected sections.
func (s *BackupService) Import(envelope SettingsExportEnvelope, sections ImportSections) (*ImportResult, error) {
	if envelope.Version != 1 {
		return nil, fmt.Errorf("%w: got %d, expected 1", ErrUnsupportedVersion, envelope.Version)
	}

	syncMode := isSyncMode(sections.Mode)

	// Capture pre-import snapshot for safety (sync mode always, merge mode optional)
	var snapshot *SettingsExportEnvelope
	if syncMode {
		snap, err := s.Export(sectionsToExportSections(sections), "pre-import-snapshot")
		if err != nil {
			slog.Warn("Failed to create pre-import snapshot", "component", "services", "error", err)
		} else {
			snapshot = snap
		}
	}

	// Begin wrapping transaction for the entire import
	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin import transaction: %w", tx.Error)
	}

	result := &ImportResult{}

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
		count, deleted, err := s.importIntegrations(tx, envelope.Integrations, syncMode)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to import integrations: %w", err)
		}
		result.IntegrationsImported = count
		result.ItemsDeleted += deleted
	}

	if sections.Rules && len(envelope.Rules) > 0 {
		count, unmatched, deleted, err := s.importRules(tx, envelope.Rules, syncMode)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to import rules: %w", err)
		}
		result.RulesImported = count
		result.RulesUnmatched = unmatched
		result.ItemsDeleted += deleted
	}

	if sections.NotificationChannels && len(envelope.NotificationChannels) > 0 {
		count, deleted, err := s.importNotificationChannels(tx, envelope.NotificationChannels, syncMode)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to import notification channels: %w", err)
		}
		result.NotificationChannelsImported = count
		result.ItemsDeleted += deleted
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit import transaction: %w", err)
	}

	// Disk groups are imported outside the main transaction because the
	// DiskGroupService uses its own *gorm.DB handle, which would deadlock
	// with SQLite's single-writer constraint if called inside a transaction.
	if sections.DiskGroups && len(envelope.DiskGroups) > 0 {
		count, deleted, err := s.importDiskGroups(envelope.DiskGroups, syncMode)
		if err != nil {
			return nil, fmt.Errorf("failed to import disk groups: %w", err)
		}
		result.DiskGroupsImported = count
		result.ItemsDeleted += deleted
	}

	result.PreImportSnapshot = snapshot

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
			"itemsDeleted":                 result.ItemsDeleted,
		},
	})

	slog.Info("Settings imported", "component", "services", "sections", sectionNames, "mode", sections.Mode)

	return result, nil
}

// sectionsToExportSections converts ImportSections to ExportSections for pre-import snapshot.
func sectionsToExportSections(s ImportSections) ExportSections {
	return ExportSections{
		Preferences:          s.Preferences,
		Rules:                s.Rules,
		Integrations:         s.Integrations,
		DiskGroups:           s.DiskGroups,
		NotificationChannels: s.NotificationChannels,
	}
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
// names to IDs via auto-match. Returns (imported count, unmatched count, deleted count, error).
// In sync mode, existing rules not matched to an import entry are deleted.
//
// Match strategy (in order):
//  1. Exact match: type + name
//  2. Type-only fallback: type alone, only if exactly one integration of that type exists
//  3. No match: skip the rule and count as unmatched
func (s *BackupService) importRules(tx *gorm.DB, rules []RuleExport, syncMode bool) (int, int, int, error) {
	// Validate all rules before importing any
	for i, r := range rules {
		if r.Field == "" || r.Operator == "" || r.Value == "" {
			return 0, 0, 0, fmt.Errorf("rule %d: field, operator, and value are required", i)
		}
		if r.Effect == "" {
			return 0, 0, 0, fmt.Errorf("rule %d: effect is required", i)
		}
		if !db.ValidEffects[r.Effect] {
			return 0, 0, 0, fmt.Errorf("rule %d: invalid effect %q", i, r.Effect)
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
		return 0, 0, 0, fmt.Errorf("failed to determine rule ordering: %w", err)
	}
	nextOrder := maxOrder + 1

	// Track IDs of rules created/touched during import for sync-mode cleanup
	touchedIDs := make([]uint, 0, len(resolved))

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
			return 0, 0, 0, fmt.Errorf("failed to insert imported rule: %w", err)
		}
		touchedIDs = append(touchedIDs, newRule.ID)
		// GORM default:true tag ignores false on Create
		if !rr.rule.Enabled {
			if err := tx.Model(&newRule).Update("enabled", false).Error; err != nil {
				return 0, 0, 0, fmt.Errorf("failed to disable imported rule: %w", err)
			}
		}
		nextOrder++
	}

	// Sync mode: delete rules that were not created during this import
	deleted := 0
	if syncMode && len(touchedIDs) > 0 {
		result := tx.Where("id NOT IN ?", touchedIDs).Delete(&db.CustomRule{})
		if result.Error != nil {
			return len(resolved), unmatched, 0, fmt.Errorf("failed to delete orphaned rules: %w", result.Error)
		}
		deleted = int(result.RowsAffected)
		if deleted > 0 {
			slog.Info("Sync mode: deleted orphaned rules", "component", "services", "count", deleted)
		}
	} else if syncMode && len(touchedIDs) == 0 {
		// All rules unmatched but sync mode — delete everything
		result := tx.Where("1 = 1").Delete(&db.CustomRule{})
		if result.Error != nil {
			return 0, unmatched, 0, fmt.Errorf("failed to delete all rules in sync mode: %w", result.Error)
		}
		deleted = int(result.RowsAffected)
	}

	return len(resolved), unmatched, deleted, nil
}

// placeholderAPIKey is the sentinel value used for imported integrations
// that don't have a real API key yet.
const placeholderAPIKey = "PLACEHOLDER_REPLACE_ME"

// importIntegrations upserts integration configs by type+name.
// Existing integrations have their URL and Enabled state updated but API keys
// are preserved. New integrations are created with a placeholder API key and
// disabled until the user configures real credentials.
// In sync mode, integrations not in the import file are deleted.
// Returns (upserted count, deleted count, error).
func (s *BackupService) importIntegrations(tx *gorm.DB, integrations []IntegrationExport, syncMode bool) (int, int, error) {
	// Validate all integrations before importing any
	for i, ie := range integrations {
		if ie.Name == "" {
			return 0, 0, fmt.Errorf("integration %d: name is required", i)
		}
		if ie.URL == "" {
			return 0, 0, fmt.Errorf("integration %d (%s): url is required", i, ie.Name)
		}
		if !db.ValidIntegrationTypes[ie.Type] {
			return 0, 0, fmt.Errorf("integration %d (%s): invalid type %q", i, ie.Name, ie.Type)
		}
	}

	// Build a set of imported (type, name) for sync-mode orphan detection
	importedKeys := make(map[string]bool, len(integrations))

	count := 0
	for _, ie := range integrations {
		importedKeys[ie.Type+":"+ie.Name] = true

		// Upsert: look up existing by type + name
		var existing db.IntegrationConfig
		err := tx.Where("type = ? AND name = ?", ie.Type, ie.Name).First(&existing).Error
		if err == nil {
			// Found — update URL and Enabled but preserve API key
			existing.URL = ie.URL
			existing.Enabled = ie.Enabled
			if dbErr := tx.Save(&existing).Error; dbErr != nil {
				return count, 0, fmt.Errorf("failed to update integration %q: %w", ie.Name, dbErr)
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
			return count, 0, fmt.Errorf("failed to create integration %q: %w", ie.Name, dbErr)
		}
		// Force disable new imports with placeholder credentials
		if dbErr := tx.Model(&ic).Update("enabled", false).Error; dbErr != nil {
			return count, 0, fmt.Errorf("failed to disable placeholder integration %q: %w", ie.Name, dbErr)
		}
		count++
	}

	// Sync mode: delete integrations not present in the import file
	deleted := 0
	if syncMode {
		var allExisting []db.IntegrationConfig
		if err := tx.Find(&allExisting).Error; err != nil {
			return count, 0, fmt.Errorf("failed to list integrations for sync: %w", err)
		}
		for _, existing := range allExisting {
			if !importedKeys[existing.Type+":"+existing.Name] {
				// Cascade: delete rules referencing this integration
				if err := tx.Where("integration_id = ?", existing.ID).Delete(&db.CustomRule{}).Error; err != nil {
					return count, deleted, fmt.Errorf("failed to delete rules for orphaned integration %q: %w", existing.Name, err)
				}
				if err := tx.Delete(&existing).Error; err != nil {
					return count, deleted, fmt.Errorf("failed to delete orphaned integration %q: %w", existing.Name, err)
				}
				deleted++
				slog.Info("Sync mode: deleted orphaned integration",
					"component", "services", "name", existing.Name, "type", existing.Type)
			}
		}
	}

	return count, deleted, nil
}

// importDiskGroups creates or updates disk groups by mount path via DiskGroupService.
// In sync mode, disk groups not in the import file are deleted.
// Returns (upserted count, deleted count, error).
func (s *BackupService) importDiskGroups(groups []DiskGroupExport, syncMode bool) (int, int, error) {
	if s.diskGroups == nil {
		return 0, 0, fmt.Errorf("disk group service not available")
	}

	// Build set of imported mount paths for sync-mode orphan detection
	importedPaths := make(map[string]bool, len(groups))

	count := 0
	for _, dge := range groups {
		importedPaths[dge.MountPath] = true
		if err := s.diskGroups.ImportUpsert(dge.MountPath, dge.ThresholdPct, dge.TargetPct, dge.TotalBytesOverride); err != nil {
			return count, 0, err
		}
		count++
	}

	// Sync mode: delete disk groups not present in the import file
	deleted := 0
	if syncMode {
		allGroups, err := s.diskGroups.List()
		if err != nil {
			return count, 0, fmt.Errorf("failed to list disk groups for sync: %w", err)
		}
		for _, g := range allGroups {
			if !importedPaths[g.MountPath] {
				if delErr := s.db.Delete(&g).Error; delErr != nil {
					return count, deleted, fmt.Errorf("failed to delete orphaned disk group %q: %w", g.MountPath, delErr)
				}
				deleted++
				slog.Info("Sync mode: deleted orphaned disk group",
					"component", "services", "mountPath", g.MountPath)
			}
		}
	}

	return count, deleted, nil
}

// placeholderWebhookURL is the sentinel value used for imported notification
// channels that don't have a real webhook URL yet.
const placeholderWebhookURL = "https://placeholder.example.com/replace-me"

// importNotificationChannels upserts notification channels by type+name.
// Existing channels have their subscription flags updated but webhook URLs
// are preserved. New channels are created with a placeholder webhook URL and
// disabled until the user configures real credentials.
// In sync mode, channels not in the import file are deleted.
// Returns (upserted count, deleted count, error).
func (s *BackupService) importNotificationChannels(tx *gorm.DB, channels []NotificationExport, syncMode bool) (int, int, error) {
	// Validate all channels before importing any
	for i, ne := range channels {
		if ne.Name == "" {
			return 0, 0, fmt.Errorf("notification channel %d: name is required", i)
		}
		if !db.ValidNotificationChannelTypes[ne.Type] {
			return 0, 0, fmt.Errorf("notification channel %d (%s): invalid type %q", i, ne.Name, ne.Type)
		}
	}

	// Build a set of imported (type, name) for sync-mode orphan detection
	importedKeys := make(map[string]bool, len(channels))

	count := 0
	for _, ne := range channels {
		importedKeys[ne.Type+":"+ne.Name] = true

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
				return count, 0, fmt.Errorf("failed to update notification channel %q: %w", ne.Name, dbErr)
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
			return count, 0, fmt.Errorf("failed to create notification channel %q: %w", ne.Name, dbErr)
		}
		// Force disable new imports with placeholder credentials
		if dbErr := tx.Model(&nc).Update("enabled", false).Error; dbErr != nil {
			return count, 0, fmt.Errorf("failed to disable placeholder notification channel %q: %w", ne.Name, dbErr)
		}
		count++
	}

	// Sync mode: delete notification channels not present in the import file
	deleted := 0
	if syncMode {
		var allExisting []db.NotificationConfig
		if err := tx.Find(&allExisting).Error; err != nil {
			return count, 0, fmt.Errorf("failed to list notification channels for sync: %w", err)
		}
		for _, existing := range allExisting {
			if !importedKeys[existing.Type+":"+existing.Name] {
				if err := tx.Delete(&existing).Error; err != nil {
					return count, deleted, fmt.Errorf("failed to delete orphaned notification channel %q: %w", existing.Name, err)
				}
				deleted++
				slog.Info("Sync mode: deleted orphaned notification channel",
					"component", "services", "name", existing.Name, "type", existing.Type)
			}
		}
	}

	return count, deleted, nil
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

// Preview action constants for ItemResolution and PreferencesResolution.
const (
	previewActionCreate    = "create"
	previewActionUpdate    = "update"
	previewActionUnchanged = "unchanged"
)

// ItemResolution describes what will happen to a single item during import.
type ItemResolution struct {
	Name    string        `json:"name"`
	Type    string        `json:"type,omitempty"`
	Action  string        `json:"action"` // "create", "update", "unchanged"
	Changes []FieldChange `json:"changes,omitempty"`
}

// FieldChange describes a single field-level change.
type FieldChange struct {
	Field    string `json:"field"`
	OldValue string `json:"oldValue"`
	NewValue string `json:"newValue"`
}

// PreferencesResolution describes changes to the singleton preferences.
type PreferencesResolution struct {
	Action  string        `json:"action"` // "update", "unchanged"
	Changes []FieldChange `json:"changes,omitempty"`
}

// DeletionPreview lists items that would be deleted in sync mode.
type DeletionPreview struct {
	Rules         []string `json:"rules,omitempty"`
	Integrations  []string `json:"integrations,omitempty"`
	Notifications []string `json:"notifications,omitempty"`
	DiskGroups    []string `json:"diskGroups,omitempty"`
}

// ImportPreview reports what would happen if the import were executed.
type ImportPreview struct {
	Rules         []RuleResolution       `json:"rules"`
	Integrations  []ItemResolution       `json:"integrations,omitempty"`
	Notifications []ItemResolution       `json:"notifications,omitempty"`
	DiskGroups    []ItemResolution       `json:"diskGroups,omitempty"`
	Preferences   *PreferencesResolution `json:"preferences,omitempty"`
	Deletions     *DeletionPreview       `json:"deletions,omitempty"`
}

// RuleOverride allows the user to manually assign an integration to a specific rule.
type RuleOverride struct {
	Index         int   `json:"index"`         // position in the rules array
	IntegrationID *uint `json:"integrationId"` // user-chosen integration (nil = global)
	Skip          bool  `json:"skip"`          // true = don't import this rule
}

// PreviewImport analyzes the export envelope against the current database and
// reports how each section would be affected without committing any changes.
// The sections parameter controls which sections to preview; syncMode controls
// whether orphan deletions are reported.
func (s *BackupService) PreviewImport(envelope SettingsExportEnvelope, sections ImportSections) (*ImportPreview, error) {
	syncMode := isSyncMode(sections.Mode)
	preview := &ImportPreview{
		Rules: make([]RuleResolution, 0, len(envelope.Rules)),
	}

	// Load all integrations once (needed for rules AND integration preview)
	var allIntegrations []db.IntegrationConfig
	if err := s.db.Find(&allIntegrations).Error; err != nil {
		return nil, fmt.Errorf("failed to load integrations for preview: %w", err)
	}

	// --- Preferences preview ---
	if sections.Preferences && envelope.Preferences != nil {
		preview.Preferences = s.previewPreferences(envelope.Preferences)
	}

	// --- Integration preview ---
	if sections.Integrations && len(envelope.Integrations) > 0 {
		preview.Integrations = s.previewIntegrations(envelope.Integrations, allIntegrations)
	}

	// --- Rules preview ---
	if sections.Rules && len(envelope.Rules) > 0 {
		preview.Rules = s.previewRules(envelope.Rules, allIntegrations)
	}

	// --- Notification preview ---
	if sections.NotificationChannels && len(envelope.NotificationChannels) > 0 {
		preview.Notifications = s.previewNotifications(envelope.NotificationChannels)
	}

	// --- Disk group preview ---
	if sections.DiskGroups && len(envelope.DiskGroups) > 0 && s.diskGroups != nil {
		preview.DiskGroups = s.previewDiskGroups(envelope.DiskGroups)
	}

	// --- Sync-mode deletion preview ---
	if syncMode {
		preview.Deletions = s.previewDeletions(envelope, sections, allIntegrations)
	}

	return preview, nil
}

// previewPreferences compares import preferences against current values.
func (s *BackupService) previewPreferences(p *PreferencesExport) *PreferencesResolution {
	var pref db.PreferenceSet
	s.db.FirstOrCreate(&pref, db.PreferenceSet{ID: 1})

	changes := make([]FieldChange, 0)
	addChange := func(field, oldVal, newVal string) {
		if oldVal != newVal {
			changes = append(changes, FieldChange{Field: field, OldValue: oldVal, NewValue: newVal})
		}
	}

	addChange("logLevel", pref.LogLevel, p.LogLevel)
	addChange("executionMode", pref.ExecutionMode, p.ExecutionMode)
	addChange("tiebreakerMethod", pref.TiebreakerMethod, p.TiebreakerMethod)
	addChange("pollIntervalSeconds", fmt.Sprintf("%d", pref.PollIntervalSeconds), fmt.Sprintf("%d", p.PollIntervalSeconds))
	addChange("auditLogRetentionDays", fmt.Sprintf("%d", pref.AuditLogRetentionDays), fmt.Sprintf("%d", p.AuditLogRetentionDays))
	addChange("snoozeDurationHours", fmt.Sprintf("%d", pref.SnoozeDurationHours), fmt.Sprintf("%d", p.SnoozeDurationHours))
	addChange("deletionsEnabled", fmt.Sprintf("%v", pref.DeletionsEnabled), fmt.Sprintf("%v", p.DeletionsEnabled))
	addChange("checkForUpdates", fmt.Sprintf("%v", pref.CheckForUpdates), fmt.Sprintf("%v", p.CheckForUpdates))

	action := previewActionUnchanged
	if len(changes) > 0 {
		action = previewActionUpdate
	}
	return &PreferencesResolution{Action: action, Changes: changes}
}

// previewIntegrations checks each imported integration against the current DB.
func (s *BackupService) previewIntegrations(imports []IntegrationExport, existing []db.IntegrationConfig) []ItemResolution {
	results := make([]ItemResolution, 0, len(imports))
	existMap := make(map[string]*db.IntegrationConfig, len(existing))
	for i := range existing {
		existMap[existing[i].Type+":"+existing[i].Name] = &existing[i]
	}

	for _, ie := range imports {
		key := ie.Type + ":" + ie.Name
		if ex, ok := existMap[key]; ok {
			changes := make([]FieldChange, 0)
			if ex.URL != ie.URL {
				changes = append(changes, FieldChange{Field: "url", OldValue: ex.URL, NewValue: ie.URL})
			}
			if ex.Enabled != ie.Enabled {
				changes = append(changes, FieldChange{Field: "enabled", OldValue: fmt.Sprintf("%v", ex.Enabled), NewValue: fmt.Sprintf("%v", ie.Enabled)})
			}
			action := previewActionUnchanged
			if len(changes) > 0 {
				action = previewActionUpdate
			}
			results = append(results, ItemResolution{Name: ie.Name, Type: ie.Type, Action: action, Changes: changes})
		} else {
			results = append(results, ItemResolution{Name: ie.Name, Type: ie.Type, Action: previewActionCreate})
		}
	}
	return results
}

// previewRules runs rule matching logic without committing.
func (s *BackupService) previewRules(rules []RuleExport, allIntegrations []db.IntegrationConfig) []RuleResolution {
	results := make([]RuleResolution, 0, len(rules))
	autoMatchCache := make(map[string]*matchResult)

	for i, r := range rules {
		res := RuleResolution{Index: i, Rule: r}

		if (r.IntegrationName == nil || *r.IntegrationName == "") &&
			(r.IntegrationType == nil || *r.IntegrationType == "") {
			res.Resolution = "unmatched"
			results = append(results, res)
			continue
		}

		intName, intType := "", ""
		if r.IntegrationName != nil {
			intName = *r.IntegrationName
		}
		if r.IntegrationType != nil {
			intType = *r.IntegrationType
		}
		lookupKey := intType + ":" + intName

		if cached, ok := autoMatchCache[lookupKey]; ok {
			res.Resolution = cached.resolution
			res.MatchedIntID = cached.id
			res.MatchedIntName = cached.name
			res.Candidates = candidatesForType(allIntegrations, intType)
			results = append(results, res)
			continue
		}

		// Strategy 1: Exact match
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
			results = append(results, res)
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
		results = append(results, res)
	}
	return results
}

// previewNotifications checks each imported notification channel against the current DB.
func (s *BackupService) previewNotifications(imports []NotificationExport) []ItemResolution {
	var existing []db.NotificationConfig
	s.db.Find(&existing)

	existMap := make(map[string]*db.NotificationConfig, len(existing))
	for i := range existing {
		existMap[existing[i].Type+":"+existing[i].Name] = &existing[i]
	}

	results := make([]ItemResolution, 0, len(imports))
	for _, ne := range imports {
		key := ne.Type + ":" + ne.Name
		if _, ok := existMap[key]; ok {
			results = append(results, ItemResolution{Name: ne.Name, Type: ne.Type, Action: previewActionUpdate})
		} else {
			results = append(results, ItemResolution{Name: ne.Name, Type: ne.Type, Action: previewActionCreate})
		}
	}
	return results
}

// previewDiskGroups checks each imported disk group against the current DB.
func (s *BackupService) previewDiskGroups(imports []DiskGroupExport) []ItemResolution {
	groups, err := s.diskGroups.List()
	if err != nil {
		return nil
	}
	existMap := make(map[string]*db.DiskGroup, len(groups))
	for i := range groups {
		existMap[groups[i].MountPath] = &groups[i]
	}

	results := make([]ItemResolution, 0, len(imports))
	for _, dge := range imports {
		if ex, ok := existMap[dge.MountPath]; ok {
			changes := make([]FieldChange, 0)
			if ex.ThresholdPct != dge.ThresholdPct {
				changes = append(changes, FieldChange{Field: "thresholdPct", OldValue: fmt.Sprintf("%.1f", ex.ThresholdPct), NewValue: fmt.Sprintf("%.1f", dge.ThresholdPct)})
			}
			if ex.TargetPct != dge.TargetPct {
				changes = append(changes, FieldChange{Field: "targetPct", OldValue: fmt.Sprintf("%.1f", ex.TargetPct), NewValue: fmt.Sprintf("%.1f", dge.TargetPct)})
			}
			action := previewActionUnchanged
			if len(changes) > 0 {
				action = previewActionUpdate
			}
			results = append(results, ItemResolution{Name: dge.MountPath, Action: action, Changes: changes})
		} else {
			results = append(results, ItemResolution{Name: dge.MountPath, Action: previewActionCreate})
		}
	}
	return results
}

// previewDeletions computes what existing items would be deleted in sync mode.
func (s *BackupService) previewDeletions(envelope SettingsExportEnvelope, sections ImportSections, allIntegrations []db.IntegrationConfig) *DeletionPreview {
	del := &DeletionPreview{}

	if sections.Integrations && len(envelope.Integrations) > 0 {
		importedKeys := make(map[string]bool, len(envelope.Integrations))
		for _, ie := range envelope.Integrations {
			importedKeys[ie.Type+":"+ie.Name] = true
		}
		for _, ic := range allIntegrations {
			if !importedKeys[ic.Type+":"+ic.Name] {
				del.Integrations = append(del.Integrations, ic.Name+" ("+ic.Type+")")
			}
		}
	}

	if sections.Rules && len(envelope.Rules) > 0 {
		var existingRules []db.CustomRule
		s.db.Find(&existingRules)
		// In sync mode all existing rules are replaced, so report them
		for _, r := range existingRules {
			del.Rules = append(del.Rules, r.Field+" "+r.Operator+" "+r.Value)
		}
	}

	if sections.NotificationChannels && len(envelope.NotificationChannels) > 0 {
		var existingNC []db.NotificationConfig
		s.db.Find(&existingNC)
		importedKeys := make(map[string]bool, len(envelope.NotificationChannels))
		for _, ne := range envelope.NotificationChannels {
			importedKeys[ne.Type+":"+ne.Name] = true
		}
		for _, nc := range existingNC {
			if !importedKeys[nc.Type+":"+nc.Name] {
				del.Notifications = append(del.Notifications, nc.Name+" ("+nc.Type+")")
			}
		}
	}

	if sections.DiskGroups && len(envelope.DiskGroups) > 0 && s.diskGroups != nil {
		groups, err := s.diskGroups.List()
		if err == nil {
			importedPaths := make(map[string]bool, len(envelope.DiskGroups))
			for _, dge := range envelope.DiskGroups {
				importedPaths[dge.MountPath] = true
			}
			for _, g := range groups {
				if !importedPaths[g.MountPath] {
					del.DiskGroups = append(del.DiskGroups, g.MountPath)
				}
			}
		}
	}

	// Return nil if nothing would be deleted
	if len(del.Rules) == 0 && len(del.Integrations) == 0 && len(del.Notifications) == 0 && len(del.DiskGroups) == 0 {
		return nil
	}
	return del
}

// CommitImport executes the import using user-provided overrides for rule
// integration assignments. Rules with overrides use the user-chosen integration
// ID instead of auto-match. Rules marked as Skip are excluded.
func (s *BackupService) CommitImport(envelope SettingsExportEnvelope, sections ImportSections, overrides []RuleOverride) (*ImportResult, error) {
	if envelope.Version != 1 {
		return nil, fmt.Errorf("%w: got %d, expected 1", ErrUnsupportedVersion, envelope.Version)
	}

	syncMode := isSyncMode(sections.Mode)

	// Capture pre-import snapshot for safety
	var snapshot *SettingsExportEnvelope
	if syncMode {
		snap, err := s.Export(sectionsToExportSections(sections), "pre-import-snapshot")
		if err != nil {
			slog.Warn("Failed to create pre-import snapshot", "component", "services", "error", err)
		} else {
			snapshot = snap
		}
	}

	tx := s.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to begin import transaction: %w", tx.Error)
	}

	result := &ImportResult{}

	if sections.Preferences && envelope.Preferences != nil {
		if err := s.importPreferences(tx, envelope.Preferences); err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to import preferences: %w", err)
		}
		result.PreferencesImported = true
	}

	if sections.Integrations && len(envelope.Integrations) > 0 {
		count, deleted, err := s.importIntegrations(tx, envelope.Integrations, syncMode)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to import integrations: %w", err)
		}
		result.IntegrationsImported = count
		result.ItemsDeleted += deleted
	}

	if sections.Rules && len(envelope.Rules) > 0 {
		count, unmatched, deleted, err := s.importRulesWithOverrides(tx, envelope.Rules, overrides, syncMode)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to import rules: %w", err)
		}
		result.RulesImported = count
		result.RulesUnmatched = unmatched
		result.ItemsDeleted += deleted
	}

	if sections.NotificationChannels && len(envelope.NotificationChannels) > 0 {
		count, deleted, err := s.importNotificationChannels(tx, envelope.NotificationChannels, syncMode)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to import notification channels: %w", err)
		}
		result.NotificationChannelsImported = count
		result.ItemsDeleted += deleted
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit import transaction: %w", err)
	}

	// Disk groups imported outside transaction (see Import() comment)
	if sections.DiskGroups && len(envelope.DiskGroups) > 0 {
		count, deleted, err := s.importDiskGroups(envelope.DiskGroups, syncMode)
		if err != nil {
			return nil, fmt.Errorf("failed to import disk groups: %w", err)
		}
		result.DiskGroupsImported = count
		result.ItemsDeleted += deleted
	}

	result.PreImportSnapshot = snapshot

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
			"itemsDeleted":                 result.ItemsDeleted,
		},
	})

	return result, nil
}

// importRulesWithOverrides creates rules using user-provided integration overrides
// instead of auto-match for overridden rules. Skipped rules are not imported.
// In sync mode, existing rules not created by this import are deleted.
// Returns (imported count, unmatched count, deleted count, error).
func (s *BackupService) importRulesWithOverrides(tx *gorm.DB, rules []RuleExport, overrides []RuleOverride, syncMode bool) (int, int, int, error) {
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
			return 0, 0, 0, fmt.Errorf("rule %d: field, operator, and value are required", i)
		}
		if r.Effect == "" {
			return 0, 0, 0, fmt.Errorf("rule %d: effect is required", i)
		}
		if !db.ValidEffects[r.Effect] {
			return 0, 0, 0, fmt.Errorf("rule %d: invalid effect %q", i, r.Effect)
		}
	}

	// Determine the starting sort_order
	var maxOrder int
	row := tx.Model(&db.CustomRule{}).Select("COALESCE(MAX(sort_order), -1)").Row()
	if err := row.Scan(&maxOrder); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to determine rule ordering: %w", err)
	}
	nextOrder := maxOrder + 1

	imported := 0
	unmatched := 0
	touchedIDs := make([]uint, 0, len(rules))

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
				return 0, 0, 0, fmt.Errorf("failed to insert imported rule %d: %w", i, err)
			}
			touchedIDs = append(touchedIDs, newRule.ID)
			if !r.Enabled {
				if err := tx.Model(&newRule).Update("enabled", false).Error; err != nil {
					return 0, 0, 0, fmt.Errorf("failed to disable imported rule %d: %w", i, err)
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
			return 0, 0, 0, fmt.Errorf("failed to insert imported rule %d: %w", i, err)
		}
		touchedIDs = append(touchedIDs, newRule.ID)
		if !r.Enabled {
			if err := tx.Model(&newRule).Update("enabled", false).Error; err != nil {
				return 0, 0, 0, fmt.Errorf("failed to disable imported rule %d: %w", i, err)
			}
		}
		nextOrder++
		imported++
	}

	// Sync mode: delete rules that were not created during this import
	deleted := 0
	if syncMode && len(touchedIDs) > 0 {
		result := tx.Where("id NOT IN ?", touchedIDs).Delete(&db.CustomRule{})
		if result.Error != nil {
			return imported, unmatched, 0, fmt.Errorf("failed to delete orphaned rules: %w", result.Error)
		}
		deleted = int(result.RowsAffected)
	} else if syncMode && len(touchedIDs) == 0 {
		result := tx.Where("1 = 1").Delete(&db.CustomRule{})
		if result.Error != nil {
			return 0, unmatched, 0, fmt.Errorf("failed to delete all rules in sync mode: %w", result.Error)
		}
		deleted = int(result.RowsAffected)
	}

	return imported, unmatched, deleted, nil
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
