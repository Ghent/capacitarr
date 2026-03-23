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
  "version": "2.0.0",
  "commit": "a1b2c3d",
  "buildDate": "2026-03-06T12:00:00Z"
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

> **Rate limit:** 10 attempts per 15 minutes per IP.

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
    "mountPath": "/mnt/media",
    "totalBytes": 2000000000000,
    "usedBytes": 1800000000000,
    "thresholdPct": 85,
    "targetPct": 75
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

## Libraries

### List all libraries

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/libraries" | jq
```

```json
[
  {
    "id": 1,
    "name": "TV Shows",
    "diskGroupId": 1,
    "thresholdPct": 90,
    "targetPct": 80,
    "createdAt": "2026-03-06T12:00:00Z",
    "updatedAt": "2026-03-06T12:00:00Z"
  }
]
```

### Get a library

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/libraries/1" | jq
```

### Create a library

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/libraries" \
  -d '{
    "name": "Movies",
    "diskGroupId": 1,
    "thresholdPct": 85,
    "targetPct": 75
  }' | jq
```

### Update a library

```bash
curl -s -X PUT -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/libraries/1" \
  -d '{
    "name": "Movies (updated)",
    "thresholdPct": 90,
    "targetPct": 80
  }' | jq
```

### Delete a library

```bash
curl -s -X DELETE -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/libraries/1" | jq
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

### Get engine run history

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/engine/history?limit=20" | jq
```

```json
{
  "status": "success",
  "data": [
    {
      "id": 42,
      "runAt": "2026-03-06T12:00:00Z",
      "evaluated": 97,
      "flagged": 12,
      "deleted": 3,
      "freedBytes": 15032385536,
      "executionMode": "approval",
      "durationMs": 1234
    }
  ]
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
    "libraryId": 1,
    "collectionDeletion": false,
    "lastSync": "2026-03-06T12:00:00Z"
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
    "enabled": true,
    "libraryId": 1
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
    "enabled": true,
    "libraryId": 1
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

Pull the latest media data from all integrations.

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/integrations/sync" | jq
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
    "value": "Firefly",
    "effect": "always_keep",
    "integrationId": 1,
    "enabled": true,
    "sortOrder": 0
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
    "value": "Firefly",
    "effect": "always_keep",
    "integrationId": 1
  }' | jq
```

### Update a protection rule

```bash
curl -s -X PUT -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/custom-rules/1" \
  -d '{
    "field": "title",
    "operator": "contains",
    "value": "Firefly",
    "effect": "always_keep",
    "integrationId": 1
  }' | jq
```

### Reorder rules

```bash
curl -s -X PUT -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/custom-rules/reorder" \
  -d '{"ids":[3,1,2]}' | jq
```

### Delete a protection rule

```bash
curl -s -X DELETE -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/custom-rules/1" | jq
```

### Get rule impact analysis

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/custom-rules/1/impact" | jq
```

### Get rule context

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/custom-rules/1/context" | jq
```

---

## Factor Weights

### List scoring factor weights

Returns all scoring factors with current weights and metadata.

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/scoring-factor-weights" | jq
```

```json
[
  {
    "key": "watch_history",
    "name": "Watch History",
    "description": "Number of times the media has been watched",
    "weight": 10,
    "defaultWeight": 10
  },
  {
    "key": "file_size",
    "name": "File Size",
    "description": "Total size of media files",
    "weight": 6,
    "defaultWeight": 6
  }
]
```

### Update scoring factor weights

Pass a map of factor keys to weight values (0–10 scale).

```bash
curl -s -X PUT -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/scoring-factor-weights" \
  -d '{
    "watch_history": 10,
    "file_size": 6,
    "rating": 5,
    "time_in_library": 4
  }' | jq
```

---

## Settings Backup

### Export settings

```bash
# Export all settings to a JSON file
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/settings/export" \
  -o capacitarr-settings.json

# Export only rules and preferences
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/settings/export?sections=rules,preferences" \
  -o capacitarr-rules-prefs.json

# View the export without saving
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/settings/export" | jq .
```

### Import settings

```bash
# Import all sections from a settings export file
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/settings/import" \
  -d "{
    \"envelope\": $(cat capacitarr-settings.json),
    \"sections\": {
      \"preferences\": true,
      \"rules\": true,
      \"integrations\": true,
      \"diskGroups\": true,
      \"notifications\": true
    }
  }"

# Import only rules from an export file
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/settings/import" \
  -d "{
    \"envelope\": $(cat capacitarr-settings.json),
    \"sections\": {\"rules\": true}
  }"
```

> **Note:** Sensitive credentials (API keys, webhook URLs) are stripped from exports. After importing integrations or notification channels, re-enter credentials manually.

