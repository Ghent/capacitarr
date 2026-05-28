package events

import (
	"fmt"
	"time"
)

// =============================================================================
// Engine Events
// =============================================================================

// EngineStartEvent is published when an engine evaluation cycle begins.
type EngineStartEvent struct {
	DiskGroupModes map[uint]string `json:"diskGroupModes"`
}

// EventType implements Event.
func (e EngineStartEvent) EventType() string { return "engine_start" }

// EventMessage implements Event.
func (e EngineStartEvent) EventMessage() string {
	return fmt.Sprintf("Engine run started (%d disk groups)", len(e.DiskGroupModes))
}

// EngineCompleteEvent is published when an engine evaluation cycle finishes.
// Note: Deleted count and FreedBytes are NOT included here because deletions
// happen asynchronously in the DeletionService worker and may not be complete
// when the engine cycle publishes this event. The frontend reads those stats
// from the REST endpoint (GET /worker/stats), which queries the DB where the
// deletion worker atomically increments the counters.
type EngineCompleteEvent struct {
	Evaluated        int             `json:"evaluated"`
	Candidates       int             `json:"candidates"`
	DurationMs       int64           `json:"durationMs"`
	DiskGroupModes   map[uint]string `json:"diskGroupModes"`
	FreedBytes       int64           `json:"freedBytes"`       // Potential bytes freed (approval/dry-run) or actual bytes queued (auto)
	CompletedAtEpoch int64           `json:"completedAtEpoch"` // Unix epoch seconds when the run finished
}

// EventType implements Event.
func (e EngineCompleteEvent) EventType() string { return "engine_complete" }

// EventMessage implements Event.
func (e EngineCompleteEvent) EventMessage() string {
	return fmt.Sprintf("Engine run completed: evaluated %d, candidates %d", e.Evaluated, e.Candidates)
}

// EngineErrorEvent is published when an engine cycle fails.
type EngineErrorEvent struct {
	Error string `json:"error"`
}

// EventType implements Event.
func (e EngineErrorEvent) EventType() string { return "engine_error" }

// EventMessage implements Event.
func (e EngineErrorEvent) EventMessage() string { return "Engine error: " + e.Error }

// EnrichmentCompleteEvent is published after the enrichment pipeline finishes.
// Provides a summary of enrichment health so the frontend can display
// enrichment statistics and surface configuration problems.
type EnrichmentCompleteEvent struct {
	EnrichersRun   int       `json:"enrichersRun"`   // Total enrichers executed
	ItemsProcessed int       `json:"itemsProcessed"` // Total items passed through the pipeline
	TotalMatches   int       `json:"totalMatches"`   // Sum of matches across all enrichers
	ZeroMatchers   []string  `json:"zeroMatchers"`   // Enrichers that produced zero matches despite having data
	Timestamp      time.Time `json:"timestamp"`
}

// EventType implements Event.
func (e EnrichmentCompleteEvent) EventType() string { return "enrichment_complete" }

// EventMessage implements Event.
func (e EnrichmentCompleteEvent) EventMessage() string {
	if len(e.ZeroMatchers) > 0 {
		return fmt.Sprintf("Enrichment complete: %d enrichers, %d matches (%d zero-match enrichers)",
			e.EnrichersRun, e.TotalMatches, len(e.ZeroMatchers))
	}
	return fmt.Sprintf("Enrichment complete: %d enrichers, %d matches", e.EnrichersRun, e.TotalMatches)
}

// ManualRunTriggeredEvent is published when a user manually triggers an engine run.
type ManualRunTriggeredEvent struct{}

// EventType implements Event.
func (e ManualRunTriggeredEvent) EventType() string { return "manual_run_triggered" }

// EventMessage implements Event.
func (e ManualRunTriggeredEvent) EventMessage() string { return "Manual engine run triggered" }

// =============================================================================
// Settings Events
// =============================================================================

// SettingsChangedEvent is published when preferences are saved.
type SettingsChangedEvent struct {
	Changes map[string]any `json:"changes,omitempty"` // Fields that changed
}

// EventType implements Event.
func (e SettingsChangedEvent) EventType() string { return "settings_changed" }

// EventMessage implements Event.
func (e SettingsChangedEvent) EventMessage() string { return "Settings updated" }

