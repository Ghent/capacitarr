package services

import (
	"testing"
	"time"

	"capacitarr/internal/db"
)

func TestEngineService_TriggerRun(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewEngineService(database, bus)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	result := svc.TriggerRun()
	if result != EngineStatusStarted {
		t.Errorf("expected %q, got %q", EngineStatusStarted, result)
	}

	// Verify ManualRunTriggeredEvent published on the EventBus
	select {
	case evt := <-ch:
		if evt.EventType() != "manual_run_triggered" {
			t.Errorf("expected event type 'manual_run_triggered', got %q", evt.EventType())
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for manual_run_triggered event")
	}
}

func TestEngineService_TriggerRun_AlreadyRunning(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewEngineService(database, bus)

	svc.SetRunning(true)
	result := svc.TriggerRun()
	if result != EngineStatusAlreadyRunning {
		t.Errorf("expected %q, got %q", EngineStatusAlreadyRunning, result)
	}
}

func TestEngineService_TriggerRun_IdempotentWhenIdle(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewEngineService(database, bus)

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	// Multiple triggers when idle should all succeed (EventBus is non-blocking)
	for i := 0; i < 3; i++ {
		result := svc.TriggerRun()
		if result != EngineStatusStarted {
			t.Errorf("trigger %d: expected %q, got %q", i, EngineStatusStarted, result)
		}
	}

	// All three events should be on the bus
	for i := 0; i < 3; i++ {
		select {
		case evt := <-ch:
			if evt.EventType() != "manual_run_triggered" {
				t.Errorf("trigger %d: expected 'manual_run_triggered', got %q", i, evt.EventType())
			}
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for event %d", i)
		}
	}
}

func TestEngineService_SetRunning(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewEngineService(database, bus)

	if svc.IsRunning() {
		t.Error("expected not running initially")
	}

	svc.SetRunning(true)
	if !svc.IsRunning() {
		t.Error("expected running after SetRunning(true)")
	}

	svc.SetRunning(false)
	if svc.IsRunning() {
		t.Error("expected not running after SetRunning(false)")
	}
}

func TestEngineService_SetLastRunStats(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewEngineService(database, bus)

	svc.SetLastRunStats(100, 15, 5)
	stats := svc.GetStats()

	if stats["lastRunEvaluated"] != int64(100) {
		t.Errorf("expected lastRunEvaluated=100, got %v", stats["lastRunEvaluated"])
	}
	if stats["lastRunCandidates"] != int64(15) {
		t.Errorf("expected lastRunCandidates=15, got %v", stats["lastRunCandidates"])
	}
	if stats["protectedCount"] != int64(5) {
		t.Errorf("expected protectedCount=5, got %v", stats["protectedCount"])
	}
}

func TestEngineService_GetStats_WithDBRecord(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewEngineService(database, bus)

	// Seed an engine run stats record with completed_at set
	completedAt := time.Now().UTC()
	runAt := completedAt.Add(-30 * time.Second) // run started 30s before completion
	runStats := db.EngineRunStats{
		RunAt:         runAt,
		CompletedAt:   &completedAt,
		Evaluated:     50,
		Candidates:    10,
		Queued:        7,
		Deleted:       3,
		FreedBytes:    5000000000,
		ExecutionMode: db.ModeApproval,
		DurationMs:    1500,
	}
	if err := database.Create(&runStats).Error; err != nil {
		t.Fatalf("Failed to create engine run stats: %v", err)
	}

	stats := svc.GetStats()

	if stats["executionMode"] != db.ModeApproval {
		t.Errorf("expected executionMode 'approval', got %v", stats["executionMode"])
	}
	if stats["lastRunFreedBytes"] != int64(5000000000) {
		t.Errorf("expected lastRunFreedBytes 5000000000, got %v", stats["lastRunFreedBytes"])
	}
	// lastRunEpoch should use completed_at, not run_at
	epoch, ok := stats["lastRunEpoch"].(int64)
	if !ok {
		t.Fatalf("expected lastRunEpoch to be int64, got %T", stats["lastRunEpoch"])
	}
	if epoch != completedAt.Unix() {
		t.Errorf("expected lastRunEpoch=%d (completed_at), got %d", completedAt.Unix(), epoch)
	}
}

func TestEngineService_GetStats_FallsBackToRunAt(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewEngineService(database, bus)

	// Seed a record WITHOUT completed_at (simulates pre-migration data)
	runAt := time.Now().UTC()
	runStats := db.EngineRunStats{
		RunAt:         runAt,
		ExecutionMode: db.ModeDryRun,
	}
	if err := database.Create(&runStats).Error; err != nil {
		t.Fatalf("Failed to create engine run stats: %v", err)
	}

	stats := svc.GetStats()

	epoch, ok := stats["lastRunEpoch"].(int64)
	if !ok {
		t.Fatalf("expected lastRunEpoch to be int64, got %T", stats["lastRunEpoch"])
	}
	if epoch != runAt.Unix() {
		t.Errorf("expected lastRunEpoch=%d (run_at fallback), got %d", runAt.Unix(), epoch)
	}
}

func TestEngineService_CreateRunStats(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewEngineService(database, bus)

	stats, err := svc.CreateRunStats(db.ModeDryRun)
	if err != nil {
		t.Fatalf("CreateRunStats error: %v", err)
	}
	if stats.ExecutionMode != db.ModeDryRun {
		t.Errorf("expected mode 'dry-run', got %q", stats.ExecutionMode)
	}
	if stats.ID == 0 {
		t.Error("expected non-zero ID")
	}
}

func TestEngineService_UpdateRunStats(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewEngineService(database, bus)

	stats, _ := svc.CreateRunStats(db.ModeDryRun)

	err := svc.UpdateRunStats(stats.ID, 100, 15, 8, 51200, 2500)
	if err != nil {
		t.Fatalf("UpdateRunStats error: %v", err)
	}

	var updated db.EngineRunStats
	database.First(&updated, stats.ID)
	if updated.Evaluated != 100 {
		t.Errorf("expected evaluated 100, got %d", updated.Evaluated)
	}
	if updated.Candidates != 15 {
		t.Errorf("expected candidates 15, got %d", updated.Candidates)
	}
	if updated.Queued != 8 {
		t.Errorf("expected queued 8, got %d", updated.Queued)
	}
	if updated.FreedBytes != 51200 {
		t.Errorf("expected freedBytes 51200, got %d", updated.FreedBytes)
	}
	if updated.CompletedAt == nil {
		t.Fatal("expected completed_at to be set after UpdateRunStats")
	}
	if updated.CompletedAt.Before(updated.RunAt) {
		t.Errorf("expected completed_at (%v) to be after run_at (%v)",
			updated.CompletedAt, updated.RunAt)
	}
}

func TestEngineService_LatestRunStatsID(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewEngineService(database, bus)

	// No rows exist yet — should return 0
	if id := svc.LatestRunStatsID(); id != 0 {
		t.Errorf("expected 0 when no rows exist, got %d", id)
	}

	// Create two run stats — latest should win
	first, _ := svc.CreateRunStats(db.ModeDryRun)
	time.Sleep(10 * time.Millisecond) // ensure distinct timestamps
	second, _ := svc.CreateRunStats(db.ModeApproval)

	id := svc.LatestRunStatsID()
	if id != second.ID {
		t.Errorf("expected latest ID %d, got %d (first was %d)", second.ID, id, first.ID)
	}
}

func TestEngineService_GetHistory(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewEngineService(database, bus)

	// Create some run stats and update them
	stats1, _ := svc.CreateRunStats(db.ModeDryRun)
	_ = svc.UpdateRunStats(stats1.ID, 50, 10, 6, 25600, 1000)

	stats2, _ := svc.CreateRunStats(db.ModeApproval)
	_ = svc.UpdateRunStats(stats2.ID, 80, 20, 12, 40960, 1500)

	points, err := svc.GetHistory(24 * time.Hour)
	if err != nil {
		t.Fatalf("GetHistory error: %v", err)
	}
	if len(points) != 2 {
		t.Fatalf("expected 2 history points, got %d", len(points))
	}

	// Verify first point has candidates and queued populated
	if points[0].Candidates != 10 {
		t.Errorf("expected point[0].Candidates=10, got %d", points[0].Candidates)
	}
	if points[0].Queued != 6 {
		t.Errorf("expected point[0].Queued=6, got %d", points[0].Queued)
	}
	if points[0].ExecutionMode != db.ModeDryRun {
		t.Errorf("expected point[0].ExecutionMode=%q, got %q", db.ModeDryRun, points[0].ExecutionMode)
	}

	// Verify second point
	if points[1].Candidates != 20 {
		t.Errorf("expected point[1].Candidates=20, got %d", points[1].Candidates)
	}
	if points[1].Queued != 12 {
		t.Errorf("expected point[1].Queued=12, got %d", points[1].Queued)
	}
	if points[1].ExecutionMode != db.ModeApproval {
		t.Errorf("expected point[1].ExecutionMode=%q, got %q", db.ModeApproval, points[1].ExecutionMode)
	}
}
