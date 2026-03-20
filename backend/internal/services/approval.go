// Package services contains business logic separated from HTTP handlers.
// Services are injectable and publish typed events to the event bus.
package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
)

// Sentinel errors for approval operations.
var (
	ErrApprovalNotFound       = errors.New("approval queue entry not found")
	ErrApprovalNotPending     = errors.New("entry is not in pending status")
	ErrApprovalNotDismissable = errors.New("entry is not in a dismissable state")
)

// ApprovalService manages the approval queue lifecycle.
type ApprovalService struct {
	db  *gorm.DB
	bus *events.EventBus
}

// NewApprovalService creates a new ApprovalService.
func NewApprovalService(database *gorm.DB, bus *events.EventBus) *ApprovalService {
	return &ApprovalService{db: database, bus: bus}
}

// Approve marks a pending item as approved and returns it.
func (s *ApprovalService) Approve(entryID uint) (*db.ApprovalQueueItem, error) {
	var entry db.ApprovalQueueItem
	if err := s.db.First(&entry, entryID).Error; err != nil {
		return nil, fmt.Errorf("%w: %v", ErrApprovalNotFound, err)
	}

	if entry.Status != db.StatusPending {
		return nil, fmt.Errorf("%w (current: %s)", ErrApprovalNotPending, entry.Status)
	}

	if err := s.db.Model(&entry).Updates(map[string]any{
		"status":     db.StatusApproved,
		"updated_at": time.Now().UTC(),
	}).Error; err != nil {
		return nil, fmt.Errorf("failed to approve entry: %w", err)
	}

	s.bus.Publish(events.ApprovalApprovedEvent{
		EntryID:   entry.ID,
		MediaName: entry.MediaName,
		MediaType: entry.MediaType,
		SizeBytes: entry.SizeBytes,
	})

	s.db.First(&entry, entryID) // Reload
	return &entry, nil
}

// Reject marks a pending item as rejected and sets a snooze duration.
func (s *ApprovalService) Reject(entryID uint, snoozeDurationHours int) (*db.ApprovalQueueItem, error) {
	var entry db.ApprovalQueueItem
	if err := s.db.First(&entry, entryID).Error; err != nil {
		return nil, fmt.Errorf("%w: %v", ErrApprovalNotFound, err)
	}

	if entry.Status != db.StatusPending {
		return nil, fmt.Errorf("%w (current: %s)", ErrApprovalNotPending, entry.Status)
	}

	snoozedUntil := time.Now().UTC().Add(time.Duration(snoozeDurationHours) * time.Hour)

	if err := s.db.Model(&entry).Updates(map[string]any{
		"status":        db.StatusRejected,
		"snoozed_until": snoozedUntil,
		"updated_at":    time.Now().UTC(),
	}).Error; err != nil {
		return nil, fmt.Errorf("failed to reject entry: %w", err)
	}

	s.bus.Publish(events.ApprovalRejectedEvent{
		EntryID:        entry.ID,
		MediaName:      entry.MediaName,
		MediaType:      entry.MediaType,
		SnoozeDuration: fmt.Sprintf("%dh", snoozeDurationHours),
	})

	s.db.First(&entry, entryID) // Reload
	return &entry, nil
}

// Unsnooze clears the snooze on a rejected item and resets it to pending.
func (s *ApprovalService) Unsnooze(entryID uint) (*db.ApprovalQueueItem, error) {
	var entry db.ApprovalQueueItem
	if err := s.db.First(&entry, entryID).Error; err != nil {
		return nil, fmt.Errorf("%w: %v", ErrApprovalNotFound, err)
	}

	if entry.Status != db.StatusRejected {
		return nil, fmt.Errorf("%w (current: %s)", ErrApprovalNotPending, entry.Status)
	}

	if err := s.db.Model(&entry).Updates(map[string]any{
		"status":        db.StatusPending,
		"snoozed_until": nil,
		"updated_at":    time.Now().UTC(),
	}).Error; err != nil {
		return nil, fmt.Errorf("failed to unsnooze entry: %w", err)
	}

	s.bus.Publish(events.ApprovalUnsnoozedEvent{
		EntryID:   entry.ID,
		MediaName: entry.MediaName,
		MediaType: entry.MediaType,
	})

	s.db.First(&entry, entryID) // Reload
	return &entry, nil
}