// ThresholdChangedEvent is published when disk group thresholds are updated.
type ThresholdChangedEvent struct {
	MountPath    string  `json:"mountPath"`
	ThresholdPct float64 `json:"thresholdPct"`
	TargetPct    float64 `json:"targetPct"`
}

// EventType implements Event.
func (e ThresholdChangedEvent) EventType() string { return "threshold_changed" }

// EventMessage implements Event.
func (e ThresholdChangedEvent) EventMessage() string {
	return fmt.Sprintf("Thresholds updated for %s: trigger at %.0f%%, target %.0f%%",
		e.MountPath, e.ThresholdPct, e.TargetPct)
}

// SettingsExportedEvent is published when settings are exported.
type SettingsExportedEvent struct {
	Sections []string `json:"sections"`
}

// EventType implements Event.
func (e SettingsExportedEvent) EventType() string { return "settings_exported" }

// EventMessage implements Event.
func (e SettingsExportedEvent) EventMessage() string {
	return fmt.Sprintf("Settings exported: %v", e.Sections)
}

// SettingsImportedEvent is published when settings are imported.
type SettingsImportedEvent struct {
	Sections []string       `json:"sections"`
	Result   map[string]any `json:"result"`
}

// EventType implements Event.
func (e SettingsImportedEvent) EventType() string { return "settings_imported" }

// EventMessage implements Event.
func (e SettingsImportedEvent) EventMessage() string {
	return fmt.Sprintf("Settings imported: %v", e.Sections)
}

// =============================================================================
// Auth Events
// =============================================================================

// LoginEvent is published on successful authentication.
type LoginEvent struct {
	Username string `json:"username"`
}

// EventType implements Event.
func (e LoginEvent) EventType() string { return "login" }

// EventMessage implements Event.
func (e LoginEvent) EventMessage() string { return "User logged in: " + e.Username }

// PasswordChangedEvent is published when a user changes their password.
type PasswordChangedEvent struct {
	Username string `json:"username"`
}

// EventType implements Event.
func (e PasswordChangedEvent) EventType() string { return "password_changed" }

// EventMessage implements Event.
func (e PasswordChangedEvent) EventMessage() string { return "Password changed for " + e.Username }

// UsernameChangedEvent is published when a user changes their username.
type UsernameChangedEvent struct {
	OldUsername string `json:"oldUsername"`
	NewUsername string `json:"newUsername"`
}

// EventType implements Event.
func (e UsernameChangedEvent) EventType() string { return "username_changed" }

// EventMessage implements Event.
func (e UsernameChangedEvent) EventMessage() string {
	return fmt.Sprintf("Username changed from %s to %s", e.OldUsername, e.NewUsername)
}

// APIKeyGeneratedEvent is published when an API key is generated.
type APIKeyGeneratedEvent struct {
	Username string `json:"username"`
	Hint     string `json:"hint"` // Last 4 chars
}

// EventType implements Event.
func (e APIKeyGeneratedEvent) EventType() string { return "api_key_generated" }

// EventMessage implements Event.
func (e APIKeyGeneratedEvent) EventMessage() string {
	return fmt.Sprintf("API key generated for %s (ending in %s)", e.Username, e.Hint)
}

// =============================================================================
// Integration Events
// =============================================================================

// IntegrationAddedEvent is published when a new integration is created.
type IntegrationAddedEvent struct {
	IntegrationID   uint   `json:"integrationId"`
	IntegrationType string `json:"integrationType"`
	Name            string `json:"name"`
}

// EventType implements Event.
func (e IntegrationAddedEvent) EventType() string { return "integration_added" }

// EventMessage implements Event.
func (e IntegrationAddedEvent) EventMessage() string {
	return fmt.Sprintf("Integration added: %s (%s)", e.Name, e.IntegrationType)
}

// IntegrationUpdatedEvent is published when an integration is modified.
type IntegrationUpdatedEvent struct {
	IntegrationID   uint   `json:"integrationId"`
	IntegrationType string `json:"integrationType"`
	Name            string `json:"name"`
}

// EventType implements Event.
func (e IntegrationUpdatedEvent) EventType() string { return "integration_updated" }

