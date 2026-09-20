package services

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"capacitarr/internal/events"
	"capacitarr/internal/integrations"
)

// Health check constants.
const (
	// connectionFailureThreshold is the number of consecutive failures required
	// before an "Integration Down" notification is published. Prevents transient
	// blips (container restarts, brief API timeouts) from triggering false alarms.
	connectionFailureThreshold = 3

	// healthCheckInterval is the regular interval for checking healthy integrations.
	healthCheckInterval = 2 * time.Minute

	// healthTickPeriod is the background ticker period. On each tick, integrations
	// due for checking (NextCheck <= now) are probed.
	healthTickPeriod = 15 * time.Second

	// backoffBase is the base delay for the linear backoff schedule on failing
	// integrations. The delay is min(backoffBase * consecutiveFailures, backoffCap).
	// Schedule: 15s → 30s → 45s → 60s → 60s...
	backoffBase = 15 * time.Second

	// backoffCap is the maximum delay between probes for failing integrations.
	backoffCap = 60 * time.Second
)

// IntegrationHealthEntry is the API-facing snapshot of a tracked integration's
// health state. Returned by IntegrationHealthService.HealthStatus().
type IntegrationHealthEntry struct {
	IntegrationID       uint      `json:"integrationId"`
	IntegrationType     string    `json:"integrationType"`
	Name                string    `json:"name"`
	ConsecutiveFailures int       `json:"consecutiveFailures"`
	LastError           string    `json:"lastError"`
	NextRetryAt         time.Time `json:"nextRetryAt"`
	Recovering          bool      `json:"recovering"`
}

// healthState holds in-memory tracking for a single integration's health.
type healthState struct {
	IntegrationID       uint
	IntegrationType     string
	Name                string
	URL                 string
	APIKey              string
	Healthy             bool
	LastError           string
	ConsecutiveFailures int
	LastCheck           time.Time
	NextCheck           time.Time // healthy: regular interval; failing: backoff
	NotificationSent    bool      // true once "down" notification was sent at threshold
}

// nextBackoff calculates the next retry delay using linear backoff.
// delay = backoffBase * consecutiveFailures, capped at backoffCap.
func (s *healthState) nextBackoff() time.Duration {
	if s.ConsecutiveFailures <= 0 {
		return backoffBase
	}
	delay := backoffBase * time.Duration(s.ConsecutiveFailures)
	if delay > backoffCap {
		delay = backoffCap
	}
	return delay
}

// IntegrationHealthService monitors integration connectivity, tracks health
// state, and publishes notification events with consecutive-failure gating.
// It replaces RecoveryService and absorbs all health-related concerns that
// were previously scattered across the poller, startup self-test, and
// IntegrationService.
type IntegrationHealthService struct {
	integrationSvc *IntegrationService
	bus            *events.EventBus

	mu       sync.Mutex
	states   map[uint]*healthState // integrationID → state (ALL enabled integrations)
	done     chan struct{}
	stopped  chan struct{}
	started  atomic.Bool
	stopOnce sync.Once
}

// NewIntegrationHealthService creates an IntegrationHealthService.
// Call Start() to begin background health monitoring.
func NewIntegrationHealthService(integrationSvc *IntegrationService, bus *events.EventBus) *IntegrationHealthService {
	return &IntegrationHealthService{
		integrationSvc: integrationSvc,
		bus:            bus,
		states:         make(map[uint]*healthState),
		done:           make(chan struct{}),
		stopped:        make(chan struct{}),
	}
}

