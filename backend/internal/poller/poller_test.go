package poller

import (
	"testing"
	"time"

	"capacitarr/internal/config"
	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/services"

	_ "github.com/ncruces/go-sqlite3/embed" // load the embedded SQLite WASM binary
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupPollerTestDB creates an in-memory SQLite database with migrations applied,
// seeds default preferences, and returns the database and a service registry.
func setupPollerTestDB(t *testing.T) (*gorm.DB, *services.Registry) {
	t.Helper()

	database, err := gorm.Open(gormlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to open in-memory SQLite: %v", err)
	}

	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("Failed to get underlying sql.DB: %v", err)
	}

	sqlDB.SetMaxOpenConns(1)

	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	pref := db.PreferenceSet{
		ID:                    1,
		ExecutionMode:         "dry-run",
		LogLevel:              "info",
		AuditLogRetentionDays: 30,
		PollIntervalSeconds:   300,
	}
	if err := database.FirstOrCreate(&pref, db.PreferenceSet{ID: 1}).Error; err != nil {
		t.Fatalf("Failed to seed preferences: %v", err)
	}

	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })

	cfg := &config.Config{JWTSecret: "test"}
	reg := services.NewRegistry(database, bus, cfg)
	reg.InitVersion("v0.0.0-test")

	return database, reg
}

// ---------- getPollInterval() ----------

func TestGetPollInterval_Default(t *testing.T) {
	_, reg := setupPollerTestDB(t)
	p := New(reg)

	interval := p.getPollInterval()
	expected := 5 * time.Minute
	if interval != expected {
		t.Errorf("Expected default interval %v, got %v", expected, interval)
	}
}

func TestGetPollInterval_BelowMinimum(t *testing.T) {
	database, reg := setupPollerTestDB(t)
	p := New(reg)

	// Set poll interval to 10s (below minimum of 30s)
	database.Model(&db.PreferenceSet{}).Where("id = 1").Update("poll_interval_seconds", 10)

	interval := p.getPollInterval()
	expected := 5 * time.Minute // falls back to 300s (5 min)
	if interval != expected {
		t.Errorf("Expected fallback interval %v for below-minimum value, got %v", expected, interval)
	}
}

func TestGetPollInterval_CustomValue(t *testing.T) {
	database, reg := setupPollerTestDB(t)
	p := New(reg)

	// Set poll interval to 60s
	database.Model(&db.PreferenceSet{}).Where("id = 1").Update("poll_interval_seconds", 60)

	interval := p.getPollInterval()
	expected := 1 * time.Minute
	if interval != expected {
		t.Errorf("Expected interval %v, got %v", expected, interval)
	}
}

// ---------- Start()/Stop() lifecycle ----------

func TestStartStop_Lifecycle(t *testing.T) {
	_, reg := setupPollerTestDB(t)
	p := New(reg)

	// Start should not panic
	p.Start()

	// Give the goroutine a moment to start
	time.Sleep(10 * time.Millisecond)

	// Stop should not panic
	p.Stop()
}

// ---------- safePoll() ----------

func TestSafePoll_NoPanic(t *testing.T) {
	_, reg := setupPollerTestDB(t)
	p := New(reg)

	// safePoll on a poller with no integrations should complete without panic.
	// It will attempt to poll but find no enabled integrations, which is safe.
	p.safePoll()
}