// EventMessage implements Event.
func (e IntegrationUpdatedEvent) EventMessage() string {
	return fmt.Sprintf("Integration updated: %s (%s)", e.Name, e.IntegrationType)
}

// IntegrationRemovedEvent is published when an integration is deleted.
type IntegrationRemovedEvent struct {
	IntegrationID   uint   `json:"integrationId"`
	IntegrationType string `json:"integrationType"`
	Name            string `json:"name"`
}

// EventType implements Event.
func (e IntegrationRemovedEvent) EventType() string { return "integration_removed" }

// EventMessage implements Event.
func (e IntegrationRemovedEvent) EventMessage() string {
	return fmt.Sprintf("Integration removed: %s (%s)", e.Name, e.IntegrationType)
}

// IntegrationTestFailedEvent is published on a failed integration connection test.
type IntegrationTestFailedEvent struct {
	IntegrationType string `json:"integrationType"`
	Name            string `json:"name"`
	URL             string `json:"url"`
	Error           string `json:"error"`
}

// EventType implements Event.
func (e IntegrationTestFailedEvent) EventType() string { return "integration_test_failed" }

// EventMessage implements Event.
func (e IntegrationTestFailedEvent) EventMessage() string {
	return fmt.Sprintf("Connection test failed: %s (%s) — %s", e.Name, e.IntegrationType, e.Error)
}

// IntegrationRecoveredEvent is published when an integration transitions from
// an error state to a healthy state (lastError cleared after being non-empty).
type IntegrationRecoveredEvent struct {
	IntegrationID   uint   `json:"integrationId"`
	IntegrationType string `json:"integrationType"`
	Name            string `json:"name"`
	URL             string `json:"url"`
}

// EventType implements Event.
func (e IntegrationRecoveredEvent) EventType() string { return "integration_recovered" }

// EventMessage implements Event.
func (e IntegrationRecoveredEvent) EventMessage() string {
	return fmt.Sprintf("Integration recovered: %s (%s)", e.Name, e.IntegrationType)
}

// IntegrationRecoveryAttemptEvent is published when the IntegrationHealthService
// probes a failing integration. Fires on both success and failure so the frontend
// can show real-time recovery progress.
type IntegrationRecoveryAttemptEvent struct {
	IntegrationID    uint   `json:"integrationId"`
	IntegrationType  string `json:"integrationType"`
	Name             string `json:"name"`
	Attempt          int    `json:"attempt"`
	Success          bool   `json:"success"`
	Error            string `json:"error,omitempty"`
	NextRetrySeconds int    `json:"nextRetrySeconds,omitempty"` // Seconds until next probe (0 if recovered)
}

// EventType implements Event.
func (e IntegrationRecoveryAttemptEvent) EventType() string {
	return "integration_recovery_attempt"
}

// EventMessage implements Event.
func (e IntegrationRecoveryAttemptEvent) EventMessage() string {
	if e.Success {
		return fmt.Sprintf("Recovery probe succeeded: %s (%s) after %d attempt(s)", e.Name, e.IntegrationType, e.Attempt)
	}
	return fmt.Sprintf("Recovery probe failed: %s (%s) — attempt %d, retry in %ds", e.Name, e.IntegrationType, e.Attempt, e.NextRetrySeconds)
}

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

// =============================================================================
// Disk Events
// =============================================================================

// ThresholdBreachedEvent is published when disk usage exceeds the configured
// threshold during an engine evaluation cycle. This is distinct from
// ThresholdChangedEvent, which fires when an admin changes the threshold
// settings — ThresholdBreachedEvent fires on actual disk usage detection.
type ThresholdBreachedEvent struct {
	MountPath    string  `json:"mountPath"`
	CurrentPct   float64 `json:"currentPct"`
	ThresholdPct float64 `json:"thresholdPct"`
	TargetPct    float64 `json:"targetPct"`
}

// EventType implements Event.
func (e ThresholdBreachedEvent) EventType() string { return "threshold_breached" }

// EventMessage implements Event.
func (e ThresholdBreachedEvent) EventMessage() string {
	return fmt.Sprintf("Disk threshold breached on %s: %.1f%% (threshold: %.0f%%)",
		e.MountPath, e.CurrentPct, e.ThresholdPct)
}

