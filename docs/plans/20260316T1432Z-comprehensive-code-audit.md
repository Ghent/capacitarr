# Comprehensive Code Audit Plan

**Status:** ✅ Complete
**Created:** 2026-03-16T14:32Z
**Scope:** Full codebase audit — service layer compliance, code quality, modularization, consistency, documentation accuracy, dead code removal

---

## Executive Summary

After a thorough audit of the entire Capacitarr codebase, I identified findings across several categories: service layer violations, code consistency issues, dead/dangling code, documentation drift, and modularization opportunities. The codebase is generally well-structured and follows the documented architecture, but there are specific areas that need remediation for a truly clean, production-quality codebase.

---

## Findings

### Finding 1: Service Layer Violation in EngineService.GetPreview()

**Severity:** HIGH  
**Location:** [`EngineService.GetPreview()`](backend/internal/services/engine.go:96)

The `GetPreview()` method performs **four direct `s.db` queries** that should go through their respective services:

1. **Line 98:** `s.db.Where("enabled = ?", true).Find(&configs)` — should use `IntegrationService.ListEnabled()`
2. **Line 142:** `s.db.FirstOrCreate(&prefs, db.PreferenceSet{ID: 1})` — should use `SettingsService.GetPreferences()`
3. **Line 145:** `s.db.Order("sort_order ASC, id ASC").Find(&rules)` — should use `RulesService.List()`
4. **Line 152:** `s.db.Find(&diskGroups)` — should use `DiskGroupService.List()`

**Remediation:**
- Add cross-service dependencies to `EngineService` (IntegrationService, SettingsService, RulesService, DiskGroupService) via the `SetDependencies()` pattern used elsewhere
- Replace all four direct DB queries with service method calls
- This is the most critical finding as it directly violates the mandatory service layer architecture

### Finding 2: Duplicate Integration Type Constants

**Severity:** MEDIUM  
**Location:** Multiple files

Integration type strings are defined as constants in three separate places:

1. [`routes/constants.go`](backend/routes/constants.go:6) — `intTypeSonarr`, `intTypeRadarr`, etc.
2. [`integrations/types.go`](backend/internal/integrations/types.go) — `IntegrationTypeSonarr`, etc.
3. Hard-coded strings in [`poller/fetch.go`](backend/internal/poller/fetch.go:54) — `"tautulli"`, `"overseerr"`, `"jellyfin"`, etc.
4. Hard-coded strings in [`services/integration.go`](backend/internal/services/integration.go:168) — `"tautulli"`, `"overseerr"`, etc.

**Remediation:**
- Use the canonical `integrations.IntegrationType*` constants everywhere
- Remove the duplicate `routes/constants.go` integration type constants (keep only scheme constants there)
- Replace all hard-coded type strings in `poller/fetch.go` and `services/integration.go` with the `integrations.IntegrationType*` constants

### Finding 3: nolint / nosemgrep Directive Audit

**Severity:** MEDIUM
**Location:** 18 directives across the codebase

Every `nolint` and `nosemgrep` directive must be individually reviewed with user approval. Below is the complete inventory with analysis and a recommendation for each.

#### Directive 1: `routes/auth.go:92` — Cookie without Secure flag
```go
c.SetCookie(&http.Cookie{ //nolint:gosec // nosemgrep
```
**Linter:** gosec (G402) + semgrep (cookie-without-secure)
**Context:** Sets the `jwt` HttpOnly cookie. `Secure` flag is conditionally set based on `SECURE_COOKIES` env var.
**Analysis:** The Secure flag is conditional by design — not all self-hosted deployments use HTTPS. The cookie IS HttpOnly and SameSite=Lax.
**Recommendation:** ✅ KEEP — but improve the justification comment to explain the conditional Secure flag.

#### Directive 2: `routes/auth.go:106` — Non-HttpOnly cookie
```go
c.SetCookie(&http.Cookie{ //nolint:gosec // nosemgrep
```
**Linter:** gosec (G402) + semgrep (cookie-without-httponly)
**Context:** Sets the `authenticated` detection cookie. Value is just `"true"` — no secrets.
**Analysis:** HttpOnly is intentionally `false` so the SPA JavaScript can detect auth state. The cookie contains no sensitive data.
**Recommendation:** ✅ KEEP — but expand the justification to explicitly state "no sensitive data".

#### Directive 3: `routes/middleware.go:42` — Switch branch style
```go
case "Bearer": //nolint:gocritic // auth method branches test different conditions
```
**Linter:** gocritic (singleCaseSwitch)
**Context:** The `switch` has `case "Bearer"`, `case "ApiKey"`, and `default` — it's NOT a single-case switch. Gocritic may be flagging the overall structure.
**Analysis:** This is a valid multi-branch switch that tests different auth methods. The directive may be unnecessary if the linter doesn't actually fire on this pattern anymore.
**Recommendation:** ⚠️ TRY REMOVING — run `make ci` to see if it passes without the directive. If the linter doesn't flag it, remove the directive.

