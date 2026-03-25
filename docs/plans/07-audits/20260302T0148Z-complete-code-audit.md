# Complete Code Audit Plan

**Created:** 2026-03-02T01:48Z  
**Branch:** `chore/code-audit` (from `feature/ux-misc-polish`)  
**Scope:** Full codebase audit — backend (Go), frontend (Vue 3 / Nuxt 3), infrastructure (Docker, CI)

> **Status:** ✅ Complete — All 7 phases implemented. Security audit, structured logging, backend code quality, comprehensive test coverage (all 9 integration clients, all route handlers, poller stats), frontend accessibility audit, CI/CD pipeline, and Go doc comments all done.

---

## Objective

Perform a comprehensive code audit of the entire Capacitarr codebase to identify and remediate:

- Security vulnerabilities
- Performance bottlenecks
- Code quality / maintainability issues
- Dead code and unused dependencies
- Error handling gaps
- Missing or inadequate test coverage
- Consistency violations (naming, patterns, structure)
- Documentation gaps
- Logging deficiencies

---

## Core Principles

These principles apply across every phase of the audit and must not be compromised.

### 1. Structured Logging

All logging must be **structured, consistent, and machine-parseable** for integration with external logging services (Loki, Datadog, Elasticsearch, etc.).

- **Format:** JSON via `slog.NewJSONHandler` (already in place)
- **All log levels supported:** `debug`, `info`, `warn`, `error`
- **Log level configurable at runtime** via the Settings UI (Advanced section) — already wired through `PreferenceSet.LogLevel`, but audit must verify:
  - Every package uses `slog` consistently (no `fmt.Println`, no `log.Printf`)
  - Every log statement includes relevant structured fields (`"component"`, `"operation"`, `"integration"`, `"duration"`, `"error"`, etc.)
  - Debug logging is _thorough_ — every significant branch, API call, cache hit/miss, scoring decision, rule evaluation, and polling cycle emits a `slog.Debug` with context
  - Error logs always include the originating error and enough context to diagnose without access to the code
  - No sensitive data (passwords, API keys, tokens) appears in any log level
  - Log level changes take effect immediately without restart (verify `logger.SetLevel` is called on preference save)

**Logging audit checklist per file:**

| Check | Description |
|-------|-------------|
| No raw `fmt` / `log` | All output goes through `slog` |
| Structured fields | Key-value pairs, not string interpolation |
| Debug coverage | Key decision points, external API calls, cache behavior, timing |
| Error context | `"error", err` always present, plus operation context |
| No sensitive leaks | API keys, passwords, tokens never logged |
| Component tag | Each log includes a `"component"` or `"module"` field for filtering |

### 2. Testing Philosophy

Tests must be **accurate, confident, and provide real value**. No test exists just to boost coverage numbers.

**Hard rules:**

- **No `t.Skip()`** — if a test can't pass, it doesn't exist. Fix the code or don't write the test.
- **No ignored errors** — every `error` return is checked; tests fail on unexpected errors.
- **No automatic passing** — every test asserts something meaningful about behavior.
- **No diminishing returns** — don't test trivial getters/setters or auto-generated code (shadcn-vue components are excluded).
- **Zero warnings policy** — `go test ./...` and `pnpm vitest run` must produce clean output with no warnings.
- **Table-driven tests** for all functions with multiple input scenarios (already partially done in `engine/` — extend everywhere).
- **Integration tests** use real HTTP test servers (`httptest.NewServer`), not mocked interfaces, for integration client testing.
- **Regression tests** — every bug fix must include a test that reproduces the bug before verifying the fix.

**Test categories:**

| Category | Scope | Framework | Coverage Target |
|----------|-------|-----------|----------------|
| Unit | Pure functions, scoring, rule matching | `testing` | Every exported function + critical internals |
| Integration | HTTP handlers, DB operations | `testing` + `httptest` + in-memory SQLite | All API routes |
| Regression | Bug reproductions | `testing` | One per fix (ongoing) |
| Frontend unit | Composables, utilities, format functions | Vitest | All composables and utils |
| Frontend component | Critical interactive components | Vitest + Vue Test Utils | RuleBuilder, ScoreBreakdown, EngineControl |

### 3. Code Quality Standards

**Zero-tolerance policy** — the codebase must compile and lint with no warnings, no deprecations, and no suppressed issues.

