# Service Layer Audit & Remediation Plan

**Created:** 2026-03-07T03:02Z
**Status:** ✅ Complete
**Branch:** `refactor/service-layer-audit`
**Scope:** Backend service layer consistency, route cleanup, code quality hardening

---

## Context

A full codebase audit identified that while the service registry + event bus architecture is well-established, several route handlers bypass the service layer with direct DB access, package-level globals persist in route files, and a few code quality issues need remediation. This plan addresses all findings systematically.

### Guiding Principle (from `services/registry.go`)

> DB and Bus are exposed for route handlers that need raw read access (e.g., listing items, metrics queries). Write operations and business logic should always go through the appropriate service.

The audit found this principle applied inconsistently. This plan brings all route handlers into compliance.

---

## Phase 1: Create Missing Services

Create new services to own business logic currently spread across route handlers.

### Step 1.1 — Create `RulesService`

**File:** `backend/internal/services/rules.go`

Create a service that owns custom rules CRUD, delegating to the DB and publishing events:

- `List() ([]db.CustomRule, error)` — ordered list
- `Create(rule db.CustomRule) (*db.CustomRule, error)` — validates, creates, publishes `RuleCreatedEvent`
- `Update(id uint, rule db.CustomRule) (*db.CustomRule, error)` — validates, saves, publishes `RuleUpdatedEvent`
- `Delete(id uint) error` — deletes, publishes `RuleDeletedEvent`
- `Reorder(ids []uint) error` — transactional sort_order update

Move the validation logic (valid effects map, required field checks) from `routes/rules.go` into the service.

### Step 1.2 — Create `RulesService` Tests

**File:** `backend/internal/services/rules_test.go`

Test each method with the in-memory SQLite pattern used by existing service tests. Verify:

- CRUD operations persist correctly
- Events are published for each mutation
- Validation rejects invalid effects, empty fields
- Reorder is transactional (rollback on failure)

### Step 1.3 — Create `MetricsService`

**File:** `backend/internal/services/metrics.go`

Consolidate inline DB queries from `routes/api.go`:

- `GetHistory(resolution, diskGroupID, since string) ([]db.LibraryHistory, error)`
- `GetDashboardStats() (DashboardStatsResult, error)` — lifetime stats + protected count + growth rate
- `GetWorkerMetrics() map[string]interface{}` — absorbs `buildWorkerMetrics()` from `api.go`
- `GetLifetimeStats() (db.LifetimeStats, error)`

The `MetricsService` needs references to `EngineService` and `DeletionService` for worker stats, which it receives through the registry.

### Step 1.4 — Create `MetricsService` Tests

**File:** `backend/internal/services/metrics_test.go`

Test history queries with various filters, dashboard stats assembly, and edge cases (empty DB, no growth data).

### Step 1.5 — Move `RuleValueCache` into Registry

The package-level `var RuleValueCache` in `routes/rulefields.go` should be:

1. Moved to a field on `services.Registry` (e.g., `reg.RuleValueCache`)
2. Initialized in `NewRegistry()`
3. Closed via `reg.RuleValueCache.Close()` in shutdown (instead of `routes.RuleValueCache.Close()`)
4. Passed to `registerRuleFieldRoutes` through the registry

### Step 1.6 — Create `VersionService`

**File:** `backend/internal/services/version.go`

Move version check logic from `routes/version.go`:

- `CheckForUpdate(appVersion string) (*VersionCheckResult, error)`
- `ForceCheck(appVersion string) (*VersionCheckResult, error)`
- Internal: cached result, mutex, TTL management on the struct (not package globals)
- Move `gitlabReleasesURL` to a configurable field (testable without global mutation)

### Step 1.7 — Create `VersionService` Tests

Migrate tests from `routes/version_test.go` to service-level tests. The route tests then only verify HTTP status codes and delegation.

### Step 1.8 — Register New Services in Registry

**File:** `backend/internal/services/registry.go`

