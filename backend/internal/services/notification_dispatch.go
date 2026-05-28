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

// eventKindError is the event kind string for error-class notifications,
// matching the key in notifications.EventTier.
const eventKindError = "error"

// maxConcurrentNotifications limits the number of concurrent notification
// deliveries. If a webhook endpoint is slow and events arrive rapidly,
// this prevents unbounded goroutine accumulation. Each delivery goroutine
// must acquire a slot from this semaphore before sending.
const maxConcurrentNotifications = 5

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

// digestEnrichment accumulates sunset expired/saved counts between
// EngineStartEvent and FlushCycleDigest so the digest can include sunset stats.
type digestEnrichment struct {
	mu      sync.Mutex
	expired map[uint]int // diskGroupID → count
	saved   map[uint]int
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
	enrichment     digestEnrichment

	mu   sync.Mutex
	ch   chan events.Event
	done chan struct{}
	sem  chan struct{} // bounded semaphore for concurrent notification deliveries
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
		sem:            make(chan struct{}, maxConcurrentNotifications),
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
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic recovered in notification dispatch loop",
				"component", "notifications", "panic", r)
		}
	}()
	for event := range s.ch {
		s.handle(event)
	}
}

func (s *NotificationDispatchService) handle(event events.Event) {
	switch e := event.(type) {
	case events.EngineStartEvent:
		// Reset digest enrichment accumulators for the new cycle.
		s.enrichment.mu.Lock()
		s.enrichment.expired = make(map[uint]int)
		s.enrichment.saved = make(map[uint]int)
		s.enrichment.mu.Unlock()
		_ = e // consumed for side-effect only

	case events.EngineErrorEvent:
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertError,
			Title:   "🔴 Engine Error",
			Message: "The evaluation engine failed. Check the application logs for details.",
		}, eventKindError)

	case events.ServerStartedEvent:
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertServerStarted,
			Title:   "🚀 Capacitarr is online",
			Message: "",
		}, "server_started")

	case events.ThresholdBreachedEvent:
		bar := notifications.ProgressBar(e.CurrentPct, 20)
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertThresholdBreached,
			Title:   "🔴 Threshold Breached",
			Message: fmt.Sprintf("`%s` **%.0f%%** / %.0f%%\nTarget: **%.0f%%**", bar, e.CurrentPct, e.ThresholdPct, e.TargetPct),
		}, "threshold_breached")

	case events.UpdateAvailableEvent:
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertUpdateAvailable,
			Title:   fmt.Sprintf("📦 Update Available: **%s**", e.LatestVersion),
			Message: fmt.Sprintf("[View Release Notes](%s)", e.ReleaseURL),
		}, "update_available")

	case events.ApprovalApprovedEvent:
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertApprovalActivity,
			Title:   "✅ Approved for Deletion",
			Message: fmt.Sprintf("**%d** item(s) approved — queued for deletion", 1),
		}, "approval_activity")

	case events.ApprovalRejectedEvent:
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertApprovalActivity,
			Title:   "😴 Item Snoozed",
			Message: fmt.Sprintf("Snoozed for %s", e.SnoozeDuration),
		}, "approval_activity")

	case events.IntegrationTestFailedEvent:
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertIntegrationStatus,
			Title:   fmt.Sprintf("🔴 Integration Down: %s", e.Name),
			Message: fmt.Sprintf("**%s** (%s) failed connection test:\n%s", e.Name, e.IntegrationType, e.Error),
		}, "integration_down")

	case events.IntegrationRecoveredEvent:
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertIntegrationStatus,
			Title:   fmt.Sprintf("🟢 Integration Recovered: %s", e.Name),
			Message: fmt.Sprintf("**%s** (%s) is back online", e.Name, e.IntegrationType),
		}, "integration_recovery")

		// Recovery attempt events are intentionally NOT dispatched to external
		// notification channels — they fire frequently during probing and would
		// spam Discord/Apprise. The IntegrationRecoveredEvent above handles the
		// one-time "back online" notification. Recovery attempts flow through
		// SSE to the frontend for real-time progress display only.

	case events.SunsetEscalatedEvent:
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertThresholdBreached,
			Title:   fmt.Sprintf("Threshold Breached — disk group %d", e.DiskGroupID),
			Message: fmt.Sprintf("Sunset escalation: %d items force-expired to free %s", e.ItemsExpired, notifications.HumanSize(e.BytesFreed)),
			Version: s.version,
		}, "threshold_breached")

	case events.SunsetMisconfiguredEvent:
		s.dispatchAlert(notifications.Alert{
			Type:    notifications.AlertError,
			Title:   "Sunset Misconfigured — " + e.MountPath,
			Message: "Sunset mode skipped — sunset threshold not configured.",
			Version: s.version,
		}, eventKindError)

	case events.SunsetExpiredEvent:
		s.enrichment.mu.Lock()
		s.enrichment.expired[e.DiskGroupID]++
		s.enrichment.mu.Unlock()

	case events.SunsetSavedEvent:
		s.enrichment.mu.Lock()
		s.enrichment.saved[e.DiskGroupID]++
		s.enrichment.mu.Unlock()
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
// pass tier resolution for the "cycle_digest" event kind. Dry-run digests
// use "dry_run_digest" which maps to TierVerbose, so only verbose channels
// receive them unless overridden.
func (s *NotificationDispatchService) dispatchDigest(digest notifications.CycleDigest) {
	configs, err := s.channels.ListEnabled()
	if err != nil {
		slog.Error("Failed to query notification configs for digest", "component", "notifications", "error", err)
		return
	}

	for _, cfg := range configs {
		eventKind := "cycle_digest"
		if digest.PrimaryMode() == notifications.ModeDryRun {
			eventKind = "dry_run_digest"
		}
		level := notifications.ParseLevel(cfg.NotificationLevel)
		override := s.resolveOverride(cfg, eventKind)
		if !notifications.ShouldNotify(level, eventKind, override) {
			continue
		}

		sender, ok := s.senders[cfg.Type]
		if !ok {
			slog.Warn("Unknown notification channel type", "component", "notifications", "type", cfg.Type)
			continue
		}

		c := cfg
		d := digest
		lvl := level
		snd := sender
		sc := notifications.SenderConfig{WebhookURL: c.WebhookURL, AppriseTags: c.AppriseTags}
		go func() {
			s.sem <- struct{}{} // acquire semaphore slot
			defer func() { <-s.sem }()
			defer func() {
				if r := recover(); r != nil {
					slog.Error("Panic recovered in digest notification sender",
						"component", "notifications", "channel", c.Name, "panic", r)
				}
			}()
			if sendErr := snd.SendDigest(sc, d, lvl); sendErr != nil {
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
					TriggerType: eventKind,
				})
			}
		}()
	}
}

