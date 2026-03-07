package db

import (
	"testing"
)

func TestMigrations_UpDown(t *testing.T) {
	// Use the shared helper from driver_test.go
	gormDB := openTestDB(t)
	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("Failed to get sql.DB: %v", err)
	}

	// Run all migrations up
	if err := RunMigrations(sqlDB); err != nil {
		t.Fatalf("Migrations UP failed: %v", err)
	}

	// Run all migrations down to version 0
	if err := RunMigrationsDown(sqlDB); err != nil {
		t.Fatalf("Migrations DOWN failed: %v", err)
	}

	// Run all migrations up again (should be clean)
	if err := RunMigrations(sqlDB); err != nil {
		t.Fatalf("Second migrations UP failed: %v", err)
	}
}

func TestMigrations_Idempotent(t *testing.T) {
	gormDB := openTestDB(t)
	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("Failed to get sql.DB: %v", err)
	}

	// Run migrations up twice — second run should be a no-op
	if err := RunMigrations(sqlDB); err != nil {
		t.Fatalf("First migrations UP failed: %v", err)
	}
	if err := RunMigrations(sqlDB); err != nil {
		t.Fatalf("Second migrations UP (idempotent) failed: %v", err)
	}
}
