package services

import (
	"os"
	"path/filepath"
	"testing"

	"capacitarr/internal/events"
)

func TestMigrationService_Status_NoBackup(t *testing.T) {
	dir := t.TempDir()
	svc := NewMigrationService(nil, nil, dir)

	status := svc.Status()
	if status.Available {
		t.Error("expected Available=false when no .v1.bak backup exists")
	}
	if status.SourceDB != "" {
		t.Errorf("expected empty SourceDB, got %q", status.SourceDB)
	}
}

func TestMigrationService_Status_BackupExists(t *testing.T) {
	dir := t.TempDir()
	// Create a .v1.bak backup file (simulates post-detection rename)
	bakPath := filepath.Join(dir, "capacitarr.db.v1.bak")
	if err := os.WriteFile(bakPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := NewMigrationService(nil, nil, dir)
	status := svc.Status()

	if !status.Available {
		t.Error("expected Available=true when .v1.bak backup exists")
	}
	if status.SourceDB != bakPath {
		t.Errorf("expected SourceDB=%q, got %q", bakPath, status.SourceDB)
	}
}

func TestMigrationService_Execute_NoSource(t *testing.T) {
	dir := t.TempDir()
	db := setupTestDB(t)
	bus := events.NewEventBus()
	defer bus.Close()

	svc := NewMigrationService(db, bus, dir)
	result := svc.Execute()

	if result.Success {
		t.Error("expected Success=false when no source backup exists")
	}
	if result.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestMigrationService_Dismiss_NoBackup(t *testing.T) {
	dir := t.TempDir()
	svc := NewMigrationService(nil, nil, dir)

	err := svc.Dismiss()
	if err == nil {
		t.Error("expected error when dismissing non-existent backup")
	}
}

func TestMigrationService_Dismiss_RemovesBackup(t *testing.T) {
	dir := t.TempDir()
	bakPath := filepath.Join(dir, "capacitarr.db.v1.bak")
	if err := os.WriteFile(bakPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := NewMigrationService(nil, nil, dir)
	if err := svc.Dismiss(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the backup file was removed
	if _, err := os.Stat(bakPath); !os.IsNotExist(err) {
		t.Error("expected .v1.bak to be removed after dismiss")
	}

	// Status should now return unavailable
	status := svc.Status()
	if status.Available {
		t.Error("expected Available=false after dismiss")
	}
}
