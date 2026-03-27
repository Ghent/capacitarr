package services

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/notifications"
)

// ErrUnknownChannelType is returned when a notification channel has an unrecognized type.
var ErrUnknownChannelType = errors.New("unknown channel type")

// ChannelProvider abstracts the notification channel service for the dispatch
// service. Satisfied by NotificationChannelService.
type ChannelProvider interface {
	ListEnabled() ([]db.NotificationConfig, error)
	GetByID(id uint) (*db.NotificationConfig, error)
}

// VersionChecker abstracts the version service for populating update banners
// in cycle digests. Satisfied by VersionService.
type VersionChecker interface {
	CheckForUpdate() (*VersionCheckResult, error)
}

// NotificationDispatchService dispatches notifications via the Sender
// interface. Cycle digest notifications are flushed explicitly by the poller
// via FlushCycleDigest(), which replaces the previous event-based two-gate
// accumulation pattern for simpler and more reliable delivery.
//
// Immediate alerts (errors, mode changes, server started, threshold breached,
// update available) are dispatched via the event bus without delay.
type NotificationDispatchService struct {
	bus            *events.EventBus
	channels       ChannelProvider
	versionChecker VersionChecker
	senders        map[string]notifications.Sender
	version        string

	mu   sync.Mutex
	ch   chan events.Event
	done chan struct{}
}

// NewNotificationDispatchService creates a new dispatch service. The
// versionChecker may be nil at construction and set later via
// SetVersionChecker().
func NewNotificationDispatchService(
	bus *events.EventBus,
	channels ChannelProvider,
	versionChecker VersionChecker,
	version string,
) *NotificationDispatchService {
	senders := map[string]notifications.Sender{
		"discord": notifications.NewDiscordSender(),
		"apprise": notifications.NewAppriseSender(),
	}

	// Verify that the sender map keys match db.ValidNotificationChannelTypes
	// at construction time. A mismatch means a notification channel type was
	// added to validation without a corresponding sender implementation (or
	// vice versa), which would cause silent runtime failures.
	for senderType := range senders {
		if !db.ValidNotificationChannelTypes[senderType] {
			panic(fmt.Sprintf("notification sender %q has no entry in db.ValidNotificationChannelTypes", senderType))
		}
	}
	for channelType := range db.ValidNotificationChannelTypes {
		if _, ok := senders[channelType]; !ok {
			panic(fmt.Sprintf("db.ValidNotificationChannelTypes has %q but no sender is registered", channelType))
		}
	}

	return &NotificationDispatchService{
		bus:            bus,
		channels:       channels,
		versionChecker: versionChecker,
		senders:        senders,
		version:        version,
		done:           make(chan struct{}),
	}
}

// SetVersionChecker sets the version checker dependency. Called after
// VersionService is initialized (it is created after the registry).
func (s *NotificationDispatchService) SetVersionChecker(vc VersionChecker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.versionChecker = vc
}

// SetVersion sets the application version string for notification embeds.
func (s *NotificationDispatchService) SetVersion(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version = v
}

// Start subscribes to the event bus and begins the background dispatch loop
// for immediate alert events. Cycle digest notifications are handled
// separately via FlushCycleDigest().
func (s *NotificationDispatchService) Start() {
	s.ch = s.bus.Subscribe()
	go s.run()
}

// Stop unsubscribes from the bus and waits for the goroutine to exit.
func (s *NotificationDispatchService) Stop() {
	s.bus.Unsubscribe(s.ch)
	<-s.done
}

// TestChannel sends a test notification to a specific channel by ID.
func (s *NotificationDispatchService) TestChannel(id uint) error {
	cfg, err := s.channels.GetByID(id)
	if err != nil {
		return err
	}

	s.mu.Lock()
	ver := s.version
	s.mu.Unlock()

	alert := notifications.Alert{
		Type:    notifications.AlertTest,
		Title:   "🔔 Test — channel is working!",
		Message: "This is a test notification from Capacitarr.",
		Version: ver,
	}

	sender, ok := s.senders[cfg.Type]
	if !ok {
		return fmt.Errorf("%w: %s", ErrUnknownChannelType, cfg.Type)
	}

	return sender.SendAlert(notifications.SenderConfig{
		WebhookURL:  cfg.WebhookURL,
		AppriseTags: cfg.AppriseTags,
	}, alert)
}

func (s *NotificationDispatchService) run() {
	defer close(s.done)
	for event := range s.ch {
		s.handle(event)
	}
}

