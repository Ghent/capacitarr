# Full Site Audit — Capacitarr

**Date:** 2026-03-04  
**Branch:** `refactor/full-site-audit`  
**Scope:** Deep audit, dead code removal, refactoring, modularity, efficiency  
**Status:** 🟡 In Progress
**Predecessor:** [`20260303T0201Z-deep-code-audit.md`](20260303T0201Z-deep-code-audit.md) (many items already fixed)

---

## Executive Summary

This is a comprehensive follow-up audit of the Capacitarr codebase. The March 3 audit identified 65+ findings and fixed the critical/high items (FIX-001 through FIX-018). This audit picks up where that left off and goes deeper — targeting dead code removal, architectural refactoring, code consolidation, modularity, efficiency, and overall codebase cleanliness.

### Codebase Inventory

| Layer | Source Files | Test Files | Largest Files |
|-------|-------------|------------|---------------|
| Go Backend | 29 source | 25 tests | `rules.go` (478), `audit.go` (398), `sonarr.go` (390) |
| Vue/TS Frontend | ~45 source | 2 tests | `settings.vue` (2348), `rules.vue` (1616), `index.vue` (768) |
| CSS | 1 file | — | `main.css` (1076 lines) |
| SQL Migrations | 15 files | — | — |
| i18n Locales | 22 languages | — | — |
| Documentation | 12+ docs | — | `openapi.yaml` (65KB) |

### What's Already Fixed (from March 3 audit)

The following items from the prior audit have been confirmed fixed/resolved:

- ✅ FIX-001: `matchesRule` → `matchesRuleWithValue` (test compile fix)
- ✅ FIX-002: Engine test expectations for multipliers
- ✅ FIX-003: Lidarr float precision
- ✅ FIX-004: Readarr in poller `createClient()`
- ✅ FIX-005: Scoring docs with correct values (×1.5, ×3.0)
- ✅ FIX-011: `motion-presets.ts` deleted
- ✅ FIX-012: Dead `GetWatchHistory()` functions removed
- ✅ FIX-014: `formatPercent`, `DataResetResponse` removed; `DiskGroupSection` emit cleaned
- ✅ FIX-015: Webhook URL scheme validation added
- ✅ FIX-016: `LibraryHistory` json tags added

### Remaining Open Items from March 3 Audit

- ✅ FIX-006: API examples wrong preference field names (verified already correct)
- ✅ FIX-007: OpenAPI license mismatch (verified already correct)
- ✅ FIX-008: OpenAPI missing `deletionsEnabled` (verified already present)
- ✅ FIX-009: Rate limit docs inconsistency (verified already correct — says 10)
- ✅ FIX-010: Deployment docs contradictory Traefik example (verified already correct — no stripprefix)
- ⬜ FIX-013: Unused CSS `data-slot` selectors (CSS has 74 `data-slot` references — need to audit which are live)
- ⬜ FIX-017: GORM `primaryKey` casing inconsistency
- ⬜ FIX-018: Swallowed errors in poller

---

## Phase 1: Dead Code & Unused Artifact Removal

**Goal:** Remove all dead code, unused imports, orphaned types, stale CSS selectors, and unreferenced files. This is the lowest-risk, highest-cleanliness phase.

### Step 1.1: CSS Dead Selector Audit

- [x] Audit all 74 `[data-slot="..."]` CSS selectors in `main.css`
- [x] Cross-reference each against Vue templates to find orphaned selectors
- [x] Remove selectors with no matching template usage — removed `[data-slot="score"]` from font rule
- [x] `nav-link-active` confirmed used dynamically via ternary in Navbar.vue
- **Note:** Prior audit's 5 orphaned selectors (FIX-013) were already removed. Only 1 new orphan found.

### Step 1.2: Frontend Dead Export Sweep

- [x] Scan all `export function`, `export interface`, `export type`, `export const` in `utils/`, `lib/`, `composables/`, `types/`
- [x] Cross-reference each against imports in `.vue` and `.ts` files
- [x] All format.ts, composable, and lib exports confirmed in use
- [x] Consolidated duplicate `DiskGroup` interface (DiskGroupSection.vue → import from types/api.ts)
- [x] Consolidated duplicate `ScoreFactor` interface (ScoreBreakdown.vue, ScoreDetailModal.vue → import from types/api.ts)
- [x] Added `matchedValue?` to shared `ScoreFactor` type to cover ScoreDetailModal usage
- **Note:** `RuleBuilder.vue` local types (`Integration`, `FieldDef`, `NameValue`, `RuleValuesResponse`) are unique to component — intentionally kept local.

