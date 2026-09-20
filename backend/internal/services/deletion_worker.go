package services

import (
	"encoding/json"
	"log/slog"
	"sort"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/events"
)

// SignalBatchSize tells the deletion service how many items were queued in this
// engine cycle. When all items are processed, DeletionBatchCompleteEvent is
// published. If count is 0 (no items to process), the event is published
// immediately — the DeletionService owns this event.
//
// The cancellation skip-list is NOT cleared here. Cancellations must survive
// until processJob honours them; a later engine cycle can otherwise wipe a
// skip-list entry for an item still sitting in the grace-period queue.
func (s *DeletionService) SignalBatchSize(count int) {
	if count == 0 {
		s.bus.Publish(events.DeletionBatchCompleteEvent{
			Succeeded: 0,
			Failed:    0,
		})
		return
	}
	s.batchExpected.Store(int64(count))
	s.batchProcessed.Store(0)
	s.batchSucceeded.Store(0)
	s.batchFailed.Store(0)
}

func (s *DeletionService) worker() {
	defer close(s.done)
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic recovered in deletion worker", "component", "services", "panic", r)
		}
	}()

	for {
		select {
		case <-s.stopCh:
			// Shutdown: drain any remaining items
			s.drainAll()
			return
		case <-s.notify:
			// Something happened — check if grace period has expired and we should drain
			if !s.graceActive.Load() && s.queueLen() > 0 {
				s.drainAll()
			}
		}
	}
}

// drainAll processes all items currently in the queue. Items added during
// draining are also processed (no new grace period until we've fully drained).
// Dry-run audit entries with UpsertAudit=true are collected and batch-flushed
// at the end to reduce per-item DB overhead.
func (s *DeletionService) drainAll() {
	s.processing.Store(true)
	defer s.processing.Store(false)

	// Stop the grace timer if it's still running (e.g., during shutdown)
	s.graceTimerMu.Lock()
	if s.graceTimer != nil {
		s.graceTimer.Stop()
		s.graceTimer = nil
	}
	s.graceTimerMu.Unlock()
	s.graceActive.Store(false)

	// For non-batch items (e.g., approval queue approvals), SignalBatchSize()
	// was never called, so batchExpected is 0. Initialize batch tracking from
	// the current queue size so publishProgress() reports meaningful percentages
	// and checkBatchComplete() fires when all items are processed.
	if s.batchExpected.Load() == 0 {
		qs := int64(s.queueLen())
		if qs > 0 {
			s.batchExpected.Store(qs)
			s.batchProcessed.Store(0)
			s.batchSucceeded.Store(0)
			s.batchFailed.Store(0)
		}
	}

	// Sort queued items by score descending so the highest-priority items are
	// processed first. This centralises deletion ordering in DeletionService
	// rather than relying on callers to enqueue in the correct order. Without
	// this, callers like Escalate() that order by time instead of score would
	// cause low-score items to be deleted before high-score items.
	s.queuedMu.Lock()
	sort.SliceStable(s.queuedItems, func(i, j int) bool {
		return s.queuedItems[i].Score > s.queuedItems[j].Score
	})
	s.queuedMu.Unlock()

	// Collect dry-run audit entries for batch flush after drain completes.
	var deferredAuditEntries []db.AuditLogEntry

drainLoop:
	for {
		job, ok := s.dequeueJob()
		if !ok {
			break
		}

		// Check for stop signal between jobs
		select {
		case <-s.stopCh:
			// Process this last job then break to flush
			s.processJob(job, &deferredAuditEntries)
			break drainLoop
		default:
		}

		// Early-exit: if execution mode changed since this job was enqueued,
		// cancel all remaining items immediately instead of processing them
		// one-by-one through the rate limiter. This avoids wasting ~3s per
		// item on jobs that processJob() would cancel anyway.
		if job.EnqueuedMode != "" {
			currentMode := s.resolveCurrentMode(job.DiskGroupID)
			if currentMode != "" && currentMode != job.EnqueuedMode {
				// Process this one job (processJob will cancel it via mode-change guard),
				// then cancel all remaining jobs in bulk without rate limiting.
				s.processJob(job, &deferredAuditEntries)
				s.cancelRemaining("mode_change", &deferredAuditEntries)
				break drainLoop
			}
		}

		// Early-exit: if this job is in the cancellation skip-list (user
		// clicked "clear all" while drain was active), process it without
		// rate limiting and then fast-drain all remaining items. This avoids
		// wasting ~3s per item on the rate limiter for 300+ items that will
		// all be immediately cancelled by processJob().
		if s.IsCancelled(job.Item.Title, string(job.Item.Type)) {
			s.processJob(job, &deferredAuditEntries)
			s.cancelRemaining("clear_all", &deferredAuditEntries)
			break drainLoop
		}

		if err := s.rateLimiter.Wait(s.stopCtx); err != nil {
			// Context cancelled during shutdown — process this final job then exit.
			s.processJob(job, &deferredAuditEntries)
			break drainLoop
		}
		s.processJob(job, &deferredAuditEntries)
	}

	// Batch-flush deferred dry-run audit entries
	if len(deferredAuditEntries) > 0 {
		if err := s.auditLog.BulkUpsertDryRun(deferredAuditEntries); err != nil {
			slog.Error("Failed to batch upsert dry-run audit entries", "component", "services",
				"count", len(deferredAuditEntries), "error", err)
		} else {
			slog.Info("Batch upserted dry-run audit entries", "component", "services",
				"count", len(deferredAuditEntries))
		}
	}
}

