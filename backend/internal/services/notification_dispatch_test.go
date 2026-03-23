package services

import (
	"sync"
	"testing"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/notifications"
)

// mockChannelProvider implements ChannelProvider for dispatch tests.
type mockChannelProvider struct {
	configs []db.NotificationConfig
}

func (m *mockChannelProvider) ListEnabled() ([]db.NotificationConfig, error) {
	return m.configs, nil
}

func (m *mockChannelProvider) GetByID(id uint) (*db.NotificationConfig, error) {
	for _, c := range m.configs {
		if c.ID == id {
			return &c, nil
		}
	}
	return nil, ErrNotFound
}

// mockSender captures payloads for test assertions.
type mockSender struct {
	mu      sync.Mutex
	digests []notifications.CycleDigest
	alerts  []notifications.Alert
}

func (m *mockSender) SendDigest(_ notifications.SenderConfig, d notifications.CycleDigest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.digests = append(m.digests, d)
	return nil
}

func (m *mockSender) SendAlert(_ notifications.SenderConfig, a notifications.Alert) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts = append(m.alerts, a)
	return nil
}

func (m *mockSender) getDigests() []notifications.CycleDigest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]notifications.CycleDigest{}, m.digests...)
}

func (m *mockSender) getAlerts() []notifications.Alert {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]notifications.Alert{}, m.alerts...)
}

// newTestDispatch creates a dispatch service with mock senders for
// external channels (discord/apprise). Returns the service and the mock sender.
func newTestDispatch(t *testing.T, channels *mockChannelProvider) (*NotificationDispatchService, *mockSender) {
	t.Helper()
	bus := newTestBus(t)
	svc := NewNotificationDispatchService(bus, channels, nil, "v1.0.0-test")

	// Replace senders with a mock so assertions can inspect payloads.
	externalMock := &mockSender{}
	svc.senders = map[string]notifications.Sender{
		"discord": externalMock,
		"apprise": externalMock,
	}

	svc.Start()
	t.Cleanup(func() { svc.Stop() })

	return svc, externalMock
}

func TestNotificationDispatch_TwoGateFlush(t *testing.T) {
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "Test Discord", Enabled: true, OnCycleDigest: true},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	// Simulate a full engine cycle
	svc.bus.Publish(events.EngineStartEvent{ExecutionMode: db.ModeAuto})
	time.Sleep(50 * time.Millisecond)

	svc.bus.Publish(events.EngineCompleteEvent{
		Evaluated:     100,
		Candidates:    3,
		DurationMs:    500,
		ExecutionMode: db.ModeAuto,
	})
	time.Sleep(50 * time.Millisecond)

	// Gate 2 — no deletion events, just batch complete
	svc.bus.Publish(events.DeletionBatchCompleteEvent{Succeeded: 3, Failed: 0})
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 1 {
		t.Fatalf("expected 1 digest, got %d", len(digests))
	}
	if digests[0].Evaluated != 100 {
		t.Errorf("expected evaluated=100, got %d", digests[0].Evaluated)
	}
	if digests[0].Candidates != 3 {
		t.Errorf("expected candidates=3, got %d", digests[0].Candidates)
	}
}

func TestNotificationDispatch_ReverseGateOrder(t *testing.T) {
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "Test", Enabled: true, OnCycleDigest: true},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	svc.bus.Publish(events.EngineStartEvent{ExecutionMode: db.ModeDryRun})
	time.Sleep(50 * time.Millisecond)

	// Gate 2 fires first
	svc.bus.Publish(events.DeletionBatchCompleteEvent{Succeeded: 0, Failed: 0})
	time.Sleep(50 * time.Millisecond)

	// Gate 1 fires second — should trigger flush
	svc.bus.Publish(events.EngineCompleteEvent{
		Evaluated:     50,
		Candidates:    0,
		DurationMs:    200,
		ExecutionMode: db.ModeDryRun,
	})
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 1 {
		t.Fatalf("expected 1 digest (reverse gate order), got %d", len(digests))
	}
	if digests[0].ExecutionMode != db.ModeDryRun {
		t.Errorf("expected execution mode 'dry-run', got %q", digests[0].ExecutionMode)
	}
}

