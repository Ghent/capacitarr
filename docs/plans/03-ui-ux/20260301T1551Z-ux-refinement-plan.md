# UX Refinement Plan

**Date:** 2026-03-01
**Status:** ✅ Complete — All 8 phases implemented.
**Branch:** `feature/ux-refinement`

---

## Phase 1: Toast Z-Index Fix

**Effort:** XS
**Issue:** Toast popups render blurred behind dialog/modal backdrops when triggered from within a dialog (e.g., testing an integration connection). The `ToastContainer` uses `z-50` but dialogs use `z-50` too with a backdrop overlay.

**Fix:** Raise `ToastContainer` to `z-[100]` to always render above dialogs and their backdrops.

**Files:**
- `frontend/app/components/ToastContainer.vue` — change `z-50` → `z-[100]`

---

## Phase 2: Build Quality — Lint & Format

**Effort:** S
**Issue:** Warnings appear during Docker build because lint and format aren't enforced. The Dockerfile is NOT the place to catch code quality issues — all linting and formatting must happen BEFORE we reach the build step.

**Approach:** Pre-build quality gate via npm scripts and Makefile:

1. **Add lint/format scripts** to `package.json`:
   ```json
   "scripts": {
     "lint": "eslint .",
     "lint:fix": "eslint . --fix",
     "format": "prettier --write .",
     "format:check": "prettier --check .",
     "typecheck": "nuxt typecheck"
   }
   ```

2. **Add a Makefile** with quality targets:
   ```makefile
   .PHONY: lint format check build

   lint:
       cd frontend && pnpm lint:fix
       cd backend && go vet ./...

   format:
       cd frontend && pnpm format

   check: lint format
       cd frontend && pnpm format:check

   build: check
       docker compose up -d --build
   ```

3. **Fix all current warnings** — clean slate

4. **Document the workflow:** `make check` before every commit, `make build` for Docker. The Docker build itself assumes clean code.

5. **GitLab CI/CD ready:** The Makefile targets map directly to `.gitlab-ci.yml` pipeline stages when CI/CD is set up later:
   ```yaml
   lint:
     stage: test
     script:
       - cd frontend && pnpm install && pnpm lint && pnpm format:check
       - cd backend && go vet ./...
   ```

**Files:**
- `frontend/package.json` — add lint/format/typecheck scripts
- `Makefile` — quality targets
- Various source files — fix existing warnings

---

## Phase 3: Integration Setup Guidance + Overseerr Fix

**Effort:** S-M

### 3.1 Plex Setup Guide

Add a contextual help popover on the Plex integration form:
- Content: "Use your Plex server URL (e.g., `http://192.168.1.100:32400`). To find your token: go to any library item in Plex Web → Get Info → View XML → look for `X-Plex-Token` in the URL."
- Link to the official Plex support article

OAuth PIN flow is out of scope for now — can be added as a future enhancement.

### 3.2 Overseerr URL Handling

**Issue:** Overseerr behind a reverse proxy at a subpath (e.g., `https://www.starshadow.com/requests/`) doesn't work.

**Root cause:** The Overseerr API client needs to handle the base URL correctly. The API endpoints are at `/api/v1/` relative to the Overseerr root.

**Fix:**
1. Normalize the URL in `overseerr.go` (ensure trailing slash, construct API paths correctly)
2. Add a tooltip with example: "Use the full URL including any subpath (e.g., `https://example.com/requests/`)"
3. Add similar guidance for each integration type

### 3.3 Per-Integration Contextual Help

Show a brief help text below the URL field that changes based on the selected integration type:
- Sonarr/Radarr/Lidarr/Readarr: "Your *arr instance URL (e.g., `http://localhost:8989`)"
- Plex: "Your Plex server URL (e.g., `http://192.168.1.100:32400`). [How to find your token →]"
- Tautulli: "Your Tautulli URL (e.g., `http://localhost:8181`)"
- Overseerr: "Full URL including subpath (e.g., `https://example.com/requests/`)"

**Files:**
- `backend/internal/integrations/overseerr.go` — fix URL path handling
- `frontend/app/pages/settings.vue` — add contextual help per integration type

---

## Phase 4: Settings Page Refactor

**Effort:** M
**Goal:** Create a cleaner, more intuitive settings layout with progressive disclosure.

### 4.1 New Tab Structure

| Tab | Contents |
|-----|----------|
| **General** | Theme, display preferences (timezone, clock format) |
| **Integrations** | Integration cards (unchanged) |
| **Security** | Username change, password change, API key |
| **Advanced** | Poll interval, audit log retention, default disk group thresholds |

### 4.2 What Moves Where

- **Poll Interval** → Advanced tab (was General)
- **Data Management / Retention** → Advanced tab (was General)
- **Theme / Display** → stays in General
- **Authentication** tab → renamed to **Security**, add username change

