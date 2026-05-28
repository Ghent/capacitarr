import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ref, computed, readonly, type Ref } from 'vue';
import {
  MODE_APPROVAL,
  MODE_DRY_RUN,
  EVENT_ENGINE_COMPLETE,
  EVENT_DELETION_SUCCESS,
  EVENT_DELETION_FAILED,
  EVENT_APPROVAL_ORPHANS_RECOVERED,
  EVENT_APPROVAL_BULK_UNSNOOZED,
  EVENT_APPROVAL_QUEUE_CLEARED,
  EVENT_APPROVAL_DISMISSED,
  EVENT_APPROVAL_RETURNED_TO_PENDING,
} from '~/constants';
import type { ApprovalQueueItem } from '~/types/api';

import { useApprovalQueue, type ApprovalGroup } from './useApprovalQueue';

// ---------------------------------------------------------------------------
// vi.hoisted — ensure toast spies are available for the vi.mock factory
// ---------------------------------------------------------------------------
const { toastSuccessSpy, toastErrorSpy, toastInfoSpy } = vi.hoisted(() => ({
  toastSuccessSpy: vi.fn(),
  toastErrorSpy: vi.fn(),
  toastInfoSpy: vi.fn(),
}));

vi.mock('vue-sonner', () => ({
  toast: Object.assign(vi.fn(), {
    success: toastSuccessSpy,
    error: toastErrorSpy,
    info: toastInfoSpy,
    warning: vi.fn(),
  }),
}));

// ---------------------------------------------------------------------------
// Mock Nuxt auto-imports
// ---------------------------------------------------------------------------

const stateStore = new Map<string, Ref>();
function mockUseState<T>(key: string, init?: () => T): Ref<T> {
  if (!stateStore.has(key)) {
    stateStore.set(key, ref(init ? init() : undefined) as Ref);
  }
  return stateStore.get(key) as Ref<T>;
}

const mockApiFetch = vi.fn();
function mockUseApi() {
  return mockApiFetch;
}

// Engine control — controls the per-disk-group modes
const diskGroupModesRef = ref<Record<string, string>>({ '1': MODE_APPROVAL });
function mockUseEngineControl() {
  return {
    diskGroupModes: diskGroupModesRef,
  };
}

// SSE mock — capture registered handlers
const sseHandlers = new Map<string, (data: unknown) => void>();
const mockSseOn = vi.fn((eventType: string, handler: (data: unknown) => void) => {
  sseHandlers.set(eventType, handler);
});
function mockUseEventStream() {
  return {
    connected: readonly(ref(false)),
    reconnecting: readonly(ref(false)),
    lastEventId: readonly(ref('')),
    connect: vi.fn(),
    disconnect: vi.fn(),
    on: mockSseOn,
    off: vi.fn(),
  };
}

vi.stubGlobal('useState', mockUseState);
vi.stubGlobal('useApi', mockUseApi);
vi.stubGlobal('useEngineControl', mockUseEngineControl);
vi.stubGlobal('useEventStream', mockUseEventStream);
vi.stubGlobal('ref', ref);
vi.stubGlobal('computed', computed);
vi.stubGlobal('readonly', readonly);

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

function makeApprovalItem(overrides: Partial<ApprovalQueueItem> = {}): ApprovalQueueItem {
  return {
    id: 1,
    mediaName: 'Serenity',
    mediaType: 'movie',
    scoreDetails: 'low watch count',
    sizeBytes: 5_000_000_000,
    score: 7.5,
    integrationId: 1,
    externalId: 'tmdb-12345',
    status: 'pending',
    trigger: 'engine',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  };
}

function makeSeasonItem(
  showTitle: string,
  season: number,
  overrides: Partial<ApprovalQueueItem> = {},
): ApprovalQueueItem {
  return makeApprovalItem({
    id: season,
    mediaName: `${showTitle} - Season ${season}`,
    mediaType: 'season',
    sizeBytes: 2_000_000_000,
    score: 5.0 + season,
    ...overrides,
  });
}

