import type { SunsetQueueItem } from '~/types/api';
import { toast } from 'vue-sonner';
import {
  EVENT_SUNSET_CREATED,
  EVENT_SUNSET_CANCELLED,
  EVENT_SUNSET_EXPIRED,
  EVENT_SUNSET_RESCHEDULED,
  EVENT_SUNSET_ESCALATED,
  EVENT_SUNSET_SAVED,
  EVENT_SUNSET_SAVED_CLEANED,
} from '~/constants';
import { useFetchStatus } from './useFetchStatus';

/** @internal */
export function _resetSunsetQueueSSE() {
  _sseRegistered = false;
}

// Module-level flag — SSE handlers are registered once globally,
// not per component instance. Same pattern as useSnoozedItems.ts.
let _sseRegistered = false;

export function useSunsetQueue() {
  const { t } = useI18n();
  const api = useApi();
  const { on } = useEventStream();
  const { runCompletionCounter } = useEngineControl();

  const sunsetItems = useState<SunsetQueueItem[]>('sunsetItems', () => []);
  const loading = useState<boolean>('sunsetLoading', () => false);
  const { lastFetchOk, loadError, markSuccess, markFailure } = useFetchStatus();

  async function fetchSunsetItems() {
    loading.value = true;
    try {
      const data = (await api('/api/v1/sunset-queue')) as SunsetQueueItem[];
      sunsetItems.value = data ?? [];
      markSuccess();
    } catch (err) {
      console.warn('[useSunsetQueue] fetchSunsetItems failed:', err);
      markFailure();
    } finally {
      loading.value = false;
    }
  }

  async function cancelItem(id: number) {
    // Optimistic removal
    const prev = [...sunsetItems.value];
    sunsetItems.value = sunsetItems.value.filter((item) => item.id !== id);

    try {
      await api(`/api/v1/sunset-queue/${id}`, { method: 'DELETE' });
      toast.success(t('sunset.cancelledToast'));
    } catch {
      sunsetItems.value = prev;
      toast.error(t('sunset.cancelFailedToast'));
    }
  }

  async function rescheduleItem(id: number, deletionDate: string) {
    try {
      const result = (await api(`/api/v1/sunset-queue/${id}`, {
        method: 'PATCH',
        body: { deletionDate },
      })) as { id: number; mediaName: string; deletionDate: string; daysRemaining: number };

      // Update local state
      const idx = sunsetItems.value.findIndex((item) => item.id === id);
      if (idx >= 0) {
        const existing = sunsetItems.value[idx];
        sunsetItems.value[idx] = Object.assign({}, existing, {
          deletionDate: result.deletionDate,
          daysRemaining: result.daysRemaining,
        });
      }
      toast.success(t('sunset.rescheduledToast', { days: result.daysRemaining }));
    } catch {
      toast.error(t('sunset.rescheduleFailedToast'));
    }
  }

  async function clearAll() {
    try {
      const result = (await api('/api/v1/sunset-queue/clear', { method: 'POST' })) as {
        cancelled: number;
      };
      sunsetItems.value = [];
      toast.success(t('sunset.clearedToast', { count: result.cancelled }));
    } catch {
      toast.error(t('sunset.clearFailedToast'));
    }
  }

  // Auto-refresh on engine run completion
  watch(runCompletionCounter, () => {
    fetchSunsetItems();
  });

  // SSE subscriptions — register once globally
  if (import.meta.client && !_sseRegistered) {
    _sseRegistered = true;

    on(EVENT_SUNSET_CREATED, () => fetchSunsetItems());
    on(EVENT_SUNSET_CANCELLED, () => fetchSunsetItems());
    on(EVENT_SUNSET_EXPIRED, () => fetchSunsetItems());
    on(EVENT_SUNSET_RESCHEDULED, () => fetchSunsetItems());
    on(EVENT_SUNSET_ESCALATED, () => fetchSunsetItems());
    on(EVENT_SUNSET_SAVED, () => fetchSunsetItems());
    on(EVENT_SUNSET_SAVED_CLEANED, () => fetchSunsetItems());
  }

  if (import.meta.hot) {
    import.meta.hot.dispose(() => {
      _sseRegistered = false;
    });
  }

  return {
    sunsetItems: readonly(sunsetItems),
    loading: readonly(loading),
    lastFetchOk,
    loadError,
    fetchSunsetItems,
    cancelItem,
    rescheduleItem,
    clearAll,
  };
}
