package services

import (
	"fmt"
	"log/slog"
	"math"
	"time"

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

// DeletionQueueGroupClearer allows DiskGroupService to clear deletion queue
// entries for a specific disk group when its mode changes. Defined as an
// interface to avoid a direct dependency on DeletionService.
type DeletionQueueGroupClearer interface {
	ClearQueueForDiskGroup(diskGroupID uint) int
}

// DiskGroupService manages disk group lifecycle: discovery, reconciliation,
// threshold configuration, and integration tracking.
type DiskGroupService struct {
	db              *gorm.DB
	bus             *events.EventBus
	engine          EngineRunTrigger          // optional; wired via SetEngineService()
	settings        SettingsReader            // optional; wired via SetSettingsReader()
	deletionClearer DeletionQueueGroupClearer // optional; wired via SetDeletionClearer()
}

// NewDiskGroupService creates a new DiskGroupService.
func NewDiskGroupService(database *gorm.DB, bus *events.EventBus) *DiskGroupService {
	return &DiskGroupService{db: database, bus: bus}
}

// Wired returns true when all lazily-injected dependencies are non-nil.
// Used by Registry.Validate() to catch missing wiring at startup.
func (s *DiskGroupService) Wired() bool {
	return s.engine != nil && s.settings != nil && s.deletionClearer != nil
}

// SetEngineService wires the EngineService dependency so that threshold changes
// can trigger an immediate engine run for queue reconciliation.
func (s *DiskGroupService) SetEngineService(engine EngineRunTrigger) {
	s.engine = engine
}

// SetSettingsReader wires the SettingsReader dependency so that newly
// auto-discovered disk groups inherit the DefaultDiskGroupMode preference.
func (s *DiskGroupService) SetSettingsReader(settings SettingsReader) {
	s.settings = settings
}

// SetDeletionClearer wires the DeletionQueueGroupClearer dependency so that
// mode changes on a disk group clear its queued deletion items.
func (s *DiskGroupService) SetDeletionClearer(clearer DeletionQueueGroupClearer) {
	s.deletionClearer = clearer
}

// List returns all disk groups.
func (s *DiskGroupService) List() ([]db.DiskGroup, error) {
	groups := make([]db.DiskGroup, 0)
	if err := s.db.Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch disk groups: %w", err)
	}
	return groups, nil
}

// GetForecastTarget resolves the disk group to use for capacity forecasting.
// If diskGroupID is non-nil, returns that specific group. Otherwise, returns
// the most degraded group (highest usage percentage). Returns an error if
// no disk groups exist or the specified group is not found.
func (s *DiskGroupService) GetForecastTarget(diskGroupID *uint) (*DiskGroupForForecast, error) {
	groups, err := s.List()
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, nil // No disk groups — caller should return empty forecast
	}

	if diskGroupID != nil {
		for i := range groups {
			if groups[i].ID == *diskGroupID {
				eff := groups[i].EffectiveTotalBytes()
				return &DiskGroupForForecast{
					ID:            groups[i].ID,
					ThresholdPct:  groups[i].ThresholdPct,
					TotalCapacity: eff,
					UsedCapacity:  groups[i].UsedBytes,
				}, nil
			}
		}
		return nil, fmt.Errorf("disk group %d not found", *diskGroupID)
	}

	// Default: most degraded group (highest usage percentage)
	bestIdx := 0
	bestPct := 0.0
	for i, g := range groups {
		eff := g.EffectiveTotalBytes()
		if eff > 0 {
			pct := float64(g.UsedBytes) / float64(eff) * 100
			if pct > bestPct {
				bestPct = pct
				bestIdx = i
			}
		}
	}
	eff := groups[bestIdx].EffectiveTotalBytes()
	return &DiskGroupForForecast{
		ID:            groups[bestIdx].ID,
		ThresholdPct:  groups[bestIdx].ThresholdPct,
		TotalCapacity: eff,
		UsedCapacity:  groups[bestIdx].UsedBytes,
	}, nil
}

// GetByID returns a single disk group by ID.
func (s *DiskGroupService) GetByID(id uint) (*db.DiskGroup, error) {
	var group db.DiskGroup
	if err := s.db.First(&group, id).Error; err != nil {
		return nil, fmt.Errorf("disk group not found: %w", err)
	}
	return &group, nil
}

