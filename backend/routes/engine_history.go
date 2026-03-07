package routes

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/services"
)

// parseDuration parses shorthand duration strings like "1h", "24h", "7d", "30d".
func parseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}

	suffix := s[len(s)-1:]
	numStr := s[:len(s)-1]

	n, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid duration number: %s", numStr)
	}

	switch suffix {
	case "h":
		return time.Duration(n) * time.Hour, nil
	case "d":
		return time.Duration(n) * 24 * time.Hour, nil
	case "m":
		return time.Duration(n) * time.Minute, nil
	default:
		return 0, fmt.Errorf("unsupported duration suffix: %s", suffix)
	}
}

// RegisterEngineHistoryRoutes registers engine history endpoints on the protected group.
func RegisterEngineHistoryRoutes(g *echo.Group, reg *services.Registry) {
	g.GET("/engine/history", func(c echo.Context) error {
		rangeParam := c.QueryParam("range")
		if rangeParam == "" {
			rangeParam = "7d"
		}

		dur, err := parseDuration(rangeParam)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid range parameter"})
		}

		points, err := reg.Engine.GetHistory(dur)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to query engine history"})
		}

		return c.JSON(http.StatusOK, points)
	})
}
