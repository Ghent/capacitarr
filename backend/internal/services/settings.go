package services

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
	"capacitarr/internal/logger"
)

// SettingsService manages application preferences and disk group thresholds.
type SettingsService struct {
	db  *gorm.DB
	bus *events.EventBus
}

// NewSettingsService creates a new SettingsService.
func NewSettingsService(database *gorm.DB, bus *events.EventBus) *SettingsService {
	return &SettingsService{db: database, bus: bus}
}

// GetPreferences returns the current preferences (singleton row).
func (s *SettingsService) GetPreferences() (db.PreferenceSet, error) {
	var pref db.PreferenceSet
	if err := s.db.FirstOrCreate(&pref, db.PreferenceSet{ID: 1}).Error; err != nil {
		return pref, fmt.Errorf("failed to fetch preferences: %w", err)
	}
	return pref, nil
}

// UpdatePreferences saves preference changes and publishes relevant events.
func (s *SettingsService) UpdatePreferences(payload db.PreferenceSet) (db.PreferenceSet, error) {
	payload.ID = 1

	// Snapshot current for change detection
	var oldPrefs db.PreferenceSet
	s.db.FirstOrCreate(&oldPrefs, db.PreferenceSet{ID: 1})

	if err := s.db.Save(&payload).Error; err != nil {
		return payload, fmt.Errorf("failed to save preferences: %w", err)
	}

	// Apply dynamic log level
	logger.SetLevel(payload.LogLevel)

	// Detect execution mode change
	if oldPrefs.ExecutionMode != payload.ExecutionMode {
		s.bus.Publish(events.EngineModeChangedEvent{
			OldMode: oldPrefs.ExecutionMode,
			NewMode: payload.ExecutionMode,
		})
	}

	s.bus.Publish(events.SettingsChangedEvent{})

	return payload, nil
}

// UpdateThresholds updates the threshold and target percentages for a disk group
// and returns the updated group.
func (s *SettingsService) UpdateThresholds(groupID uint, threshold, target float64) (*db.DiskGroup, error) {
	var group db.DiskGroup
	if err := s.db.First(&group, groupID).Error; err != nil {
		return nil, fmt.Errorf("disk group not found: %w", err)
	}

	if err := s.db.Model(&group).Updates(map[string]any{
		"threshold_pct": threshold,
		"target_pct":    target,
	}).Error; err != nil {
		return nil, fmt.Errorf("failed to update thresholds: %w", err)
	}

	s.bus.Publish(events.ThresholdChangedEvent{
		MountPath:    group.MountPath,
		ThresholdPct: threshold,
		TargetPct:    target,
	})

	// Reload the updated group
	s.db.First(&group, groupID)
	return &group, nil
}

// ListDiskGroups returns all disk groups.
func (s *SettingsService) ListDiskGroups() ([]db.DiskGroup, error) {
	groups := make([]db.DiskGroup, 0)
	if err := s.db.Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch disk groups: %w", err)
	}
	return groups, nil
}

// GetDiskGroup returns a single disk group by ID.
func (s *SettingsService) GetDiskGroup(id uint) (*db.DiskGroup, error) {
	var group db.DiskGroup
	if err := s.db.First(&group, id).Error; err != nil {
		return nil, fmt.Errorf("disk group not found: %w", err)
	}
	return &group, nil
}

// UpsertDiskGroup creates or updates a disk group from discovered disk space.
// Shared by the sync route and the poller.
func (s *SettingsService) UpsertDiskGroup(disk integrations.DiskSpace) (*db.DiskGroup, error) {
	var group db.DiskGroup
	result := s.db.Where("mount_path = ?", disk.Path).First(&group)

	usedBytes := disk.TotalBytes - disk.FreeBytes

	if result.Error != nil {
		// Create new disk group
		group = db.DiskGroup{
			MountPath:  disk.Path,
			TotalBytes: disk.TotalBytes,
			UsedBytes:  usedBytes,
		}
		if err := s.db.Create(&group).Error; err != nil {
			return nil, fmt.Errorf("failed to create disk group: %w", err)
		}
	} else {
		// Update existing
		if err := s.db.Model(&group).Updates(map[string]any{
			"total_bytes": disk.TotalBytes,
			"used_bytes":  usedBytes,
		}).Error; err != nil {
			return nil, fmt.Errorf("failed to update disk group: %w", err)
		}
	}

	return &group, nil
}

// CleanOrphanedDiskGroups removes disk groups whose mount paths are not in the
// provided set of active mount paths.
func (s *SettingsService) CleanOrphanedDiskGroups(activeMounts map[string]bool) (int64, error) {
	var allGroups []db.DiskGroup
	if err := s.db.Find(&allGroups).Error; err != nil {
		return 0, fmt.Errorf("failed to fetch disk groups: %w", err)
	}

	var deleted int64
	for _, g := range allGroups {
		if !activeMounts[g.MountPath] {
			if err := s.db.Delete(&g).Error; err != nil {
				return deleted, fmt.Errorf("failed to delete orphaned disk group %q: %w", g.MountPath, err)
			}
			deleted++
		}
	}

	return deleted, nil
}

// PruneOldActivities deletes activity events older than the given number of days.
func (s *SettingsService) PruneOldActivities(days int) (int64, error) {
	if days <= 0 {
		return 0, nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	result := s.db.Where("created_at < ?", cutoff).Delete(&db.ActivityEvent{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to prune activity events: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// CreateActivity creates a new activity event record.
func (s *SettingsService) CreateActivity(eventType, message, metadata string) error {
	entry := db.ActivityEvent{
		EventType: eventType,
		Message:   message,
		Metadata:  metadata,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.db.Create(&entry).Error; err != nil {
		return fmt.Errorf("failed to create activity event: %w", err)
	}
	return nil
}

// ListRecentActivities returns the most recent N activity events.
func (s *SettingsService) ListRecentActivities(limit int) ([]db.ActivityEvent, error) {
	activities := make([]db.ActivityEvent, 0, limit)
	if err := s.db.Order("created_at desc").Limit(limit).Find(&activities).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch recent activities: %w", err)
	}
	return activities, nil
}
