# Brittleness Hardening Plan

**Status:** 🟡 Planned  
**Created:** 2026-03-26  
**Category:** Architecture / Code Quality  
**Branch:** `refactor/brittleness-hardening`

## Overview

Comprehensive hardening of 14 identified brittleness points in the Capacitarr codebase, plus two targeted structural improvements. The issues were identified through a full codebase audit covering the backend service layer, integration clients, event system, poller, database layer, notification dispatch, CSP handling, and test infrastructure.

No major architecture restructure is needed — the service layer, capability-interface pattern, and event bus are all architecturally sound. The brittleness is at integration edges: wiring safety, serialization assumptions, duplicated constants, and pattern fragility.

## Phases

### Phase 1: Safety Nets (Fail-Fast on Startup)

Highest risk reduction, smallest changes. Ensures bugs surface at startup rather than at 3 AM during a deletion cycle.

#### Step 1.1 — Add `Wired()` methods to all services with lazy dependencies

**Files to modify:**
- `internal/services/deletion.go` — add `Wired() bool` checking `settings`, `engine`, `metrics`, `approvalReturner`
- `internal/services/settings.go` — add `Wired() bool` checking `deletionClearer`
- `internal/services/integration.go` — add `Wired() bool` checking `diskGroups`
- `internal/services/diskgroup.go` — add `Wired() bool` checking `engineSvc`
- `internal/services/backup.go` — add `Wired() bool` checking `diskGroupSvc`
- `internal/services/metrics.go` — add `Wired() bool` checking `settingsSvc`
- `internal/services/preview.go` — add `Wired() bool` checking `integrations`, `preferences`, `rules`, `diskGroups`, `approvalQueue`, `deletionState`
- `internal/services/rules.go` — add `Wired() bool` checking `previewSource`, `integrationProvider`
- `internal/services/watch_analytics.go` — add `Wired() bool` checking `rulesSource`, `diskGroupLister`
- `internal/services/data.go` — add `Wired() bool` checking `previewSvc`
- `internal/services/migration.go` — add `Wired() bool` checking `engineSvc`

Each `Wired()` method returns `true` only when all lazily-injected dependencies are non-nil. Pattern:

```go
func (s *DeletionService) Wired() bool {
    return s.settings != nil && s.engine != nil && s.metrics != nil && s.approvalReturner != nil
}
```

#### Step 1.2 — Add `Validate()` to `services.Registry`

**Files to modify:**
- `internal/services/registry.go` — add `Validate()` method and call it at end of `NewRegistry()`

`Validate()` calls `Wired()` on every service and panics with a descriptive message listing all unwired dependencies. This mirrors the existing fail-fast pattern in `main.go` (e.g., `getSubFS()` panics on embed errors).

#### Step 1.3 — Add startup guard to `DeletionService.Start()`

**Files to modify:**
- `internal/services/deletion.go` — add `Wired()` check at top of `Start()`

Panics if `Start()` is called before `SetDependencies()`. Catches misuse in tests that construct a `DeletionService` directly without the registry.

#### Step 1.4 — Use `engine.DefaultFactors()` in test helper seeding

**Files to modify:**
- `internal/testutil/testutil.go` — replace hardcoded 7-entry `defaultWeights` list with the same bridge pattern used in `main.go`

```go
defaultFactors := engine.DefaultFactors()
factorDefaults := make([]db.FactorDefault, len(defaultFactors))
for i, f := range defaultFactors {
    factorDefaults[i] = db.FactorDefault{Key: f.Key(), DefaultWeight: f.DefaultWeight()}
}
db.SeedFactorWeights(database, factorDefaults)
```

Ensures tests always use the same factor set as production. Adding a new scoring factor automatically flows into all tests.

#### Step 1.5 — Run `make ci` and verify all tests pass

---

### Phase 2: Observability (Log Problems, Don't Silence Them)

Ensures problems that cannot be prevented are at least visible.

#### Step 2.1 — Extract CSP policy to shared function

**Files to create:**
- `routes/csp.go` — new file with `BuildCSP(nonce string) string` function

**Files to modify:**
- `main.go` — security headers middleware calls `routes.BuildCSP(nonce)` instead of inline string
- `internal/testutil/testutil.go` — security headers middleware calls `routes.BuildCSP(nonce)` instead of inline string

