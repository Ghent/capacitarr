package migration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/ncruces/go-sqlite3/embed" // embed: SQLite WASM binary
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestDetectLegacySchema_NoFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "capacitarr.db")

	if DetectLegacySchema(dbPath) {
		t.Error("expected false for non-existent file")
	}
}

func TestDetectLegacySchema_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "capacitarr.db")
	if err := os.WriteFile(dbPath, []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	if DetectLegacySchema(dbPath) {
		t.Error("expected false for empty file")
	}
}

func TestDetectLegacySchema_V1Database(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "capacitarr.db")

	// Create a minimal 1.x-like database (has goose_db_version, no libraries table)
	database, err := gorm.Open(gormlite.Open(dbPath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	sqlDB, _ := database.DB()
	_, _ = sqlDB.ExecContext(ctx, "CREATE TABLE goose_db_version (id INTEGER PRIMARY KEY, version_id INTEGER)")
	_, _ = sqlDB.ExecContext(ctx, "INSERT INTO goose_db_version (id, version_id) VALUES (1, 0), (2, 1), (3, 10)")
	_, _ = sqlDB.ExecContext(ctx, "CREATE TABLE auth_configs (id INTEGER PRIMARY KEY, username TEXT)")
	_ = sqlDB.Close()

	if !DetectLegacySchema(dbPath) {
		t.Error("expected true for 1.x database (has goose_db_version, no libraries)")
	}
}

func TestDetectLegacySchema_V2Database(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "capacitarr.db")

	// Create a 2.0-like database (has goose_db_version AND libraries table)
	database, err := gorm.Open(gormlite.Open(dbPath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	sqlDB, _ := database.DB()
	_, _ = sqlDB.ExecContext(ctx, "CREATE TABLE goose_db_version (id INTEGER PRIMARY KEY, version_id INTEGER)")
	_, _ = sqlDB.ExecContext(ctx, "INSERT INTO goose_db_version (id, version_id) VALUES (1, 0), (2, 1)")
	_, _ = sqlDB.ExecContext(ctx, "CREATE TABLE libraries (id INTEGER PRIMARY KEY, name TEXT)")
	_ = sqlDB.Close()

	if DetectLegacySchema(dbPath) {
		t.Error("expected false for 2.0 database (has libraries table)")
	}
}

func TestDetect1xBackup_NoFile(t *testing.T) {
	dir := t.TempDir()
	if Detect1xBackup(dir) {
		t.Error("expected false when no .v1.bak exists")
	}
}

func TestDetect1xBackup_FileExists(t *testing.T) {
	dir := t.TempDir()
	bakPath := filepath.Join(dir, "capacitarr.db.v1.bak")
	if err := os.WriteFile(bakPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}

	if !Detect1xBackup(dir) {
		t.Error("expected true when .v1.bak exists")
	}
}
