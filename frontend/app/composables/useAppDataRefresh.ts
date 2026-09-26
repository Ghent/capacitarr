/**
 * Coordinated refetch after SSE replay_gap. Shared useState composables
 * are refreshed here. Pages with local lists (activity feed) watch
 * replayGapCounter.
 */
export function useAppDataRefresh() {
  const replayGapCounter = useState<number>('sse:replayGap', () => 0);

  function notifyReplayGap() {
    replayGapCounter.value += 1;
  }

  async function refetchAll() {
    notifyReplayGap();
    const approval = useApprovalQueue();
    const deletion = useDeletionQueue();
    const sunset = useSunsetQueue();
    const snoozed = useSnoozedItems();
    const engine = useEngineControl();
    const history = useEngineHistory();

    await Promise.allSettled([
      approval.fetchQueue(),
      deletion.fetchQueue(),
      sunset.fetchSunsetItems(),
      snoozed.fetchSnoozedItems(),
      engine.fetchStats(),
      history.fetchHistory(),
    ]);
  }

  return {
    replayGapCounter: readonly(replayGapCounter),
    notifyReplayGap,
    refetchAll,
  };
}
