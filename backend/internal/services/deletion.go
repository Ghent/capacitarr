package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
)

// DeleteJob describes a media item to be deleted.
type DeleteJob struct {
	Client          integrations.MediaDeleter
	Item            integrations.MediaItem
	Score           float64
	Factors         []engine.ScoreFactor
	Trigger         string // "engine", "user", "approval"
	RunStatsID      uint   // Engine run stats row to increment Deleted counter
	DiskGroupID     *uint  // Disk group that triggered this deletion (nil for user-initiated deletes)
	ForceDryRun     bool   // When true, skip actual deletion even if DeletionsEnabled=true
	UpsertAudit     bool   // When true, use AuditLog.UpsertDryRun() (idempotent poller dry-runs); when false, use AuditLog.Create() (append-only)
	ApprovalEntryID uint   // Non-zero if this job originated from an approval queue item
	CollectionGroup string // Non-empty if this job is part of a collection deletion (e.g., "Sonic the Hedgehog Collection")
}

// DeleteJobSummary is a serialisable snapshot of a queued deletion job,
// suitable for API responses. It deliberately excludes the Integration
// client to avoid exposing internal state.
type DeleteJobSummary struct {
	MediaName     string  `json:"mediaName"`
	MediaType     string  `json:"mediaType"`
	SizeBytes     int64   `json:"sizeBytes"`
	IntegrationID uint    `json:"integrationId"`
	Score         float64 `json:"score"`
	PosterURL     string  `json:"posterUrl,omitempty"`
}

// DeletionService manages the background deletion worker and queue.
// It replaces the old init()-based goroutine and package-level globals.
//
// Grace period: When items enter the queue, a configurable grace period timer
// starts (default 30 seconds). The timer resets on any queue mutation
// (additions or cancellations). When the timer expires, all queued items are
// processed with rate limiting. Items added during processing are queued
// normally but do not restart the grace period until the current batch
// completes and a new item arrives.
type DeletionService struct {
	bus              *events.EventBus
	auditLog         *AuditLogService
	settings         SettingsReader
	engine           EngineStatsWriter
	metrics          DeletionStatsWriter
	approvalReturner ApprovalReturner
	rateLimiter      *rate.Limiter
	done             chan struct{}

	// Observable state
	currentlyDeleting atomic.Value // string
	processed         atomic.Int64
	failed            atomic.Int64

	// Batch tracking — per engine cycle. Set by the poller via SignalBatchSize(),
	// incremented by processJob(). When batchProcessed reaches batchExpected,
	// DeletionBatchCompleteEvent is published.
	batchExpected  atomic.Int64
	batchProcessed atomic.Int64
	batchSucceeded atomic.Int64
	batchFailed    atomic.Int64

	// Cancellation skip-list. Items are added via CancelDeletion() and
	// checked in processJob(). The map key is "mediaName:mediaType".
	cancelled sync.Map

	// Parallel tracking slice — holds queued items so callers can list and
	// inspect the queue (Go channels don't support peeking). Also serves as
	// the pending-jobs store for the grace-period-aware worker.
	queuedMu    sync.Mutex
	queuedItems []DeleteJob // full jobs (worker reads from here after grace period)

	// Grace period state
	graceTimerMu  sync.Mutex
	graceTimer    *time.Timer
	graceDeadline time.Time     // absolute time when grace period expires
	graceActive   atomic.Bool   // true while grace period is running
	processing    atomic.Bool   // true while the worker is draining the queue
	notify        chan struct{} // signals the worker that something happened
	stopCh        chan struct{} // closed when Stop() is called
}

// SettingsReader provides read access to application preferences and scoring factor weights.
// Defined here to avoid import cycles between DeletionService and SettingsService.
type SettingsReader interface {
	GetPreferences() (db.PreferenceSet, error)
	GetWeightMap() (map[string]int, error)
}

// EngineStatsWriter provides write access to engine run stats.
type EngineStatsWriter interface {
	IncrementDeletedStats(runStatsID uint, sizeBytes int64) error
}

// DeletionStatsWriter provides write access to lifetime deletion stats.
type DeletionStatsWriter interface {
	IncrementDeletionStats(sizeBytes int64) error
}

// ApprovalReturner allows the DeletionService to return dry-deleted approval
// queue items back to pending status without importing ApprovalService directly.
type ApprovalReturner interface {
	ReturnToPending(entryID uint) error
}