#### Directive 4: `events/sse_broadcaster.go:99` — Unchecked error
```go
escapedMsg, _ := json.Marshal(humanMsg) //nolint:errcheck // string marshal can't fail
```
**Linter:** errcheck
**Context:** `json.Marshal()` on a `string` value. Marshaling a string to JSON can never fail.
**Analysis:** Correct — `json.Marshal` of a Go `string` always succeeds.
**Recommendation:** ✅ KEEP — justification is accurate.

#### Directive 5: `services/deletion.go:160` — Unchecked error
```go
_ = s.rateLimiter.Wait(context.Background()) //nolint:errcheck // Wait with background context never returns non-nil error
```
**Linter:** errcheck
**Context:** `rate.Limiter.Wait()` with `context.Background()` — the only error case is context cancellation, which can't happen with background context.
**Analysis:** Correct — `Wait(context.Background())` can only return `nil`.
**Recommendation:** ✅ KEEP — justification is accurate.

#### Directive 6: `services/engine.go:105` — Non-exhaustive switch
```go
switch integrations.IntegrationType(cfg.Type) { //nolint:exhaustive // *arr types handled by NewClient below
```
**Linter:** exhaustive
**Context:** The switch handles Plex, Tautulli, Overseerr, Jellyfin, Emby explicitly; all other types fall through to `NewClient()`.
**Analysis:** All types ARE handled — some explicitly in the switch and others via the default path. The `exhaustive` linter can't know this.
**Recommendation:** ✅ KEEP — but this code should be refactored as part of the enrichment client factory work (Finding 6) which would eliminate this switch entirely.

#### Directive 7: `config/config.go:85` — Logging trusted value
```go
slog.Info("Trusted reverse proxy auth header configured", ..., "header", authHeader) //nolint:gosec // G706: authHeader is from trusted env var, not user input
```
**Linter:** gosec G706 (potential information exposure)
**Context:** Logs the name of the AUTH_HEADER env var (e.g., "Remote-User") at startup.
**Analysis:** This is a config key NAME, not a secret value. Logging it at startup is standard practice for operational visibility.
**Recommendation:** ✅ KEEP — justification is correct. The value is an env var name, not user input.

#### Directive 8: `config/config.go:92` — Logging trusted value
```go
slog.Warn("SECURITY: AUTH_HEADER is set ...", ..., "header", authHeader) //nolint:gosec // G706: authHeader is from trusted env var, not user input
```
**Linter:** gosec G706
**Context:** Same as Directive 7 — logs the header name in a security warning.
**Recommendation:** ✅ KEEP — same reasoning as Directive 7.

#### Directive 9: `integrations/httpclient.go:42` — HTTP request with variable URL
```go
resp, err := sharedHTTPClient.Do(req) //nolint:gosec // G704: URL is from admin-configured integration settings
```
**Linter:** gosec G107 (URL provided to HTTP request as taint input)
**Context:** The URL comes from `IntegrationConfig.URL`, which is admin-configured and validated (scheme check) at the route layer.
**Analysis:** The URL is from a trusted admin source, not end-user input.
**Recommendation:** ✅ KEEP — the URL is admin-configured and scheme-validated.

#### Directive 10: `integrations/httpclient.go:50` — Logging response info
```go
slog.Debug("Integration API response", ...) //nolint:gosec // G706: sanitizedURL is safe, status/duration are server-side values
```
**Linter:** gosec G706
**Context:** Debug-level logging of API response status code and duration. URL is sanitized via `logger.SanitizeURL()`.
**Recommendation:** ✅ KEEP — all logged values are sanitized or server-side integers.

#### Directive 11: `integrations/arr_helpers.go:224` — HTTP request with variable URL
```go
resp, err := sharedHTTPClient.Do(req) //nolint:gosec // G704: URL is from admin-configured integration settings
```
**Linter:** gosec G107
**Context:** Same pattern as Directive 9 — DELETE request to *arr API.
**Recommendation:** ✅ KEEP — same reasoning.

#### Directive 12: `integrations/plex.go:185` — Non-exhaustive switch
```go
switch MediaType(m.Type) { //nolint:exhaustive // Plex only returns movie, show, season, and episode types
```
**Linter:** exhaustive
**Context:** Plex API only returns `movie`, `show`, `season`, and `episode` types. Other MediaType variants (artist, album, book) are never seen from Plex.
**Recommendation:** ✅ KEEP — Plex legitimately doesn't return all media types. The default returns `nil` to skip unknown types.

