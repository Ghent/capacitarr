package events

import "fmt"

// =============================================================================
// Approval Events
// =============================================================================

// ApprovalApprovedEvent is published when a queued item is approved for deletion.
type ApprovalApprovedEvent struct {
	EntryID   uint   `json:"entryId"`
	MediaName string `json:"mediaName"`
	MediaType string `json:"mediaType"`
	SizeBytes int64  `json:"sizeBytes"`
}

// EventType implements Event.
func (e ApprovalApprovedEvent) EventType() string { return "approval_approved" }

// EventMessage implements Event.
func (e ApprovalApprovedEvent) EventMessage() string {
	return fmt.Sprintf("Approved for deletion: %s", e.MediaName)
}

// ApprovalRejectedEvent is published when a queued item is rejected (snoozed).
type ApprovalRejectedEvent struct {
	EntryID        uint   `json:"entryId"`
	MediaName      string `json:"mediaName"`
	MediaType      string `json:"mediaType"`
	SnoozeDuration string `json:"snoozeDuration"` // e.g. "24h"
}

// EventType implements Event.
func (e ApprovalRejectedEvent) EventType() string { return "approval_rejected" }

// EventMessage implements Event.
func (e ApprovalRejectedEvent) EventMessage() string {
	return fmt.Sprintf("Rejected (snoozed): %s", e.MediaName)
}

// ApprovalUnsnoozedEvent is published when a snoozed item is manually unsnoozed.
type ApprovalUnsnoozedEvent struct {
	EntryID   uint   `json:"entryId"`
	MediaName string `json:"mediaName"`
	MediaType string `json:"mediaType"`
}

// EventType implements Event.
func (e ApprovalUnsnoozedEvent) EventType() string { return "approval_unsnoozed" }

// EventMessage implements Event.
func (e ApprovalUnsnoozedEvent) EventMessage() string {
	return fmt.Sprintf("Unsnoozed: %s", e.MediaName)
}

// ApprovalBulkUnsnoozedEvent is published when all snoozed items are cleared
// because disk usage dropped below threshold.
type ApprovalBulkUnsnoozedEvent struct {
	Count int `json:"count"`
}

// EventType implements Event.
func (e ApprovalBulkUnsnoozedEvent) EventType() string { return "approval_bulk_unsnoozed" }

// EventMessage implements Event.
func (e ApprovalBulkUnsnoozedEvent) EventMessage() string {
	return fmt.Sprintf("Bulk unsnoozed %d items (disk below threshold)", e.Count)
}

// ApprovalOrphansRecoveredEvent is published when orphaned approval items
// are requeued after a restart or integration reconnection.
type ApprovalOrphansRecoveredEvent struct {
	Count int `json:"count"`
}

// EventType implements Event.
func (e ApprovalOrphansRecoveredEvent) EventType() string { return "approval_orphans_recovered" }

// EventMessage implements Event.
func (e ApprovalOrphansRecoveredEvent) EventMessage() string {
	return fmt.Sprintf("Recovered %d orphaned approval items", e.Count)
}

// ApprovalQueueClearedEvent is published when the approval queue is cleared
// because disk usage dropped below threshold.
type ApprovalQueueClearedEvent struct {
	Count int `json:"count"`
}

// EventType implements Event.
func (e ApprovalQueueClearedEvent) EventType() string { return "approval_queue_cleared" }

// EventMessage implements Event.
func (e ApprovalQueueClearedEvent) EventMessage() string {
	return fmt.Sprintf("Approval queue cleared: %d items removed (disk below threshold)", e.Count)
}

// ApprovalQueueReconciledEvent is published when stale pending items are
// dismissed from a disk group's approval queue during per-cycle reconciliation.
type ApprovalQueueReconciledEvent struct {
	DiskGroupID uint `json:"diskGroupId"`
	Dismissed   int  `json:"dismissed"`
}

// EventType implements Event.
func (e ApprovalQueueReconciledEvent) EventType() string { return "approval_queue_reconciled" }

// EventMessage implements Event.
func (e ApprovalQueueReconciledEvent) EventMessage() string {
	return fmt.Sprintf("Approval queue reconciled for disk group %d: %d stale items dismissed", e.DiskGroupID, e.Dismissed)
}

// ApprovalDismissedEvent is published when a single approval queue item is
// manually dismissed (removed without approving or snoozing).
type ApprovalDismissedEvent struct {
	EntryID   uint   `json:"entryId"`
	MediaName string `json:"mediaName"`
	MediaType string `json:"mediaType"`
}

// EventType implements Event.
func (e ApprovalDismissedEvent) EventType() string { return "approval_dismissed" }

// EventMessage implements Event.
func (e ApprovalDismissedEvent) EventMessage() string {
	return fmt.Sprintf("Dismissed from queue: %s", e.MediaName)
}

// ApprovalReturnedToPendingEvent is published when a dry-deleted approval queue
// item is returned to pending status, creating the intentional dry-run loop:
// approve → dry-delete → return to pending.
type ApprovalReturnedToPendingEvent struct {
	EntryID   uint   `json:"entryId"`
	MediaName string `json:"mediaName"`
	MediaType string `json:"mediaType"`
}

// EventType implements Event.
func (e ApprovalReturnedToPendingEvent) EventType() string {
	return "approval_returned_to_pending"
}

// EventMessage implements Event.
func (e ApprovalReturnedToPendingEvent) EventMessage() string {
	return fmt.Sprintf("Returned to pending after dry-delete: %s", e.MediaName)
}
