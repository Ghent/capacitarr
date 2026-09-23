package notifications

import (
	"fmt"
	"strings"

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
	// The level parameter lets senders filter which group sections to show
	// based on the channel's notification tier.
	SendDigest(config SenderConfig, digest CycleDigest, level NotificationTier) error
	// SendAlert delivers an immediate alert notification.
	SendAlert(config SenderConfig, alert Alert) error
}

// CycleDigest contains the data for a single engine cycle notification.
// Built by the poller from its own counters and passed to the dispatch service.
// Per-group metrics are in the Groups slice; top-level fields are cycle-wide.
type CycleDigest struct {
	Groups     []GroupDigest `json:"groups"`
	DurationMs int64         `json:"durationMs"`
	Version    string        `json:"version"`

	// Update information — populated when a newer version is available.
	UpdateAvailable bool   `json:"updateAvailable"`
	LatestVersion   string `json:"latestVersion"`
	ReleaseURL      string `json:"releaseUrl"`
}

// GroupDigest contains per-disk-group metrics for a single engine cycle.
type GroupDigest struct {
	MountPath          string  `json:"mountPath"`
	Mode               string  `json:"mode"`
	Evaluated          int     `json:"evaluated"`
	Candidates         int     `json:"candidates"`
	Deleted            int     `json:"deleted"`
	Failed             int     `json:"failed"`
	FreedBytes         int64   `json:"freedBytes"`
	DiskUsagePct       float64 `json:"diskUsagePct"`
	DiskThreshold      float64 `json:"diskThreshold"`
	DiskTargetPct      float64 `json:"diskTargetPct"`
	CollectionsDeleted int     `json:"collectionsDeleted"`
	SunsetQueued       int     `json:"sunsetQueued"`
	SunsetExpired      int     `json:"sunsetExpired"`
	SunsetSaved        int     `json:"sunsetSaved"`
	EscalatedItems     int     `json:"escalatedItems"`
	EscalatedBytes     int64   `json:"escalatedBytes"`
	QueueFullSkipped   int     `json:"queueFullSkipped,omitempty"`
}

// TotalEvaluated returns the sum of Evaluated across all groups.
func (d CycleDigest) TotalEvaluated() int {
	total := 0
	for _, g := range d.Groups {
		total += g.Evaluated
	}
	return total
}

// TotalCandidates returns the sum of Candidates across all groups.
func (d CycleDigest) TotalCandidates() int {
	total := 0
	for _, g := range d.Groups {
		total += g.Candidates
	}
	return total
}

// TotalDeleted returns the sum of Deleted across all groups.
func (d CycleDigest) TotalDeleted() int {
	total := 0
	for _, g := range d.Groups {
		total += g.Deleted
	}
	return total
}

// TotalFreedBytes returns the sum of FreedBytes across all groups.
func (d CycleDigest) TotalFreedBytes() int64 {
	var total int64
	for _, g := range d.Groups {
		total += g.FreedBytes
	}
	return total
}

// TotalCollectionsDeleted returns the sum of CollectionsDeleted across all groups.
func (d CycleDigest) TotalCollectionsDeleted() int {
	total := 0
	for _, g := range d.Groups {
		total += g.CollectionsDeleted
	}
	return total
}

// TotalFailed returns the sum of Failed across all groups.
func (d CycleDigest) TotalFailed() int {
	total := 0
	for _, g := range d.Groups {
		total += g.Failed
	}
	return total
}

// PrimaryMode returns the execution mode from the first group, or empty string
// if there are no groups. Used for backward-compatible digest rendering.
func (d CycleDigest) PrimaryMode() string {
	if len(d.Groups) > 0 {
		return d.Groups[0].Mode
	}
	return ""
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

// Alert represents an immediate notification dispatched via the event bus.
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
	if d.TotalCandidates() == 0 {
		return titleAllClear
	}
	switch d.PrimaryMode() {
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

// filterGroups returns the subset of groups that should be shown at the given tier.
func filterGroups(groups []GroupDigest, level NotificationTier) []GroupDigest {
	var result []GroupDigest
	for _, g := range groups {
		switch {
		case level >= TierVerbose:
			// Verbose: show everything including dry-run
			result = append(result, g)
		case level >= TierNormal:
			// Normal: show all groups except dry-run
			if g.Mode != ModeDryRun {
				result = append(result, g)
			}
		case level >= TierImportant:
			// Important: only groups where action was taken
			if hasActivity(g) && g.Mode != ModeDryRun {
				result = append(result, g)
			}
		}
		// Critical and Off: no digest groups shown
	}
	return result
}

// hasActivity returns true if the group had any meaningful action this cycle.
func hasActivity(g GroupDigest) bool {
	return g.Deleted > 0 || g.Candidates > 0 || g.FreedBytes > 0 ||
		g.SunsetQueued > 0 || g.SunsetExpired > 0 || g.SunsetSaved > 0 ||
		g.EscalatedItems > 0 || g.QueueFullSkipped > 0
}

// groupDescription returns a mode-specific summary line for a group.
func groupDescription(g GroupDigest) string {
	switch g.Mode {
	case "auto":
		if g.Deleted > 0 {
			return fmt.Sprintf("Deleted %d of %d, freeing %s", g.Deleted, g.Evaluated, HumanSize(g.FreedBytes))
		}
		return fmt.Sprintf("Evaluated %d items — all within threshold", g.Evaluated)
	case "approval":
		if g.Candidates > 0 {
			return fmt.Sprintf("Queued %d of %d for approval", g.Candidates, g.Evaluated)
		}
		return fmt.Sprintf("Evaluated %d items — all within threshold", g.Evaluated)
	case "sunset":
		parts := []string{}
		if g.SunsetQueued > 0 {
			parts = append(parts, fmt.Sprintf("%d items entered sunset", g.SunsetQueued))
		}
		if g.SunsetExpired > 0 {
			parts = append(parts, fmt.Sprintf("%d expired", g.SunsetExpired))
		}
		if g.SunsetSaved > 0 {
			parts = append(parts, fmt.Sprintf("%d saved", g.SunsetSaved))
		}
		if g.EscalatedItems > 0 {
			parts = append(parts, fmt.Sprintf("%d force-expired (%s freed)", g.EscalatedItems, HumanSize(g.EscalatedBytes)))
		}
		if len(parts) == 0 {
			return fmt.Sprintf("Evaluated %d items — all within threshold", g.Evaluated)
		}
		return strings.Join(parts, " · ")
	case ModeDryRun:
		if g.Candidates > 0 {
			return fmt.Sprintf("%d candidates, would free %s", g.Candidates, HumanSize(g.FreedBytes))
		}
		return fmt.Sprintf("Evaluated %d items — all within threshold", g.Evaluated)
	default:
		return fmt.Sprintf("Evaluated %d items", g.Evaluated)
	}
}

// groupIcon returns a mode-specific emoji prefix for a group.
func groupIcon(mode string) string {
	switch mode {
	case "auto":
		return "🧹"
	case "approval":
		return "📋"
	case "sunset":
		return "☀️"
	case ModeDryRun:
		return "🔍"
	default:
		return "📊"
	}
}

// digestColor returns a Discord embed color based on the most severe group action.
func digestColor(groups []GroupDigest) int {
	for _, g := range groups {
		if g.EscalatedItems > 0 {
			return ColorRed // escalation happened
		}
	}
	for _, g := range groups {
		if g.Deleted > 0 || g.SunsetExpired > 0 {
			return ColorAmber // items removed
		}
	}
	return ColorGreen // healthy
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
