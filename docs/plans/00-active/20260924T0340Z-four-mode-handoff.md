# Four-Mode Implementation Handoff

**Status:** A+B+C+D+E+F+G shipped on this PR; next is H
**Priority:** High
**Spec:** [`20260924T0335Z-four-mode-spec.md`](./20260924T0335Z-four-mode-spec.md)
**PR / branch:** https://github.com/Ghent/capacitarr/pull/66 — `feature/four-mode`

This is the start-here doc for a new session. Do not redesign the four modes. Implement the spec, slice by slice, **on this same branch**. Do not open a new PR.

---

## Read first (in this order)

1. This file.
2. The spec — especially §2 (shared machine), §6–§7 (matrix + transitions), §11 (vs today), §13 (slices), §15 (flows).
3. Shared pipeline: `backend/internal/orchestrator/orchestrator.go` (`EvaluateDiskGroup` → `scoreCandidates` → `filterCandidates` → `expandCollections` → `dispatchByMode`, including the sunset arm and escalate step 3).
4. Mode write path: `backend/internal/services/diskgroup.go` `UpdateThresholds` (writes mode → Exit sunset if leaving → clear deletion queue → `TriggerRun`).
5. Sunset exit: `backend/internal/services/sunset_wiring.go` `SunsetGroupExiter` → `CancelAllForDiskGroup`.

Conversation that produced the spec (do not relitigate unless Ghent overrides):

- Four pills are **presets** of auth / timing / comms, not four engines.
- Dry-run / approval / auto are structurally fine. Sunset is the unfinished product (private scorer, missing escalation step 3). Exit now exists (slice A). Step 3 now exists (slice G).
- `holdFor=30d` already exists (`prefs.SunsetDays` → `deletion_date`). The spec did not add a second timer.
- Modes should be modular as **bindings on one machine**, not as a plugin `Mode` interface.

---

## Your job this session

**Stay on `feature/four-mode`.** Push and update PR #66. Do not branch off `main`. Do not open a second PR.

| Slice | What | Status |
|---|---|---|
| **A** | Leaving sunset cancels that group’s sunset holds and restores labels/posters | **Done** on this PR. |
| **B** | Copy matches the product | **Done** on this PR. |
| **C** | Honest executor: `QueueFromSunset` IntegrationID; kill-switch unclaim; `SignalBatchSize` from sunset cycle | **Done** on this PR. |
| **D** | Fold sunset into score → filter → expand → `dispatchByMode`. Same-candidate test vs dry-run. | **Done** on this PR. |
| **E** | `onDiskGroupModeChange` for all exits in §7, including approval engine-queue dismiss + mode-changed event. | **Done** on this PR. |
| **F** | Unique identity; write `expired`; reconcile preserves `user_initiated`; snooze on escalate; manual delete → sunset hold. | **Done** on this PR. |
| **G** | Escalation step 3: live-admit unheld candidates when steps 1–2 cannot meet target. | **Done** on this PR. |

---

## Locked decisions (do not reopen)

If a later slice needs a different rule, change the spec in that same commit. Until then:

1. Manual delete on a sunset group → sunset hold, not live (slice F, not A).
2. Escalation step 3 is required (slice G, shipped). Still this branch.
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

## Slice C — shipped (do not redo)

- `QueueFromSunset` sets `MediaItem.IntegrationID`.
- `processExpiredItem` no longer strips labels/posters before handoff.
- `executeDryRun` calls `SunsetQueueCleaner.UnclaimExpired` so kill switch / simulate does not consume the hold.
- `evaluateSunsetMode` returns the escalate release count so `SignalBatchSize` is not 0 on a sunset-only escalate cycle.

---

## Slice D — shipped (do not redo)

- `EvaluateDiskGroup` no longer early-returns to `evaluateSunsetMode`. Sunset uses `evaluateAt` / hold budget bindings, then `scoreCandidates` → `filterCandidates` → `expandCollections` → `dispatchByMode` sunset arm (`BulkQueueSunset`, collection group set).
- Escalate still runs after Admit when `used >= thresholdPct`.
- Below `sunsetPct`: no new admits, existing holds stay (approval queue is not cleared).
- `TestEvaluateDiskGroup_DryRunAndSunsetAdmitSameTitles` — one fixture library; dry-run and sunset admit the same titles (show/season dedup, snooze, MCU expand).