// Start seeds state from DB, runs an initial health check of all integrations
// (replacing the startup self-test), then begins the background ticker.
func (h *IntegrationHealthService) Start() {
	h.seed()

	// Initial health check of all integrations (replaces startup self-test).
	h.checkAll()

	if !h.started.CompareAndSwap(false, true) {
		return
	}

	go func() {
		defer close(h.stopped)
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("Panic recovered in health service goroutine",
					"component", "health", "panic", rec)
			}
		}()
		ticker := time.NewTicker(healthTickPeriod)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				h.tick()
			case <-h.done:
				return
			}
		}
	}()

	h.mu.Lock()
	count := len(h.states)
	h.mu.Unlock()

	slog.Info("Integration health service started",
		"component", "health", "trackedIntegrations", count)
}

// Stop signals the background goroutine to exit and waits for it to finish.
// Safe to call if Start was never invoked.
func (h *IntegrationHealthService) Stop() {
	h.stopOnce.Do(func() {
		close(h.done)
		slog.Info("Integration health service stopped", "component", "health")
	})
	if h.started.Load() {
		<-h.stopped
	}
}

// seed loads all enabled integrations from the DB and populates the states map.
// Failing integrations (non-empty LastError) start on the backoff schedule;
// healthy ones start on the regular health check interval.
func (h *IntegrationHealthService) seed() {
	configs, err := h.integrationSvc.ListEnabled()
	if err != nil {
		slog.Error("Health seed: failed to list integrations",
			"component", "health", "error", err)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	for _, cfg := range configs {
		state := &healthState{
			IntegrationID:       cfg.ID,
			IntegrationType:     cfg.Type,
			Name:                cfg.Name,
			URL:                 cfg.URL,
			APIKey:              cfg.APIKey,
			ConsecutiveFailures: cfg.ConsecutiveFailures,
			LastError:           cfg.LastError,
		}

		if cfg.LastError != "" {
			state.Healthy = false
			state.NextCheck = now // check immediately on first tick
			state.NotificationSent = cfg.ConsecutiveFailures >= connectionFailureThreshold
		} else {
			state.Healthy = true
			state.NextCheck = now // check immediately on first tick
		}

		h.states[cfg.ID] = state
	}

	if len(configs) > 0 {
		slog.Info("Health seed: loaded integrations from DB",
			"component", "health", "count", len(configs))
	}
}

// checkAll probes all tracked integrations. Used for the initial startup check.
func (h *IntegrationHealthService) checkAll() {
	h.mu.Lock()
	toCheck := make([]*healthState, 0, len(h.states))
	for _, state := range h.states {
		cp := *state
		toCheck = append(toCheck, &cp)
	}
	h.mu.Unlock()

	for _, state := range toCheck {
		h.checkIntegration(state)
	}
}

// tick checks which integrations are due for probing and tests them.
func (h *IntegrationHealthService) tick() {
	now := time.Now()

	h.mu.Lock()
	var due []*healthState
	for _, state := range h.states {
		if now.After(state.NextCheck) || now.Equal(state.NextCheck) {
			cp := *state
			due = append(due, &cp)
		}
	}
	h.mu.Unlock()

	if len(due) == 0 {
		return
	}

	for _, state := range due {
		h.checkIntegration(state)
	}
}

// checkIntegration creates a client and tests connectivity to a single integration.
// Assumes integration factories are already registered (via main.go startup).
func (h *IntegrationHealthService) checkIntegration(state *healthState) {
	rawClient := integrations.CreateClient(state.IntegrationType, state.URL, state.APIKey)
	if rawClient == nil {
		slog.Warn("Health check: unknown integration type",
			"component", "health",
			"integrationID", state.IntegrationID,
			"type", state.IntegrationType)
		return
	}

	conn, ok := rawClient.(integrations.Connectable)
	if !ok {
		return
	}

	err := conn.TestConnection()

	if err != nil {
		h.recordFailure(state.IntegrationID, err)
	} else {
		h.recordSuccess(state.IntegrationID)
	}
}

// recordFailure handles a health check or reported failure for an integration.
// Increments the consecutive failure counter, updates the DB, publishes
// threshold-gated notification events, and schedules the next backoff check.
func (h *IntegrationHealthService) recordFailure(id uint, err error) {
	h.mu.Lock()
	state, exists := h.states[id]
	if !exists {
		// Integration added after last seed — reload from DB
		h.mu.Unlock()
		h.reloadIntegration(id)
		h.mu.Lock()
		state = h.states[id]
		if state == nil {
			h.mu.Unlock()
			return
		}
	}

	state.Healthy = false
	state.ConsecutiveFailures++
	state.LastError = err.Error()
	state.LastCheck = time.Now()
	state.NextCheck = time.Now().Add(state.nextBackoff())

	failures := state.ConsecutiveFailures
	shouldNotify := failures == connectionFailureThreshold && !state.NotificationSent
	if shouldNotify {
		state.NotificationSent = true
	}

	// Snapshot for events/DB updates
	intType := state.IntegrationType
	name := state.Name
	url := state.URL
	errMsg := state.LastError
	attempt := failures

	h.mu.Unlock()

	// Update DB
	if dbErr := h.integrationSvc.UpdateSyncStatusDirect(id, nil, errMsg, failures); dbErr != nil {
		slog.Error("Health: failed to update sync status after failure",
			"component", "health", "integrationID", id, "error", dbErr)
	}

	// Publish threshold-gated notification event
	if shouldNotify {
		h.bus.Publish(events.IntegrationTestFailedEvent{
			IntegrationType: intType,
			Name:            name,
			URL:             url,
			Error:           errMsg,
		})
	}

	// Publish recovery attempt event (for SSE — always fires)
	nextDelay := backoffBase * time.Duration(failures)
	if nextDelay > backoffCap {
		nextDelay = backoffCap
	}
	h.bus.Publish(events.IntegrationRecoveryAttemptEvent{
		IntegrationID:    id,
		IntegrationType:  intType,
		Name:             name,
		Attempt:          attempt,
		Success:          false,
		Error:            errMsg,
		NextRetrySeconds: int(nextDelay.Seconds()),
	})

	slog.Debug("Health: integration failure recorded",
		"component", "health",
		"integrationID", id,
		"name", name,
		"failures", failures,
		"nextCheck", state.NextCheck.Format(time.RFC3339))
}

// recordSuccess handles a health check or reported success for an integration.
// If the integration was previously failing, publishes a recovery event and
// resets the failure counter.
func (h *IntegrationHealthService) recordSuccess(id uint) {
	h.mu.Lock()
	state, exists := h.states[id]
	if !exists {
		// Integration added after last seed — reload from DB
		h.mu.Unlock()
		h.reloadIntegration(id)
		h.mu.Lock()
		state = h.states[id]
		if state == nil {
			h.mu.Unlock()
			return
		}
	}

	wasFailures := state.ConsecutiveFailures > 0
	prevAttempt := state.ConsecutiveFailures

	state.Healthy = true
	state.ConsecutiveFailures = 0
	state.LastError = ""
	state.LastCheck = time.Now()
	state.NextCheck = time.Now().Add(healthCheckInterval)
	state.NotificationSent = false

	// Snapshot for events/DB
	intType := state.IntegrationType
	name := state.Name
	url := state.URL

	h.mu.Unlock()

	// Update DB
	now := time.Now()
	if dbErr := h.integrationSvc.UpdateSyncStatusDirect(id, &now, "", 0); dbErr != nil {
		slog.Error("Health: failed to update sync status after success",
			"component", "health", "integrationID", id, "error", dbErr)
	}

	// Publish recovery event if this was previously failing
	if wasFailures {
		h.bus.Publish(events.IntegrationRecoveredEvent{
			IntegrationID:   id,
			IntegrationType: intType,
			Name:            name,
			URL:             url,
		})

		slog.Info("Health: integration recovered",
			"component", "health",
			"integrationID", id,
			"name", name,
			"type", intType,
			"afterFailures", prevAttempt)
	}

	// Publish recovery attempt event (for SSE)
	if wasFailures {
		h.bus.Publish(events.IntegrationRecoveryAttemptEvent{
			IntegrationID:   id,
			IntegrationType: intType,
			Name:            name,
			Attempt:         prevAttempt + 1,
			Success:         true,
		})
	}
}

// reloadIntegration loads a single integration from the DB and adds it to the
// states map. Used when an external caller reports a failure/success for an
// integration that was added after the last seed.
func (h *IntegrationHealthService) reloadIntegration(id uint) {
	cfg, err := h.integrationSvc.GetByID(id)
	if err != nil {
		slog.Warn("Health: failed to reload integration from DB",
			"component", "health", "integrationID", id, "error", err)
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.states[id]; exists {
		return // already added by another goroutine
	}

	h.states[id] = &healthState{
		IntegrationID:       cfg.ID,
		IntegrationType:     cfg.Type,
		Name:                cfg.Name,
		URL:                 cfg.URL,
		APIKey:              cfg.APIKey,
		Healthy:             cfg.LastError == "",
		LastError:           cfg.LastError,
		ConsecutiveFailures: cfg.ConsecutiveFailures,
		NextCheck:           time.Now().Add(healthCheckInterval),
	}
}

// ReportFailure is called by external components (poller data fetch, SyncAll)
// when an I/O operation against an integration fails. Equivalent to a health
// check failure — updates state, increments counter, may trigger notification.
func (h *IntegrationHealthService) ReportFailure(id uint, err error) {
	h.recordFailure(id, err)
}

// ReportSuccess is called by external components when an I/O operation
// succeeds, confirming the integration is reachable.
func (h *IntegrationHealthService) ReportSuccess(id uint) {
	h.recordSuccess(id)
}

// IsHealthy returns whether a specific integration is currently healthy.
// Used by the poller to decide whether to fetch data from an integration.
func (h *IntegrationHealthService) IsHealthy(id uint) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	state, exists := h.states[id]
	if !exists {
		return true // unknown integrations are assumed healthy until proven otherwise
	}
	return state.Healthy
}

// HealthyIDs returns the set of integration IDs currently considered healthy.
func (h *IntegrationHealthService) HealthyIDs() map[uint]bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	ids := make(map[uint]bool, len(h.states))
	for id, state := range h.states {
		if state.Healthy {
			ids[id] = true
		}
	}
	return ids
}

