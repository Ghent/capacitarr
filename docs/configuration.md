# Configuration Reference

Capacitarr is configured entirely through environment variables. All variables are optional — sensible defaults are used when a variable is not set.

## General

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `PORT` | `2187` | No | HTTP listen port for the Capacitarr server. |
| `BASE_URL` | `/` | No | Base URL path for reverse proxy subdirectory deployments. Must start and end with `/`. Example: `/capacitarr/`. |
| `DB_PATH` | `/config/capacitarr.db` | No | File path for the SQLite database. The directory must exist and be writable. |
| `DEBUG` | `false` | No | Enable debug logging. Set to `true` for verbose output. In debug mode, CORS defaults to `*` (allow all origins) and a static JWT secret is used. |

## Authentication

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `JWT_SECRET` | *(auto-generated)* | Recommended | Secret key for signing JWT authentication tokens. If not set, a random secret is generated at startup — sessions will not persist across container restarts. Set this for stable sessions. |
| `SECURE_COOKIES` | `false` | No | Set to `true` when serving Capacitarr over HTTPS. Marks authentication cookies with the `Secure` flag so they are only sent over encrypted connections. |
| `AUTH_HEADER` | *(none)* | No | Trusted reverse proxy authentication header name. When set, Capacitarr trusts this header for authentication instead of requiring JWT login. Common values: `Remote-User` (Authelia), `X-authentik-username` (Authentik), `X-WEBAUTH-USER` (Organizr). |

!!! warning "AUTH_HEADER Security"
    Only enable `AUTH_HEADER` when Capacitarr is **exclusively** accessible through your reverse proxy. If the server is directly reachable, any client can forge this header and bypass authentication entirely.

## CORS

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `CORS_ORIGINS` | *(none)* | No | Comma-separated list of allowed CORS origins. Example: `http://localhost:3000,https://app.example.com`. When not set: same-origin only (unless `DEBUG=true`, which allows all origins). |

## Frontend

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `NUXT_APP_BASE_URL` | `/` | No | Frontend base URL path. This is a **build-time** variable — it must match `BASE_URL` and is baked into the frontend at container build time. Only needed for subdirectory deployments. |

## Docker

These variables are handled by the container entrypoint script, not the Go backend.

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `PUID` | `1000` | No | User ID for the container process. Used to set file ownership on the `/config` volume. |
| `PGID` | `1000` | No | Group ID for the container process. Used to set file ownership on the `/config` volume. |

## Example: Docker Compose

```yaml
services:
  capacitarr:
    image: ghentstarshadow/capacitarr:stable
    container_name: capacitarr
    ports:
      - "2187:2187"
    environment:
      - PUID=1000
      - PGID=1000
      - JWT_SECRET=change-me-to-a-random-string
      - SECURE_COOKIES=true
      - DEBUG=false
    volumes:
      - capacitarr-config:/config
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:2187/api/v1/health"]
      interval: 30s
      timeout: 5s
      start_period: 15s
      retries: 3
    restart: unless-stopped

volumes:
  capacitarr-config:
```

## Example: Subdirectory Deployment

When running behind a reverse proxy at a subdirectory (e.g., `https://example.com/capacitarr/`):

```yaml
environment:
  - BASE_URL=/capacitarr/
  - NUXT_APP_BASE_URL=/capacitarr/
  - JWT_SECRET=change-me-to-a-random-string
  - SECURE_COOKIES=true
```

See the [Deployment Guide](deployment.md) for full reverse proxy configuration examples.

## Example: Proxy Authentication

When using Authelia, Authentik, or Organizr for authentication:

```yaml
environment:
  - AUTH_HEADER=Remote-User
  - JWT_SECRET=change-me-to-a-random-string
```

See the [Deployment Guide](deployment.md#proxy-authentication-authelia--authentik--organizr) for details on proxy authentication setup.

## Data & Backup

All persistent data is stored in a single SQLite database file:

| File | Default Path | Description |
|------|-------------|-------------|
| `capacitarr.db` | `/config/capacitarr.db` | All application data: integrations, rules, preferences, audit logs, engine stats, disk groups, notification channels, and authentication credentials. |

The `/config` directory (mapped via Docker volumes) is the **only directory that needs to be backed up**. No other files or directories contain user data.

### Backup Recommendations

- **Stop the container** before copying the database to ensure consistency: `docker compose stop && cp /path/to/volume/capacitarr.db backup.db && docker compose start`
- Alternatively, use [SQLite's `.backup` command](https://www.sqlite.org/cli.html#special_commands_to_sqlite3_dot_commands_) for online backups.
- The database path can be customized via the `DB_PATH` environment variable.

## Approval Queue

When the engine mode is set to **Approval**, items that meet deletion criteria are placed in the `approval_queue` table instead of being deleted automatically. A user must explicitly approve each item before deletion proceeds.

The approval queue is a separate table from the audit log. Items flow through a state machine: `pending` → `approved` → deleted (moved to `audit_log`), or `pending` → `rejected` (snoozed).

### Deletions-Disabled Safety Guard

When **Deletions Enabled** is turned off in **Settings → Advanced**, approving items from the approval queue is blocked. The approve action returns a `409 Conflict` error:

> Deletions are currently disabled in settings. Enable deletions before approving items.

Re-enable **Deletions Enabled** in Advanced settings before approving queued items. This prevents accidental approvals while the system is in a safe/paused state.

### Orphan Recovery

If the container restarts while items are in the **approved** (processing) state — meaning they were approved but not yet deleted — those items are automatically reverted to **pending** on startup. They reappear in the approval queue so no items are silently lost.

This recovery also runs at the start of each engine poll cycle as an additional safety measure. The `approval_orphans_recovered` activity event is published when orphans are detected and recovered.

## Real-Time Updates (SSE)

Capacitarr uses Server-Sent Events (SSE) to push real-time updates to all connected browser tabs. The SSE endpoint is `GET /api/v1/events` (authenticated).

When running behind a reverse proxy, ensure the proxy does not buffer responses for the SSE endpoint. See the [Deployment Guide](deployment.md#sse-server-sent-events-proxy-configuration) for proxy-specific configuration.

### Data Retention

| Data | Retention | Configuration |
|------|-----------|---------------|
| Activity events | 7 days | Fixed (not configurable) |
| Audit log entries | Configurable | `auditLogRetentionDays` preference |
| Engine run stats | Last 1000 rows | Fixed |
| In-app notifications | Same as audit log | Follows `auditLogRetentionDays` |
| Metrics time-series | Rolling rollups | Hourly/daily/weekly resolution |
