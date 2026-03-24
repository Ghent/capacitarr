/**
 * Engine control composable — shared state for execution mode and run status.
 * Used by the navbar engine popover and dashboard engine activity section.
 *
 * Engine state updates arrive via SSE events (engine_start, engine_complete,
 * engine_error) instead of polling. fetchStats() is kept for initial hydration
 * on mount and after explicit user actions (mode change).
 */
import type { WorkerStats, PreferenceSet, DeletionProgress } from '~/types/api';
import { MODE_DRY_RUN, MODE_AUTO, MODE_APPROVAL } from '~/constants';

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
  const { addToast } = useToast();
  const { on } = useEventStream();

  const workerStats = useState<WorkerStats | null>('engineWorkerStats', () => null);
  const runNowLoading = ref(false);
  const changingMode = ref(false);

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

  const executionMode = computed(() => workerStats.value?.executionMode || MODE_DRY_RUN);
  const lastRunEpoch = computed(() => workerStats.value?.lastRunEpoch || 0);
  const lastRunEvaluated = computed(() => workerStats.value?.lastRunEvaluated || 0);
  const lastRunCandidates = computed(() => workerStats.value?.lastRunCandidates || 0);
  const lastRunFreedBytes = computed(() => workerStats.value?.lastRunFreedBytes || 0);
  const queueDepth = computed(() => workerStats.value?.queueDepth || 0);
  const isRunning = computed(() => workerStats.value?.isRunning === true);
  const pollIntervalSeconds = computed(() => workerStats.value?.pollIntervalSeconds || 300);

  function modeLabel(mode: string): string {
    switch (mode) {
      case MODE_AUTO:
        return 'Auto';
      case MODE_APPROVAL:
        return 'Approval';
      default:
        return 'Dry-Run';
    }
  }

  // -------------------------------------------------------------------------
  // SSE subscriptions — registered once globally
  // -------------------------------------------------------------------------
  if (import.meta.client && !_sseRegistered) {
    _sseRegistered = true;

    on('engine_start', (data: unknown) => {
      const event = data as { executionMode?: string };
      if (workerStats.value) {
        workerStats.value = {
          ...workerStats.value,
          isRunning: true,
          executionMode: event.executionMode || workerStats.value.executionMode,
        };
      }
      prevIsRunning.value = true;
    });

    on('engine_complete', (data: unknown) => {
      const event = data as {
        evaluated?: number;
        candidates?: number;
        durationMs?: number;
        executionMode?: string;
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
          executionMode: event.executionMode || workerStats.value.executionMode,
        };
      }
      prevIsRunning.value = false;
      runNowLoading.value = false;

      // Completion detection — toast + counter
      if (wasRunning) {
        const evaluated = event.evaluated ?? 0;
        const candidates = event.candidates ?? 0;
        addToast(
          `Engine run complete — evaluated ${evaluated.toLocaleString()} items, ${candidates.toLocaleString()} candidates`,
          'success',
        );
      }
      // Always increment counter so dashboard refreshes data
      runCompletionCounter.value++;
    });

    on('engine_error', (data: unknown) => {
      const event = data as { error?: string };
      if (workerStats.value) {
        workerStats.value = {
          ...workerStats.value,
          isRunning: false,
        };
      }
      prevIsRunning.value = false;
      runNowLoading.value = false;
      addToast(`Engine error: ${event.error || 'Unknown error'}`, 'error');
    });

    on('engine_mode_changed', (data: unknown) => {
      const event = data as { newMode?: string };
      if (workerStats.value && event.newMode) {
        workerStats.value = {
          ...workerStats.value,
          executionMode: event.newMode,
        };
      }
    });

    on('deletion_progress', (data: unknown) => {
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

    on('deletion_batch_complete', () => {
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

  async function setMode(mode: string) {
    changingMode.value = true;
    try {
      const currentPrefs = (await api('/api/v1/preferences')) as PreferenceSet;
      await api('/api/v1/preferences', {
        method: 'PUT',
        body: { ...currentPrefs, executionMode: mode },
      });
      // Refresh stats to pick up the new mode immediately
      await fetchStats();
      addToast(`Execution mode set to ${modeLabel(mode)}`, 'success');
    } catch {
      addToast('Failed to change execution mode', 'error');
    } finally {
      changingMode.value = false;
    }
  }

  async function triggerRunNow() {
    runNowLoading.value = true;
    try {
      await api('/api/v1/engine/run', { method: 'POST' });
      addToast('Engine run triggered', 'info');
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
      addToast('Failed to trigger engine run', 'error');
      runNowLoading.value = false;
    }
  }

  return {
    workerStats: readonly(workerStats),
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
    changingMode: readonly(changingMode),
    runCompletionCounter: readonly(runCompletionCounter),
    modeLabel,
    fetchStats,
    setMode,
    triggerRunNow,
  };
}
