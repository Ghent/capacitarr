// Package routes defines HTTP handlers and middleware for the Capacitarr API.
package routes

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

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

	// Notification routes (channels CRUD + in-app notifications)
	RegisterNotificationRoutes(protected, reg)

	// Data management routes (reset/clear)
	RegisterDataRoutes(protected, reg)

	// Version check routes (update check with cache)
	RegisterVersionRoutes(protected, reg)
}