#### Directive 13: `integrations/sonarr.go:193` — Non-exhaustive switch
```go
switch item.Type { //nolint:exhaustive // Sonarr only handles shows and seasons
```
**Linter:** exhaustive
**Context:** Sonarr's delete API only works on shows and seasons. Other types are structurally impossible here.
**Recommendation:** ✅ KEEP — Sonarr cannot delete movies, artists, or books.

#### Directive 14: `integrations/sonarr.go:241` — HTTP request with variable URL
```go
resp, err := sharedHTTPClient.Do(req) //nolint:gosec // G704: URL is from admin-configured integration settings
```
**Linter:** gosec G107
**Context:** DELETE request for Sonarr bulk episode file deletion.
**Recommendation:** ✅ KEEP — same reasoning as Directive 9.

#### Directive 15: `notifications/httpclient.go:52` — HTTP request with variable URL
```go
resp, err := webhookHTTPClient.Do(req) //nolint:gosec // URL is from admin-configured webhook settings
```
**Linter:** gosec G107
**Context:** POST request to Discord/Apprise webhook URL, admin-configured and scheme-validated.
**Recommendation:** ✅ KEEP — same reasoning.

#### Directive 16: `services/auth.go:213` — Logging username
```go
slog.Info("Auto-created user from proxy auth header", ..., "username", username) //nolint:gosec // username is from a trusted reverse proxy header
```
**Linter:** gosec G706
**Context:** Logs the username extracted from a trusted reverse proxy header during auto-creation.
**Analysis:** The username comes from a trusted proxy header (AUTH_HEADER config). However, if a user accesses directly without a proxy, they COULD spoof the header. This is a known accepted risk documented in the codebase.
**Recommendation:** ✅ KEEP — the risk is documented and accepted. Users are warned about proxy requirements.

#### Directive 17: `services/version.go:161` — HTTP request with variable URL
```go
resp, err := client.Do(req) //nolint:gosec // URL is set at construction time (DefaultGitLabReleasesURL or test URL), not user-tainted
```
**Linter:** gosec G107
**Context:** The URL is either `DefaultGitLabReleasesURL` (compile-time constant) or a test-injected URL.
**Analysis:** The URL is safe but the `http.Client{}` on line 160 is created without a timeout. The context provides a 10s timeout for the request, but best practice is to also set `client.Timeout` as a defense-in-depth measure.
**Recommendation:** ✅ KEEP the nolint — but **fix the bare `http.Client{}`** by adding a timeout: `client := &http.Client{Timeout: 15 * time.Second}`.

#### Directive 18: `services/version.go:173` — Logging response status
```go
slog.Warn("GitLab releases API returned non-200 status", //nolint:gosec // G706: status code is a server-side integer, not user-tainted
```
**Linter:** gosec G706
**Context:** Logs the HTTP status code from GitLab API response.
**Recommendation:** ✅ KEEP — status codes are server-side integers.

#### Directive 19: `testutil/testutil.go:240` — Test-only nosemgrep
```go
tokenString, err := token.SignedString([]byte(TestJWTSecret)) // nosemgrep: go.jwt-go.security.jwt.hardcoded-jwt-key — test-only constant, not a production secret
```
**Linter:** semgrep (hardcoded JWT key)
**Context:** Test utility using a well-known test secret for unit test JWT generation.
**Recommendation:** ✅ KEEP — test-only code with a clear justification.

#### Summary Table

| # | File | Linter | Recommendation |
|---|------|--------|---------------|
| 1 | routes/auth.go:92 | gosec+semgrep | ✅ Keep (improve comment) |
| 2 | routes/auth.go:106 | gosec+semgrep | ✅ Keep (improve comment) |
| 3 | routes/middleware.go:42 | gocritic | ⚠️ Try removing |
| 4 | events/sse_broadcaster.go:99 | errcheck | ✅ Keep |
| 5 | services/deletion.go:160 | errcheck | ✅ Keep |
| 6 | services/engine.go:105 | exhaustive | ✅ Keep (refactored away by factory) |
| 7 | config/config.go:85 | gosec G706 | ✅ Keep |
| 8 | config/config.go:92 | gosec G706 | ✅ Keep |
| 9 | integrations/httpclient.go:42 | gosec G107 | ✅ Keep |
| 10 | integrations/httpclient.go:50 | gosec G706 | ✅ Keep |
| 11 | integrations/arr_helpers.go:224 | gosec G107 | ✅ Keep |
| 12 | integrations/plex.go:185 | exhaustive | ✅ Keep |
| 13 | integrations/sonarr.go:193 | exhaustive | ✅ Keep |
| 14 | integrations/sonarr.go:241 | gosec G107 | ✅ Keep |
| 15 | notifications/httpclient.go:52 | gosec G107 | ✅ Keep |
| 16 | services/auth.go:213 | gosec G706 | ✅ Keep |
| 17 | services/version.go:161 | gosec G107 | ✅ Keep (fix bare client) |
| 18 | services/version.go:173 | gosec G706 | ✅ Keep |
| 19 | testutil/testutil.go:240 | semgrep | ✅ Keep |

