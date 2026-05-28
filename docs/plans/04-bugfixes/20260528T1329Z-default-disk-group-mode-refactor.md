# Refactor: Eliminate `DefaultDiskGroupMode` from Runtime Decision Paths

**Status:** ✅ Complete
**Created:** 2026-05-28
**Scope:** Backend (services, poller, routes), Frontend (composables)
**Priority:** Medium — no user-facing bug remaining after the fix in `fix/approval-dry-run-wrong-mode`, but the architectural confusion remains

## Background

In v2.x, Capacitarr had a single global execution mode (`ExecutionMode`) that governed all behavior. In v3.0, execution mode moved to per-disk-group (`DiskGroup.Mode`), but the global field was renamed to `DefaultDiskGroupMode` and kept on `PreferenceSet`.

This vestigial field has caused:
- **A critical bug** (fixed in `fix/approval-dry-run-wrong-mode`): `ExecuteApproval` checked `prefs.DefaultDiskGroupMode` instead of the actual disk group's mode, causing approved items to dry-delete and bounce back to the approval queue.
- **Dead code**: `useEngineControl.ts` exports a `setMode()` function that PUTs to `/api/v1/preferences`, but it is never called from any Vue component. This was presumably a v2.x dashboard toggle that was never wired up after the v3 per-disk-group migration.
- **Dead documented purpose**: The model comment says "used only as the default for newly auto-discovered disk groups", but `DiskGroupService.Upsert()` actually uses the GORM column default (`"dry-run"`) and never reads this preference.

## Goals

1. Make `DefaultDiskGroupMode` truly serve only its documented purpose: the default mode applied to newly auto-discovered disk groups
2. Remove all runtime decision paths that read `DefaultDiskGroupMode` when they should use `DiskGroup.Mode`
3. Remove dead frontend code related to the global mode toggle (`setMode()` in `useEngineControl.ts`)
4. Implement the "default for new groups" behavior that the model claims but doesn't implement
5. Fix `ManualDelete` to resolve mode from the integration's disk group

## Design Decision: No Global Mode Toggle

The v2.x global mode toggle no longer exists in the UI. Mode is set **per-disk-group** on the Rules page (`RuleDiskThresholds.vue`). There is no plan to reintroduce a global toggle. The `DefaultDiskGroupMode` preference remains in the schema only as:
- A default template for newly auto-discovered disk groups
- A fallback for legacy code paths where no disk group association exists
- A backup/restore compatibility field

## Current Usages (Audit)

| Location | Usage | Action |
|----------|-------|--------|
| `services/approval.go:551` | Fallback when `DiskGroupID` is nil | Keep as fallback (already fixed) |
| `services/deletion.go:810` | Fallback in `resolveCurrentMode()` when no disk group | Keep as fallback |
| `routes/deletion.go:135` | `ManualDelete` uses it for mode determination | Phase 2: resolve from integration's disk group |
| `poller/poller.go:242,252,257,339` | Engine run stats + events + logging | Phase 3: derive from per-group modes |
| `poller/poller.go:473` | `ModeAuto` check zeroes `writeFreedBytes` | Phase 3: derive from per-group modes (runtime bug) |
| `poller/poller.go:494` | `EngineCompleteEvent` payload | Phase 3: derive from per-group modes |
| `poller/evaluate.go:194` | Logging in candidate selection | Phase 3: derive from per-group modes |
| `services/metrics.go:379` | Worker stats API response | Phase 3: derive from disk groups |
| `services/diskgroup.go:129` | New group creation — does NOT read the preference | Phase 1: fix to read preference |
| `services/settings.go:153-168,221-235` | Mode change → queue clear + `EngineModeChangedEvent` | Phase 2: remove global path; replace with per-group clearing in `DiskGroupService` |
| `services/notification_dispatch.go:197` | `EngineModeChangedEvent` → `AlertModeChanged` | Phase 2: remove (dead event) |
| `events/types.go:82-94` | `EngineModeChangedEvent` struct definition | Phase 2: remove |
| Frontend `useEngineControl.ts:60` | Dashboard mode display (reads from worker stats) | Phase 3: derive from disk groups |
| Frontend `useEngineControl.ts:165-173` | SSE handler for `engine_mode_changed` | Phase 2: remove (dead event) |
| Frontend `useEngineControl.ts:227` | `setMode()` writes to preference | Phase 2: remove (dead code) |
| Frontend `useApprovalQueue.ts:67,76` | Derives `isApprovalMode` from single mode | Phase 3: update to per-group map |
| Frontend `DeletionQueueCard.vue:26,45` | Empty state message based on mode | Phase 3: update to per-group map |
| Frontend `index.vue:504` | Fallback when no disk groups exist | Keep as fallback |
| Frontend `index.vue:567,654,794` | Activity feed icon/color/subscription for mode event | Phase 2: remove |
| Frontend `constants.ts:42` | `EVENT_ENGINE_MODE_CHANGED` | Phase 2: remove |
| `services/backup.go` | Export/import | Keep (compatibility) |
| `routes/preferences.go` | API validation on update | Keep (still a valid preference) |

