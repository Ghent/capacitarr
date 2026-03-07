package routes

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/services"
)

// RegisterDataRoutes registers data management endpoints on the protected group.
func RegisterDataRoutes(g *echo.Group, reg *services.Registry) {
	g.DELETE("/data/reset", handleDataReset(reg))
}

func handleDataReset(reg *services.Registry) echo.HandlerFunc {
	return func(c echo.Context) error {
		summary, err := reg.Data.Reset()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		return c.JSON(http.StatusOK, map[string]any{
			"status":  "success",
			"message": "All scraped data has been cleared",
			"cleared": summary,
		})
	}
}