### Finding 4: Architecture Documentation Drift

**Severity:** MEDIUM  
**Location:** [`docs/architecture.md`](docs/architecture.md)

1. **Service Registry listing is incomplete** (line 124-144) — missing `DiskGroup` service in the code block but it exists in the table above
2. **EventBus code example is outdated** (line 152-167) — shows `subscribers []chan Event` (slice) but actual implementation uses `map[chan Event]struct{}` (map). Also shows `Subscribe() <-chan Event` but actual returns `chan Event` (bidirectional)
3. **Event count is stale** — header says "40 total" but the table should be counted to verify
4. **SSE event ID format is outdated** — architecture doc shows `id: 1741199820-001` (timestamp-based) but actual implementation uses auto-incrementing integers
5. **Project structure mentions `poller/` as "Engine orchestrator + deletion worker"** but the deletion worker is actually in `services/deletion.go`

**Remediation:** Update all five items to match current implementation

### Finding 5: CONTRIBUTING.md Documentation Drift

**Severity:** LOW  
**Location:** [`CONTRIBUTING.md`](CONTRIBUTING.md:50)

1. **Line 50:** Mentions "notification dispatcher (Discord/Slack)" — Slack was removed, should say "Discord/Apprise"

**Remediation:** Fix the Slack reference to Apprise

### Finding 6: Duplicate Enrichment Client Construction

**Severity:** MEDIUM
**Location:** [`poller/fetch.go:56-75`](backend/internal/poller/fetch.go:56) and [`services/engine.go:104-123`](backend/internal/services/engine.go:104)

The same enrichment client construction logic (Plex, Tautulli, Overseerr, Jellyfin, Emby) is duplicated between `fetchAllIntegrations()` in the poller and `GetPreview()` in EngineService. Both iterate through integration configs, check the type, and call the appropriate `NewXxxClient()` constructor. This violates DRY and means any change to enrichment client construction needs to be updated in two places.

**Remediation:**
- Add a `BuildEnrichmentClients()` factory method to `IntegrationService` that creates an `integrations.EnrichmentClients` struct from enabled configs
- Refactor both the poller and EngineService.GetPreview() to use this factory
- This centralizes enrichment client creation in the service layer and eliminates the duplicate switch/case blocks

### Finding 7: Rate Limiter Goroutine Leak Risk

**Severity:** LOW  
**Location:** [`routes/ratelimit.go`](backend/routes/ratelimit.go:30)

Rate limiters are created with `newLoginRateLimiter()` which spawns a background cleanup goroutine, but `Stop()` is never called on any of the three rate limiters created in the routes (`loginRL` in auth.go:58, `integrationTestRL` in integrations.go:154, `engineRunRL` in engine.go:65). These goroutines run for the lifetime of the process, so it's not technically a leak, but it's not clean.

**Remediation:**
- Either document that the goroutines are intentionally never stopped (process-lifetime), or store the rate limiters somewhere accessible for graceful shutdown
- The simplest approach: add a comment documenting the lifecycle expectation

### Finding 8: Inconsistent Error Response Patterns

**Severity:** LOW  
**Location:** [`routes/approval.go`](backend/routes/approval.go:49)

Some route handlers use `apiError()` (the standard pattern) while a few use inline `c.JSON()` for errors:
- [`approval.go:49-52`](backend/routes/approval.go:49) — uses `c.JSON(http.StatusInternalServerError, map[string]string{"error": "..."})` instead of `apiError()`
- [`approval.go:54-56`](backend/routes/approval.go:54) — same pattern for deletions disabled error
- [`approval.go:91-94`](backend/routes/approval.go:91) — same pattern for preferences error

**Remediation:** Replace all inline `c.JSON(status, map[string]string{"error": "..."})` calls with `apiError()` for consistency

### Finding 9: DiskGroup Service Missing in EngineService

**Severity:** MEDIUM  
**Location:** [`services/engine.go:96-193`](backend/internal/services/engine.go:96)

Related to Finding 1, `EngineService` does not have dependency injection for the services it needs. It only receives `*gorm.DB` and `*events.EventBus` in its constructor. To fix Finding 1, it needs:
- `IntegrationService` for `ListEnabled()`
- `SettingsService` for `GetPreferences()`
- `RulesService` for `List()`
- `DiskGroupService` for `List()`

**Remediation:** Add a `SetDependencies()` method following the same pattern as `DeletionService.SetDependencies()`

### Finding 10: Makefile Go Version Reference

**Severity:** LOW  
**Location:** [`Makefile:74`](Makefile:74)

