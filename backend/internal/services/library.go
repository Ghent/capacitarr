package services

import (
	"fmt"

	"capacitarr/internal/db"
	"capacitarr/internal/events"

	"gorm.io/gorm"
)

// LibraryService manages Library entities — grouping integrations with
// optional threshold overrides. Threshold hierarchy:
// integration override → library override → disk group default.
type LibraryService struct {
	db  *gorm.DB
	bus *events.EventBus
}

// NewLibraryService creates a new LibraryService.
func NewLibraryService(database *gorm.DB, bus *events.EventBus) *LibraryService {
	return &LibraryService{db: database, bus: bus}
}

// List returns all libraries.
func (s *LibraryService) List() ([]db.Library, error) {
	var libraries []db.Library
	if err := s.db.Order("name ASC").Find(&libraries).Error; err != nil {
		return nil, fmt.Errorf("list libraries: %w", err)
	}
	return libraries, nil
}

// GetByID returns a single library by ID.
func (s *LibraryService) GetByID(id uint) (*db.Library, error) {
	var library db.Library
	if err := s.db.First(&library, id).Error; err != nil {
		return nil, fmt.Errorf("get library %d: %w", id, err)
	}
	return &library, nil
}

// Create creates a new library.
func (s *LibraryService) Create(library *db.Library) error {
	if library.Name == "" {
		return fmt.Errorf("library name is required")
	}
	if err := s.db.Create(library).Error; err != nil {
		return fmt.Errorf("create library: %w", err)
	}
	return nil
}

// Update updates an existing library.
func (s *LibraryService) Update(library *db.Library) error {
	if library.ID == 0 {
		return fmt.Errorf("library ID is required")
	}
	if library.Name == "" {
		return fmt.Errorf("library name is required")
	}
	if err := s.db.Save(library).Error; err != nil {
		return fmt.Errorf("update library %d: %w", library.ID, err)
	}
	return nil
}

// Delete removes a library by ID. Integrations referencing this library
// will have their library_id set to NULL (ON DELETE SET NULL).
func (s *LibraryService) Delete(id uint) error {
	result := s.db.Delete(&db.Library{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete library %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("library %d not found", id)
	}
	return nil
}

// EffectiveThresholdForIntegration resolves the threshold percentage for an integration
// by walking the override chain: integration → library → disk group → default (85%).
func (s *LibraryService) EffectiveThresholdForIntegration(integrationID uint) (float64, float64, error) {
	var integration db.IntegrationConfig
	if err := s.db.First(&integration, integrationID).Error; err != nil {
		return 0, 0, fmt.Errorf("get integration %d: %w", integrationID, err)
	}

	// Check integration-level override
	if integration.ThresholdPct != nil && integration.TargetPct != nil {
		return *integration.ThresholdPct, *integration.TargetPct, nil
	}

	// Check library-level override
	if integration.LibraryID != nil {
		var library db.Library
		if err := s.db.First(&library, *integration.LibraryID).Error; err == nil {
			if library.ThresholdPct != nil && library.TargetPct != nil {
				return *library.ThresholdPct, *library.TargetPct, nil
			}
			// Check library's disk group
			if library.DiskGroupID != nil {
				var dg db.DiskGroup
				if err := s.db.First(&dg, *library.DiskGroupID).Error; err == nil {
					return dg.ThresholdPct, dg.TargetPct, nil
				}
			}
		}
	}

	// Fallback: find disk group via disk_group_integrations junction table
	var dgi db.DiskGroupIntegration
	if err := s.db.Where("integration_id = ?", integrationID).First(&dgi).Error; err == nil {
		var dg db.DiskGroup
		if err := s.db.First(&dg, dgi.DiskGroupID).Error; err == nil {
			return dg.ThresholdPct, dg.TargetPct, nil
		}
	}

	// Ultimate default
	return 85.0, 75.0, nil
}

// GetIntegrationsForLibrary returns all integrations assigned to a library.
func (s *LibraryService) GetIntegrationsForLibrary(libraryID uint) ([]db.IntegrationConfig, error) {
	var integrations []db.IntegrationConfig
	if err := s.db.Where("library_id = ?", libraryID).Find(&integrations).Error; err != nil {
		return nil, fmt.Errorf("get integrations for library %d: %w", libraryID, err)
	}
	return integrations, nil
}
