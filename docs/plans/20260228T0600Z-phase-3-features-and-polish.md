# Capacitarr Phase 3: Features & Polish

**Status:** ✅ Complete

This plan covers the next development iteration after the v2 migration (see `20260228T0130Z-v2-migration-plan.md`). It addresses backend correctness bugs discovered during live testing, scoring engine UX improvements, dashboard enhancements, new integrations, and visual polish.

---

## Phase 1: Backend Logic Fixes

Critical correctness and reliability fixes identified from production log analysis.

### 1.1 Show/Season Deduplication in Poller

**Problem:** Sonarr returns both show-level and season-level `MediaItem` entries. The `evaluateAndCleanDisk()` function iterates over all items and actions both independently. This causes:
- Double-counting of freed bytes (show + each season counted separately)
- Duplicate audit log entries ("Sunny" and "Sunny - Season 1" back-to-back with identical scores/sizes)
- Potential double-deletion attempts in auto mode

**Fix:** In `evaluateAndCleanDisk()`, filter the evaluated list before the action loop. When a show-level item is actioned, skip all child seasons of that show. Track actioned show titles in a `map[string]bool` and skip season items whose `ShowTitle` matches.

**Files:**
- `backend/internal/poller/poller.go` — add dedup logic in `evaluateAndCleanDisk()`

### 1.2 Per-Factor Score Breakdown in Engine

**Problem:** The `calculateScore()` function returns only `"Composite relative score"` as the reason string. The audit log shows `"Score: 0.89 (Composite relative score)"` which gives zero insight into *why* that score was assigned.

**Fix:** Build a per-factor breakdown string showing each factor's contribution:
```
WatchHistory: 0.28, LastWatched: 0.22, FileSize: 0.12, Rating: 0.14, TimeInLibrary: 0.11, Availability: 0.08
```

This breakdown is carried through to audit logs and the frontend scoring preview.

**Files:**
- `backend/internal/engine/score.go` — rewrite reason assembly in `calculateScore()`
- `backend/internal/engine/score_test.go` — update tests for new reason format

### 1.3 Cron Rollup `disk_group_id` Fix

**Problem:** The `rollupData()` function in cron.go averages capacity across **all** disk groups in a single query (no `GROUP BY disk_group_id`). The resulting rolled-up `LibraryHistory` record has `DiskGroupID: nil`, losing the per-group association. Multi-disk setups get blended data.

**Fix:** Query distinct `disk_group_id` values from the source resolution, then rollup each group separately. Set `DiskGroupID` on the resulting record.

**Files:**
- `backend/internal/jobs/cron.go` — rewrite `rollupData()` to iterate per disk group

### 1.4 HTTP Client Timeouts

**Problem:** Both `SonarrClient.doRequest()` and `RadarrClient.doRequest()` use `http.DefaultClient` which has **no timeout**. A hung Sonarr/Radarr instance will block the poller goroutine indefinitely.

**Fix:** Create a shared `*http.Client` with a 30-second timeout in a new `httpclient` helper package and use it in all integration clients.

**Files:**
- `backend/internal/integrations/httpclient.go` — new shared HTTP client with timeout
- `backend/internal/integrations/sonarr.go` — use shared client
- `backend/internal/integrations/radarr.go` — use shared client
- `backend/internal/integrations/plex.go` — use shared client

### 1.5 Shared `doRequest` Helper

**Problem:** `SonarrClient.doRequest()` and `RadarrClient.doRequest()` are nearly identical 20-line functions (create request → set header → do request → check status → read body). `PlexClient.doRequest()` is similar but sets `X-Plex-Token` instead of `X-Api-Key`.

**Fix:** Extract a generic `DoAPIRequest(client *http.Client, url, headerKey, headerValue string) ([]byte, error)` function into the shared httpclient package. Each integration's `doRequest` becomes a one-liner delegating to the shared helper.

**Files:**
- `backend/internal/integrations/httpclient.go` — add `DoAPIRequest()` function
- `backend/internal/integrations/sonarr.go` — delegate to shared helper
- `backend/internal/integrations/radarr.go` — delegate to shared helper
- `backend/internal/integrations/plex.go` — delegate to shared helper

---

## Phase 2: Scoring Engine UX Overhaul

### 2.1 Rename "Protection Rules" to "Custom Rules"

The current name "Protection Rules" is misleading since rules can also *target* media for deletion. Rename throughout:
- Backend model/table remains `protection_rules` (no migration needed)
- All frontend labels, page titles, and nav items: "Custom Rules"
- API route comments updated

**Files:**
- `frontend/app/pages/rules.vue` — rename all UI labels
- `frontend/app/components/Navbar.vue` — rename nav link
- `backend/routes/rules.go` — update comments

