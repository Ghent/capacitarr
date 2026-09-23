package backup

import (
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"capacitarr/internal/db"
)

// importPreferences updates the singleton PreferenceSet row and scoring factor weights.
func (s *BackupService) importPreferences(tx *gorm.DB, p *PreferencesExport) error {
	var pref db.PreferenceSet
	if err := tx.FirstOrCreate(&pref, db.PreferenceSet{ID: 1}).Error; err != nil {
		return err
	}

	pref.LogLevel = p.LogLevel
	pref.AuditLogRetentionDays = p.AuditLogRetentionDays
	pref.PollIntervalSeconds = p.PollIntervalSeconds
	pref.DefaultDiskGroupMode = p.EffectiveMode()
	pref.TiebreakerMethod = p.TiebreakerMethod
	pref.DeletionsEnabled = p.DeletionsEnabled
	pref.SnoozeDurationHours = p.SnoozeDurationHours
	pref.CheckForUpdates = p.CheckForUpdates
	if p.BackupRetentionDays > 0 {
		pref.BackupRetentionDays = p.BackupRetentionDays
	}
	if p.PosterOverlayStyle != "" {
		pref.PosterOverlayStyle = p.PosterOverlayStyle
	}
	// Backward compat: old backups have posterOverlayEnabled but no "off" style.
	// If the old boolean was explicitly false, override style to "off".
	if p.PosterOverlayEnabled != nil && !*p.PosterOverlayEnabled {
		pref.PosterOverlayStyle = "off"
	}

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
		slog.Error("Rule integration match failed, skipping rule",
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
			// Found — update URL, Enabled, and toggle settings but preserve API key
			existing.URL = ie.URL
			existing.Enabled = ie.Enabled
			existing.CollectionDeletion = ie.CollectionDeletion
			existing.ShowLevelOnly = ie.ShowLevelOnly
			existing.AddImportExclusion = ie.AddImportExclusion
			if dbErr := tx.Save(&existing).Error; dbErr != nil {
				return count, 0, fmt.Errorf("failed to update integration %q: %w", ie.Name, dbErr)
			}
			count++
			continue
		}

		// Not found — create new with placeholder API key, forced disabled
		ic := db.IntegrationConfig{
			Name:               ie.Name,
			Type:               ie.Type,
			URL:                ie.URL,
			APIKey:             placeholderAPIKey,
			Enabled:            true, // GORM default:true workaround — disable below
			CollectionDeletion: ie.CollectionDeletion,
			ShowLevelOnly:      ie.ShowLevelOnly,
			AddImportExclusion: ie.AddImportExclusion,
		}
		if dbErr := tx.Create(&ic).Error; dbErr != nil {
			return count, 0, fmt.Errorf("failed to create integration %q: %w", ie.Name, dbErr)
		}
		// Force disable new imports with placeholder credentials
		if dbErr := tx.Model(&ic).Update("enabled", false).Error; dbErr != nil {
			return count, 0, fmt.Errorf("failed to disable placeholder integration %q: %w", ie.Name, dbErr)
		}
		// GORM skips false booleans on Create() when the DB default is true.
		// Explicitly set add_import_exclusion=false when the import says so.
		if !ie.AddImportExclusion {
			if dbErr := tx.Model(&ic).Update("add_import_exclusion", false).Error; dbErr != nil {
				return count, 0, fmt.Errorf("failed to set add_import_exclusion for %q: %w", ie.Name, dbErr)
			}
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

// mapLegacyBoolsToTier maps pre-tier boolean notification flags to a tier string.
// Used during import of old backup files that predate the tier system.
func mapLegacyBoolsToTier(ne NotificationExport) string {
	boolVal := func(b *bool) bool { return b != nil && *b }
	allFalse := !boolVal(ne.LegacyOnCycleDigest) && !boolVal(ne.LegacyOnError) &&
		!boolVal(ne.LegacyOnModeChanged) && !boolVal(ne.LegacyOnServerStarted) &&
		!boolVal(ne.LegacyOnThresholdBreach) && !boolVal(ne.LegacyOnUpdateAvailable) &&
		!boolVal(ne.LegacyOnApprovalActivity) && !boolVal(ne.LegacyOnIntegrationStatus)
	allTrue := boolVal(ne.LegacyOnCycleDigest) && boolVal(ne.LegacyOnError) &&
		boolVal(ne.LegacyOnModeChanged) && boolVal(ne.LegacyOnServerStarted) &&
		boolVal(ne.LegacyOnThresholdBreach) && boolVal(ne.LegacyOnUpdateAvailable) &&
		boolVal(ne.LegacyOnApprovalActivity) && boolVal(ne.LegacyOnIntegrationStatus) &&
		boolVal(ne.LegacyOnDryRunDigest)
	if allFalse {
		return "off"
	}
	if allTrue {
		return "verbose"
	}
	return "normal"
}

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
	for i := range channels {
		ne := &channels[i]

		// Backwards compatibility: map legacy boolean fields to tier system
		if ne.NotificationLevel == "" && ne.LegacyOnCycleDigest != nil {
			ne.NotificationLevel = mapLegacyBoolsToTier(*ne)
		}
		if ne.NotificationLevel == "" {
			ne.NotificationLevel = "normal" // default
		}

		importedKeys[ne.Type+":"+ne.Name] = true

		// Upsert: look up existing by type + name
		var existing db.NotificationConfig
		err := tx.Where("type = ? AND name = ?", ne.Type, ne.Name).First(&existing).Error
		if err == nil {
			// Found — update subscription flags but preserve webhook URL
			existing.Enabled = ne.Enabled
			existing.AppriseTags = ne.AppriseTags
			existing.NotificationLevel = ne.NotificationLevel
			existing.OverrideCycleDigest = ne.OverrideCycleDigest
			existing.OverrideError = ne.OverrideError
			existing.OverrideModeChanged = ne.OverrideModeChanged
			existing.OverrideServerStarted = ne.OverrideServerStarted
			existing.OverrideThresholdBreach = ne.OverrideThresholdBreach
			existing.OverrideUpdateAvailable = ne.OverrideUpdateAvailable
			existing.OverrideApprovalActivity = ne.OverrideApprovalActivity
			existing.OverrideIntegrationStatus = ne.OverrideIntegrationStatus
			if dbErr := tx.Save(&existing).Error; dbErr != nil {
				return count, 0, fmt.Errorf("failed to update notification channel %q: %w", ne.Name, dbErr)
			}
			count++
			continue
		}

		// Not found — create new with placeholder webhook URL, forced disabled
		nc := db.NotificationConfig{
			Name:                      ne.Name,
			Type:                      ne.Type,
			WebhookURL:                placeholderWebhookURL,
			Enabled:                   true, // GORM default:true workaround — disable below
			AppriseTags:               ne.AppriseTags,
			NotificationLevel:         ne.NotificationLevel,
			OverrideCycleDigest:       ne.OverrideCycleDigest,
			OverrideError:             ne.OverrideError,
			OverrideModeChanged:       ne.OverrideModeChanged,
			OverrideServerStarted:     ne.OverrideServerStarted,
			OverrideThresholdBreach:   ne.OverrideThresholdBreach,
			OverrideUpdateAvailable:   ne.OverrideUpdateAvailable,
			OverrideApprovalActivity:  ne.OverrideApprovalActivity,
			OverrideIntegrationStatus: ne.OverrideIntegrationStatus,
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
				slog.Error("Rule integration match failed in override path, skipping rule",
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
