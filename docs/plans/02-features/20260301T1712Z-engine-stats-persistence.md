# Engine Stats Persistence

**Date:** 2026-03-01
**Status:** ✅ Complete
**Branch:** `feature/ux-refinement`
**Effort:** S-M

## Problem

Worker stats (`lastRunEpoch`, `lastRunEvaluated`, `lastRunFlagged`, `lastRunFreedBytes`, etc.) are stored as in-memory `atomic.Int64` values in `poller.go`. They reset to zero every time the container restarts. Users see "Evaluated: 0 · Flagged: 0" in the engine control popover even when the engine has been running for hours (data is in the audit log from the previous container instance).

## Solution

Two complementary persistence mechanisms:

### 1. EngineRunStats Table (per-run tracking)

New SQLite table that stores one row per engine run:

```sql
-- +goose Up
CREATE TABLE IF NOT EXISTS engine_run_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    evaluated INTEGER NOT NULL DEFAULT 0,
    flagged INTEGER NOT NULL DEFAULT 0,
    freed_bytes INTEGER NOT NULL DEFAULT 0,
    execution_mode TEXT NOT NULL DEFAULT 'dry-run',
    duration_ms INTEGER NOT NULL DEFAULT 0,
    error_message TEXT
);

CREATE INDEX idx_engine_run_stats_run_at ON engine_run_stats(run_at);
```

### 2. Audit Log Aggregation (historical summaries)

Already exists: `GET /api/v1/audit/activity` derives flagged/deleted counts from the audit log. No changes needed — this already works for sparklines and history.

## Implementation Steps

### Step 1: Add GORM Model

**File:** `backend/internal/db/models.go`

```go
type EngineRunStats struct {
    ID            uint      `gorm:"primaryKey" json:"id"`
    RunAt         time.Time `json:"runAt"`
    Evaluated     int       `json:"evaluated"`
    Flagged       int       `json:"flagged"`
    FreedBytes    int64     `json:"freedBytes"`
    ExecutionMode string    `json:"executionMode"`
    DurationMs    int64     `json:"durationMs"`
    ErrorMessage  string    `json:"errorMessage,omitempty"`
}
```

### Step 2: Add Goose Migration

**File:** `backend/internal/db/migrations/00004_add_engine_run_stats.sql`

The SQL above.

### Step 3: Write Stats After Each Poller Run

**File:** `backend/internal/poller/poller.go`

At the end of `poll()` (after evaluation completes), insert a row:

```go
runStats := db.EngineRunStats{
    RunAt:         time.Now(),
    Evaluated:     evaluatedCount,
    Flagged:       flaggedCount,
    FreedBytes:    freedBytes,
    ExecutionMode: currentMode,
    DurationMs:    elapsed.Milliseconds(),
}
db.DB.Create(&runStats)
```

Also update the in-memory atomics so they reflect the current run (for real-time "currently running" status).

### Step 4: Read Stats from DB in GetWorkerMetrics()

**File:** `backend/internal/poller/poller.go` — `GetWorkerMetrics()`

Replace the atomic reads with a DB query for the latest `EngineRunStats` row:

```go
func GetWorkerMetrics() map[string]interface{} {
    var lastRun db.EngineRunStats
    db.DB.Order("run_at DESC").First(&lastRun)

    return map[string]interface{}{
        "lastRunEpoch":     lastRun.RunAt.Unix(),
        "lastRunEvaluated": lastRun.Evaluated,
        "lastRunFlagged":   lastRun.Flagged,
        "lastRunFreedBytes": lastRun.FreedBytes,
        "executionMode":    currentMode(), // still read from preferences
        "queueDepth":       len(deleteQueue),
        "currentlyDeleting": currentlyDeletingVal.Load(),
        // Cumulative totals from DB
        "totalEvaluated": totalEvaluated, // SELECT SUM(evaluated) FROM engine_run_stats
        "totalFlagged":   totalFlagged,
    }
}
```

### Step 5: Startup Initialization

On container start, `GetWorkerMetrics()` automatically returns the latest persisted row — no special initialization needed since we query the DB.

### Step 6: Cleanup Old Stats (Optional)

Add retention: keep only the last 1000 engine run stats (or 30 days). Run cleanup in the existing daily cron job.

## Files to Modify

| File | Change |
|------|--------|
| `backend/internal/db/models.go` | Add `EngineRunStats` struct |
| `backend/internal/db/migrations/00004_add_engine_run_stats.sql` | New migration |
| `backend/internal/poller/poller.go` | Write stats after each run + read from DB |
| `backend/routes/api.go` | No change (uses `GetWorkerMetrics()` which we're modifying) |

## UI Impact

No frontend changes needed — the engine control popover and dashboard already read from `/api/v1/worker/stats`. Once the backend persists and returns real data, the UI automatically shows correct values.

## Testing

1. Start container → engine popover should show stats from the last run (from DB)
2. Click "Run Now" → stats update immediately after run completes
3. Restart container → stats persist (from DB, not in-memory atomics)
4. Sparklines continue working independently (audit log, unaffected)
