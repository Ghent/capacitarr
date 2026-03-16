# Disk Group Integration Badges & Orphan Cleanup

**Date:** 2026-03-16
**Status:** ✅ Complete
**Scope:** `capacitarr` (single repo)
**Branch:** `feature/disk-group-integration-tracking`

## Motivation

Two related issues identified during the disk-size-override feature work:

1. **Integration badges on disk groups** — Users can't tell which integrations contribute to a disk group. When Sonarr and Radarr share the same mount path, the disk group card just shows the path with no indication of which services are using it.

2. **Stale disk groups after integration removal** — When all integrations are deleted, disk groups persist indefinitely. The poller early-returns at zero integrations (line 149 of `poller.go`), so `CleanOrphanedDiskGroups()` never runs. `IntegrationService.Delete()` has no awareness of disk groups at all.

## Root Cause Analysis

Two co-dependent bugs prevent disk group cleanup:

| Bug | Location | Behavior |
|---|---|---|
| Poller early-return | `poller.go:149-152` | When `len(configs) == 0`, the poller returns before reaching `CleanOrphanedDiskGroups()` at line 190 |
| No cleanup on delete | `integration.go:414-431` | `IntegrationService.Delete()` only deletes the DB record and publishes an event — no disk group awareness |

Additionally, the cleanup guard at line 190 (`if len(mediaMounts) > 0`) would also skip cleanup even if the early return were removed, since zero integrations produce zero media mounts.

## Architectural Change: Extract DiskGroupService

Disk group operations are currently scattered across `SettingsService`, `BackupService` (direct DB), `IntegrationService`, and the poller. This work extracts a dedicated `DiskGroupService` to centralize all disk group lifecycle management.

### Current Touchpoints (to be rewired)

| Location | Current access | Change |
|---|---|---|
| `SettingsService` | Owns 5 DG methods | Methods move to `DiskGroupService` |
| `BackupService` export | Direct `s.db.Find` | Call `DiskGroupService.List()` |
| `BackupService` import | Direct `s.db.Where/Create/Save` | Call `DiskGroupService.ImportUpsert()` |
| `IntegrationService.SyncAll()` | Via `DiskGroupUpserter` interface | Interface points to `DiskGroupService` |
| `Poller.poll()` | Via `reg.Settings.*` | Via `reg.DiskGroup.*` |
| `disk_groups.go` routes | Via `reg.Settings.*` | Via `reg.DiskGroup.*` |

### Indirect References (no changes needed)

| Location | Usage |
|---|---|
| `MetricsService` | Queries `LibraryHistory` by `disk_group_id` FK |
| `Poller.evaluateAndCleanDisk()` | Receives `db.DiskGroup` as parameter |

### DiskGroupService API

```go
type DiskGroupService struct {
    db  *gorm.DB
    bus *events.EventBus
}

// CRUD
func (s *DiskGroupService) List() ([]db.DiskGroup, error)
func (s *DiskGroupService) GetByID(id uint) (*db.DiskGroup, error)
func (s *DiskGroupService) Upsert(disk integrations.DiskSpace) (*db.DiskGroup, error)
func (s *DiskGroupService) UpdateThresholds(groupID uint, threshold, target float64, totalOverride *int64) (*db.DiskGroup, error)

// Lifecycle
func (s *DiskGroupService) RemoveAll() (int64, error)
func (s *DiskGroupService) ReconcileActiveMounts(activeMounts map[string]bool) (int64, error)

// Backup support
func (s *DiskGroupService) ImportUpsert(mountPath string, threshold, target float64, totalOverride *int64) error

// Integration tracking (junction table)
func (s *DiskGroupService) SyncIntegrationLinks(diskGroupID uint, integrationIDs []uint) error
func (s *DiskGroupService) ListWithIntegrations() ([]DiskGroupWithIntegrations, error)
```

### Registry Changes

```go
type Registry struct {
    // ... existing fields ...
    DiskGroup *DiskGroupService  // NEW
}
```

### IntegrationService Dependency Change

