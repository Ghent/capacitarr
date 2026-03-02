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
| `NUXT_APP_BASE_URL` | `/` | Frontend base URL path (build-time, must match `BASE_URL`) |

---

## Reverse Proxy Configuration

### Subdomain Deployment

The simplest approach — no `BASE_URL` configuration required.

#### Traefik (Docker labels)

```yaml
services:
  capacitarr:
    image: capacitarr:latest
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

When serving Capacitarr at a subdirectory (e.g. `https://example.com/capacitarr/`), set both `BASE_URL` and `NUXT_APP_BASE_URL`:

```yaml
services:
  capacitarr:
    image: capacitarr:latest
    environment:
      - BASE_URL=/capacitarr/
      - NUXT_APP_BASE_URL=/capacitarr/
      - JWT_SECRET=your-secret-here
      - SECURE_COOKIES=true
```

#### Traefik (Docker labels)

```yaml
labels:
  - "traefik.http.routers.capacitarr.rule=Host(`example.com`) && PathPrefix(`/capacitarr`)"
  - "traefik.http.middlewares.capacitarr-strip.stripprefix.prefixes=/capacitarr"
  - "traefik.http.routers.capacitarr.middlewares=capacitarr-strip"
```

> **Note:** Do NOT strip the prefix — Capacitarr handles the base URL path internally. Remove the `stripprefix` middleware if using `BASE_URL`.

#### Caddy

```
example.com {
    handle_path /capacitarr/* {
        reverse_proxy capacitarr:2187
    }
}
```

#### nginx

```nginx
location /capacitarr/ {
    proxy_pass http://127.0.0.1:2187/capacitarr/;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

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
    image: capacitarr:latest
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
