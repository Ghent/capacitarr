# Remove Plex MediaSource & Fix DiskGroup Scoping

**Created:** 2026-03-20T15:43Z
**Status:** ✅ Complete
**Branch:** v2 (breaking changes allowed — no incremental migrations needed)

## Overview

Two related issues discovered during codebase audit:

1. **Plex (and potentially other media servers) registered as `MediaSource`** — adds unmanaged media items to the evaluation pool, inflating counts, polluting analytics, and causing deletion failures when items lack a `MediaDeleter` capability.

2. **Global operations that should be scoped to disk groups** — approval queue, preview cache, analytics, capacity forecast, dashboard stats, and audit log lack disk group context.

## Phase 1: Enforce MediaSource Exclusivity for Media Management Integrations

### Architectural Principle

**`MediaSource` must only be implemented by integrations that also implement `MediaDeleter` and `DiskReporter`** — i.e., integrations that authoritatively manage media content and can delete it (*arr integrations). Enrichment-only integrations (media servers, analytics services, request managers) must NEVER provide media items into the evaluation pool.

This is because:
- Only *arr integrations can delete what they report — Plex/Jellyfin/Emby/Tautulli/Seerr cannot
- Only *arr integrations report accurate disk space and root folders for DiskGroup matching
- Enrichment sources see ALL content including unmanaged items, causing duplicates and false positives
- Mixing authoritative and non-authoritative media items makes analytics unreliable

### Current Violations

| Integration | Currently implements MediaSource? | Should implement? | Action |
|---|---|---|---|
| Sonarr | ✅ | ✅ (authoritative) | No change |
| Radarr | ✅ | ✅ (authoritative) | No change |
| Lidarr | ✅ | ✅ (authoritative) | No change |
| Readarr | ✅ | ✅ (authoritative) | No change |
| **Plex** | **✅ (BUG)** | **❌** | **Remove** |
| Tautulli | ❌ | ❌ | Verified OK |
| Seerr | ❌ | ❌ | Verified OK |
| Jellyfin | ❌ | ❌ | Verified OK |
| Emby | ❌ | ❌ | Verified OK |

### Problem Details

`PlexClient` implements `MediaSource` (line 310 of `plex.go`), so the poller's `fetchAllIntegrations()` calls `GetMediaItems()` on Plex and merges those items into the same `allItems` pool alongside Sonarr/Radarr items. This causes:

