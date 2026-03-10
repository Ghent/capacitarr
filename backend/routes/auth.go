package routes

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"capacitarr/internal/services"
)

// LoginRequest holds the JSON body of login requests.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// RegisterAuthRoutes sets up login, logout, password change, first-user
// bootstrap, and API key management endpoints.
func RegisterAuthRoutes(public *echo.Group, protected *echo.Group, reg *services.Registry) {
	cfg := reg.Cfg

	// Auth status — public endpoint for first-login UX detection.
	// Also detects AUTH_HEADER misconfiguration: if AUTH_HEADER is set but the
	// configured header is absent from this request, the user is likely accessing
	// the application directly (not through the expected reverse proxy), which
	// means AUTH_HEADER would allow authentication bypass from anyone who
	// adds the header manually. The frontend uses this flag to show a warning.
	public.GET("/auth/status", func(c echo.Context) error {
		initialized, err := reg.Auth.IsInitialized()
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to check auth status")
		}

		response := map[string]any{
			"initialized": initialized,
		}

		// SECURITY: Detect missing reverse proxy when AUTH_HEADER is configured.
		// If AUTH_HEADER is set but this request doesn't contain the header value,
		// the user is accessing directly without a proxy — a dangerous misconfiguration.
		if cfg.AuthHeader != "" {
			headerValue := strings.TrimSpace(c.Request().Header.Get(cfg.AuthHeader))
			if headerValue == "" {
				response["authHeaderWarning"] = "AUTH_HEADER is configured (" + cfg.AuthHeader + ") but no proxy header was detected in this request. " +
					"If Capacitarr is not behind a reverse proxy that sets this header, any client can bypass authentication by spoofing the header. " +
					"Either place Capacitarr behind a trusted reverse proxy or remove the AUTH_HEADER environment variable."
			}
		}

		return c.JSON(http.StatusOK, response)
	})

	// Rate-limit login endpoint: 10 attempts per IP per 15-minute window
	loginRL := newLoginRateLimiter(10, 15*time.Minute)

	public.POST("/auth/login", func(c echo.Context) error {
		var req LoginRequest
		if err := c.Bind(&req); err != nil {
			return apiError(c, http.StatusBadRequest, "Invalid request body")
		}

		if req.Username == "" || req.Password == "" {
			return apiError(c, http.StatusBadRequest, "Username and password are required")
		}

		// Try to find existing user
		_, err := reg.Auth.GetByUsername(req.Username)
		if err != nil {
			// If no user exists in DB at all, bootstrap the first user.
			user, bootstrapErr := reg.Auth.Bootstrap(req.Username, req.Password)
			if bootstrapErr != nil {
				slog.Error("First-user bootstrap failed", "component", "auth", "operation", "bootstrap_user", "error", bootstrapErr)
				return apiError(c, http.StatusInternalServerError, "Failed to create initial user")
			}
			if user == nil {
				return apiError(c, http.StatusUnauthorized, "Invalid credentials")
			}
		}

		// Delegate credential check + JWT generation + event publishing to AuthService
		tokenString, loginErr := reg.Auth.Login(req.Username, req.Password)
		if loginErr != nil {
			return apiError(c, http.StatusUnauthorized, "Invalid credentials")
		}

		// Set HttpOnly JWT cookie for secure transport.
		// Secure flag is conditional on SECURE_COOKIES=true (for HTTPS deployments).
		c.SetCookie(&http.Cookie{ //nolint:gosec // nosemgrep
			Name:     "jwt",
			Value:    tokenString,
			Expires:  time.Now().Add(24 * time.Hour),
			HttpOnly: true,
			Secure:   cfg.SecureCookies,
			Path:     cfg.BaseURL,
			SameSite: http.SameSiteLaxMode,
		})

		// Set a non-HttpOnly cookie so the SPA can detect auth state.
		// This cookie contains no secrets (just "true") — the JWT cookie above is the sensitive one.
		// HttpOnly is intentionally false so JavaScript can read it for auth state detection.
		// Secure flag is conditional on SECURE_COOKIES=true (for HTTPS deployments).
		c.SetCookie(&http.Cookie{ //nolint:gosec // nosemgrep
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
			return apiError(c, http.StatusBadRequest, "Invalid request body")
		}

		if req.CurrentPassword == "" || req.NewPassword == "" {
			return apiError(c, http.StatusBadRequest, "Current and new password are required")
		}
		if len(req.NewPassword) < 8 {
			return apiError(c, http.StatusBadRequest, "New password must be at least 8 characters")
		}

		if err := reg.Auth.ChangePassword(username, req.CurrentPassword, req.NewPassword); err != nil {
			if errors.Is(err, services.ErrWrongPassword) {
				return apiError(c, http.StatusUnauthorized, "Current password is incorrect")
			}
			return apiError(c, http.StatusInternalServerError, err.Error())
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
			return apiError(c, http.StatusBadRequest, "Invalid request body")
		}

		if req.NewUsername == "" || req.CurrentPassword == "" {
			return apiError(c, http.StatusBadRequest, "New username and current password are required")
		}
		if len(req.NewUsername) < 3 {
			return apiError(c, http.StatusBadRequest, "Username must be at least 3 characters")
		}

		// Check if new username is already taken
		taken, err := reg.Auth.IsUsernameTaken(req.NewUsername)
		if err != nil {
			return apiError(c, http.StatusInternalServerError, "Failed to check username availability")
		}
		if taken {
			return apiError(c, http.StatusConflict, "Username already taken")
		}

		if err := reg.Auth.ChangeUsername(currentUser, req.NewUsername, req.CurrentPassword); err != nil {
			if errors.Is(err, services.ErrWrongPassword) {
				return apiError(c, http.StatusUnauthorized, "Current password is incorrect")
			}
			return apiError(c, http.StatusInternalServerError, err.Error())
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
			return apiError(c, http.StatusInternalServerError, err.Error())
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
			return apiError(c, http.StatusNotFound, "User not found")
		}

		// Never return the actual API key (it's hashed in the DB). Instead
		// return whether a key has been generated and the last 4 chars hint
		// so the UI can show a recognisable masked version.
		hasKey := user.APIKey != ""
		return c.JSON(http.StatusOK, map[string]any{
			"has_key": hasKey,
			"hint":    user.APIKeyHint,
		})
	})
}
