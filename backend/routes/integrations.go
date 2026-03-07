package routes

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
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
			configs[i].APIKey = maskAPIKey(configs[i].APIKey)
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
		config.APIKey = maskAPIKey(config.APIKey)

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
		created.APIKey = maskAPIKey(created.APIKey)
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
			if urlErr != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "url must be a valid HTTP or HTTPS URL"})
			}
			existing.URL = update.URL
		}
		if update.APIKey != "" && !isMaskedKey(update.APIKey) {
			existing.APIKey = update.APIKey
		}
		existing.Enabled = update.Enabled

		updated, updateErr := reg.Integration.Update(existing.ID, *existing)
		if updateErr != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update integration"})
		}

		// Mask API key in response
		updated.APIKey = maskAPIKey(updated.APIKey)
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

		return c.JSON(http.StatusOK, map[string]string{"message": "Integration deleted"})
	})

	// Test connection
	g.POST("/integrations/test", func(c echo.Context) error {
		var req struct {
			Type          string `json:"type"`
			URL           string `json:"url"`
			APIKey        string `json:"apiKey"`
			IntegrationID *int   `json:"integrationId,omitempty"` // Optional: invalidate cache for this integration
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		}

		// If the API key is masked and we have an integration ID, look up the real key
		if (req.APIKey == "" || isMaskedKey(req.APIKey)) && req.IntegrationID != nil && *req.IntegrationID > 0 {
			existing, err := reg.Integration.GetByID(uint(*req.IntegrationID))
			if err == nil {
				req.APIKey = existing.APIKey
			}
		}

		// Tautulli and Overseerr don't implement the full Integration interface,
		// so handle their test connections separately
		if req.Type == intTypeTautulli {
			tautulli := integrations.NewTautulliClient(req.URL, req.APIKey)
			if err := tautulli.TestConnection(); err != nil {
				reg.Integration.PublishTestFailure(req.Type, req.Type, req.URL, err.Error())
				return c.JSON(http.StatusOK, map[string]interface{}{
					"success": false,
					"error":   err.Error(),
				})
			}
			reg.Integration.PublishTestSuccess(req.Type, req.Type, req.URL)
			return c.JSON(http.StatusOK, map[string]interface{}{
				"success": true,
				"message": "Connection successful",
			})
		}

		if req.Type == intTypeOverseerr {
			overseerr := integrations.NewOverseerrClient(req.URL, req.APIKey)
			if err := overseerr.TestConnection(); err != nil {
				reg.Integration.PublishTestFailure(req.Type, req.Type, req.URL, err.Error())
				return c.JSON(http.StatusOK, map[string]interface{}{
					"success": false,
					"error":   err.Error(),
				})
			}
			reg.Integration.PublishTestSuccess(req.Type, req.Type, req.URL)
			return c.JSON(http.StatusOK, map[string]interface{}{
				"success": true,
				"message": "Connection successful",
			})
		}

		client := integrations.NewClient(req.Type, req.URL, req.APIKey)
		if client == nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Unknown integration type"})
		}

		if err := client.TestConnection(); err != nil {
			reg.Integration.PublishTestFailure(req.Type, req.Type, req.URL, err.Error())
			return c.JSON(http.StatusOK, map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}

		// Invalidate rule value cache for this integration on successful test
		if req.IntegrationID != nil {
			reg.RuleValueCache.InvalidatePrefix(strconv.Itoa(*req.IntegrationID) + ":")
		}

		reg.Integration.PublishTestSuccess(req.Type, req.Type, req.URL)

		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "Connection successful",
		})
	})

	// Sync all integrations (trigger a manual poll)
	g.POST("/integrations/sync", func(c echo.Context) error {
		configs, err := reg.Integration.ListEnabled()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch integrations"})
		}

		// Invalidate all rule value caches on sync
		reg.RuleValueCache.InvalidateAll()

		results := make([]map[string]interface{}, 0)
		for _, cfg := range configs {
			client := integrations.NewClient(cfg.Type, cfg.URL, cfg.APIKey)
			if client == nil {
				continue
			}

			result := map[string]interface{}{
				"id":   cfg.ID,
				"name": cfg.Name,
				"type": cfg.Type,
			}

			// Test connection
			if err := client.TestConnection(); err != nil {
				result["status"] = "error"
				result["error"] = err.Error()
				results = append(results, result)
				continue
			}

			// Get disk space
			disks, err := client.GetDiskSpace()
			if err != nil {
				result["diskError"] = err.Error()
			} else {
				result["diskSpace"] = disks
				// Update disk groups via service
				for _, d := range disks {
					_, _ = reg.Settings.UpsertDiskGroup(d)
				}
			}

			// Get media items count
			items, err := client.GetMediaItems()
			if err != nil {
				result["mediaError"] = err.Error()
			} else {
				result["mediaCount"] = len(items)
			}

			result["status"] = "ok"
			results = append(results, result)
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"results": results,
		})
	})
}

// maskAPIKey returns a masked version of the key, showing only the last 4 characters.
func maskAPIKey(key string) string {
	if len(key) <= 4 {
		return "••••"
	}
	return strings.Repeat("•", len(key)-4) + key[len(key)-4:]
}

// isMaskedKey checks if an API key string is a masked version (starts with "•").
func isMaskedKey(key string) bool {
	return strings.HasPrefix(key, "•")
}
