// Package services contains business logic separated from HTTP handlers.
// Services are injectable and publish typed events to the event bus.
package services

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
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
		return nil, fmt.Errorf("approval queue entry not found: %w", err)
	}

	if entry.Status != db.StatusPending {
		return nil, fmt.Errorf("entry is not pending (current status: %s)", entry.Status)
	}

	if err := s.db.Model(&entry).Updates(map[string]interface{}{
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
		return nil, fmt.Errorf("approval queue entry not found: %w", err)
	}

	if entry.Status != db.StatusPending {
		return nil, fmt.Errorf("entry is not pending (current status: %s)", entry.Status)
	}

	snoozedUntil := time.Now().UTC().Add(time.Duration(snoozeDurationHours) * time.Hour)

	if err := s.db.Model(&entry).Updates(map[string]interface{}{
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
		return nil, fmt.Errorf("approval queue entry not found: %w", err)
	}

	if entry.Status != db.StatusRejected {
		return nil, fmt.Errorf("entry is not rejected (current status: %s)", entry.Status)
	}

	if err := s.db.Model(&entry).Updates(map[string]interface{}{
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
		if err := s.db.Model(&existing).Updates(map[string]interface{}{
			"reason":         item.Reason,
			"score_details":  item.ScoreDetails,
			"size_bytes":     item.SizeBytes,
			"poster_url":     item.PosterURL,
			"integration_id": item.IntegrationID,
			"external_id":    item.ExternalID,
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
func (s *ApprovalService) IsSnoozed(mediaName, mediaType string) bool {
	var count int64
	s.db.Model(&db.ApprovalQueueItem{}).Where(
		"media_name = ? AND media_type = ? AND status = ? AND snoozed_until IS NOT NULL AND snoozed_until > ?",
		mediaName, mediaType, db.StatusRejected, time.Now().UTC(),
	).Count(&count)
	return count > 0
}

// BulkUnsnooze clears all active snoozes and resets items to pending.
// This is called when disk usage drops below threshold.
func (s *ApprovalService) BulkUnsnooze() (int, error) {
	result := s.db.Model(&db.ApprovalQueueItem{}).
		Where("status = ? AND snoozed_until IS NOT NULL", db.StatusRejected).
		Updates(map[string]interface{}{
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
		Updates(map[string]interface{}{
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

// RecoverOrphans finds approved items that have no corresponding active deletion
// and requeues them as pending. This handles recovery after crashes or restarts.
func (s *ApprovalService) RecoverOrphans() (int, error) {
	result := s.db.Model(&db.ApprovalQueueItem{}).
		Where("status = ?", db.StatusApproved).
		Updates(map[string]interface{}{
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
