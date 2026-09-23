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

// --- Shared Helper Tests ---

func TestFilterGroups(t *testing.T) {
	groups := []GroupDigest{
		{MountPath: "/media/tv", Mode: "sunset", SunsetQueued: 8, SunsetExpired: 2, Evaluated: 100},
		{MountPath: "/media/movies", Mode: "auto", Deleted: 5, FreedBytes: 1073741824, Evaluated: 234},
		{MountPath: "/media/music", Mode: "dry-run", Candidates: 3, FreedBytes: 8388608000, Evaluated: 50},
		{MountPath: "/media/photos", Mode: "auto", Evaluated: 80}, // no activity
	}

	t.Run("verbose shows all groups", func(t *testing.T) {
		result := filterGroups(groups, TierVerbose)
		if len(result) != 4 {
			t.Errorf("expected 4 groups, got %d", len(result))
		}
	})

	t.Run("normal excludes dry-run", func(t *testing.T) {
		result := filterGroups(groups, TierNormal)
		if len(result) != 3 {
			t.Errorf("expected 3 groups, got %d", len(result))
		}
		for _, g := range result {
			if g.Mode == "dry-run" {
				t.Error("dry-run group should not appear at normal tier")
			}
		}
	})

	t.Run("important shows only active non-dry-run groups", func(t *testing.T) {
		result := filterGroups(groups, TierImportant)
		if len(result) != 2 {
			t.Errorf("expected 2 groups (sunset + auto with deletions), got %d", len(result))
		}
	})

	t.Run("critical shows no groups", func(t *testing.T) {
		result := filterGroups(groups, TierCritical)
		if len(result) != 0 {
			t.Errorf("expected 0 groups at critical tier, got %d", len(result))
		}
	})

	t.Run("off shows no groups", func(t *testing.T) {
		result := filterGroups(groups, TierOff)
		if len(result) != 0 {
			t.Errorf("expected 0 groups at off tier, got %d", len(result))
		}
	})
}

