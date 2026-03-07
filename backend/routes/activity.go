package routes

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/services"
)

// RegisterActivityRoutes sets up the API endpoints for activity events.
func RegisterActivityRoutes(g *echo.Group, reg *services.Registry) {
	// Recent activity events (system events only)
	g.GET("/activity/recent", func(c echo.Context) error {
		limit := 5
		if l := c.QueryParam("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		if limit > 100 {
			limit = 100
		}

		// Decision: Activity reads go through SettingsService rather than creating
		// a dedicated ActivityService, since activity events are a lightweight
		// operational concern (7-day retention, no business logic).
		activities, err := reg.Settings.ListRecentActivities(limit)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch recent activity events"})
		}

		return c.JSON(http.StatusOK, activities)
	})
}
