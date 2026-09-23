package events

import (
	"fmt"
	"time"
)

// =============================================================================
// Auth Events
// =============================================================================

// LoginEvent is published on successful authentication.
type LoginEvent struct {
	Username string `json:"username"`
}

// EventType implements Event.
func (e LoginEvent) EventType() string { return "login" }

// EventMessage implements Event.
func (e LoginEvent) EventMessage() string { return "User logged in: " + e.Username }

// PasswordChangedEvent is published when a user changes their password.
type PasswordChangedEvent struct {
	Username string `json:"username"`
}

// EventType implements Event.
func (e PasswordChangedEvent) EventType() string { return "password_changed" }

// EventMessage implements Event.
func (e PasswordChangedEvent) EventMessage() string { return "Password changed for " + e.Username }

// UsernameChangedEvent is published when a user changes their username.
type UsernameChangedEvent struct {
	OldUsername string `json:"oldUsername"`
	NewUsername string `json:"newUsername"`
}

// EventType implements Event.
func (e UsernameChangedEvent) EventType() string { return "username_changed" }

// EventMessage implements Event.
func (e UsernameChangedEvent) EventMessage() string {
	return fmt.Sprintf("Username changed from %s to %s", e.OldUsername, e.NewUsername)
}

// APIKeyGeneratedEvent is published when an API key is generated.
type APIKeyGeneratedEvent struct {
	Username string `json:"username"`
	Hint     string `json:"hint"` // Last 4 chars
}

// EventType implements Event.
func (e APIKeyGeneratedEvent) EventType() string { return "api_key_generated" }

// EventMessage implements Event.
func (e APIKeyGeneratedEvent) EventMessage() string {
	return fmt.Sprintf("API key generated for %s (ending in %s)", e.Username, e.Hint)
}

// =============================================================================
// Disk Events
// =============================================================================

// ThresholdBreachedEvent is published when disk usage exceeds the configured
// threshold during an engine evaluation cycle. This is distinct from
// ThresholdChangedEvent, which fires when an admin changes the threshold
// settings — ThresholdBreachedEvent fires on actual disk usage detection.
type ThresholdBreachedEvent struct {
	MountPath    string  `json:"mountPath"`
	CurrentPct   float64 `json:"currentPct"`
	ThresholdPct float64 `json:"thresholdPct"`
	TargetPct    float64 `json:"targetPct"`
}

// EventType implements Event.
func (e ThresholdBreachedEvent) EventType() string { return "threshold_breached" }

// EventMessage implements Event.
func (e ThresholdBreachedEvent) EventMessage() string {
	return fmt.Sprintf("Disk threshold breached on %s: %.1f%% (threshold: %.0f%%)",
		e.MountPath, e.CurrentPct, e.ThresholdPct)
}

// =============================================================================
// Disk Group Lifecycle Events
// =============================================================================

// DiskGroupStaleEvent is published when a disk group is marked stale (no longer
// reported by any integration). Used by the activity feed — not user-actionable.
type DiskGroupStaleEvent struct {
	DiskGroupID uint   `json:"diskGroupId"`
	MountPath   string `json:"mountPath"`
}

// EventType implements Event.
func (e DiskGroupStaleEvent) EventType() string { return "disk_group_stale" }

// EventMessage implements Event.
func (e DiskGroupStaleEvent) EventMessage() string {
	return fmt.Sprintf("Disk group %s marked stale — not reported by any integration", e.MountPath)
}

// DiskGroupReapedEvent is published when a stale disk group is permanently
// deleted after the grace period expires.
type DiskGroupReapedEvent struct {
	DiskGroupID uint   `json:"diskGroupId"`
	MountPath   string `json:"mountPath"`
	StaleDays   int    `json:"staleDays"`
}

// EventType implements Event.
func (e DiskGroupReapedEvent) EventType() string { return "disk_group_reaped" }

// EventMessage implements Event.
func (e DiskGroupReapedEvent) EventMessage() string {
	return fmt.Sprintf("Disk group %s removed after %d days without integration data", e.MountPath, e.StaleDays)
}

