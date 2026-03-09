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
			return apiError(c, http.StatusBadRequest, "type must be discord or apprise")
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
		req.CreatedAt = time.Now()
		req.UpdatedAt = time.Now()

		created, createErr := reg.NotificationChannel.Create(req)
		if createErr != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to create notification channel")
		}

		return c.JSON(http.StatusCreated, created)
	})

	// PUT /api/v1/notifications/channels/:id — update a channel
	g.PUT("/notifications/channels/:id", func(c echo.Context) error {
		id := c.Param("id")

		idNum, convErr := strconv.ParseUint(id, 10, 64)
		if convErr != nil {
			return apiError(c, http.StatusBadRequest, "Invalid ID")
		}

		existing, err := reg.NotificationChannel.GetByID(uint(idNum))
		if err != nil {
			return apiError(c, http.StatusNotFound, "Notification channel not found")
		}

		var req db.NotificationConfig
		if err := c.Bind(&req); err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid request body")
		}

		// Validate type if changed
		if req.Type != "" && !db.ValidNotificationChannelTypes[req.Type] {
			return apiError(c, http.StatusBadRequest, "type must be discord or apprise")
		}

		// Validate webhook URL scheme (must be http or https to prevent SSRF via exotic schemes)
		if req.WebhookURL != "" {
			parsedURL, err := url.Parse(req.WebhookURL)
			if err != nil || (parsedURL.Scheme != schemeHTTP && parsedURL.Scheme != schemeHTTPS) || parsedURL.Host == "" {
				return apiError(c, http.StatusBadRequest, "webhookUrl must be a valid HTTP or HTTPS URL")
			}
		}

		// Merge partial updates into the existing record
		if req.Name != "" {
			existing.Name = req.Name
		}
		if req.Type != "" {
			existing.Type = req.Type
		}
		existing.WebhookURL = req.WebhookURL
		existing.AppriseTags = req.AppriseTags
		existing.Enabled = req.Enabled
		existing.OnCycleDigest = req.OnCycleDigest
		existing.OnError = req.OnError
		existing.OnModeChanged = req.OnModeChanged
		existing.OnServerStarted = req.OnServerStarted
		existing.OnThresholdBreach = req.OnThresholdBreach
		existing.OnUpdateAvailable = req.OnUpdateAvailable
		existing.OnApprovalActivity = req.OnApprovalActivity
		existing.UpdatedAt = time.Now()

		updated, updateErr := reg.NotificationChannel.Update(existing.ID, *existing)
		if updateErr != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to update notification channel")
		}

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
