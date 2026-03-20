package routes

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/services"
)

// RegisterAnalyticsRoutes registers all analytics-related endpoints.
func RegisterAnalyticsRoutes(g *echo.Group, reg *services.Registry) {
	g.GET("/analytics/quality", analyticsQualityHandler(reg))
	g.GET("/analytics/bloat", analyticsBloatHandler(reg))
	g.GET("/analytics/dead-content", analyticsDeadContentHandler(reg))
	g.GET("/analytics/stale-content", analyticsStaleContentHandler(reg))
	g.GET("/analytics/forecast", analyticsForecastHandler(reg))
	g.GET("/analytics/storage-breakdown", analyticsStorageBreakdownHandler(reg))
	g.GET("/analytics/status-breakdown", analyticsStatusBreakdownHandler(reg))
}

func analyticsQualityHandler(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		data := reg.Analytics.GetQualityDistribution()
		return c.JSON(http.StatusOK, data)
	}
}

func analyticsBloatHandler(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		data := reg.Analytics.GetSizeAnomalies()
		return c.JSON(http.StatusOK, data)
	}
}

func analyticsDeadContentHandler(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		minDays := 90
		if v := c.QueryParam("minDays"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
				minDays = parsed
			}
		}
		data := reg.WatchAnalytics.GetDeadContent(minDays)
		return c.JSON(http.StatusOK, data)
	}
}

func analyticsStaleContentHandler(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		staleDays := 180
		if v := c.QueryParam("staleDays"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
				staleDays = parsed
			}
		}
		data := reg.WatchAnalytics.GetStaleContent(staleDays)
		return c.JSON(http.StatusOK, data)
	}
}

// analyticsForecastHandler returns capacity forecast data based on linear
// regression of recent usage history. Uses the first disk group's settings
// for threshold and capacity values.
func analyticsForecastHandler(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		groups, err := reg.DiskGroup.List()
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to list disk groups")
		}

		if len(groups) == 0 {
			// No disk groups configured — return empty forecast
			return c.JSON(http.StatusOK, &services.CapacityForecast{
				DaysUntilThreshold: -1,
				DaysUntilFull:      -1,
			})
		}

		// Use the first disk group for threshold and capacity
		group := groups[0]
		totalCapacity := group.EffectiveTotalBytes()
		usedCapacity := group.UsedBytes

		forecast, err := reg.Metrics.GetCapacityForecast(group.ThresholdPct, totalCapacity, usedCapacity)
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to compute forecast")
		}

		return c.JSON(http.StatusOK, forecast)
	}
}

// analyticsStorageBreakdownHandler returns hierarchical storage data for the
// sunburst chart: media type → quality profile → size.
func analyticsStorageBreakdownHandler(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		data := reg.Analytics.GetStorageSunburst()
		return c.JSON(http.StatusOK, data)
	}
}

// analyticsStatusBreakdownHandler returns library items classified by lifecycle
// status (dead, stale, protected, active), grouped by media type within each bucket.
func analyticsStatusBreakdownHandler(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		data := reg.WatchAnalytics.GetLibraryStatusBreakdown()
		return c.JSON(http.StatusOK, data)
	}
}
