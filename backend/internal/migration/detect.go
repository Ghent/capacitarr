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
// Detection uses a three-tier approach:
//
//  1. schema_info table: migration 00005 writes schema_family='v2'. This is the
//     definitive marker. Once applied, all subsequent startups identify the
//     database unambiguously.
//
//  2. disk_groups table: transitional fallback for 2.0 databases that haven't
//     run migration 00005 yet. The disk_groups table is created by the v2
//     baseline (00001) and is a permanent part of the schema.
//
//  3. goose_db_version table: if present without either 2.0 marker, the
//     database is a 1.x schema.
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

	// Tier 1: Explicit schema version marker (definitive, added by migration 00005)
	if hasSchemaFamily(sqlDB, "v2") {
		return false
	}

	// Tier 2: Transitional fallback for 2.0 databases that predate migration 00005.
	// disk_groups is created by the v2 baseline and will never be removed.
	if tableExists(sqlDB, "disk_groups") {
		return false
	}

	// Has goose_db_version but neither 2.0 marker → 1.x schema
	slog.Info("Detected 1.x database schema",
		"component", "migration", "path", dbPath)
	return true
}

// ConfirmNotV2 is a defense-in-depth check called after DetectLegacySchema
// returns true. It independently verifies the database does NOT contain any
// 2.0-specific tables before the startup code renames it to a backup. This
// prevents a false positive in DetectLegacySchema from destroying a valid
// 2.0 database.
//
// Returns true if the database is safe to rename (confirmed not 2.0).
// Returns false if any 2.0 marker is found (abort the rename).
func ConfirmNotV2(dbPath string) bool {
	database, err := gorm.Open(gormlite.Open(dbPath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		// Can't open → be conservative, don't rename
		return false
	}
	sqlDB, err := database.DB()
	if err != nil {
		return false
	}
	defer func() { _ = sqlDB.Close() }()

	// Check multiple 2.0 tables to be thorough — any one of these existing
	// means this is a 2.0 database that must not be renamed.
	v2Tables := []string{
		"schema_info",            // migration 00005 (definitive marker)
		"disk_groups",            // v2 baseline (core table)
		"approval_queue_items",   // v2 baseline (never in 1.x)
		"scoring_factor_weights", // v2 baseline (never in 1.x)
	}
	for _, table := range v2Tables {
		if tableExists(sqlDB, table) {
			slog.Warn("ConfirmNotV2: found 2.0 table in database — aborting legacy rename",
				"component", "migration", "table", table, "dbPath", dbPath)
			return false
		}
	}
	return true
}

// hasSchemaFamily checks whether the schema_info table exists and contains the
// given schema_family value. Returns false if the table doesn't exist, the row
// is missing, or any query error occurs.
func hasSchemaFamily(db *sql.DB, family string) bool {
	var value string
	err := db.QueryRowContext(context.Background(),
		"SELECT value FROM schema_info WHERE key = 'schema_family'",
	).Scan(&value)
	return err == nil && value == family
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
