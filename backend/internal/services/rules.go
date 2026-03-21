package services

import (
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/engine"
	"capacitarr/internal/events"
)

// ErrRuleNotFound is returned when a rule cannot be found by ID.
var ErrRuleNotFound = errors.New("rule not found")

// ErrRuleValidation is returned when a rule fails input validation.
var ErrRuleValidation = errors.New("rule validation failed")

// RulesService manages custom rule CRUD and reordering.
type RulesService struct {
	db      *gorm.DB
	bus     *events.EventBus
	preview PreviewDataSource // optional, for rule impact preview
}

// NewRulesService creates a new RulesService.
func NewRulesService(database *gorm.DB, bus *events.EventBus) *RulesService {
	return &RulesService{db: database, bus: bus}
}

// SetPreviewSource sets the preview data source for rule impact calculations.
func (s *RulesService) SetPreviewSource(preview PreviewDataSource) {
	s.preview = preview
}

// List returns all custom rules ordered by sort_order ASC, id ASC.
func (s *RulesService) List() ([]db.CustomRule, error) {
	rules := make([]db.CustomRule, 0)
	if err := s.db.Order("sort_order ASC, id ASC").Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch custom rules: %w", err)
	}
	return rules, nil
}

// GetEnabledRules returns only enabled custom rules, ordered by sort_order ASC, id ASC.
// Used by analytics services to check protection status without including disabled rules.
func (s *RulesService) GetEnabledRules() ([]db.CustomRule, error) {
	rules := make([]db.CustomRule, 0)
	if err := s.db.Where("enabled = ?", true).Order("sort_order ASC, id ASC").Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch enabled rules: %w", err)
	}
	return rules, nil
}

// validateRule checks required fields and effect validity.
// Called by both Create() and Update() to maintain invariants.
func (s *RulesService) validateRule(rule db.CustomRule) error {
	if rule.Field == "" || rule.Operator == "" || rule.Value == "" {
		return fmt.Errorf("%w: field, operator, and value are required", ErrRuleValidation)
	}
	if rule.Effect == "" {
		return fmt.Errorf("%w: effect field is required", ErrRuleValidation)
	}
	if !db.ValidEffects[rule.Effect] {
		return fmt.Errorf("%w: effect must be one of: always_keep, prefer_keep, lean_keep, lean_remove, prefer_remove, always_remove", ErrRuleValidation)
	}
	return nil
}

// Create validates and persists a new custom rule.
func (s *RulesService) Create(rule db.CustomRule) (*db.CustomRule, error) {
	if err := s.validateRule(rule); err != nil {
		return nil, err
	}

	// Ensure new rules are enabled by default
	rule.Enabled = true

	if err := s.db.Create(&rule).Error; err != nil {
		slog.Error("Failed to create custom rule", "component", "services", "error", err)
		return nil, fmt.Errorf("failed to create rule: %w", err)
	}

	s.bus.Publish(events.RuleCreatedEvent{
		RuleID: rule.ID,
		Field:  rule.Field,
		Effect: rule.Effect,
	})

	return &rule, nil
}

// Update modifies an existing custom rule identified by id.
func (s *RulesService) Update(id uint, rule db.CustomRule) (*db.CustomRule, error) {
	var existing db.CustomRule
	if err := s.db.First(&existing, id).Error; err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleNotFound, err)
	}

	if err := s.validateRule(rule); err != nil {
		return nil, err
	}

	// Preserve the ID and sort order from the existing record
	rule.ID = existing.ID
	if rule.SortOrder == 0 {
		rule.SortOrder = existing.SortOrder
	}

	if err := s.db.Save(&rule).Error; err != nil {
		slog.Error("Failed to update custom rule", "component", "services", "id", id, "error", err)
		return nil, fmt.Errorf("failed to update rule: %w", err)
	}

	s.bus.Publish(events.RuleUpdatedEvent{
		RuleID: rule.ID,
		Field:  rule.Field,
		Effect: rule.Effect,
	})

	return &rule, nil
}

// Delete removes a custom rule identified by id.
func (s *RulesService) Delete(id uint) error {
	var existing db.CustomRule
	if err := s.db.First(&existing, id).Error; err != nil {
		return fmt.Errorf("%w: %v", ErrRuleNotFound, err)
	}

	if err := s.db.Delete(&existing).Error; err != nil {
		slog.Error("Failed to delete custom rule", "component", "services", "id", id, "error", err)
		return fmt.Errorf("failed to delete rule: %w", err)
	}

	s.bus.Publish(events.RuleDeletedEvent{
		RuleID: existing.ID,
		Field:  existing.Field,
	})

	return nil
}

// Reorder updates the sort_order for each rule ID in the provided slice.
// The position in the slice determines the new sort_order value.
func (s *RulesService) Reorder(ids []uint) error {
	tx := s.db.Begin()
	for idx, ruleID := range ids {
		if err := tx.Model(&db.CustomRule{}).Where("id = ?", ruleID).Update("sort_order", idx).Error; err != nil {
			tx.Rollback()
			slog.Error("Failed to update rule sort order", "component", "services", "ruleId", ruleID, "error", err)
			return fmt.Errorf("failed to reorder rules: %w", err)
		}
	}
	return tx.Commit().Error
}

// RuleImpact holds the impact preview for a single rule.
type RuleImpact struct {
	RuleID        uint `json:"ruleId"`
	AffectedCount int  `json:"affectedCount"`
	TotalItems    int  `json:"totalItems"`
}

// GetRuleImpact returns how many preview cache items the given rule affects.
// Uses the engine's rule matching logic against the current preview cache.
func (s *RulesService) GetRuleImpact(ruleID uint) (*RuleImpact, error) {
	var rule db.CustomRule
	if err := s.db.First(&rule, ruleID).Error; err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleNotFound, err)
	}

	if s.preview == nil {
		return &RuleImpact{RuleID: ruleID, AffectedCount: 0, TotalItems: 0}, nil
	}

	items := s.preview.GetCachedItems()
	if len(items) == 0 {
		return &RuleImpact{RuleID: ruleID, AffectedCount: 0, TotalItems: 0}, nil
	}

	// Test the single rule against each item using the engine
	singleRule := []db.CustomRule{rule}
	affected := 0
	for _, item := range items {
		isProtected, modifier, _, _ := engine.ApplyRulesExported(item, singleRule)
		// If the rule matched, isProtected will be true or modifier will differ from 1.0
		if isProtected || modifier != 1.0 {
			affected++
		}
	}

	return &RuleImpact{
		RuleID:        ruleID,
		AffectedCount: affected,
		TotalItems:    len(items),
	}, nil
}
