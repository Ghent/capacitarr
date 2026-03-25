/**
 * Approval queue composable — shared state for the approval workflow.
 * The approval_queue table is the single source of truth. Items are grouped
 * by show title for season-level entries (e.g., "Big Mouth - Season 1" through
 * "Big Mouth - Season 8" become one group with 8 seasons).
 *
 * State is stored via useState so it persists across page navigations
 * and is shared between components on the same page.
 */
import type { FetchError } from 'ofetch';
import { MODE_APPROVAL } from '~/constants';
import type { ApprovalQueueItem } from '~/types/api';

export interface ApprovalGroup {
  key: string;
  showTitle: string;
  type: 'show' | 'movie' | 'artist' | 'book' | string;
  totalSizeBytes: number;
  score: number;
  seasonCount: number;
  posterUrl?: string;
  seasons: ReadonlyArray<{
    title: string;
    sizeBytes: number;
    score: number;
    auditId: number | null;
    scoreDetails: string;
    type: string;
  }>;
  /** Flattened approval state for the whole group */
  state: 'pending' | 'snoozed' | 'approved';
  /** All approval queue IDs for this group, for approve/reject actions */
  auditIds: readonly number[];
  snoozedUntil?: string;
  scoreDetails: string;
  /** Collection group name if this item is part of a collection deletion */
  collectionGroup?: string;
}

// Module-level flag: SSE handlers registered once globally.
let _approvalSseRegistered = false;

/**
 * Extract the show title from a season media name.
 * "Big Mouth - Season 1" → "Big Mouth"
 * "The Strain - Season 3" → "The Strain"
 * Returns null if this isn't a season-format name.
 */
function extractShowTitle(mediaName: string): string | null {
  const match = mediaName.match(/^(.+?)\s+-\s+Season\s+\d+$/);
  return match && match[1] ? match[1] : null;
}

