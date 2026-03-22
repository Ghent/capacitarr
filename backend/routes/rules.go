package routes

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/db"
	"capacitarr/internal/services"
)

// RegisterRuleRoutes sets up the endpoints for managing custom rules, preferences,
// and score preview.
func RegisterRuleRoutes(protected *echo.Group, reg *services.Registry) {
	// Delegate preference, preview, and scoring factor weight routes to their own files
	RegisterPreferenceRoutes(protected, reg)
	RegisterPreviewRoutes(protected, reg)
	RegisterFactorWeightRoutes(protected, reg)

	// Delegate rule-field and rule-value routes to rulefields.go
	RegisterRuleFieldRoutes(protected, reg)

	// Rule impact preview
	protected.GET("/custom-rules/:id/impact", func(c echo.Context) error {
		id, err := strconv.ParseUint(c.Param("id"), 10, 32)
		if err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid rule ID")
		}
		impact, err := reg.Rules.GetRuleImpact(uint(id))
		if err != nil {
			return apiError(c, http.StatusNotFound, err.Error())
		}
		return c.JSON(http.StatusOK, impact)
	})

	// Rule context — combined data for the rule editor (fields + values + rule)
	protected.GET("/custom-rules/:id/context", func(c echo.Context) error {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid ID")
		}

		ctx, err := reg.Rules.GetRuleContext(uint(id))
		if err != nil {
			if errors.Is(err, services.ErrRuleNotFound) {
				return apiError(c, http.StatusNotFound, "Rule not found")
			}
			slog.Error("Failed to get rule context", "component", "api", "id", id, "error", err)
			return apiError(c, http.StatusInternalServerError, "Failed to get rule context")
		}
		return c.JSON(http.StatusOK, ctx)
	})

	// ---------------------------------------------------------
	// CUSTOM RULES (protection/targeting)
	// ---------------------------------------------------------
	protected.GET("/custom-rules", func(c echo.Context) error {
		rules, err := reg.Rules.List()
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to fetch custom rules")
		}
		return c.JSON(http.StatusOK, rules)
	})

	protected.PUT("/custom-rules/reorder", func(c echo.Context) error {
		var payload struct {
			Order []uint `json:"order"`
		}
		if err := c.Bind(&payload); err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid request payload")
		}
		if len(payload.Order) == 0 {
			return apiError(c, http.StatusBadRequest, "Order array must not be empty")
		}

		if err := reg.Rules.Reorder(payload.Order); err != nil {
			slog.Error("Failed to reorder rules", "component", "api", "error", err)
			return apiError(c, http.StatusInternalServerError, "Failed to reorder rules")
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	protected.PUT("/custom-rules/:id", func(c echo.Context) error {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid ID")
		}

		var updated db.CustomRule
		if err := c.Bind(&updated); err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid request payload")
		}

		rule, err := reg.Rules.Update(uint(id), updated)
		if err != nil {
			if errors.Is(err, services.ErrRuleNotFound) {
				return apiError(c, http.StatusNotFound, "Rule not found")
			}
			if errors.Is(err, services.ErrRuleValidation) {
				return apiError(c, http.StatusBadRequest, err.Error())
			}
			slog.Error("Failed to update custom rule", "component", "api", "operation", "update_rule", "id", id, "error", err)
			return apiError(c, http.StatusInternalServerError, "Failed to update rule")
		}
		return c.JSON(http.StatusOK, rule)
	})

	protected.POST("/custom-rules", func(c echo.Context) error {
		var newRule db.CustomRule
		if err := c.Bind(&newRule); err != nil {
			slog.Debug("Failed to bind rule payload", "component", "api", "operation", "create_rule", "error", err)
			return apiError(c, http.StatusBadRequest, "Invalid request payload: "+err.Error())
		}

		rule, err := reg.Rules.Create(newRule)
		if err != nil {
			if errors.Is(err, services.ErrRuleValidation) {
				return apiError(c, http.StatusBadRequest, err.Error())
			}
			slog.Error("Failed to create custom rule", "component", "api", "operation", "create_rule", "error", err)
			return apiError(c, http.StatusInternalServerError, "Failed to create rule")
		}
		return c.JSON(http.StatusCreated, rule)
	})

	protected.DELETE("/custom-rules/:id", func(c echo.Context) error {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid ID")
		}

		if err := reg.Rules.Delete(uint(id)); err != nil {
			if errors.Is(err, services.ErrRuleNotFound) {
				return apiError(c, http.StatusNotFound, "Rule not found")
			}
			slog.Error("Failed to delete custom rule", "component", "api", "operation", "delete_rule", "id", id, "error", err)
			return apiError(c, http.StatusInternalServerError, "Failed to delete rule")
		}
		return c.NoContent(http.StatusNoContent)
	})
}