The Makefile references `golang:1.26-alpine` for the test and security Docker images. This should be verified against the actual `go.mod` version to ensure it matches. The `.gitlab-ci.yml` should also be checked for consistency.

**Remediation:** Ensure `go.mod`, `Makefile`, `.gitlab-ci.yml`, and `Dockerfile` all reference the same Go version

### Finding 11: BackupService Missing DiskGroup Service

**Severity:** LOW (already wired)  
**Location:** [`services/registry.go:82`](backend/internal/services/registry.go:82)

This is already properly handled — `backupSvc.SetDiskGroupService(diskGroupSvc)` is called in the registry. No action needed but noted during audit.

### Finding 12: Minor — `loginRateLimiter` Naming Convention

**Severity:** VERY LOW
**Location:** [`routes/ratelimit.go`](backend/routes/ratelimit.go)

The type `loginRateLimiter` is used for all rate limiting (login, integration test, engine run), not just login. The naming is slightly misleading.

**Remediation:** Consider renaming to `ipRateLimiter` or `rateLimiterIP` and `LoginRateLimit` middleware to `IPRateLimit` for accuracy. The `newLoginRateLimiter` constructor should become `newIPRateLimiter`.

### Finding 13: Missing IntegrationType Constants

**Severity:** MEDIUM
**Location:** [`integrations/types.go`](backend/internal/integrations/types.go:8)

The `IntegrationType` constant block only defines 6 types: Plex, Sonarr, Radarr, Tautulli, Overseerr, Lidarr. **Three types are missing**: Readarr, Jellyfin, Emby. These types exist as string values throughout the codebase but don't have named `IntegrationType` constants.

**Remediation:** Add `IntegrationTypeReadarr`, `IntegrationTypeJellyfin`, and `IntegrationTypeEmby` constants to the `types.go` const block. This also supports Finding 2 (constant consolidation) since all types need to be defined before they can be used everywhere.

### Finding 14: SECURITY.md Accuracy Issues

**Severity:** HIGH (documentation integrity for a security document)
**Location:** [`SECURITY.md`](SECURITY.md)

Cross-referencing every claim in SECURITY.md against the codebase reveals the following discrepancies:

1. **Nosemgrep line numbers are stale** — The nosemgrep table (lines 124-134) references incorrect line numbers:
   - `testutil/testutil.go:167` → actual is **line 240**
   - `routes/auth.go:71` → actual is **line 92**
   - `routes/auth.go:85` → actual is **line 106**

2. **Missing golangci-lint nolint directive documentation** — SECURITY.md documents Semgrep `nosemgrep` annotations and the Gosec G117 policy in detail, but does NOT document any of the 18 `//nolint:` directives from golangci-lint. This is a significant gap — the security document should provide a complete picture of all suppressed findings.

3. **Semgrep file count may be stale** — Line 107 claims "Semgrep scans 487 files." This should be re-verified as the codebase has grown.

4. **ZAP baseline date** — Line 151 references "Latest baseline (2026-03-10)". After code changes from this audit, a new ZAP scan should be run and the baseline updated.

5. **Rate limiting documentation is incomplete** — Line 64 only mentions "Login endpoint is rate-limited" but three endpoints are actually rate-limited: login (10/15min), integration test (30/5min), and engine run (5/5min).

6. **Missing `IntegrationTypeReadarr/Jellyfin/Emby` from security list** — Line 199 says "All integration client structs" use `json:"-"`, which is correct in practice (verified all 8 client structs), but the `IntegrationType` constants list is incomplete as noted in Finding 13.

**Remediation:**
- Fix all stale line numbers in the nosemgrep table
- Add a new section documenting all golangci-lint `//nolint:` directives
- Re-count Semgrep scanned files and update the claim
- Run ZAP scan after code changes and update the baseline
- Update rate limiting documentation to list all 3 endpoints with their limits
- Add ZAP testing recommendation: run after every release or significant code change (at minimum before each tag/release)

---

## Implementation Plan

### Phase 1: Service Layer Compliance (Critical)

1. [x] **Add cross-service dependencies to EngineService** — Created `IntegrationLister`, `RulesProvider`, and `DiskGroupLister` interfaces in `engine.go`; reused existing `SettingsReader` interface from `deletion.go`. Added `SetDependencies()` method following the established pattern.
2. [x] **Refactor `EngineService.GetPreview()`** — Replaced all four direct DB queries (`s.db.Where(...)`, `s.db.FirstOrCreate(...)`, `s.db.Order(...)`, `s.db.Find(...)`) with service method calls (`s.integrations.ListEnabled()`, `s.preferences.GetPreferences()`, `s.rules.List()`, `s.diskGroups.List()`). Added proper error handling for each service call.
3. [x] **Wire new dependencies in `services/registry.go`** — Added `engineSvc.SetDependencies(reg.Integration, settingsSvc, reg.Rules, diskGroupSvc)` in the cross-service wiring section after registry construction.
4. [x] **Update tests** — Added three new tests: `TestEngineService_GetPreview_NoIntegrations` (verifies empty preview with no integrations), `TestEngineService_GetPreview_WithDiskGroups` (verifies disk context calculation with seeded disk groups), and `TestEngineService_SetDependencies` (verifies field wiring). All use real services with in-memory SQLite following the established test pattern.

