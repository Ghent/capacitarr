// Package backup handles settings export and import.
package backup

import (
	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

// diskGroupStore is the DiskGroupService surface used by export/import.
// *services.DiskGroupService satisfies it; the interface lives here so this
// package does not import the composition root.
type diskGroupStore interface {
	List() ([]db.DiskGroup, error)
	ImportUpsert(mountPath string, threshold, target float64, totalOverride *int64) error
}

// BackupService handles settings export and import operations.
//
//nolint:revive // existing type name; package split is not a rename
type BackupService struct {
	db         *gorm.DB
	bus        *events.EventBus
	diskGroups diskGroupStore
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
func (s *BackupService) SetDiskGroupService(dg diskGroupStore) {
	s.diskGroups = dg
}