### 4.3 New Advanced Settings

1. **Default disk group thresholds** — When a new disk group is discovered, use these defaults:
   - Default threshold: 85% (configurable, range 50-99%)
   - Default target: 75% (configurable, range 50-98%)
   - Stored in `PreferenceSet` model

2. ~~Custom cron schedule~~ **Removed** — The simple interval dropdown (30s–1h) covers every realistic use case. Power users who need exotic schedules can use the API + their own cron/systemd timer.

3. ~~Deletion rate throttle~~ **Removed** — The 3s delay is hardcoded and sensible. Too fast causes I/O storms, too slow is just annoyance. Not user-configurable.

### 4.4 Username Change

Add `PUT /api/v1/auth/username` endpoint:
1. Requires current password for verification
2. Validates new username (non-empty, no spaces, min 3 chars)
3. Updates the `AuthConfig` record
4. Invalidates current session (redirect to login)

**Files:**
- `frontend/app/pages/settings.vue` — restructure tabs, add Advanced tab, add username form
- `backend/internal/db/models.go` — add default threshold/target to `PreferenceSet`
- `backend/routes/api.go` — add username change endpoint
- `backend/routes/rules.go` — include new preference fields
- `backend/internal/poller/poller.go` — read default thresholds when creating new disk groups

---

## Phase 5: Engine Controls in Navbar

**Effort:** S-M
**Goal:** Consolidate the execution mode toggle and Run Now button into a single, accessible control in the Navbar.

**Design:** A single "Engine" button in the navbar that opens a popover/dropdown:

```
┌─────────────────────────────────────────┐
│  Engine Control                         │
│                                         │
│  Mode:  [Dry-Run] [Approval] [Auto]    │
│                                         │
│  Last run: 2 minutes ago                │
│  Evaluated: 142 · Flagged: 3            │
│                                         │
│  [▶ Run Now]                            │
└─────────────────────────────────────────┘
```

- The navbar shows an engine icon button (like ⚡ or ▶)
- Badge overlay shows current mode state: green dot (dry-run), amber (approval), red (auto)
- Clicking opens a popover with the mode toggle + stats + Run Now
- Auto mode requires a confirmation dialog before activating
- While running: spinner replaces the badge

**Why a popover instead of inline navbar buttons:**
- Keeps navbar clean (one button instead of 4)
- Accessible from every page, not just dashboard
- Engine Activity section on dashboard becomes a read-only stats display

**Implementation:**
1. Create `useEngineControl` composable — shared state for mode + run status
2. Create `EngineControlPopover` component
3. Add to Navbar
4. Simplify dashboard Engine Activity to read-only

**Files:**
- `frontend/app/composables/useEngineControl.ts` — new composable
- `frontend/app/components/EngineControlPopover.vue` — new component
- `frontend/app/components/Navbar.vue` — add engine button
- `frontend/app/pages/index.vue` — simplify Engine Activity section

---

## Phase 6: Custom Loading Screen

**Effort:** XS
**Issue:** Nuxt's default SPA loading indicator doesn't match Capacitarr branding.

**Fix:** Add a branded inline splash screen via the `nuxt.config.ts` head script (which already runs pre-Vue). This replaces the Nuxt loading bar with a centered brand icon + "Loading Capacitarr" that fades when the app mounts.

The inline script in `nuxt.config.ts` head already runs before Vue loads (it sets theme/dark mode). Extend it to also create a splash element that `app.vue` removes on mount.

**Files:**
- `frontend/nuxt.config.ts` — add inline splash CSS/HTML to head
- `frontend/app/app.vue` — remove splash element on mount

---

## Phase 7: Additional Integration Support

**Effort:** M per integration

### 7.1 Readarr

Same API pattern as Sonarr/Radarr. Quick addition.

### 7.2 Jellyfin + Emby

Media server alternatives to Plex — provides watch data for the scoring engine. Capacitarr is designed to be **media server agnostic**, so all major servers should be supported. Jellyfin forked from Emby, so their APIs are structurally similar — implementing one gives us most of the other at very low marginal cost.

| Service | Recommendation |
|---------|----------------|
| **Readarr** | ✅ Add (books, same *arr pattern) |
| **Jellyfin** | ✅ Add (media server, Plex/Emby alternative) |
| **Emby** | ✅ Add (media server, shares API structure with Jellyfin) |
| **Whisparr** | ⏸ Defer (niche audience) |
| **Bazarr** | ❌ Skip (subtitles, no media to delete) |
| **Prowlarr** | ❌ Skip (indexer, no capacity relevance) |

**Files per integration:**
- `backend/internal/integrations/<service>.go` — client implementation
- `backend/internal/poller/poller.go` — wire into poll loop
- `frontend/app/pages/settings.vue` — add to type dropdown + icon

---

---

## Phase 8: Version Display & About Section