// NewDeletionService creates a new DeletionService.
// The settings, engine, and metrics dependencies are injected via SetDependencies()
// after registry construction to avoid circular initialization.
func NewDeletionService(bus *events.EventBus, auditLog *AuditLogService) *DeletionService {
	return &DeletionService{
		bus:         bus,
		auditLog:    auditLog,
		rateLimiter: rate.NewLimiter(rate.Every(3*time.Second), 1),
		done:        make(chan struct{}),
		notify:      make(chan struct{}, 1),
		stopCh:      make(chan struct{}),
	}
}

// SetDependencies wires cross-service dependencies that cannot be injected
// at construction time due to circular initialization in the registry.
func (s *DeletionService) SetDependencies(settings SettingsReader, engine EngineStatsWriter, metrics DeletionStatsWriter, approvalReturner ApprovalReturner) {
	s.settings = settings
	s.engine = engine
	s.metrics = metrics
	s.approvalReturner = approvalReturner
}

// Start begins the background deletion worker.
func (s *DeletionService) Start() {
	go s.worker()
}

// Stop signals the worker to finish and waits for completion.
func (s *DeletionService) Stop() {
	close(s.stopCh)
	<-s.done
}

// QueueDeletion enqueues a media item for background deletion.
// Starts or resets the grace period timer.
func (s *DeletionService) QueueDeletion(job DeleteJob) error {
	s.queuedMu.Lock()
	if len(s.queuedItems) >= 500 {
		s.queuedMu.Unlock()
		return fmt.Errorf("deletion queue is full")
	}
	s.queuedItems = append(s.queuedItems, job)
	queueSize := len(s.queuedItems)
	s.queuedMu.Unlock()

	s.bus.Publish(events.DeletionQueuedEvent{
		MediaName:     job.Item.Title,
		MediaType:     string(job.Item.Type),
		SizeBytes:     job.Item.SizeBytes,
		IntegrationID: job.Item.IntegrationID,
	})

	// Reset grace period if not currently processing
	if !s.processing.Load() {
		s.resetGracePeriod(queueSize)
	}

	// Wake up the worker
	s.poke()

	return nil
}