// =============================================================================
// Disk Group Lifecycle Events
// =============================================================================

// DiskGroupStaleEvent is published when a disk group is marked stale (no longer
// reported by any integration). Used by the activity feed — not user-actionable.
type DiskGroupStaleEvent struct {
	DiskGroupID uint   `json:"diskGroupId"`
	MountPath   string `json:"mountPath"`
}

// EventType implements Event.
func (e DiskGroupStaleEvent) EventType() string { return "disk_group_stale" }

// EventMessage implements Event.
func (e DiskGroupStaleEvent) EventMessage() string {
	return fmt.Sprintf("Disk group %s marked stale — not reported by any integration", e.MountPath)
}

// DiskGroupReapedEvent is published when a stale disk group is permanently
// deleted after the grace period expires.
type DiskGroupReapedEvent struct {
	DiskGroupID uint   `json:"diskGroupId"`
	MountPath   string `json:"mountPath"`
	StaleDays   int    `json:"staleDays"`
}

// EventType implements Event.
func (e DiskGroupReapedEvent) EventType() string { return "disk_group_reaped" }

// EventMessage implements Event.
func (e DiskGroupReapedEvent) EventMessage() string {
	return fmt.Sprintf("Disk group %s removed after %d days without integration data", e.MountPath, e.StaleDays)
}

// DiskGroupResurrectedEvent is published when a stale disk group is restored to
// active status because its mount path was reported again by an integration.
type DiskGroupResurrectedEvent struct {
	DiskGroupID uint   `json:"diskGroupId"`
	MountPath   string `json:"mountPath"`
	StaleDays   int    `json:"staleDays"` // How long it was stale before resurrection
}

// EventType implements Event.
func (e DiskGroupResurrectedEvent) EventType() string { return "disk_group_resurrected" }

// EventMessage implements Event.
func (e DiskGroupResurrectedEvent) EventMessage() string {
	return fmt.Sprintf("Disk group %s restored (was stale for %d days)", e.MountPath, e.StaleDays)
}

// =============================================================================
// Version Events
// =============================================================================

// UpdateAvailableEvent is published when the VersionService detects a new
// release for the first time. It fires at most once per version to avoid
// repeated notifications on cache refresh.
type UpdateAvailableEvent struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	ReleaseURL     string `json:"releaseUrl"`
}

// EventType implements Event.
func (e UpdateAvailableEvent) EventType() string { return "update_available" }

// EventMessage implements Event.
func (e UpdateAvailableEvent) EventMessage() string {
	return fmt.Sprintf("Update available: %s → %s", e.CurrentVersion, e.LatestVersion)
}

// VersionCheckEvent is published every time the VersionService performs an
// update check, regardless of whether an update is available. This provides
// activity log visibility into when checks are happening.
type VersionCheckEvent struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
}

// EventType implements Event.
func (e VersionCheckEvent) EventType() string { return "version_check" }

// EventMessage implements Event.
func (e VersionCheckEvent) EventMessage() string {
	if e.UpdateAvailable {
		return fmt.Sprintf("Version check: update available (%s → %s)", e.CurrentVersion, e.LatestVersion)
	}
	return fmt.Sprintf("Version check: up to date (%s)", e.CurrentVersion)
}

// =============================================================================
// Rule Events
// =============================================================================

// RuleCreatedEvent is published when a custom rule is created.
type RuleCreatedEvent struct {
	RuleID uint   `json:"ruleId"`
	Field  string `json:"field"`
	Effect string `json:"effect"`
}

// EventType implements Event.
func (e RuleCreatedEvent) EventType() string { return "rule_created" }

// EventMessage implements Event.
func (e RuleCreatedEvent) EventMessage() string {
	return fmt.Sprintf("Custom rule created: %s → %s", e.Field, e.Effect)
}

// RuleUpdatedEvent is published when a custom rule is modified.
type RuleUpdatedEvent struct {
	RuleID uint   `json:"ruleId"`
	Field  string `json:"field"`
	Effect string `json:"effect"`
}

// EventType implements Event.
func (e RuleUpdatedEvent) EventType() string { return "rule_updated" }

