package services

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

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

// PreferencesExport contains all PreferenceSet fields except ID and UpdatedAt,
// plus scoring factor weights as a dynamic map.
//
// For backward compatibility with 2.x backup files, ExecutionMode is kept as a
// fallback field during import. New backups write DefaultDiskGroupMode.
type PreferencesExport struct {
	LogLevel              string         `json:"logLevel"`
	AuditLogRetentionDays int            `json:"auditLogRetentionDays"`
	PollIntervalSeconds   int            `json:"pollIntervalSeconds"`
	DefaultDiskGroupMode  string         `json:"defaultDiskGroupMode"`
	ExecutionMode         string         `json:"executionMode,omitempty"` // 2.x compat: read during import, not written in 3.x exports
	TiebreakerMethod      string         `json:"tiebreakerMethod"`
	DeletionsEnabled      bool           `json:"deletionsEnabled"`
	SnoozeDurationHours   int            `json:"snoozeDurationHours"`
	CheckForUpdates       bool           `json:"checkForUpdates"`
	SunsetDays            int            `json:"sunsetDays,omitempty"`
	SunsetLabel           string         `json:"sunsetLabel,omitempty"`
	PosterOverlayEnabled  *bool          `json:"posterOverlayEnabled,omitempty"` // 3.x compat: read during import to derive style, not written in new exports
	PosterOverlayStyle    string         `json:"posterOverlayStyle,omitempty"`
	BackupRetentionDays   int            `json:"backupRetentionDays,omitempty"`
	FactorWeights         map[string]int `json:"factorWeights,omitempty"` // factor_key → weight (0-10)
}

// EffectiveMode returns the disk group mode from the export, handling backward
// compatibility with 2.x backups that used ExecutionMode instead of DefaultDiskGroupMode.
func (p PreferencesExport) EffectiveMode() string {
	if p.DefaultDiskGroupMode != "" {
		return p.DefaultDiskGroupMode
	}
	if p.ExecutionMode != "" {
		return p.ExecutionMode
	}
	return db.ModeDryRun
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
	Name               string `json:"name"`
	Type               string `json:"type"`
	URL                string `json:"url"`
	Enabled            bool   `json:"enabled"`
	CollectionDeletion bool   `json:"collectionDeletion"`
	ShowLevelOnly      bool   `json:"showLevelOnly"`
	AddImportExclusion bool   `json:"addImportExclusion"`
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
	Name                      string `json:"name"`
	Type                      string `json:"type"`
	Enabled                   bool   `json:"enabled"`
	AppriseTags               string `json:"appriseTags,omitempty"`
	NotificationLevel         string `json:"notificationLevel"`
	OverrideCycleDigest       *bool  `json:"overrideCycleDigest,omitempty"`
	OverrideError             *bool  `json:"overrideError,omitempty"`
	OverrideModeChanged       *bool  `json:"overrideModeChanged,omitempty"`
	OverrideServerStarted     *bool  `json:"overrideServerStarted,omitempty"`
	OverrideThresholdBreach   *bool  `json:"overrideThresholdBreach,omitempty"`
	OverrideUpdateAvailable   *bool  `json:"overrideUpdateAvailable,omitempty"`
	OverrideApprovalActivity  *bool  `json:"overrideApprovalActivity,omitempty"`
	OverrideIntegrationStatus *bool  `json:"overrideIntegrationStatus,omitempty"`

	// Legacy fields for backwards compatibility with pre-tier backups.
	// Read during import; never written during export (omitempty).
	LegacyOnCycleDigest       *bool `json:"onCycleDigest,omitempty"`
	LegacyOnDryRunDigest      *bool `json:"onDryRunDigest,omitempty"`
	LegacyOnError             *bool `json:"onError,omitempty"`
	LegacyOnModeChanged       *bool `json:"onModeChanged,omitempty"`
	LegacyOnServerStarted     *bool `json:"onServerStarted,omitempty"`
	LegacyOnThresholdBreach   *bool `json:"onThresholdBreach,omitempty"`
	LegacyOnUpdateAvailable   *bool `json:"onUpdateAvailable,omitempty"`
	LegacyOnApprovalActivity  *bool `json:"onApprovalActivity,omitempty"`
	LegacyOnIntegrationStatus *bool `json:"onIntegrationStatus,omitempty"`
	LegacyOnSunsetActivity    *bool `json:"onSunsetActivity,omitempty"`
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
			DefaultDiskGroupMode:  pref.DefaultDiskGroupMode,
			TiebreakerMethod:      pref.TiebreakerMethod,
			DeletionsEnabled:      pref.DeletionsEnabled,
			SnoozeDurationHours:   pref.SnoozeDurationHours,
			CheckForUpdates:       pref.CheckForUpdates,
			SunsetDays:            pref.SunsetDays,
			SunsetLabel:           pref.SunsetLabel,
			PosterOverlayStyle:    pref.PosterOverlayStyle,
			BackupRetentionDays:   pref.BackupRetentionDays,
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
				Name:               ic.Name,
				Type:               ic.Type,
				URL:                ic.URL,
				Enabled:            ic.Enabled,
				CollectionDeletion: ic.CollectionDeletion,
				ShowLevelOnly:      ic.ShowLevelOnly,
				AddImportExclusion: ic.AddImportExclusion,
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
				Name:                      nc.Name,
				Type:                      nc.Type,
				Enabled:                   nc.Enabled,
				AppriseTags:               nc.AppriseTags,
				NotificationLevel:         nc.NotificationLevel,
				OverrideCycleDigest:       nc.OverrideCycleDigest,
				OverrideError:             nc.OverrideError,
				OverrideModeChanged:       nc.OverrideModeChanged,
				OverrideServerStarted:     nc.OverrideServerStarted,
				OverrideThresholdBreach:   nc.OverrideThresholdBreach,
				OverrideUpdateAvailable:   nc.OverrideUpdateAvailable,
				OverrideApprovalActivity:  nc.OverrideApprovalActivity,
				OverrideIntegrationStatus: nc.OverrideIntegrationStatus,
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