- **Go:** `golangci-lint run` with full ruleset — zero findings. No `//nolint` comments unless accompanied by a justification comment.
- **Frontend:** `pnpm eslint .` — zero warnings, zero errors. No `eslint-disable` comments unless justified.
- **TypeScript:** Strict mode, no `any` types, no `@ts-ignore`.
- **CSS:** No `!important` overrides (per project rules). No dead rules. All styling through CSS variables and Tailwind utilities.
- **Dependencies:** No known vulnerabilities (`go vuln check`, `pnpm audit`). No deprecated packages.
- **Compilation:** `go build` clean, `pnpm build` clean — zero warnings.

---

## Codebase Overview

| Layer | Stack | Key Directories | Approx. Size |
|-------|-------|-----------------|-------------|
| Backend | Go 1.25, Echo, GORM, SQLite | `backend/` | ~115 KB source |
| Frontend | Vue 3, Nuxt 3, Tailwind CSS, shadcn-vue | `frontend/app/` | ~180 KB source |
| Infrastructure | Docker multi-stage, Makefile | root | ~3 KB |
| UI Components | shadcn-vue (Radix Vue) | `frontend/app/components/ui/` | ~60 KB |

### Backend Package Map

| Package | Path | Responsibility |
|---------|------|---------------|
| `main` | `backend/main.go` | Server bootstrap, SPA handler, embed FS, graceful shutdown |
| `config` | `backend/internal/config/` | Environment variable loading |
| `db` | `backend/internal/db/` | GORM models, migrations, SQLite init |
| `engine` | `backend/internal/engine/` | Scoring algorithm, rule evaluation |
| `integrations` | `backend/internal/integrations/` | Sonarr, Radarr, Lidarr, Readarr, Plex, Jellyfin, Emby, Tautulli, Overseerr |
| `poller` | `backend/internal/poller/` | Background polling loop, media fetch, deletion worker |
| `jobs` | `backend/internal/jobs/` | Cron-based time-series rollups |
| `cache` | `backend/internal/cache/` | In-memory TTL cache |
| `logger` | `backend/internal/logger/` | slog initialization |
| `routes` | `backend/routes/` | API handlers, middleware, auth |

### Frontend Page Map

| Page | Path | Purpose |
|------|------|---------|
| Dashboard | `frontend/app/pages/index.vue` | Capacity overview, charts, engine stats |
| Scoring Engine | `frontend/app/pages/rules.vue` | Preference weights, disk thresholds, rule builder |
| Audit History | `frontend/app/pages/audit.vue` | Historical log of engine decisions |

---

## Audit Phases

### Phase 1: Security Audit

**Priority:** Critical  
**Estimated effort:** 4–6 hours

| # | Area | Files | What to Check |
|---|------|-------|---------------|
| 1.1 | Authentication & JWT | `routes/api.go`, `routes/middleware.go` | JWT signing/validation, token expiry, cookie flags (HttpOnly, Secure, SameSite), first-user bootstrap race condition |
| 1.2 | Password handling | `routes/api.go` | bcrypt cost, password policy, timing-safe comparison |
| 1.3 | API key security | `routes/middleware.go`, `routes/api.go` | Key generation entropy, storage (plaintext vs hashed), key rotation |
| 1.4 | Input validation | `routes/rules.go`, `routes/integrations.go`, `routes/api.go` | SQL injection via GORM, XSS in user-supplied strings, path traversal, integer overflow on IDs |
| 1.5 | Proxy auth header trust | `routes/middleware.go`, `config/config.go` | AUTH_HEADER spoofing when not behind proxy, auto-user-creation abuse |
| 1.6 | CORS configuration | `config/config.go`, `main.go` | Wildcard CORS in debug mode leaking to production |
| 1.7 | Integration credential storage | `db/models.go` | API keys stored in plaintext in SQLite, consider encryption at rest |
| 1.8 | Rate limiting | All API routes | Absence of rate limiting on login, API endpoints |
| 1.9 | Error message information leakage | All route handlers | Stack traces, internal paths, or DB details in error responses |

#### Specific Concerns Identified During Review

- **First-user bootstrap in `LoginRequest` handler:** If `count == 0`, any credentials are accepted and a new user is created. Race condition possible if two concurrent requests arrive before any user exists.
- **API keys stored in plaintext** in `AuthConfig` — should be hashed like passwords.
- **JWT cookie** set without explicit `HttpOnly`, `Secure`, or `SameSite` flags visible in the login handler.
- **`AUTH_HEADER`** in `RequireAuth` — if the header is configured but the server is directly exposed (not behind a proxy), any client can spoof the header.

