import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ref, type Ref } from 'vue';

import { interpolatePath, useApi } from './useApi';

const { capturedOptions, mockFetchInstance } = vi.hoisted(() => {
  const options: { value: Record<string, unknown> } = { value: {} };
  const instance = vi.fn();
  return { capturedOptions: options, mockFetchInstance: instance };
});

vi.mock('ofetch', () => ({
  ofetch: {
    create: (opts: Record<string, unknown>) => {
      capturedOptions.value = opts;
      return mockFetchInstance;
    },
  },
}));

function mockUseRuntimeConfig() {
  return {
    public: {
      apiBaseUrl: 'http://localhost:2187',
    },
    app: {
      baseURL: '/',
    },
  };
}

const mockAuthCookie: Ref<string | null> = ref('true');
function mockUseAuthCookie() {
  return mockAuthCookie;
}

const mockOnConnectionLost = vi.fn();
const mockOnConnectionRestored = vi.fn();
function mockUseConnectionHealth() {
  return {
    onConnectionLost: mockOnConnectionLost,
    onConnectionRestored: mockOnConnectionRestored,
  };
}

const mockRouterPush = vi.fn();
function mockUseRouter() {
  return { push: mockRouterPush };
}

vi.stubGlobal('useRuntimeConfig', mockUseRuntimeConfig);
vi.stubGlobal('useAuthCookie', mockUseAuthCookie);
vi.stubGlobal('useConnectionHealth', mockUseConnectionHealth);
vi.stubGlobal('useRouter', mockUseRouter);

describe('interpolatePath', () => {
  it('leaves static paths unchanged', () => {
    expect(interpolatePath('/preview')).toBe('/preview');
  });

  it('replaces path parameters', () => {
    expect(interpolatePath('/custom-rules/{id}', { id: 42 })).toBe('/custom-rules/42');
  });

  it('encodes reserved characters in path values', () => {
    expect(interpolatePath('/deletion-queue', { unused: 'x' })).toBe('/deletion-queue');
    expect(interpolatePath('/items/{name}', { name: 'Firefly/Serenity' })).toBe(
      '/items/Firefly%2FSerenity',
    );
  });

  it('throws when a path parameter is missing', () => {
    expect(() => interpolatePath('/custom-rules/{id}', {})).toThrow('Missing path parameter: id');
  });
});

describe('useApi', () => {
  beforeEach(() => {
    capturedOptions.value = {};
    mockFetchInstance.mockReset();
    mockOnConnectionLost.mockReset();
    mockOnConnectionRestored.mockReset();
    mockRouterPush.mockReset();
    mockAuthCookie.value = 'true';
  });

  describe('return value', () => {
    it('returns GET/POST/PUT/PATCH/DELETE methods, not the raw ofetch instance', () => {
      const api = useApi();
      expect(api).not.toBe(mockFetchInstance);
      expect(typeof api.GET).toBe('function');
      expect(typeof api.POST).toBe('function');
      expect(typeof api.PUT).toBe('function');
      expect(typeof api.PATCH).toBe('function');
      expect(typeof api.DELETE).toBe('function');
    });
  });

  describe('ofetch.create configuration', () => {
    it('sets baseURL from runtime config', () => {
      useApi();
      expect(capturedOptions.value.baseURL).toBe('http://localhost:2187');
    });

    it('sets credentials to "include" for cookie-based auth', () => {
      useApi();
      expect(capturedOptions.value.credentials).toBe('include');
    });
  });

  describe('typed methods', () => {
    it('GET prefixes /api/v1 and returns the JSON body', async () => {
      mockFetchInstance.mockResolvedValueOnce({ title: 'Firefly' });

      const api = useApi();
      const result = await api.GET('/preview');

      expect(mockFetchInstance).toHaveBeenCalledWith('/api/v1/preview', { method: 'GET' });
      expect(result).toEqual({ title: 'Firefly' });
    });

    it('GET passes query parameters through to ofetch', async () => {
      mockFetchInstance.mockResolvedValueOnce([]);

      const api = useApi();
      await api.GET('/preview', { query: { force: true } });

      expect(mockFetchInstance).toHaveBeenCalledWith('/api/v1/preview', {
        method: 'GET',
        query: { force: true },
      });
    });

    it('POST interpolates path params and sends the body', async () => {
      mockFetchInstance.mockResolvedValueOnce({ status: 'approved' });

      const api = useApi();
      await api.POST('/approval-queue/{id}/approve', { path: { id: 7 } });

      expect(mockFetchInstance).toHaveBeenCalledWith('/api/v1/approval-queue/7/approve', {
        method: 'POST',
      });
    });

    it('DELETE sends required query params', async () => {
      mockFetchInstance.mockResolvedValueOnce(undefined);

      const api = useApi();
      await api.DELETE('/deletion-queue', {
        query: { mediaName: 'Firefly', mediaType: 'show' },
      });

      expect(mockFetchInstance).toHaveBeenCalledWith('/api/v1/deletion-queue', {
        method: 'DELETE',
        query: { mediaName: 'Firefly', mediaType: 'show' },
      });
    });
  });

  describe('onResponse interceptor', () => {
    it('calls onConnectionRestored on any successful response', () => {
      useApi();
      const onResponse = capturedOptions.value.onResponse as () => void;
      expect(onResponse).toBeDefined();

      onResponse();
      expect(mockOnConnectionRestored).toHaveBeenCalledTimes(1);
    });
  });

  describe('onResponseError interceptor', () => {
    it('clears auth cookie and redirects to /login on 401', () => {
      useApi();
      const onResponseError = capturedOptions.value.onResponseError as (ctx: {
        response: { status: number };
      }) => void;
      expect(onResponseError).toBeDefined();

      onResponseError({ response: { status: 401 } });

      expect(mockAuthCookie.value).toBeNull();
      expect(mockRouterPush).toHaveBeenCalledWith('/login');
    });

    it('does not redirect on non-401 errors', () => {
      useApi();
      const onResponseError = capturedOptions.value.onResponseError as (ctx: {
        response: { status: number };
      }) => void;

      onResponseError({ response: { status: 500 } });

      expect(mockAuthCookie.value).toBe('true');
      expect(mockRouterPush).not.toHaveBeenCalled();
    });

    it('calls onConnectionRestored even on error responses (backend is reachable)', () => {
      useApi();
      const onResponseError = capturedOptions.value.onResponseError as (ctx: {
        response: { status: number };
      }) => void;

      onResponseError({ response: { status: 500 } });
      expect(mockOnConnectionRestored).toHaveBeenCalledTimes(1);
    });
  });

  describe('onRequestError interceptor', () => {
    it('calls onConnectionLost on network-level failures', () => {
      useApi();
      const onRequestError = capturedOptions.value.onRequestError as () => void;
      expect(onRequestError).toBeDefined();

      onRequestError();
      expect(mockOnConnectionLost).toHaveBeenCalledTimes(1);
    });
  });

  describe('error handling', () => {
    it('propagates fetch errors to the caller', async () => {
      const networkError = new Error('Network error');
      mockFetchInstance.mockRejectedValueOnce(networkError);

      const api = useApi();
      await expect(api.GET('/preview')).rejects.toThrow('Network error');
    });

    it('propagates HTTP errors from ofetch', async () => {
      const httpError = Object.assign(new Error('Not Found'), {
        statusCode: 404,
        data: { error: 'Serenity not found' },
      });
      mockFetchInstance.mockRejectedValueOnce(httpError);

      const api = useApi();
      await expect(api.GET('/custom-rules/{id}/impact', { path: { id: 999 } })).rejects.toThrow(
        'Not Found',
      );
    });
  });
});
