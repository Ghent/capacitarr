package integrations

import (
	"fmt"
	"log/slog"
)

// IntegrationFactory creates a client for a given integration type.
// Each factory returns an opaque interface{} that the IntegrationRegistry
// will introspect for capability interfaces via type assertions.
type IntegrationFactory func(url, apiKey string) interface{}

// factoryRegistry maps integration type strings to their factory functions.
// Populated by RegisterFactory() calls, typically in init() or RegisterAllFactories().
var factoryRegistry = make(map[string]IntegrationFactory)

// RegisterFactory registers a factory for the given integration type.
func RegisterFactory(intType string, factory IntegrationFactory) {
	factoryRegistry[intType] = factory
}

// CreateClient creates a client for the given integration type using the
// registered factory. Returns nil if no factory is registered for the type.
func CreateClient(intType, url, apiKey string) interface{} {
	factory, ok := factoryRegistry[intType]
	if !ok {
		slog.Warn("No factory registered for integration type", "component", "factory", "type", intType)
		return nil
	}
	return factory(url, apiKey)
}

// RegisteredTypes returns all registered integration type strings.
func RegisteredTypes() []string {
	types := make([]string, 0, len(factoryRegistry))
	for t := range factoryRegistry {
		types = append(types, t)
	}
	return types
}

// HasFactory returns true if a factory is registered for the given type.
func HasFactory(intType string) bool {
	_, ok := factoryRegistry[intType]
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
		"count", len(factoryRegistry), "types", fmt.Sprintf("%v", RegisteredTypes()))
}
