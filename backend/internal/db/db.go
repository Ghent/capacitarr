// Package db provides database initialization, migrations, and model definitions.
package db

import (
	"log/slog"

	"capacitarr/internal/config"
	"capacitarr/internal/logger"
	_ "github.com/ncruces/go-sqlite3/embed" // load the embedded SQLite WASM binary
	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// DB is the package-level database connection used throughout the application.
var DB *gorm.DB

// Init opens the SQLite database, runs migrations, and stores the connection in the package-level DB variable.
func Init(cfg *config.Config) error {
	logLevel := gormlogger.Warn
	if cfg.Debug {
		logLevel = gormlogger.Info
	}

	db, err := gorm.Open(gormlite.Open(cfg.Database), &gorm.Config{
		Logger: gormlogger.Default.LogMode(logLevel),
	})
	if err != nil {
		slog.Error("Failed to connect to database", "component", "db", "operation", "connect", "error", err)
		return err
	}

	// Run Goose migrations (sole schema management — no AutoMigrate)
	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("Failed to get underlying sql.DB for migrations", "component", "db", "operation", "get_sql_db", "error", err)
		return err
	}
	if err := RunMigrations(sqlDB); err != nil {
		slog.Error("Failed to run database migrations", "component", "db", "operation", "migrate", "error", err)
		return err
	}

	// Ensure default preferences exist with strictly safe defaults
	var pref PreferenceSet
	if err := db.FirstOrCreate(&pref, PreferenceSet{
		ID:                    1,
		ExecutionMode:         "dry-run",
		LogLevel:              "info",
		AuditLogRetentionDays: 30,
		PollIntervalSeconds:   300,
		WatchHistoryWeight:    10,
		LastWatchedWeight:     8,
		FileSizeWeight:        6,
		RatingWeight:          5,
		TimeInLibraryWeight:   4,
		SeriesStatusWeight:    3,
		TiebreakerMethod:      "size_desc",
		DeletionsEnabled:      true,
		SnoozeDurationHours:   24,
		CheckForUpdates:       true,
	}).Error; err != nil {
		slog.Error("Failed to seed default preferences", "component", "db", "operation", "seed_preferences", "error", err)
	}

	// Apply dynamic log level from preferences
	logger.SetLevel(pref.LogLevel)

	slog.Info("Database initialized successfully", "component", "db", "path", cfg.Database)
	DB = db
	return nil
}
