package db_test

import (
	"testing"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)

// TestValidIntegrationTypes_MatchesFactoryRegistry verifies that the
// ValidIntegrationTypes validation map stays in sync with the integration
// factory registry. Every factory-registered type must be in the validation
// map and vice versa — a drift means users can't create a valid integration
// type through the API (like the jellystat bug) or validation accepts a type
// that has no factory.
func TestValidIntegrationTypes_MatchesFactoryRegistry(t *testing.T) {
	// Ensure all factories are registered (idempotent; safe to call multiple times)
	integrations.RegisterAllFactories()

	registeredTypes := integrations.RegisteredTypes()

	// Every factory-registered type must be in ValidIntegrationTypes
	for _, intType := range registeredTypes {
		if !db.ValidIntegrationTypes[intType] {
			t.Errorf("integration type %q is registered in the factory but missing from db.ValidIntegrationTypes", intType)
		}
	}

	// Every ValidIntegrationTypes entry must have a factory
	for intType := range db.ValidIntegrationTypes {
		if !integrations.HasFactory(intType) {
			t.Errorf("integration type %q is in db.ValidIntegrationTypes but has no registered factory", intType)
		}
	}

	// Counts must match (catches duplicates or other anomalies)
	if len(db.ValidIntegrationTypes) != len(registeredTypes) {
		t.Errorf("ValidIntegrationTypes has %d entries but factory registry has %d", len(db.ValidIntegrationTypes), len(registeredTypes))
	}
}
