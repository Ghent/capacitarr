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
	Client      integrations.MediaDeleter
	Item        integrations.MediaItem
	Reason      string
	Score       float64
	Factors     []engine.ScoreFactor
	RunStatsID  uint  // Engine run stats row to increment Deleted counter
	DiskGroupID *uint // Disk group that triggered this deletion (nil for user-initiated deletes)
	ForceDryRun bool  // When true, skip actual deletion even if DeletionsEnabled=true
	UpsertAudit bool  // When true, use AuditLog.UpsertDryRun() (idempotent poller dry-runs); when false, use AuditLog.Create() (append-only)
}

// DeleteJobSummary is a serialisable snapshot of a queued deletion job,
// suitable for API responses. It deliberately excludes the Integration
// client to avoid exposing internal state.
type DeleteJobSummary struct {
	MediaName     string `json:"mediaName"`
	MediaType     string `json:"mediaType"`
	SizeBytes     int64  `json:"sizeBytes"`
	IntegrationID uint   `json:"integrationId"`
	Reason        string `json:"reason"`
}

// DeletionService manages the background deletion worker and queue.
// It replaces the old init()-based goroutine and package-level globals.
type DeletionService struct {
	bus         *events.EventBus
	auditLog    *AuditLogService
	settings    SettingsReader
	engine      EngineStatsWriter
	metrics     DeletionStatsWriter
	queue       chan DeleteJob
	rateLimiter *rate.Limiter
	done        chan struct{}

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

	// Parallel tracking slice — mirrors the channel contents so callers
	// can list queued items (Go channels don't support peeking).
	queuedMu    sync.Mutex
	queuedItems []DeleteJobSummary
}

// SettingsReader provides read access to application preferences.
// Defined here to avoid import cycles between DeletionService and SettingsService.
type SettingsReader interface {
	GetPreferences() (db.PreferenceSet, error)
}

// EngineStatsWriter provides write access to engine run stats.
type EngineStatsWriter interface {
	IncrementDeletedStats(runStatsID uint, sizeBytes int64) error
}

// DeletionStatsWriter provides write access to lifetime deletion stats.
type DeletionStatsWriter interface {
	IncrementDeletionStats(sizeBytes int64) error
}

// NewDeletionService creates a new DeletionService.
// The settings, engine, and metrics dependencies are injected via SetDependencies()
// after registry construction to avoid circular initialization.
func NewDeletionService(bus *events.EventBus, auditLog *AuditLogService) *DeletionService {
	return &DeletionService{
		bus:         bus,
		auditLog:    auditLog,
		queue:       make(chan DeleteJob, 500),
		rateLimiter: rate.NewLimiter(rate.Every(3*time.Second), 1),
		done:        make(chan struct{}),
	}
}

// SetDependencies wires cross-service dependencies that cannot be injected
// at construction time due to circular initialization in the registry.
func (s *DeletionService) SetDependencies(settings SettingsReader, engine EngineStatsWriter, metrics DeletionStatsWriter) {
	s.settings = settings
	s.engine = engine
	s.metrics = metrics
}

// Start begins the background deletion worker.
func (s *DeletionService) Start() {
	go s.worker()
}

// Stop signals the worker to finish and waits for completion.
func (s *DeletionService) Stop() {
	close(s.queue)
	<-s.done
}

// QueueDeletion enqueues a media item for background deletion.
// Returns an error if the queue is full.
func (s *DeletionService) QueueDeletion(job DeleteJob) error {
	select {
	case s.queue <- job:
		s.queuedMu.Lock()
		s.queuedItems = append(s.queuedItems, DeleteJobSummary{
			MediaName:     job.Item.Title,
			MediaType:     string(job.Item.Type),
			SizeBytes:     job.Item.SizeBytes,
			IntegrationID: job.Item.IntegrationID,
			Reason:        job.Reason,
		})
		s.queuedMu.Unlock()

		s.bus.Publish(events.DeletionQueuedEvent{
			MediaName:     job.Item.Title,
			MediaType:     string(job.Item.Type),
			SizeBytes:     job.Item.SizeBytes,
			IntegrationID: job.Item.IntegrationID,
		})

		return nil
	default:
		return fmt.Errorf("deletion queue is full")
	}
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

func (s *DeletionService) worker() {
	defer close(s.done)
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic recovered in deletion worker", "component", "services", "panic", r)
		}
	}()

	for job := range s.queue {
		_ = s.rateLimiter.Wait(context.Background()) //nolint:errcheck // Wait with background context never returns non-nil error
		s.processJob(job)
	}
}

