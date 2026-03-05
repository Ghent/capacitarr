package routes

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"capacitarr/internal/config"
	"capacitarr/internal/db"
)

// bcryptCost is the cost factor for bcrypt password hashing across all auth
// operations. The Go default is 10; we use 12 for stronger brute-force
// resistance while keeping hashing under ~250ms on typical hardware.
const bcryptCost = 12

// LoginRequest holds the JSON body of login requests.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RegisterAuthRoutes sets up login, logout, password change, first-user
// bootstrap, and API key management endpoints.
func RegisterAuthRoutes(public *echo.Group, protected *echo.Group, database *gorm.DB, cfg *config.Config) {
	// Auth status — public endpoint for first-login UX detection
	public.GET("/auth/status", func(c echo.Context) error {
		var count int64
		database.Model(&db.AuthConfig{}).Count(&count)
		return c.JSON(http.StatusOK, map[string]interface{}{
			"initialized": count > 0,
		})
	})

	// Rate-limit login endpoint: 10 attempts per IP per 15-minute window
	loginRL := newLoginRateLimiter(10, 15*time.Minute)

	public.POST("/auth/login", func(c echo.Context) error {
		var req LoginRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		}

		if req.Username == "" || req.Password == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Username and password are required"})
		}

		var user db.AuthConfig
		if err := database.Where("username = ?", req.Username).First(&user).Error; err != nil {
			// If no user exists in DB at all, bootstrap the first user.
			// Use a transaction to prevent a race condition where two concurrent
			// requests both see count==0 and create duplicate users. The unique
			// index on username provides an additional safety net.
			var bootstrapped bool
			txErr := database.Transaction(func(tx *gorm.DB) error {
				var count int64
				if err := tx.Model(&db.AuthConfig{}).Count(&count).Error; err != nil {
					return err
				}
				if count > 0 {
					return nil // Another request already created the first user
				}
				hashed, hashErr := bcrypt.GenerateFromPassword([]byte(req.Password), bcryptCost)
				if hashErr != nil {
					return hashErr
				}
				user = db.AuthConfig{Username: req.Username, Password: string(hashed)}
				if err := tx.Create(&user).Error; err != nil {
					return err
				}
				bootstrapped = true
				slog.Info("First user bootstrapped", "component", "auth", "username", req.Username)
				return nil
			})
			if txErr != nil {
				slog.Error("First-user bootstrap failed", "component", "auth", "operation", "bootstrap_user", "error", txErr)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create initial user"})
			}
			if !bootstrapped {
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

		// Log login activity event
		db.LogActivity(database, db.EventLogin, fmt.Sprintf("User '%s' logged in", user.Username))

		return c.JSON(http.StatusOK, map[string]string{"message": "success", "token": tokenString})
	}, LoginRateLimit(loginRL))

	// Password change
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

		hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcryptCost)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to hash password"})
		}

		if err := database.Model(&user).Update("password", string(hashed)).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update password"})
		}

		db.LogActivity(database, db.EventPasswordChanged, fmt.Sprintf("Password changed for user '%s'", username))

		return c.JSON(http.StatusOK, map[string]string{"message": "Password changed successfully"})
	})

	// Username change
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

	// Generate API key
	protected.POST("/auth/apikey", func(c echo.Context) error {
		username, ok := c.Get("user").(string)
		if !ok || username == "" {
			return echo.ErrUnauthorized
		}

		// Generate a cryptographically random API key (256 bits of entropy)
		keyBytes := make([]byte, 32)
		if _, err := rand.Read(keyBytes); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error generating API key"})
		}
		plaintextKey := hex.EncodeToString(keyBytes)

		// Store only the SHA-256 hash — the plaintext is returned once and never stored.
		// Also persist the last 4 characters as a hint for identification.
		hashedKey := HashAPIKey(plaintextKey)
		hint := plaintextKey[len(plaintextKey)-4:]
		if err := database.Model(&db.AuthConfig{}).Where("username = ?", username).Updates(map[string]interface{}{
			"api_key":      hashedKey,
			"api_key_hint": hint,
		}).Error; err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Database error"})
		}

		return c.JSON(http.StatusOK, map[string]string{"api_key": plaintextKey})
	})

	// Check API key status
	protected.GET("/auth/apikey", func(c echo.Context) error {
		username, ok := c.Get("user").(string)
		if !ok || username == "" {
			return echo.ErrUnauthorized
		}

		var user db.AuthConfig
		if err := database.Where("username = ?", username).First(&user).Error; err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
		}

		// Never return the actual API key (it's hashed in the DB). Instead
		// return whether a key has been generated and the last 4 chars hint
		// so the UI can show a recognisable masked version.
		hasKey := user.APIKey != ""
		return c.JSON(http.StatusOK, map[string]interface{}{
			"has_key": hasKey,
			"hint":    user.APIKeyHint,
		})
	})
}
