// Package db provides database initialization, migrations, and model definitions.
package db

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"capacitarr/internal/config"
	"capacitarr/internal/logger"
	_ "github.com/ncruces/go-sqlite3/embed" // load the embedded SQLite WASM binary
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// FactorDefault describes a scoring factor's key and default weight for seeding.
// Defined here to avoid importing the engine package from db (which would create
// a circular dependency: db → engine → db). The engine package provides its own
// DefaultFactors() which returns richer types; main.go bridges the two.
type FactorDefault struct {
	Key           string
	DefaultWeight int
}

// buildDSN converts a bare database path into a file: URI with SQLite PRAGMAs
// that enable WAL mode and a busy timeout. In-memory databases (":memory:")
// are returned unchanged because WAL mode is not supported for them.
//
// The resulting DSN sets:
//   - journal_mode=wal: allows concurrent readers during writes, eliminating
//     most "database is locked" errors from goroutine contention.
//   - busy_timeout=5000: waits up to 5 seconds for a lock before returning
//     SQLITE_BUSY, instead of failing immediately.
//   - _txlock=immediate: acquires a write lock at BEGIN instead of at first
//     write statement, preventing SQLITE_BUSY mid-transaction.
func buildDSN(dbPath string) string {
	if dbPath == ":memory:" || strings.HasPrefix(dbPath, "file:") {
		return dbPath
	}

	params := url.Values{}
	params.Add("_pragma", "journal_mode(wal)")
	params.Add("_pragma", "busy_timeout(5000)")
	params.Set("_txlock", "immediate")

	return fmt.Sprintf("file:%s?%s", dbPath, params.Encode())
}

// Init opens the SQLite database, runs migrations, and returns the connection.
//
// The database is opened with WAL journal mode and a 5-second busy timeout to
// prevent "database is locked" errors from concurrent goroutine access (poller,
// deletion worker, activity persister, cron jobs, HTTP handlers). The connection
// pool is limited to a single connection because SQLite supports only one
// concurrent writer; serializing all access through one connection eliminates
// file-level lock contention entirely.
func Init(cfg *config.Config) (*gorm.DB, error) {
	logLevel := gormlogger.Warn
	if cfg.Debug {
		logLevel = gormlogger.Info
	}

	dsn := buildDSN(cfg.Database)
	slog.Info("Opening database", "component", "db", "dsn", dsn)

	database, err := gorm.Open(gormlite.Open(dsn), &gorm.Config{
		Logger: logger.NewGormLogger(0).LogMode(logLevel),
	})
	if err != nil {
		slog.Error("Failed to connect to database", "component", "db", "operation", "connect", "error", err)
		return nil, err
	}

	// Limit the connection pool to a single connection. SQLite only supports
	// one concurrent writer; multiple pool connections cause file-level lock
	// contention ("database is locked" errors). A single connection serializes
	// all reads and writes through one path, which combined with WAL mode
	// provides the best throughput for SQLite's single-writer architecture.
	sqlDB, err := database.DB()
	if err != nil {
		slog.Error("Failed to get underlying sql.DB for configuration", "component", "db", "operation", "get_sql_db", "error", err)
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)

	if err := RunMigrations(sqlDB); err != nil {
		slog.Error("Failed to run database migrations", "component", "db", "operation", "migrate", "error", err)
		return nil, err
	}

	// Ensure default preferences exist with strictly safe defaults
	var pref PreferenceSet
	if err := database.FirstOrCreate(&pref, PreferenceSet{
		ID:                    1,
		ExecutionMode:         ModeDryRun,
		LogLevel:              LogLevelInfo,
		AuditLogRetentionDays: 30,
		PollIntervalSeconds:   300,
		TiebreakerMethod:      TiebreakerSizeDesc,
		DeletionsEnabled:      true,
		SnoozeDurationHours:   24,
		CheckForUpdates:       true,
		DeadContentMinDays:    90,
		StaleContentDays:      180,
	}).Error; err != nil {
		slog.Error("Failed to seed default preferences", "component", "db", "operation", "seed_preferences", "error", err)
	}

	// Apply dynamic log level from preferences
	logger.SetLevel(pref.LogLevel)

	// Log the active journal mode to confirm WAL is in effect.
	// In-memory databases use "memory" mode; file-based databases should report "wal".
	var journalMode string
	if err := database.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
		slog.Warn("Failed to query journal_mode", "component", "db", "error", err)
	}

	slog.Info("Database initialized successfully", "component", "db",
		"path", cfg.Database, "journalMode", journalMode, "maxOpenConns", 1)
	return database, nil
}

// InitReadOnly opens a read-only connection to the same database. In WAL mode,
// SQLite supports concurrent readers alongside a single writer. Services that
// are read-heavy (metrics, preview, analytics) can use this connection to avoid
// blocking behind write transactions on the primary connection.
//
// The returned connection has mode=ro to prevent accidental writes. The pool
// is sized to allow multiple concurrent readers (up to 4) since SQLite WAL
// mode supports this without lock contention.
//
// For in-memory databases (used in tests), nil is returned because ":memory:"
// databases do not support multiple connections — each connection gets its own
// independent in-memory database.
func InitReadOnly(cfg *config.Config) *gorm.DB {
	if cfg.Database == ":memory:" || strings.HasPrefix(cfg.Database, "file::memory:") {
		return nil
	}

	// Build a read-only DSN with mode=ro
	params := url.Values{}
	params.Add("_pragma", "journal_mode(wal)")
	params.Add("_pragma", "busy_timeout(5000)")
	params.Set("mode", "ro")

	dsn := fmt.Sprintf("file:%s?%s", cfg.Database, params.Encode())

	readDB, err := gorm.Open(gormlite.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		slog.Warn("Failed to open read-only database connection — falling back to shared connection",
			"component", "db", "error", err)
		return nil
	}

	sqlDB, err := readDB.DB()
	if err != nil {
		slog.Warn("Failed to configure read-only connection pool",
			"component", "db", "error", err)
		return nil
	}
	sqlDB.SetMaxOpenConns(4)

	slog.Info("Read-only database connection initialized", "component", "db", "maxOpenConns", 4)
	return readDB
}

// SeedFactorWeights ensures a scoring_factor_weights row exists for each
// registered factor. Missing keys are inserted with their DefaultWeight;
// existing rows are left unchanged so user customizations are preserved.
//
// Called from main.go after Init, passing FactorDefaults derived from
// engine.DefaultFactors() to avoid a circular import.
func SeedFactorWeights(database *gorm.DB, defaults []FactorDefault) {
	for _, fd := range defaults {
		var existing ScoringFactorWeight
		result := database.Where("factor_key = ?", fd.Key).First(&existing)
		if result.Error != nil {
			// Row doesn't exist — seed it
			row := ScoringFactorWeight{
				FactorKey: fd.Key,
				Weight:    fd.DefaultWeight,
			}
			if err := database.Create(&row).Error; err != nil {
				slog.Error("Failed to seed scoring factor weight",
					"component", "db", "factor", fd.Key, "error", err)
			} else {
				slog.Debug("Seeded scoring factor weight",
					"component", "db", "factor", fd.Key, "weight", fd.DefaultWeight)
			}
		}
	}
}
