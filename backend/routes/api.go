package routes

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"capacitarr/internal/config"
	"capacitarr/internal/db"
	"capacitarr/internal/poller"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

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

	// Public Auth
	g.POST("/auth/login", func(c echo.Context) error {
		var req LoginRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		}

		if req.Username == "" || req.Password == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Username and password are required"})
		}

		var user db.AuthConfig
		if err := database.Where("username = ?", req.Username).First(&user).Error; err != nil {
			// If no user exists in DB at all, bootstrap the first user
			var count int64
			database.Model(&db.AuthConfig{}).Count(&count)
			if count == 0 {
				hashed, hashErr := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
				if hashErr != nil {
					return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to hash password"})
				}
				user = db.AuthConfig{Username: req.Username, Password: string(hashed)}
				database.Create(&user)
			} else {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
			}
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": user.Username,
			"exp": time.Now().Add(24 * time.Hour).Unix(),
		})

		tokenString, err := token.SignedString([]byte(cfg.JWTSecret))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error generating token"})
		}

		// Set HttpOnly JWT cookie for secure transport
		c.SetCookie(&http.Cookie{
			Name:     "jwt",
			Value:    tokenString,
			Expires:  time.Now().Add(24 * time.Hour),
			HttpOnly: true,
			Secure:   cfg.SecureCookies,
			Path:     cfg.BaseURL,
			SameSite: http.SameSiteLaxMode,
		})

		// Set a non-HttpOnly cookie so the SPA can detect auth state
		c.SetCookie(&http.Cookie{
			Name:     "authenticated",
			Value:    "true",
			Expires:  time.Now().Add(24 * time.Hour),
			HttpOnly: false,
			Secure:   cfg.SecureCookies,
			Path:     cfg.BaseURL,
			SameSite: http.SameSiteLaxMode,
		})

		return c.JSON(http.StatusOK, map[string]string{"message": "success", "token": tokenString})
	})

	// Protected Routes
	protected := g.Group("")
	protected.Use(RequireAuth(database, cfg))

	protected.PUT("/auth/password", func(c echo.Context) error {
		username, ok := c.Get("user").(string)
		if !ok || username == "" {
			return echo.ErrUnauthorized
		}

		var req struct {
			CurrentPassword string `json:"currentPassword"`
			NewPassword     string `json:"newPassword"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		}

		if req.CurrentPassword == "" || req.NewPassword == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Current and new password are required"})
		}
		if len(req.NewPassword) < 8 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "New password must be at least 8 characters"})
		}

		var user db.AuthConfig
		if err := database.Where("username = ?", username).First(&user).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Current password is incorrect"})
		}

		hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to hash password"})
		}

		if err := database.Model(&user).Update("password", string(hashed)).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update password"})
		}

		return c.JSON(http.StatusOK, map[string]string{"message": "Password changed successfully"})
	})

	protected.PUT("/auth/username", func(c echo.Context) error {
		currentUser, ok := c.Get("user").(string)
		if !ok || currentUser == "" {
			return echo.ErrUnauthorized
		}

		var req struct {
			NewUsername     string `json:"newUsername"`
			CurrentPassword string `json:"currentPassword"`
		}
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		}

		if req.NewUsername == "" || req.CurrentPassword == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "New username and current password are required"})
		}
		if len(req.NewUsername) < 3 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Username must be at least 3 characters"})
		}

		var user db.AuthConfig
		if err := database.Where("username = ?", currentUser).First(&user).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
		}

		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Current password is incorrect"})
		}

		// Check if new username is already taken
		var existing db.AuthConfig
		if err := database.Where("username = ?", req.NewUsername).First(&existing).Error; err == nil {
			return c.JSON(http.StatusConflict, map[string]string{"error": "Username already taken"})
		}

		if err := database.Model(&user).Update("username", req.NewUsername).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update username"})
		}

		return c.JSON(http.StatusOK, map[string]string{"message": "Username changed successfully"})
	})

	protected.POST("/auth/apikey", func(c echo.Context) error {
		username, ok := c.Get("user").(string)
		if !ok || username == "" {
			return echo.ErrUnauthorized
		}

		bytes := make([]byte, 32)
		if _, err := rand.Read(bytes); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error generating API key"})
		}
		apiKey := hex.EncodeToString(bytes)

		if err := database.Model(&db.AuthConfig{}).Where("username = ?", username).Update("api_key", apiKey).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Database error"})
		}

		return c.JSON(http.StatusOK, map[string]string{"api_key": apiKey})
	})

	protected.GET("/auth/apikey", func(c echo.Context) error {
		username, ok := c.Get("user").(string)
		if !ok || username == "" {
			return echo.ErrUnauthorized
		}

		var user db.AuthConfig
		if err := database.Where("username = ?", username).First(&user).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
		}

		return c.JSON(http.StatusOK, map[string]string{"api_key": user.APIKey})
	})

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

	// Audit routes
	RegisterAuditRoutes(protected, database)

	// Data management routes (reset/clear)
	RegisterDataRoutes(protected, database)
}
