// Package routes defines HTTP handlers and middleware for the Capacitarr API.
package routes

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/events"
	"capacitarr/internal/services"
)

// RegisterAPIRoutes sets up all API routes: public endpoints, auth, and
// protected resource endpoints.
func RegisterAPIRoutes(g *echo.Group, reg *services.Registry, appVersion, appCommit, appBuildDate string, sseBroadcaster *events.SSEBroadcaster) {
	// Health check
	g.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	// Version info (public — no auth required)
	g.GET("/version", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"version":   appVersion,
			"commit":    appCommit,
			"buildDate": appBuildDate,
		})
	})

	// Protected Routes
	protected := g.Group("")
	protected.Use(RequireAuth(reg))

	// Auth routes (login is public, password/username/apikey are protected)
	RegisterAuthRoutes(g, protected, reg)

	// Migration routes (status is public for pre-auth detection, execute is protected)
	RegisterMigrationRoutes(g, protected, reg)

	// SSE event stream — authenticated, long-lived connection
	if sseBroadcaster != nil {
		protected.GET("/events", sseBroadcaster.HandleSSE)
	}

	// Metrics and statistics routes
	RegisterMetricsRoutes(protected, reg)

	// Integration management routes
	RegisterIntegrationRoutes(protected, reg)

	// Preference and Rules routes
	RegisterRuleRoutes(protected, reg)

	// Disk Groups routes
	RegisterDiskGroupRoutes(protected, reg)

	// Audit routes (history-only)
	RegisterAuditRoutes(protected, reg)

	// Approval queue routes
	RegisterApprovalRoutes(protected, reg)

	// Activity event routes
	RegisterActivityRoutes(protected, reg)

	// Engine routes (history + run)
	RegisterEngineRoutes(protected, reg)

	// Notification routes (channels CRUD)
	RegisterNotificationRoutes(protected, reg)

	// Data management routes (reset/clear)
	RegisterDataRoutes(protected, reg)

	// Version check routes (update check with cache)
	RegisterVersionRoutes(protected, reg)

	// Settings backup (export/import)
	RegisterBackupRoutes(protected, reg, appVersion)

	// Deletion queue management (list + cancel)
	RegisterDeletionQueueRoutes(protected, reg)

	// Analytics routes (watch intelligence: dead content, stale content, forecast)
	RegisterAnalyticsRoutes(protected, reg)

	// Library management routes (CRUD)
	RegisterLibraryRoutes(protected, reg)
}
