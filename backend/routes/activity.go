package routes

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"capacitarr/internal/db"
)

// RegisterActivityRoutes sets up the API endpoints for activity events.
func RegisterActivityRoutes(g *echo.Group, database *gorm.DB) {
	// Recent activity events (system events only)
	g.GET("/activity/recent", func(c echo.Context) error {
		limit := 5
		if l := c.QueryParam("limit"); l != "" {
			if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
				limit = parsed
			}
		}
		if limit > 50 {
			limit = 50
		}

		events := make([]db.ActivityEvent, 0, limit)
		if err := database.Order("created_at desc").Limit(limit).Find(&events).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch recent activity events"})
		}

		return c.JSON(http.StatusOK, events)
	})
}