### Preview import

Preview what an import would change before applying.

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/settings/import/preview" \
  -d "{
    \"envelope\": $(cat capacitarr-settings.json),
    \"sections\": {
      \"preferences\": true,
      \"rules\": true
    }
  }" | jq
```

### Commit import

Apply a previously previewed import.

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/settings/import/commit" \
  -d "{
    \"envelope\": $(cat capacitarr-settings.json),
    \"sections\": {
      \"preferences\": true,
      \"rules\": true
    }
  }" | jq
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
    "title": "Firefly S01",
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
  "id": 1,
  "logLevel": "info",
  "auditLogRetentionDays": 30,
  "pollIntervalSeconds": 300,
  "executionMode": "dry-run",
  "tiebreakerMethod": "size_desc",
  "deletionsEnabled": true,
  "snoozeDurationHours": 24,
  "checkForUpdates": true,
  "deletionQueueDelaySeconds": 30,
  "deadContentMinDays": 90,
  "staleContentDays": 180,
  "updatedAt": "2026-03-06T12:00:00Z"
}
```

### Update preferences

```bash
curl -s -X PUT -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/preferences" \
  -d '{
    "logLevel": "info",
    "auditLogRetentionDays": 30,
    "pollIntervalSeconds": 300,
    "executionMode": "dry-run",
    "tiebreakerMethod": "size_desc",
    "deletionsEnabled": true,
    "snoozeDurationHours": 24,
    "checkForUpdates": true,
    "deletionQueueDelaySeconds": 30,
    "deadContentMinDays": 90,
    "staleContentDays": 180
  }' | jq
```

> **Note:** Scoring factor weights are managed separately via the `/scoring-factor-weights` endpoint.

---

## Approval Queue

### List approval queue items

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/approval-queue" | jq
```

```json
[
  {
    "id": 1,
    "mediaName": "Firefly S01",
    "mediaType": "season",
    "scoreDetails": "[{\"factor\":\"watchHistory\",\"rawScore\":1.0,\"weight\":10}]",
    "sizeBytes": 42949672960,
    "score": 87.5,
    "integrationId": 1,
    "externalId": "12345",
    "status": "pending",
    "trigger": "engine",
    "createdAt": "2026-03-06T12:00:00Z",
    "updatedAt": "2026-03-06T12:00:00Z"
  }
]
```

**Query parameters:**

| Parameter | Values | Default | Description |
|-----------|--------|---------|-------------|
| `status` | `pending`, `approved`, `rejected` | — | Filter by status |

### Approve an item

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/approval-queue/1/approve" | jq
```

> **Note:** Returns `409 Conflict` if deletions are disabled in settings. Enable deletions before approving items.

### Reject (snooze) an item

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/approval-queue/1/reject" | jq
```

### Unsnooze a rejected item

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/approval-queue/1/unsnooze" | jq
```

### Remove an item from the queue

```bash
curl -s -X DELETE -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/approval-queue/1" | jq
```

### Clear all approval queue items

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/approval-queue/clear" | jq
```

---

## Deletion Queue

### List deletion queue items

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/deletion-queue" | jq
```

### Cancel a deletion

Remove a specific item from the deletion queue by name. The item is snoozed in the approval queue.

```bash
curl -s -X DELETE -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/deletion-queue" \
  -d '{"mediaName":"Firefly S01","mediaType":"season"}' | jq
```

### Snooze a deletion queue item

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/deletion-queue/snooze" \
  -d '{"mediaName":"Firefly S01","mediaType":"season"}' | jq
```

### Clear the deletion queue

Cancel all pending deletions.

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/deletion-queue/clear" | jq
```

### Get grace period state

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/deletion-queue/grace-period" | jq
```

```json
{
  "active": true,
  "remainingSeconds": 15,
  "queueSize": 3
}
```

### Manually queue items for deletion

Queue one or more media items for deletion, bypassing the normal engine evaluation.

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/delete" \
  -d '[{
    "mediaName": "Serenity",
    "mediaType": "movie",
    "integrationId": 1,
    "externalId": "42",
    "sizeBytes": 5069636198,
    "posterUrl": "",
    "score": 85.0,
    "scoreDetails": "[]"
  }]' | jq
```

---

## Audit Log

The audit log stores a permanent, append-only history of deletions and dry-runs. It does not contain approval queue items — those live in the approval queue.

