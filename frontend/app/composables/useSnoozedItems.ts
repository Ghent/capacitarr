/**
 * Snoozed items composable — shared state for the snoozed items card.
 *
 * Fetches snoozed items from the approval queue API (status=rejected with
 * active snoozedUntil), provides an unsnooze action, and subscribes to SSE
 * events for real-time updates. Visible in ALL execution modes, not just
 * approval mode.
 *
 * State is stored via useState so it persists across page navigations and
 * is shared between components on the same page.
 */
import type { ApprovalQueueItem } from '~/types/api';
import {
  EVENT_APPROVAL_REJECTED,
  EVENT_APPROVAL_UNSNOOZED,
  EVENT_APPROVAL_BULK_UNSNOOZED,
  EVENT_APPROVAL_QUEUE_CLEARED,
  EVENT_APPROVAL_DISMISSED,
  EVENT_ENGINE_COMPLETE,
} from '~/constants';
import { useFetchStatus } from './useFetchStatus';

export interface SnoozedItem {
  id: number;
  mediaName: string;
  mediaType: string;
  sizeBytes: number;
  snoozedUntil: string;
  posterUrl?: string;
  score: number;
}

// Module-level flag: SSE handlers are registered once globally.
let _snoozedSseRegistered = false;

/**
 * Reset the SSE registration flag. Used only in tests to allow fresh
 * handler registration after state is cleared between test cases.
 * @internal
 */
export function _resetSnoozedItemsSSE() {
  _snoozedSseRegistered = false;
}

export function useSnoozedItems() {
  const api = useApi();
  const { on } = useEventStream();
  const { runCompletionCounter } = useEngineControl();

  const snoozedItems = useState<SnoozedItem[]>('snoozedItems', () => []);
  const loading = ref(false);
  const { lastFetchOk, loadError, markSuccess, markFailure } = useFetchStatus();

  async function fetchSnoozedItems() {
    loading.value = true;
    try {
      const allRejected = (await api(
        '/api/v1/approval-queue?status=rejected&limit=1000',
      )) as ApprovalQueueItem[];

      const now = new Date();
      snoozedItems.value = allRejected
        .filter((item) => item.snoozedUntil && new Date(item.snoozedUntil) > now)
        .map((item) => ({
          id: item.id,
          mediaName: item.mediaName,
          mediaType: item.mediaType,
          sizeBytes: item.sizeBytes,
          snoozedUntil: item.snoozedUntil!,
          posterUrl: item.posterUrl,
          score: item.score,
        }));
      markSuccess();
    } catch {
      markFailure();
    } finally {
      loading.value = false;
    }
  }

  async function unsnooze(id: number) {
    // Optimistic removal
    const prev = snoozedItems.value;
    snoozedItems.value = snoozedItems.value.filter((item) => item.id !== id);

    try {
      await api(`/api/v1/approval-queue/${id}/unsnooze`, { method: 'POST' });
    } catch {
      // Revert on failure
      snoozedItems.value = prev;
      await fetchSnoozedItems();
    }
  }

  // Auto-refresh on engine run completion
  watch(runCompletionCounter, () => {
    fetchSnoozedItems();
  });

  // SSE subscriptions (register once)
  if (import.meta.client && !_snoozedSseRegistered) {
    _snoozedSseRegistered = true;

    // Refresh when snooze state changes
    on(EVENT_APPROVAL_REJECTED, () => fetchSnoozedItems());
    on(EVENT_APPROVAL_UNSNOOZED, () => fetchSnoozedItems());
    on(EVENT_APPROVAL_BULK_UNSNOOZED, () => fetchSnoozedItems());
    on(EVENT_APPROVAL_QUEUE_CLEARED, () => {
      snoozedItems.value = [];
    });
    on(EVENT_APPROVAL_DISMISSED, () => fetchSnoozedItems());
    on(EVENT_ENGINE_COMPLETE, () => fetchSnoozedItems());
  }

  if (import.meta.hot) {
    import.meta.hot.dispose(() => {
      _snoozedSseRegistered = false;
    });
  }

  return {
    snoozedItems: readonly(snoozedItems),
    loading: readonly(loading),
    lastFetchOk,
    loadError,
    fetchSnoozedItems,
    unsnooze,
  };
}
