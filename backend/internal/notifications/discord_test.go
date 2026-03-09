package notifications

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscordSender_SendDigest_Format(t *testing.T) {
	var received discordPayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("failed to decode Discord payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewDiscordSender()
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

	if len(received.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(received.Embeds))
	}

	embed := received.Embeds[0]
	if embed.Title == "" {
		t.Error("expected non-empty embed title")
	}
	if embed.Color == 0 {
		t.Error("expected non-zero embed color")
	}
}

func TestDiscordSender_SendAlert_Format(t *testing.T) {
	var received discordPayload

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("failed to decode Discord payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	sender := NewDiscordSender()
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

	if len(received.Embeds) != 1 {
		t.Fatalf("expected 1 embed, got %d", len(received.Embeds))
	}

	embed := received.Embeds[0]
	if embed.Title != "Capacitarr Started" {
		t.Errorf("expected title 'Capacitarr Started', got %q", embed.Title)
	}
	// Verify the author line includes the trigger label
	if embed.Author == nil {
		t.Fatal("expected non-nil author")
	}
	expectedAuthor := "⚡ Capacitarr v1.0.0 • Server Started"
	if embed.Author.Name != expectedAuthor {
		t.Errorf("expected author name %q, got %q", expectedAuthor, embed.Author.Name)
	}
}