// DiskGroupResurrectedEvent is published when a stale disk group is restored to
// active status because its mount path was reported again by an integration.
type DiskGroupResurrectedEvent struct {
	DiskGroupID uint   `json:"diskGroupId"`
	MountPath   string `json:"mountPath"`
	StaleDays   int    `json:"staleDays"` // How long it was stale before resurrection
}

// EventType implements Event.
func (e DiskGroupResurrectedEvent) EventType() string { return "disk_group_resurrected" }

// EventMessage implements Event.
func (e DiskGroupResurrectedEvent) EventMessage() string {
	return fmt.Sprintf("Disk group %s restored (was stale for %d days)", e.MountPath, e.StaleDays)
}

// =============================================================================
// Version Events
// =============================================================================

// UpdateAvailableEvent is published when the VersionService detects a new
// release for the first time. It fires at most once per version to avoid
// repeated notifications on cache refresh.
type UpdateAvailableEvent struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	ReleaseURL     string `json:"releaseUrl"`
}

// EventType implements Event.
func (e UpdateAvailableEvent) EventType() string { return "update_available" }

// EventMessage implements Event.
func (e UpdateAvailableEvent) EventMessage() string {
	return fmt.Sprintf("Update available: %s → %s", e.CurrentVersion, e.LatestVersion)
}

// VersionCheckEvent is published every time the VersionService performs an
// update check, regardless of whether an update is available. This provides
// activity log visibility into when checks are happening.
type VersionCheckEvent struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
}

// EventType implements Event.
func (e VersionCheckEvent) EventType() string { return "version_check" }

// EventMessage implements Event.
func (e VersionCheckEvent) EventMessage() string {
	if e.UpdateAvailable {
		return fmt.Sprintf("Version check: update available (%s → %s)", e.CurrentVersion, e.LatestVersion)
	}
	return fmt.Sprintf("Version check: up to date (%s)", e.CurrentVersion)
}

// =============================================================================
// Rule Events
// =============================================================================

// RuleCreatedEvent is published when a custom rule is created.
type RuleCreatedEvent struct {
	RuleID uint   `json:"ruleId"`
	Field  string `json:"field"`
	Effect string `json:"effect"`
}

// EventType implements Event.
func (e RuleCreatedEvent) EventType() string { return "rule_created" }

// EventMessage implements Event.
func (e RuleCreatedEvent) EventMessage() string {
	return fmt.Sprintf("Custom rule created: %s → %s", e.Field, e.Effect)
}

// RuleUpdatedEvent is published when a custom rule is modified.
type RuleUpdatedEvent struct {
	RuleID uint   `json:"ruleId"`
	Field  string `json:"field"`
	Effect string `json:"effect"`
}

// EventType implements Event.
func (e RuleUpdatedEvent) EventType() string { return "rule_updated" }

// EventMessage implements Event.
func (e RuleUpdatedEvent) EventMessage() string {
	return fmt.Sprintf("Custom rule updated: %s → %s", e.Field, e.Effect)
}

// RuleDeletedEvent is published when a custom rule is deleted.
type RuleDeletedEvent struct {
	RuleID uint   `json:"ruleId"`
	Field  string `json:"field"`
}

// EventType implements Event.
func (e RuleDeletedEvent) EventType() string { return "rule_deleted" }

// EventMessage implements Event.
func (e RuleDeletedEvent) EventMessage() string {
	return fmt.Sprintf("Custom rule deleted: %s (ID %d)", e.Field, e.RuleID)
}

// =============================================================================
// Notification Events
// =============================================================================

// NotificationChannelAddedEvent is published when a notification channel is created.
type NotificationChannelAddedEvent struct {
	ChannelID   uint   `json:"channelId"`
	ChannelType string `json:"channelType"`
	Name        string `json:"name"`
}

// EventType implements Event.
func (e NotificationChannelAddedEvent) EventType() string { return "notification_channel_added" }

