package routes

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/services"
)

// RegisterMigrationRoutes sets up the 1.x → 2.0 migration endpoints.
// The status endpoint is public (pre-auth) so the frontend can detect a
// 1.x database before the user logs in. The execute endpoint is protected.
func RegisterMigrationRoutes(public *echo.Group, protected *echo.Group, reg *services.Registry) {
	// GET /migration/status — public, used by the migration page to detect 1.x DB
	public.GET("/migration/status", func(c echo.Context) error {
		status := reg.Migration.Status()
		return c.JSON(http.StatusOK, status)
	})

	// POST /migration/execute — protected, runs the actual migration
	protected.POST("/migration/execute", func(c echo.Context) error {
		// Check availability first
		status := reg.Migration.Status()
		if !status.Available {
			return apiError(c, http.StatusConflict, "No 1.x database found to migrate from")
		}

		result := reg.Migration.Execute()
		if !result.Success {
			return c.JSON(http.StatusInternalServerError, result)
		}

		return c.JSON(http.StatusOK, result)
	})
}
