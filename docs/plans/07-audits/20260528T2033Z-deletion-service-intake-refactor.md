# Deletion Service Intake Layer Refactor

**Status:** ✅ Complete  
**Type:** Refactor  
**Priority:** High  
**Related:** fix/approval-queue-mode-mismatch (commit a2ffa99)

## Problem Statement

`DeletionService.QueueDeletion()` accepts a raw 14-field `DeleteJob` struct and trusts every caller to populate it correctly. Four independent callers (engine poller, `ExecuteApproval`, `ManualDelete`, sunset expiry) each re-implement the same resolution workflow: look up integration → create client → resolve disk group → determine mode → set fields. This has produced two confirmed bugs:

1. `ExecuteApproval` omitted `DiskGroupID` and `CollectionGroup`, causing approved items to be falsely cancelled by the mode-change safety check (fixed in a2ffa99)
2. `ManualDelete` has the same latent omission (unfixed)

The root cause is structural: Go's zero-value initialization means missing fields compile fine, and the "defense-in-depth" check in `processJob` catches the inconsistency at runtime as a false cancellation — masking the real bug.

## Goals

1. Eliminate the class of bugs where callers forget to populate `DeleteJob` fields
2. Establish a single, consistent pipeline for all deletion paths
3. Preserve the engine's hot-path performance (no redundant DB lookups)
4. Reduce coupling between callers and `DeletionService` internals
5. Improve code organization (1042-line file → logical units)

## Non-Goals

- Changing the worker/processing internals (grace period, rate limiting, batch tracking)
- Changing audit logging behavior
- Changing the event bus contract
- Adding new features (this is a pure refactor)

## Design

### File Structure

Split the single `deletion.go` into logical units (same type, multiple files):

```
internal/services/
├── deletion.go              # Struct, constructor, SetDependencies, Start/Stop, interfaces
├── deletion_intake.go       # QueueFromEngine, QueueFromApproval, QueueFromSunset, QueueManual
├── deletion_queue.go        # enqueue(), dequeue, grace period, cancellation, list/find
├── deletion_worker.go       # worker(), drainAll(), cancelRemaining(), processJob()
├── deletion_execution.go    # executeDryRun(), executeDeletion(), postDeletion(), helpers
```

### New Interfaces

```go
// ClientResolver creates a MediaDeleter and retrieves config from an integration ID.
// Decouples DeletionService from integration factory internals.
type ClientResolver interface {
    GetDeleter(integrationID uint) (integrations.MediaDeleter, error)
    GetIntegrationConfig(integrationID uint) (*db.IntegrationConfig, error)
}

// DiskGroupResolver expands on DiskGroupModeReader to support integration-based lookups.
type DiskGroupResolver interface {
    GetByID(id uint) (*db.DiskGroup, error)
    GetDiskGroupIDForIntegration(integrationID uint) *uint
}
```

### Public API (Intake Layer)

```go
// Engine: pre-resolved client + full context (hot path, no DB lookups)
func (s *DeletionService) QueueFromEngine(req EngineDeleteRequest) error

// Approval: item from DB, service resolves client + disk group
func (s *DeletionService) QueueFromApproval(item *db.ApprovalQueueItem) error

// Sunset: item from DB, service resolves client + disk group
func (s *DeletionService) QueueFromSunset(item *db.SunsetQueueItem) error

// Manual: user-initiated, service resolves disk group from integration link
func (s *DeletionService) QueueManual(items []ManualDeleteRequest) (ManualDeleteResult, error)
```

### Request Types

