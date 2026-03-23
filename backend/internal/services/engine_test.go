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
	if stats["lastRunFlagged"] != int64(15) {
		t.Errorf("expected lastRunFlagged=15, got %v", stats["lastRunFlagged"])
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
		Flagged:       10,
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

	err := svc.UpdateRunStats(stats.ID, 100, 15, 2500)
	if err != nil {
		t.Fatalf("UpdateRunStats error: %v", err)
	}

	var updated db.EngineRunStats
	database.First(&updated, stats.ID)
	if updated.Evaluated != 100 {
		t.Errorf("expected evaluated 100, got %d", updated.Evaluated)
	}
	if updated.Flagged != 15 {
		t.Errorf("expected flagged 15, got %d", updated.Flagged)
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

	// Create some run stats
	_, _ = svc.CreateRunStats(db.ModeDryRun)
	_, _ = svc.CreateRunStats(db.ModeApproval)

	points, err := svc.GetHistory(24 * time.Hour)
	if err != nil {
		t.Fatalf("GetHistory error: %v", err)
	}
	if len(points) != 2 {
		t.Errorf("expected 2 history points, got %d", len(points))
	}
}
