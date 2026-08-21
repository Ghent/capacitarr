package services

import (
	"errors"
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

// --------------------------------------------------------------------------
// Linear backoff calculation
// --------------------------------------------------------------------------

func TestHealthState_NextBackoff(t *testing.T) {
	tests := []struct {
		name     string
		failures int
		want     time.Duration
	}{
		{"zero failures", 0, 15 * time.Second},
		{"first failure", 1, 15 * time.Second},
		{"second failure", 2, 30 * time.Second},
		{"third failure", 3, 45 * time.Second},
		{"fourth failure caps", 4, 60 * time.Second},
		{"fifth failure caps", 5, 60 * time.Second},
		{"tenth failure caps", 10, 60 * time.Second},
		{"huge failures caps", 100, 60 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &healthState{ConsecutiveFailures: tt.failures}
			got := s.nextBackoff()
			if got != tt.want {
				t.Errorf("nextBackoff() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --------------------------------------------------------------------------
// Seed from DB
// --------------------------------------------------------------------------

func TestHealthService_Seed(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	integrationSvc := NewIntegrationService(database, bus)

	// Create integrations — one healthy, one failing
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Healthy Sonarr", URL: "http://localhost:8989",
		APIKey: "key1", Enabled: true, LastError: "",
	})
	database.Create(&db.IntegrationConfig{
		Type: "radarr", Name: "Broken Radarr", URL: "http://localhost:7878",
		APIKey: "key2", Enabled: true, LastError: "connection refused",
		ConsecutiveFailures: 3,
	})
	// Disabled integration — should NOT be seeded
	disabledCfg := db.IntegrationConfig{
		Type: "plex", Name: "Disabled Plex", URL: "http://localhost:32400",
		APIKey: "key3", Enabled: true, LastError: "timeout",
	}
	database.Create(&disabledCfg)
	database.Model(&disabledCfg).Update("enabled", false)

	svc := NewIntegrationHealthService(integrationSvc, bus)
	svc.seed()

	// All enabled integrations should be seeded (both healthy and failing)
	if svc.TrackedCount() != 2 {
		t.Fatalf("expected 2 tracked (all enabled), got %d", svc.TrackedCount())
	}

	// Verify health status includes both healthy and failing
	entries := svc.HealthStatus()
	if len(entries) != 2 {
		t.Fatalf("expected 2 health entries, got %d", len(entries))
	}

	// Check that the failing integration's state was loaded
	var found bool
	for _, e := range entries {
		if e.Name == "Broken Radarr" {
			found = true
			if e.ConsecutiveFailures != 3 {
				t.Errorf("expected 3 consecutive failures from DB seed, got %d", e.ConsecutiveFailures)
			}
			if !e.Recovering {
				t.Error("expected recovering=true for broken integration")
			}
		}
	}
	if !found {
		t.Error("expected 'Broken Radarr' in health entries")
	}
}

func TestHealthService_Seed_FailingIntegration_NotificationSentPreset(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	integrationSvc := NewIntegrationService(database, bus)

	// Create integration that has already surpassed the notification threshold
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Long-Failing Sonarr", URL: "http://localhost:8989",
		APIKey: "key1", Enabled: true, LastError: "connection refused",
		ConsecutiveFailures: 5,
	})

	svc := NewIntegrationHealthService(integrationSvc, bus)
	svc.seed()

	// NotificationSent should be pre-set since failures >= threshold
	svc.mu.Lock()
	var state *healthState
	for _, s := range svc.states {
		state = s
	}
	svc.mu.Unlock()

	if state == nil {
		t.Fatal("expected state to be seeded")
	}
	if !state.NotificationSent {
		t.Error("expected NotificationSent=true for integration with failures >= threshold")
	}
}

// --------------------------------------------------------------------------
// recordFailure threshold-gated notification
// --------------------------------------------------------------------------

func TestHealthService_RecordFailure_ThresholdNotification(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	integrationSvc := NewIntegrationService(database, bus)

	// Create integration in DB
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989",
		APIKey: "key1", Enabled: true,
	})

	svc := NewIntegrationHealthService(integrationSvc, bus)
	svc.seed()

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	testErr := errors.New("connection refused")

	// Failure 1 — no IntegrationTestFailedEvent (below threshold)
	svc.recordFailure(1, testErr)
	assertNoEventOfType(t, ch, "integration_test_failed")

	// Failure 2 — no IntegrationTestFailedEvent (below threshold)
	svc.recordFailure(1, testErr)
	assertNoEventOfType(t, ch, "integration_test_failed")

	// Failure 3 — threshold reached, IntegrationTestFailedEvent published
	svc.recordFailure(1, testErr)
	assertEventOfType(t, ch, "integration_test_failed")

	// Failure 4 — no additional IntegrationTestFailedEvent (already sent)
	svc.recordFailure(1, testErr)
	assertNoEventOfType(t, ch, "integration_test_failed")

	// Verify DB state
	var cfg db.IntegrationConfig
	database.First(&cfg, 1)
	if cfg.ConsecutiveFailures != 4 {
		t.Errorf("expected consecutive_failures=4 in DB, got %d", cfg.ConsecutiveFailures)
	}
	if cfg.LastError != "connection refused" {
		t.Errorf("expected last_error='connection refused', got %q", cfg.LastError)
	}
}

