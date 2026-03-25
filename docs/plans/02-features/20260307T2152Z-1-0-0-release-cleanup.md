# 1.0.0 Release Cleanup Plan

**Status:** ✅ Complete  
**Created:** 2026-03-07T21:52Z  
**Scope:** Full codebase audit remediation — code quality, testing, documentation, and release readiness  

This plan addresses all 25 findings from the pre-release audit. Work is organized into 5 phases, ordered by dependency and risk. Each phase should be a separate branch and MR.

---

## Phase 1: Critical Code Fixes

**Branch:** `refactor/1-0-0-code-cleanup`

These are structural issues that affect API surface, correctness, or guideline compliance. They must be done first because later phases depend on clean code.

### Step 1.1 — Relocate `HashAPIKey` and `IsHashedAPIKey` from routes to services

**Files:**
- `routes/api.go` — remove `HashAPIKey()`, `IsHashedAPIKey()`, and `apiKeyHashPrefix`
- `internal/services/auth.go` — add `HashAPIKey()`, `IsHashedAPIKey()`, and `apiKeyHashPrefix`
- `routes/middleware.go` — update any references (currently none, but verify)

**Why:** Route handlers must not own cryptographic functions. These belong in the auth service per the service layer architecture rules. This is an API surface concern — after 1.0.0, moving exported functions is a breaking change for any external consumers.

**Acceptance:** `HashAPIKey` and `IsHashedAPIKey` are only defined in `services/auth.go`. No route file imports crypto packages for API key hashing.

### Step 1.2 — Relocate `IsMaskedKey` from services to a shared location

**Files:**
- `internal/services/integration.go` — remove `IsMaskedKey()`
- `internal/db/validation.go` — add `IsMaskedKey()`
- `routes/integrations.go` — update import to use `db.IsMaskedKey()`

**Why:** `IsMaskedKey` is a validation utility used by both routes and services. Placing it in `db/validation.go` alongside `ValidIntegrationTypes` and other validators follows the existing pattern.

**Acceptance:** `IsMaskedKey` is defined once in `db/validation.go`. Both `routes/integrations.go` and `internal/services/integration.go` import from `db`.

### Step 1.3 — Relocate `maskAPIKey` to co-locate with `IsMaskedKey`

**Files:**
- `routes/integrations.go` — remove `maskAPIKey()`
- `internal/db/validation.go` — add `MaskAPIKey()` (exported)
- `routes/integrations.go` — update calls to `db.MaskAPIKey()`

**Why:** Masking and mask-detection are two sides of the same concern. Co-locating them prevents drift and makes the API key handling story clear in one file.

**Acceptance:** `MaskAPIKey` and `IsMaskedKey` are adjacent in `db/validation.go`.

### Step 1.4 — Unify integration type constants (eliminate triple-definition)

**Files:**
- `routes/constants.go` — remove `intType*` constants; replace with references to `integrations.IntegrationType*` values or `db.ValidIntegrationTypes` keys
- `routes/rulefields.go` — update references from `intTypeSonarr` etc. to the canonical source
- `routes/notifications.go` — update `notifTypeDiscord`/`notifTypeSlack` to use `db` validation map keys
- All route files that reference the old constants

**Why:** Triple-definition of integration types across `integrations/types.go`, `db/validation.go`, and `routes/constants.go` risks silent drift. Canonical source should be `integrations/types.go` for type safety. `routes/constants.go` should only import and alias if needed, or be eliminated entirely.

**Decision:** Keep `integrations.IntegrationType*` as the canonical typed constants. Add a `string()` conversion or define string constants in `integrations/types.go` that routes can use. For `routes/constants.go`, keep only non-integration constants (URL scheme constants). For notification types, define them in `db/validation.go` next to `ValidNotificationChannelTypes`.

**Acceptance:** `routes/constants.go` contains only `schemeHTTP`/`schemeHTTPS`. All integration type references trace back to `integrations/types.go` or `db/validation.go`.

### Step 1.5 — Fix `npm run release` script to strip `v` prefix