// Dismiss removes a single approval queue entry that is in a dismissable state
// (pending or rejected). Approved items cannot be dismissed because they have
// already been forwarded to the deletion pipeline.
func (s *ApprovalService) Dismiss(entryID uint) error {
	var entry db.ApprovalQueueItem
	if err := s.db.First(&entry, entryID).Error; err != nil {
		return fmt.Errorf("%w: %v", ErrApprovalNotFound, err)
	}

	if entry.Status != db.StatusPending && entry.Status != db.StatusRejected {
		return fmt.Errorf("%w (current: %s)", ErrApprovalNotDismissable, entry.Status)
	}

	if err := s.db.Delete(&entry).Error; err != nil {
		return fmt.Errorf("failed to dismiss entry: %w", err)
	}

	s.bus.Publish(events.ApprovalDismissedEvent{
		EntryID:   entry.ID,
		MediaName: entry.MediaName,
		MediaType: entry.MediaType,
	})

	return nil
}

// UpsertPending creates or updates a pending approval queue item.
// If a pending entry for the same media already exists, it is updated.
// Returns true if a new entry was created, false if updated.
func (s *ApprovalService) UpsertPending(item db.ApprovalQueueItem) (bool, error) {
	item.Status = db.StatusPending
	now := time.Now().UTC()

	var existing db.ApprovalQueueItem
	result := s.db.Where(
		"media_name = ? AND media_type = ? AND status = ?",
		item.MediaName, item.MediaType, db.StatusPending,
	).First(&existing)

	if result.Error == nil {
		// Update existing pending entry
		if err := s.db.Model(&existing).Updates(map[string]any{
			"reason":         item.Reason,
			"score_details":  item.ScoreDetails,
			"size_bytes":     item.SizeBytes,
			"score":          item.Score,
			"poster_url":     item.PosterURL,
			"integration_id": item.IntegrationID,
			"external_id":    item.ExternalID,
			"disk_group_id":  item.DiskGroupID,
			"updated_at":     now,
		}).Error; err != nil {
			return false, fmt.Errorf("failed to update pending entry: %w", err)
		}
		return false, nil
	}

	// Create new pending entry
	item.CreatedAt = now
	item.UpdatedAt = now
	if err := s.db.Create(&item).Error; err != nil {
		return false, fmt.Errorf("failed to create pending entry: %w", err)
	}
	return true, nil
}

// IsSnoozed checks if a media item is currently snoozed (rejected with an active snooze window).
// When diskGroupID is non-nil, the check is scoped to that specific disk group.
func (s *ApprovalService) IsSnoozed(mediaName, mediaType string, diskGroupID ...uint) bool {
	var count int64
	query := s.db.Model(&db.ApprovalQueueItem{}).Where(
		"media_name = ? AND media_type = ? AND status = ? AND snoozed_until IS NOT NULL AND snoozed_until > ?",
		mediaName, mediaType, db.StatusRejected, time.Now().UTC(),
	)
	if len(diskGroupID) > 0 {
		query = query.Where("disk_group_id = ?", diskGroupID[0])
	}
	query.Count(&count)
	return count > 0
}

// BulkUnsnooze clears all active snoozes and resets items to pending.
// When diskGroupID is non-nil, only items for that disk group are unsnoozed.
func (s *ApprovalService) BulkUnsnooze(diskGroupID *uint) (int, error) {
	query := s.db.Model(&db.ApprovalQueueItem{}).
		Where("status = ? AND snoozed_until IS NOT NULL", db.StatusRejected)

	if diskGroupID != nil {
		query = query.Where("disk_group_id = ?", *diskGroupID)
	}

	result := query.Updates(map[string]any{
		"status":        db.StatusPending,
		"snoozed_until": nil,
		"updated_at":    time.Now().UTC(),
	})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to bulk unsnooze: %w", result.Error)
	}

	count := int(result.RowsAffected)
	if count > 0 {
		s.bus.Publish(events.ApprovalBulkUnsnoozedEvent{Count: count})
		slog.Info("Bulk unsnoozed approval items", "component", "services", "count", count)
	}

	return count, nil
}

