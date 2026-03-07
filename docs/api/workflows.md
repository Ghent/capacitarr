# Common Workflows

Multi-step guides for typical Capacitarr API tasks. Each workflow combines several API calls in sequence.

## Setup

All workflows assume these environment variables are set:

```bash
export CAPACITARR_URL="http://localhost:2187/api/v1"
export CAPACITARR_API_KEY="your-api-key-here"
```

---

## Workflow 1: Initial Setup

Go from a fresh install to a working configuration with your first integration synced.

### Step 1: Verify the server is running

```bash
curl "$CAPACITARR_URL/health"
```

Expect the text `OK`. If the server is not reachable, check that the container is running and port 2187 is exposed.

### Step 2: Login to get a JWT

```bash
TOKEN=$(curl -s -X POST "$CAPACITARR_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"password":"your-password"}' | jq -r '.token')

echo "$TOKEN"
```

The default password is set during first-run setup. Save the token — you need it for the next step.

### Step 3: Generate an API key

```bash
API_KEY=$(curl -s -X POST "$CAPACITARR_URL/auth/apikey" \
  -H "Authorization: Bearer $TOKEN" | jq -r '.api_key')

echo "$API_KEY"
export CAPACITARR_API_KEY="$API_KEY"
```

Store this key securely. All remaining steps use the API key for authentication.

### Step 4: Add your first integration

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

Note the `id` in the response — you need it for the next steps.

### Step 5: Test the connection

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

A successful response confirms Capacitarr can reach your Sonarr instance. If it fails, verify the URL and API key.

### Step 6: Trigger the first engine run

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/engine/run" | jq
```

The engine will sync media data from your integrations, evaluate items against the scoring algorithm, and populate the dashboard. Check the worker status to monitor progress:

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/worker/stats" | jq
```

---

## Workflow 2: Configure Capacity Management

Set up thresholds, scoring weights, custom rules, and verify the configuration with a preview.

### Step 1: View disk groups

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/disk-groups" | jq
```

Identify the disk group you want to manage. Note its `id`, `totalBytes`, and `usedBytes` to understand current usage.

### Step 2: Set thresholds

Configure when the engine should activate (threshold) and how much space to free (target):

```bash
curl -s -X PUT -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/disk-groups/1" \
  -d '{"thresholdPct":90,"targetPct":80}' | jq
```

- **thresholdPct (90):** Engine activates when disk usage exceeds 90%
- **targetPct (80):** Engine removes media until usage drops to 80%

### Step 3: Configure scoring weights

Adjust how media is ranked for deletion. Higher weights give that factor more influence on the final score:

```bash
curl -s -X PUT -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/preferences" \
  -d '{
    "watchHistoryWeight": 10,
    "fileSizeWeight": 6,
    "ratingWeight": 5,
    "executionMode": "dry-run",
    "tiebreakerMethod": "size_desc"
  }' | jq
```

Start with `executionMode: "dry-run"` so nothing is deleted while you tune the configuration.

### Step 4: Add custom rules

Protect media that should never be deleted:

```bash
# Protect anything with "Star Wars" in the title
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/custom-rules" \
  -d '{
    "field": "title",
    "operator": "contains",
    "value": "Star Wars",
    "effect": "always_keep",
    "integrationId": null
  }' | jq
```

To see what fields and operators are available:

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/rule-fields" | jq
```

### Step 5: Preview the results

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/preview" | jq
```

Review the scored list. Protected items will have `"protected": true`. Items at the top have the highest deletion scores. Adjust weights and rules as needed, then re-check the preview.

---

## Workflow 3: Monitor and Review

Check system health, view statistics, and review what the engine has done.

### Step 1: Check worker status

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/worker/stats" | jq
```

Look for the current worker state, last run time, and any errors.

### Step 2: View dashboard stats

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/dashboard-stats" | jq
```

Key fields to check:
- `totalBytesReclaimed` — total disk space freed by the engine
- `totalItemsRemoved` — number of media items deleted
- `totalEngineRuns` — how many times the engine has executed
- `growthBytesPerWeek` — estimated weekly disk growth rate

### Step 3: Review the audit log

```bash
# Most recent 20 entries (history of deletions and dry-runs)
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/audit-log?limit=20&offset=0" | jq

# Grouped audit log entries
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/audit-log/grouped" | jq
```

The audit log records every deletion, dry-run, and dry-delete action taken by the engine.

### Step 4: Review the approval queue

If the engine runs in approval mode, check for items awaiting your review:

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/approval-queue?status=pending" | jq
```

### Step 5: Check recent activity

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/activity/recent?limit=50" | jq
```

Activity events capture all operational events (engine runs, config changes, logins) and are retained for 7 days.

### Step 6: Export metrics history

Pull historical disk usage data for analysis or external dashboards:

```bash
# Last 24 hours at hourly resolution
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/metrics/history?resolution=hourly&since=24h" | jq

# Last 30 days at daily resolution for a specific disk group
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/metrics/history?resolution=daily&since=30d&disk_group_id=1" | jq
```

---

## Workflow 4: Real-Time Monitoring with SSE

Subscribe to the Server-Sent Events stream to receive notifications as they happen, without polling.

### Step 1: Connect to the event stream

```bash
curl -N -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/events"
```

The connection stays open and events arrive as they occur:

```
id: 1741199820-001
event: engine_start
data: {"message":"Engine run started in approval mode","executionMode":"approval"}