// --------------------------------------------------------------------------
// recordSuccess recovery event
// --------------------------------------------------------------------------

func TestHealthService_RecordSuccess_AfterFailures(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	integrationSvc := NewIntegrationService(database, bus)

	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989",
		APIKey: "key1", Enabled: true, LastError: "old error",
		ConsecutiveFailures: 5,
	})

	svc := NewIntegrationHealthService(integrationSvc, bus)
	svc.seed()

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	// Report success — should publish IntegrationRecoveredEvent
	svc.recordSuccess(1)
	assertEventOfType(t, ch, "integration_recovered")

	// Verify DB state reset
	var cfg db.IntegrationConfig
	database.First(&cfg, 1)
	if cfg.ConsecutiveFailures != 0 {
		t.Errorf("expected consecutive_failures=0 in DB after recovery, got %d", cfg.ConsecutiveFailures)
	}
	if cfg.LastError != "" {
		t.Errorf("expected empty last_error after recovery, got %q", cfg.LastError)
	}

	// Verify in-memory state
	if !svc.IsHealthy(1) {
		t.Error("expected integration to be healthy after recovery")
	}
}

func TestHealthService_RecordSuccess_NoPriorFailures(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	integrationSvc := NewIntegrationService(database, bus)

	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989",
		APIKey: "key1", Enabled: true,
	})

	svc := NewIntegrationHealthService(integrationSvc, bus)
	svc.seed()

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	// Report success with no prior failures — no recovery event
	svc.recordSuccess(1)
	assertNoEventOfType(t, ch, "integration_recovered")
}

// --------------------------------------------------------------------------
// ReportFailure / ReportSuccess (external API)
// --------------------------------------------------------------------------

func TestHealthService_ReportFailure(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	integrationSvc := NewIntegrationService(database, bus)

	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989",
		APIKey: "key1", Enabled: true,
	})

	svc := NewIntegrationHealthService(integrationSvc, bus)
	svc.seed()

	svc.ReportFailure(1, errors.New("timeout"))

	if svc.IsHealthy(1) {
		t.Error("expected integration to be unhealthy after ReportFailure")
	}
}

func TestHealthService_ReportSuccess(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	integrationSvc := NewIntegrationService(database, bus)

	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989",
		APIKey: "key1", Enabled: true, LastError: "old error",
		ConsecutiveFailures: 2,
	})

	svc := NewIntegrationHealthService(integrationSvc, bus)
	svc.seed()

	// Starts unhealthy
	if svc.IsHealthy(1) {
		t.Fatal("expected integration to start unhealthy")
	}

	svc.ReportSuccess(1)

	if !svc.IsHealthy(1) {
		t.Error("expected integration to be healthy after ReportSuccess")
	}
}

func TestHealthService_ReportFailure_UnknownID(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	integrationSvc := NewIntegrationService(database, bus)

	// Create integration but DON'T seed the health service
	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989",
		APIKey: "key1", Enabled: true,
	})

	svc := NewIntegrationHealthService(integrationSvc, bus)
	// Don't call seed — states map is empty

	// Should reload from DB and track
	svc.ReportFailure(1, errors.New("timeout"))

	if svc.TrackedCount() != 1 {
		t.Fatalf("expected 1 tracked after ReportFailure with unknown ID, got %d", svc.TrackedCount())
	}
}

// --------------------------------------------------------------------------
// Query methods: IsHealthy, HealthyIDs, UnhealthyTypes
// --------------------------------------------------------------------------

func TestHealthService_QueryMethods(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	integrationSvc := NewIntegrationService(database, bus)

	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Healthy Sonarr", URL: "http://sonarr:8989",
		APIKey: "key1", Enabled: true,
	})
	database.Create(&db.IntegrationConfig{
		Type: "radarr", Name: "Broken Radarr", URL: "http://radarr:7878",
		APIKey: "key2", Enabled: true, LastError: "connection refused",
		ConsecutiveFailures: 1,
	})
	database.Create(&db.IntegrationConfig{
		Type: "plex", Name: "Broken Plex", URL: "http://plex:32400",
		APIKey: "key3", Enabled: true, LastError: "timeout",
		ConsecutiveFailures: 2,
	})

	svc := NewIntegrationHealthService(integrationSvc, bus)
	svc.seed()

	// IsHealthy
	if !svc.IsHealthy(1) {
		t.Error("expected integration 1 (Healthy Sonarr) to be healthy")
	}
	if svc.IsHealthy(2) {
		t.Error("expected integration 2 (Broken Radarr) to be unhealthy")
	}
	if svc.IsHealthy(3) {
		t.Error("expected integration 3 (Broken Plex) to be unhealthy")
	}

	// IsHealthy for unknown ID — assumed healthy
	if !svc.IsHealthy(999) {
		t.Error("expected unknown integration ID to be assumed healthy")
	}

	// HealthyIDs
	healthyIDs := svc.HealthyIDs()
	if !healthyIDs[1] {
		t.Error("expected ID 1 in HealthyIDs")
	}
	if healthyIDs[2] {
		t.Error("expected ID 2 NOT in HealthyIDs")
	}
	if healthyIDs[3] {
		t.Error("expected ID 3 NOT in HealthyIDs")
	}

	// UnhealthyTypes
	unhealthyTypes := svc.UnhealthyTypes()
	typeSet := make(map[string]bool)
	for _, typ := range unhealthyTypes {
		typeSet[typ] = true
	}
	if !typeSet["radarr"] {
		t.Error("expected 'radarr' in UnhealthyTypes")
	}
	if !typeSet["plex"] {
		t.Error("expected 'plex' in UnhealthyTypes")
	}
	if typeSet["sonarr"] {
		t.Error("expected 'sonarr' NOT in UnhealthyTypes")
	}
}

