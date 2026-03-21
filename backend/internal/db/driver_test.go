package db

import (
	"fmt"
	"sync"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/embed" // load the embedded SQLite WASM binary
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// openTestDB creates an in-memory SQLite database for driver-level tests.
// It limits the connection pool to 1 to avoid the ncruces/go-sqlite3 behavior
// where each connection to ":memory:" creates a separate database.
func openTestDB(t *testing.T) *gorm.DB {
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

	return database
}

// TestMigrationUpDownUp runs all Goose migrations up, then down, then up
// again. This catches any SQL syntax that ncruces/go-sqlite3 handles
// differently from mattn/go-sqlite3 (e.g., different SQLite compile options).
func TestMigrationUpDownUp(t *testing.T) {
	database := openTestDB(t)

	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("Failed to get underlying sql.DB: %v", err)
	}

	// First up
	if err := RunMigrations(sqlDB); err != nil {
		t.Fatalf("First migration up failed: %v", err)
	}

	// Down — roll back all migrations
	if err := RunMigrationsDown(sqlDB); err != nil {
		t.Fatalf("Migration down failed: %v", err)
	}

	// Second up — re-apply from scratch
	if err := RunMigrations(sqlDB); err != nil {
		t.Fatalf("Second migration up failed: %v", err)
	}

	// Verify a core table exists by querying it
	var count int64
	if err := database.Raw("SELECT COUNT(*) FROM preference_sets").Scan(&count).Error; err != nil {
		t.Fatalf("Failed to query preference_sets after re-migration: %v", err)
	}
}

// TestConcurrentAccess verifies that 10 goroutines can do concurrent reads
// and writes without errors. ncruces/go-sqlite3 uses a WASM-based SQLite
// runtime, so this test confirms GORM's connection pool and the driver's
// thread safety work correctly together.
func TestConcurrentAccess(t *testing.T) {
	database := openTestDB(t)

	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("Failed to get underlying sql.DB: %v", err)
	}

	if err := RunMigrations(sqlDB); err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Seed a preference row for reads
	pref := PreferenceSet{ID: 1, ExecutionMode: "dry-run", LogLevel: "info"}
	if err := database.Create(&pref).Error; err != nil {
		t.Fatalf("Failed to seed preferences: %v", err)
	}

	const goroutines = 10
	var wg sync.WaitGroup
	errs := make(chan error, goroutines*2)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Write: create an audit log entry
			log := AuditLogEntry{
				MediaName: fmt.Sprintf("concurrent-test-%d", idx),
				MediaType: "movie",
				Action:    "deleted",
				SizeBytes: int64(idx * 1000),
			}
			if err := database.Create(&log).Error; err != nil {
				errs <- fmt.Errorf("goroutine %d write failed: %w", idx, err)
				return
			}

			// Read: query preferences
			var p PreferenceSet
			if err := database.First(&p, 1).Error; err != nil {
				errs <- fmt.Errorf("goroutine %d read failed: %w", idx, err)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}

	// Verify all writes landed
	var count int64
	database.Model(&AuditLogEntry{}).Where("media_name LIKE ?", "concurrent-test-%").Count(&count)
	if count != goroutines {
		t.Errorf("Expected %d audit logs, got %d", goroutines, count)
	}
}

// TestJournalMode verifies the SQLite journal mode after database
// initialization. Different SQLite drivers may default to different
// journal modes (delete, wal, memory, etc.).
func TestJournalMode(t *testing.T) {
	database := openTestDB(t)

	var journalMode string
	if err := database.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
		t.Fatalf("Failed to query journal_mode: %v", err)
	}

	// In-memory databases typically use "memory" journal mode.
	// For file-based databases, ncruces defaults to "delete".
	// We just verify it returns a valid, non-empty value.
	validModes := map[string]bool{
		"delete":   true,
		"truncate": true,
		"persist":  true,
		"memory":   true,
		"wal":      true,
		"off":      true,
	}

	if !validModes[journalMode] {
		t.Errorf("Unexpected journal_mode %q", journalMode)
	}

	t.Logf("journal_mode = %s", journalMode)
}

