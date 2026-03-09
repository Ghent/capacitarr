package notifications

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAppriseSender_SendDigest_Format(t *testing.T) {
	var received apprisePayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("failed to decode Apprise payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewAppriseSender()
	digest := CycleDigest{
		ExecutionMode: ModeAuto,
		Evaluated:     42,
		Flagged:       5,
		Deleted:       3,
		Failed:        1,
		FreedBytes:    1073741824, // 1 GB
		DurationMs:    1500,
		DiskUsagePct:  87.5,
		DiskThreshold: 85.0,
		DiskTargetPct: 75.0,
		Version:       "v1.0.0",
	}

	err := sender.SendDigest(SenderConfig{WebhookURL: server.URL}, digest)
	if err != nil {
		t.Fatalf("SendDigest failed: %v", err)
	}

	if received.Title == "" {
		t.Error("expected non-empty title")
	}
	if received.Body == "" {
		t.Error("expected non-empty body")
	}
	if received.Type != "info" {
		t.Errorf("expected type 'info', got %q", received.Type)
	}
	if received.Tag != "" {
		t.Errorf("expected empty tag (no tags configured), got %q", received.Tag)
	}
}

func TestAppriseSender_SendDigest_WithTags(t *testing.T) {
	var received apprisePayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("failed to decode Apprise payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewAppriseSender()
	digest := CycleDigest{
		ExecutionMode: ModeDryRun,
		Evaluated:     100,
		Flagged:       3,
		FreedBytes:    5368709120,
		DurationMs:    2500,
		Version:       "v1.0.0",
	}

	err := sender.SendDigest(SenderConfig{
		WebhookURL:  server.URL,
		AppriseTags: "admin,alerts",
	}, digest)
	if err != nil {
		t.Fatalf("SendDigest failed: %v", err)
	}

	if received.Tag != "admin,alerts" {
		t.Errorf("expected tag 'admin,alerts', got %q", received.Tag)
	}
}

func TestAppriseSender_SendDigest_EmptyURL(t *testing.T) {
	sender := NewAppriseSender()
	err := sender.SendDigest(SenderConfig{WebhookURL: ""}, CycleDigest{})
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestAppriseSender_SendAlert_Format(t *testing.T) {
	var received apprisePayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("failed to decode Apprise payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewAppriseSender()
	alert := Alert{
		Type:    AlertServerStarted,
		Title:   "Capacitarr Started",
		Message: "Serenity server is online",
		Version: "v1.0.0",
	}

	err := sender.SendAlert(SenderConfig{WebhookURL: server.URL}, alert)
	if err != nil {
		t.Fatalf("SendAlert failed: %v", err)
	}

	if received.Title == "" {
		t.Error("expected non-empty title")
	}
	if received.Body == "" {
		t.Error("expected non-empty body")
	}
	if received.Type != "success" {
		t.Errorf("expected type 'success' for server_started, got %q", received.Type)
	}
}

func TestAppriseSender_SendAlert_WithFields(t *testing.T) {
	var received apprisePayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("failed to decode Apprise payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewAppriseSender()
	alert := Alert{
		Type:    AlertThresholdBreached,
		Title:   "🔴 Threshold Breached",
		Message: "Disk usage exceeded threshold.",
		Fields: map[string]string{
			"Mount":     "/mnt/media",
			"Usage":     "87%",
			"Threshold": "85%",
		},
		Version: "v1.0.0",
	}

	err := sender.SendAlert(SenderConfig{
		WebhookURL:  server.URL,
		AppriseTags: "ops",
	}, alert)
	if err != nil {
		t.Fatalf("SendAlert failed: %v", err)
	}

	if received.Type != "failure" {
		t.Errorf("expected type 'failure' for threshold_breached, got %q", received.Type)
	}
	if received.Tag != "ops" {
		t.Errorf("expected tag 'ops', got %q", received.Tag)
	}
}

func TestAppriseSender_SendAlert_EmptyURL(t *testing.T) {
	sender := NewAppriseSender()
	err := sender.SendAlert(SenderConfig{WebhookURL: ""}, Alert{})
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestMapAppriseType(t *testing.T) {
	tests := []struct {
		alertType AlertType
		expected  string
	}{
		{AlertError, "failure"},
		{AlertThresholdBreached, "failure"},
		{AlertModeChanged, "warning"},
		{AlertServerStarted, "success"},
		{AlertUpdateAvailable, "info"},
		{AlertApprovalActivity, "info"},
		{AlertTest, "info"},
		{AlertType("unknown"), "info"},
	}

	for _, tt := range tests {
		t.Run(string(tt.alertType), func(t *testing.T) {
			got := mapAppriseType(tt.alertType)
			if got != tt.expected {
				t.Errorf("mapAppriseType(%q) = %q, want %q", tt.alertType, got, tt.expected)
			}
		})
	}
}
