# Refactor: Eliminate `DefaultDiskGroupMode` from Runtime Decision Paths

**Status:** Planned
**Created:** 2026-05-28
**Scope:** Backend (services, poller, routes), Frontend (composables, dashboard)
**Priority:** Medium — no user-facing bug remaining after the fix in `fix/approval-dry-run-wrong-mode`, but the architectural confusion remains

## Background

In v2.x, Capacitarr had a single global execution mode (`ExecutionMode`) that governed all behavior. In v3.0, execution mode moved to per-disk-group (`DiskGroup.Mode`), but the global field was renamed to `DefaultDiskGroupMode` and kept on `PreferenceSet`.

This vestigial field has caused:
- **A critical bug** (fixed in `fix/approval-dry-run-wrong-mode`): `ExecuteApproval` checked `prefs.DefaultDiskGroupMode` instead of the actual disk group's mode, causing approved items to dry-delete and bounce back to the approval queue.
- **UX confusion**: The dashboard mode toggle writes to `DefaultDiskGroupMode`, but actual behavior is governed by per-disk-group modes. Users with a single disk group don't notice the disconnect until they hit edge cases.
- **Dead documented purpose**: The model comment says "used only as the default for newly auto-discovered disk groups", but `DiskGroupService.Upsert()` actually uses the GORM column default (`"dry-run"`) and never reads this preference.

## Goals

1. Make `DefaultDiskGroupMode` truly serve only its documented purpose: the default mode applied to newly auto-discovered disk groups
2. Remove all runtime decision paths that read `DefaultDiskGroupMode` when they should use `DiskGroup.Mode`
3. Fix the dashboard mode toggle to propagate to all disk groups (not just save a preference)
4. Implement the "default for new groups" behavior that the model claims but doesn't implement

## Current Usages (Audit)

| Location | Usage | Action |
|----------|-------|--------|
| `services/approval.go:551` | Fallback when `DiskGroupID` is nil | Keep as fallback (already fixed) |
| `services/deletion.go:810` | Fallback in `resolveCurrentMode()` when no disk group | Keep as fallback |
| `routes/deletion.go:135` | `ManualDelete` uses it for mode determination | Phase 2: resolve from integration's disk group |
| `poller/poller.go:242,252,257,494` | Engine run stats + events + logging | Phase 3: derive from per-group modes |
| `services/metrics.go:379` | Worker stats API response | Phase 3: derive or deprecate |
| `services/diskgroup.go:129` | New group creation — does NOT read the preference | Phase 1: fix to read preference |
| Frontend `useEngineControl.ts:60` | Dashboard mode display | Phase 3: derive from disk groups |
| Frontend `useEngineControl.ts:227` | `setMode()` writes to preference | Phase 2: propagate to all groups |
| Frontend `index.vue:504` | Fallback when no disk groups exist | Keep as fallback |
| `services/backup.go` | Export/import | Keep (compatibility) |
| `routes/preferences.go` | API validation on update | Keep (still a valid preference) |
| `services/settings.go` | Mode change event + queue clear | Phase 2: update to also propagate |

## Implementation Plan

### Phase 1: Fix "Default for New Groups" (Backend Only)

**Goal:** Make `DiskGroupService.Upsert()` actually apply `DefaultDiskGroupMode` when creating a new disk group.

- [ ] 1.1. Add a `SettingsReader` dependency to `DiskGroupService` (or pass preferences in from the poller)
- [ ] 1.2. In `DiskGroupService.Upsert()`, when creating a new group (`result.Error != nil` path), read `prefs.DefaultDiskGroupMode` and set `group.Mode` to that value instead of relying on the GORM column default
- [ ] 1.3. Update `DiskGroupService` tests to verify new groups inherit the preference
- [ ] 1.4. Run `make ci`

### Phase 2: Mode Toggle Propagation (Backend + Frontend)

**Goal:** When the user changes mode via the dashboard toggle, propagate to all active disk groups (not just the global preference).

