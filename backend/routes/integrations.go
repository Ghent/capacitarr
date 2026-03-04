package routes

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/integrations"
)



// RegisterIntegrationRoutes adds integration management endpoints
func RegisterIntegrationRoutes(g *echo.Group, database *gorm.DB) {
	// List all integrations
	g.GET("/integrations", func(c echo.Context) error {
		configs := make([]db.IntegrationConfig, 0)
		if err := database.Order("created_at asc").Find(&configs).Error; err != nil {
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
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID"})
		}

		var config db.IntegrationConfig
		if err := database.First(&config, id).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Integration not found"})
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
		validTypes := map[string]bool{
			intTypePlex: true, intTypeSonarr: true, intTypeRadarr: true, intTypeLidarr: true,
			intTypeReadarr: true, intTypeTautulli: true, intTypeOverseerr: true,
			intTypeJellyfin: true, intTypeEmby: true,
		}
		if !validTypes[config.Type] {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "type must be one of: plex, sonarr, radarr, lidarr, readarr, tautulli, overseerr, jellyfin, emby"})
		}

		config.ID = 0 // Ensure auto-increment
		config.Enabled = true
		if err := database.Create(&config).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create integration"})
		}

		// Mask API key in response
		config.APIKey = maskAPIKey(config.APIKey)
		return c.JSON(http.StatusCreated, config)
	})

	// Update integration
	g.PUT("/integrations/:id", func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID"})
		}

		var existing db.IntegrationConfig
		if err := database.First(&existing, id).Error; err != nil {
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

		if err := database.Save(&existing).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update integration"})
		}

		// Mask API key in response
		existing.APIKey = maskAPIKey(existing.APIKey)
		return c.JSON(http.StatusOK, existing)
	})

	// Delete integration
	g.DELETE("/integrations/:id", func(c echo.Context) error {
		id, err := strconv.Atoi(c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID"})
		}

		if err := database.Delete(&db.IntegrationConfig{}, id).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete integration"})
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
		if (req.APIKey == "" || isMaskedKey(req.APIKey)) && req.IntegrationID != nil {
			var existing db.IntegrationConfig
			if err := database.First(&existing, *req.IntegrationID).Error; err == nil {
				req.APIKey = existing.APIKey
			}
		}

		// Tautulli and Overseerr don't implement the full Integration interface,
		// so handle their test connections separately
		if req.Type == intTypeTautulli {
			tautulli := integrations.NewTautulliClient(req.URL, req.APIKey)
			if err := tautulli.TestConnection(); err != nil {
				return c.JSON(http.StatusOK, map[string]interface{}{
					"success": false,
					"error":   err.Error(),
				})
			}
			return c.JSON(http.StatusOK, map[string]interface{}{
				"success": true,
				"message": "Connection successful",
			})
		}

		if req.Type == intTypeOverseerr {
			overseerr := integrations.NewOverseerrClient(req.URL, req.APIKey)
			if err := overseerr.TestConnection(); err != nil {
				return c.JSON(http.StatusOK, map[string]interface{}{
					"success": false,
					"error":   err.Error(),
				})
			}
			return c.JSON(http.StatusOK, map[string]interface{}{
				"success": true,
				"message": "Connection successful",
			})
		}

		client := CreateClient(req.Type, req.URL, req.APIKey)
		if client == nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Unknown integration type"})
		}

		if err := client.TestConnection(); err != nil {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
		}

		// Invalidate rule value cache for this integration on successful test
		if req.IntegrationID != nil {
			RuleValueCache.InvalidatePrefix(strconv.Itoa(*req.IntegrationID) + ":")
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "Connection successful",
		})
	})

	// Sync all integrations (trigger a manual poll)
	g.POST("/integrations/sync", func(c echo.Context) error {
		var configs []db.IntegrationConfig
		if err := database.Where("enabled = ?", true).Find(&configs).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch integrations"})
		}

		// Invalidate all rule value caches on sync
		RuleValueCache.InvalidateAll()

		results := make([]map[string]interface{}, 0)
		for _, cfg := range configs {
			client := CreateClient(cfg.Type, cfg.URL, cfg.APIKey)
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
				// Update disk groups
				for _, d := range disks {
					updateDiskGroup(database, d)
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

// CreateClient creates the appropriate integration client based on type
func CreateClient(intType, url, apiKey string) integrations.Integration {
	switch intType {
	case intTypeSonarr:
		return integrations.NewSonarrClient(url, apiKey)
	case intTypeRadarr:
		return integrations.NewRadarrClient(url, apiKey)
	case intTypeLidarr:
		return integrations.NewLidarrClient(url, apiKey)
	case intTypeReadarr:
		return integrations.NewReadarrClient(url, apiKey)
	case intTypePlex:
		return integrations.NewPlexClient(url, apiKey)
	default:
		return nil
	}
}

// updateDiskGroup creates or updates a disk group from discovered disk space
func updateDiskGroup(database *gorm.DB, disk integrations.DiskSpace) {
	var group db.DiskGroup
	result := database.Where("mount_path = ?", disk.Path).First(&group)

	usedBytes := disk.TotalBytes - disk.FreeBytes

	if result.Error != nil {
		// Create new disk group
		group = db.DiskGroup{
			MountPath:  disk.Path,
			TotalBytes: disk.TotalBytes,
			UsedBytes:  usedBytes,
		}
		database.Create(&group)
	} else {
		// Update existing
		database.Model(&group).Updates(map[string]interface{}{
			"total_bytes": disk.TotalBytes,
			"used_bytes":  usedBytes,
		})
	}
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
