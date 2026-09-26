# Capacitarr frontend

Nuxt 4 SPA (`ssr: false`) for Capacitarr. shadcn-vue / Reka UI, ECharts, `@tanstack/vue-virtual`, `@nuxtjs/i18n`, vue-sonner, PWA via `@vite-pwa/nuxt`.

## Scripts

From this directory (`frontend/`):

```bash
pnpm install
pnpm dev          # http://localhost:3000
pnpm test
pnpm lint
pnpm typecheck
pnpm build
```

From the repo root, `make ci` runs the same lint/test/security images as GitHub Actions.

## API base URL

`NUXT_PUBLIC_API_BASE_URL` is the backend origin the browser talks to. Leave it empty when the UI is served from the same host as the API (the Docker image). Set it for split local development, e.g. `http://localhost:8080`.

Subdirectory deploys use `NUXT_APP_BASE_URL`. Auth cookies must go through `useAuthCookie()` so the path matches that base URL.

## Generated types

OpenAPI types live in `app/types/generated/`. After changing `docs/reference/api/openapi.yaml`:

```bash
make api:generate
```

Commit the result. CI fails if the generated file drifts. `useApi()` is still untyped `ofetch`; typed fetch is a later slice.

## i18n

`app/locales/en.json` is the source of truth. Edit that file only. The other locale files stay copies until a real translation pass.

New user-visible strings, including toasts, go through `$t()` / `t()`.

## PWA

`@vite-pwa/nuxt` caches static assets only. Workbox `navigateFallbackDenylist` excludes `/api/` so API responses are never cached as the SPA shell.
