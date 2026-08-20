package services

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/logger"
)

// DeletionQueueClearer allows SettingsService to clear the deletion queue
// when execution mode changes, without importing DeletionService directly.
type DeletionQueueClearer interface {
	ClearQueue() int
}

// SunsetLabelMigrator allows SettingsService to trigger sunset label migration
// when the user changes the sunset label, without importing SunsetService directly.
type SunsetLabelMigrator interface {
	MigrateSunsetLabel(oldLabel, newLabel string) error
}

// SettingsService manages application preferences and activity events.
type SettingsService struct {
	db              *gorm.DB
	bus             *events.EventBus
	deletionClearer DeletionQueueClearer // injected after construction via SetDeletionClearer()
	labelMigrator   SunsetLabelMigrator  // injected after construction via SetLabelMigrator()
}

// NewSettingsService creates a new SettingsService.
func NewSettingsService(database *gorm.DB, bus *events.EventBus) *SettingsService {
	return &SettingsService{db: database, bus: bus}
}

// Ping checks that the database is reachable. Used by GET /health.
func (s *SettingsService) Ping() error {
	return s.db.Exec("SELECT 1").Error
}

// Wired returns true when all lazily-injected dependencies are non-nil.
// Used by Registry.Validate() to catch missing wiring at startup.
func (s *SettingsService) Wired() bool {
	return s.deletionClearer != nil && s.labelMigrator != nil
}

// SetDeletionClearer wires the cross-service dependency that allows
// SettingsService to clear the deletion queue on execution mode changes.
func (s *SettingsService) SetDeletionClearer(clearer DeletionQueueClearer) {
	s.deletionClearer = clearer
}

// SetLabelMigrator wires the cross-service dependency that allows
// SettingsService to trigger sunset label migration when the sunset
// label preference changes.
func (s *SettingsService) SetLabelMigrator(migrator SunsetLabelMigrator) {
	s.labelMigrator = migrator
}

// sunsetLabelMigratorAdapter implements SunsetLabelMigrator by building a
// fresh integration registry and delegating to SunsetService.MigrateLabel.
type sunsetLabelMigratorAdapter struct {
	sunset      *SunsetService
	integration *IntegrationService
	mapping     *MappingService
}

// MigrateSunsetLabel builds a fresh integration registry and migrates labels.
func (a *sunsetLabelMigratorAdapter) MigrateSunsetLabel(oldLabel, newLabel string) error {
	registry, err := a.integration.BuildIntegrationRegistry()
	if err != nil {
		return fmt.Errorf("build registry for label migration: %w", err)
	}
	return a.sunset.MigrateLabel(oldLabel, newLabel, registry, a.mapping)
}

// NewSunsetLabelMigrator creates a SunsetLabelMigrator that delegates to the
// given services. Used to wire SettingsService without a direct SunsetService import.
func NewSunsetLabelMigrator(sunset *SunsetService, integration *IntegrationService, mapping *MappingService) SunsetLabelMigrator {
	return &sunsetLabelMigratorAdapter{sunset: sunset, integration: integration, mapping: mapping}
}

// GetPreferences returns the current preferences (singleton row).
func (s *SettingsService) GetPreferences() (db.PreferenceSet, error) {
	var pref db.PreferenceSet
	if err := s.db.FirstOrCreate(&pref, db.PreferenceSet{ID: 1}).Error; err != nil {
		return pref, fmt.Errorf("failed to fetch preferences: %w", err)
	}
	return pref, nil
}

// ─── Patch Request Types ────────────────────────────────────────────────────

// EnginePreferencePatch contains fields for the engine behavior settings group.
type EnginePreferencePatch struct {
	DefaultDiskGroupMode *string `json:"defaultDiskGroupMode"`
	TiebreakerMethod     *string `json:"tiebreakerMethod"`
	DeletionsEnabled     *bool   `json:"deletionsEnabled"`
	SnoozeDurationHours  *int    `json:"snoozeDurationHours"`
}