**Files:**
- `package.json` — update the `release` script

**Current (broken):**
```
npm version $(git cliff --bumped-version) --no-git-tag-version
```

**Fixed:**
```
VERSION=$(git cliff --bumped-version) && SEMVER=${VERSION#v} && npm version "$SEMVER" --no-git-tag-version
```

Apply the same fix for the `cd frontend && npm version ...` part.

**Acceptance:** Running `npm run release` produces a valid semver (no `v` prefix) in both `package.json` and `frontend/package.json`.

### Step 1.6 — Audit `DeletionsEnabled` default value consistency

**Files:**
- `internal/db/db.go` — add `DeletionsEnabled: true` to the `FirstOrCreate` seed in `Init()`
- `internal/testutil/testutil.go` — add `DeletionsEnabled: true` to the test seed
- Also add `SnoozeDurationHours: 24`, `CheckForUpdates: true`, `TiebreakerMethod: "size_desc"`, and `PollIntervalSeconds: 300` to both seeds

**Why:** The GORM `default:true` tag only applies when GORM creates via AutoMigrate. Since we use Goose migrations, the `FirstOrCreate` seed is the actual default source. Any field not in the seed gets Go's zero value, not the GORM tag default. `DeletionsEnabled` defaulting to `false` (zero value) when the schema says `default:true` is a subtle and dangerous inconsistency.

**Acceptance:** Every field in `PreferenceSet` that has a `gorm:"default:..."` tag also has the same value in both the `db.Init()` seed and the `testutil.SetupTestDB()` seed.

### Step 1.7 — Standardize API error response envelope

**Files:**
- Create `routes/errors.go` with a helper:
  ```go
  func apiError(c echo.Context, status int, message string) error {
      return c.JSON(status, map[string]string{"error": message})
  }
  ```
- Update all route handlers to use `apiError()` instead of inline `map[string]string{"error": ...}`
- Ensure no handler returns `map[string]interface{}` for error responses

**Why:** Consistent error envelope across all endpoints. After 1.0.0, clients will depend on this shape.

**Acceptance:** All error responses across all route handlers use the same JSON shape: `{"error": "message"}`. No error responses use `map[string]interface{}`.

### Step 1.8 — Fix trailing blank lines in route files

**Files:**
- `routes/audit.go` — remove blank line before final `}`
- `routes/notifications.go` — remove blank line before final `}`

**Acceptance:** `gofmt` produces no diff after changes.

### Step 1.9 — Fix invalid nolint comments in `config.go`

**Files:**
- `internal/config/config.go:85` — remove or correct `//nolint:gosec // G706`
- `internal/config/config.go:92` — remove or correct `//nolint:gosec // G706`

**Why:** G706 is not a valid gosec rule ID. These comments are misleading. If gosec is flagging these lines, find the actual rule ID and document it. If not, remove the nolint directives entirely.

**Acceptance:** No nolint directives reference non-existent rule IDs.

---

## Phase 2: Code Quality Refactoring

**Branch:** `refactor/1-0-0-code-quality`

These improve maintainability and reduce duplication but don't change behavior.

### Step 2.1 — Refactor `TestConnection` duplication in `IntegrationService`

**Files:**
- `internal/services/integration.go`

**Approach:** Extract a helper:
```go
func (s *IntegrationService) testEnrichmentClient(intType, url, apiKey string, testFn func() error) TestConnectionResult {
    if err := testFn(); err != nil {
        s.PublishTestFailure(intType, intType, url, err.Error())
        return TestConnectionResult{Success: false, Error: err.Error()}
    }
    s.PublishTestSuccess(intType, intType, url)
    return TestConnectionResult{Success: true, Message: "Connection successful"}
}
```

Then the switch arms become one-liners:
```go
case "tautulli":
    client := integrations.NewTautulliClient(url, apiKey)
    return s.testEnrichmentClient(intType, url, apiKey, client.TestConnection)
```

**Acceptance:** The `TestConnection` method switch has no duplicated publish/return patterns. Each enrichment arm is ≤3 lines.

