package backup

import (
	"fmt"
	"log/slog"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

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
			slog.Error("Failed to create pre-import snapshot", "component", "services", "error", err)
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

	if sections.DiskGroups && len(envelope.DiskGroups) > 0 {
		count, deleted, err := s.importDiskGroups(tx, envelope.DiskGroups, syncMode)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to import disk groups: %w", err)
		}
		result.DiskGroupsImported = count
		result.ItemsDeleted += deleted
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit import transaction: %w", err)
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
	addChange("defaultDiskGroupMode", pref.DefaultDiskGroupMode, p.EffectiveMode())
	addChange("tiebreakerMethod", pref.TiebreakerMethod, p.TiebreakerMethod)
	addChange("pollIntervalSeconds", fmt.Sprintf("%d", pref.PollIntervalSeconds), fmt.Sprintf("%d", p.PollIntervalSeconds))
	addChange("auditLogRetentionDays", fmt.Sprintf("%d", pref.AuditLogRetentionDays), fmt.Sprintf("%d", p.AuditLogRetentionDays))
	addChange("snoozeDurationHours", fmt.Sprintf("%d", pref.SnoozeDurationHours), fmt.Sprintf("%d", p.SnoozeDurationHours))
	addChange("deletionsEnabled", fmt.Sprintf("%v", pref.DeletionsEnabled), fmt.Sprintf("%v", p.DeletionsEnabled))
	addChange("checkForUpdates", fmt.Sprintf("%v", pref.CheckForUpdates), fmt.Sprintf("%v", p.CheckForUpdates))
	if p.PosterOverlayStyle != "" {
		addChange("posterOverlayStyle", pref.PosterOverlayStyle, p.PosterOverlayStyle)
	}

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
			if ex.CollectionDeletion != ie.CollectionDeletion {
				changes = append(changes, FieldChange{Field: "collectionDeletion", OldValue: fmt.Sprintf("%v", ex.CollectionDeletion), NewValue: fmt.Sprintf("%v", ie.CollectionDeletion)})
			}
			if ex.ShowLevelOnly != ie.ShowLevelOnly {
				changes = append(changes, FieldChange{Field: "showLevelOnly", OldValue: fmt.Sprintf("%v", ex.ShowLevelOnly), NewValue: fmt.Sprintf("%v", ie.ShowLevelOnly)})
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
			slog.Error("Failed to create pre-import snapshot", "component", "services", "error", err)
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

	if sections.DiskGroups && len(envelope.DiskGroups) > 0 {
		count, deleted, err := s.importDiskGroups(tx, envelope.DiskGroups, syncMode)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to import disk groups: %w", err)
		}
		result.DiskGroupsImported = count
		result.ItemsDeleted += deleted
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit import transaction: %w", err)
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
