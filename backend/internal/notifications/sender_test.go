package notifications

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- Helper Functions Tests ---

func TestHumanSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{67108864000, "62.5 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tt := range tests {
		got := HumanSize(tt.bytes)
		if got != tt.expected {
			t.Errorf("HumanSize(%d) = %q, want %q", tt.bytes, got, tt.expected)
		}
	}
}

func TestProgressBar(t *testing.T) {
	tests := []struct {
		pct      float64
		width    int
		expected string
	}{
		{0, 10, "░░░░░░░░░░"},
		{50, 10, "▓▓▓▓▓░░░░░"},
		{100, 10, "▓▓▓▓▓▓▓▓▓▓"},
		{75, 20, "▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓░░░░░"},
		{-10, 5, "░░░░░"},
		{150, 5, "▓▓▓▓▓"},
	}

	for _, tt := range tests {
		got := ProgressBar(tt.pct, tt.width)
		if got != tt.expected {
			t.Errorf("ProgressBar(%.0f, %d) = %q, want %q", tt.pct, tt.width, got, tt.expected)
		}
	}
}

// --- Discord Sender Tests ---

func TestDiscordSender_SendDigest_AutoMode(t *testing.T) {
	var captured discordPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sender := NewDiscordSender()
	digest := CycleDigest{
		ExecutionMode: "auto",
		Evaluated:     847,
		Flagged:       3,
		Deleted:       3,
		FreedBytes:    67108864000, // ~62.5 GB
		DurationMs:    1200,
		DiskUsagePct:  72,
		DiskThreshold: 85,
		DiskTargetPct: 75,
		Version:       "v1.4.0",
	}

	if err := sender.SendDigest(SenderConfig{WebhookURL: srv.URL}, digest); err != nil {
		t.Fatalf("SendDigest returned error: %v", err)
	}

	if len(captured.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(captured.Embeds))
	}

	embed := captured.Embeds[0]
	if embed.Author == nil || embed.Author.Name == "" {
		t.Error("expected non-empty author")
	}
	if embed.Title != titleCleanupComplete {
		t.Errorf("expected title %q, got %q", titleCleanupComplete, embed.Title)
	}
	if embed.Color != ColorGreen {
		t.Errorf("expected color %d (green), got %d", ColorGreen, embed.Color)
	}
}

func TestDiscordSender_SendDigest_DryRunMode(t *testing.T) {
	var captured discordPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sender := NewDiscordSender()
	digest := CycleDigest{
		ExecutionMode: "dry-run",
		Evaluated:     847,
		Flagged:       3,
		FreedBytes:    67108864000,
		DurationMs:    1200,
		Version:       "v1.4.0",
	}

	if err := sender.SendDigest(SenderConfig{WebhookURL: srv.URL}, digest); err != nil {
		t.Fatalf("SendDigest returned error: %v", err)
	}

	embed := captured.Embeds[0]
	if embed.Title != "🔍 Dry-Run Complete" {
		t.Errorf("expected title '🔍 Dry-Run Complete', got %q", embed.Title)
	}
	if embed.Color != ColorBlue {
		t.Errorf("expected color %d (blue), got %d", ColorBlue, embed.Color)
	}
}

func TestDiscordSender_SendDigest_AllClear(t *testing.T) {
	var captured discordPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sender := NewDiscordSender()
	digest := CycleDigest{
		ExecutionMode: "auto",
		Evaluated:     847,
		Flagged:       0,
		DurationMs:    500,
		Version:       "v1.4.0",
	}

	if err := sender.SendDigest(SenderConfig{WebhookURL: srv.URL}, digest); err != nil {
		t.Fatalf("SendDigest returned error: %v", err)
	}

	embed := captured.Embeds[0]
	if embed.Title != titleAllClear {
		t.Errorf("expected title %q, got %q", titleAllClear, embed.Title)
	}
	if embed.Color != ColorGreen {
		t.Errorf("expected color %d (green), got %d", ColorGreen, embed.Color)
	}
}

func TestDiscordSender_SendDigest_WithUpdateBanner(t *testing.T) {
	var captured discordPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sender := NewDiscordSender()
	digest := CycleDigest{
		ExecutionMode:   "auto",
		Evaluated:       100,
		Flagged:         2,
		Deleted:         2,
		FreedBytes:      1073741824,
		DurationMs:      800,
		Version:         "v1.4.0",
		UpdateAvailable: true,
		LatestVersion:   "v1.5.0",
		ReleaseURL:      "https://example.com/releases/v1.5.0",
	}

	if err := sender.SendDigest(SenderConfig{WebhookURL: srv.URL}, digest); err != nil {
		t.Fatalf("SendDigest returned error: %v", err)
	}

	embed := captured.Embeds[0]
	if embed.Description == "" {
		t.Error("expected non-empty description")
	}
	// Check that the update banner is present in the description
	if !strings.Contains(embed.Description, "v1.5.0") {
		t.Error("expected description to contain update version 'v1.5.0'")
	}
}

func TestDiscordSender_SendAlert(t *testing.T) {
	tests := []struct {
		name          string
		alertType     AlertType
		expectedColor int
	}{
		{"error", AlertError, ColorRed},
		{"mode_changed", AlertModeChanged, ColorOrange},
		{"server_started", AlertServerStarted, ColorGreen},
		{"threshold_breached", AlertThresholdBreached, ColorRed},
		{"update_available", AlertUpdateAvailable, ColorBlue},
		{"test", AlertTest, ColorBlue},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured discordPayload
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &captured)
				w.WriteHeader(http.StatusNoContent)
			}))
			defer srv.Close()

			sender := NewDiscordSender()
			alert := Alert{
				Type:    tt.alertType,
				Title:   "Test Alert",
				Message: "Test message",
				Version: "v1.4.0",
			}

			if err := sender.SendAlert(SenderConfig{WebhookURL: srv.URL}, alert); err != nil {
				t.Fatalf("SendAlert returned error: %v", err)
			}

			if len(captured.Embeds) != 1 {
				t.Fatalf("expected 1 embed, got %d", len(captured.Embeds))
			}
			if captured.Embeds[0].Color != tt.expectedColor {
				t.Errorf("expected color %d, got %d", tt.expectedColor, captured.Embeds[0].Color)
			}
		})
	}
}

// --- TriggerLabel Tests ---

func TestTriggerLabel(t *testing.T) {
	tests := []struct {
		alertType AlertType
		expected  string
	}{
		{AlertError, "Engine Error"},
		{AlertModeChanged, "Mode Change"},
		{AlertServerStarted, "Server Started"},
		{AlertThresholdBreached, "Threshold Breached"},
		{AlertUpdateAvailable, "Update Available"},
		{AlertApprovalActivity, "Approval Activity"},
		{AlertTest, "Test"},
		{AlertType("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.alertType), func(t *testing.T) {
			got := TriggerLabel(tt.alertType)
			if got != tt.expected {
				t.Errorf("TriggerLabel(%q) = %q, want %q", tt.alertType, got, tt.expected)
			}
		})
	}
}
