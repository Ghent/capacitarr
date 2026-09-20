package routes

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/db"
	"capacitarr/internal/services"
)

// Notification channel type constants.
const (
	notifTypeDiscord = "discord"
	notifTypeApprise = "apprise"
)

// RegisterNotificationRoutes sets up CRUD endpoints for notification channels.
func RegisterNotificationRoutes(g *echo.Group, reg *services.Registry) {
	// --- Notification Channel CRUD ---

	// GET /api/v1/notifications/channels — list all notification configs
	g.GET("/notifications/channels", func(c echo.Context) error {
		configs, err := reg.NotificationChannel.List()
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to fetch notification channels")
		}
		for i := range configs {
			configs[i].WebhookURL = db.MaskAPIKey(configs[i].WebhookURL)
		}
		return c.JSON(http.StatusOK, configs)
	})

	// POST /api/v1/notifications/channels — create a new channel
	g.POST("/notifications/channels", func(c echo.Context) error {
		var req db.NotificationConfig
		if err := c.Bind(&req); err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid request body")
		}

		// Validate required fields
		if req.Type == "" || req.Name == "" {
			return apiError(c, http.StatusBadRequest, "type and name are required")
		}
		if !db.ValidNotificationChannelTypes[req.Type] {
			return apiError(c, http.StatusBadRequest, "type must be one of: "+db.FormatValidKeys(db.ValidNotificationChannelTypes))
		}
		if (req.Type == notifTypeDiscord || req.Type == notifTypeApprise) && req.WebhookURL == "" {
			return apiError(c, http.StatusBadRequest, "webhookUrl is required for discord and apprise channels")
		}

		// Validate webhook URL scheme (must be http or https to prevent SSRF via exotic schemes)
		if req.WebhookURL != "" {
			parsedURL, err := url.Parse(req.WebhookURL)
			if err != nil || (parsedURL.Scheme != schemeHTTP && parsedURL.Scheme != schemeHTTPS) || parsedURL.Host == "" {
				return apiError(c, http.StatusBadRequest, "webhookUrl must be a valid HTTP or HTTPS URL")
			}
		}

		req.ID = 0 // ensure auto-increment

		created, createErr := reg.NotificationChannel.Create(req)
		if createErr != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to create notification channel")
		}

		created.WebhookURL = db.MaskAPIKey(created.WebhookURL)
		return c.JSON(http.StatusCreated, created)
	})

	// PUT /api/v1/notifications/channels/:id — update a channel
	g.PUT("/notifications/channels/:id", func(c echo.Context) error {
		id := c.Param("id")

		idNum, convErr := strconv.ParseUint(id, 10, 64)
		if convErr != nil {
			return apiError(c, http.StatusBadRequest, "Invalid ID")
		}

		var req db.NotificationConfig
		if err := c.Bind(&req); err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid request body")
		}

		// Validate type if changed
		if req.Type != "" && !db.ValidNotificationChannelTypes[req.Type] {
			return apiError(c, http.StatusBadRequest, "type must be one of: "+db.FormatValidKeys(db.ValidNotificationChannelTypes))
		}

		// Validate webhook URL scheme (must be http or https to prevent SSRF via exotic schemes).
		// Masked URLs are ignored so a round-trip save cannot clobber the stored secret.
		if req.WebhookURL != "" && !db.IsMaskedKey(req.WebhookURL) {
			parsedURL, err := url.Parse(req.WebhookURL)
			if err != nil || (parsedURL.Scheme != schemeHTTP && parsedURL.Scheme != schemeHTTPS) || parsedURL.Host == "" {
				return apiError(c, http.StatusBadRequest, "webhookUrl must be a valid HTTP or HTTPS URL")
			}
		}

		updated, updateErr := reg.NotificationChannel.PartialUpdate(uint(idNum), req)
		if updateErr != nil {
			if errors.Is(updateErr, services.ErrNotFound) {
				return apiError(c, http.StatusNotFound, "Notification channel not found")
			}
			return apiError(c, http.StatusInternalServerError, "Failed to update notification channel")
		}

		updated.WebhookURL = db.MaskAPIKey(updated.WebhookURL)
		return c.JSON(http.StatusOK, updated)
	})

	// DELETE /api/v1/notifications/channels/:id — delete a channel
	g.DELETE("/notifications/channels/:id", func(c echo.Context) error {
		id := c.Param("id")

		idNum, convErr := strconv.ParseUint(id, 10, 64)
		if convErr != nil {
			return apiError(c, http.StatusBadRequest, "Invalid ID")
		}

		if deleteErr := reg.NotificationChannel.Delete(uint(idNum)); deleteErr != nil {
			return apiError(c, http.StatusNotFound, "Notification channel not found")
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
	})

	// POST /api/v1/notifications/channels/:id/test — send a test notification
	g.POST("/notifications/channels/:id/test", func(c echo.Context) error {
		id := c.Param("id")

		idNum, convErr := strconv.ParseUint(id, 10, 64)
		if convErr != nil {
			return apiError(c, http.StatusBadRequest, "Invalid ID")
		}

		if err := reg.NotificationDispatch.TestChannel(uint(idNum)); err != nil {
			if errors.Is(err, services.ErrNotFound) {
				return apiError(c, http.StatusNotFound, "Notification channel not found")
			}
			return apiError(c, http.StatusBadGateway, "Test notification failed: "+err.Error())
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "sent"})
	})
}
