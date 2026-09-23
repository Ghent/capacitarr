package events

import "fmt"

// =============================================================================
// Integration Events
// =============================================================================

// IntegrationAddedEvent is published when a new integration is created.
type IntegrationAddedEvent struct {
	IntegrationID   uint   `json:"integrationId"`
	IntegrationType string `json:"integrationType"`
	Name            string `json:"name"`
}

// EventType implements Event.
func (e IntegrationAddedEvent) EventType() string { return "integration_added" }

// EventMessage implements Event.
func (e IntegrationAddedEvent) EventMessage() string {
	return fmt.Sprintf("Integration added: %s (%s)", e.Name, e.IntegrationType)
}

// IntegrationUpdatedEvent is published when an integration is modified.
type IntegrationUpdatedEvent struct {
	IntegrationID   uint   `json:"integrationId"`
	IntegrationType string `json:"integrationType"`
	Name            string `json:"name"`
}

// EventType implements Event.
func (e IntegrationUpdatedEvent) EventType() string { return "integration_updated" }

// EventMessage implements Event.
func (e IntegrationUpdatedEvent) EventMessage() string {
	return fmt.Sprintf("Integration updated: %s (%s)", e.Name, e.IntegrationType)
}

// IntegrationRemovedEvent is published when an integration is deleted.
type IntegrationRemovedEvent struct {
	IntegrationID   uint   `json:"integrationId"`
	IntegrationType string `json:"integrationType"`
	Name            string `json:"name"`
}

// EventType implements Event.
func (e IntegrationRemovedEvent) EventType() string { return "integration_removed" }

// EventMessage implements Event.
func (e IntegrationRemovedEvent) EventMessage() string {
	return fmt.Sprintf("Integration removed: %s (%s)", e.Name, e.IntegrationType)
}

// IntegrationTestFailedEvent is published on a failed integration connection test.
type IntegrationTestFailedEvent struct {
	IntegrationType string `json:"integrationType"`
	Name            string `json:"name"`
	URL             string `json:"url"`
	Error           string `json:"error"`
}

// EventType implements Event.
func (e IntegrationTestFailedEvent) EventType() string { return "integration_test_failed" }

// EventMessage implements Event.
func (e IntegrationTestFailedEvent) EventMessage() string {
	return fmt.Sprintf("Connection test failed: %s (%s) — %s", e.Name, e.IntegrationType, e.Error)
}

// IntegrationRecoveredEvent is published when an integration transitions from
// an error state to a healthy state (lastError cleared after being non-empty).
type IntegrationRecoveredEvent struct {
	IntegrationID   uint   `json:"integrationId"`
	IntegrationType string `json:"integrationType"`
	Name            string `json:"name"`
	URL             string `json:"url"`
}

// EventType implements Event.
func (e IntegrationRecoveredEvent) EventType() string { return "integration_recovered" }

// EventMessage implements Event.
func (e IntegrationRecoveredEvent) EventMessage() string {
	return fmt.Sprintf("Integration recovered: %s (%s)", e.Name, e.IntegrationType)
}

// IntegrationRecoveryAttemptEvent is published when the IntegrationHealthService
// probes a failing integration. Fires on both success and failure so the frontend
// can show real-time recovery progress.
type IntegrationRecoveryAttemptEvent struct {
	IntegrationID    uint   `json:"integrationId"`
	IntegrationType  string `json:"integrationType"`
	Name             string `json:"name"`
	Attempt          int    `json:"attempt"`
	Success          bool   `json:"success"`
	Error            string `json:"error,omitempty"`
	NextRetrySeconds int    `json:"nextRetrySeconds,omitempty"` // Seconds until next probe (0 if recovered)
}

// EventType implements Event.
func (e IntegrationRecoveryAttemptEvent) EventType() string {
	return "integration_recovery_attempt"
}

// EventMessage implements Event.
func (e IntegrationRecoveryAttemptEvent) EventMessage() string {
	if e.Success {
		return fmt.Sprintf("Recovery probe succeeded: %s (%s) after %d attempt(s)", e.Name, e.IntegrationType, e.Attempt)
	}
	return fmt.Sprintf("Recovery probe failed: %s (%s) — attempt %d, retry in %ds", e.Name, e.IntegrationType, e.Attempt, e.NextRetrySeconds)
}