// SunsetPreferencePatch contains fields for the sunset behavior settings group.
type SunsetPreferencePatch struct {
	SunsetDays           *int    `json:"sunsetDays"`
	SunsetLabel          *string `json:"sunsetLabel"`
	PosterOverlayStyle   *string `json:"posterOverlayStyle"`
	SunsetRescoreEnabled *bool   `json:"sunsetRescoreEnabled"`
	SavedDurationDays    *int    `json:"savedDurationDays"`
	SavedLabel           *string `json:"savedLabel"`
}

// validOverlayStyles is the set of accepted values for PosterOverlayStyle.
var validOverlayStyles = map[string]bool{
	"off":       true,
	"countdown": true,
	"simple":    true,
}

// ContentPreferencePatch contains fields for the content analytics settings group.
type ContentPreferencePatch struct {
	DeadContentMinDays *int `json:"deadContentMinDays"`
	StaleContentDays   *int `json:"staleContentDays"`
}

// AdvancedPreferencePatch contains fields for the advanced settings group.
type AdvancedPreferencePatch struct {
	LogLevel                  *string `json:"logLevel"`
	PollIntervalSeconds       *int    `json:"pollIntervalSeconds"`
	DeletionQueueDelaySeconds *int    `json:"deletionQueueDelaySeconds"`
	AuditLogRetentionDays     *int    `json:"auditLogRetentionDays"`
	CheckForUpdates           *bool   `json:"checkForUpdates"`
	BackupRetentionDays       *int    `json:"backupRetentionDays"`
}

// UpdatePreferences saves preference changes and publishes relevant events.
func (s *SettingsService) UpdatePreferences(payload db.PreferenceSet) (db.PreferenceSet, error) {
	payload.ID = 1

	// Snapshot current for change detection
	var oldPrefs db.PreferenceSet
	s.db.FirstOrCreate(&oldPrefs, db.PreferenceSet{ID: 1})

	if err := s.db.Save(&payload).Error; err != nil {
		return payload, fmt.Errorf("failed to save preferences: %w", err)
	}

	// Apply dynamic log level
	logger.SetLevel(payload.LogLevel)

	// Detect DeletionsEnabled toggle (true → false) — same class of bug:
	// auto-mode items in the grace period would still execute if we don't
	// clear the queue when the user disables deletions.
	if oldPrefs.DeletionsEnabled && !payload.DeletionsEnabled {
		if s.deletionClearer != nil {
			cleared := s.deletionClearer.ClearQueue()
			if cleared > 0 {
				slog.Info("Cleared deletion queue on deletions disabled",
					"component", "services", "cleared", cleared)
			}
		}
	}

	s.bus.Publish(events.SettingsChangedEvent{})

	return payload, nil
}

// PatchEnginePreferences updates only the provided engine behavior fields.
func (s *SettingsService) PatchEnginePreferences(patch EnginePreferencePatch) (db.PreferenceSet, error) {
	var oldPrefs db.PreferenceSet
	s.db.FirstOrCreate(&oldPrefs, db.PreferenceSet{ID: 1})

	updates := make(map[string]any)
	if patch.DefaultDiskGroupMode != nil {
		updates["default_disk_group_mode"] = *patch.DefaultDiskGroupMode
	}
	if patch.TiebreakerMethod != nil {
		updates["tiebreaker_method"] = *patch.TiebreakerMethod
	}
	if patch.DeletionsEnabled != nil {
		updates["deletions_enabled"] = *patch.DeletionsEnabled
	}
	if patch.SnoozeDurationHours != nil {
		updates["snooze_duration_hours"] = *patch.SnoozeDurationHours
	}

	if len(updates) == 0 {
		return oldPrefs, nil
	}

	if err := s.db.Model(&db.PreferenceSet{}).Where("id = ?", 1).Updates(updates).Error; err != nil {
		return db.PreferenceSet{}, fmt.Errorf("failed to patch engine preferences: %w", err)
	}

	// Re-read for side-effect detection and response
	var newPrefs db.PreferenceSet
	s.db.First(&newPrefs, 1)

	// Detect deletions disabled → clear queue
	if patch.DeletionsEnabled != nil && oldPrefs.DeletionsEnabled && !*patch.DeletionsEnabled {
		if s.deletionClearer != nil {
			cleared := s.deletionClearer.ClearQueue()
			if cleared > 0 {
				slog.Info("Cleared deletion queue on deletions disabled",
					"component", "services", "cleared", cleared)
			}
		}
	}

	s.bus.Publish(events.SettingsChangedEvent{})
	return newPrefs, nil
}

