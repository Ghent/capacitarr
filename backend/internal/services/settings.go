package services

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/logger"
)

// SettingsService manages application preferences and activity events.
type SettingsService struct {
	db  *gorm.DB
	bus *events.EventBus
}

// NewSettingsService creates a new SettingsService.
func NewSettingsService(database *gorm.DB, bus *events.EventBus) *SettingsService {
	return &SettingsService{db: database, bus: bus}
}

// GetPreferences returns the current preferences (singleton row).
func (s *SettingsService) GetPreferences() (db.PreferenceSet, error) {
	var pref db.PreferenceSet
	if err := s.db.FirstOrCreate(&pref, db.PreferenceSet{ID: 1}).Error; err != nil {
		return pref, fmt.Errorf("failed to fetch preferences: %w", err)
	}
	return pref, nil
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

	// Detect execution mode change
	if oldPrefs.ExecutionMode != payload.ExecutionMode {
		s.bus.Publish(events.EngineModeChangedEvent{
			OldMode: oldPrefs.ExecutionMode,
			NewMode: payload.ExecutionMode,
		})
	}

	s.bus.Publish(events.SettingsChangedEvent{})

	return payload, nil
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