### Phase 2: Enrichment Client Factory

5. [x] **Add `BuildEnrichmentClients()` method to IntegrationService** — Added `EnrichmentBuildResult` struct and `BuildEnrichmentClients()` method to `services/integration.go`. The method queries enabled integrations via `ListEnabled()`, classifies each config as enrichment (Plex, Tautulli, Overseerr, Jellyfin, Emby) or *arr (Sonarr, Radarr, Lidarr, Readarr), builds the appropriate enrichment client for each enrichment type, and returns a result containing the `EnrichmentClients` struct, the enrichment configs (for connection testing in the poller), and the remaining *arr configs. Also added an `enrichmentTestFn()` helper in `poller/fetch.go` to map enrichment config types to their client's `TestConnection` function.
6. [x] **Refactor `poller/fetch.go:fetchAllIntegrations()`** — Changed signature from `fetchAllIntegrations(configs []db.IntegrationConfig, integrationSvc *services.IntegrationService)` to `fetchAllIntegrations(integrationSvc *services.IntegrationService)`. The function now calls `BuildEnrichmentClients()` internally to get enrichment clients and *arr configs, eliminating the inline enrichment client construction switch block. Enrichment connection testing uses the new `enrichmentTestFn()` helper to look up the `TestConnection` function and the existing `connectEnrichment()` pattern. Updated the call site in `poller.go` accordingly.
7. [x] **Refactor `EngineService.GetPreview()`** — Expanded `IntegrationLister` interface to include `BuildEnrichmentClients() (*EnrichmentBuildResult, error)`. Refactored `GetPreview()` to call `s.integrations.BuildEnrichmentClients()` and iterate only the returned `ArrConfigs` for media items, removing the duplicate enrichment switch block and its `//nolint:exhaustive` directive.
8. [x] **Update tests** — Added four new tests to `integration_test.go`: `TestIntegrationService_BuildEnrichmentClients_NoConfigs`, `TestIntegrationService_BuildEnrichmentClients_MixedConfigs`, `TestIntegrationService_BuildEnrichmentClients_OnlyArr`, and `TestIntegrationService_BuildEnrichmentClients_OnlyEnrichment`. Updated `fetch_test.go` to match the new `fetchAllIntegrations` signature (removed config parameter, `TestFetchAllIntegrations_UnknownType` now creates configs in DB instead of passing directly). All existing engine and poller tests continue to pass unchanged.

### Phase 3: Constant Consolidation

9. [x] **Add missing IntegrationType constants** — Consolidated `IntegrationTypeReadarr`, `IntegrationTypeJellyfin`, `IntegrationTypeEmby` into the main const block in `integrations/types.go` (they were previously in a separate const block added during Phase 2). Removed the duplicate second const block.
10. [x] **Remove duplicate integration type constants from `routes/constants.go`** — Removed all 9 `intType*` constants, keeping only `schemeHTTP` and `schemeHTTPS`. Updated `routes/rulefields.go` to import `capacitarr/internal/integrations` and use `string(integrations.IntegrationType*)` for all type comparisons: `detectEnrichment()` switch cases, `serviceType == intTypeSonarr` comparisons, `cfg.Type == intTypeSonarr` check, and the `arrTypes` map literal.
11. [x] **Replace hard-coded type strings in `poller/fetch.go`** — Replaced two `cfg.Type == "sonarr"` comparisons with `cfg.Type == string(integrations.IntegrationTypeSonarr)`. The `enrichmentTestFn()` helper and `BuildEnrichmentClients()` usage were already using typed constants from Phase 2. Confirmed `evaluate.go` has no integration type raw strings.
12. [x] **Replace hard-coded type strings in `services/integration.go`** — Replaced 5 raw string cases in `TestConnection()` switch (`"tautulli"`, `"overseerr"`, `"jellyfin"`, `"emby"`, `"plex"`) with `string(integrations.IntegrationType*)`. `BuildEnrichmentClients()` and `FetchCollectionValues()` already used typed constants from Phase 2.
13. [x] **Replace hard-coded type strings in `routes/rulefields.go`** — Completed as part of step 10 (same file). All 9 `intType*` references replaced.
14. [x] **Verify no other files use raw integration type strings** — Ran `grep` across all non-test `.go` files excluding `db/validation.go`. The only remaining raw strings are the canonical constant definitions in `integrations/types.go`.

### Phase 4: Code Quality & nolint Audit

