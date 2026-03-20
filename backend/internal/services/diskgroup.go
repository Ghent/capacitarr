package services

import (
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
)

// EngineRunTrigger is the subset of EngineService needed by DiskGroupService
// to trigger an immediate engine run after threshold changes. Defined as an
// interface to avoid a direct dependency on EngineService and to simplify
// testing.
type EngineRunTrigger interface {
	TriggerRun() string
}

// DiskGroupService manages disk group lifecycle: discovery, reconciliation,
// threshold configuration, and integration tracking.
type DiskGroupService struct {
	db     *gorm.DB
	bus    *events.EventBus
	engine EngineRunTrigger // optional; wired via SetEngineService()
}

// NewDiskGroupService creates a new DiskGroupService.
func NewDiskGroupService(database *gorm.DB, bus *events.EventBus) *DiskGroupService {
	return &DiskGroupService{db: database, bus: bus}
}

// SetEngineService wires the EngineService dependency so that threshold changes
// can trigger an immediate engine run for queue reconciliation.
func (s *DiskGroupService) SetEngineService(engine EngineRunTrigger) {
	s.engine = engine
}

// List returns all disk groups.
func (s *DiskGroupService) List() ([]db.DiskGroup, error) {
	groups := make([]db.DiskGroup, 0)
	if err := s.db.Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch disk groups: %w", err)
	}
	return groups, nil
}

// GetByID returns a single disk group by ID.
func (s *DiskGroupService) GetByID(id uint) (*db.DiskGroup, error) {
	var group db.DiskGroup
	if err := s.db.First(&group, id).Error; err != nil {
		return nil, fmt.Errorf("disk group not found: %w", err)
	}
	return &group, nil
}

// Upsert creates or updates a disk group from discovered disk space.
// Shared by the sync route and the poller.
func (s *DiskGroupService) Upsert(disk integrations.DiskSpace) (*db.DiskGroup, error) {
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

// UpdateThresholds updates the threshold and target percentages for a disk group,
// along with an optional total-bytes override, and returns the updated group.
func (s *DiskGroupService) UpdateThresholds(groupID uint, threshold, target float64, totalOverride *int64) (*db.DiskGroup, error) {
	var group db.DiskGroup
	if err := s.db.First(&group, groupID).Error; err != nil {
		return nil, fmt.Errorf("disk group not found: %w", err)
	}

	updates := map[string]any{
		"threshold_pct": threshold,
		"target_pct":    target,
	}
	if totalOverride != nil && *totalOverride > 0 {
		updates["total_bytes_override"] = *totalOverride
	}
	if err := s.db.Model(&group).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update thresholds: %w", err)
	}
	// GORM's Updates() skips nil/zero values in maps, so clearing the override
	// requires a separate Update call to explicitly set the column to NULL.
	if totalOverride == nil || *totalOverride == 0 {
		if err := s.db.Model(&group).Update("total_bytes_override", gorm.Expr("NULL")).Error; err != nil {
			return nil, fmt.Errorf("failed to clear override: %w", err)
		}
	}

	s.bus.Publish(events.ThresholdChangedEvent{
		MountPath:    group.MountPath,
		ThresholdPct: threshold,
		TargetPct:    target,
	})

	// Trigger an immediate engine run so the approval queue is reconciled
	// against the new thresholds. The engine cycle's per-group reconciliation
	// will dismiss stale pending items that no longer qualify.
	if s.engine != nil {
		status := s.engine.TriggerRun()
		slog.Info("Threshold change triggered engine run for queue reconciliation",
			"component", "diskgroup_service", "mount", group.MountPath, "status", status)
	}

	// Reload the updated group
	s.db.First(&group, groupID)
	return &group, nil
}

