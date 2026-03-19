package migration

import (
	"context"
	"database/sql"
	"log/slog"
	"os"

	_ "github.com/ncruces/go-sqlite3/embed" // embed: SQLite WASM binary required for gormlite
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// DetectLegacySchema checks whether the database file at dbPath contains a 1.x
// schema that is incompatible with the 2.0 baseline migration.
//
// Detection criteria:
//   - The database file exists
//   - The goose_db_version table exists (managed by Goose, present in all 1.x databases)
//   - The libraries table does NOT exist (2.0-only table, never present in 1.x)
//
// When all three conditions are true, the database is a 1.x schema. The 2.0
// baseline migration (version 1) would be skipped by Goose because the 1.x
// database already has version 1 recorded in goose_db_version.
//
// Returns false for: fresh installs (no file), empty databases, already-migrated
// 2.0 databases, or any databases where the schema cannot be determined.
func DetectLegacySchema(dbPath string) bool {
	// Check if the file exists at all
	info, err := os.Stat(dbPath)
	if err != nil || info.IsDir() {
		return false
	}

	// Open briefly to inspect the schema. We only run SELECT queries and close
	// immediately — no data is modified. The gormlite driver does not support
	// the ?mode=ro URI parameter, so we open in default (read-write) mode.
	database, err := gorm.Open(gormlite.Open(dbPath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		slog.Warn("Failed to open database for legacy schema detection",
			"component", "migration", "path", dbPath, "error", err)
		return false
	}
	sqlDB, err := database.DB()
	if err != nil {
		slog.Warn("Failed to get sql.DB for legacy schema detection",
			"component", "migration", "error", err)
		return false
	}
	defer func() {
		if closeErr := sqlDB.Close(); closeErr != nil {
			slog.Warn("Failed to close detection database", "error", closeErr)
		}
	}()

	// Check for goose_db_version table (present in all Goose-managed databases)
	if !tableExists(sqlDB, "goose_db_version") {
		// No goose table → not a managed database (empty file or non-Capacitarr DB)
		return false
	}

	// Check that libraries table does NOT exist (2.0-only table)
	if tableExists(sqlDB, "libraries") {
		// Has the 2.0 libraries table → already migrated to 2.0 schema
		return false
	}

	// Has goose_db_version but no libraries → 1.x schema
	slog.Info("Detected 1.x database schema",
		"component", "migration", "path", dbPath)
	return true
}

// tableExists checks whether a table with the given name exists in the SQLite database.
func tableExists(db *sql.DB, tableName string) bool {
	var name string
	err := db.QueryRowContext(context.Background(),
		"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
		tableName,
	).Scan(&name)
	return err == nil && name == tableName
}
