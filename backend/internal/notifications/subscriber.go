package notifications

import (
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

// EventBusSubscriber subscribes to the event bus and dispatches notifications
// to configured channels. It replaces the inline Dispatch() calls scattered
// throughout the codebase.
type EventBusSubscriber struct {
	database *gorm.DB
	bus      *events.EventBus
	ch       chan events.Event
	done     chan struct{}
}

// NewEventBusSubscriber creates a new notification subscriber.
func NewEventBusSubscriber(database *gorm.DB, bus *events.EventBus) *EventBusSubscriber {
	return &EventBusSubscriber{
		database: database,
		bus:      bus,
		done:     make(chan struct{}),
	}
}

// Start subscribes to the event bus and begins dispatching notifications.
func (s *EventBusSubscriber) Start() {
	s.ch = s.bus.Subscribe()
	go s.run()
}

// Stop unsubscribes from the bus and waits for the background goroutine.
func (s *EventBusSubscriber) Stop() {
	s.bus.Unsubscribe(s.ch)
	<-s.done
}

func (s *EventBusSubscriber) run() {
	defer close(s.done)
	for event := range s.ch {
		s.handle(event)
	}
}

// handle maps typed events to notification events and dispatches them.
func (s *EventBusSubscriber) handle(event events.Event) {
	notifEvent, notifType := mapToNotification(event)
	if notifType == "" {
		return // This event type doesn't trigger notifications
	}

	var configs []db.NotificationConfig
	if err := s.database.Where("enabled = ?", true).Find(&configs).Error; err != nil {
		slog.Error("Failed to query notification configs", "component", "notifications", "error", err)
		return
	}

	for _, cfg := range configs {
		if !subscribes(cfg, notifType) {
			continue
		}

		c := cfg
		ne := notifEvent
		go func() {
			var err error
			switch c.Type {
			case "discord":
				err = SendDiscord(c.WebhookURL, ne)
			case "slack":
				err = SendSlack(c.WebhookURL, ne)
			case "inapp":
				err = SendInApp(ne)
			default:
				slog.Warn("Unknown notification channel type", "component", "notifications", "type", c.Type)
				return
			}

			if err != nil {
				slog.Error("Failed to send notification",
					"component", "notifications",
					"channel", c.Name,
					"type", c.Type,
					"event", ne.Type,
					"error", err,
				)
			} else {
				slog.Debug("Notification sent via event bus subscriber",
					"component", "notifications",
					"channel", c.Name,
					"type", c.Type,
					"event", ne.Type,
				)
			}
		}()
	}
}

// mapToNotification converts a typed event bus event to a NotificationEvent.
// Returns empty notifType if the event doesn't trigger notifications.
func mapToNotification(event events.Event) (NotificationEvent, string) {
	switch e := event.(type) {
	// Engine events
	case events.EngineCompleteEvent:
		return NotificationEvent{
			Type:    EventEngineComplete,
			Title:   "Engine Run Complete",
			Message: e.EventMessage(),
			Fields: map[string]string{
				"Evaluated": fmt.Sprintf("%d", e.Evaluated),
				"Flagged":   fmt.Sprintf("%d", e.Flagged),
				"Deleted":   fmt.Sprintf("%d", e.Deleted),
				"Duration":  fmt.Sprintf("%dms", e.DurationMs),
			},
		}, EventEngineComplete

	case events.EngineErrorEvent:
		return NotificationEvent{
			Type:    EventEngineError,
			Title:   "Engine Error",
			Message: e.EventMessage(),
			Fields: map[string]string{
				"Error": e.Error,
			},
		}, EventEngineError

	// Deletion events
	case events.DeletionSuccessEvent:
		return NotificationEvent{
			Type:    EventDeletionExecuted,
			Title:   "Deletion Executed",
			Message: e.EventMessage(),
			Fields: map[string]string{
				"Media":  e.MediaName,
				"Action": "Deleted",
				"Size":   fmt.Sprintf("%d bytes", e.SizeBytes),
			},
		}, EventDeletionExecuted

	case events.DeletionDryRunEvent:
		return NotificationEvent{
			Type:    EventDeletionExecuted,
			Title:   "Deletion Executed (Dry-Run)",
			Message: e.EventMessage(),
			Fields: map[string]string{
				"Media":  e.MediaName,
				"Action": "Dry-Run",
				"Size":   fmt.Sprintf("%d bytes", e.SizeBytes),
			},
		}, EventDeletionExecuted

	case events.DeletionFailedEvent:
		return NotificationEvent{
			Type:    EventEngineError,
			Title:   "Deletion Failed",
			Message: e.EventMessage(),
			Fields: map[string]string{
				"Media": e.MediaName,
				"Error": e.Error,
			},
		}, EventEngineError

	default:
		return NotificationEvent{}, ""
	}
}