**Effort:** S
**Goal:** Show the app version discreetly in the UI and add project info to the Help page.

### 8.1 Version Strategy (Decoupling-Ready)

Two version numbers exist, tracked independently:
- **Frontend version:** from `frontend/package.json`, injected at build time via `nuxt.config.ts` runtimeConfig
- **API version:** from a new `GET /api/v1/version` endpoint that returns `{ version, commit, buildDate }`

In **bundled mode** (current): both versions match because they ship in one Docker image.
In **decoupled mode** (future): they can differ. The frontend fetches the API version on mount to display both.

### 8.2 Version in Navbar

Always display both versions next to the brand:

```
[🟣] Capacitarr  UI v1.2.3 · API v1.2.3     Dashboard  Scoring Engine  Audit Log  Settings
```

- `text-xs text-muted-foreground/50` — visible but not distracting
- Always shows both versions — respects that they are independent components even when bundled
- Frontend version from build-time config; API version fetched from `GET /api/v1/version` on mount
- If the API is unreachable, show: `UI v1.2.3 · API ···`

### 8.3 About Section on Help Page

Add an "About Capacitarr" collapsible section at the bottom of the Help page:

**Project Info:**
- App version(s): Frontend v1.2.3 · API v1.2.3
- Build date + commit hash
- Project description: "Intelligent Media Capacity Management"
- Links: GitLab repo, documentation, changelog
- License (MIT)

**Tech Stack:**
- **Frontend:** Vue 3, Nuxt 3, Tailwind CSS v4, shadcn-vue, ApexCharts, Lucide Icons
- **Backend:** Go, Echo HTTP framework, GORM + SQLite, Goose migrations
- **Auth:** JWT, bcrypt, API key, proxy header support
- **Infrastructure:** Docker, Alpine Linux

**Credits / Acknowledgments:**
- shadcn-vue — component library
- Tailwind CSS — utility-first CSS framework
- Nuxt — Vue meta-framework
- Geist — typography (Vercel)
- Lucide — icon system
- The *arr community

### 8.4 Backend Version Endpoint

Add `GET /api/v1/version` (public, no auth required):
```json
{
  "version": "1.2.3",
  "commit": "abc1234",
  "buildDate": "2026-03-01T15:00:00Z",
  "goVersion": "1.25"
}
```

Version and commit injected at Go build time via `-ldflags`:
```bash
go build -ldflags="-X main.version=1.2.3 -X main.commit=$(git rev-parse --short HEAD) -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
```

### 8.5 Update Check (Future Enhancement)

**Not in this phase** — document as a future item:
- Periodically check GitLab releases API for newer versions
- Show a subtle notification badge on the Help nav link or in the About section
- "v1.3.0 available — see changelog" with link
- Actual updating is the user's Docker workflow (Watchtower, `docker compose pull`, etc.)
- Self-updating containers: explicitly NOT supported (too risky)

**Files:**
- `frontend/nuxt.config.ts` — inject frontend version from package.json
- `frontend/app/components/Navbar.vue` — display version next to brand
- `frontend/app/pages/help.vue` — add About section with tech stack
- `frontend/app/composables/useVersion.ts` — fetch API version, compare with frontend
- `backend/main.go` — add version/commit/buildDate vars with ldflags
- `backend/routes/api.go` — add `GET /api/v1/version` endpoint
- `Dockerfile` — pass ldflags during Go build
- `package.json` — ensure `version` field is maintained

---

## Implementation Summary

| Phase | Item | Effort | Category |
|-------|------|--------|----------|
| 1 | Toast z-index fix | XS | Bug fix |
| 2 | Lint & format enforcement | S | DX |
| 3 | Integration guidance + Overseerr fix | S-M | UX + Bug |
| 4 | Settings refactor + advanced settings | M | UX |
| 5 | Engine controls in navbar | S-M | UX |
| 6 | Custom loading screen | XS | Polish |
| 7 | Additional integrations (Readarr, Jellyfin, Emby) | M | Feature |
| 8 | About section on Help page | XS | Polish |

**Execution order:** 1 → 2 → 3 → 4 → 5 → 8 → 6 → 7

Bugs first, then developer experience, then UX restructuring, then polish, then new features.

---

## Decisions Made

| Topic | Decision | Rationale |
|-------|----------|-----------|
| Cron schedule | ❌ Removed | Simple interval dropdown covers all use cases; power users can use API + their own cron |
| Deletion rate throttle | ❌ Removed | Hardcoded 3s is sensible; not user-tuneable |
| Lint in Dockerfile | ❌ Removed | Dockerfile is for building, not catching code quality. Lint before build via Makefile/scripts |
| Run Now + Mode toggle | Consolidated | Single "Engine" popover in navbar instead of separate buttons |
| Plex OAuth | Deferred | Ship setup guide first; OAuth PIN flow is a future enhancement |