id: 1741199825-002
event: engine_complete
data: {"message":"Engine run completed: evaluated 97, flagged 12","evaluated":97,"flagged":12}
```

### Step 2: Trigger actions while connected

In a separate terminal, trigger an engine run:

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/engine/run" | jq
```

You will see `manual_run_triggered`, `engine_start`, and `engine_complete` events arrive on the SSE connection in real-time.

### Step 3: Handle reconnection

If the connection drops, resume from the last received event ID:

```bash
curl -N -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Last-Event-ID: 1741199825-002" \
  "$CAPACITARR_URL/events"
```

The server replays any events that occurred between the last received ID and now (up to 100 events from the ring buffer).

### Supported event types

All 34 event types are documented in the [Architecture](../architecture.md#event-types-34-total) page.

---

## Workflow 5: Emergency — Stop Deletions

If the engine is actively deleting media and you need it to stop immediately, switch the execution mode to `dry-run`.

### Step 1: Set execution mode to dry-run

```bash
curl -s -X PUT -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/preferences" \
  -d '{"executionMode":"dry-run"}' | jq
```

This takes effect immediately. The engine will continue to score media but will not delete anything.

### Step 2: Verify the change

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/preferences" | jq '.executionMode'
```

Expect: `"dry-run"`

### Step 3: Review what happened

Check the audit log to see what was deleted before you intervened:

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/audit-log?limit=50&offset=0" | jq
```

### Step 4: Re-enable when ready

Once you have reviewed and adjusted your configuration, switch back to live mode:

```bash
curl -s -X PUT -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/preferences" \
  -d '{"executionMode":"live"}' | jq
```

---

## Workflow 6: Approval Queue Management

Work through items flagged by the engine in approval mode.

### Step 1: View pending items

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/approval-queue?status=pending" | jq
```

### Step 2: Approve items for deletion

```bash
# Approve a specific item
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/approval-queue/1/approve" | jq
```

The item will be queued for deletion by the deletion worker.

> **Note:** Approvals are blocked when `deletionsEnabled` is `false` in preferences. The server returns `409 Conflict` in this case.

### Step 3: Reject (snooze) items

```bash
# Reject a specific item (items are snoozed for a configurable duration)
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/approval-queue/1/reject" | jq
```

Rejected items are snoozed and will not appear again until the snooze period expires or they are manually unsnoozed.

### Step 4: Unsnooze a rejected item

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/approval-queue/1/unsnooze" | jq
```

The item returns to `pending` status in the queue.

---

## Workflow 7: Add a New Integration

Add a second media server (e.g., Radarr) to an existing Capacitarr instance.

### Step 1: Create the integration

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/integrations" \
  -d '{
    "type": "radarr",
    "name": "Radarr Movies",
    "url": "http://radarr:7878",
    "apiKey": "your-radarr-api-key",
    "enabled": true
  }' | jq
```

Note the `id` in the response (e.g., `2`).

### Step 2: Test the connection

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  -H "Content-Type: application/json" \
  "$CAPACITARR_URL/integrations/test" \
  -d '{
    "type": "radarr",
    "url": "http://radarr:7878",
    "apiKey": "your-radarr-api-key"
  }' | jq
```

A successful response confirms connectivity. If it fails, double-check the URL and API key, and ensure the Radarr instance is reachable from the Capacitarr container.

### Step 3: Trigger an engine run

```bash
curl -s -X POST -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/engine/run" | jq
```

The engine will sync media from the new integration and include it in scoring.

### Step 4: Verify media appears in preview

```bash
curl -s -H "X-Api-Key: $CAPACITARR_API_KEY" \
  "$CAPACITARR_URL/preview" | jq '.[0:5]'
```

You should see media from both Sonarr and Radarr in the scored list. If the new integration's media is missing, check that the integration is enabled and the engine run completed successfully.

---

## Workflow 8: Import/Export Rules

Portable rule import/export allows backing up, sharing, and migrating custom rule configurations between Capacitarr instances.

### Export Workflow

1. **Export rules** — `GET /custom-rules/export`
2. Save the JSON response to a file

The export replaces integration IDs with human-readable `integrationName` and `integrationType` fields so the file is portable across instances.

### Import Workflow

1. **Read export file** — parse the JSON
2. **Attempt import** — `POST /custom-rules/import` with the payload
3. **Handle unmapped integrations** — if the response is 400 with an `unmapped` array:
   a. List local integrations — `GET /integrations`
   b. Build an `integrationMapping` object mapping each unmapped `"type:name"` to a local integration ID
   c. Retry the import with the mapping included

Import is always **additive** — existing rules are never modified or deleted. Imported rules are appended after the last existing rule's sort order.

### Example: Full Migration

```bash
# 1. Export from source instance
curl -s http://source:2187/api/v1/custom-rules/export \
  -H "Authorization: Bearer $SOURCE_TOKEN" \
  -o rules-backup.json

# 2. Import into target instance
curl -s http://target:2187/api/v1/custom-rules/import \
  -H "Authorization: Bearer $TARGET_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"payload\": $(cat rules-backup.json)}"

# 3. If unmapped, check available integrations on target
curl -s http://target:2187/api/v1/integrations \
  -H "Authorization: Bearer $TARGET_TOKEN" | jq '.[] | {id, name, type}'

# 4. Retry with mapping
curl -s http://target:2187/api/v1/custom-rules/import \
  -H "Authorization: Bearer $TARGET_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "payload": '"$(cat rules-backup.json)"',
    "integrationMapping": {"sonarr:OldName": 5}
  }'
```