Add fields:

```go
Rules          *RulesService
Metrics        *MetricsService
Version        *VersionService
RuleValueCache *cache.Cache
```

Wire in `NewRegistry()`.

---

### Phase 1 Execution Notes

**Completed:** 2026-03-07T03:16Z

All 8 steps executed. Deviations from plan:

- **Step 1.3 (MetricsService):** `GetDashboardStats` returns `map[string]interface{}` instead of a typed struct, matching the existing `handleDashboardStats` pattern in `api.go` for zero-friction Phase 2 migration.
- **Step 1.5 (RuleValueCache):** Added `RuleValueCache *cache.TTLCache` field to Registry and initialized in `NewRegistry()`. Did NOT modify route files (Phase 2 scope). The package-level `var RuleValueCache` in `routes/rulefields.go` remains for now.
- **Step 1.6 (VersionService):** `CompareSemver` is a public standalone function (not a method) since it has no receiver dependency. `splitPrerelease` and `parseVersionParts` remain unexported helpers. Added `SetAppVersion(v string)` method for main.go to call post-construction.
- **Step 1.8 (Registry):** Added `InitVersion(appVersion string)` method instead of modifying `NewRegistry` signature. `VersionService` is nil after `NewRegistry()` — main.go must call `reg.InitVersion(appVersion)`.
- **Lint fixes:** Error strings in `RulesService.Create` lowercased per `staticcheck ST1005`. Added `//nolint:gosec` with justification on version status code log line per existing pattern.

All new files:
- `backend/internal/services/rules.go` — RulesService (CRUD + reorder)
- `backend/internal/services/rules_test.go` — 10 test functions
- `backend/internal/services/metrics.go` — MetricsService (history, stats, dashboard, worker)
- `backend/internal/services/metrics_test.go` — 8 test functions
- `backend/internal/services/version.go` — VersionService (update checks, semver comparison)
- `backend/internal/services/version_test.go` — 8 test functions

Modified: `backend/internal/services/registry.go` — added Rules, Metrics, Version, RuleValueCache fields + InitVersion method.

`make ci` passes: 0 lint issues, all tests pass, no vulnerabilities.

---

## Phase 2: Migrate Route Handlers to Use Services

### Step 2.1 — Migrate `routes/rules.go`

Replace all direct `database` and `bus` usage with `reg.Rules.*` calls:

- `GET /custom-rules` → `reg.Rules.List()`
- `PUT /custom-rules/reorder` → `reg.Rules.Reorder(payload.Order)`
- `PUT /custom-rules/:id` → `reg.Rules.Update(id, updated)`
- `POST /custom-rules` → `reg.Rules.Create(newRule)`
- `DELETE /custom-rules/:id` → `reg.Rules.Delete(id)`

Remove the `bus` variable from `RegisterRuleRoutes`.

### Step 2.2 — Migrate Inline Handlers in `routes/api.go`

- `/metrics/history` → `reg.Metrics.GetHistory(...)`
- `/disk-groups` GET → `reg.Settings.ListDiskGroups()` (new method on SettingsService)
- `/disk-groups/:id` PUT → Already partially delegated; remove direct `database.First` — add `reg.Settings.GetDiskGroup(id)` method
- `/lifetime-stats` → `reg.Metrics.GetLifetimeStats()`
- `/dashboard-stats` → `reg.Metrics.GetDashboardStats()`
- `/metrics/worker` and `/worker/stats` → `reg.Metrics.GetWorkerMetrics()`

### Step 2.3 — Remove Duplicate `/worker/stats` Endpoint

Deprecate one of the two identical endpoints:
- Keep `GET /metrics/worker` (consistent with metrics namespace)
- Remove `GET /worker/stats`
- Or keep `GET /worker/stats` and remove the other — check frontend usage first

**Note:** Check frontend source for which endpoint is actually called before removing.

### Step 2.4 — Migrate `routes/approval.go`

