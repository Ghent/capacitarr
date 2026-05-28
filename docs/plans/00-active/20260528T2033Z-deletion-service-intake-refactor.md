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

- [ ] **1.1** Create branch `refactor/deletion-service-intake` from `main`

### Phase 2: File Split (No Behavioral Changes)

- [ ] **2.1** Create `deletion_intake.go` — initially empty, package declaration only
- [ ] **2.2** Create `deletion_queue.go` — move queue management code: `enqueue` (extracted from `QueueDeletion`), `dequeueJob()`, `resetGracePeriod()`, `poke()`, `getGraceDelay()`, `GracePeriodState()`, `QueueLen()`/`queueLen()`, `ListQueuedItems()`, `FindQueuedItem()`, cancellation methods (`CancelDeletion`, `IsCancelled`, `clearCancelled`, `ClearQueue`, `ClearQueueForDiskGroup`, `cancelKey`)
- [ ] **2.3** Create `deletion_worker.go` — move processing code: `worker()`, `drainAll()`, `cancelRemaining()`, `processJob()`, `SignalBatchSize()`, `checkBatchComplete()`, `publishProgress()`
- [ ] **2.4** Create `deletion_execution.go` — move execution code: `executeDryRun()`, `executeDeletion()`, `postDeletion()`, `resolveCurrentMode()`, `determineDryRunReason()`, `SnoozeDeletionItem()`
- [ ] **2.5** `deletion.go` retains: struct definition, `DeleteJob` (still exported for now), `DeleteJobSummary`, interfaces, constructor, `SetDependencies()`, `Wired()`, `Start()`, `Stop()`, `CurrentlyDeleting()`, `Processed()`, `Failed()`
- [ ] **2.6** Verify `make ci` passes — pure code move, zero behavioral change

### Phase 3: Define New Interfaces and Types

- [ ] **3.1** Define `ClientResolver` interface in `deletion.go`
- [ ] **3.2** Expand `DiskGroupModeReader` → `DiskGroupResolver` interface (add `GetDiskGroupIDForIntegration(uint) *uint`). Keep old name as type alias or migrate all references.
- [ ] **3.3** Define `EngineDeleteRequest` struct in `deletion_intake.go`
- [ ] **3.4** Define `ManualDeleteRequest` struct in `deletion_intake.go` (replaces current `ManualDeleteItem` in approval.go)
- [ ] **3.5** Define `ManualDeleteResult` struct in `deletion_intake.go`
- [ ] **3.6** Define `DeletionDeps` struct in `deletion.go`. Replace `SetDependencies(7 params)` with `SetDependencies(DeletionDeps)`. Update `Wired()` to validate all struct fields non-nil.
- [ ] **3.7** Implement `ClientResolver` — create adapter struct in `services/` that wraps `IntegrationService` + `integrations.CreateClient`. Integration clients are stateless HTTP wrappers (struct with URL + API key + `*http.Client`), so no caching is needed. Register on `services.Registry`.
- [ ] **3.8** Implement `DiskGroupResolver` — add `GetDiskGroupIDForIntegration` to `DiskGroupService` (query `disk_group_integrations` junction table)
- [ ] **3.9** Update `services.Registry` wiring to pass `DeletionDeps{}` via `SetDependencies`
- [ ] **3.10** Verify `make ci` passes — interfaces exist but aren't used yet

### Phase 4: Implement Intake Layer

- [ ] **4.1** Extract internal `enqueue(job deleteJob) error` from current `QueueDeletion` body (same logic, unexported job type as param)
- [ ] **4.2** Implement `QueueFromEngine(req EngineDeleteRequest) error` — convert request to `deleteJob`, call `enqueue()`
- [ ] **4.3** Implement `QueueFromApproval(item *db.ApprovalQueueItem) error` — full resolution: client, config, factors, disk group mode, forceDryRun, then `enqueue()`
- [ ] **4.4** Implement `QueueFromSunset(item *db.SunsetQueueItem) error` — full resolution: client, config, factors, then `enqueue()`
- [ ] **4.5** Implement `QueueManual(items []ManualDeleteRequest) (ManualDeleteResult, error)` — per-item resolution with approval-mode routing
- [ ] **4.6** Write unit tests for each intake method (mock `ClientResolver`, `DiskGroupResolver`, `SettingsReader`)
- [ ] **4.7** Verify `make ci` passes — intake exists alongside old `QueueDeletion` (both work)