15. [x] **Standardize error responses in `routes/approval.go`** — Replace 3 inline `c.JSON()` error calls with `apiError()`
16. [x] **Rename rate limiter types for accuracy** — `loginRateLimiter` → `ipRateLimiter`, `newLoginRateLimiter` → `newIPRateLimiter`, `LoginRateLimit` → `IPRateLimit`. Updated all callers in `auth.go`, `integrations.go`, `engine.go`, and `ratelimit_test.go`. Log message updated from "Login rate limit exceeded" to "Rate limit exceeded".
17. [x] **Add lifecycle documentation for rate limiter goroutines** — Document that they are intentionally process-lifetime. Added to `newIPRateLimiter` constructor comment.
18. [x] **Improve nolint comments on cookie directives** — Expanded inline comments in `routes/auth.go` to explicitly state justification: JWT cookie explains conditional Secure flag, authenticated cookie explains intentional HttpOnly=false and no-secrets design.
19. [x] **Try removing `gocritic` directive in `routes/middleware.go:42`** — Removed successfully. `go vet ./...` and `go build ./...` both pass cleanly. The switch has 3 cases (Bearer, ApiKey, default) so gocritic's singleCaseSwitch does not fire.
20. [x] **Fix bare `http.Client{}` in `services/version.go:160`** — Add `Timeout: 15 * time.Second` for defense-in-depth (the context already has a 10s timeout but best practice is to set both)

### Phase 5: Documentation Updates

21. [x] **Update `docs/architecture.md` Service Registry code block** — Reordered fields to match actual `registry.go` (alphabetical after shared deps), added `DiskGroup *DiskGroupService`. Also added `DiskGroupService` and `BackupService` to the service category table.
22. [x] **Update `docs/architecture.md` EventBus code example** — Changed `subscribers` from `[]chan Event` to `map[chan Event]struct{}`, added `closed bool` field, fixed `Subscribe()` and `Unsubscribe()` signatures from receive-only to bidirectional channels, added `Close()` method.
23. [x] **Update `docs/architecture.md` event count** — Actual count is 42 (not 40). Added `approval_queue_cleared` to Approval row and `version_check` to System row. Also fixed stale "39 event types" in the Activity Events table description.
24. [x] **Update `docs/architecture.md` SSE event ID format** — Changed from timestamp-based `1741199820-001` format to auto-incrementing integer IDs (`1`, `2`).
25. [x] **Update `docs/architecture.md` project structure** — Changed poller description from "Engine orchestrator + deletion worker" to "Engine orchestrator (scheduled disk monitoring)". Fixed db description from "single baseline migration" to "schema migrations". Added missing `cache/` and `testutil/` directories.
26. [x] **Fix `CONTRIBUTING.md` Slack reference** — Changed "Discord/Slack" to "Discord/Apprise".

### Phase 6: SECURITY.md Comprehensive Refresh

Every claim in SECURITY.md must be verified against the current codebase. This is a security-critical document and must be 100% accurate.

27. [x] **Fix stale nosemgrep line numbers** — Update the nosemgrep table entries:
    - `testutil/testutil.go:167` → actual **line 240** ✅
    - `routes/auth.go:71` → actual **line 92** ✅
    - `routes/auth.go:85` → actual **line 106** ✅
    - Also fixed: `overseerr_test.go` 204,224 → 205,223; `useEventStream.ts` 179 → 180
28. [x] **Add golangci-lint nolint directive documentation** — Added "Inline `nolint` annotations" table with 16 production-code `//nolint:` directives (not 18 — actual count after Phase 4 removed the `routes/middleware.go` gocritic directive). Each entry includes file, line, linter rule, and rationale.
29. [x] **Update rate limiting documentation** — Replaced single-line mention with all three rate-limited endpoints:
    - Login: 10 attempts per IP per 15 minutes ✅
    - Integration test: 30 attempts per IP per 5 minutes ✅
    - Engine run: 5 attempts per IP per 5 minutes ✅