// CleanExpiredSnoozes clears stale snoozed_until values where the snooze has expired,
// and resets those items to pending.
func (s *ApprovalService) CleanExpiredSnoozes() (int, error) {
	now := time.Now().UTC()
	result := s.db.Model(&db.ApprovalQueueItem{}).
		Where("status = ? AND snoozed_until IS NOT NULL AND snoozed_until <= ?", db.StatusRejected, now).
		Updates(map[string]any{
			"status":        db.StatusPending,
			"snoozed_until": nil,
			"updated_at":    now,
		})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to clean expired snoozes: %w", result.Error)
	}

	count := int(result.RowsAffected)
	if count > 0 {
		slog.Info("Cleaned expired snoozes", "component", "services", "count", count)
	}

	return count, nil
}

// ClearQueue removes all pending and rejected items from the approval queue.
// Approved items (mid-deletion) and user-initiated items are preserved. This is
// called when disk usage drops below threshold to ensure the queue only contains
// current, actionable candidates.
func (s *ApprovalService) ClearQueue() (int, error) {
	result := s.db.Where(
		"status IN ? AND user_initiated = ?",
		[]string{string(db.StatusPending), string(db.StatusRejected)},
		false,
	).Delete(&db.ApprovalQueueItem{})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to clear approval queue: %w", result.Error)
	}

	count := int(result.RowsAffected)
	if count > 0 {
		s.bus.Publish(events.ApprovalQueueClearedEvent{Count: count})
		slog.Info("Approval queue cleared (disk below threshold)",
			"component", "services", "count", count)
	}

	return count, nil
}

// ClearQueueForDiskGroup removes pending and rejected items for a specific disk group.
// Like ClearQueue, approved items and user-initiated items are preserved.
func (s *ApprovalService) ClearQueueForDiskGroup(diskGroupID uint) (int, error) {
	result := s.db.Where(
		"status IN ? AND user_initiated = ? AND disk_group_id = ?",
		[]string{string(db.StatusPending), string(db.StatusRejected)},
		false,
		diskGroupID,
	).Delete(&db.ApprovalQueueItem{})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to clear approval queue for disk group %d: %w", diskGroupID, result.Error)
	}

	count := int(result.RowsAffected)
	if count > 0 {
		s.bus.Publish(events.ApprovalQueueClearedEvent{Count: count})
		slog.Info("Approval queue cleared for disk group (disk below threshold)",
			"component", "services", "diskGroupID", diskGroupID, "count", count)
	}

	return count, nil
}

// ListQueue returns approval queue items filtered by optional status and disk group,
// capped to limit. Pass diskGroupID=nil to list across all disk groups.
func (s *ApprovalService) ListQueue(status string, limit int, diskGroupID *uint) ([]db.ApprovalQueueItem, error) {
	items := make([]db.ApprovalQueueItem, 0, limit)
	query := s.db.Model(&db.ApprovalQueueItem{})

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if diskGroupID != nil {
		query = query.Where("disk_group_id = ?", *diskGroupID)
	}

	if err := query.Order("created_at desc").Limit(limit).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch approval queue: %w", err)
	}

	return items, nil
}