// PatchSunsetPreferences updates only the provided sunset behavior fields.
func (s *SettingsService) PatchSunsetPreferences(patch SunsetPreferencePatch) (db.PreferenceSet, error) {
	// Snapshot the current sunset label before applying updates so we can
	// detect changes and trigger label migration.
	var oldPrefs db.PreferenceSet
	s.db.FirstOrCreate(&oldPrefs, db.PreferenceSet{ID: 1})

	updates := make(map[string]any)
	if patch.SunsetDays != nil {
		updates["sunset_days"] = *patch.SunsetDays
	}
	if patch.SunsetLabel != nil {
		updates["sunset_label"] = *patch.SunsetLabel
	}
	if patch.PosterOverlayStyle != nil {
		if !validOverlayStyles[*patch.PosterOverlayStyle] {
			return db.PreferenceSet{}, fmt.Errorf("invalid poster overlay style: %q", *patch.PosterOverlayStyle)
		}
		updates["poster_overlay_style"] = *patch.PosterOverlayStyle
	}
	if patch.SunsetRescoreEnabled != nil {
		updates["sunset_rescore_enabled"] = *patch.SunsetRescoreEnabled
	}
	if patch.SavedDurationDays != nil {
		updates["saved_duration_days"] = *patch.SavedDurationDays
	}
	if patch.SavedLabel != nil {
		updates["saved_label"] = *patch.SavedLabel
	}

	if len(updates) == 0 {
		return oldPrefs, nil
	}

	if err := s.db.Model(&db.PreferenceSet{}).Where("id = ?", 1).Updates(updates).Error; err != nil {
		return db.PreferenceSet{}, fmt.Errorf("failed to patch sunset preferences: %w", err)
	}

	var pref db.PreferenceSet
	s.db.First(&pref, 1)

	// Migrate sunset labels on media server items if the label changed
	if patch.SunsetLabel != nil && *patch.SunsetLabel != oldPrefs.SunsetLabel && s.labelMigrator != nil {
		if err := s.labelMigrator.MigrateSunsetLabel(oldPrefs.SunsetLabel, *patch.SunsetLabel); err != nil {
			slog.Error("Failed to migrate sunset labels",
				"component", "services", "oldLabel", oldPrefs.SunsetLabel,
				"newLabel", *patch.SunsetLabel, "error", err)
		}
	}

	s.bus.Publish(events.SettingsChangedEvent{})
	return pref, nil
}

// PatchContentPreferences updates only the provided content analytics fields.
func (s *SettingsService) PatchContentPreferences(patch ContentPreferencePatch) (db.PreferenceSet, error) {
	updates := make(map[string]any)
	if patch.DeadContentMinDays != nil {
		updates["dead_content_min_days"] = *patch.DeadContentMinDays
	}
	if patch.StaleContentDays != nil {
		updates["stale_content_days"] = *patch.StaleContentDays
	}

	if len(updates) == 0 {
		var pref db.PreferenceSet
		s.db.FirstOrCreate(&pref, db.PreferenceSet{ID: 1})
		return pref, nil
	}

	if err := s.db.Model(&db.PreferenceSet{}).Where("id = ?", 1).Updates(updates).Error; err != nil {
		return db.PreferenceSet{}, fmt.Errorf("failed to patch content preferences: %w", err)
	}

	var pref db.PreferenceSet
	s.db.First(&pref, 1)

	s.bus.Publish(events.SettingsChangedEvent{})
	return pref, nil
}

