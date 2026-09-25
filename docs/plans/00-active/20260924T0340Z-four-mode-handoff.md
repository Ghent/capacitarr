# Four-Mode Implementation Handoff

**Status:** A–J shipped on this PR
**Priority:** High
**Spec:** [`20260924T0335Z-four-mode-spec.md`](./20260924T0335Z-four-mode-spec.md)
**PR / branch:** https://github.com/Ghent/capacitarr/pull/66 — `feature/four-mode`

This is the start-here doc for a new session. Do not redesign the four modes. The spec slices on this branch are complete. Do not open a new PR unless Ghent asks for follow-up work.

---

## Read first (in this order)

1. This file.
2. The spec — especially §2 (shared machine), §6–§7 (matrix + transitions), §11 (vs today), §13 (slices), §15 (flows).
3. Shared pipeline: `backend/internal/orchestrator/orchestrator.go` (`EvaluateDiskGroup` → `scoreCandidates` → `filterCandidates` → `expandCollections` → `dispatchByMode`, including the sunset arm and escalate step 3).
4. Mode write path: `backend/internal/services/diskgroup.go` `UpdateThresholds` (writes mode → Exit sunset if leaving → clear deletion queue → `TriggerRun`).
5. Sunset exit: `backend/internal/services/sunset_wiring.go` `SunsetGroupExiter` → `CancelAllForDiskGroup`.

Conversation that produced the spec (do not relitigate unless Ghent overrides):

- Four pills are **presets** of auth / timing / comms, not four engines.
- Dry-run / approval / auto are structurally fine. Sunset is the unfinished product (private scorer, missing escalation step 3). Exit, step 3, posters-on-create, engine rescore, and `DiskGroupPolicy` now exist.
- `holdFor=30d` already exists (`prefs.SunsetDays` → `deletion_date`). The spec did not add a second timer.
- Modes should be modular as **bindings on one machine**, not as a plugin `Mode` interface.

---

## Your job this session

**Stay on `feature/four-mode`.** Push and update PR #66. Do not branch off `main`. Do not open a second PR.

| Slice | What | Status |
|---|---|---|
| **A** | Leaving sunset cancels that group’s sunset holds and restores labels/posters | **Done** |
| **B** | Copy matches the product | **Done** |
| **C** | Honest executor: `QueueFromSunset` IntegrationID; kill-switch unclaim; `SignalBatchSize` from sunset cycle | **Done** |
| **D** | Fold sunset into score → filter → expand → `dispatchByMode`. Same-candidate test vs dry-run. | **Done** |
| **E** | `onDiskGroupModeChange` for all exits in §7, including approval engine-queue dismiss + mode-changed event. | **Done** |
| **F** | Unique identity; write `expired`; reconcile preserves `user_initiated`; snooze on escalate; manual delete → sunset hold. | **Done** |
| **G** | Escalation step 3: live-admit unheld candidates when steps 1–2 cannot meet target. | **Done** |
| **H** | Posters on sunset hold create (label + overlay). Apply-fail is retryable. | **Done** |
| **I** | Rescore pending holds through the engine, same weights/rules as Admit. | **Done** |
| **J** | `DiskGroupPolicy` binds §2–§3. Refactor only. | **Done** |

---

## Locked decisions (do not reopen)

If a later change needs a different rule, change the spec in that same commit. Until then:

1. Manual delete on a sunset group → sunset hold, not live (slice F, not A).
2. Escalation step 3 is required (slice G, shipped).
3. Exit never converts holds into live deletes. New preset re-admits under its own budget.
4. `ReconcileQueue` must not dismiss `user_initiated` (slice F).
5. Snooze applies to escalate (slice F).
6. Two tables stay. No fifth pill. No `Mode` plugin API.
7. Identity becomes `(disk_group_id, integration_id, external_id)` in slice F, not A.
8. Do not rewrite auto / approval / dry-run dispatch arms.

Open only if Ghent contradicts the spec. Otherwise implement.

---

## Slice H — shipped (do not redo)

`QueueSunset` / `BulkQueueSunset` call `applyCreateComms`: sunset label, then `PosterOverlay.UpdateOverlay` when style is not `off`. Apply-fail leaves the hold; `label_applied` / `poster_overlay_active` stay false for daily retry. Daily cron `UpdateAll` still refreshes countdown text.

## Slice I — shipped (do not redo)

`RescoreAndSave` scores `Preview.GetCachedItems()` through `engine.Evaluator` with the same weights/rules/eval context as Admit. Lookup is `db.ItemKey`. Current score ≤ 50% of score-at-admit → `saved`. Empty library skips the cycle. Unknown identity is skipped (not saved). Cron loads enabled rules and builds `EvaluationContext` from enabled integrations.

## Slice J — shipped (do not redo)

`orchestrator.DiskGroupPolicy` / `PolicyFor` is the §2–§3 binding (`evaluateAt` / `target` / `escalateAt`, hold vs action budget, duration-hold escalate). `EvaluateDiskGroup` uses it. No product change; auto / approval / dry-run dispatch arms unchanged.

---

## Repo / process

- **One branch:** `feature/four-mode` (`feature/` per CONTRIBUTING.md).
- Conventional commits (`fix:`, `docs:`).
- Do not use `gh` / forge CLIs to open a PR — this work already has PR #66. Use the session’s PR tool to **update** #66.
- Plans live in `docs/plans/00-active/`.

### Tests to run

```text
go test ./internal/services/ ./internal/orchestrator/ ./internal/poller/ ./internal/jobs/ -count=1 -timeout 120s
```

from `backend/`.

---

## Current-code map (after A–J)

| Concern | Where it lives today |
|---|---|
| Shared-machine bindings | `orchestrator.PolicyFor` / `DiskGroupPolicy` |
| Sunset admit | `orchestrator.go` `dispatchByMode` sunset arm; escalate after Admit |
| Escalate step 3 | `dispatchSunsetEscalateLive` — unheld extras as live `QueueFromEngine` (`EnqueuedMode` sunset) |
| Sunset hold create | `BulkQueueSunset` / `QueueUserHold`; label + poster on create; `deletion_date = now + prefs.SunsetDays` |
| Sunset expire / escalate | `ProcessExpired`; `Escalate` skips snooze; writes `expired`; step 3 live extras |
| Rescore | `RescoreAndSave` via engine on preview library |
| Handoff to delete | `QueueFromSunset` sets `IntegrationID`; simulate unclaims |
| Mode write + exits | `onDiskGroupModeChange` → sunset Exit / approval Exit / queue clear / `mode_changed` |
| Approval reconcile | `ReconcileQueue` uses `ItemKey`; keeps `user_initiated` |
| Manual delete | Approval → approval hold; sunset → sunset hold; else live/dry-run |
| Kill switch | `deletion_worker.go` process-time `DeletionsEnabled`; sunset simulate unclaims |
| Sunset tooltip | `frontend/app/locales/en.json` `mode.sunsetTooltip` (countdown + escalate) |

---

## What success looks like for the next session

- Spec slices A–J are on PR #66. Follow-up only if Ghent asks.
