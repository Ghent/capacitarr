import { ofetch, type $Fetch } from 'ofetch';
import type { paths } from '~/types/generated/openapi';

/**
 * Typed REST client over generated OpenAPI paths.
 *
 * Paths are spec keys (`/preview`, `/custom-rules/{id}`). The wrapper prefixes
 * `/api/v1` and interpolates `{param}` from `opts.path`.
 *
 * Login, migration, connection-health polling, and EventSource stay on raw
 * ofetch — they are not REST resources this client owns.
 */

const API_PREFIX = '/api/v1';

type HttpMethod = 'get' | 'put' | 'post' | 'delete' | 'patch';

type PathsWithMethod<M extends HttpMethod> = {
  [P in keyof paths]: paths[P][M] extends { responses: unknown } ? P : never;
}[keyof paths];

type Operation<P extends keyof paths, M extends HttpMethod> = paths[P][M];

type IsNever<T> = [T] extends [never] ? true : false;

type PathParams<Op> = Op extends { parameters: { path?: infer P } }
  ? IsNever<P> extends true
    ? undefined
    : P
  : undefined;

type QueryParams<Op> = Op extends { parameters: { query?: infer Q } }
  ? IsNever<Q> extends true
    ? undefined
    : Q
  : undefined;

type PathRequired<Op> = Op extends { parameters: { path: infer P } }
  ? undefined extends P
    ? false
    : IsNever<P> extends true
      ? false
      : true
  : false;

type QueryRequired<Op> = Op extends { parameters: { query: infer Q } }
  ? undefined extends Q
    ? false
    : IsNever<Q> extends true
      ? false
      : true
  : false;

type JsonBody<Op> = Op extends { requestBody: { content: { 'application/json': infer B } } }
  ? B
  : undefined;

type BodyRequired<Op> = Op extends { requestBody: infer RB }
  ? undefined extends RB
    ? false
    : IsNever<RB> extends true
      ? false
      : true
  : false;

type SuccessStatus = 200 | 201 | 204;

type JsonContent<R> = R extends { content: { 'application/json': infer C } } ? C : undefined;

type SuccessData<Op> = Op extends { responses: infer R }
  ? { [S in keyof R]: S extends SuccessStatus ? JsonContent<R[S]> : never }[keyof R]
  : undefined;

type RequestOpts<Op> = (PathRequired<Op> extends true
  ? { path: NonNullable<PathParams<Op>> }
  : object) &
  (QueryRequired<Op> extends true
    ? { query: NonNullable<QueryParams<Op>> }
    : QueryParams<Op> extends undefined
      ? object
      : { query?: NonNullable<QueryParams<Op>> }) &
  (BodyRequired<Op> extends true
    ? { body: NonNullable<JsonBody<Op>> }
    : JsonBody<Op> extends undefined
      ? object
      : { body?: NonNullable<JsonBody<Op>> });

type NeedsArg<Op> =
  PathRequired<Op> extends true
    ? true
    : QueryRequired<Op> extends true
      ? true
      : BodyRequired<Op> extends true
        ? true
        : false;

type MethodFn<M extends HttpMethod> = <P extends PathsWithMethod<M>>(
  path: P,
  ...args: NeedsArg<Operation<P, M>> extends true
    ? [opts: RequestOpts<Operation<P, M>>]
    : [opts?: RequestOpts<Operation<P, M>>]
) => Promise<SuccessData<Operation<P, M>>>;

export interface ApiClient {
  GET: MethodFn<'get'>;
  POST: MethodFn<'post'>;
  PUT: MethodFn<'put'>;
  PATCH: MethodFn<'patch'>;
  DELETE: MethodFn<'delete'>;
}

export function interpolatePath(
  template: string,
  params?: Record<string, string | number | undefined>,
): string {
  return template.replace(/\{([^}]+)\}/g, (_, key: string) => {
    const value = params?.[key];
    if (value === undefined || value === null || value === '') {
      throw new Error(`Missing path parameter: ${key}`);
    }
    return encodeURIComponent(String(value));
  });
}

function createClient(apiFetch: $Fetch): ApiClient {
  const request = ((
    method: HttpMethod,
    path: string,
    opts?: { path?: Record<string, string | number>; query?: unknown; body?: unknown },
  ) => {
    const url = `${API_PREFIX}${interpolatePath(path, opts?.path)}`;
    const init: {
      method: string;
      query?: Record<string, unknown>;
      body?: Record<string, unknown> | unknown[];
    } = {
      method: method.toUpperCase(),
    };
    if (opts?.query !== undefined) init.query = opts.query as Record<string, unknown>;
    if (opts?.body !== undefined) {
      init.body = opts.body as Record<string, unknown> | unknown[];
    }
    return apiFetch(url, init);
  }) as (method: HttpMethod, path: string, opts?: object) => Promise<unknown>;

  return {
    GET: (path, ...args) => request('get', path, args[0]) as never,
    POST: (path, ...args) => request('post', path, args[0]) as never,
    PUT: (path, ...args) => request('put', path, args[0]) as never,
    PATCH: (path, ...args) => request('patch', path, args[0]) as never,
    DELETE: (path, ...args) => request('delete', path, args[0]) as never,
  };
}

export const useApi = (): ApiClient => {
  const config = useRuntimeConfig();
  const authenticated = useAuthCookie();
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

  return createClient(apiFetch);
};