### Step 2.2 — Refactor `fetchAllIntegrations` enrichment client setup

**Files:**
- `internal/poller/fetch.go`

**Approach:** Use a struct/function map or extract a helper for the repeated pattern:
```
create client → test connection → update sync status → log → continue
```

**Acceptance:** Each enrichment type setup is ≤5 lines. No copy-paste patterns in `fetchAllIntegrations`.

### Step 2.3 — Remove `enrichItems` wrapper in `poller/fetch.go`

**Files:**
- `internal/poller/fetch.go` — remove `enrichItems()` function
- `internal/poller/poller.go:158` — change call to `integrations.EnrichItems(fetched.allItems, fetched.enrichment)`

**Acceptance:** No `enrichItems` function exists in `poller/fetch.go`. The call in `poller.go` goes directly to `integrations.EnrichItems()`.

### Step 2.4 — Replace `interface{}` with `any` across the codebase

**Files:**
- `internal/cache/cache.go` — `Entry.Value interface{}` → `Entry.Value any`
- `internal/cache/cache.go` — `Get()` return type `interface{}` → `any`
- Any other occurrences found via grep

**Acceptance:** `grep -r "interface{}" backend/` returns zero results (excluding vendor/generated code).

### Step 2.5 — Add shutdown mechanism to `loginRateLimiter`

**Files:**
- `routes/ratelimit.go`

**Approach:** Add a `done` channel to `loginRateLimiter`, check it in the `cleanup()` goroutine's ticker loop, and add a `Stop()` method. Wire `Stop()` into the graceful shutdown in `main.go`.

**Acceptance:** `loginRateLimiter` has a `Stop()` method. The cleanup goroutine exits when `Stop()` is called.

### Step 2.6 — Make `SyncAll` test enrichment services

**Files:**
- `internal/services/integration.go` — update `SyncAll()` to also test enrichment-only services (Tautulli, Overseerr, Jellyfin, Emby, Plex)

**Approach:** After the existing loop over `NewClient`-compatible types, add a second pass for enrichment types that creates the specific client, tests the connection, and includes the result in the return.

**Acceptance:** Clicking "Sync All" in the UI tests ALL enabled integrations, including enrichment-only services. Each shows success/failure in the response.

### Step 2.7 — Refactor `db.DB` package-level global

**Files:**
- `internal/db/db.go` — change `Init()` to return `*gorm.DB` instead of setting `db.DB`
- `backend/main.go` — receive the return value: `database, err := db.Init(cfg)`
- Remove `var DB *gorm.DB` from `db.go`

**Why:** Package-level globals are a code smell and make testing harder. The DB connection is already passed through the registry.

**Acceptance:** No package-level `DB` variable in `internal/db/`. The connection is returned from `Init()` and passed to `services.NewRegistry()`.

---

## Phase 3: Testing

**Branch:** `test/1-0-0-test-coverage`

### Step 3.1 — Add `poller/fetch_test.go`

**Files:**
- Create `internal/poller/fetch_test.go`

**Tests to write:**
- `TestFetchAllIntegrations_EmptyConfigs` — returns empty result
- `TestFetchAllIntegrations_ArrClients` — mock Sonarr/Radarr clients return items
- `TestFetchAllIntegrations_EnrichmentClients` — Tautulli/Overseerr/Plex/Jellyfin/Emby setup
- `TestFetchAllIntegrations_DiskSpaceAggregation` — verifies largest disk wins for same path
- `TestEnrichItems_CallsThrough` — verifies the delegating wrapper works correctly (if not removed in Phase 2)

Use canonical names: "Firefly" for TV, "Serenity" for movies.

**Acceptance:** `go test ./internal/poller/... -v` passes with ≥3 new fetch tests.

### Step 3.2 — Add `config/config_test.go`

**Files:**
- Create `internal/config/config_test.go`

