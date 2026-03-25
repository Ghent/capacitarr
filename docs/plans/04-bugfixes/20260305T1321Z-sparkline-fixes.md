# Sparkline Fixes

**Date:** 2026-03-05
**Branch:** `fix/sparkline-fixes`
**Status:** ✅ Complete
**Size:** M (51–200 lines changed)

## Problems

### 1. Sparklines look flat/line-like instead of curved with fill

The recent switch from `audit_logs`-based time-bucketed data to raw per-engine-run `engine_run_stats` data (in the `ux-enhancements-batch` branch) introduced a visual regression. With the engine running every few minutes, there are hundreds of tightly-spaced data points over a 7-day range. ApexCharts' `curve: 'smooth'` interpolation between densely-packed points produces near-flat line segments, and the gradient fill area becomes imperceptibly thin.

**Affected sparklines:** Main (flagged/deleted), Space Freed.
**Not affected:** Duration (per-run granularity is valuable for spotting outlier slow runs).

### 2. "Space Freed" shows wildly inflated numbers (e.g., 281 TB)

`freedBytes` is accumulated in `evaluate.go:238-240` for every flagged item regardless of execution mode. In dry-run or approval mode, nothing is actually deleted, but every engine poll cycle records the full size of all flagged items as "freed bytes." Over hundreds of poll cycles, this produces absurdly large totals.

**Root cause:** `freedBytes` should only be counted when `DeleteMediaItem()` succeeds in the deletion worker (`delete.go`), not during evaluation/flagging.

### 3. Mini sparkline tooltips show raw numbers with no labels

The `miniSparklineOptions()` function lacks a `tooltip.y.formatter`. Hovering over the "Space Freed" sparkline shows a raw byte integer (e.g., `309237645312`). The duration sparkline shows raw milliseconds with no "ms" suffix.

## Plan

| # | Task | Files | Status |
|---|------|-------|--------|
| 1 | Remove `freedBytes` accumulation from `evaluate.go` (line 240) | `backend/internal/poller/evaluate.go` | ✅ |
| 2 | Add `freed_bytes` increment in deletion worker after successful `DeleteMediaItem()` | `backend/internal/poller/delete.go` | ✅ |
| 3 | Remove `lastRunFreedBytes` atomic (no longer needed in-memory) | `backend/internal/poller/stats.go`, `poller.go` | ✅ |
| 4 | Update stats tests to reflect new freed_bytes behavior | `backend/internal/poller/stats_test.go` | ✅ (existing tests valid — they seed DB directly) |
| 5 | Add `y.formatter` to mini sparkline tooltips (formatBytes for freed, ms for duration) | `frontend/app/pages/index.vue` | ✅ |
| 6 | Add hourly bucketing utility for sparkline data | `frontend/app/pages/index.vue` | ✅ |
| 7 | Apply bucketing to main sparkline (flagged + deleted) series | `frontend/app/pages/index.vue` | ✅ |
| 8 | Apply bucketing to freed sparkline series | `frontend/app/pages/index.vue` | ✅ |
| 9 | Keep duration sparkline using raw per-run data (no change) | — | ✅ |
| 10 | Run `make ci` to verify | — | ✅ |

## Design Notes

### Hourly bucketing (frontend-side)

The `/engine/history` endpoint continues to return raw per-run data. The frontend buckets data into hourly groups using `Math.floor(timestamp / 3600000)` and SUMs values within each bucket. This is applied to:

- Main sparkline: `flagged` and `deleted` series
- Space freed sparkline: `freedBytes` series

The duration sparkline keeps raw data — individual per-run values are valuable for spotting outlier slow runs.

### freed_bytes moved to deletion worker

The `freed_bytes` field on `EngineRunStats` is now updated in the `deletionWorker` after a successful `DeleteMediaItem()` call, using `gorm.Expr("freed_bytes + ?", job.item.SizeBytes)` — the same pattern already used for the `deleted` counter. This ensures:

- Dry-run mode → freed_bytes = 0
- Approval mode → freed_bytes only increments after user approves and deletion succeeds
- Auto mode → freed_bytes only increments after successful deletion
- Failed deletions → not counted
