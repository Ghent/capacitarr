package notifications

import (
	"encoding/json"
	"fmt"
)

// SlackSender implements Sender for Slack webhook delivery using Block Kit.
type SlackSender struct{}

// NewSlackSender creates a new SlackSender.
func NewSlackSender() *SlackSender {
	return &SlackSender{}
}

// Slack Block Kit payload types.
type slackPayload struct {
	Blocks []slackBlock `json:"blocks"`
}

type slackBlock struct {
	Type   string      `json:"type"`
	Text   *slackText  `json:"text,omitempty"`
	Fields []slackText `json:"fields,omitempty"`
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// SendDigest delivers a cycle digest notification to a Slack webhook.
func (s *SlackSender) SendDigest(webhookURL string, digest CycleDigest) error {
	if webhookURL == "" {
		return fmt.Errorf("slack webhook URL is empty")
	}

	// Build header: "⚡ Capacitarr v1.4.0 • auto"
	header := fmt.Sprintf("⚡ Capacitarr %s", digest.Version)
	if digest.ExecutionMode != "" {
		header += " • " + digest.ExecutionMode
	}

	desc := digestDescription(digest)

	// Append disk usage progress bar
	if digest.DiskUsagePct > 0 && (digest.ExecutionMode == ModeAuto || digest.Flagged == 0) {
		bar := ProgressBar(digest.DiskUsagePct, 20)
		if digest.ExecutionMode == ModeAuto && digest.Flagged > 0 {
			desc += fmt.Sprintf("\n\n`%s` *%.0f%%* → %.0f%%", bar, digest.DiskUsagePct, digest.DiskTargetPct)
		} else {
			desc += fmt.Sprintf("\n\n`%s` *%.0f%%* / %.0f%%", bar, digest.DiskUsagePct, digest.DiskThreshold)
		}
	}

	// Append version update banner
	if digest.UpdateAvailable && digest.LatestVersion != "" {
		desc += fmt.Sprintf("\n\n📦 *%s* available!", digest.LatestVersion)
	}

	blocks := []slackBlock{
		{
			Type: "header",
			Text: &slackText{Type: "plain_text", Text: header},
		},
		{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: digestTitle(digest) + "\n\n" + desc},
		},
	}

	return sendSlackPayload(webhookURL, slackPayload{Blocks: blocks})
}

// SendAlert delivers an immediate alert notification to a Slack webhook.
func (s *SlackSender) SendAlert(webhookURL string, alert Alert) error {
	if webhookURL == "" {
		return fmt.Errorf("slack webhook URL is empty")
	}

	header := fmt.Sprintf("⚡ Capacitarr %s", alert.Version)

	blocks := []slackBlock{
		{
			Type: "header",
			Text: &slackText{Type: "plain_text", Text: header},
		},
		{
			Type: "section",
			Text: &slackText{Type: "mrkdwn", Text: alert.Title + "\n\n" + alert.Message},
		},
	}

	// Add fields block if there are key-value pairs
	if len(alert.Fields) > 0 {
		var fields []slackText
		for k, v := range alert.Fields {
			fields = append(fields, slackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("*%s:*\n%s", k, v),
			})
		}
		blocks = append(blocks, slackBlock{
			Type:   "section",
			Fields: fields,
		})
	}

	return sendSlackPayload(webhookURL, slackPayload{Blocks: blocks})
}

// sendSlackPayload marshals and sends a Slack webhook payload with retry.
func sendSlackPayload(webhookURL string, payload slackPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal slack payload: %w", err)
	}
	return sendWebhookRequest(webhookURL, body)
}
