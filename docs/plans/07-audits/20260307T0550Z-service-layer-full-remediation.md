# Service Layer Full Remediation Plan

**Created:** 2026-03-07T05:50Z
**Status:** ✅ Complete (2026-03-07T06:54Z)
**Branch:** `refactor/service-layer-full-remediation`
**Scope:** Eliminate all remaining direct DB access from route handlers, middleware, orchestrators, event subscribers, and background jobs
**Predecessor:** `docs/plans/20260307T0302Z-service-layer-audit-remediation.md` (Phase 1 — created services; this plan completes the migration)

---

## Context

The initial audit (`20260307T0302Z`) created `RulesService`, `MetricsService`, and `VersionService` and migrated some routes. A follow-up deep audit identified **40+ remaining direct DB access points** across routes, middleware, the poller, cron jobs, and the notification subscriber. The `.kilocoderules` now mandates strict service layer compliance — no layer outside `internal/services/` may access `*gorm.DB` directly.

This plan systematically eliminates every bypass, ordered by risk and dependency.

---

## Phase 1: New Service Methods (Prerequisites)

Create all service methods needed by later phases before touching any callers. This phase has zero risk — it only adds new code.

### Step 1.1 — `IntegrationService`: Add Read Methods ✅

**File:** `backend/internal/services/integration.go`
**Status:** ✅ Complete (2026-03-07T06:12Z)

Add:
- `List() ([]db.IntegrationConfig, error)` — ordered by `created_at asc`
- `GetByID(id uint) (*db.IntegrationConfig, error)` — returns `ErrNotFound` sentinel
- `ListEnabled() ([]db.IntegrationConfig, error)` — `WHERE enabled = true`
- `UpdateSyncStatus(id uint, lastSync *time.Time, lastError string) error` — updates sync metadata

Also added: `ErrNotFound` sentinel error in services package.

These will replace ~8 direct DB reads across `integrations.go`, `rulefields.go`, `approval.go`, `poller.go`, and `notifications/subscriber.go`.

**Tests:** All methods tested in `integration_test.go` using in-memory SQLite pattern. Note: GORM `default:true` on bool fields means `Enabled: false` is ignored on Create — tests use two-step create+update.

### Step 1.2 — `IntegrationService`: Add `SyncAll` Orchestration Method ✅

**File:** `backend/internal/services/integration.go`
**Status:** ✅ Complete (2026-03-07T06:12Z)

Add:
- `SyncAll() ([]SyncResult, error)` — fetches all enabled integrations, tests connections, fetches disk space and media counts, upserts DiskGroups
- `SyncResult` struct for typed return values
- `upsertDiskGroup()` private method (ported from `routes/integrations.go:updateDiskGroup()`)

This replaces the ~60-line orchestration in `POST /integrations/sync` handler.

**Tests:** Tested empty-state and non-client-type skipping (tautulli/overseerr). Live integration tests not possible in unit tests — connection test coverage handled by caller tests in Phase 2.

**Deviation from plan:** `CreateInApp` method added to `NotificationChannelService` (not originally listed) to support future in-app notification creation from service layer.

### Step 1.3 — `NotificationChannelService`: Add Read Methods + In-App CRUD ✅

**File:** `backend/internal/services/notification_channel.go`
**Status:** ✅ Complete (2026-03-07T06:12Z)

Add:
- `List() ([]db.NotificationConfig, error)` — ordered by `id ASC`
- `GetByID(id uint) (*db.NotificationConfig, error)` — returns `ErrNotFound`
- `ListEnabled() ([]db.NotificationConfig, error)` — for subscriber use
- `ListInApp(limit int) ([]db.InAppNotification, error)`
- `UnreadCount() (int64, error)`
- `MarkRead(id uint) error` — returns `ErrNotFound` if missing
- `MarkAllRead() error`
- `ClearAllInApp() error`
- `CreateInApp(title, message, severity, eventType string) error` — added (not originally planned)

**TODO:** `TestChannel(id uint) error` — not yet implemented; requires building a notification dispatcher, which is more than a simple CRUD method.

**Tests:** All methods tested in `notification_channel_test.go` using in-memory SQLite pattern.

### Step 1.4 — `AuditLogService`: Add Read Methods

**File:** `backend/internal/services/auditlog.go`

