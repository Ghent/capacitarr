package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"capacitarr/internal/db"
	"capacitarr/internal/events"
)

// DefaultGitHubReleasesURL is the GitHub API endpoint for Capacitarr releases.
const DefaultGitHubReleasesURL = "https://api.github.com/repos/Ghent/capacitarr/releases?per_page=1"

// PreferencesReader provides read access to application preferences.
// Satisfied by SettingsService.
type PreferencesReader interface {
	GetPreferences() (db.PreferenceSet, error)
}

// VersionCheckResult holds the cached update check response.
type VersionCheckResult struct {
	Current         string    `json:"current"`
	Latest          string    `json:"latest"`
	UpdateAvailable bool      `json:"updateAvailable"`
	ReleaseURL      string    `json:"releaseUrl"`
	CheckedAt       time.Time `json:"checkedAt"`
}

// VersionService manages update checks against the GitHub releases API.
type VersionService struct {
	preferences         PreferencesReader
	bus                 *events.EventBus
	appVersion          string
	releasesURL         string
	cache               *VersionCheckResult
	mu                  sync.Mutex
	cacheTTL            time.Duration
	lastNotifiedVersion string // tracks which version we've already published UpdateAvailableEvent for
}

// NewVersionService creates a new VersionService.
// releasesURL is the GitHub releases API URL; pass DefaultGitHubReleasesURL for production.
func NewVersionService(preferences PreferencesReader, bus *events.EventBus, appVersion, releasesURL string) *VersionService {
	return &VersionService{
		preferences: preferences,
		bus:         bus,
		appVersion:  appVersion,
		releasesURL: releasesURL,
		cacheTTL:    6 * time.Hour,
	}
}

// SetAppVersion sets the application version. Called by main.go after Registry
// construction when the version string is known.
func (s *VersionService) SetAppVersion(v string) {
	s.mu.Lock()
	s.appVersion = v
	s.mu.Unlock()
}

// SetReleasesURL overrides the GitHub releases URL. Intended for use in tests only.
func (s *VersionService) SetReleasesURL(url string) {
	s.mu.Lock()
	s.releasesURL = url
	s.mu.Unlock()
}

// CheckForUpdate loads preferences, checks if updates are enabled, and returns
// a cached result if fresh or fetches a new one from the GitHub releases API.
func (s *VersionService) CheckForUpdate() (*VersionCheckResult, error) {
	// Load preferences to check if update checks are enabled
	pref, err := s.preferences.GetPreferences()
	if err != nil {
		slog.Warn("Failed to load preferences for version check", "component", "version", "error", err)
		return &VersionCheckResult{Current: s.appVersion}, nil
	}

	if !pref.CheckForUpdates {
		return &VersionCheckResult{Current: s.appVersion}, nil
	}

	// Check cache
	s.mu.Lock()
	if s.cache != nil && time.Since(s.cache.CheckedAt) < s.cacheTTL {
		result := *s.cache
		s.mu.Unlock()
		return &result, nil
	}
	s.mu.Unlock()

	// Fetch latest release from GitHub
	result := s.fetchLatestRelease()

	// Cache the result
	s.mu.Lock()
	s.cache = &result
	s.mu.Unlock()

	return &result, nil
}

// ForceCheck bypasses the cache and always fetches fresh data from the
// GitHub releases API.
func (s *VersionService) ForceCheck() (*VersionCheckResult, error) {
	// Load preferences to check if update checks are enabled
	pref, err := s.preferences.GetPreferences()
	if err != nil {
		slog.Warn("Failed to load preferences for version check", "component", "version", "error", err)
		return &VersionCheckResult{Current: s.appVersion}, nil
	}

	if !pref.CheckForUpdates {
		return &VersionCheckResult{Current: s.appVersion}, nil
	}

	// Bypass cache — always fetch fresh
	result := s.fetchLatestRelease()

	// Update the cache with fresh result
	s.mu.Lock()
	s.cache = &result
	s.mu.Unlock()

	return &result, nil
}

// ResetCache clears the cached version check result. Intended for testing.
func (s *VersionService) ResetCache() {
	s.mu.Lock()
	s.cache = nil
	s.mu.Unlock()
}