Single source of truth for CSP policy. Eliminates the `http:` in `img-src` drift between production and test.

#### Step 2.2 — Add panic recovery state cleanup to poller

**Files to modify:**
- `internal/poller/poller.go` — expand `safePoll()` recovery handler to:
  1. Publish `EngineErrorEvent` with the panic message
  2. Call `reg.Deletion.SignalBatchSize(0)` to unblock the notification two-gate flush
  3. Call `reg.Preview.Invalidate()` to clear potentially stale preview cache

#### Step 2.3 — Add stale accumulator warning to notification dispatch

**Files to modify:**
- `internal/services/notification_dispatch.go` — in the `EngineStartEvent` handler, log a warning if the previous accumulator exists but neither gate fired (indicates dropped events from previous cycle)

Also add a `createdAt` timestamp field to `cycleAccumulator` for age reporting in the warning message.

#### Step 2.4 — Add optional per-subscriber buffer size to EventBus

**Files to modify:**
- `internal/events/bus.go` — add `SubscribeWithBuffer(size int) chan Event` method

**Files to modify (consumers):**
- `internal/services/notification_dispatch.go` — use `SubscribeWithBuffer(512)` instead of `Subscribe()` (notification dispatch processes events quickly; larger buffer prevents dropped gate events during high-volume deletion cycles)

The default `Subscribe()` remains at 256 for all other subscribers.

#### Step 2.5 — Run `make ci` and verify all tests pass

---

### Phase 3: Serialization & Pattern Safety

Eliminates assumptions about data formats.

#### Step 3.1 — Replace SSE JSON byte manipulation with structured marshal

**Files to modify:**
- `internal/events/sse_broadcaster.go` — replace the manual `strip trailing '}'` logic in `broadcast()` with a `ssePayload` wrapper type that implements `MarshalJSON()`

The custom `MarshalJSON()` preserves the performance benefit of the byte-manipulation approach for the happy path (struct events produce `{...}`) but adds an explicit format check and a safe fallback for non-object events.

**Files to create or modify for tests:**
- `internal/events/sse_broadcaster_test.go` — add test case for `ssePayload.MarshalJSON()` with both object and edge-case inputs

#### Step 3.2 — Replace CSP nonce hardcoded patterns with regex + validation

**Files to modify:**
- `csp.go` — replace `bytes.Replace` calls in `injectNoncePlaceholders()` with a compiled regex that handles whitespace/attribute variations

Add a `NonceInjectionCount()` helper or have `injectNoncePlaceholders()` return the number of replacements made, so `buildHTMLTemplates()` in `main.go` can log a warning when zero scripts were nonced.

**Files to modify:**
- `main.go` (or wherever `buildHTMLTemplates` is defined) — add startup warning log when nonce injection found zero scripts

**Files to create or modify for tests:**
- `csp_test.go` — add test cases for regex-based nonce injection with various HTML formats

#### Step 3.3 — Replace global factory map with `sync.Map`

**Files to modify:**
- `internal/integrations/factory.go` — change `factoryRegistry` from `map[string]IntegrationFactory` to `sync.Map`
- Update `RegisterFactory()`, `CreateClient()`, `RegisteredTypes()`, `HasFactory()` to use `sync.Map` methods

This makes the factory registry safe for concurrent reads during test parallelism.

#### Step 3.4 — Run `make ci` and verify all tests pass

---

### Phase 4: Structural Improvements

Targeted restructures that reduce bug surface area for entire classes of issues.

#### Step 4.1 — Extract `arrBaseClient` embedded struct

**Files to create:**
- `internal/integrations/arr_base.go` — new file containing:
  - `arrBaseClient` struct with `URL`, `APIKey`, `APIPrefix` fields
  - `doRequest()` method
  - `TestConnection()` method
  - `GetDiskSpace()` method (delegates to `arrFetchDiskSpace`)
  - `GetRootFolders()` method (delegates to `arrFetchRootFolders`)
  - `GetQualityProfiles()` method (delegates to `arrFetchQualityProfiles`)
  - `GetTags()` method (delegates to `arrFetchTags`)
  - `GetLanguages()` method (delegates to `arrFetchLanguages`)

