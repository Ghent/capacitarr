package events

import (
	"fmt"
	"time"
)

// =============================================================================
// Engine Events
// =============================================================================

// EngineStartEvent is published when an engine evaluation cycle begins.
type EngineStartEvent struct {
	DiskGroupModes map[uint]string `json:"diskGroupModes"`
}

// EventType implements Event.
func (e EngineStartEvent) EventType() string { return "engine_start" }

// EventMessage implements Event.
func (e EngineStartEvent) EventMessage() string {
	return fmt.Sprintf("Engine run started (%d disk groups)", len(e.DiskGroupModes))
}

// EngineCompleteEvent is published when an engine evaluation cycle finishes.
// Note: Deleted count and FreedBytes are NOT included here because deletions
// happen asynchronously in the DeletionService worker and may not be complete
// when the engine cycle publishes this event. The frontend reads those stats
// from the REST endpoint (GET /worker/stats), which queries the DB where the
// deletion worker atomically increments the counters.
type EngineCompleteEvent struct {
	Evaluated        int             `json:"evaluated"`
	Candidates       int             `json:"candidates"`
	DurationMs       int64           `json:"durationMs"`
	DiskGroupModes   map[uint]string `json:"diskGroupModes"`
	FreedBytes       int64           `json:"freedBytes"`       // Potential bytes freed (approval/dry-run) or actual bytes queued (auto)
	CompletedAtEpoch int64           `json:"completedAtEpoch"` // Unix epoch seconds when the run finished
}

// EventType implements Event.
func (e EngineCompleteEvent) EventType() string { return "engine_complete" }

// EventMessage implements Event.
func (e EngineCompleteEvent) EventMessage() string {
	return fmt.Sprintf("Engine run completed: evaluated %d, candidates %d", e.Evaluated, e.Candidates)
}

// EngineErrorEvent is published when an engine cycle fails.
type EngineErrorEvent struct {
	Error string `json:"error"`
}

// EventType implements Event.
func (e EngineErrorEvent) EventType() string { return "engine_error" }

// EventMessage implements Event.
func (e EngineErrorEvent) EventMessage() string { return "Engine error: " + e.Error }

// EnrichmentCompleteEvent is published after the enrichment pipeline finishes.
// Provides a summary of enrichment health so the frontend can display
// enrichment statistics and surface configuration problems.
type EnrichmentCompleteEvent struct {
	EnrichersRun   int       `json:"enrichersRun"`   // Total enrichers executed
	ItemsProcessed int       `json:"itemsProcessed"` // Total items passed through the pipeline
	TotalMatches   int       `json:"totalMatches"`   // Sum of matches across all enrichers
	ZeroMatchers   []string  `json:"zeroMatchers"`   // Enrichers that produced zero matches despite having data
	Timestamp      time.Time `json:"timestamp"`
}

// EventType implements Event.
func (e EnrichmentCompleteEvent) EventType() string { return "enrichment_complete" }

// EventMessage implements Event.
func (e EnrichmentCompleteEvent) EventMessage() string {
	if len(e.ZeroMatchers) > 0 {
		return fmt.Sprintf("Enrichment complete: %d enrichers, %d matches (%d zero-match enrichers)",
			e.EnrichersRun, e.TotalMatches, len(e.ZeroMatchers))
	}
	return fmt.Sprintf("Enrichment complete: %d enrichers, %d matches", e.EnrichersRun, e.TotalMatches)
}

// ManualRunTriggeredEvent is published when a user manually triggers an engine run.
type ManualRunTriggeredEvent struct{}

// EventType implements Event.
func (e ManualRunTriggeredEvent) EventType() string { return "manual_run_triggered" }

// EventMessage implements Event.
func (e ManualRunTriggeredEvent) EventMessage() string { return "Manual engine run triggered" }