// ExecuteApproval encapsulates the full approval workflow:
// approve → look up integration → build client → reconstruct MediaItem →
// parse score details → queue for deletion.
// The caller must provide pre-validated DeletionService and IntegrationService references
// via the ExecuteApprovalDeps argument.
func (s *ApprovalService) ExecuteApproval(entryID uint, deps ExecuteApprovalDeps) (*db.ApprovalQueueItem, error) {
	// 1. Mark as approved via Approve (single fetch + status validation)
	approved, err := s.Approve(entryID)
	if err != nil {
		return nil, err
	}

	// 2. Look up the integration to construct a client for deletion
	integration, err := deps.Integration.GetByID(approved.IntegrationID)
	if err != nil {
		return approved, fmt.Errorf("integration not found for approval %d: %w", entryID, err)
	}

	// 3. Build the client via factory and extract MediaDeleter capability
	rawClient := integrations.CreateClient(integration.Type, integration.URL, integration.APIKey)
	if rawClient == nil {
		return approved, fmt.Errorf("unsupported integration type %q for approval %d", integration.Type, entryID)
	}
	client, ok := rawClient.(integrations.MediaDeleter)
	if !ok {
		return approved, fmt.Errorf("integration type %q does not support deletion for approval %d", integration.Type, entryID)
	}

	// 4. Reconstruct the MediaItem from stored approval data
	item := integrations.MediaItem{
		ExternalID:    approved.ExternalID,
		IntegrationID: approved.IntegrationID,
		Type:          integrations.MediaType(approved.MediaType),
		Title:         approved.MediaName,
		SizeBytes:     approved.SizeBytes,
	}

	// 5. Parse stored score details back into factors
	var factors []engine.ScoreFactor
	if approved.ScoreDetails != "" {
		if jsonErr := json.Unmarshal([]byte(approved.ScoreDetails), &factors); jsonErr != nil {
			slog.Warn("Failed to parse score details for approval", "id", approved.ID, "error", jsonErr)
		}
	}

	// 6. Attribute this deletion to the most recent engine run stats row so the
	// dashboard sparkline "deleted" counter reflects approval-mode deletions.
	var runStatsID uint
	if deps.Engine != nil {
		runStatsID = deps.Engine.LatestRunStatsID()
	}

	// 7. Queue for background deletion
	if queueErr := deps.Deletion.QueueDeletion(DeleteJob{
		Client:          client,
		Item:            item,
		Reason:          approved.Reason,
		Score:           approved.Score,
		Factors:         factors,
		RunStatsID:      runStatsID,
		ForceDryRun:     deps.ForceDryRun,
		ApprovalEntryID: approved.ID,
	}); queueErr != nil {
		return approved, fmt.Errorf("deletion queue is full: %w", queueErr)
	}

	return approved, nil
}

// ExecuteApprovalDeps holds the service dependencies needed by ExecuteApproval.
// This avoids circular references — ApprovalService doesn't need to import
// the full Registry.
type ExecuteApprovalDeps struct {
	Integration *IntegrationService
	Deletion    *DeletionService
	Engine      *EngineService
	ForceDryRun bool // When true, the queued DeleteJob will simulate deletion
}

// ManualDeleteItem contains the data needed for a user-initiated deletion.
type ManualDeleteItem struct {
	MediaName     string
	MediaType     string
	IntegrationID uint
	ExternalID    string
	SizeBytes     int64
	ScoreDetails  string
	PosterURL     string
	Score         float64
}

// ManualDeleteDeps holds the service dependencies needed by ManualDelete.
type ManualDeleteDeps struct {
	Integration *IntegrationService
	Deletion    *DeletionService
	Engine      *EngineService
}

// ManualDeleteResult contains the outcome of a ManualDelete call.
type ManualDeleteResult struct {
	Queued int    `json:"queued"`
	Total  int    `json:"total"`
	Mode   string `json:"mode"`
}