---

### Phase 2: Structured Logging Audit

**Priority:** High  
**Estimated effort:** 4–6 hours

The current logging foundation is solid (`slog` + JSON handler + dynamic `LevelVar`), but needs consistent application across the entire codebase.

| # | Area | Files | What to Do |
|---|------|-------|------------|
| 2.1 | Eliminate raw logging | All `.go` files | Find and replace any `fmt.Println`, `fmt.Printf`, `log.Printf`, `log.Fatal` with `slog` equivalents |
| 2.2 | Add component tags | All packages | Every `slog` call must include `"component", "<package>"` for log filtering by service consumers |
| 2.3 | Structured error logging | All error paths | Ensure `"error", err` is always present, plus `"operation"`, `"integration"`, `"mediaId"`, etc. |
| 2.4 | Debug logging coverage | `poller/`, `engine/`, `integrations/`, `routes/` | Add `slog.Debug` at: API call start/end with timing, cache hit/miss, rule evaluation per-item, scoring factor calculations, deletion decisions, poll cycle start/end with stats |
| 2.5 | Sensitive data scrubbing | All files | Audit every log call — API keys, passwords, JWT tokens, and integration URLs with embedded tokens must never be logged |
| 2.6 | Log level runtime verification | `routes/rules.go`, `logger/logger.go` | Verify saving preferences with a new log level calls `logger.SetLevel()` and takes effect immediately |
| 2.7 | Request logging enrichment | `main.go`, `routes/middleware.go` | Add request ID generation and propagation; include `"requestId"` in Echo logger middleware |
| 2.8 | Startup logging | `main.go` | Log all non-sensitive configuration on startup: port, base URL, debug mode, CORS origins, auth header name |

#### Debug Logging Depth Requirements

Every significant operation must emit debug logs at minimum:

```
Poller:     cycle start → integration fetch → media count → score calculation → deletion decision → cycle complete (with duration)
Engine:     per-item score entry → each factor raw score → rule evaluation per-rule → final score → sort position
Integrations: request URL → response status → response time → parsed item count
Routes:     request received → validation passed → DB operation → response sent
Cache:      hit/miss per key → eviction → TTL expiry
```

---

### Phase 3: Backend Code Quality

**Priority:** High  
**Estimated effort:** 6–8 hours

| # | Area | Files | What to Check |
|---|------|-------|---------------|
| 3.1 | Error handling consistency | All backend files | Ensure all errors are logged **or** returned, never swallowed; consistent JSON error response format |
| 3.2 | Poller decomposition | `poller/poller.go` (26 KB — largest file) | Break into: `poller.go` (loop), `fetch.go` (integration calls), `evaluate.go` (scoring), `delete.go` (deletion worker), `stats.go` (stat tracking) |
| 3.3 | Route handler splitting | `routes/rules.go` (22 KB), `routes/api.go` (13.5 KB) | Extract validation logic, split by resource (preferences, rules, preview) |
| 3.4 | Database operations | `db/db.go`, `db/migrate.go`, all routes | Transaction usage for multi-step operations, N+1 queries, index coverage |
| 3.5 | Integration client resilience | `integrations/*.go` | Timeouts, retries, connection pooling, error propagation, context usage |
| 3.6 | Scoring engine correctness | `engine/score.go`, `engine/rules.go` | Edge cases (division by zero, nil items, empty preference sets), score normalization |
| 3.7 | Concurrency safety | `poller/poller.go`, `cache/cache.go` | Shared state protection, atomic operations correctness, channel semantics |
| 3.8 | Resource cleanup | `main.go`, `poller/poller.go`, `jobs/cron.go` | Graceful shutdown ordering, DB connection close, channel drain |
| 3.9 | Static analysis | All `.go` files | Run `golangci-lint` with expanded ruleset — zero findings required |
| 3.10 | Dead code removal | All packages | Remove unexported functions that are unused, orphaned types, commented-out code |

#### golangci-lint Configuration Update

Expand `.golangci.yml` to include:

```yaml
linters:
  enable:
    - gofmt
    - govet
    - errcheck
    - staticcheck
    - ineffassign
    - typecheck
    - misspell
    - unparam
    - unused
    # New additions:
    - gosec          # Security scanner
    - gocritic       # Code quality suggestions
    - revive         # Superset of golint
    - nilerr         # Nil error returns
    - bodyclose      # HTTP body close
    - exhaustive     # Exhaustive switch/enum
    - goconst        # Repeated strings → constants
    - prealloc       # Slice preallocation
    - noctx          # HTTP requests without context

run:
  timeout: 5m
  tests: true  # Lint test files too — no exceptions

issues:
  max-issues-per-linter: 0  # Report all issues
  max-same-issues: 0        # Don't suppress duplicates
```

