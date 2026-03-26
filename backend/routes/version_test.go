package routes_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"capacitarr/internal/db"
	"capacitarr/internal/services"
	"capacitarr/internal/testutil"
)

// mockGitLabReleases spins up a local HTTP server that returns a canned
// GitLab-style releases response, then configures the VersionService to
// use it. The server is closed when the test finishes.
func mockGitLabReleases(t *testing.T, reg *services.Registry, responseJSON string) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(json.RawMessage(responseJSON))
	}))
	t.Cleanup(srv.Close)

	reg.Version.SetReleasesURL(srv.URL)
}

// ---------- GET /api/version/check ----------

func TestVersionCheck_DisabledByPreference(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e, reg := testutil.SetupTestServerWithRegistry(t, database)
	reg.Version.ResetCache()

	// Disable update checks in preferences
	if err := database.Model(&db.PreferenceSet{}).Where("id = 1").Update("check_for_updates", false).Error; err != nil {
		t.Fatalf("Failed to update preferences: %v", err)
	}

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/version/check", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Current         string `json:"current"`
		Latest          string `json:"latest"`
		UpdateAvailable bool   `json:"updateAvailable"`
		ReleaseURL      string `json:"releaseUrl"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.UpdateAvailable {
		t.Error("Expected updateAvailable to be false when checks are disabled")
	}
	if resp.Latest != "" {
		t.Errorf("Expected empty latest version when checks are disabled, got %q", resp.Latest)
	}
	if resp.ReleaseURL != "" {
		t.Errorf("Expected empty releaseUrl when checks are disabled, got %q", resp.ReleaseURL)
	}
}

func TestVersionCheck_ReturnsCurrentVersion(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e, reg := testutil.SetupTestServerWithRegistry(t, database)
	reg.Version.ResetCache()
	mockGitLabReleases(t, reg, `[{"tag_name":"v1.2.3"}]`)

	req := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/version/check", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Current         string `json:"current"`
		Latest          string `json:"latest"`
		UpdateAvailable bool   `json:"updateAvailable"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Current != "v0.0.0-test" {
		t.Errorf("Expected current version 'v0.0.0-test', got %q", resp.Current)
	}
	if resp.Latest != "v1.2.3" {
		t.Errorf("Expected latest version 'v1.2.3', got %q", resp.Latest)
	}
	if !resp.UpdateAvailable {
		t.Error("Expected updateAvailable to be true when latest > current")
	}
}

// ---------- POST /api/version/check (manual "check now") ----------

func TestVersionCheckNow_BypassesCache(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e, reg := testutil.SetupTestServerWithRegistry(t, database)
	reg.Version.ResetCache()
	mockGitLabReleases(t, reg, `[{"tag_name":"v2.0.0"}]`)

	// First: warm the cache via GET (returns v2.0.0)
	reqGet := testutil.AuthenticatedRequest(t, http.MethodGet, "/api/version/check", nil)
	recGet := httptest.NewRecorder()
	e.ServeHTTP(recGet, reqGet)

	if recGet.Code != http.StatusOK {
		t.Fatalf("GET Expected 200, got %d: %s", recGet.Code, recGet.Body.String())
	}

	var getResp struct {
		Latest string `json:"latest"`
	}
	if err := json.Unmarshal(recGet.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("Failed to parse GET response: %v", err)
	}
	if getResp.Latest != "v2.0.0" {
		t.Fatalf("Expected GET latest 'v2.0.0', got %q", getResp.Latest)
	}

	// Second: POST should bypass cache and fetch fresh
	reqPost := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/version/check", nil)
	recPost := httptest.NewRecorder()
	e.ServeHTTP(recPost, reqPost)

	if recPost.Code != http.StatusOK {
		t.Fatalf("POST Expected 200, got %d: %s", recPost.Code, recPost.Body.String())
	}

	var postResp struct {
		Current         string `json:"current"`
		Latest          string `json:"latest"`
		UpdateAvailable bool   `json:"updateAvailable"`
	}
	if err := json.Unmarshal(recPost.Body.Bytes(), &postResp); err != nil {
		t.Fatalf("Failed to parse POST response: %v", err)
	}
	if postResp.Current != "v0.0.0-test" {
		t.Errorf("Expected current 'v0.0.0-test', got %q", postResp.Current)
	}
	if postResp.Latest != "v2.0.0" {
		t.Errorf("Expected latest 'v2.0.0', got %q", postResp.Latest)
	}
	if !postResp.UpdateAvailable {
		t.Error("Expected updateAvailable to be true")
	}
}

func TestVersionCheckNow_DisabledByPreference(t *testing.T) {
	database := testutil.SetupTestDB(t)
	e, reg := testutil.SetupTestServerWithRegistry(t, database)
	reg.Version.ResetCache()

	// Disable update checks
	if err := database.Model(&db.PreferenceSet{}).Where("id = 1").Update("check_for_updates", false).Error; err != nil {
		t.Fatalf("Failed to update preferences: %v", err)
	}

	req := testutil.AuthenticatedRequest(t, http.MethodPost, "/api/version/check", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		UpdateAvailable bool   `json:"updateAvailable"`
		Latest          string `json:"latest"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp.UpdateAvailable {
		t.Error("Expected updateAvailable to be false when checks are disabled")
	}
	if resp.Latest != "" {
		t.Errorf("Expected empty latest when checks are disabled, got %q", resp.Latest)
	}
}