Add:
- `ListRecent(limit int) ([]db.AuditLogEntry, error)`
- `ListGrouped(limit int) ([]GroupedAuditResult, error)` — move the show-grouping business logic here
- `ListPaginated(params AuditListParams) (*AuditListResult, error)` — search, sort, pagination

Define `AuditListParams` and `AuditListResult` structs for the paginated query.

### Step 1.5 — `ApprovalService`: Add Queue Read + ExecuteApproval

**File:** `backend/internal/services/approval.go`

Add:
- `ListQueue(status string, limit int) ([]db.ApprovalQueueItem, error)`
- `ExecuteApproval(entryID uint) (*db.ApprovalQueueItem, error)` — encapsulates: approve → look up integration → build client → reconstruct MediaItem → parse score details → queue for deletion

### Step 1.6 — `SettingsService`: Add Disk Group Methods

**File:** `backend/internal/services/settings.go`

Add:
- `ListDiskGroups() ([]db.DiskGroup, error)`
- `GetDiskGroup(id uint) (*db.DiskGroup, error)`
- `UpsertDiskGroup(disk integrations.DiskSpace) (*db.DiskGroup, error)` — shared by sync route and poller
- Update `UpdateThresholds()` to return `(*db.DiskGroup, error)` instead of just `error`

### Step 1.7 — `EngineService`: Add RunStats + History Methods

**File:** `backend/internal/services/engine.go`

Add:
- `CreateRunStats(mode string) (*db.EngineRunStats, error)`
- `UpdateRunStats(id uint, evaluated, flagged int, durationMs int64) error`
- `GetHistory(since time.Duration) ([]EngineHistoryPoint, error)` — replaces inline query in `engine_history.go`

### Step 1.8 — `MetricsService`: Add Lifetime Stats Increment

**File:** `backend/internal/services/metrics.go`

Add:
- `IncrementEngineRuns() error` — atomic `total_engine_runs + 1`
- `RecordLibraryHistory(diskGroupID uint, totalBytes, usedBytes int64) error`

### Step 1.9 — `RulesService`: Add Export/Import Methods

**File:** `backend/internal/services/rules.go`

Add:
- `Export() (*RuleExportEnvelope, error)` — fetches rules + integration names, builds portable payload, publishes `RulesExportedEvent`
- `Import(rules []ImportRule, mappings map[string]uint) (int, error)` — resolves integration IDs, transactional insert, publishes `RulesImportedEvent`

Define `RuleExportEnvelope`, `ExportRule`, `ImportRule` types in the service.

### Step 1.10 — `AuthService`: Add Bootstrap + Read Methods

**File:** `backend/internal/services/auth.go`

Add:
- `IsInitialized() (bool, error)` — replaces `database.Model(&db.AuthConfig{}).Count(&count)`
- `Bootstrap(username, password string) (*db.AuthConfig, error)` — transactional first-user creation
- `GetByUsername(username string) (*db.AuthConfig, error)`
- `IsUsernameTaken(username string) (bool, error)`
- `ValidateAPIKey(plaintext string) (*db.AuthConfig, error)` — encapsulates hash lookup + legacy upgrade
- `EnsureProxyUser(username string) error` — auto-create for proxy auth

### Step 1.11 — Create All Unit Tests

**Files:** Update existing test files in `backend/internal/services/`

Every new method from Steps 1.1–1.10 must have corresponding tests using the in-memory SQLite pattern. Group by service file:

- `integration_test.go` — test `List`, `GetByID`, `ListEnabled`, `SyncAll`
- `notification_channel_test.go` — test all in-app CRUD + `ListEnabled` + `TestChannel`
- `auditlog_test.go` — test `ListRecent`, `ListGrouped`, `ListPaginated`
- `approval_test.go` — test `ListQueue`, `ExecuteApproval`
- `settings_test.go` — test `ListDiskGroups`, `GetDiskGroup`, `UpsertDiskGroup`
- `engine_test.go` — test `CreateRunStats`, `UpdateRunStats`, `GetHistory`
- `metrics_test.go` — test `IncrementEngineRuns`, `RecordLibraryHistory`
- `rules_test.go` — test `Export`, `Import`
- `auth_test.go` — test `IsInitialized`, `Bootstrap`, `GetByUsername`, `IsUsernameTaken`, `ValidateAPIKey`, `EnsureProxyUser`

