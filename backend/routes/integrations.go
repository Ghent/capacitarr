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
		configs, err := reg.Integration.List()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch integrations"})
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
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID"})
		}

		config, err := reg.Integration.GetByID(uint(id))
		if err != nil {
			if errors.Is(err, services.ErrNotFound) {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "Integration not found"})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch integration"})
		}

		// Mask API key
		config.APIKey = db.MaskAPIKey(config.APIKey)

		return c.JSON(http.StatusOK, config)
	})

	// Create integration
	g.POST("/integrations", func(c echo.Context) error {
		var config db.IntegrationConfig
		if err := c.Bind(&config); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		}

		// Validate required fields
		if config.Type == "" || config.Name == "" || config.URL == "" || config.APIKey == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "type, name, url, and apiKey are required"})
		}

		// Validate URL scheme (must be http or https to prevent SSRF via exotic schemes)
		parsedURL, err := url.Parse(config.URL)
		if err != nil || (parsedURL.Scheme != schemeHTTP && parsedURL.Scheme != schemeHTTPS) || parsedURL.Host == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "url must be a valid HTTP or HTTPS URL"})
		}

		// Validate type
		if !db.ValidIntegrationTypes[config.Type] {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "type must be one of: plex, sonarr, radarr, lidarr, readarr, tautulli, overseerr, jellyfin, emby"})
		}

		config.ID = 0 // Ensure auto-increment
		config.Enabled = true
		created, createErr := reg.Integration.Create(config)
		if createErr != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create integration"})
		}

		// Mask API key in response
		created.APIKey = db.MaskAPIKey(created.APIKey)
		return c.JSON(http.StatusCreated, created)
	})

	// Update integration
	g.PUT("/integrations/:id", func(c echo.Context) error {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID"})
		}

		existing, err := reg.Integration.GetByID(uint(id))
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Integration not found"})
		}

		var update db.IntegrationConfig
		if err := c.Bind(&update); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		}

		// Update fields
		if update.Name != "" {
			existing.Name = update.Name
		}
		if update.URL != "" {
			// Validate URL scheme on update as well
			parsedURL, urlErr := url.Parse(update.URL)
			if urlErr != nil || (parsedURL.Scheme != schemeHTTP && parsedURL.Scheme != schemeHTTPS) || parsedURL.Host == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "url must be a valid HTTP or HTTPS URL"})
			}
			existing.URL = update.URL
		}
		if update.APIKey != "" && !db.IsMaskedKey(update.APIKey) {
			existing.APIKey = update.APIKey
		}
		existing.Enabled = update.Enabled

		updated, updateErr := reg.Integration.Update(existing.ID, *existing)
		if updateErr != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update integration"})
		}

		// Mask API key in response
		updated.APIKey = db.MaskAPIKey(updated.APIKey)
		return c.JSON(http.StatusOK, updated)
	})

	// Delete integration
	g.DELETE("/integrations/:id", func(c echo.Context) error {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil || id == 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID"})
		}

		if deleteErr := reg.Integration.Delete(uint(id)); deleteErr != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Integration not found"})
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
	})

	// Test connection — delegates to IntegrationService.TestConnection()
	// Rate-limited: 30 attempts per IP per 5-minute window to prevent abuse of outbound connections
	integrationTestRL := newLoginRateLimiter(30, 5*time.Minute)
	g.POST("/integrations/test", func(c echo.Context) error {
		var req struct {
			Type          string `json:"type"`
			URL           string `json:"url"`
			APIKey        string `json:"apiKey"`
			IntegrationID *int   `json:"integrationId,omitempty"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		}

		result := reg.Integration.TestConnection(req.Type, req.URL, req.APIKey, req.IntegrationID)
		return c.JSON(http.StatusOK, result)
	}, LoginRateLimit(integrationTestRL))

	// Sync all integrations (trigger a manual poll)
	g.POST("/integrations/sync", func(c echo.Context) error {
		// Invalidate all cached rule values before re-syncing
		reg.Integration.InvalidateAllRuleValueCaches()

		// Delegate to IntegrationService.SyncAll() which handles connection
		// testing, disk space discovery, media item counting, and disk group
		// upserts via SettingsService.
		results, err := reg.Integration.SyncAll()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to sync integrations"})
		}

		return c.JSON(http.StatusOK, map[string]any{
			"results": results,
		})
	})
}
