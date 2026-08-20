// Package routes_test contains security regression tests.
//
// These tests verify that critical security controls are active and functioning.
// Each test targets a specific security mechanism documented in SECURITY.md.
// If any test fails, it indicates a security regression that must be fixed
// before the change can be merged.
package routes_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"capacitarr/internal/testutil"
)

// ─── Security Header Tests ──────────────────────────────────────────────────

// TestSecurityHeaders_CSP verifies that Content-Security-Policy is set on all
// responses with restrictive directives that prevent XSS and resource injection.
// The script-src directive uses a per-request nonce to allow only the server's
// own inline scripts while blocking any injected inline scripts.
func TestSecurityHeaders_CSP(t *testing.T) {
	e := testutil.SetupTestServer(t, testutil.SetupTestDB(t))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Fatal("Content-Security-Policy header is missing")
	}

	// Verify critical CSP directives are present
	requiredDirectives := []string{
		"default-src 'self'",
		"frame-ancestors 'none'",
		"base-uri 'self'",
		"form-action 'self'",
	}
	for _, directive := range requiredDirectives {
		if !strings.Contains(csp, directive) {
			t.Errorf("CSP missing directive %q, got: %s", directive, csp)
		}
	}

	// script-src must contain 'self' and a nonce
	if !strings.Contains(csp, "script-src 'self' 'nonce-") {
		t.Errorf("CSP script-src missing nonce, got: %s", csp)
	}
}

// TestSecurityHeaders_CSP_NonceChangesPerRequest verifies that two requests
// produce different CSP nonces, ensuring the nonce is cryptographically random.
func TestSecurityHeaders_CSP_NonceChangesPerRequest(t *testing.T) {
	e := testutil.SetupTestServer(t, testutil.SetupTestDB(t))

	req1 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/health", nil)
	rec1 := httptest.NewRecorder()
	e.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/health", nil)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)

	csp1 := rec1.Header().Get("Content-Security-Policy")
	csp2 := rec2.Header().Get("Content-Security-Policy")

	if csp1 == csp2 {
		t.Error("CSP nonce must be different between requests, but both are identical")
	}
}

// TestSecurityHeaders_AllPresent verifies that all security headers documented
// in SECURITY.md § Network Security are present on every response.
func TestSecurityHeaders_AllPresent(t *testing.T) {
	e := testutil.SetupTestServer(t, testutil.SetupTestDB(t))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	expectedHeaders := map[string]string{
		"X-Content-Type-Options":            "nosniff",
		"X-Frame-Options":                   "DENY",
		"Referrer-Policy":                   "strict-origin-when-cross-origin",
		"Permissions-Policy":                "camera=(), microphone=(), geolocation=()",
		"Cross-Origin-Opener-Policy":        "same-origin",
		"Cross-Origin-Resource-Policy":      "same-origin",
		"X-Permitted-Cross-Domain-Policies": "none",
	}

	for header, expected := range expectedHeaders {
		actual := rec.Header().Get(header)
		if actual != expected {
			t.Errorf("Header %s = %q, want %q", header, actual, expected)
		}
	}
}

// TestSecurityHeaders_NoHSTS_WithoutSecureCookies verifies that HSTS is NOT
// sent when SECURE_COOKIES is not enabled (to prevent HSTS preload on HTTP).
func TestSecurityHeaders_NoHSTS_WithoutSecureCookies(t *testing.T) {
	e := testutil.SetupTestServer(t, testutil.SetupTestDB(t))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	hsts := rec.Header().Get("Strict-Transport-Security")
	if hsts != "" {
		t.Errorf("HSTS should not be set when SECURE_COOKIES is not enabled, got: %s", hsts)
	}
}

// ─── Authentication Security Tests ──────────────────────────────────────────

