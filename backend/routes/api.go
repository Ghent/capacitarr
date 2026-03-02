package routes

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"capacitarr/internal/config"
	"capacitarr/internal/db"
	"capacitarr/internal/poller"
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
func RegisterAPIRoutes(g *echo.Group, database *gorm.DB, cfg *config.Config, appVersion, appCommit, appBuildDate string) {
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
	RegisterAuthRoutes(g, protected, database, cfg)

	// Metrics History
	protected.GET("/metrics/history", func(c echo.Context) error {
		resolution := c.QueryParam("resolution")
		if resolution == "" {
			resolution = "raw"
		}

		diskGroupID := c.QueryParam("disk_group_id")
		since := c.QueryParam("since") // e.g. "1h", "24h", "7d", "30d"

		query := database.Where("resolution = ?", resolution)
		if diskGroupID != "" {
			query = query.Where("disk_group_id = ?", diskGroupID)
		}

		// Apply time range filter
		if since != "" {
			var duration time.Duration
			switch since {
			case "1h":
				duration = 1 * time.Hour
			case "24h":
				duration = 24 * time.Hour
			case "7d":
				duration = 7 * 24 * time.Hour
			case "30d":
				duration = 30 * 24 * time.Hour
			}
			if duration > 0 {
				cutoff := time.Now().Add(-duration)
				query = query.Where("timestamp >= ?", cutoff)
			}
		}

		var history []db.LibraryHistory
		if err := query.Order("timestamp asc").Limit(1000).Find(&history).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error fetching metrics"})
		}

		return c.JSON(http.StatusOK, map[string]interface{}{"status": "success", "data": history})
	})

	// Integration management routes
	RegisterIntegrationRoutes(protected, database)

	// Preference and Rules routes
	RegisterRuleRoutes(protected, database)

	// Disk Groups routes
	protected.GET("/disk-groups", func(c echo.Context) error {
		var groups []db.DiskGroup
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

		if err := database.Model(&group).Select("threshold_pct", "target_pct").Updates(db.DiskGroup{
			ThresholdPct: req.ThresholdPct,
			TargetPct:    req.TargetPct,
		}).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update disk group"})
		}
		group.ThresholdPct = req.ThresholdPct
		group.TargetPct = req.TargetPct
		return c.JSON(http.StatusOK, group)
	})

	// Worker Metrics
	protected.GET("/metrics/worker", func(c echo.Context) error {
		metrics := poller.GetWorkerMetrics()
		return c.JSON(http.StatusOK, metrics)
	})

	// Worker Stats (alias for dashboard consumption)
	protected.GET("/worker/stats", func(c echo.Context) error {
		metrics := poller.GetWorkerMetrics()
		return c.JSON(http.StatusOK, metrics)
	})

	// Engine Run Now - trigger an immediate evaluation cycle
	protected.POST("/engine/run", func(c echo.Context) error {
		select {
		case poller.RunNowCh <- struct{}{}:
			return c.JSON(http.StatusOK, map[string]string{"status": "triggered"})
		default:
			return c.JSON(http.StatusOK, map[string]string{"status": "already_pending"})
		}
	})

	// Lifetime stats (cumulative counters, not cleared by data reset)
	protected.GET("/lifetime-stats", func(c echo.Context) error {
		var stats db.LifetimeStats
		if err := database.FirstOrCreate(&stats, db.LifetimeStats{ID: 1}).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch lifetime stats"})
		}
		return c.JSON(http.StatusOK, stats)
	})

	// Dashboard stats (aggregates lifetime stats, protected count, library growth rate)
	protected.GET("/dashboard-stats", handleDashboardStats(database))

	// Audit routes
	RegisterAuditRoutes(protected, database)

	// Data management routes (reset/clear)
	RegisterDataRoutes(protected, database)
}

func handleDashboardStats(database *gorm.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		// 1. Lifetime stats
		var lifetime db.LifetimeStats
		database.FirstOrCreate(&lifetime, db.LifetimeStats{ID: 1})

		// 2. Protected count from worker metrics
		metrics := poller.GetWorkerMetrics()
		protectedCount, _ := metrics["protectedCount"].(int64)

		// 3. Library growth rate: compare most recent entry to 7 days ago
		var recent db.LibraryHistory
		var weekAgo db.LibraryHistory
		growthBytes := int64(0)
		hasGrowthData := false

		cutoff := time.Now().Add(-7 * 24 * time.Hour)
		// Most recent entry
		if err := database.Where("resolution = ?", "raw").
			Order("timestamp DESC").First(&recent).Error; err == nil {
			// Entry closest to 7 days ago
			if err := database.Where("resolution = ? AND timestamp <= ?", "raw", cutoff).
				Order("timestamp DESC").First(&weekAgo).Error; err == nil {
				growthBytes = recent.UsedCapacity - weekAgo.UsedCapacity
				hasGrowthData = true
			}
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"totalBytesReclaimed": lifetime.TotalBytesReclaimed,
			"totalItemsRemoved":   lifetime.TotalItemsRemoved,
			"totalEngineRuns":     lifetime.TotalEngineRuns,
			"protectedCount":      protectedCount,
			"growthBytesPerWeek":  growthBytes,
			"hasGrowthData":       hasGrowthData,
		})
	}
}
