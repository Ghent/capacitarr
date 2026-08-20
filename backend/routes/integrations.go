package routes

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/db"
	"capacitarr/internal/services"
)

// RegisterIntegrationRoutes adds integration management endpoints
func RegisterIntegrationRoutes(g *echo.Group, reg *services.Registry) {
	// List all integrations
	g.GET("/integrations", func(c echo.Context) error {
		configs, err := reg.Integration.ListWithOverrideState()
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to fetch integrations")
		}

		// Mask API keys in response
		for i := range configs {
			configs[i].APIKey = db.MaskAPIKey(configs[i].APIKey)
		}

		return c.JSON(http.StatusOK, configs)
	})

	// Get single integration
	g.GET("/integrations/:id", func(c echo.Context) error {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid ID")
		}

		config, err := reg.Integration.GetWithOverrideState(uint(id))
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				return apiError(c, http.StatusNotFound, "Integration not found")
			}
			return apiError(c, http.StatusInternalServerError, "Failed to fetch integration")
		}

		// Mask API key
		config.APIKey = db.MaskAPIKey(config.APIKey)

		return c.JSON(http.StatusOK, config)
	})

	// Create integration
	g.POST("/integrations", func(c echo.Context) error {
		var config db.IntegrationConfig
		if err := c.Bind(&config); err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid request body")
		}

		// Validate required fields
		if config.Type == "" || config.Name == "" || config.URL == "" || config.APIKey == "" {
			return apiError(c, http.StatusBadRequest, "type, name, url, and apiKey are required")
		}

		// Validate URL scheme (must be http or https to prevent SSRF via exotic schemes)
		parsedURL, err := url.Parse(config.URL)
		if err != nil || (parsedURL.Scheme != schemeHTTP && parsedURL.Scheme != schemeHTTPS) || parsedURL.Host == "" {
			return apiError(c, http.StatusBadRequest, "url must be a valid HTTP or HTTPS URL")
		}

		// Validate type
		if !db.ValidIntegrationTypes[config.Type] {
			return apiError(c, http.StatusBadRequest, "type must be one of: "+db.FormatValidKeys(db.ValidIntegrationTypes))
		}

		config.ID = 0 // Ensure auto-increment
		config.Enabled = true
		created, createErr := reg.Integration.Create(config)
		if createErr != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to create integration")
		}

		// Return enriched response with override state (always false for new integrations
		// since they are not yet linked to any disk group).
		resp := services.IntegrationResponse{IntegrationConfig: *created}
		resp.APIKey = db.MaskAPIKey(resp.APIKey)
		return c.JSON(http.StatusCreated, resp)
	})

	// Update integration
	g.PUT("/integrations/:id", func(c echo.Context) error {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid ID")
		}

		var update services.IntegrationUpdate
		if err := c.Bind(&update); err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid request body")
		}

		// Validate URL scheme on update if URL is provided
		if update.URL != "" {
			parsedURL, urlErr := url.Parse(update.URL)
			if urlErr != nil || (parsedURL.Scheme != schemeHTTP && parsedURL.Scheme != schemeHTTPS) || parsedURL.Host == "" {
				return apiError(c, http.StatusBadRequest, "url must be a valid HTTP or HTTPS URL")
			}
		}

		updated, updateErr := reg.Integration.PartialUpdate(uint(id), update)
		if updateErr != nil {
			if errors.Is(updateErr, services.ErrNotFound) {
				return apiError(c, http.StatusNotFound, "Integration not found")
			}
			return apiError(c, http.StatusInternalServerError, "Failed to update integration")
		}

		// Return enriched response with override state.
		resp, respErr := reg.Integration.GetWithOverrideState(updated.ID)
		if respErr != nil {
			// Fallback: return the update result without override metadata.
			updated.APIKey = db.MaskAPIKey(updated.APIKey)
			return c.JSON(http.StatusOK, updated)
		}
		resp.APIKey = db.MaskAPIKey(resp.APIKey)
		return c.JSON(http.StatusOK, resp)
	})

	// Delete integration
	g.DELETE("/integrations/:id", func(c echo.Context) error {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || id == 0 {
			return apiError(c, http.StatusBadRequest, "Invalid ID")
		}

		if deleteErr := reg.Integration.Delete(uint(id)); deleteErr != nil {
			return apiError(c, http.StatusNotFound, "Integration not found")
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
	})

	// Test connection — delegates to IntegrationService.TestConnection()
	// Rate-limited: 30 attempts per IP per 5-minute window to prevent abuse of outbound connections
	integrationTestRL := newIPRateLimiter(30, 5*time.Minute)
	g.POST("/integrations/test", func(c echo.Context) error {
		var req struct {
			Type          string `json:"type"`
			URL           string `json:"url"`
			APIKey        string `json:"apiKey"`
			IntegrationID *int   `json:"integrationId,omitempty"`
		}
		if err := c.Bind(&req); err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid request body")
		}

		if req.Type == "" || req.URL == "" || req.APIKey == "" {
			return apiError(c, http.StatusBadRequest, "type, url, and apiKey are required")
		}

		parsedURL, parseErr := url.Parse(req.URL)
		if parseErr != nil || (parsedURL.Scheme != schemeHTTP && parsedURL.Scheme != schemeHTTPS) || parsedURL.Host == "" {
			return apiError(c, http.StatusBadRequest, "url must be a valid HTTP or HTTPS URL")
		}

		result := reg.Integration.TestConnection(req.Type, req.URL, req.APIKey, req.IntegrationID)
		return c.JSON(http.StatusOK, result)
	}, IPRateLimit(integrationTestRL))

	// Integration health / recovery status
	g.GET("/integrations/health", func(c echo.Context) error {
		entries := reg.Health.HealthStatus()
		return c.JSON(http.StatusOK, entries)
	})

	// Sync all integrations (trigger a manual poll)
	g.POST("/integrations/sync", func(c echo.Context) error {
		// Invalidate all cached rule values before re-syncing
		reg.Integration.InvalidateAllRuleValueCaches()

		// Delegate to IntegrationService.SyncAll() which handles connection
		// testing, disk space discovery, media item counting, and disk group
		// upserts via SettingsService.
		results, err := reg.Integration.SyncAll()
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to sync integrations")
		}

		return c.JSON(http.StatusOK, map[string]any{
			"results": results,
		})
	})
}
