// Shared constants for the Capacitarr frontend.
// These mirror the backend constants in internal/db/models.go and must be kept in sync.

// Execution modes — used in DiskGroup.mode and PreferenceSet.defaultDiskGroupMode.
export const MODE_AUTO = 'auto' as const;
export const MODE_DRY_RUN = 'dry-run' as const;
export const MODE_APPROVAL = 'approval' as const;
export const MODE_SUNSET = 'sunset' as const;

// Tiebreaker methods — used in PreferenceSet.tiebreakerMethod field.
export const TIEBREAKER_SIZE_DESC = 'size_desc' as const;

// ---------------------------------------------------------------------------
// SSE event types — centralized to prevent typo-induced silent failures.
// Must stay in sync with backend/internal/events/types.go.
// ---------------------------------------------------------------------------

// Deletion events
export const EVENT_DELETION_SUCCESS = 'deletion_success' as const;
export const EVENT_DELETION_DRY_RUN = 'deletion_dry_run' as const;
export const EVENT_DELETION_PROGRESS = 'deletion_progress' as const;
export const EVENT_DELETION_QUEUED = 'deletion_queued' as const;
export const EVENT_DELETION_FAILED = 'deletion_failed' as const;
export const EVENT_DELETION_CANCELLED = 'deletion_cancelled' as const;
export const EVENT_DELETION_BATCH_COMPLETE = 'deletion_batch_complete' as const;
export const EVENT_DELETION_GRACE_PERIOD = 'deletion_grace_period' as const;

// Approval events
export const EVENT_APPROVAL_APPROVED = 'approval_approved' as const;
export const EVENT_APPROVAL_REJECTED = 'approval_rejected' as const;
export const EVENT_APPROVAL_DISMISSED = 'approval_dismissed' as const;
export const EVENT_APPROVAL_UNSNOOZED = 'approval_unsnoozed' as const;
export const EVENT_APPROVAL_BULK_UNSNOOZED = 'approval_bulk_unsnoozed' as const;
export const EVENT_APPROVAL_QUEUE_CLEARED = 'approval_queue_cleared' as const;
export const EVENT_APPROVAL_ORPHANS_RECOVERED = 'approval_orphans_recovered' as const;
export const EVENT_APPROVAL_RETURNED_TO_PENDING = 'approval_returned_to_pending' as const;

// Engine events
export const EVENT_ENGINE_START = 'engine_start' as const;
export const EVENT_ENGINE_COMPLETE = 'engine_complete' as const;
export const EVENT_ENGINE_ERROR = 'engine_error' as const;

// Integration events
export const EVENT_INTEGRATION_ADDED = 'integration_added' as const;
export const EVENT_INTEGRATION_UPDATED = 'integration_updated' as const;
export const EVENT_INTEGRATION_REMOVED = 'integration_removed' as const;
export const EVENT_INTEGRATION_RECOVERED = 'integration_recovered' as const;
export const EVENT_INTEGRATION_RECOVERY_ATTEMPT = 'integration_recovery_attempt' as const;

// Settings / system events
export const EVENT_SETTINGS_CHANGED = 'settings_changed' as const;
export const EVENT_SETTINGS_IMPORTED = 'settings_imported' as const;
export const EVENT_DATA_RESET = 'data_reset' as const;
export const EVENT_ANALYTICS_UPDATED = 'analytics_updated' as const;
export const EVENT_PREVIEW_UPDATED = 'preview_updated' as const;
export const EVENT_PREVIEW_INVALIDATED = 'preview_invalidated' as const;

// Sunset events
export const EVENT_SUNSET_CREATED = 'sunset_created' as const;
export const EVENT_SUNSET_ESCALATED = 'sunset_escalated' as const;
export const EVENT_SUNSET_EXPIRED = 'sunset_expired' as const;
export const EVENT_SUNSET_SAVED = 'sunset_saved' as const;
export const EVENT_SUNSET_CANCELLED = 'sunset_cancelled' as const;
export const EVENT_SUNSET_RESCHEDULED = 'sunset_rescheduled' as const;
export const EVENT_SUNSET_SAVED_CLEANED = 'sunset_saved_cleaned' as const;