// PatchAdvancedPreferences updates only the provided advanced settings fields.
func (s *SettingsService) PatchAdvancedPreferences(patch AdvancedPreferencePatch) (db.PreferenceSet, error) {
	updates := make(map[string]any)
	if patch.LogLevel != nil {
		updates["log_level"] = *patch.LogLevel
	}
	if patch.PollIntervalSeconds != nil {
		updates["poll_interval_seconds"] = *patch.PollIntervalSeconds
	}
	if patch.DeletionQueueDelaySeconds != nil {
		updates["deletion_queue_delay_seconds"] = *patch.DeletionQueueDelaySeconds
	}
	if patch.AuditLogRetentionDays != nil {
		updates["audit_log_retention_days"] = *patch.AuditLogRetentionDays
	}
	if patch.CheckForUpdates != nil {
		updates["check_for_updates"] = *patch.CheckForUpdates
	}
	if patch.BackupRetentionDays != nil {
		updates["backup_retention_days"] = *patch.BackupRetentionDays
	}

	if len(updates) == 0 {
		var pref db.PreferenceSet
		s.db.FirstOrCreate(&pref, db.PreferenceSet{ID: 1})
		return pref, nil
	}

	if err := s.db.Model(&db.PreferenceSet{}).Where("id = ?", 1).Updates(updates).Error; err != nil {
		return db.PreferenceSet{}, fmt.Errorf("failed to patch advanced preferences: %w", err)
	}

	var pref db.PreferenceSet
	s.db.First(&pref, 1)

	// Apply dynamic log level if it was changed
	if patch.LogLevel != nil {
		logger.SetLevel(*patch.LogLevel)
	}

	s.bus.Publish(events.SettingsChangedEvent{})
	return pref, nil
}

// PruneOldActivities deletes activity events older than the given number of days.
func (s *SettingsService) PruneOldActivities(days int) (int64, error) {
	if days <= 0 {
		return 0, nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -days)
	result := s.db.Where("created_at < ?", cutoff).Delete(&db.ActivityEvent{})
	if result.Error != nil {
		return 0, fmt.Errorf("failed to prune activity events: %w", result.Error)
	}
	return result.RowsAffected, nil
}

// CreateActivity creates a new activity event record.
func (s *SettingsService) CreateActivity(eventType, message, metadata string) error {
	entry := db.ActivityEvent{
		EventType: eventType,
		Message:   message,
		Metadata:  metadata,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.db.Create(&entry).Error; err != nil {
		return fmt.Errorf("failed to create activity event: %w", err)
	}
	return nil
}

// ListRecentActivities returns the most recent N activity events.
func (s *SettingsService) ListRecentActivities(limit int) ([]db.ActivityEvent, error) {
	activities := make([]db.ActivityEvent, 0, limit)
	if err := s.db.Order("created_at desc").Limit(limit).Find(&activities).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch recent activities: %w", err)
	}
	return activities, nil
}

// ─── Scoring Factor Weights ─────────────────────────────────────────────────

// ListFactorWeights returns all scoring factor weight rows, ordered by factor_key.
func (s *SettingsService) ListFactorWeights() ([]db.ScoringFactorWeight, error) {
	var weights []db.ScoringFactorWeight
	if err := s.db.Order("factor_key").Find(&weights).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch scoring factor weights: %w", err)
	}
	return weights, nil
}

// GetWeightMap returns a map of factor_key → weight for use in the scoring engine.
func (s *SettingsService) GetWeightMap() (map[string]int, error) {
	weights, err := s.ListFactorWeights()
	if err != nil {
		return nil, err
	}
	m := make(map[string]int, len(weights))
	for _, w := range weights {
		m[w.FactorKey] = w.Weight
	}
	return m, nil
}

// UpdateFactorWeights updates the weight for each factor key in the map.
// Keys that don't exist in the DB are silently skipped. Values are clamped to 0-10.
func (s *SettingsService) UpdateFactorWeights(weights map[string]int) error {
	for key, weight := range weights {
		// Clamp to 0-10
		if weight < 0 {
			weight = 0
		}
		if weight > 10 {
			weight = 10
		}
		if err := s.db.Model(&db.ScoringFactorWeight{}).
			Where("factor_key = ?", key).
			Updates(map[string]any{"weight": weight, "updated_at": time.Now().UTC()}).Error; err != nil {
			return fmt.Errorf("failed to update weight for factor %q: %w", key, err)
		}
	}

	s.bus.Publish(events.SettingsChangedEvent{})
	return nil
}