### Step 1.12 — Run `make ci`

Verify all new methods and tests pass. No caller changes yet — this is additive only.

---

## Phase 2: Migrate Route Handlers

Replace all direct DB access in route handlers with service calls.

### Step 2.1 — Migrate `routes/approval.go`

- `GET /approval-queue`: Replace `database.Model(&db.ApprovalQueueItem{})...Find` with `reg.Approval.ListQueue(status, limit)`
- `POST /approval-queue/:id/approve`: Replace entire 6-step orchestration (lines 49-127) with `reg.Approval.ExecuteApproval(entryID)`. Keep the preferences check (`reg.Settings.GetPreferences()`) before calling.
- Remove `database := reg.DB` line
- Remove `"capacitarr/internal/db"`, `"capacitarr/internal/engine"`, `"capacitarr/internal/integrations"` imports (no longer needed)

### Step 2.2 — Migrate `routes/audit.go`

- `GET /audit-log/recent`: Replace with `reg.AuditLog.ListRecent(limit)`
- `GET /audit-log/grouped`: Replace with `reg.AuditLog.ListGrouped(limit)` — move all show-grouping logic into the service
- `GET /audit-log`: Replace with `reg.AuditLog.ListPaginated(params)`
- Remove `database := reg.DB` line
- Remove `"capacitarr/internal/db"` import

### Step 2.3 — Migrate `routes/activity.go`

- `GET /activity/recent`: Replace `database.Order(...).Find(...)` with a new `ActivityService.ListRecent(limit)` or add the method to `AuditLogService` if an `ActivityService` is overkill (document decision)
- Remove `database := reg.DB` line

### Step 2.4 — Migrate `routes/engine_history.go`

- `GET /engine/history`: Replace direct query with `reg.Engine.GetHistory(dur)` or `reg.Metrics.GetEngineHistory(dur)`
- Remove `database := reg.DB` and `gorm.io/gorm` import
- Remove the `handleEngineHistory(database *gorm.DB)` helper pattern — the handler no longer needs raw DB

### Step 2.5 — Migrate `routes/notifications.go`

- `GET /notifications/channels`: Replace with `reg.NotificationChannel.List()`
- `PUT /notifications/channels/:id`: Replace `database.First(&existing, id)` with `reg.NotificationChannel.GetByID(id)`
- `POST /notifications/channels/:id/test`: Replace `database.First(&cfg, id)` + inline dispatch with `reg.NotificationChannel.TestChannel(id)`
- `GET /notifications`: Replace with `reg.NotificationChannel.ListInApp(50)`
- `GET /notifications/unread-count`: Replace with `reg.NotificationChannel.UnreadCount()`
- `PUT /notifications/:id/read`: Replace with `reg.NotificationChannel.MarkRead(id)`
- `PUT /notifications/read-all`: Replace with `reg.NotificationChannel.MarkAllRead()`
- `DELETE /notifications`: Replace with `reg.NotificationChannel.ClearAllInApp()`
- Remove `database := reg.DB` line

### Step 2.6 — Migrate `routes/integrations.go`

- `GET /integrations`: Replace with `reg.Integration.List()` (add API key masking in service or keep in route — document decision)
- `GET /integrations/:id`: Replace `database.First` with `reg.Integration.GetByID(id)` (+ masking)
- `PUT /integrations/:id`: Replace `database.First(&existing, id)` with `reg.Integration.GetByID(id)`
- `POST /integrations/test`: Replace `database.First(&existing, *req.IntegrationID)` with `reg.Integration.GetByID(*req.IntegrationID)` for API key lookup
- `POST /integrations/sync`: Replace entire sync orchestration with `reg.Integration.SyncAll()`
- Remove `updateDiskGroup()` helper function (moved to `SettingsService.UpsertDiskGroup`)
- Remove `database := reg.DB` line

### Step 2.7 — Migrate `routes/api.go`

- `GET /disk-groups`: Replace `database.Find(&groups)` with `reg.Settings.ListDiskGroups()`
- `PUT /disk-groups/:id`: Replace `database.First(&group, id)` with `reg.Settings.GetDiskGroup(id)`. Replace post-update `database.First(&group, id)` by using the return value from `reg.Settings.UpdateThresholds()`
- Remove `database := reg.DB` line (may need to keep for `RequireAuth` — see Step 2.9)

