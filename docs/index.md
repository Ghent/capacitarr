# Capacitarr

**Capacitarr** is an intelligent media library capacity manager. It monitors disk usage across your media servers and automatically identifies content for cleanup when storage runs low — using a preference-based scoring engine instead of rigid rules.

## How It Works

1. **Connect your integrations** — Sonarr, Radarr, Lidarr, Readarr, Plex, Jellyfin, Emby, Tautulli, and Overseerr.
2. **Group services by shared disk** — Capacitarr tracks capacity per disk group.
3. **Set a threshold** — choose when cleanup should trigger (e.g., disk ≥ 85%).
4. **Adjust preference sliders** — tell the engine what you value (watch history, file size, rating, etc.).
5. **Add protection rules** — mark content as untouchable based on quality, tags, genre, or any other property.
6. **Preview or automate** — see exactly what would be deleted before anything happens, or let the engine run automatically.

## Documentation

| Section | Description |
|---------|-------------|
| [Quick Start](quick-start.md) | Get Capacitarr running in under 60 seconds |
| [Deployment Guide](deployment.md) | Reverse proxy configuration, subdirectory deployments, and authentication |
| [Configuration Reference](configuration.md) | All environment variables with defaults and descriptions |
| [Scoring Algorithm](scoring.md) | How items are ranked for deletion — factors, weights, rules, and tiebreakers |
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