**Tests to write:**
- `TestLoad_Defaults` — verify all defaults when no env vars set
- `TestLoad_CustomPort` — `PORT=8080`
- `TestLoad_BaseURL_Normalization` — ensures leading/trailing slashes
- `TestLoad_JWTSecret_Debug` — uses static secret in debug mode
- `TestLoad_JWTSecret_Production` — generates random secret
- `TestLoad_CORSOrigins_Parsing` — comma-separated parsing
- `TestLoad_AuthHeader` — trusted proxy header

Use `t.Setenv()` for environment variable management in Go 1.17+.

**Acceptance:** `go test ./internal/config/... -v` passes with ≥5 new tests.

### Step 3.3 — Add `notifications/httpclient_test.go`

**Files:**
- Create `internal/notifications/httpclient_test.go`

**Tests to write:**
- `TestSendWebhookRequest_Success` — 200 response, no retries
- `TestSendWebhookRequest_Retry429` — mock returns 429 then 200
- `TestSendWebhookRequest_Retry500` — mock returns 500 then 200
- `TestSendWebhookRequest_RetryAfterHeader` — verify Retry-After is respected
- `TestSendWebhookRequest_ClientError` — 400 response, no retry
- `TestSendWebhookRequest_MaxRetriesExhausted` — always 500, fails after max retries

Use `httptest.NewServer` for mocking.

**Acceptance:** `go test ./internal/notifications/... -v` passes with ≥4 new retry tests.

### Step 3.4 — Add individual sender formatting tests

**Files:**
- Create `internal/notifications/discord_test.go`
- Create `internal/notifications/slack_test.go`

**Tests to write:**
- Discord: verify embed structure, color codes for each execution mode, field formatting
- Slack: verify Block Kit structure, section formatting, field ordering

**Acceptance:** Each sender has ≥2 dedicated formatting tests.

### Step 3.5 — Add migration round-trip test

**Files:**
- Extend `internal/db/driver_test.go` or create `internal/db/migrate_test.go`

**Tests to write:**
- `TestMigrations_UpDown` — runs all migrations up, then all down to 0, then up again
- `TestMigrations_Idempotent` — runs up twice in a row (should be no-op second time)

**Acceptance:** Migration up/down/up round-trip passes without errors.

### Step 3.6 — Audit canonical media names in all test files

**Files:**
- All `*_test.go` files

**Approach:** Search for non-canonical media names (anything other than "Firefly" for TV and "Serenity" for movies) and replace with canonical names.

**Acceptance:** `grep -rn "Breaking Bad\|Game of Thrones\|The Matrix\|Stranger Things" backend/*_test.go` returns zero results.

---

## Phase 4: Documentation

**Branch:** `docs/1-0-0-documentation`

### Step 4.1 — Fix `CONTRIBUTING.md` in-app notification reference

**Files:**
- `CONTRIBUTING.md:50`

**Change:** Replace "notification dispatcher (Discord/Slack/in-app)" with "notification dispatcher (Discord/Slack)".

**Acceptance:** No mention of "in-app" notifications in `CONTRIBUTING.md`.

### Step 4.2 — Add Readarr to `architecture.md` diagram

**Files:**
- `docs/architecture.md`

**Change:** Add `READARR["Readarr"]` to the `ARR_APPS` subgraph.

**Acceptance:** The architecture diagram shows Sonarr, Radarr, Lidarr, and Readarr.

### Step 4.3 — Fix `configuration.md` Docker image reference

**Files:**
- `docs/configuration.md:52`

**Change:** Replace `image: capacitarr:latest` with `image: registry.gitlab.com/starshadow/software/capacitarr:stable` to match `quick-start.md`.

**Acceptance:** All Docker Compose examples in docs use the full registry path with `:stable`.

### Step 4.4 — Create `SECURITY.md`

**Files:**
- Create `SECURITY.md` in project root

**Content should cover:**
- How to report vulnerabilities (private issue or email)
- Security model overview (JWT auth, API key hashing, integration key storage decisions)
- Scope of security (self-hosted tool, single-user/trusted-network assumptions)
- Supported versions (only latest release receives security fixes)

