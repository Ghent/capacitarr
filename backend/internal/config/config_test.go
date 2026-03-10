package config

import (
	"io"
	"log"
	"log/slog"
	"testing"
)

// silenceLogs suppresses slog and standard log output for the duration of
// the test. config.Load() emits WARN-level messages for missing JWT_SECRET
// and AUTH_HEADER configuration which are expected during tests but pollute
// test output. See also testutil.SilenceLogs for the shared version.
func silenceLogs(t *testing.T) {
	t.Helper()
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	prevOut := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(prevOut) })
}

func TestLoad_Defaults(t *testing.T) {
	silenceLogs(t)
	// Clear all env vars that Load() reads by setting them to empty
	for _, key := range []string{"PORT", "BASE_URL", "DB_PATH", "DEBUG", "JWT_SECRET", "CORS_ORIGINS", "SECURE_COOKIES", "AUTH_HEADER"} {
		t.Setenv(key, "")
	}

	cfg := Load()

	if cfg.Port != "2187" {
		t.Errorf("expected default port 2187, got %s", cfg.Port)
	}
	if cfg.BaseURL != "/" {
		t.Errorf("expected default baseURL /, got %s", cfg.BaseURL)
	}
	if cfg.Database != "/config/capacitarr.db" {
		t.Errorf("expected default db path /config/capacitarr.db, got %s", cfg.Database)
	}
	if cfg.Debug {
		t.Error("expected debug=false by default")
	}
	if cfg.SecureCookies {
		t.Error("expected secureCookies=false by default")
	}
	if cfg.AuthHeader != "" {
		t.Errorf("expected empty authHeader, got %s", cfg.AuthHeader)
	}
	// JWT secret should be auto-generated (non-empty, not the debug value)
	if cfg.JWTSecret == "" {
		t.Error("expected auto-generated JWT secret, got empty string")
	}
	if cfg.JWTSecret == "development_secret_do_not_use_in_production" {
		t.Error("expected random JWT secret in non-debug mode, got debug value")
	}
}

func TestLoad_CustomPort(t *testing.T) {
	silenceLogs(t)
	t.Setenv("PORT", "8080")

	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("expected port 8080, got %s", cfg.Port)
	}
}

func TestLoad_BaseURL_Normalization(t *testing.T) {
	silenceLogs(t)
	tests := []struct {
		input    string
		expected string
	}{
		{"", "/"},
		{"/", "/"},
		{"capacitarr", "/capacitarr/"},
		{"/capacitarr", "/capacitarr/"},
		{"/capacitarr/", "/capacitarr/"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Setenv("BASE_URL", tt.input)
			cfg := Load()
			if cfg.BaseURL != tt.expected {
				t.Errorf("BaseURL(%q) = %q, want %q", tt.input, cfg.BaseURL, tt.expected)
			}
		})
	}
}

func TestLoad_JWTSecret_Debug(t *testing.T) {
	silenceLogs(t)
	t.Setenv("DEBUG", "true")
	t.Setenv("JWT_SECRET", "")

	cfg := Load()

	if cfg.JWTSecret != "development_secret_do_not_use_in_production" {
		t.Errorf("expected debug JWT secret, got %s", cfg.JWTSecret)
	}
}

func TestLoad_JWTSecret_Explicit(t *testing.T) {
	silenceLogs(t)
	t.Setenv("JWT_SECRET", "my-custom-secret-key")

	cfg := Load()

	if cfg.JWTSecret != "my-custom-secret-key" {
		t.Errorf("expected custom JWT secret, got %s", cfg.JWTSecret)
	}
}

func TestLoad_CORSOrigins_Parsing(t *testing.T) {
	silenceLogs(t)
	t.Setenv("CORS_ORIGINS", "http://localhost:3000, https://app.example.com , http://other.com")

	cfg := Load()

	if len(cfg.CORSOrigins) != 3 {
		t.Fatalf("expected 3 CORS origins, got %d: %v", len(cfg.CORSOrigins), cfg.CORSOrigins)
	}
	if cfg.CORSOrigins[0] != "http://localhost:3000" {
		t.Errorf("expected first origin http://localhost:3000, got %s", cfg.CORSOrigins[0])
	}
	if cfg.CORSOrigins[1] != "https://app.example.com" {
		t.Errorf("expected second origin trimmed, got %s", cfg.CORSOrigins[1])
	}
}

func TestLoad_CORSOrigins_DebugDefault(t *testing.T) {
	silenceLogs(t)
	t.Setenv("DEBUG", "true")
	t.Setenv("CORS_ORIGINS", "")

	cfg := Load()

	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "*" {
		t.Errorf("expected debug CORS default [*], got %v", cfg.CORSOrigins)
	}
}

func TestLoad_AuthHeader(t *testing.T) {
	silenceLogs(t)
	t.Setenv("AUTH_HEADER", "Remote-User")

	cfg := Load()

	if cfg.AuthHeader != "Remote-User" {
		t.Errorf("expected AuthHeader Remote-User, got %s", cfg.AuthHeader)
	}
}

func TestLoad_SecureCookies(t *testing.T) {
	silenceLogs(t)
	t.Setenv("SECURE_COOKIES", "true")

	cfg := Load()

	if !cfg.SecureCookies {
		t.Error("expected secureCookies=true")
	}
}
