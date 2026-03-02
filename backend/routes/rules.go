package routes

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"capacitarr/internal/cache"
	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)

// RuleValueCache is the package-level TTL cache for rule value lookups.
// Exported so that integration test/sync endpoints can invalidate it.
var RuleValueCache = cache.New(5 * time.Minute)

// RegisterRuleRoutes sets up the endpoints for managing custom rules, preferences,
// and score preview.
func RegisterRuleRoutes(protected *echo.Group, database *gorm.DB) {
	// Delegate preference and preview routes to their own files
	RegisterPreferenceRoutes(protected, database)
	RegisterPreviewRoutes(protected, database)

	// ---------------------------------------------------------
	// RULE FIELD OPTIONS (dynamic based on integrations)
	// Accepts optional ?service_type=sonarr to filter fields.
	// Without the parameter, returns all fields (backward compat).
	// ---------------------------------------------------------
	protected.GET("/rule-fields", func(c echo.Context) error {
		serviceType := c.QueryParam("service_type")

		// Base fields available for all *arr integration types
		fields := []map[string]interface{}{
			{"field": "title", "label": "Title", "type": "string", "operators": []string{"==", "!=", "contains", "!contains"}},
			{"field": "quality", "label": "Quality Profile", "type": "string", "operators": []string{"==", "!="}},
			{"field": "tag", "label": "Tags", "type": "string", "operators": []string{"contains", "!contains"}},
			{"field": "genre", "label": "Genre", "type": "string", "operators": []string{"==", "!=", "contains", "!contains"}},
			{"field": "rating", "label": "Rating", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
			{"field": "sizebytes", "label": "Size (bytes)", "type": "number", "operators": []string{">", ">=", "<", "<="}},
			{"field": "timeinlibrary", "label": "Time in Library (days)", "type": "number", "operators": []string{">", ">=", "<", "<="}},
			{"field": "monitored", "label": "Monitored", "type": "boolean", "operators": []string{"=="}},
			{"field": "year", "label": "Year", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
			{"field": "language", "label": "Language", "type": "string", "operators": []string{"==", "!="}},
		}

		// When service_type is specified, add type-specific fields
		if serviceType == intTypeSonarr || serviceType == "" {
			// Sonarr-specific fields (TV)
			sonarrFields := []map[string]interface{}{
				{"field": "seriesstatus", "label": "Show Status", "type": "string", "operators": []string{"==", "!="}},
				{"field": "seasoncount", "label": "Season Count", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
				{"field": "episodecount", "label": "Episode Count", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
			}

			if serviceType == intTypeSonarr {
				fields = append(fields, sonarrFields...)
			} else {
				// No service_type filter: conditionally add based on enabled integrations
				var configs []db.IntegrationConfig
				database.Where("enabled = ?", true).Find(&configs)
				hasTV := false
				for _, cfg := range configs {
					if cfg.Type == intTypeSonarr {
						hasTV = true
						break
					}
				}
				if hasTV {
					fields = append(fields, sonarrFields...)
				}
			}
		}

		// Enrichment fields from Tautulli / Overseerr / media servers
		// These apply to all *arr services when the enrichment service is enabled
		if serviceType == "" {
			// No filter: check which enrichment services are enabled
			var configs []db.IntegrationConfig
			database.Where("enabled = ?", true).Find(&configs)
			hasTautulli := false
			hasOverseerr := false
			hasMediaServer := false
			for _, cfg := range configs {
				switch cfg.Type {
				case intTypeTautulli:
					hasTautulli = true
				case intTypeOverseerr:
					hasOverseerr = true
				case intTypePlex, intTypeJellyfin, intTypeEmby:
					hasMediaServer = true
				}
			}
			if hasTautulli || hasMediaServer {
				fields = append(fields,
					map[string]interface{}{"field": "playcount", "label": "Play Count", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
				)
			}
			if hasOverseerr {
				fields = append(fields,
					map[string]interface{}{"field": "requested", "label": "Is Requested", "type": "boolean", "operators": []string{"=="}},
					map[string]interface{}{"field": "requestcount", "label": "Request Count", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
				)
			}
		} else {
			// service_type is specified — enrichment fields always available for *arr services
			arrTypes := map[string]bool{intTypeSonarr: true, intTypeRadarr: true, intTypeLidarr: true, intTypeReadarr: true}
			if arrTypes[serviceType] {
				var configs []db.IntegrationConfig
				database.Where("enabled = ?", true).Find(&configs)
				hasTautulli := false
				hasOverseerr := false
				hasMediaServer := false
				for _, cfg := range configs {
					switch cfg.Type {
					case intTypeTautulli:
						hasTautulli = true
					case intTypeOverseerr:
						hasOverseerr = true
					case intTypePlex, intTypeJellyfin, intTypeEmby:
						hasMediaServer = true
					}
				}
				if hasTautulli || hasMediaServer {
					fields = append(fields,
						map[string]interface{}{"field": "playcount", "label": "Play Count", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
					)
				}
				if hasOverseerr {
					fields = append(fields,
						map[string]interface{}{"field": "requested", "label": "Is Requested", "type": "boolean", "operators": []string{"=="}},
						map[string]interface{}{"field": "requestcount", "label": "Request Count", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
					)
				}
			}
		}

		// Media Type field (always available)
		fields = append(fields,
			map[string]interface{}{"field": "type", "label": "Media Type", "type": "string", "operators": []string{"==", "!="}},
		)

		return c.JSON(http.StatusOK, fields)
	})

	// ---------------------------------------------------------
	// RULE VALUES — Autocomplete for rule value input
	// GET /api/v1/rule-values?integration_id=X&action=Y
	// Returns value options based on integration + action type.
	// ---------------------------------------------------------
	protected.GET("/rule-values", func(c echo.Context) error {
		integrationIDStr := c.QueryParam("integration_id")
		action := c.QueryParam("action")
		if integrationIDStr == "" || action == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "integration_id and action are required"})
		}

		integrationID, err := strconv.Atoi(integrationIDStr)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid integration_id"})
		}

		// Check cache first
		cacheKey := fmt.Sprintf("%d:%s", integrationID, action)
		if cached, ok := RuleValueCache.Get(cacheKey); ok {
			return c.JSON(http.StatusOK, cached)
		}

		// Handle static/built-in value types that don't need an API call
		switch action {
		case "seriesstatus": // Show Status
			result := map[string]interface{}{
				"type": "closed",
				"options": []integrations.NameValue{
					{Value: "continuing", Label: "Continuing"},
					{Value: "ended", Label: "Ended"},
					{Value: "upcoming", Label: "Upcoming"},
					{Value: "deleted", Label: "Deleted"},
				},
			}
			RuleValueCache.Set(cacheKey, result)
			return c.JSON(http.StatusOK, result)

		case "monitored", "requested": // Boolean fields
			result := map[string]interface{}{
				"type": "closed",
				"options": []integrations.NameValue{
					{Value: "true", Label: "Yes"},
					{Value: "false", Label: "No"},
				},
			}
			RuleValueCache.Set(cacheKey, result)
			return c.JSON(http.StatusOK, result)

		case "type": // Media Type
			result := map[string]interface{}{
				"type": "closed",
				"options": []integrations.NameValue{
					{Value: "movie", Label: "Movie"},
					{Value: "show", Label: "Show"},
					{Value: "season", Label: "Season"},
					{Value: "artist", Label: "Artist"},
					{Value: "book", Label: "Book"},
				},
			}
			RuleValueCache.Set(cacheKey, result)
			return c.JSON(http.StatusOK, result)

		// Free-text fields — return input metadata
		case "title":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "text", "placeholder": "e.g., Breaking Bad", "suffix": "",
			})
		case "rating":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "number", "placeholder": "e.g., 7.5", "suffix": "",
			})
		case "sizebytes":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "number", "placeholder": "e.g., 5368709120", "suffix": "bytes (≈ GB)",
			})
		case "timeinlibrary":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "number", "placeholder": "e.g., 30", "suffix": "days",
			})
		case "year":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "number", "placeholder": "e.g., 2020", "suffix": "",
			})
		case "seasoncount":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "number", "placeholder": "e.g., 5", "suffix": "",
			})
		case "episodecount":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "number", "placeholder": "e.g., 100", "suffix": "",
			})
		case "playcount":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "number", "placeholder": "e.g., 0", "suffix": "",
			})
		case "requestcount":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "number", "placeholder": "e.g., 3", "suffix": "",
			})
		}

		// Dynamic fields — require API call to the *arr service
		var cfg db.IntegrationConfig
		if err := database.First(&cfg, integrationID).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Integration not found"})
		}

		// Create the appropriate client and check if it implements RuleValueFetcher
		client := CreateClient(cfg.Type, cfg.URL, cfg.APIKey)
		if client == nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Unsupported integration type for rule values"})
		}

		fetcher, ok := client.(integrations.RuleValueFetcher)
		if !ok {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Integration does not support rule value lookups"})
		}

		var result map[string]interface{}

		switch action {
		case "quality":
			profiles, fetchErr := fetcher.GetQualityProfiles()
			if fetchErr != nil {
				slog.Warn("Failed to fetch quality profiles for rule values", "component", "api", "integrationId", integrationID, "error", fetchErr)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch quality profiles"})
			}
			result = map[string]interface{}{"type": "closed", "options": profiles}

		case "tag":
			tags, fetchErr := fetcher.GetTags()
			if fetchErr != nil {
				slog.Warn("Failed to fetch tags for rule values", "component", "api", "integrationId", integrationID, "error", fetchErr)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch tags"})
			}
			result = map[string]interface{}{"type": "combobox", "suggestions": tags}

		case "genre":
			// Genre suggestions are free-form combobox with no API source
			result = map[string]interface{}{
				"type": "combobox",
				"suggestions": []integrations.NameValue{
					{Value: "Action", Label: "Action"},
					{Value: "Adventure", Label: "Adventure"},
					{Value: "Animation", Label: "Animation"},
					{Value: "Comedy", Label: "Comedy"},
					{Value: "Crime", Label: "Crime"},
					{Value: "Documentary", Label: "Documentary"},
					{Value: "Drama", Label: "Drama"},
					{Value: "Fantasy", Label: "Fantasy"},
					{Value: "Horror", Label: "Horror"},
					{Value: "Mystery", Label: "Mystery"},
					{Value: "Romance", Label: "Romance"},
					{Value: "Sci-Fi", Label: "Sci-Fi"},
					{Value: "Thriller", Label: "Thriller"},
				},
			}

		case "language":
			langs, fetchErr := fetcher.GetLanguages()
			if fetchErr != nil {
				slog.Warn("Failed to fetch languages for rule values", "component", "api", "integrationId", integrationID, "error", fetchErr)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch languages"})
			}
			if langs == nil {
				// Service doesn't support language lookup — return free input
				return c.JSON(http.StatusOK, map[string]interface{}{
					"type": "free", "inputType": "text", "placeholder": "e.g., English", "suffix": "",
				})
			}
			result = map[string]interface{}{"type": "closed", "options": langs}

		default:
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Unknown action: " + action})
		}

		RuleValueCache.Set(cacheKey, result)
		return c.JSON(http.StatusOK, result)
	})

	// ---------------------------------------------------------
	// CUSTOM RULES (protection/targeting)
	// ---------------------------------------------------------
	protected.GET("/custom-rules", func(c echo.Context) error {
		var rules []db.CustomRule
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
