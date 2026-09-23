package services

import (
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

// PreviewCacheClearer provides the ability to clear the persisted media cache.
// Defined here to avoid import cycles between DataService and PreviewService.
type PreviewCacheClearer interface {
	ClearPersistedCache()
	InvalidatePreviewCache(reason string)
}

// DataService handles data reset operations.
type DataService struct {
	db      *gorm.DB
	bus     *events.EventBus
	preview PreviewCacheClearer
}

// NewDataService creates a new DataService.
func NewDataService(database *gorm.DB, bus *events.EventBus) *DataService {
	return &DataService{db: database, bus: bus}
}

// Wired returns true when all lazily-injected dependencies are non-nil.
// Used by Registry.Validate() to catch missing wiring at startup.
func (s *DataService) Wired() bool {
	return s.preview != nil
}

// SetPreviewService wires the preview service dependency for cache clearing.
// Called by Registry after construction to avoid circular initialization.
func (s *DataService) SetPreviewService(preview PreviewCacheClearer) {
	s.preview = preview
}

// Reset clears all scraped data. Returns a summary of rows affected.
// This clears audit_log, approval_queue, library_histories, engine_run_stats,
// and resets transient fields on disk_groups and integration_configs.
// Lifetime stats and preferences are NOT cleared.
// All writes run in one transaction; a mid-way error rolls back everything.
func (s *DataService) Reset() (map[string]int64, error) {
	summary := map[string]int64{}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&db.AuditLogEntry{})
		if res.Error != nil {
			return fmt.Errorf("failed to clear audit log: %w", res.Error)
		}
		summary["auditLog"] = res.RowsAffected

		res = tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&db.ApprovalQueueItem{})
		if res.Error != nil {
			return fmt.Errorf("failed to clear approval queue: %w", res.Error)
		}
		summary["approvalQueue"] = res.RowsAffected

		res = tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&db.LibraryHistory{})
		if res.Error != nil {
			return fmt.Errorf("failed to clear library history: %w", res.Error)
		}
		summary["libraryHistories"] = res.RowsAffected

		res = tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&db.EngineRunStats{})
		if res.Error != nil {
			return fmt.Errorf("failed to clear engine run stats: %w", res.Error)
		}
		summary["engineRunStats"] = res.RowsAffected

		res = tx.Model(&db.DiskGroup{}).Where("1 = 1").Updates(map[string]any{
			"total_bytes": 0,
			"used_bytes":  0,
		})
		if res.Error != nil {
			return fmt.Errorf("failed to reset disk groups: %w", res.Error)
		}
		summary["diskGroupsReset"] = res.RowsAffected

		res = tx.Model(&db.IntegrationConfig{}).Where("1 = 1").Updates(map[string]any{
			"media_size_bytes": 0,
			"media_count":      0,
			"last_sync":        nil,
			"last_error":       "",
		})
		if res.Error != nil {
			return fmt.Errorf("failed to reset integration stats: %w", res.Error)
		}
		summary["integrationsReset"] = res.RowsAffected

		if s.preview != nil {
			if err := tx.Where("id = ?", 1).Delete(&db.MediaCache{}).Error; err != nil {
				return fmt.Errorf("failed to clear media cache: %w", err)
			}
		}
		summary["mediaCacheCleared"] = int64(1)
		return nil
	})
	if err != nil {
		return nil, err
	}

	if s.preview != nil {
		s.preview.InvalidatePreviewCache("data_reset")
	}

	s.bus.Publish(events.DataResetEvent{Summary: summary})
	slog.Info("Data reset completed", "component", "services", "summary", summary)

	return summary, nil
}
