package services

import (
	"log/slog"

	"capacitarr/internal/integrations"
)

// sunsetGroupExiterAdapter implements SunsetGroupExiter by building SunsetDeps
// the same way the sunset cancel route does, then calling CancelAllForDiskGroup.
// Lives in the composition root because it references leftover services.
type sunsetGroupExiterAdapter struct {
	sunset        *SunsetService
	integration   *IntegrationService
	settings      SettingsReader
	posterOverlay *PosterOverlayService
	mapping       *MappingService
	deletion      *DeletionService
	engine        *EngineService
}

// Exit restores posters, removes labels, and deletes sunset rows for one group.
// A failed integration-registry build is logged and Exit continues so rows
// still delete even when label/poster restore cannot run.
func (a *sunsetGroupExiterAdapter) Exit(diskGroupID uint) (int, error) {
	var registry *integrations.IntegrationRegistry
	if a.integration != nil {
		built, err := a.integration.BuildIntegrationRegistry()
		if err != nil {
			slog.Error("Failed to build integration registry for sunset exit — label/poster restore may be skipped",
				"component", "registry", "diskGroupID", diskGroupID, "error", err)
		} else {
			registry = built
		}
	}
	return a.sunset.CancelAllForDiskGroup(diskGroupID, SunsetDeps{
		Registry:      registry,
		Deletion:      a.deletion,
		Engine:        a.engine,
		Settings:      a.settings,
		PosterOverlay: a.posterOverlay,
		Mapping:       a.mapping,
	})
}

// NewSunsetGroupExiter creates a SunsetGroupExiter that delegates to
// SunsetService.CancelAllForDiskGroup. Used to wire DiskGroupService without
// a direct SunsetService import at the call site.
func NewSunsetGroupExiter(
	sunset *SunsetService,
	integration *IntegrationService,
	settings SettingsReader,
	posterOverlay *PosterOverlayService,
	mapping *MappingService,
	deletion *DeletionService,
	engine *EngineService,
) SunsetGroupExiter {
	return &sunsetGroupExiterAdapter{
		sunset:        sunset,
		integration:   integration,
		settings:      settings,
		posterOverlay: posterOverlay,
		mapping:       mapping,
		deletion:      deletion,
		engine:        engine,
	}
}