// EventMessage implements Event.
func (e RuleUpdatedEvent) EventMessage() string {
	return fmt.Sprintf("Custom rule updated: %s → %s", e.Field, e.Effect)
}

// RuleDeletedEvent is published when a custom rule is deleted.
type RuleDeletedEvent struct {
	RuleID uint   `json:"ruleId"`
	Field  string `json:"field"`
}

// EventType implements Event.
func (e RuleDeletedEvent) EventType() string { return "rule_deleted" }

// EventMessage implements Event.
func (e RuleDeletedEvent) EventMessage() string {
	return fmt.Sprintf("Custom rule deleted: %s (ID %d)", e.Field, e.RuleID)
}

// =============================================================================
// Notification Events
// =============================================================================

// NotificationChannelAddedEvent is published when a notification channel is created.
type NotificationChannelAddedEvent struct {
	ChannelID   uint   `json:"channelId"`
	ChannelType string `json:"channelType"`
	Name        string `json:"name"`
}

// EventType implements Event.
func (e NotificationChannelAddedEvent) EventType() string { return "notification_channel_added" }

// EventMessage implements Event.
func (e NotificationChannelAddedEvent) EventMessage() string {
	return fmt.Sprintf("Notification channel added: %s (%s)", e.Name, e.ChannelType)
}

// NotificationChannelUpdatedEvent is published when a notification channel is modified.
type NotificationChannelUpdatedEvent struct {
	ChannelID   uint   `json:"channelId"`
	ChannelType string `json:"channelType"`
	Name        string `json:"name"`
}

// EventType implements Event.
func (e NotificationChannelUpdatedEvent) EventType() string { return "notification_channel_updated" }

// EventMessage implements Event.
func (e NotificationChannelUpdatedEvent) EventMessage() string {
	return fmt.Sprintf("Notification channel updated: %s (%s)", e.Name, e.ChannelType)
}

// NotificationChannelRemovedEvent is published when a notification channel is deleted.
type NotificationChannelRemovedEvent struct {
	ChannelID   uint   `json:"channelId"`
	ChannelType string `json:"channelType"`
	Name        string `json:"name"`
}

// EventType implements Event.
func (e NotificationChannelRemovedEvent) EventType() string { return "notification_channel_removed" }

// EventMessage implements Event.
func (e NotificationChannelRemovedEvent) EventMessage() string {
	return fmt.Sprintf("Notification channel removed: %s (%s)", e.Name, e.ChannelType)
}

// NotificationSentEvent is published when a notification is successfully delivered.
type NotificationSentEvent struct {
	ChannelID   uint   `json:"channelId"`
	ChannelType string `json:"channelType"`
	Name        string `json:"name"`
	TriggerType string `json:"triggerType"` // The event type that triggered the notification
}

// EventType implements Event.
func (e NotificationSentEvent) EventType() string { return "notification_sent" }

// EventMessage implements Event.
func (e NotificationSentEvent) EventMessage() string {
	return fmt.Sprintf("Notification sent via %s (%s)", e.Name, e.ChannelType)
}

// NotificationDeliveryFailedEvent is published when a notification delivery fails.
type NotificationDeliveryFailedEvent struct {
	ChannelID   uint   `json:"channelId"`
	ChannelType string `json:"channelType"`
	Name        string `json:"name"`
	Error       string `json:"error"`
}

// EventType implements Event.
func (e NotificationDeliveryFailedEvent) EventType() string { return "notification_delivery_failed" }

// EventMessage implements Event.
func (e NotificationDeliveryFailedEvent) EventMessage() string {
	return fmt.Sprintf("Notification delivery failed: %s (%s) — %s", e.Name, e.ChannelType, e.Error)
}

// =============================================================================
// Preview Events
// =============================================================================

// PreviewUpdatedEvent is published when the preview cache is populated with
// fresh data (after a poller cycle or a force-refresh computation).
type PreviewUpdatedEvent struct {
	ItemCount int       `json:"itemCount"`
	Timestamp time.Time `json:"timestamp"`
}

// EventType implements Event.
func (e PreviewUpdatedEvent) EventType() string { return "preview_updated" }

// EventMessage implements Event.
func (e PreviewUpdatedEvent) EventMessage() string {
	return fmt.Sprintf("Preview updated: %d items scored", e.ItemCount)
}

