# Quick Start

Get Capacitarr running in under 60 seconds.

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/) installed
- At least one *arr app (Sonarr, Radarr, Lidarr, or Readarr) running and accessible

## 1. Create the Compose File

Create a `docker-compose.yml` file:

```yaml
services:
  capacitarr:
    image: ghentstarshadow/capacitarr:stable
    # Or use an alternative registry:
    #   image: ghcr.io/ghent/capacitarr:stable
    #   image: registry.gitlab.com/starshadow/software/capacitarr:stable
    container_name: capacitarr
    ports:
      - "2187:2187"
    environment:
      - PUID=1000
      - PGID=1000
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

## 2. Start the Container

```bash
docker compose up -d
```

## 3. Create Your Account

Open `http://localhost:2187` in your browser. On first launch you will be prompted to create an admin account with a username and password.

## 4. Connect Your First Integration

1. Navigate to **Settings** → **Integrations**
2. Click **Add Integration**
3. Select your *arr app type (e.g., Sonarr)
4. Enter the URL and API key for your *arr instance
5. Test the connection and save

## 5. Configure Disk Thresholds

Capacitarr automatically detects disk groups from the root folders reported by your *arr integrations. No manual setup is needed — disk groups appear on the Dashboard as soon as integrations are connected and the engine runs.

To configure when cleanup triggers:

1. Navigate to the **Rules** page → **Disk Thresholds** section
2. Set a **threshold** — the disk usage percentage that triggers cleanup evaluation (e.g., 85%)
3. Set a **target** — the disk usage percentage the engine tries to reach (e.g., 75%)

## 6. Tune Your Weights

Navigate to the **Weights** page and adjust the scoring sliders to tell Capacitarr what matters to you:

- **Watch History** — Unwatched content scores higher for deletion
- **Last Watched** — Content watched long ago (or never) scores higher
- **File Size** — Larger files score higher, freeing more space per deletion
- **Rating** — Lower rated content scores higher
- **Time in Library** — Older content scores higher for deletion
- **Series Status** — Ended shows score higher for deletion than continuing shows

## 7. Preview & Run

Go back to the **Dashboard** and click **Run Engine** (in dry-run mode by default). Capacitarr will score every item and show you what would be cleaned up — without actually deleting anything.

When you're happy with the results, switch to approval or auto mode in the engine settings.

---

## Environment Variables

All environment variables are optional — sensible defaults are used when not set.

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `2187` | HTTP listen port |
| `PUID` | `1000` | Container process user ID |
| `PGID` | `1000` | Container process group ID |
| `JWT_SECRET` | *(auto-generated)* | Secret for JWT auth tokens (see below) |
| `SECURE_COOKIES` | `false` | Set `true` when using HTTPS |
| `DB_PATH` | `/config/capacitarr.db` | SQLite database location |

### About JWT_SECRET

Capacitarr uses JWT (JSON Web Tokens) for session authentication. When you don't set `JWT_SECRET`, a random secret is generated at container startup — this means **all user sessions are invalidated every time the container restarts**.

For persistent sessions, set `JWT_SECRET` to any random string:

```bash
# Generate a secure random secret
openssl rand -hex 32
```

Then add it to your compose file:

```yaml
environment:
  - JWT_SECRET=your-generated-secret-here
```

This is purely a session-signing key — it is not a password and is never exposed to users.

---

## Next Steps

- [Deployment Guide](deployment.md) — Reverse proxy setup, subdirectory deployments, proxy authentication
- [Configuration Reference](configuration.md) — All environment variables
- [Scoring Algorithm](scoring.md) — How the scoring engine works
