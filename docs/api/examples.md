# API Examples

Curl examples for every Capacitarr API endpoint.

## Setup

Set these environment variables before running the examples:

```bash
export CAPACITARR_URL="http://localhost:2187/api/v1"
export CAPACITARR_API_KEY="your-api-key-here"
```

All examples use the `X-Api-Key` header for authentication unless the endpoint is unauthenticated or requires a Bearer token.

---

## Health

### Check server health

```bash
curl "$CAPACITARR_URL/health"
```

```
OK
```

### Get version information

```bash
curl -s "$CAPACITARR_URL/version" | jq
```

```json
{
  "version": "1.2.0",
  "commit": "a1b2c3d",
  "buildDate": "2026-03-01T12:00:00Z"
}
```

---

## Auth

### Login

Obtain a JWT token. No authentication required.

```bash
curl -s -X POST "$CAPACITARR_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"password":"your-password"}' | jq
```

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs..."
}
```

The response also sets a `jwt` cookie for browser-based clients.

> **Rate limit:** 5 attempts per 15 minutes per IP.

### Change password

Requires Bearer token authentication.

```bash
curl -s -X PUT "$CAPACITARR_URL/auth/password" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"currentPassword":"old-password","newPassword":"new-password"}' | jq
```

### Change username

Requires Bearer token authentication.

```bash
curl -s -X PUT "$CAPACITARR_URL/auth/username" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username":"new-username"}' | jq
```

### Get API key status

Check whether an API key has been generated.

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/auth/apikey" | jq
```

### Generate API key

Creates a new API key, replacing any existing one. Requires Bearer token authentication.

```bash
curl -s -X POST "$CAPACITARR_URL/auth/apikey" \
  -H "Authorization: Bearer $TOKEN" | jq
```

```json
{
  "api_key": "ck_a1b2c3d4e5f6..."
}
```

> **Important:** Store this key securely — it cannot be retrieved again.

---

## Disk Groups

### List all disk groups

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/disk-groups" | jq
```

```json
[
  {
    "id": 1,
    "name": "media",
    "totalBytes": 2000000000000,
    "usedBytes": 1800000000000,
    "thresholdPct": 90,
    "targetPct": 80
  }
]
```

### Update a disk group

Set threshold and target percentages for a disk group.

```bash
curl -s -X PUT -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/disk-groups/1" \
  -d '{"thresholdPct":90,"targetPct":80}' | jq
```

---

## Dashboard

### Get dashboard stats

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/dashboard-stats" | jq
```

```json
{
  "totalBytesReclaimed": 524288000000,
  "totalItemsRemoved": 42,
  "totalEngineRuns": 15,
  "protectedCount": 128,
  "growthBytesPerWeek": 10737418240,
  "hasGrowthData": true
}
```

### Get lifetime stats

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/lifetime-stats" | jq
```

### Get worker stats

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/worker/stats" | jq
```

This is an alias for `/metrics/worker`.

### Get worker metrics

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/metrics/worker" | jq
```

---

## Metrics

### Get metrics history

Query historical disk usage metrics with configurable resolution and time range.

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/metrics/history?resolution=hourly&since=24h" | jq
```

**Query parameters:**

| Parameter | Values | Default | Description |
|-----------|--------|---------|-------------|
| `resolution` | `raw`, `hourly`, `daily` | `raw` | Aggregation level |
| `since` | `1h`, `24h`, `7d`, `30d` | — | Time range |
| `disk_group_id` | integer | — | Filter by disk group |

**Filter by disk group:**

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/metrics/history?resolution=daily&since=7d&disk_group_id=1" | jq
```

---

## Engine

### Trigger an engine run

Start the scoring engine manually. Returns immediately — the engine runs asynchronously.

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/engine/run" | jq
```

```json
{
  "status": "triggered"
}
```

If an engine run is already in progress or queued:

```json
{
  "status": "already_pending"
}
```

---

## Integrations

### List all integrations

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/integrations" | jq
```

```json
[
  {
    "id": 1,
    "type": "sonarr",
    "name": "Sonarr Main",
    "url": "http://sonarr:8989",
    "apiKey": "abc123...",
    "enabled": true,
    "lastSync": "2026-03-01T12:00:00Z"
  }
]
```

### Create an integration

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/integrations" \
  -d '{
    "type": "sonarr",
    "name": "Sonarr Main",
    "url": "http://sonarr:8989",
    "apiKey": "your-sonarr-api-key",
    "enabled": true
  }' | jq
```

