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

// Service type constants for rule field definitions.
const serviceTypeSonarr = "sonarr"

// Rule field name constants (shared across services to satisfy goconst linter).
const ruleFieldSeriesStatus = "seriesstatus"

// arrServiceTypes maps *arr integration type strings to true for quick lookup.
var arrServiceTypes = map[string]bool{
	serviceTypeSonarr: true,
	"radarr":          true,
	"lidarr":          true,
	"readarr":         true,
}

// IntegrationContextProvider provides integration metadata needed for
// building rule context (field definitions + value options). Defined here
// to avoid import cycles between RulesService and IntegrationService.
type IntegrationContextProvider interface {
	GetByID(id uint) (*db.IntegrationConfig, error)
	DetectEnrichment() EnrichmentPresence
	FetchRuleValues(integrationID uint, action string) (any, error)
}

// RulesService manages custom rule CRUD and reordering.
type RulesService struct {
	db           *gorm.DB
	bus          *events.EventBus
	preview      PreviewDataSource          // optional, for rule impact preview
	integrations IntegrationContextProvider // optional, for rule context endpoint
}

// NewRulesService creates a new RulesService.
func NewRulesService(database *gorm.DB, bus *events.EventBus) *RulesService {
	return &RulesService{db: database, bus: bus}
}

// SetPreviewSource sets the preview data source for rule impact calculations.
func (s *RulesService) SetPreviewSource(preview PreviewDataSource) {
	s.preview = preview
}

