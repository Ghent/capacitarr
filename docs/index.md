# Capacitarr

[![Pipeline](https://img.shields.io/gitlab/pipeline-status/starshadow%2Fsoftware%2Fcapacitarr?branch=main&logo=gitlab&label=pipeline)](https://gitlab.com/starshadow/software/capacitarr/-/pipelines)
[![Release](https://img.shields.io/gitlab/v/release/starshadow%2Fsoftware%2Fcapacitarr?logo=gitlab&label=release)](https://gitlab.com/starshadow/software/capacitarr/-/releases)
[![License](https://img.shields.io/badge/license-PolyForm%20NC%201.0-blue)](https://gitlab.com/starshadow/software/capacitarr/-/blob/main/LICENSE)
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

**Capacitarr** is an intelligent media library capacity manager. It monitors disk usage across your media servers and automatically identifies content for cleanup when storage runs low — using a preference-based scoring engine instead of rigid rules.

## How It Works

1. **Connect your integrations** — Sonarr, Radarr, Lidarr, Readarr, Plex, Jellyfin, Emby, Tautulli, and Overseerr.
2. **Disk groups are auto-detected** — Capacitarr tracks capacity per root folder across your integrations.
3. **Set a threshold** — choose when cleanup should trigger (e.g., disk ≥ 85%).
4. **Adjust preference sliders** — tell the engine what you value (watch history, file size, rating, etc.).
5. **Add protection rules** — mark content as untouchable based on quality, tags, genre, or any other property.
6. **Preview or automate** — see exactly what would be deleted before anything happens, or let the engine run automatically.

## Documentation

| Section | Description |
|---------|-------------|
| [Quick Start](quick-start.md) | Get Capacitarr running in under 60 seconds |
| [Architecture](architecture.md) | Service layer, event bus, SSE, and database schema |
| [Deployment Guide](deployment.md) | Reverse proxy configuration, SSE proxy notes, subdirectory deployments, and authentication |
| [Configuration Reference](configuration.md) | All environment variables with defaults and descriptions |
| [Scoring Algorithm](scoring.md) | How items are ranked for deletion — factors, weights, rules, and tiebreakers |
| [API Documentation](api/README.md) | REST API reference, examples, and workflows |
| [Release Workflow](releasing.md) | Semantic versioning, git-cliff changelog, and CI/CD release pipeline |

## Quick Start

```yaml
services:
  capacitarr:
    image: capacitarr:latest
    ports:
      - "2187:2187"
    environment:
      - JWT_SECRET=your-secret-here
    volumes:
      - capacitarr-config:/config
    restart: unless-stopped

volumes:
  capacitarr-config:
```

Then open `http://localhost:2187` in your browser to complete setup.

---

*Author: Ghent Starshadow*
