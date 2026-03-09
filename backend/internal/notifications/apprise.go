package notifications

import (
	"encoding/json"
	"fmt"
	"strings"
)

// AppriseSender implements Sender for Apprise notification server delivery.
// It supports both the stateless API (POST /api/notify/) and the persistent
// store API (POST /api/notify/{key}/). The user provides the full API endpoint
// URL in the WebhookURL field; optional comma-separated tags are passed via
// SenderConfig.AppriseTags for notification routing on the Apprise server.
type AppriseSender struct{}

// NewAppriseSender creates a new AppriseSender.
func NewAppriseSender() *AppriseSender {
	return &AppriseSender{}
}

// apprisePayload matches the Apprise notification API request body.
type apprisePayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Type  string `json:"type"`          // "info", "success", "warning", "failure"
	Tag   string `json:"tag,omitempty"` // Comma-separated Apprise tags
}

// SendDigest delivers a cycle digest notification to an Apprise server.
func (s *AppriseSender) SendDigest(config SenderConfig, digest CycleDigest) error {
	if config.WebhookURL == "" {
		return fmt.Errorf("apprise URL is empty")
	}

	title := fmt.Sprintf("⚡ Capacitarr %s", digest.Version)
	if digest.ExecutionMode != "" {
		title += " • " + digest.ExecutionMode
	}

	body := digestTitle(digest) + "\n\n" + digestDescription(digest)

	// Append disk usage progress bar
	if digest.DiskUsagePct > 0 && (digest.ExecutionMode == ModeAuto || digest.Flagged == 0) {
		bar := ProgressBar(digest.DiskUsagePct, 20)
		if digest.ExecutionMode == ModeAuto && digest.Flagged > 0 {
			body += fmt.Sprintf("\n\n%s %.0f%% → %.0f%%", bar, digest.DiskUsagePct, digest.DiskTargetPct)
		} else {
			body += fmt.Sprintf("\n\n%s %.0f%% / %.0f%%", bar, digest.DiskUsagePct, digest.DiskThreshold)
		}
	}

	// Append version update banner
	if digest.UpdateAvailable && digest.LatestVersion != "" {
		body += fmt.Sprintf("\n\n📦 %s available!", digest.LatestVersion)
	}

	payload := apprisePayload{
		Title: title,
		Body:  body,
		Type:  "info",
		Tag:   strings.TrimSpace(config.AppriseTags),
	}

	return sendApprisePayload(config.WebhookURL, payload)
}

// SendAlert delivers an immediate alert notification to an Apprise server.
func (s *AppriseSender) SendAlert(config SenderConfig, alert Alert) error {
	if config.WebhookURL == "" {
		return fmt.Errorf("apprise URL is empty")
	}

	title := fmt.Sprintf("⚡ Capacitarr %s • %s", alert.Version, TriggerLabel(alert.Type))

	body := alert.Title
	if alert.Message != "" {
		body += "\n\n" + alert.Message
	}

	// Append fields as key-value lines
	if len(alert.Fields) > 0 {
		body += "\n"
		for k, v := range alert.Fields {
			body += fmt.Sprintf("\n%s: %s", k, v)
		}
	}

	payload := apprisePayload{
		Title: title,
		Body:  body,
		Type:  mapAppriseType(alert.Type),
		Tag:   strings.TrimSpace(config.AppriseTags),
	}

	return sendApprisePayload(config.WebhookURL, payload)
}

// mapAppriseType maps an AlertType to the corresponding Apprise notification
// type string: "info", "success", "warning", or "failure".
func mapAppriseType(t AlertType) string {
	switch t {
	case AlertError, AlertThresholdBreached:
		return "failure"
	case AlertModeChanged:
		return "warning"
	case AlertServerStarted:
		return "success"
	case AlertUpdateAvailable, AlertApprovalActivity, AlertTest:
		return "info"
	default:
		return "info"
	}
}

// sendApprisePayload marshals and sends an Apprise API payload with retry.
func sendApprisePayload(url string, payload apprisePayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal apprise payload: %w", err)
	}
	return sendWebhookRequest(url, body)
}