func (s *NotificationDispatchService) handle(event events.Event) {
	switch e := event.(type) {
	case events.EngineErrorEvent:
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertError,
			Title:   "🔴 Engine Error",
			Message: "The evaluation engine failed. Check the application logs for details.",
		}, func(cfg db.NotificationConfig) bool { return cfg.OnError })

	case events.EngineModeChangedEvent:
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertModeChanged,
			Title:   fmt.Sprintf("⚠️ Mode: **%s** → **%s**", e.OldMode, e.NewMode),
			Message: modeChangedMessage(e.NewMode),
		}, func(cfg db.NotificationConfig) bool { return cfg.OnModeChanged })

	case events.ServerStartedEvent:
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertServerStarted,
			Title:   "🚀 Capacitarr is online",
			Message: "",
		}, func(cfg db.NotificationConfig) bool { return cfg.OnServerStarted })

	case events.ThresholdBreachedEvent:
		bar := notifications.ProgressBar(e.CurrentPct, 20)
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertThresholdBreached,
			Title:   "🔴 Threshold Breached",
			Message: fmt.Sprintf("`%s` **%.0f%%** / %.0f%%\nTarget: **%.0f%%**", bar, e.CurrentPct, e.ThresholdPct, e.TargetPct),
		}, func(cfg db.NotificationConfig) bool { return cfg.OnThresholdBreach })

	case events.UpdateAvailableEvent:
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertUpdateAvailable,
			Title:   fmt.Sprintf("📦 Update Available: **%s**", e.LatestVersion),
			Message: fmt.Sprintf("[View Release Notes](%s)", e.ReleaseURL),
		}, func(cfg db.NotificationConfig) bool { return cfg.OnUpdateAvailable })

	case events.ApprovalApprovedEvent:
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertApprovalActivity,
			Title:   "✅ Approved for Deletion",
			Message: fmt.Sprintf("**%d** item(s) approved — queued for deletion", 1),
		}, func(cfg db.NotificationConfig) bool { return cfg.OnApprovalActivity })

	case events.ApprovalRejectedEvent:
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertApprovalActivity,
			Title:   "😴 Item Snoozed",
			Message: fmt.Sprintf("Snoozed for %s", e.SnoozeDuration),
		}, func(cfg db.NotificationConfig) bool { return cfg.OnApprovalActivity })

	case events.IntegrationTestFailedEvent:
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertIntegrationStatus,
			Title:   fmt.Sprintf("🔴 Integration Down: %s", e.Name),
			Message: fmt.Sprintf("**%s** (%s) failed connection test:\n%s", e.Name, e.IntegrationType, e.Error),
		}, func(cfg db.NotificationConfig) bool { return cfg.OnIntegrationStatus })

	case events.IntegrationRecoveredEvent:
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertIntegrationStatus,
			Title:   fmt.Sprintf("🟢 Integration Recovered: %s", e.Name),
			Message: fmt.Sprintf("**%s** (%s) is back online", e.Name, e.IntegrationType),
		}, func(cfg db.NotificationConfig) bool { return cfg.OnIntegrationStatus })
	}
}

// FlushCycleDigest dispatches a cycle digest notification to all enabled
// channels. Called directly by the poller at the end of each engine cycle,
// replacing the event-based two-gate accumulation pattern. The poller builds
// the digest from its own counters, eliminating the fragile gate coordination.
func (s *NotificationDispatchService) FlushCycleDigest(digest notifications.CycleDigest) {
	s.mu.Lock()
	ver := s.version
	vc := s.versionChecker
	s.mu.Unlock()

	digest.Version = ver

	// Populate update banner from VersionService
	if vc != nil {
		if result, err := vc.CheckForUpdate(); err == nil && result.UpdateAvailable {
			digest.UpdateAvailable = true
			digest.LatestVersion = result.Latest
			digest.ReleaseURL = result.ReleaseURL
		}
	}

	s.dispatchDigest(digest)
}