// EventMessage implements Event.
func (e NotificationChannelAddedEvent) EventMessage() string {
	return fmt.Sprintf("Notification channel added: %s (%s)", e.Name, e.ChannelType)
}

// NotificationChannelUpdatedEvent is published when a notification channel is modified.
type NotificationChannelUpdatedEvent struct {
	ChannelID   uint   `json:"channelId"`
	ChannelType string `json:"channelType"`
	Name        string `json:"name"`
}

// EventType implements Event.
func (e NotificationChannelUpdatedEvent) EventType() string { return "notification_channel_updated" }

// EventMessage implements Event.
func (e NotificationChannelUpdatedEvent) EventMessage() string {
	return fmt.Sprintf("Notification channel updated: %s (%s)", e.Name, e.ChannelType)
}

// NotificationChannelRemovedEvent is published when a notification channel is deleted.
type NotificationChannelRemovedEvent struct {
	ChannelID   uint   `json:"channelId"`
	ChannelType string `json:"channelType"`
	Name        string `json:"name"`
}

// EventType implements Event.
func (e NotificationChannelRemovedEvent) EventType() string { return "notification_channel_removed" }

// EventMessage implements Event.
func (e NotificationChannelRemovedEvent) EventMessage() string {
	return fmt.Sprintf("Notification channel removed: %s (%s)", e.Name, e.ChannelType)
}

// NotificationSentEvent is published when a notification is successfully delivered.
type NotificationSentEvent struct {
	ChannelID   uint   `json:"channelId"`
	ChannelType string `json:"channelType"`
	Name        string `json:"name"`
	TriggerType string `json:"triggerType"` // The event type that triggered the notification
}

// EventType implements Event.
func (e NotificationSentEvent) EventType() string { return "notification_sent" }

// EventMessage implements Event.
func (e NotificationSentEvent) EventMessage() string {
	return fmt.Sprintf("Notification sent via %s (%s)", e.Name, e.ChannelType)
}

// NotificationDeliveryFailedEvent is published when a notification delivery fails.
type NotificationDeliveryFailedEvent struct {
	ChannelID   uint   `json:"channelId"`
	ChannelType string `json:"channelType"`
	Name        string `json:"name"`
	Error       string `json:"error"`
}

// EventType implements Event.
func (e NotificationDeliveryFailedEvent) EventType() string { return "notification_delivery_failed" }

// EventMessage implements Event.
func (e NotificationDeliveryFailedEvent) EventMessage() string {
	return fmt.Sprintf("Notification delivery failed: %s (%s) — %s", e.Name, e.ChannelType, e.Error)
}

// =============================================================================
// Preview Events
// =============================================================================

// PreviewUpdatedEvent is published when the preview cache is populated with
// fresh data (after a poller cycle or a force-refresh computation).
type PreviewUpdatedEvent struct {
	ItemCount int       `json:"itemCount"`
	Timestamp time.Time `json:"timestamp"`
}

// EventType implements Event.
func (e PreviewUpdatedEvent) EventType() string { return "preview_updated" }

// EventMessage implements Event.
func (e PreviewUpdatedEvent) EventMessage() string {
	return fmt.Sprintf("Preview updated: %d items scored", e.ItemCount)
}

// AnalyticsUpdatedEvent is published alongside PreviewUpdatedEvent to signal
// that analytics data (composition, quality, watch intelligence) should be
// refetched by the frontend. The analytics APIs aggregate from the preview
// cache, so they're only valid after a cache refresh.
type AnalyticsUpdatedEvent struct {
	ItemCount int       `json:"itemCount"`
	Timestamp time.Time `json:"timestamp"`
}

// EventType implements Event.
func (e AnalyticsUpdatedEvent) EventType() string { return "analytics_updated" }

// EventMessage implements Event.
func (e AnalyticsUpdatedEvent) EventMessage() string {
	return fmt.Sprintf("Analytics updated: %d items available", e.ItemCount)
}