func TestNotificationDispatch_Accumulation(t *testing.T) {
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "Test", Enabled: true, OnCycleDigest: true},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	svc.bus.Publish(events.EngineStartEvent{ExecutionMode: db.ModeAuto})
	time.Sleep(50 * time.Millisecond)

	// 3 successful deletions
	for i := 0; i < 3; i++ {
		svc.bus.Publish(events.DeletionSuccessEvent{
			MediaName: "Serenity",
			MediaType: "movie",
			SizeBytes: 1073741824, // 1 GB each
		})
	}
	time.Sleep(50 * time.Millisecond)

	svc.bus.Publish(events.EngineCompleteEvent{
		Evaluated:     200,
		Candidates:    3,
		DurationMs:    1000,
		ExecutionMode: db.ModeAuto,
	})
	time.Sleep(50 * time.Millisecond)

	svc.bus.Publish(events.DeletionBatchCompleteEvent{Succeeded: 3, Failed: 0})
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 1 {
		t.Fatalf("expected 1 digest, got %d", len(digests))
	}
	if digests[0].Deleted != 3 {
		t.Errorf("expected deleted=3, got %d", digests[0].Deleted)
	}
	if digests[0].FreedBytes != 3*1073741824 {
		t.Errorf("expected freedBytes=%d, got %d", 3*1073741824, digests[0].FreedBytes)
	}
}

func TestNotificationDispatch_ImmediateAlerts(t *testing.T) {
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "Test", Enabled: true,
				OnError: true, OnModeChanged: true, OnServerStarted: true,
				OnThresholdBreach: true, OnUpdateAvailable: true},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	// EngineErrorEvent → immediate alert
	svc.bus.Publish(events.EngineErrorEvent{Error: "test error"})
	time.Sleep(200 * time.Millisecond)

	alerts := mock.getAlerts()
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert after EngineErrorEvent, got %d", len(alerts))
	}
	if alerts[0].Type != notifications.AlertError {
		t.Errorf("expected alert type 'error', got %q", alerts[0].Type)
	}
}

func TestNotificationDispatch_SubscriptionFiltering(t *testing.T) {
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "No Digest", Enabled: true, OnCycleDigest: false},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	svc.bus.Publish(events.EngineStartEvent{ExecutionMode: db.ModeAuto})
	time.Sleep(50 * time.Millisecond)
	svc.bus.Publish(events.EngineCompleteEvent{Evaluated: 10, Candidates: 0, DurationMs: 100, ExecutionMode: db.ModeAuto})
	time.Sleep(50 * time.Millisecond)
	svc.bus.Publish(events.DeletionBatchCompleteEvent{Succeeded: 0, Failed: 0})
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 0 {
		t.Fatalf("expected 0 digests (channel has OnCycleDigest=false), got %d", len(digests))
	}
}

func TestNotificationDispatch_ModeChangedAlert(t *testing.T) {
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "Test", Enabled: true, OnModeChanged: true},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	svc.bus.Publish(events.EngineModeChangedEvent{OldMode: db.ModeDryRun, NewMode: db.ModeAuto})
	time.Sleep(200 * time.Millisecond)

	alerts := mock.getAlerts()
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert for mode change, got %d", len(alerts))
	}
	if alerts[0].Type != notifications.AlertModeChanged {
		t.Errorf("expected alert type 'mode_changed', got %q", alerts[0].Type)
	}
}

func TestNotificationDispatch_ServerStartedAlert(t *testing.T) {
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "Test", Enabled: true, OnServerStarted: true},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	svc.bus.Publish(events.ServerStartedEvent{Version: "v1.0.0"})
	time.Sleep(200 * time.Millisecond)

	alerts := mock.getAlerts()
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert for server started, got %d", len(alerts))
	}
	if alerts[0].Type != notifications.AlertServerStarted {
		t.Errorf("expected alert type 'server_started', got %q", alerts[0].Type)
	}
}

func TestNotificationDispatch_ApprovalActivityFiltering(t *testing.T) {
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "No Approval", Enabled: true, OnApprovalActivity: false},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	svc.bus.Publish(events.ApprovalApprovedEvent{MediaName: "Serenity", MediaType: "movie"})
	time.Sleep(200 * time.Millisecond)

	alerts := mock.getAlerts()
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (channel has OnApprovalActivity=false), got %d", len(alerts))
	}
}