export function useApprovalQueue() {
  const api = useApi();
  const { addToast } = useToast();
  const { executionMode } = useEngineControl();
  const { on: sseOn } = useEventStream();

  // State — shared across pages via useState
  const pendingItems = useState<ApprovalGroup[]>('approvalPending', () => []);
  const snoozedItems = useState<ApprovalGroup[]>('approvalSnoozed', () => []);
  const approvedItems = useState<ApprovalGroup[]>('approvalApproved', () => []);
  const loading = useState<Record<string, boolean>>('approvalLoading', () => ({}));

  const isApprovalMode = computed(() => executionMode.value === MODE_APPROVAL);

  /**
   * Fetch all approval queue items and group them for display.
   * The approval_queue table IS the source of truth — no preview cross-referencing.
   */
  async function fetchQueue() {
    if (!isApprovalMode.value) {
      pendingItems.value = [];
      snoozedItems.value = [];
      approvedItems.value = [];
      return;
    }

    try {
      // Fetch all approval queue items (all statuses)
      const allItems = (await api('/api/v1/approval-queue?limit=1000')) as ApprovalQueueItem[];

      // Group items: seasons under their parent show, standalone items as-is
      const groupMap = new Map<
        string,
        {
          showTitle: string;
          type: string;
          items: ApprovalQueueItem[];
          isShowGroup: boolean;
        }
      >();

      for (const item of allItems) {
        const showTitle = extractShowTitle(item.mediaName);

        if (showTitle && item.mediaType === 'season') {
          // Season entry — group under the show title
          const existing = groupMap.get(showTitle);
          if (existing) {
            existing.items.push(item);
          } else {
            groupMap.set(showTitle, {
              showTitle,
              type: 'show',
              items: [item],
              isShowGroup: true,
            });
          }
        } else {
          // Standalone item (movie, show without seasons, artist, book, etc.)
          groupMap.set(item.mediaName, {
            showTitle: item.mediaName,
            type: item.mediaType,
            items: [item],
            isShowGroup: false,
          });
        }
      }

      // Convert groups to ApprovalGroup format and categorize by state
      const pending: ApprovalGroup[] = [];
      const snoozed: ApprovalGroup[] = [];
      const approved: ApprovalGroup[] = [];

      for (const [key, group] of groupMap) {
        const auditIds = group.items.map((i) => i.id);
        let totalSize = 0;
        let bestScore = 0;
        let bestScoreDetails = '';
        let hasAnySnoozed = false;
        let hasAllApproved = true;
        let groupSnoozedUntil: string | undefined;
        const now = new Date();

        const seasons: Array<ApprovalGroup['seasons'][number]> = [];

        for (const item of group.items) {
          totalSize += item.sizeBytes;
          const score = item.score ?? 0;
          if (score > bestScore) {
            bestScore = score;
            bestScoreDetails = item.scoreDetails;
          }

          seasons.push({
            title: item.mediaName,
            sizeBytes: item.sizeBytes,
            score,
            auditId: item.id,
            scoreDetails: item.scoreDetails,
            type: item.mediaType,
          });

          // Track group-level state
          if (item.status !== 'approved') hasAllApproved = false;
          if (
            item.status === 'rejected' &&
            item.snoozedUntil &&
            new Date(item.snoozedUntil) > now
          ) {
            hasAnySnoozed = true;
            groupSnoozedUntil = item.snoozedUntil;
          }
        }

        // Sort seasons by name with numeric awareness so
        // "Season 2" sorts before "Season 10"
        seasons.sort((a, b) => a.title.localeCompare(b.title, undefined, { numeric: true }));

        // Determine group state: snoozed wins over pending, approved only if ALL are approved
        let state: ApprovalGroup['state'] = 'pending';
        if (hasAnySnoozed) {
          state = 'snoozed';
        } else if (hasAllApproved && auditIds.length > 0) {
          state = 'approved';
        }

        // Use the first item's collectionGroup — all items in the group should share it
        const collectionGroup = group.items[0]?.collectionGroup || undefined;

        const approvalGroup: ApprovalGroup = {
          key,
          showTitle: group.showTitle,
          type: group.type,
          totalSizeBytes: totalSize,
          score: bestScore,
          seasonCount: group.isShowGroup ? group.items.length : 0,
          posterUrl: group.items[0]?.posterUrl || undefined,
          seasons,
          state,
          auditIds,
          snoozedUntil: groupSnoozedUntil,
          scoreDetails: bestScoreDetails,
          collectionGroup,
        };

        if (state === 'approved') {
          approved.push(approvalGroup);
        } else if (state === 'snoozed') {
          snoozed.push(approvalGroup);
        } else {
          pending.push(approvalGroup);
        }
      }

      // Sort each list by score descending (highest-priority items first)
      const byScore = (a: ApprovalGroup, b: ApprovalGroup) => b.score - a.score;
      pendingItems.value = pending.sort(byScore);
      snoozedItems.value = snoozed.sort(byScore);
      approvedItems.value = approved.sort(byScore);
    } catch (e) {
      // Non-critical — queue just won't display
      console.warn('[useApprovalQueue] fetchQueue failed:', e);
    }
  }

  /** Approve all items in a group — calls POST /approval-queue/:id/approve for each ID */
  async function approveGroup(group: ApprovalGroup) {
    if (group.auditIds.length === 0) return;

    // Optimistic update: move from pending to approved
    pendingItems.value = pendingItems.value.filter((g) => g.key !== group.key);
    snoozedItems.value = snoozedItems.value.filter((g) => g.key !== group.key);
    approvedItems.value = [...approvedItems.value, { ...group, state: 'approved' }];

    try {
      await Promise.all(
        group.auditIds.map((id) => api(`/api/v1/approval-queue/${id}/approve`, { method: 'POST' })),
      );
      addToast('Group approved for deletion', 'success');
    } catch (e: unknown) {
      // Revert optimistic update on failure
      approvedItems.value = approvedItems.value.filter((g) => g.key !== group.key);
      pendingItems.value = [...pendingItems.value, group];

      const fe = e as FetchError;
      if (fe?.statusCode === 409) {
        addToast(fe.data?.error || 'Approval blocked — deletions are disabled', 'error');
      } else {
        addToast('Failed to approve group', 'error');
      }
    }
  }

  /** Reject (snooze) all items in a group — calls POST /approval-queue/:id/reject for each ID */
  async function rejectGroup(group: ApprovalGroup) {
    if (group.auditIds.length === 0) return;

    // Optimistic update: move from pending to snoozed
    pendingItems.value = pendingItems.value.filter((g) => g.key !== group.key);
    const snoozeExpiry = new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString();
    snoozedItems.value = [
      ...snoozedItems.value,
      { ...group, state: 'snoozed', snoozedUntil: snoozeExpiry },
    ];

    try {
      await Promise.all(
        group.auditIds.map((id) => api(`/api/v1/approval-queue/${id}/reject`, { method: 'POST' })),
      );
      addToast('Group snoozed', 'info');
      // Background refresh to get accurate snooze duration from server
      fetchQueue();
    } catch {
      // Revert optimistic update on failure
      snoozedItems.value = snoozedItems.value.filter((g) => g.key !== group.key);
      pendingItems.value = [...pendingItems.value, group];
      addToast('Failed to snooze group', 'error');
    }
  }

  /** Undo snooze for all items in a group — calls POST /approval-queue/:id/unsnooze for each ID */
  async function unsnoozeGroup(group: ApprovalGroup) {
    if (group.auditIds.length === 0) return;

    // Optimistic update: move from snoozed to pending
    snoozedItems.value = snoozedItems.value.filter((g) => g.key !== group.key);
    pendingItems.value = [...pendingItems.value, { ...group, state: 'pending' }];

    try {
      await Promise.all(
        group.auditIds.map((id) =>
          api(`/api/v1/approval-queue/${id}/unsnooze`, { method: 'POST' }),
        ),
      );
      addToast('Snooze removed — group re-queued for approval', 'success');
      // Background refresh to sync with server state
      fetchQueue();
    } catch {
      // Revert optimistic update on failure
      pendingItems.value = pendingItems.value.filter((g) => g.key !== group.key);
      snoozedItems.value = [...snoozedItems.value, group];
      addToast('Failed to unsnooze group', 'error');
    }
  }

  /** Approve a single season by its approval queue ID, then refresh the queue */
  async function approveSeason(auditId: number) {
    try {
      await api(`/api/v1/approval-queue/${auditId}/approve`, { method: 'POST' });
      addToast('Season approved for deletion', 'success');
      fetchQueue();
    } catch (e: unknown) {
      const fe = e as FetchError;
      if (fe?.statusCode === 409) {
        addToast(fe.data?.error || 'Approval blocked — deletions are disabled', 'error');
      } else {
        addToast('Failed to approve season', 'error');
      }
    }
  }

  /** Snooze a single season by its approval queue ID, then refresh the queue */
  async function snoozeSeason(auditId: number) {
    try {
      await api(`/api/v1/approval-queue/${auditId}/reject`, { method: 'POST' });
      addToast('Season snoozed', 'info');
      fetchQueue();
    } catch {
      addToast('Failed to snooze season', 'error');
    }
  }

  /** Dismiss all items in a group — calls DELETE /approval-queue/:id for each ID */
  async function dismissGroup(group: ApprovalGroup) {
    if (group.auditIds.length === 0) return;

    // Optimistic update: remove from both lists
    pendingItems.value = pendingItems.value.filter((g) => g.key !== group.key);
    snoozedItems.value = snoozedItems.value.filter((g) => g.key !== group.key);

    try {
      await Promise.all(
        group.auditIds.map((id) => api(`/api/v1/approval-queue/${id}`, { method: 'DELETE' })),
      );
      addToast('Dismissed from queue', 'info');
    } catch {
      // Revert optimistic update on failure
      if (group.state === 'snoozed') {
        snoozedItems.value = [...snoozedItems.value, group];
      } else {
        pendingItems.value = [...pendingItems.value, group];
      }
      addToast('Failed to dismiss group', 'error');
    }
  }

  /** Dismiss a single season by its approval queue ID, then refresh the queue */
  async function dismissSeason(auditId: number) {
    try {
      await api(`/api/v1/approval-queue/${auditId}`, { method: 'DELETE' });
      addToast('Season dismissed', 'info');
      fetchQueue();
    } catch {
      addToast('Failed to dismiss season', 'error');
    }
  }

  /** Clear the entire approval queue (pending + rejected items) */
  async function clearQueue() {
    // Optimistic update: clear pending and snoozed lists
    const prevPending = pendingItems.value;
    const prevSnoozed = snoozedItems.value;
    pendingItems.value = [];
    snoozedItems.value = [];

    try {
      await api('/api/v1/approval-queue/clear', { method: 'POST' });
      addToast('Queue cleared', 'info');
    } catch {
      // Revert optimistic update on failure
      pendingItems.value = prevPending;
      snoozedItems.value = prevSnoozed;
      addToast('Failed to clear queue', 'error');
    }
  }

  // ---------------------------------------------------------------------------
  // SSE subscriptions — registered once globally to refresh queue on changes
  // ---------------------------------------------------------------------------
  if (import.meta.client && !_approvalSseRegistered) {
    _approvalSseRegistered = true;

    const refreshOnEvent = () => {
      // Only refresh if we're in approval mode
      if (isApprovalMode.value) fetchQueue();
    };

    // Queue state changes that warrant a refresh
    sseOn('engine_complete', refreshOnEvent);
    sseOn('deletion_success', refreshOnEvent);
    sseOn('deletion_failed', refreshOnEvent);
    sseOn('approval_orphans_recovered', refreshOnEvent);
    sseOn('approval_bulk_unsnoozed', refreshOnEvent);
    sseOn('approval_queue_cleared', refreshOnEvent);
    sseOn('approval_dismissed', refreshOnEvent);
  }

  return {
    // State
    pendingItems: readonly(pendingItems),
    snoozedItems: readonly(snoozedItems),
    approvedItems: readonly(approvedItems),
    loading: readonly(loading),
    isApprovalMode,

    // Actions
    fetchQueue,
    approveGroup,
    rejectGroup,
    unsnoozeGroup,
    approveSeason,
    snoozeSeason,
    dismissGroup,
    dismissSeason,
    clearQueue,
  };
}
