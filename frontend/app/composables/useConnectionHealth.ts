/**
 * Tracks backend connectivity using SSE as the primary indicator,
 * with API health polling as a fallback.
 *
 * When authenticated, the SSE EventSource connection is the fastest signal
 * for detecting disconnection (sub-second vs waiting for an API request to
 * fail). API-based detection via useApi callbacks supplements SSE for cases
 * where SSE is not yet established (e.g., login page) or is unsupported.
 *
 * Usage:
 *   const { isConnected, isReconnected, isReconnecting } = useConnectionHealth()
 *   // isConnected:    false when backend is unreachable
 *   // isReconnected:  briefly true after recovery (for "restored" banner)
 *   // isReconnecting: true when SSE is attempting to reconnect
 */
export function useConnectionHealth() {
  // API-level connection state — driven by useApi callbacks
  const _apiOk = useState<boolean>('connection:apiOk', () => true);
  const isReconnected = useState<boolean>('connection:reconnected', () => false);
  const _polling = useState<boolean>('connection:polling', () => false);

  const config = useRuntimeConfig();
  const authenticated = useAuthCookie();

  // SSE connection state — primary indicator when authenticated
  const { connected: sseConnected, reconnecting: sseReconnecting } = useEventStream();

  // Combined connection state:
  // - When authenticated, SSE is the primary signal (instant disconnect detection)
  // - API health is the fallback (only detects on request failure)
  // - When not authenticated (login page), only API health is used
  const isConnected = computed(() => {
    if (authenticated.value) {
      return sseConnected.value || _apiOk.value;
    }
    return _apiOk.value;
  });

  // True when SSE is attempting to reconnect (exponential backoff in progress)
  const isReconnecting = computed(() => {
    return sseReconnecting.value && !sseConnected.value;
  });

  /**
   * Called by useApi when a network-level error occurs (not HTTP errors).
   * Marks API connection as lost and starts health polling.
   */
  function onConnectionLost() {
    if (!_apiOk.value) return; // already lost
    _apiOk.value = false;
    isReconnected.value = false;
    startHealthPolling();
  }

  /**
   * Called by useApi when a successful response is received,
   * or when SSE reconnects successfully.
   * If connection was previously lost, triggers the "restored" banner.
   */
  function onConnectionRestored() {
    const wasDisconnected = !_apiOk.value;
    _apiOk.value = true;

    if (wasDisconnected) {
      isReconnected.value = true;
      setTimeout(() => {
        isReconnected.value = false;
      }, 4000);
    }
  }

  /**
   * Poll the backend until it responds, then call onConnectionRestored.
   * Acts as a fallback when SSE is not available or not sufficient.
   */
  function startHealthPolling() {
    if (_polling.value) return;
    _polling.value = true;

    const baseURL = config.public.apiBaseUrl as string;
    const interval = setInterval(async () => {
      try {
        const response = await fetch(`${baseURL}/api/v1/preferences`, {
          method: 'GET',
          credentials: 'include',
          signal: AbortSignal.timeout(5000),
        });
        if (response.ok || response.status === 401) {
          // 401 means the backend is up (auth required) — still counts as connected
          clearInterval(interval);
          _polling.value = false;
          onConnectionRestored();
        }
      } catch (err) {
        // Intentionally silent — background polling, toast would spam
        console.warn('[ConnectionHealth] health poll failed:', err);
      }
    }, 5000);
  }

  // Watch SSE connection state transitions to drive the "restored" banner.
  // When SSE reconnects, we know the backend is back — trigger restoration
  // without waiting for the next API request.
  if (import.meta.client) {
    let _wasConnected = false;

    watch(sseConnected, (connected) => {
      if (connected && _wasConnected) {
        // SSE reconnected after a previous connection — backend is back
        onConnectionRestored();
      }
      _wasConnected = connected;
    });
  }

  return {
    isConnected: readonly(isConnected),
    isReconnected: readonly(isReconnected),
    isReconnecting: readonly(isReconnecting),
    onConnectionLost,
    onConnectionRestored,
  };
}
