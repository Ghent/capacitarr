import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ref, readonly, type Ref } from 'vue';
import { _resetDeletionQueueSSE, useDeletionQueue } from './useDeletionQueue';

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

function mockUseEventStream() {
  return {
    connected: readonly(ref(false)),
    reconnecting: readonly(ref(false)),
    lastEventId: readonly(ref('')),
    connect: vi.fn(),
    disconnect: vi.fn(),
    on: vi.fn(),
    off: vi.fn(),
  };
}

vi.stubGlobal('useState', mockUseState);
vi.stubGlobal('useApi', mockUseApi);
vi.stubGlobal('useEventStream', mockUseEventStream);
vi.stubGlobal('ref', ref);
vi.stubGlobal('readonly', readonly);

describe('useDeletionQueue cancel', () => {
  beforeEach(() => {
    stateStore.clear();
    mockApiFetch.mockReset();
    _resetDeletionQueueSSE();
  });

  it('DELETEs the queued item with mediaName and mediaType query params', async () => {
    mockApiFetch.mockResolvedValue([]);
    const { cancelItem } = useDeletionQueue();
    await cancelItem('Firefly', 'show');

    expect(mockApiFetch).toHaveBeenCalledWith(
      '/api/v1/deletion-queue?mediaName=Firefly&mediaType=show',
      { method: 'DELETE' },
    );
  });
});
