import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ref, computed, readonly, watch, type Ref } from 'vue';
import { useConnectionHealth } from './useConnectionHealth';

// ---------------------------------------------------------------------------
// Mock Nuxt auto-imports before importing the composable under test
// ---------------------------------------------------------------------------

const stateStore = new Map<string, Ref>();
function mockUseState<T>(key: string, init?: () => T): Ref<T> {
  if (!stateStore.has(key)) {
    stateStore.set(key, ref(init ? init() : undefined) as Ref);
  }
  return stateStore.get(key) as Ref<T>;
}

function mockUseRuntimeConfig() {
  return {
    public: {
      apiBaseUrl: 'http://localhost:2187/api/v1',
    },
  };
}

const mockCookieRef = ref<string | null>('true');
function mockUseAuthCookie() {
  return mockCookieRef;
}

// Mock SSE connected/reconnecting state
const sseConnected = ref(false);
const sseReconnecting = ref(false);
function mockUseEventStream() {
  return {
    connected: readonly(sseConnected),
    reconnecting: readonly(sseReconnecting),
    lastEventId: readonly(ref('')),
    connect: vi.fn(),
    disconnect: vi.fn(),
    on: vi.fn(),
    off: vi.fn(),
  };
}

// Stub globals — use Vue's real implementations
vi.stubGlobal('useState', mockUseState);
vi.stubGlobal('useRuntimeConfig', mockUseRuntimeConfig);
vi.stubGlobal('useAuthCookie', mockUseAuthCookie);
vi.stubGlobal('useEventStream', mockUseEventStream);
vi.stubGlobal('computed', computed);
vi.stubGlobal('readonly', readonly);
vi.stubGlobal('watch', watch);

describe('useConnectionHealth', () => {
  beforeEach(() => {
    stateStore.clear();
    sseConnected.value = false;
    sseReconnecting.value = false;
    mockCookieRef.value = 'true';
    vi.useFakeTimers();
  });

  it('starts with isConnected=true (optimistic default)', () => {
    const { isConnected } = useConnectionHealth();
    // Default apiOk is true, SSE not connected yet, but apiOk alone makes it connected
    expect(isConnected.value).toBe(true);
  });

  it('isConnected becomes false after onConnectionLost', () => {
    const { isConnected, onConnectionLost } = useConnectionHealth();
    onConnectionLost();
    // Without SSE connected, apiOk=false → disconnected
    expect(isConnected.value).toBe(false);
  });

  it('isConnected becomes true after onConnectionRestored', () => {
    const { isConnected, onConnectionLost, onConnectionRestored } = useConnectionHealth();
    onConnectionLost();
    expect(isConnected.value).toBe(false);

    onConnectionRestored();
    expect(isConnected.value).toBe(true);
  });

  it('isReconnected is briefly true after recovery', () => {
    const { isReconnected, onConnectionLost, onConnectionRestored } = useConnectionHealth();
    onConnectionLost();
    onConnectionRestored();

    expect(isReconnected.value).toBe(true);

    // After 4 seconds, it resets
    vi.advanceTimersByTime(4000);
    expect(isReconnected.value).toBe(false);
  });

  it('isReconnected stays false if never disconnected', () => {
    const { isReconnected, onConnectionRestored } = useConnectionHealth();
    // Call restored without prior disconnect
    onConnectionRestored();
    expect(isReconnected.value).toBe(false);
  });

  it('isReconnecting is true when SSE is reconnecting and not connected', () => {
    sseReconnecting.value = true;
    sseConnected.value = false;

    const { isReconnecting } = useConnectionHealth();
    expect(isReconnecting.value).toBe(true);
  });

  it('isReconnecting is false when SSE is connected', () => {
    sseReconnecting.value = true;
    sseConnected.value = true;

    const { isReconnecting } = useConnectionHealth();
    expect(isReconnecting.value).toBe(false);
  });

  it('isConnected uses SSE as primary signal when authenticated', () => {
    mockCookieRef.value = 'true';
    sseConnected.value = true;

    const { isConnected, onConnectionLost } = useConnectionHealth();
    // Even if API is "lost", SSE connected means still connected
    onConnectionLost();
    expect(isConnected.value).toBe(true);
  });

  it('isConnected uses API only when not authenticated', () => {
    mockCookieRef.value = null;
    sseConnected.value = true;

    const { isConnected, onConnectionLost } = useConnectionHealth();
    expect(isConnected.value).toBe(true);

    onConnectionLost();
    // Not authenticated, so SSE doesn't count — only API matters
    expect(isConnected.value).toBe(false);
  });

  it('duplicate onConnectionLost calls do not restart polling', () => {
    const { onConnectionLost, isConnected } = useConnectionHealth();
    onConnectionLost();
    onConnectionLost(); // second call should be ignored — no error thrown
    expect(isConnected.value).toBe(false);
  });
});