// ManualDelete encapsulates mode-aware deletion for user-initiated actions.
// In auto/dry-run mode, items are queued to the DeletionService immediately.
// In approval mode, items are upserted as pending approval queue entries with
// UserInitiated=true.
func (s *ApprovalService) ManualDelete(items []ManualDeleteItem, mode string, deletionsEnabled bool, deps ManualDeleteDeps) (ManualDeleteResult, error) {
	var queued int

	// Get the latest run stats ID for attribution
	var runStatsID uint
	if deps.Engine != nil {
		runStatsID = deps.Engine.LatestRunStatsID()
	}

	forceDryRun := mode == db.ModeDryRun || !deletionsEnabled

	for _, item := range items {
		if mode == db.ModeApproval {
			// In approval mode, upsert as pending with UserInitiated=true
			if _, err := s.UpsertPending(db.ApprovalQueueItem{
				MediaName:     item.MediaName,
				MediaType:     item.MediaType,
				Reason:        fmt.Sprintf("Score: %.2f (user-initiated)", item.Score),
				ScoreDetails:  item.ScoreDetails,
				SizeBytes:     item.SizeBytes,
				Score:         item.Score,
				PosterURL:     item.PosterURL,
				IntegrationID: item.IntegrationID,
				ExternalID:    item.ExternalID,
				UserInitiated: true,
			}); err != nil {
				slog.Error("Failed to upsert manual delete as pending", "component", "services",
					"media", item.MediaName, "error", err)
				continue
			}
			queued++
			continue
		}

		// Auto / dry-run mode: build integration client and queue for deletion
		integration, err := deps.Integration.GetByID(item.IntegrationID)
		if err != nil {
			slog.Error("Integration not found for manual delete", "component", "services",
				"integrationId", item.IntegrationID, "media", item.MediaName, "error", err)
			continue
		}

		rawClient := integrations.CreateClient(integration.Type, integration.URL, integration.APIKey)
		if rawClient == nil {
			slog.Error("Unsupported integration type for manual delete", "component", "services",
				"type", integration.Type, "media", item.MediaName)
			continue
		}
		client, ok := rawClient.(integrations.MediaDeleter)
		if !ok {
			slog.Error("Integration does not support deletion for manual delete", "component", "services",
				"type", integration.Type, "media", item.MediaName)
			continue
		}

		// Parse score details into factors
		var factors []engine.ScoreFactor
		if item.ScoreDetails != "" {
			if jsonErr := json.Unmarshal([]byte(item.ScoreDetails), &factors); jsonErr != nil {
				slog.Warn("Failed to parse score details for manual delete", "component", "services",
					"media", item.MediaName, "error", jsonErr)
			}
		}

		mediaItem := integrations.MediaItem{
			ExternalID:    item.ExternalID,
			IntegrationID: item.IntegrationID,
			Type:          integrations.MediaType(item.MediaType),
			Title:         item.MediaName,
			SizeBytes:     item.SizeBytes,
		}

		if queueErr := deps.Deletion.QueueDeletion(DeleteJob{
			Client:      client,
			Item:        mediaItem,
			Reason:      fmt.Sprintf("Score: %.2f (user-initiated)", item.Score),
			Score:       item.Score,
			Factors:     factors,
			RunStatsID:  runStatsID,
			ForceDryRun: forceDryRun,
		}); queueErr != nil {
			slog.Warn("Deletion queue full for manual delete", "component", "services",
				"media", item.MediaName, "error", queueErr)
			continue
		}
		queued++
	}

	return ManualDeleteResult{
		Queued: queued,
		Total:  len(items),
		Mode:   mode,
	}, nil
}

// ListPendingForDiskGroup returns all pending (non-user-initiated) items for a
// specific disk group. Used by the engine's per-cycle reconciliation to identify
// stale items that are no longer needed after threshold changes.
// User-initiated items are excluded because they should not be pruned by
// reconciliation — they were explicitly requested by a user.
func (s *ApprovalService) ListPendingForDiskGroup(diskGroupID uint) ([]db.ApprovalQueueItem, error) {
	var items []db.ApprovalQueueItem
	if err := s.db.Where(
		"status = ? AND user_initiated = ? AND disk_group_id = ?",
		db.StatusPending, false, diskGroupID,
	).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("failed to list pending items for disk group %d: %w", diskGroupID, err)
	}
	return items, nil
}

// ReconcileQueue removes pending items for a disk group that are no longer in
// the provided "still-needed" set. Items whose MediaName+MediaType key is NOT
// in the neededKeys set are dismissed. Returns the number of items dismissed.
//
// This is called after each engine evaluation cycle to trim stale entries that
// no longer qualify (e.g., threshold was raised, scores changed).
// Rejected/snoozed items are left untouched — only status=pending items are
// eligible for reconciliation.
func (s *ApprovalService) ReconcileQueue(diskGroupID uint, neededKeys map[string]bool) (int, error) {
	pending, err := s.ListPendingForDiskGroup(diskGroupID)
	if err != nil {
		return 0, err
	}

	var dismissed int
	for _, item := range pending {
		key := item.MediaName + "|" + item.MediaType
		if neededKeys[key] {
			continue
		}

		if delErr := s.db.Delete(&item).Error; delErr != nil {
			slog.Error("Failed to dismiss stale approval item during reconciliation",
				"component", "services", "id", item.ID, "media", item.MediaName, "error", delErr)
			continue
		}
		dismissed++
	}

	if dismissed > 0 {
		s.bus.Publish(events.ApprovalQueueReconciledEvent{
			DiskGroupID: diskGroupID,
			Dismissed:   dismissed,
		})
		slog.Info("Approval queue reconciled — stale items dismissed",
			"component", "services", "diskGroupID", diskGroupID, "dismissed", dismissed)
	}

	return dismissed, nil
}

