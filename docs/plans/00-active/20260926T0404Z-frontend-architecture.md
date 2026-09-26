# Frontend Architecture

**Status:** Planned
**Priority:** High (trust, then ownership)
**Origin:** Supersedes `20260408T1336Z-frontend-architecture-polish`. Principal-engineer review of `main` against that April plan, after four-mode, scale-model, trust-path, and API contracts.

The April plan is archived. Do not execute it. Line numbers, “two largest pages,” “edit all 22 locale files,” and “toast every console.warn” are stale or wrong.

---

## Summary

Give the frontend the same treatment the backend just got: **ownership, visible failure, no silent skip.**

The UI already has the right primitives — generated OpenAPI types, `diskGroupMode.ts`, vue-sonner, `ConnectionBanner`, preview `truncated` / `totalItems` on the composable. It does not use them consistently. Failed fetches look like empty libraries. Queue-full skips toast as success. SSE reconnect drops the ring buffer. That is a trust problem, not a line-count problem.

---

## Current model (all true today)

- Nuxt 4 SPA (`ssr: false`), shadcn-vue / Reka UI, ECharts, `@tanstack/vue-virtual`, `@nuxtjs/i18n`, vue-sonner, PWA via `@vite-pwa/nuxt`.
- Pages orchestrate some domains (`useEngineControl`, `usePreview`, `useApprovalQueue`) and still **own** others (`index.vue` sparkline + ~40-event activity SSE + event-icon switches; `help.vue` is 1202 lines of copy-pasted `<details>`).
- `useApi()` is untyped `ofetch`. Call sites cast. Types are generated; fetch is not.
- `useEventStream` is a module singleton. `lastEventId` is stored and **never sent** on reconnect. `replay_gap` is dispatched and **has no consumer**. Custom backoff closes `EventSource`, so the browser cannot attach `Last-Event-ID` itself.
- `EVENT_DELETION_QUEUE_FULL` is defined and unused. `usePreview` exposes `truncated` / `totalItems`; `library.vue` does not bind them. `POST /api/v1/delete` returns `queueFullSkipped`; the library delete handler casts `{ queued, total, mode }` and toasts success.
- `useApprovalQueue` does not subscribe to `approval_approved` / `approval_rejected`. Those refresh only while the dashboard is mounted.
- Fetch failures are `console.warn` plus empty arrays. `DashboardEmptyState`, “no items pending,” and an empty library are indistinguishable from a failed GET.
- `useConnectionHealth` reads `useCookie('authenticated')` **without** `path: app.baseURL`. `useAuthCookie` exists specifically so subdirectory deploys do not split that cookie.
- Date-range **keys** exist (`dashboard.lastHour` … `dashboard.allTime`) and are unused. Collection Deletion help + settings copy is still raw English. Action toasts in `useApprovalQueue`, `rules.vue`, and several settings files are raw English. `en.json` is source of truth; the other 21 locale files are `.cursorignore` copies.
- `frontend/README.md` is still the Nuxt UI starter template. `CONTRIBUTING.md` still says “use ECharts via DashboardCard” — that component does not exist.
- Unused UI directories: `components/ui/command`, `ui/combobox`, `ui/skeleton`. ECharts plugin registers `BarChart`; every chart is `line` or `gauge`.
- File sizes that matter for later extraction, not for this plan’s first slices: `index.vue` 1235, `help.vue` 1202, `LibraryTable.vue` 1137, `RuleDiskThresholds.vue` 806, `SettingsIntegrations.vue` 783.

---

## Chosen direction

Match the backend rules already on `main`:

| Backend (already true) | Frontend (this plan) |
|---|---|
| Silent skip is forbidden | Failed fetch ≠ empty. Queue-full ≠ success toast. Truncated preview ≠ “the library.” |
| Domain modules, not god functions | Pages compose. Domain maps live in `utils/` (`diskGroupMode.ts` pattern). Cards are views. SSE lives in the composable that owns the state. |
| Generated contracts, CI drift check | Types stay generated. Fetch wrapping is a later slice, not a rewrite of every call site first. |
| One owner per concern | `app.vue` owns the SSE connection and `replay_gap`. Each queue/preview/engine composable owns its event list. Dashboard does not re-own engine or approval SSE. |

**Composition rule** (copy into `docs/development.md` when the first extract ships):

1. Pages orchestrate. They do not contain icon/color/label switches or ECharts option builders.
2. Shared domain maps live in `utils/` (see `diskGroupMode.ts`). New mode or event-type maps go there, not into a page.
3. Feature cards are presentational. They consume composables; they do not register app-lifetime SSE.
4. Fetch errors go through the error policy below. `console.warn` alone is not an error strategy.