## Implementation Plan

### Phase 1: Fix "Default for New Groups" (Backend Only)

**Goal:** Make `DiskGroupService.Upsert()` actually apply `DefaultDiskGroupMode` when creating a new disk group.

- [x] 1.1. Add a `SettingsReader` dependency to `DiskGroupService` (or pass preferences in from the poller)
- [x] 1.2. In `DiskGroupService.Upsert()`, when creating a new group (`result.Error != nil` path), read `prefs.DefaultDiskGroupMode` and set `group.Mode` to that value instead of relying on the GORM column default
- [x] 1.3. Update `DiskGroupService` tests to verify new groups inherit the preference
- [x] 1.4. Run `make ci`

### Phase 2: Dead Code Removal + ManualDelete Fix (Backend + Frontend)

**Goal:** Remove the dead global mode toggle code and fix `ManualDelete` to resolve mode per-disk-group.

**Note on `deletionClearer`:** The `DeletionQueueClearer` interface and its injection into `SettingsService` must be **preserved**. Only the mode-change calls (`lines 154-155, 222-223`) are removed. The `DeletionsEnabled` toggle calls (`lines 175-176, 240-241`) are NOT dead code — they clear the queue when a user disables deletions globally, which is still valid behavior.

- [x] 2.1. Remove `setMode()` function from `useEngineControl.ts` (dead code — never called from any component)
- [x] 2.2. In `SettingsService.UpdatePreferences()` and `PatchEnginePreferences()`, remove only the mode-change code paths that clear the deletion queue and publish `EngineModeChangedEvent` when `DefaultDiskGroupMode` changes (lines 154-155, 165-168, 222-223, 232-235). Keep the `DeletionsEnabled` toggle paths (lines 175-176, 240-241) intact. Keep the `deletionClearer` interface and injection — they are still used.
- [x] 2.3. Remove `EngineModeChangedEvent` struct and its `EventType()`/`EventMessage()` methods from `events/types.go:82-94`
- [x] 2.4. Remove the notification dispatcher handler for `EngineModeChangedEvent` at `notification_dispatch.go:197-202` and the `AlertModeChanged` alert type if it becomes unreferenced
- [x] 2.5. Remove the SSE handler in `useEngineControl.ts` that listens for `engine_mode_changed` events (lines ~165-173) since the event is no longer published
- [x] 2.6. Remove `EVENT_ENGINE_MODE_CHANGED` constant from `frontend/app/constants.ts:42`
- [x] 2.7. Update `frontend/app/pages/index.vue` — remove `'engine_mode_changed'` from the `activityEventTypes` array (line 794) and the activity feed icon/color mappings (lines 567, 654). Note: existing persisted activity events with this type will still render in the feed but without a custom icon (falls through to default).
- [x] 2.8. Fix `ManualDelete` route: resolve the disk group mode from the integration's linked disk group (`DiskGroupIntegration` junction table), falling back to `DefaultDiskGroupMode` only when no disk group link exists. This must go through a service method (per service layer rules) — add a method like `DiskGroupService.GetModeForIntegration(integrationID uint) (string, error)` rather than resolving in the route handler.
- [x] 2.9. Add per-disk-group deletion queue clearing on any mode change. In `DiskGroupService.UpdateThresholds()`, when the mode changes (regardless of direction), clear the deletion queue entries scoped to that disk group. Use `DeletionService.ClearQueueForDiskGroup(diskGroupID)` (add this method if it doesn't exist — it should remove pending deletion queue entries for that group). Any mode change invalidates the assumptions under which items were queued — the safe route is to always clear and let the engine re-evaluate under the new mode.
- [x] 2.10. Update tests:
  - Remove/update `settings_test.go:76-92` (mode-change event assertion) and lines 106-195 (`mockDeletionQueueClearer` mode-change assertions — keep the `DeletionsEnabled` test cases)
  - Remove `notification_dispatch_test.go:247` (`EngineModeChangedEvent` dispatch test)
  - Remove `useEngineControl.test.ts` SSE `engine_mode_changed` test suite (lines 302-341)
  - Add test: ManualDelete resolves per-disk-group mode from integration link
  - Add test: `UpdateThresholds()` clears deletion queue on any mode change (auto → dry-run, dry-run → auto, etc.)
- [x] 2.11. Run `make ci`

### Phase 3: Poller + Stats Cleanup (Backend + Frontend)

**Goal:** Make logging, stats, events, and the dashboard report the actual mode of each disk group instead of a single global label.

**Approach:** Replace the flat `ExecutionMode string` field with a structured `DiskGroupModes map[uint]string` (groupID→mode) in events, stats, and API responses. This gives consumers full visibility into per-group modes without lossy reduction to a single value.

#### 3a: Schema + Struct Changes

- [x] 3.1. Drop the `execution_mode` column from `engine_run_stats` via a new migration (`00019_drop_execution_mode.sql`) using `ALTER TABLE engine_run_stats DROP COLUMN execution_mode`.
- [x] 3.2. Remove the `ExecutionMode` field from the `EngineRunStats` struct and the `EngineHistoryPoint` struct. Update `CreateRunStats()` to no longer accept a mode string parameter. Update `GetStats()` and `GetHistory()` to return `DiskGroupModes` instead of `ExecutionMode`.
- [x] 3.3. Add `DiskGroupModes map[uint]string` to `EngineStartEvent` and `EngineCompleteEvent`. Remove the `ExecutionMode string` field from both event structs.

#### 3b: Poller Changes

- [x] 3.4. In the poller, populate `DiskGroupModes` from the actual groups being processed in the current cycle. Update `poller.go:242` to call `CreateRunStats()` without a mode parameter (removed in 3.2). Replace `prefs.DefaultDiskGroupMode` reads at `poller.go:252,494` with the per-group mode map on the event structs.
- [x] 3.5. Fix `poller.go:473` — the `if pctx.prefs.DefaultDiskGroupMode == db.ModeAuto` check that zeroes `writeFreedBytes`. This should check whether **any** of the groups processed in the current cycle are in auto mode (i.e., check the per-group map). This is a **runtime decision bug**, not just cosmetic.
- [x] 3.6. Update poller logging (`poller.go:257,339`, `evaluate.go:194`) to log the per-group mode map instead of the global preference.

#### 3c: API + Frontend Changes

- [x] 3.7. In the worker stats API (`services/metrics.go`), replace `defaultDiskGroupMode` with `diskGroupModes` (the per-group map from disk group service).
- [x] 3.8. Update `useEngineControl.ts`: replace the `executionMode` computed (which reads `workerStats.defaultDiskGroupMode`) with a `diskGroupModes` computed that exposes the per-group map. Update the SSE handlers for `engine_start` and `engine_complete` to read `diskGroupModes` from the event payload. Always display all modes individually — do not collapse.
- [x] 3.9. Update `useApprovalQueue.ts:67,76`: currently derives `isApprovalMode` from the single `executionMode` value. Change to check whether **any** disk group is in approval mode from the per-group map exported by `useEngineControl`.
- [x] 3.10. Update `DeletionQueueCard.vue:26,45`: currently uses `executionMode` for the empty state message. Change to derive from the per-group map (e.g., show context based on which groups have deletion-capable modes).
- [x] 3.11. Update `index.vue:440,496,504,510`: the `effectiveMode` and `allDryRun` computeds use `engineExecutionMode` as a fallback. Replace with per-group map consumption. Keep the "no disk groups exist" fallback using `DefaultDiskGroupMode` from preferences directly.

#### 3d: Test Updates

- [x] 3.12. Update backend tests that seed or assert `ExecutionMode` on `EngineRunStats`.
- [x] 3.13. Update event tests that reference `ExecutionMode` on `EngineStartEvent`/`EngineCompleteEvent`.
- [x] 3.14. Update `useEngineControl.test.ts` and `useApprovalQueue.test.ts` to reflect the new per-group map API shape.
- [x] 3.15. Run `make ci`

### Phase 4: Documentation + Finalization

- [x] 4.1. Update the settings page tooltip/help text in the frontend i18n files to clarify what "Default Disk Group Mode" means (template for new groups only)
- [x] 4.2. Add a brief note in `docs/development.md` about the architecture: mode is per-disk-group, global preference is only a default template for auto-discovery
- [x] 4.3. Mark this plan as `✅ Complete` and move it from `docs/plans/00-active/` to `docs/plans/04-bugfixes/` using `git mv`

## Non-Goals

- **Removing `DefaultDiskGroupMode` from the database schema** — it remains as a valid preference (default template + backup compatibility)
- **Breaking the preferences API contract** — the field continues to be readable/writable via `/api/v1/preferences`
- **Reintroducing a global mode toggle** — mode is per-disk-group; there is no UX need for a global override
- **Changing per-disk-group mode setting** — the Rules page already allows setting mode per-group; this plan only fixes runtime paths and removes dead code

## Risks

- **Phase 2.2 removes the deletion queue clear on global mode change.** Currently this is dead code (no UI triggers it), but if any external API consumer PATCHes `defaultDiskGroupMode` directly, their queue would no longer be cleared. This is acceptable since the preference is now documented as "default for new groups only".
- **The `ManualDelete` fix (Phase 2.8) adds a DB query** to resolve integration→disk group mappings. For the common case (1-2 integrations), this is negligible.
- **Phase 3.5 changes runtime behavior:** `writeFreedBytes` zeroing will depend on per-group modes rather than the global preference. Users with mixed modes (some auto, some dry-run) will see more accurate stats.
- **Phase 2.9 adds new queue-clearing behavior.** When a disk group's mode changes in any direction, the deletion queue for that group is explicitly cleared. This is stricter than the previous behavior (which relied on indirect reconciliation via a triggered engine run). Any mode change invalidates the assumptions under which items were queued — the engine will re-evaluate and re-queue as appropriate under the new mode on its next run.

## Related

- Bug fix: `fix/approval-dry-run-wrong-mode` branch (immediate fix for the critical path)
- Service layer audit: `docs/plans/07-audits/20260307T0302Z-service-layer-audit-remediation.md`