// RecoverOrphans finds approved items that have no corresponding active deletion
// and requeues them as pending. This handles recovery after crashes or restarts.
func (s *ApprovalService) RecoverOrphans() (int, error) {
	result := s.db.Model(&db.ApprovalQueueItem{}).
		Where("status = ?", db.StatusApproved).
		Updates(map[string]any{
			"status":     db.StatusPending,
			"updated_at": time.Now().UTC(),
		})

	if result.Error != nil {
		return 0, fmt.Errorf("failed to recover orphaned approvals: %w", result.Error)
	}

	count := int(result.RowsAffected)
	if count > 0 {
		s.bus.Publish(events.ApprovalOrphansRecoveredEvent{Count: count})
		slog.Info("Recovered orphaned approval items", "component", "services", "count", count)
	}

	return count, nil
}

// ReturnToPending resets an approved approval queue item back to pending status.
// This is called by the DeletionService when a dry-deleted item originated from
// the approval queue, creating the intentional loop: approve → dry-delete →
// return to pending. The user can then approve again when deletions are enabled.
func (s *ApprovalService) ReturnToPending(entryID uint) error {
	var entry db.ApprovalQueueItem
	if err := s.db.First(&entry, entryID).Error; err != nil {
		return fmt.Errorf("%w: %v", ErrApprovalNotFound, err)
	}

	if entry.Status != db.StatusApproved {
		return fmt.Errorf("cannot return to pending: entry %d is in %q status, expected %q", entryID, entry.Status, db.StatusApproved)
	}

	if err := s.db.Model(&entry).Updates(map[string]any{
		"status":     db.StatusPending,
		"updated_at": time.Now().UTC(),
	}).Error; err != nil {
		return fmt.Errorf("failed to return entry %d to pending: %w", entryID, err)
	}

	s.bus.Publish(events.ApprovalReturnedToPendingEvent{
		EntryID:   entry.ID,
		MediaName: entry.MediaName,
		MediaType: entry.MediaType,
	})

	slog.Info("Returned dry-deleted approval item to pending",
		"component", "services", "entryID", entryID, "media", entry.MediaName)

	return nil
}

// CreateSnoozedEntry creates or updates an approval queue entry with
// status=rejected and snoozed_until set to now + snoozeDurationHours.
// If an entry for the same media already exists (any status), it is updated
// to rejected with the new snooze time. Returns the snooze expiry time.
func (s *ApprovalService) CreateSnoozedEntry(mediaName, mediaType string, integrationID uint, snoozeDurationHours int) (*time.Time, error) {
	snoozedUntil := time.Now().UTC().Add(time.Duration(snoozeDurationHours) * time.Hour)

	var existing db.ApprovalQueueItem
	err := s.db.Where("media_name = ? AND media_type = ?", mediaName, mediaType).First(&existing).Error
	if err == nil {
		// Entry exists — update to snoozed state
		if err := s.db.Model(&existing).Updates(map[string]any{
			"status":        db.StatusRejected,
			"snoozed_until": snoozedUntil,
			"updated_at":    time.Now().UTC(),
		}).Error; err != nil {
			return nil, fmt.Errorf("failed to update snoozed entry: %w", err)
		}
		return &snoozedUntil, nil
	}

	// Create new snoozed entry
	entry := db.ApprovalQueueItem{
		MediaName:     mediaName,
		MediaType:     mediaType,
		IntegrationID: integrationID,
		Reason:        "Snoozed from deletion queue",
		Status:        db.StatusRejected,
		SnoozedUntil:  &snoozedUntil,
	}
	if err := s.db.Create(&entry).Error; err != nil {
		return nil, fmt.Errorf("failed to create snoozed entry: %w", err)
	}

	slog.Info("Created snoozed approval entry", "component", "services",
		"media", mediaName, "type", mediaType, "snoozedUntil", snoozedUntil)
	return &snoozedUntil, nil
}
