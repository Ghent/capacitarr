package events

import (
	"encoding/json"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"capacitarr/internal/db"
)

// ActivityPersister subscribes to an EventBus and writes every event
// as an ActivityEvent row in the database. It runs as a background goroutine.
type ActivityPersister struct {
	database *gorm.DB
	bus      *EventBus
	ch       chan Event
	done     chan struct{}
}

// NewActivityPersister creates a new ActivityPersister wired to the given bus and database.
func NewActivityPersister(database *gorm.DB, bus *EventBus) *ActivityPersister {
	return &ActivityPersister{
		database: database,
		bus:      bus,
		done:     make(chan struct{}),
	}
}

// Start subscribes to the bus and begins persisting events in the background.
// Call Stop() to gracefully shut down.
func (p *ActivityPersister) Start() {
	p.ch = p.bus.Subscribe()
	go p.run()
}

// Stop unsubscribes from the bus and waits for the background goroutine to finish.
func (p *ActivityPersister) Stop() {
	p.bus.Unsubscribe(p.ch)
	<-p.done
}

func (p *ActivityPersister) run() {
	defer close(p.done)

	for event := range p.ch {
		p.persist(event)
	}
}

// persist writes a single event as an ActivityEvent row.
func (p *ActivityPersister) persist(event Event) {
	metadata := ""
	if jsonBytes, err := json.Marshal(event); err == nil {
		metadata = string(jsonBytes)
	}

	entry := db.ActivityEvent{
		EventType: event.EventType(),
		Message:   event.EventMessage(),
		Metadata:  metadata,
		CreatedAt: time.Now().UTC(),
	}

	if err := p.database.Create(&entry).Error; err != nil {
		slog.Error("Failed to persist activity event",
			"component", "events",
			"eventType", event.EventType(),
			"message", event.EventMessage(),
			"error", err,
		)
	}
}
