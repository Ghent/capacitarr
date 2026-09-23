package services

import (
	"gorm.io/gorm"

	"capacitarr/internal/events"
)

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

// Wired returns true when all lazily-injected dependencies are non-nil.
// Used by Registry.Validate() to catch missing wiring at startup.
func (s *BackupService) Wired() bool {
	return s.diskGroups != nil
}

// SetDiskGroupService wires the DiskGroupService dependency for disk group
// export and import. Called by Registry after construction.
func (s *BackupService) SetDiskGroupService(dg *DiskGroupService) {
	s.diskGroups = dg
}
