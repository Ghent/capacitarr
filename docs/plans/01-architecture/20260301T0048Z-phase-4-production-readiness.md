# Capacitarr Phase 4: Production Readiness

**Date:** 2026-03-01
**Status:** ✅ Complete — All 12 phases implemented and verified.
**Supersedes:** Remaining items from Phase 3 (`20260228T0600Z`), Theming (`20260228T1620Z`), and Visual Polish (`20260228T2025Z`)

This plan consolidates all outstanding work from previous plans plus new requirements into a single prioritized execution order. Previous plan files have been updated to mark deferred items as "→ Deferred to Phase 4".

---

## Prior Plan Status Summary

| Plan File | Status |
|-----------|--------|
| `20260227T0001Z-implementation-plan.md` | ✅ Complete (historical) |
| `20260227T1235Z-scoring-design.md` | ✅ Complete (historical) |
| `20260228T0130Z-v2-migration-plan.md` | ✅ Complete (historical) |
| `20260228T0600Z-phase-3-features-and-polish.md` | ✅ Complete — remaining items deferred here |
| `20260228T0648Z-structured-score-transparency.md` | ✅ Complete (historical) |
| `20260228T1620Z-theming-and-shadcn-integration.md` | ✅ Complete — visual quality items deferred here |
| `20260228T2025Z-visual-design-polish.md` | ⏸ Deferred — entire plan absorbed into Phase 9 below |

---

## Execution Order

### Phase 1: Poller Concurrency Guard

**Effort:** XS
**Source:** New (discovered during plan review)

**Problem:** The `poll()` function in `poller.go` has no mutex or atomic guard. If a ticker fires while a manual `poll()` (via Run Now) is still running, both execute concurrently. Both would independently call `evaluateAndCleanDisk()`, potentially enqueuing the same media items twice into the delete queue.

**Fix:** Add an `atomic.Bool` guard at the top of `poll()`:

```go
var pollRunning atomic.Bool

func poll() {
    if !pollRunning.CompareAndSwap(false, true) {
        slog.Info("Poller: skipping — previous run still in progress")
        return
    }
    defer pollRunning.Store(false)
    // ... rest of poll()
}
```

**Files:**
- `backend/internal/poller/poller.go` — add `pollRunning` guard

---

### Phase 2: Goose Migration Framework

**Effort:** M
**Source:** New