**Error policy** (copy into `docs/development.md` when Slice A ships):

| Category | Treatment | Examples |
|---|---|---|
| **Initial page load** | Inline error + retry. Keep last-good data if any. Never show the empty-state component. | Dashboard disk groups, preview, rules list, approval queue first fetch |
| **User-initiated action** | Error toast (`$t(...)`). | Run now, approve, save weights, manual delete |
| **Background refresh** | Stay silent unless consecutive failures. Do not fight `ConnectionBanner`. | SSE-triggered refetches, integration refresh |
| **Parse / display** | Inline fallback + `console.warn`. | Score-detail JSON parse |
| **Background polling** | Silent. Comment the site. | Connection health poll, automatic version check |

Do not invent a “3 failures then toast” counter without a shared helper. Prefer `useFetchStatus` (or equivalent) with `ok | loading | error` and `lastGood`.

**i18n process** (current, keep it):

- Edit `frontend/app/locales/en.json` only.
- Other locale files stay copies until a real translation pass. Do not resurrect the “add the key to all 22 files” ritual.
- New user-visible strings, including toasts, go through `$t()` / `t()`.

**SSE reconnect** (absorbs the scale-model later item):

- `connect()` must send `Last-Event-ID` on every reconnect (header on a fetch-based stream, or a documented query if the EventSource constructor stays).
- `app.vue` registers `replay_gap` once and triggers a coordinated refetch (preview, approval, deletion, sunset, engine stats, dashboard history).
- Scale-model’s “Later PRs” line for this work now points here. Do not implement it twice.

---

## Slices (separate PRs)

CONTRIBUTING: one logical change per PR. Conventional Commits. Branch prefix `refactor/` (or `fix/` when the slice is a user-visible defect).

### Slice A — Trust (do this first)

User-visible lies. No page decomposition.

1. **Failed vs empty.** Dashboard, library/preview, approval queue, deletion queue, snoozed items: on fetch failure do not replace last-good data with `[]`. Show an inline error + retry on first load. `DashboardEmptyState` only when the last fetch **succeeded** and there are no groups.
2. **Preview cap honesty.** Bind `truncated` / `totalItems` from `usePreview` on the library page. Banner when truncated: the table is a slice; client filters do not see the rest. Do not imply the full library.
3. **Queue-full is visible.** Cast/type the delete response to include `queueFullSkipped`. Warning toast when `> 0`. Subscribe to `EVENT_DELETION_QUEUE_FULL` in `useDeletionQueue` (banner or toast + do not no-op). Surface `queueFullRejections` if the worker-stats payload is already on the client.
4. **Auth cookie path.** `useConnectionHealth` must use `useAuthCookie()`, not raw `useCookie('authenticated')`.
5. **OpenAPI.** Add `queueFullSkipped` (and `mode` if missing) to `POST /api/v1/delete` so `make api:generate` matches Go. Do not leave the frontend casting a smaller shape than the backend returns.

Tests: preview already has a truncated unit test — add a library-level assertion or composable test that failure does not clear items. Deletion/approval fetch-failure tests. `make ci`.

Copy the error policy into `docs/development.md`.

### Slice B — SSE correctness

1. Send `Last-Event-ID` on reconnect. One `connect()` path.
2. `replay_gap` → `refetchAll()` from `app.vue` (or a tiny shell composable it owns).
3. Move `approval_approved` / `approval_rejected` into `useApprovalQueue`. Remove the dashboard-only copies.
4. `ACTIVITY_FEED_EVENT_TYPES` in `constants.ts`. `index.vue` stops listing raw strings.
5. `import.meta.hot.dispose` on `useEventStream` (`disconnect()`, clear maps) and the singleton once-flags. Reuse `_resetSSERegistration` / `_resetDeletionQueueSSE` / `_resetSnoozedItemsSSE` — do not invent a second reset API. The snippet in the April plan (`disconnectSSE()` inside the SSE module) is the wrong name.

Manual: one EventSource after HMR; events not double-handled. Tests for reconnect header / query and `replay_gap` dispatch.

### Slice C — Dashboard ownership

Extract **ownership**, not “get `index.vue` under 500 lines.”

