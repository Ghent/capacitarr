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
    image: capacitarr:latest
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
