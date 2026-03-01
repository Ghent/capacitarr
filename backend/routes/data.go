package routes

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"capacitarr/internal/db"
)

// RegisterDataRoutes registers data management endpoints on the protected group.
func RegisterDataRoutes(g *echo.Group, database *gorm.DB) {
	g.DELETE("/data/reset", handleDataReset(database))
}

func handleDataReset(database *gorm.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		summary := map[string]int64{}

		// 1. Delete all audit_logs
		res := database.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&db.AuditLog{})
		if res.Error != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to clear audit logs: " + res.Error.Error()})
		}
		summary["auditLogs"] = res.RowsAffected

		// 2. Delete all library_histories
		res = database.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&db.LibraryHistory{})
		if res.Error != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to clear library history: " + res.Error.Error()})
		}
		summary["libraryHistories"] = res.RowsAffected

		// 3. Delete all engine_run_stats
		res = database.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&db.EngineRunStats{})
		if res.Error != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to clear engine run stats: " + res.Error.Error()})
		}
		summary["engineRunStats"] = res.RowsAffected

		// 4. Delete all disk_groups
		res = database.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&db.DiskGroup{})
		if res.Error != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to clear disk groups: " + res.Error.Error()})
		}
		summary["diskGroups"] = res.RowsAffected

		// 5. Reset transient fields on integration_configs
		res = database.Model(&db.IntegrationConfig{}).Where("1 = 1").Updates(map[string]interface{}{
			"media_size_bytes": 0,
			"media_count":     0,
			"last_sync":       nil,
			"last_error":      "",
		})
		if res.Error != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to reset integration stats: " + res.Error.Error()})
		}
		summary["integrationsReset"] = res.RowsAffected

		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":  "success",
			"message": "All scraped data has been cleared",
			"cleared": summary,
		})
	}
}
