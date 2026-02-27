package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/capacitarr/capacitarr/backend/internal/config"
	"github.com/capacitarr/capacitarr/backend/internal/db"
	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const UserContextKey contextKey = "user"

// AuthMiddleware validates JWT tokens from cookies or Authorization header
func AuthMiddleware(cfg *config.Config, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var tokenStr string

		// 1. Try to get token from cookie (for Web UI)
		if cookie, err := r.Cookie("jwt"); err == nil {
			tokenStr = cookie.Value
		}

		// 2. Try to get from Authorization header (Bearer)
		if tokenStr == "" {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if tokenStr == "" {
			http.Error(w, "Unauthorized: missing token", http.StatusUnauthorized)
			return
		}

		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
			return
		}

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			username, _ := claims["sub"].(string)
			ctx := context.WithValue(r.Context(), UserContextKey, username)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		http.Error(w, "Unauthorized: invalid claims", http.StatusUnauthorized)
	}
}

// APIKeyMiddleware validates programmatic API keys
func APIKeyMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			http.Error(w, "Unauthorized: missing API key", http.StatusUnauthorized)
			return
		}

		var user db.AuthConfig
		if err := db.DB.Where("api_key = ?", apiKey).First(&user).Error; err != nil {
			http.Error(w, "Unauthorized: invalid API key", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user.Username)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
