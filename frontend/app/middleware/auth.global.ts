/**
 * Global auth middleware.
 * Redirects unauthenticated users to /login for all pages except /login itself.
 *
 * Uses the 'authenticated' cookie (non-HttpOnly, set by the backend on login)
 * because the 'jwt' cookie is HttpOnly and cannot be read by client-side JS.
 */
export default defineNuxtRouteMiddleware((to) => {
  const authenticated = useCookie('authenticated');

  if (to.path !== '/login' && !authenticated.value) {
    return navigateTo('/login');
  }

  // If already authenticated and trying to visit login, redirect to dashboard
  if (to.path === '/login' && authenticated.value) {
    return navigateTo('/');
  }
});
