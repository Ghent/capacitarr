package services

import (
	"fmt"
	"log/slog"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"

	"gorm.io/gorm"
)

// SunsetService manages the sunset queue lifecycle. Handles countdown tracking,
// cancellation, rescheduling, expiry processing, escalation, and label management.
// Does NOT perform deletions — hands items to DeletionService when countdowns expire.
//
// Follows the established service pattern: accepts *gorm.DB and *events.EventBus
// in constructor; registered on services.Registry as reg.Sunset.
type SunsetService struct {
	db  *gorm.DB
	bus *events.EventBus
}

// PreviewScoreReader provides read access to cached preview scores for
// sunset re-scoring. Satisfied by PreviewService.
type PreviewScoreReader interface {
	GetCachedScoreMap() map[string]float64
}

// SunsetDeps holds service dependencies for label management and deletion handoff.
// Follows the same pattern as ExecuteApprovalDeps in approval.go.
// SettingsReader is the existing interface defined in deletion.go — reuse it.
type SunsetDeps struct {
	Registry      *integrations.IntegrationRegistry
	Deletion      *DeletionService
	Engine        *EngineService
	Settings      SettingsReader
	Preview       PreviewScoreReader    // Optional: provides current scores for rescore comparisons
	PosterOverlay *PosterOverlayService // Optional: if set, posters are restored on cancel/expire/escalate
	Mapping       *MappingService       // Persistent TMDb→NativeID mapping; replaces ephemeral BuildMappingMaps()
}

// NewSunsetService creates a new sunset queue service.
func NewSunsetService(database *gorm.DB, bus *events.EventBus) *SunsetService {
	return &SunsetService{db: database, bus: bus}
}

// QueueSunset creates a new sunset_queue entry with a deletion_date.
// Also applies the sunset label to the item in all enabled media servers.
func (s *SunsetService) QueueSunset(item db.SunsetQueueItem, deps SunsetDeps) error {
	if err := s.db.Create(&item).Error; err != nil {
		return fmt.Errorf("create sunset queue item: %w", err)
	}

	// Apply label to media servers
	if deps.Registry != nil && deps.Settings != nil {
		if prefs, err := deps.Settings.GetPreferences(); err == nil && prefs.SunsetLabel != "" {
			if s.applyLabel(item, prefs.SunsetLabel, deps.Registry, deps.Mapping) {
				item.LabelApplied = true
				s.db.Model(&item).Update("label_applied", true)
			}
		}
	}

	daysRemaining := s.DaysRemaining(item)
	s.bus.Publish(events.SunsetCreatedEvent{
		MediaName:     item.MediaName,
		MediaType:     item.MediaType,
		DiskGroupID:   item.DiskGroupID,
		DaysRemaining: daysRemaining,
		DeletionDate:  item.DeletionDate.Format("2006-01-02"),
	})

	return nil
}

