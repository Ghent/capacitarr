package routes

import (
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
	notifTypeSlack   = "slack"
	notifTypeInApp   = "inapp"
)

// RegisterNotificationRoutes sets up CRUD endpoints for notification channels
// and management endpoints for in-app notifications.
func RegisterNotificationRoutes(g *echo.Group, reg *services.Registry) {
	// --- Notification Channel CRUD ---

	// GET /api/v1/notifications/channels — list all notification configs
	g.GET("/notifications/channels", func(c echo.Context) error {
		configs, err := reg.NotificationChannel.List()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch notification channels"})
		}
		return c.JSON(http.StatusOK, configs)
	})

	// POST /api/v1/notifications/channels — create a new channel
	g.POST("/notifications/channels", func(c echo.Context) error {
		var req db.NotificationConfig
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		}

		// Validate required fields
		if req.Type == "" || req.Name == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "type and name are required"})
		}
		if !db.ValidNotificationChannelTypes[req.Type] {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "type must be discord, slack, or inapp"})
		}
		if (req.Type == notifTypeDiscord || req.Type == notifTypeSlack) && req.WebhookURL == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "webhookUrl is required for discord and slack channels"})
		}

		// Validate webhook URL scheme (must be http or https to prevent SSRF via exotic schemes)
		if req.WebhookURL != "" {
			parsedURL, err := url.Parse(req.WebhookURL)
			if err != nil || (parsedURL.Scheme != schemeHTTP && parsedURL.Scheme != schemeHTTPS) || parsedURL.Host == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "webhookUrl must be a valid HTTP or HTTPS URL"})
			}
		}

		req.ID = 0 // ensure auto-increment
		req.CreatedAt = time.Now()
		req.UpdatedAt = time.Now()

		created, createErr := reg.NotificationChannel.Create(req)
		if createErr != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create notification channel"})
		}

		return c.JSON(http.StatusCreated, created)
	})

	// PUT /api/v1/notifications/channels/:id — update a channel
	g.PUT("/notifications/channels/:id", func(c echo.Context) error {
		id := c.Param("id")

		idNum, convErr := strconv.ParseUint(id, 10, 64)
		if convErr != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID"})
		}

		existing, err := reg.NotificationChannel.GetByID(uint(idNum))
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Notification channel not found"})
		}

		var req db.NotificationConfig
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		}

		// Validate type if changed
		if req.Type != "" && !db.ValidNotificationChannelTypes[req.Type] {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "type must be discord, slack, or inapp"})
		}

		// Validate webhook URL scheme (must be http or https to prevent SSRF via exotic schemes)
		if req.WebhookURL != "" {
			parsedURL, err := url.Parse(req.WebhookURL)
			if err != nil || (parsedURL.Scheme != schemeHTTP && parsedURL.Scheme != schemeHTTPS) || parsedURL.Host == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "webhookUrl must be a valid HTTP or HTTPS URL"})
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
		existing.Enabled = req.Enabled
		existing.OnThresholdBreach = req.OnThresholdBreach
		existing.OnDeletionExecuted = req.OnDeletionExecuted
		existing.OnEngineError = req.OnEngineError
		existing.OnEngineComplete = req.OnEngineComplete
		existing.UpdatedAt = time.Now()

		updated, updateErr := reg.NotificationChannel.Update(existing.ID, *existing)
		if updateErr != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update notification channel"})
		}

		return c.JSON(http.StatusOK, updated)
	})

	// DELETE /api/v1/notifications/channels/:id — delete a channel
	g.DELETE("/notifications/channels/:id", func(c echo.Context) error {
		id := c.Param("id")

		idNum, convErr := strconv.ParseUint(id, 10, 64)
		if convErr != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID"})
		}

		if deleteErr := reg.NotificationChannel.Delete(uint(idNum)); deleteErr != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Notification channel not found"})
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
	})

	// POST /api/v1/notifications/channels/:id/test — send a test notification
	g.POST("/notifications/channels/:id/test", func(c echo.Context) error {
		id := c.Param("id")

		idNum, convErr := strconv.ParseUint(id, 10, 64)
		if convErr != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID"})
		}

		if err := reg.NotificationChannel.TestChannel(uint(idNum)); err != nil {
			if err.Error() == "not found" {
				return c.JSON(http.StatusNotFound, map[string]string{"error": "Notification channel not found"})
			}
			return c.JSON(http.StatusBadGateway, map[string]string{"error": "Test notification failed: " + err.Error()})
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "sent"})
	})

	// --- In-App Notification Management ---

	// GET /api/v1/notifications — list in-app notifications (newest first, limit 50)
	g.GET("/notifications", func(c echo.Context) error {
		items, err := reg.NotificationChannel.ListInApp(50)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch notifications"})
		}
		return c.JSON(http.StatusOK, items)
	})

	// GET /api/v1/notifications/unread-count — return count of unread notifications
	g.GET("/notifications/unread-count", func(c echo.Context) error {
		count, err := reg.NotificationChannel.UnreadCount()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to count unread notifications"})
		}
		return c.JSON(http.StatusOK, map[string]int64{"count": count})
	})

	// PUT /api/v1/notifications/:id/read — mark a notification as read
	g.PUT("/notifications/:id/read", func(c echo.Context) error {
		id := c.Param("id")

		idNum, convErr := strconv.ParseUint(id, 10, 64)
		if convErr != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid ID"})
		}

		if err := reg.NotificationChannel.MarkRead(uint(idNum)); err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Notification not found"})
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "read"})
	})

	// PUT /api/v1/notifications/read-all — mark all notifications as read
	g.PUT("/notifications/read-all", func(c echo.Context) error {
		if err := reg.NotificationChannel.MarkAllRead(); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to mark all notifications as read"})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "all_read"})
	})

	// DELETE /api/v1/notifications — delete all in-app notifications
	g.DELETE("/notifications", func(c echo.Context) error {
		if err := reg.NotificationChannel.ClearAllInApp(); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to clear notifications"})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "cleared"})
	})
}
