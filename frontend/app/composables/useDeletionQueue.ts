/**
 * Deletion queue composable — shared state for the deletion queue card.
 *
 * Fetches the list of queued deletion items from the API, provides a cancel
 * function, and tracks completed items from the current batch via SSE events.
 * State is stored via useState so it persists across page navigations and is
 * shared between components on the same page.
 */
import type { DeletionQueueItem, DeletionCompletedItem } from '~/types/api';

// Module-level flag: SSE handlers are registered once globally.
let _sseRegistered = false;

/**
 * Reset the SSE registration flag. Used only in tests to allow fresh
 * handler registration after state is cleared between test cases.
 * @internal
 */
export function _resetDeletionQueueSSE() {
  _sseRegistered = false;
}

export function useDeletionQueue() {
  const api = useApi();
  const { on } = useEventStream();

  const queuedItems = useState<DeletionQueueItem[]>('deletionQueueItems', () => []);
  const completedItems = useState<DeletionCompletedItem[]>('deletionCompletedItems', () => []);
  const loading = ref(false);

  async function fetchQueue() {
    try {
      const data = await api<DeletionQueueItem[]>('/api/deletion-queue');
      queuedItems.value = data ?? [];
    } catch {
      queuedItems.value = [];
    }
  }

  async function cancelItem(mediaName: string, mediaType: string) {
    try {
      await api(
        `/api/deletion-queue?mediaName=${encodeURIComponent(mediaName)}&mediaType=${encodeURIComponent(mediaType)}`,
        {
          method: 'DELETE',
        },
      );
      // Optimistically remove from local list
      queuedItems.value = queuedItems.value.filter(
        (item) => !(item.mediaName === mediaName && item.mediaType === mediaType),
      );
    } catch {
      // Refresh to get accurate state
      await fetchQueue();
    }
  }

  // SSE subscriptions (register once)
  if (!_sseRegistered) {
    _sseRegistered = true;

    on('deletion_progress', () => {
      // Queue shrinks as items are processed — refresh
      fetchQueue();
    });

    on('deletion_success', (raw: unknown) => {
      const data = raw as { mediaName: string; mediaType: string; sizeBytes: number };
      completedItems.value.push({
        mediaName: data.mediaName,
        mediaType: data.mediaType,
        sizeBytes: data.sizeBytes,
        status: 'success',
        timestamp: new Date().toISOString(),
      });
    });

    on('deletion_failed', (raw: unknown) => {
      const data = raw as { mediaName: string; mediaType: string };
      completedItems.value.push({
        mediaName: data.mediaName,
        mediaType: data.mediaType,
        sizeBytes: 0,
        status: 'failed',
        timestamp: new Date().toISOString(),
      });
    });

    on('deletion_cancelled', (raw: unknown) => {
      const data = raw as { mediaName: string; mediaType: string; sizeBytes: number };
      completedItems.value.push({
        mediaName: data.mediaName,
        mediaType: data.mediaType,
        sizeBytes: data.sizeBytes,
        status: 'cancelled',
        timestamp: new Date().toISOString(),
      });
      // Also remove from queued items
      queuedItems.value = queuedItems.value.filter(
        (item) => !(item.mediaName === data.mediaName && item.mediaType === data.mediaType),
      );
    });

    on('deletion_batch_complete', () => {
      // Clear completed items and queue after batch finishes
      completedItems.value = [];
      queuedItems.value = [];
    });
  }

  return {
    queuedItems: readonly(queuedItems),
    completedItems: readonly(completedItems),
    loading,
    fetchQueue,
    cancelItem,
  };
}
