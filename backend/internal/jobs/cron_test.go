package jobs

import (
	"testing"

	"capacitarr/internal/config"
	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/services"

	_ "github.com/ncruces/go-sqlite3/embed" // load the embedded SQLite WASM binary
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupCronTestDB creates an in-memory SQLite database with migrations applied,
// seeds default preferences, and returns a service registry suitable for cron tests.
func setupCronTestDB(t *testing.T) *services.Registry {
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
		ExecutionMode:         db.ModeDryRun,
		LogLevel:              db.LogLevelInfo,
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

	return reg
}

func TestStart_ReturnsValidScheduler(t *testing.T) {
	reg := setupCronTestDB(t)

	c := Start(reg)
	if c == nil {
		t.Fatal("Expected Start() to return a non-nil *cron.Cron")
	}
	t.Cleanup(func() { c.Stop() })
}

func TestStart_RegistersExpectedEntries(t *testing.T) {
	reg := setupCronTestDB(t)

	c := Start(reg)
	t.Cleanup(func() { c.Stop() })

	entries := c.Entries()
	// Expected: hourly rollup, daily rollup, weekly rollup, monthly prune,
	// engine stats prune, activity prune, audit log prune
	if len(entries) != 7 {
		t.Errorf("Expected 7 cron entries, got %d", len(entries))
	}
}

func TestStart_StopCleanly(t *testing.T) {
	reg := setupCronTestDB(t)

	c := Start(reg)

	// Stopping should not panic
	c.Stop()
}