The existing `DiskGroupUpserter` interface expands:

```go
// Before
type DiskGroupUpserter interface {
    UpsertDiskGroup(disk integrations.DiskSpace) (*db.DiskGroup, error)
}

// After
type DiskGroupManager interface {
    Upsert(disk integrations.DiskSpace) (*db.DiskGroup, error)
    RemoveAll() (int64, error)
}
```

`IntegrationService.Delete()` checks if any enabled integrations remain after deletion. If none, calls `RemoveAll()`.

## Part 1: Integration Badges

### Junction Table

Add a `disk_group_integrations` junction table:

```sql
CREATE TABLE disk_group_integrations (
    disk_group_id INTEGER NOT NULL REFERENCES disk_groups(id) ON DELETE CASCADE,
    integration_id INTEGER NOT NULL REFERENCES integration_configs(id) ON DELETE CASCADE,
    PRIMARY KEY (disk_group_id, integration_id)
);
```

During each poll, after upserting disk groups, populate this table with the integration IDs that reported each mount path. Clear and repopulate on each poll cycle.

### API Response

The disk groups endpoint enriches each group with integration metadata:

```json
{
  "id": 1,
  "mountPath": "/host/data",
  "totalBytes": 43980465111040,
  "usedBytes": 42970332569600,
  "thresholdPct": 85,
  "targetPct": 75,
  "integrations": [
    { "id": 2, "name": "Sonarr", "type": "sonarr" },
    { "id": 3, "name": "Radarr", "type": "radarr" }
  ]
}
```

### Frontend Display

Show small badges next to the mount path in both `DiskGroupSection.vue` and `RuleDiskThresholds.vue`:

```
/host/data  [sonarr] [radarr]
39.1 TB / 40.0 TB
```

Use `UiBadge variant="outline"` with the integration type as text.

## Part 2: Orphan Cleanup

### Poller Fix

When `len(configs) == 0`, call `reg.DiskGroup.RemoveAll()` before returning. This handles:
- All integrations disabled
- All integrations deleted
- Manual engine run with no active integrations

### IntegrationService.Delete() Fix

After deleting the integration:
1. Check `ListEnabled()` — if empty, call `DiskGroupManager.RemoveAll()`
2. If not empty, skip — the next poll cycle will reconcile active mounts

### Edge Cases

| Scenario | Behavior |
|---|---|
| Delete integration, others still report same mount | Disk group preserved until next poll |
| Delete integration, no others report that mount | Removed at next poll cycle |
| Delete all integrations | All disk groups removed immediately |
| All integrations disabled | All disk groups removed at next poll/manual run |
| Integration has transient error | Disk groups preserved until next successful poll |

## Implementation Steps

### Step 1: Create DiskGroupService

**File:** `backend/internal/services/diskgroup.go`

Extract these methods from `SettingsService`:
- `UpsertDiskGroup()` → `Upsert()`
- `CleanOrphanedDiskGroups()` → `ReconcileActiveMounts()`
- `ListDiskGroups()` → `List()`
- `GetDiskGroup()` → `GetByID()`
- `UpdateThresholds()` → `UpdateThresholds()`

Add new methods:
- `RemoveAll()` — deletes all disk groups
- `ImportUpsert()` — creates/updates a disk group from backup import data

Constructor: `NewDiskGroupService(database *gorm.DB, bus *events.EventBus) *DiskGroupService`

### Step 2: Register DiskGroupService on Registry

**File:** `backend/internal/services/registry.go`

- Add `DiskGroup *DiskGroupService` field to `Registry`
- Construct in `NewRegistry()`
- Wire `IntegrationService` dependency: `reg.Integration.SetDiskGroupService(diskGroupSvc)` (replaces `SetSettingsService`)
- Wire `BackupService` dependency: add `SetDiskGroupService()` setter

### Step 3: Remove disk group methods from SettingsService

**File:** `backend/internal/services/settings.go`

Delete: `UpsertDiskGroup()`, `CleanOrphanedDiskGroups()`, `ListDiskGroups()`, `GetDiskGroup()`, `UpdateThresholds()`

