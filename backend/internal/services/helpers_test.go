package services

import (
	"testing"

	_ "github.com/ncruces/go-sqlite3/embed"
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

// setupTestDB creates an in-memory SQLite database with migrations applied.
// Local helper to avoid importing testutil (which pulls in routes → services).
func setupTestDB(t *testing.T) *gorm.DB {
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
	if err := db.AutoMigrateAll(database); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	pref := db.PreferenceSet{ID: 1, DefaultDiskGroupMode: db.ModeDryRun, LogLevel: db.LogLevelInfo, AuditLogRetentionDays: 30}
	if err := database.FirstOrCreate(&pref, db.PreferenceSet{ID: 1}).Error; err != nil {
		t.Fatalf("Failed to seed preferences: %v", err)
	}

	return database
}

// seedTestIntegration creates a test integration and returns a pointer to its ID.
func seedTestIntegration(t *testing.T, database *gorm.DB) *uint {
	t.Helper()
	ic := db.IntegrationConfig{
		Name:    "my-sonarr",
		Type:    "sonarr",
		URL:     "http://localhost:8989",
		APIKey:  "test-api-key",
		Enabled: true,
	}
	if err := database.Create(&ic).Error; err != nil {
		t.Fatalf("Failed to seed test integration: %v", err)
	}
	return &ic.ID
}

func newTestBus(t *testing.T) *events.EventBus {
	t.Helper()
	bus := events.NewEventBus()
	t.Cleanup(func() { bus.Close() })
	return bus
}

func seedIntegration(t *testing.T, database *gorm.DB) uint {
	t.Helper()
	ic := db.IntegrationConfig{
		Type:   "sonarr",
		Name:   "Test Sonarr",
		URL:    "http://localhost:8989",
		APIKey: "test-key",
	}
	if err := database.Create(&ic).Error; err != nil {
		t.Fatalf("Failed to seed integration: %v", err)
	}
	return ic.ID
}

func seedDiskGroup(t *testing.T, database *gorm.DB) uint {
	t.Helper()
	dg := db.DiskGroup{
		MountPath:    "/mnt/media",
		TotalBytes:   1000000000,
		UsedBytes:    900000000,
		ThresholdPct: 80.0,
		TargetPct:    70.0,
	}
	if err := database.Create(&dg).Error; err != nil {
		t.Fatalf("Failed to seed disk group: %v", err)
	}
	return dg.ID
}