### Phase 5: Migrate Callers

- [ ] **5.1** Migrate poller `dispatchByMode` (evaluate.go) — replace `QueueDeletion(DeleteJob{...})` with `QueueFromEngine(EngineDeleteRequest{...})`
- [ ] **5.2** Migrate `ExecuteApproval` (approval.go) — replace 40+ lines of resolution with `deps.Deletion.QueueFromApproval(approved)`. Remove `ClientResolver`-like code from `ExecuteApprovalDeps`.
- [ ] **5.3** Migrate `ManualDelete` (approval.go) — replace client creation + queue logic with `deps.Deletion.QueueManual(items)`. Approval-mode routing moves into `QueueManual` (the pipeline owns all routing — this is the point of the refactor). `ManualDelete` becomes a thin adapter that converts HTTP request data to `[]ManualDeleteRequest`.
- [ ] **5.4** Migrate sunset `expireItem` (sunset.go) — replace resolution + `QueueDeletion(DeleteJob{...})` with `deps.Deletion.QueueFromSunset(&item)`
- [ ] **5.5** Update `ExecuteApprovalDeps` — remove `Integration`, `Settings`, `DiskGroups` fields that are no longer needed by the caller (they're now internal to `DeletionService`)
- [ ] **5.6** Update `ManualDeleteDeps` — remove `Integration` field; `Deletion` is the only dep needed
- [ ] **5.7** Update route handlers that call `ManualDelete` / `ExecuteApproval` to pass simplified deps
- [ ] **5.8** Verify `make ci` passes — all callers migrated, old path unused

### Phase 6: Unexport and Clean Up

- [ ] **6.1** Verify no external references to `DeleteJob` remain: `grep -r "DeleteJob" --include="*.go" backend/` must show only hits within `internal/services/`. If any external references exist (poller, routes, tests outside services package), migrate them first.
- [ ] **6.2** Rename `DeleteJob` → `deleteJob` (unexport). Update all internal references.
- [ ] **6.3** Remove exported `QueueDeletion(DeleteJob) error` method
- [ ] **6.4** Remove `ManualDeleteItem` from approval.go (replaced by `ManualDeleteRequest` in deletion_intake.go)
- [ ] **6.5** Remove dead code: resolution logic in `ExecuteApproval`, `ManualDelete`, and `expireItem` that was replaced by intake methods
- [ ] **6.6** Update `DeleteJobSummary` if any fields changed (likely unchanged)
- [ ] **6.7** Update all deletion_test.go tests — replace direct `QueueDeletion(DeleteJob{...})` calls with appropriate `QueueFromX()` calls. Tests that specifically test the worker/processing internals can use `enqueue()` directly (same package, unexported is accessible).
- [ ] **6.8** Remove `GetDiskGroupIDForIntegration` from `diskgroup.go` if it was added during the interrupted fix (should not exist outside this refactor)
- [ ] **6.9** Final `make ci` — full pipeline green

### Phase 7: Validation

- [ ] **7.1** Run the full test suite with `-race` flag to catch any concurrency issues from the restructuring
- [ ] **7.2** Manual smoke test via Docker: approve an item from approval queue, verify it deletes correctly (the original bug scenario)
- [ ] **7.3** Manual smoke test: manual delete from a non-default-mode disk group
- [ ] **7.4** Manual smoke test: sunset expiry processes correctly
- [ ] **7.5** Verify no regressions in grace period behavior (queue items, cancel, clear)

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