### Step 4: Update IntegrationService

**File:** `backend/internal/services/integration.go`

- Rename `DiskGroupUpserter` → `DiskGroupManager` with expanded interface
- Rename `SetSettingsService()` → `SetDiskGroupService()`
- In `SyncAll()`: `s.settings.UpsertDiskGroup(d)` → `s.diskGroups.Upsert(d)`
- In `Delete()`: after deletion, check `ListEnabled()` — if empty, call `s.diskGroups.RemoveAll()`

### Step 5: Update BackupService

**File:** `backend/internal/services/backup.go`

- Add `diskGroups *DiskGroupService` field and `SetDiskGroupService()` setter
- Export: replace `s.db.Find(&groups)` with `s.diskGroups.List()`
- Import: replace direct `s.db.Where/Create/Save` with `s.diskGroups.ImportUpsert()`

### Step 6: Update Poller

**File:** `backend/internal/poller/poller.go`

- Replace `p.reg.Settings.UpsertDiskGroup()` with `p.reg.DiskGroup.Upsert()`
- Replace `p.reg.Settings.CleanOrphanedDiskGroups()` with `p.reg.DiskGroup.ReconcileActiveMounts()`
- Before the zero-integration early return: add `p.reg.DiskGroup.RemoveAll()`

### Step 7: Update Route Handlers

**File:** `backend/routes/disk_groups.go`

- Replace `reg.Settings.ListDiskGroups()` with `reg.DiskGroup.List()` (or `reg.DiskGroup.ListWithIntegrations()` for badge support)
- Replace `reg.Settings.GetDiskGroup()` with `reg.DiskGroup.GetByID()`
- Replace `reg.Settings.UpdateThresholds()` with `reg.DiskGroup.UpdateThresholds()`

### Step 8: Migration — Add Junction Table

**File:** `backend/internal/db/migrations/00009_disk_group_integrations.sql`

### Step 9: Update DiskGroup Model

**File:** `backend/internal/db/models.go`

Add `DiskGroupIntegration` model for the junction table. Add `IntegrationInfo` struct for API responses.

### Step 10: Add Integration Tracking to Poller

**File:** `backend/internal/poller/poller.go`

After upserting disk groups, call `DiskGroupService.SyncIntegrationLinks()` with the integration IDs that reported each mount path.

### Step 11: Update API Response

**File:** `backend/routes/disk_groups.go`

Use `ListWithIntegrations()` for the GET endpoint to include integration names/types.

### Step 12: Update Frontend Types

**File:** `frontend/app/types/api.ts`

Add integration info to the `DiskGroup` TypeScript interface:

```typescript
export interface DiskGroupIntegration {
  id: number;
  name: string;
  type: string;
}

export interface DiskGroup {
  // ... existing fields ...
  integrations?: DiskGroupIntegration[];
}
```

### Step 13: Update Frontend Components

**Files:**
- `frontend/app/components/DiskGroupSection.vue` — add integration badges next to mount path
- `frontend/app/components/rules/RuleDiskThresholds.vue` — add integration badges

### Step 14: Write Tests

**File:** `backend/internal/services/diskgroup_test.go`

- Move disk group tests from `settings_test.go`
- Add tests for `RemoveAll()`
- Add tests for `ReconcileActiveMounts()` edge cases
- Add tests for `ImportUpsert()`
- Add tests for `SyncIntegrationLinks()`
- Add tests for `ListWithIntegrations()`

**File:** `backend/internal/services/integration_test.go`

- Add test: `Delete()` calls `RemoveAll()` when last integration is deleted
- Add test: `Delete()` does NOT call `RemoveAll()` when other integrations remain

**File:** `backend/internal/poller/poller_test.go`

- Add/update test: zero integrations triggers `RemoveAll()`

**File:** `backend/routes/disk_groups_test.go`

- Update existing tests to work with `reg.DiskGroup` instead of `reg.Settings`
- Add test: GET endpoint returns integration badges

### Step 15: Run `make ci`