// CurrentlyDeleting returns the name of the item currently being deleted, or empty string.
func (s *DeletionService) CurrentlyDeleting() string {
	v := s.currentlyDeleting.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

// Processed returns the total number of items processed (deleted or dry-deleted).
func (s *DeletionService) Processed() int64 {
	return s.processed.Load()
}

// Failed returns the total number of failed deletion attempts.
func (s *DeletionService) Failed() int64 {
	return s.failed.Load()
}

// SignalBatchSize tells the deletion service how many items were queued in this
// engine cycle. When all items are processed, DeletionBatchCompleteEvent is
// published. If count is 0 (no items to process), the event is published
// immediately — the DeletionService owns this event.
//
// Also clears the cancellation skip-list from the previous cycle.
func (s *DeletionService) SignalBatchSize(count int) {
	s.clearCancelled()

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

// GracePeriodState returns the current grace period status for the API.
func (s *DeletionService) GracePeriodState() (active bool, remainingSeconds int, queueSize int) {
	s.queuedMu.Lock()
	queueSize = len(s.queuedItems)
	s.queuedMu.Unlock()

	active = s.graceActive.Load()
	if active {
		s.graceTimerMu.Lock()
		remaining := time.Until(s.graceDeadline)
		s.graceTimerMu.Unlock()
		if remaining > 0 {
			remainingSeconds = int(remaining.Seconds()) + 1 // round up
		}
	}
	return active, remainingSeconds, queueSize
}

// getGraceDelay reads the configured grace period from preferences.
// The route handler validates the range (10-300). Here we accept any positive
// value to support fast tests without artificial minimums.
func (s *DeletionService) getGraceDelay() time.Duration {
	if s.settings == nil {
		return 30 * time.Second
	}
	prefs, err := s.settings.GetPreferences()
	if err != nil {
		return 30 * time.Second
	}
	delay := prefs.DeletionQueueDelaySeconds
	if delay <= 0 {
		delay = 30
	}
	return time.Duration(delay) * time.Second
}

// resetGracePeriod starts or resets the grace period timer.
func (s *DeletionService) resetGracePeriod(queueSize int) {
	delay := s.getGraceDelay()

	s.graceTimerMu.Lock()
	if s.graceTimer != nil {
		s.graceTimer.Stop()
	}
	s.graceTimer = time.AfterFunc(delay, func() {
		s.graceActive.Store(false)
		// Publish grace period expired event
		s.queuedMu.Lock()
		qs := len(s.queuedItems)
		s.queuedMu.Unlock()
		s.bus.Publish(events.DeletionGracePeriodEvent{
			RemainingSeconds: 0,
			QueueSize:        qs,
			Active:           false,
		})
		s.poke() // wake up worker to start draining
	})
	s.graceDeadline = time.Now().Add(delay)
	s.graceTimerMu.Unlock()

	s.graceActive.Store(true)

	// Publish grace period started/reset event
	s.bus.Publish(events.DeletionGracePeriodEvent{
		RemainingSeconds: int(delay.Seconds()),
		QueueSize:        queueSize,
		Active:           true,
	})
}

// poke sends a non-blocking signal to the worker goroutine.
func (s *DeletionService) poke() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
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

// queueLen returns the number of queued items.
func (s *DeletionService) queueLen() int {
	s.queuedMu.Lock()
	defer s.queuedMu.Unlock()
	return len(s.queuedItems)
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

		_ = s.rateLimiter.Wait(context.Background()) //nolint:errcheck // Wait with background context never returns non-nil error
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

// dequeueJob pops the first job from the queued items slice.
func (s *DeletionService) dequeueJob() (DeleteJob, bool) {
	s.queuedMu.Lock()
	defer s.queuedMu.Unlock()

	if len(s.queuedItems) == 0 {
		return DeleteJob{}, false
	}

	job := s.queuedItems[0]
	s.queuedItems = s.queuedItems[1:]
	return job, true
}

// processJob handles a single deletion job. When deferredAuditEntries is non-nil,
// dry-run entries with UpsertAudit=true are collected for batch flush instead of
// being written individually to the database.
func (s *DeletionService) processJob(job DeleteJob, deferredAuditEntries *[]db.AuditLogEntry) {
	s.currentlyDeleting.Store(job.Item.Title)
	defer s.currentlyDeleting.Store("")
	defer s.checkBatchComplete()

	// Check cancellation skip-list before doing any work.
	if s.IsCancelled(job.Item.Title, string(job.Item.Type)) {
		s.cancelled.Delete(job.Item.Title + ":" + string(job.Item.Type))

		s.processed.Add(1)
		s.batchSucceeded.Add(1)

		logEntry := db.AuditLogEntry{
			MediaName:       job.Item.Title,
			MediaType:       string(job.Item.Type),
			Action:          db.ActionCancelled,
			SizeBytes:       job.Item.SizeBytes,
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

	// Check whether actual deletions are enabled via SettingsService
	deletionsEnabled := false
	if prefs, err := s.settings.GetPreferences(); err == nil {
		deletionsEnabled = prefs.DeletionsEnabled
	}

	factorsJSON, marshalErr := json.Marshal(job.Factors)
	if marshalErr != nil {
		slog.Error("Failed to marshal score factors", "component", "services", "error", marshalErr)
		factorsJSON = []byte("[]")
	}

	if !deletionsEnabled || job.ForceDryRun {
		// Dry-Delete: log but do not actually remove the file
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
			// Defer to batch flush at end of drainAll()
			*deferredAuditEntries = append(*deferredAuditEntries, logEntry)
		} else if job.UpsertAudit {
			// No collector available (e.g., shutdown path), write immediately
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
		return
	}

	// Actual deletion — nil-safety check for dry-run jobs that have no client
	if job.Client == nil {
		slog.Error("Deletion job has nil client — cannot perform actual deletion",
			"component", "services", "media", job.Item.Title)
		s.failed.Add(1)
		s.batchFailed.Add(1)
		s.publishProgress()
		return
	}
	if err := job.Client.DeleteMediaItem(job.Item); err != nil {
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

	// Increment deleted counter and freed bytes on the engine run stats row via EngineService
	if err := s.engine.IncrementDeletedStats(job.RunStatsID, job.Item.SizeBytes); err != nil {
		slog.Error("Failed to increment engine deleted stats", "component", "services", "error", err)
	}

	// Increment lifetime stats via MetricsService
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
		MediaName:     job.Item.Title,
		MediaType:     string(job.Item.Type),
		SizeBytes:     job.Item.SizeBytes,
		IntegrationID: job.Item.IntegrationID,
	})
	s.publishProgress()

	slog.Info("Deletion completed", "component", "services",
		"media", job.Item.Title, "action", "Deleted", "freed", job.Item.SizeBytes)
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

// ---------------------------------------------------------------------------
// Cancellation skip-list
// ---------------------------------------------------------------------------

// cancelKey builds the map key for the cancellation skip-list.
func cancelKey(mediaName, mediaType string) string {
	return mediaName + ":" + mediaType
}

// CancelDeletion marks a queued item for cancellation. When processJob
// encounters the item it will skip the actual deletion and log the
// cancellation instead. Returns true if the item was found in the queued
// items tracking slice (best-effort — the item may already be processing).
//
// Also resets the grace period timer if not currently processing, since
// the queue was mutated.
func (s *DeletionService) CancelDeletion(mediaName, mediaType string) bool {
	key := cancelKey(mediaName, mediaType)

	// Check whether the item exists in the tracking slice.
	s.queuedMu.Lock()
	found := false
	for _, item := range s.queuedItems {
		if item.Item.Title == mediaName && string(item.Item.Type) == mediaType {
			found = true
			break
		}
	}
	queueSize := len(s.queuedItems)
	s.queuedMu.Unlock()

	if !found {
		return false
	}

	s.cancelled.Store(key, true)

	// Reset grace period on queue mutation if not processing
	if !s.processing.Load() && queueSize > 0 {
		s.resetGracePeriod(queueSize)
	}

	return true
}

// IsCancelled checks whether a given item has been marked for cancellation.
func (s *DeletionService) IsCancelled(mediaName, mediaType string) bool {
	_, ok := s.cancelled.Load(cancelKey(mediaName, mediaType))
	return ok
}

// clearCancelled removes all entries from the cancellation skip-list.
// Called at the start of each batch via SignalBatchSize.
func (s *DeletionService) clearCancelled() {
	s.cancelled.Range(func(key, _ any) bool {
		s.cancelled.Delete(key)
		return true
	})
}

// ClearQueue cancels all items currently in the deletion queue.
// Returns the number of items cancelled. Resets the grace period timer.
func (s *DeletionService) ClearQueue() int {
	s.queuedMu.Lock()
	count := len(s.queuedItems)
	for _, job := range s.queuedItems {
		s.cancelled.Store(cancelKey(job.Item.Title, string(job.Item.Type)), true)
	}
	s.queuedMu.Unlock()

	// Stop the grace timer since there's nothing to process
	s.graceTimerMu.Lock()
	if s.graceTimer != nil {
		s.graceTimer.Stop()
		s.graceTimer = nil
	}
	s.graceTimerMu.Unlock()
	s.graceActive.Store(false)

	// Publish grace period deactivation
	s.bus.Publish(events.DeletionGracePeriodEvent{
		RemainingSeconds: 0,
		QueueSize:        0,
		Active:           false,
	})

	return count
}

// ---------------------------------------------------------------------------
// Queued items tracking
// ---------------------------------------------------------------------------

// FindQueuedItem returns the summary of a queued item by name and type,
// or nil if not found. Used by the snooze endpoint to look up integration details.
func (s *DeletionService) FindQueuedItem(mediaName, mediaType string) *DeleteJobSummary {
	s.queuedMu.Lock()
	defer s.queuedMu.Unlock()

	for _, job := range s.queuedItems {
		if job.Item.Title == mediaName && string(job.Item.Type) == mediaType {
			return &DeleteJobSummary{
				MediaName:     job.Item.Title,
				MediaType:     string(job.Item.Type),
				SizeBytes:     job.Item.SizeBytes,
				IntegrationID: job.Item.IntegrationID,
				Score:         job.Score,
				PosterURL:     job.Item.PosterURL,
			}
		}
	}
	return nil
}

// ListQueuedItems returns a snapshot copy of the items currently waiting in
// the deletion queue. The returned slice is safe to mutate.
func (s *DeletionService) ListQueuedItems() []DeleteJobSummary {
	s.queuedMu.Lock()
	defer s.queuedMu.Unlock()

	out := make([]DeleteJobSummary, 0, len(s.queuedItems))
	for _, job := range s.queuedItems {
		out = append(out, DeleteJobSummary{
			MediaName:     job.Item.Title,
			MediaType:     string(job.Item.Type),
			SizeBytes:     job.Item.SizeBytes,
			IntegrationID: job.Item.IntegrationID,
			Score:         job.Score,
			PosterURL:     job.Item.PosterURL,
		})
	}
	return out
}
