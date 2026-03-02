package routes

import (
	"log/slog"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"capacitarr/internal/config"
	"capacitarr/internal/db"
)

// validateAPIKey checks the given plaintext API key against stored (hashed or
// legacy plaintext) keys. If a legacy plaintext key matches, it is
// transparently upgraded to a SHA-256 hash in the database. Returns the
// matching AuthConfig on success, or nil if no match.
func validateAPIKey(database *gorm.DB, plaintextKey string) *db.AuthConfig {
	hashedKey := HashAPIKey(plaintextKey)

	// Fast path: look up by the hashed value (new-style keys)
	var auth db.AuthConfig
	if err := database.Where("api_key = ?", hashedKey).First(&auth).Error; err == nil {
		return &auth
	}

	// Slow path: legacy plaintext key — look up directly and upgrade in-place
	if err := database.Where("api_key = ?", plaintextKey).First(&auth).Error; err == nil {
		if !IsHashedAPIKey(auth.APIKey) {
			database.Model(&auth).Update("api_key", hashedKey)
			slog.Info("Upgraded legacy plaintext API key to SHA-256 hash", "username", auth.Username)
		}
		return &auth
	}

	return nil
}

func RequireAuth(database *gorm.DB, cfg *config.Config) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 1. Trusted reverse proxy auth header (Authelia/Authentik/Organizr)
			if cfg.AuthHeader != "" {
				headerUser := strings.TrimSpace(c.Request().Header.Get(cfg.AuthHeader))
				if headerUser != "" {
					// Auto-create user record if the header user doesn't exist
					var auth db.AuthConfig
					if err := database.Where("username = ?", headerUser).First(&auth).Error; err != nil {
						// Generate a random unusable password hash for proxy-auth users
						placeholder, _ := bcrypt.GenerateFromPassword([]byte("proxy-auth-placeholder"), bcryptCost)
						auth = db.AuthConfig{
							Username: headerUser,
							Password: string(placeholder),
						}
						database.Create(&auth)
						slog.Info("Auto-created user from proxy auth header", "username", headerUser)
					}
					c.Set("user", headerUser)
					return next(c)
				}
			}

			var tokenStr string

			// 2. Check Authorization header (Bearer JWT or ApiKey)
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) != 2 {
					return echo.ErrUnauthorized
				}

				if parts[0] == "Bearer" {
					tokenStr = parts[1]
				} else if parts[0] == "ApiKey" {
					if auth := validateAPIKey(database, parts[1]); auth != nil {
						c.Set("user", auth.Username)
						return next(c)
					}
					return echo.ErrUnauthorized
				} else {
					return echo.ErrUnauthorized
				}
			}

			// 3. Check X-Api-Key header or apikey query param
			if tokenStr == "" {
				apiKey := c.Request().Header.Get("X-Api-Key")
				if apiKey == "" {
					apiKey = c.QueryParam("apikey")
				}
				if apiKey != "" {
					if auth := validateAPIKey(database, apiKey); auth != nil {
						c.Set("user", auth.Username)
						return next(c)
					}
					return echo.ErrUnauthorized
				}
			}

			// 4. Fallback: check jwt cookie
			if tokenStr == "" {
				cookie, err := c.Cookie("jwt")
				if err != nil || cookie.Value == "" {
					return echo.ErrUnauthorized
				}
				tokenStr = cookie.Value
			}

			// 5. Validate JWT token
			token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
				// Ensure the signing method is what we expect
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, echo.ErrUnauthorized
				}
				return []byte(cfg.JWTSecret), nil
			})

			if err != nil || !token.Valid {
				return echo.ErrUnauthorized
			}

			// Safe type assertions with comma-ok pattern
			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				return echo.ErrUnauthorized
			}

			sub, ok := claims["sub"].(string)
			if !ok || sub == "" {
				return echo.ErrUnauthorized
			}

			c.Set("user", sub)
			return next(c)
		}
	}
}
