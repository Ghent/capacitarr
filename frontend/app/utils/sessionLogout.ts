import { ofetch } from 'ofetch';

/**
 * POST /auth/logout so the server can clear the HTTP-only jwt cookie.
 * Failures are ignored — callers still clear local session state.
 */
export async function logoutSession(apiBaseUrl: string): Promise<void> {
  await ofetch(`${apiBaseUrl}/api/v1/auth/logout`, {
    method: 'POST',
    credentials: 'include',
  });
}
