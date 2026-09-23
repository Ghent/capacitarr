package services

import "fmt"

// sunsetLabelMigratorAdapter implements SunsetLabelMigrator by building a
// fresh integration registry and delegating to SunsetService.MigrateLabel.
// Lives in the composition root because it references leftover services.
type sunsetLabelMigratorAdapter struct {
	sunset      *SunsetService
	integration *IntegrationService
	mapping     *MappingService
}

// MigrateSunsetLabel builds a fresh integration registry and migrates labels.
func (a *sunsetLabelMigratorAdapter) MigrateSunsetLabel(oldLabel, newLabel string) error {
	registry, err := a.integration.BuildIntegrationRegistry()
	if err != nil {
		return fmt.Errorf("build registry for label migration: %w", err)
	}
	return a.sunset.MigrateLabel(oldLabel, newLabel, registry, a.mapping)
}

// NewSunsetLabelMigrator creates a SunsetLabelMigrator that delegates to the
// given services. Used to wire SettingsService without a direct SunsetService import.
func NewSunsetLabelMigrator(sunset *SunsetService, integration *IntegrationService, mapping *MappingService) SunsetLabelMigrator {
	return &sunsetLabelMigratorAdapter{sunset: sunset, integration: integration, mapping: mapping}
}
