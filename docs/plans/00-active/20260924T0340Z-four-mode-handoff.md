# Four-Mode Implementation Handoff

**Status:** A+B shipped on this PR; next is C
**Priority:** High
**Spec:** [`20260924T0335Z-four-mode-spec.md`](./20260924T0335Z-four-mode-spec.md)
**PR / branch:** https://github.com/Ghent/capacitarr/pull/64 — `cursor/four-mode-spec-fe9a`

This is the start-here doc for a new session. Do not redesign the four modes. Implement the spec, slice by slice, **on this same branch**. Do not open a new PR.

---

## Read first (in this order)

1. This file.
2. The spec — especially §2 (shared machine), §6–§7 (matrix + transitions), §11 (vs today), §13 (slices), §15 (flows).
3. Current engine fork: `backend/internal/orchestrator/orchestrator.go` (`EvaluateDiskGroup` early-return at sunset, `evaluateSunsetMode`, `dispatchByMode`).
4. Mode write path: `backend/internal/services/diskgroup.go` `UpdateThresholds` (writes mode → Exit sunset if leaving → clear deletion queue → `TriggerRun`).
5. Sunset exit: `backend/internal/services/sunset_wiring.go` `SunsetGroupExiter` → `CancelAllForDiskGroup`.

Conversation that produced the spec (do not relitigate unless Ghent overrides):

- Four pills are **presets** of auth / timing / comms, not four engines.
- Dry-run / approval / auto are structurally fine. Sunset is the unfinished product (private scorer, missing escalation step 3). Exit now exists (slice A).
- `holdFor=30d` already exists (`prefs.SunsetDays` → `deletion_date`). The spec did not add a second timer.
- Modes should be modular as **bindings on one machine**, not as a plugin `Mode` interface.

---

## Your job this session

**Stay on `cursor/four-mode-spec-fe9a`.** Push and update PR #64. Do not branch off `main`. Do not open a second PR.

| Slice | What | Status |
|---|---|---|
| **A** | Leaving sunset cancels that group’s sunset holds and restores labels/posters | **Done** on this PR. |
| **B** | Copy matches the product | **Done** on this PR. |
| **C** | Honest executor: `QueueFromSunset` IntegrationID; kill-switch unclaim; `SignalBatchSize` from sunset cycle | **Next.** |

If C lands cleanly and there is time, **D** is next. Stop before the sunset fold unless Ghent says to continue.

---

## Locked decisions (do not reopen)

If a later slice needs a different rule, change the spec in that same commit. Until then:

1. Manual delete on a sunset group → sunset hold, not live (slice F, not A).
2. Escalation step 3 is required (slice G). Still this branch; review carefully.
3. Exit never converts holds into live deletes. New preset re-admits under its own budget.
4. `ReconcileQueue` must not dismiss `user_initiated` (slice F).
5. Snooze applies to escalate (slice F).
6. Two tables stay. No fifth pill. No `Mode` plugin API.
7. Identity becomes `(disk_group_id, integration_id, external_id)` in slice F, not A.
8. Do not rewrite auto / approval / dry-run dispatch arms.

Open only if Ghent contradicts the spec. Otherwise implement.

---

## Slice A — shipped (do not redo)

`UpdateThresholds` writes the new mode, then `SunsetGroupExiter.Exit` → `CancelAllForDiskGroup`, then clears the deletion queue, then `TriggerRun`. Per-group only. Rows still delete if label/poster restore fails. Auto↔dry-run still only clears the deletion queue.

Tests in `backend/internal/services/diskgroup_test.go`:

- `TestDiskGroupService_UpdateThresholds_ExitsSunsetOnLeave`
- `TestDiskGroupService_UpdateThresholds_ExitSunsetCancelsOnlyThatGroup`

---

## Slice B — shipped (do not redo)

| File | Now says |
|---|---|
| `frontend/app/locales/en.json` `mode.sunsetTooltip` | Countdown + household labels/posters; escalate at critical. |
| `frontend/app/locales/en.json` `help.executionModes.safetyGuardDesc` | Brake simulates auto, approval, **and** sunset releases. |
| `frontend/app/utils/diskGroupMode.ts` header | Sunset is countdown + escalate. |

---

## Slice C — how to implement (next)

| File | Change |
|---|---|
| `backend/internal/services/deletion/deletion_intake.go` `QueueFromSunset` | Set `IntegrationID` on the media item. |
| `backend/internal/services/sunset.go` `processExpiredItem` | Unclaim on simulate / kill switch — today it strips comms then hands off. |
| `backend/internal/orchestrator/orchestrator.go` `evaluateSunsetMode` | Returns 0 today → `poller.go` `SignalBatchSize` is wrong on escalate. |

Do not fold sunset into `dispatchByMode` (that is D). Do not add unique indexes (F). Do not publish a mode-changed event (E).

---

## After C (do not start unless asked)

| Slice | Entry points |
|---|---|
| **D** | Delete the sunset early-return. Sunset uses `scoreCandidates` → `filterCandidates` → `expandCollections` → new `dispatchByMode` arm. **Prove with a test that dry-run and sunset admit the same titles** on one fixture library. This is the first change that can change who gets queued. |
| **E** | `onDiskGroupModeChange` for approval Exit + per-group mode-changed event. |
| **F–J** | Spec §13. Do not combine with D. |

---

## Repo / process

- **One branch:** `cursor/four-mode-spec-fe9a`. Fetch it, checkout, continue. Do not create `cursor/<slice>-*` off `main`.
- Conventional commits (`fix:`, `docs:`).
- Do not use `gh` / forge CLIs to open a PR — this work already has PR #64. Use the session’s PR tool to **update** #64.
- Plans live in `docs/plans/00-active/`. When a slice ships, tick it in spec §13 on this same branch.

### Tests to run for C

```text
go test ./internal/services/ ./internal/orchestrator/ ./internal/poller/ -count=1 -timeout 120s
```

from `backend/`.

---

## Current-code map (after A+B)

| Concern | Where it lives today |
|---|---|
| Sunset private evaluator | `orchestrator.go` `evaluateSunsetMode` (~684) |
| Shared pipeline | `scoreCandidates`, `filterCandidates`, `expandCollections`, `dispatchByMode` |
| Sunset hold create | `BulkQueueSunset`; `deletion_date = now + prefs.SunsetDays` |
| Sunset expire / escalate | `ProcessExpired` (cron `jobs/cron.go`), `Escalate` (no step 3, no snooze) |
| Handoff to delete | `QueueFromSunset` — missing `MediaItem.IntegrationID` |
| Mode write + sunset exit | `DiskGroupService.UpdateThresholds` → `SunsetGroupExiter` |
| Approval reconcile | `approval/approval.go` `ReconcileQueue` — can dismiss `user_initiated` |
| Manual delete | `deletion_intake.go` `QueueManual` — only approval is special-cased |
| Kill switch | `deletion_worker.go` process-time `DeletionsEnabled`; sunset claims `expired_at` first |
| Sunset tooltip | `frontend/app/locales/en.json` `mode.sunsetTooltip` (countdown + escalate) |
| Help text | `help.executionModes.sunsetDesc` + safety-guard names sunset |

---

## What success looks like for the next session

- Slice C on PR #64, tests green. Same branch.
- `QueueFromSunset` carries `IntegrationID`.
- Simulate / kill switch unclaims sunset rows instead of stripping comms and handing off.
- `SignalBatchSize` reflects sunset cycle actions (including escalate).
- Spec §13 ticks C. Next session can start D from that table without rereading the review.
