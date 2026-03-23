# Deep Code Audit — Capacitarr

**Date:** 2026-03-03  
**Status:** ✅ Complete  
**Branch:** `audit/deep-code-review`  
**Scope:** Full codebase audit — Go backend, Vue/TS frontend, documentation, database, infrastructure

---

## Executive Summary

Comprehensive audit of the Capacitarr codebase covering 66 Go files, 47 frontend source files, 12 SQL migrations, and all documentation. The project is well-structured with solid patterns overall. The audit identified **2 critical test failures**, **10 high-priority issues**, and **30+ medium/low findings**.

### Audit Statistics

| Category | Files Audited | Issues Found |
|----------|--------------|-------------|
| Go Backend | 47 files (22 test) | 25 findings |
| Vue/TS Frontend | 47 files (2 test) | 22 findings |
| Documentation | 12 docs + 65KB OpenAPI | 11 findings |
| Database/Migrations | 15 files | 4 findings |
| Infrastructure | 6 files | 3 findings |

---

## Phase 1: Fixes Applied in This Audit

### 1.1 Critical — Broken Tests

#### FIX-001: Engine `matchesRule` → `matchesRuleWithValue` (COMPILE FAILURE)
- **File:** `backend/internal/engine/rules_test.go` lines 190, 203, 213, 224
- **Issue:** Tests reference deleted function `matchesRule()`. Function was renamed to `matchesRuleWithValue()` which returns `(bool, string)` instead of `bool`.
- **Fix:** Update all 4 call sites to `matched, _ := matchesRuleWithValue(item, rule)`.

#### FIX-002: Engine test expectations wrong for prefer_remove multiplier
- **File:** `backend/internal/engine/rules_test.go` lines 355, 362
- **Issue:** Tests expect `prefer_remove` multiplier ×2.0, but code uses ×3.0. Also `lean_remove` expects ×1.2 but code uses ×1.5.
- **Fix:** Update expected values: "Prefer keep + prefer remove = net protection" → 0.6 (not 0.4), "Stacked prefer remove" → 9.0 (not 4.0).

#### FIX-003: Lidarr test floating-point precision
- **File:** `backend/internal/integrations/lidarr_test.go` line 189
- **Issue:** Direct `!=` comparison of `float64` values is fragile.
- **Fix:** Use epsilon-based comparison: `math.Abs(actual - expected) < 0.001`.

### 1.2 High Priority — Code Correctness

#### FIX-004: Readarr missing from poller `createClient()`
- **File:** `backend/internal/poller/poller.go` lines 275-288
- **Issue:** The poller's `createClient()` switch doesn't include `readarr`, so Readarr media items are never fetched during polling despite the integration being fully implemented.
- **Fix:** Add `case "readarr"` to the switch.

#### FIX-005: Scoring docs — wrong rule modifier values
- **File:** `docs/scoring.md` lines 152-153
- **Issue:** `lean_remove` documented as ×1.2 (actual: ×1.5), `prefer_remove` documented as ×2.0 (actual: ×3.0).
- **Fix:** Update documentation to match code.

#### FIX-006: API examples — wrong preference field names
- **File:** `docs/api/examples.md` line 480
- **Issue:** Shows `weightAge`, `weightSize`, etc. Actual JSON tags are `watchHistoryWeight`, `lastWatchedWeight`, `fileSizeWeight`, `ratingWeight`, `timeInLibraryWeight`, `seriesStatusWeight`.
- **Fix:** Update field names in examples.

#### FIX-007: OpenAPI license mismatch
- **File:** `docs/api/openapi.yaml` line 21
- **Issue:** Says "GPL-3.0-or-later" but README and LICENSE file say "PolyForm Noncommercial 1.0.0".
- **Fix:** Update OpenAPI to match the actual license.

#### FIX-008: OpenAPI missing `deletionsEnabled` in PreferenceSet
- **File:** `docs/api/openapi.yaml` PreferenceSet schema
- **Issue:** `deletionsEnabled` field exists in the Go model but is missing from the OpenAPI spec.
- **Fix:** Add the field to the PreferenceSet schema.

#### FIX-009: Rate limit docs inconsistency
- **File:** `docs/api/README.md` line 141 vs `docs/api/openapi.yaml` line 106
- **Issue:** README says 5 attempts/15min, OpenAPI says 10/15min. Code uses 10.
- **Fix:** Update README to say 10.

#### FIX-010: Deployment docs — contradictory Traefik example
- **File:** `docs/deployment.md` lines 82-91
- **Issue:** Shows stripprefix middleware then warns "Do NOT strip the prefix."
- **Fix:** Remove the stripprefix middleware from the Traefik example YAML.

