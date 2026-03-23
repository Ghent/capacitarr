// Shared constants for the Capacitarr frontend.
// These mirror the backend constants in internal/db/models.go and must be kept in sync.

// Execution modes — used in PreferenceSet.executionMode field.
export const MODE_AUTO = 'auto' as const;
export const MODE_DRY_RUN = 'dry-run' as const;
export const MODE_APPROVAL = 'approval' as const;

// Tiebreaker methods — used in PreferenceSet.tiebreakerMethod field.
export const TIEBREAKER_SIZE_DESC = 'size_desc' as const;
export const TIEBREAKER_SIZE_ASC = 'size_asc' as const;
export const TIEBREAKER_NAME_ASC = 'name_asc' as const;
export const TIEBREAKER_OLDEST_FIRST = 'oldest_first' as const;
export const TIEBREAKER_NEWEST_FIRST = 'newest_first' as const;

// SSE event types related to deletion.
export const EVENT_DELETION_SUCCESS = 'deletion_success' as const;
export const EVENT_DELETION_DRY_RUN = 'deletion_dry_run' as const;
export const EVENT_DELETION_PROGRESS = 'deletion_progress' as const;
export const EVENT_DELETION_QUEUED = 'deletion_queued' as const;
export const EVENT_DELETION_FAILED = 'deletion_failed' as const;
export const EVENT_DELETION_BATCH_COMPLETE = 'deletion_batch_complete' as const;