```go
// EngineDeleteRequest — engine already has everything resolved.
// DiskGroupID is uint (not *uint) because the engine always resolves disk groups
// during evaluation — it iterates per-disk-group. The intake method converts to
// *uint internally (diskGroupID: &req.DiskGroupID).
type EngineDeleteRequest struct {
    Client             integrations.MediaDeleter
    Item               integrations.MediaItem
    Score              float64
    Factors            []engine.ScoreFactor
    RunStatsID         uint
    DiskGroupID        uint
    CollectionGroup    string
    AddImportExclusion bool
    UpsertAudit        bool
}

// ManualDeleteRequest — user submits identity data only.
type ManualDeleteRequest struct {
    MediaName     string
    MediaType     string
    IntegrationID uint
    ExternalID    string
    SizeBytes     int64
    Score         float64
    ScoreDetails  string
    PosterURL     string
}

// ManualDeleteResult — aggregate outcome of QueueManual.
type ManualDeleteResult struct {
    QueuedCount   int    // Items queued for deletion
    ApprovalCount int    // Items routed to approval queue
    TotalCount    int    // Total items processed
    ResolvedMode  string // The mode that was resolved (for UI feedback)
}
```

### Internal Job Type

`DeleteJob` becomes unexported `deleteJob`. Only intake methods construct it.

```go
type deleteJob struct {
    client             integrations.MediaDeleter
    item               integrations.MediaItem
    score              float64
    factors            []engine.ScoreFactor
    trigger            string
    runStatsID         uint
    diskGroupID        *uint
    collectionGroup    string
    enqueuedMode       string
    forceDryRun        bool
    upsertAudit        bool
    approvalEntryID    uint
    sunsetQueueItemID  uint
    addImportExclusion bool
}
```

### Intake Resolution Logic

**`QueueFromEngine`** — no lookups:
- Convert `EngineDeleteRequest` to `deleteJob`
- Set `trigger = "engine"`, `enqueuedMode = "auto"`
- Call `enqueue()`

**`QueueFromApproval`** — full resolution:
- `clientResolver.GetDeleter(item.IntegrationID)` → client
- `clientResolver.GetIntegrationConfig(item.IntegrationID)` → `AddImportExclusion`
- Parse `item.ScoreDetails` → factors
- Resolve mode from `item.DiskGroupID` via `diskGroups.GetByID()` or `settings.DefaultDiskGroupMode`
- Determine `forceDryRun` from resolved mode + `deletionsEnabled`
- Set `trigger = "approval"`, `enqueuedMode = resolved mode`
- Propagate `diskGroupID`, `collectionGroup`, `approvalEntryID`
- Call `enqueue()`

**`QueueFromSunset`** — full resolution:
- `clientResolver.GetDeleter(item.IntegrationID)` → client
- `clientResolver.GetIntegrationConfig(item.IntegrationID)` → `AddImportExclusion`
- Parse `item.ScoreDetails` → factors
- Set `trigger = "engine"`, `enqueuedMode = "sunset"`
- Set `diskGroupID = &item.DiskGroupID`, `sunsetQueueItemID = item.ID`
- Call `enqueue()`

**`QueueManual`** — resolves per-item:
- For each item:
  - `clientResolver.GetDeleter(item.IntegrationID)` → client
  - `clientResolver.GetIntegrationConfig(item.IntegrationID)` → `AddImportExclusion`
  - `diskGroupResolver.GetDiskGroupIDForIntegration(item.IntegrationID)` → `diskGroupID`
  - Resolve mode from `diskGroupID` or fall back to `settings.DefaultDiskGroupMode`
  - If mode == "approval": delegate to approval queue upsert (same as current behavior)
  - Determine `forceDryRun`
  - Set `trigger = "user"`, `enqueuedMode = resolved mode`
  - Parse score details → factors
  - Call `enqueue()`
- Return `ManualDeleteResult` (queued count, total, mode)

### Dependency Changes

| Interface | Current | After |
|-----------|---------|-------|
| `DiskGroupModeReader` | `GetByID(uint)` | Replaced by `DiskGroupResolver` (adds `GetDiskGroupIDForIntegration`) |
| `ClientResolver` | N/A (new) | `GetDeleter(uint)`, `GetIntegrationConfig(uint)` |