// --------------------------------------------------------------------------
// HealthStatus returns ALL integrations
// --------------------------------------------------------------------------

func TestHealthService_HealthStatus_ReturnsAll(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	integrationSvc := NewIntegrationService(database, bus)

	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Healthy Sonarr", URL: "http://localhost:8989",
		APIKey: "key1", Enabled: true,
	})
	database.Create(&db.IntegrationConfig{
		Type: "radarr", Name: "Broken Radarr", URL: "http://localhost:7878",
		APIKey: "key2", Enabled: true, LastError: "error",
		ConsecutiveFailures: 1,
	})

	svc := NewIntegrationHealthService(integrationSvc, bus)
	svc.seed()

	entries := svc.HealthStatus()
	if len(entries) != 2 {
		t.Fatalf("expected 2 health entries (all integrations), got %d", len(entries))
	}

	// Verify both healthy and failing are present
	var healthyFound, failingFound bool
	for _, e := range entries {
		if e.Name == "Healthy Sonarr" && !e.Recovering {
			healthyFound = true
		}
		if e.Name == "Broken Radarr" && e.Recovering {
			failingFound = true
		}
	}
	if !healthyFound {
		t.Error("expected healthy integration in HealthStatus")
	}
	if !failingFound {
		t.Error("expected failing integration in HealthStatus")
	}
}

func TestHealthService_HealthStatus_Empty(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	integrationSvc := NewIntegrationService(database, bus)
	svc := NewIntegrationHealthService(integrationSvc, bus)

	entries := svc.HealthStatus()
	if len(entries) != 0 {
		t.Fatalf("expected 0 health entries, got %d", len(entries))
	}
}

// --------------------------------------------------------------------------
// Recovery event: IntegrationRecoveryAttemptEvent
// --------------------------------------------------------------------------

func TestHealthService_RecoveryAttemptEvent(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	integrationSvc := NewIntegrationService(database, bus)

	database.Create(&db.IntegrationConfig{
		Type: "sonarr", Name: "Firefly Sonarr", URL: "http://localhost:8989",
		APIKey: "key1", Enabled: true,
	})

	svc := NewIntegrationHealthService(integrationSvc, bus)
	svc.seed()

	ch := bus.Subscribe()
	defer bus.Unsubscribe(ch)

	// Failure should publish IntegrationRecoveryAttemptEvent (for SSE)
	svc.recordFailure(1, errors.New("timeout"))
	assertEventOfType(t, ch, "integration_recovery_attempt")
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// assertEventOfType drains the channel until an event of the given type is found,
// or fails if none appears within a short timeout. Non-matching events are skipped.
func assertEventOfType(t *testing.T, ch chan events.Event, eventType string) {
	t.Helper()
	timeout := time.After(100 * time.Millisecond)
	for {
		select {
		case evt := <-ch:
			if evt.EventType() == eventType {
				return // found
			}
			// skip non-matching events
		case <-timeout:
			t.Errorf("expected event of type %q but none received", eventType)
			return
		}
	}
}

// assertNoEventOfType drains the channel and fails if an event of the given type
// appears within a short timeout. Non-matching events are allowed.
func assertNoEventOfType(t *testing.T, ch chan events.Event, eventType string) {
	t.Helper()
	timeout := time.After(50 * time.Millisecond)
	for {
		select {
		case evt := <-ch:
			if evt.EventType() == eventType {
				t.Errorf("did NOT expect event of type %q but received one", eventType)
				return
			}
			// skip non-matching events
		case <-timeout:
			return // good — no matching event
		}
	}
}

func TestHealthService_StopJoinsGoroutine(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationHealthService(NewIntegrationService(database, bus), bus)
	svc.Start()

	done := make(chan struct{})
	go func() {
		svc.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop did not return after joining the health goroutine")
	}
}

func TestHealthService_StopWithoutStart(t *testing.T) {
	database := setupTestDB(t)
	bus := newTestBus(t)
	svc := NewIntegrationHealthService(NewIntegrationService(database, bus), bus)

	done := make(chan struct{})
	go func() {
		svc.Stop()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop hung when Start was never called")
	}
}
