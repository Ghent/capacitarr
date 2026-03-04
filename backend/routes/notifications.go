package routes

import (
	"net/http"
	"net/url"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"capacitarr/internal/db"
	"capacitarr/internal/notifications"
)

// Notification channel type constants.
const (
	notifTypeDiscord = "discord"
	notifTypeSlack   = "slack"
	notifTypeInApp   = "inapp"
)

// RegisterNotificationRoutes sets up CRUD endpoints for notification channels
// and management endpoints for in-app notifications.
func RegisterNotificationRoutes(g *echo.Group, database *gorm.DB) {
	// --- Notification Channel CRUD ---

	// GET /api/v1/notifications/channels — list all notification configs
	g.GET("/notifications/channels", func(c echo.Context) error {
		configs := make([]db.NotificationConfig, 0)
		if err := database.Order("id ASC").Find(&configs).Error; err != nil {
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
		if req.Type != notifTypeDiscord && req.Type != notifTypeSlack && req.Type != notifTypeInApp {
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

		if err := database.Create(&req).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create notification channel"})
		}

		return c.JSON(http.StatusCreated, req)
	})

	// PUT /api/v1/notifications/channels/:id — update a channel
	g.PUT("/notifications/channels/:id", func(c echo.Context) error {
		id := c.Param("id")

		var existing db.NotificationConfig
		if err := database.First(&existing, id).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Notification channel not found"})
		}

		var req db.NotificationConfig
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		}

		// Validate type if changed
		if req.Type != "" && req.Type != notifTypeDiscord && req.Type != notifTypeSlack && req.Type != notifTypeInApp {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "type must be discord, slack, or inapp"})
		}

		// Validate webhook URL scheme (must be http or https to prevent SSRF via exotic schemes)
		if req.WebhookURL != "" {
			parsedURL, err := url.Parse(req.WebhookURL)
			if err != nil || (parsedURL.Scheme != schemeHTTP && parsedURL.Scheme != schemeHTTPS) || parsedURL.Host == "" {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": "webhookUrl must be a valid HTTP or HTTPS URL"})
			}
		}

		updates := map[string]interface{}{
			"updated_at": time.Now(),
		}
		if req.Name != "" {
			updates["name"] = req.Name
		}
		if req.Type != "" {
			updates["type"] = req.Type
		}
		updates["webhook_url"] = req.WebhookURL
		updates["enabled"] = req.Enabled
		updates["on_threshold_breach"] = req.OnThresholdBreach
		updates["on_deletion_executed"] = req.OnDeletionExecuted
		updates["on_engine_error"] = req.OnEngineError
		updates["on_engine_complete"] = req.OnEngineComplete

		if err := database.Model(&existing).Updates(updates).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update notification channel"})
		}

		// Re-fetch to return full updated record
		database.First(&existing, id)
		return c.JSON(http.StatusOK, existing)
	})

	// DELETE /api/v1/notifications/channels/:id — delete a channel
	g.DELETE("/notifications/channels/:id", func(c echo.Context) error {
		id := c.Param("id")

		var existing db.NotificationConfig
		if err := database.First(&existing, id).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Notification channel not found"})
		}

		if err := database.Delete(&existing).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete notification channel"})
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "deleted"})
	})

	// POST /api/v1/notifications/channels/:id/test — send a test notification
	g.POST("/notifications/channels/:id/test", func(c echo.Context) error {
		id := c.Param("id")

		var cfg db.NotificationConfig
		if err := database.First(&cfg, id).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Notification channel not found"})
		}

		event := notifications.NotificationEvent{
			Type:    notifications.EventEngineComplete,
			Title:   "Test Notification",
			Message: "This is a test notification from Capacitarr. If you see this, the channel is working correctly!",
			Fields: map[string]string{
				"Channel": cfg.Name,
				"Type":    cfg.Type,
			},
		}

		var err error
		switch cfg.Type {
		case notifTypeDiscord:
			err = notifications.SendDiscord(cfg.WebhookURL, event)
		case notifTypeSlack:
			err = notifications.SendSlack(cfg.WebhookURL, event)
		case notifTypeInApp:
			err = notifications.SendInApp(event)
		default:
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Unknown channel type"})
		}

		if err != nil {
			return c.JSON(http.StatusBadGateway, map[string]string{"error": "Test notification failed: " + err.Error()})
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "sent"})
	})

	// --- In-App Notification Management ---

	// GET /api/v1/notifications — list in-app notifications (newest first, limit 50)
	g.GET("/notifications", func(c echo.Context) error {
		items := make([]db.InAppNotification, 0)
		if err := database.Order("created_at DESC").Limit(50).Find(&items).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch notifications"})
		}
		return c.JSON(http.StatusOK, items)
	})

	// GET /api/v1/notifications/unread-count — return count of unread notifications
	g.GET("/notifications/unread-count", func(c echo.Context) error {
		var count int64
		if err := database.Model(&db.InAppNotification{}).Where("read = ?", false).Count(&count).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to count unread notifications"})
		}
		return c.JSON(http.StatusOK, map[string]int64{"count": count})
	})

	// PUT /api/v1/notifications/:id/read — mark a notification as read
	g.PUT("/notifications/:id/read", func(c echo.Context) error {
		id := c.Param("id")

		var notif db.InAppNotification
		if err := database.First(&notif, id).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Notification not found"})
		}

		if err := database.Model(&notif).Update("read", true).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to mark notification as read"})
		}

		return c.JSON(http.StatusOK, map[string]string{"status": "read"})
	})

	// PUT /api/v1/notifications/read-all — mark all notifications as read
	g.PUT("/notifications/read-all", func(c echo.Context) error {
		if err := database.Model(&db.InAppNotification{}).Where("read = ?", false).Update("read", true).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to mark all notifications as read"})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "all_read"})
	})

	// DELETE /api/v1/notifications — delete all in-app notifications
	g.DELETE("/notifications", func(c echo.Context) error {
		if err := database.Where("1 = 1").Delete(&db.InAppNotification{}).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to clear notifications"})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "cleared"})
	})
}