**Acceptance:** `SECURITY.md` exists and covers reporting, scope, and supported versions.

### Step 4.5 — Validate OpenAPI spec

**Approach:** Run the OpenAPI spec through a validator. Add a `make` target or CI step:
```bash
npx @redocly/cli lint docs/api/openapi.yaml
```

Review and fix any reported issues. Verify all current endpoints are documented, especially:
- `GET/POST /version/check`
- `DELETE /data/reset`
- Notification channel subscription fields (`onCycleDigest`, `onError`, etc.)

**Acceptance:** OpenAPI spec passes validation with zero errors. All current endpoints are documented.

---

## Phase 5: Release Readiness

**Branch:** `chore/1-0-0-release-prep`

### Step 5.1 — API versioning freeze audit

**Approach:** Review every endpoint's request/response shape in the route handlers. Compare against `docs/api/openapi.yaml`. Document any discrepancies. This is the 1.0.0 API contract — after release, breaking changes require a new API version.

**Acceptance:** Every endpoint in `routes/api.go` has a matching entry in `openapi.yaml` with correct request/response schemas.

### Step 5.2 — Database migration forward-compatibility test

**Approach:** 
1. Use a pre-1.0 database snapshot (if available) or create one from the last RC
2. Run migrations against it and verify no errors
3. Verify data integrity after migration

**Acceptance:** A database from the latest RC migrates cleanly to the 1.0.0 schema.

### Step 5.3 — Add rate limits to sensitive endpoints

**Files:**
- `routes/integrations.go` — add rate limit to `POST /integrations/test`
- `routes/engine.go` — add rate limit to `POST /engine/run`

**Approach:** Reuse the `loginRateLimiter` pattern (or extract a generic rate limiter) with appropriate limits:
- Integration test: 30 requests/min per IP
- Engine run: 5 requests/min per IP

**Acceptance:** `POST /integrations/test` and `POST /engine/run` return 429 when rate limited.

### Step 5.4 — Graceful shutdown integration test

**Files:**
- Add to `internal/services/deletion_test.go` or `internal/poller/poller_test.go`

**Test:** Queue a deletion job, then call `Stop()`. Verify the job completes before `Stop()` returns.

**Acceptance:** Test demonstrates that in-flight deletions complete during graceful shutdown.

### Step 5.5 — CHANGELOG cleanup for 1.0.0

**Approach:**
1. Review `CHANGELOG.md` for formatting consistency
2. Ensure all entries follow Conventional Commits grouping
3. Ensure the upcoming 1.0.0 entry clearly marks it as the first stable release
4. Consider adding a "Migration from RC" section if needed

**Acceptance:** CHANGELOG is well-formatted, chronologically ordered, and has a clear 1.0.0 section.

### Step 5.6 — Container image security scan

**Approach:** Build the Docker image and scan with Trivy:
```bash
docker build -t capacitarr:1.0.0-pre .
docker run --rm aquasec/trivy image capacitarr:1.0.0-pre
```

Fix any HIGH or CRITICAL CVEs. MEDIUM and below can be documented as known issues.

**Acceptance:** Trivy reports zero HIGH/CRITICAL vulnerabilities in the final image.

### Step 5.7 — Final `make ci` verification

**Approach:** Run the full CI pipeline locally after all changes are merged:
```bash
make ci
```

**Acceptance:** `make ci` passes with zero warnings and zero failures.

---

## Execution Order

| Phase | Branch | Depends On | Description |
|-------|--------|------------|-------------|
| 1 | `refactor/1-0-0-code-cleanup` | None | Critical code fixes |
| 2 | `refactor/1-0-0-code-quality` | Phase 1 | Refactoring and quality |
| 3 | `test/1-0-0-test-coverage` | Phase 2 | New and improved tests |
| 4 | `docs/1-0-0-documentation` | Phase 1 | Documentation fixes |
| 5 | `chore/1-0-0-release-prep` | Phases 1-4 | Final release validation |

Phases 3 and 4 can run in parallel since they modify different files. Phase 5 must be last as it validates the cumulative result.
