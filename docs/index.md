---
title: Capacitarr
hideTitle: true
---

**Capacitarr** is an intelligent media library capacity manager. It monitors disk usage across your media servers and automatically identifies content for cleanup when storage runs low — using a preference-based scoring engine instead of rigid rules.

:icon{name="i-simple-icons-discord" class="size-4 align-middle text-[#5865F2]"} [Join our Discord](https://discord.gg/fbFkND5qgt){target="_blank"} · :icon{name="i-simple-icons-reddit" class="size-4 align-middle text-[#FF4500]"} [r/capacitarr](https://www.reddit.com/r/capacitarr/){target="_blank"}

## How It Works

1. **Connect your integrations** — Sonarr, Radarr, Lidarr, Readarr, Plex, Jellyfin, Emby, Tautulli, Jellystat, Tracearr, and Seerr (Overseerr/Jellyseerr).
2. **Disk groups are auto-detected** — Capacitarr tracks capacity per root folder across your integrations.
3. **Set a threshold** — choose when cleanup should trigger (e.g., disk ≥ 85%).
4. **Adjust preference sliders** — tell the engine what you value (watch history, file size, rating, etc.).
5. **Add protection rules** — mark content as untouchable based on quality, tags, genre, or any other property.
6. **Preview or automate** — see exactly what would be deleted before anything happens, or let the engine run automatically.

## Documentation

| Section | Description |
|---------|-------------|
| [Quick Start](getting-started/quick-start.md) | Get Capacitarr running in under 60 seconds |
| [Architecture](reference/architecture.md) | Service layer, event bus, SSE, and database schema |
| [Deployment Guide](getting-started/deployment.md) | Reverse proxy configuration, SSE proxy notes, subdirectory deployments, and authentication |
| [Configuration Reference](getting-started/configuration.md) | All environment variables with defaults and descriptions |
| [Scoring Algorithm](guides/scoring.md) | How items are ranked for deletion — factors, weights, rules, and tiebreakers |
| [API Documentation](reference/api/README.md) | REST API reference, examples, and workflows |
| [Release Workflow](releasing.md) | Semantic versioning, git-cliff changelog, and CI/CD release pipeline |

## Quick Start

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
    volumes:
      - capacitarr-config:/config
    restart: unless-stopped

volumes:
  capacitarr-config:
```

Then open `http://localhost:2187` in your browser to complete setup.

## About

**Capacitarr** is free, open-source software created by **Ghent Starshadow**.
Licensed under [PolyForm Noncommercial 1.0.0](https://github.com/Ghent/capacitarr/blob/main/LICENSE).
Built with Go, Nuxt 4, and SQLite.

---

> 🇺🇦 **I stand with Ukraine.** This project is built with the belief that freedom, sovereignty, and self-determination matter — for people and for software.

---

### 🐾 Support Animal Rescue

Capacitarr is free software. The creator strongly prefers that donations go to animal rescue over developer support:

- **[UAnimals](https://uanimals.org/en/)** — rescuing and protecting animals in Ukraine 🇺🇦
- **[ASPCA](https://www.aspca.org/ways-to-help)** — preventing cruelty to animals

If you still want to support development directly: [GitHub Sponsors](https://github.com/sponsors/ghent) · [Ko-fi](https://ko-fi.com/ghent) · [Buy Me a Coffee](https://buymeacoffee.com/ghentgames)

*Author: Ghent Starshadow*
