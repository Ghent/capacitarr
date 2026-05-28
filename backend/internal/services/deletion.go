package services

import (
	"context"
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
	Client             integrations.MediaDeleter
	Item               integrations.MediaItem
	Score              float64
	Factors            []engine.ScoreFactor
	Trigger            string // "engine", "user", "approval"
	RunStatsID         uint   // Engine run stats row to increment Deleted counter
	DiskGroupID        *uint  // Disk group that triggered this deletion (nil for user-initiated deletes)
	ForceDryRun        bool   // When true, skip actual deletion even if DeletionsEnabled=true
	UpsertAudit        bool   // When true, use AuditLog.UpsertDryRun() (idempotent poller dry-runs); when false, use AuditLog.Create() (append-only)
	ApprovalEntryID    uint   // Non-zero if this job originated from an approval queue item
	SunsetQueueItemID  uint   // Non-zero if this job originated from a sunset queue item; cleaned up after successful deletion
	CollectionGroup    string // Non-empty if this job is part of a collection deletion (e.g., "Sonic the Hedgehog Collection")
	EnqueuedMode       string // Execution mode when this job was enqueued (defense-in-depth: processJob cancels if mode changed)
	AddImportExclusion bool   // When true, tell the *arr server to add the item to its import exclusion list on delete
}

// DeleteJobSummary is a serialisable snapshot of a queued deletion job,
// suitable for API responses. It deliberately excludes the Integration
// client to avoid exposing internal state.
type DeleteJobSummary struct {
	MediaName       string  `json:"mediaName"`
	MediaType       string  `json:"mediaType"`
	SizeBytes       int64   `json:"sizeBytes"`
	IntegrationID   uint    `json:"integrationId"`
	Score           float64 `json:"score"`
	PosterURL       string  `json:"posterUrl,omitempty"`
	CollectionGroup string  `json:"collectionGroup,omitempty"`
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
	approvalSnoozer  ApprovalSnoozer
	diskGroups       DiskGroupModeReader
	sunsetCleaner    SunsetQueueCleaner
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
	// checked in processJob(). The map key is produced by cancelKey() which
	// delegates to db.MediaKey() for a consistent key format.
	cancelled sync.Map

	// Parallel tracking slice — holds queued items so callers can list and
	// inspect the queue (Go channels don't support peeking). Also serves as
	// the pending-jobs store for the grace-period-aware worker.
	queuedMu    sync.Mutex
	queuedItems []DeleteJob // full jobs (worker reads from here after grace period)

	// Grace period state
	graceTimerMu  sync.Mutex
	graceTimer    *time.Timer
	graceDeadline time.Time          // absolute time when grace period expires
	graceActive   atomic.Bool        // true while grace period is running
	processing    atomic.Bool        // true while the worker is draining the queue
	notify        chan struct{}      // signals the worker that something happened
	stopCh        chan struct{}      // closed when Stop() is called
	stopCtx       context.Context    // cancelled when Stop() is called; passed to rate limiter
	stopCancel    context.CancelFunc // cancels stopCtx
}

// ---------------------------------------------------------------------------
// Interfaces — defined here to avoid import cycles between services.
// ---------------------------------------------------------------------------

// SettingsReader provides read access to application preferences and scoring factor weights.
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

// ApprovalReturner allows the DeletionService to manage approval queue items
// after deletion without importing ApprovalService directly.
type ApprovalReturner interface {
	ReturnToPending(entryID uint) error
	RemoveEntry(entryID uint) error
}

// ApprovalSnoozer allows the DeletionService to create snoozed entries in the
// approval queue without importing ApprovalService directly.
type ApprovalSnoozer interface {
	CreateSnoozedEntry(mediaName, mediaType string, integrationID uint, snoozeDurationHours int) (*time.Time, error)
}

// DiskGroupModeReader allows the DeletionService to look up the per-disk-group
// execution mode. Used by the mode-change safety check so it compares against
// the actual group mode rather than the global default.
type DiskGroupModeReader interface {
	GetByID(id uint) (*db.DiskGroup, error)
}

// SunsetQueueCleaner allows the DeletionService to remove sunset queue items
// after a file has been successfully deleted. This closes the sunset lifecycle:
// item enters queue → countdown expires → DeletionService deletes file → row removed.
type SunsetQueueCleaner interface {
	RemoveCompleted(id uint) error
}

// ---------------------------------------------------------------------------
// Constructor and lifecycle
// ---------------------------------------------------------------------------

// NewDeletionService creates a new DeletionService.
// The settings, engine, and metrics dependencies are injected via SetDependencies()
// after registry construction to avoid circular initialization.
func NewDeletionService(bus *events.EventBus, auditLog *AuditLogService) *DeletionService {
	ctx, cancel := context.WithCancel(context.Background())
	return &DeletionService{
		bus:         bus,
		auditLog:    auditLog,
		rateLimiter: rate.NewLimiter(rate.Every(3*time.Second), 1),
		done:        make(chan struct{}),
		notify:      make(chan struct{}, 1),
		stopCh:      make(chan struct{}),
		stopCtx:     ctx,
		stopCancel:  cancel,
	}
}

// Wired returns true when all lazily-injected dependencies are non-nil.
// Used by Registry.Validate() to catch missing wiring at startup.
func (s *DeletionService) Wired() bool {
	return s.settings != nil && s.engine != nil && s.metrics != nil && s.approvalReturner != nil && s.approvalSnoozer != nil && s.diskGroups != nil && s.sunsetCleaner != nil
}

// SetDependencies wires cross-service dependencies that cannot be injected
// at construction time due to circular initialization in the registry.
func (s *DeletionService) SetDependencies(settings SettingsReader, engine EngineStatsWriter, metrics DeletionStatsWriter, approvalReturner ApprovalReturner, approvalSnoozer ApprovalSnoozer, diskGroups DiskGroupModeReader, sunsetCleaner SunsetQueueCleaner) {
	s.settings = settings
	s.engine = engine
	s.metrics = metrics
	s.approvalReturner = approvalReturner
	s.approvalSnoozer = approvalSnoozer
	s.diskGroups = diskGroups
	s.sunsetCleaner = sunsetCleaner
}

// Start begins the background deletion worker. Panics if SetDependencies()
// has not been called — catches misuse in tests that construct a
// DeletionService directly without the registry.
func (s *DeletionService) Start() {
	if !s.Wired() {
		panic("DeletionService.Start() called before SetDependencies()")
	}
	go s.worker()
}

// Stop signals the worker to finish and waits for completion.
// The context cancellation ensures the rate limiter returns immediately
// instead of blocking for up to 3s per remaining queued item.
func (s *DeletionService) Stop() {
	close(s.stopCh)
	s.stopCancel()
	<-s.done
}

// ---------------------------------------------------------------------------
// Observable state
// ---------------------------------------------------------------------------

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
