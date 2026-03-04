package poller

import (
	"sync/atomic"
	"time"

	"capacitarr/internal/db"
)

// Per-run metrics (reset each engine evaluation cycle, read by GetWorkerMetrics
// for real-time "currently running" feedback while the poll is in progress)
var (
	lastRunEvaluated  int64
	lastRunFlagged    int64
	lastRunFreedBytes int64
	lastRunProtected  int64
)

// GetWorkerMetrics returns the current state of the backend deletion worker.
// Per-run stats are read from the DB (persisted across container restarts);
// real-time values (currentlyDeleting, queueDepth) come from in-memory atomics.
func GetWorkerMetrics() map[string]interface{} {
	var prefs db.PreferenceSet
	db.DB.FirstOrCreate(&prefs, db.PreferenceSet{ID: 1})

	mode := prefs.ExecutionMode
	if mode == "" {
		mode = "dry-run"
	}

	// Safely load currentlyDeleting (may be nil on first access)
	currentlyDeletion := ""
	if v := currentlyDeletingVal.Load(); v != nil {
		currentlyDeletion = v.(string)
	}

	// Read the latest persisted engine run stats from DB
	var lastRun db.EngineRunStats
	db.DB.Order("run_at DESC").First(&lastRun)

	// Compute cumulative totals from all persisted runs
	var totals struct {
		TotalEvaluated int64
		TotalFlagged   int64
		TotalFreed     int64
	}
	db.DB.Model(&db.EngineRunStats{}).
		Select("COALESCE(SUM(evaluated), 0) as total_evaluated, COALESCE(SUM(flagged), 0) as total_flagged, COALESCE(SUM(freed_bytes), 0) as total_freed").
		Scan(&totals)

	// If a poll is currently running, prefer the live in-memory atomics for real-time feedback
	lastRunEval := int64(lastRun.Evaluated)
	lastRunFlag := int64(lastRun.Flagged)
	lastRunFreed := lastRun.FreedBytes
	lastRunEpochVal := lastRun.RunAt.Unix()
	if pollRunning.Load() {
		lastRunEval = atomic.LoadInt64(&lastRunEvaluated)
		lastRunFlag = atomic.LoadInt64(&lastRunFlagged)
		lastRunFreed = atomic.LoadInt64(&lastRunFreedBytes)
		lastRunEpochVal = time.Now().Unix()
	}

	return map[string]interface{}{
		"executionMode":       mode,
		"isRunning":           pollRunning.Load(),
		"pollIntervalSeconds": prefs.PollIntervalSeconds,
		"queueDepth":          len(deleteQueue),
		"lastRunEvaluated":    lastRunEval,
		"lastRunFlagged":      lastRunFlag,
		"lastRunFreedBytes":   lastRunFreed,
		"lastRunEpoch":        lastRunEpochVal,
		"currentlyDeleting":   currentlyDeletion,
		"protectedCount":      atomic.LoadInt64(&lastRunProtected),
		// Cumulative totals from DB
		"evaluated":  totals.TotalEvaluated,
		"actioned":   totals.TotalFlagged,
		"freedBytes": totals.TotalFreed,
		"processed":  atomic.LoadInt64(&metricsProcessed),
		"failed":     atomic.LoadInt64(&metricsFailed),
	}
}
