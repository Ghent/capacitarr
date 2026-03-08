/**
 * useAuthCookie — thin wrapper around useCookie('authenticated') that includes
 * the correct cookie path for subdirectory (BASE_URL) deployments.
 *
 * The backend sets the 'authenticated' cookie with Path=cfg.BaseURL (e.g.
 * "/capacitarr/"). Nuxt's useCookie defaults to path="/". Without matching
 * paths, clearing the cookie on logout creates a *new* cookie at "/" while the
 * original at "/capacitarr/" persists — the browser still sends
 * authenticated=true and logout silently fails.
 *
 * By centralising the cookie access here, every consumer automatically gets the
 * correct path derived from Nuxt's app.baseURL, which the backend rewrites at
 * startup to match cfg.BaseURL.
 */
export function useAuthCookie() {
  const config = useRuntimeConfig();
  return useCookie('authenticated', {
    path: config.app.baseURL || '/',
  });
}
