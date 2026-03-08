# Capacitarr

[![Pipeline](https://img.shields.io/gitlab/pipeline-status/starshadow%2Fsoftware%2Fcapacitarr?branch=main&logo=gitlab&label=pipeline)](https://gitlab.com/starshadow/software/capacitarr/-/pipelines)
[![Release](https://img.shields.io/gitlab/v/release/starshadow%2Fsoftware%2Fcapacitarr?logo=gitlab&label=release)](https://gitlab.com/starshadow/software/capacitarr/-/releases)
[![License](https://img.shields.io/badge/license-PolyForm%20NC%201.0-blue)](LICENSE)
[![Docker Hub](https://img.shields.io/docker/v/ghentstarshadow/capacitarr?label=Docker%20Hub&logo=docker&sort=semver)](https://hub.docker.com/r/ghentstarshadow/capacitarr)
[![Docker Pulls](https://img.shields.io/docker/pulls/ghentstarshadow/capacitarr?logo=docker&label=pulls)](https://hub.docker.com/r/ghentstarshadow/capacitarr)
[![Docker Image Size](https://img.shields.io/docker/image-size/ghentstarshadow/capacitarr?sort=semver&logo=docker&label=image%20size)](https://hub.docker.com/r/ghentstarshadow/capacitarr)
[![GHCR](https://img.shields.io/badge/GHCR-ghcr.io-blue?logo=github)](https://github.com/ghent/packages/container/package/capacitarr)
[![GitLab Registry](https://img.shields.io/badge/GitLab_Registry-orange?logo=gitlab)](https://gitlab.com/starshadow/software/capacitarr/container_registry)
[![Go](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![Nuxt](https://img.shields.io/badge/nuxt-4-00DC82?logo=nuxt&logoColor=white)](https://nuxt.com/)
[![GitLab Stars](https://img.shields.io/gitlab/stars/starshadow%2Fsoftware%2Fcapacitarr?logo=gitlab&label=stars)](https://gitlab.com/starshadow/software/capacitarr)
[![Discord](https://img.shields.io/badge/discord-join-5865F2?logo=discord&logoColor=white)](https://discord.gg/fbFkND5qgt)
[![Reddit](https://img.shields.io/badge/r%2Fcapacitarr-join-FF4500?logo=reddit&logoColor=white)](https://www.reddit.com/r/capacitarr/)

**Intelligent media library capacity manager for the *arr ecosystem.**

Capacitarr integrates with your *arr apps, media servers, and request managers to automatically manage disk capacity. When disk space runs low, it scores every media item across multiple dimensions — watch history, recency, file size, ratings, age, and series status — then removes the least-valuable items first. A visual rule builder lets you protect specific content from ever being deleted.

## Features

- **Intelligent Scoring Engine** — Six weighted factors rank every media item for deletion priority
- **Cascading Rule Builder** — Visual rule builder with `always_keep`, `prefer_keep`, `prefer_delete`, and `always_delete` actions
- **Multi-Integration Support** — Connects to Sonarr, Radarr, Lidarr, Readarr, Plex, Jellyfin, Emby, Overseerr, and Tautulli
- **Disk Group Monitoring** — Tracks capacity across multiple disk groups with configurable thresholds
- **Real-Time Updates** — Server-Sent Events (SSE) push engine state, deletions, and activity to the browser instantly — no polling
- **Approval Queue** — Review and approve deletion candidates before they are removed, with snooze and bulk operations
- **Score Transparency** — Full per-item score breakdowns showing each factor's contribution
- **Complete Audit Trail** — Separate approval queue (state machine) and audit log (permanent deletion history)
- **39 Activity Event Types** — Every user-visible action is tracked and streamable via SSE
- **Notifications** — Discord, Slack, and in-app notifications driven by the event bus
- **Themeable UI** — Light/dark mode with customizable accent colors, 22 languages
- **Reverse Proxy Ready** — Subdirectory deployments, proxy authentication (Authelia, Authentik, Organizr), SSE-compatible
- **Single Container** — Go backend + Nuxt 4 frontend + SQLite database in one Docker image
- **PUID/PGID Support** — Runs as any user/group for proper volume permissions

## Quick Start (Docker Compose)

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
      - JWT_SECRET=change-me-to-a-random-string
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

Start the container:

```bash
docker compose up -d
```

Open `http://localhost:2187` in your browser. On first launch, you will be prompted to create an admin account.

## Configuration

All configuration is done via environment variables. Every variable is optional — sensible defaults are used when not set.

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `2187` | HTTP listen port |
| `BASE_URL` | `/` | Base URL path for subdirectory reverse proxy deployments |
| `DB_PATH` | `/config/capacitarr.db` | SQLite database file path |
| `DEBUG` | `false` | Enable debug logging and permissive CORS |
| `JWT_SECRET` | *(auto-generated)* | Secret for signing JWT tokens. Set for persistent sessions across restarts |
| `SECURE_COOKIES` | `false` | Enable the `Secure` flag on cookies (set `true` for HTTPS) |
| `AUTH_HEADER` | *(none)* | Trusted reverse proxy auth header (e.g. `Remote-User`, `X-authentik-username`) |
| `CORS_ORIGINS` | *(none)* | Comma-separated allowed CORS origins |
| `PUID` | `1000` | User ID for the container process *(Docker only)* |
| `PGID` | `1000` | Group ID for the container process *(Docker only)* |

> **⚠️ AUTH_HEADER Security:** Only enable `AUTH_HEADER` when Capacitarr is exclusively accessible through your reverse proxy. If the server is directly reachable, any client can forge this header and bypass authentication.

For the complete configuration reference including subdirectory deployment and proxy authentication examples, see the [Configuration Guide](docs/configuration.md).

## Supported Integrations

### *arr Apps (Library Managers)

| Service | Type | Capabilities |
|---------|------|-------------|
| **Sonarr** | TV Shows | Disk space, media items, quality profiles, tags, deletion |
| **Radarr** | Movies | Disk space, media items, quality profiles, tags, deletion |
| **Lidarr** | Music | Disk space, media items, quality profiles, tags, deletion |
| **Readarr** | Books | Disk space, media items, quality profiles, tags, deletion |

### Media Servers (Watch Data)

| Service | Capabilities |
|---------|-------------|
| **Plex** | Play count, last played date, library metadata |
| **Jellyfin** | Play count, last played date, library metadata |
| **Emby** | Play count, last played date, library metadata |

### Enrichment Services

| Service | Capabilities |
|---------|-------------|
| **Tautulli** | Detailed play history and watch statistics for Plex |
| **Overseerr** | Request status, requester info, request counts |

## Architecture Overview

Capacitarr is a single-container application that bundles a Go backend, a Nuxt 4 (Vue 3) frontend, and a SQLite database. The frontend is statically generated at build time and embedded into the Go binary via `go:embed`, producing a single self-contained executable.

The backend uses a **service layer** pattern — all business logic lives in injectable services, not in route handlers. A typed **event bus** publishes 39 event types that drive activity logging, notifications, and real-time UI updates via **Server-Sent Events (SSE)**.

```mermaid
flowchart TD
    subgraph CONTAINER["Docker Container"]
        FRONTEND["Nuxt 4 Frontend<br/>Vue 3 + Tailwind CSS 4 + shadcn-vue"]
        BACKEND["Go Backend<br/>Echo + GORM + Service Layer"]
        DB["SQLite Database<br/>/config/capacitarr.db"]
        ENGINE["Scoring Engine<br/>Weighted factors + protection rules"]
        POLLER["Engine Orchestrator<br/>Scheduled disk monitoring"]
        EVENT_BUS["Event Bus<br/>Typed pub/sub fan-out"]

        FRONTEND -->|"REST API + SSE"| BACKEND
        BACKEND --> DB
        BACKEND --> ENGINE
        BACKEND --> POLLER
        BACKEND --> EVENT_BUS
        EVENT_BUS -->|"SSE"| FRONTEND
    end

    subgraph ARR_APPS["*arr Apps"]
        SONARR["Sonarr"]
        RADARR["Radarr"]
        LIDARR["Lidarr"]
        READARR["Readarr"]
    end

    subgraph MEDIA_SERVERS["Media Servers"]
        PLEX["Plex"]
        JELLYFIN["Jellyfin"]
        EMBY["Emby"]
    end

    subgraph ENRICHMENT["Enrichment"]
        TAUTULLI["Tautulli"]
        OVERSEERR["Overseerr"]
    end

    POLLER -->|"Fetch media + disk space"| ARR_APPS
    POLLER -->|"Fetch watch data"| MEDIA_SERVERS
    POLLER -->|"Fetch requests + history"| ENRICHMENT
    ENGINE -->|"Delete lowest-scored items"| ARR_APPS
```

| Layer | Technology | Purpose |
|-------|-----------|---------|
| **Frontend** | Nuxt 4, Vue 3, Tailwind CSS 4, shadcn-vue, Lucide, ApexCharts | Dashboard UI, rule builder, real-time updates via SSE |
| **Backend** | Go, Echo, GORM, Service Layer, Event Bus | REST API, SSE, authentication, integration clients, scheduling |
| **Database** | SQLite | Configuration, approval queue, audit log, engine statistics |
| **Container** | Alpine Linux, multi-stage Docker build | Minimal runtime image (~30 MB) |

For the full architecture documentation, see [docs/architecture.md](docs/architecture.md).

## Scoring Algorithm

Capacitarr uses a two-layer system to decide which items to remove:

1. **Preference-based scoring** — Each item is scored across six weighted factors (0–10 weight per factor). Higher score = more likely to be deleted.
2. **Protection rules** — Override scores with `always_keep`, `prefer_keep`, `prefer_delete`, or `always_delete` actions based on conditions like genre, tag, quality profile, or rating.

### Scoring Factors

| Factor | What It Measures | High Score Means |
|--------|-----------------|-----------------|
| **Watch History** | Play count | Unwatched → delete first |
| **Last Watched** | Time since last play | Watched long ago → delete first |
| **File Size** | Disk space consumed | Larger files → delete first |
| **Rating** | Community/critic rating | Low-rated → delete first |
| **Time in Library** | How long the item has been in the library | Older items → delete first |
| **Series Status** | Series status (continuing vs. ended) | Ended shows → delete first |

Each factor's contribution is normalized against the total configured weight, producing a final score between 0.0 (keep) and 1.0 (delete). Protection rules then apply modifiers or absolute overrides to the calculated score.

For the complete scoring algorithm documentation, see the [Scoring Guide](docs/scoring.md).

## Development Setup

Development uses Docker Compose to build and run the application in a container that mirrors production:

```bash
# Clone the repository
git clone https://gitlab.com/starshadow/software/capacitarr.git
cd capacitarr

# Build and start the container
docker compose up --build

# Or run in detached mode
docker compose up -d --build

# View logs
docker compose logs -f

# Tear down
docker compose down
```

The container exposes port **2187** and serves both the Go backend API and the Nuxt 4 frontend.

### Project Structure

```
capacitarr/
├── backend/                        # Go backend
│   ├── main.go                     # Application entrypoint, wiring
│   ├── internal/
│   │   ├── config/                 # Environment variable loading
│   │   ├── db/                     # SQLite models, single baseline migration
│   │   ├── engine/                 # Scoring + rule evaluation
│   │   ├── events/                 # Event bus, typed events, SSE broadcaster, activity persister
│   │   ├── integrations/           # *arr, Plex, Jellyfin, Emby, Overseerr, Tautulli clients
│   │   ├── jobs/                   # Cron scheduling (retention cleanup, time-series rollups)
│   │   ├── notifications/          # Discord, Slack, in-app notification dispatcher
│   │   ├── poller/                 # Engine orchestrator + deletion worker
│   │   ├── services/               # Service layer (business logic)
│   │   └── logger/                 # Structured logging
│   └── routes/                     # REST API handlers + middleware
├── frontend/                       # Nuxt 4 frontend
│   ├── app/
│   │   ├── components/             # Vue components (shadcn-vue based)
│   │   ├── composables/            # Vue composables (useEventStream, useEngineControl, etc.)
│   │   ├── pages/                  # Nuxt pages (dashboard, audit, rules, settings, help)
│   │   ├── locales/                # i18n translations (22 languages)
│   │   └── assets/css/             # Tailwind CSS + theme variables
│   └── nuxt.config.ts              # Nuxt configuration
├── site/                           # Project marketing site (Nuxt UI Pro)
├── docs/                           # Documentation
│   ├── api/                        # OpenAPI spec, examples, workflows
│   └── plans/                      # Internal plan documents
├── docker-compose.yml              # Development/deployment compose file
├── Dockerfile                      # Multi-stage build (Node → Go → Alpine)
├── Makefile                        # CI/CD targets (lint, test, security, build)
└── entrypoint.sh                   # Container entrypoint (PUID/PGID handling)
```

## Contributing

Contributions are welcome! Please read the [Contributing Guide](CONTRIBUTING.md) before submitting merge requests. All contributions are subject to the [Contributor License Agreement](CONTRIBUTING.md#contributor-license-agreement-cla).

### Quick Guidelines

- Follow [Conventional Commits](https://www.conventionalcommits.org/) for all commit messages
- Create feature branches from `main` (e.g. `feature/my-feature`, `fix/my-fix`)
- Ensure all tests pass before submitting

## Documentation

Full documentation is available on the [Capacitarr documentation site](https://capacitarr.app/).

Key documentation pages:

- [Architecture](docs/architecture.md) — Service layer, event bus, SSE, and database schema
- [Configuration Reference](docs/configuration.md) — All environment variables and examples
- [Deployment Guide](docs/deployment.md) — Reverse proxy, SSE proxy, subdirectory, and proxy auth setup
- [Scoring Algorithm](docs/scoring.md) — Detailed scoring factor documentation
- [API Reference](docs/api/README.md) — REST API and SSE endpoint documentation
- [Releasing](docs/releasing.md) — Release process and versioning

## Community

- [Discord](https://discord.gg/fbFkND5qgt) — Chat, support, and feature discussions
- [Reddit](https://www.reddit.com/r/capacitarr/) — r/capacitarr community

## License

Capacitarr is licensed under the [PolyForm Noncommercial 1.0.0](LICENSE) license.

You are free to use, modify, and distribute Capacitarr for any **noncommercial** purpose. See the [LICENSE](LICENSE) file for full terms.

## Author

**Ghent Starshadow** — [gitlab.com/starshadow](https://gitlab.com/starshadow)