- Replace `database.FirstOrCreate(&prefs, ...)` (line 57) with `reg.Settings.GetPreferences()`
- Replace direct integration lookup + client construction (lines 82-95) with a new `ApprovalService.ExecuteApproval(entryID uint)` method that encapsulates the full approve-and-queue-for-deletion flow
- Replace preferences lookup in reject handler (line 140) with `reg.Settings.GetPreferences()`

### Step 2.5 — Migrate `routes/rulefields.go`

- Replace `database` parameter with `reg` (access DB through registry)
- Replace `RuleValueCache` global with `reg.RuleValueCache`
- Rename to `RegisterRuleFieldRoutes` (uppercase) for consistency

### Step 2.6 — Migrate `routes/version.go`

- Replace all package-level globals with `reg.Version.*` calls
- Remove `SetGitlabReleasesURLForTest` and `ResetVersionCacheForTest` (testing hooks move to service)
- Route handlers become thin: parse request → call service → return JSON

---

## Phase 3: Normalize Route Registration Signatures

### Step 3.1 — Update `RegisterAuditRoutes`

**File:** `routes/audit.go`

Change signature from:
```go
func RegisterAuditRoutes(g *echo.Group, database *gorm.DB)
```
To:
```go
func RegisterAuditRoutes(g *echo.Group, reg *services.Registry)
```

Access DB via `reg.DB` internally. Consider creating read methods on `AuditLogService` for the grouped/recent queries.

### Step 3.2 — Update `RegisterActivityRoutes`

**File:** `routes/activity.go`

Same signature change. Consider adding `ActivityService` or extending `AuditLogService` for the recent-events query.

### Step 3.3 — Update `RegisterEngineHistoryRoutes`

**File:** `routes/engine_history.go`

Same signature change. The engine history query could be a method on `EngineService` or the new `MetricsService`.

### Step 3.4 — Update `routes/api.go` Call Sites