// AnalyticsUpdatedEvent is published alongside PreviewUpdatedEvent to signal
// that analytics data (composition, quality, watch intelligence) should be
// refetched by the frontend. The analytics APIs aggregate from the preview
// cache, so they're only valid after a cache refresh.
type AnalyticsUpdatedEvent struct {
	ItemCount int       `json:"itemCount"`
	Timestamp time.Time `json:"timestamp"`
}

// EventType implements Event.
func (e AnalyticsUpdatedEvent) EventType() string { return "analytics_updated" }

// EventMessage implements Event.
func (e AnalyticsUpdatedEvent) EventMessage() string {
	return fmt.Sprintf("Analytics updated: %d items available", e.ItemCount)
}

// PreviewInvalidatedEvent is published when the preview cache is cleared due
// to a configuration change that affects scoring (rules, settings,
// integrations, thresholds). Connected clients should show a stale indicator
// and fetch fresh data.
type PreviewInvalidatedEvent struct {
	Reason string `json:"reason"` // e.g. "rule_changed", "settings_changed"
}

// EventType implements Event.
func (e PreviewInvalidatedEvent) EventType() string { return "preview_invalidated" }

// EventMessage implements Event.
func (e PreviewInvalidatedEvent) EventMessage() string {
	return fmt.Sprintf("Preview cache invalidated: %s", e.Reason)
}

// =============================================================================
// Data Events
// =============================================================================

// DataResetEvent is published when all scraped data is cleared.
type DataResetEvent struct {
	Summary map[string]int64 `json:"summary"` // e.g. {"audit_log": 42, "approval_queue": 5}
}

// EventType implements Event.
func (e DataResetEvent) EventType() string { return "data_reset" }

// EventMessage implements Event.
func (e DataResetEvent) EventMessage() string { return "All scraped data has been reset" }

// =============================================================================
// System Events
// =============================================================================

// ServerStartedEvent is published when the application starts.
type ServerStartedEvent struct {
	Version string `json:"version"`
}

// EventType implements Event.
func (e ServerStartedEvent) EventType() string { return "server_started" }

// EventMessage implements Event.
func (e ServerStartedEvent) EventMessage() string {
	if e.Version != "" {
		return fmt.Sprintf("Server started (version %s)", e.Version)
	}
	return "Server started"
}

// =============================================================================
// Sunset Events
// =============================================================================

// SunsetCreatedEvent is published when an item is added to the sunset queue.
type SunsetCreatedEvent struct {
	MediaName     string `json:"mediaName"`
	MediaType     string `json:"mediaType"`
	DiskGroupID   uint   `json:"diskGroupId"`
	DaysRemaining int    `json:"daysRemaining"`
	DeletionDate  string `json:"deletionDate"`
}

// EventType implements Event.
func (e SunsetCreatedEvent) EventType() string { return "sunset_created" }

// EventMessage implements Event.
func (e SunsetCreatedEvent) EventMessage() string {
	return fmt.Sprintf("%s added to sunset queue — leaving in %d days", e.MediaName, e.DaysRemaining)
}

// SunsetCancelledEvent is published when a sunset item is cancelled (removed from queue).
type SunsetCancelledEvent struct {
	MediaName   string `json:"mediaName"`
	MediaType   string `json:"mediaType"`
	DiskGroupID uint   `json:"diskGroupId"`
}

// EventType implements Event.
func (e SunsetCancelledEvent) EventType() string { return "sunset_cancelled" }

// EventMessage implements Event.
func (e SunsetCancelledEvent) EventMessage() string {
	return fmt.Sprintf("%s removed from sunset queue", e.MediaName)
}

// SunsetExpiredEvent is published when a sunset countdown expires and the item
// is handed to DeletionService for actual removal.
type SunsetExpiredEvent struct {
	MediaName   string `json:"mediaName"`
	MediaType   string `json:"mediaType"`
	DiskGroupID uint   `json:"diskGroupId"`
	SizeBytes   int64  `json:"sizeBytes"`
}

// EventType implements Event.
func (e SunsetExpiredEvent) EventType() string { return "sunset_expired" }