### Get an integration

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/integrations/1" | jq
```

### Update an integration

```bash
curl -s -X PUT -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/integrations/1" \
  -d '{
    "type": "sonarr",
    "name": "Sonarr Main (updated)",
    "url": "http://sonarr:8989",
    "apiKey": "your-sonarr-api-key",
    "enabled": true
  }' | jq
```

### Delete an integration

```bash
curl -s -X DELETE -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/integrations/1" | jq
```

### Test an integration connection

Verify connectivity to an external service without saving the integration.

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/integrations/test" \
  -d '{
    "type": "sonarr",
    "url": "http://sonarr:8989",
    "apiKey": "your-sonarr-api-key"
  }' | jq
```

### Trigger integration sync

Pull the latest media data from an integration.

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/integrations/1/sync" | jq
```

---

## Rules (Protections)

### Get available rule fields

Returns the fields and operators you can use when creating custom rules.

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/rule-fields" | jq
```

```json
[
  {
    "field": "title",
    "type": "string",
    "operators": ["contains", "not_contains", "equals", "not_equals", "regex"]
  },
  {
    "field": "sizeBytes",
    "type": "number",
    "operators": ["gt", "lt", "gte", "lte", "equals"]
  }
]
```

### List all custom rules

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/custom-rules" | jq
```

```json
[
  {
    "id": 1,
    "field": "title",
    "operator": "contains",
    "value": "Star Wars",
    "effect": "protect",
    "integrationId": null
  }
]
```

### Create a protection rule

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/custom-rules" \
  -d '{
    "field": "title",
    "operator": "contains",
    "value": "Star Wars",
    "effect": "protect",
    "integrationId": null
  }' | jq
```

### Get a protection rule

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/custom-rules/1" | jq
```

### Update a protection rule

```bash
curl -s -X PUT -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/custom-rules/1" \
  -d '{
    "field": "title",
    "operator": "contains",
    "value": "Star Trek",
    "effect": "protect",
    "integrationId": null
  }' | jq
```

### Delete a protection rule

```bash
curl -s -X DELETE -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/custom-rules/1" | jq
```

---

## Preview

### Preview scored media

Returns all media items ranked by their composite deletion score. Items at the top of the list would be deleted first when the engine runs.

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/preview" | jq
```

```json
[
  {
    "title": "Example Show S01",
    "sizeBytes": 42949672960,
    "score": 87.5,
    "factors": {
      "age": 25.0,
      "size": 30.0,
      "lastWatched": 20.0,
      "popularity": 12.5
    },
    "protected": false,
    "integration": "Sonarr Main"
  }
]
```

> **Tip:** Pipe through `jq '.[:5]'` to see only the top 5 deletion candidates.

---

## Preferences

### Get current preferences

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/preferences" | jq
```

```json
{
  "weightAge": 25,
  "weightSize": 30,
  "weightLastWatched": 20,
  "weightPopularity": 12.5,
  "weightSeeding": 12.5,
  "executionMode": "dry-run",
  "tiebreakerMethod": "size",
  "engineSchedule": "0 3 * * *"
}
```

### Update preferences

```bash
curl -s -X PUT -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/preferences" \
  -d '{
    "weightAge": 25,
    "weightSize": 30,
    "weightLastWatched": 20,
    "weightPopularity": 12.5,
    "weightSeeding": 12.5,
    "executionMode": "dry-run",
    "tiebreakerMethod": "size",
    "engineSchedule": "0 3 * * *"
  }' | jq
```

---

## Audit

### List audit entries (paginated)

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/audit?limit=20&offset=0" | jq
```

```json
[
  {
    "id": 42,
    "action": "engine_run",
    "detail": "Removed 3 items, reclaimed 128 GB",
    "timestamp": "2026-03-01T03:00:00Z"
  }
]
```

**Pagination parameters:**

| Parameter | Default | Description |
|-----------|---------|-------------|
| `limit` | `20` | Number of entries to return |
| `offset` | `0` | Number of entries to skip |

### Get activity timeline

Returns data points for graphing activity over time.

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/audit/activity?days=30" | jq
```

### Get grouped audit entries

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/audit/grouped" | jq
```

---

## Data

### Reset application data

**Danger:** This deletes all application data including integrations, rules, metrics, and audit history. This action is irreversible.

```bash
curl -s -X DELETE -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/data/reset" | jq
```
