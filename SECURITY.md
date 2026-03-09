# Security Policy

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
- **Security headers:** `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`

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

### Important Caveats

- **`AUTH_HEADER` trust model:** When enabled, Capacitarr unconditionally trusts the configured header. The server **must** be behind a reverse proxy that sets this header. Direct internet exposure with `AUTH_HEADER` enabled allows authentication bypass
- **Single-user design:** Capacitarr does not implement role-based access control. All authenticated users have full access
- **Local network assumption:** The security model assumes the application runs on a trusted local network or behind a reverse proxy