// GetModeForIntegration resolves the execution mode for a given integration
// by looking up its linked disk group via the DiskGroupIntegration junction
// table. Returns the disk group's Mode if found, or an empty string if the
// integration has no linked disk group.
func (s *DiskGroupService) GetModeForIntegration(integrationID uint) (string, error) {
	var diskGroupID uint
	if err := s.db.Raw(
		"SELECT disk_group_id FROM disk_group_integrations WHERE integration_id = ? LIMIT 1",
		integrationID,
	).Scan(&diskGroupID).Error; err != nil {
		return "", fmt.Errorf("failed to query disk group integrations: %w", err)
	}
	if diskGroupID == 0 {
		return "", nil // No link found — caller should fall back to preference
	}
	var group db.DiskGroup
	if err := s.db.First(&group, diskGroupID).Error; err != nil {
		return "", fmt.Errorf("disk group %d not found for integration %d: %w", diskGroupID, integrationID, err)
	}
	return group.Mode, nil
}

// Upsert creates or updates a disk group from discovered disk space.
// Shared by the sync route and the poller. If an existing group is stale
// (stale_since != NULL), it is resurrected — stale_since is cleared and
// all configuration (thresholds, mode, override) is preserved.
func (s *DiskGroupService) Upsert(disk integrations.DiskSpace) (*db.DiskGroup, error) {
	var group db.DiskGroup
	result := s.db.Where("mount_path = ?", disk.Path).First(&group)

	usedBytes := disk.TotalBytes - disk.FreeBytes

	if result.Error != nil {
		// Create new disk group, inheriting the default mode from preferences.
		group = db.DiskGroup{
			MountPath:  disk.Path,
			TotalBytes: disk.TotalBytes,
			UsedBytes:  usedBytes,
		}
		if s.settings != nil {
			if prefs, err := s.settings.GetPreferences(); err == nil && prefs.DefaultDiskGroupMode != "" {
				group.Mode = prefs.DefaultDiskGroupMode
			}
		}
		if err := s.db.Create(&group).Error; err != nil {
			return nil, fmt.Errorf("failed to create disk group: %w", err)
		}
	} else {
		// Resurrect if stale: clear stale_since alongside the byte update.
		// gorm.Expr("NULL") is required because GORM skips nil pointer values in maps.
		wasStale := group.StaleSince != nil
		if err := s.db.Model(&group).Updates(map[string]any{
			"total_bytes": disk.TotalBytes,
			"used_bytes":  usedBytes,
			"stale_since": gorm.Expr("NULL"),
		}).Error; err != nil {
			return nil, fmt.Errorf("failed to update disk group: %w", err)
		}

		if wasStale {
			staleDays := int(math.Ceil(time.Since(*group.StaleSince).Hours() / 24))
			slog.Info("Resurrected stale disk group",
				"component", "diskgroup_service", "mount", group.MountPath, "staleDays", staleDays)
			s.bus.Publish(events.DiskGroupResurrectedEvent{
				DiskGroupID: group.ID,
				MountPath:   group.MountPath,
				StaleDays:   staleDays,
			})
			group.StaleSince = nil
		}
	}

	return &group, nil
}

