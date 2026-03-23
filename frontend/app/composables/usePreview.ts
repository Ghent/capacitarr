import type { EvaluatedItem, DiskContext, DeletionProgress, PreviewResponse } from '~/types/api';
import {
  EVENT_DELETION_SUCCESS,
  EVENT_DELETION_DRY_RUN,
  EVENT_DELETION_PROGRESS,
  EVENT_DELETION_BATCH_COMPLETE,
} from '~/constants';

/**
 * usePreview — SSE-reactive preview data composable.
 *
 * Fetches scored media items from `/api/v1/preview` and listens for
 * `preview_updated` and `preview_invalidated` SSE events to auto-refresh.
 *
 * Shared by library.vue (Library Management page) and rules.vue (Scoring
 * Engine live preview).
 */
export function usePreview() {
  const api = useApi();
  const { on, off } = useEventStream();

  const items = ref<EvaluatedItem[]>([]);
  const diskContext = ref<DiskContext | null>(null);
  const loading = ref(false);
  const stale = ref(false);

  // ---------------------------------------------------------------------------
  // Data fetching
  // ---------------------------------------------------------------------------

  async function refresh(force = false): Promise<void> {
    loading.value = true;
    try {
      const url = force ? '/api/v1/preview?force=true' : '/api/v1/preview';
      const data = (await api(url)) as PreviewResponse;
      items.value = data?.items ?? [];
      diskContext.value = data?.diskContext ?? null;
      stale.value = false;
    } catch (err) {
      console.warn('[usePreview] fetch failed:', err);
      items.value = [];
      diskContext.value = null;
    } finally {
      loading.value = false;
    }
  }

  // ---------------------------------------------------------------------------
  // SSE handlers
  // ---------------------------------------------------------------------------

  function handlePreviewUpdated() {
    // Cache has fresh data — fetch it (should be instant from cache)
    refresh();
  }

  function handlePreviewInvalidated() {
    // Cache was cleared due to config change — mark stale and request fresh data
    stale.value = true;
    refresh(true);
  }

  /**
   * Remove a successfully deleted item from the local list so the Library page
   * and Deletion Priority card update in real-time without waiting for the next
   * engine cycle.
   */
  function handleDeletionSuccess(data: unknown) {
    const event = data as { mediaName: string; mediaType: string };
    items.value = items.value.filter(
      (item) => !(item.item.title === event.mediaName && item.item.type === event.mediaType),
    );
  }

  /**
   * Remove dry-deleted items from the local list for consistency — they've been
   * "processed" even if not actually deleted. The audit log is the authoritative
   * record of dry-deletions.
   */
  function handleDeletionDryRun(data: unknown) {
    const event = data as { mediaName: string; mediaType: string };
    items.value = items.value.filter(
      (item) => !(item.item.title === event.mediaName && item.item.type === event.mediaType),
    );
  }

  /**
   * When a deletion_progress event arrives, mark the currently-deleting item
   * so the UI shows a real-time "Deleting..." badge. Clear the status for any
   * item that was previously marked as deleting but is no longer the current item.
   */
  function handleDeletionProgress(data: unknown) {
    const event = data as DeletionProgress;
    const currentItem = event.currentItem;

    for (const entry of items.value) {
      if (entry.item.title === currentItem) {
        entry.queueStatus = 'deleting';
      } else if (entry.queueStatus === 'deleting') {
        // Clear stale deleting status — item was processed
        entry.queueStatus = undefined;
      }
    }
  }

  /**
   * After all deletions in a cycle are processed, reconcile with the server to
   * ensure the local list matches the authoritative backend state.
   */
  function handleDeletionBatchComplete() {
    refresh(true);
  }

  // ---------------------------------------------------------------------------
  // Lifecycle
  // ---------------------------------------------------------------------------

  onMounted(() => {
    on('preview_updated', handlePreviewUpdated);
    on('preview_invalidated', handlePreviewInvalidated);
    on(EVENT_DELETION_SUCCESS, handleDeletionSuccess);
    on(EVENT_DELETION_DRY_RUN, handleDeletionDryRun);
    on(EVENT_DELETION_PROGRESS, handleDeletionProgress);
    on(EVENT_DELETION_BATCH_COMPLETE, handleDeletionBatchComplete);
  });

  onUnmounted(() => {
    off('preview_updated', handlePreviewUpdated);
    off('preview_invalidated', handlePreviewInvalidated);
    off(EVENT_DELETION_SUCCESS, handleDeletionSuccess);
    off(EVENT_DELETION_DRY_RUN, handleDeletionDryRun);
    off(EVENT_DELETION_PROGRESS, handleDeletionProgress);
    off(EVENT_DELETION_BATCH_COMPLETE, handleDeletionBatchComplete);
  });

  return {
    /** Evaluated items from the preview cache. Mutable ref for component compatibility. */
    items,
    /** Disk context from the preview cache. */
    diskContext: readonly(diskContext),
    /** Whether a fetch is in progress. */
    loading: readonly(loading),
    /** Whether the cached data is stale (invalidated, awaiting refresh). */
    stale: readonly(stale),
    refresh,
  };
}
