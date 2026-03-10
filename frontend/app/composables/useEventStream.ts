/**
 * useEventStream — SSE composable for real-time event streaming.
 *
 * Singleton pattern: one EventSource connection shared across all components.
 * Connection is managed in app.vue; handlers are registered/unregistered by
 * individual composables and page components via on()/off().
 *
 * Features:
 * - Auto-reconnect with exponential backoff
 * - Last-Event-ID tracking (tracked locally but not replayed on manual reconnect;
 *   the browser EventSource API only auto-sends Last-Event-ID on its own built-in
 *   reconnect, which we bypass for exponential backoff. The backend ring buffer
 *   supports replay if the header is present — see sse_broadcaster.go.)
 * - Typed event handlers via on()/off()
 */

// ---------------------------------------------------------------------------
// Module-level singleton state (client-only)
// ---------------------------------------------------------------------------
let _eventSource: EventSource | null = null;
let _reconnectTimer: ReturnType<typeof setTimeout> | null = null;
let _reconnectAttempts = 0;
const _handlers = new Map<string, Set<(data: unknown) => void>>();
const MAX_RECONNECT_DELAY = 30_000;

// Track which event types have a DOM listener on the current EventSource
// to avoid duplicate addEventListener calls.
const _registeredTypes = new Set<string>();

// Reactive state — lazily initialised via useState so it works from any call site.
let _connectedRef: Ref<boolean> | null = null;
let _reconnectingRef: Ref<boolean> | null = null;
let _lastEventIdRef: Ref<string> | null = null;

function getConnectedRef(): Ref<boolean> {
  if (!_connectedRef) _connectedRef = useState<boolean>('sse:connected', () => false);
  return _connectedRef;
}

function getReconnectingRef(): Ref<boolean> {
  if (!_reconnectingRef) _reconnectingRef = useState<boolean>('sse:reconnecting', () => false);
  return _reconnectingRef;
}

function getLastEventIdRef(): Ref<string> {
  if (!_lastEventIdRef) _lastEventIdRef = useState<string>('sse:lastEventId', () => '');
  return _lastEventIdRef;
}

// ---------------------------------------------------------------------------
// Connection management
// ---------------------------------------------------------------------------

function connect() {
  if (!import.meta.client) return;
  if (_eventSource) return;

  // Clear listener tracking for the new EventSource instance
  _registeredTypes.clear();

  const config = useRuntimeConfig();
  const baseURL = (config.public.apiBaseUrl as string) || '';
  const es = new EventSource(`${baseURL}/api/v1/events`, { withCredentials: true });

  es.onopen = () => {
    getConnectedRef().value = true;
    getReconnectingRef().value = false;
    _reconnectAttempts = 0;
  };

  es.onerror = () => {
    getConnectedRef().value = false;
    es.close();
    _eventSource = null;
    scheduleReconnect();
  };

  _eventSource = es;

  // Register listeners for all currently-registered handler types
  for (const eventType of _handlers.keys()) {
    registerEventListener(es, eventType);
  }
}

function disconnect() {
  if (_reconnectTimer) {
    clearTimeout(_reconnectTimer);
    _reconnectTimer = null;
  }

  if (_eventSource) {
    _eventSource.close();
    _eventSource = null;
  }

  _registeredTypes.clear();

  if (import.meta.client) {
    getConnectedRef().value = false;
    getReconnectingRef().value = false;
  }
  _reconnectAttempts = 0;
}

function scheduleReconnect() {
  if (_reconnectTimer) return;

  getReconnectingRef().value = true;

  const delay = Math.min(1000 * Math.pow(2, _reconnectAttempts), MAX_RECONNECT_DELAY);
  _reconnectAttempts++;

  _reconnectTimer = setTimeout(() => {
    _reconnectTimer = null;
    connect();
  }, delay);
}

// ---------------------------------------------------------------------------
// Handler registration
// ---------------------------------------------------------------------------

function on(eventType: string, handler: (data: unknown) => void) {
  if (!_handlers.has(eventType)) {
    _handlers.set(eventType, new Set());
  }
  _handlers.get(eventType)!.add(handler);

  // If the EventSource is already open, register a listener for this type
  if (_eventSource) {
    registerEventListener(_eventSource, eventType);
  }
}

function off(eventType: string, handler: (data: unknown) => void) {
  const set = _handlers.get(eventType);
  if (set) {
    set.delete(handler);
    if (set.size === 0) {
      _handlers.delete(eventType);
    }
  }
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

function registerEventListener(es: EventSource, eventType: string) {
  if (_registeredTypes.has(eventType)) return;
  _registeredTypes.add(eventType);

  es.addEventListener(eventType, ((event: Event) => {
    handleEvent(eventType, event as MessageEvent);
  }) as EventListener);
}

function handleEvent(eventType: string, event: MessageEvent) {
  if (event.lastEventId) {
    if (import.meta.client) {
      getLastEventIdRef().value = event.lastEventId;
    }
  }

  let data: unknown = event.data;
  try {
    data = JSON.parse(event.data as string);
  } catch {
    // Not JSON — use raw string
  }

  const set = _handlers.get(eventType);
  if (set) {
    for (const handler of set) {
      try {
        handler(data);
      } catch (err) {
        // eventType is an internal SSE event name, not user input
        console.warn(`[useEventStream] handler error for "${eventType}":`, err); // nosemgrep
      }
    }
  }
}

// ---------------------------------------------------------------------------
// Composable entry point
// ---------------------------------------------------------------------------

export function useEventStream() {
  const connected = getConnectedRef();
  const reconnecting = getReconnectingRef();
  const lastEventId = getLastEventIdRef();

  return {
    connected: readonly(connected),
    reconnecting: readonly(reconnecting),
    lastEventId: readonly(lastEventId),
    connect,
    disconnect,
    on,
    off,
  };
}