// UpdateThresholds updates the threshold and target percentages for a disk group,
// along with an optional total-bytes override, and returns the updated group.
func (s *DiskGroupService) UpdateThresholds(groupID uint, threshold, target float64, totalOverride *int64, mode string, sunsetPct *float64) (*db.DiskGroup, error) {
	var group db.DiskGroup
	if err := s.db.First(&group, groupID).Error; err != nil {
		return nil, fmt.Errorf("disk group not found: %w", err)
	}

	// Validate sunset configuration at save-time. Rejects invalid configs
	// rather than letting them fail silently at engine evaluation time.
	effectiveMode := mode
	if effectiveMode == "" {
		effectiveMode = group.Mode
	}
	if err := ValidateSunsetConfig(effectiveMode, sunsetPct, target, threshold); err != nil {
		return nil, err
	}

	oldMode := group.Mode

	updates := map[string]any{
		"threshold_pct": threshold,
		"target_pct":    target,
	}
	if totalOverride != nil && *totalOverride > 0 {
		updates["total_bytes_override"] = *totalOverride
	}
	if mode != "" {
		updates["mode"] = mode
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
	// Handle sunset_pct: set if mode is sunset and value provided, clear otherwise
	if mode == db.ModeSunset && sunsetPct != nil {
		if err := s.db.Model(&group).Update("sunset_pct", *sunsetPct).Error; err != nil {
			return nil, fmt.Errorf("failed to set sunset threshold: %w", err)
		}
	} else if mode != "" && mode != db.ModeSunset {
		// Clear sunset_pct when switching away from sunset mode
		if err := s.db.Model(&group).Update("sunset_pct", gorm.Expr("NULL")).Error; err != nil {
			return nil, fmt.Errorf("failed to clear sunset threshold: %w", err)
		}
	}

	// Clear deletion queue for this disk group when mode changes.
	// Any mode change invalidates the assumptions under which items were queued —
	// the engine will re-evaluate and re-queue as appropriate under the new mode.
	if mode != "" && mode != oldMode && s.deletionClearer != nil {
		cleared := s.deletionClearer.ClearQueueForDiskGroup(group.ID)
		if cleared > 0 {
			slog.Info("Cleared deletion queue on disk group mode change",
				"component", "diskgroup_service",
				"mount", group.MountPath,
				"oldMode", oldMode,
				"newMode", mode,
				"cleared", cleared)
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
		slog.Error("Failed to clear disk group integration links", "component", "diskgroup_service", "error", err)
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

// ReconcileActiveMounts marks disk groups stale whose mount paths are not in
// the provided set of active mount paths. Groups already stale are left
// untouched (their stale_since clock is not reset). Junction table links are
// preserved so that resurrected groups retain their integration associations.
// Returns the count of newly-stale groups (not total stale).
func (s *DiskGroupService) ReconcileActiveMounts(activeMounts map[string]bool) (int64, error) {
	var allGroups []db.DiskGroup
	if err := s.db.Find(&allGroups).Error; err != nil {
		return 0, fmt.Errorf("failed to fetch disk groups: %w", err)
	}

	var newlyStale int64
	for _, g := range allGroups {
		if !activeMounts[g.MountPath] {
			if g.StaleSince != nil {
				continue // Already stale — don't reset the clock
			}
			if err := s.markStaleByID(g.ID, g.MountPath); err != nil {
				return newlyStale, fmt.Errorf("failed to mark disk group %q stale: %w", g.MountPath, err)
			}
			newlyStale++
		}
	}

	return newlyStale, nil
}

// ImportUpsert creates or updates a disk group from backup import data.
// Only configuration fields (thresholds, override) are imported — discovery
// fields (total_bytes, used_bytes) are left at zero for new groups since
// they will be populated by the next poll cycle. If the target group is stale,
// it is resurrected (stale_since cleared).
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
		existing.StaleSince = nil // Resurrect if stale — import is an explicit user action
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

// markStaleByID sets stale_since = NOW() on a single disk group by ID and
// publishes a DiskGroupStaleEvent. Internal helper — callers are responsible
// for checking that the group is not already stale.
func (s *DiskGroupService) markStaleByID(id uint, mountPath string) error {
	now := time.Now()
	if err := s.db.Model(&db.DiskGroup{}).Where("id = ?", id).
		Update("stale_since", now).Error; err != nil {
		return fmt.Errorf("failed to mark disk group %d stale: %w", id, err)
	}
	slog.Info("Marked disk group stale",
		"component", "diskgroup_service", "id", id, "mount", mountPath)
	s.bus.Publish(events.DiskGroupStaleEvent{
		DiskGroupID: id,
		MountPath:   mountPath,
	})
	return nil
}

// MarkStale sets stale_since = NOW() on a single disk group if it is not
// already stale (idempotent — does not reset the clock on already-stale groups).
func (s *DiskGroupService) MarkStale(id uint) error {
	var group db.DiskGroup
	if err := s.db.First(&group, id).Error; err != nil {
		return fmt.Errorf("disk group not found: %w", err)
	}
	if group.StaleSince != nil {
		return nil // Already stale — don't reset the clock
	}
	return s.markStaleByID(group.ID, group.MountPath)
}

// MarkAllStale sets stale_since = NOW() on all active disk groups (where
// stale_since IS NULL). Returns the count of newly-stale groups. Does NOT
// touch junction table links — those are preserved for resurrection.
func (s *DiskGroupService) MarkAllStale() (int64, error) {
	// Fetch active groups first to publish per-group events
	var activeGroups []db.DiskGroup
	if err := s.db.Where("stale_since IS NULL").Find(&activeGroups).Error; err != nil {
		return 0, fmt.Errorf("failed to fetch active disk groups: %w", err)
	}
	if len(activeGroups) == 0 {
		return 0, nil
	}

	now := time.Now()
	result := s.db.Model(&db.DiskGroup{}).
		Where("stale_since IS NULL").
		Update("stale_since", now)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to mark all disk groups stale: %w", result.Error)
	}

	// Publish per-group events for activity feed
	for _, g := range activeGroups {
		slog.Info("Marked disk group stale",
			"component", "diskgroup_service", "id", g.ID, "mount", g.MountPath)
		s.bus.Publish(events.DiskGroupStaleEvent{
			DiskGroupID: g.ID,
			MountPath:   g.MountPath,
		})
	}

	return result.RowsAffected, nil
}

// ReapStale deletes disk groups whose grace period has expired (stale_since
// is set and older than gracePeriodDays). Also deletes corresponding junction
// table rows. If gracePeriodDays == 0, all stale groups are reaped immediately.
// Returns the count of reaped groups.
func (s *DiskGroupService) ReapStale(gracePeriodDays int) (int64, error) {
	// SELECT stale groups past the grace period to publish per-group events
	cutoff := time.Now().AddDate(0, 0, -gracePeriodDays)
	var expired []db.DiskGroup
	query := s.db.Where("stale_since IS NOT NULL AND stale_since < ?", cutoff)
	if err := query.Find(&expired).Error; err != nil {
		return 0, fmt.Errorf("failed to find expired stale disk groups: %w", err)
	}
	if len(expired) == 0 {
		return 0, nil
	}

	// Collect IDs for batch delete
	expiredIDs := make([]uint, len(expired))
	for i, g := range expired {
		expiredIDs[i] = g.ID
	}

	// Delete junction table rows
	if err := s.db.Where("disk_group_id IN ?", expiredIDs).
		Delete(&db.DiskGroupIntegration{}).Error; err != nil {
		slog.Error("Failed to clear junction rows for reaped disk groups",
			"component", "diskgroup_service", "error", err)
	}

	// Delete the disk groups
	result := s.db.Where("id IN ?", expiredIDs).Delete(&db.DiskGroup{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to reap stale disk groups: %w", result.Error)
	}

	// Publish per-group events for activity feed
	for _, g := range expired {
		staleDays := int(math.Ceil(time.Since(*g.StaleSince).Hours() / 24))
		slog.Info("Reaped expired stale disk group",
			"component", "diskgroup_service", "id", g.ID, "mount", g.MountPath, "staleDays", staleDays)
		s.bus.Publish(events.DiskGroupReapedEvent{
			DiskGroupID: g.ID,
			MountPath:   g.MountPath,
			StaleDays:   staleDays,
		})
	}

	return result.RowsAffected, nil
}

// ListStale returns disk groups where stale_since IS NOT NULL, ordered by
// stale_since ascending (oldest stale first).
func (s *DiskGroupService) ListStale() ([]db.DiskGroup, error) {
	var groups []db.DiskGroup
	if err := s.db.Where("stale_since IS NOT NULL").
		Order("stale_since ASC").
		Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("failed to list stale disk groups: %w", err)
	}
	return groups, nil
}

// SunsetLinkedIntegrationIDs returns the set of integration IDs that are linked
// to at least one disk group with mode = "sunset". Used for batch override
// computation in API list endpoints to avoid N+1 queries.
func (s *DiskGroupService) SunsetLinkedIntegrationIDs() (map[uint]bool, error) {
	var ids []uint
	err := s.db.Table("disk_group_integrations").
		Joins("JOIN disk_groups ON disk_groups.id = disk_group_integrations.disk_group_id").
		Where("disk_groups.mode = ?", db.ModeSunset).
		Pluck("disk_group_integrations.integration_id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list sunset-linked integration IDs: %w", err)
	}
	result := make(map[uint]bool, len(ids))
	for _, id := range ids {
		result[id] = true
	}
	return result, nil
}

// HasSunsetModeForIntegration returns true if the given integration is linked
// to at least one disk group with mode = "sunset". Used by IntegrationService
// to compute the virtual ShowLevelOnly override.
func (s *DiskGroupService) HasSunsetModeForIntegration(integrationID uint) (bool, error) {
	var count int64
	err := s.db.Table("disk_group_integrations").
		Joins("JOIN disk_groups ON disk_groups.id = disk_group_integrations.disk_group_id").
		Where("disk_group_integrations.integration_id = ? AND disk_groups.mode = ?", integrationID, db.ModeSunset).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check sunset-mode linkage for integration %d: %w", integrationID, err)
	}
	return count > 0, nil
}