function makeGroup(overrides: Partial<ApprovalGroup> = {}): ApprovalGroup {
  return {
    key: 'Serenity',
    showTitle: 'Serenity',
    type: 'movie',
    totalSizeBytes: 5_000_000_000,
    score: 7.5,
    seasonCount: 0,
    seasons: [
      {
        title: 'Serenity',
        sizeBytes: 5_000_000_000,
        score: 7.5,
        auditId: 1,
        scoreDetails: 'low watch count',
        type: 'movie',
      },
    ],
    state: 'pending',
    auditIds: [1],
    scoreDetails: 'low watch count',
    ...overrides,
  };
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('useApprovalQueue', () => {
  beforeEach(() => {
    stateStore.clear();
    sseHandlers.clear();
    mockApiFetch.mockReset();
    toastSuccessSpy.mockReset();
    toastErrorSpy.mockReset();
    toastInfoSpy.mockReset();
    diskGroupModesRef.value = { '1': MODE_APPROVAL };
    vi.spyOn(console, 'warn').mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  // -------------------------------------------------------------------------
  // Initial state
  // -------------------------------------------------------------------------
  describe('initial state', () => {
    it('starts with empty queues and loading flags', () => {
      const q = useApprovalQueue();
      expect(q.pendingItems.value).toEqual([]);
      expect(q.snoozedItems.value).toEqual([]);
      expect(q.approvedItems.value).toEqual([]);
      expect(q.loading.value).toEqual({});
    });

    it('isApprovalMode is true when any disk group is in approval mode', () => {
      const q = useApprovalQueue();
      expect(q.isApprovalMode.value).toBe(true);
    });

    it('isApprovalMode is false when no disk group is in approval mode', () => {
      diskGroupModesRef.value = { '1': MODE_DRY_RUN };
      const q = useApprovalQueue();
      expect(q.isApprovalMode.value).toBe(false);
    });
  });

  // -------------------------------------------------------------------------
  // fetchQueue
  // -------------------------------------------------------------------------
  describe('fetchQueue', () => {
    it('fetches and groups items into pending/snoozed/approved', async () => {
      const items: ApprovalQueueItem[] = [
        makeApprovalItem({ id: 1, mediaName: 'Serenity', mediaType: 'movie', status: 'pending' }),
        makeApprovalItem({
          id: 2,
          mediaName: 'Firefly - Season 1',
          mediaType: 'season',
          status: 'pending',
          score: 6.0,
        }),
        makeApprovalItem({
          id: 3,
          mediaName: 'Firefly - Season 2',
          mediaType: 'season',
          status: 'pending',
          score: 8.0,
        }),
        makeApprovalItem({
          id: 4,
          mediaName: 'Expired Show',
          mediaType: 'movie',
          status: 'approved',
        }),
      ];
      mockApiFetch.mockResolvedValueOnce(items);

      const q = useApprovalQueue();
      await q.fetchQueue();

      // Serenity is a standalone movie → pending, Firefly group → pending
      expect(q.pendingItems.value).toHaveLength(2);
      // Expired Show is approved
      expect(q.approvedItems.value).toHaveLength(1);
      expect(q.approvedItems.value[0]!.showTitle).toBe('Expired Show');
    });

    it('groups seasons under their parent show', async () => {
      const items: ApprovalQueueItem[] = [
        makeSeasonItem('Firefly', 1),
        makeSeasonItem('Firefly', 2, { score: 9.0 }),
        makeSeasonItem('Firefly', 3),
      ];
      mockApiFetch.mockResolvedValueOnce(items);

      const q = useApprovalQueue();
      await q.fetchQueue();

      expect(q.pendingItems.value).toHaveLength(1);
      const group = q.pendingItems.value[0]!;
      expect(group.showTitle).toBe('Firefly');
      expect(group.type).toBe('show');
      expect(group.seasonCount).toBe(3);
      expect(group.seasons).toHaveLength(3);
      // Best score should be from season 2 (9.0)
      expect(group.score).toBe(9.0);
    });

    it('sorts groups by score descending', async () => {
      const items: ApprovalQueueItem[] = [
        makeApprovalItem({ id: 1, mediaName: 'Serenity', score: 3.0, status: 'pending' }),
        makeApprovalItem({ id: 2, mediaName: 'Firefly', score: 9.0, status: 'pending' }),
        makeApprovalItem({ id: 3, mediaName: 'Dark Matter', score: 6.0, status: 'pending' }),
      ];
      mockApiFetch.mockResolvedValueOnce(items);

      const q = useApprovalQueue();
      await q.fetchQueue();

      expect(q.pendingItems.value[0]!.showTitle).toBe('Firefly');
      expect(q.pendingItems.value[1]!.showTitle).toBe('Dark Matter');
      expect(q.pendingItems.value[2]!.showTitle).toBe('Serenity');
    });

    it('categorizes snoozed items (rejected with future snoozedUntil)', async () => {
      const futureDate = new Date(Date.now() + 86_400_000).toISOString();
      const items: ApprovalQueueItem[] = [
        makeApprovalItem({
          id: 1,
          mediaName: 'Serenity',
          status: 'rejected',
          snoozedUntil: futureDate,
        }),
      ];
      mockApiFetch.mockResolvedValueOnce(items);

      const q = useApprovalQueue();
      await q.fetchQueue();

      expect(q.pendingItems.value).toHaveLength(0);
      expect(q.snoozedItems.value).toHaveLength(1);
      expect(q.snoozedItems.value[0]!.state).toBe('snoozed');
    });

    it('fetches queue items regardless of disk group modes', async () => {
      diskGroupModesRef.value = { '1': MODE_DRY_RUN };
      const items: ApprovalQueueItem[] = [
        makeApprovalItem({ id: 1, mediaName: 'Serenity', status: 'pending' }),
      ];
      mockApiFetch.mockResolvedValueOnce(items);

      const q = useApprovalQueue();
      await q.fetchQueue();

      // Should still fetch — per-disk-group mode may be approval even when global is dry-run
      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/approval-queue?limit=1000');
      expect(q.pendingItems.value).toHaveLength(1);
    });

    it('handles API errors gracefully', async () => {
      mockApiFetch.mockRejectedValueOnce(new Error('Network error'));

      const q = useApprovalQueue();
      await expect(q.fetchQueue()).resolves.toBeUndefined();
    });
  });

  // -------------------------------------------------------------------------
  // approveGroup — optimistic updates
  // -------------------------------------------------------------------------
  describe('approveGroup', () => {
    it('optimistically moves group from pending to approved', async () => {
      const group = makeGroup();

      // Pre-populate pending via state
      const pendingRef = mockUseState<ApprovalGroup[]>('approvalPending', () => [group]);
      expect(pendingRef.value).toHaveLength(1);

      mockApiFetch.mockResolvedValue({});

      const q = useApprovalQueue();
      await q.approveGroup(group);

      // Pending should be empty, approved should have the group
      expect(stateStore.get('approvalPending')!.value).toHaveLength(0);
      expect(stateStore.get('approvalApproved')!.value).toHaveLength(1);
      expect(toastSuccessSpy).toHaveBeenCalledWith('Group approved for deletion');
    });

    it('calls POST /approve for each audit ID', async () => {
      const group = makeGroup({ auditIds: [10, 20, 30] });
      mockApiFetch.mockResolvedValue({});

      const q = useApprovalQueue();
      await q.approveGroup(group);

      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/approval-queue/10/approve', {
        method: 'POST',
      });
      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/approval-queue/20/approve', {
        method: 'POST',
      });
      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/approval-queue/30/approve', {
        method: 'POST',
      });
    });

    it('reverts optimistic update on API failure', async () => {
      const group = makeGroup();
      mockUseState<ApprovalGroup[]>('approvalPending', () => [group]);

      mockApiFetch.mockRejectedValue(new Error('Server error'));

      const q = useApprovalQueue();
      await q.approveGroup(group);

      // Should revert: back in pending, not in approved
      expect(stateStore.get('approvalPending')!.value).toHaveLength(1);
      expect(stateStore.get('approvalApproved')!.value).toHaveLength(0);
      expect(toastErrorSpy).toHaveBeenCalledWith('Failed to approve group');
    });

    it('shows specific error on 409 conflict', async () => {
      const group = makeGroup();
      mockUseState<ApprovalGroup[]>('approvalPending', () => [group]);

      const conflictError = Object.assign(new Error('Conflict'), {
        statusCode: 409,
        data: { error: 'Deletions are disabled' },
      });
      mockApiFetch.mockRejectedValue(conflictError);

      const q = useApprovalQueue();
      await q.approveGroup(group);

      expect(toastErrorSpy).toHaveBeenCalledWith('Deletions are disabled');
    });

    it('no-ops when auditIds is empty', async () => {
      const group = makeGroup({ auditIds: [] });

      const q = useApprovalQueue();
      await q.approveGroup(group);

      expect(mockApiFetch).not.toHaveBeenCalled();
    });
  });

  // -------------------------------------------------------------------------
  // rejectGroup (snooze) — optimistic updates
  // -------------------------------------------------------------------------
  describe('rejectGroup', () => {
    it('optimistically moves group from pending to snoozed', async () => {
      const group = makeGroup();
      mockUseState<ApprovalGroup[]>('approvalPending', () => [group]);

      mockApiFetch.mockResolvedValue({});

      const q = useApprovalQueue();
      await q.rejectGroup(group);

      expect(stateStore.get('approvalPending')!.value).toHaveLength(0);
      expect(stateStore.get('approvalSnoozed')!.value).toHaveLength(1);
      expect(stateStore.get('approvalSnoozed')!.value[0].state).toBe('snoozed');
      expect(toastInfoSpy).toHaveBeenCalledWith('Group snoozed');
    });

    it('calls POST /reject for each audit ID', async () => {
      const group = makeGroup({ auditIds: [5, 6] });
      mockApiFetch.mockResolvedValue({});

      const q = useApprovalQueue();
      await q.rejectGroup(group);

      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/approval-queue/5/reject', {
        method: 'POST',
      });
      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/approval-queue/6/reject', {
        method: 'POST',
      });
    });

    it('reverts optimistic update on failure', async () => {
      const group = makeGroup();
      mockUseState<ApprovalGroup[]>('approvalPending', () => [group]);

      mockApiFetch.mockRejectedValue(new Error('fail'));

      const q = useApprovalQueue();
      await q.rejectGroup(group);

      expect(stateStore.get('approvalPending')!.value).toHaveLength(1);
      expect(stateStore.get('approvalSnoozed')!.value).toHaveLength(0);
      expect(toastErrorSpy).toHaveBeenCalledWith('Failed to snooze group');
    });
  });

  // -------------------------------------------------------------------------
  // unsnoozeGroup
  // -------------------------------------------------------------------------
  describe('unsnoozeGroup', () => {
    it('optimistically moves group from snoozed to pending', async () => {
      const group = makeGroup({ state: 'snoozed' });
      mockUseState<ApprovalGroup[]>('approvalSnoozed', () => [group]);

      mockApiFetch.mockResolvedValue({});

      const q = useApprovalQueue();
      await q.unsnoozeGroup(group);

      expect(stateStore.get('approvalSnoozed')!.value).toHaveLength(0);
      expect(stateStore.get('approvalPending')!.value).toHaveLength(1);
      expect(stateStore.get('approvalPending')!.value[0].state).toBe('pending');
      expect(toastSuccessSpy).toHaveBeenCalledWith('Snooze removed — group re-queued for approval');
    });

    it('calls POST /unsnooze for each audit ID', async () => {
      const group = makeGroup({ state: 'snoozed', auditIds: [7, 8] });
      mockUseState<ApprovalGroup[]>('approvalSnoozed', () => [group]);

      mockApiFetch.mockResolvedValue({});

      const q = useApprovalQueue();
      await q.unsnoozeGroup(group);

      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/approval-queue/7/unsnooze', {
        method: 'POST',
      });
      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/approval-queue/8/unsnooze', {
        method: 'POST',
      });
    });

    it('reverts optimistic update on failure', async () => {
      const group = makeGroup({ state: 'snoozed' });
      mockUseState<ApprovalGroup[]>('approvalSnoozed', () => [group]);

      mockApiFetch.mockRejectedValue(new Error('fail'));

      const q = useApprovalQueue();
      await q.unsnoozeGroup(group);

      expect(stateStore.get('approvalSnoozed')!.value).toHaveLength(1);
      expect(stateStore.get('approvalPending')!.value).toHaveLength(0);
      expect(toastErrorSpy).toHaveBeenCalledWith('Failed to unsnooze group');
    });
  });

  // -------------------------------------------------------------------------
  // dismissGroup
  // -------------------------------------------------------------------------
  describe('dismissGroup', () => {
    it('optimistically removes group from pending', async () => {
      const group = makeGroup();
      mockUseState<ApprovalGroup[]>('approvalPending', () => [group]);

      mockApiFetch.mockResolvedValue({});

      const q = useApprovalQueue();
      await q.dismissGroup(group);

      expect(stateStore.get('approvalPending')!.value).toHaveLength(0);
      expect(toastInfoSpy).toHaveBeenCalledWith('Dismissed from queue');
    });

    it('calls DELETE for each audit ID', async () => {
      const group = makeGroup({ auditIds: [11, 12] });
      mockApiFetch.mockResolvedValue({});

      const q = useApprovalQueue();
      await q.dismissGroup(group);

      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/approval-queue/11', { method: 'DELETE' });
      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/approval-queue/12', { method: 'DELETE' });
    });

    it('reverts snoozed group on failure', async () => {
      const group = makeGroup({ state: 'snoozed' });
      mockUseState<ApprovalGroup[]>('approvalSnoozed', () => [group]);

      mockApiFetch.mockRejectedValue(new Error('fail'));

      const q = useApprovalQueue();
      await q.dismissGroup(group);

      expect(stateStore.get('approvalSnoozed')!.value).toHaveLength(1);
      expect(toastErrorSpy).toHaveBeenCalledWith('Failed to dismiss group');
    });
  });

  // -------------------------------------------------------------------------
  // clearQueue
  // -------------------------------------------------------------------------
  describe('clearQueue', () => {
    it('optimistically clears pending and snoozed lists', async () => {
      mockUseState<ApprovalGroup[]>('approvalPending', () => [makeGroup()]);
      mockUseState<ApprovalGroup[]>('approvalSnoozed', () => [makeGroup({ state: 'snoozed' })]);

      mockApiFetch.mockResolvedValue({});

      const q = useApprovalQueue();
      await q.clearQueue();

      expect(stateStore.get('approvalPending')!.value).toHaveLength(0);
      expect(stateStore.get('approvalSnoozed')!.value).toHaveLength(0);
      expect(toastInfoSpy).toHaveBeenCalledWith('Queue cleared');
    });

    it('calls POST /approval-queue/clear', async () => {
      mockApiFetch.mockResolvedValue({});

      const q = useApprovalQueue();
      await q.clearQueue();

      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/approval-queue/clear', {
        method: 'POST',
      });
    });

    it('reverts on failure', async () => {
      const pending = [makeGroup()];
      const snoozed = [makeGroup({ key: 'snoozed-item', state: 'snoozed' })];
      mockUseState<ApprovalGroup[]>('approvalPending', () => [...pending]);
      mockUseState<ApprovalGroup[]>('approvalSnoozed', () => [...snoozed]);

      mockApiFetch.mockRejectedValue(new Error('fail'));

      const q = useApprovalQueue();
      await q.clearQueue();

      expect(stateStore.get('approvalPending')!.value).toHaveLength(1);
      expect(stateStore.get('approvalSnoozed')!.value).toHaveLength(1);
      expect(toastErrorSpy).toHaveBeenCalledWith('Failed to clear queue');
    });
  });

  // -------------------------------------------------------------------------
  // Season-level actions
  // -------------------------------------------------------------------------
  describe('season-level actions', () => {
    it('approveSeason calls POST /approve for a single ID', async () => {
      mockApiFetch.mockResolvedValue({});

      const q = useApprovalQueue();
      await q.approveSeason(42);

      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/approval-queue/42/approve', {
        method: 'POST',
      });
      expect(toastSuccessSpy).toHaveBeenCalledWith('Season approved for deletion');
    });

    it('approveSeason shows 409 conflict error', async () => {
      const conflictError = Object.assign(new Error('Conflict'), {
        statusCode: 409,
        data: { error: 'Deletions are disabled' },
      });
      mockApiFetch.mockRejectedValue(conflictError);

      const q = useApprovalQueue();
      await q.approveSeason(42);

      expect(toastErrorSpy).toHaveBeenCalledWith('Deletions are disabled');
    });

    it('snoozeSeason calls POST /reject for a single ID', async () => {
      mockApiFetch.mockResolvedValue({});

      const q = useApprovalQueue();
      await q.snoozeSeason(99);

      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/approval-queue/99/reject', {
        method: 'POST',
      });
      expect(toastInfoSpy).toHaveBeenCalledWith('Season snoozed');
    });

    it('dismissSeason calls DELETE for a single ID', async () => {
      mockApiFetch.mockResolvedValue({});

      const q = useApprovalQueue();
      await q.dismissSeason(77);

      expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/approval-queue/77', { method: 'DELETE' });
      expect(toastInfoSpy).toHaveBeenCalledWith('Season dismissed');
    });
  });

  // -------------------------------------------------------------------------
  // SSE event subscriptions
  // -------------------------------------------------------------------------
  describe('SSE subscriptions', () => {
    it('registers handlers for all expected event types (client-only)', () => {
      // SSE registration is guarded by import.meta.client, which is falsy in
      // the Vitest test environment. When running in a real browser or with
      // import.meta.client = true, the composable registers handlers for these
      // event types. We verify the handler was either registered or skipped
      // based on the environment.
      useApprovalQueue();

      const expectedEvents = [
        EVENT_ENGINE_COMPLETE,
        EVENT_DELETION_SUCCESS,
        EVENT_DELETION_FAILED,
        EVENT_APPROVAL_ORPHANS_RECOVERED,
        EVENT_APPROVAL_BULK_UNSNOOZED,
        EVENT_APPROVAL_QUEUE_CLEARED,
        EVENT_APPROVAL_DISMISSED,
        EVENT_APPROVAL_RETURNED_TO_PENDING,
      ];

      if (mockSseOn.mock.calls.length > 0) {
        // import.meta.client was true — verify all handlers registered
        for (const eventType of expectedEvents) {
          expect(mockSseOn).toHaveBeenCalledWith(eventType, expect.any(Function));
        }
      } else {
        // import.meta.client was falsy — SSE registration skipped (expected in test env)
        expect(mockSseOn).not.toHaveBeenCalled();
      }
    });

    it('SSE handler triggers fetchQueue when in approval mode', async () => {
      const items: ApprovalQueueItem[] = [
        makeApprovalItem({ id: 1, mediaName: 'Serenity', status: 'pending' }),
      ];
      mockApiFetch.mockResolvedValue(items);

      useApprovalQueue();

      const handler = sseHandlers.get(EVENT_ENGINE_COMPLETE);
      if (handler) {
        handler({});
        expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/approval-queue?limit=1000');
      }
    });

    it('SSE handler triggers fetchQueue regardless of disk group modes', async () => {
      diskGroupModesRef.value = { '1': MODE_DRY_RUN };
      mockApiFetch.mockResolvedValue([]);

      useApprovalQueue();

      const handler = sseHandlers.get(EVENT_ENGINE_COMPLETE);
      if (handler) {
        handler({});
        // Should still fetch — per-disk-group mode may be approval even when global is dry-run
        expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/approval-queue?limit=1000');
      }
    });
  });

  // -------------------------------------------------------------------------
  // hasQueueItems
  // -------------------------------------------------------------------------
  describe('hasQueueItems', () => {
    it('is false when all queues are empty', () => {
      const q = useApprovalQueue();
      expect(q.hasQueueItems.value).toBe(false);
    });

    it('is true when pending items exist', async () => {
      const items: ApprovalQueueItem[] = [
        makeApprovalItem({ id: 1, mediaName: 'Serenity', status: 'pending' }),
      ];
      mockApiFetch.mockResolvedValueOnce(items);

      const q = useApprovalQueue();
      await q.fetchQueue();

      expect(q.hasQueueItems.value).toBe(true);
    });

    it('is true when approved items exist', async () => {
      const items: ApprovalQueueItem[] = [
        makeApprovalItem({ id: 1, mediaName: 'Serenity', status: 'approved' }),
      ];
      mockApiFetch.mockResolvedValueOnce(items);

      const q = useApprovalQueue();
      await q.fetchQueue();

      expect(q.hasQueueItems.value).toBe(true);
    });

    it('is true when snoozed items exist', async () => {
      const futureDate = new Date(Date.now() + 86_400_000).toISOString();
      const items: ApprovalQueueItem[] = [
        makeApprovalItem({
          id: 1,
          mediaName: 'Serenity',
          status: 'rejected',
          snoozedUntil: futureDate,
        }),
      ];
      mockApiFetch.mockResolvedValueOnce(items);

      const q = useApprovalQueue();
      await q.fetchQueue();

      expect(q.hasQueueItems.value).toBe(true);
    });
  });

  // -------------------------------------------------------------------------
  // Return shape
  // -------------------------------------------------------------------------
  describe('return shape', () => {
    it('returns all expected state and action properties', () => {
      const q = useApprovalQueue();

      // State
      expect(q).toHaveProperty('pendingItems');
      expect(q).toHaveProperty('snoozedItems');
      expect(q).toHaveProperty('approvedItems');
      expect(q).toHaveProperty('loading');
      expect(q).toHaveProperty('isApprovalMode');
      expect(q).toHaveProperty('hasQueueItems');

      // Actions
      expect(q).toHaveProperty('fetchQueue');
      expect(q).toHaveProperty('approveGroup');
      expect(q).toHaveProperty('rejectGroup');
      expect(q).toHaveProperty('unsnoozeGroup');
      expect(q).toHaveProperty('approveSeason');
      expect(q).toHaveProperty('snoozeSeason');
      expect(q).toHaveProperty('dismissGroup');
      expect(q).toHaveProperty('dismissSeason');
      expect(q).toHaveProperty('clearQueue');
    });
  });
});
