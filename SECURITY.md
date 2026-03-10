# Security Policy

## Security Philosophy

Capacitarr is developed using AI-assisted coding ("vibe coding"). We believe this makes rigorous, transparent security practices **more important, not less**. Every line of code — whether human-written or AI-generated — passes through the same gauntlet of static analysis, dependency scanning, container scanning, and dynamic application security testing before it reaches a release.

This document is our commitment to transparency. Every security exception, suppression, and allowlist entry is individually documented with rationale — not hidden in config files. We want you to be able to audit our security posture yourself.

**We actively welcome independent security assessments.** If you run a scan, penetration test, or code review and find something, we want to hear about it. We will prioritize investigating and remediating any findings brought to our attention. See [Reporting a Vulnerability](#reporting-a-vulnerability) below for how to reach us.

## Supported Versions

Only the latest stable release receives security fixes. Pre-release versions (alpha, beta, RC) are not covered.

| Version | Supported |
|---------|-----------|
| Latest stable (1.x) | ✅ |
| Pre-release (RC, beta) | ❌ |

## Reporting a Vulnerability

If you discover a security vulnerability, please report it privately:

1. **GitLab:** Open a [confidential issue](https://gitlab.com/starshadow/software/capacitarr/-/issues/new?confidential=true) with the `security` label
2. **Email:** Send details to the project maintainer listed in [CONTRIBUTORS.md](CONTRIBUTORS.md)

**Do not** open a public issue for security vulnerabilities.

### What to Include

- Description of the vulnerability
- Steps to reproduce
- Affected version(s)
- Potential impact assessment
- Suggested fix (if you have one)

### Response Timeline

- **Acknowledgment:** Within 72 hours
- **Initial assessment:** Within 1 week
- **Fix release:** Dependent on severity; critical issues target a patch release within 2 weeks

## Security Model

Capacitarr is designed as a **self-hosted, single-instance** application for home lab environments. The security model reflects this:

### Authentication

- **Password authentication:** bcrypt-hashed passwords (cost factor 12)
- **JWT sessions:** HMAC-SHA256 signed tokens with 24-hour expiry. Set `JWT_SECRET` for persistent sessions across restarts
- **API keys:** SHA-256 hashed before storage; plaintext shown once on generation and never stored
- **Reverse proxy auth:** Optional trusted header authentication (`AUTH_HEADER`) for SSO integration (Authelia, Authentik, Organizr)

### Data Protection

- **Integration API keys:** Stored in plaintext in the SQLite database. This is an accepted trade-off: full encryption-at-rest would require a master key, adding complexity with minimal practical benefit when the database file is on a user-owned machine. Ensure the `/config` volume has restrictive file permissions (`chmod 600`)
- **API key masking:** Integration API keys are masked in all API responses (only last 4 characters visible)
- **Cookie security:** Set `SECURE_COOKIES=true` when serving over HTTPS

### Network Security

- **SSRF protection:** All user-provided URLs are validated to use HTTP or HTTPS schemes only
- **CORS:** Same-origin by default; explicit `CORS_ORIGINS` configuration required for cross-origin access
- **Rate limiting:** Login endpoint is rate-limited to prevent brute-force attacks
- **Response body limits:** Upstream API responses are capped at 64 MiB via `io.LimitReader` to prevent denial-of-service from oversized responses
- **Security headers:** All responses include:
  - `Content-Security-Policy` — restricts resource loading to same-origin (script-src, connect-src, font-src, frame-ancestors, base-uri, form-action)
  - `Strict-Transport-Security` — HSTS with 2-year max-age (only when `SECURE_COOKIES=true`)
  - `X-Content-Type-Options: nosniff`
  - `X-Frame-Options: DENY`
  - `Referrer-Policy: strict-origin-when-cross-origin`
  - `Permissions-Policy: camera=(), microphone=(), geolocation=()`
  - `Cross-Origin-Opener-Policy: same-origin`
  - `Cross-Origin-Resource-Policy: same-origin`
  - `X-Permitted-Cross-Domain-Policies: none`

### CI Security Scanning (SAST + SCA)

Every push and merge request is scanned by 7 static security tools. **All are blocking** — failures prevent merge. Run all scans locally with: `make security:ci`

#### Tool Inventory

| Tool | Type | What It Catches | CI Job | Blocking? |
|------|------|-----------------|--------|-----------|
| **gosec** | SAST (Go) | SQL injection, hardcoded credentials, weak crypto, insecure TLS, SSRF | `lint:go` (via golangci-lint) | ✅ Yes |
| **govulncheck** | SCA (Go) | Known vulnerabilities in Go dependencies (call-graph analysis) | `security:govulncheck` | ✅ Yes |
| **pnpm audit** | SCA (Node.js) | Known vulnerabilities in npm/pnpm dependencies | `security:pnpm-audit` | ✅ Yes |
| **Trivy (FS)** | SCA (multi-lang) | Filesystem scan for Go module + Node.js dependency CVEs (HIGH/CRITICAL) | `security:trivy` | ✅ Yes |
| **Trivy (image)** | Container scan | Alpine OS packages + binary CVEs in the Docker image | `security:trivy-image` | ✅ Yes |
| **Gitleaks** | Secret scanning | Accidentally committed API keys, passwords, tokens in git history | `security:gitleaks` | ✅ Yes |
| **Semgrep** | SAST (multi-lang) | 338 rules across Go, TypeScript, Vue, YAML, Dockerfile, Bash | `security:semgrep` | ✅ Yes |

#### Gitleaks Configuration (`.gitleaks.toml`)

Gitleaks scans the entire git history for accidentally committed secrets. The following paths are allowlisted because they contain **intentional example/test credentials**, not real secrets:

| Allowlisted Path Pattern | Reason |
|--------------------------|--------|
| `*_test.go` | Go test files contain fake API keys (`valid-api-key-12345`, `secret12345678`) and test JWT tokens used as fixtures in middleware, integration, and auth tests |
| `docs/api/` | API documentation contains example JWT tokens (`eyJhbGciOiJIUzI1NiIs...`) and example API keys (`ck_a1b2c3d4e5f6...`) in curl command examples |
| `docs/plans/` | Plan documents may reference example credentials in design discussions |

**All allowlisted patterns are documented in `.gitleaks.toml` with rationale.** Gitleaks remains active for all production source code, configuration files, and scripts.

#### Semgrep Configuration (`.semgrepignore` and `nosemgrep`)

Semgrep scans **487 files** (every file tracked by git except the marketing site). Test files, utility files, and all production code are scanned.

**`.semgrepignore` exclusion — 1 directory:**

| Excluded Path | Files | Reason |
|---------------|-------|--------|
| `site/` | 37 files | Completely separate Nuxt Content marketing website with its own `package.json`, dependencies, and deployment. Has no authentication, no database, no API. The `ProseCode.vue` component uses `v-html` to render Mermaid SVG diagrams generated by the Mermaid library from trusted markdown — not user input. Would require its own security review. |

**Files skipped by Semgrep's built-in 1 MB size limit — 8 files:**

| Skipped File | Size | Type | Security Relevance |
|-------------|------|------|--------------------|
| `backend/capacitarr` | ~15 MB | Compiled Go binary | None — binary artifact from local builds, not source code |
| `screenshots/*.png` (7 files) | 1-5 MB each | PNG images | None — documentation screenshots, not parseable code |

**Inline `nosemgrep` annotations — every suppressed finding with rationale:**

| File | Line | Semgrep Rule | Rationale |
|------|------|-------------|-----------|
| `backend/internal/testutil/testutil.go` | 167 | `go.jwt-go.security.jwt.hardcoded-jwt-key` | `TestJWTSecret` is a test-only constant used to sign JWT tokens in unit tests. It is never used in production code. |
| `backend/routes/auth.go` | 71 | `go.lang.security.audit.net.cookie-missing-secure` | The `Secure` flag is set to `cfg.SecureCookies` which evaluates to `true` when `SECURE_COOKIES=true`. Semgrep cannot evaluate runtime configuration. |
| `backend/routes/auth.go` | 85 | `cookie-missing-httponly`, `cookie-missing-secure` | The `authenticated` cookie is intentionally non-HttpOnly so the Vue SPA can detect auth state via JavaScript. It contains no secrets (just the string `"true"`). The JWT cookie (which holds the actual token) IS HttpOnly. `Secure` is conditional as above. |
| `backend/routes/middleware_test.go` | 130 | `go.jwt-go.security.jwt.hardcoded-jwt-key` | Test intentionally signs a JWT with the wrong secret (`"wrong-secret"`) to verify the middleware rejects tokens signed with incorrect keys. |
| `backend/routes/middleware_test.go` | 233 | `cookie-missing-httponly`, `cookie-missing-secure` | Test request attaches a JWT cookie to simulate browser behavior. HttpOnly/Secure are server-side attributes set when the cookie is issued by the login handler, not when the browser sends the cookie back. |
| `backend/internal/integrations/overseerr_test.go` | 204, 224 | `no-direct-write-to-responsewriter` | Mock HTTP server in test code returns canned JSON responses for Overseerr pagination testing. Not production code — `httptest.NewServer` handlers are test-only. |
| `backend/internal/services/version_test.go` | 26 | `no-direct-write-to-responsewriter` | Mock HTTP server returns canned GitLab release API responses for version check tests. Not production code. |
| `backend/routes/version_test.go` | 22 | `no-direct-write-to-responsewriter` | Mock HTTP server returns canned version API responses. Not production code. |
| `frontend/app/composables/useEventStream.ts` | 179 | `unsafe-formatstring` | Template literal in `console.warn` uses `eventType` which is an internal SSE event type name from the server's event bus, not user-supplied input. |

**Semgrep partial-parse warnings** (files that are scanned but where Semgrep's parser can't fully parse certain syntax):

| File | Parsed | Unparsed Lines | Reason |
|------|--------|----------------|--------|
| `Dockerfile` | ~45% | Lines 29-62 (multi-stage build args + `RUN` commands) | Semgrep's Dockerfile parser has limited support for complex `ARG`/`RUN` syntax. The important security-relevant parts (base images, package installs) are parsed. |
| `frontend/nuxt.config.ts` | ~99.5% | <1% of lines | Complex TypeScript config structure; nearly fully parsed. |
| `scripts/docker-build.sh` | ~97% | ~3% | Shell script parsing limitation. |
| `scripts/docker-mirror.sh` | ~94% | ~6% | Shell script parsing limitation. |

### Dynamic Application Security Testing (DAST)

In addition to static analysis, Capacitarr is tested with [OWASP ZAP](https://www.zaproxy.org/) — the industry-standard open-source DAST tool. ZAP makes real HTTP requests with attack payloads against a running instance, testing for the OWASP Top 10 and 50+ additional vulnerability categories.

Run locally: `make build && make security:zap`

**Latest baseline (2026-03-10):** 53 active scan rules tested, **52 PASS, 0 FAIL, 1 WARN**

| Category | Tests | Result |
|----------|-------|--------|
| SQL Injection (6 database engines) | 6 | ✅ All PASS |
| Cross-Site Scripting (Reflected, Persistent, DOM) | 5 | ✅ All PASS |
| Remote Code/Command Execution | 5 | ✅ All PASS |
| Server-Side Attacks (XXE, SSTI, SSRF, SOAP) | 6 | ✅ All PASS |
| Path Traversal & File Disclosure | 5 | ✅ All PASS |
| Known CVEs (Log4Shell, Spring4Shell) | 4 | ✅ All PASS |
| Infrastructure (Buffer Overflow, CRLF, Cloud Metadata) | 16 | ✅ All PASS |
| Authentication & Session | 3 | ✅ All PASS |
| Unexpected Content-Type (SPA fallback) | 1 | ⚠️ WARN (expected) |

The full test-by-test breakdown with rule IDs is in [`docs/security/zap-baseline-20260310.md`](docs/security/zap-baseline-20260310.md).

### Gosec G117 — JSON Secret Field Policy

Gosec rule [G117](https://securego.io/docs/rules/g117.html) flags exported struct fields whose JSON key names match secret patterns (`password`, `apiKey`, `token`, `secret`). The rule aims to prevent accidental serialization of sensitive data — for example, a secret leaking into logs when a struct is formatted with `%+v` or marshaled to JSON in an error response.

**How Capacitarr handles this:**

1. **All internal structs use `json:"-"` tags on secret fields.** This includes:
   - `config.Config.JWTSecret` — application configuration
   - `db.AuthConfig.Password` and `db.AuthConfig.APIKey` — user credentials
   - All integration client structs (`EmbyClient.APIKey`, `PlexClient.Token`, etc.)

   These fields are **never** serialized to JSON. The `json:"-"` tag is the correct structural fix for G117 and prevents accidental exposure regardless of how the struct is used.

2. **Three files are excluded from G117 via per-file linter exclusion.** These files define structs where secret-pattern JSON keys are part of the REST API contract:
   - `internal/db/models.go` — `IntegrationConfig.APIKey` (`json:"apiKey"`) is the user-configured integration credential. It is **masked** before inclusion in any API response; only the last 4 characters are visible.
   - `routes/auth.go` — `LoginRequest.Password` (`json:"password"`) accepts the user's password for authentication. This struct is decode-only and is never JSON-encoded.
   - `routes/integrations.go` — Connection test request accepts an API key. Decode-only, never JSON-encoded.

   These exclusions are defined in `backend/.golangci.yml` using path+text matching. G117 remains **active for all other files** — any new struct with a secret-pattern JSON key will be flagged and must be addressed with either a `json:"-"` tag or an explicit addition to the exclusion list after security review.

**Why not a global G117 exclusion?** A global exclusion would silently pass any future struct that accidentally exposes a secret field in JSON. The per-file approach ensures that each exemption is explicitly documented and the rest of the codebase remains protected.

### Container Hardening

The official Docker image uses a hardened Alpine runtime:

- **Alpine digest pinned:** The runtime base image is pinned to a specific SHA-256 digest for reproducible, auditable builds. The digest is updated periodically (or via Renovate Bot) to pick up security patches
- **Package manager removed:** `apk` is deleted after installing runtime dependencies (`ca-certificates`, `tzdata`, `su-exec`). An attacker with code execution cannot install additional tools
- **No curl/wget packages:** Healthchecks use busybox `wget` (built into Alpine's busybox), eliminating the `curl` package from the attack surface
- **Capabilities dropped:** `cap_drop: ALL` removes all Linux capabilities, then `cap_add` restores only the 4 needed by the PUID/PGID entrypoint: `CHOWN` (chown /config), `DAC_OVERRIDE` (create user in /etc/passwd), `SETUID`/`SETGID` (su-exec drops to PUID:PGID). The Go binary itself needs zero capabilities
- **No privilege escalation:** `no-new-privileges: true` prevents any child process from gaining privileges via setuid/setgid binaries
- **Non-root execution:** The `entrypoint.sh` creates a user with the configured PUID/PGID and uses `su-exec` to drop from root to that user before starting the application

**Optional additional hardening** for advanced users:

```yaml
# Add to your docker-compose.yml for maximum lockdown:
services:
  capacitarr:
    read_only: true        # Immutable root filesystem
    tmpfs:
      - /tmp:size=10M      # Writable temp directory in RAM
    user: "1001:1001"      # Fixed UID/GID (replaces PUID/PGID env vars)
```

> **Note:** `read_only: true` requires using `user:` instead of `PUID/PGID` because the PUID/PGID entrypoint writes to `/etc/passwd` at startup. The `/config` volume is always writable regardless of `read_only`.

### Important Caveats

- **`AUTH_HEADER` trust model:** When enabled, Capacitarr unconditionally trusts the configured header. The server **must** be behind a reverse proxy that sets this header. Direct internet exposure with `AUTH_HEADER` enabled allows authentication bypass
- **Single-user design:** Capacitarr does not implement role-based access control. All authenticated users have full access
- **Local network assumption:** The security model assumes the application runs on a trusted local network or behind a reverse proxy
