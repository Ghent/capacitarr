package routes

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
	"capacitarr/internal/services"
)

// RegisterRuleFieldRoutes sets up the /rule-fields and /rule-values endpoints.
// These are extracted from RegisterRuleRoutes for modularity.
func RegisterRuleFieldRoutes(protected *echo.Group, reg *services.Registry) {

	ruleValueCache := reg.RuleValueCache

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
			{"field": "timeinlibrary", "label": "Time in Library (days)", "type": "number", "operators": []string{">", ">=", "<", "<=", "in_last", "over_ago"}},
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
				configs, _ = reg.Integration.ListEnabled()
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
			configs, _ = reg.Integration.ListEnabled()
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
					map[string]interface{}{"field": "lastplayed", "label": "Last Watched", "type": "date", "operators": []string{"in_last", "over_ago", "never"}},
				)
			}
			if hasOverseerr {
				fields = append(fields,
					map[string]interface{}{"field": "requested", "label": "Is Requested", "type": "boolean", "operators": []string{"=="}},
					map[string]interface{}{"field": "requestcount", "label": "Request Count", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
					map[string]interface{}{"field": "requestedby", "label": "Requested By", "type": "string", "operators": []string{"==", "!=", "contains", "!contains"}},
				)
			}
			if hasMediaServer {
				fields = append(fields,
					map[string]interface{}{"field": "incollection", "label": "In Collection", "type": "boolean", "operators": []string{"=="}},
				)
			}
			if hasOverseerr && (hasTautulli || hasMediaServer) {
				fields = append(fields,
					map[string]interface{}{"field": "watchedbyreq", "label": "Watched by Requestor", "type": "boolean", "operators": []string{"=="}},
				)
			}
		} else {
			// service_type is specified — enrichment fields always available for *arr services
			arrTypes := map[string]bool{intTypeSonarr: true, intTypeRadarr: true, intTypeLidarr: true, intTypeReadarr: true}
			if arrTypes[serviceType] {
				var configs []db.IntegrationConfig
				configs, _ = reg.Integration.ListEnabled()
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
						map[string]interface{}{"field": "lastplayed", "label": "Last Watched", "type": "date", "operators": []string{"in_last", "over_ago", "never"}},
					)
				}
				if hasOverseerr {
					fields = append(fields,
						map[string]interface{}{"field": "requested", "label": "Is Requested", "type": "boolean", "operators": []string{"=="}},
						map[string]interface{}{"field": "requestcount", "label": "Request Count", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
						map[string]interface{}{"field": "requestedby", "label": "Requested By", "type": "string", "operators": []string{"==", "!=", "contains", "!contains"}},
					)
				}
				if hasMediaServer {
					fields = append(fields,
						map[string]interface{}{"field": "incollection", "label": "In Collection", "type": "boolean", "operators": []string{"=="}},
					)
				}
				if hasOverseerr && (hasTautulli || hasMediaServer) {
					fields = append(fields,
						map[string]interface{}{"field": "watchedbyreq", "label": "Watched by Requestor", "type": "boolean", "operators": []string{"=="}},
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

		integrationID, err := strconv.ParseUint(integrationIDStr, 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid integration_id"})
		}

		// Check cache first
		cacheKey := fmt.Sprintf("%d:%s", integrationID, action)
		if cached, ok := ruleValueCache.Get(cacheKey); ok {
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
			ruleValueCache.Set(cacheKey, result)
			return c.JSON(http.StatusOK, result)

		case "monitored", "requested", "incollection", "watchedbyreq": // Boolean fields
			result := map[string]interface{}{
				"type": "closed",
				"options": []integrations.NameValue{
					{Value: "true", Label: "Yes"},
					{Value: "false", Label: "No"},
				},
			}
			ruleValueCache.Set(cacheKey, result)
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
			ruleValueCache.Set(cacheKey, result)
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
		case "lastplayed":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "number", "placeholder": "e.g., 30", "suffix": "days",
			})
		case "requestedby":
			return c.JSON(http.StatusOK, map[string]interface{}{
				"type": "free", "inputType": "text", "placeholder": "e.g., john", "suffix": "",
			})
		}

		// Dynamic fields — require API call to the *arr service
		cfg, err := reg.Integration.GetByID(uint(integrationID))
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Integration not found"})
		}

		// Create the appropriate client and check if it implements RuleValueFetcher
		client := integrations.NewClient(cfg.Type, cfg.URL, cfg.APIKey)
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

		ruleValueCache.Set(cacheKey, result)
		return c.JSON(http.StatusOK, result)
	})
}
