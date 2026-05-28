package services

import (
	"fmt"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)

// clientResolverAdapter implements ClientResolver by wrapping IntegrationService
// and the integrations.CreateClient factory. Integration clients are stateless
// HTTP wrappers (struct with URL + API key + *http.Client), so no caching is
// needed — creation is cheap.
type clientResolverAdapter struct {
	integrations *IntegrationService
}

// NewClientResolver creates a ClientResolver backed by IntegrationService.
func NewClientResolver(integrations *IntegrationService) ClientResolver {
	return &clientResolverAdapter{integrations: integrations}
}

// GetDeleter looks up an integration by ID and creates a MediaDeleter client.
func (r *clientResolverAdapter) GetDeleter(integrationID uint) (integrations.MediaDeleter, error) {
	config, err := r.integrations.GetByID(integrationID)
	if err != nil {
		return nil, fmt.Errorf("integration %d not found: %w", integrationID, err)
	}

	rawClient := integrations.CreateClient(config.Type, config.URL, config.APIKey)
	if rawClient == nil {
		return nil, fmt.Errorf("unsupported integration type %q for integration %d", config.Type, integrationID)
	}

	deleter, ok := rawClient.(integrations.MediaDeleter)
	if !ok {
		return nil, fmt.Errorf("integration %d (type %q) does not support deletion", integrationID, config.Type)
	}

	return deleter, nil
}

// GetIntegrationConfig returns the full integration config for a given ID.
func (r *clientResolverAdapter) GetIntegrationConfig(integrationID uint) (*db.IntegrationConfig, error) {
	return r.integrations.GetByID(integrationID)
}