// PreviewInvalidatedEvent is published when the preview cache is cleared due
// to a configuration change that affects scoring (rules, settings,
// integrations, thresholds). Connected clients should show a stale indicator
// and fetch fresh data.
type PreviewInvalidatedEvent struct {
	Reason string `json:"reason"` // e.g. "rule_changed", "settings_changed"
}

// EventType implements Event.
func (e PreviewInvalidatedEvent) EventType() string { return "preview_invalidated" }

// EventMessage implements Event.
func (e PreviewInvalidatedEvent) EventMessage() string {
	return fmt.Sprintf("Preview cache invalidated: %s", e.Reason)
}

// =============================================================================
// Data Events
// =============================================================================

// DataResetEvent is published when all scraped data is cleared.
type DataResetEvent struct {
	Summary map[string]int64 `json:"summary"` // e.g. {"audit_log": 42, "approval_queue": 5}
}

// EventType implements Event.
func (e DataResetEvent) EventType() string { return "data_reset" }

// EventMessage implements Event.
func (e DataResetEvent) EventMessage() string { return "All scraped data has been reset" }

// =============================================================================
// System Events
// =============================================================================

// ServerStartedEvent is published when the application starts.
type ServerStartedEvent struct {
	Version string `json:"version"`
}

// EventType implements Event.
func (e ServerStartedEvent) EventType() string { return "server_started" }

// EventMessage implements Event.
func (e ServerStartedEvent) EventMessage() string {
	if e.Version != "" {
		return fmt.Sprintf("Server started (version %s)", e.Version)
	}
	return "Server started"
}

// =============================================================================
// Sunset Events
// =============================================================================

// SunsetCreatedEvent is published when an item is added to the sunset queue.
type SunsetCreatedEvent struct {
	MediaName     string `json:"mediaName"`
	MediaType     string `json:"mediaType"`
	DiskGroupID   uint   `json:"diskGroupId"`
	DaysRemaining int    `json:"daysRemaining"`
	DeletionDate  string `json:"deletionDate"`
}

// EventType implements Event.
func (e SunsetCreatedEvent) EventType() string { return "sunset_created" }

// EventMessage implements Event.
func (e SunsetCreatedEvent) EventMessage() string {
	return fmt.Sprintf("%s added to sunset queue — leaving in %d days", e.MediaName, e.DaysRemaining)
}

// SunsetCancelledEvent is published when a sunset item is cancelled (removed from queue).
type SunsetCancelledEvent struct {
	MediaName   string `json:"mediaName"`
	MediaType   string `json:"mediaType"`
	DiskGroupID uint   `json:"diskGroupId"`
}

// EventType implements Event.
func (e SunsetCancelledEvent) EventType() string { return "sunset_cancelled" }

// EventMessage implements Event.
func (e SunsetCancelledEvent) EventMessage() string {
	return fmt.Sprintf("%s removed from sunset queue", e.MediaName)
}

// SunsetExpiredEvent is published when a sunset countdown expires and the item
// is handed to DeletionService for actual removal.
type SunsetExpiredEvent struct {
	MediaName   string `json:"mediaName"`
	MediaType   string `json:"mediaType"`
	DiskGroupID uint   `json:"diskGroupId"`
	SizeBytes   int64  `json:"sizeBytes"`
}

// EventType implements Event.
func (e SunsetExpiredEvent) EventType() string { return "sunset_expired" }

// EventMessage implements Event.
func (e SunsetExpiredEvent) EventMessage() string {
	return fmt.Sprintf("%s sunset countdown expired — queued for deletion", e.MediaName)
}

// SunsetRescheduledEvent is published when a sunset item's deletion date is changed.
type SunsetRescheduledEvent struct {
	MediaName        string `json:"mediaName"`
	MediaType        string `json:"mediaType"`
	DiskGroupID      uint   `json:"diskGroupId"`
	NewDaysRemaining int    `json:"newDaysRemaining"`
	NewDeletionDate  string `json:"newDeletionDate"`
}