### 1.3 Medium Priority — Dead Code Removal

#### FIX-011: Remove `motion-presets.ts` (dead file)
- **File:** `frontend/app/lib/motion-presets.ts`
- **Issue:** All 5 exported presets and 2 helper functions are completely unused. Every component inlines motion configs.
- **Fix:** Delete the file.

#### FIX-012: Remove unused Go functions
- **Files:** Multiple integration files
- **Issue:** Dead functions never called:
  - `PlexClient.GetWatchHistory()` — Tautulli used instead
  - `PlexClient.GetServerIdentity()` — never called
  - `EmbyClient.GetWatchHistory()` — bulk method used instead
  - `JellyfinClient.GetWatchHistory()` — bulk method used instead
  - `EmbyWatchData` struct — only used by dead function
  - `JellyfinWatchData` struct — only used by dead function
  - `Poller.SetInterval()` — documented no-op placeholder
  - `Poller.Trigger()` — bypassed by direct channel write
- **Fix:** Remove all dead functions and their associated types.

#### FIX-013: Remove unused CSS data-slot selectors
- **File:** `frontend/app/assets/css/main.css`
- **Issue:** 5 `[data-slot="..."]` CSS selectors have no matching templates:
  - `[data-slot="section-label"]`
  - `[data-slot="section-divider"]`
  - `[data-slot="score-bar"]`
  - `[data-slot="integration-card"]`
  - `[data-slot="preset-chip"]`
- **Fix:** Remove the unused CSS rules.

#### FIX-014: Remove unused frontend exports
- **Files:** Multiple frontend files
- **Issue:**
  - `formatPercent()` in `utils/format.ts` — exported but never called
  - `DataResetResponse` in `types/api.ts` — interface defined but never used
  - `toggle()` in `composables/useColorMode.ts` — legacy function never called
  - `DiskGroupSection.vue` declares emit `'updated'` but never fires it
- **Fix:** Remove unused exports.

### 1.4 Medium Priority — Code Quality

#### FIX-015: Webhook URL scheme validation
- **File:** `backend/routes/notifications.go` line 42
- **Issue:** Notification channel webhook URLs aren't validated for `http://` or `https://` scheme, unlike integration URLs which have this check.
- **Fix:** Add scheme validation matching integrations.

#### FIX-016: Add json tags to LibraryHistory and LifetimeStats
- **File:** `backend/internal/db/models.go`
- **Issue:** `LibraryHistory` has no json tags — Go defaults to PascalCase serialization. `LifetimeStats` missing json tags on `CreatedAt`/`UpdatedAt`. Inconsistent with all other models using camelCase.
- **Fix:** Add json tags for consistent camelCase serialization.

#### FIX-017: GORM primaryKey casing inconsistency
- **File:** `backend/internal/db/models.go` line 117
- **Issue:** `EngineRunStats` uses `gorm:"primaryKey"` (camelCase K) while all other models use `gorm:"primarykey"` (lowercase).
- **Fix:** Normalize to consistent casing.

#### FIX-018: Swallowed errors in poller
- **Files:** `backend/internal/poller/poller.go` line 103, `backend/internal/poller/evaluate.go` line 62
- **Issue:** `db.DB.FirstOrCreate()` and `db.DB.Find()` errors not checked.
- **Fix:** Check and log errors.

---

## Phase 2: Future Work (Not Fixed in This Audit)

### 2.1 i18n Completion (Large Effort)
- **~50% of user-visible text remains hardcoded in English** across settings.vue, rules.vue, audit.vue, help.vue, Navbar.vue, RuleBuilder.vue, ScoreDetailModal.vue, DiskGroupSection.vue, EngineControlPopover.vue, CapacityChart.vue.
- **~10 orphaned i18n keys** in `en.json` not referenced by any template.
- **Estimate:** 2-3 days of focused work to complete i18n coverage.

### 2.2 Component Decomposition (Architectural)
- `settings.vue` at 2322 lines should be split by tab into sub-components.
- `rules.vue` at 1641 lines should extract preference weights, preview table, and label maps.
- `ruleConflicts()` in rules.vue should be memoized with `computed` (currently O(n²) per render).

### 2.3 OpenAPI Spec Completion
- **14 endpoints missing from OpenAPI** — entire Notifications API (12 endpoints), Plex OAuth (2 endpoints), rule update, rule reorder.
- **Estimate:** 4-6 hours to document all missing endpoints with schemas.