**Files to modify:**
- `internal/integrations/sonarr.go` — embed `arrBaseClient`, remove duplicated methods, keep only `GetMediaItems()` and `DeleteMediaItem()`
- `internal/integrations/radarr.go` — same refactor
- `internal/integrations/lidarr.go` — same refactor
- `internal/integrations/readarr.go` — same refactor

Constructors change from:

```go
func NewSonarrClient(url, apiKey string) *SonarrClient {
    return &SonarrClient{URL: strings.TrimRight(url, "/"), APIKey: apiKey}
}
```

To:

```go
func NewSonarrClient(url, apiKey string) *SonarrClient {
    return &SonarrClient{
        arrBaseClient: arrBaseClient{
            URL:       strings.TrimRight(url, "/"),
            APIKey:    apiKey,
            APIPrefix: "/api/v3",
        },
    }
}
```

All existing tests must pass without changes since the behavior is identical.

#### Step 4.2 — Add `CollectionNameFetcher` capability interface

**Files to modify:**
- `internal/integrations/types.go` — add `CollectionNameFetcher` interface:
  ```go
  type CollectionNameFetcher interface {
      GetCollectionNames() ([]string, error)
  }
  ```
- `internal/services/integration.go` — refactor `FetchCollectionValues()` to use factory + type assertion instead of switch statement:
  ```go
  client := integrations.CreateClient(cfg.Type, cfg.URL, cfg.APIKey)
  if fetcher, ok := client.(integrations.CollectionNameFetcher); ok {
      names, fetchErr = fetcher.GetCollectionNames()
  }
  ```

The Plex, Jellyfin, and Emby clients already implement `GetCollectionNames()` — they satisfy the interface without changes.

#### Step 4.3 — Run `make ci` and verify all tests pass

---

### Phase 5: Notification Restructure (Explicit Cycle Digest)

Replace the fragile two-gate accumulator with explicit orchestration.

#### Step 5.1 — Add `FlushCycleDigest()` method to `NotificationDispatchService`

**Files to modify:**
- `internal/services/notification_dispatch.go` — add `FlushCycleDigest(digest notifications.CycleDigest)` method that:
  1. Checks for update info if version checker is available
  2. Dispatches the digest to all enabled channels with `OnCycleDigest` enabled
  3. Resets the accumulator

This method is called directly by the poller, not via the event bus.

#### Step 5.2 — Build cycle digest in the poller

**Files to modify:**
- `internal/poller/poller.go` — at the end of `poll()`, after `SignalBatchSize()`, build a `notifications.CycleDigest` struct from the poller's own counters and call `reg.NotificationDispatch.FlushCycleDigest(digest)`

The poller already has `lastRunEvaluated`, `lastRunCandidates`, `lastRunFreedBytes`, `lastRunProtected`, the execution mode, and the poll duration. No new data plumbing needed.

#### Step 5.3 — Remove accumulator logic from event handler

**Files to modify:**
- `internal/services/notification_dispatch.go` — remove the `cycleAccumulator` struct and all accumulation logic from `handle()`:
  - Remove `EngineStartEvent` → create accumulator
  - Remove `EngineCompleteEvent` → gate 1
  - Remove `DeletionSuccessEvent` → accumulate counts
  - Remove `DeletionDryRunEvent` → accumulate bytes
  - Remove `DeletionFailedEvent` → accumulate failures
  - Remove `DeletionBatchCompleteEvent` → gate 2
  - Remove `tryFlush()` method

Keep all **immediate alert** handlers unchanged (EngineErrorEvent, EngineModeChangedEvent, ServerStartedEvent, ThresholdBreachedEvent, UpdateAvailableEvent, approval events, integration status events).

#### Step 5.4 — Update notification dispatch tests

**Files to modify:**
- `internal/services/notification_dispatch_test.go` — update tests to call `FlushCycleDigest()` directly instead of simulating the two-gate event sequence

#### Step 5.5 — Run `make ci` and verify all tests pass

---

### Phase 6: Database Layer Cleanup

#### Step 6.1 — Remove GORM retry callbacks

**Files to modify:**
- `internal/db/retry.go` — keep the file but remove `RegisterRetryCallbacks()` and `retryOnBusy()`. Keep `isSQLiteBusy()` and `cryptoRandInt64()` as they may be useful for future retry patterns.
- `internal/db/db.go` — remove the `RegisterRetryCallbacks(database)` call from `Init()`
- `internal/db/retry_test.go` — update tests to remove callback-specific test cases

