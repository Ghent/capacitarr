# Capacitarr API

The Capacitarr REST API provides programmatic access to capacity management for media servers such as Sonarr, Radarr, and Plex. Use it to manage disk groups, libraries, integrations, protection rules, and the scoring engine that evaluates media for potential deletion when disk space runs low.

## Base URL

```
http://localhost:2187/api/v1
```

All requests and responses use `Content-Type: application/json` unless noted otherwise (the `/health` endpoint returns plain text).

## Authentication

The API supports three authentication methods. Choose the one that best fits your use case.

### API Key (recommended for CLI and scripts)

Pass your API key as a header or query parameter:

```bash
# Header (preferred)
curl -H "X-Api-Key: $CAPACITARR_API_KEY" "$CAPACITARR_URL/disk-groups"

# Query parameter
curl "$CAPACITARR_URL/disk-groups?apikey=$CAPACITARR_API_KEY"
```

Generate an API key through the web UI or via the auth endpoints (see the quick start below).

### Bearer JWT

Obtain a token from `POST /auth/login` and pass it in the `Authorization` header:

```bash
# Login
TOKEN=$(curl -s -X POST "$CAPACITARR_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"password":"your-password"}' | jq -r '.token')

# Use the token
curl -H "Authorization: Bearer $TOKEN" "$CAPACITARR_URL/disk-groups"
```

### Cookie JWT

The embedded web UI authenticates with a `jwt` cookie set by the login endpoint. This method is **not recommended** for external clients — use an API key or Bearer token instead.

## Quick Start

Go from zero to "preview what would be deleted" in five steps.

### Step 1: Check server health

```bash
curl http://localhost:2187/api/v1/health
# OK
```

A 200 response with the text `OK` confirms the server is running.

### Step 2: Login to get a JWT

```bash
curl -s -X POST http://localhost:2187/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"password":"your-password"}'
```

```json
{"token":"eyJhbGciOiJIUzI1NiIs..."}
```

Save the token for the next step.

### Step 3: Generate an API key

```bash
curl -s -X POST http://localhost:2187/api/v1/auth/apikey \
  -H "Authorization: Bearer <token-from-step-2>"
```

```json
{"api_key":"ck_a1b2c3d4e5f6..."}
```

Store this key securely — you will use it for all subsequent requests.

### Step 4: View disk groups

```bash
curl -s -H "X-Api-Key: <your-api-key>" \
  http://localhost:2187/api/v1/disk-groups
```

```json
[
  {
    "id": 1,
    "mountPath": "/mnt/media",
    "totalBytes": 2000000000000,
    "usedBytes": 1800000000000,
    "thresholdPct": 85,
    "targetPct": 75
  }
]
```

### Step 5: Preview scoring

```bash
curl -s -H "X-Api-Key: <your-api-key>" \
  http://localhost:2187/api/v1/preview
```

This returns scored media items with deletion candidates ranked by their composite score. Review the output to understand what the engine would remove.

## Error Handling

All error responses follow a consistent format:

```json
{"error": "message describing what went wrong"}
```

Common HTTP status codes:

| Status | Meaning |
|--------|---------|
| `200` | Success |
| `400` | Bad request — invalid input or parameters |
| `401` | Unauthorized — missing or invalid authentication |
| `404` | Not found — resource does not exist |
| `429` | Too many requests — rate limit exceeded |
| `500` | Internal server error |

## Rate Limiting

Authentication endpoints are rate-limited to prevent brute-force attacks:

- **Login** (`POST /auth/login`): 10 attempts per 15 minutes per IP address

When rate-limited, the server returns a `429 Too Many Requests` response. Wait for the window to reset before retrying.

## Real-Time Events (SSE)

Capacitarr supports Server-Sent Events for real-time updates. Connect to `GET /events` (authenticated) to receive a stream of typed events as they occur — engine runs, deletions, config changes, and more.

```bash
curl -N -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/events"
```

The SSE endpoint supports reconnection replay via the `Last-Event-ID` header. See the [Architecture](../architecture.md) documentation for the complete list of 39 event types.

## Endpoint Overview

| Group | Endpoints |
|-------|-----------|
| **Health** | `GET /health`, `GET /version` |
| **Auth** | `GET /auth/status`, `POST /auth/login`, `PUT /auth/password`, `PUT /auth/username`, `POST /auth/apikey`, `GET /auth/apikey` |
| **Libraries** | `GET /libraries`, `GET /libraries/:id`, `POST /libraries`, `PUT /libraries/:id`, `DELETE /libraries/:id` |
| **Disk Groups** | `GET /disk-groups`, `PUT /disk-groups/:id` |
| **Engine** | `POST /engine/run`, `GET /engine/history` |
| **Integrations** | `GET /integrations`, `GET /integrations/:id`, `POST /integrations`, `PUT /integrations/:id`, `DELETE /integrations/:id`, `POST /integrations/test`, `POST /integrations/sync` |
| **Rules** | `GET /custom-rules`, `POST /custom-rules`, `PUT /custom-rules/:id`, `DELETE /custom-rules/:id`, `PUT /custom-rules/reorder`, `GET /custom-rules/:id/impact`, `GET /custom-rules/:id/context`, `GET /rule-fields`, `GET /rule-values` |
| **Factor Weights** | `GET /scoring-factor-weights`, `PUT /scoring-factor-weights` |
| **Approval Queue** | `GET /approval-queue`, `POST /approval-queue/:id/approve`, `POST /approval-queue/:id/reject`, `POST /approval-queue/:id/unsnooze`, `DELETE /approval-queue/:id`, `POST /approval-queue/clear` |
| **Deletion Queue** | `GET /deletion-queue`, `DELETE /deletion-queue`, `POST /deletion-queue/snooze`, `POST /deletion-queue/clear`, `GET /deletion-queue/grace-period`, `POST /delete` |
| **Audit Log** | `GET /audit-log`, `GET /audit-log/recent`, `GET /audit-log/grouped` |
| **Activity** | `GET /activity/recent` |
| **SSE** | `GET /events` |
| **Preferences** | `GET /preferences`, `PUT /preferences` |
| **Preview** | `GET /preview` |
| **Metrics** | `GET /metrics/history`, `GET /worker/stats` |
| **Dashboard** | `GET /dashboard-stats`, `GET /lifetime-stats` |
| **Notifications** | `GET /notifications/channels`, `POST /notifications/channels`, `PUT /notifications/channels/:id`, `DELETE /notifications/channels/:id`, `POST /notifications/channels/:id/test` |
| **Analytics** | `GET /analytics/dead-content`, `GET /analytics/stale-content`, `GET /analytics/forecast` |
| **Migration** | `GET /migration/status`, `POST /migration/execute`, `POST /migration/dismiss` |
| **Settings Backup** | `GET /settings/export`, `POST /settings/import`, `POST /settings/import/preview`, `POST /settings/import/commit` |
| **Version Check** | `GET /version/check`, `POST /version/check` |
| **Data** | `DELETE /data/reset` |

## Further Reading

- [OpenAPI Specification](openapi.yaml) — full machine-readable API schema
- [API Examples](examples.md) — curl examples for every endpoint
- [Common Workflows](workflows.md) — multi-step guides for typical tasks
- [API Versioning](versioning.md) — version history, breaking changes, and compatibility notes
- [Architecture](../architecture.md) — service layer, event bus, and SSE documentation
