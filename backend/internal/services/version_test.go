package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"capacitarr/internal/db"
)

// mockPreferencesReader satisfies the PreferencesReader interface for testing.
type mockPreferencesReader struct {
	pref db.PreferenceSet
	err  error
}

func (m *mockPreferencesReader) GetPreferences() (db.PreferenceSet, error) {
	return m.pref, m.err
}

// mockGitHubServer creates a test HTTP server that returns canned release JSON.
func mockGitHubServer(t *testing.T, responseJSON string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(json.RawMessage(responseJSON))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// ---------- CheckForUpdate ----------

func TestVersionService_CheckForUpdate_Disabled(t *testing.T) {
	mock := &mockPreferencesReader{
		pref: db.PreferenceSet{CheckForUpdates: false},
	}

	svc := NewVersionService(mock, nil, "v1.0.0", "http://unused")

	result, err := svc.CheckForUpdate()
	if err != nil {
		t.Fatalf("CheckForUpdate returned error: %v", err)
	}
	if result.Current != "v1.0.0" {
		t.Errorf("Expected current 'v1.0.0', got %q", result.Current)
	}
	if result.UpdateAvailable {
		t.Error("Expected updateAvailable to be false when checks are disabled")
	}
	if result.Latest != "" {
		t.Errorf("Expected empty latest when checks are disabled, got %q", result.Latest)
	}
}

func TestVersionService_CheckForUpdate_Cached(t *testing.T) {
	mock := &mockPreferencesReader{
		pref: db.PreferenceSet{CheckForUpdates: true},
	}

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"tag_name":"v2.0.0"}]`))
	}))
	t.Cleanup(srv.Close)

	svc := NewVersionService(mock, nil, "v1.0.0", srv.URL)

	// First call — should fetch from server
	result1, err := svc.CheckForUpdate()
	if err != nil {
		t.Fatalf("First CheckForUpdate returned error: %v", err)
	}
	if result1.Latest != "v2.0.0" {
		t.Errorf("Expected latest 'v2.0.0', got %q", result1.Latest)
	}
	if !result1.UpdateAvailable {
		t.Error("Expected updateAvailable to be true")
	}
	if callCount != 1 {
		t.Fatalf("Expected 1 server call, got %d", callCount)
	}

	// Second call — should use cache (no additional server call)
	result2, err := svc.CheckForUpdate()
	if err != nil {
		t.Fatalf("Second CheckForUpdate returned error: %v", err)
	}
	if result2.Latest != "v2.0.0" {
		t.Errorf("Expected cached latest 'v2.0.0', got %q", result2.Latest)
	}
	if callCount != 1 {
		t.Errorf("Expected still 1 server call (cached), got %d", callCount)
	}
}

func TestVersionService_CheckForUpdate_Enabled(t *testing.T) {
	mock := &mockPreferencesReader{
		pref: db.PreferenceSet{CheckForUpdates: true},
	}
	url := mockGitHubServer(t, `[{"tag_name":"v3.0.0"}]`)

	svc := NewVersionService(mock, nil, "v1.0.0", url)

	result, err := svc.CheckForUpdate()
	if err != nil {
		t.Fatalf("CheckForUpdate returned error: %v", err)
	}
	if result.Current != "v1.0.0" {
		t.Errorf("Expected current 'v1.0.0', got %q", result.Current)
	}
	if result.Latest != "v3.0.0" {
		t.Errorf("Expected latest 'v3.0.0', got %q", result.Latest)
	}
	if !result.UpdateAvailable {
		t.Error("Expected updateAvailable to be true")
	}
	if result.ReleaseURL == "" {
		t.Error("Expected non-empty releaseUrl")
	}
}

// ---------- ForceCheck ----------

func TestVersionService_ForceCheck(t *testing.T) {
	mock := &mockPreferencesReader{
		pref: db.PreferenceSet{CheckForUpdates: true},
	}

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"tag_name":"v2.0.0"}]`))
	}))
	t.Cleanup(srv.Close)

	svc := NewVersionService(mock, nil, "v1.0.0", srv.URL)

	// Warm the cache
	_, err := svc.CheckForUpdate()
	if err != nil {
		t.Fatalf("CheckForUpdate returned error: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("Expected 1 server call, got %d", callCount)
	}

	// ForceCheck should bypass cache
	result, err := svc.ForceCheck()
	if err != nil {
		t.Fatalf("ForceCheck returned error: %v", err)
	}
	if callCount != 2 {
		t.Errorf("Expected 2 server calls after ForceCheck, got %d", callCount)
	}
	if result.Latest != "v2.0.0" {
		t.Errorf("Expected latest 'v2.0.0', got %q", result.Latest)
	}
}

func TestVersionService_ForceCheck_Disabled(t *testing.T) {
	mock := &mockPreferencesReader{
		pref: db.PreferenceSet{CheckForUpdates: false},
	}

	svc := NewVersionService(mock, nil, "v1.0.0", "http://unused")

	result, err := svc.ForceCheck()
	if err != nil {
		t.Fatalf("ForceCheck returned error: %v", err)
	}
	if result.UpdateAvailable {
		t.Error("Expected updateAvailable to be false when checks are disabled")
	}
}

// ---------- CompareSemver ----------

func TestVersionService_CompareSemver(t *testing.T) {
	tests := []struct {
		name     string
		a, b     string
		expected int
	}{
		{"equal versions", "1.0.0", "1.0.0", 0},
		{"patch higher", "1.0.1", "1.0.0", 1},
		{"patch lower", "1.0.0", "1.0.1", -1},
		{"stable > prerelease", "1.0.0", "1.0.0-rc.1", 1},
		{"prerelease higher", "1.0.0-rc.3", "1.0.0-rc.1", 1},
		{"major higher", "2.0.0", "1.9.9", 1},
		{"with v prefix", "v1.0.0", "v1.0.0", 0},
		{"mixed v prefix", "v1.0.1", "1.0.0", 1},
		{"prerelease < stable", "1.0.0-rc.1", "1.0.0", -1},
		{"minor higher", "1.1.0", "1.0.9", 1},
		{"build metadata ignored equal", "1.0.0+build.1", "1.0.0", 0},
		{"build metadata ignored both", "1.0.0+build.1", "1.0.0+build.2", 0},
		{"prerelease with build metadata", "1.0.0-rc.1+build.1", "1.0.0", -1},
		{"major higher with build metadata", "2.0.0+meta", "1.0.0", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareSemver(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("CompareSemver(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

// ---------- ResetCache ----------

func TestVersionService_ResetCache(t *testing.T) {
	mock := &mockPreferencesReader{
		pref: db.PreferenceSet{CheckForUpdates: true},
	}

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"tag_name":"v2.0.0"}]`))
	}))
	t.Cleanup(srv.Close)

	svc := NewVersionService(mock, nil, "v1.0.0", srv.URL)

	// Warm cache
	_, _ = svc.CheckForUpdate()
	if callCount != 1 {
		t.Fatalf("Expected 1 server call, got %d", callCount)
	}

	// Reset cache
	svc.ResetCache()

	// Next call should fetch again
	_, _ = svc.CheckForUpdate()
	if callCount != 2 {
		t.Errorf("Expected 2 server calls after cache reset, got %d", callCount)
	}
}