// cancelRemaining drains all remaining queued items and processes them
// without rate limiting. Each item's cancellation is handled by processJob()
// which checks the cancellation skip-list and mode-change guards.
// Called by drainAll() when it detects that remaining items should be
// cancelled immediately instead of trickling through the rate limiter
// (e.g., user clicked "clear all", or execution mode changed mid-drain).
func (s *DeletionService) cancelRemaining(reason string, deferredAuditEntries *[]db.AuditLogEntry) {
	var cancelled int
	for {
		job, ok := s.dequeueJob()
		if !ok {
			break
		}
		s.processJob(job, deferredAuditEntries)
		cancelled++
	}
	if cancelled > 0 {
		slog.Info("Cancelled remaining queued items",
			"component", "services", "reason", reason, "cancelled", cancelled)
	}
}

// processJob handles a single deletion job. When deferredAuditEntries is non-nil,
// dry-run entries with UpsertAudit=true are collected for batch flush instead of
// being written individually to the database.
func (s *DeletionService) processJob(job deleteJob, deferredAuditEntries *[]db.AuditLogEntry) {
	s.currentlyDeleting.Store(job.Item.Title)
	defer s.currentlyDeleting.Store("")
	defer s.clearInFlight(job)
	defer s.checkBatchComplete()

	// Marshal score factors early so all code paths (including cancellation)
	// can include the score breakdown in the audit log entry. The deleteJob
	// carries Score and Factors from the engine evaluation; preserving them
	// in the audit trail lets the history log show what score an item had
	// even when the deletion was cancelled or mode-changed.
	if job.Factors == nil {
		job.Factors = []engine.ScoreFactor{}
	}
	factorsJSON, marshalErr := json.Marshal(job.Factors)
	if marshalErr != nil {
		slog.Error("Failed to marshal score factors", "component", "services", "error", marshalErr)
		factorsJSON = []byte("[]")
	}

	// Check cancellation skip-list before doing any work.
	if s.IsCancelled(job.Item.Title, string(job.Item.Type)) {
		s.cancelled.Delete(cancelKey(job.Item.Title, string(job.Item.Type)))

		s.processed.Add(1)
		s.batchSucceeded.Add(1)

		logEntry := db.AuditLogEntry{
			MediaName:       job.Item.Title,
			MediaType:       string(job.Item.Type),
			ScoreDetails:    string(factorsJSON),
			Action:          db.ActionCancelled,
			SizeBytes:       job.Item.SizeBytes,
			Score:           job.Score,
			Trigger:         job.Trigger,
			DiskGroupID:     job.DiskGroupID,
			CollectionGroup: job.CollectionGroup,
		}
		if err := s.auditLog.Create(logEntry); err != nil {
			slog.Error("Failed to create audit log entry", "component", "services", "error", err)
		}

		s.bus.Publish(events.DeletionCancelledEvent{
			MediaName: job.Item.Title,
			MediaType: string(job.Item.Type),
			SizeBytes: job.Item.SizeBytes,
		})
		s.publishProgress()

		slog.Info("Deletion cancelled by user", "component", "services", "media", job.Item.Title)
		return
	}

	// Defense-in-depth: if the execution mode changed since this job was enqueued,
	// treat it as cancelled. This catches items that were dequeued between the
	// ClearQueue() call and the mode change, or race conditions where the worker
	// dequeues an item just before ClearQueue() marks it.
	// Uses the per-disk-group mode when DiskGroupID is available, falling back
	// to DefaultDiskGroupMode for jobs without a group (shouldn't happen, but
	// keeps the safety net).
	if job.EnqueuedMode != "" {
		currentMode := s.resolveCurrentMode(job.DiskGroupID)
		if currentMode != "" && currentMode != job.EnqueuedMode {
			s.processed.Add(1)
			s.batchSucceeded.Add(1)

			logEntry := db.AuditLogEntry{
				MediaName:       job.Item.Title,
				MediaType:       string(job.Item.Type),
				ScoreDetails:    string(factorsJSON),
				Action:          db.ActionCancelled,
				SizeBytes:       job.Item.SizeBytes,
				Score:           job.Score,
				Trigger:         job.Trigger,
				DiskGroupID:     job.DiskGroupID,
				CollectionGroup: job.CollectionGroup,
			}
			if err := s.auditLog.Create(logEntry); err != nil {
				slog.Error("Failed to create audit log entry", "component", "services", "error", err)
			}

			s.bus.Publish(events.DeletionCancelledEvent{
				MediaName: job.Item.Title,
				MediaType: string(job.Item.Type),
				SizeBytes: job.Item.SizeBytes,
			})
			s.publishProgress()

			slog.Info("Deletion cancelled — execution mode changed since enqueue",
				"component", "services",
				"media", job.Item.Title,
				"enqueuedMode", job.EnqueuedMode,
				"currentMode", currentMode)
			return
		}
	}

	// Re-read DeletionsEnabled at processing time (not enqueue time) as a safety net.
	deletionsEnabled := false
	if prefs, err := s.settings.GetPreferences(); err == nil {
		deletionsEnabled = prefs.DeletionsEnabled
	}

	if !deletionsEnabled || job.ForceDryRun {
		s.executeDryRun(job, factorsJSON, deletionsEnabled, deferredAuditEntries)
		return
	}

	s.executeDeletion(job, factorsJSON)
}

// publishProgress publishes a DeletionProgressEvent with the current batch
// progress counters. Called after each job completes (success, failure, or
// dry-run) to provide real-time progress data for the frontend.
func (s *DeletionService) publishProgress() {
	s.bus.Publish(events.DeletionProgressEvent{
		CurrentItem: s.CurrentlyDeleting(),
		QueueDepth:  s.queueLen(),
		Processed:   int(s.batchSucceeded.Load()) + int(s.batchFailed.Load()),
		Succeeded:   int(s.batchSucceeded.Load()),
		Failed:      int(s.batchFailed.Load()),
		BatchTotal:  int(s.batchExpected.Load()),
	})
}

// checkBatchComplete increments the batch processed counter and publishes
// DeletionBatchCompleteEvent when all expected items have been processed.
func (s *DeletionService) checkBatchComplete() {
	expected := s.batchExpected.Load()
	if expected <= 0 {
		return
	}

	processed := s.batchProcessed.Add(1)
	if processed >= expected {
		s.bus.Publish(events.DeletionBatchCompleteEvent{
			Succeeded: int(s.batchSucceeded.Load()),
			Failed:    int(s.batchFailed.Load()),
		})
		s.batchExpected.Store(0) // reset for next cycle
	}
}