### 2.4 Test Coverage Expansion
- **5 Go packages have zero test coverage:** `config`, `db`, `jobs`, `notifications`, `testutil`.
- **No Vue component tests** exist — 0 component test files.
- **No composable tests** beyond `useEngineControl`.
- **Estimate:** 2-3 days for meaningful coverage improvement.

### 2.5 Security Improvements
- No JWT token revocation mechanism — changing password doesn't invalidate existing tokens.
- No private IP range filtering for SSRF prevention on integration URLs.
- No CSRF protection on cookie-authenticated requests.
- No rate limiting on password change, API key, or data reset endpoints.
- Delete worker and poller goroutines have no panic recovery.

### 2.6 Performance Improvements
- Tautulli enrichment is O(N) API calls per item — very slow with large libraries.
- `RuleValueCache` janitor goroutine leaked (Close() never called at shutdown).
- Duplicate integration queries in rules handler (3 queries per request for same data).
- Missing composite database index on `library_histories(resolution, timestamp, disk_group_id)`.
- Missing indexes on `audit_logs.action` and `audit_logs.created_at`.

### 2.7 Accessibility
- No keyboard alternative for rule drag-to-reorder.
- No skip-to-content link for keyboard users.
- Inconsistent `confirm()` vs custom dialog for dangerous actions.

### 2.8 Frontend Type Safety
- Duplicate local interfaces in `DiskGroupSection.vue`, `ScoreBreakdown.vue`, `ScoreDetailModal.vue`, `RuleBuilder.vue` instead of importing from `~/types/api`.
- `as` type assertions on all API responses bypass runtime validation.
- Execution mode string inconsistency (`'dry_run'` vs `'dry-run'`).

### 2.9 Infrastructure
- MkDocs nav does not include API docs section.
- CHANGELOG.md is stale (only v0.0.0 and v0.1.0).
- CONTRIBUTING.md says "pull request" instead of "merge request" (GitLab terminology).
- README missing features: Notifications, Plex OAuth.

---

## Appendix A: File-by-File Finding Map

### Go Backend

| File | Findings |
|------|----------|
| `main.go` | %v instead of %w in panic, CORS acceptable for debug |
| `config/config.go` | No issues, matches docs |
| `db/db.go` | Global DB variable safe by init sequencing |
| `db/models.go` | Missing json tags (FIX-016), GORM primaryKey casing (FIX-017) |
| `db/migrate.go` | Clean |
| `cache/cache.go` | Close() never called (future work) |
| `engine/rules.go` | Clean, proper rule evaluation |
| `engine/rules_test.go` | matchesRule broken (FIX-001), wrong multipliers (FIX-002) |
| `engine/score.go` | Clean |
| `engine/score_test.go` | Clean |
| `integrations/*.go` | Dead functions (FIX-012), Plex token in query params |
| `integrations/lidarr_test.go` | Float comparison (FIX-003) |
| `jobs/cron.go` | No tests (future work) |
| `logger/*.go` | Clean |
| `notifications/*.go` | No tests (future work) |
| `poller/poller.go` | Missing readarr (FIX-004), dead functions (FIX-012), swallowed errors (FIX-018) |
| `poller/evaluate.go` | Error not checked (FIX-018) |
| `poller/delete.go` | No panic recovery (future work) |
| `routes/rules.go` | Cache leak (future work) |
| `routes/notifications.go` | Missing webhook validation (FIX-015) |
| `routes/middleware.go` | Inconsistent error response format (future work) |

### Frontend

| File | Findings |
|------|----------|
| `lib/motion-presets.ts` | Entirely dead (FIX-011) |
| `utils/format.ts` | Unused formatPercent (FIX-014) |
| `types/api.ts` | Unused DataResetResponse (FIX-014) |
| `composables/useColorMode.ts` | Unused toggle() (FIX-014) |
| `components/DiskGroupSection.vue` | Unused emit, local type definition |
| `assets/css/main.css` | 5 unused data-slot selectors (FIX-013) |
| `pages/settings.vue` | 2322 lines, hardcoded English (future work) |
| `pages/rules.vue` | 1641 lines, O(n²) conflicts, hardcoded English (future work) |

### Documentation

| File | Findings |
|------|----------|
| `docs/scoring.md` | Wrong modifier values (FIX-005) |
| `docs/api/examples.md` | Wrong preference field names (FIX-006) |
| `docs/api/openapi.yaml` | Wrong license (FIX-007), missing field (FIX-008), 14 missing endpoints |
| `docs/api/README.md` | Wrong rate limit (FIX-009) |
| `docs/deployment.md` | Contradictory Traefik example (FIX-010) |
| `CHANGELOG.md` | Stale (future work) |
