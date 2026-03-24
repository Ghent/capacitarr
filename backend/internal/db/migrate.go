package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

// RunMigrations applies all pending Goose migrations using the embedded SQL files,
// then runs post-migration schema fixups for changes that can't be expressed as
// conditional DDL in pure SQL (e.g., column renames that must be idempotent).
// For existing databases the baseline migration (00001) is a no-op because every
// statement uses IF NOT EXISTS / INSERT OR IGNORE.  For fresh installs it creates
// the full schema.
func RunMigrations(sqlDB *sql.DB) error {
	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose set dialect: %w", err)
	}

	if err := goose.Up(sqlDB, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}

	// Post-migration schema fixups — idempotent DDL that checks column existence.
	if err := fixupEngineRunStats(sqlDB); err != nil {
		return fmt.Errorf("post-migration fixup: %w", err)
	}

	slog.Info("Database migrations applied successfully", "component", "db")
	return nil
}

// RunMigrationsDown rolls back all Goose migrations to version 0.
// This is used only in tests to verify migration reversibility.
func RunMigrationsDown(sqlDB *sql.DB) error {
	goose.SetBaseFS(embedMigrations)

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose set dialect: %w", err)
	}

	if err := goose.DownTo(sqlDB, "migrations", 0); err != nil {
		return fmt.Errorf("goose down: %w", err)
	}

	return nil
}

// fixupEngineRunStats applies idempotent schema changes to engine_run_stats:
//   - Renames "flagged" column to "candidates" (if flagged still exists)
//   - Adds "queued" column (if it doesn't exist yet)
//
// This handles the transition for databases created before the flagged→candidates
// rename. Fresh installs already have the correct schema from the baseline migration.
func fixupEngineRunStats(sqlDB *sql.DB) error {
	// Check which columns exist
	hasFlagged := hasColumn(sqlDB, "flagged")
	hasCandidates := hasColumn(sqlDB, "candidates")
	hasQueued := hasColumn(sqlDB, "queued")

	ctx := context.Background()

	// Rename flagged → candidates if the old column still exists
	if hasFlagged && !hasCandidates {
		slog.Info("Renaming engine_run_stats.flagged → candidates", "component", "db")
		if _, err := sqlDB.ExecContext(ctx, "ALTER TABLE engine_run_stats RENAME COLUMN flagged TO candidates"); err != nil {
			return fmt.Errorf("rename flagged→candidates: %w", err)
		}
	}

	// Add queued column if it doesn't exist
	if !hasQueued {
		slog.Info("Adding engine_run_stats.queued column", "component", "db")
		if _, err := sqlDB.ExecContext(ctx, "ALTER TABLE engine_run_stats ADD COLUMN queued INTEGER NOT NULL DEFAULT 0"); err != nil {
			return fmt.Errorf("add queued column: %w", err)
		}
	}

	return nil
}

// hasColumn checks if engine_run_stats has a specific column using PRAGMA table_info.
// Hardcoded to engine_run_stats to avoid string-formatted SQL queries (semgrep).
func hasColumn(sqlDB *sql.DB, column string) bool {
	// nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
	// Table name is hardcoded, not user input.
	rows, err := sqlDB.QueryContext(context.Background(), "PRAGMA table_info(engine_run_stats)")
	if err != nil {
		return false
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			continue
		}
		if name == column {
			return true
		}
	}
	return false
}
