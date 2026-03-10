package routes_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"capacitarr/internal/config"
	"capacitarr/internal/db"
	"capacitarr/internal/events"
	"capacitarr/internal/services"
	"capacitarr/internal/testutil"
	"capacitarr/routes"
)

// setupAuthTest creates a test Echo app with a single protected endpoint
// and returns the Echo instance and database.
func setupAuthTest(t *testing.T, cfg *config.Config) *echo.Echo {
	t.Helper()
	database := testutil.SetupTestDB(t)

	// Create a test user with known password
	hashed, err := bcrypt.GenerateFromPassword([]byte("testpassword"), 10)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}
	user := db.AuthConfig{
		Username: "testadmin",
		Password: string(hashed),
		APIKey:   services.HashAPIKey("valid-api-key-12345"),
	}
	if err := database.Create(&user).Error; err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	bus := events.NewEventBus()
	reg := services.NewRegistry(database, bus, cfg)

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	g := e.Group("/api")
	g.Use(routes.RequireAuth(reg))
	g.GET("/protected", func(c echo.Context) error {
		username := c.Get("user").(string)
		return c.JSON(http.StatusOK, map[string]string{"user": username})
	})

	return e
}

func TestMiddleware_ValidJWT(t *testing.T) {
	cfg := testutil.TestConfig()
	e := setupAuthTest(t, cfg)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "testadmin",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMiddleware_ExpiredJWT(t *testing.T) {
	cfg := testutil.TestConfig()
	e := setupAuthTest(t, cfg)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "testadmin",
		"exp": time.Now().Add(-1 * time.Hour).Unix(), // expired
	})
	tokenString, err := token.SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for expired JWT, got %d", rec.Code)
	}
}

func TestMiddleware_MalformedJWT(t *testing.T) {
	cfg := testutil.TestConfig()
	e := setupAuthTest(t, cfg)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer not.a.valid.jwt.token")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for malformed JWT, got %d", rec.Code)
	}
}

func TestMiddleware_WrongSigningKey(t *testing.T) {
	cfg := testutil.TestConfig()
	e := setupAuthTest(t, cfg)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "testadmin",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte("wrong-secret")) // nosemgrep: go.jwt-go.security.jwt.hardcoded-jwt-key — test-only: intentionally wrong key to verify rejection
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for wrong signing key, got %d", rec.Code)
	}
}

func TestMiddleware_MissingAuth(t *testing.T) {
	cfg := testutil.TestConfig()
	e := setupAuthTest(t, cfg)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/protected", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for missing auth, got %d", rec.Code)
	}
}

func TestMiddleware_ValidAPIKey_AuthorizationHeader(t *testing.T) {
	cfg := testutil.TestConfig()
	e := setupAuthTest(t, cfg)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/protected", nil)
	req.Header.Set("Authorization", "ApiKey valid-api-key-12345")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 for valid API key, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMiddleware_InvalidAPIKey(t *testing.T) {
	cfg := testutil.TestConfig()
	e := setupAuthTest(t, cfg)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/protected", nil)
	req.Header.Set("Authorization", "ApiKey invalid-key")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for invalid API key, got %d", rec.Code)
	}
}

func TestMiddleware_ValidAPIKey_XApiKeyHeader(t *testing.T) {
	cfg := testutil.TestConfig()
	e := setupAuthTest(t, cfg)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/protected", nil)
	req.Header.Set("X-Api-Key", "valid-api-key-12345")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 for X-Api-Key header, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMiddleware_ValidAPIKey_QueryParam(t *testing.T) {
	cfg := testutil.TestConfig()
	e := setupAuthTest(t, cfg)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/protected?apikey=valid-api-key-12345", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 for apikey query param, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMiddleware_JWTCookie(t *testing.T) {
	cfg := testutil.TestConfig()
	e := setupAuthTest(t, cfg)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "testadmin",
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		t.Fatalf("Failed to sign token: %v", err)
	}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/protected", nil)
	req.AddCookie(&http.Cookie{Name: "jwt", Value: tokenString}) // nosemgrep — test request: HttpOnly/Secure are set by the server when issuing cookies, not when sending them in test requests
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 for JWT cookie, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMiddleware_ProxyAuthHeader(t *testing.T) {
	cfg := testutil.TestConfig()
	cfg.AuthHeader = "Remote-User"
	e := setupAuthTest(t, cfg)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/protected", nil)
	req.Header.Set("Remote-User", "proxy-user")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 for proxy auth, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMiddleware_ProxyAuthHeader_Empty(t *testing.T) {
	cfg := testutil.TestConfig()
	cfg.AuthHeader = "Remote-User"
	e := setupAuthTest(t, cfg)

	// Empty header value should fall through to other auth methods
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/protected", nil)
	req.Header.Set("Remote-User", "")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for empty proxy auth header, got %d", rec.Code)
	}
}

func TestMiddleware_ProxyAuthDisabled(t *testing.T) {
	cfg := testutil.TestConfig()
	cfg.AuthHeader = "" // proxy auth not configured
	e := setupAuthTest(t, cfg)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/protected", nil)
	req.Header.Set("Remote-User", "spoofed-user")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// Without AuthHeader configured, the header should be ignored
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 when proxy auth is disabled, got %d", rec.Code)
	}
}

func TestMiddleware_InvalidAuthScheme(t *testing.T) {
	cfg := testutil.TestConfig()
	e := setupAuthTest(t, cfg)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/protected", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for Basic auth scheme, got %d", rec.Code)
	}
}

func TestMiddleware_MalformedAuthorizationHeader(t *testing.T) {
	cfg := testutil.TestConfig()
	e := setupAuthTest(t, cfg)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/protected", nil)
	req.Header.Set("Authorization", "nospaceatall")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for malformed Authorization header, got %d", rec.Code)
	}
}