// TestDataTypeRoundTrip writes specific values for each column type used
// in the schema (DATETIME, REAL, INTEGER, TEXT, NULL) and reads them back.
// This catches any type conversion differences in the WASM bridge.
func TestDataTypeRoundTrip(t *testing.T) {
	database := openTestDB(t)

	sqlDB, err := database.DB()
	if err != nil {
		t.Fatalf("Failed to get underlying sql.DB: %v", err)
	}

	if err := RunMigrations(sqlDB); err != nil {
		t.Fatalf("Migration failed: %v", err)
	}

	// Test DATETIME round-trip via AuditLogEntry.CreatedAt
	now := time.Now().Truncate(time.Second)
	log := AuditLogEntry{
		MediaName: "type-test",
		MediaType: "series",
		Action:    "deleted",
		SizeBytes: 9876543210, // large INTEGER
		CreatedAt: now,
	}
	if err := database.Create(&log).Error; err != nil {
		t.Fatalf("Failed to create audit log: %v", err)
	}

	var retrieved AuditLogEntry
	if err := database.First(&retrieved, log.ID).Error; err != nil {
		t.Fatalf("Failed to retrieve audit log: %v", err)
	}

	// INTEGER round-trip
	if retrieved.SizeBytes != 9876543210 {
		t.Errorf("INTEGER round-trip: expected 9876543210, got %d", retrieved.SizeBytes)
	}

	// DATETIME round-trip (compare truncated to second)
	if !retrieved.CreatedAt.Truncate(time.Second).Equal(now) {
		t.Errorf("DATETIME round-trip: expected %v, got %v", now, retrieved.CreatedAt)
	}

	// TEXT round-trip
	if retrieved.MediaName != "type-test" {
		t.Errorf("TEXT round-trip: expected 'type-test', got %q", retrieved.MediaName)
	}

	// REAL round-trip via DiskGroup.ThresholdPct
	group := DiskGroup{
		MountPath:    "/test-roundtrip",
		TotalBytes:   1000000000000,
		UsedBytes:    750000000000,
		ThresholdPct: 85.5,
		TargetPct:    72.3,
	}
	if err := database.Create(&group).Error; err != nil {
		t.Fatalf("Failed to create disk group: %v", err)
	}

	var retrievedGroup DiskGroup
	if err := database.First(&retrievedGroup, group.ID).Error; err != nil {
		t.Fatalf("Failed to retrieve disk group: %v", err)
	}

	if retrievedGroup.ThresholdPct != 85.5 {
		t.Errorf("REAL round-trip (ThresholdPct): expected 85.5, got %f", retrievedGroup.ThresholdPct)
	}
	if retrievedGroup.TargetPct != 72.3 {
		t.Errorf("REAL round-trip (TargetPct): expected 72.3, got %f", retrievedGroup.TargetPct)
	}

	// NULL round-trip — ScoreDetails is nullable TEXT
	logWithNull := AuditLogEntry{
		MediaName: "null-test",
		MediaType: "movie",
		Action:    "dry_delete",
	}
	if err := database.Create(&logWithNull).Error; err != nil {
		t.Fatalf("Failed to create audit log with null fields: %v", err)
	}

	var retrievedNull AuditLogEntry
	if err := database.First(&retrievedNull, logWithNull.ID).Error; err != nil {
		t.Fatalf("Failed to retrieve null audit log: %v", err)
	}

	// ScoreDetails should be empty/null
	if retrievedNull.ScoreDetails != "" {
		t.Errorf("NULL round-trip: expected empty ScoreDetails, got %q", retrievedNull.ScoreDetails)
	}

	// Zero-value DATETIME — verify it doesn't panic or corrupt
	var zeroTime time.Time
	logZero := AuditLogEntry{
		MediaName: "zero-time-test",
		MediaType: "movie",
		Action:    "deleted",
		CreatedAt: zeroTime,
	}
	if err := database.Create(&logZero).Error; err != nil {
		t.Fatalf("Failed to create audit log with zero time: %v", err)
	}

	var retrievedZero AuditLogEntry
	if err := database.First(&retrievedZero, logZero.ID).Error; err != nil {
		t.Fatalf("Failed to retrieve zero-time audit log: %v", err)
	}

	// Verify the row was stored and retrieved without error
	if retrievedZero.MediaName != "zero-time-test" {
		t.Errorf("Zero-time round-trip: expected 'zero-time-test', got %q", retrievedZero.MediaName)
	}
}

// TestSQLiteVersion logs the SQLite version compiled into ncruces/go-sqlite3.
// Not a pass/fail test — purely informational for debugging.
func TestSQLiteVersion(t *testing.T) {
	database := openTestDB(t)

	var version string
	if err := database.Raw("SELECT sqlite_version()").Scan(&version).Error; err != nil {
		t.Fatalf("Failed to query sqlite_version(): %v", err)
	}

	t.Logf("SQLite version: %s", version)

	// Verify it's a reasonable version (3.x)
	if len(version) < 3 || version[0] != '3' {
		t.Errorf("Unexpected SQLite version: %s", version)
	}
}

// TestForeignKeysEnabled verifies that foreign keys are supported by the
// driver, even if not enforced by default (GORM doesn't enable them by default).
func TestForeignKeysEnabled(t *testing.T) {
	database := openTestDB(t)

	var fkEnabled int
	if err := database.Raw("PRAGMA foreign_keys").Scan(&fkEnabled).Error; err != nil {
		t.Fatalf("Failed to query foreign_keys pragma: %v", err)
	}

	// Just log it — GORM doesn't enable foreign keys by default
	t.Logf("foreign_keys = %d", fkEnabled)

	// Verify we CAN enable foreign keys (driver supports it)
	if err := database.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	if err := database.Raw("PRAGMA foreign_keys").Scan(&fkEnabled).Error; err != nil {
		t.Fatalf("Failed to re-query foreign_keys pragma: %v", err)
	}

	if fkEnabled != 1 {
		t.Errorf("Expected foreign_keys=1 after enabling, got %d", fkEnabled)
	}
}