`SetDependencies` is replaced by a deps struct pattern. At 8 dependencies, positional params are unreadable and fragile. The `Wired() bool` method already validates all deps are non-nil at startup, mitigating the zero-value struct risk.

```go
type DeletionDeps struct {
    Settings       SettingsReader
    DiskGroups     DiskGroupResolver
    Clients        ClientResolver
    Approval       ApprovalReturner
    Stats          StatsWriter
    Audit          AuditWriter
    Notifications  NotificationPublisher
    EventBus       *events.EventBus
}

func (s *DeletionService) SetDependencies(deps DeletionDeps) { ... }
```

### Callers After Refactor

| Caller | Before | After |
|--------|--------|-------|
| Poller `dispatchByMode` | Builds `DeleteJob{14 fields}` | `QueueFromEngine(EngineDeleteRequest{...})` |
| `ExecuteApproval` | 40+ lines of resolution + `QueueDeletion(DeleteJob{...})` | `deps.Deletion.QueueFromApproval(approved)` |
| `ManualDelete` | Client creation + mode routing + `QueueDeletion(DeleteJob{...})` | `deps.Deletion.QueueManual(items)` — `QueueManual` owns approval-mode routing internally via `ApprovalReturner` |
| Sunset `expireItem` | 30+ lines of resolution + `QueueDeletion(DeleteJob{...})` | `deps.Deletion.QueueFromSunset(&item)` |

---

## Execution Plan

### Phase 1: Preparation

- [x] **1.1** Create branch `refactor/deletion-service-intake` from `main`

### Phase 2: File Split (No Behavioral Changes)

- [x] **2.1** Create `deletion_intake.go` — initially empty, package declaration only
- [x] **2.2** Create `deletion_queue.go` — move queue management code: `enqueue` (extracted from `QueueDeletion`), `dequeueJob()`, `resetGracePeriod()`, `poke()`, `getGraceDelay()`, `GracePeriodState()`, `QueueLen()`/`queueLen()`, `ListQueuedItems()`, `FindQueuedItem()`, cancellation methods (`CancelDeletion`, `IsCancelled`, `clearCancelled`, `ClearQueue`, `ClearQueueForDiskGroup`, `cancelKey`)
- [x] **2.3** Create `deletion_worker.go` — move processing code: `worker()`, `drainAll()`, `cancelRemaining()`, `processJob()`, `SignalBatchSize()`, `checkBatchComplete()`, `publishProgress()`
- [x] **2.4** Create `deletion_execution.go` — move execution code: `executeDryRun()`, `executeDeletion()`, `postDeletion()`, `resolveCurrentMode()`, `determineDryRunReason()`, `SnoozeDeletionItem()`
- [x] **2.5** `deletion.go` retains: struct definition, `DeleteJob` (still exported for now), `DeleteJobSummary`, interfaces, constructor, `SetDependencies()`, `Wired()`, `Start()`, `Stop()`, `CurrentlyDeleting()`, `Processed()`, `Failed()`
- [x] **2.6** Verify `make ci` passes — pure code move, zero behavioral change

### Phase 3: Define New Interfaces and Types