**Files to modify:**
- `internal/db/db.go` — increase `busy_timeout` from 5000 to 10000 in `buildDSN()`:
  ```go
  params.Add("_pragma", "busy_timeout(10000)")
  ```

**Rationale:** With `SetMaxOpenConns(1)`, all operations are serialized through a single connection. `SQLITE_BUSY` can only occur from external processes. The increased busy_timeout (10s) covers backup/migration edge cases. The GORM callback retry is fragile (raw SQL reconstruction) and provides no additional safety over the timeout.

#### Step 6.2 — Add `WithRetry()` utility function

**Files to modify:**
- `internal/db/retry.go` — add a `WithRetry(fn func() error, maxAttempts int) error` function that wraps any function call with exponential backoff retry on `SQLITE_BUSY`

This provides application-level retry at the *service call* level (where the full context is available) rather than inside GORM's callback pipeline (where only raw SQL is available). Service methods that need retry can opt in:

```go
err := db.WithRetry(func() error {
    return s.db.Create(&row).Error
}, 3)
```

#### Step 6.3 — Parameterize `hasColumn()` for multiple tables

**Files to modify:**
- `internal/db/migrate.go` — rename `hasColumn()` to `hasColumnInTable()` with a `tableName` parameter and an `allowedMigrationTables` whitelist for safety

#### Step 6.4 — Run `make ci` and verify all tests pass

---

### Phase 7: Frontend SSE Cleanup

#### Step 7.1 — Add auto-cleanup scope parameter to SSE `on()` function

**Files to modify:**
- `frontend/app/composables/useEventStream.ts` — add optional `scope` parameter to `on()` that auto-registers an `onUnmounted` cleanup callback:

```typescript
function on(
    eventType: string,
    handler: (data: unknown) => void,
    scope?: { onUnmounted: (fn: () => void) => void }
) {
    // ... existing registration logic ...
    if (scope) {
        scope.onUnmounted(() => off(eventType, handler));
    }
}
```

**Files to modify (consumers):**
- All components that use `useEventStream().on()` — update to pass `{ onUnmounted }` as the third argument for automatic cleanup

This prevents handler leaks from unmounted components without requiring manual `off()` calls in `onUnmounted` hooks.

#### Step 7.2 — Run frontend tests (`pnpm test` inside Docker) and verify they pass

---

## Files Changed Summary

| Phase | New Files | Modified Files |
|-------|-----------|----------------|
| 1 | 0 | ~13 (services + testutil) |
| 2 | 1 (routes/csp.go) | ~4 (main.go, testutil, poller, notification_dispatch, bus) |
| 3 | 0 | ~4 (sse_broadcaster, csp.go, main.go, factory.go) |
| 4 | 1 (arr_base.go) | ~6 (sonarr, radarr, lidarr, readarr, types.go, integration.go) |
| 5 | 0 | ~3 (notification_dispatch.go, poller.go, tests) |
| 6 | 0 | ~3 (retry.go, db.go, migrate.go) |
| 7 | 0 | ~1+ (useEventStream.ts + consumer components) |

## Risk Assessment

All phases are backward-compatible. No database migration needed. No API contract changes. No frontend API changes. Tests must pass at the end of each phase before proceeding.

Phase 5 (notification restructure) is the highest-risk change since it modifies the notification delivery mechanism. It should be tested manually by verifying Discord/Apprise notifications fire correctly after an engine cycle.

## Commit Strategy

No commits during a phase — all changes within a phase are made as working tree modifications. Only commit once `make ci` passes at the end of each phase. One commit per phase, following conventional commits:

1. `refactor(services): add startup validation for lazy dependencies`
2. `refactor(routes): extract CSP policy, improve panic recovery and event bus`
3. `fix(events): use structured JSON marshal for SSE, regex-based CSP nonce, sync.Map factory`
4. `refactor(integrations): extract arrBaseClient, add CollectionNameFetcher capability`
5. `refactor(notifications): replace event accumulator with explicit cycle digest`
6. `refactor(db): remove GORM retry callbacks, add WithRetry utility`
7. `refactor(frontend): add auto-cleanup scope to SSE event handlers`