func TestNotificationDispatch_ApprovalModeFreedBytes(t *testing.T) {
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "Test", Enabled: true, OnCycleDigest: true, OnApprovalActivity: true},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	svc.bus.Publish(events.EngineStartEvent{ExecutionMode: db.ModeApproval})
	time.Sleep(50 * time.Millisecond)

	// In approval mode, no DeletionDryRun/DeletionSuccess events are published.
	// FreedBytes comes from the EngineCompleteEvent instead.
	svc.bus.Publish(events.EngineCompleteEvent{
		Evaluated:     2232,
		Candidates:    80,
		DurationMs:    11900,
		ExecutionMode: db.ModeApproval,
		FreedBytes:    5368709120, // ~5 GB potential savings
	})
	time.Sleep(50 * time.Millisecond)

	svc.bus.Publish(events.DeletionBatchCompleteEvent{Succeeded: 0, Failed: 0})
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 1 {
		t.Fatalf("expected 1 digest, got %d", len(digests))
	}
	if digests[0].FreedBytes != 5368709120 {
		t.Errorf("expected freedBytes=5368709120, got %d", digests[0].FreedBytes)
	}
	if digests[0].ExecutionMode != db.ModeApproval {
		t.Errorf("expected execution mode 'approval', got %q", digests[0].ExecutionMode)
	}
	if digests[0].Candidates != 80 {
		t.Errorf("expected candidates=80, got %d", digests[0].Candidates)
	}
}

func TestNotificationDispatch_ApprovalModeDigestSuppressed(t *testing.T) { //nolint:dupl // test structure intentionally similar
	// When OnApprovalActivity=false, approval-mode cycle digests should be
	// suppressed — users who turn off "Approval Activity" expect ALL
	// approval-related notifications to stop, including the engine cycle
	// digest that says "Items Queued for Approval".
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "No Approval Digest", Enabled: true,
				OnCycleDigest: true, OnApprovalActivity: false},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	svc.bus.Publish(events.EngineStartEvent{ExecutionMode: db.ModeApproval})
	time.Sleep(50 * time.Millisecond)

	svc.bus.Publish(events.EngineCompleteEvent{
		Evaluated:     100,
		Candidates:    5,
		DurationMs:    500,
		ExecutionMode: db.ModeApproval,
		FreedBytes:    1073741824,
	})
	time.Sleep(50 * time.Millisecond)

	svc.bus.Publish(events.DeletionBatchCompleteEvent{Succeeded: 0, Failed: 0})
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 0 {
		t.Fatalf("expected 0 digests (OnApprovalActivity=false suppresses approval-mode digests), got %d", len(digests))
	}
}

func TestNotificationDispatch_NonApprovalDigestNotAffected(t *testing.T) {
	// When OnApprovalActivity=false, non-approval-mode digests (auto, dry-run)
	// should still be delivered normally.
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "Test", Enabled: true,
				OnCycleDigest: true, OnApprovalActivity: false},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	// Run an auto-mode cycle
	svc.bus.Publish(events.EngineStartEvent{ExecutionMode: db.ModeAuto})
	time.Sleep(50 * time.Millisecond)

	svc.bus.Publish(events.EngineCompleteEvent{
		Evaluated:     50,
		Candidates:    2,
		DurationMs:    300,
		ExecutionMode: db.ModeAuto,
	})
	time.Sleep(50 * time.Millisecond)

	svc.bus.Publish(events.DeletionBatchCompleteEvent{Succeeded: 2, Failed: 0})
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 1 {
		t.Fatalf("expected 1 digest (auto mode unaffected by OnApprovalActivity=false), got %d", len(digests))
	}
	if digests[0].ExecutionMode != db.ModeAuto {
		t.Errorf("expected execution mode 'auto', got %q", digests[0].ExecutionMode)
	}
}

func TestNotificationDispatch_AppriseChannel(t *testing.T) {
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "apprise", Name: "Apprise Server", WebhookURL: "http://apprise:8000/api/notify/mykey/",
				AppriseTags: "admin", Enabled: true, OnCycleDigest: true},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	svc.bus.Publish(events.EngineStartEvent{ExecutionMode: db.ModeAuto})
	time.Sleep(50 * time.Millisecond)

	svc.bus.Publish(events.EngineCompleteEvent{
		Evaluated:     50,
		Candidates:    2,
		DurationMs:    300,
		ExecutionMode: db.ModeAuto,
	})
	time.Sleep(50 * time.Millisecond)

	svc.bus.Publish(events.DeletionBatchCompleteEvent{Succeeded: 2, Failed: 0})
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 1 {
		t.Fatalf("expected 1 digest from apprise channel, got %d", len(digests))
	}
}