// EventType implements Event.
func (e SunsetRescheduledEvent) EventType() string { return "sunset_rescheduled" }

// EventMessage implements Event.
func (e SunsetRescheduledEvent) EventMessage() string {
	return fmt.Sprintf("%s rescheduled — now leaving in %d days", e.MediaName, e.NewDaysRemaining)
}

// SunsetEscalatedEvent is published when a sunset-mode disk group breaches
// thresholdPct and items are force-expired to free space down to targetPct.
type SunsetEscalatedEvent struct {
	DiskGroupID  uint  `json:"diskGroupId"`
	ItemsExpired int   `json:"itemsExpired"`
	BytesFreed   int64 `json:"bytesFreed"`
}

// EventType implements Event.
func (e SunsetEscalatedEvent) EventType() string { return "sunset_escalated" }

// EventMessage implements Event.
func (e SunsetEscalatedEvent) EventMessage() string {
	return fmt.Sprintf("Sunset escalation: %d items force-expired to free space", e.ItemsExpired)
}

// SunsetMisconfiguredEvent is published when the engine skips a sunset-mode
// disk group because sunsetPct is NULL (not yet configured by the user).
type SunsetMisconfiguredEvent struct {
	DiskGroupID uint   `json:"diskGroupId"`
	MountPath   string `json:"mountPath"`
}

// EventType implements Event.
func (e SunsetMisconfiguredEvent) EventType() string { return "sunset_misconfigured" }

// EventMessage implements Event.
func (e SunsetMisconfiguredEvent) EventMessage() string {
	return fmt.Sprintf("Sunset mode skipped for %s — sunset threshold not configured", e.MountPath)
}

// SunsetLabelAppliedEvent is published when the sunset label is applied to an
// item in a media server.
type SunsetLabelAppliedEvent struct {
	MediaName     string `json:"mediaName"`
	IntegrationID uint   `json:"integrationId"`
	Label         string `json:"label"`
}

// EventType implements Event.
func (e SunsetLabelAppliedEvent) EventType() string { return "sunset_label_applied" }

// EventMessage implements Event.
func (e SunsetLabelAppliedEvent) EventMessage() string {
	return fmt.Sprintf("Label %q applied to %s", e.Label, e.MediaName)
}

// SunsetLabelRemovedEvent is published when the sunset label is removed from an
// item in a media server.
type SunsetLabelRemovedEvent struct {
	MediaName     string `json:"mediaName"`
	IntegrationID uint   `json:"integrationId"`
	Label         string `json:"label"`
}

// EventType implements Event.
func (e SunsetLabelRemovedEvent) EventType() string { return "sunset_label_removed" }

// EventMessage implements Event.
func (e SunsetLabelRemovedEvent) EventMessage() string {
	return fmt.Sprintf("Label %q removed from %s", e.Label, e.MediaName)
}

// SunsetLabelFailedEvent is published when a label operation fails on a media server.
type SunsetLabelFailedEvent struct {
	MediaName     string `json:"mediaName"`
	IntegrationID uint   `json:"integrationId"`
	Label         string `json:"label"`
	Error         string `json:"error"`
}

// EventType implements Event.
func (e SunsetLabelFailedEvent) EventType() string { return "sunset_label_failed" }

// EventMessage implements Event.
func (e SunsetLabelFailedEvent) EventMessage() string {
	return fmt.Sprintf("Failed to apply label %q to %s: %s", e.Label, e.MediaName, e.Error)
}

// SunsetSavedEvent is published when a sunset item is saved due to a score drop
// during the daily rescore check ("saved by popular demand").
type SunsetSavedEvent struct {
	MediaName     string  `json:"mediaName"`
	MediaType     string  `json:"mediaType"`
	DiskGroupID   uint    `json:"diskGroupId"`
	OriginalScore float64 `json:"originalScore"`
	NewScore      float64 `json:"newScore"`
}

// EventType implements Event.
func (e SunsetSavedEvent) EventType() string { return "sunset_saved" }

