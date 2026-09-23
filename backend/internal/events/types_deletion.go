package events

import "fmt"

// =============================================================================
// Deletion Events
// =============================================================================

// DeletionSuccessEvent is published when a media item is successfully deleted.
type DeletionSuccessEvent struct {
	MediaName       string `json:"mediaName"`
	MediaType       string `json:"mediaType"`
	SizeBytes       int64  `json:"sizeBytes"`
	IntegrationID   uint   `json:"integrationId"`
	CollectionGroup string `json:"collectionGroup,omitempty"` // Non-empty if part of a collection deletion
}

// EventType implements Event.
func (e DeletionSuccessEvent) EventType() string { return "deletion_success" }

// EventMessage implements Event.
func (e DeletionSuccessEvent) EventMessage() string {
	sizeGB := float64(e.SizeBytes) / (1024 * 1024 * 1024)
	return fmt.Sprintf("Deleted: %s (%.2f GB freed)", e.MediaName, sizeGB)
}

// DeletionFailedEvent is published when a deletion attempt fails.
type DeletionFailedEvent struct {
	MediaName     string `json:"mediaName"`
	MediaType     string `json:"mediaType"`
	IntegrationID uint   `json:"integrationId"`
	Error         string `json:"error"`
}

// EventType implements Event.
func (e DeletionFailedEvent) EventType() string { return "deletion_failed" }

// EventMessage implements Event.
func (e DeletionFailedEvent) EventMessage() string {
	return fmt.Sprintf("Deletion failed: %s — %s", e.MediaName, e.Error)
}

// DeletionDryRunEvent is published when a dry-run deletion is recorded.
type DeletionDryRunEvent struct {
	MediaName string `json:"mediaName"`
	MediaType string `json:"mediaType"`
	SizeBytes int64  `json:"sizeBytes"`
}

// EventType implements Event.
func (e DeletionDryRunEvent) EventType() string { return "deletion_dry_run" }

// EventMessage implements Event.
func (e DeletionDryRunEvent) EventMessage() string {
	return fmt.Sprintf("Dry-run flagged: %s", e.MediaName)
}

// DeletionQueuedEvent is published when a media item is added to the
// deletion queue. This is especially useful in approval mode, where
// approved items enter the deletion queue asynchronously — the frontend
// subscribes to this event to refresh the deletion queue card.
type DeletionQueuedEvent struct {
	MediaName     string `json:"mediaName"`
	MediaType     string `json:"mediaType"`
	SizeBytes     int64  `json:"sizeBytes"`
	IntegrationID uint   `json:"integrationId"`
}

// EventType implements Event.
func (e DeletionQueuedEvent) EventType() string { return "deletion_queued" }

// EventMessage implements Event.
func (e DeletionQueuedEvent) EventMessage() string {
	return fmt.Sprintf("Queued for deletion: %s", e.MediaName)
}

// DeletionCancelledEvent is published when a queued deletion is cancelled
// by the user before it executes.
type DeletionCancelledEvent struct {
	MediaName string `json:"mediaName"`
	MediaType string `json:"mediaType"`
	SizeBytes int64  `json:"sizeBytes"`
}

// EventType implements Event.
func (e DeletionCancelledEvent) EventType() string { return "deletion_cancelled" }

// EventMessage implements Event.
func (e DeletionCancelledEvent) EventMessage() string {
	return fmt.Sprintf("Deletion cancelled: %s", e.MediaName)
}

// DeletionBatchCompleteEvent is published when all queued deletions for an
// engine cycle have been processed (successfully or not). Used by the SSE
// broadcaster and audit log to signal batch completion.
type DeletionBatchCompleteEvent struct {
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

// EventType implements Event.
func (e DeletionBatchCompleteEvent) EventType() string { return "deletion_batch_complete" }

// EventMessage implements Event.
func (e DeletionBatchCompleteEvent) EventMessage() string {
	return fmt.Sprintf("Deletion batch complete: %d succeeded, %d failed", e.Succeeded, e.Failed)
}

// DeletionProgressEvent is published after each deletion job completes,
// providing real-time progress data for the frontend progress indicator
// and sparkline updates.
type DeletionProgressEvent struct {
	CurrentItem string `json:"currentItem"`
	QueueDepth  int    `json:"queueDepth"`
	Processed   int    `json:"processed"`
	Succeeded   int    `json:"succeeded"`
	Failed      int    `json:"failed"`
	BatchTotal  int    `json:"batchTotal"`
}

// EventType implements Event.
func (e DeletionProgressEvent) EventType() string { return "deletion_progress" }

// EventMessage implements Event.
func (e DeletionProgressEvent) EventMessage() string {
	return fmt.Sprintf("Deletion progress: %d/%d completed (%d succeeded, %d failed)",
		e.Processed, e.BatchTotal, e.Succeeded, e.Failed)
}

// DeletionGracePeriodEvent is published when the deletion queue grace period
// starts, resets, or expires. The frontend uses this to show a countdown timer
// before the queue begins processing.
type DeletionGracePeriodEvent struct {
	RemainingSeconds int  `json:"remainingSeconds"`
	QueueSize        int  `json:"queueSize"`
	Active           bool `json:"active"` // true = grace period running, false = processing started
}

// EventType implements Event.
func (e DeletionGracePeriodEvent) EventType() string { return "deletion_grace_period" }

// EventMessage implements Event.
func (e DeletionGracePeriodEvent) EventMessage() string {
	if e.Active {
		return fmt.Sprintf("Deletion grace period active: %ds remaining, %d items queued", e.RemainingSeconds, e.QueueSize)
	}
	return fmt.Sprintf("Deletion grace period expired: processing %d items", e.QueueSize)
}
