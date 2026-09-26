/**
 * Shared fetch-status helper for initial page loads.
 *
 * lastFetchOk is true only after a successful fetch.
 * loadError is true only when the first fetch failed (never flip empty-state on).
 * Background refresh failures leave lastFetchOk unchanged.
 */
export function useFetchStatus() {
  const lastFetchOk = ref(false);
  const loadError = ref(false);

  function markSuccess() {
    lastFetchOk.value = true;
    loadError.value = false;
  }

  function markFailure() {
    if (!lastFetchOk.value) {
      loadError.value = true;
    }
  }

  function retryReset() {
    loadError.value = false;
  }

  return {
    lastFetchOk: readonly(lastFetchOk),
    loadError: readonly(loadError),
    markSuccess,
    markFailure,
    retryReset,
  };
}
