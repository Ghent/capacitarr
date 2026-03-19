package services

import (
	"os"
	"path/filepath"
	"testing"

	"capacitarr/internal/events"
)

func TestMigrationService_Status_NoDatabase(t *testing.T) {
	dir := t.TempDir()
	svc := NewMigrationService(nil, nil, dir)

	status := svc.Status()
	if status.Available {
		t.Error("expected Available=false when no 1.x database exists")
	}
	if status.SourceDB != "" {
		t.Errorf("expected empty SourceDB, got %q", status.SourceDB)
	}
}

func TestMigrationService_Status_DatabaseExists(t *testing.T) {
	dir := t.TempDir()
	// Create a fake 1.x database file
	dbPath := filepath.Join(dir, "capacitarr.db")
	if err := os.WriteFile(dbPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := NewMigrationService(nil, nil, dir)
	status := svc.Status()

	if !status.Available {
		t.Error("expected Available=true when 1.x database exists")
	}
	if status.SourceDB != dbPath {
		t.Errorf("expected SourceDB=%q, got %q", dbPath, status.SourceDB)
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
		t.Error("expected Success=false when no source database exists")
	}
	if result.Error == "" {
		t.Error("expected non-empty error message")
	}
}
