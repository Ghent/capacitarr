package services

import (
	"fmt"
	"log/slog"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/integrations"
)

// FactorWeightResponse is the API response for a single scoring factor weight,
// enriched with metadata from the engine's factor registry.
type FactorWeightResponse struct {
	Key              string `json:"key"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Weight           int    `json:"weight"`
	DefaultWeight    int    `json:"defaultWeight"`
	IntegrationError bool   `json:"integrationError,omitempty"` // true when the required integration has a non-empty LastError
}

type factorIntegrationState struct {
	active   map[integrations.IntegrationType]bool
	erroring map[integrations.IntegrationType]bool
}

// ValidateFactorWeightPayload checks that every key is a known scoring factor
// and every weight is in 0–10. The returned error message is safe to send to
// API clients.
func (s *SettingsService) ValidateFactorWeightPayload(payload map[string]int) error {
	known := make(map[string]struct{}, 16)
	for _, f := range engine.DefaultFactors() {
		known[f.Key()] = struct{}{}
	}
	for key, w := range payload {
		if _, ok := known[key]; !ok {
			return fmt.Errorf("unknown scoring factor key: %s", key)
		}
		if w < 0 || w > 10 {
			return fmt.Errorf("weight for %s must be between 0 and 10", key)
		}
	}
	return nil
}

// ListFactorWeightsForAPI returns applicable scoring factors with current
// weights and registry metadata. enabled is the result of listing integrations;
// listErr is that list call's error (nil when enabled is valid). The JSON
// contract matches the previous route-layer DTO.
func (s *SettingsService) ListFactorWeightsForAPI(enabled []db.IntegrationConfig, listErr error) ([]FactorWeightResponse, error) {
	dbWeights, err := s.ListFactorWeights()
	if err != nil {
		return nil, err
	}

	if listErr != nil {
		slog.Error("Failed to list enabled integrations for factor filtering",
			"component", "settings", "error", listErr)
		enabled = nil
	}

	state := buildFactorIntegrationState(enabled)

	knownKeys := make(map[string]bool)
	resp := make([]FactorWeightResponse, 0, len(dbWeights))

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
		resp = append(resp, FactorWeightResponse{
			Key:              f.Key(),
			Name:             f.Name(),
			Description:      f.Description(),
			Weight:           w,
			DefaultWeight:    f.DefaultWeight(),
			IntegrationError: hasIntegrationError(f, state),
		})
	}

	for _, dbw := range dbWeights {
		if !knownKeys[dbw.FactorKey] {
			resp = append(resp, FactorWeightResponse{
				Key:           dbw.FactorKey,
				Name:          dbw.FactorKey,
				Description:   "",
				Weight:        dbw.Weight,
				DefaultWeight: 5,
			})
		}
	}

	return resp, nil
}

func buildFactorIntegrationState(configs []db.IntegrationConfig) factorIntegrationState {
	active := make(map[integrations.IntegrationType]bool, len(configs))
	erroring := make(map[integrations.IntegrationType]bool)
	for _, cfg := range configs {
		t := integrations.IntegrationType(cfg.Type)
		active[t] = true
		if cfg.LastError != "" {
			erroring[t] = true
		}
	}
	return factorIntegrationState{active: active, erroring: erroring}
}

func isFactorApplicableForAPI(f engine.ScoringFactor, state factorIntegrationState) bool {
	if ri, ok := f.(engine.RequiresIntegration); ok {
		return state.active[ri.RequiredIntegrationType()]
	}
	return true
}

func hasIntegrationError(f engine.ScoringFactor, state factorIntegrationState) bool {
	if ri, ok := f.(engine.RequiresIntegration); ok {
		return state.erroring[ri.RequiredIntegrationType()]
	}
	if rai, ok := f.(engine.RequiresAnyIntegration); ok {
		anyConfigured := false
		anyHealthy := false
		for _, t := range rai.RequiredIntegrationTypes() {
			if state.active[t] {
				anyConfigured = true
				if !state.erroring[t] {
					anyHealthy = true
					break
				}
			}
		}
		return anyConfigured && !anyHealthy
	}
	return false
}
