package db

import (
	"log/slog"
	"time"

	"gorm.io/gorm"
)

// Activity event type constants.
const (
	EventEngineStart           = "engine_start"
	EventEngineComplete        = "engine_complete"
	EventEngineError           = "engine_error"
	EventEnginePaused          = "engine_paused"
	EventEngineResumed         = "engine_resumed"
	EventEngineModeChanged     = "engine_mode_changed"
	EventSettingsChanged       = "settings_changed"
	EventLogin                 = "login"
	EventIntegrationTest       = "integration_test"
	EventIntegrationAdded      = "integration_added"
	EventIntegrationRemoved    = "integration_removed"
	EventApprovalApproved      = "approval_approved"
	EventApprovalRejected      = "approval_rejected"
	EventRuleCreated           = "rule_created"
	EventRuleUpdated           = "rule_updated"
	EventRuleDeleted           = "rule_deleted"
	EventIntegrationTestFailed = "integration_test_failed"
	EventServerStarted         = "server_started"
	EventPasswordChanged       = "password_changed"
)

// LogActivity records a system activity event. It is fire-and-forget:
// errors are logged but never returned to avoid disrupting the caller's
// primary workflow.
func LogActivity(database *gorm.DB, eventType, message string) {
	event := ActivityEvent{
		EventType: eventType,
		Message:   message,
		CreatedAt: time.Now().UTC(),
	}
	if err := database.Create(&event).Error; err != nil {
		slog.Error("Failed to log activity event",
			"component", "db",
			"eventType", eventType,
			"message", message,
			"error", err,
		)
	}
}

// LogActivityWithMetadata records a system activity event with optional JSON metadata.
func LogActivityWithMetadata(database *gorm.DB, eventType, message, metadata string) {
	event := ActivityEvent{
		EventType: eventType,
		Message:   message,
		Metadata:  metadata,
		CreatedAt: time.Now().UTC(),
	}
	if err := database.Create(&event).Error; err != nil {
		slog.Error("Failed to log activity event",
			"component", "db",
			"eventType", eventType,
			"message", message,
			"error", err,
		)
	}
}
