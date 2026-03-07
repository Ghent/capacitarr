package routes

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/services"
)

// bcryptCost references the shared bcrypt cost factor from the services package.
const bcryptCost = services.BcryptCost

// LoginRequest holds the JSON body of login requests.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RegisterAuthRoutes sets up login, logout, password change, first-user
// bootstrap, and API key management endpoints.
func RegisterAuthRoutes(public *echo.Group, protected *echo.Group, reg *services.Registry) {
	cfg := reg.Cfg

	// Auth status — public endpoint for first-login UX detection
	public.GET("/auth/status", func(c echo.Context) error {
		initialized, err := reg.Auth.IsInitialized()
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to check auth status"})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{
			"initialized": initialized,
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

		// Try to find existing user
		_, err := reg.Auth.GetByUsername(req.Username)
		if err != nil {
			// If no user exists in DB at all, bootstrap the first user.
			user, bootstrapErr := reg.Auth.Bootstrap(req.Username, req.Password)
			if bootstrapErr != nil {
				slog.Error("First-user bootstrap failed", "component", "auth", "operation", "bootstrap_user", "error", bootstrapErr)
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create initial user"})
			}
			if user == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
			}
		}

		// Delegate credential check + JWT generation + event publishing to AuthService
		tokenString, loginErr := reg.Auth.Login(req.Username, req.Password)
		if loginErr != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid credentials"})
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
	}, LoginRateLimit(loginRL))

	// Password change — delegates to AuthService
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

		if err := reg.Auth.ChangePassword(username, req.CurrentPassword, req.NewPassword); err != nil {
			// Distinguish between "wrong password" and other errors
			if err.Error() == "current password is incorrect" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		return c.JSON(http.StatusOK, map[string]string{"message": "Password changed successfully"})
	})

	// Username change — delegates to AuthService
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

		// Check if new username is already taken
		taken, err := reg.Auth.IsUsernameTaken(req.NewUsername)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to check username availability"})
		}
		if taken {
			return c.JSON(http.StatusConflict, map[string]string{"error": "Username already taken"})
		}

		if err := reg.Auth.ChangeUsername(currentUser, req.NewUsername, req.CurrentPassword); err != nil {
			if err.Error() == "password is incorrect" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Current password is incorrect"})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		return c.JSON(http.StatusOK, map[string]string{"message": "Username changed successfully"})
	})

	// Generate API key — delegates to AuthService
	protected.POST("/auth/apikey", func(c echo.Context) error {
		username, ok := c.Get("user").(string)
		if !ok || username == "" {
			return echo.ErrUnauthorized
		}

		plaintext, err := reg.Auth.GenerateAPIKey(username)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		return c.JSON(http.StatusOK, map[string]string{"api_key": plaintext})
	})

	// Check API key status
	protected.GET("/auth/apikey", func(c echo.Context) error {
		username, ok := c.Get("user").(string)
		if !ok || username == "" {
			return echo.ErrUnauthorized
		}

		user, err := reg.Auth.GetByUsername(username)
		if err != nil {
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