### 2.2 Surface Threshold/Target on Scoring Page

Display the current disk group threshold/target percentages alongside the scoring weight sliders so users understand the full context of when and how aggressively the engine acts.

**Files:**
- `frontend/app/pages/rules.vue` — add threshold/target display card

### 2.3 Two-Column Slider Layout with Preset Chips

Replace the current single-column weight sliders with a responsive 2-column grid. Add preset chips ("Balanced", "Space Saver", "Hoarder", "Watch-Based") that populate sliders to predefined configurations.

**Files:**
- `frontend/app/pages/rules.vue` — redesign slider layout, add preset chips

### 2.4 Service-Specific Rule Fields

Add rule fields that are specific to certain integration types:
- **Sonarr:** `seasonCount`, `episodeCount`, `showStatus`
- **Radarr:** `studio`, `certification`, `inCinemas`

The `matchesRule()` function in rules.go gains new cases for these fields.

**Files:**
- `backend/internal/engine/rules.go` — add new field matchers
- `backend/internal/integrations/types.go` — add fields to `MediaItem`
- `frontend/app/pages/rules.vue` — add field options to rule builder

---

## Phase 3: Dashboard & Audit Improvements

### 3.1 Worker Stats Real-Time Graph

Add a small sparkline chart to the dashboard showing deletion worker throughput (processed/failed per minute). The backend already exposes `GetWorkerMetrics()` — add a `/api/worker/stats` endpoint that returns time-series data.

**Files:**
- `backend/routes/api.go` — add `/api/worker/stats` endpoint
- `frontend/app/pages/index.vue` — add worker stats sparkline

### 3.2 Audit Score Transparency

Leverage the new per-factor breakdown (Phase 1.2) to show a tooltip or expandable row in audit logs revealing the full factor breakdown for each scored item.

**Files:**
- `frontend/app/pages/audit.vue` — add expandable score breakdown

### 3.3 Show/Season Grouping in Preview & Audit

Group season entries under their parent show in the audit view and scoring preview. Seasons appear as indented sub-rows under the show, with individual sizes but the parent show's aggregate score.

**Files:**
- `frontend/app/pages/audit.vue` — group seasons under shows
- `backend/routes/audit.go` — optional `?group=show` query parameter

---

## Phase 4: New Integrations

### 4.1 Tautulli Integration

Pull watch history, play counts, and last-played timestamps from Tautulli to enrich scoring data beyond what Plex provides directly.

**Files:**
- `backend/internal/integrations/tautulli.go` — new integration client
- `backend/internal/integrations/types.go` — add `IntegrationTypeTautulli`
- `backend/internal/poller/poller.go` — handle tautulli in `createClient()`
- `frontend/app/pages/settings.vue` — add Tautulli config card

### 4.2 Overseerr/Jellyseerr Integration

Check request status to protect media that was recently requested. Media with active requests gets a score reduction.

**Files:**
- `backend/internal/integrations/overseerr.go` — new integration client
- `backend/internal/integrations/types.go` — add `IntegrationTypeOverseerr`
- `backend/internal/poller/poller.go` — handle overseerr in `createClient()`
- `frontend/app/pages/settings.vue` — add Overseerr config card

### 4.3 Lidarr Integration

Extend capacity management to music libraries via Lidarr API.

**Files:**
- `backend/internal/integrations/lidarr.go` — new integration client
- `backend/internal/integrations/types.go` — add `IntegrationTypeLidarr`, `MediaTypeAlbum`, `MediaTypeArtist`
- `backend/internal/poller/poller.go` — handle lidarr in `createClient()`
- `frontend/app/pages/settings.vue` — add Lidarr config card

---

## Phase 5: Visual Polish

### 5.1 Spring Animations

Add spring-based enter/exit animations using `@vueuse/motion` to:
- Page transitions (fade + slide)
- Card mount animations (stagger children)
- Modal open/close

**Files:**
- `frontend/app/app.vue` — add page transition wrapper
- `frontend/app/lib/motion-presets.ts` — expand preset library

### 5.2 Glassmorphism Refinements

Apply consistent glassmorphism to cards and modals:
- `backdrop-filter: blur(12px)` with semi-transparent backgrounds
- Subtle border gradients
- Dark mode glass variant

**Files:**
- `frontend/app/assets/css/main.css` — add glass utility classes

### 5.3 Page Transitions

Implement view transitions between pages using Nuxt's built-in `<NuxtPage>` transition support with matched spring presets.

**Files:**
- `frontend/nuxt.config.ts` — enable view transitions
- `frontend/app/app.vue` — configure transition component