// UnhealthyTypes returns integration types currently failing. Used to populate
// EvaluationContext.BrokenIntegrationTypes so the scoring engine skips factors
// whose required integration is broken rather than penalizing items.
func (h *IntegrationHealthService) UnhealthyTypes() []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	seen := make(map[string]bool)
	for _, state := range h.states {
		if !state.Healthy {
			seen[state.IntegrationType] = true
		}
	}

	types := make([]string, 0, len(seen))
	for t := range seen {
		types = append(types, t)
	}
	return types
}

// HealthStatus returns the API-facing health snapshot for GET /integrations/health.
// Returns entries for ALL tracked integrations (healthy and failing), unlike the
// old RecoveryService which only returned failing ones.
func (h *IntegrationHealthService) HealthStatus() []IntegrationHealthEntry {
	h.mu.Lock()
	defer h.mu.Unlock()

	entries := make([]IntegrationHealthEntry, 0, len(h.states))
	for _, state := range h.states {
		entries = append(entries, IntegrationHealthEntry{
			IntegrationID:       state.IntegrationID,
			IntegrationType:     state.IntegrationType,
			Name:                state.Name,
			ConsecutiveFailures: state.ConsecutiveFailures,
			LastError:           state.LastError,
			NextRetryAt:         state.NextCheck,
			Recovering:          !state.Healthy,
		})
	}
	return entries
}

// TrackedCount returns the number of integrations being monitored (all enabled,
// not just failing).
func (h *IntegrationHealthService) TrackedCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.states)
}
