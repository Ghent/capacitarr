// Package routes defines HTTP handlers and middleware for the Capacitarr API.
package routes

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/services"
)

// apiKeyHashPrefix marks a stored API key as already hashed with SHA-256.
// Legacy plaintext keys (without this prefix) are transparently upgraded
// on first use.
const apiKeyHashPrefix = "sha256:"

// HashAPIKey produces a deterministic SHA-256 hash of the given plaintext
// API key, prefixed with "sha256:" so we can distinguish hashed from legacy
// plaintext keys in the database. SHA-256 (without salt) is appropriate here
// because API keys are 256-bit random values — effectively unguessable and
// immune to rainbow table attacks.
func HashAPIKey(plaintext string) string {
	h := sha256.Sum256([]byte(plaintext))
	return apiKeyHashPrefix + hex.EncodeToString(h[:])
}

// IsHashedAPIKey returns true if the stored key already uses the hashed format.
func IsHashedAPIKey(stored string) bool {
	return strings.HasPrefix(stored, apiKeyHashPrefix)
}

// RegisterAPIRoutes sets up all API routes: public endpoints, auth, and
// protected resource endpoints.
func RegisterAPIRoutes(g *echo.Group, reg *services.Registry, appVersion, appCommit, appBuildDate string, sseBroadcaster *events.SSEBroadcaster) {
	database := reg.DB
	cfg := reg.Cfg

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
	protected.Use(RequireAuth(database, cfg))

	// Auth routes (login is public, password/username/apikey are protected)
	RegisterAuthRoutes(g, protected, reg)

	// SSE event stream — authenticated, long-lived connection
	if sseBroadcaster != nil {
		protected.GET("/events", sseBroadcaster.HandleSSE)
	}

	// Metrics History
	protected.GET("/metrics/history", func(c echo.Context) error {
		resolution := c.QueryParam("resolution")
		diskGroupID := c.QueryParam("disk_group_id")
		since := c.QueryParam("since")

		history, err := reg.Metrics.GetHistory(resolution, diskGroupID, since)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error fetching metrics"})
		}

		return c.JSON(http.StatusOK, map[string]interface{}{"status": "success", "data": history})
	})

	// Integration management routes
	RegisterIntegrationRoutes(protected, reg)

	// Preference and Rules routes
	RegisterRuleRoutes(protected, reg)

	// Disk Groups routes
	protected.GET("/disk-groups", func(c echo.Context) error {
		groups := make([]db.DiskGroup, 0)
		if err := database.Find(&groups).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch disk groups"})
		}
		return c.JSON(http.StatusOK, groups)
	})

	protected.PUT("/disk-groups/:id", func(c echo.Context) error {
		id := c.Param("id")
		var group db.DiskGroup
		if err := database.First(&group, id).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Disk group not found"})
		}

		var req struct {
			ThresholdPct float64 `json:"thresholdPct"`
			TargetPct    float64 `json:"targetPct"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		}

		// Validate thresholds
		if req.ThresholdPct < 1 || req.ThresholdPct > 99 || req.TargetPct < 1 || req.TargetPct > 99 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Threshold and target must be between 1 and 99"})
		}
		if req.ThresholdPct <= req.TargetPct {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Threshold must be greater than target"})
		}

		if err := reg.Settings.UpdateThresholds(group.ID, req.ThresholdPct, req.TargetPct); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update disk group"})
		}

		// Reload the updated group
		database.First(&group, id)
		return c.JSON(http.StatusOK, group)
	})

	// Worker Stats
	protected.GET("/worker/stats", func(c echo.Context) error {
		return c.JSON(http.StatusOK, reg.Metrics.GetWorkerMetrics())
	})

	// Engine Run Now - trigger an immediate evaluation cycle
	protected.POST("/engine/run", func(c echo.Context) error {
		status := reg.Engine.TriggerRun()
		return c.JSON(http.StatusOK, map[string]string{"status": status})
	})

	// Lifetime stats (cumulative counters, not cleared by data reset)
	protected.GET("/lifetime-stats", func(c echo.Context) error {
		stats, err := reg.Metrics.GetLifetimeStats()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch lifetime stats"})
		}
		return c.JSON(http.StatusOK, stats)
	})

	// Dashboard stats (aggregates lifetime stats, protected count, library growth rate)
	protected.GET("/dashboard-stats", func(c echo.Context) error {
		stats, err := reg.Metrics.GetDashboardStats()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch dashboard stats"})
		}
		return c.JSON(http.StatusOK, stats)
	})

	// Audit routes (history-only)
	RegisterAuditRoutes(protected, reg)

	// Approval queue routes
	RegisterApprovalRoutes(protected, reg)

	// Activity event routes
	RegisterActivityRoutes(protected, reg)

	// Engine history routes
	RegisterEngineHistoryRoutes(protected, reg)

	// Notification routes (channels CRUD + in-app notifications)
	RegisterNotificationRoutes(protected, reg)

	// Data management routes (reset/clear)
	RegisterDataRoutes(protected, reg)

	// Version check routes (update check with cache)
	RegisterVersionRoutes(protected, reg)
}