// RemoveAll deletes all disk groups. Used when no enabled integrations remain.
func (s *DiskGroupService) RemoveAll() (int64, error) {
	// Also clear junction table entries
	if err := s.db.Where("1 = 1").Delete(&db.DiskGroupIntegration{}).Error; err != nil {
		slog.Warn("Failed to clear disk group integration links", "component", "diskgroup_service", "error", err)
	}

	result := s.db.Where("1 = 1").Delete(&db.DiskGroup{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to remove all disk groups: %w", result.Error)
	}
	if result.RowsAffected > 0 {
		slog.Info("Removed all disk groups (no enabled integrations)", "component", "diskgroup_service", "count", result.RowsAffected)
	}
	return result.RowsAffected, nil
}

// ReconcileActiveMounts removes disk groups whose mount paths are not in the
// provided set of active mount paths.
func (s *DiskGroupService) ReconcileActiveMounts(activeMounts map[string]bool) (int64, error) {
	var allGroups []db.DiskGroup
	if err := s.db.Find(&allGroups).Error; err != nil {
		return 0, fmt.Errorf("failed to fetch disk groups: %w", err)
	}

	var deleted int64
	for _, g := range allGroups {
		if !activeMounts[g.MountPath] {
			// Remove junction table entries for this group
			s.db.Where("disk_group_id = ?", g.ID).Delete(&db.DiskGroupIntegration{})

			if err := s.db.Delete(&g).Error; err != nil {
				return deleted, fmt.Errorf("failed to delete orphaned disk group %q: %w", g.MountPath, err)
			}
			deleted++
		}
	}

	return deleted, nil
}

// ImportUpsert creates or updates a disk group from backup import data.
// Only configuration fields (thresholds, override) are imported — discovery
// fields (total_bytes, used_bytes) are left at zero for new groups since
// they will be populated by the next poll cycle.
func (s *DiskGroupService) ImportUpsert(mountPath string, threshold, target float64, totalOverride *int64) error {
	var existing db.DiskGroup
	err := s.db.Where("mount_path = ?", mountPath).First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check disk group %q: %w", mountPath, err)
	}
	if err == gorm.ErrRecordNotFound {
		dg := db.DiskGroup{
			MountPath:          mountPath,
			ThresholdPct:       threshold,
			TargetPct:          target,
			TotalBytesOverride: totalOverride,
		}
		if createErr := s.db.Create(&dg).Error; createErr != nil {
			return fmt.Errorf("failed to create disk group %q: %w", mountPath, createErr)
		}
	} else {
		existing.ThresholdPct = threshold
		existing.TargetPct = target
		existing.TotalBytesOverride = totalOverride
		if saveErr := s.db.Save(&existing).Error; saveErr != nil {
			return fmt.Errorf("failed to update disk group %q: %w", mountPath, saveErr)
		}
	}
	return nil
}

// SyncIntegrationLinks replaces the integration associations for a disk group.
// Called by the poller after upserting disk groups to track which integrations
// reported each mount path.
func (s *DiskGroupService) SyncIntegrationLinks(diskGroupID uint, integrationIDs []uint) error {
	// Delete existing links for this disk group
	if err := s.db.Where("disk_group_id = ?", diskGroupID).Delete(&db.DiskGroupIntegration{}).Error; err != nil {
		return fmt.Errorf("failed to clear integration links for disk group %d: %w", diskGroupID, err)
	}

	// Insert new links
	for _, intID := range integrationIDs {
		link := db.DiskGroupIntegration{
			DiskGroupID:   diskGroupID,
			IntegrationID: intID,
		}
		if err := s.db.Create(&link).Error; err != nil {
			return fmt.Errorf("failed to create integration link (dg=%d, int=%d): %w", diskGroupID, intID, err)
		}
	}

	return nil
}

// DiskGroupWithIntegrations is a disk group enriched with its associated integration info.
type DiskGroupWithIntegrations struct {
	db.DiskGroup
	Integrations []IntegrationInfo `json:"integrations"`
}

// IntegrationInfo is a lightweight representation of an integration for API responses.
type IntegrationInfo struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// ListWithIntegrations returns all disk groups enriched with their associated
// integration names and types from the junction table.
func (s *DiskGroupService) ListWithIntegrations() ([]DiskGroupWithIntegrations, error) {
	groups, err := s.List()
	if err != nil {
		return nil, err
	}

	if len(groups) == 0 {
		return []DiskGroupWithIntegrations{}, nil
	}

	// Collect all group IDs
	groupIDs := make([]uint, len(groups))
	for i, g := range groups {
		groupIDs[i] = g.ID
	}

	// Fetch all junction rows + integration info in one query
	type linkRow struct {
		DiskGroupID     uint
		IntegrationID   uint
		IntegrationName string
		IntegrationType string
	}
	var rows []linkRow
	s.db.Table("disk_group_integrations").
		Select("disk_group_integrations.disk_group_id, disk_group_integrations.integration_id, integration_configs.name AS integration_name, integration_configs.type AS integration_type").
		Joins("JOIN integration_configs ON integration_configs.id = disk_group_integrations.integration_id").
		Where("disk_group_integrations.disk_group_id IN ?", groupIDs).
		Scan(&rows)

	// Build a map of group ID → integration info
	linkMap := make(map[uint][]IntegrationInfo)
	for _, r := range rows {
		linkMap[r.DiskGroupID] = append(linkMap[r.DiskGroupID], IntegrationInfo{
			ID:   r.IntegrationID,
			Name: r.IntegrationName,
			Type: r.IntegrationType,
		})
	}

	// Assemble result
	result := make([]DiskGroupWithIntegrations, len(groups))
	for i, g := range groups {
		integs := linkMap[g.ID]
		if integs == nil {
			integs = []IntegrationInfo{}
		}
		result[i] = DiskGroupWithIntegrations{
			DiskGroup:    g,
			Integrations: integs,
		}
	}

	return result, nil
}