---

### Phase 4: Test Coverage

**Priority:** High  
**Estimated effort:** 10–14 hours

All tests must follow the testing philosophy defined in Core Principles above.

#### Backend Tests

| # | Area | Current State | What to Write |
|---|------|--------------|---------------|
| 4.1 | Engine scoring | `score_test.go` (5.7 KB) — partial | Complete table-driven tests for every scoring factor, edge cases (zero weights, nil dates, max values), normalization bounds (0.0-1.0) |
| 4.2 | Engine rules | `rules_test.go` (7.7 KB) — partial | Complete table-driven tests for every operator (`==`, `!=`, `>`, `>=`, `<`, `<=`, `contains`, `!contains`), every field type, cascading rules, rule combination effects |
| 4.3 | Auth middleware | None | Test JWT validation, API key auth, proxy header auth, expired tokens, malformed tokens, missing auth |
| 4.4 | Route handlers | None | Integration tests with `httptest.Server` + in-memory SQLite for: login, preferences CRUD, integration CRUD, rule CRUD, audit log queries, engine trigger |
| 4.5 | Poller logic | None | Test poll cycle with mock integrations, deletion queue behavior, concurrent poll prevention, dynamic interval |
| 4.6 | Integration clients | None | Test each integration client against `httptest.NewServer` mock responses — success, error, timeout, malformed JSON, empty results |
| 4.7 | Cache | None | Test TTL expiry, concurrent access, invalidation |
| 4.8 | Database migrations | None | Test each migration applies cleanly on fresh DB; test migration sequence integrity |

#### Frontend Tests

| # | Area | What to Write |
|---|------|---------------|
| 4.9 | Utility functions | Test `format.ts` — `formatBytes`, `formatRelativeTime`, all edge cases |
| 4.10 | Composables | Test `useEngineControl` — state management, API call behavior, error handling |
| 4.11 | Critical components | Test `RuleBuilder` — rule add/remove/edit, validation; `ScoreBreakdown` — rendering with various data shapes |

#### Test Infrastructure Setup

- **Backend:** Create `testutil` package with: `SetupTestDB()` (in-memory SQLite), `SetupTestServer()` (Echo + routes), `MockIntegrationServer()` (httptest patterns)
- **Frontend:** Configure Vitest in `frontend/vitest.config.ts`, add `@vue/test-utils` dependency

---

### Phase 5: Frontend Code Quality

**Priority:** High  
**Estimated effort:** 5–7 hours

| # | Area | Files | What to Check |
|---|------|-------|---------------|
| 5.1 | TypeScript strictness | All `.vue`, `.ts` files | Zero `any` types, proper typing of all API responses with shared interfaces |
| 5.2 | Component decomposition | `pages/index.vue`, `pages/rules.vue`, `pages/audit.vue` | Extract reusable sub-components from oversized pages |
| 5.3 | API layer | Composables, `$fetch` calls | Consistent error handling, loading states, base URL handling, typed responses |
| 5.4 | State management | Composables | Proper reactive cleanup on unmount, no leaked intervals/timers/event listeners |
| 5.5 | Accessibility (a11y) | All components | ARIA labels, keyboard navigation, focus management, color contrast |
| 5.6 | CSS audit | `assets/css/main.css` (32 KB) | Remove dead rules, eliminate any `!important`, verify theme variable completeness |
| 5.7 | ESLint zero warnings | All frontend files | `pnpm eslint .` produces zero output |
| 5.8 | Build audit | `nuxt.config.ts`, `package.json` | Zero build warnings, no unused dependencies, tree-shaking verification |
| 5.9 | Console cleanup | All components | No `console.log` in production code; use structured logging or remove |

---

### Phase 6: Infrastructure & Build

**Priority:** Medium  
**Estimated effort:** 3–4 hours

