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

		config, err := reg.Integration.GetByID(uint(id))
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
			return apiError(c, http.StatusBadRequest, "type must be one of: plex, sonarr, radarr, lidarr, readarr, tautulli, seerr, jellyfin, emby, jellystat")
		}

		config.ID = 0 // Ensure auto-increment
		config.Enabled = true
		created, createErr := reg.Integration.Create(config)
		if createErr != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to create integration")
		}

		// Mask API key in response
		created.APIKey = db.MaskAPIKey(created.APIKey)
		return c.JSON(http.StatusCreated, created)
	})

	// Update integration
	g.PUT("/integrations/:id", func(c echo.Context) error {
		id, err := strconv.ParseUint(c.Param("id"), 10, 64)
		if err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid ID")
		}

		existing, err := reg.Integration.GetByID(uint(id))
		if err != nil {
			return apiError(c, http.StatusNotFound, "Integration not found")
		}

		// Use a dedicated struct with pointer fields so we can distinguish
		// "field not sent" (nil) from "explicitly set to false/empty".
		// This prevents partial updates (e.g. toggling enabled) from
		// accidentally zeroing out other fields.
		var update struct {
			Name               string `json:"name"`
			URL                string `json:"url"`
			APIKey             string `json:"apiKey"`
			Enabled            *bool  `json:"enabled"`
			CollectionDeletion *bool  `json:"collectionDeletion"`
			ShowLevelOnly      *bool  `json:"showLevelOnly"`
		}
		if err := c.Bind(&update); err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid request body")
		}

		// Update fields only when explicitly provided
		if update.Name != "" {
			existing.Name = update.Name
		}
		if update.URL != "" {
			// Validate URL scheme on update as well
			parsedURL, urlErr := url.Parse(update.URL)
			if urlErr != nil || (parsedURL.Scheme != schemeHTTP && parsedURL.Scheme != schemeHTTPS) || parsedURL.Host == "" {
				return apiError(c, http.StatusBadRequest, "url must be a valid HTTP or HTTPS URL")
			}
			existing.URL = update.URL
		}
		if update.APIKey != "" && !db.IsMaskedKey(update.APIKey) {
			existing.APIKey = update.APIKey
		}
		if update.Enabled != nil {
			existing.Enabled = *update.Enabled
		}
		if update.CollectionDeletion != nil {
			existing.CollectionDeletion = *update.CollectionDeletion
		}
		if update.ShowLevelOnly != nil {
			existing.ShowLevelOnly = *update.ShowLevelOnly
		}

		// Clear stale sync status — configuration has changed, so the
		// previous error and sync time are no longer valid.
		existing.LastError = ""
		existing.LastSync = nil

		updated, updateErr := reg.Integration.Update(existing.ID, *existing)
		if updateErr != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to update integration")
		}

		// Mask API key in response
		updated.APIKey = db.MaskAPIKey(updated.APIKey)
		return c.JSON(http.StatusOK, updated)
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

		result := reg.Integration.TestConnection(req.Type, req.URL, req.APIKey, req.IntegrationID)
		return c.JSON(http.StatusOK, result)
	}, IPRateLimit(integrationTestRL))

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
