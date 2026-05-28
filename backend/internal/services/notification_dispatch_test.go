package services

import (
	"sort"
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

func (m *mockSender) SendDigest(_ notifications.SenderConfig, d notifications.CycleDigest, _ notifications.NotificationTier) error {
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

// boolPtr returns a pointer to the given bool value.
func boolPtr(b bool) *bool { return &b }

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

// makeDigest creates a CycleDigest with a single GroupDigest for testing.
func makeDigest(mode string, evaluated, candidates, deleted int, freedBytes int64, durationMs int64) notifications.CycleDigest {
	return notifications.CycleDigest{
		Groups: []notifications.GroupDigest{
			{
				MountPath:    "/media",
				Mode:         mode,
				Evaluated:    evaluated,
				Candidates:   candidates,
				Deleted:      deleted,
				FreedBytes:   freedBytes,
				DiskUsagePct: 0,
			},
		},
		DurationMs: durationMs,
	}
}

func TestNotificationDispatch_FlushCycleDigest(t *testing.T) {
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "Test Discord", Enabled: true, NotificationLevel: "normal"},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	// FlushCycleDigest is called directly by the poller — no event sequence needed.
	svc.FlushCycleDigest(makeDigest(db.ModeAuto, 100, 3, 3, 3*1073741824, 500))
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 1 {
		t.Fatalf("expected 1 digest, got %d", len(digests))
	}
	if digests[0].TotalEvaluated() != 100 {
		t.Errorf("expected evaluated=100, got %d", digests[0].TotalEvaluated())
	}
	if digests[0].TotalCandidates() != 3 {
		t.Errorf("expected candidates=3, got %d", digests[0].TotalCandidates())
	}
	if digests[0].TotalDeleted() != 3 {
		t.Errorf("expected deleted=3, got %d", digests[0].TotalDeleted())
	}
	if digests[0].TotalFreedBytes() != 3*1073741824 {
		t.Errorf("expected freedBytes=%d, got %d", 3*1073741824, digests[0].TotalFreedBytes())
	}
	// FlushCycleDigest should set the version from the service
	if digests[0].Version != "v1.0.0-test" {
		t.Errorf("expected version='v1.0.0-test', got %q", digests[0].Version)
	}
}

func TestNotificationDispatch_FlushCycleDigest_DryRun(t *testing.T) {
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "Test", Enabled: true, NotificationLevel: "verbose"},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	svc.FlushCycleDigest(makeDigest(db.ModeDryRun, 50, 5, 0, 1073741824, 200))
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 1 {
		t.Fatalf("expected 1 digest, got %d", len(digests))
	}
	if digests[0].PrimaryMode() != db.ModeDryRun {
		t.Errorf("expected execution mode 'dry-run', got %q", digests[0].PrimaryMode())
	}
}

func TestNotificationDispatch_FlushCycleDigest_CollectionGroups(t *testing.T) {
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "Test", Enabled: true, NotificationLevel: "normal"},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	digest := notifications.CycleDigest{
		Groups: []notifications.GroupDigest{
			{
				MountPath:          "/media",
				Mode:               db.ModeAuto,
				Evaluated:          200,
				Candidates:         10,
				Deleted:            10,
				FreedBytes:         5 * 1073741824,
				CollectionsDeleted: 2,
			},
		},
		DurationMs: 1000,
	}
	svc.FlushCycleDigest(digest)
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 1 {
		t.Fatalf("expected 1 digest, got %d", len(digests))
	}
	if digests[0].TotalCollectionsDeleted() != 2 {
		t.Errorf("expected collectionsDeleted=2, got %d", digests[0].TotalCollectionsDeleted())
	}
}

func TestNotificationDispatch_ImmediateAlerts(t *testing.T) {
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "Test", Enabled: true, NotificationLevel: "critical"},
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
	// Channel at "critical" level should NOT receive cycle_digest (which is TierNormal).
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "No Digest", Enabled: true, NotificationLevel: "critical"},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	svc.FlushCycleDigest(makeDigest(db.ModeAuto, 10, 0, 0, 0, 100))
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 0 {
		t.Fatalf("expected 0 digests (channel at critical level), got %d", len(digests))
	}
}

func TestNotificationDispatch_ServerStartedAlert(t *testing.T) {
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "Test", Enabled: true, NotificationLevel: "normal"},
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
	// Channel at critical level should NOT receive approval_activity (TierImportant).
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "No Approval", Enabled: true, NotificationLevel: "critical"},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	svc.bus.Publish(events.ApprovalApprovedEvent{MediaName: "Serenity", MediaType: "movie"})
	time.Sleep(200 * time.Millisecond)

	alerts := mock.getAlerts()
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts (channel at critical level), got %d", len(alerts))
	}
}

func TestNotificationDispatch_ApprovalModeFreedBytes(t *testing.T) {
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "Test", Enabled: true, NotificationLevel: "normal"},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	// In approval mode, FreedBytes comes from the poller's counters.
	svc.FlushCycleDigest(makeDigest(db.ModeApproval, 2232, 80, 0, 5368709120, 11900))
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 1 {
		t.Fatalf("expected 1 digest, got %d", len(digests))
	}
	if digests[0].TotalFreedBytes() != 5368709120 {
		t.Errorf("expected freedBytes=5368709120, got %d", digests[0].TotalFreedBytes())
	}
	if digests[0].PrimaryMode() != db.ModeApproval {
		t.Errorf("expected execution mode 'approval', got %q", digests[0].PrimaryMode())
	}
	if digests[0].TotalCandidates() != 80 {
		t.Errorf("expected candidates=80, got %d", digests[0].TotalCandidates())
	}
}