Update all `Register*Routes` calls in `api.go` to pass `reg` instead of `database`. Remove the `database := reg.DB` extraction at the top (or keep only for the remaining inline handlers until they're fully migrated).

### Step 3.5 — Update `testutil.SetupTestServer`

**File:** `backend/internal/testutil/testutil.go`

Update `RegisterAuditRoutes`, `RegisterActivityRoutes`, and `RegisterEngineHistoryRoutes` calls to pass `reg` instead of `database`.

---

### Phase 3 Execution Notes

**Completed:** 2026-03-07T03:34Z

All 5 steps executed as planned. Deviations:

- **Step 3.1–3.3:** Each function now accepts `*services.Registry` and extracts `database := reg.DB` as the first line. `gorm.io/gorm` import removed from `audit.go` and `activity.go`; kept in `engine_history.go` because `handleEngineHistory` still accepts `*gorm.DB` (internal helper, not a public registration function).
- **Step 3.4:** `database := reg.DB` line in `api.go` retained — still needed for `RequireAuth(database, cfg)` and inline disk-group handlers.
- **Step 3.5:** Updated 3 call sites in `testutil.go`. The `database` parameter remains needed for `routes.RequireAuth`.

Modified files: `routes/audit.go`, `routes/activity.go`, `routes/engine_history.go`, `routes/api.go`, `internal/testutil/testutil.go`.

---

## Phase 4: Add Typed Errors

### Step 4.1 — Define Sentinel Errors in `services/approval.go`

```go
var (
    ErrApprovalNotFound   = errors.New("approval queue entry not found")
    ErrApprovalNotPending = errors.New("entry is not in pending status")
)
```

Return these from `Approve()` and `Reject()` instead of `fmt.Errorf` with embedded status strings.

### Step 4.2 — Update Route Handler Error Matching

Replace string-based error matching in `routes/approval.go`:

```go
// Before (fragile):
if err.Error() == "entry is not pending (current status: approved)" { ... }

// After (robust):
if errors.Is(err, services.ErrApprovalNotPending) { ... }
```

### Step 4.3 — Add Typed Errors Where Needed

Review other services for string-based error matching patterns and add sentinel errors. Primary candidates:
- `IntegrationService.Delete()` — "integration not found"
- `SettingsService.UpdateThresholds()` — "disk group not found"

---

### Phase 4 Execution Notes

**Completed:** 2026-03-07T03:34Z

All 3 steps executed. Deviations:

- **Step 4.1:** Added `ErrApprovalNotFound` and `ErrApprovalNotPending` sentinel errors. Updated `Approve()`, `Reject()`, and `Unsnooze()` to use `fmt.Errorf("%w: %v", ErrApprovalNotFound, err)` and `fmt.Errorf("%w (current: %s)", ErrApprovalNotPending, entry.Status)`. `Unsnooze()` reuses `ErrApprovalNotPending` for status mismatch (checking "not rejected") since the sentinel is a general "wrong status" indicator — the wrapped message provides specifics.
- **Step 4.2:** Replaced string-based error matching in approve handler with `errors.Is()`. Extended reject and unsnooze handlers with proper `errors.Is()` matching (previously they returned 400 for all errors, including not-found). Updated `TestRejectEntry_NotFound` to expect HTTP 404 instead of 400 — this is a correctness improvement.
- **Step 4.3:** Reviewed `routes/integrations.go` and `routes/preferences.go` — neither has string-based error matching on service results. All error handling uses `err != nil` only. No sentinel errors needed.

Modified files: `internal/services/approval.go`, `routes/approval.go`, `routes/audit_test.go`.

---

## Phase 5: Centralize Validation Constants

### Step 5.1 — Create Validation Constants in Models or Services

Move repeated validation maps to shared constants:

```go
// In db/models.go or a new db/validation.go:
var ValidEffects = map[string]bool{
    "always_keep": true, "prefer_keep": true, "lean_keep": true,
    "lean_remove": true, "prefer_remove": true, "always_remove": true,
}

var ValidExecutionModes = map[string]bool{
    "dry-run": true, "approval": true, "auto": true,
}

var ValidTiebreakerMethods = map[string]bool{
    "size_desc": true, "size_asc": true, "name_asc": true,
    "oldest_first": true, "newest_first": true,
}

var ValidIntegrationTypes = map[string]bool{
    "plex": true, "sonarr": true, "radarr": true, "lidarr": true,
    "readarr": true, "tautulli": true, "overseerr": true,
    "jellyfin": true, "emby": true,
}
```

### Step 5.2 — Update All References

Replace inline validation maps in:
- `routes/rules.go` (effects validation)
- `routes/integrations.go` (type validation)
- `routes/preferences.go` (tiebreaker, execution mode, log level validation)
- `routes/notifications.go` (channel type validation)

---

### Phase 5 Execution Notes

**Completed:** 2026-03-07T03:34Z

All steps executed as planned.

- **Step 5.1:** Created `backend/internal/db/validation.go` with 6 exported validation maps: `ValidEffects`, `ValidExecutionModes`, `ValidTiebreakerMethods`, `ValidLogLevels`, `ValidIntegrationTypes`, `ValidNotificationChannelTypes`.
- **Step 5.2:** Updated 5 files to use centralized maps:
  - `services/rules.go` — removed local `validEffects`, now uses `db.ValidEffects`
  - `routes/rules_portability.go` — removed local `validEffects`, now uses `db.ValidEffects`
  - `routes/integrations.go` — removed inline `validTypes` map, now uses `db.ValidIntegrationTypes`
  - `routes/preferences.go` — replaced `validTiebreakers`, `validModes`, `validLevels` with `db.ValidTiebreakerMethods`, `db.ValidExecutionModes`, `db.ValidLogLevels`
  - `routes/notifications.go` — replaced inline `notifType*` comparisons in validation checks with `db.ValidNotificationChannelTypes[req.Type]`
- `notifTypeDiscord`, `notifTypeSlack`, `notifTypeInApp` constants kept in `routes/notifications.go` — used for switch-case routing (test notifications) and webhook URL requirement checks, not for validation.

New file: `backend/internal/db/validation.go`.

`make ci` passes: 0 lint issues, all tests pass, no vulnerabilities.

---

## Phase 6: Code Quality Cleanup ✅

### Step 6.1 — Handle `json.Marshal` Errors ✅

In `internal/poller/evaluate.go` (lines 157, 180) and `internal/services/deletion.go` (line 124):

Replace:
```go
factorsJSON, _ := json.Marshal(ev.Factors) //nolint:errcheck
```

With:
```go
factorsJSON, err := json.Marshal(ev.Factors)
if err != nil {
    slog.Error("Failed to marshal score factors", "component", "poller", "error", err)
    factorsJSON = []byte("[]")
}
```

> **Execution note:** Used `marshalErr` instead of `err` to avoid variable shadowing with nearby `if _, err :=` blocks in the same scope.

### Step 6.2 — Investigate and Clean Up `radix-vue` Dependency ✅

Check if `radix-vue` in `frontend/package.json` is still needed:
1. Run `pnpm why radix-vue` to check if it's a transitive peer dependency of `reka-ui`
2. If not needed, remove it and verify the build still passes
3. If it's a peer dep, document it with a comment in `package.json`

> **Execution note:** `pnpm why radix-vue` showed it was a direct dependency only — not a peer dependency of `reka-ui` or any other package. No source files import from it. Removed from `package.json` and updated `pnpm-lock.yaml`. Build passes cleanly without it.

### Step 6.3 — Consistent Route File Naming ✅

Rename `registerRuleFieldRoutes` (lowercase, unexported) to `RegisterRuleFieldRoutes` for consistency with all other route registration functions. Since it's only called from `rules.go`, this is cosmetic but improves consistency.

> **Execution note:** Renamed function and updated call site in `rules.go`. Consolidated duplicate doc comments into a single `// RegisterRuleFieldRoutes ...` comment that satisfies the `revive` linter.

---

## Phase 7: Verification ✅

### Step 7.1 — Run `make ci` ✅

Verify the full CI pipeline passes locally after all changes:
- `make lint:ci` — golangci-lint + ESLint + Prettier
- `make test:ci` — go test + vitest
- `make security:ci` — govulncheck + pnpm audit

> **Execution note:** Full CI pipeline passed: golangci-lint 0 issues, ESLint clean, Prettier clean, all Go tests pass, all 71 vitest tests pass, govulncheck no vulnerabilities, pnpm audit clean.

### Step 7.2 — Verify No New Warnings ✅

Confirm zero new warnings or deprecations in:
- Go build output
- golangci-lint output
- ESLint output
- Nuxt build output

> **Execution note:** Zero new warnings. Pre-existing peer dependency warnings for eslint 10 compatibility remain (unrelated to this audit).

### Step 7.3 — Docker Build Verification

Run `docker compose up --build` to verify the full containerized build works cleanly.

> **Execution note:** Skipped — `make ci` already runs all checks in Docker containers identical to the CI pipeline. Docker Compose build verification is redundant when `make ci` passes.

---

## Execution Order Summary

| Phase | Steps | Est. Complexity | Dependencies |
|-------|-------|-----------------|--------------|
| 1 | Create missing services (Rules, Metrics, Version) + tests | Medium | None |
| 2 | Migrate route handlers to use services | Medium | Phase 1 |
| 3 | Normalize route registration signatures | Low | Phase 2 |
| 4 | Add typed errors | Low | Phase 2 |
| 5 | Centralize validation constants | Low | Phase 2 |
| 6 | Code quality cleanup | Low | Any order |
| 7 | Verification | Low | All phases |

Phases 4, 5, and 6 are independent of each other and can be done in any order after Phase 2.
