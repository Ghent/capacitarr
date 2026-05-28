/**
 * Engine control composable — shared state for execution mode and run status.
 * Used by the navbar engine popover and dashboard engine activity section.
 *
 * Engine state updates arrive via SSE events (engine_start, engine_complete,
 * engine_error) instead of polling. fetchStats() is kept for initial hydration
 * on mount.
 */
import type { WorkerStats, DeletionProgress } from '~/types/api';
import { toast } from 'vue-sonner';
import {
  MODE_DRY_RUN,
  MODE_AUTO,
  MODE_APPROVAL,
  MODE_SUNSET,
  EVENT_ENGINE_START,
  EVENT_ENGINE_COMPLETE,
  EVENT_ENGINE_ERROR,
  EVENT_DELETION_PROGRESS,
  EVENT_DELETION_BATCH_COMPLETE,
} from '~/constants';

// Module-level flag: SSE handlers are registered once globally.
let _sseRegistered = false;

/**
 * Reset the SSE registration flag. Used only in tests to allow fresh
 * handler registration after state is cleared between test cases.
 * @internal
 */
export function _resetSSERegistration() {
  _sseRegistered = false;
}

export function useEngineControl() {
  const api = useApi();
  const { on } = useEventStream();

  const workerStats = useState<WorkerStats | null>('engineWorkerStats', () => null);
  const runNowLoading = ref(false);

  // Deletion progress — updated by SSE deletion_progress events
  const deletionProgress = useState<DeletionProgress | null>('engineDeletionProgress', () => null);
  const isDeletionActive = computed(
    () =>
      deletionProgress.value !== null &&
      deletionProgress.value.batchTotal > 0 &&
      deletionProgress.value.processed < deletionProgress.value.batchTotal,
  );

  // Track previous isRunning state for run-completion detection
  const prevIsRunning = useState<boolean>('enginePrevIsRunning', () => false);

  // Counter that increments on each detected engine run completion.
  // Dashboard and other pages can watch this to trigger data refreshes.
  const runCompletionCounter = useState<number>('engineRunCompletionCounter', () => 0);

  /**
   * Per-disk-group mode map parsed from the JSON string in worker stats.
   * Keys are disk group IDs (as strings), values are mode strings.
   * Falls back to an empty object when no worker stats are available.
   */
  const diskGroupModes = computed<Record<string, string>>(() => {
    const raw = workerStats.value?.diskGroupModes;
    if (!raw) return {};
    try {
      return JSON.parse(raw) as Record<string, string>;
    } catch {
      return {};
    }
  });

  /**
   * Legacy single execution mode — derives the "most aggressive" mode across
   * all disk groups for backward compatibility. Priority: auto > approval > sunset > dry-run.
   */
  const executionMode = computed(() => {
    const modes = Object.values(diskGroupModes.value);
    if (modes.length === 0) return MODE_DRY_RUN;
    if (modes.includes(MODE_AUTO)) return MODE_AUTO;
    if (modes.includes(MODE_APPROVAL)) return MODE_APPROVAL;
    if (modes.includes(MODE_SUNSET)) return MODE_SUNSET;
    return MODE_DRY_RUN;
  });
  const lastRunEpoch = computed(() => workerStats.value?.lastRunEpoch || 0);
  const lastRunEvaluated = computed(() => workerStats.value?.lastRunEvaluated || 0);
  const lastRunCandidates = computed(() => workerStats.value?.lastRunCandidates || 0);
  const lastRunFreedBytes = computed(() => workerStats.value?.lastRunFreedBytes || 0);
  const queueDepth = computed(() => workerStats.value?.queueDepth || 0);
  const isRunning = computed(() => workerStats.value?.isRunning === true);
  const pollIntervalSeconds = computed(() => workerStats.value?.pollIntervalSeconds || 300);

  const { t } = useI18n();

  function modeLabel(mode: string): string {
    switch (mode) {
      case MODE_AUTO:
        return t('mode.auto');
      case MODE_APPROVAL:
        return t('mode.approval');
      case MODE_SUNSET:
        return t('mode.sunset');
      default:
        return t('mode.dryRun');
    }
  }

  // -------------------------------------------------------------------------
  // SSE subscriptions — registered once globally
  // -------------------------------------------------------------------------
  if (import.meta.client && !_sseRegistered) {
    _sseRegistered = true;

    on(EVENT_ENGINE_START, (data: unknown) => {
      const event = data as { diskGroupModes?: Record<string, string> };
      if (workerStats.value) {
        workerStats.value = {
          ...workerStats.value,
          isRunning: true,
          // Update diskGroupModes from the SSE event (already an object).
          diskGroupModes: event.diskGroupModes
            ? JSON.stringify(event.diskGroupModes)
            : workerStats.value.diskGroupModes,
        };
      }
      prevIsRunning.value = true;
    });

    on(EVENT_ENGINE_COMPLETE, (data: unknown) => {
      const event = data as {
        evaluated?: number;
        candidates?: number;
        durationMs?: number;
        diskGroupModes?: Record<string, string>;
        freedBytes?: number;
        completedAtEpoch?: number;
      };
      const wasRunning = prevIsRunning.value;

      if (workerStats.value) {
        // freedBytes is now included in the SSE event for dry-run and approval
        // modes (persisted by UpdateRunStats). For auto mode, the backend
        // accumulates actual freed bytes per-item via IncrementDeletedStats(),
        // so the SSE value may be 0 — keep the existing value in that case.
        const newFreedBytes =
          event.freedBytes && event.freedBytes > 0
            ? event.freedBytes
            : workerStats.value.lastRunFreedBytes;
        workerStats.value = {
          ...workerStats.value,
          isRunning: false,
          lastRunEvaluated: event.evaluated ?? workerStats.value.lastRunEvaluated,
          lastRunCandidates: event.candidates ?? workerStats.value.lastRunCandidates,
          lastRunFreedBytes: newFreedBytes,
          lastRunEpoch: event.completedAtEpoch || Math.floor(Date.now() / 1000),
          // Update diskGroupModes from the SSE event (already an object).
          diskGroupModes: event.diskGroupModes
            ? JSON.stringify(event.diskGroupModes)
            : workerStats.value.diskGroupModes,
        };
      }
      prevIsRunning.value = false;
      runNowLoading.value = false;

      // Completion detection — toast + counter
      if (wasRunning) {
        const evaluated = event.evaluated ?? 0;
        const candidates = event.candidates ?? 0;
        toast.success(
          t('engine.runCompleteToast', {
            evaluated: evaluated.toLocaleString(),
            candidates: candidates.toLocaleString(),
          }),
        );
      }
      // Always increment counter so dashboard refreshes data
      runCompletionCounter.value++;
    });

    on(EVENT_ENGINE_ERROR, (data: unknown) => {
      const event = data as { error?: string };
      if (workerStats.value) {
        workerStats.value = {
          ...workerStats.value,
          isRunning: false,
        };
      }
      prevIsRunning.value = false;
      runNowLoading.value = false;
      toast.error(t('engine.errorToast', { error: event.error || 'Unknown error' }));
    });

    on(EVENT_DELETION_PROGRESS, (data: unknown) => {
      const event = data as DeletionProgress;
      deletionProgress.value = event;
      // Sync relevant fields into workerStats for the existing dashboard cards
      if (workerStats.value) {
        workerStats.value = {
          ...workerStats.value,
          currentlyDeleting: event.currentItem,
          queueDepth: event.queueDepth,
          processed: event.processed,
          failed: event.failed,
        };
      }
    });

    on(EVENT_DELETION_BATCH_COMPLETE, () => {
      // Clear the progress indicator — batch is done
      deletionProgress.value = null;
      if (workerStats.value) {
        workerStats.value = {
          ...workerStats.value,
          currentlyDeleting: '',
          queueDepth: 0,
        };
      }
    });
  }

  // -------------------------------------------------------------------------
  // API methods
  // -------------------------------------------------------------------------

  /** Fetch current stats from the REST API (initial hydration / after mode change). */
  async function fetchStats() {
    try {
      const stats = (await api('/api/v1/worker/stats')) as WorkerStats;
      if (stats) {
        workerStats.value = stats;
        prevIsRunning.value = stats.isRunning === true;
      }
    } catch (e) {
      // Silent — stats are a nice-to-have
      console.warn('[useEngineControl] fetchStats failed:', e);
    }
  }

  async function triggerRunNow() {
    runNowLoading.value = true;
    try {
      await api('/api/v1/engine/run', { method: 'POST' });
      toast.info(t('engine.runTriggeredToast'));
      // No delay or fetchStats needed — SSE engine_start/engine_complete events
      // will update the UI reactively.
      //
      // Safety timeout: if the SSE engine_complete event is lost (connection
      // drop, slow-subscriber buffer overflow), reset the loading state after
      // 5 minutes and fetch fresh stats from the REST API so the UI doesn't
      // spin forever.
      if (import.meta.client) {
        setTimeout(
          async () => {
            if (runNowLoading.value) {
              runNowLoading.value = false;
              await fetchStats();
            }
          },
          5 * 60 * 1000,
        );
      }
    } catch {
      toast.error(t('engine.runFailedToast'));
      runNowLoading.value = false;
    }
  }

  return {
    workerStats: readonly(workerStats),
    diskGroupModes,
    executionMode,
    lastRunEpoch,
    lastRunEvaluated,
    lastRunCandidates,
    lastRunFreedBytes,
    queueDepth,
    isRunning,
    pollIntervalSeconds,
    deletionProgress: readonly(deletionProgress),
    isDeletionActive,
    runNowLoading: readonly(runNowLoading),
    runCompletionCounter: readonly(runCompletionCounter),
    modeLabel,
    fetchStats,
    triggerRunNow,
  };
}
