// Package notifications dispatches alerts via Discord and Apprise channels.
package notifications

import (
	"encoding/json"
	"fmt"
)

// DiscordSender implements Sender for Discord webhook delivery using rich embeds.
type DiscordSender struct{}

// NewDiscordSender creates a new DiscordSender.
func NewDiscordSender() *DiscordSender {
	return &DiscordSender{}
}

// discordPayload matches the Discord webhook embed structure.
type discordPayload struct {
	Embeds []discordEmbed `json:"embeds"`
}

type discordEmbed struct {
	Author      *discordAuthor `json:"author,omitempty"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Color       int            `json:"color"`
	Fields      []discordField `json:"fields,omitempty"`
}

type discordAuthor struct {
	Name    string `json:"name"`
	IconURL string `json:"icon_url,omitempty"`
}

// TODO: populate capacitarrIconURL once a hosted logo is available.
// Discord gracefully ignores empty/missing icon_url fields.
const capacitarrIconURL = ""

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

// SendDigest delivers a cycle digest notification to a Discord webhook.
func (s *DiscordSender) SendDigest(config SenderConfig, digest CycleDigest) error {
	webhookURL := config.WebhookURL
	if webhookURL == "" {
		return fmt.Errorf("discord webhook URL is empty")
	}

	// Build author line: "Capacitarr v1.4.0 • auto"
	authorName := fmt.Sprintf("⚡ Capacitarr %s", digest.Version)
	if digest.ExecutionMode != "" {
		authorName += " • " + digest.ExecutionMode
	}

	desc := digestDescription(digest)

	// Append disk usage progress bar for auto mode or all-clear
	if digest.DiskUsagePct > 0 && (digest.ExecutionMode == ModeAuto || digest.Flagged == 0) {
		bar := ProgressBar(digest.DiskUsagePct, 20)
		if digest.ExecutionMode == ModeAuto && digest.Flagged > 0 {
			desc += fmt.Sprintf("\n\n`%s` **%.0f%%** → %.0f%%", bar, digest.DiskUsagePct, digest.DiskTargetPct)
		} else {
			desc += fmt.Sprintf("\n\n`%s` **%.0f%%** / %.0f%%", bar, digest.DiskUsagePct, digest.DiskThreshold)
		}
	}

	// Append version update banner
	if digest.UpdateAvailable && digest.LatestVersion != "" {
		desc += fmt.Sprintf("\n\n📦 **%s** available!", digest.LatestVersion)
	}

	embed := discordEmbed{
		Author:      &discordAuthor{Name: authorName, IconURL: capacitarrIconURL},
		Title:       digestTitle(digest),
		Description: desc,
		Color:       digestColor(digest),
	}

	return sendDiscordPayload(webhookURL, discordPayload{Embeds: []discordEmbed{embed}})
}

// SendAlert delivers an immediate alert notification to a Discord webhook.
func (s *DiscordSender) SendAlert(config SenderConfig, alert Alert) error {
	webhookURL := config.WebhookURL
	if webhookURL == "" {
		return fmt.Errorf("discord webhook URL is empty")
	}

	// Include the trigger label so recipients know what action produced this alert
	authorName := fmt.Sprintf("⚡ Capacitarr %s • %s", alert.Version, TriggerLabel(alert.Type))

	embed := discordEmbed{
		Author:      &discordAuthor{Name: authorName, IconURL: capacitarrIconURL},
		Title:       alert.Title,
		Description: alert.Message,
		Color:       alertColor(alert.Type),
	}

	// Add fields for alerts that carry structured data
	for k, v := range alert.Fields {
		embed.Fields = append(embed.Fields, discordField{
			Name:   k,
			Value:  v,
			Inline: true,
		})
	}

	return sendDiscordPayload(webhookURL, discordPayload{Embeds: []discordEmbed{embed}})
}

// sendDiscordPayload marshals and sends a Discord webhook payload with retry.
func sendDiscordPayload(webhookURL string, payload discordPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal discord payload: %w", err)
	}
	return sendWebhookRequest(webhookURL, body)
}
