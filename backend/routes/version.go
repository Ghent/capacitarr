package routes

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

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"capacitarr/internal/db"
)

// versionCheckResult holds the cached update check response.
type versionCheckResult struct {
	Current         string    `json:"current"`
	Latest          string    `json:"latest"`
	UpdateAvailable bool      `json:"updateAvailable"`
	ReleaseURL      string    `json:"releaseUrl"`
	CheckedAt       time.Time `json:"checkedAt"`
}

var (
	cachedVersionCheck *versionCheckResult
	versionCheckMu     sync.Mutex
	versionCacheTTL    = 6 * time.Hour
)

// gitlabReleasesURL is the GitLab API endpoint for Capacitarr releases.
// Extracted as a package-level variable so tests can override it via
// SetGitlabReleasesURLForTest.
var gitlabReleasesURL = "https://gitlab.com/api/v4/projects/79833150/releases?per_page=1"

// SetGitlabReleasesURLForTest overrides the GitLab releases URL.
// Intended for use in tests only.
func SetGitlabReleasesURLForTest(url string) {
	gitlabReleasesURL = url
}

// ResetVersionCacheForTest clears the cached version check result so that
// tests start with a cold cache. Intended for use in tests only.
func ResetVersionCacheForTest() {
	versionCheckMu.Lock()
	cachedVersionCheck = nil
	versionCheckMu.Unlock()
}

// RegisterVersionRoutes sets up the version check endpoint on the protected group.
func RegisterVersionRoutes(g *echo.Group, database *gorm.DB, appVersion string) {
	g.GET("/version/check", handleVersionCheck(database, appVersion))
}

func handleVersionCheck(database *gorm.DB, appVersion string) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Load preferences to check if update checks are enabled
		var pref db.PreferenceSet
		if err := database.First(&pref, 1).Error; err != nil {
			slog.Warn("Failed to load preferences for version check", "component", "version", "error", err)
			return c.JSON(http.StatusOK, versionCheckResult{
				Current: appVersion,
			})
		}

		if !pref.CheckForUpdates {
			return c.JSON(http.StatusOK, versionCheckResult{
				Current: appVersion,
			})
		}

		// Check cache
		versionCheckMu.Lock()
		if cachedVersionCheck != nil && time.Since(cachedVersionCheck.CheckedAt) < versionCacheTTL {
			result := *cachedVersionCheck
			versionCheckMu.Unlock()
			return c.JSON(http.StatusOK, result)
		}
		versionCheckMu.Unlock()

		// Fetch latest release from GitLab
		result := fetchLatestRelease(appVersion)

		// Cache the result
		versionCheckMu.Lock()
		cachedVersionCheck = &result
		versionCheckMu.Unlock()

		return c.JSON(http.StatusOK, result)
	}
}

// fetchLatestRelease queries the GitLab releases API and returns a versionCheckResult.
// On any failure it returns a graceful degradation response with only the current version.
func fetchLatestRelease(appVersion string) versionCheckResult {
	fallback := versionCheckResult{
		Current:   appVersion,
		CheckedAt: time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gitlabReleasesURL, nil)
	if err != nil {
		slog.Warn("Failed to create request for version check", "component", "version", "error", err)
		return fallback
	}
	req.Header.Set("User-Agent", fmt.Sprintf("Capacitarr/%s", appVersion))

	client := &http.Client{}
	resp, err := client.Do(req) //nolint:gosec // G704: URL is a hardcoded constant (gitlabReleasesURL), not user-tainted
	if err != nil {
		slog.Warn("Failed to fetch latest release from GitLab", "component", "version", "error", err)
		return fallback
	}
	defer func() {
		// Drain and close the body to allow connection reuse
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("GitLab releases API returned non-200 status", //nolint:gosec // G706: status code is a server-side integer, not user-tainted
			"component", "version",
			"status", strconv.Itoa(resp.StatusCode))
		return fallback
	}

	var releases []struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		slog.Warn("Failed to parse GitLab releases response", "component", "version", "error", err)
		return fallback
	}

	if len(releases) == 0 {
		slog.Warn("No releases found from GitLab API", "component", "version")
		return fallback
	}

	latestTag := releases[0].TagName
	latestClean := strings.TrimPrefix(latestTag, "v")
	currentClean := strings.TrimPrefix(appVersion, "v")

	updateAvailable := compareSemver(latestClean, currentClean) > 0

	return versionCheckResult{
		Current:         appVersion,
		Latest:          latestTag,
		UpdateAvailable: updateAvailable,
		ReleaseURL:      fmt.Sprintf("https://gitlab.com/starshadow/software/capacitarr/-/releases/%s", latestTag),
		CheckedAt:       time.Now(),
	}
}

// CompareSemverForTest is an exported wrapper around compareSemver for use in tests.
func CompareSemverForTest(a, b string) int {
	return compareSemver(a, b)
}

// compareSemver compares two semantic versions and returns:
//
//	-1 if a < b, 0 if a == b, 1 if a > b.
//
// Both versions may optionally have a leading "v" prefix and a prerelease
// suffix separated by "-". A release version (no prerelease) is considered
// greater than its prerelease counterpart (e.g. "1.0.0" > "1.0.0-rc.3").
func compareSemver(a, b string) int {
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
// If there is no prerelease suffix, the second return value is "".
func splitPrerelease(v string) (string, string) {
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
