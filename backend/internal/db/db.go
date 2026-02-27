package db

import (
	"log/slog"

	"github.com/capacitarr/capacitarr/backend/internal/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init(cfg *config.Config) error {
	logLevel := gormlogger.Warn
	if cfg.Debug {
		logLevel = gormlogger.Info
	}

	db, err := gorm.Open(sqlite.Open(cfg.Database), &gorm.Config{
		Logger: gormlogger.Default.LogMode(logLevel),
	})
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		return err
	}

	// Run migrations
	err = db.AutoMigrate(&AuthConfig{}, &LibraryHistory{})
	if err != nil {
		slog.Error("Failed to migrate database", "error", err)
		return err
	}

	slog.Info("Database initialized successfully", "path", cfg.Database)
	DB = db
	return nil
}
