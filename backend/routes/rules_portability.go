package routes

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/services"
)

// exportRulePayload is a single rule in the portable export format.
// Integration fields are pointers so they serialise as JSON null when absent.
type exportRulePayload struct {
	Field           string  `json:"field"`
	Operator        string  `json:"operator"`
	Value           string  `json:"value"`
	Effect          string  `json:"effect"`
	Enabled         bool    `json:"enabled"`
	IntegrationName *string `json:"integrationName"`
	IntegrationType *string `json:"integrationType"`
}

// exportEnvelope wraps exported rules with version metadata.
type exportEnvelope struct {
	Version    int                 `json:"version"`
	ExportedAt string              `json:"exportedAt"`
	Rules      []exportRulePayload `json:"rules"`
}

// importRulePayload is a single rule in the incoming import payload.
type importRulePayload struct {
	Field           string  `json:"field"`
	Operator        string  `json:"operator"`
	Value           string  `json:"value"`
	Effect          string  `json:"effect"`
	Enabled         bool    `json:"enabled"`
	IntegrationName *string `json:"integrationName"`
	IntegrationType *string `json:"integrationType"`
}

// importEnvelope wraps the rules array with version metadata.
type importEnvelope struct {
	Version    int                 `json:"version"`
	ExportedAt string              `json:"exportedAt"`
	Rules      []importRulePayload `json:"rules"`
}

// importRequest is the top-level request body for POST /custom-rules/import.
type importRequest struct {
	Payload            importEnvelope  `json:"payload"`
	IntegrationMapping map[string]uint `json:"integrationMapping"`
}

// validEffects enumerates the allowed effect values for custom rules.
var validEffects = map[string]bool{
	"always_keep": true, "prefer_keep": true, "lean_keep": true,
	"lean_remove": true, "prefer_remove": true, "always_remove": true,
}

// RegisterRulePortabilityRoutes sets up the export/import endpoints for custom rules.
func RegisterRulePortabilityRoutes(protected *echo.Group, reg *services.Registry) {
	database := reg.DB
	bus := reg.Bus

	protected.GET("/custom-rules/export", handleExportRules(database, bus))
	protected.POST("/custom-rules/import", handleImportRules(database, bus))
}

// handleExportRules returns all custom rules in a portable JSON format.
func handleExportRules(database *gorm.DB, bus *events.EventBus) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Fetch all rules ordered by sort_order
		rules := make([]db.CustomRule, 0)
		if err := database.Order("sort_order ASC, id ASC").Find(&rules).Error; err != nil {
			slog.Error("Failed to fetch rules for export", "component", "api", "operation", "export_rules", "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch rules"})
		}

		// Collect all integration IDs that are referenced
		integrationIDs := make([]uint, 0)
		for _, r := range rules {
			if r.IntegrationID != nil {
				integrationIDs = append(integrationIDs, *r.IntegrationID)
			}
		}

		// Batch-load referenced integrations into a lookup map
		integrationMap := make(map[uint]db.IntegrationConfig)
		if len(integrationIDs) > 0 {
			var integrations []db.IntegrationConfig
			if err := database.Where("id IN ?", integrationIDs).Find(&integrations).Error; err != nil {
				slog.Error("Failed to fetch integrations for export", "component", "api", "operation", "export_rules", "error", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch integrations"})
			}
			for _, ic := range integrations {
				integrationMap[ic.ID] = ic
			}
		}

		// Build the portable payload
		exported := make([]exportRulePayload, 0, len(rules))
		for _, r := range rules {
			ep := exportRulePayload{
				Field:    r.Field,
				Operator: r.Operator,
				Value:    r.Value,
				Effect:   r.Effect,
				Enabled:  r.Enabled,
			}
			if r.IntegrationID != nil {
				if ic, ok := integrationMap[*r.IntegrationID]; ok {
					ep.IntegrationName = &ic.Name
					ep.IntegrationType = &ic.Type
				}
			}
			exported = append(exported, ep)
		}

		now := time.Now().UTC()
		envelope := exportEnvelope{
			Version:    1,
			ExportedAt: now.Format(time.RFC3339),
			Rules:      exported,
		}

		filename := fmt.Sprintf("capacitarr-rules-%s.json", now.Format("2006-01-02"))
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

		bus.Publish(events.RulesExportedEvent{Count: len(exported)})

		return c.JSON(http.StatusOK, envelope)
	}
}