// EventMessage implements Event.
func (e SunsetExpiredEvent) EventMessage() string {
	return fmt.Sprintf("%s sunset countdown expired — queued for deletion", e.MediaName)
}

// SunsetRescheduledEvent is published when a sunset item's deletion date is changed.
type SunsetRescheduledEvent struct {
	MediaName        string `json:"mediaName"`
	MediaType        string `json:"mediaType"`
	DiskGroupID      uint   `json:"diskGroupId"`
	NewDaysRemaining int    `json:"newDaysRemaining"`
	NewDeletionDate  string `json:"newDeletionDate"`
}

// EventType implements Event.
func (e SunsetRescheduledEvent) EventType() string { return "sunset_rescheduled" }

// EventMessage implements Event.
func (e SunsetRescheduledEvent) EventMessage() string {
	return fmt.Sprintf("%s rescheduled — now leaving in %d days", e.MediaName, e.NewDaysRemaining)
}

// SunsetEscalatedEvent is published when a sunset-mode disk group breaches
// thresholdPct and items are force-expired to free space down to targetPct.
type SunsetEscalatedEvent struct {
	DiskGroupID  uint  `json:"diskGroupId"`
	ItemsExpired int   `json:"itemsExpired"`
	BytesFreed   int64 `json:"bytesFreed"`
}

// EventType implements Event.
func (e SunsetEscalatedEvent) EventType() string { return "sunset_escalated" }

// EventMessage implements Event.
func (e SunsetEscalatedEvent) EventMessage() string {
	return fmt.Sprintf("Sunset escalation: %d items force-expired to free space", e.ItemsExpired)
}

// SunsetMisconfiguredEvent is published when the engine skips a sunset-mode
// disk group because sunsetPct is NULL (not yet configured by the user).
type SunsetMisconfiguredEvent struct {
	DiskGroupID uint   `json:"diskGroupId"`
	MountPath   string `json:"mountPath"`
}

// EventType implements Event.
func (e SunsetMisconfiguredEvent) EventType() string { return "sunset_misconfigured" }

// EventMessage implements Event.
func (e SunsetMisconfiguredEvent) EventMessage() string {
	return fmt.Sprintf("Sunset mode skipped for %s — sunset threshold not configured", e.MountPath)
}

// SunsetLabelAppliedEvent is published when the sunset label is applied to an
// item in a media server.
type SunsetLabelAppliedEvent struct {
	MediaName     string `json:"mediaName"`
	IntegrationID uint   `json:"integrationId"`
	Label         string `json:"label"`
}

// EventType implements Event.
func (e SunsetLabelAppliedEvent) EventType() string { return "sunset_label_applied" }

// EventMessage implements Event.
func (e SunsetLabelAppliedEvent) EventMessage() string {
	return fmt.Sprintf("Label %q applied to %s", e.Label, e.MediaName)
}

// SunsetLabelRemovedEvent is published when the sunset label is removed from an
// item in a media server.
type SunsetLabelRemovedEvent struct {
	MediaName     string `json:"mediaName"`
	IntegrationID uint   `json:"integrationId"`
	Label         string `json:"label"`
}

// EventType implements Event.
func (e SunsetLabelRemovedEvent) EventType() string { return "sunset_label_removed" }

// EventMessage implements Event.
func (e SunsetLabelRemovedEvent) EventMessage() string {
	return fmt.Sprintf("Label %q removed from %s", e.Label, e.MediaName)
}

// SunsetLabelFailedEvent is published when a label operation fails on a media server.
type SunsetLabelFailedEvent struct {
	MediaName     string `json:"mediaName"`
	IntegrationID uint   `json:"integrationId"`
	Label         string `json:"label"`
	Error         string `json:"error"`
}

// EventType implements Event.
func (e SunsetLabelFailedEvent) EventType() string { return "sunset_label_failed" }

// EventMessage implements Event.
func (e SunsetLabelFailedEvent) EventMessage() string {
	return fmt.Sprintf("Failed to apply label %q to %s: %s", e.Label, e.MediaName, e.Error)
}

// SunsetSavedEvent is published when a sunset item is saved due to a score drop
// during the daily rescore check ("saved by popular demand").
type SunsetSavedEvent struct {
	MediaName     string  `json:"mediaName"`
	MediaType     string  `json:"mediaType"`
	DiskGroupID   uint    `json:"diskGroupId"`
	OriginalScore float64 `json:"originalScore"`
	NewScore      float64 `json:"newScore"`
}

