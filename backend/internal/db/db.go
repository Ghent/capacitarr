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

// FactorDefault describes a scoring factor's key and default weight for seeding.
// Defined here to avoid importing the engine package from db (which would create
// a circular dependency: db → engine → db). The engine package provides its own
// DefaultFactors() which returns richer types; main.go bridges the two.
type FactorDefault struct {
	Key           string
	DefaultWeight int
}

// Init opens the SQLite database, runs migrations, and returns the connection.
func Init(cfg *config.Config) (*gorm.DB, error) {
	logLevel := gormlogger.Warn
	if cfg.Debug {
		logLevel = gormlogger.Info
	}

	database, err := gorm.Open(gormlite.Open(cfg.Database), &gorm.Config{
		Logger: gormlogger.Default.LogMode(logLevel),
	})
	if err != nil {
		slog.Error("Failed to connect to database", "component", "db", "operation", "connect", "error", err)
		return nil, err
	}

	// Run Goose migrations (sole schema management — no AutoMigrate)
	sqlDB, err := database.DB()
	if err != nil {
		slog.Error("Failed to get underlying sql.DB for migrations", "component", "db", "operation", "get_sql_db", "error", err)
		return nil, err
	}
	if err := RunMigrations(sqlDB); err != nil {
		slog.Error("Failed to run database migrations", "component", "db", "operation", "migrate", "error", err)
		return nil, err
	}

	// Ensure default preferences exist with strictly safe defaults
	var pref PreferenceSet
	if err := database.FirstOrCreate(&pref, PreferenceSet{
		ID:                    1,
		ExecutionMode:         "dry-run",
		LogLevel:              "info",
		AuditLogRetentionDays: 30,
		PollIntervalSeconds:   300,
		TiebreakerMethod:      "size_desc",
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

	slog.Info("Database initialized successfully", "component", "db", "path", cfg.Database)
	return database, nil
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
