# Development Notes

## Execution Mode Architecture

Execution mode (dry-run, approval, auto, sunset) is configured **per-disk-group** on the Rules page. Each disk group has its own `Mode` field that governs how the engine acts on candidates within that group.

The `DefaultDiskGroupMode` field on `PreferenceSet` serves only as a **template for newly auto-discovered disk groups**. When the poller discovers a new mount path and creates a `DiskGroup` record, the group inherits the current `DefaultDiskGroupMode` value. After creation, the group's mode is independent of the global preference.

### Key invariants

- Route handlers and orchestrators must never use `prefs.DefaultDiskGroupMode` for runtime decisions about existing groups. Always resolve the mode from the disk group itself.
- The only valid uses of `DefaultDiskGroupMode` are:
  1. `DiskGroupService.Upsert()` — setting mode on newly created groups
  2. Fallback paths where no disk group association exists (e.g., `resolveCurrentMode()` in deletion worker for legacy queue items without a `DiskGroupID`)
  3. Backup/restore compatibility
- The worker stats API returns `diskGroupModes` (a JSON map of group ID → mode) rather than a single global mode string.
- SSE events (`engine_start`, `engine_complete`) carry `diskGroupModes` for real-time UI updates.