### 5.4 Skeleton Loaders

Replace empty states during data fetching with skeleton loaders matching the final layout shape for:
- Dashboard capacity cards
- Audit log table
- Settings integration cards
- Rules list

**Files:**
- `frontend/app/components/SkeletonCard.vue` — new skeleton component
- `frontend/app/pages/index.vue` — use skeleton during load
- `frontend/app/pages/audit.vue` — use skeleton during load
- `frontend/app/pages/settings.vue` — use skeleton during load

---

## Implementation Priority

| Phase | Effort | Impact | Target |
|-------|--------|--------|--------|
| Phase 1: Backend Logic Fixes | S-M | Critical correctness | Immediate |
| Phase 2: Scoring UX | M | High UX value | Week 1 |
| Phase 3: Dashboard & Audit | M | Medium UX value | Week 1-2 |
| Phase 4: New Integrations | L | Feature expansion | Week 2-3 |
| Phase 5: Visual Polish | S-M | Polish | Ongoing |

---

## Completion Status (2026-02-28)

| Item | Status | Notes |
|------|--------|-------|
| **Phase 1: Backend Logic Fixes** | | |
| 1.1 Show/Season Dedup | ✅ Complete | Dedup logic in `evaluateAndCleanDisk()` |
| 1.2 Per-Factor Score Breakdown | ✅ Complete | `ScoreFactors` struct, `scoreDetails` JSON in audit logs, `ScoreBreakdown.vue` + `ScoreDetailModal.vue` |
| 1.3 Cron Rollup `disk_group_id` Fix | ✅ Complete | `rollupData()` iterates per disk group |
| 1.4 HTTP Client Timeouts | ✅ Complete | `sharedHTTPClient` with 30s timeout; all `http.DefaultClient` refs replaced |
| 1.5 Shared `doRequest` Helper | ✅ Complete | `DoAPIRequest()` in `httpclient.go`; all clients delegate to it |
| **Phase 2: Scoring Engine UX** | | |
| 2.1 Rename to "Custom Rules" | ✅ Complete | Frontend labels updated |
| 2.2 Surface Threshold/Target | ✅ Complete | Editable disk group cards on rules page |
| 2.3 Two-Column Sliders + Presets | ✅ Complete | 2-col grid, preset chips (Balanced/Space Saver/Hoarder/Watch-Based) |
| 2.4 Service-Specific Rule Fields | → Deferred to Phase 4 | See `20260301T0048Z-phase-4-production-readiness.md` §10 |
| **Phase 3: Dashboard & Audit** | | |
| 3.1 Worker Stats Real-Time Graph | ✅ Complete | `/api/v1/worker/stats` + dashboard sparkline |
| 3.2 Audit Score Transparency | ✅ Complete | `ScoreDetailModal` clickable from audit + preview |
| 3.3 Show/Season Grouping | → Deferred to Phase 4 | See `20260301T0048Z-phase-4-production-readiness.md` §11 |
| **Phase 4: New Integrations** | | |
| 4.1 Tautulli | → Deferred to Phase 4 | See `20260301T0048Z-phase-4-production-readiness.md` §9 |
| 4.2 Overseerr/Jellyseerr | → Deferred to Phase 4 | See `20260301T0048Z-phase-4-production-readiness.md` §9 |
| 4.3 Lidarr | → Deferred to Phase 4 | See `20260301T0048Z-phase-4-production-readiness.md` §9 |
| **Phase 5: Visual Polish** | | |
| 5.1 Spring Animations | ✅ Complete | `v-motion` on cards, modals, page elements |
| 5.2 Glassmorphism | ✅ Complete | Glass utility classes in `main.css` |
| 5.3 Page Transitions | ✅ Complete | Nuxt `pageTransition` with fade+slide CSS |
| 5.4 Skeleton Loaders | ✅ Complete | `SkeletonCard.vue`, `SkeletonTable.vue` on dashboard + audit |

### Additional Items Completed (beyond original plan)

- **Tiebreaker configuration** — `TiebreakerMethod` on PreferenceSet, `SortEvaluated()` with 5 methods, frontend dropdown
- **Run Now button** — Dashboard trigger endpoint + manual poller run
- **Toast notification system** — `ToastContainer.vue` with success/error/warning/info toasts
- **Score detail modal** — Shared `ScoreDetailModal.vue` with per-factor horizontal bars
- **Help page** — `/help` with FAQ, factor explanations, tiebreaker docs
- **Disk threshold persistence fix** — Switched from raw `fetch()` to `api()` composable (CORS fix)
- **Consistent caret markers** — Target ▼ above bar, Threshold ▲ below bar on all capacity displays