### Step 1.3: Backend Dead Code Sweep

- [x] Run `go vet ./...` — passes clean, zero warnings
- [x] All Go tests pass (`go test -count=1 ./...` — 7 packages OK)
- [x] Fixed swallowed error on `db.DB.Find()` in poller orphan cleanup (FIX-018 remnant)
- [x] Added panic recovery to deletion worker goroutine (auto-restarts on panic)
- [x] Added `safePoll()` wrapper with panic recovery for poller goroutine
- [x] GORM `primaryKey` casing confirmed consistent (`primarykey` throughout)
- [ ] Check `cache.go` — confirm `Close()` is either called at shutdown or documented as intentional

### Step 1.4: i18n Dead Key Audit

- [ ] Parse `en.json` keys into a set
- [ ] Scan all `.vue` files for `$t('...')` and `t('...')` references
- [ ] Identify orphaned keys (in JSON but not referenced)
- [ ] Identify hardcoded English strings (in templates but not in JSON)
- [ ] Remove orphaned keys from all 22 locale files
- [ ] Document remaining hardcoded strings for future i18n work

### Step 1.5: SQL Migration Cleanup Check

- [ ] Verify migration files are sequentially numbered with no gaps
- [ ] Ensure each migration has clear header comments
- [ ] Check for any reversible patterns or missing down-migrations (document if intentional)

---

## Phase 2: Component Decomposition & Modularity

**Goal:** Break apart monolithic files into focused, reusable, testable modules. Improve code organization and readability.

### Step 2.1: Split `settings.vue` (2348 lines)

- [ ] Identify logical sections/tabs in the settings page
- [ ] Extract each tab into its own component (e.g., `SettingsGeneral.vue`, `SettingsIntegrations.vue`, `SettingsNotifications.vue`, `SettingsAbout.vue`)
- [ ] Create a shared composable for settings state management if needed
- [ ] Keep the parent `settings.vue` as a thin orchestrator with tab routing
- [ ] Target: parent page ≤200 lines, each sub-component ≤500 lines

### Step 2.2: Split `rules.vue` (1616 lines)

- [ ] Extract preference weight editor into `RuleWeightEditor.vue`
- [ ] Extract preview table into `RulePreviewTable.vue`
- [ ] Extract label/field maps into a utility file `utils/ruleFieldMaps.ts`
- [ ] Move `ruleConflicts()` logic to `computed` to eliminate O(n²) per-render cost
- [ ] Target: parent page ≤300 lines

### Step 2.3: Evaluate `Navbar.vue` (416 lines) & `ApprovalQueueCard.vue` (505 lines)