Set up [goose](https://github.com/pressly/goose) as the sole schema management tool. Goose handles ALL schema changes — the baseline migration captures the current schema in full, and all future changes go through Goose SQL files only. Remove `AutoMigrate` entirely.

**Implementation:**
1. Add `github.com/pressly/goose/v3` dependency
2. Create `backend/migrations/` directory for SQL migration files
3. Create initial baseline migration (`00001_baseline.sql`) that captures the current schema in full (no-op for existing databases, creates all tables for fresh installs)
4. Add migration runner in `backend/internal/db/migrate.go` using `embed.FS` for Docker single-binary deployment
5. Replace `AutoMigrate` call in `db.Init()` with the Goose migration runner
6. Migration file format: `-- +goose Up` / `-- +goose Down` markers in single files

**Files:**
- `backend/go.mod` — add goose dependency
- `backend/internal/db/migrate.go` — new migration runner
- `backend/internal/db/db.go` — replace `AutoMigrate` with Goose migration runner
- `backend/internal/db/migrations/00001_baseline.sql` — full current schema as baseline

---

### Phase 3: Reverse Proxy & Auth Header Support

**Effort:** S-M
**Source:** New

Complete the reverse proxy support that's partially implemented and add trusted reverse proxy authentication headers for users who deploy Authelia, Authentik, or Organizr in front of Capacitarr. The `BASE_URL` env var and API routing already work, but several gaps remain.

**3.1 Reverse Proxy Fixes:**
1. Set JWT cookie `Path` to `cfg.BaseURL` instead of hardcoded `/`
2. Add `NUXT_APP_BASE_URL` env var support in `nuxt.config.ts` for build-time asset path configuration
3. Document deployment patterns (subdirectory via Traefik/Caddy/nginx, subdomain)

**3.2 Proxy Auth Header Support (Authelia/Authentik/Organizr):**

This is **additive** — built-in JWT auth remains the primary mechanism.

1. Add `AUTH_HEADER` env var to `Config` (e.g., `Remote-User`, `X-authentik-username`)
2. In auth middleware: if `AUTH_HEADER` is configured and the header is present + non-empty, trust it and skip JWT validation
3. Auto-create user record if the header user doesn't exist in `AuthConfig`

**Files:**
- `backend/routes/api.go` — fix cookie path
- `backend/internal/config/config.go` — add `AuthHeader` field
- `backend/routes/middleware.go` — add header-based auth bypass
- `frontend/nuxt.config.ts` — read `NUXT_APP_BASE_URL` env var
- `README.md` or `docs/deployment.md` — reverse proxy configuration examples

---

### Phase 4: Settings Restructure + Configurable Poll Interval

**Effort:** M
**Source:** New

Restructure the settings page into tabs and add a configurable poll interval to replace the hardcoded 15-second ticker.

**Backend:**
1. Add `PollIntervalSeconds` field to `PreferenceSet` model (default: 300, minimum: 30)
2. Modify poller to read interval from DB on each tick and adjust dynamically
3. Include `PollIntervalSeconds` in the existing `GET/PUT /api/v1/preferences` endpoints

**Frontend:**
1. Split settings page into 3 tabs: **General** | **Integrations** | **Authentication**
2. General tab: poll interval dropdown (30s, 1m, 5m, 15m, 30m, 1h), execution mode, log level, audit retention
3. Integrations tab: existing integration cards
4. Authentication tab: password change form (Phase 5)

**Files:**
- `backend/internal/db/models.go` — add `PollIntervalSeconds` to `PreferenceSet`
- `backend/internal/poller/poller.go` — dynamic interval from DB
- `backend/routes/rules.go` — include new field in preferences API
- `frontend/app/pages/settings.vue` — tab layout restructure

---

### Phase 5: Admin Password Management

**Effort:** S
**Source:** New

Add the ability to change the admin password from the UI.

**Implementation:**
1. `PUT /api/v1/auth/password` endpoint — requires current password + new password, validates current, hashes new with bcrypt
2. Frontend form in Settings > Authentication tab with current password, new password, confirm password fields
3. Success toast + redirect to login on password change

**Files:**
- `backend/routes/api.go` — add password change endpoint
- `frontend/app/pages/settings.vue` — Authentication tab content

---

### Phase 6: API Key Authentication

**Effort:** S
**Source:** New

Enable the existing `AuthConfig.APIKey` field for authentication via `X-Api-Key` header or `?apikey=` query parameter. This enables external tool integration.

**Implementation:**
1. Generate API key on first user creation (or via Settings > Authentication)
2. In auth middleware: check `X-Api-Key` header or `apikey` query param before JWT
3. Display/regenerate API key in Settings > Authentication tab

**Files:**
- `backend/routes/middleware.go` — add API key auth check
- `backend/routes/api.go` — API key generation/regeneration endpoint
- `frontend/app/pages/settings.vue` — display API key with copy button

---

### Phase 7: New Integrations + Service-Specific Rule Fields

**Effort:** L
**Source:** Phase 3 §4 + §2.4 (deferred)

Complete the Tautulli, Overseerr/Jellyseerr, and Lidarr integrations that are currently scaffolded but not fully connected. Simultaneously add service-specific matchers to the custom rules engine so rules can target fields unique to each integration (e.g., quality profile, language, tag, monitored status, request count).

**7.1 Tautulli** — Enrich media items with watch history data from Tautulli's API
**7.2 Overseerr/Jellyseerr** — Track media requests to factor into scoring (requested = higher value)
**7.3 Lidarr** — Full music library integration (media listing, disk space, deletion)
**7.4 Service-specific rule matchers** — Extend the custom rules engine with fields from each integration type. The rule builder UI dynamically shows available fields based on which integrations are configured.

**Files:**
- `backend/internal/integrations/tautulli.go` — complete integration
- `backend/internal/integrations/overseerr.go` — complete integration
- `backend/internal/integrations/lidarr.go` — complete integration
- `backend/internal/poller/poller.go` — wire integrations into poll loop
- `backend/internal/engine/rules.go` — new field matchers per integration type
- `backend/internal/db/models.go` — extend `ProtectionRule` fields
- `frontend/app/pages/rules.vue` — dynamic field options based on integration type

---

### Phase 8: Show/Season Grouping in Audit

**Effort:** M
**Source:** Phase 3 §3.3 (deferred)

Group show-level and season-level entries in the audit log and preview tables into a tree view, so "The Strain" shows as a parent with "Season 1", "Season 2" as collapsible children.

**Files:**
- `backend/routes/audit.go` — add grouping query option
- `frontend/app/pages/audit.vue` — tree/collapsible rendering

---

### Phase 9: Visual Design Polish

**Effort:** L
**Source:** `20260228T2025Z-visual-design-polish.md` (deferred in full)

Comprehensive visual quality pass over the entire UI. Done near the end because all structural/feature work is complete, so this is a single pass over the final surface area.

**⚠️ Branch strategy:**

1. Create `feature/visual-polish-apexcharts` branch from `main`
2. Implement ALL visual fixes on `feature/visual-polish-apexcharts`:
   - Fix broken sliders (shadcn-vue Slider thumb not rendering)
   - Install Geist fonts — Geist Sans (body) + Geist Mono (numbers/scores)
   - Typography hierarchy — heading sizes, weights, muted text, section labels
   - Dark mode border fix — white borders → `border-border` semantic class
   - Navbar overlap fix — content hidden behind fixed navbar
   - Dropdown background fix — transparent backgrounds on select/dropdown
   - Execution mode card active states — visual distinction for selected mode
   - Preset chip contrast — more visual weight on scoring presets
   - Section separators — themed dividers instead of plain white lines
3. Fix ApexCharts theme integration on `feature/visual-polish-apexcharts` — oklch colors matched to theme
4. Create `feature/visual-polish-uplot` branch from `feature/visual-polish-apexcharts` (inherits all visual fixes)
5. On `feature/visual-polish-uplot` only: replace ApexCharts with uPlot in `CapacityChart.vue`
6. Compare both branches side-by-side, pick the winner, merge that branch to `main`

**Full specification:** See `20260228T2025Z-visual-design-polish.md` for detailed implementation notes per issue.

---

### Phase 10: Run Now UX Polish

**Effort:** S
**Source:** Existing feature enhancement

The Run Now button and `POST /api/v1/run-now` endpoint already work. Polish the UX after visual design is finalized:

1. Show a spinner/loading state on the button while the run is in progress
2. Disable the button while a run is already in progress (read from `/api/v1/worker/stats`)
3. Show a toast with results when the run completes (items evaluated, actions taken, bytes freed)

**Files:**
- `frontend/app/pages/index.vue` — Run Now button UX improvements

---

### Phase 11: Touchscreen Support + Pull-to-Refresh

**Effort:** S-M
**Source:** New

1. Add `@media (hover: hover)` guards around hover-dependent effects (glassmorphism, tooltips)
2. Ensure minimum 44×44px tap targets on all interactive elements
3. Implement pull-to-refresh gesture on dashboard and audit pages
4. Test slider interaction on touch devices

**Files:**
- `frontend/app/assets/css/main.css` — hover media queries
- `frontend/app/composables/usePullToRefresh.ts` — new composable
- `frontend/app/pages/index.vue` — pull-to-refresh integration
- `frontend/app/pages/audit.vue` — pull-to-refresh integration

---

### Phase 12: Frontend/Backend Decoupling & OpenAPI Spec

**Effort:** M
**Source:** Architecture discussion (2026-03-01)

Formalize the clean separation between the Capacitarr REST API and the premium Vue/Nuxt frontend, enabling third-party frontends (React, Svelte, mobile apps, CLI tools) to plug into the backend.

**Current state (already decoupled at ~90%):**
- All endpoints are under `/api/v1/` with standard JSON request/response, no SSR
- CORS middleware already exists (activated via `CORS_ORIGINS` env var)
- Auth supports 4 methods: cookie JWT, `Authorization: Bearer`, `X-Api-Key` header, proxy auth header
- Login endpoint already returns the JWT in the response body for non-cookie clients

**Remaining work:**

1. **OpenAPI 3.1 specification** — Write/generate a complete OpenAPI spec for all `/api/v1/` endpoints. This is the #1 enabler for third-party developers.
2. **Optional frontend embedding** — Make `go:embed frontend/dist` conditional so the backend can be built as an API-only binary (headless mode).
3. **Headless Docker image variant** — Dockerfile target that skips the frontend build stage, producing a smaller API-only image.
4. **CORS documentation** — Document `CORS_ORIGINS` env var and headless deployment patterns.
5. **API versioning strategy** — Formalize the `/api/v1/` contract: what's stable, what's experimental.

**Deployment models enabled:**
- **Bundled (current)** — Single container with embedded frontend
- **Headless** — API-only container; users bring their own frontend
- **Separate services** — Backend + frontend as independent docker-compose services

**Files:**
- `docs/api/openapi.yaml` — OpenAPI 3.1 specification
- `backend/main.go` — conditional frontend embedding
- `Dockerfile` — multi-target build (bundled vs headless)
- `docs/deployment.md` — headless deployment documentation

---

## Deferred to Advanced Configuration Backlog

These items are tracked in `20260301T0048Z-advanced-configuration-backlog.md` and will be implemented when demand warrants:

1. **Manual deletion rate limiting configuration** — User-configurable deletion throttle (currently hardcoded 3s in `poller.go`)
2. **Manual cron entry configuration** — Power-user cron expression for poll schedule (beyond the simple presets in Phase 4)

---

## Implementation Summary

| Phase | Item | Effort | Type |
|-------|------|--------|------|
| 1 | Poller concurrency guard | XS | Bug fix |
| 2 | Goose migration framework | M | Infrastructure |
| 3 | Reverse proxy & auth header support | S-M | Deployment + Feature |
| 4 | Settings restructure + poll interval | M | Feature + UX |
| 5 | Admin password management | S | Security |
| 6 | API key authentication | S | Feature |
| 7 | New integrations + service-specific rule fields | L | Feature |
| 8 | Show/season grouping in audit | M | UX |
| 9 | Visual design polish (on `feature/visual-polish-apexcharts` branch) | L | Visual |
| 10 | Run Now UX polish | S | UX |
| 11 | Touchscreen + pull-to-refresh | S-M | Accessibility |
| 12 | Frontend/backend decoupling & OpenAPI spec | M | Architecture |

---

## Completion Status (2026-03-02)

All 12 phases have been implemented and verified against the codebase:

| Phase | Item | Status |
|-------|------|--------|
| 1 | Poller concurrency guard | ✅ `pollRunning atomic.Bool` in `poller.go` |
| 2 | Goose migration framework | ✅ `migrate.go` + 8 migration files |
| 3 | Reverse proxy & auth header | ✅ `AuthHeader` config, proxy auth middleware, cookie path fix |
| 4 | Settings restructure + poll interval | ✅ 4-tab layout, `PollIntervalSeconds` in preferences |
| 5 | Admin password management | ✅ `PUT /auth/password` + `PUT /auth/username` |
| 6 | API key authentication | ✅ SHA-256 hashed keys, `POST /auth/apikey` |
| 7 | New integrations + service-specific rules | ✅ Tautulli, Overseerr, Lidarr, Readarr, Jellyfin, Emby |
| 8 | Show/season grouping | ✅ `groupedLogs` in audit, `groupedPreview` in rules |
| 9 | Visual design polish | ✅ UiSlider, Geist fonts, `useThemeColors.ts` |
| 10 | Run Now UX polish | ✅ `EngineControlPopover.vue` with spinner + stats |
| 11 | Touchscreen + pull-to-refresh | ✅ `usePullToRefresh.ts` composable |
| 12 | Frontend/backend decoupling & OpenAPI | ✅ OpenAPI spec, API docs, versioning docs. Headless mode will not be implemented — superseded by formalized API infrastructure. |
