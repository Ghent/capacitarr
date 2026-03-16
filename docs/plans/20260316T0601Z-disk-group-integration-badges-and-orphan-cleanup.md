# Disk Group Integration Badges & Orphan Cleanup

**Date:** 2026-03-16
**Status:** 📋 Planned
**Scope:** `capacitarr` (single repo)
**Branch:** `feature/disk-group-integration-tracking`

## Motivation

Two related issues identified during the disk-size-override feature work:

1. **Integration badges on disk groups** — Users can't tell which integrations contribute to a disk group. When Sonarr and Radarr share the same mount path, the disk group card just shows the path with no indication of which services are using it.

2. **Stale disk groups after integration removal** — When an integration is deleted, its disk groups persist until the next poll cycle (up to 5 minutes). This creates a confusing state where the dashboard shows disk groups from a service that no longer exists.

## Part 1: Integration Badges

### Current Behavior

Disk groups are identified by mount path only. There's no FK relationship between `disk_groups` and `integration_configs`. The poller discovers disk groups from *arr API disk space responses and creates/updates them by mount path. Multiple integrations sharing the same mount path get merged into one disk group.

### Proposed Design

Track which integrations reported each disk group during the last poll cycle.

#### Option A: Junction Table

Add a `disk_group_integrations` junction table:

```sql
CREATE TABLE disk_group_integrations (
    disk_group_id INTEGER NOT NULL REFERENCES disk_groups(id) ON DELETE CASCADE,
    integration_id INTEGER NOT NULL REFERENCES integration_configs(id) ON DELETE CASCADE,
    PRIMARY KEY (disk_group_id, integration_id)
);
```

During each poll, after upserting disk groups, populate this table with the integration IDs that reported each mount path. Clear and repopulate on each poll cycle.

#### Option B: JSON Column

Add an `integration_ids` JSON column to `disk_groups`:

```sql
ALTER TABLE disk_groups ADD COLUMN integration_ids TEXT DEFAULT '[]';
```

Simpler but less relational. Updated during each poll.

#### Frontend Display

Show small badges next to the mount path in both the dashboard `DiskGroupSection` and the rules `RuleDiskThresholds`:

```
/host/data  [sonarr] [radarr]
39.1 TB / 40.0 TB
```

Use `UiBadge variant="outline"` with the integration type as text, matching the existing badge style used in the error banner.

## Part 2: Orphan Cleanup on Integration Delete

### Current Behavior

`CleanOrphanedDiskGroups()` runs during each poll cycle. It removes disk groups whose mount paths are not in the set of active mount paths discovered during that poll. When an integration is deleted, the disk groups persist until the next poll proves they're orphaned.

### Proposed Design

When an integration is deleted via the API, trigger an immediate orphan check:

1. After deleting the integration, run a poll-like check:
   - Fetch disk space from all remaining enabled integrations
   - Call `CleanOrphanedDiskGroups()` with the resulting active mount set
2. If no enabled integrations remain, clear all disk groups immediately

This should be implemented in the `IntegrationService.Delete()` method, not in the route handler (per service layer architecture rules).

### Edge Cases

| Scenario | Behavior |
|---|---|
| Delete integration, others still report same mount | Disk group preserved (still active) |
| Delete integration, no others report that mount | Disk group removed immediately |
| Delete all integrations | All disk groups removed immediately |
| Integration has transient error (not deleted) | Disk groups preserved until next successful poll |

## Implementation Steps

### Step 1: Migration — Add Junction Table

**File:** `backend/internal/db/migrations/00009_disk_group_integrations.sql`

### Step 2: Update DiskGroup Model

Add `IntegrationIDs` field (populated from junction table) to the API response.

### Step 3: Update Poller

After upserting disk groups, populate the junction table with the integration IDs that reported each mount path.

### Step 4: Update SettingsService

Add `GetDiskGroupIntegrations()` method to fetch the junction data.

### Step 5: Update API Response

Include integration names/types in the disk group API response.

### Step 6: Update Frontend Types

Add integration info to the `DiskGroup` TypeScript interface.

### Step 7: Update Frontend Components

Show integration badges in `DiskGroupSection.vue` and `RuleDiskThresholds.vue`.

### Step 8: Implement Orphan Cleanup on Delete

Update `IntegrationService.Delete()` to trigger `CleanOrphanedDiskGroups()` after deletion.

### Step 9: Write Tests

- Junction table population during poll
- Orphan cleanup on integration delete
- API response includes integration info
- Frontend badge rendering

### Step 10: Run `make ci`
