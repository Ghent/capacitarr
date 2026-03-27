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

	// Create a minimal 1.x-like database (has goose_db_version, no schema_info or disk_groups)
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
		t.Error("expected true for 1.x database (has goose_db_version, no schema_info or disk_groups)")
	}
}

func TestDetectLegacySchema_V2Database_WithSchemaInfo(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "capacitarr.db")

	// Tier 1: 2.0 database with the schema_info marker (migration 00005+)
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
	_, _ = sqlDB.ExecContext(ctx, "CREATE TABLE schema_info (key TEXT PRIMARY KEY, value TEXT NOT NULL)")
	_, _ = sqlDB.ExecContext(ctx, "INSERT INTO schema_info (key, value) VALUES ('schema_family', 'v2')")
	_ = sqlDB.Close()

	if DetectLegacySchema(dbPath) {
		t.Error("expected false for 2.0 database (has schema_info with v2)")
	}
}

func TestDetectLegacySchema_V2Database_TransitionalFallback(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "capacitarr.db")

	// Tier 2: 2.0 database that predates migration 00005 (no schema_info,
	// but has disk_groups from the v2 baseline)
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
	_, _ = sqlDB.ExecContext(ctx, "CREATE TABLE disk_groups (id INTEGER PRIMARY KEY, mount_path TEXT)")
	_ = sqlDB.Close()

	if DetectLegacySchema(dbPath) {
		t.Error("expected false for 2.0 database (has disk_groups, no schema_info yet)")
	}
}

func TestDetectLegacySchema_V2Database_LibrariesDropped(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "capacitarr.db")

	// Regression test: 2.0 database after migration 00003 dropped the
	// libraries table. The old detection checked for libraries and would
	// have falsely detected this as 1.x.
	database, err := gorm.Open(gormlite.Open(dbPath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	sqlDB, _ := database.DB()
	_, _ = sqlDB.ExecContext(ctx, "CREATE TABLE goose_db_version (id INTEGER PRIMARY KEY, version_id INTEGER)")
	_, _ = sqlDB.ExecContext(ctx, "INSERT INTO goose_db_version (id, version_id) VALUES (1, 0), (2, 1), (3, 2), (4, 3)")
	_, _ = sqlDB.ExecContext(ctx, "CREATE TABLE disk_groups (id INTEGER PRIMARY KEY, mount_path TEXT)")
	// No libraries table — it was dropped by migration 00003
	_ = sqlDB.Close()

	if DetectLegacySchema(dbPath) {
		t.Error("expected false for 2.0 database after libraries table was dropped")
	}
}

func TestConfirmNotV2_BlocksRenameForV2Database(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "capacitarr.db")

	// Create a database with a 2.0 table — ConfirmNotV2 should return false
	database, err := gorm.Open(gormlite.Open(dbPath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	sqlDB, _ := database.DB()
	_, _ = sqlDB.ExecContext(ctx, "CREATE TABLE disk_groups (id INTEGER PRIMARY KEY, mount_path TEXT)")
	_ = sqlDB.Close()

	if ConfirmNotV2(dbPath) {
		t.Error("expected false — database has 2.0 disk_groups table, rename should be blocked")
	}
}

func TestConfirmNotV2_AllowsRenameForV1Database(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "capacitarr.db")

	// Create a database with only 1.x tables — ConfirmNotV2 should return true
	database, err := gorm.Open(gormlite.Open(dbPath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	sqlDB, _ := database.DB()
	_, _ = sqlDB.ExecContext(ctx, "CREATE TABLE goose_db_version (id INTEGER PRIMARY KEY, version_id INTEGER)")
	_, _ = sqlDB.ExecContext(ctx, "CREATE TABLE auth_configs (id INTEGER PRIMARY KEY, username TEXT)")
	_ = sqlDB.Close()

	if !ConfirmNotV2(dbPath) {
		t.Error("expected true — database has no 2.0 tables, rename should proceed")
	}
}

func TestDetect1xBackup_NoFile(t *testing.T) {
	dir := t.TempDir()
	if Detect1xBackup(dir) {
		t.Error("expected false when no backup exists")
	}
}

func TestDetect1xBackup_FileExists(t *testing.T) {
	dir := t.TempDir()
	bakPath := filepath.Join(dir, backupFilename)
	if err := os.WriteFile(bakPath, []byte("fake"), 0o600); err != nil {
		t.Fatal(err)
	}

	if !Detect1xBackup(dir) {
		t.Error("expected true when backup exists")
	}
}
