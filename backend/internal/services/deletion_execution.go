package services

import (
	"fmt"
	"log/slog"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
)

// executeDryRun handles the dry-delete path: logs the action but does not
// actually remove the file from the media server.
func (s *DeletionService) executeDryRun(job deleteJob, factorsJSON []byte, deletionsEnabled bool, deferredAuditEntries *[]db.AuditLogEntry) {
	s.processed.Add(1)
	s.batchSucceeded.Add(1)

	logEntry := db.AuditLogEntry{
		MediaName:       job.Item.Title,
		MediaType:       string(job.Item.Type),
		ScoreDetails:    string(factorsJSON),
		Action:          db.ActionDryDelete,
		SizeBytes:       job.Item.SizeBytes,
		Score:           job.Score,
		Trigger:         job.Trigger,
		DryRunReason:    determineDryRunReason(deletionsEnabled, job.ForceDryRun),
		DiskGroupID:     job.DiskGroupID,
		CollectionGroup: job.CollectionGroup,
	}
	if job.UpsertAudit && deferredAuditEntries != nil {
		*deferredAuditEntries = append(*deferredAuditEntries, logEntry)
	} else if job.UpsertAudit {
		if err := s.auditLog.UpsertDryRun(logEntry); err != nil {
			slog.Error("Failed to upsert audit log entry", "component", "services", "error", err)
		}
	} else if err := s.auditLog.Create(logEntry); err != nil {
		slog.Error("Failed to create audit log entry", "component", "services", "error", err)
	}

	s.bus.Publish(events.DeletionDryRunEvent{
		MediaName: job.Item.Title,
		MediaType: string(job.Item.Type),
		SizeBytes: job.Item.SizeBytes,
	})
	s.publishProgress()

	// Return approval queue items to pending after dry-delete so the user
	// can approve again when deletions are actually enabled.
	if job.ApprovalEntryID != 0 && s.approvalReturner != nil {
		if err := s.approvalReturner.ReturnToPending(job.ApprovalEntryID); err != nil {
			slog.Error("Failed to return dry-deleted item to approval queue",
				"component", "services", "entryID", job.ApprovalEntryID, "error", err)
		}
	}

	slog.Info("Dry-Delete completed", "component", "services",
		"media", job.Item.Title, "action", "Dry-Delete", "freed", job.Item.SizeBytes)
}

// executeDeletion performs the actual media file deletion via the integration
// client, updates stats, logs the audit entry, and cleans up related queues.
func (s *DeletionService) executeDeletion(job deleteJob, factorsJSON []byte) {
	// Nil-safety check for dry-run jobs that have no client
	if job.Client == nil {
		slog.Error("Deletion job has nil client — cannot perform actual deletion",
			"component", "services", "media", job.Item.Title,
			"enqueuedMode", job.EnqueuedMode, "forceDryRun", job.ForceDryRun)
		s.failed.Add(1)
		s.batchFailed.Add(1)
		s.publishProgress()
		return
	}
	if err := job.Client.DeleteMediaItem(job.Item, integrations.DeleteOptions{
		AddImportExclusion: job.AddImportExclusion,
	}); err != nil {
		slog.Error("Deletion failed", "component", "services", "item", job.Item.Title, "error", err)
		s.failed.Add(1)
		s.batchFailed.Add(1)

		s.bus.Publish(events.DeletionFailedEvent{
			MediaName:     job.Item.Title,
			MediaType:     string(job.Item.Type),
			IntegrationID: job.Item.IntegrationID,
			Error:         err.Error(),
		})
		s.publishProgress()
		return
	}

	s.processed.Add(1)
	s.batchSucceeded.Add(1)

	// Increment deleted counter and freed bytes on the engine run stats row
	if err := s.engine.IncrementDeletedStats(job.RunStatsID, job.Item.SizeBytes); err != nil {
		slog.Error("Failed to increment engine deleted stats", "component", "services", "error", err)
	}

	// Increment lifetime stats
	if err := s.metrics.IncrementDeletionStats(job.Item.SizeBytes); err != nil {
		slog.Error("Failed to increment lifetime deletion stats", "component", "services", "error", err)
	}

	logEntry := db.AuditLogEntry{
		MediaName:       job.Item.Title,
		MediaType:       string(job.Item.Type),
		ScoreDetails:    string(factorsJSON),
		Action:          db.ActionDeleted,
		SizeBytes:       job.Item.SizeBytes,
		Score:           job.Score,
		Trigger:         job.Trigger,
		DiskGroupID:     job.DiskGroupID,
		CollectionGroup: job.CollectionGroup,
	}
	if err := s.auditLog.Create(logEntry); err != nil {
		slog.Error("Failed to create audit log entry", "component", "services", "error", err)
	}

	s.bus.Publish(events.DeletionSuccessEvent{
		MediaName:       job.Item.Title,
		MediaType:       string(job.Item.Type),
		SizeBytes:       job.Item.SizeBytes,
		IntegrationID:   job.Item.IntegrationID,
		CollectionGroup: job.CollectionGroup,
	})
	s.publishProgress()

	s.postDeletion(job)

	slog.Info("Deletion completed", "component", "services",
		"media", job.Item.Title, "action", "Deleted", "freed", job.Item.SizeBytes)
}

