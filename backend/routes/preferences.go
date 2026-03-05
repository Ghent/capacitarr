package routes

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/logger"
)

// RegisterPreferenceRoutes sets up the endpoints for managing the PreferenceSet singleton.
func RegisterPreferenceRoutes(protected *echo.Group, database *gorm.DB) {
	protected.GET("/preferences", func(c echo.Context) error {
		var pref db.PreferenceSet
		// Always return the first/only record, or implicitly create default
		if err := database.FirstOrCreate(&pref, db.PreferenceSet{ID: 1}).Error; err != nil {
			slog.Error("Failed to fetch preferences", "component", "api", "operation", "fetch_preferences", "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch preferences"})
		}
		return c.JSON(http.StatusOK, pref)
	})

	protected.PUT("/preferences", func(c echo.Context) error {
		var payload db.PreferenceSet
		if err := c.Bind(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
		}
		// Force ID to 1 to ensure a single singleton record
		payload.ID = 1

		// Snapshot current execution mode for change detection
		var oldPrefs db.PreferenceSet
		database.FirstOrCreate(&oldPrefs, db.PreferenceSet{ID: 1})

		// Validate weight values (0-10)
		weights := []int{
			payload.WatchHistoryWeight, payload.LastWatchedWeight,
			payload.FileSizeWeight, payload.RatingWeight,
			payload.TimeInLibraryWeight, payload.SeriesStatusWeight,
		}
		for _, w := range weights {
			if w < 0 || w > 10 {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "Weight values must be between 0 and 10"})
			}
		}

		// Validate tiebreaker method
		validTiebreakers := map[string]bool{"size_desc": true, "size_asc": true, "name_asc": true, "oldest_first": true, "newest_first": true}
		if payload.TiebreakerMethod == "" {
			payload.TiebreakerMethod = "size_desc"
		}
		if !validTiebreakers[payload.TiebreakerMethod] {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Tiebreaker method must be size_desc, size_asc, name_asc, oldest_first, or newest_first"})
		}

		// Validate execution mode
		validModes := map[string]bool{"dry-run": true, "approval": true, "auto": true}
		if !validModes[payload.ExecutionMode] {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Execution mode must be dry-run, approval, or auto"})
		}

		// Validate log level
		validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
		if !validLevels[payload.LogLevel] {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Log level must be debug, info, warn, or error"})
		}

		// Validate poll interval (minimum 30s, default 300s)
		if payload.PollIntervalSeconds < 30 {
			payload.PollIntervalSeconds = 300
		}

		if err := database.Save(&payload).Error; err != nil {
			slog.Error("Failed to update preferences", "component", "api", "operation", "update_preferences", "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update preferences"})
		}

		// Apply dynamic log level
		logger.SetLevel(payload.LogLevel)

		// Log engine mode change if execution_mode was modified
		if oldPrefs.ExecutionMode != payload.ExecutionMode {
			db.LogActivity(database, db.EventEngineModeChanged, fmt.Sprintf("Engine mode changed from %s to %s", oldPrefs.ExecutionMode, payload.ExecutionMode))
		}

		// Log settings change activity event
		db.LogActivity(database, db.EventSettingsChanged, "Settings updated")

		return c.JSON(http.StatusOK, payload)
	})
}