1. `utils/eventIcons.ts` — `eventIcon` / `eventIconClass` (same pattern as `diskGroupMode.ts`).
2. `diskGroupMode.ts` grows `modeLabel()` and `mostAggressiveMode()`. Delete the duplicate priority switch in `index.vue` and the local label switch in `DiskGroupSection.vue`.
3. `useEngineHistory` — sparkline bucketing, ECharts option builders, date range. Wire the existing `dashboard.lastHour` keys.
4. `EngineActivityCard.vue` is a **view**. It consumes `useEngineControl` + `useEngineHistory`. It does not register SSE.
5. Leave activity-feed SSE on the page or a `useDashboardActivity` composable — not in the card.

Tests move with the pure functions. `make ci`. Copy the composition rule into `docs/development.md`.

### Slice D — Help + Collection Deletion i18n

1. `components/help/HelpSection.vue` — `<details>` wrapper (`title`, optional `icon`, `defaultOpen`, slot).
2. `HelpAbout.vue` and `HelpCollectionDeletion.vue` only. Do **not** extract twelve FAQ files.
3. i18n the Collection Deletion section, About `techStack` / `credits`, and `SettingsIntegrations.vue` Collection Deletion strings. `en.json` only.

`help.vue` should compose sections. Tests: page still renders the same headings (i18n keys). `make ci`.

### Slice E — i18n honesty + dead surface

1. Wire keys that already exist and are ignored: `dashboard.errorBanner.*` (`IntegrationErrorBanner`), `notifications.level*` (`SettingsNotifications`), approval/rules toasts, `audit.loadingMore` / `audit.showingOf`, `dashboard.updated`.
2. Prune orphan `en.json` keys after wiring (legacy Insights/storage/chart clusters, `help.whatsNew.*` if the section is gone). Do not mass-edit the 21 copies.
3. Remove unused UI: `ui/command`, `ui/combobox`, `ui/skeleton` (app uses `SkeletonCard` / `SkeletonTable`). Drop unused leafs (`CardAction`, `TableCaption`, `TableEmpty`, `DialogScrollContent`, …) or leave a one-line note in `components.json` that they are CLI baggage — pick one and stick to it.
4. Drop unused `BarChart` from `plugins/echarts.client.ts`. Confirm `@tailwindcss/postcss` is unused and remove it if so.
5. Replace `frontend/README.md` (still the Nuxt starter) with Capacitarr frontend notes: `pnpm` scripts, `NUXT_PUBLIC_API_BASE_URL`, `make api:generate`, i18n source-of-truth, PWA denylist.
6. Fix `CONTRIBUTING.md`: “ECharts via DashboardCard” → ECharts via the dashboard/history components. Product docs must not name a deleted component.

### Slice F — Typed fetch (later)

Not a rewrite of the UI. Thin wrapper (`openapi-fetch` or a hand-rolled `api.GET('/api/v1/preview')`) over `generated/openapi.ts`. Deprecate bare `as` except SSE payloads in `api-extra.ts`. Only after A–B so we are not typing a lying client.

---

## Out of scope

- **Paginated preview / chunked cache** — already named on the scale-model plan.
- **Durable deletion queue** — scale-model later PR.
- **Boiling `LibraryTable`, `RuleDiskThresholds`, `SettingsIntegrations`** — apply the composition rule the next time those files are already open. Do not open a “make them smaller” PR.
- **Translating the 21 non-English locale files** — process issue, not this architecture.
- **vue-sonner, dead 2026-04 UI dirs, Radix → Lucide** — already done.
- **Frontend coverage as a goal** — add tests when logic moves (A–C). Do not start a coverage campaign.

---

## Risk

- **Slice A changes user-visible behavior.** That is the point. Empty-state copy will appear less often; errors will appear more. Prefer inline retry over toast spam. `ConnectionBanner` remains the connectivity signal.
- **Slice B is the only production SSE behavior change.** Wrong `Last-Event-ID` handling can replay too much or too little. Pair with `replay_gap` refetch so a bad resume still converges.
- **Slices C–E are structural / honesty.** Low runtime risk if tests move with extracts.
- **Slice F is optional sequencing.** Do not block A–E on it.

---

## Done when

- [x] April plan archived; this file is the active frontend architecture inbox
- [x] Slice A: failed ≠ empty; truncated banner; queue-full visible; auth cookie path fixed; delete response typed
- [x] Slice B: Last-Event-ID on reconnect; `replay_gap` refetch; approval SSE owned by `useApprovalQueue`; HMR dispose
- [x] Slice C: `eventIcons` + history composable + presentational engine card; date-range i18n wired
- [x] Slice D: Help wrapper + two extracts; Collection Deletion / About i18n
- [x] Slice E: existing keys wired or pruned; starter README gone; CONTRIBUTING does not mention DashboardCard
- [x] Error policy and composition rule live in `docs/development.md`, not only in this journal
- [ ] `make ci` green on each slice
