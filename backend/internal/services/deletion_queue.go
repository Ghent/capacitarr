package services

import (
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

// enqueue enqueues a media item for background deletion.
// Starts or resets the grace period timer.
func (s *DeletionService) enqueue(job deleteJob) error {
	s.queuedMu.Lock()
	if len(s.queuedItems) >= 500 {
		s.queuedMu.Unlock()
		return ErrDeletionQueueFull
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
// The graceActive flag is set BEFORE the timer is created (under the same
// lock) to prevent a race where a very short timer fires before
// graceActive is set to true, causing subsequent Store(true) to overwrite
// the timer callback's Store(false) and leave grace permanently active.
func (s *DeletionService) resetGracePeriod(queueSize int) {
	delay := s.getGraceDelay()

	s.graceTimerMu.Lock()
	if s.graceTimer != nil {
		s.graceTimer.Stop()
	}
	s.graceActive.Store(true)
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

// queueLen returns the number of queued items (internal).
func (s *DeletionService) queueLen() int {
	s.queuedMu.Lock()
	defer s.queuedMu.Unlock()
	return len(s.queuedItems)
}

// QueueLen returns the number of items currently waiting in the deletion queue.
// Exported for use by MetricsService to report accurate queue depth in the REST API.
func (s *DeletionService) QueueLen() int {
	return s.queueLen()
}

// dequeueJob pops the first job from the queued items slice.
func (s *DeletionService) dequeueJob() (deleteJob, bool) {
	s.queuedMu.Lock()
	defer s.queuedMu.Unlock()

	if len(s.queuedItems) == 0 {
		return deleteJob{}, false
	}

	job := s.queuedItems[0]
	s.queuedItems = s.queuedItems[1:]
	return job, true
}

// ---------------------------------------------------------------------------
// Cancellation skip-list
// ---------------------------------------------------------------------------

// cancelKey builds the map key for the cancellation skip-list.
// Delegates to db.MediaKey for a consistent key format across the codebase.
func cancelKey(mediaName, mediaType string) string {
	return db.MediaKey(mediaName, mediaType)
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

// ClearQueueForDiskGroup cancels all queued items associated with a specific
// disk group. Used when a disk group's mode changes to invalidate items queued
// under assumptions of the previous mode. Returns the number of items cancelled.
func (s *DeletionService) ClearQueueForDiskGroup(diskGroupID uint) int {
	s.queuedMu.Lock()
	var count int
	for _, job := range s.queuedItems {
		if job.DiskGroupID != nil && *job.DiskGroupID == diskGroupID {
			s.cancelled.Store(cancelKey(job.Item.Title, string(job.Item.Type)), true)
			count++
		}
	}
	s.queuedMu.Unlock()
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
				MediaName:       job.Item.Title,
				MediaType:       string(job.Item.Type),
				SizeBytes:       job.Item.SizeBytes,
				IntegrationID:   job.Item.IntegrationID,
				Score:           job.Score,
				PosterURL:       job.Item.PosterURL,
				CollectionGroup: job.CollectionGroup,
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
			MediaName:       job.Item.Title,
			MediaType:       string(job.Item.Type),
			SizeBytes:       job.Item.SizeBytes,
			IntegrationID:   job.Item.IntegrationID,
			Score:           job.Score,
			PosterURL:       job.Item.PosterURL,
			CollectionGroup: job.CollectionGroup,
		})
	}
	return out
}
