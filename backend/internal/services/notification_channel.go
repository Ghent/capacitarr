package services

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

// NotificationChannelService manages notification channel CRUD.
type NotificationChannelService struct {
	db  *gorm.DB
	bus *events.EventBus
}

// NewNotificationChannelService creates a new NotificationChannelService.
func NewNotificationChannelService(database *gorm.DB, bus *events.EventBus) *NotificationChannelService {
	return &NotificationChannelService{db: database, bus: bus}
}

// Create persists a new notification channel config.
func (s *NotificationChannelService) Create(config db.NotificationConfig) (*db.NotificationConfig, error) {
	if err := s.db.Create(&config).Error; err != nil {
		return nil, fmt.Errorf("failed to create notification channel: %w", err)
	}

	s.bus.Publish(events.NotificationChannelAddedEvent{
		ChannelID:   config.ID,
		ChannelType: config.Type,
		Name:        config.Name,
	})

	return &config, nil
}

// Update modifies an existing notification channel config (full-replace).
// Production code uses PartialUpdate; this method exists for test convenience.
func (s *NotificationChannelService) Update(id uint, config db.NotificationConfig) (*db.NotificationConfig, error) {
	var existing db.NotificationConfig
	if err := s.db.First(&existing, id).Error; err != nil {
		return nil, fmt.Errorf("notification channel not found: %w", err)
	}

	config.ID = id
	if err := s.db.Save(&config).Error; err != nil {
		return nil, fmt.Errorf("failed to update notification channel: %w", err)
	}

	s.bus.Publish(events.NotificationChannelUpdatedEvent{
		ChannelID:   config.ID,
		ChannelType: config.Type,
		Name:        config.Name,
	})

	return &config, nil
}

// PartialUpdate merges the provided fields into the existing notification channel
// and saves the result. Empty Name and Type are treated as "not provided" (keep existing).
// Empty or masked WebhookURL values are ignored so a round-trip save of a masked
// URL cannot clobber the stored secret. All other fields are always applied from
// req (they have meaningful zero values).
func (s *NotificationChannelService) PartialUpdate(id uint, req db.NotificationConfig) (*db.NotificationConfig, error) {
	existing, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	// Merge: only overwrite Name/Type if explicitly provided
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Type != "" {
		existing.Type = req.Type
	}

	// These fields always apply (zero values are meaningful)
	if req.WebhookURL != "" && !db.IsMaskedKey(req.WebhookURL) {
		existing.WebhookURL = req.WebhookURL
	}
	existing.AppriseTags = req.AppriseTags
	existing.Enabled = req.Enabled
	existing.NotificationLevel = req.NotificationLevel
	existing.OverrideCycleDigest = req.OverrideCycleDigest
	existing.OverrideError = req.OverrideError
	existing.OverrideModeChanged = req.OverrideModeChanged
	existing.OverrideServerStarted = req.OverrideServerStarted
	existing.OverrideThresholdBreach = req.OverrideThresholdBreach
	existing.OverrideUpdateAvailable = req.OverrideUpdateAvailable
	existing.OverrideApprovalActivity = req.OverrideApprovalActivity
	existing.OverrideIntegrationStatus = req.OverrideIntegrationStatus
	existing.UpdatedAt = time.Now()

	if err := s.db.Save(existing).Error; err != nil {
		return nil, fmt.Errorf("failed to update notification channel: %w", err)
	}

	s.bus.Publish(events.NotificationChannelUpdatedEvent{
		ChannelID:   existing.ID,
		ChannelType: existing.Type,
		Name:        existing.Name,
	})

	return existing, nil
}

// Delete removes a notification channel config.
func (s *NotificationChannelService) Delete(id uint) error {
	var config db.NotificationConfig
	if err := s.db.First(&config, id).Error; err != nil {
		return fmt.Errorf("notification channel not found: %w", err)
	}

	if err := s.db.Delete(&config).Error; err != nil {
		return fmt.Errorf("failed to delete notification channel: %w", err)
	}

	s.bus.Publish(events.NotificationChannelRemovedEvent{
		ChannelID:   config.ID,
		ChannelType: config.Type,
		Name:        config.Name,
	})

	return nil
}

// List returns all notification channel configs ordered by id ascending.
func (s *NotificationChannelService) List() ([]db.NotificationConfig, error) {
	var configs []db.NotificationConfig
	if err := s.db.Order("id ASC").Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("failed to list notification channels: %w", err)
	}
	return configs, nil
}

// GetByID returns a single notification channel config by primary key.
// Returns ErrNotFound if the record does not exist.
func (s *NotificationChannelService) GetByID(id uint) (*db.NotificationConfig, error) {
	var config db.NotificationConfig
	if err := s.db.First(&config, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get notification channel: %w", err)
	}
	return &config, nil
}

// ListEnabled returns all notification channel configs where enabled = true.
func (s *NotificationChannelService) ListEnabled() ([]db.NotificationConfig, error) {
	var configs []db.NotificationConfig
	if err := s.db.Where("enabled = ?", true).Find(&configs).Error; err != nil {
		return nil, fmt.Errorf("failed to list enabled notification channels: %w", err)
	}
	return configs, nil
}
