# Deployment Guide

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `2187` | HTTP listen port |
| `BASE_URL` | `/` | Base URL path for reverse proxy subdirectory deployments |
| `DB_PATH` | `/config/capacitarr.db` | SQLite database file path |
| `JWT_SECRET` | (auto-generated) | Secret for signing JWT tokens. Set for persistent sessions across restarts |
| `SECURE_COOKIES` | `false` | Set to `true` when serving over HTTPS |
| `CORS_ORIGINS` | (none) | Comma-separated CORS origins (e.g. `http://localhost:3000`) |
| `DEBUG` | `false` | Enable debug logging |
| `AUTH_HEADER` | (none) | Trusted reverse proxy authentication header name |
| `PUID` | `1000` | User ID for the container process (Docker only) |
| `PGID` | `1000` | Group ID for the container process (Docker only) |

---

## Reverse Proxy Configuration

### Subdomain Deployment

The simplest approach — no `BASE_URL` configuration required.

#### Traefik (Docker labels)

```yaml
services:
  capacitarr:
    image: ghcr.io/ghent/capacitarr:stable
    environment:
      - JWT_SECRET=your-secret-here
      - SECURE_COOKIES=true
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.capacitarr.rule=Host(`capacitarr.example.com`)"
      - "traefik.http.routers.capacitarr.tls=true"
      - "traefik.http.services.capacitarr.loadbalancer.server.port=2187"
```

#### Caddy

```
capacitarr.example.com {
    reverse_proxy capacitarr:2187
}
```

#### nginx

```nginx
server {
    server_name capacitarr.example.com;

    location / {
        proxy_pass http://127.0.0.1:2187;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

### Subdirectory Deployment

When serving Capacitarr at a subdirectory (e.g. `https://example.com/capacitarr/`), set `BASE_URL`:

```yaml
services:
  capacitarr:
    image: ghcr.io/ghent/capacitarr:stable
    environment:
      - BASE_URL=/capacitarr/
      - JWT_SECRET=your-secret-here
      - SECURE_COOKIES=true
```

Only `BASE_URL` is needed — the frontend HTML is automatically rewritten at startup to use the correct asset paths, API base URL, and Vue Router base path.

#### Traefik (Docker labels)

```yaml
labels:
  - "traefik.http.routers.capacitarr.rule=Host(`example.com`) && PathPrefix(`/capacitarr`)"
```

> **Note:** Do **not** use a `stripprefix` middleware with Capacitarr. When `BASE_URL` is set, the application expects to receive the full prefixed path and handles routing internally. Stripping the prefix causes asset and API route mismatches.

#### Caddy

```
example.com {
    handle /capacitarr/* {
        reverse_proxy capacitarr:2187
    }
}
```

> **Note:** Use `handle` (not `handle_path`). The `handle_path` directive strips the prefix before forwarding, but Capacitarr expects to receive the full prefixed path when `BASE_URL` is set.

#### nginx

```nginx
location /capacitarr/ {
    proxy_pass http://127.0.0.1:2187/capacitarr/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}

# SSE endpoint — disable buffering for real-time event streaming
location /capacitarr/api/v1/events {
    proxy_pass http://127.0.0.1:2187/capacitarr/api/v1/events;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Connection '';
    proxy_http_version 1.1;
    chunked_transfer_encoding off;
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 86400s;
}
```

---

## SSE (Server-Sent Events) Proxy Configuration

Capacitarr uses Server-Sent Events (SSE) for real-time updates via `GET /api/v1/events`. SSE requires that reverse proxies do **not** buffer responses — otherwise events will queue up and arrive in batches instead of streaming in real-time.

### nginx

Add `proxy_buffering off` and increase timeouts for the SSE endpoint:

```nginx
location /api/v1/events {
    proxy_pass http://127.0.0.1:2187;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Connection '';
    proxy_http_version 1.1;
    chunked_transfer_encoding off;
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 86400s;
}
```

### Caddy

Caddy handles SSE correctly by default — no special configuration is needed. Caddy does not buffer proxied responses, so events stream through immediately.

### Traefik

Traefik handles SSE correctly by default. If you have custom middleware that buffers responses, ensure it does not apply to the `/api/v1/events` path.

### Cloudflare

If you use Cloudflare in front of your reverse proxy, note that Cloudflare's free plan buffers HTTP responses. SSE connections will work but events may arrive with latency. To avoid this:

- Use a Cloudflare **orange cloud off** (DNS only) rule for the SSE path
- Or upgrade to a plan that supports streaming responses

---

## Proxy Authentication (Authelia / Authentik / Organizr)

If you run an authentication proxy (Authelia, Authentik, Organizr, etc.) in front of Capacitarr, you can configure it to trust the proxy's authentication header instead of requiring users to log in again.

Set `AUTH_HEADER` to the name of the header your auth proxy sets:

| Auth Proxy | Typical Header |
|-----------|---------------|
| Authelia | `Remote-User` |
| Authentik | `X-authentik-username` |
| Organizr | `X-WEBAUTH-USER` |

```yaml
services:
  capacitarr:
    image: ghcr.io/ghent/capacitarr:stable
    environment:
      - AUTH_HEADER=Remote-User
      - JWT_SECRET=your-secret-here
```

**How it works:**
1. When `AUTH_HEADER` is set and the configured header is present in a request, Capacitarr trusts the header value as the authenticated username
2. If the user doesn't exist in the database, Capacitarr auto-creates a user record
3. JWT validation is skipped entirely for requests with the trusted header
4. Built-in JWT authentication continues to work as a fallback

> **⚠️ Security:** Only enable `AUTH_HEADER` when Capacitarr is exclusively accessible through your auth proxy. If Capacitarr is also directly reachable, an attacker could forge the header and bypass authentication.

---

## Database Backups

Capacitarr stores all configuration, rules, audit history, and statistics in a single SQLite database file at `/config/capacitarr.db`. This is the only file you need to back up.

### Backup Methods

#### Simple File Copy

SQLite uses WAL (Write-Ahead Logging) mode, which allows safe file copies while the application is running:

```bash
# Copy the database file (safe while Capacitarr is running)
cp /path/to/config/capacitarr.db /path/to/backup/capacitarr-$(date +%Y%m%d).db
```

#### SQLite Backup Command

For a guaranteed-consistent backup, use the SQLite backup command:

```bash
docker exec capacitarr sqlite3 /config/capacitarr.db ".backup /config/capacitarr-backup.db"
```

### What's Included

The database contains:

- **Authentication:** User credentials and API keys
- **Integrations:** Connection URLs and API keys for \*arr apps, Plex, Tautulli, etc.
- **Preferences:** Scoring weights, execution mode, poll interval, thresholds
- **Custom Rules:** All user-defined keep/remove rules
- **Notification Channels:** Discord/Apprise webhook configurations
- **Audit Log:** History of all deletions and dry-runs
- **Approval Queue:** Pending, approved, and snoozed items
- **Engine Statistics:** Run history and lifetime counters
- **Disk Groups:** Detected mount points and threshold settings

### Backup Recommendations

- Back up before upgrading Capacitarr to a new version
- Schedule regular backups (daily or weekly) for production use
- Store backups in a separate location from the `/config` volume
- The database file is typically small (under 50 MB) even with extensive history