// BulkQueueSunset creates multiple sunset entries in a transaction.
// Applies labels to all items in enabled media servers.
func (s *SunsetService) BulkQueueSunset(items []db.SunsetQueueItem, deps SunsetDeps) (int, error) {
	if len(items) == 0 {
		return 0, nil
	}

	var prefs db.PreferenceSet
	if deps.Settings != nil {
		var err error
		prefs, err = deps.Settings.GetPreferences()
		if err != nil {
			return 0, fmt.Errorf("load preferences: %w", err)
		}
	}

	created := 0
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for i := range items {
			if err := tx.Create(&items[i]).Error; err != nil {
				return fmt.Errorf("create item %d: %w", i, err)
			}
			created++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	// Apply labels outside the transaction (media server calls shouldn't block DB)
	if deps.Registry != nil && prefs.SunsetLabel != "" {
		for i := range items {
			if s.applyLabel(items[i], prefs.SunsetLabel, deps.Registry, deps.Mapping) {
				items[i].LabelApplied = true
				s.db.Model(&items[i]).Update("label_applied", true)
			}
		}
	}

	// Publish events
	for _, item := range items {
		s.bus.Publish(events.SunsetCreatedEvent{
			MediaName:     item.MediaName,
			MediaType:     item.MediaType,
			DiskGroupID:   item.DiskGroupID,
			DaysRemaining: s.DaysRemaining(item),
			DeletionDate:  item.DeletionDate.Format("2006-01-02"),
		})
	}

	return created, nil
}

// Cancel removes a sunset item. Removes the label from media servers.
// Publishes sunset_cancelled event.
func (s *SunsetService) Cancel(entryID uint, deps SunsetDeps) error {
	var item db.SunsetQueueItem
	if err := s.db.First(&item, entryID).Error; err != nil {
		return fmt.Errorf("find sunset item %d: %w", entryID, err)
	}

	// Restore poster overlay
	if item.PosterOverlayActive && deps.PosterOverlay != nil && deps.Registry != nil {
		if err := deps.PosterOverlay.RestoreOriginal(item, PosterDeps{Registry: deps.Registry, Mapping: deps.Mapping}); err != nil {
			slog.Error("Failed to restore poster on cancel",
				"component", "services", "mediaName", item.MediaName, "error", err)
		}
	}

	// Remove label from media servers
	if item.LabelApplied && deps.Registry != nil && deps.Settings != nil {
		if prefs, err := deps.Settings.GetPreferences(); err == nil && prefs.SunsetLabel != "" {
			s.removeLabel(item, prefs.SunsetLabel, deps.Registry, deps.Mapping)
		}
	}

	if err := s.db.Delete(&item).Error; err != nil {
		return fmt.Errorf("delete sunset item %d: %w", entryID, err)
	}

	s.bus.Publish(events.SunsetCancelledEvent{
		MediaName:   item.MediaName,
		MediaType:   item.MediaType,
		DiskGroupID: item.DiskGroupID,
	})

	return nil
}

// Reschedule updates the deletion_date for a sunset item.
func (s *SunsetService) Reschedule(entryID uint, newDate time.Time) (*db.SunsetQueueItem, error) {
	var item db.SunsetQueueItem
	if err := s.db.First(&item, entryID).Error; err != nil {
		return nil, fmt.Errorf("find sunset item %d: %w", entryID, err)
	}

	item.DeletionDate = newDate
	if err := s.db.Save(&item).Error; err != nil {
		return nil, fmt.Errorf("update sunset item %d: %w", entryID, err)
	}

	s.bus.Publish(events.SunsetRescheduledEvent{
		MediaName:        item.MediaName,
		MediaType:        item.MediaType,
		DiskGroupID:      item.DiskGroupID,
		NewDaysRemaining: s.DaysRemaining(item),
		NewDeletionDate:  newDate.Format("2006-01-02"),
	})

	return &item, nil
}

// ListForDiskGroup returns all sunset items for a given disk group.
func (s *SunsetService) ListForDiskGroup(diskGroupID uint) ([]db.SunsetQueueItem, error) {
	var items []db.SunsetQueueItem
	err := s.db.Where("disk_group_id = ?", diskGroupID).Order("deletion_date ASC").Find(&items).Error
	return items, err
}

// ListAll returns all sunset items across all disk groups, ordered by deletion_date.
func (s *SunsetService) ListAll() ([]db.SunsetQueueItem, error) {
	var items []db.SunsetQueueItem
	err := s.db.Order("deletion_date ASC").Find(&items).Error
	return items, err
}

// GetExpired returns items where deletion_date <= now that have not already
// been handed to DeletionService (expired_at IS NULL). Ordered by score DESC
// so highest-priority items are processed first by ProcessExpired().
func (s *SunsetService) GetExpired() ([]db.SunsetQueueItem, error) {
	var items []db.SunsetQueueItem
	err := s.db.Where("deletion_date <= ? AND expired_at IS NULL", time.Now().UTC()).Order("score DESC").Find(&items).Error
	return items, err
}

// DaysRemaining calculates the countdown for a given sunset item.
// Returns 0 if the deletion date is in the past.
func (s *SunsetService) DaysRemaining(item db.SunsetQueueItem) int {
	remaining := int(time.Until(item.DeletionDate).Hours() / 24)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ProcessExpired transitions all expired items to DeletionService.
// Removes labels from media servers. Called by the daily cron job.
func (s *SunsetService) ProcessExpired(deps SunsetDeps) (int, error) {
	expired, err := s.GetExpired()
	if err != nil {
		return 0, fmt.Errorf("get expired items: %w", err)
	}
	if len(expired) == 0 {
		return 0, nil
	}

	var prefs db.PreferenceSet
	if deps.Settings != nil {
		var prefsErr error
		prefs, prefsErr = deps.Settings.GetPreferences()
		if prefsErr != nil {
			slog.Error("Failed to load preferences for sunset expiry — label removal may be skipped",
				"component", "services", "error", prefsErr)
		}
	}

	processed := 0
	for _, item := range expired {
		if s.processExpiredItem(item, prefs, deps) {
			processed++
		}
	}

	return processed, nil
}

// RescoreAndSave checks each pending sunset item against the current preview
// cache scores. If an item's current score dropped below 50% of its original
// score at queue time, it transitions to "saved" status instead of continuing
// the countdown — the item has seen enough new activity to warrant keeping.
// Called by the daily cron when SunsetRescoreEnabled is true.
//
// The prefs and weights parameters are passed by the caller (cron job) to avoid
// interface mismatch — SunsetDeps.Settings is a SettingsReader which may not
// have GetWeightMap on all implementations. The weights parameter is reserved
// for future full engine re-scoring integration.
func (s *SunsetService) RescoreAndSave(deps SunsetDeps, prefs db.PreferenceSet, weights map[string]int) (int, error) {
	// weights will be used for full engine re-scoring in a future iteration.
	_ = weights

	// Get pending items (not yet expired, not already saved)
	var items []db.SunsetQueueItem
	if err := s.db.Where("status = ? AND expired_at IS NULL", db.SunsetStatusPending).Find(&items).Error; err != nil {
		return 0, fmt.Errorf("list pending sunset items for rescore: %w", err)
	}
	if len(items) == 0 {
		return 0, nil
	}

	// Look up the current preview cache to obtain fresh scores.
	// If the preview cache is unavailable (nil PreviewDataSource or empty
	// cache), we skip re-scoring for this cycle rather than producing
	// incorrect results.
	currentScores := s.buildScoreLookup(deps)

	saved := 0
	for _, item := range items {
		// Look up the item's current score from the preview cache. If the
		// item is no longer in the cache (e.g., already removed from the
		// *arr integration), keep the original score unchanged.
		key := item.MediaName + "|" + item.MediaType
		newScore, found := currentScores[key]
		if !found {
			continue // Item not in current preview — skip this cycle
		}

		// If the current score dropped below 50% of the original score at
		// queue time, the item has seen enough new activity to warrant saving.
		if newScore < item.Score*0.5 {
			now := time.Now().UTC()
			reason := fmt.Sprintf("Score dropped from %.1f to %.1f due to recent activity", item.Score, newScore)

			s.db.Model(&item).Updates(map[string]any{
				"status":       db.SunsetStatusSaved,
				"saved_at":     now,
				"saved_score":  newScore,
				"saved_reason": reason,
			})

			// Replace sunset label with saved label
			if item.LabelApplied && deps.Registry != nil {
				s.removeLabel(item, prefs.SunsetLabel, deps.Registry, deps.Mapping)
				s.applyLabel(item, prefs.SavedLabel, deps.Registry, deps.Mapping)
			}

			// Replace countdown overlay with the green "Saved" badge
			if item.PosterOverlayActive && deps.PosterOverlay != nil && deps.Registry != nil {
				if overlayErr := deps.PosterOverlay.UpdateSavedOverlay(item, PosterDeps{
					Registry: deps.Registry, Mapping: deps.Mapping,
				}); overlayErr != nil {
					slog.Error("Failed to update poster with saved overlay",
						"component", "services", "mediaName", item.MediaName, "error", overlayErr)
				}
			}

			s.bus.Publish(events.SunsetSavedEvent{
				MediaName:     item.MediaName,
				MediaType:     item.MediaType,
				DiskGroupID:   item.DiskGroupID,
				OriginalScore: item.Score,
				NewScore:      newScore,
			})
			saved++
			slog.Info("Sunset item saved by popular demand",
				"component", "services", "mediaName", item.MediaName,
				"originalScore", fmt.Sprintf("%.1f", item.Score),
				"newScore", fmt.Sprintf("%.1f", newScore))
		}
	}

	return saved, nil
}

// buildScoreLookup returns a map of "MediaName|MediaType" → current score
// from the preview cache. Returns an empty map if no preview data is available.
func (s *SunsetService) buildScoreLookup(deps SunsetDeps) map[string]float64 {
	if deps.Preview == nil {
		return map[string]float64{}
	}
	return deps.Preview.GetCachedScoreMap()
}

// CleanupSaved removes saved items whose saved marker duration has expired.
// Restores original posters and removes saved labels. Called by the daily cron.
func (s *SunsetService) CleanupSaved(deps SunsetDeps) (int, error) {
	prefs, prefsErr := deps.Settings.GetPreferences()
	if prefsErr != nil {
		return 0, fmt.Errorf("load preferences for saved cleanup: %w", prefsErr)
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -prefs.SavedDurationDays)

	var items []db.SunsetQueueItem
	if err := s.db.Where("status = ? AND saved_at IS NOT NULL AND saved_at <= ?", db.SunsetStatusSaved, cutoff).Find(&items).Error; err != nil {
		return 0, fmt.Errorf("list expired saved items: %w", err)
	}
	if len(items) == 0 {
		return 0, nil
	}

	cleaned := 0
	for _, item := range items {
		// Restore original poster
		if item.PosterOverlayActive && deps.PosterOverlay != nil && deps.Registry != nil {
			if err := deps.PosterOverlay.RestoreOriginal(item, PosterDeps{
				Registry: deps.Registry, Mapping: deps.Mapping,
			}); err != nil {
				slog.Error("Failed to restore poster during saved cleanup",
					"component", "services", "mediaName", item.MediaName, "error", err)
			}
		}

		// Remove saved label
		if item.LabelApplied && deps.Registry != nil {
			s.removeLabel(item, prefs.SavedLabel, deps.Registry, deps.Mapping)
		}

		// Hard-delete the record
		if err := s.db.Delete(&item).Error; err != nil {
			slog.Error("Failed to delete saved sunset item",
				"component", "services", "mediaName", item.MediaName, "error", err)
			continue
		}

		s.bus.Publish(events.SunsetSavedCleanedEvent{
			MediaName:   item.MediaName,
			MediaType:   item.MediaType,
			DiskGroupID: item.DiskGroupID,
		})
		cleaned++
	}

	return cleaned, nil
}

// Escalate force-expires sunset items for a disk group during threshold breach.
// Processes expired first, then oldest-in-queue, freeing only enough to reach
// targetBytes. Returns bytes freed.
func (s *SunsetService) Escalate(diskGroupID uint, targetBytes int64, deps SunsetDeps) (int64, error) {
	var freedBytes int64
	itemsExpired := 0

	var prefs db.PreferenceSet
	if deps.Settings != nil {
		var prefsErr error
		prefs, prefsErr = deps.Settings.GetPreferences()
		if prefsErr != nil {
			slog.Error("Failed to load preferences for sunset escalation — label removal may be skipped",
				"component", "services", "error", prefsErr)
		}
	}

	// Step 1: Delete already-expired items first (skip those already handed off).
	// Ordered by score DESC so highest-priority items are escalated first.
	var expired []db.SunsetQueueItem
	s.db.Where("disk_group_id = ? AND deletion_date <= ? AND expired_at IS NULL", diskGroupID, time.Now().UTC()).
		Order("score DESC").Find(&expired)

	for _, item := range expired {
		if freedBytes >= targetBytes {
			break
		}
		if s.processExpiredItem(item, prefs, deps) {
			freedBytes += item.SizeBytes
			itemsExpired++
		}
	}

	if freedBytes >= targetBytes {
		s.publishEscalationEvent(diskGroupID, itemsExpired, freedBytes)
		return freedBytes, nil
	}

	// Step 2: Delete highest-score items that haven't expired yet.
	// Previously ordered by created_at ASC ("most warning given"), but this
	// caused low-score items to be deleted before high-score items when items
	// were added across multiple engine cycles.
	var oldest []db.SunsetQueueItem
	s.db.Where("disk_group_id = ? AND deletion_date > ? AND expired_at IS NULL", diskGroupID, time.Now().UTC()).
		Order("score DESC").Find(&oldest)

	for _, item := range oldest {
		if freedBytes >= targetBytes {
			break
		}
		if s.processExpiredItem(item, prefs, deps) {
			freedBytes += item.SizeBytes
			itemsExpired++
		}
	}

	s.publishEscalationEvent(diskGroupID, itemsExpired, freedBytes)
	return freedBytes, nil
}

// CancelAll cancels all sunset items (emergency button). Returns count removed.
func (s *SunsetService) CancelAll(deps SunsetDeps) (int, error) {
	items, err := s.ListAll()
	if err != nil {
		return 0, err
	}

	for _, item := range items {
		// Restore poster overlays before clearing
		if item.PosterOverlayActive && deps.PosterOverlay != nil && deps.Registry != nil {
			if err := deps.PosterOverlay.RestoreOriginal(item, PosterDeps{Registry: deps.Registry, Mapping: deps.Mapping}); err != nil {
				slog.Error("Failed to restore poster during clear-all",
					"component", "services", "mediaName", item.MediaName, "error", err)
			}
		}
	}

	if deps.Registry != nil && deps.Settings != nil {
		if prefs, prefsErr := deps.Settings.GetPreferences(); prefsErr == nil && prefs.SunsetLabel != "" {
			for _, item := range items {
				if item.LabelApplied {
					s.removeLabel(item, prefs.SunsetLabel, deps.Registry, deps.Mapping)
				}
			}
		}
	}

	result := s.db.Where("1 = 1").Delete(&db.SunsetQueueItem{})
	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}

// CancelAllForDiskGroup cancels all sunset items for a specific disk group.
// Production code uses CancelAll (all groups); this per-group variant exists
// for test convenience.
func (s *SunsetService) CancelAllForDiskGroup(diskGroupID uint, deps SunsetDeps) (int, error) {
	items, err := s.ListForDiskGroup(diskGroupID)
	if err != nil {
		return 0, err
	}

	for _, item := range items {
		// Restore poster overlays before clearing
		if item.PosterOverlayActive && deps.PosterOverlay != nil && deps.Registry != nil {
			if err := deps.PosterOverlay.RestoreOriginal(item, PosterDeps{Registry: deps.Registry, Mapping: deps.Mapping}); err != nil {
				slog.Error("Failed to restore poster during disk-group clear",
					"component", "services", "mediaName", item.MediaName, "error", err)
			}
		}
	}

	if deps.Registry != nil && deps.Settings != nil {
		if prefs, prefsErr := deps.Settings.GetPreferences(); prefsErr == nil && prefs.SunsetLabel != "" {
			for _, item := range items {
				if item.LabelApplied {
					s.removeLabel(item, prefs.SunsetLabel, deps.Registry, deps.Mapping)
				}
			}
		}
	}

	result := s.db.Where("disk_group_id = ?", diskGroupID).Delete(&db.SunsetQueueItem{})
	if result.Error != nil {
		return 0, result.Error
	}
	return int(result.RowsAffected), nil
}

// RemoveCompleted hard-deletes a sunset queue item after the file has been
// successfully deleted by DeletionService. No label removal or poster restore
// is needed — processExpiredItem already handled those before handoff.
func (s *SunsetService) RemoveCompleted(id uint) error {
	return s.db.Delete(&db.SunsetQueueItem{}, id).Error
}

// IsSunsetted checks if a media item is already in the sunset queue.
func (s *SunsetService) IsSunsetted(mediaName, mediaType string, diskGroupID uint) bool {
	var count int64
	s.db.Model(&db.SunsetQueueItem{}).
		Where("media_name = ? AND media_type = ? AND disk_group_id = ?", mediaName, mediaType, diskGroupID).
		Count(&count)
	return count > 0
}

// ListSunsettedKeys returns "mediaName|mediaType" keys for O(1) lookups.
// Same pattern as ApprovalService.ListSnoozedKeys().
func (s *SunsetService) ListSunsettedKeys(diskGroupID uint) (map[string]bool, error) {
	var items []db.SunsetQueueItem
	err := s.db.Select("media_name, media_type").
		Where("disk_group_id = ?", diskGroupID).Find(&items).Error
	if err != nil {
		return nil, err
	}

	keys := make(map[string]bool, len(items))
	for _, item := range items {
		keys[item.MediaName+"|"+item.MediaType] = true
	}
	return keys, nil
}

// RefreshLabels re-applies the sunset label to all pending queue items that
// don't have labels yet (label_applied=false). Called via the API when label
// application previously failed (e.g., due to missing TMDb→NativeID mappings).
func (s *SunsetService) RefreshLabels(deps SunsetDeps) (int, error) {
	var items []db.SunsetQueueItem
	if err := s.db.Where("label_applied = ? AND status = ?", false, db.SunsetStatusPending).Find(&items).Error; err != nil {
		return 0, fmt.Errorf("list unlabeled items: %w", err)
	}
	if len(items) == 0 {
		return 0, nil
	}

	prefs, prefsErr := deps.Settings.GetPreferences()
	if prefsErr != nil {
		return 0, fmt.Errorf("load preferences for label refresh: %w", prefsErr)
	}

	applied := 0
	for i := range items {
		if s.applyLabel(items[i], prefs.SunsetLabel, deps.Registry, deps.Mapping) {
			items[i].LabelApplied = true
			s.db.Model(&items[i]).Update("label_applied", true)
			applied++
		}
	}

	if applied > 0 {
		slog.Info("Refreshed sunset labels", "component", "services",
			"applied", applied, "total", len(items))
	}
	return applied, nil
}

// MigrateLabel removes the old label and applies the new label across all
// queued items in all enabled media servers. Called from the settings save
// handler when the user changes SunsetLabel.
func (s *SunsetService) MigrateLabel(oldLabel, newLabel string, registry *integrations.IntegrationRegistry, mapping *MappingService) error {
	var items []db.SunsetQueueItem
	if err := s.db.Where("label_applied = ?", true).Find(&items).Error; err != nil {
		return fmt.Errorf("list labeled items: %w", err)
	}

	for _, item := range items {
		s.removeLabel(item, oldLabel, registry, mapping)
		s.applyLabel(item, newLabel, registry, mapping)
	}

	slog.Info("Migrated sunset labels", "component", "services",
		"oldLabel", oldLabel, "newLabel", newLabel, "items", len(items))
	return nil
}

// ─── Internal helpers ───────────────────────────────────────────────────────

// applyLabel applies the sunset label to an item across all LabelManager-capable
// media servers. Returns true if at least one media server succeeded.
// Errors are logged but not propagated (label is best-effort).
func (s *SunsetService) applyLabel(item db.SunsetQueueItem, label string, registry *integrations.IntegrationRegistry, mapping *MappingService) bool {
	if registry == nil || item.TmdbID == nil || mapping == nil {
		return false
	}
	anySuccess := false
	searchers := registry.NativeIDSearchers()
	for integrationID, mgr := range registry.LabelManagers() {
		nativeID, err := mapping.Resolve(*item.TmdbID, integrationID)
		if err != nil {
			continue
		}
		if err := mgr.AddLabel(nativeID, label); err != nil {
			// Layer 1: 404 recovery — native ID may be stale
			if integrations.IsNotFoundError(err) {
				if searcher, ok := searchers[integrationID]; ok {
					if newID, reErr := mapping.InvalidateAndResolve(*item.TmdbID, integrationID, item.MediaName, searcher); reErr == nil {
						if mgr.AddLabel(newID, label) == nil {
							anySuccess = true
							s.bus.Publish(events.SunsetLabelAppliedEvent{
								MediaName: item.MediaName, IntegrationID: integrationID, Label: label,
							})
							continue
						}
					}
				}
			}
			slog.Error("Failed to apply sunset label",
				"component", "services", "mediaName", item.MediaName,
				"integrationID", integrationID, "error", err)
			s.bus.Publish(events.SunsetLabelFailedEvent{
				MediaName: item.MediaName, IntegrationID: integrationID,
				Label: label, Error: err.Error(),
			})
			continue
		}
		anySuccess = true
		s.bus.Publish(events.SunsetLabelAppliedEvent{
			MediaName: item.MediaName, IntegrationID: integrationID, Label: label,
		})
	}
	return anySuccess
}

// removeLabel removes the sunset label from an item across all LabelManager-capable
// media servers.
func (s *SunsetService) removeLabel(item db.SunsetQueueItem, label string, registry *integrations.IntegrationRegistry, mapping *MappingService) {
	if registry == nil || item.TmdbID == nil || mapping == nil {
		return
	}
	searchers := registry.NativeIDSearchers()
	for integrationID, mgr := range registry.LabelManagers() {
		nativeID, err := mapping.Resolve(*item.TmdbID, integrationID)
		if err != nil {
			continue
		}
		if err := mgr.RemoveLabel(nativeID, label); err != nil {
			// Layer 1: 404 recovery — native ID may be stale
			if integrations.IsNotFoundError(err) {
				if searcher, ok := searchers[integrationID]; ok {
					if newID, reErr := mapping.InvalidateAndResolve(*item.TmdbID, integrationID, item.MediaName, searcher); reErr == nil {
						if mgr.RemoveLabel(newID, label) == nil {
							s.bus.Publish(events.SunsetLabelRemovedEvent{
								MediaName: item.MediaName, IntegrationID: integrationID, Label: label,
							})
							continue
						}
					}
				}
			}
			slog.Error("Failed to remove sunset label",
				"component", "services", "mediaName", item.MediaName,
				"integrationID", integrationID, "error", err)
			s.bus.Publish(events.SunsetLabelFailedEvent{
				MediaName: item.MediaName, IntegrationID: integrationID,
				Label: label, Error: err.Error(),
			})
			continue
		}
		s.bus.Publish(events.SunsetLabelRemovedEvent{
			MediaName: item.MediaName, IntegrationID: integrationID, Label: label,
		})
	}
}

// processExpiredItem handles a single expired/escalated item: restores poster,
// removes label, queues for deletion, and marks as expired. The item is NOT
// deleted from sunset_queue — it remains visible in the dashboard until the
// user removes it via Cancel or Clear All. The ExpiredAt timestamp prevents
// re-processing on subsequent engine cycles and cron runs.
//
// If deletion handoff fails (no registry or deleter unavailable), the item is
// NOT marked expired — it will be retried on the next cron run.
func (s *SunsetService) processExpiredItem(item db.SunsetQueueItem, prefs db.PreferenceSet, deps SunsetDeps) bool {
	// Skip if already expired or saved
	if item.ExpiredAt != nil || item.Status == db.SunsetStatusSaved {
		return false
	}

	// Restore poster overlay before deletion
	if item.PosterOverlayActive && deps.PosterOverlay != nil && deps.Registry != nil {
		if err := deps.PosterOverlay.RestoreOriginal(item, PosterDeps{Registry: deps.Registry, Mapping: deps.Mapping}); err != nil {
			slog.Error("Failed to restore poster before expiry/escalation",
				"component", "services", "mediaName", item.MediaName, "error", err)
		}
	}

	// Remove label
	if item.LabelApplied && deps.Registry != nil && prefs.SunsetLabel != "" {
		s.removeLabel(item, prefs.SunsetLabel, deps.Registry, deps.Mapping)
	}

	// Hand off to DeletionService intake layer — it handles client resolution,
	// config lookup, and factor parsing internally.
	if deps.Deletion == nil {
		slog.Warn("Skipping sunset item expiry — deletion service unavailable (will retry)",
			"component", "services", "mediaName", item.MediaName)
		return false
	}

	if queueErr := deps.Deletion.QueueFromSunset(&item); queueErr != nil {
		slog.Error("Failed to queue sunset item for deletion (will retry next cycle)",
			"component", "services", "mediaName", item.MediaName, "error", queueErr)
		s.bus.Publish(events.EngineErrorEvent{
			Error: fmt.Sprintf("sunset expiry queue failed for %q: %v", item.MediaName, queueErr),
		})
		return false
	}

	s.bus.Publish(events.SunsetExpiredEvent{
		MediaName:   item.MediaName,
		MediaType:   item.MediaType,
		DiskGroupID: item.DiskGroupID,
		SizeBytes:   item.SizeBytes,
	})

	// Mark as expired — item stays in sunset_queue for dashboard visibility
	now := time.Now().UTC()
	s.db.Model(&item).Update("expired_at", now)
	return true
}

// ValidateSunsetConfig validates sunset-mode configuration on a disk group.
// Returns an error if the configuration is invalid:
//   - mode is "sunset" but sunsetPct is nil → must be explicitly configured
//   - mode is "sunset" and sunsetPct >= targetPct → ordering violated
//   - mode is "sunset" and sunsetPct >= thresholdPct → ordering violated
//
// This is called from the disk group update path to reject invalid configs
// at save-time rather than silently failing at engine evaluation time.
func ValidateSunsetConfig(mode string, sunsetPct *float64, targetPct, thresholdPct float64) error {
	if mode != db.ModeSunset {
		return nil
	}
	if sunsetPct == nil {
		return fmt.Errorf("sunset mode requires a sunset threshold to be configured")
	}
	if *sunsetPct >= targetPct {
		return fmt.Errorf("sunset threshold (%.1f%%) must be less than target threshold (%.1f%%)", *sunsetPct, targetPct)
	}
	if *sunsetPct >= thresholdPct {
		return fmt.Errorf("sunset threshold (%.1f%%) must be less than critical threshold (%.1f%%)", *sunsetPct, thresholdPct)
	}
	return nil
}

// publishEscalationEvent publishes the escalation summary.
func (s *SunsetService) publishEscalationEvent(diskGroupID uint, itemsExpired int, bytesFreed int64) {
	if itemsExpired > 0 {
		s.bus.Publish(events.SunsetEscalatedEvent{
			DiskGroupID:  diskGroupID,
			ItemsExpired: itemsExpired,
			BytesFreed:   bytesFreed,
		})
	}
}