// dispatchAlert sends an immediate alert to all enabled channels that pass
// tier resolution for the given event kind.
func (s *NotificationDispatchService) dispatchAlert(alert notifications.Alert, eventKind string) {
	s.mu.Lock()
	alert.Version = s.version
	s.mu.Unlock()

	configs, err := s.channels.ListEnabled()
	if err != nil {
		slog.Error("Failed to query notification configs for alert", "component", "notifications", "error", err)
		return
	}

	for _, cfg := range configs {
		level := notifications.ParseLevel(cfg.NotificationLevel)
		override := s.resolveOverride(cfg, eventKind)
		if !notifications.ShouldNotify(level, eventKind, override) {
			continue
		}

		sender, ok := s.senders[cfg.Type]
		if !ok {
			slog.Warn("Unknown notification channel type", "component", "notifications", "type", cfg.Type)
			continue
		}

		c := cfg
		a := alert
		snd := sender
		sc := notifications.SenderConfig{WebhookURL: c.WebhookURL, AppriseTags: c.AppriseTags}
		go func() {
			s.sem <- struct{}{} // acquire semaphore slot
			defer func() { <-s.sem }()
			defer func() {
				if r := recover(); r != nil {
					slog.Error("Panic recovered in alert notification sender",
						"component", "notifications", "channel", c.Name, "panic", r)
				}
			}()
			if sendErr := snd.SendAlert(sc, a); sendErr != nil {
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

// resolveOverride returns the per-event override for a given notification
// channel config and event kind. Returns nil when no override applies,
// letting the tier-based default take effect.
func (s *NotificationDispatchService) resolveOverride(cfg db.NotificationConfig, eventKind string) *bool {
	switch eventKind {
	case "cycle_digest", "dry_run_digest":
		return cfg.OverrideCycleDigest
	case eventKindError:
		return cfg.OverrideError
	case "mode_changed":
		return cfg.OverrideModeChanged
	case "server_started":
		return cfg.OverrideServerStarted
	case "threshold_breached":
		return cfg.OverrideThresholdBreach
	case "update_available":
		return cfg.OverrideUpdateAvailable
	case "approval_activity":
		return cfg.OverrideApprovalActivity
	case "integration_down", "integration_recovery":
		return cfg.OverrideIntegrationStatus
	default:
		return nil
	}
}