// postDeletion cleans up approval and sunset queue entries after a successful
// actual deletion.
func (s *DeletionService) postDeletion(job deleteJob) {
	// Clean up the approval queue entry after successful actual deletion.
	if job.ApprovalEntryID != 0 && s.approvalReturner != nil {
		if err := s.approvalReturner.RemoveEntry(job.ApprovalEntryID); err != nil {
			slog.Error("Failed to clean up approval entry after deletion",
				"component", "services", "entryID", job.ApprovalEntryID, "error", err)
		}
	}

	// Clean up the sunset queue entry after successful actual deletion.
	if job.SunsetQueueItemID != 0 && s.sunsetCleaner != nil {
		if err := s.sunsetCleaner.RemoveCompleted(job.SunsetQueueItemID); err != nil {
			slog.Error("Failed to clean up sunset queue entry after deletion",
				"component", "services", "sunsetItemID", job.SunsetQueueItemID, "error", err)
		}
	}
}

// resolveCurrentMode returns the current execution mode for a deletion job.
// When the job has a DiskGroupID, it looks up the per-group mode. Falls back
// to DefaultDiskGroupMode from preferences if the group lookup fails or the
// job has no group. Returns "" if the mode could not be determined.
func (s *DeletionService) resolveCurrentMode(diskGroupID *uint) string {
	if diskGroupID != nil && s.diskGroups != nil {
		group, err := s.diskGroups.GetByID(*diskGroupID)
		if err != nil {
			// Group lookup failed (e.g., group deleted while jobs were queued).
			// Return "" so the caller's "currentMode != ''" guard skips the
			// comparison rather than using a potentially wrong fallback.
			return ""
		}
		return group.Mode
	}
	// Fallback for jobs without a disk group (shouldn't happen for engine jobs)
	if prefs, err := s.settings.GetPreferences(); err == nil {
		return prefs.DefaultDiskGroupMode
	}
	return ""
}

// determineDryRunReason returns the structured reason for a dry-run.
// Returns "deletions_disabled" if deletions are globally disabled,
// "execution_mode" if the job was forced to dry-run by the execution mode,
// or "" if the job is not a dry-run.
func determineDryRunReason(deletionsEnabled, forceDryRun bool) string {
	if !deletionsEnabled {
		return db.DryRunReasonDeletionsDisabled
	}
	if forceDryRun {
		return db.DryRunReasonExecutionMode
	}
	return db.DryRunReasonNone
}

// SnoozeDeletionItem encapsulates the multi-step snooze workflow: look up the
// queued item for its integration ID, cancel the deletion, read the snooze
// duration from preferences, and create a snoozed entry in the approval queue.
// Returns the snoozedUntil time on success.
func (s *DeletionService) SnoozeDeletionItem(mediaName, mediaType string) (*time.Time, error) {
	// Look up the item in the queue to get integration ID and disk group
	queuedItem := s.FindQueuedItem(mediaName, mediaType)
	var integrationID uint
	var diskGroupID *uint
	if queuedItem != nil {
		integrationID = queuedItem.IntegrationID
		diskGroupID = queuedItem.DiskGroupID
	}

	// Remove from deletion queue
	s.CancelDeletion(mediaName, mediaType)

	// Get snooze duration from preferences
	prefs, err := s.settings.GetPreferences()
	if err != nil {
		return nil, fmt.Errorf("failed to load preferences for snooze: %w", err)
	}

	// Create snoozed entry in approval queue
	snoozedUntil, err := s.approvalSnoozer.CreateSnoozedEntry(mediaName, mediaType, integrationID, diskGroupID, prefs.SnoozeDurationHours)
	if err != nil {
		return nil, fmt.Errorf("failed to create snoozed entry: %w", err)
	}

	return snoozedUntil, nil
}
