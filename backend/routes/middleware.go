package routes

import (
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"

	"capacitarr/internal/services"
)

// RequireAuth returns Echo middleware that authenticates requests via trusted proxy header, JWT, or API key.
func RequireAuth(reg *services.Registry) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cfg := reg.Cfg

			// 1. Trusted reverse proxy auth header (Authelia/Authentik/Organizr)
			if cfg.AuthHeader != "" {
				headerUser := strings.TrimSpace(c.Request().Header.Get(cfg.AuthHeader))
				if headerUser != "" {
					// Auto-create user record if the header user doesn't exist
					if err := reg.Auth.EnsureProxyUser(headerUser); err != nil {
						return echo.ErrUnauthorized
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

				switch parts[0] {
				case "Bearer": //nolint:gocritic // auth method branches test different conditions
					tokenStr = parts[1]
				case "ApiKey":
					auth, err := reg.Auth.ValidateAPIKey(parts[1])
					if err != nil {
						return echo.ErrUnauthorized
					}
					c.Set("user", auth.Username)
					return next(c)
				default:
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
					auth, err := reg.Auth.ValidateAPIKey(apiKey)
					if err != nil {
						return echo.ErrUnauthorized
					}
					c.Set("user", auth.Username)
					return next(c)
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
			token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
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