// dispatchDigest sends the cycle digest to all enabled channels that
// subscribe to OnCycleDigest. Mode-specific digests are additionally gated
// by their respective subscription flags:
//   - Dry-run digests require OnDryRunDigest so users can silence the
//     periodic "would delete N items" summaries independently.
//   - Approval-mode digests require OnApprovalActivity so that disabling
//     "Approval Activity" silences all approval-related notifications.
func (s *NotificationDispatchService) dispatchDigest(digest notifications.CycleDigest) {
	configs, err := s.channels.ListEnabled()
	if err != nil {
		slog.Error("Failed to query notification configs for digest", "component", "notifications", "error", err)
		return
	}

	for _, cfg := range configs {
		if !cfg.OnCycleDigest {
			continue
		}
		// Dry-run digests are gated by both OnCycleDigest and
		// OnDryRunDigest so users can suppress periodic dry-run
		// summaries without losing auto-mode cleanup digests.
		if digest.ExecutionMode == notifications.ModeDryRun && !cfg.OnDryRunDigest {
			continue
		}
		// Approval-mode digests are gated by both OnCycleDigest and
		// OnApprovalActivity so that disabling "Approval Activity"
		// silences all approval-related notifications.
		if digest.ExecutionMode == notifications.ModeApproval && !cfg.OnApprovalActivity {
			continue
		}

		sender, ok := s.senders[cfg.Type]
		if !ok {
			slog.Warn("Unknown notification channel type", "component", "notifications", "type", cfg.Type)
			continue
		}

		c := cfg
		d := digest
		sc := notifications.SenderConfig{WebhookURL: c.WebhookURL, AppriseTags: c.AppriseTags}
		go func() {
			if sendErr := sender.SendDigest(sc, d); sendErr != nil {
				slog.Error("Failed to send digest notification",
					"component", "notifications",
					"channel", c.Name,
					"type", c.Type,
					"error", sendErr,
				)
				s.bus.Publish(events.NotificationDeliveryFailedEvent{
					ChannelID:   c.ID,
					ChannelType: c.Type,
					Name:        c.Name,
					Error:       sendErr.Error(),
				})
			} else {
				slog.Debug("Digest notification sent",
					"component", "notifications",
					"channel", c.Name,
					"type", c.Type,
				)
				s.bus.Publish(events.NotificationSentEvent{
					ChannelID:   c.ID,
					ChannelType: c.Type,
					Name:        c.Name,
					TriggerType: "cycle_digest",
				})
			}
		}()
	}
}

// dispatchAlert sends an immediate alert to all enabled channels matching
// the subscription filter.
func (s *NotificationDispatchService) dispatchAlert(alert notifications.Alert, subscribes func(db.NotificationConfig) bool) {
	s.mu.Lock()
	alert.Version = s.version
	s.mu.Unlock()

	configs, err := s.channels.ListEnabled()
	if err != nil {
		slog.Error("Failed to query notification configs for alert", "component", "notifications", "error", err)
		return
	}

	for _, cfg := range configs {
		if !subscribes(cfg) {
			continue
		}

		sender, ok := s.senders[cfg.Type]
		if !ok {
			slog.Warn("Unknown notification channel type", "component", "notifications", "type", cfg.Type)
			continue
		}

		c := cfg
		a := alert
		sc := notifications.SenderConfig{WebhookURL: c.WebhookURL, AppriseTags: c.AppriseTags}
		go func() {
			if sendErr := sender.SendAlert(sc, a); sendErr != nil {
				slog.Error("Failed to send alert notification",
					"component", "notifications",
					"channel", c.Name,
					"type", c.Type,
					"alertType", a.Type,
					"error", sendErr,
				)
				s.bus.Publish(events.NotificationDeliveryFailedEvent{
					ChannelID:   c.ID,
					ChannelType: c.Type,
					Name:        c.Name,
					Error:       sendErr.Error(),
				})
			} else {
				slog.Debug("Alert notification sent",
					"component", "notifications",
					"channel", c.Name,
					"type", c.Type,
					"alertType", a.Type,
				)
				s.bus.Publish(events.NotificationSentEvent{
					ChannelID:   c.ID,
					ChannelType: c.Type,
					Name:        c.Name,
					TriggerType: string(a.Type),
				})
			}
		}()
	}
}

// modeChangedMessage returns a human-friendly explanation of mode change implications.
func modeChangedMessage(newMode string) string {
	switch newMode {
	case notifications.ModeAuto:
		return "Capacitarr will now delete files when the disk threshold is breached."
	case notifications.ModeDryRun:
		return "Capacitarr will now only simulate deletions (no files will be removed)."
	case notifications.ModeApproval:
		return "Capacitarr will now queue items for manual approval before deletion."
	default:
		return fmt.Sprintf("Execution mode changed to %s.", newMode)
	}
}
