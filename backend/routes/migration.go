package routes

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/services"
)

// RegisterMigrationRoutes sets up the 1.x → 2.0 migration endpoints.
// The status endpoint is public (pre-auth) so the frontend can detect
// a pending migration before the user navigates away from login.
// The execute and dismiss endpoints are protected — auth is auto-imported
// from the 1.x backup at startup so the user can log in first.
func RegisterMigrationRoutes(public *echo.Group, protected *echo.Group, reg *services.Registry) {
	// GET /migration/status — public, used by the login page to detect pending migration
	public.GET("/migration/status", func(c echo.Context) error {
		status := reg.Migration.Status()
		return c.JSON(http.StatusOK, status)
	})

	// POST /migration/execute — protected, runs the full settings import from .v1.bak
	protected.POST("/migration/execute", func(c echo.Context) error {
		// Check availability first
		status := reg.Migration.Status()
		if !status.Available {
			return apiError(c, http.StatusConflict, "No 1.x database backup found to migrate from")
		}

		result := reg.Migration.Execute()
		if !result.Success {
			return c.JSON(http.StatusInternalServerError, result)
		}

		return c.JSON(http.StatusOK, result)
	})

	// POST /migration/dismiss — protected, removes the .v1.bak file without importing
	protected.POST("/migration/dismiss", func(c echo.Context) error {
		status := reg.Migration.Status()
		if !status.Available {
			return apiError(c, http.StatusConflict, "No 1.x database backup to dismiss")
		}

		if err := reg.Migration.Dismiss(); err != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to dismiss migration: "+err.Error())
		}

		return c.JSON(http.StatusOK, map[string]string{"message": "Migration dismissed — 1.x backup removed"})
	})
}
