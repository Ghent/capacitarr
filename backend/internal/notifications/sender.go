package notifications

import (
	"fmt"

	"capacitarr/internal/db"
)

// SenderConfig holds the configuration passed to a Sender for each delivery.
// All senders receive a WebhookURL; channel-specific fields (e.g. AppriseTags)
// are only used by their respective sender implementations.
type SenderConfig struct {
	WebhookURL  string
	AppriseTags string // Only used by AppriseSender
}

// Sender is the interface for delivering notifications to external channels.
// Each channel type (Discord, Apprise) implements this interface.
type Sender interface {
	// SendDigest delivers a cycle digest notification summarizing an engine run.
	SendDigest(config SenderConfig, digest CycleDigest) error
	// SendAlert delivers an immediate alert notification.
	SendAlert(config SenderConfig, alert Alert) error
}

// CycleDigest contains the accumulated data for a single engine cycle
// notification. The dispatch service builds this from event accumulation.
type CycleDigest struct {
	ExecutionMode      string  `json:"executionMode"`
	Evaluated          int     `json:"evaluated"`
	Flagged            int     `json:"flagged"`
	Deleted            int     `json:"deleted"`
	Failed             int     `json:"failed"`
	FreedBytes         int64   `json:"freedBytes"`
	DurationMs         int64   `json:"durationMs"`
	DiskUsagePct       float64 `json:"diskUsagePct"`
	DiskThreshold      float64 `json:"diskThreshold"`
	DiskTargetPct      float64 `json:"diskTargetPct"`
	CollectionsDeleted int     `json:"collectionsDeleted"` // Number of distinct collections that triggered group deletions
	Version            string  `json:"version"`

	// Update information — populated when a newer version is available.
	UpdateAvailable bool   `json:"updateAvailable"`
	LatestVersion   string `json:"latestVersion"`
	ReleaseURL      string `json:"releaseUrl"`
}

// AlertType identifies the category of an immediate alert notification.
type AlertType string

// Alert type constants.
const (
	AlertError             AlertType = "error"
	AlertModeChanged       AlertType = "mode_changed"
	AlertServerStarted     AlertType = "server_started"
	AlertThresholdBreached AlertType = "threshold_breached"
	AlertUpdateAvailable   AlertType = "update_available"
	AlertApprovalActivity  AlertType = "approval_activity"
	AlertIntegrationStatus AlertType = "integration_status"
	AlertTest              AlertType = "test"
)

// Alert represents an immediate notification that does not wait for the
// two-gate flush (unlike cycle digests).
type Alert struct {
	Type    AlertType         `json:"type"`
	Title   string            `json:"title"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	Version string            `json:"version"`
}

// TriggerLabel returns a human-readable label for the alert type,
// suitable for display in notification headers (e.g., "Update Available").
func TriggerLabel(t AlertType) string {
	switch t {
	case AlertError:
		return "Engine Error"
	case AlertModeChanged:
		return "Mode Change"
	case AlertServerStarted:
		return "Server Started"
	case AlertThresholdBreached:
		return "Threshold Breached"
	case AlertUpdateAvailable:
		return "Update Available"
	case AlertApprovalActivity:
		return "Approval Activity"
	case AlertIntegrationStatus:
		return "Integration Status"
	case AlertTest:
		return "Test"
	default:
		return string(t)
	}
}

// Execution mode aliases for readability in this package.
const (
	ModeAuto     = db.ModeAuto
	ModeDryRun   = db.ModeDryRun
	ModeApproval = db.ModeApproval
)

// Digest title constants.
const (
	titleCleanupComplete = "🧹 Cleanup Complete"
	titleAllClear        = "✅ All Clear"
)

// Discord embed colors.
const (
	ColorGreen  = 0x2ECC71 // success
	ColorBlue   = 0x3498DB // info
	ColorAmber  = 0xF1C40F // attention
	ColorOrange = 0xE67E22 // warning
	ColorRed    = 0xE74C3C // error
)

// HumanSize converts a byte count to a human-readable string with one decimal
// place, choosing the appropriate unit (B, KB, MB, GB, TB).
func HumanSize(bytes int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
		tb = gb * 1024
	)
	switch {
	case bytes >= tb:
		return fmt.Sprintf("%.1f TB", float64(bytes)/float64(tb))
	case bytes >= gb:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	case bytes >= mb:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
	case bytes >= kb:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(kb))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// ProgressBar returns a text-based progress bar using block characters.
// Width is the total number of characters. The bar uses ▓ for filled and ░
// for empty segments.
func ProgressBar(pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}
	bar := make([]rune, width)
	for i := range bar {
		if i < filled {
			bar[i] = '▓'
		} else {
			bar[i] = '░'
		}
	}
	return string(bar)
}

// digestTitle returns the appropriate title and emoji for a cycle digest
// based on execution mode and action counts.
func digestTitle(d CycleDigest) string {
	if d.Flagged == 0 {
		return titleAllClear
	}
	switch d.ExecutionMode {
	case ModeAuto:
		return titleCleanupComplete
	case ModeDryRun:
		return "🔍 Dry-Run Complete"
	case ModeApproval:
		return "📋 Items Queued for Approval"
	default:
		return titleCleanupComplete
	}
}

// digestColor returns the embed color for a cycle digest.
func digestColor(d CycleDigest) int {
	if d.Flagged == 0 {
		return ColorGreen
	}
	switch d.ExecutionMode {
	case ModeAuto:
		return ColorGreen
	case ModeDryRun:
		return ColorBlue
	case ModeApproval:
		return ColorAmber
	default:
		return ColorGreen
	}
}

// digestDescription builds the description text for a cycle digest embed.
func digestDescription(d CycleDigest) string {
	durSec := float64(d.DurationMs) / 1000.0

	if d.Flagged == 0 {
		return fmt.Sprintf("Evaluated **%d** items — no action needed", d.Evaluated)
	}

	switch d.ExecutionMode {
	case ModeAuto:
		desc := fmt.Sprintf("Deleted **%d** of **%d** evaluated items\nin **%.1fs**, freeing **%s**",
			d.Deleted, d.Evaluated, durSec, HumanSize(d.FreedBytes))
		if d.CollectionsDeleted > 0 {
			desc += fmt.Sprintf("\n📦 Included **%d** collection group deletion(s)", d.CollectionsDeleted)
		}
		if d.Failed > 0 {
			desc += fmt.Sprintf("\n⚠️ %d deletion(s) failed", d.Failed)
		}
		return desc
	case ModeDryRun:
		return fmt.Sprintf("Flagged **%d** of **%d** items in **%.1fs**\nWould free **%s**",
			d.Flagged, d.Evaluated, durSec, HumanSize(d.FreedBytes))
	case ModeApproval:
		return fmt.Sprintf("Queued **%d** of **%d** items in **%.1fs**\nPotential **%s**",
			d.Flagged, d.Evaluated, durSec, HumanSize(d.FreedBytes))
	default:
		return fmt.Sprintf("Processed **%d** of **%d** items in **%.1fs**",
			d.Flagged, d.Evaluated, durSec)
	}
}

// alertColor returns the embed color for an alert type.
func alertColor(t AlertType) int {
	switch t {
	case AlertError:
		return ColorRed
	case AlertModeChanged:
		return ColorOrange
	case AlertServerStarted:
		return ColorGreen
	case AlertThresholdBreached:
		return ColorRed
	case AlertUpdateAvailable:
		return ColorBlue
	case AlertApprovalActivity:
		return ColorAmber
	case AlertIntegrationStatus:
		return ColorOrange
	case AlertTest:
		return ColorBlue
	default:
		return ColorBlue
	}
}