func (s *DeletionService) processJob(job DeleteJob) {
	s.currentlyDeleting.Store(job.Item.Title)
	defer s.currentlyDeleting.Store("")
	defer s.checkBatchComplete()

	// Remove this item from the parallel tracking slice.
	s.removeQueuedItem(job.Item.Title, string(job.Item.Type))

	// Check cancellation skip-list before doing any work.
	if s.IsCancelled(job.Item.Title, string(job.Item.Type)) {
		s.cancelled.Delete(job.Item.Title + ":" + string(job.Item.Type))

		s.processed.Add(1)
		s.batchSucceeded.Add(1)

		logEntry := db.AuditLogEntry{
			MediaName:   job.Item.Title,
			MediaType:   string(job.Item.Type),
			Reason:      "Cancelled by user",
			Action:      db.ActionCancelled,
			SizeBytes:   job.Item.SizeBytes,
			DiskGroupID: job.DiskGroupID,
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
			MediaName:    job.Item.Title,
			MediaType:    string(job.Item.Type),
			Reason:       fmt.Sprintf("Score: %.2f (%s)", job.Score, job.Reason),
			ScoreDetails: string(factorsJSON),
			Action:       db.ActionDryDelete,
			SizeBytes:    job.Item.SizeBytes,
			Score:        job.Score,
			DiskGroupID:  job.DiskGroupID,
		}
		if job.UpsertAudit {
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
		MediaName:    job.Item.Title,
		MediaType:    string(job.Item.Type),
		Reason:       fmt.Sprintf("Score: %.2f (%s)", job.Score, job.Reason),
		ScoreDetails: string(factorsJSON),
		Action:       db.ActionDeleted,
		SizeBytes:    job.Item.SizeBytes,
		Score:        job.Score,
		DiskGroupID:  job.DiskGroupID,
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

// publishProgress publishes a DeletionProgressEvent with the current batch
// progress counters. Called after each job completes (success, failure, or
// dry-run) to provide real-time progress data for the frontend.
func (s *DeletionService) publishProgress() {
	s.bus.Publish(events.DeletionProgressEvent{
		CurrentItem: s.CurrentlyDeleting(),
		QueueDepth:  len(s.queue),
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
func (s *DeletionService) CancelDeletion(mediaName, mediaType string) bool {
	key := cancelKey(mediaName, mediaType)

	// Check whether the item exists in the tracking slice.
	s.queuedMu.Lock()
	found := false
	for _, item := range s.queuedItems {
		if item.MediaName == mediaName && item.MediaType == mediaType {
			found = true
			break
		}
	}
	s.queuedMu.Unlock()

	if !found {
		return false
	}

	s.cancelled.Store(key, true)
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

// ---------------------------------------------------------------------------
// Queued items tracking
// ---------------------------------------------------------------------------

// ListQueuedItems returns a snapshot copy of the items currently waiting in
// the deletion queue. The returned slice is safe to mutate.
func (s *DeletionService) ListQueuedItems() []DeleteJobSummary {
	s.queuedMu.Lock()
	defer s.queuedMu.Unlock()

	out := make([]DeleteJobSummary, len(s.queuedItems))
	copy(out, s.queuedItems)
	return out
}

// removeQueuedItem removes the first matching entry from the tracking slice.
// Called by processJob when the worker picks up the item.
func (s *DeletionService) removeQueuedItem(mediaName, mediaType string) {
	s.queuedMu.Lock()
	defer s.queuedMu.Unlock()

	for i, item := range s.queuedItems {
		if item.MediaName == mediaName && item.MediaType == mediaType {
			s.queuedItems = append(s.queuedItems[:i], s.queuedItems[i+1:]...)
			return
		}
	}
}
