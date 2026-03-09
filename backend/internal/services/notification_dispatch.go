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

// NotificationDispatchService subscribes to the event bus and dispatches
// notifications via the Sender interface. It accumulates cycle events using
// a two-gate flush pattern (EngineComplete + DeletionBatchComplete) before
// building and sending a single digest notification per engine cycle.
//
// Immediate alerts (errors, mode changes, server started, threshold breached,
// update available) are dispatched without waiting for the cycle gates.
type NotificationDispatchService struct {
	bus            *events.EventBus
	channels       ChannelProvider
	versionChecker VersionChecker
	senders        map[string]notifications.Sender
	version        string

	mu          sync.Mutex
	accumulator *cycleAccumulator
	ch          chan events.Event
	done        chan struct{}
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

// Start subscribes to the event bus and begins the background dispatch loop.
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
	case events.EngineStartEvent:
		s.mu.Lock()
		s.accumulator = newCycleAccumulator(e.ExecutionMode)
		s.mu.Unlock()

	case events.EngineCompleteEvent:
		s.mu.Lock()
		if s.accumulator != nil {
			s.accumulator.evaluated = e.Evaluated
			s.accumulator.flagged = e.Flagged
			s.accumulator.durationMs = e.DurationMs
			s.accumulator.executionMode = e.ExecutionMode
			s.accumulator.engineComplete = true
			// Use FreedBytes from the engine event for approval/dry-run modes where
			// no individual DeletionSuccess/DeletionDryRun events carry size data.
			if e.FreedBytes > 0 && s.accumulator.totalFreedBytes == 0 {
				s.accumulator.totalFreedBytes = e.FreedBytes
			}
		}
		s.mu.Unlock()
		s.tryFlush()

	case events.DeletionSuccessEvent:
		s.mu.Lock()
		if s.accumulator != nil {
			s.accumulator.deletedCount++
			s.accumulator.totalFreedBytes += e.SizeBytes
		}
		s.mu.Unlock()

	case events.DeletionDryRunEvent:
		s.mu.Lock()
		if s.accumulator != nil {
			s.accumulator.totalFreedBytes += e.SizeBytes
		}
		s.mu.Unlock()

	case events.DeletionFailedEvent:
		s.mu.Lock()
		if s.accumulator != nil {
			s.accumulator.failedCount++
		}
		s.mu.Unlock()

	case events.DeletionBatchCompleteEvent:
		s.mu.Lock()
		if s.accumulator != nil {
			s.accumulator.batchComplete = true
			// Use the batch-level counts if the accumulator doesn't have them
			// (e.g., for the zero-items case)
			if s.accumulator.deletedCount == 0 && s.accumulator.failedCount == 0 {
				s.accumulator.deletedCount = e.Succeeded
				s.accumulator.failedCount = e.Failed
			}
		}
		s.mu.Unlock()
		s.tryFlush()

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
	}
}

// tryFlush checks whether both gates are open and, if so, builds and
// dispatches the cycle digest notification.
func (s *NotificationDispatchService) tryFlush() {
	s.mu.Lock()
	acc := s.accumulator
	if acc == nil || !acc.engineComplete || !acc.batchComplete {
		s.mu.Unlock()
		return
	}
	// Consume the accumulator
	s.accumulator = nil
	ver := s.version
	vc := s.versionChecker
	s.mu.Unlock()

	digest := acc.buildDigest(ver)

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
// subscribe to OnCycleDigest. For approval-mode digests, channels must
// also subscribe to OnApprovalActivity — users who disable "Approval
// Activity" expect all approval-related notifications to be silenced,
// including the cycle digest that summarises items queued for approval.
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

// =============================================================================
// cycleAccumulator — unexported helper that tracks events within a single
// engine cycle, implementing the two-gate flush pattern.
// =============================================================================

type cycleAccumulator struct {
	executionMode string

	// Gates
	engineComplete bool // gate 1: EngineCompleteEvent received
	batchComplete  bool // gate 2: DeletionBatchCompleteEvent received

	// Engine stats (from EngineCompleteEvent)
	evaluated  int
	flagged    int
	durationMs int64

	// Deletion accumulation (from DeletionSuccess/Failed/DryRun events)
	deletedCount    int
	failedCount     int
	totalFreedBytes int64
}

func newCycleAccumulator(executionMode string) *cycleAccumulator {
	return &cycleAccumulator{
		executionMode: executionMode,
	}
}

func (a *cycleAccumulator) buildDigest(version string) notifications.CycleDigest {
	return notifications.CycleDigest{
		ExecutionMode: a.executionMode,
		Evaluated:     a.evaluated,
		Flagged:       a.flagged,
		Deleted:       a.deletedCount,
		Failed:        a.failedCount,
		FreedBytes:    a.totalFreedBytes,
		DurationMs:    a.durationMs,
		Version:       version,
	}
}
