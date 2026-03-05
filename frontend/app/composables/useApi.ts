import { ofetch } from 'ofetch';

export const useApi = () => {
  const config = useRuntimeConfig();
  const authenticated = useCookie('authenticated');
  const { onConnectionLost, onConnectionRestored } = useConnectionHealth();

  const apiFetch = ofetch.create({
    baseURL: config.public.apiBaseUrl as string,
    // The HttpOnly 'jwt' cookie is sent automatically by the browser
    // for same-origin requests — no need to set Authorization header manually.
    credentials: 'include',
    onResponse() {
      // Any successful response means the backend is reachable
      onConnectionRestored();
    },
    onResponseError({ response }) {
      if (response.status === 401) {
        const router = useRouter();
        authenticated.value = null;
        router.push('/login');
      }
      // HTTP error responses still mean the backend is reachable
      onConnectionRestored();
    },
    onRequestError() {
      // Network-level failures: timeout, connection refused, DNS, etc.
      onConnectionLost();
    },
  });

  return apiFetch;
};
