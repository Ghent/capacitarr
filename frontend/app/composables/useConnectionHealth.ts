/**
 * Tracks backend connectivity and exposes reactive state for the UI.
 *
 * When any API request fails with a network error (timeout, connection refused,
 * DNS failure), the composable marks the connection as lost and begins polling
 * a lightweight endpoint until the backend responds again.
 *
 * Usage:
 *   const { isConnected, isReconnected } = useConnectionHealth()
 *   // isConnected: false when backend is unreachable
 *   // isReconnected: briefly true after recovery (for "restored" banner)
 */
export function useConnectionHealth() {
  const isConnected = useState<boolean>('connection:connected', () => true);
  const isReconnected = useState<boolean>('connection:reconnected', () => false);

  // Tracks whether health polling is active (avoid duplicate intervals)
  const _polling = useState<boolean>('connection:polling', () => false);

  const config = useRuntimeConfig();

  /**
   * Called by useApi when a network-level error occurs (not HTTP errors).
   * Marks connection as lost and starts health polling.
   */
  function onConnectionLost() {
    if (!isConnected.value) return; // already lost
    isConnected.value = false;
    isReconnected.value = false;
    startHealthPolling();
  }

  /**
   * Called by useApi when a successful response is received.
   * If connection was previously lost, mark as reconnected.
   */
  function onConnectionRestored() {
    if (isConnected.value) return; // already connected
    isConnected.value = true;
    isReconnected.value = true;

    // Clear "reconnected" after 4 seconds
    setTimeout(() => {
      isReconnected.value = false;
    }, 4000);
  }

  /**
   * Poll the backend until it responds, then call onConnectionRestored.
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
      } catch {
        // Still unreachable — keep polling
      }
    }, 5000);
  }

  return {
    isConnected: readonly(isConnected),
    isReconnected: readonly(isReconnected),
    onConnectionLost,
    onConnectionRestored,
  };
}