- Duplicate items (Sonarr's "Firefly" + Plex's "Firefly" both appear)
- Unmanaged items (Plex-only content that no *arr manages) entering evaluation
- Deletion failures (Plex doesn't implement `MediaDeleter`)
- Inflated media stats per integration
- Polluted analytics (quality distribution, bloat, dead content include non-*arr items)

### Solution

Remove the `MediaSource` interface from `PlexClient`. Plex should only provide enrichment data via `WatchDataProvider` and `WatchlistProvider`. The capability comment in `types.go:37` should be updated to reflect:

```
Plex: Connectable + WatchDataProvider + WatchlistProvider
```

Note: `FetchCollectionValues()` in `integration.go:76-124` directly calls `client.GetMediaItems()` on Plex to extract collection names. This must be refactored to use a dedicated method or a lightweight collection-only API call rather than fetching all media items.

### Refactoring Detail: PlexClient.GetMediaItems()

`GetMediaItems()` is currently public and satisfies the `MediaSource` interface. However, it's also called internally by:

1. **`GetBulkWatchData()`** (`plex.go:245`) — calls `GetMediaItems()` to build a title→watch-data map for enrichment
2. **`FetchCollectionValues()`** (`integration.go:96-97`) — creates a temporary `PlexClient` and calls `GetMediaItems()` to extract collection names from all items

Neither of these callers needs the `MediaSource` interface — they just need the raw item data.

**Approach**: Rename `GetMediaItems()` → `getMediaItems()` (unexported). This:
- Removes `PlexClient` from the `MediaSource` interface (no exported `GetMediaItems`)
- Keeps `GetBulkWatchData()` working (calls the private method)
- Requires a new `GetCollectionNames() ([]string, error)` public method for collection extraction
- Requires `FetchCollectionValues()` to call the new `GetCollectionNames()` instead of `GetMediaItems()`

Note: The other enrichment sources (Jellyfin, Emby, Tautulli, Seerr) don't need this refactoring — they never had `GetMediaItems()` or `MediaSource`. They each use their own purpose-built methods for enrichment data.

### Steps

- [x] **1.1** Rename `GetMediaItems()` → `getMediaItems()` (unexported) on `PlexClient` in `plex.go`
- [x] **1.2** Update `GetBulkWatchData()` in `plex.go:245` to call `p.getMediaItems()` instead of `p.GetMediaItems()`
- [x] **1.3** Remove `var _ MediaSource = (*PlexClient)(nil)` compile-time assertion from `plex.go:310`
- [x] **1.4** Update capability comment in `types.go:36-41` to:
  - Remove `MediaSource` from Plex's line
  - Add an explicit note: "Only *arr integrations implement MediaSource — enrichment sources must NOT"
- [x] **1.5** Add a new `GetCollectionNames() ([]string, error)` public method to `PlexClient`:
  - Calls `p.getMediaItems()` internally
  - Extracts and deduplicates collection tag strings from all items
  - Returns a sorted list of unique collection names
- [x] **1.6** Refactor `FetchCollectionValues()` in `integration.go:76-124`:
  - Change `client.GetMediaItems()` → `client.GetCollectionNames()`
  - Remove the nested loop that extracted collections from items (now done by `GetCollectionNames()`)
  - Simplify the seen-map logic
- [x] **1.7** Update all Plex tests in `plex_test.go`:
  - Remove `MediaSource` / `GetMediaItems` test cases
  - Add a negative compile-time assertion: `var _ MediaSource = (*PlexClient)(nil)` should NOT compile (verify via test comment)
  - Add tests for `GetCollectionNames()`
- [x] **1.8** Verify all enrichment-only integrations do NOT implement `MediaSource`:
  - Jellyfin: `Connectable + WatchDataProvider + WatchlistProvider` ✅ (confirmed no `MediaSource`)
  - Emby: `Connectable + WatchDataProvider + WatchlistProvider` ✅ (confirmed no `MediaSource`)
  - Tautulli: `Connectable` ✅ (confirmed no `MediaSource`)
  - Seerr: `Connectable + RequestProvider` ✅ (confirmed no `MediaSource`)
- [x] **1.9** Add a comment to the `IntegrationRegistry.Register()` method noting that if a client implements `MediaSource` but not `MediaDeleter`, it's likely a misconfiguration. Also added runtime warning via slog.Warn.
- [x] **1.10** Run `make ci` to verify no compilation errors or test failures

## Phase 2: Add DiskGroupID to Approval Queue and Audit Log

### Problem

`ApprovalQueueItem` and `AuditLogEntry` have no `DiskGroupID` field. Operations like `ClearQueue()`, `ListQueue()`, and audit log queries run globally with no ability to filter by which disk group triggered the action.

### Solution

Add `DiskGroupID` as a nullable FK to both models. The poller knows which disk group is being evaluated in `evaluateAndCleanDisk()` and can pass it through to `UpsertPending()` and `AuditLog.Create()`.

### Steps

- [x] **2.1** Add `DiskGroupID *uint` field to `ApprovalQueueItem` model in `models.go` with JSON tag and gorm index
- [x] **2.2** Add `DiskGroupID *uint` field to `AuditLogEntry` model in `models.go` with JSON tag and gorm index
- [x] **2.3** Update baseline migration `00001_v2_baseline.sql`:
  - Add `disk_group_id INTEGER REFERENCES disk_groups(id) ON DELETE SET NULL` to `approval_queue` table
  - Add `disk_group_id INTEGER REFERENCES disk_groups(id) ON DELETE SET NULL` to `audit_log` table
  - Add indexes: `idx_approval_queue_disk_group_id`, `idx_audit_log_disk_group_id`
- [ ] **2.4** Manually update the running database to add the columns and indexes (SQL statements noted in plan for reference)
- [x] **2.5** Verify that the migration detect logic (which compares running schema vs baseline) still works correctly — confirmed via TestMigrationUpDownUp passing

## Phase 3: Thread DiskGroupID Through the Poller

### Problem

The poller's `evaluateAndCleanDisk()` has the `group db.DiskGroup` parameter with the disk group ID, but doesn't pass it to `ApprovalService.UpsertPending()` or `AuditLogService.UpsertDryRun()`.

### Steps

- [x] **3.1** Update `ApprovalService.UpsertPending()` to accept and store `DiskGroupID`
- [x] **3.2** Update the `evaluate.go` approval-mode block to pass `group.ID` as `DiskGroupID` in the `ApprovalQueueItem`
- [x] **3.3** Update the `evaluate.go` dry-run block to pass `group.ID` as `DiskGroupID` in the `AuditLogEntry`
- [x] **3.4** Update the `DeleteJob` struct to include `DiskGroupID *uint`
- [x] **3.5** Update the auto-mode block in `evaluate.go` to include `DiskGroupID` in the `DeleteJob`
- [x] **3.6** Update `DeletionService.processJob()` to pass `DiskGroupID` through to `AuditLogService.Create()` when recording the deletion (all three audit log entries: cancelled, dry-delete, deleted)
- [x] **3.7** Update `processForceDeletes()` — force deletes use nil `DiskGroupID` by default (no field set on DeleteJob)
- [x] **3.8** Update relevant tests — existing tests pass with default nil DiskGroupID

## Phase 4: Scope Approval Queue Operations to DiskGroup

### Problem

- `ClearQueue()` deletes all pending/rejected items globally
- `ListQueue()` returns all items without disk group filtering
- `IsSnoozed()` checks globally

### Steps

- [x] **4.1** Add `ClearQueueForDiskGroup(diskGroupID uint)` method to `ApprovalService`
- [x] **4.2** Poller queue clearing uses global ClearQueue when all groups below threshold; per-group clearing available via ClearQueueForDiskGroup
- [x] **4.3** Add `diskGroupID *uint` filter parameter to `ListQueue()` (nil means all)
- [x] **4.4** Update approval routes to accept optional `disk_group_id` query parameter
- [x] **4.5** Update `IsSnoozed()` to optionally scope by disk group via variadic param
- [x] **4.6** Update `BulkUnsnooze()` to accept `diskGroupID *uint` for per-group semantics
- [x] **4.7** Update approval tests, preview mock interface, and all callers

## Phase 5: Scope Analytics and Preview Cache

### Problem

The preview cache is a single global `*PreviewResult`. All analytics methods (`GetQualityDistribution`, `GetSizeAnomalies`, `GetStorageSunburst`, `GetDeadContent`, `GetStaleContent`, `GetLibraryStatusBreakdown`) operate on the full global item set with no disk group filtering.

### Solution

Rather than creating per-disk-group caches (which would multiply memory and complexity), add a filtering layer: analytics methods accept an optional `diskGroupID` and filter items by checking if the item's path falls under the disk group's mount path.

To enable this filtering, the preview cache items need to retain their `Path` field (they already do in `MediaItem`), and the analytics service needs access to disk group mount paths.

### Steps

- [x] **5.1** Add `diskGroupID *uint` parameter to all analytics methods
- [x] **5.2** Add `DiskGroupLister` dependency to `AnalyticsService` and `WatchAnalyticsService`; extended existing `DiskGroupLister` interface in preview.go with `GetByID()`
- [x] **5.3** Implement `filterItemsByDiskGroup()` helper on both services
- [x] **5.4** Update all analytics route handlers with `parseDiskGroupID()` helper and `disk_group_id` query param
- [ ] **5.5** Update `buildDiskContext()` in the preview service — deferred: the existing preview route doesn't need this for the current plan scope
- [ ] **5.6** Update preview route handler — deferred: tied to 5.5
- [x] **5.7** Update analytics and preview tests (benchmark tests, analytics_test.go, preview_test.go mock)

## Phase 6: Fix Capacity Forecast and Dashboard Stats

### Problem

- `analyticsForecastHandler()` hardcodes `groups[0]` — should accept a `disk_group_id` parameter
- `GetCapacityForecast()` fetches history without disk group filtering
- `GetDashboardStats()` library growth rate crosses disk groups randomly

### Steps

- [x] **6.1** Update `analyticsForecastHandler()` to accept `disk_group_id` query parameter; default to the most degraded group (highest usage %) instead of `groups[0]`
- [x] **6.2** `GetCapacityForecast()` already accepts threshold/capacity/used params — the route handler now selects the correct disk group and passes its values
- [ ] **6.3** `GetDashboardStats()` aggregates globally — left as-is since dashboard shows lifetime totals; per-disk-group scoping would be a future enhancement
- [x] **6.4** Dashboard stats tests unchanged — global aggregation behavior preserved

## Phase 7: Run Full CI Verification

- [x] **7.1** Run `make ci` from the `capacitarr/` directory
- [x] **7.2** Fixed all compilation errors, lint warnings (gofmt), and test failures (factory_test.go, analytics_test.go, benchmark_test.go, preview_test.go)
- [x] **7.3** Migration detection logic verified — TestMigrationUpDownUp passes with updated baseline

## Phase 8: Approval Queue Reconciliation on Threshold Change

**Problem:** The approval queue was append-only during each engine cycle. When a user adjusted thresholds (e.g., raised from 95% to 98%), the queue retained stale items from previous cycles that were no longer needed at the new threshold level.

**Solution (two parts):**

### Part 1: Per-cycle queue reconciliation in `evaluateAndCleanDisk()`

After the candidate loop in `evaluate.go`, a reconciliation step now:
1. Tracks which items were actually upserted/kept this cycle in a `neededKeys` set
2. Calls `ApprovalService.ReconcileQueue()` to dismiss pending items not in the set
3. Leaves rejected/snoozed items untouched — only `status=pending` items are eligible

- [x] **8.1** Add `ListPendingForDiskGroup(diskGroupID uint)` to `ApprovalService`
- [x] **8.2** Add `ReconcileQueue(diskGroupID uint, neededKeys map[string]bool)` to `ApprovalService`
- [x] **8.3** Add `ApprovalQueueReconciledEvent` to `events/types.go`
- [x] **8.4** Track `neededKeys` in the approval-mode candidate loop in `evaluate.go`
- [x] **8.5** Call `ReconcileQueue()` after the candidate loop (approval mode only)

### Part 2: Instant feedback on threshold change

When the user changes thresholds via the UI, a `ThresholdChangedEvent` fires. The `DiskGroupService.UpdateThresholds()` now also triggers an immediate engine run via `EngineService.TriggerRun()`. The engine cycle naturally reconciles the queue using Part 1's logic.

- [x] **8.6** Add `EngineRunTrigger` interface and `SetEngineService()` to `DiskGroupService`
- [x] **8.7** Call `engine.TriggerRun()` in `UpdateThresholds()` after publishing the event
- [x] **8.8** Wire `DiskGroupService → EngineService` in `NewRegistry()`

### Tests

- [x] **8.9** `TestApprovalService_ListPendingForDiskGroup` — verifies disk group scoping
- [x] **8.10** `TestApprovalService_ReconcileQueue_DismissesStaleItems` — verifies stale items are dismissed
- [x] **8.11** `TestApprovalService_ReconcileQueue_LeavesRejectedUntouched` — verifies rejected items survive
- [x] **8.12** `TestApprovalService_ReconcileQueue_NoopWhenAllNeeded` — verifies no-op when all items are current
- [x] **8.13** `TestDiskGroupService_UpdateThresholds_TriggersEngineRun` — verifies engine trigger
- [x] **8.14** `TestDiskGroupService_UpdateThresholds_NoEngineService` — verifies nil-safety
- [x] **8.15** `TestEvaluateAndCleanDisk_ReconcilesDismissesStaleItems` — end-to-end poller reconciliation
- [x] **8.16** `TestEvaluateAndCleanDisk_ReconcileNoopInDryRun` — verifies reconciliation skipped in dry-run

## Manual Database Update SQL

Since this is a breaking-change branch, the running database must be updated manually:

```sql
-- Add disk_group_id to approval_queue
ALTER TABLE approval_queue ADD COLUMN disk_group_id INTEGER REFERENCES disk_groups(id) ON DELETE SET NULL;
CREATE INDEX idx_approval_queue_disk_group_id ON approval_queue(disk_group_id);

-- Add disk_group_id to audit_log
ALTER TABLE audit_log ADD COLUMN disk_group_id INTEGER REFERENCES disk_groups(id) ON DELETE SET NULL;
CREATE INDEX idx_audit_log_disk_group_id ON audit_log(disk_group_id);
```

## Integration Capability Matrix (After Changes)

| Integration | Connectable | MediaSource | DiskReporter | MediaDeleter | WatchDataProvider | WatchlistProvider | RequestProvider | RuleValueFetcher |
|---|---|---|---|---|---|---|---|---|
| Sonarr | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ |
| Radarr | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ |
| Lidarr | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ |
| Readarr | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ | ✅ |
| Plex | ✅ | ❌ | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ |
| Tautulli | ✅ | ❌ | ❌ | ❌ | ❌* | ❌ | ❌ | ❌ |
| Seerr | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ |
| Jellyfin | ✅ | ❌ | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ |
| Emby | ✅ | ❌ | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ |

*Tautulli uses per-item queries via `TautulliEnricher`, not the `WatchDataProvider` bulk interface.

## Files Modified

### Phase 1 (Plex MediaSource removal)
- `backend/internal/integrations/plex.go` — remove `GetMediaItems()`, remove `MediaSource` assertion
- `backend/internal/integrations/types.go` — update capability comment
- `backend/internal/integrations/plex_test.go` — remove MediaSource tests
- `backend/internal/services/integration.go` — refactor `FetchCollectionValues()`

### Phase 2 (Schema changes)
- `backend/internal/db/models.go` — add `DiskGroupID` to `ApprovalQueueItem` and `AuditLogEntry`
- `backend/internal/db/migrations/00001_v2_baseline.sql` — add columns and indexes

### Phase 3 (Poller threading)
- `backend/internal/poller/evaluate.go` — pass disk group ID through all code paths
- `backend/internal/services/approval.go` — accept disk group ID in `UpsertPending()`
- `backend/internal/services/deletion.go` — `DeleteJob` gains `DiskGroupID`
- `backend/internal/services/auditlog.go` — `Create()` and `UpsertDryRun()` gain disk group context

### Phase 4 (Approval queue scoping)
- `backend/internal/services/approval.go` — new per-group methods
- `backend/internal/poller/poller.go` — per-group queue clearing
- `backend/routes/approval.go` — disk group filter parameter

### Phase 5 (Analytics scoping)
- `backend/internal/services/analytics.go` — disk group filter parameter
- `backend/internal/services/watch_analytics.go` — disk group filter parameter
- `backend/internal/services/preview.go` — disk group aware build context
- `backend/routes/analytics.go` — query parameter

### Phase 6 (Forecast/dashboard scoping)
- `backend/internal/services/metrics.go` — disk group scoping
- `backend/routes/analytics.go` — forecast handler
- `backend/routes/metrics.go` — dashboard stats handler

### Phase 8 (Approval queue reconciliation)
- `backend/internal/services/approval.go` — `ListPendingForDiskGroup()`, `ReconcileQueue()`
- `backend/internal/events/types.go` — `ApprovalQueueReconciledEvent`
- `backend/internal/poller/evaluate.go` — `neededKeys` tracking + reconciliation after candidate loop
- `backend/internal/services/diskgroup.go` — `EngineRunTrigger` interface, `SetEngineService()`, engine trigger in `UpdateThresholds()`
- `backend/internal/services/registry.go` — wire `DiskGroupService → EngineService`
- `backend/internal/services/approval_test.go` — reconciliation tests
- `backend/internal/services/diskgroup_test.go` — engine trigger tests
- `backend/internal/poller/evaluate_test.go` — end-to-end reconciliation tests
