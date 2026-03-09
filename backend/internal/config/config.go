// Package config handles application configuration loading from environment variables.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"strings"
)

// Config holds the application-wide configuration loaded from environment variables.
type Config struct {
	Port          string
	BaseURL       string
	Database      string
	Debug         bool
	JWTSecret     string `json:"-"`
	CORSOrigins   []string
	SecureCookies bool
	AuthHeader    string // Trusted reverse proxy auth header (e.g. "Remote-User", "X-authentik-username")
}

// Load reads environment variables and returns a populated Config.
func Load() *Config {
	debug := strings.ToLower(os.Getenv("DEBUG")) == "true"

	port := os.Getenv("PORT")
	if port == "" {
		port = "2187"
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "/"
	}
	if !strings.HasPrefix(baseURL, "/") {
		baseURL = "/" + baseURL
	}
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "/config/capacitarr.db"
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		if debug {
			jwtSecret = "development_secret_do_not_use_in_production"
			slog.Warn("Using default JWT secret — this is only acceptable in debug mode", "component", "config")
		} else {
			// Generate a random secret for this run and warn the user
			bytes := make([]byte, 32)
			if _, err := rand.Read(bytes); err != nil {
				slog.Error("Failed to generate random JWT secret", "component", "config", "operation", "generate_jwt_secret", "error", err)
				os.Exit(1)
			}
			jwtSecret = hex.EncodeToString(bytes)
			slog.Warn("No JWT_SECRET set — generated a random secret for this session. Sessions will not persist across restarts. Set JWT_SECRET environment variable for persistent sessions.", "component", "config")
		}
	}

	// CORS origins configuration
	corsOrigins := []string{}
	corsEnv := os.Getenv("CORS_ORIGINS")
	if corsEnv != "" {
		for _, origin := range strings.Split(corsEnv, ",") {
			origin = strings.TrimSpace(origin)
			if origin != "" {
				corsOrigins = append(corsOrigins, origin)
			}
		}
	} else if debug {
		corsOrigins = []string{"*"}
	}
	// If no CORS origins and not debug, leave empty (same-origin only)

	secureCookies := strings.ToLower(os.Getenv("SECURE_COOKIES")) == "true"

	authHeader := strings.TrimSpace(os.Getenv("AUTH_HEADER"))
	if authHeader != "" {
		slog.Info("Trusted reverse proxy auth header configured", "component", "config", "header", authHeader) //nolint:gosec // G706: authHeader is from trusted env var, not user input
		// SECURITY: When AUTH_HEADER is set, the server trusts that header
		// unconditionally for authentication. If the server is directly
		// exposed to the internet (not behind a reverse proxy), any client
		// can spoof this header and gain access. Only use this setting when
		// Capacitarr is behind a trusted reverse proxy (Authelia, Authentik,
		// Organizr, Caddy, Traefik, etc.) that strips and re-sets the header.
		slog.Warn("SECURITY: AUTH_HEADER is set — ensure Capacitarr is behind a trusted reverse proxy. If exposed directly, any client can spoof this header and bypass authentication.", "component", "config", "header", authHeader) //nolint:gosec // G706: authHeader is from trusted env var, not user input
	}

	return &Config{
		Port:          port,
		BaseURL:       baseURL,
		Database:      dbPath,
		Debug:         debug,
		JWTSecret:     jwtSecret,
		CORSOrigins:   corsOrigins,
		SecureCookies: secureCookies,
		AuthHeader:    authHeader,
	}
}