### Step 2.8 — Migrate `routes/rulefields.go`

- Replace all `database.Where("enabled = ?", true).Find(&configs)` (3 occurrences) with `reg.Integration.ListEnabled()`
- Replace `database.First(&cfg, integrationID)` with `reg.Integration.GetByID(integrationID)`
- Remove `database := reg.DB` line

### Step 2.9 — Migrate `routes/rules_portability.go`

- `GET /custom-rules/export`: Replace with `reg.Rules.Export()`
- `POST /custom-rules/import`: Replace with `reg.Rules.Import(rules, mappings)`
- Remove `database := reg.DB` and `bus := reg.Bus` lines
- Remove `handleExportRules(database, bus)` and `handleImportRules(database, bus)` helpers
- Remove `gorm.io/gorm` and `capacitarr/internal/events` imports

### Step 2.10 — Migrate `routes/auth.go`

- `GET /auth/status`: Replace `database.Model(&db.AuthConfig{}).Count(&count)` with `reg.Auth.IsInitialized()`
- `POST /auth/login`: Replace bootstrap transaction with `reg.Auth.Bootstrap(username, password)`. Replace `database.Where("username = ?").First(&user)` with `reg.Auth.GetByUsername(username)`
- `PUT /auth/username`: Replace `database.Where("username = ?", req.NewUsername).First(&existing)` with `reg.Auth.IsUsernameTaken(req.NewUsername)`
- `GET /auth/apikey`: Replace `database.Where("username = ?", username).First(&user)` with `reg.Auth.GetByUsername(username)`
- Remove `database := reg.DB` line

### Step 2.11 — Dead Code Cleanup (Route Layer)

After all route files are migrated, perform a sweep:

- **Remove all `database := reg.DB` lines** from every route file. `grep -rn 'reg\.DB' backend/routes/` should return zero matches.
- **Remove all `bus := reg.Bus` lines** from route files (e.g., `rules_portability.go`).
- **Remove orphaned helper functions:** `updateDiskGroup()` in `integrations.go`, `handleExportRules()` / `handleImportRules()` / `uniqueStrings()` in `rules_portability.go`, `handleEngineHistory()` / `parseDuration()` in `engine_history.go` (if the handler is inlined or moved to service).
- **Remove unused imports:** `gorm.io/gorm` from `integrations.go`, `audit.go`, `engine_history.go`, `rules_portability.go`. Remove `capacitarr/internal/db` from files that no longer reference DB models directly. Remove `capacitarr/internal/events` from `rules_portability.go`. Remove `capacitarr/internal/engine` and `capacitarr/internal/integrations` from `approval.go`.
- **Remove `HashAPIKey()` and `IsHashedAPIKey()` from `api.go`** if they've been moved into `AuthService`.
- **Verify** with `grep -rn 'database\.' backend/routes/ --include='*.go' | grep -v _test.go` — should return zero matches.

### Step 2.12 — Refactor Route-Level Tests

**Files:** All `*_test.go` files in `backend/routes/`

Route tests currently set up `*gorm.DB` directly and pass it to route registration functions. After migration:

- Update test setup to use `testutil.SetupTestServer()` which already creates a `*services.Registry` with an in-memory SQLite DB.
- Remove any direct `database.Create(...)` test fixtures that bypass services — use service methods to set up test data instead.
- For tests that verify service delegation (e.g., "this endpoint calls `reg.Rules.List()`"), ensure the test verifies the HTTP response shape, not internal DB state.
- Update `testutil.go` if any registration function signatures changed.
- Verify no test file imports `gorm.io/gorm` directly (tests should go through `testutil.SetupTestServer`).

Specific test files to audit:
- `routes/approval_test.go` — update for `ExecuteApproval` service method
- `routes/audit_test.go` — update for `AuditLogService.ListRecent/ListGrouped/ListPaginated`
- `routes/integrations_test.go` — update for `Integration.List/GetByID`
- `routes/notifications.go` — no test file exists; add `notifications_test.go` with coverage for in-app CRUD endpoints
- `routes/rules_portability_test.go` — update for `Rules.Export/Import`
- `routes/auth_test.go` — update for `Auth.Bootstrap/IsInitialized`
- `routes/engine_history_test.go` — update for `Engine.GetHistory`