// handleImportRules imports custom rules from a portable JSON payload.
func handleImportRules(database *gorm.DB, bus *events.EventBus) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req importRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		}

		// Validate version
		if req.Payload.Version != 1 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Unsupported export version"})
		}

		// Validate required fields and effect values on each rule
		for i, r := range req.Payload.Rules {
			if r.Field == "" || r.Operator == "" || r.Value == "" || r.Effect == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"error": fmt.Sprintf("Rule at index %d is missing required fields (field, operator, value, effect)", i),
				})
			}
			if !validEffects[r.Effect] {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"error": fmt.Sprintf("Rule at index %d has invalid effect %q", i, r.Effect),
				})
			}
		}

		// Resolve integration IDs for each rule
		type resolvedRule struct {
			rule          importRulePayload
			integrationID *uint
		}
		resolved := make([]resolvedRule, 0, len(req.Payload.Rules))
		unmapped := make([]string, 0)

		// Cache for auto-matched integrations to avoid repeated queries
		autoMatchCache := make(map[string]*uint)

		for _, r := range req.Payload.Rules {
			// Rule has no integration reference
			if (r.IntegrationName == nil || *r.IntegrationName == "") &&
				(r.IntegrationType == nil || *r.IntegrationType == "") {
				resolved = append(resolved, resolvedRule{rule: r, integrationID: nil})
				continue
			}

			intName := ""
			intType := ""
			if r.IntegrationName != nil {
				intName = *r.IntegrationName
			}
			if r.IntegrationType != nil {
				intType = *r.IntegrationType
			}
			lookupKey := intType + ":" + intName

			// Check explicit mapping first
			if req.IntegrationMapping != nil {
				if mappedID, ok := req.IntegrationMapping[lookupKey]; ok {
					id := mappedID
					resolved = append(resolved, resolvedRule{rule: r, integrationID: &id})
					continue
				}
			}

			// Auto-match by type and name
			if cachedID, ok := autoMatchCache[lookupKey]; ok {
				if cachedID != nil {
					resolved = append(resolved, resolvedRule{rule: r, integrationID: cachedID})
				} else {
					unmapped = append(unmapped, lookupKey)
				}
				continue
			}

			var ic db.IntegrationConfig
			err := database.Where("type = ? AND name = ?", intType, intName).First(&ic).Error
			if err != nil {
				autoMatchCache[lookupKey] = nil
				unmapped = append(unmapped, lookupKey)
				continue
			}
			id := ic.ID
			autoMatchCache[lookupKey] = &id
			resolved = append(resolved, resolvedRule{rule: r, integrationID: &id})
		}

		// De-duplicate unmapped entries
		unmapped = uniqueStrings(unmapped)

		if len(unmapped) > 0 {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":    "unmapped integrations",
				"unmapped": unmapped,
			})
		}

		// Begin transaction for both the max sort_order query and inserts
		// to prevent concurrent imports from producing overlapping sort orders.
		tx := database.Begin()
		if tx.Error != nil {
			slog.Error("Failed to begin transaction", "component", "api", "operation", "import_rules", "error", tx.Error)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to import rules"})
		}

		// Determine the starting sort_order (after current max)
		var maxOrder int
		row := tx.Model(&db.CustomRule{}).Select("COALESCE(MAX(sort_order), -1)").Row()
		if err := row.Scan(&maxOrder); err != nil {
			tx.Rollback()
			slog.Error("Failed to query max sort_order", "component", "api", "operation", "import_rules", "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to determine rule ordering"})
		}
		nextOrder := maxOrder + 1

		// Insert all resolved rules
		for _, rr := range resolved {
			newRule := db.CustomRule{
				IntegrationID: rr.integrationID,
				Field:         rr.rule.Field,
				Operator:      rr.rule.Operator,
				Value:         rr.rule.Value,
				Effect:        rr.rule.Effect,
				Enabled:       true, // Create with default; fix below if disabled
				SortOrder:     nextOrder,
			}
			if err := tx.Create(&newRule).Error; err != nil {
				tx.Rollback()
				slog.Error("Failed to insert imported rule", "component", "api", "operation", "import_rules", "error", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to import rules"})
			}
			// GORM's default:true tag ignores false (the Go bool zero value)
			// on Create, so explicitly set disabled rules after insertion.
			if !rr.rule.Enabled {
				if err := tx.Model(&newRule).Update("enabled", false).Error; err != nil {
					tx.Rollback()
					slog.Error("Failed to disable imported rule", "component", "api", "operation", "import_rules", "error", err)
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to import rules"})
				}
			}
			nextOrder++
		}

		if err := tx.Commit().Error; err != nil {
			slog.Error("Failed to commit imported rules", "component", "api", "operation", "import_rules", "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to import rules"})
		}

		slog.Info("Imported custom rules", "component", "api", "operation", "import_rules", "count", len(resolved))

		bus.Publish(events.RulesImportedEvent{Count: len(resolved)})

		return c.JSON(http.StatusOK, map[string]interface{}{
			"imported": len(resolved),
			"skipped":  0,
		})
	}
}

// uniqueStrings removes duplicate strings while preserving order.
func uniqueStrings(input []string) []string {
	seen := make(map[string]bool, len(input))
	result := make([]string, 0, len(input))
	for _, s := range input {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}