- [x] **3.1** Define `ClientResolver` interface in `deletion.go`
- [x] **3.2** Expand `DiskGroupModeReader` → `DiskGroupResolver` interface (add `GetDiskGroupIDForIntegration(uint) *uint`). Kept `DiskGroupModeReader` as the embedded base; `DiskGroupResolver` extends it.
- [x] **3.3** Define `EngineDeleteRequest` struct in `deletion_intake.go`
- [x] **3.4** Define `ManualDeleteRequest` struct in `deletion_intake.go` (replaces current `ManualDeleteItem` in approval.go)
- [x] **3.5** Define `ManualDeleteResult` struct in `deletion_intake.go`. Kept existing field names (`Queued`, `Total`, `Mode`) to preserve API backward compatibility — the planned `QueuedCount`/`ApprovalCount`/`TotalCount`/`ResolvedMode` fields would have been a breaking API change (non-goal of this refactor).
- [x] **3.6** Define `DeletionDeps` struct in `deletion.go`. Replace `SetDependencies(7 params)` with `SetDependencies(DeletionDeps)`. Update `Wired()` to validate all struct fields non-nil. **Note:** Actual struct mirrors existing deps (`Settings`, `Engine`, `Metrics`, `Approval`, `Snoozer`, `DiskGroups`, `Clients`, `SunsetCleaner`) rather than the idealized design in the Design section.
- [x] **3.7** Implement `ClientResolver` — created `client_resolver.go` with `clientResolverAdapter` wrapping `IntegrationService` + `integrations.CreateClient`.
- [x] **3.8** Implement `DiskGroupResolver` — added `GetDiskGroupIDForIntegration` to `DiskGroupService` (query `disk_group_integrations` junction table)
- [x] **3.9** Update `services.Registry` wiring to pass `DeletionDeps{}` via `SetDependencies`. Also deduplicated `IntegrationService` construction.
- [x] **3.10** Verify `make ci` passes — interfaces exist but aren't used yet

### Phase 4: Implement Intake Layer

- [x] **4.1** Extract internal `enqueue(job deleteJob) error` — intake methods call `enqueue()` (renamed from `QueueDeletion` in Phase 6.3). During Phase 4 the method was still named `QueueDeletion`; the rename happened in Phase 6 as part of the unexport pass.
- [x] **4.2** Implement `QueueFromEngine(req EngineDeleteRequest) error` — convert request to `deleteJob`, call `enqueue()`. Added `ForceDryRun` field to support the engine's dry-run disk group mode.
- [x] **4.3** Implement `QueueFromApproval(item *db.ApprovalQueueItem) error` — full resolution: client, config, factors, disk group mode, forceDryRun, then `enqueue()`
- [x] **4.4** Implement `QueueFromSunset(item *db.SunsetQueueItem) error` — full resolution: client, config, factors, then `enqueue()`
- [x] **4.5** Implement `QueueManual(items []ManualDeleteRequest, approvalUpserter ApprovalReturnerUpserter) (ManualDeleteResult, error)` — per-item resolution with approval-mode routing. **Note:** Added `ApprovalReturnerUpserter` parameter to avoid circular import of `ApprovalService`.
- [x] **4.6** Write unit tests for each intake method (mock `ClientResolver`, `DiskGroupResolver`, `SettingsReader`) — 8 tests in `deletion_intake_test.go`
- [x] **4.7** Verify `make ci` passes — intake exists alongside old `QueueDeletion` (both work)

### Phase 5: Migrate Callers

- [x] **5.1** Migrate poller `dispatchByMode` (evaluate.go) — both auto and dry-run modes now use `QueueFromEngine(EngineDeleteRequest{...})`
- [x] **5.2** Migrate `ExecuteApproval` (approval.go) — replaced 70+ lines with `deps.Deletion.QueueFromApproval(approved)`
- [x] **5.3** Migrate `ManualDelete` (approval.go) — now a thin adapter that passes `[]ManualDeleteRequest` to `deps.Deletion.QueueManual(items, s)` where `s` (ApprovalService) satisfies `ApprovalReturnerUpserter`
- [x] **5.4** Migrate sunset `processExpiredItem` (sunset.go) — replaced 50+ lines with `deps.Deletion.QueueFromSunset(&item)`
- [x] **5.5** Update `ExecuteApprovalDeps` — reduced to single field `{Deletion *DeletionService}`. Removed `Integration`, `Engine`, `Settings`, `DiskGroups`.
- [x] **5.6** Update `ManualDeleteDeps` — reduced to single field `{Deletion *DeletionService}`. Removed `Integration`, `Engine`.
- [x] **5.7** Update route handlers that call `ManualDelete` / `ExecuteApproval` to pass simplified deps. Also removed mode-resolution logic from `handleManualDelete` route handler.
- [x] **5.8** Verify `make ci` passes — all callers migrated, old path unused