// SetIntegrationProvider wires the integration context provider for
// GetRuleContext(). Called from NewRegistry() after both services are created.
func (s *RulesService) SetIntegrationProvider(provider IntegrationContextProvider) {
	s.integrations = provider
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

// FieldDef describes a rule field available for matching.
type FieldDef struct {
	Field     string   `json:"field"`
	Label     string   `json:"label"`
	Type      string   `json:"type"`
	Operators []string `json:"operators"`
}

// EnrichmentPresence tracks which enrichment integration types are enabled.
// Used by GetFieldDefinitions to conditionally include enrichment-dependent fields.
type EnrichmentPresence struct {
	HasTautulli bool
	HasSeerr    bool
	HasMedia    bool
	HasSonarr   bool
}

// GetFieldDefinitions returns available rule fields based on the service type
// and enrichment integrations. If serviceType is empty, returns all fields
// (including Sonarr-specific fields if Sonarr is enabled). The enrichment
// parameter controls which enrichment-dependent fields are included.
func (s *RulesService) GetFieldDefinitions(serviceType string, enrichment EnrichmentPresence) []FieldDef {
	// Base fields available for all *arr integration types
	fields := []FieldDef{
		{Field: "title", Label: "Title", Type: "string", Operators: []string{"==", "!=", "contains", "!contains"}},
		{Field: "quality", Label: "Quality Profile", Type: "string", Operators: []string{"==", "!="}},
		{Field: "tag", Label: "Tags", Type: "string", Operators: []string{"contains", "!contains"}},
		{Field: "genre", Label: "Genre", Type: "string", Operators: []string{"==", "!=", "contains", "!contains"}},
		{Field: "rating", Label: "Rating", Type: "number", Operators: []string{"==", "!=", ">", ">=", "<", "<="}},
		{Field: "sizebytes", Label: "Size (bytes)", Type: "number", Operators: []string{">", ">=", "<", "<="}},
		{Field: "timeinlibrary", Label: "Time in Library (days)", Type: "number", Operators: []string{">", ">=", "<", "<=", "in_last", "over_ago"}},
		{Field: "monitored", Label: "Monitored", Type: "boolean", Operators: []string{"=="}},
		{Field: "year", Label: "Year", Type: "number", Operators: []string{"==", "!=", ">", ">=", "<", "<="}},
		{Field: "language", Label: "Language", Type: "string", Operators: []string{"==", "!="}},
	}

	// Sonarr-specific fields
	sonarrFields := []FieldDef{
		{Field: ruleFieldSeriesStatus, Label: "Show Status", Type: "string", Operators: []string{"==", "!="}},
		{Field: "seasoncount", Label: "Season Count", Type: "number", Operators: []string{"==", "!=", ">", ">=", "<", "<="}},
		{Field: "episodecount", Label: "Episode Count", Type: "number", Operators: []string{"==", "!=", ">", ">=", "<", "<="}},
	}

	if serviceType == serviceTypeSonarr {
		fields = append(fields, sonarrFields...)
	} else if serviceType == "" && enrichment.HasSonarr {
		fields = append(fields, sonarrFields...)
	}

	// Enrichment fields — add for *arr service types or when unfiltered
	addEnrichment := serviceType == ""
	if !addEnrichment {
		addEnrichment = arrServiceTypes[serviceType]
	}
	if addEnrichment {
		fields = appendEnrichmentFieldDefs(fields, enrichment)
	}

	// Media Type field (always available)
	fields = append(fields, FieldDef{Field: "type", Label: "Media Type", Type: "string", Operators: []string{"==", "!="}})

	return fields
}

// appendEnrichmentFieldDefs adds enrichment-dependent rule fields based on
// which enrichment integrations are enabled.
func appendEnrichmentFieldDefs(fields []FieldDef, p EnrichmentPresence) []FieldDef {
	if p.HasTautulli || p.HasMedia {
		fields = append(fields,
			FieldDef{Field: "playcount", Label: "Play Count", Type: "number", Operators: []string{"==", "!=", ">", ">=", "<", "<="}},
			FieldDef{Field: "lastplayed", Label: "Last Watched", Type: "date", Operators: []string{"in_last", "over_ago", "never"}},
		)
	}
	if p.HasSeerr {
		fields = append(fields,
			FieldDef{Field: "requested", Label: "Is Requested", Type: "boolean", Operators: []string{"=="}},
			FieldDef{Field: "requestcount", Label: "Request Count", Type: "number", Operators: []string{"==", "!=", ">", ">=", "<", "<="}},
			FieldDef{Field: "requestedby", Label: "Requested By", Type: "string", Operators: []string{"==", "!=", "contains", "!contains"}},
		)
	}
	if p.HasMedia {
		fields = append(fields,
			FieldDef{Field: "incollection", Label: "In Collection", Type: "boolean", Operators: []string{"=="}},
			FieldDef{Field: "watchlist", Label: "On Watchlist", Type: "boolean", Operators: []string{"=="}},
			FieldDef{Field: "collection", Label: "Collection Name", Type: "string", Operators: []string{"==", "!=", "contains", "!contains"}},
		)
	}
	if p.HasSeerr && (p.HasTautulli || p.HasMedia) {
		fields = append(fields,
			FieldDef{Field: "watchedbyreq", Label: "Watched by Requestor", Type: "boolean", Operators: []string{"=="}},
		)
	}
	return fields
}

// RuleContext contains all data needed to prepopulate the rule editor for
// an existing rule. Returns field definitions, value options, and the rule
// itself in a single response to minimize round-trips.
type RuleContext struct {
	Rule   db.CustomRule `json:"rule"`
	Fields []FieldDef    `json:"fields"`
	Values any           `json:"values,omitempty"`
}

// GetRuleContext returns the rule, its available field definitions, and value
// options/suggestions for the rule's current field. This provides all data the
// frontend needs to prepopulate the rule editor in a single round-trip.
func (s *RulesService) GetRuleContext(id uint) (*RuleContext, error) {
	var rule db.CustomRule
	if err := s.db.First(&rule, id).Error; err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuleNotFound, err)
	}

	if s.integrations == nil || rule.IntegrationID == nil {
		// No integration provider wired or rule has no integration — return rule with empty fields/values
		return &RuleContext{Rule: rule}, nil
	}

	integrationID := *rule.IntegrationID

	// Look up the integration to determine the service type
	config, err := s.integrations.GetByID(integrationID)
	if err != nil {
		slog.Warn("Failed to get integration for rule context", "ruleId", id, "integrationId", integrationID, "error", err)
		// Still return the rule — fields/values are nice-to-have
		return &RuleContext{Rule: rule}, nil
	}

	enrichment := s.integrations.DetectEnrichment()
	fields := s.GetFieldDefinitions(config.Type, enrichment)

	// Fetch value options for the current field
	var values any
	if rule.Field != "" {
		ruleValues, valErr := s.integrations.FetchRuleValues(integrationID, rule.Field)
		if valErr != nil {
			slog.Warn("Failed to fetch rule values for context", "ruleId", id, "field", rule.Field, "error", valErr)
			// Non-fatal — values are nice-to-have for prepopulation
		} else {
			values = ruleValues
		}
	}

	return &RuleContext{
		Rule:   rule,
		Fields: fields,
		Values: values,
	}, nil
}