| # | Area | Files | What to Check |
|---|------|-------|---------------|
| 6.1 | Dockerfile optimization | `Dockerfile` | Layer caching, image size, security (non-root user, minimal base), Go version correctness |
| 6.2 | Docker Compose | `docker-compose.yml` | Health checks, restart policy, volume permissions |
| 6.3 | Build reproducibility | `Makefile`, `package.json` | Deterministic builds, version pinning, lockfile integrity |
| 6.4 | CI/CD pipeline | Create `.gitlab-ci.yml` | Automated lint + test + build + security scan on every push |
| 6.5 | Dependency audit | `go.mod`, `package.json` | `govulncheck`, `pnpm audit` — zero known CVEs |
| 6.6 | Release process | `cliff.toml`, `CHANGELOG.md` | git-cliff config correctness, version bump automation |

#### CI/CD Pipeline Specification

```yaml
stages:
  - lint
  - test
  - build
  - security

lint:go:
  # golangci-lint with full config — must pass with zero findings

lint:frontend:
  # pnpm eslint . — must pass with zero findings

test:go:
  # go test ./... -race -count=1 — zero failures, zero warnings

test:frontend:
  # pnpm vitest run — zero failures, zero warnings

build:docker:
  # docker build — zero warnings in both build stages

security:deps:
  # govulncheck + pnpm audit — zero known vulnerabilities
```

---

### Phase 7: Documentation & Consistency

**Priority:** Medium  
**Estimated effort:** 2–3 hours

| # | Area | What to Check |
|---|------|---------------|
| 7.1 | README accuracy | Verify all instructions work, fix typo "Depolyment" → "Deployment" |
| 7.2 | API documentation | Generate OpenAPI/Swagger spec from route definitions |
| 7.3 | Code comments | Every exported Go function has a doc comment. No stale/misleading comments. |
| 7.4 | Environment variable docs | Document all env vars in a central table in README or dedicated doc |
| 7.5 | Contributing guide | Verify `CONTRIBUTING.md` reflects current practices |
| 7.6 | Plan file cleanup | Archive completed plans |

---

## Execution Strategy

### Commit Strategy

Each audit phase produces atomic commits using Conventional Commits. Each commit must leave the codebase in a compiling, passing state.

```
fix(security): hash API keys instead of storing plaintext
fix(auth): add HttpOnly and Secure flags to JWT cookie
feat(logging): add structured component tags to all slog calls
feat(logging): add comprehensive debug logging to poller cycle
refactor(poller): decompose into fetch, evaluate, delete, stats modules
refactor(routes): split rules.go into preference, rule, and preview handlers
chore(lint): expand golangci-lint config with security and quality linters
fix(lint): resolve all golangci-lint findings
test(engine): complete table-driven tests for all scoring factors
test(routes): add integration tests for auth and preference endpoints
test(integrations): add httptest-based client tests
chore(deps): update vulnerable dependencies
docs: fix README typos and add env var documentation
```

### Prioritized Execution Order

1. **Phase 1 — Security** (Critical path, must be first)
2. **Phase 2 — Structured Logging** (Foundation for debugging everything else)
3. **Phase 3 — Backend Quality** (Refactor before testing)
4. **Phase 4 — Test Coverage** (Lock in behavior after refactoring)
5. **Phase 5 — Frontend Quality** (CSS audit, TypeScript strictness)
6. **Phase 6 — Infrastructure** (CI/CD, dependency audit)
7. **Phase 7 — Documentation** (Final pass)

### Definition of Done

- [ ] `golangci-lint run` — zero findings with expanded config
- [ ] `go build` — zero warnings
- [ ] `go test ./... -race -count=1` — all pass, zero warnings, zero skips
- [ ] `pnpm eslint .` — zero findings
- [ ] `pnpm build` — zero warnings
- [ ] `pnpm vitest run` — all pass, zero warnings, zero skips
- [ ] `govulncheck ./...` — zero known vulnerabilities
- [ ] `pnpm audit` — zero known vulnerabilities
- [ ] No `fmt.Println`, `log.Printf`, or `console.log` in production code
- [ ] Every `slog` call has structured fields and a component tag
- [ ] Debug logging covers all significant operations (see Phase 2 depth requirements)
- [ ] Log level is configurable at runtime via UI and takes effect immediately
- [ ] JWT cookie has HttpOnly + Secure + SameSite flags
- [ ] API keys are hashed in the database
- [ ] `poller.go` decomposed to files under 500 lines each
- [ ] `routes/rules.go` split into logical handler groups
- [ ] `main.css` reduced by removing dead rules — no `!important`
- [ ] Every exported Go function has a doc comment
- [ ] CI/CD pipeline runs lint + test + build + security on every push
- [ ] README typos fixed, env var table added