// fetchLatestRelease queries the GitHub releases API and returns a VersionCheckResult.
// On any failure it returns a graceful degradation response with only the current version.
func (s *VersionService) fetchLatestRelease() VersionCheckResult {
	fallback := VersionCheckResult{
		Current:   s.appVersion,
		CheckedAt: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.releasesURL, nil)
	if err != nil {
		slog.Warn("Failed to create request for version check", "component", "version", "error", err)
		return fallback
	}
	req.Header.Set("User-Agent", fmt.Sprintf("Capacitarr/%s", s.appVersion))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req) //nolint:gosec // URL is set at construction time (DefaultGitHubReleasesURL or test URL), not user-tainted
	if err != nil {
		slog.Warn("Failed to fetch latest release from GitHub", "component", "version", "error", err)
		return fallback
	}
	defer func() {
		// Drain and close the body to allow connection reuse
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("GitHub releases API returned non-200 status", //nolint:gosec // G706: status code is a server-side integer, not user-tainted
			"component", "version",
			"status", strconv.Itoa(resp.StatusCode))
		return fallback
	}

	var releases []struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		slog.Warn("Failed to parse GitHub releases response", "component", "version", "error", err)
		return fallback
	}

	if len(releases) == 0 {
		slog.Warn("No releases found from GitHub API", "component", "version")
		return fallback
	}

	latestTag := releases[0].TagName
	latestClean := strings.TrimPrefix(latestTag, "v")
	currentClean := strings.TrimPrefix(s.appVersion, "v")

	updateAvailable := CompareSemver(latestClean, currentClean) > 0

	result := VersionCheckResult{
		Current:         s.appVersion,
		Latest:          latestTag,
		UpdateAvailable: updateAvailable,
		ReleaseURL:      fmt.Sprintf("https://github.com/Ghent/capacitarr/releases/tag/%s", latestTag),
		CheckedAt:       time.Now(),
	}

	// Always publish a check event for activity log visibility
	if s.bus != nil {
		s.bus.Publish(events.VersionCheckEvent{
			CurrentVersion:  s.appVersion,
			LatestVersion:   latestTag,
			UpdateAvailable: updateAvailable,
		})
	}

	// Publish UpdateAvailableEvent once per detected version to avoid
	// repeated notifications on every cache refresh cycle.
	if updateAvailable && s.bus != nil && latestClean != s.lastNotifiedVersion {
		s.lastNotifiedVersion = latestClean
		s.bus.Publish(events.UpdateAvailableEvent{
			CurrentVersion: s.appVersion,
			LatestVersion:  latestTag,
			ReleaseURL:     result.ReleaseURL,
		})
	}

	return result
}

// CompareSemver compares two semantic versions and returns:
//
//	-1 if a < b, 0 if a == b, 1 if a > b.
//
// Both versions may optionally have a leading "v" prefix and a prerelease
// suffix separated by "-". A release version (no prerelease) is considered
// greater than its prerelease counterpart (e.g. "1.0.0" > "1.0.0-rc.3").
func CompareSemver(a, b string) int {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")

	aParts, aPre := splitPrerelease(a)
	bParts, bPre := splitPrerelease(b)

	aVer := parseVersionParts(aParts)
	bVer := parseVersionParts(bParts)

	// Compare major, minor, patch numerically
	for i := 0; i < 3; i++ {
		av, bv := 0, 0
		if i < len(aVer) {
			av = aVer[i]
		}
		if i < len(bVer) {
			bv = bVer[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}

	// Numeric parts are equal — compare prerelease
	// No prerelease > has prerelease (stable > pre-release)
	if aPre == "" && bPre == "" {
		return 0
	}
	if aPre == "" {
		return 1 // a is stable, b is prerelease → a > b
	}
	if bPre == "" {
		return -1 // a is prerelease, b is stable → a < b
	}

	// Both have prerelease — compare lexicographically
	if aPre > bPre {
		return 1
	}
	if aPre < bPre {
		return -1
	}
	return 0
}

// splitPrerelease splits "1.2.3-rc.1" into ("1.2.3", "rc.1").
// Build metadata (everything after "+") is stripped per SemVer spec —
// it MUST be ignored when determining version precedence.
// If there is no prerelease suffix, the second return value is "".
func splitPrerelease(v string) (string, string) {
	// Strip build metadata first (e.g., "1.0.0+build.123" → "1.0.0")
	if plusIdx := strings.Index(v, "+"); plusIdx >= 0 {
		v = v[:plusIdx]
	}
	idx := strings.Index(v, "-")
	if idx < 0 {
		return v, ""
	}
	return v[:idx], v[idx+1:]
}

// parseVersionParts splits "1.2.3" into [1, 2, 3].
func parseVersionParts(v string) []int {
	parts := strings.Split(v, ".")
	result := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			result = append(result, 0)
		} else {
			result = append(result, n)
		}
	}
	return result
}