30. [x] **Re-verify Semgrep scanned file count** — Actual count is **514** (was 487). Updated.
31. [x] **Add ZAP testing cadence recommendation** — Added "Testing cadence" paragraph after ZAP baseline section.
32. [x] **Verify all other SECURITY.md claims** — Line-by-line verification complete:
    - **Authentication:** bcrypt cost 12 (`BcryptCost = 12` in services/auth.go:31) ✅, JWT 24h (`time.Now().Add(24 * time.Hour)` in auth.go:58) ✅, SHA-256 API keys (`sha256.Sum256` in auth.go:228) ✅
    - **Container hardening:** Dockerfile confirms `ca-certificates`, `tzdata`, `su-exec` are the only packages ✅; `rm -rf /sbin/apk` removes package manager ✅; docker-compose.yml confirms `cap_drop: ALL` with 4 caps + `no-new-privileges:true` ✅
    - **Dependency overrides:** Table was **stale** — missing 6 overrides (minimatch ×3, rollup, serialize-javascript, svgo) added since the last SECURITY.md update. Updated table from 5 to 9 rows with full GHSA links and upstream dep chains. Date updated to 2026-03-16.
    - **G117 policy:** Three excluded file paths (`internal/db/models.go`, `routes/auth.go`, `routes/integrations.go`) confirmed matching `.golangci.yml` regex ✅; `json:"-"` tags confirmed on all internal secret fields (config.Config.JWTSecret, db.AuthConfig.Password/APIKey, all integration client structs) ✅
    - **Gitleaks allowlist:** Three path patterns (`_test.go`, `docs/api/`, `docs/plans/`) match `.gitleaks.toml` exactly ✅
    - **Semgrep partial-parse warnings:** Cannot re-verify without running Semgrep; left as-is (percentages may have changed with codebase growth)

### Phase 7: Verification

33. [x] **Run `make ci` to verify all changes pass** — Full lint, test, and security pipeline
    - **First run:** 2 `exhaustive` lint errors in `poller/fetch.go:42` and `services/integration.go:526` — switch statements on `IntegrationType` were missing cases for `Sonarr`, `Radarr`, `Lidarr`, `Readarr`. Fixed by adding explicit cases to both switch statements.
    - **Second run:** All stages passed cleanly — golangci-lint (0 issues), ESLint/Prettier/TypeScript (clean), go test (all pass), vitest (73/73), govulncheck (0 vulns), pnpm audit (0 vulns), trivy (0 vulns), gitleaks (no leaks), semgrep (0 findings).
34. [ ] **Run `make security:zap` after all changes** — Update the ZAP baseline in SECURITY.md with new results
    - **Deferred to post-commit:** The ZAP DAST scan (`make security:zap`) requires a running Capacitarr instance via `docker compose up --build`. This should be run separately after the changes are committed and the Docker image is built, as it is a long-running test against the live application.

---

## Files Affected

| File | Changes |
|------|---------|
| `backend/internal/services/engine.go` | Add SetDependencies, refactor GetPreview, use enrichment factory |
| `backend/internal/services/registry.go` | Wire EngineService dependencies |
| `backend/internal/services/engine_test.go` | Update test setup for new dependencies |
| `backend/internal/services/integration.go` | Add BuildEnrichmentClients(), use IntegrationType constants, add exhaustive switch cases for *arr types |
| `backend/internal/services/integration_test.go` | Add tests for BuildEnrichmentClients() |
| `backend/internal/services/version.go` | Add Timeout to http.Client |
| `backend/internal/integrations/types.go` | Add missing IntegrationType constants (Readarr, Jellyfin, Emby) |
| `backend/internal/poller/fetch.go` | Use IntegrationType constants, use enrichment factory, add exhaustive switch cases for *arr types |
| `backend/routes/constants.go` | Remove integration type constants |
| `backend/routes/rulefields.go` | Use IntegrationType constants |
| `backend/routes/approval.go` | Standardize error responses |
| `backend/routes/ratelimit.go` | Rename types, add documentation |
| `backend/routes/integrations.go` | Update rate limiter constructor name |
| `backend/routes/engine.go` | Update rate limiter constructor name |
| `backend/routes/auth.go` | Update rate limiter constructor name, improve nolint comments |
| `backend/routes/middleware.go` | Remove gocritic directive if possible |
| `SECURITY.md` | Comprehensive refresh: nolint documentation, fix line numbers, update rate limiting, ZAP cadence |
| `docs/architecture.md` | Five corrections |
| `CONTRIBUTING.md` | Fix Slack → Apprise |

---

## What Was NOT Found (Clean Areas)

The following areas passed the audit with no issues:

- **Route handlers** — All route handlers correctly delegate to services for data access. No direct `reg.DB` access found in any route handler.
- **Middleware** — Auth middleware correctly uses `reg.Auth` service methods. No direct DB access.
- **Event subscribers** — ActivityPersister and NotificationDispatchService use the `ActivityWriter` interface, not direct DB access. SSEBroadcaster has no DB access at all.
- **Jobs/Cron** — All cron jobs correctly use service methods on `reg.Metrics`, `reg.Engine`, `reg.Settings`, `reg.AuditLog`.
- **Service constructors** — All services follow the established pattern: accept `*gorm.DB` and `*events.EventBus`, register on `services.Registry`.
- **Database layer** — Clean model definitions, validation maps, migration approach via goose.
- **Cache package** — Well-implemented TTL cache with proper cleanup.
- **Config package** — Clean env var loading with sensible defaults and security warnings.
- **Logger package** — Dynamic level support with proper global var pattern.
- **Notification senders** — Properly abstracted behind `Sender` interface.