func TestNotificationDispatch_ApprovalModeDigestSuppressed(t *testing.T) { //nolint:dupl // test structure intentionally similar
	// Override cycle_digest to false for approval-mode digests.
	overrideFalse := boolPtr(false)
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "No Approval Digest", Enabled: true,
				NotificationLevel: "normal", OverrideCycleDigest: overrideFalse},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	svc.FlushCycleDigest(makeDigest(db.ModeApproval, 100, 5, 0, 1073741824, 500))
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 0 {
		t.Fatalf("expected 0 digests (OverrideCycleDigest=false suppresses digests), got %d", len(digests))
	}
}

func TestNotificationDispatch_DryRunDigestSuppressed(t *testing.T) { //nolint:dupl // test structure intentionally similar
	// Channel at "normal" level won't receive dry_run_digest (TierVerbose).
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "No DryRun Digest", Enabled: true,
				NotificationLevel: "normal"},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	svc.FlushCycleDigest(makeDigest(db.ModeDryRun, 100, 5, 0, 1073741824, 500))
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 0 {
		t.Fatalf("expected 0 digests (dry_run_digest needs verbose level), got %d", len(digests))
	}
}

func TestNotificationDispatch_NonDryRunDigestNotAffected(t *testing.T) {
	// Channel at "normal" level should receive auto-mode cycle_digest
	// even though dry_run_digest requires verbose.
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "Test", Enabled: true,
				NotificationLevel: "normal"},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	svc.FlushCycleDigest(makeDigest(db.ModeAuto, 50, 2, 2, 0, 300))
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 1 {
		t.Fatalf("expected 1 digest (auto mode unaffected by dry-run tier), got %d", len(digests))
	}
	if digests[0].PrimaryMode() != db.ModeAuto {
		t.Errorf("expected execution mode 'auto', got %q", digests[0].PrimaryMode())
	}
}

func TestNotificationDispatch_NonApprovalDigestNotAffected(t *testing.T) {
	// Channel at "normal" level should receive auto-mode cycle_digest.
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "Test", Enabled: true,
				NotificationLevel: "normal"},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	svc.FlushCycleDigest(makeDigest(db.ModeAuto, 50, 2, 2, 0, 300))
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 1 {
		t.Fatalf("expected 1 digest (auto mode at normal level), got %d", len(digests))
	}
	if digests[0].PrimaryMode() != db.ModeAuto {
		t.Errorf("expected execution mode 'auto', got %q", digests[0].PrimaryMode())
	}
}

// TestSenderMap_MatchesValidNotificationChannelTypes verifies that the sender
// map in NewNotificationDispatchService stays in sync with
// db.ValidNotificationChannelTypes. If a new channel type is added to the
// validation map but no sender is registered, dispatching will silently skip
// that channel type.
func TestSenderMap_MatchesValidNotificationChannelTypes(t *testing.T) {
	bus := newTestBus(t)
	svc := NewNotificationDispatchService(bus, &mockChannelProvider{}, nil, "v1.0.0-test")

	// Collect sender map keys
	senderKeys := make([]string, 0, len(svc.senders))
	for k := range svc.senders {
		senderKeys = append(senderKeys, k)
	}
	sort.Strings(senderKeys)

	// Collect validation map keys
	validKeys := make([]string, 0, len(db.ValidNotificationChannelTypes))
	for k := range db.ValidNotificationChannelTypes {
		validKeys = append(validKeys, k)
	}
	sort.Strings(validKeys)

	// Every valid channel type must have a sender
	for _, k := range validKeys {
		if svc.senders[k] == nil {
			t.Errorf("channel type %q is in db.ValidNotificationChannelTypes but has no sender in the dispatch service", k)
		}
	}

	// Every sender key must be a valid channel type
	for _, k := range senderKeys {
		if !db.ValidNotificationChannelTypes[k] {
			t.Errorf("sender key %q is registered in dispatch service but missing from db.ValidNotificationChannelTypes", k)
		}
	}

	// Counts must match
	if len(svc.senders) != len(db.ValidNotificationChannelTypes) {
		t.Errorf("sender map has %d entries but ValidNotificationChannelTypes has %d",
			len(svc.senders), len(db.ValidNotificationChannelTypes))
	}
}

func TestNotificationDispatch_AppriseChannel(t *testing.T) {
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "apprise", Name: "Apprise Server", WebhookURL: "http://apprise:8000/api/notify/mykey/",
				AppriseTags: "admin", Enabled: true, NotificationLevel: "normal"},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	svc.FlushCycleDigest(makeDigest(db.ModeAuto, 50, 2, 2, 0, 300))
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 1 {
		t.Fatalf("expected 1 digest from apprise channel, got %d", len(digests))
	}
}

func TestNotificationDispatch_VersionPopulated(t *testing.T) {
	// Verify FlushCycleDigest populates the version from the service,
	// overriding any value in the input digest.
	channels := &mockChannelProvider{
		configs: []db.NotificationConfig{
			{ID: 1, Type: "discord", Name: "Test", Enabled: true, NotificationLevel: "normal"},
		},
	}

	svc, mock := newTestDispatch(t, channels)

	// Pass a digest with a different version — should be overridden
	digest := makeDigest(db.ModeAuto, 10, 0, 0, 0, 0)
	digest.Version = "should-be-overridden"
	svc.FlushCycleDigest(digest)
	time.Sleep(200 * time.Millisecond)

	digests := mock.getDigests()
	if len(digests) != 1 {
		t.Fatalf("expected 1 digest, got %d", len(digests))
	}
	if digests[0].Version != "v1.0.0-test" {
		t.Errorf("expected version='v1.0.0-test', got %q", digests[0].Version)
	}
}