## Slice E — shipped (do not redo)

- `UpdateThresholds` calls `onDiskGroupModeChange` after the mode write: sunset Exit, clear in-flight deletion jobs, approval Exit, then `mode_changed`.
- Approval Exit returns approved → pending, then dismisses engine-queued pending/rejected. `user_initiated` and active snoozes stay.
- Per-group `DiskGroupModeChangedEvent` (`mode_changed`, Important).

## Slice F — shipped (do not redo)

- Sunset unique identity `(disk_group_id, integration_id, external_id)` + `db.ItemKey` for hold lookup / reconcile.
- Successful handoff writes `status=expired` with `expired_at`. Unclaim returns the row to pending.
- `ReconcileQueue` still skips `user_initiated` (explicit + test).
- Escalate skips snoozed `MediaKey`s (`SunsetDeps.SnoozedKeys`).
- Manual delete on a sunset group creates a user sunset hold; already-held is a no-op.

## Slice G — shipped (do not redo)

- After sunset Admit, `Escalate` (steps 1–2) still frees down to `target`. If remaining action budget (`targetBytes − freed`) is > 0 and the held set is known, `dispatchSunsetEscalateLive` walks the **full** scored set (not the hold-budget prefix) through the same filter/expand.
- Already-held and newly admitted holds are skipped. Snoozed identities are skipped. `ListSunsettedKeys` failure (`skipSunsetAdmit`) skips step 3 (cannot know who is held); steps 1–2 still run.
- Live extras use `QueueFromEngine` with `EnqueuedMode=sunset`. The auto / approval / dry-run dispatch arms are unchanged. Mode-change safety still cancels these jobs if the group leaves sunset.
- Step 3 counts toward `SignalBatchSize` and publishes `SunsetEscalated` when it queues extras.

---

## After G (do not start unless asked)

| Slice | Entry points |
|---|---|
| **H** | Posters on create. Spec §13. |
| **I** | Rescore through the engine. Spec §13. |
| **J** | Optional `DiskGroupPolicy`. Spec §13. |

Do not combine H–J with each other unless asked.

---

## Repo / process

- **One branch:** `feature/four-mode` (`feature/` per CONTRIBUTING.md). Fetch it, checkout, continue. Do not create a second branch off `main`.
- Conventional commits (`fix:`, `docs:`).
- Do not use `gh` / forge CLIs to open a PR — this work already has PR #66. Use the session’s PR tool to **update** #66.
- Plans live in `docs/plans/00-active/`. When a slice ships, tick it in spec §13 on this same branch.

### Tests to run

```text
go test ./internal/services/ ./internal/orchestrator/ ./internal/poller/ -count=1 -timeout 120s
```

from `backend/`.

---

## Current-code map (after A+B+C+D+E+F+G)

| Concern | Where it lives today |
|---|---|
| Sunset admit | `orchestrator.go` `dispatchByMode` sunset arm; escalate after Admit |
| Escalate step 3 | `orchestrator.go` `dispatchSunsetEscalateLive` — unheld extras as live `QueueFromEngine` (`EnqueuedMode` sunset) |
| Shared pipeline | `scoreCandidates`, `filterCandidates`, `expandCollections`, `dispatchByMode` |
| Sunset hold create | `BulkQueueSunset` / `QueueUserHold`; `deletion_date = now + prefs.SunsetDays` |
| Sunset expire / escalate | `ProcessExpired`; `Escalate` skips snooze; writes `expired`; step 3 live extras in orchestrator |
| Handoff to delete | `QueueFromSunset` sets `IntegrationID`; simulate unclaims |
| Mode write + exits | `onDiskGroupModeChange` → sunset Exit / approval Exit / queue clear / `mode_changed` |
| Approval reconcile | `ReconcileQueue` uses `ItemKey`; keeps `user_initiated` |
| Manual delete | Approval → approval hold; sunset → sunset hold; else live/dry-run |
| Kill switch | `deletion_worker.go` process-time `DeletionsEnabled`; sunset simulate unclaims |
| Sunset tooltip | `frontend/app/locales/en.json` `mode.sunsetTooltip` (countdown + escalate) |
| Help text | `help.executionModes.sunsetDesc` + safety-guard names sunset |

---

## What success looks like for the next session

- Slice H on PR #66 only if asked. Same branch. Do not combine H with I–J.
