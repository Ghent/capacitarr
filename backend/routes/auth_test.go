package routes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"capacitarr/internal/testutil"
)

const testLoginBody = `{"username":"admin","password":"password123"}`

func TestAuthStatus_NoUser(t *testing.T) {
	e := testutil.SetupTestServer(t, testutil.SetupTestDB(t))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/auth/status", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp["initialized"] != false {
		t.Errorf("Expected initialized=false when no user exists, got %v", resp["initialized"])
	}
}

func TestAuthStatus_AfterUserCreated(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Bootstrap a user via login
	body := testLoginBody
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Bootstrap failed: %d: %s", rec.Code, rec.Body.String())
	}

	// Now check status — should be initialized
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/auth/status", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp["initialized"] != true {
		t.Errorf("Expected initialized=true after user created, got %v", resp["initialized"])
	}
}

func TestLoginHandler_FirstUserBootstrap(t *testing.T) {
	e := testutil.SetupTestServer(t, testutil.SetupTestDB(t))

	body := testLoginBody
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 for bootstrap login, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp["token"] == "" {
		t.Error("Expected non-empty token in response")
	}
	if resp["message"] != "success" {
		t.Errorf("Expected message 'success', got %q", resp["message"])
	}

	// Verify JWT cookie was set
	cookies := rec.Result().Cookies()
	var foundJWT bool
	for _, c := range cookies {
		if c.Name == "jwt" {
			foundJWT = true
			if c.Value == "" {
				t.Error("JWT cookie value is empty")
			}
		}
	}
	if !foundJWT {
		t.Error("Expected jwt cookie to be set")
	}
}

func TestLoginHandler_SuccessfulLogin(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Bootstrap first user
	body := testLoginBody
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Bootstrap failed: %d", rec.Code)
	}

	// Now login again
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 for valid login, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLoginHandler_WrongPassword(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Bootstrap first user
	body := testLoginBody
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Try with wrong password
	body = `{"username":"admin","password":"wrongpassword"}`
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 for wrong password, got %d", rec.Code)
	}
}

func TestLoginHandler_MissingFields(t *testing.T) {
	e := testutil.SetupTestServer(t, testutil.SetupTestDB(t))

	tests := []struct {
		name string
		body string
	}{
		{"missing username", `{"password":"test"}`},
		{"missing password", `{"username":"test"}`},
		{"empty username", `{"username":"","password":"test"}`},
		{"empty password", `{"username":"test","password":""}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/auth/login", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("Expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestPasswordChange(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Bootstrap user
	body := testLoginBody
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Bootstrap failed: %d", rec.Code)
	}

	// Extract token
	var loginResp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("Failed to parse login response: %v", err)
	}

	// Change password
	body = `{"currentPassword":"password123","newPassword":"newpassword123"}`
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/auth/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginResp["token"])
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify new password works
	body = `{"username":"admin","password":"newpassword123"}`
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 for new password login, got %d", rec.Code)
	}
}

func TestPasswordChange_ShortPassword(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Bootstrap user
	body := testLoginBody
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var loginResp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Try short new password
	body = `{"currentPassword":"password123","newPassword":"short"}`
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/auth/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginResp["token"])
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for short password, got %d", rec.Code)
	}
}

func TestAPIKeyGeneration(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e := testutil.SetupTestServer(t, database)

	// Bootstrap user
	body := testLoginBody
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var loginResp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Generate API key
	req = httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/auth/apikey", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp["token"])
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var keyResp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &keyResp); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if keyResp["api_key"] == "" {
		t.Error("Expected non-empty API key")
	}
	if len(keyResp["api_key"]) != 64 { // 32 bytes = 64 hex chars
		t.Errorf("Expected 64-char hex API key, got length %d", len(keyResp["api_key"]))
	}

	// Check API key status
	req = httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/auth/apikey", nil)
	req.Header.Set("Authorization", "Bearer "+loginResp["token"])
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", rec.Code)
	}

	var statusResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &statusResp); err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	if statusResp["has_key"] != true {
		t.Errorf("Expected has_key=true, got %v", statusResp["has_key"])
	}
}
