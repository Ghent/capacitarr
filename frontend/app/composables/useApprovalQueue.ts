/**
 * Approval queue composable — shared state for the approval workflow.
 * Uses /api/v1/preview as the single source of truth for deletion candidates,
 * then cross-references with audit log entries to determine approval state.
 *
 * State is stored via useState so it persists across page navigations
 * and is shared between components on the same page.
 */
import type { AuditResponse, AuditLog, PreviewResponse } from '~/types/api';
import { groupEvaluatedItems } from '~/utils/groupPreview';

export interface ApprovalGroup {
  key: string;
  showTitle: string;
  type: 'show' | 'movie' | 'artist' | 'book' | string;
  totalSizeBytes: number;
  score: number;
  seasonCount: number;
  seasons: Array<{
    title: string;
    sizeBytes: number;
    score: number;
    auditId: number | null;
    scoreDetails: string;
    type: string;
  }>;
  /** Flattened approval state for the whole group */
  state: 'pending' | 'snoozed' | 'approved';
  /** All audit IDs for this group, for approve/reject actions */
  auditIds: number[];
  snoozedUntil?: string;
  scoreDetails: string;
}

export function useApprovalQueue() {
  const api = useApi();
  const { addToast } = useToast();
  const { executionMode } = useEngineControl();

  // State — shared across pages via useState
  const pendingItems = useState<ApprovalGroup[]>('approvalPending', () => []);
  const snoozedItems = useState<ApprovalGroup[]>('approvalSnoozed', () => []);
  const approvedItems = useState<ApprovalGroup[]>('approvalApproved', () => []);
  const loading = useState<Record<string, boolean>>('approvalLoading', () => ({}));

  const isApprovalMode = computed(() => executionMode.value === 'approval');

  /** Fetch all approval queue data using preview as source of truth */
  async function fetchQueue() {
    if (!isApprovalMode.value) {
      pendingItems.value = [];
      snoozedItems.value = [];
      approvedItems.value = [];
      return;
    }

    try {
      // Fetch preview + all three audit states in parallel
      const [previewData, pendingAudit, rejectedAudit, approvedAudit] = await Promise.all([
        api('/api/v1/preview') as Promise<PreviewResponse>,
        api('/api/v1/audit?action=Queued+for+Approval&limit=1000') as Promise<AuditResponse>,
        api('/api/v1/audit?action=Rejected&limit=1000') as Promise<AuditResponse>,
        api('/api/v1/audit?action=Approved&limit=1000') as Promise<AuditResponse>,
      ]);

      // Group seasons under shows using shared utility
      const groups = groupEvaluatedItems(previewData.items || []);

      // Compute deletion line from preview's diskContext
      const bytesToFree = previewData.diskContext?.bytesToFree ?? 0;
      const aboveTheLineGroups: typeof groups = [];

      if (bytesToFree > 0) {
        let cumulative = 0;
        for (const group of groups) {
          if (group.entry.isProtected) continue;

          // Accumulate group entry size plus any season sizes
          let groupSize = group.entry.item?.sizeBytes ?? 0;
          if (group.seasons.length > 0) {
            for (const season of group.seasons) {
              if (!season.isProtected) {
                groupSize += season.item?.sizeBytes ?? 0;
              }
            }
          }
          cumulative += groupSize;
          aboveTheLineGroups.push(group);
          if (cumulative >= bytesToFree) break;
        }
      }

      // Build audit lookup maps: mediaName → audit entry (most recent by ID)
      const pendingAuditMap = buildAuditMap(pendingAudit.data || []);
      const snoozedAuditMap = buildSnoozedAuditMap(rejectedAudit.data || []);
      const approvedAuditMap = buildAuditMap(approvedAudit.data || []);

      // Convert above-the-line groups into ApprovalGroups with audit state
      const pending: ApprovalGroup[] = [];
      const snoozed: ApprovalGroup[] = [];
      const approved: ApprovalGroup[] = [];

      for (const group of aboveTheLineGroups) {
        const showTitle = group.entry.item?.title ?? 'Unknown';
        const entryType = group.entry.item?.type ?? 'movie';

        // Match each season/item title to audit entries
        const seasons: ApprovalGroup['seasons'] = [];
        const auditIds: number[] = [];
        let totalSize = 0;
        let hasAnyApproved = true; // assume all approved until we find one that isn't
        let hasAnySnoozed = false;
        let groupSnoozedUntil: string | undefined;
        let groupScoreDetails = '';

        if (group.seasons.length > 0) {
          // Show with seasons — check both show-level and season-level audit entries.
          // The engine may create audit entries at the show level ("The Strain") when
          // Sonarr provides show-level items, or at season level ("The Strain - Season 1")
          // when only season entries exist.
          const showAuditEntry =
            pendingAuditMap.get(showTitle) ||
            snoozedAuditMap.get(showTitle) ||
            approvedAuditMap.get(showTitle);
          if (showAuditEntry) {
            auditIds.push(showAuditEntry.id);
            // Check show-level approval state
            if (!approvedAuditMap.has(showTitle)) hasAnyApproved = false;
            if (snoozedAuditMap.has(showTitle)) {
              hasAnySnoozed = true;
              const snoozedEntry = snoozedAuditMap.get(showTitle);
              if (snoozedEntry?.snoozedUntil) groupSnoozedUntil = snoozedEntry.snoozedUntil;
            }
          }

          for (const season of group.seasons) {
            const sTitle = season.item?.title ?? '';
            const auditEntry =
              pendingAuditMap.get(sTitle) ||
              snoozedAuditMap.get(sTitle) ||
              approvedAuditMap.get(sTitle);
            const auditId = auditEntry?.id ?? null;
            if (auditId !== null) auditIds.push(auditId);

            seasons.push({
              title: sTitle,
              sizeBytes: season.item?.sizeBytes ?? 0,
              score: season.score ?? 0,
              auditId,
              scoreDetails: season.factors ? JSON.stringify(season.factors) : '',
              type: season.item?.type ?? 'season',
            });
            totalSize += season.item?.sizeBytes ?? 0;

            // Check season-level approval state
            if (!approvedAuditMap.has(sTitle) && !approvedAuditMap.has(showTitle))
              hasAnyApproved = false;
            if (snoozedAuditMap.has(sTitle)) {
              hasAnySnoozed = true;
              const snoozedEntry = snoozedAuditMap.get(sTitle);
              if (snoozedEntry?.snoozedUntil) groupSnoozedUntil = snoozedEntry.snoozedUntil;
            }
            if (!groupScoreDetails && season.factors) {
              groupScoreDetails = JSON.stringify(season.factors);
            }
          }

          // Also use show-level score details if no season-level details found
          if (!groupScoreDetails && group.entry.factors) {
            groupScoreDetails = JSON.stringify(group.entry.factors);
          }
        } else {
          // Single item (movie, artist, book, etc.)
          const auditEntry =
            pendingAuditMap.get(showTitle) ||
            snoozedAuditMap.get(showTitle) ||
            approvedAuditMap.get(showTitle);
          const auditId = auditEntry?.id ?? null;
          if (auditId !== null) auditIds.push(auditId);

          seasons.push({
            title: showTitle,
            sizeBytes: group.entry.item?.sizeBytes ?? 0,
            score: group.entry.score ?? 0,
            auditId,
            scoreDetails: group.entry.factors ? JSON.stringify(group.entry.factors) : '',
            type: group.entry.item?.type ?? 'movie',
          });
          totalSize = group.entry.item?.sizeBytes ?? 0;

          if (!approvedAuditMap.has(showTitle)) hasAnyApproved = false;
          if (snoozedAuditMap.has(showTitle)) {
            hasAnySnoozed = true;
            const snoozedEntry = snoozedAuditMap.get(showTitle);
            if (snoozedEntry?.snoozedUntil) groupSnoozedUntil = snoozedEntry.snoozedUntil;
          }
          if (group.entry.factors) {
            groupScoreDetails = JSON.stringify(group.entry.factors);
          }
        }

        // Determine group state (edge case 3: worst state wins)
        let state: ApprovalGroup['state'] = 'pending';
        if (auditIds.length > 0 && hasAnyApproved && !hasAnySnoozed) {
          state = 'approved';
        } else if (hasAnySnoozed) {
          state = 'snoozed';
        }

        const approvalGroup: ApprovalGroup = {
          key: group.key,
          showTitle,
          type: entryType,
          totalSizeBytes: totalSize,
          score: group.entry.score ?? 0,
          seasonCount: group.seasons.length,
          seasons,
          state,
          auditIds,
          snoozedUntil: groupSnoozedUntil,
          scoreDetails: groupScoreDetails,
        };

        if (state === 'approved') {
          approved.push(approvalGroup);
        } else if (state === 'snoozed') {
          snoozed.push(approvalGroup);
        } else {
          pending.push(approvalGroup);
        }
      }

      pendingItems.value = pending;
      snoozedItems.value = snoozed;
      approvedItems.value = approved;
    } catch (e) {
      // Non-critical — queue just won't display
      console.warn('[useApprovalQueue] fetchQueue failed:', e)
    }
  }

  /** Build a map from mediaName → most recent audit entry */
  function buildAuditMap(entries: AuditLog[]): Map<string, AuditLog> {
    const map = new Map<string, AuditLog>();
    for (const entry of entries) {
      const existing = map.get(entry.mediaName);
      if (!existing || entry.id > existing.id) {
        map.set(entry.mediaName, entry);
      }
    }
    return map;
  }

  /** Build a map from mediaName → most recent snoozed audit entry (with active snooze) */
  function buildSnoozedAuditMap(entries: AuditLog[]): Map<string, AuditLog> {
    const map = new Map<string, AuditLog>();
    const now = new Date();
    for (const entry of entries) {
      if (entry.snoozedUntil && new Date(entry.snoozedUntil) > now) {
        const existing = map.get(entry.mediaName);
        if (!existing || entry.id > existing.id) {
          map.set(entry.mediaName, entry);
        }
      }
    }
    return map;
  }

  /** Approve all items in a group — calls POST /audit/:id/approve for each audit ID */
  async function approveGroup(group: ApprovalGroup) {
    if (group.auditIds.length === 0) return;

    // Optimistic update: move from pending to approved
    pendingItems.value = pendingItems.value.filter((g) => g.key !== group.key);
    snoozedItems.value = snoozedItems.value.filter((g) => g.key !== group.key);
    approvedItems.value = [...approvedItems.value, { ...group, state: 'approved' }];

    try {
      await Promise.all(
        group.auditIds.map((id) => api(`/api/v1/audit/${id}/approve`, { method: 'POST' })),
      );
      addToast('Group approved for deletion', 'success');
    } catch {
      // Revert optimistic update on failure
      approvedItems.value = approvedItems.value.filter((g) => g.key !== group.key);
      pendingItems.value = [...pendingItems.value, group];
      addToast('Failed to approve group', 'error');
    }
  }

  /** Reject (snooze) all items in a group — calls POST /audit/:id/reject for each audit ID */
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
        group.auditIds.map((id) => api(`/api/v1/audit/${id}/reject`, { method: 'POST' })),
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

  /** Undo snooze for all items in a group — calls POST /audit/:id/unsnooze for each audit ID */
  async function unsnoozeGroup(group: ApprovalGroup) {
    if (group.auditIds.length === 0) return;

    // Optimistic update: move from snoozed to pending
    snoozedItems.value = snoozedItems.value.filter((g) => g.key !== group.key);
    pendingItems.value = [...pendingItems.value, { ...group, state: 'pending' }];

    try {
      await Promise.all(
        group.auditIds.map((id) => api(`/api/v1/audit/${id}/unsnooze`, { method: 'POST' })),
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
  };
}
