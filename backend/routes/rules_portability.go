package routes

import (
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/db"
	"capacitarr/internal/services"
)

// importEnvelope wraps the rules array with version metadata.
type importEnvelope struct {
	Version    int                   `json:"version"`
	ExportedAt string                `json:"exportedAt"`
	Rules      []services.ImportRule `json:"rules"`
}

// importRequest is the top-level request body for POST /custom-rules/import.
type importRequest struct {
	Payload            importEnvelope  `json:"payload"`
	IntegrationMapping map[string]uint `json:"integrationMapping"`
}

// RegisterRulePortabilityRoutes sets up the export/import endpoints for custom rules.
func RegisterRulePortabilityRoutes(protected *echo.Group, reg *services.Registry) {
	protected.GET("/custom-rules/export", func(c echo.Context) error {
		envelope, err := reg.Rules.Export()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch rules"})
		}

		now := time.Now().UTC()
		filename := fmt.Sprintf("capacitarr-rules-%s.json", now.Format("2006-01-02"))
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

		return c.JSON(http.StatusOK, envelope)
	})

	protected.POST("/custom-rules/import", func(c echo.Context) error {
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
			if !db.ValidEffects[r.Effect] {
				return c.JSON(http.StatusBadRequest, map[string]string{
					"error": fmt.Sprintf("Rule at index %d has invalid effect %q", i, r.Effect),
				})
			}
		}

		imported, unmapped, err := reg.Rules.Import(req.Payload.Rules, req.IntegrationMapping)
		if err != nil {
			if len(unmapped) > 0 {
				return c.JSON(http.StatusBadRequest, map[string]any{
					"error":    "unmapped integrations",
					"unmapped": unmapped,
				})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to import rules"})
		}

		return c.JSON(http.StatusOK, map[string]any{
			"imported": imported,
			"skipped":  0,
		})
	})
}