// EventMessage implements Event.
func (e SunsetSavedEvent) EventMessage() string {
	return fmt.Sprintf("%s saved by popular demand — score dropped from %.1f to %.1f", e.MediaName, e.OriginalScore, e.NewScore)
}

// SunsetSavedCleanedEvent is published when a saved item's marker duration expires
// and it is fully removed from the queue.
type SunsetSavedCleanedEvent struct {
	MediaName   string `json:"mediaName"`
	MediaType   string `json:"mediaType"`
	DiskGroupID uint   `json:"diskGroupId"`
}

// EventType implements Event.
func (e SunsetSavedCleanedEvent) EventType() string { return "sunset_saved_cleaned" }

// EventMessage implements Event.
func (e SunsetSavedCleanedEvent) EventMessage() string {
	return fmt.Sprintf("%s saved marker removed — fully restored", e.MediaName)
}

// =============================================================================
// Poster Overlay Events
// =============================================================================

// PosterOverlayAppliedEvent is published when an overlay poster is uploaded
// to a media server.
type PosterOverlayAppliedEvent struct {
	MediaName     string `json:"mediaName"`
	IntegrationID uint   `json:"integrationId"`
	DaysRemaining int    `json:"daysRemaining"`
}

// EventType implements Event.
func (e PosterOverlayAppliedEvent) EventType() string { return "poster_overlay_applied" }

// EventMessage implements Event.
func (e PosterOverlayAppliedEvent) EventMessage() string {
	return fmt.Sprintf("Poster overlay applied to %s — leaving in %d days", e.MediaName, e.DaysRemaining)
}

// PosterOverlayRestoredEvent is published when an original poster is restored
// on a media server (after cancel, expiry, or escalation).
type PosterOverlayRestoredEvent struct {
	MediaName     string `json:"mediaName"`
	IntegrationID uint   `json:"integrationId"`
}

// EventType implements Event.
func (e PosterOverlayRestoredEvent) EventType() string { return "poster_overlay_restored" }

// EventMessage implements Event.
func (e PosterOverlayRestoredEvent) EventMessage() string {
	return fmt.Sprintf("Original poster restored for %s", e.MediaName)
}

// PosterOverlayFailedEvent is published when a poster overlay operation fails.
type PosterOverlayFailedEvent struct {
	MediaName     string `json:"mediaName"`
	IntegrationID uint   `json:"integrationId"`
	Error         string `json:"error"`
}

// EventType implements Event.
func (e PosterOverlayFailedEvent) EventType() string { return "poster_overlay_failed" }

// EventMessage implements Event.
func (e PosterOverlayFailedEvent) EventMessage() string {
	return fmt.Sprintf("Poster overlay failed for %s: %s", e.MediaName, e.Error)
}

// =============================================================================
// Database Backup Events
// =============================================================================

// DatabaseBackupCompletedEvent is published when a scheduled database backup completes successfully.
type DatabaseBackupCompletedEvent struct {
	Path            string `json:"path"`
	SizeBytes       int64  `json:"sizeBytes"`
	BackupsRetained int    `json:"backupsRetained"`
}

// EventType implements Event.
func (e DatabaseBackupCompletedEvent) EventType() string { return "database_backup_completed" }

// EventMessage implements Event.
func (e DatabaseBackupCompletedEvent) EventMessage() string {
	return fmt.Sprintf("Database backup completed: %s (%d bytes, %d backups retained)", e.Path, e.SizeBytes, e.BackupsRetained)
}

// DatabaseBackupFailedEvent is published when a scheduled database backup fails.
type DatabaseBackupFailedEvent struct {
	Error string `json:"error"`
}

// EventType implements Event.
func (e DatabaseBackupFailedEvent) EventType() string { return "database_backup_failed" }

// EventMessage implements Event.
func (e DatabaseBackupFailedEvent) EventMessage() string {
	return fmt.Sprintf("Database backup failed: %s", e.Error)
}
