package services

import (
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

// newTestMetricsService creates a MetricsService with real engine/deletion dependencies
// backed by the test DB. Engine and deletion services are constructed but not started.
func newTestMetricsService(t *testing.T) *MetricsService {
	t.Helper()
	database := setupTestDB(t)
	bus := newTestBus(t)
	engine := NewEngineService(database, bus)
	auditLog := NewAuditLogService(database)
	deletion := NewDeletionService(bus, auditLog)
	settings := NewSettingsService(database, bus)
	svc := NewMetricsService(database, engine, deletion)
	svc.SetSettingsService(settings)
	deletion.SetDependencies(settings, engine, svc, nil)
	return svc
}

// ---------- GetHistory ----------

func TestMetricsService_GetHistory_Empty(t *testing.T) {
	svc := newTestMetricsService(t)

	history, err := svc.GetHistory("raw", "", "")
	if err != nil {
		t.Fatalf("GetHistory returned error: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("Expected 0 history entries, got %d", len(history))
	}
}

func TestMetricsService_GetHistory_WithSeededData(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	engine := NewEngineService(database, bus)
	auditLog := NewAuditLogService(database)
	deletion := NewDeletionService(bus, auditLog)
	svc := NewMetricsService(database, engine, deletion)

	// Seed history entries
	now := time.Now()
	database.Create(&db.LibraryHistory{
		Timestamp:     now.Add(-2 * time.Hour),
		TotalCapacity: 1000,
		UsedCapacity:  500,
		Resolution:    "raw",
	})
	database.Create(&db.LibraryHistory{
		Timestamp:     now.Add(-1 * time.Hour),
		TotalCapacity: 1000,
		UsedCapacity:  600,
		Resolution:    "raw",
	})
	database.Create(&db.LibraryHistory{
		Timestamp:     now,
		TotalCapacity: 1000,
		UsedCapacity:  700,
		Resolution:    "hourly",
	})

	// Fetch raw resolution — should get 2
	history, err := svc.GetHistory("raw", "", "")
	if err != nil {
		t.Fatalf("GetHistory returned error: %v", err)
	}
	if len(history) != 2 {
		t.Errorf("Expected 2 raw entries, got %d", len(history))
	}

	// Fetch hourly resolution — should get 1
	history, err = svc.GetHistory("hourly", "", "")
	if err != nil {
		t.Fatalf("GetHistory returned error: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("Expected 1 hourly entry, got %d", len(history))
	}
}

func TestMetricsService_GetHistory_WithTimeFilter(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	engine := NewEngineService(database, bus)
	auditLog := NewAuditLogService(database)
	deletion := NewDeletionService(bus, auditLog)
	svc := NewMetricsService(database, engine, deletion)

	now := time.Now()
	database.Create(&db.LibraryHistory{
		Timestamp:     now.Add(-48 * time.Hour),
		TotalCapacity: 1000,
		UsedCapacity:  400,
		Resolution:    "raw",
	})
	database.Create(&db.LibraryHistory{
		Timestamp:     now.Add(-30 * time.Minute),
		TotalCapacity: 1000,
		UsedCapacity:  500,
		Resolution:    "raw",
	})

	// With "1h" filter — should only get the recent entry
	history, err := svc.GetHistory("raw", "", "1h")
	if err != nil {
		t.Fatalf("GetHistory returned error: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("Expected 1 entry within 1h, got %d", len(history))
	}

	// With "7d" filter — should get both
	history, err = svc.GetHistory("raw", "", "7d")
	if err != nil {
		t.Fatalf("GetHistory returned error: %v", err)
	}
	if len(history) != 2 {
		t.Errorf("Expected 2 entries within 7d, got %d", len(history))
	}
}

func TestMetricsService_GetHistory_DefaultResolution(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	engine := NewEngineService(database, bus)
	auditLog := NewAuditLogService(database)
	deletion := NewDeletionService(bus, auditLog)
	svc := NewMetricsService(database, engine, deletion)

	database.Create(&db.LibraryHistory{
		Timestamp:     time.Now(),
		TotalCapacity: 1000,
		UsedCapacity:  500,
		Resolution:    "raw",
	})

	// Empty resolution should default to "raw"
	history, err := svc.GetHistory("", "", "")
	if err != nil {
		t.Fatalf("GetHistory returned error: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("Expected 1 entry with default resolution, got %d", len(history))
	}
}

// ---------- GetLifetimeStats ----------

func TestMetricsService_GetLifetimeStats_CreatesDefault(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	engine := NewEngineService(database, bus)
	auditLog := NewAuditLogService(database)
	deletion := NewDeletionService(bus, auditLog)
	svc := NewMetricsService(database, engine, deletion)

	stats, err := svc.GetLifetimeStats()
	if err != nil {
		t.Fatalf("GetLifetimeStats returned error: %v", err)
	}
	if stats.ID != 1 {
		t.Errorf("Expected ID 1, got %d", stats.ID)
	}
	if stats.TotalBytesReclaimed != 0 {
		t.Errorf("Expected TotalBytesReclaimed 0, got %d", stats.TotalBytesReclaimed)
	}
	if stats.TotalItemsRemoved != 0 {
		t.Errorf("Expected TotalItemsRemoved 0, got %d", stats.TotalItemsRemoved)
	}
	if stats.TotalEngineRuns != 0 {
		t.Errorf("Expected TotalEngineRuns 0, got %d", stats.TotalEngineRuns)
	}
}

// ---------- GetDashboardStats ----------

func TestMetricsService_GetDashboardStats_EmptyDB(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	engine := NewEngineService(database, bus)
	auditLog := NewAuditLogService(database)
	deletion := NewDeletionService(bus, auditLog)
	svc := NewMetricsService(database, engine, deletion)

	stats, err := svc.GetDashboardStats()
	if err != nil {
		t.Fatalf("GetDashboardStats returned error: %v", err)
	}

	// On empty DB, growth data should be false
	if stats["hasGrowthData"] != false {
		t.Error("Expected hasGrowthData to be false on empty DB")
	}
	if stats["growthBytesPerWeek"] != int64(0) {
		t.Errorf("Expected growthBytesPerWeek 0, got %v", stats["growthBytesPerWeek"])
	}

	// Lifetime stats should be zero defaults
	if stats["totalBytesReclaimed"] != int64(0) {
		t.Errorf("Expected totalBytesReclaimed 0, got %v", stats["totalBytesReclaimed"])
	}
	if stats["totalItemsRemoved"] != 0 {
		t.Errorf("Expected totalItemsRemoved 0, got %v", stats["totalItemsRemoved"])
	}
}

func TestMetricsService_GetDashboardStats_WithGrowthData(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	engine := NewEngineService(database, bus)
	auditLog := NewAuditLogService(database)
	deletion := NewDeletionService(bus, auditLog)
	svc := NewMetricsService(database, engine, deletion)

	now := time.Now()

	// Seed a "week ago" entry and a recent entry
	database.Create(&db.LibraryHistory{
		Timestamp:     now.Add(-8 * 24 * time.Hour), // 8 days ago (before cutoff)
		TotalCapacity: 1000000,
		UsedCapacity:  400000,
		Resolution:    "raw",
	})
	database.Create(&db.LibraryHistory{
		Timestamp:     now.Add(-1 * time.Hour), // Recent
		TotalCapacity: 1000000,
		UsedCapacity:  600000,
		Resolution:    "raw",
	})

	// Seed lifetime stats
	database.Create(&db.LifetimeStats{
		ID:                  1,
		TotalBytesReclaimed: 50000,
		TotalItemsRemoved:   5,
		TotalEngineRuns:     10,
	})

	stats, err := svc.GetDashboardStats()
	if err != nil {
		t.Fatalf("GetDashboardStats returned error: %v", err)
	}

	if stats["hasGrowthData"] != true {
		t.Error("Expected hasGrowthData to be true")
	}
	growth, ok := stats["growthBytesPerWeek"].(int64)
	if !ok {
		t.Fatalf("Expected growthBytesPerWeek to be int64, got %T", stats["growthBytesPerWeek"])
	}
	if growth != 200000 {
		t.Errorf("Expected growthBytesPerWeek 200000, got %d", growth)
	}
}

// ---------- GetCapacityForecast ----------

func TestMetricsService_GetCapacityForecast_NoHistory(t *testing.T) {
	svc := newTestMetricsService(t)

	forecast, err := svc.GetCapacityForecast(85.0, 1000000, 500000)
	if err != nil {
		t.Fatalf("GetCapacityForecast returned error: %v", err)
	}
	if forecast.DaysUntilThreshold != -1 {
		t.Errorf("expected -1 days until threshold with no data, got %d", forecast.DaysUntilThreshold)
	}
	if forecast.DaysUntilFull != -1 {
		t.Errorf("expected -1 days until full with no data, got %d", forecast.DaysUntilFull)
	}
	if forecast.CurrentUsedPct != 50 {
		t.Errorf("expected 50%% used, got %.1f%%", forecast.CurrentUsedPct)
	}
}

func TestMetricsService_GetCapacityForecast_WithGrowingHistory(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	engine := NewEngineService(database, bus)
	auditLog := NewAuditLogService(database)
	deletion := NewDeletionService(bus, auditLog)
	svc := NewMetricsService(database, engine, deletion)

	// Seed 30 days of linearly growing history: 500 GB → 530 GB (1 GB/day)
	now := time.Now()
	for i := 30; i >= 0; i-- {
		h := db.LibraryHistory{
			Timestamp:     now.Add(-time.Duration(i) * 24 * time.Hour),
			TotalCapacity: 1000 * 1024 * 1024 * 1024,            // 1 TB
			UsedCapacity:  int64(500+30-i) * 1024 * 1024 * 1024, // Growing 1 GB/day
			Resolution:    "raw",
		}
		database.Create(&h)
	}

	totalCap := int64(1000) * 1024 * 1024 * 1024 // 1 TB
	usedCap := int64(530) * 1024 * 1024 * 1024   // 530 GB
	forecast, err := svc.GetCapacityForecast(85.0, totalCap, usedCap)
	if err != nil {
		t.Fatalf("GetCapacityForecast returned error: %v", err)
	}

	if forecast.GrowthRatePerDay <= 0 {
		t.Errorf("expected positive growth rate, got %d", forecast.GrowthRatePerDay)
	}
	if forecast.DaysUntilThreshold <= 0 {
		t.Errorf("expected positive days until threshold, got %d", forecast.DaysUntilThreshold)
	}
	if forecast.DaysUntilFull <= 0 {
		t.Errorf("expected positive days until full, got %d", forecast.DaysUntilFull)
	}
	if forecast.DaysUntilFull <= forecast.DaysUntilThreshold {
		t.Errorf("expected days until full (%d) > days until threshold (%d)",
			forecast.DaysUntilFull, forecast.DaysUntilThreshold)
	}
}

func TestMetricsService_GetCapacityForecast_ShrinkingUsage(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	engine := NewEngineService(database, bus)
	auditLog := NewAuditLogService(database)
	deletion := NewDeletionService(bus, auditLog)
	svc := NewMetricsService(database, engine, deletion)

	// Seed 10 days of shrinking history: 800 GB → 700 GB
	now := time.Now()
	for i := 10; i >= 0; i-- {
		h := db.LibraryHistory{
			Timestamp:     now.Add(-time.Duration(i) * 24 * time.Hour),
			TotalCapacity: 1000 * 1024 * 1024 * 1024,
			UsedCapacity:  int64(700+i*10) * 1024 * 1024 * 1024, // Shrinking 10 GB/day
			Resolution:    "raw",
		}
		database.Create(&h)
	}

	totalCap := int64(1000) * 1024 * 1024 * 1024
	usedCap := int64(700) * 1024 * 1024 * 1024
	forecast, err := svc.GetCapacityForecast(85.0, totalCap, usedCap)
	if err != nil {
		t.Fatalf("GetCapacityForecast returned error: %v", err)
	}

	if forecast.GrowthRatePerDay >= 0 {
		t.Errorf("expected negative growth rate for shrinking usage, got %d", forecast.GrowthRatePerDay)
	}
	if forecast.DaysUntilThreshold != -1 {
		t.Errorf("expected -1 days until threshold when shrinking, got %d", forecast.DaysUntilThreshold)
	}
	if forecast.DaysUntilFull != -1 {
		t.Errorf("expected -1 days until full when shrinking, got %d", forecast.DaysUntilFull)
	}
}

// ---------- GetWorkerMetrics ----------

func TestMetricsService_GetWorkerMetrics_ReturnsExpectedKeys(t *testing.T) {
	database := setupTestDB(t)
	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })
	engine := NewEngineService(database, bus)
	auditLog := NewAuditLogService(database)
	deletion := NewDeletionService(bus, auditLog)
	settings := NewSettingsService(database, bus)
	svc := NewMetricsService(database, engine, deletion)
	svc.SetSettingsService(settings)
	deletion.SetDependencies(settings, engine, svc, nil)

	metrics := svc.GetWorkerMetrics()

	expectedKeys := []string{
		"isRunning",
		"lastRunEvaluated",
		"lastRunFlagged",
		"protectedCount",
		"pollIntervalSeconds",
		"executionMode",
		"queueDepth",
		"currentlyDeleting",
		"processed",
		"failed",
	}

	for _, key := range expectedKeys {
		if _, ok := metrics[key]; !ok {
			t.Errorf("Expected key %q in worker metrics", key)
		}
	}
}

func TestMetricsService_GetWorkerMetrics_ExecutionModeFromPreferences(t *testing.T) {
	database := setupTestDB(t)
	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })
	engine := NewEngineService(database, bus)
	auditLog := NewAuditLogService(database)
	deletion := NewDeletionService(bus, auditLog)
	settings := NewSettingsService(database, bus)
	svc := NewMetricsService(database, engine, deletion)
	svc.SetSettingsService(settings)
	deletion.SetDependencies(settings, engine, svc, nil)

	// Create an engine run stats record with "dry-run" mode (simulating a past run)
	database.Create(&db.EngineRunStats{ExecutionMode: "dry-run"})

	// Change the preference to "auto" (user changed mode without running engine)
	database.Save(&db.PreferenceSet{ID: 1, ExecutionMode: "auto", PollIntervalSeconds: 300, TiebreakerMethod: "size_desc", LogLevel: "info"})

	metrics := svc.GetWorkerMetrics()

	// The worker metrics should reflect the PREFERENCE value, not the last run
	mode, ok := metrics["executionMode"].(string)
	if !ok {
		t.Fatal("Expected executionMode to be a string")
	}
	if mode != "auto" {
		t.Errorf("Expected executionMode %q from preferences, got %q (likely reading from engine run stats)", "auto", mode)
	}
}
