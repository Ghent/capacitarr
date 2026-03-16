package routes

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/integrations"
	"capacitarr/internal/services"
)

// enrichmentPresence tracks which enrichment integration types are enabled.
type enrichmentPresence struct {
	hasTautulli  bool
	hasOverseerr bool
	hasMedia     bool
}

// detectEnrichment scans enabled integrations and returns which enrichment
// services are available (Tautulli, Overseerr, Plex/Jellyfin/Emby).
func detectEnrichment(reg *services.Registry) enrichmentPresence {
	configs, _ := reg.Integration.ListEnabled()
	var p enrichmentPresence
	for _, cfg := range configs {
		switch cfg.Type {
		case string(integrations.IntegrationTypeTautulli):
			p.hasTautulli = true
		case string(integrations.IntegrationTypeOverseerr):
			p.hasOverseerr = true
		case string(integrations.IntegrationTypePlex), string(integrations.IntegrationTypeJellyfin), string(integrations.IntegrationTypeEmby):
			p.hasMedia = true
		}
	}
	return p
}

// appendEnrichmentFields adds enrichment-dependent rule fields (play count,
// last watched, requested, in collection, watched by requestor) based on
// which enrichment integrations are enabled.
func appendEnrichmentFields(fields []map[string]any, p enrichmentPresence) []map[string]any {
	if p.hasTautulli || p.hasMedia {
		fields = append(fields,
			map[string]any{"field": "playcount", "label": "Play Count", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
			map[string]any{"field": "lastplayed", "label": "Last Watched", "type": "date", "operators": []string{"in_last", "over_ago", "never"}},
		)
	}
	if p.hasOverseerr {
		fields = append(fields,
			map[string]any{"field": "requested", "label": "Is Requested", "type": "boolean", "operators": []string{"=="}},
			map[string]any{"field": "requestcount", "label": "Request Count", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
			map[string]any{"field": "requestedby", "label": "Requested By", "type": "string", "operators": []string{"==", "!=", "contains", "!contains"}},
		)
	}
	if p.hasMedia {
		fields = append(fields,
			map[string]any{"field": "incollection", "label": "In Collection", "type": "boolean", "operators": []string{"=="}},
			map[string]any{"field": "watchlist", "label": "On Watchlist", "type": "boolean", "operators": []string{"=="}},
			map[string]any{"field": "collection", "label": "Collection Name", "type": "string", "operators": []string{"==", "!=", "contains", "!contains"}},
		)
	}
	if p.hasOverseerr && (p.hasTautulli || p.hasMedia) {
		fields = append(fields,
			map[string]any{"field": "watchedbyreq", "label": "Watched by Requestor", "type": "boolean", "operators": []string{"=="}},
		)
	}
	return fields
}

// RegisterRuleFieldRoutes sets up the /rule-fields and /rule-values endpoints.
// These are extracted from RegisterRuleRoutes for modularity.
func RegisterRuleFieldRoutes(protected *echo.Group, reg *services.Registry) {
	// ---------------------------------------------------------
	// RULE FIELD OPTIONS (dynamic based on integrations)
	// Accepts optional ?service_type=sonarr to filter fields.
	// Without the parameter, returns all fields (backward compat).
	// ---------------------------------------------------------
	protected.GET("/rule-fields", func(c echo.Context) error {
		serviceType := c.QueryParam("service_type")

		// Base fields available for all *arr integration types
		fields := []map[string]any{
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
		if serviceType == string(integrations.IntegrationTypeSonarr) || serviceType == "" {
			// Sonarr-specific fields (TV)
			sonarrFields := []map[string]any{
				{"field": "seriesstatus", "label": "Show Status", "type": "string", "operators": []string{"==", "!="}},
				{"field": "seasoncount", "label": "Season Count", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
				{"field": "episodecount", "label": "Episode Count", "type": "number", "operators": []string{"==", "!=", ">", ">=", "<", "<="}},
			}

			if serviceType == string(integrations.IntegrationTypeSonarr) {
				fields = append(fields, sonarrFields...)
			} else {
				// No service_type filter: conditionally add based on enabled integrations
				configs, _ := reg.Integration.ListEnabled()
				hasTV := false
				for _, cfg := range configs {
					if cfg.Type == string(integrations.IntegrationTypeSonarr) {
						hasTV = true
						break
					}
				}
				if hasTV {
					fields = append(fields, sonarrFields...)
				}
			}
		}

		// Enrichment fields from Tautulli / Overseerr / media servers.
		// For unfiltered requests (serviceType == ""), always check enrichment.
		// For filtered requests, only add enrichment for *arr service types.
		addEnrichment := serviceType == ""
		if !addEnrichment {
			arrTypes := map[string]bool{
				string(integrations.IntegrationTypeSonarr):  true,
				string(integrations.IntegrationTypeRadarr):  true,
				string(integrations.IntegrationTypeLidarr):  true,
				string(integrations.IntegrationTypeReadarr): true,
			}
			addEnrichment = arrTypes[serviceType]
		}
		if addEnrichment {
			fields = appendEnrichmentFields(fields, detectEnrichment(reg))
		}

		// Media Type field (always available)
		fields = append(fields,
			map[string]any{"field": "type", "label": "Media Type", "type": "string", "operators": []string{"==", "!="}},
		)

		return c.JSON(http.StatusOK, fields)
	})

	// ---------------------------------------------------------
	// RULE VALUES — Autocomplete for rule value input
	// GET /api/v1/rule-values?integration_id=X&action=Y
	// Delegates to IntegrationService.FetchRuleValues() which handles
	// caching, external API calls, and static field metadata.
	// ---------------------------------------------------------
	protected.GET("/rule-values", func(c echo.Context) error {
		integrationIDStr := c.QueryParam("integration_id")
		action := c.QueryParam("action")
		if integrationIDStr == "" || action == "" {
			return apiError(c, http.StatusBadRequest, "integration_id and action are required")
		}

		integrationID, err := strconv.ParseUint(integrationIDStr, 10, 64)
		if err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid integration_id")
		}

		result, fetchErr := reg.Integration.FetchRuleValues(uint(integrationID), action)
		if fetchErr != nil {
			switch {
			case errors.Is(fetchErr, services.ErrNotFound):
				return apiError(c, http.StatusNotFound, "Integration not found")
			case errors.Is(fetchErr, services.ErrUnsupportedIntegrationType),
				errors.Is(fetchErr, services.ErrIntegrationNoRuleValues):
				return apiError(c, http.StatusBadRequest, fetchErr.Error())
			case errors.Is(fetchErr, services.ErrUnknownAction):
				return apiError(c, http.StatusBadRequest, fetchErr.Error())
			default:
				slog.Warn("Failed to fetch rule values", "component", "api", "integrationId", integrationID, "action", action, "error", fetchErr)
				return apiError(c, http.StatusInternalServerError, "Failed to fetch rule values")
			}
		}

		return c.JSON(http.StatusOK, result)
	})
}
