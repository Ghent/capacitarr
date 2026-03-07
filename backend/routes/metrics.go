package routes

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/services"
)

// RegisterMetricsRoutes registers metrics and statistics endpoints on the protected group.
func RegisterMetricsRoutes(g *echo.Group, reg *services.Registry) {
	// Metrics History
	g.GET("/metrics/history", func(c echo.Context) error {
		resolution := c.QueryParam("resolution")
		diskGroupID := c.QueryParam("disk_group_id")
		since := c.QueryParam("since")

		history, err := reg.Metrics.GetHistory(resolution, diskGroupID, since)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error fetching metrics"})
		}

		return c.JSON(http.StatusOK, map[string]interface{}{"status": "success", "data": history})
	})

	// Worker Stats
	g.GET("/worker/stats", func(c echo.Context) error {
		return c.JSON(http.StatusOK, reg.Metrics.GetWorkerMetrics())
	})

	// Lifetime stats (cumulative counters, not cleared by data reset)
	g.GET("/lifetime-stats", func(c echo.Context) error {
		stats, err := reg.Metrics.GetLifetimeStats()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch lifetime stats"})
		}
		return c.JSON(http.StatusOK, stats)
	})

	// Dashboard stats (aggregates lifetime stats, protected count, library growth rate)
	g.GET("/dashboard-stats", func(c echo.Context) error {
		stats, err := reg.Metrics.GetDashboardStats()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch dashboard stats"})
		}
		return c.JSON(http.StatusOK, stats)
	})
}