- [ ] 2.1. Add a new service method `DiskGroupService.SetModeAll(mode string) (int64, error)` that updates `Mode` on all non-stale disk groups
- [ ] 2.2. In `SettingsService.UpdatePreferences()` (the PUT handler path), when `DefaultDiskGroupMode` changes, also call `DiskGroupService.SetModeAll(newMode)`. This ensures the mode toggle in the UI actually changes behavior for existing groups.
- [ ] 2.3. In `SettingsService.PatchPreferences()` (the PATCH handler path), same as above when `DefaultDiskGroupMode` is in the patch payload
- [ ] 2.4. Update the existing mode-change event publishing to include the count of groups updated
- [ ] 2.5. Fix `ManualDelete` route: resolve the disk group mode from the integration's linked disk group (`DiskGroupIntegration` junction table), falling back to `DefaultDiskGroupMode` only when no disk group link exists. Add a `DiskGroupModeReader` parameter to `ManualDeleteDeps` or resolve inside the service.
- [ ] 2.6. Update frontend `setMode()` — no code change needed if backend propagation works (the PUT to preferences already triggers the backend propagation)
- [ ] 2.7. Add tests: changing `DefaultDiskGroupMode` via PUT also updates all disk groups
- [ ] 2.8. Add test: ManualDelete resolves per-disk-group mode from integration link
- [ ] 2.9. Run `make ci`

### Phase 3: Cosmetic Cleanup (Poller + Frontend)

**Goal:** Make logging, stats, and the dashboard reflect actual per-group modes instead of a single global label.

- [ ] 3.1. In the poller, change `EngineStartEvent.ExecutionMode` to report the "most aggressive" mode across all evaluated groups (auto > approval > sunset > dry-run), or a structured map of group→mode. Update the event type if needed.
- [ ] 3.2. In `EngineRunStats`, store a JSON field `DiskGroupModes` (map of groupID→mode) alongside or replacing the flat `ExecutionMode` string. Keep the flat field for backward compatibility with the dashboard sparkline, but populate it as the derived "most aggressive" mode.
- [ ] 3.3. In the worker stats API (`services/metrics.go`), include both the global `defaultDiskGroupMode` (for the preferences page) and a derived `effectiveMode` (from actual disk groups) so the frontend can use the correct one.
- [ ] 3.4. Update poller logging to show per-group modes in the "Processing disk groups" line
- [ ] 3.5. Run `make ci`

### Phase 4: Documentation + Migration Note

- [ ] 4.1. Add a note in `CHANGELOG.md` explaining the behavioral change: "Mode toggle now applies to all disk groups"
- [ ] 4.2. Update the settings page tooltip/help text in the frontend i18n files to clarify what "Default Disk Group Mode" means
- [ ] 4.3. Add a brief note in `docs/development.md` (or equivalent) about the architecture: mode is per-disk-group, global preference is only a default template

## Non-Goals

- **Removing `DefaultDiskGroupMode` from the database schema** — it remains as a valid preference (default template + backup compatibility)
- **Breaking the preferences API contract** — the field continues to be readable/writable via `/api/v1/preferences`
- **Changing per-disk-group mode setting** — the disk group settings page already allows setting mode per-group; this plan only fixes the dashboard shortcut and runtime decision paths

## Risks

- Phase 2 changes behavior: users who have different per-group modes intentionally (e.g., one group in auto, another in dry-run) would have those overwritten if they use the dashboard toggle. Consider adding a confirmation dialog when groups have heterogeneous modes.
- The `ManualDelete` fix in Phase 2.5 requires resolving integration→disk group mappings, which adds a DB query. For the common case (1-2 integrations), this is negligible.

## Related

- Bug fix: `fix/approval-dry-run-wrong-mode` branch (immediate fix for the critical path)
- Service layer audit: `docs/plans/07-audits/20260307T0302Z-service-layer-audit-remediation.md`
