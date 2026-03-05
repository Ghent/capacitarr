/**
 * Version composable — provides both frontend and backend API versions.
 *
 * Frontend version is injected at build time from package.json.
 * API version is fetched from GET /api/v1/version on mount.
 */
export function useVersion() {
  const config = useRuntimeConfig();
  const uiVersion = (config.public.appVersion as string) || '0.0.0';
  const uiBuildDate = (config.public.appBuildDate as string) || '';

  const apiVersion = ref('');
  const apiCommit = ref('');
  const apiBuildDate = ref('');

  async function fetchApiVersion() {
    try {
      const api = useApi();
      const data = (await api('/api/v1/version')) as {
        version?: string;
        commit?: string;
        buildDate?: string;
      };
      apiVersion.value = data.version || '';
      apiCommit.value = data.commit || '';
      apiBuildDate.value = data.buildDate || '';
    } catch (e) {
      // API version endpoint may not exist yet — graceful degradation
      console.warn('[useVersion] fetchApiVersion failed:', e);
      apiVersion.value = '';
    }
  }

  onMounted(() => {
    fetchApiVersion();
  });

  return {
    uiVersion,
    uiBuildDate,
    apiVersion: readonly(apiVersion),
    apiCommit: readonly(apiCommit),
    apiBuildDate: readonly(apiBuildDate),
  };
}
