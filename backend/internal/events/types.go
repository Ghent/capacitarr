package events

// Event is the interface all typed events implement.
type Event interface {
	// EventType returns a machine-readable event type string (e.g. "engine_start").
	EventType() string
	// EventMessage returns a human-readable description of the event.
	EventMessage() string
}
