package integrations

import (
	"fmt"
	"log/slog"
	"sync"
)

// IntegrationFactory creates a client for a given integration type.
// Each factory returns an opaque interface{} that the IntegrationRegistry
// will introspect for capability interfaces via type assertions.
type IntegrationFactory func(url, apiKey string) interface{}

// factoryRegistry maps integration type strings to their factory functions.
// Uses sync.Map for safe concurrent reads during test parallelism.
// Populated by RegisterFactory() calls, typically in init() or RegisterAllFactories().
var factoryRegistry sync.Map

// RegisterFactory registers a factory for the given integration type.
func RegisterFactory(intType string, factory IntegrationFactory) {
	factoryRegistry.Store(intType, factory)
}

// CreateClient creates a client for the given integration type using the
// registered factory. Returns nil if no factory is registered for the type.
func CreateClient(intType, url, apiKey string) interface{} {
	val, ok := factoryRegistry.Load(intType)
	if !ok {
		slog.Warn("No factory registered for integration type", "component", "factory", "type", intType)
		return nil
	}
	factory := val.(IntegrationFactory)
	return factory(url, apiKey)
}

// RegisteredTypes returns all registered integration type strings.
func RegisteredTypes() []string {
	var types []string
	factoryRegistry.Range(func(key, _ any) bool {
		types = append(types, key.(string))
		return true
	})
	return types
}

// HasFactory returns true if a factory is registered for the given type.
func HasFactory(intType string) bool {
	_, ok := factoryRegistry.Load(intType)
	return ok
}

// RegisterAllFactories registers factories for all built-in integration types.
// Called once during application startup.
func RegisterAllFactories() {
	RegisterFactory(string(IntegrationTypeSonarr), func(url, apiKey string) interface{} {
		return NewSonarrClient(url, apiKey)
	})
	RegisterFactory(string(IntegrationTypeRadarr), func(url, apiKey string) interface{} {
		return NewRadarrClient(url, apiKey)
	})
	RegisterFactory(string(IntegrationTypeLidarr), func(url, apiKey string) interface{} {
		return NewLidarrClient(url, apiKey)
	})
	RegisterFactory(string(IntegrationTypeReadarr), func(url, apiKey string) interface{} {
		return NewReadarrClient(url, apiKey)
	})
	RegisterFactory(string(IntegrationTypePlex), func(url, apiKey string) interface{} {
		return NewPlexClient(url, apiKey)
	})
	RegisterFactory(string(IntegrationTypeTautulli), func(url, apiKey string) interface{} {
		return NewTautulliClient(url, apiKey)
	})
	RegisterFactory(string(IntegrationTypeSeerr), func(url, apiKey string) interface{} {
		return NewSeerrClient(url, apiKey)
	})
	RegisterFactory(string(IntegrationTypeJellyfin), func(url, apiKey string) interface{} {
		return NewJellyfinClient(url, apiKey)
	})
	RegisterFactory(string(IntegrationTypeEmby), func(url, apiKey string) interface{} {
		return NewEmbyClient(url, apiKey)
	})
	RegisterFactory(string(IntegrationTypeJellystat), func(url, apiKey string) interface{} {
		return NewJellystatClient(url, apiKey)
	})
	RegisterFactory(string(IntegrationTypeTracearr), func(url, apiKey string) interface{} {
		return NewTracearrClient(url, apiKey)
	})

	slog.Debug("All integration factories registered", "component", "factory",
		"count", len(RegisteredTypes()), "types", fmt.Sprintf("%v", RegisteredTypes()))
}

// ValidateFactories verifies that every defined IntegrationTypeXxx constant
// has a registered factory, and vice versa. Called at startup after
// RegisterAllFactories() to catch missing registrations at boot time rather
// than at runtime when a user tries to create an integration.
//
// The allTypes parameter should be the keys of db.ValidIntegrationTypes.
// It is passed as a parameter rather than imported directly to avoid a
// circular import (integrations → db → integrations).
func ValidateFactories(allTypes []string) error {
	// Every valid type must have a factory
	for _, intType := range allTypes {
		if !HasFactory(intType) {
			return fmt.Errorf("integration type %q has no registered factory", intType)
		}
	}

	// Every registered factory must be in the valid types list
	validSet := make(map[string]bool, len(allTypes))
	for _, t := range allTypes {
		validSet[t] = true
	}
	for _, regType := range RegisteredTypes() {
		if !validSet[regType] {
			return fmt.Errorf("factory registered for unknown integration type %q", regType)
		}
	}

	return nil
}