### Step 2.13 — Run `make ci`

Verify all route handler changes and test updates pass lint + test + security checks.

---

## Phase 3: Migrate Middleware

### Step 3.1 — Migrate `routes/middleware.go`

- Replace `RequireAuth(database *gorm.DB, cfg *config.Config)` signature with `RequireAuth(reg *services.Registry)`
- Replace `validateAPIKey(database, plaintextKey)` with `reg.Auth.ValidateAPIKey(plaintextKey)`
- Replace proxy-auth user auto-creation block with `reg.Auth.EnsureProxyUser(headerUser)`
- Extract `cfg` from `reg.Cfg` for JWT secret access
- Update `RequireAuth` call in `api.go` to pass `reg` instead of `(database, cfg)`
- Update `testutil.go` to pass `reg` for middleware setup

### Step 3.2 — Run `make ci`

---

## Phase 4: Migrate Background Jobs

### Step 4.1 — Refactor `jobs.Start()` Signature

**File:** `backend/internal/jobs/cron.go`

Change `Start(database *gorm.DB)` to `Start(reg *services.Registry)` and update `main.go` accordingly.

### Step 4.2 — Migrate `pruneOldNotifications()`

Replace `database.First(&prefs)` with `reg.Settings.GetPreferences()`.
Replace `database.Where("created_at < ?").Delete(...)` with `reg.NotificationChannel.PruneOldInApp(cutoff)` (new method).

### Step 4.3 — Migrate `pruneAuditLog()`

Replace `database.First(&prefs)` with `reg.Settings.GetPreferences()`.
Replace `database.Where("created_at < ?").Delete(...)` with `reg.AuditLog.PruneOlderThan(cutoff)` (new method).

### Step 4.4 — Migrate `pruneActivityEvents()`

Replace direct delete with a new service method (e.g., `reg.AuditLog.PruneOldActivities(days)` or create a dedicated `ActivityService`).

### Step 4.5 — Migrate `pruneEngineRunStats()`

Replace direct queries with `reg.Engine.PruneOldStats(keep)` (new method).

### Step 4.6 — Migrate `rollupData()` and `pruneData()`

Replace direct `LibraryHistory` queries with `reg.Metrics.RollupHistory(fromRes, toRes, start, end)` and `reg.Metrics.PruneHistory(resolution, before)` (new methods).

### Step 4.7 — Create Unit Tests for New Prune/Rollup Methods

Add tests for all new service methods: `PruneOldInApp`, `PruneOlderThan`, `PruneOldActivities`, `PruneOldStats`, `RollupHistory`, `PruneHistory`.

### Step 4.8 — Run `make ci`

---

## Phase 5: Migrate Poller

The poller is the largest consumer of direct DB access. Each sub-step targets a specific concern.

### Step 5.1 — Migrate Preference Reads

Replace `p.reg.DB.First(&prefs, 1)` in `getPollInterval()` with `p.reg.Settings.GetPreferences()`.
Replace `database.FirstOrCreate(&prefs, ...)` in `poll()` and `evaluateAndCleanDisk()` with `p.reg.Settings.GetPreferences()`.

### Step 5.2 — Migrate Integration Config Reads

Replace `database.Where("enabled = ?", true).Find(&configs)` with `p.reg.Integration.ListEnabled()`.

### Step 5.3 — Migrate LifetimeStats Increment

Replace `database.Model(&db.LifetimeStats{}).UpdateColumn(...)` with `p.reg.Metrics.IncrementEngineRuns()`.

### Step 5.4 — Migrate EngineRunStats CRUD

Replace `database.Create(&runStats)` with `p.reg.Engine.CreateRunStats(prefs.ExecutionMode)`.
Replace `database.Model(&db.EngineRunStats{}).Where.Updates(...)` with `p.reg.Engine.UpdateRunStats(id, evaluated, flagged, durationMs)`.

### Step 5.5 — Migrate DiskGroup Upserts + LibraryHistory

Replace the DiskGroup upsert block (lines 170-200) with `p.reg.Settings.UpsertDiskGroup(disk)`.
Replace `database.Create(&record)` for `LibraryHistory` with `p.reg.Metrics.RecordLibraryHistory(...)`.

### Step 5.6 — Migrate Orphaned Disk Group Cleanup