// EventType implements Event.
func (e SunsetSavedEvent) EventType() string { return "sunset_saved" }

// EventMessage implements Event.
func (e SunsetSavedEvent) EventMessage() string {
	return fmt.Sprintf("%s saved by popular demand — score dropped from %.1f to %.1f", e.MediaName, e.OriginalScore, e.NewScore)
}

// SunsetSavedCleanedEvent is published when a saved item's marker duration expires
// and it is fully removed from the queue.
type SunsetSavedCleanedEvent struct {
	MediaName   string `json:"mediaName"`
	MediaType   string `json:"mediaType"`
	DiskGroupID uint   `json:"diskGroupId"`
}

// EventType implements Event.
func (e SunsetSavedCleanedEvent) EventType() string { return "sunset_saved_cleaned" }

// EventMessage implements Event.
func (e SunsetSavedCleanedEvent) EventMessage() string {
	return fmt.Sprintf("%s saved marker removed — fully restored", e.MediaName)
}

// =============================================================================
// Poster Overlay Events
// =============================================================================

// PosterOverlayAppliedEvent is published when an overlay poster is uploaded
// to a media server.
type PosterOverlayAppliedEvent struct {
	MediaName     string `json:"mediaName"`
	IntegrationID uint   `json:"integrationId"`
	DaysRemaining int    `json:"daysRemaining"`
}

// EventType implements Event.
func (e PosterOverlayAppliedEvent) EventType() string { return "poster_overlay_applied" }

// EventMessage implements Event.
func (e PosterOverlayAppliedEvent) EventMessage() string {
	return fmt.Sprintf("Poster overlay applied to %s — leaving in %d days", e.MediaName, e.DaysRemaining)
}

// PosterOverlayRestoredEvent is published when an original poster is restored
// on a media server (after cancel, expiry, or escalation).
type PosterOverlayRestoredEvent struct {
	MediaName     string `json:"mediaName"`
	IntegrationID uint   `json:"integrationId"`
}

// EventType implements Event.
func (e PosterOverlayRestoredEvent) EventType() string { return "poster_overlay_restored" }

// EventMessage implements Event.
func (e PosterOverlayRestoredEvent) EventMessage() string {
	return fmt.Sprintf("Original poster restored for %s", e.MediaName)
}

// PosterOverlayFailedEvent is published when a poster overlay operation fails.
type PosterOverlayFailedEvent struct {
	MediaName     string `json:"mediaName"`
	IntegrationID uint   `json:"integrationId"`
	Error         string `json:"error"`
}

// EventType implements Event.
func (e PosterOverlayFailedEvent) EventType() string { return "poster_overlay_failed" }

// EventMessage implements Event.
func (e PosterOverlayFailedEvent) EventMessage() string {
	return fmt.Sprintf("Poster overlay failed for %s: %s", e.MediaName, e.Error)
}

// =============================================================================
// Database Backup Events
// =============================================================================

// DatabaseBackupCompletedEvent is published when a scheduled database backup completes successfully.
type DatabaseBackupCompletedEvent struct {
	Path            string `json:"path"`
	SizeBytes       int64  `json:"sizeBytes"`
	BackupsRetained int    `json:"backupsRetained"`
}

// EventType implements Event.
func (e DatabaseBackupCompletedEvent) EventType() string { return "database_backup_completed" }

// EventMessage implements Event.
func (e DatabaseBackupCompletedEvent) EventMessage() string {
	return fmt.Sprintf("Database backup completed: %s (%d bytes, %d backups retained)", e.Path, e.SizeBytes, e.BackupsRetained)
}

// DatabaseBackupFailedEvent is published when a scheduled database backup fails.
type DatabaseBackupFailedEvent struct {
	Error string `json:"error"`
}

// EventType implements Event.
func (e DatabaseBackupFailedEvent) EventType() string { return "database_backup_failed" }

// EventMessage implements Event.
func (e DatabaseBackupFailedEvent) EventMessage() string {
	return fmt.Sprintf("Database backup failed: %s", e.Error)
}
