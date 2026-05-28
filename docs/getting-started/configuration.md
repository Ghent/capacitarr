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
      test: ["CMD", "wget", "-qO", "/dev/null", "http://localhost:2187/api/v1/health"]
      interval: 30s
      timeout: 5s
      start_period: 15s
      retries: 3
    security_opt:
      - no-new-privileges:true
    cap_drop:
      - ALL
    cap_add:
      - CHOWN
      - DAC_OVERRIDE
      - SETUID
      - SETGID
    restart: unless-stopped

volumes:
  capacitarr-config:
```

## Example: Subdirectory Deployment

When running behind a reverse proxy at a subdirectory (e.g., `https://example.com/capacitarr/`):

```yaml
environment:
  - BASE_URL=/capacitarr/
  - JWT_SECRET=change-me-to-a-random-string
  - SECURE_COOKIES=true
```

Only `BASE_URL` is needed — the frontend HTML is automatically rewritten at startup to use the correct asset paths, API base URL, and Vue Router base path.

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

### Settings Export & Import

Capacitarr provides a built-in settings export/import feature that lets you back up and restore your configuration without dealing with raw database files.

#### Exporting Settings

1. Navigate to **Settings** → **Backup & Restore**
2. Select which sections to include in the export:
   - **Preferences** — Scoring weights, default disk group mode, tiebreaker method
   - **Rules** — All custom protection rules
   - **Integrations** — Integration names, types, URLs, and enabled status
   - **Disk Groups** — Mount paths, threshold/target percentages, and per-group execution modes
   - **Notifications** — Channel names, types, event subscriptions, and Apprise tags
3. Click **Export** to download a JSON file

> **Tip:** A **Backup & Restore** shortcut on the Scoring Engine page navigates to Settings with the Rules section pre-selected, making rules-only backups a single click.

#### Importing Settings

1. Navigate to **Settings** → **Backup & Restore**
2. Upload a previously exported JSON file
3. Review the import preview showing which sections are available
4. Select which sections to import (you can choose a subset)
5. Click **Import** to apply the settings

#### What's Included and Excluded

For security, **sensitive credentials are always stripped** from exports:

| Section | Included | Excluded |
|---------|----------|----------|
| **Preferences** | All scoring weights, default disk group mode, tiebreaker | Internal IDs, timestamps |
| **Rules** | Field, operator, value, effect, enabled, integration reference | Internal IDs |
| **Integrations** | Name, type, URL, enabled status | **API keys** |
| **Disk Groups** | Mount path, threshold %, target %, per-group mode | Transient disk usage data |
| **Notifications** | Name, type, enabled, subscriptions, Apprise tags | **Webhook URLs** |

After importing integrations or notification channels, you will need to re-enter API keys and webhook URLs manually.

#### Rules-Only Backup

To back up just your custom rules, select only the **Rules** section during export. This produces a portable JSON file containing all your protection rules that can be imported into another Capacitarr instance. Integration references use human-readable names instead of IDs for cross-instance compatibility.

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

### Automatic Queue Clearing

When disk usage drops below the configured threshold, the approval queue is automatically cleared of all **pending** and **rejected** (snoozed) items. This ensures the queue only contains current, actionable deletion candidates — stale items from a previous threshold breach are removed rather than left for manual cleanup.

Items that have already been **approved** and are actively being processed for deletion are preserved and will complete normally.

When the threshold is breached again on a subsequent engine run, the scoring engine re-evaluates all media and populates the queue with fresh candidates based on current disk usage and media metadata.

## Real-Time Updates (SSE)

Capacitarr uses Server-Sent Events (SSE) to push real-time updates to all connected browser tabs. The SSE endpoint is `GET /api/v1/events` (authenticated).

When running behind a reverse proxy, ensure the proxy does not buffer responses for the SSE endpoint. See the [Deployment Guide](deployment.md#sse-server-sent-events-proxy-configuration) for proxy-specific configuration.

## Application Preferences

Beyond environment variables, Capacitarr has application-level preferences managed through the Settings UI or the `GET/PUT /preferences` and `PATCH /preferences/*` API endpoints. These are stored in the database and can be changed at runtime.

### Engine & Mode

| Setting | Default | Description |
|---------|---------|-------------|
| `defaultDiskGroupMode` | `dry-run` | Default execution mode for new disk groups: `dry-run`, `approval`, `auto`, `sunset` |
| `tiebreakerMethod` | `size_desc` | Tiebreaker when deletion scores are equal: `size_desc`, `size_asc`, `name_asc`, `oldest_first`, `newest_first` |
| `deletionsEnabled` | `true` | Safety guard — actual file deletions only occur when true. When false, all deletions are simulated. |
| `snoozeDurationHours` | `24` | Hours to snooze rejected approval queue items (1-720) |
| `deletionQueueDelaySeconds` | `30` | Grace period in seconds before processing queued deletions (10-300) |
| `pollIntervalSeconds` | `300` | Engine poll interval in seconds (minimum 60) |

### Sunset Mode

See the [Sunset Mode Guide](../guides/sunset-mode.md) for full documentation.

| Setting | Default | Description |
|---------|---------|-------------|
| `sunsetDays` | `30` | Countdown duration in days before sunset items are deleted (7-90) |
| `sunsetLabel` | `capacitarr-sunset` | Label/tag applied to media server items in the sunset queue |
| `savedLabel` | `capacitarr-saved` | Label/tag applied to items saved by popular demand |
| `posterOverlayStyle` | `countdown` | Poster overlay style: `off`, `countdown`, `simple` |
| `sunsetRescoreEnabled` | `true` | Enable daily re-scoring of sunset queue items |
| `savedDurationDays` | `7` | Days the "Saved by popular demand" marker persists (3-30) |

### Content Analytics

| Setting | Default | Description |
|---------|---------|-------------|
| `deadContentMinDays` | `90` | Minimum days in library for media to appear in dead content analytics (30-365) |
| `staleContentDays` | `180` | Days since last watch for media to appear in stale content analytics (60-365) |

### Data Management

| Setting | Default | Description |
|---------|---------|-------------|
| `logLevel` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `auditLogRetentionDays` | `30` | Days to keep audit log entries (0 = indefinite) |
| `backupRetentionDays` | `7` | Days to keep automatic database backups (3, 7, 14, 30) |
| `checkForUpdates` | `true` | Enable outbound update check to GitHub (cached 6 hours) |

### Disk Group Grace Period

| Setting | Default | Description |
|---------|---------|-------------|
| `diskGroupGracePeriodDays` | `7` | Days before stale disk groups are permanently removed (0 = immediate) |

Disk groups not reported by any integration are marked **stale** rather than deleted immediately. This protects against accidental data loss when integrations are temporarily disabled or unreachable. Stale groups retain all configuration (thresholds, mode, overrides) and are automatically restored when the mount path reappears. Set to `0` to restore the legacy immediate-deletion behavior.

### Data Retention

| Data | Retention | Configuration |
|------|-----------|---------------|
| Activity events | 7 days | Fixed (not configurable) |
| Audit log entries | Configurable | `auditLogRetentionDays` preference |
| Engine run stats | Last 1000 rows | Fixed |
| Metrics (raw) | 2 hours | Fixed — rolled up to hourly |
| Metrics (hourly) | 7 days | Fixed — rolled up to daily |
| Metrics (daily) | 30 days | Fixed — rolled up to weekly |
| Metrics (weekly) | 1 year | Fixed |