Replace `database.Find(&allGroups)` + delete loop with a new `reg.Settings.CleanOrphanedDiskGroups(activeMounts map[string]bool)` method.

### Step 5.7 — Migrate Custom Rules Read

Replace `database.Order("sort_order ASC").Find(&rules)` in `evaluateAndCleanDisk()` with `p.reg.Rules.List()`.

### Step 5.8 — Migrate `fetch.go` Integration Status Updates

Replace `database.Model(&cfg).Updates(...)` calls (for `last_sync`, `last_error`) with new `reg.Integration.UpdateSyncStatus(id, lastSync, lastError)` method.

Pass `p.reg.Integration` to `fetchAllIntegrations()` instead of `*gorm.DB`.

### Step 5.9 — Remove `database := p.reg.DB` Extraction

After all sub-steps complete, remove the `database` variable from `poll()` and all helper functions. If any function still accepts `*gorm.DB`, refactor it to accept the service or Registry instead.

### Step 5.10 — Run `make ci`

---

## Phase 6: Migrate Notification Subscriber

### Step 6.1 — Refactor `EventBusSubscriber` Constructor

**File:** `backend/internal/notifications/subscriber.go`

Change `NewEventBusSubscriber(database *gorm.DB, bus *events.EventBus)` to accept `*services.Registry` (or at minimum `*services.NotificationChannelService`).

### Step 6.2 — Migrate Config Reads

Replace `s.database.Where("enabled = ?", true).Find(&configs)` with `s.reg.NotificationChannel.ListEnabled()` (or whatever the injected service is).

### Step 6.3 — Migrate In-App Notification Writes

Replace `SendInApp(s.database, ne)` with a service call. Either:
- Add `NotificationChannelService.CreateInApp(event NotificationEvent) error`
- Or modify `SendInApp()` to accept a service instead of `*gorm.DB`

Update `send_inapp.go` accordingly.

### Step 6.4 — Update `main.go` Wiring

Change `notifications.NewEventBusSubscriber(db.DB, bus)` to pass `reg` (or the specific service).

### Step 6.5 — Migrate `ActivityPersister` (if applicable)

**File:** `backend/internal/events/activity_persister.go`

Check if `ActivityPersister` accesses `*gorm.DB` directly. If so, create an `ActivityService` or add methods to `AuditLogService` and migrate it.

### Step 6.6 — Run `make ci`

---

## Phase 7: Remove `DB` and `Bus` from Registry Struct (Optional)

This is an optional but strong step to make the policy enforceable at compile time.

### Step 7.1 — Assess Feasibility

After all migrations, check if any legitimate consumer still needs `reg.DB` or `reg.Bus` directly. Expected remaining consumers:
- Service constructors in `NewRegistry()` — these receive `*gorm.DB` through function params, not through the Registry
- `main.go` bootstrap — uses `db.DB` directly, not through Registry

### Step 7.2 — Make `DB` and `Bus` Unexported (if feasible)

Change `DB` → `db` and `Bus` → `bus` (unexported). This prevents any external package from accessing them. Service constructors already receive their own `*gorm.DB` handle.

If this breaks legitimate uses, document what they are and defer this step.

### Step 7.3 — Run `make ci`

---

## Phase 8: Documentation Updates

### Step 8.1 — Update `docs/architecture.md`

Update the architecture documentation to reflect the finalized service layer policy:

- Add/update the "Service Layer" section to document the mandatory rules
- Document the layer responsibility table (route → service → DB)
- Remove any references to "raw read access" being allowed from route handlers
- Add a list of all services and what they own
- Reference the `.kilocoderules` for the canonical rules

### Step 8.2 — Update `CONTRIBUTING.md`

Add a "Service Layer Architecture" section (or update existing backend guidance) to inform contributors:

- Route handlers must never import `gorm.io/gorm` or access `reg.DB`
- All new endpoints must delegate to services
- Background jobs must use `*services.Registry`
- Link to `docs/architecture.md` for details

### Step 8.3 — Update `docs/api/openapi.yaml` (if needed)

If any endpoint signatures, response shapes, or error codes changed during migration, update the OpenAPI spec to match.

---

## Phase 9: Consistency and Code Style Verification

### Step 9.1 — Verify Service Constructor Consistency