- [ ] Assess if `Navbar.vue` can extract mobile nav into `NavbarMobile.vue`
- [ ] Assess if `ApprovalQueueCard.vue` can extract list renderer into a sub-component
- [ ] Only split if it genuinely improves readability (don't split for the sake of it)

### Step 2.4: Backend Route Handler Modularity

- [ ] Evaluate `routes/rules.go` (478 lines) — can rule-field generation move to a dedicated file?
- [ ] Evaluate `routes/audit.go` (398 lines) — is there shared handler logic to extract?
- [ ] Check if integration type constants (`intTypeSonarr`, etc.) should move to a shared `constants.go`
- [ ] Assess whether `routes/api.go` handler registrations can be grouped more logically

### Step 2.5: Backend Integration DRY-up

- [ ] Audit `sonarr.go`, `radarr.go`, `lidarr.go`, `readarr.go` for duplicated patterns
- [ ] Identify shared HTTP request/response patterns across *arr integrations
- [ ] Evaluate whether a generic `arrClient` base struct with shared methods would reduce duplication
- [ ] Same analysis for `emby.go` vs `jellyfin.go` (they share API shape)
- [ ] Document any shared patterns without over-abstracting (don't generalize prematurely)

---

## Phase 3: Code Quality & Efficiency

**Goal:** Improve error handling, type safety, performance, and robustness.

### Step 3.1: Error Handling Audit

- [ ] Audit all `catch {}` blocks in frontend — ensure errors are at minimum logged to console in dev
- [ ] Audit all `if err != nil` paths in backend — ensure no swallowed errors (FIX-018 remnants)
- [ ] Check that `gorm.DB` errors are checked after every `Find`, `Create`, `Save`, `Delete`, `FirstOrCreate`
- [ ] Verify poller/delete worker goroutines have panic recovery
- [ ] Check that all `defer` statements in backend are ordered correctly (defer runs LIFO)

### Step 3.2: Type Safety Improvements

- [ ] Identify all `as` type assertions on API responses in frontend
- [ ] Evaluate adding runtime validation for critical API responses (at minimum, null checks)
- [ ] Consolidate duplicate local interfaces in components vs shared `types/api.ts`
- [ ] Check for `any` type usage — replace with proper types where feasible
- [ ] Audit execution mode string inconsistency (`'dry_run'` vs `'dry-run'`)

### Step 3.3: Performance Audit

- [ ] Check for N+1 query patterns in route handlers (particularly preview/audit endpoints)
- [ ] Verify database indexes cover common query patterns:
  - `audit_logs.action` + `audit_logs.created_at` composite index
  - `library_histories(resolution, timestamp, disk_group_id)` composite index
- [ ] Check `RuleValueCache` lifecycle — ensure janitor goroutine is properly cleaned up
- [ ] Audit frontend re-render triggers — ensure `computed` is used over methods for derived state
- [ ] Check for unnecessary `reactive()` wrapping or deep reactivity where `ref()` suffices

### Step 3.4: API Response Consistency

- [ ] Audit error response format across all route handlers — should be consistent
  - Verify whether we use `{"error": "..."}` vs `{"message": "..."}` consistently
- [ ] Check HTTP status codes: are 400/401/403/404/500 used correctly and consistently?
- [ ] Verify all list endpoints return `[]` (not `null`) for empty results

### Step 3.5: Security Hardening

- [ ] Verify no sensitive data (API keys, tokens, passwords) in logs (sanitize audit)
- [ ] Check rate limiting coverage — ensure all auth-related endpoints are covered
- [ ] Verify CORS config is appropriate for production (not overly permissive)
- [ ] Check for any Plex tokens in query params (should be in headers)
- [ ] Audit JWT token lifecycle — document token revocation limitations

---

## Phase 4: Test Coverage & Reliability

**Goal:** Improve test coverage for under-tested areas without being completionist.

### Step 4.1: Identify Coverage Gaps

- [ ] Run `go test -cover ./...` and document per-package coverage
- [ ] Identify the 5 Go packages with zero coverage: `config`, `db`, `jobs`, `notifications`, `testutil`
- [ ] Prioritize which packages benefit most from tests (by risk and complexity)

### Step 4.2: Add Critical Backend Tests

- [ ] Add tests for `config/config.go` — environment variable parsing, defaults
- [ ] Add tests for `notifications/dispatcher.go` — dispatch routing logic
- [ ] Add tests for `jobs/cron.go` — job scheduling setup
- [ ] Verify existing test assertions use proper comparison methods (no float `!=`)

### Step 4.3: Frontend Test Strategy

- [ ] Currently only 2 test files (`useEngineControl.test.ts`, `format.test.ts`)
- [ ] Add tests for `useApprovalQueue.ts` — the most complex composable
- [ ] Add tests for `groupPreview.ts` — pure utility with clear inputs/outputs
- [ ] Add tests for `useConnectionHealth.ts` — connection state machine
- [ ] Evaluate adding basic component mount tests for `Navbar.vue` and `ScoreBreakdown.vue`

---

## Phase 5: Documentation & Standards

**Goal:** Keep documentation accurate and complete. Close remaining doc gaps.

### Step 5.1: Fix Remaining Doc Issues

- [x] FIX-006: Update `docs/api/examples.md` preference field names — verified already correct
- [x] FIX-007: Update `docs/api/openapi.yaml` license to match README — verified already correct
- [x] FIX-008: Add `deletionsEnabled` to OpenAPI `PreferenceSet` schema — verified already present
- [x] FIX-009: Update `docs/api/README.md` rate limit from 5 to 10 — verified already correct
- [x] FIX-010: Remove contradictory `stripprefix` from `docs/deployment.md` Traefik example — verified already correct

### Step 5.2: OpenAPI Completion

- [ ] Document missing Notifications API endpoints (12 endpoints)
- [ ] Document missing Plex OAuth endpoints (2 endpoints)
- [ ] Document missing rule update and rule reorder endpoints
- [ ] Add approval-mode endpoints to OpenAPI spec

### Step 5.3: Code Comments & Documentation

- [ ] Ensure all exported Go functions have godoc comments
- [ ] Ensure all Vue composables have JSDoc header comments
- [ ] Add inline comments to complex algorithms (score calculation, rule evaluation)
- [x] Update `CONTRIBUTING.md` — change "pull request" to "merge request"
- [ ] Update `README.md` — add missing features (Notifications, Plex OAuth, Approval mode)

### Step 5.4: GORM Model Consistency

- [ ] FIX-017: Normalize `primaryKey` vs `primarykey` casing across all models
- [ ] Verify all model structs have complete json tags
- [ ] Verify all model structs have consistent GORM tag formatting

---

## Phase 6: Infrastructure & Build

**Goal:** Clean up build configuration, CI/CD, and development tooling.

### Step 6.1: Docker & Build

- [x] Audit `Dockerfile` for unnecessary layers or cache-busting — **No issues found.** Multi-stage build is well-structured with proper layer caching, `CGO_ENABLED=0`, minimal alpine runtime, OCI labels, healthcheck, and PUID/PGID support.
- [x] Verify `.dockerignore` covers all appropriate paths — **Fixed:** Added exclusions for CI/build config files (`Makefile`, `docker-compose.yml`, `.gitlab-ci.yml`, `.goreleaser.yml`, `cliff.toml`, root `package.json`/`package-lock.json`) to reduce Docker build context size.
- [x] Check `docker-compose.yml` for any hardcoded values that should be environment variables — **Acceptable.** `PUID=1001`, `PGID=1001`, `DEBUG=true` are hardcoded but appropriate for a development compose file. Production users override via their own compose overrides.
- [x] Verify `Makefile` targets are all functional and documented — **No issues.** All 8 targets documented in `help`, `.PHONY` is complete, targets are well-organized by section.

### Step 6.2: CI/CD Pipeline

- [ ] Review `.gitlab-ci.yml` for redundant jobs or optimization opportunities
- [ ] Check if test stage covers all packages
- [ ] Verify lint stage runs both Go and frontend linters
- [ ] Check for cache optimization in CI (go mod cache, pnpm store)

### Step 6.3: Dependency Audit

- [ ] Check for outdated Go dependencies (`go list -m -u all`)
- [ ] Check for outdated npm dependencies (`pnpm outdated`)
- [ ] Review `pnpm.overrides` — are the security overrides still needed?
- [ ] Check for unused dependencies in both `go.mod` and `package.json`

---

## Execution Order & Priority

The phases are ordered by ascending risk and effort:

| Phase | Risk | Effort | Priority |
|-------|------|--------|----------|
| Phase 1: Dead Code Removal | 🟢 Low | 🟡 Medium | **P1** — safest, most visible cleanup |
| Phase 2: Component Decomposition | 🟡 Medium | 🔴 High | **P2** — biggest architectural win |
| Phase 3: Code Quality | 🟡 Medium | 🟡 Medium | **P3** — correctness and efficiency |
| Phase 5: Documentation | 🟢 Low | 🟡 Medium | **P4** — parallel with any phase |
| Phase 4: Test Coverage | 🟢 Low | 🟡 Medium | **P5** — safety net for future changes |
| Phase 6: Infrastructure | 🟢 Low | 🟢 Low | **P6** — polish and optimization |

### Suggested Commit Strategy

Each phase should produce 1-3 atomic commits following Conventional Commits:

```
refactor: remove dead CSS selectors and unused frontend exports
refactor(settings): decompose settings page into tab sub-components
refactor(rules): extract rule preview and weight editor components
fix: consolidate error handling and add panic recovery
docs: fix remaining OpenAPI and deployment documentation issues
test: add coverage for config, notifications, and approval queue
chore: audit and update dependencies
```

---

## Success Criteria

- [ ] `go vet ./...` passes with zero warnings
- [ ] `go test ./...` passes with zero failures
- [ ] `pnpm lint` passes with zero errors
- [ ] `pnpm typecheck` passes with zero errors
- [ ] Docker build completes successfully
- [ ] No file over 600 lines (excluding tests, CSS, and generated files)
- [ ] All exported functions have doc comments
- [ ] Zero dead code (unused exports, orphaned CSS, unreachable branches)
- [ ] All error paths are handled (no swallowed errors)
- [ ] Consistent patterns across similar code (integration clients, route handlers)
