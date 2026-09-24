# Four-Mode Implementation Handoff

**Status:** Ready to implement
**Priority:** High
**Spec:** [`20260924T0335Z-four-mode-spec.md`](./20260924T0335Z-four-mode-spec.md)
**Spec PR:** https://github.com/Ghent/capacitarr/pull/64 (draft, docs only)

This is the start-here doc for a new session. Do not redesign the four modes. Implement the spec, slice by slice.

---

## Read first (in this order)

1. This file.
2. The spec — especially §2 (shared machine), §6–§7 (matrix + transitions), §11 (vs today), §13 (slices), §15 (flows).
3. Current engine fork: `backend/internal/orchestrator/orchestrator.go` (`EvaluateDiskGroup` early-return at sunset, `evaluateSunsetMode`, `dispatchByMode`).
4. Mode write path: `backend/internal/services/diskgroup.go` `UpdateThresholds` (clears deletion queue + `sunset_pct`; does **not** cancel sunset rows).
5. Sunset cancel that already does the right compensate: `backend/internal/services/sunset.go` `CancelAllForDiskGroup`.

Conversation that produced the spec (do not relitigate unless Ghent overrides):

- Four pills are **presets** of auth / timing / comms, not four engines.
- Dry-run / approval / auto are structurally fine. Sunset is the unfinished product (private scorer, no Exit, missing escalation step 3).
- `holdFor=30d` already exists (`prefs.SunsetDays` → `deletion_date`). The spec did not add a second timer.
- Modes should be modular as **bindings on one machine**, not as a plugin `Mode` interface.

---

## Your job this session

**Implement slices A and B only.** One implementation PR, new branch off latest `main`. Do not pile code onto PR #64 unless Ghent asks.

| Slice | What | Done when |
|---|---|---|
| **A** | Leaving sunset cancels that group’s sunset holds and restores labels/posters | Test: group with queued items + `label_applied` / `poster_overlay_active` → after `UpdateThresholds(..., dry-run, ...)` queue is empty and compensate ran. Sibling group’s rows untouched. Auto↔dry-run still only clears the deletion queue. |
| **B** | Copy matches the product | `mode.sunsetTooltip` and `diskGroupMode.ts` describe countdown + escalation, not “wind down until empty.” Safety-guard help names sunset. |

If A+B land cleanly and there is time, **C** is next (not D). Stop before the sunset fold unless Ghent says to continue.

---

## Locked decisions (do not reopen)

If a later PR needs a different rule, change the spec in that same PR. Until then:

1. Manual delete on a sunset group → sunset hold, not live (slice F, not A).
2. Escalation step 3 is required (slice G, own PR).
3. Exit never converts holds into live deletes. New preset re-admits under its own budget.
4. `ReconcileQueue` must not dismiss `user_initiated` (slice F).
5. Snooze applies to escalate (slice F).
6. Two tables stay. No fifth pill. No `Mode` plugin API.
7. Identity becomes `(disk_group_id, integration_id, external_id)` in slice F, not A.
8. Do not rewrite auto / approval / dry-run dispatch arms.

Open only if Ghent contradicts the spec. Otherwise implement.

---

## Slice A — how to implement

`CancelAllForDiskGroup` already restores posters, removes labels, deletes rows. Production never calls it. `UpdateThresholds` is the hook.

**Do not** call `CancelAll` (all groups). Per-group only.

### Wiring

`DiskGroupService` has `deletionClearer` but no sunset exiter. Add a narrow interface (same style as `DeletionQueueGroupClearer`):

```go
type SunsetGroupExiter interface {
    Exit(diskGroupID uint) (int, error)
}
```

Implement it as a small adapter in the registry (or on `SunsetService`) that builds `SunsetDeps` the same way `routes/sunset.go` cancel does:

- `reg.Integration.BuildIntegrationRegistry()` (log and continue if this fails — labels/posters may skip; **rows must still delete**)
- `Settings`, `PosterOverlay`, `Mapping`, `Deletion`, `Engine`