### List audit log entries (paginated)

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/audit-log?limit=20&offset=0" | jq
```

```json
{
  "data": [
    {
      "id": 42,
      "mediaName": "Firefly S01",
      "mediaType": "season",
      "action": "deleted",
      "sizeBytes": 42949672960,
      "score": 87.5,
      "trigger": "engine",
      "integrationId": 1,
      "createdAt": "2026-03-06T12:00:00Z"
    }
  ],
  "total": 150
}
```

**Pagination parameters:**

| Parameter | Default | Description |
|-----------|---------|-------------|
| `limit` | `20` | Number of entries to return |
| `offset` | `0` | Number of entries to skip |
| `action` | — | Filter by action: `deleted`, `dry_run`, `dry_delete` |

### Get recent audit log entries

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/audit-log/recent?limit=10" | jq
```

### Get grouped audit log entries

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/audit-log/grouped" | jq
```

---

## Activity Events

### Get recent activity events

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/activity/recent?limit=100" | jq
```

```json
[
  {
    "id": 1,
    "eventType": "engine_complete",
    "message": "Engine run completed: evaluated 97, flagged 12",
    "metadata": "{\"evaluated\":97,\"flagged\":12}",
    "createdAt": "2026-03-06T12:01:00Z"
  }
]
```

---

## SSE (Server-Sent Events)

### Subscribe to real-time events

Connect to the SSE endpoint for real-time event streaming. This is a long-lived HTTP connection.

```bash
curl -N -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/events"
```

```
id: 1741199820-001
event: engine_start
data: {"message":"Engine run started in approval mode","executionMode":"approval"}

id: 1741199825-002
event: engine_complete
data: {"message":"Engine run completed: evaluated 97, flagged 12","evaluated":97,"flagged":12}

id: 1741199826-003
event: deletion_success
data: {"message":"Deleted: Serenity (4.72 GB freed)","title":"Serenity","sizeBytes":5069636198}
```

To resume from a specific event after disconnection, pass the `Last-Event-ID` header:

```bash
curl -N -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Last-Event-ID: 1741199825-002" \
  "$CAPACITARR_URL/events"
```

See the [Architecture](../architecture.md) documentation for the complete list of 39 event types.

---

## Notifications

### List notification channels

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/notifications/channels" | jq
```

### Create a Discord notification channel

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/notifications/channels" \
  -d '{
    "type": "discord",
    "name": "My Discord",
    "webhookUrl": "https://discord.com/api/webhooks/...",
    "enabled": true,
    "onThresholdBreach": true,
    "onCycleDigest": true,
    "onError": true,
    "onModeChanged": true,
    "onServerStarted": true,
    "onUpdateAvailable": true,
    "onApprovalActivity": true,
    "onIntegrationStatus": true
  }' | jq
```

### Create an Apprise notification channel

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/notifications/channels" \
  -d '{
    "type": "apprise",
    "name": "Telegram via Apprise",
    "webhookUrl": "http://apprise:8000",
    "appriseTags": "telegram",
    "enabled": true,
    "onThresholdBreach": true,
    "onCycleDigest": true,
    "onError": true
  }' | jq
```

### Update a notification channel

```bash
curl -s -X PUT -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/notifications/channels/1" \
  -d '{
    "name": "My Discord (updated)",
    "enabled": true,
    "onCycleDigest": false
  }' | jq
```

### Delete a notification channel

```bash
curl -s -X DELETE -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/notifications/channels/1" | jq
```

### Test a notification channel

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/notifications/channels/1/test" | jq
```

---

## Analytics

### Get dead content

Returns media items that have never been watched since being added to the library.

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/analytics/dead-content" | jq

# With custom minimum days in library
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/analytics/dead-content?minDays=60" | jq

# Filter by disk group
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/analytics/dead-content?disk_group_id=1" | jq
```

### Get stale content

Returns media items that haven't been watched in a long time.

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/analytics/stale-content" | jq

# With custom stale threshold
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/analytics/stale-content?staleDays=120" | jq
```

### Get capacity forecast

Returns a capacity forecast based on linear regression of recent usage history.

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/analytics/forecast" | jq

# For a specific disk group
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/analytics/forecast?disk_group_id=1" | jq
```

---

## Migration

### Check migration status

Check whether a v1 database migration is available. This endpoint is public (no auth required).

```bash
curl -s "$CAPACITARR_URL/migration/status" | jq
```

### Execute migration

Perform the v1-to-v2 database migration.

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/migration/execute" | jq
```

### Dismiss migration

Dismiss the migration prompt without executing.

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/migration/dismiss" | jq
```

---

## Data

### Reset application data

**Danger:** This deletes all application data including integrations, rules, metrics, approval queue, and audit history. Lifetime stats and disk group thresholds are preserved. This action is irreversible.

```bash
curl -s -X DELETE -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/data/reset" | jq
```

---

## Version Check

### Check for updates

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/version/check" | jq
```

Results are cached for 6 hours.

### Force a fresh update check

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/version/check" | jq
```
