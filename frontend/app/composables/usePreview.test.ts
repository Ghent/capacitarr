import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ref, readonly } from 'vue';
import { usePreview } from './usePreview';

// ---------------------------------------------------------------------------
// Mock Nuxt auto-imports
// ---------------------------------------------------------------------------

const mockApiFetch = vi.fn();
function mockUseApi() {
  return mockApiFetch;
}

const sseHandlers = new Map<string, (data: unknown) => void>();
const mockSseOn = vi.fn((eventType: string, handler: (data: unknown) => void) => {
  sseHandlers.set(eventType, handler);
});
const mockSseOff = vi.fn();
function mockUseEventStream() {
  return {
    connected: readonly(ref(false)),
    reconnecting: readonly(ref(false)),
    lastEventId: readonly(ref('')),
    connect: vi.fn(),
    disconnect: vi.fn(),
    on: mockSseOn,
    off: mockSseOff,
  };
}

// Stub Nuxt auto-imports — use Vue's real implementations
vi.stubGlobal('useApi', mockUseApi);
vi.stubGlobal('useEventStream', mockUseEventStream);
vi.stubGlobal('useState', (_key: string, init?: () => unknown) => ref(init ? init() : undefined));
vi.stubGlobal('watch', vi.fn());
vi.stubGlobal('ref', ref);
vi.stubGlobal('readonly', readonly);
vi.stubGlobal('onMounted', (fn: () => void) => fn());
vi.stubGlobal('onUnmounted', vi.fn());

describe('usePreview', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    sseHandlers.clear();
    mockApiFetch.mockReset();
  });

  it('starts with empty items and loading=false', () => {
    const { items, loading, stale } = usePreview();
    expect(items.value).toEqual([]);
    expect(loading.value).toBe(false);
    expect(stale.value).toBe(false);
  });

  it('refresh fetches preview data', async () => {
    mockApiFetch.mockResolvedValueOnce({
      items: [
        { item: { title: 'Firefly', type: 'show' }, score: 5.0 },
        { item: { title: 'Serenity', type: 'movie' }, score: 3.5 },
      ],
      diskContext: { mountPath: '/media', usedPct: 80 },
    });

    const { items, diskContext, truncated, totalItems, refresh } = usePreview();
    await refresh();

    expect(items.value).toHaveLength(2);
    expect(items.value[0]!.item.title).toBe('Firefly');
    expect(diskContext.value).toEqual({ mountPath: '/media', usedPct: 80 });
    expect(truncated.value).toBe(false);
    expect(totalItems.value).toBe(2);
  });

  it('refresh records truncated preview cap', async () => {
    mockApiFetch.mockResolvedValueOnce({
      items: [{ item: { title: 'Firefly', type: 'show' }, score: 5.0 }],
      diskContext: null,
      truncated: true,
      totalItems: 2500,
    });

    const { items, truncated, totalItems, refresh } = usePreview();
    await refresh();

    expect(items.value).toHaveLength(1);
    expect(truncated.value).toBe(true);
    expect(totalItems.value).toBe(2500);
  });

  it('refresh(force=true) adds force query param', async () => {
    mockApiFetch.mockResolvedValueOnce({ items: [], diskContext: null });

    const { refresh } = usePreview();
    await refresh(true);

    expect(mockApiFetch).toHaveBeenCalledWith('/api/v1/preview?force=true');
  });

  it('refresh handles API errors without clearing last-good data', async () => {
    mockApiFetch.mockResolvedValueOnce({
      items: [{ item: { title: 'Firefly', type: 'show' }, score: 5.0 }],
      diskContext: { mountPath: '/media', usedPct: 80 },
    });
    const { items, diskContext, loadError, refresh } = usePreview();
    await refresh();
    expect(items.value).toHaveLength(1);

    mockApiFetch.mockRejectedValueOnce(new Error('Network error'));
    await refresh();

    expect(items.value).toHaveLength(1);
    expect(diskContext.value).toEqual({ mountPath: '/media', usedPct: 80 });
    expect(loadError.value).toBe(false);
  });

  it('sets loadError on first-fetch failure', async () => {
    mockApiFetch.mockRejectedValueOnce(new Error('Network error'));
    const { items, loadError, refresh } = usePreview();
    await refresh();
    expect(items.value).toEqual([]);
    expect(loadError.value).toBe(true);
  });

  it('deletion_success event removes item from list', async () => {
    mockApiFetch.mockResolvedValueOnce({
      items: [
        { item: { title: 'Firefly', type: 'show' }, score: 5.0 },
        { item: { title: 'Serenity', type: 'movie' }, score: 3.5 },
      ],
      diskContext: null,
    });

    const { items, refresh } = usePreview();
    await refresh();
    expect(items.value).toHaveLength(2);

    // Simulate SSE deletion_success event
    const handler = sseHandlers.get('deletion_success');
    expect(handler).toBeDefined();
    handler!({ mediaName: 'Firefly', mediaType: 'show' });

    expect(items.value).toHaveLength(1);
    expect(items.value[0]!.item.title).toBe('Serenity');
  });

  it('deletion_dry_run event removes item from list', async () => {
    mockApiFetch.mockResolvedValueOnce({
      items: [{ item: { title: 'Serenity', type: 'movie' }, score: 3.5 }],
      diskContext: null,
    });

    const { items, refresh } = usePreview();
    await refresh();

    const handler = sseHandlers.get('deletion_dry_run');
    expect(handler).toBeDefined();
    handler!({ mediaName: 'Serenity', mediaType: 'movie' });

    expect(items.value).toHaveLength(0);
  });

  it('deletion_progress event marks item as deleting', async () => {
    mockApiFetch.mockResolvedValueOnce({
      items: [
        { item: { title: 'Firefly', type: 'show' }, score: 5.0 },
        { item: { title: 'Serenity', type: 'movie' }, score: 3.5 },
      ],
      diskContext: null,
    });

    const { items, refresh } = usePreview();
    await refresh();

    const handler = sseHandlers.get('deletion_progress');
    expect(handler).toBeDefined();
    handler!({ currentItem: 'Firefly', total: 2, completed: 0 });

    expect(items.value[0]!.queueStatus).toBe('deleting');
    expect(items.value[1]!.queueStatus).toBeUndefined();
  });

  it('preview_invalidated marks data as stale', () => {
    // preview_invalidated should set stale=true and trigger a force refresh
    mockApiFetch.mockResolvedValueOnce({ items: [], diskContext: null });

    const { stale } = usePreview();

    const handler = sseHandlers.get('preview_invalidated');
    expect(handler).toBeDefined();
    handler!(undefined);

    expect(stale.value).toBe(true);
  });

  it('registers SSE handlers on mount', () => {
    usePreview();

    const expectedEvents = [
      'preview_updated',
      'preview_invalidated',
      'deletion_success',
      'deletion_dry_run',
      'deletion_progress',
      'deletion_batch_complete',
    ];

    for (const eventName of expectedEvents) {
      expect(mockSseOn).toHaveBeenCalledWith(
        eventName,
        expect.any(Function),
        expect.objectContaining({ onUnmounted: expect.any(Function) }),
      );
    }
  });
});