### Phase 6: Unexport and Clean Up

- [x] **6.1** Verified no external references to `DeleteJob` remain (grep confirmed only `internal/services/` hits)
- [x] **6.2** Rename `DeleteJob` → `deleteJob` (unexport). Updated all internal references.
- [x] **6.3** Renamed `QueueDeletion` → `enqueue` (unexported)
- [x] **6.4** Remove `ManualDeleteItem` from approval.go. Updated `ManualDelete` signature and route handler to use `ManualDeleteRequest` directly.
- [x] **6.5** Dead code removed: resolution logic in `ExecuteApproval`, `ManualDelete`, `processExpiredItem`. Unused imports (`encoding/json`, `engine`, `integrations`) cleaned from approval.go and sunset.go.
- [x] **6.6** `DeleteJobSummary` unchanged (no field changes needed)
- [x] **6.7** Updated all deletion_test.go tests — renamed `QueueDeletion` → `enqueue` and `DeleteJob` → `deleteJob`. Worker/processing tests use `enqueue()` directly (same package). Route tests migrated to `QueueFromEngine`.
- [x] **6.8** N/A — `GetDiskGroupIDForIntegration` was added by this refactor (Phase 3.8), not by the interrupted fix. No removal needed.
- [x] **6.9** Final `make ci` — full pipeline green

### Phase 7: Validation

- [x] **7.1** Run the full test suite with `-race` flag — no data races detected (services: 162s, poller: 6s, routes: 146s)
- [x] **7.2** Manual smoke test via Docker: seeded approval queue item via `POST /delete` with disk group in approval mode → approved via `POST /approval-queue/1/approve` → item appeared in deletion queue with correct fields. `QueueFromApproval` resolved client and mode correctly.
- [x] **7.3** Manual smoke test: set disk group to `auto` (non-default; global default is `dry-run`) → `POST /delete` → response shows `mode=auto` and item landed in deletion queue (not approval queue). `QueueManual` correctly resolved per-disk-group mode.
- [x] **7.4** Sunset queue was empty in this environment (no items past their deletion date). Cannot fabricate an expired item without DB manipulation or waiting for a real countdown. Unit test `TestProcessExpired_WithDeletion` verifies this path works with `QueueFromSunset`. **Accepted:** path is covered by automated tests; manual verification requires a populated sunset queue.
- [x] **7.5** Verified: grace period activates on enqueue (30s remaining, active=true), resets on queue mutation, cancellation via `DELETE /deletion-queue` works (status=cancelled), `POST /deletion-queue/clear` cancels all items (cancelled=2).

---

## Risk Assessment

| Risk | Likelihood | Mitigation |
|------|-----------|------------|
| Circular dependency between DeletionService and ApprovalService | Medium | `QueueManual` handles approval-mode routing internally via `ApprovalReturner` interface (already exists). No direct `ApprovalService` import needed. |
| Breaking existing tests (50+ deletion tests) | High | Phase 6.6 is mechanical — same logic, different entry point. Run after each phase. |
| Engine performance regression | Low | `QueueFromEngine` does zero DB lookups — it's a struct conversion + enqueue. |
| `ClientResolver` adapter complexity | Low | Thin wrapper: call `IntegrationService.GetByID()` then `integrations.CreateClient()`. 10-15 lines. |
| `QueueManual` approval-mode routing complexity | Low | `QueueManual` owns all routing. It delegates approval-mode items to `ApprovalReturner.UpsertPending()` via existing interface — no direct `ApprovalService` import. |

## Success Criteria

1. No caller outside `DeletionService` can construct a `deleteJob` directly
2. All 4 deletion paths produce consistent, complete jobs
3. The mode-change safety check in `processJob` only fires for genuine runtime mode changes, never for missing fields
4. `make ci` passes with zero new warnings
5. No performance regression on the engine evaluation hot path
