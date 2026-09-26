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
export const EVENT_DELETION_QUEUE_FULL = 'deletion_queue_full' as const;

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

// Auth / settings / rules / notifications (activity feed)
export const EVENT_MANUAL_RUN_TRIGGERED = 'manual_run_triggered' as const;
export const EVENT_THRESHOLD_CHANGED = 'threshold_changed' as const;
export const EVENT_LOGIN = 'login' as const;
export const EVENT_PASSWORD_CHANGED = 'password_changed' as const;
export const EVENT_USERNAME_CHANGED = 'username_changed' as const;
export const EVENT_API_KEY_GENERATED = 'api_key_generated' as const;
export const EVENT_INTEGRATION_TEST = 'integration_test' as const;
export const EVENT_INTEGRATION_TEST_FAILED = 'integration_test_failed' as const;
export const EVENT_THRESHOLD_BREACHED = 'threshold_breached' as const;
export const EVENT_UPDATE_AVAILABLE = 'update_available' as const;
export const EVENT_RULE_CREATED = 'rule_created' as const;
export const EVENT_RULE_UPDATED = 'rule_updated' as const;
export const EVENT_RULE_DELETED = 'rule_deleted' as const;
export const EVENT_NOTIFICATION_CHANNEL_ADDED = 'notification_channel_added' as const;
export const EVENT_NOTIFICATION_CHANNEL_UPDATED = 'notification_channel_updated' as const;
export const EVENT_NOTIFICATION_CHANNEL_REMOVED = 'notification_channel_removed' as const;
export const EVENT_NOTIFICATION_SENT = 'notification_sent' as const;
export const EVENT_NOTIFICATION_DELIVERY_FAILED = 'notification_delivery_failed' as const;
export const EVENT_SERVER_STARTED = 'server_started' as const;
export const EVENT_REPLAY_GAP = 'replay_gap' as const;

/** Event types prepended to the dashboard activity feed. */
export const ACTIVITY_FEED_EVENT_TYPES = [
  EVENT_ENGINE_START,
  EVENT_ENGINE_COMPLETE,
  EVENT_ENGINE_ERROR,
  EVENT_MANUAL_RUN_TRIGGERED,
  EVENT_SETTINGS_CHANGED,
  EVENT_THRESHOLD_CHANGED,
  EVENT_LOGIN,
  EVENT_PASSWORD_CHANGED,
  EVENT_USERNAME_CHANGED,
  EVENT_API_KEY_GENERATED,
  EVENT_INTEGRATION_ADDED,
  EVENT_INTEGRATION_UPDATED,
  EVENT_INTEGRATION_REMOVED,
  EVENT_INTEGRATION_TEST,
  EVENT_INTEGRATION_TEST_FAILED,
  EVENT_INTEGRATION_RECOVERED,
  EVENT_INTEGRATION_RECOVERY_ATTEMPT,
  EVENT_APPROVAL_APPROVED,
  EVENT_APPROVAL_REJECTED,
  EVENT_APPROVAL_UNSNOOZED,
  EVENT_APPROVAL_BULK_UNSNOOZED,
  EVENT_APPROVAL_ORPHANS_RECOVERED,
  EVENT_APPROVAL_RETURNED_TO_PENDING,
  EVENT_DELETION_QUEUED,
  EVENT_DELETION_SUCCESS,
  EVENT_DELETION_FAILED,
  EVENT_DELETION_DRY_RUN,
  EVENT_DELETION_BATCH_COMPLETE,
  EVENT_DELETION_PROGRESS,
  EVENT_THRESHOLD_BREACHED,
  EVENT_UPDATE_AVAILABLE,
  EVENT_RULE_CREATED,
  EVENT_RULE_UPDATED,
  EVENT_RULE_DELETED,
  EVENT_NOTIFICATION_CHANNEL_ADDED,
  EVENT_NOTIFICATION_CHANNEL_UPDATED,
  EVENT_NOTIFICATION_CHANNEL_REMOVED,
  EVENT_NOTIFICATION_SENT,
  EVENT_NOTIFICATION_DELIVERY_FAILED,
  EVENT_DATA_RESET,
  EVENT_SETTINGS_IMPORTED,
  EVENT_SERVER_STARTED,
] as const;
