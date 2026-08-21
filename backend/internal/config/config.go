// Package config handles application configuration loading from environment variables.
package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
)

// Config holds the application-wide configuration loaded from environment variables.
type Config struct {
	Port             string
	BaseURL          string
	Database         string
	Debug            bool
	JWTSecret        string `json:"-"`
	CORSOrigins      []string
	SecureCookies    bool
	AuthHeader       string       // Trusted reverse proxy auth header (e.g. "Remote-User", "X-authentik-username")
	TrustedProxies   []string     // Original TRUSTED_PROXIES entries (IPs and CIDRs)
	TrustedProxyNets []*net.IPNet // Parsed networks used by IsTrustedProxy
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

	trustedProxies, trustedNets := ParseTrustedProxies(os.Getenv("TRUSTED_PROXIES"))

	authHeader := strings.TrimSpace(os.Getenv("AUTH_HEADER"))
	if authHeader != "" {
		slog.Info("Trusted reverse proxy auth header configured", "component", "config", "header", authHeader) //nolint:gosec // G706: authHeader is from trusted env var, not user input
		if len(trustedNets) == 0 {
			slog.Warn("SECURITY: AUTH_HEADER is set but TRUSTED_PROXIES is empty — proxy header authentication is ignored. Set TRUSTED_PROXIES to the reverse proxy IP or CIDR (e.g. 127.0.0.1). JWT and API key auth continue to work.", "component", "config", "header", authHeader) //nolint:gosec // G706: authHeader is from trusted env var, not user input
		} else {
			slog.Warn("SECURITY: AUTH_HEADER is set — the header is accepted only from TRUSTED_PROXIES. Bind Capacitarr to localhost or a private network so clients cannot reach it except through the proxy.", "component", "config", "header", authHeader, "trustedProxies", trustedProxies) //nolint:gosec // G706: authHeader is from trusted env var, not user input
		}
	}

	return &Config{
		Port:             port,
		BaseURL:          baseURL,
		Database:         dbPath,
		Debug:            debug,
		JWTSecret:        jwtSecret,
		CORSOrigins:      corsOrigins,
		SecureCookies:    secureCookies,
		AuthHeader:       authHeader,
		TrustedProxies:   trustedProxies,
		TrustedProxyNets: trustedNets,
	}
}

// ParseTrustedProxies parses a comma-separated list of IPs and CIDRs.
// Bare IPv4 addresses become /32; bare IPv6 addresses become /128.
// Invalid entries are skipped and logged.
func ParseTrustedProxies(spec string) ([]string, []*net.IPNet) {
	var names []string
	var nets []*net.IPNet
	for _, raw := range strings.Split(spec, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		n, err := parseTrustedProxy(raw)
		if err != nil {
			slog.Warn("Ignoring invalid TRUSTED_PROXIES entry", "component", "config", "entry", raw, "error", err)
			continue
		}
		names = append(names, raw)
		nets = append(nets, n)
	}
	return names, nets
}

func parseTrustedProxy(entry string) (*net.IPNet, error) {
	if strings.Contains(entry, "/") {
		_, n, err := net.ParseCIDR(entry)
		if err != nil {
			return nil, err
		}
		return n, nil
	}
	ip := net.ParseIP(entry)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP or CIDR")
	}
	bits := 128
	if ip.To4() != nil {
		bits = 32
	}
	_, n, err := net.ParseCIDR(fmt.Sprintf("%s/%d", ip.String(), bits))
	if err != nil {
		return nil, err
	}
	return n, nil
}

// IsTrustedProxy reports whether remoteAddr (host:port or bare IP) is in TRUSTED_PROXIES.
// Returns false when no proxies are configured (fail closed for AUTH_HEADER).
func (c *Config) IsTrustedProxy(remoteAddr string) bool {
	if c == nil || len(c.TrustedProxyNets) == 0 {
		return false
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	host = strings.Trim(host, "[]")
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, n := range c.TrustedProxyNets {
		if n != nil && n.Contains(ip) {
			return true
		}
	}
	return false
}