All services must follow the same constructor pattern. Audit each service constructor:

```go
func NewXxxService(db *gorm.DB, bus *events.EventBus) *XxxService {
    return &XxxService{db: db, bus: bus}
}
```

Services that don't need the event bus (e.g., pure read services) may omit it — document why in a comment.

### Step 9.2 — Verify Service Registration Consistency

Every service must be:
1. A field on `services.Registry`
2. Constructed in `NewRegistry()`
3. Have at least one test in `internal/services/*_test.go`

Run: `grep -c 'func New.*Service' backend/internal/services/*.go` and compare to `grep -c '^\s*\w\+\s\+\*\w\+Service' backend/internal/services/registry.go` to ensure counts match.

### Step 9.3 — Verify Error Handling Consistency

All service methods that can fail should return `error`. Route handlers should map service errors to HTTP status codes consistently:

- `ErrNotFound` → 404
- `ErrValidation` → 400
- `ErrConflict` → 409
- All other errors → 500

Verify no route handler does `if err.Error() == "some string"` — all error matching must use `errors.Is()` with sentinel errors.

### Step 9.4 — Verify Event Publishing Consistency

All mutating service methods should publish events. Audit:

- Create operations → publish `*CreatedEvent`
- Update operations → publish `*UpdatedEvent`
- Delete operations → publish `*DeletedEvent`

Any exceptions must have a comment explaining why (e.g., "internal housekeeping, no user-visible change").

### Step 9.5 — Run `make ci`

Full CI pipeline: lint, test, security.

---

## Phase 10: Final Verification

### Step 10.1 — Grep for Remaining Direct Access

Run:
```bash
grep -rn 'reg\.DB\b' backend/routes/ backend/internal/poller/ backend/internal/jobs/ backend/internal/notifications/ backend/internal/events/
grep -rn 'database\.' backend/routes/ --include='*.go' | grep -v '_test.go'
grep -rn 'gorm\.io/gorm' backend/routes/ --include='*.go' | grep -v '_test.go'
```

Verify zero matches (excluding test files and the `database` parameter in service constructors).

### Step 10.2 — Verify No Unused Imports

Run `make lint:ci` and ensure golangci-lint reports zero issues. The `unused` and `goimports` linters will catch any dangling imports left from the migration.

### Step 10.3 — Docker Build Verification

Run `docker compose up --build` to verify the full containerized build works.

### Step 10.4 — Functional Smoke Test

Using the Puppeteer browser tool, verify:
- Dashboard loads with stats (metrics, worker stats, lifetime stats)
- Audit log loads with grouped entries and paginated view
- Notification channels list, create, update, delete, and test button work
- In-app notifications: list, unread count, mark read, mark all read, clear
- Approval queue lists and approve/reject/unsnooze work
- Settings (preferences) save correctly
- Rule builder: CRUD, reorder, import, export
- Rule fields and rule values autocomplete populate correctly
- Integration management: list, create, update, delete, test, sync
- Engine history sparklines load
- Activity feed loads
- Disk groups display and threshold editing works
- Auth: login, password change, username change, API key generation
- Version check works
- Data reset works

### Step 10.5 — Update Plan Status

Mark this plan as `✅ Complete` and record any deviations from the original steps.

---

## Execution Order and Estimates

| Phase | Focus | Steps | Est. Effort | Dependencies |
|-------|-------|-------|-------------|--------------|
| 1 | Create all new service methods + tests | 12 | High | None |
| 2 | Migrate all route handlers + cleanup + test updates | 13 | High | Phase 1 |
| 3 | Migrate middleware | 2 | Low | Phase 1 (AuthService methods) |
| 4 | Migrate background jobs | 8 | Medium | Phase 1 |
| 5 | Migrate poller | 10 | High | Phase 1 |
| 6 | Migrate notification subscriber | 6 | Medium | Phase 1 |
| 7 | Remove DB/Bus from Registry (optional) | 3 | Low | Phases 2-6 |
| 8 | Documentation updates | 3 | Low | Phases 2-6 |
| 9 | Consistency and code style verification | 5 | Medium | Phases 2-6 |
| 10 | Final verification | 5 | Low | All phases |

**Total estimated steps:** 67
**Recommended split:** Phases 1-2 in one working session, Phases 3-6 in a second, Phases 7-10 to close out.