func TestHasActivity(t *testing.T) {
	tests := []struct {
		name     string
		group    GroupDigest
		expected bool
	}{
		{"deleted items", GroupDigest{Deleted: 3}, true},
		{"candidates", GroupDigest{Candidates: 5}, true},
		{"freed bytes", GroupDigest{FreedBytes: 1024}, true},
		{"sunset queued", GroupDigest{SunsetQueued: 2}, true},
		{"sunset expired", GroupDigest{SunsetExpired: 1}, true},
		{"sunset saved", GroupDigest{SunsetSaved: 1}, true},
		{"escalated items", GroupDigest{EscalatedItems: 1}, true},
		{"queue full skipped", GroupDigest{QueueFullSkipped: 3}, true},
		{"no activity", GroupDigest{Evaluated: 100}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasActivity(tt.group)
			if got != tt.expected {
				t.Errorf("hasActivity() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGroupDescription(t *testing.T) {
	tests := []struct {
		name     string
		group    GroupDigest
		contains string
	}{
		{"auto with deletions", GroupDigest{Mode: "auto", Deleted: 5, Evaluated: 100, FreedBytes: 1073741824}, "Deleted 5 of 100"},
		{"auto all clear", GroupDigest{Mode: "auto", Evaluated: 100}, "all within threshold"},
		{"approval with candidates", GroupDigest{Mode: "approval", Candidates: 3, Evaluated: 50}, "Queued 3 of 50"},
		{"approval all clear", GroupDigest{Mode: "approval", Evaluated: 50}, "all within threshold"},
		{"sunset with queued", GroupDigest{Mode: "sunset", SunsetQueued: 8, SunsetExpired: 2, Evaluated: 100}, "8 items entered sunset"},
		{"sunset all clear", GroupDigest{Mode: "sunset", Evaluated: 100}, "all within threshold"},
		{"dry-run with candidates", GroupDigest{Mode: "dry-run", Candidates: 3, FreedBytes: 8388608000, Evaluated: 50}, "3 candidates"},
		{"dry-run all clear", GroupDigest{Mode: "dry-run", Evaluated: 50}, "all within threshold"},
		{"unknown mode", GroupDigest{Mode: "custom", Evaluated: 10}, "Evaluated 10 items"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := groupDescription(tt.group)
			if !strings.Contains(got, tt.contains) {
				t.Errorf("groupDescription() = %q, want it to contain %q", got, tt.contains)
			}
		})
	}
}

func TestGroupIcon(t *testing.T) {
	tests := []struct {
		mode     string
		expected string
	}{
		{"auto", "🧹"},
		{"approval", "📋"},
		{"sunset", "☀️"},
		{"dry-run", "🔍"},
		{"custom", "📊"},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			got := groupIcon(tt.mode)
			if got != tt.expected {
				t.Errorf("groupIcon(%q) = %q, want %q", tt.mode, got, tt.expected)
			}
		})
	}
}

func TestDigestColor(t *testing.T) {
	t.Run("escalation returns red", func(t *testing.T) {
		groups := []GroupDigest{{EscalatedItems: 2}}
		if got := digestColor(groups); got != ColorRed {
			t.Errorf("expected ColorRed (%d), got %d", ColorRed, got)
		}
	})

	t.Run("deletions return amber", func(t *testing.T) {
		groups := []GroupDigest{{Deleted: 5}}
		if got := digestColor(groups); got != ColorAmber {
			t.Errorf("expected ColorAmber (%d), got %d", ColorAmber, got)
		}
	})

	t.Run("sunset expired returns amber", func(t *testing.T) {
		groups := []GroupDigest{{SunsetExpired: 3}}
		if got := digestColor(groups); got != ColorAmber {
			t.Errorf("expected ColorAmber (%d), got %d", ColorAmber, got)
		}
	})

	t.Run("no activity returns green", func(t *testing.T) {
		groups := []GroupDigest{{Evaluated: 100}}
		if got := digestColor(groups); got != ColorGreen {
			t.Errorf("expected ColorGreen (%d), got %d", ColorGreen, got)
		}
	})
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
		Groups: []GroupDigest{
			{
				MountPath:     "/media",
				Mode:          ModeAuto,
				Evaluated:     847,
				Candidates:    3,
				Deleted:       3,
				FreedBytes:    67108864000, // ~62.5 GB
				DiskUsagePct:  72,
				DiskThreshold: 85,
				DiskTargetPct: 75,
			},
		},
		DurationMs: 1200,
		Version:    "v1.4.0",
	}

	if err := sender.SendDigest(SenderConfig{WebhookURL: srv.URL}, digest, TierNormal); err != nil {
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
	// Auto mode with deletions → amber color
	if embed.Color != ColorAmber {
		t.Errorf("expected color %d (amber), got %d", ColorAmber, embed.Color)
	}
	if !strings.Contains(embed.Description, "/media") {
		t.Error("expected description to contain mount path '/media'")
	}
	if !strings.Contains(embed.Description, "Deleted 3 of 847") {
		t.Error("expected description to contain per-group deletion summary")
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
		Groups: []GroupDigest{
			{
				MountPath:  "/media",
				Mode:       ModeDryRun,
				Evaluated:  847,
				Candidates: 3,
				FreedBytes: 67108864000,
			},
		},
		DurationMs: 1200,
		Version:    "v1.4.0",
	}

	// Dry-run at verbose tier should appear
	if err := sender.SendDigest(SenderConfig{WebhookURL: srv.URL}, digest, TierVerbose); err != nil {
		t.Fatalf("SendDigest returned error: %v", err)
	}

	embed := captured.Embeds[0]
	if embed.Title != "🔍 Dry-Run Complete" {
		t.Errorf("expected title '🔍 Dry-Run Complete', got %q", embed.Title)
	}
	if !strings.Contains(embed.Description, "3 candidates") {
		t.Error("expected description to contain dry-run candidate count")
	}
}

func TestDiscordSender_SendDigest_DryRunFilteredAtNormal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sender := NewDiscordSender()
	digest := CycleDigest{
		Groups: []GroupDigest{
			{
				MountPath:  "/media",
				Mode:       ModeDryRun,
				Evaluated:  847,
				Candidates: 3,
				FreedBytes: 67108864000,
			},
		},
		DurationMs: 1200,
		Version:    "v1.4.0",
	}

	// Dry-run only digest at normal tier → nothing sent (returns nil, no HTTP call)
	err := sender.SendDigest(SenderConfig{WebhookURL: srv.URL}, digest, TierNormal)
	if err != nil {
		t.Fatalf("expected nil return for dry-run at normal tier, got: %v", err)
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
		Groups: []GroupDigest{
			{
				MountPath:  "/media",
				Mode:       ModeAuto,
				Evaluated:  847,
				Candidates: 0,
			},
		},
		DurationMs: 500,
		Version:    "v1.4.0",
	}

	if err := sender.SendDigest(SenderConfig{WebhookURL: srv.URL}, digest, TierNormal); err != nil {
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
		Groups: []GroupDigest{
			{
				MountPath:  "/media",
				Mode:       ModeAuto,
				Evaluated:  100,
				Candidates: 2,
				Deleted:    2,
				FreedBytes: 1073741824,
			},
		},
		DurationMs:      800,
		Version:         "v1.4.0",
		UpdateAvailable: true,
		LatestVersion:   "v1.5.0",
		ReleaseURL:      "https://example.com/releases/v1.5.0",
	}

	if err := sender.SendDigest(SenderConfig{WebhookURL: srv.URL}, digest, TierNormal); err != nil {
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

func TestDiscordSender_SendDigest_MultiGroup(t *testing.T) {
	var captured discordPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &captured)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	sender := NewDiscordSender()
	digest := CycleDigest{
		Groups: []GroupDigest{
			{
				MountPath:     "/media/tv",
				Mode:          "sunset",
				Evaluated:     100,
				SunsetQueued:  8,
				SunsetExpired: 2,
				DiskUsagePct:  62,
				DiskThreshold: 70,
			},
			{
				MountPath:     "/media/movies",
				Mode:          ModeAuto,
				Evaluated:     234,
				Candidates:    5,
				Deleted:       5,
				FreedBytes:    45208219648, // ~42.1 GB
				DiskUsagePct:  72,
				DiskThreshold: 85,
				DiskTargetPct: 75,
			},
			{
				MountPath:  "/media/music",
				Mode:       ModeDryRun,
				Evaluated:  50,
				Candidates: 3,
				FreedBytes: 8388608000,
			},
		},
		DurationMs: 2400,
		Version:    "v3.0.0",
	}

	if err := sender.SendDigest(SenderConfig{WebhookURL: srv.URL}, digest, TierNormal); err != nil {
		t.Fatalf("SendDigest returned error: %v", err)
	}

	embed := captured.Embeds[0]
	// Should contain both non-dry-run groups
	if !strings.Contains(embed.Description, "/media/tv") {
		t.Error("expected description to contain '/media/tv'")
	}
	if !strings.Contains(embed.Description, "/media/movies") {
		t.Error("expected description to contain '/media/movies'")
	}
	// Dry-run should be excluded at normal tier
	if strings.Contains(embed.Description, "/media/music") {
		t.Error("expected dry-run group '/media/music' to be excluded at normal tier")
	}
	// Sunset group description
	if !strings.Contains(embed.Description, "8 items entered sunset") {
		t.Error("expected description to contain sunset group summary")
	}
}

func TestDiscordSender_SendDigest_EmptyGroupsAtTier(t *testing.T) {
	// No HTTP server needed — should return nil without sending
	sender := NewDiscordSender()
	digest := CycleDigest{
		Groups:     []GroupDigest{},
		DurationMs: 100,
		Version:    "v1.0.0",
	}

	err := sender.SendDigest(SenderConfig{WebhookURL: "http://example.com"}, digest, TierNormal)
	if err != nil {
		t.Fatalf("expected nil return for empty groups, got: %v", err)
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