// TestLogin_ReturnsHttpOnlyCookie verifies that the JWT cookie is HttpOnly
// (prevents JavaScript access, mitigates XSS token theft).
func TestLogin_ReturnsHttpOnlyCookie(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Bootstrap a user via login
	body := `{"username":"Firefly","password":"Serenity123!"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Login failed: %d %s", rec.Code, rec.Body.String())
	}

	// Check that the JWT cookie is HttpOnly
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "jwt" {
			if !cookie.HttpOnly {
				t.Error("JWT cookie must be HttpOnly to prevent XSS token theft")
			}
			if cookie.SameSite != http.SameSiteLaxMode {
				t.Errorf("JWT cookie SameSite = %v, want Lax", cookie.SameSite)
			}
			return
		}
	}
	t.Error("JWT cookie not found in login response")
}

// TestLogin_ReturnsNonHttpOnlyAuthenticatedCookie verifies that the
// "authenticated" indicator cookie is intentionally NOT HttpOnly (the SPA
// needs to read it), but IS SameSite=Lax.
func TestLogin_ReturnsNonHttpOnlyAuthenticatedCookie(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	body := `{"username":"Firefly","password":"Serenity123!"}`
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Login failed: %d %s", rec.Code, rec.Body.String())
	}

	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "authenticated" {
			if cookie.HttpOnly {
				t.Error("authenticated cookie should NOT be HttpOnly (SPA needs to read it)")
			}
			if cookie.Value != "true" {
				t.Errorf("authenticated cookie value = %q, want %q", cookie.Value, "true")
			}
			return
		}
	}
	t.Error("authenticated cookie not found in login response")
}

// ─── API Key Masking Tests ──────────────────────────────────────────────────

// TestIntegrationAPIKey_MaskedInResponse verifies that integration API keys
// are masked in API responses (only last 4 characters visible).
func TestIntegrationAPIKey_MaskedInResponse(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Create an integration with a known API key
	createBody := `{"type":"sonarr","name":"Firefly","url":"http://localhost:8989","apiKey":"abcdef1234567890abcdef1234567890","enabled":true}` //nolint:gosec // G101: test-only fixture API key, not a real credential
	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/integrations", strings.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
		t.Fatalf("Create integration failed: %d %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("Failed to parse create response: %v", err)
	}
	if created.ID == 0 {
		t.Fatal("expected non-zero integration ID from create")
	}

	// Fetch the integration list
	req = testutil.AuthenticatedRequest(t, http.MethodGet, "/api/integrations", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	responseBody := rec.Body.String()

	// The full API key must NOT appear in the response
	if strings.Contains(responseBody, "abcdef1234567890abcdef1234567890") {
		t.Error("SECURITY: Full integration API key found in list API response — keys must be masked")
	}

	req = testutil.AuthenticatedRequest(t, http.MethodGet, fmt.Sprintf("/api/integrations/%d", created.ID), nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if strings.Contains(rec.Body.String(), "abcdef1234567890abcdef1234567890") {
		t.Error("SECURITY: Full integration API key found in GET /integrations/:id response — keys must be masked")
	}

	req = testutil.AuthenticatedRequest(t, http.MethodPut, fmt.Sprintf("/api/integrations/%d", created.ID), strings.NewReader(`{"name":"Firefly Updated"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Update integration failed: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "abcdef1234567890abcdef1234567890") {
		t.Error("SECURITY: Full integration API key found in PUT /integrations/:id response — keys must be masked")
	}
}

// ─── AUTH_HEADER Warning Tests ──────────────────────────────────────────────

// TestAuthStatus_NoAuthHeaderWarning_WhenNotConfigured verifies that no
// authHeaderWarning is returned when AUTH_HEADER is not set.
func TestAuthStatus_NoAuthHeaderWarning_WhenNotConfigured(t *testing.T) {
	e := testutil.SetupTestServer(t, testutil.SetupTestDB(t))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if _, hasWarning := resp["authHeaderWarning"]; hasWarning {
		t.Error("authHeaderWarning should not be present when AUTH_HEADER is not configured")
	}
}
