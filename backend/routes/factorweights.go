package routes

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/engine"
	"capacitarr/internal/integrations"
	"capacitarr/internal/services"
)

// factorWeightResponse is the API response for a single scoring factor weight,
// enriched with metadata from the engine's factor registry.
type factorWeightResponse struct {
	Key              string `json:"key"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Weight           int    `json:"weight"`
	DefaultWeight    int    `json:"defaultWeight"`
	IntegrationError bool   `json:"integrationError,omitempty"` // true when the required integration has a non-empty LastError
}

// RegisterFactorWeightRoutes sets up the endpoints for managing scoring factor weights.
func RegisterFactorWeightRoutes(protected *echo.Group, reg *services.Registry) {
	// Build a factor metadata lookup from the engine's default factors.
	// This runs once at route registration time — factor list is static.
	factorMeta := make(map[string]engine.ScoringFactor)
	for _, f := range engine.DefaultFactors() {
		factorMeta[f.Key()] = f
	}

	// integrationState holds the active types and which types have errors.
	type integrationState struct {
		active   map[integrations.IntegrationType]bool
		erroring map[integrations.IntegrationType]bool
	}

	// getIntegrationState queries enabled integrations and returns which types
	// are active and which have a non-empty LastError.
	getIntegrationState := func() integrationState {
		configs, err := reg.Integration.ListEnabled()
		if err != nil {
			slog.Error("Failed to list enabled integrations for factor filtering",
				"component", "api", "error", err)
			return integrationState{
				active:   make(map[integrations.IntegrationType]bool),
				erroring: make(map[integrations.IntegrationType]bool),
			}
		}
		active := make(map[integrations.IntegrationType]bool, len(configs))
		erroring := make(map[integrations.IntegrationType]bool)
		for _, cfg := range configs {
			t := integrations.IntegrationType(cfg.Type)
			active[t] = true
			if cfg.LastError != "" {
				erroring[t] = true
			}
		}
		return integrationState{active: active, erroring: erroring}
	}

	// isFactorApplicableForAPI checks whether a factor should be exposed in the
	// API based on RequiresIntegration. MediaTypeScoped is NOT checked here —
	// it's a per-item runtime check, so e.g. Show Status is visible when Sonarr
	// is configured even though it won't apply to Radarr items at scoring time.
	isFactorApplicableForAPI := func(f engine.ScoringFactor, state integrationState) bool {
		if ri, ok := f.(engine.RequiresIntegration); ok {
			return state.active[ri.RequiredIntegrationType()]
		}
		return true
	}

	// hasIntegrationError returns true if the factor's required integration
	// type has a non-empty LastError on any enabled instance.
	hasIntegrationError := func(f engine.ScoringFactor, state integrationState) bool {
		if ri, ok := f.(engine.RequiresIntegration); ok {
			return state.erroring[ri.RequiredIntegrationType()]
		}
		return false
	}

	// GET /api/v1/scoring-factor-weights — list applicable factors with current weights + metadata
	protected.GET("/scoring-factor-weights", func(c echo.Context) error {
		dbWeights, err := reg.Settings.ListFactorWeights()
		if err != nil {
			slog.Error("Failed to fetch scoring factor weights",
				"component", "api", "operation", "list_factor_weights", "error", err)
			return apiError(c, http.StatusInternalServerError, "Failed to fetch scoring factor weights")
		}

		state := getIntegrationState()

		// Build ordered response: use the engine's DefaultFactors() order,
		// enriching each with the DB weight. Only include factors whose
		// RequiresIntegration dependency is met.
		knownKeys := make(map[string]bool)
		resp := make([]factorWeightResponse, 0, len(dbWeights))

		// First pass: applicable factors in engine registry order.
		// Mark ALL known factor keys regardless of applicability so the
		// orphan pass doesn't leak inapplicable factors.
		for _, f := range engine.DefaultFactors() {
			knownKeys[f.Key()] = true
			if !isFactorApplicableForAPI(f, state) {
				continue
			}
			w := f.DefaultWeight()
			for _, dbw := range dbWeights {
				if dbw.FactorKey == f.Key() {
					w = dbw.Weight
					break
				}
			}
			resp = append(resp, factorWeightResponse{
				Key:              f.Key(),
				Name:             f.Name(),
				Description:      f.Description(),
				Weight:           w,
				DefaultWeight:    f.DefaultWeight(),
				IntegrationError: hasIntegrationError(f, state),
			})
		}

		// Second pass: truly orphan DB rows (key not in any engine factor — defensive only)
		for _, dbw := range dbWeights {
			if !knownKeys[dbw.FactorKey] {
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

		// Return the updated list (filtered by applicability)
		dbWeights, err := reg.Settings.ListFactorWeights()
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Weights saved but failed to reload")
		}

		state := getIntegrationState()

		resp := make([]factorWeightResponse, 0, len(dbWeights))
		for _, f := range engine.DefaultFactors() {
			if !isFactorApplicableForAPI(f, state) {
				continue
			}
			w := f.DefaultWeight()
			for _, dbw := range dbWeights {
				if dbw.FactorKey == f.Key() {
					w = dbw.Weight
					break
				}
			}
			resp = append(resp, factorWeightResponse{
				Key:              f.Key(),
				Name:             f.Name(),
				Description:      f.Description(),
				Weight:           w,
				DefaultWeight:    f.DefaultWeight(),
				IntegrationError: hasIntegrationError(f, state),
			})
		}

		return c.JSON(http.StatusOK, resp)
	})
}
