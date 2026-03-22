package routes

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/engine"
	"capacitarr/internal/services"
)

// factorWeightResponse is the API response for a single scoring factor weight,
// enriched with metadata from the engine's factor registry.
type factorWeightResponse struct {
	Key           string `json:"key"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Weight        int    `json:"weight"`
	DefaultWeight int    `json:"defaultWeight"`
}

// RegisterFactorWeightRoutes sets up the endpoints for managing scoring factor weights.
func RegisterFactorWeightRoutes(protected *echo.Group, reg *services.Registry) {
	// Build a factor metadata lookup from the engine's default factors.
	// This runs once at route registration time — factor list is static.
	factorMeta := make(map[string]engine.ScoringFactor)
	for _, f := range engine.DefaultFactors() {
		factorMeta[f.Key()] = f
	}

	// GET /api/v1/scoring-factor-weights — list all factors with current weights + metadata
	protected.GET("/scoring-factor-weights", func(c echo.Context) error {
		dbWeights, err := reg.Settings.ListFactorWeights()
		if err != nil {
			slog.Error("Failed to fetch scoring factor weights",
				"component", "api", "operation", "list_factor_weights", "error", err)
			return apiError(c, http.StatusInternalServerError, "Failed to fetch scoring factor weights")
		}

		// Build ordered response: use the engine's DefaultFactors() order,
		// enriching each with the DB weight. Factors in the DB but not in the
		// registry are appended at the end (shouldn't happen, but defensive).
		seen := make(map[string]bool)
		resp := make([]factorWeightResponse, 0, len(dbWeights))

		// First pass: factors in engine registry order
		for _, f := range engine.DefaultFactors() {
			w := f.DefaultWeight()
			for _, dbw := range dbWeights {
				if dbw.FactorKey == f.Key() {
					w = dbw.Weight
					break
				}
			}
			resp = append(resp, factorWeightResponse{
				Key:           f.Key(),
				Name:          f.Name(),
				Description:   f.Description(),
				Weight:        w,
				DefaultWeight: f.DefaultWeight(),
			})
			seen[f.Key()] = true
		}

		// Second pass: orphan DB rows (no matching factor — defensive only)
		for _, dbw := range dbWeights {
			if !seen[dbw.FactorKey] {
				resp = append(resp, factorWeightResponse{
					Key:           dbw.FactorKey,
					Name:          dbw.FactorKey,
					Description:   "",
					Weight:        dbw.Weight,
					DefaultWeight: 5,
				})
			}
		}

		return c.JSON(http.StatusOK, resp)
	})

	// PUT /api/v1/scoring-factor-weights — update weights (accepts map[string]int)
	protected.PUT("/scoring-factor-weights", func(c echo.Context) error {
		var payload map[string]int
		if err := c.Bind(&payload); err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid request payload — expected {\"factor_key\": weight, ...}")
		}

		// Validate all keys exist in the factor registry
		for key := range payload {
			if _, ok := factorMeta[key]; !ok {
				return apiError(c, http.StatusBadRequest, "Unknown scoring factor key: "+key)
			}
		}

		// Validate weight values (0-10)
		for key, w := range payload {
			if w < 0 || w > 10 {
				return apiError(c, http.StatusBadRequest, "Weight for "+key+" must be between 0 and 10")
			}
		}

		if err := reg.Settings.UpdateFactorWeights(payload); err != nil {
			slog.Error("Failed to update scoring factor weights",
				"component", "api", "operation", "update_factor_weights", "error", err)
			return apiError(c, http.StatusInternalServerError, "Failed to update scoring factor weights")
		}

		// Return the updated list
		dbWeights, err := reg.Settings.ListFactorWeights()
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Weights saved but failed to reload")
		}

		resp := make([]factorWeightResponse, 0, len(dbWeights))
		for _, f := range engine.DefaultFactors() {
			w := f.DefaultWeight()
			for _, dbw := range dbWeights {
				if dbw.FactorKey == f.Key() {
					w = dbw.Weight
					break
				}
			}
			resp = append(resp, factorWeightResponse{
				Key:           f.Key(),
				Name:          f.Name(),
				Description:   f.Description(),
				Weight:        w,
				DefaultWeight: f.DefaultWeight(),
			})
		}

		return c.JSON(http.StatusOK, resp)
	})
}
