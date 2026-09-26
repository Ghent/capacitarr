/**
 * Tests for useEventStream composable.
 *
 * NOTE: The composable's connect(), disconnect(), on(), and off() functions
 * guard on `import.meta.client`. Vitest's `define` config does not replace
 * `import.meta.client` at runtime in Vitest 4 / Vite 8, so these functions
 * are no-ops in tests. Tests that require active connections use conditional
 * assertions (like the existing useEngineControl.test.ts pattern).
 *
 * The tests below verify:
 * - Return shape and initial reactive state
 * - Reactive state types and defaults
 * - That the composable is safe to call in SSR/test contexts (no-ops)
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { ref, readonly, type Ref } from 'vue';

// Import AFTER stubs
import { eventsURL, useEventStream } from './useEventStream';

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

function mockUseRuntimeConfig() {
  return {
    public: {
      apiBaseUrl: 'http://localhost:2187',
    },
  };
}

vi.stubGlobal('useState', mockUseState);
vi.stubGlobal('useRuntimeConfig', mockUseRuntimeConfig);
vi.stubGlobal('readonly', readonly);

describe('useEventStream', () => {
  beforeEach(() => {
    stateStore.clear();
  });

  afterEach(() => {
    // Tear down any active connection
    const { disconnect } = useEventStream();
    disconnect();
  });

  // -------------------------------------------------------------------------
  // Return shape
  // -------------------------------------------------------------------------
  describe('return shape', () => {
    it('returns expected properties and methods', () => {
      const stream = useEventStream();

      expect(stream).toHaveProperty('connected');
      expect(stream).toHaveProperty('reconnecting');
      expect(stream).toHaveProperty('lastEventId');
      expect(stream).toHaveProperty('connect');
      expect(stream).toHaveProperty('disconnect');
      expect(stream).toHaveProperty('on');
      expect(stream).toHaveProperty('off');
    });

    it('exposes connect as a function', () => {
      const { connect } = useEventStream();
      expect(typeof connect).toBe('function');
    });

    it('exposes disconnect as a function', () => {
      const { disconnect } = useEventStream();
      expect(typeof disconnect).toBe('function');
    });

    it('exposes on as a function', () => {
      const { on } = useEventStream();
      expect(typeof on).toBe('function');
    });

    it('exposes off as a function', () => {
      const { off } = useEventStream();
      expect(typeof off).toBe('function');
    });
  });

  // -------------------------------------------------------------------------
  // Reactive state defaults
  // -------------------------------------------------------------------------
  describe('reactive state defaults', () => {
    it('starts with connected=false', () => {
      const { connected } = useEventStream();
      expect(connected.value).toBe(false);
    });

    it('starts with reconnecting=false', () => {
      const { reconnecting } = useEventStream();
      expect(reconnecting.value).toBe(false);
    });

    it('starts with empty lastEventId', () => {
      const { lastEventId } = useEventStream();
      expect(lastEventId.value).toBe('');
    });

    it('reactive state is readonly (write is silently ignored)', () => {
      const { connected, reconnecting, lastEventId } = useEventStream();
      // Vue's readonly() does not throw — it silently ignores writes and
      // emits a console warning in dev mode.
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});

      // @ts-expect-error — testing runtime readonly enforcement
      connected.value = true;
      expect(connected.value).toBe(false); // unchanged

      // @ts-expect-error — testing runtime readonly enforcement
      reconnecting.value = true;
      expect(reconnecting.value).toBe(false); // unchanged

      // @ts-expect-error — testing runtime readonly enforcement
      lastEventId.value = 'test';
      expect(lastEventId.value).toBe(''); // unchanged

      // Vue should have warned about the readonly set operation
      expect(warnSpy).toHaveBeenCalled();
      warnSpy.mockRestore();
    });
  });

  // -------------------------------------------------------------------------
  // Shared state via useState
  // -------------------------------------------------------------------------
  describe('shared state via useState', () => {
    it('returns the same reactive state across multiple calls', () => {
      const stream1 = useEventStream();
      const stream2 = useEventStream();

      // Both should reference the same underlying useState-backed ref
      expect(stream1.connected.value).toBe(stream2.connected.value);
      expect(stream1.reconnecting.value).toBe(stream2.reconnecting.value);
      expect(stream1.lastEventId.value).toBe(stream2.lastEventId.value);
    });

    it('state is backed by useState (singleton pattern)', () => {
      // The composable uses module-level singleton refs initialized via useState.
      // After the first call, the refs are cached at the module level — subsequent
      // calls to useEventStream() return the same refs without calling useState
      // again. We verify the singleton pattern works by checking that two calls
      // return identical ref objects.
      const stream1 = useEventStream();
      const stream2 = useEventStream();

      // Same readonly ref wrapper → same underlying state
      expect(stream1.connected).toBe(stream2.connected);
      expect(stream1.reconnecting).toBe(stream2.reconnecting);
      expect(stream1.lastEventId).toBe(stream2.lastEventId);
    });
  });

  // -------------------------------------------------------------------------
  // SSR/test safety — functions are no-ops when import.meta.client is falsy
  // -------------------------------------------------------------------------
  describe('SSR/test safety', () => {
    it('connect() is safe to call (no-op when not client)', () => {
      const { connect } = useEventStream();
      // Should not throw, even though there's no real EventSource
      expect(() => connect()).not.toThrow();
    });

    it('disconnect() is safe to call', () => {
      const { disconnect } = useEventStream();
      expect(() => disconnect()).not.toThrow();
    });

    it('on() is safe to call (no-op when not client)', () => {
      const { on } = useEventStream();
      const handler = vi.fn();
      expect(() => on('test_event', handler)).not.toThrow();
    });

    it('off() is safe to call (no-op when not client)', () => {
      const { off } = useEventStream();
      const handler = vi.fn();
      expect(() => off('test_event', handler)).not.toThrow();
    });

    it('on() with scope parameter is safe to call', () => {
      const { on } = useEventStream();
      const handler = vi.fn();
      const scope = { onUnmounted: vi.fn() };
      expect(() => on('test_event', handler, scope)).not.toThrow();
    });

    it('disconnect() after connect() is safe', () => {
      const { connect, disconnect } = useEventStream();
      connect();
      expect(() => disconnect()).not.toThrow();
    });

    it('multiple disconnect() calls are safe', () => {
      const { disconnect } = useEventStream();
      expect(() => {
        disconnect();
        disconnect();
        disconnect();
      }).not.toThrow();
    });
  });

  // -------------------------------------------------------------------------
  // State after disconnect()
  // -------------------------------------------------------------------------
  describe('eventsURL', () => {
    it('omits the query when lastEventId is empty', () => {
      expect(eventsURL('http://localhost:2187', '')).toBe('http://localhost:2187/api/v1/events');
    });

    it('appends lastEventId on reconnect', () => {
      expect(eventsURL('http://localhost:2187', '42')).toBe(
        'http://localhost:2187/api/v1/events?lastEventId=42',
      );
    });
  });

  describe('state after disconnect', () => {
    it('connected is false after disconnect', () => {
      const { disconnect, connected } = useEventStream();
      disconnect();
      // import.meta.client is falsy in tests, so disconnect is a partial no-op,
      // but the internal state should remain false
      expect(connected.value).toBe(false);
    });

    it('reconnecting is false after disconnect', () => {
      const { disconnect, reconnecting } = useEventStream();
      disconnect();
      expect(reconnecting.value).toBe(false);
    });
  });
});
