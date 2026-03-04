package routes

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"capacitarr/internal/db"
)

// RegisterRuleRoutes sets up the endpoints for managing custom rules, preferences,
// and score preview.
func RegisterRuleRoutes(protected *echo.Group, database *gorm.DB) {
	// Delegate preference and preview routes to their own files
	RegisterPreferenceRoutes(protected, database)
	RegisterPreviewRoutes(protected, database)

	// Delegate rule-field and rule-value routes to rulefields.go
	registerRuleFieldRoutes(protected, database)

	// ---------------------------------------------------------
	// CUSTOM RULES (protection/targeting)
	// ---------------------------------------------------------
	protected.GET("/custom-rules", func(c echo.Context) error {
		rules := make([]db.CustomRule, 0)
		if err := database.Order("sort_order ASC, id ASC").Find(&rules).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch custom rules"})
		}
		return c.JSON(http.StatusOK, rules)
	})

	protected.PUT("/custom-rules/reorder", func(c echo.Context) error {
		var payload struct {
			Order []uint `json:"order"`
		}
		if err := c.Bind(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
		}
		if len(payload.Order) == 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Order array must not be empty"})
		}

		tx := database.Begin()
		for idx, ruleID := range payload.Order {
			if err := tx.Model(&db.CustomRule{}).Where("id = ?", ruleID).Update("sort_order", idx).Error; err != nil {
				tx.Rollback()
				slog.Error("Failed to update rule sort order", "component", "api", "ruleId", ruleID, "error", err)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to reorder rules"})
			}
		}
		tx.Commit()
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	protected.PUT("/custom-rules/:id", func(c echo.Context) error {
		id := c.Param("id")
		var existing db.CustomRule
		if err := database.First(&existing, id).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Rule not found"})
		}

		var updated db.CustomRule
		if err := c.Bind(&updated); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
		}

		// Preserve the ID from URL param
		updated.ID = existing.ID
		if err := database.Save(&updated).Error; err != nil {
			slog.Error("Failed to update custom rule", "component", "api", "operation", "update_rule", "id", id, "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update rule"})
		}
		return c.JSON(http.StatusOK, updated)
	})

	protected.POST("/custom-rules", func(c echo.Context) error {
		var newRule db.CustomRule
		if err := c.Bind(&newRule); err != nil {
			slog.Debug("Failed to bind rule payload", "component", "api", "operation", "create_rule", "error", err)
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request payload: " + err.Error()})
		}

		// Ensure new rules are enabled by default
		newRule.Enabled = true

		// Validate required fields for the new payload shape
		if newRule.Field == "" || newRule.Operator == "" || newRule.Value == "" {
			slog.Debug("Rule creation missing required fields", "component", "api", "field", newRule.Field, "operator", newRule.Operator, "value", newRule.Value, "effect", newRule.Effect)
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Field, Operator, and Value are required"})
		}

		// New payload: require effect field
		if newRule.Effect != "" { //nolint:gocritic // branches test different payload shapes
			validEffects := map[string]bool{
				"always_keep": true, "prefer_keep": true, "lean_keep": true,
				"lean_remove": true, "prefer_remove": true, "always_remove": true,
			}
			if !validEffects[newRule.Effect] {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "Effect must be one of: always_keep, prefer_keep, lean_keep, lean_remove, prefer_remove, always_remove"})
			}
		} else if newRule.Type != "" && newRule.Intensity != "" {
			// Legacy payload: type + intensity — auto-populate effect
			switch {
			case newRule.Type == "protect" && newRule.Intensity == "absolute":
				newRule.Effect = "always_keep"
			case newRule.Type == "protect" && newRule.Intensity == "strong":
				newRule.Effect = "prefer_keep"
			case newRule.Type == "protect":
				newRule.Effect = "lean_keep"
			case newRule.Type == "target" && newRule.Intensity == "absolute":
				newRule.Effect = "always_remove"
			case newRule.Type == "target" && newRule.Intensity == "strong":
				newRule.Effect = "prefer_remove"
			case newRule.Type == "target":
				newRule.Effect = "lean_remove"
			}
		} else {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Either 'effect' or both 'type' and 'intensity' are required"})
		}

		if err := database.Create(&newRule).Error; err != nil {
			slog.Error("Failed to create custom rule", "component", "api", "operation", "create_rule", "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create rule"})
		}
		return c.JSON(http.StatusCreated, newRule)
	})

	protected.DELETE("/custom-rules/:id", func(c echo.Context) error {
		id := c.Param("id")
		if err := database.Delete(&db.CustomRule{}, id).Error; err != nil {
			slog.Error("Failed to delete custom rule", "component", "api", "operation", "delete_rule", "id", id, "error", err)
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete rule"})
		}
		return c.NoContent(http.StatusNoContent)
	})
}
