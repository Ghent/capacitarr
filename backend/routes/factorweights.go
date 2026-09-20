package routes

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/services"
)

// RegisterFactorWeightRoutes sets up the endpoints for managing scoring factor weights.
func RegisterFactorWeightRoutes(protected *echo.Group, reg *services.Registry) {
	// GET /api/v1/scoring-factor-weights — list applicable factors with current weights + metadata
	protected.GET("/scoring-factor-weights", func(c echo.Context) error {
		enabled, listErr := reg.Integration.ListEnabled()
		resp, err := reg.Settings.ListFactorWeightsForAPI(enabled, listErr)
		if err != nil {
			slog.Error("Failed to fetch scoring factor weights",
				"component", "api", "operation", "list_factor_weights", "error", err)
			return apiError(c, http.StatusInternalServerError, "Failed to fetch scoring factor weights")
		}
		return c.JSON(http.StatusOK, resp)
	})

	// PUT /api/v1/scoring-factor-weights — update weights (accepts map[string]int)
	protected.PUT("/scoring-factor-weights", func(c echo.Context) error {
		var payload map[string]int
		if err := c.Bind(&payload); err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid request payload — expected {\"factor_key\": weight, ...}")
		}

		if err := reg.Settings.ValidateFactorWeightPayload(payload); err != nil {
			return apiError(c, http.StatusBadRequest, err.Error())
		}

		if err := reg.Settings.UpdateFactorWeights(payload); err != nil {
			slog.Error("Failed to update scoring factor weights",
				"component", "api", "operation", "update_factor_weights", "error", err)
			return apiError(c, http.StatusInternalServerError, "Failed to update scoring factor weights")
		}

		enabled, listErr := reg.Integration.ListEnabled()
		resp, err := reg.Settings.ListFactorWeightsForAPI(enabled, listErr)
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Weights saved but failed to reload")
		}

		return c.JSON(http.StatusOK, resp)
	})
}