On `mode != "" && mode != oldMode && oldMode == db.ModeSunset`, call `Exit(group.ID)` **after** the mode column is written (so a crash mid-exit cannot leave the group in sunset with an empty queue… actually: if you write mode first then fail exit, labels linger on a dry-run group — still better than deleting files. If you exit first then fail the write, you cancelled holds while still in sunset — next cycle re-admits. **Prefer write mode, then Exit.** Spec §7: Exit before the new mode is *observable to the next engine run*. The triggered `TriggerRun()` at the end of `UpdateThresholds` is that run. So: write mode → Exit sunset → clear deletion queue → TriggerRun.

Keep the existing deletion-queue clear for every mode change.

### Tests to add

In `backend/internal/services/diskgroup_test.go` (next to `TestDiskGroupService_UpdateThresholds_ClearsDeletionQueueOnModeChange`):

1. Sunset → dry-run calls `Exit` once for that group ID; non-sunset → * does not.
2. With real `SunsetService` + two groups: items on group 1 gone, group 2 remain; `sunset_cancelled` / poster-restore events if you assert the bus.
3. Existing mode-change deletion-queue tests still pass.

Mock `SunsetGroupExiter` for the “was it called” tests. Use the real service for the compensate test (`TestCancelAllForDiskGroup` in `sunset_test.go` is the model).

### Files

| File | Why |
|---|---|
| `backend/internal/services/diskgroup.go` | `UpdateThresholds` + new setter + `Wired()` if you add a required dep. Making the exiter **optional** (nil-safe) is fine so existing tests that do not set it still compile; production `NewRegistry` must set it. |
| `backend/internal/services/registry.go` | Wire the adapter. |
| `backend/internal/services/diskgroup_test.go` | New tests. |
| `backend/internal/services/sunset.go` | Only if you add a `Exit` wrapper; do not change expire/escalate. |

### Anti-goals for A

- Do not fold `evaluateSunsetMode` into `dispatchByMode`.
- Do not add unique indexes.
- Do not change `QueueFromSunset` or `SignalBatchSize`.
- Do not publish a new mode-changed event yet (slice E).

---

## Slice B — how to implement

Copy only. No backend.

| File | Change |
|---|---|
| `frontend/app/locales/en.json` `mode.sunsetTooltip` | Countdown hold + household labels/posters; escalate at critical. Not empty-the-disk. Align with `help.executionModes.sunsetDesc` (already correct). |
| `frontend/app/locales/en.json` `help.executionModes.safetyGuardDesc` | Name sunset: brake simulates auto, approval, **and** sunset releases. |
| `frontend/app/utils/diskGroupMode.ts` header comment | Sunset is countdown + escalate, not “winds down, reduces target percentage.” |

Only `en.json` exists under `frontend/app/locales/`. Help page sunset body is already right — do not rewrite it.

---

## After A+B (do not start unless asked)

| Slice | Entry points |
|---|---|
| **C** | `deletion/deletion_intake.go` `QueueFromSunset` (set `IntegrationID`); `sunset.go` `processExpiredItem` (unclaim on simulate / kill switch — today it strips comms then hands off); `orchestrator.go` `evaluateSunsetMode` returns 0 → `poller.go` `SignalBatchSize` is wrong on escalate. |
| **D** | Delete the sunset early-return. Sunset uses `scoreCandidates` → `filterCandidates` → `expandCollections` → new `dispatchByMode` arm. **Prove with a test that dry-run and sunset admit the same titles** on one fixture library. This is the first PR that can change who gets queued. |
| **E** | `onDiskGroupModeChange` for approval Exit + per-group mode-changed event. |
| **F–J** | Spec §13. Do not combine with D. |

---

## Repo / process

- Branch for implementation: `cursor/<slice-name>-fe9a` from latest `main` (`git fetch origin main` first).
- Conventional commits (`fix:`, `docs:`).
- Do not use `gh` / forge CLIs to open the PR — use the session’s PR tool.
- After tests/fixes, push and update that implementation PR. Leave #64 as the spec PR unless you are told to merge them.
- Plans live in `docs/plans/00-active/`. When a slice ships, tick it in the spec §13 in the same PR if the spec is already on `main`; otherwise do not edit the spec “to keep it current” on a side branch.

### Tests to run for A+B

```text
go test ./internal/services/ -count=1 -timeout 120s
```

from `backend/`. Plus any frontend typecheck/lint the repo already uses for `en.json` / `diskGroupMode.ts`.

---

## Current-code map (as of the spec branch)

| Concern | Where it lives today |
|---|---|
| Sunset private evaluator | `orchestrator.go` `evaluateSunsetMode` (~684) |
| Shared pipeline | `scoreCandidates`, `filterCandidates`, `expandCollections`, `dispatchByMode` |
| Sunset hold create | `BulkQueueSunset`; `deletion_date = now + prefs.SunsetDays` |
| Sunset expire / escalate | `ProcessExpired` (cron `jobs/cron.go`), `Escalate` (no step 3, no snooze) |
| Handoff to delete | `QueueFromSunset` — missing `MediaItem.IntegrationID` |
| Mode write | `DiskGroupService.UpdateThresholds` |
| Sunset exit (unused in prod) | `CancelAllForDiskGroup` |
| Approval reconcile | `approval/approval.go` `ReconcileQueue` — can dismiss `user_initiated` |
| Manual delete | `deletion_intake.go` `QueueManual` — only approval is special-cased |
| Kill switch | `deletion_worker.go` process-time `DeletionsEnabled`; sunset claims `expired_at` first |
| Wrong tooltip | `frontend/app/locales/en.json` `mode.sunsetTooltip` |
| Correct help text | `help.executionModes.sunsetDesc` |

---

## What success looks like for this session

- Implementation PR with A+B, tests green.
- Leaving sunset no longer leaves labels/posters/cron work on a group that is no longer sunset.
- The rules-page tooltip no longer describes a drain-to-empty product.
- Spec unchanged except a pointer to this handoff if you must touch it.
- Next session can start C or D from the spec §13 table without rereading the review.
