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

## Frontend composition

1. Pages orchestrate. They do not contain icon/color/label switches or ECharts option builders.
2. Shared domain maps live in `utils/` (see `diskGroupMode.ts`). New mode or event-type maps go there, not into a page.
3. Feature cards are presentational. They consume composables; they do not register app-lifetime SSE.
4. Fetch errors go through the error policy below. `console.warn` alone is not an error strategy.

## Frontend error policy

| Category | Treatment | Examples |
|---|---|---|
| **Initial page load** | Inline error + retry. Keep last-good data if any. Never show the empty-state component. | Dashboard disk groups, preview, rules list, approval queue first fetch |
| **User-initiated action** | Error toast (`$t(...)`). | Run now, approve, save weights, manual delete |
| **Background refresh** | Stay silent unless consecutive failures. Do not fight `ConnectionBanner`. | SSE-triggered refetches, integration refresh |
| **Parse / display** | Inline fallback + `console.warn`. | Score-detail JSON parse |
| **Background polling** | Silent. Comment the site. | Connection health poll, automatic version check |

Do not invent a “3 failures then toast” counter without a shared helper. Prefer `useFetchStatus` with `ok | loading | error` and last-good data.
