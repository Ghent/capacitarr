/**
 * Deletion queue composable — shared state for the deletion queue card.
 *
 * Fetches the list of queued deletion items from the API, provides a cancel
 * function, and tracks completed items from the current batch via SSE events.
 * State is stored via useState so it persists across page navigations and is
 * shared between components on the same page.
 */
import { toast } from 'vue-sonner';
import type { DeletionQueueItem, DeletionCompletedItem } from '~/types/api';
import {
  EVENT_DELETION_QUEUED,
  EVENT_DELETION_PROGRESS,
  EVENT_DELETION_SUCCESS,
  EVENT_DELETION_FAILED,
  EVENT_DELETION_CANCELLED,
  EVENT_DELETION_GRACE_PERIOD,
  EVENT_DELETION_BATCH_COMPLETE,
  EVENT_DELETION_QUEUE_FULL,
} from '~/constants';
import { useFetchStatus } from './useFetchStatus';

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

export interface GracePeriodState {
  active: boolean;
  remainingSeconds: number;
  queueSize: number;
}

export function useDeletionQueue() {
  const api = useApi();
  const { on } = useEventStream();
  const { t } = useI18n();
  const { lastFetchOk, loadError, markSuccess, markFailure } = useFetchStatus();

  const queuedItems = useState<DeletionQueueItem[]>('deletionQueueItems', () => []);
  const completedItems = useState<DeletionCompletedItem[]>('deletionCompletedItems', () => []);
  const loading = ref(false);

  // Grace period state — updated via SSE and local countdown
  const gracePeriod = useState<GracePeriodState>('deletionGracePeriod', () => ({
    active: false,
    remainingSeconds: 0,
    queueSize: 0,
  }));
  const countdown = ref(0);
  let countdownTimer: ReturnType<typeof setInterval> | null = null;

  function startCountdown(seconds: number) {
    stopCountdown();
    countdown.value = seconds;
    countdownTimer = setInterval(() => {
      countdown.value = Math.max(0, countdown.value - 1);
      if (countdown.value <= 0) {
        stopCountdown();
      }
    }, 1000);
  }

  function stopCountdown() {
    if (countdownTimer) {
      clearInterval(countdownTimer);
      countdownTimer = null;
    }
    countdown.value = 0;
  }

  async function fetchQueue() {
    try {
      const data = await api.GET('/deletion-queue');
      queuedItems.value = data ?? [];
      markSuccess();
    } catch {
      markFailure();
    }
  }

  async function cancelItem(mediaName: string, mediaType: string) {
    try {
      await api.DELETE('/deletion-queue', { query: { mediaName, mediaType } });
      // Optimistically remove from local list
      queuedItems.value = queuedItems.value.filter(
        (item) => !(item.mediaName === mediaName && item.mediaType === mediaType),
      );
    } catch {
      // Refresh to get accurate state
      await fetchQueue();
    }
  }

  async function snoozeItem(mediaName: string, mediaType: string) {
    try {
      await api.POST('/deletion-queue/snooze', { body: { mediaName, mediaType } });
      // Optimistically remove from local list
      queuedItems.value = queuedItems.value.filter(
        (item) => !(item.mediaName === mediaName && item.mediaType === mediaType),
      );
    } catch {
      await fetchQueue();
    }
  }

  async function clearAll() {
    try {
      await api.POST('/deletion-queue/clear');
      queuedItems.value = [];
      stopCountdown();
    } catch {
      await fetchQueue();
    }
  }

  // SSE subscriptions (register once)
  if (!_sseRegistered) {
    _sseRegistered = true;

    on(EVENT_DELETION_QUEUED, () => {
      // New item entered the deletion queue (e.g. approved in approval mode) — refresh
      fetchQueue();
    });

    on(EVENT_DELETION_PROGRESS, () => {
      // Queue shrinks as items are processed — refresh
      fetchQueue();
    });

    on(EVENT_DELETION_SUCCESS, (raw: unknown) => {
      const data = raw as { mediaName: string; mediaType: string; sizeBytes: number };
      completedItems.value.push({
        mediaName: data.mediaName,
        mediaType: data.mediaType,
        sizeBytes: data.sizeBytes,
        status: 'success',
        timestamp: new Date().toISOString(),
      });
    });

    on(EVENT_DELETION_FAILED, (raw: unknown) => {
      const data = raw as { mediaName: string; mediaType: string };
      completedItems.value.push({
        mediaName: data.mediaName,
        mediaType: data.mediaType,
        sizeBytes: 0,
        status: 'failed',
        timestamp: new Date().toISOString(),
      });
    });

    on(EVENT_DELETION_CANCELLED, (raw: unknown) => {
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

    on(EVENT_DELETION_BATCH_COMPLETE, () => {
      // Clear completed items and queue after batch finishes
      completedItems.value = [];
      queuedItems.value = [];
      stopCountdown();
    });

    on(EVENT_DELETION_GRACE_PERIOD, (raw: unknown) => {
      const data = raw as GracePeriodState;
      gracePeriod.value = { ...data };
      if (data.active) {
        startCountdown(data.remainingSeconds);
      } else {
        stopCountdown();
      }
    });

    on(EVENT_DELETION_QUEUE_FULL, () => {
      toast.warning(t('deletion.queueFull'));
      fetchQueue();
    });
  }

  if (import.meta.hot) {
    import.meta.hot.dispose(() => {
      _sseRegistered = false;
    });
  }

  return {
    queuedItems: readonly(queuedItems),
    completedItems: readonly(completedItems),
    gracePeriod: readonly(gracePeriod),
    countdown: readonly(countdown),
    loading,
    lastFetchOk,
    loadError,
    fetchQueue,
    cancelItem,
    snoozeItem,
    clearAll,
  };
}
