package routes

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/services"
)

// parseDiskGroupID extracts an optional disk_group_id query parameter.
// Returns nil if not provided or invalid.
func parseDiskGroupID(c echo.Context) *uint {
	if dgStr := c.QueryParam("disk_group_id"); dgStr != "" {
		if parsed, err := strconv.ParseUint(dgStr, 10, 64); err == nil {
			dgID := uint(parsed)
			return &dgID
		}
	}
	return nil
}

// RegisterAnalyticsRoutes registers all analytics-related endpoints.
func RegisterAnalyticsRoutes(g *echo.Group, reg *services.Registry) {
	g.GET("/analytics/dead-content", analyticsDeadContentHandler(reg))
	g.GET("/analytics/stale-content", analyticsStaleContentHandler(reg))
	g.GET("/analytics/forecast", analyticsForecastHandler(reg))
}

func analyticsDeadContentHandler(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		minDays := 90
		if v := c.QueryParam("minDays"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
				minDays = parsed
			}
		}
		data := reg.WatchAnalytics.GetDeadContent(minDays, parseDiskGroupID(c))
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
		data := reg.WatchAnalytics.GetStaleContent(staleDays, parseDiskGroupID(c))
		return c.JSON(http.StatusOK, data)
	}
}

// analyticsForecastHandler returns capacity forecast data based on linear
// regression of recent usage history. Accepts an optional disk_group_id
// query parameter; defaults to the most degraded (highest usage %) disk group.
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

		// Determine which disk group to use for forecast
		var group *services.DiskGroupForForecast
		if dgID := parseDiskGroupID(c); dgID != nil {
			// Use the specified disk group
			for i := range groups {
				if groups[i].ID == *dgID {
					eff := groups[i].EffectiveTotalBytes()
					group = &services.DiskGroupForForecast{
						ID:            groups[i].ID,
						ThresholdPct:  groups[i].ThresholdPct,
						TotalCapacity: eff,
						UsedCapacity:  groups[i].UsedBytes,
					}
					break
				}
			}
			if group == nil {
				return apiError(c, http.StatusNotFound, "Disk group not found")
			}
		} else {
			// Default to the most degraded group (highest usage percentage)
			bestIdx := 0
			bestPct := 0.0
			for i, g := range groups {
				eff := g.EffectiveTotalBytes()
				if eff > 0 {
					pct := float64(g.UsedBytes) / float64(eff) * 100
					if pct > bestPct {
						bestPct = pct
						bestIdx = i
					}
				}
			}
			eff := groups[bestIdx].EffectiveTotalBytes()
			group = &services.DiskGroupForForecast{
				ID:            groups[bestIdx].ID,
				ThresholdPct:  groups[bestIdx].ThresholdPct,
				TotalCapacity: eff,
				UsedCapacity:  groups[bestIdx].UsedBytes,
			}
		}

		forecast, err := reg.Metrics.GetCapacityForecast(group.ThresholdPct, group.TotalCapacity, group.UsedCapacity)
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to compute forecast")
		}

		return c.JSON(http.StatusOK, forecast)
	}
}
