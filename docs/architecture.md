# Architecture

Capacitarr is a single-container application that bundles a Go backend, a Nuxt 4 (Vue 3) frontend, and a SQLite database. The frontend is statically generated at build time and embedded into the Go binary via `go:embed`, producing a single self-contained executable.

## High-Level Overview

```mermaid
flowchart TD
    subgraph CONTAINER["Docker Container"]
        FRONTEND["Nuxt 4 Frontend<br/>Vue 3 + Tailwind CSS 4 + shadcn-vue"]
        BACKEND["Go Backend<br/>Echo + GORM + Service Layer"]
        DB["SQLite Database<br/>/config/capacitarr.db"]
        ENGINE["Scoring Engine<br/>Weighted factors + protection rules"]
        POLLER["Engine Orchestrator<br/>Scheduled disk monitoring"]
        EVENT_BUS["Event Bus<br/>Typed pub/sub fan-out"]
        SSE["SSE Broadcaster<br/>Real-time event stream"]

        FRONTEND -->|"REST API + SSE"| BACKEND
        BACKEND --> DB
        BACKEND --> ENGINE
        BACKEND --> POLLER
        BACKEND --> EVENT_BUS
        EVENT_BUS --> SSE
        SSE -->|"Server-Sent Events"| FRONTEND
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

## Technology Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| **Frontend** | Nuxt 4, Vue 3, Tailwind CSS 4, shadcn-vue, Lucide, ApexCharts | Dashboard UI, rule builder, score visualization, real-time updates via SSE |
| **Backend** | Go, Echo, GORM | REST API, authentication, integration clients, scheduling |
| **Service Layer** | Go (custom) | Business logic, event publishing, dependency injection |
| **Event System** | Go (custom) | Typed event bus, activity persistence, SSE broadcast, notification dispatch |
| **Database** | SQLite | Configuration, approval queue, audit log, engine statistics |
| **Container** | Alpine Linux, multi-stage Docker build | Minimal runtime image (~30 MB) |

## Backend Architecture

### Service Layer

All business logic lives in the service layer (`backend/internal/services/`). Route handlers are thin — they parse requests, call services, and return responses.

```mermaid
flowchart TD
    subgraph HTTP_LAYER["HTTP Layer (thin handlers)"]
        ROUTES["Route Handlers<br/>Parse request → call service → return response"]
    end

    subgraph SERVICE_LAYER["Service Layer"]
        APPROVAL_SVC["ApprovalService<br/>Approve / Reject / Unsnooze<br/>BulkUnsnooze / CleanExpired"]
        DELETION_SVC["DeletionService<br/>Execute / DryRun / HandleFailure"]
        ENGINE_SVC["EngineService<br/>TriggerRun / GetStats"]
        SETTINGS_SVC["SettingsService<br/>UpdatePreferences / UpdateThresholds"]
        INTEGRATION_SVC["IntegrationService<br/>CRUD / Test / Sync"]
        AUTH_SVC["AuthService<br/>Login / ChangePassword / ChangeUsername / GenerateAPIKey"]
        AUDIT_SVC["AuditLogService<br/>Create / Upsert / Dedup"]
        NOTIF_CHANNEL_SVC["NotificationChannelService<br/>CRUD for notification channels"]
        NOTIF_DISPATCH_SVC["NotificationDispatchService<br/>Two-gate flush / digest + alerts"]
        DATA_SVC["DataService<br/>Data reset operations"]
        RULES_SVC["RulesService<br/>Custom rule CRUD + validation"]
        METRICS_SVC["MetricsService<br/>History / rollup / lifetime stats"]
        VERSION_SVC["VersionService<br/>Update check via GitLab releases"]
    end

    subgraph EVENT_SYSTEM["Event System"]
        BUS["EventBus<br/>Typed pub/sub"]
        ACTIVITY_SUB["ActivityPersister<br/>(subscriber → activity_events table)"]
        NOTIF_SUB["NotificationDispatcher<br/>(subscriber → Discord/Slack/InApp)"]
        SSE_SUB["SSEBroadcaster<br/>(subscriber → browser tabs)"]
    end

    subgraph DATA_LAYER["Data Layer"]
        DB["*gorm.DB<br/>(injected, not global)"]
    end

    ROUTES --> SERVICE_LAYER
    SERVICE_LAYER --> BUS
    SERVICE_LAYER --> DB
    BUS --> ACTIVITY_SUB
    BUS --> NOTIF_SUB
    BUS --> SSE_SUB
    ACTIVITY_SUB --> DB
```

### Service Registry

All services are instantiated in `main.go` and held in a `services.Registry` struct that is passed to route registration functions:

```go
type Registry struct {
    DB  *gorm.DB
    Bus *events.EventBus
    Cfg *config.Config

    Approval             *ApprovalService
    Deletion             *DeletionService
    AuditLog             *AuditLogService
    Engine               *EngineService
    Settings             *SettingsService
    Integration          *IntegrationService
    Auth                 *AuthService
    NotificationChannel  *NotificationChannelService
    NotificationDispatch *NotificationDispatchService
    Data                 *DataService
    Rules                *RulesService
    Metrics              *MetricsService
    Version              *VersionService
}
```

Each service receives a `*gorm.DB` and `*events.EventBus` via constructor injection — no global state.

### Event Bus

The event bus uses a fan-out pattern with one goroutine per subscriber and buffered channels.

```go
// Event is the interface all typed events implement.
type Event interface {
    EventType() string
    EventMessage() string
}

type EventBus struct {
    mu          sync.RWMutex
    subscribers []chan Event
}

func (b *EventBus) Publish(event Event)
func (b *EventBus) Subscribe() <-chan Event
func (b *EventBus) Unsubscribe(ch <-chan Event)
```

When a service performs an action (e.g., approving an item, completing an engine run), it publishes a typed event to the bus. Three subscribers react to every event:

1. **ActivityPersister** — writes the event to the `activity_events` table for the dashboard feed
2. **NotificationDispatcher** — filters events against notification channel subscriptions and delivers to Discord/Slack/in-app
3. **SSEBroadcaster** — serializes the event as an SSE message and pushes it to all connected browser tabs

### Notification Dispatch

The `NotificationDispatchService` uses a **two-gate flush pattern** to ensure cycle digest notifications contain complete data from both the evaluation phase and the deletion phase of an engine run.

```mermaid
flowchart LR
    ENGINE_COMPLETE["EngineCompleteEvent<br/>Gate 1: evaluation stats"]
    DELETION_BATCH["DeletionBatchCompleteEvent<br/>Gate 2: deletion stats"]
    FLUSH["Flush<br/>Build digest + dispatch"]
    CHANNELS["Discord / Slack"]
    INAPP["In-App<br/>always-on"]

    ENGINE_COMPLETE --> FLUSH
    DELETION_BATCH --> FLUSH
    FLUSH --> CHANNELS
    FLUSH --> INAPP
```

**Cycle digests** are batched summaries sent once per engine run. They include evaluated count, flagged count, deleted count, freed bytes, duration, and disk usage. The digest is only dispatched after both gates fire, ensuring deletion results are included.

**Instant alerts** fire immediately when their trigger event occurs — they are not batched. Alert types include engine errors, mode changes, server started, threshold breaches, update available, and approval activity.

**In-app notifications** are always-on: every digest and alert is automatically recorded to the in-app notification store regardless of whether the user has configured any external channels. Users do not need to create a channel for in-app notifications — they work out of the box. See [notifications.md](notifications.md) for the full user-facing guide, and [plans/20260307T1403Z-notification-overhaul.md](plans/20260307T1403Z-notification-overhaul.md) for the design details.

### Event Types (39 total)

| Category | Events |
|----------|--------|
| **Engine** | `engine_start`, `engine_complete`, `engine_error`, `manual_run_triggered` |
| **Settings** | `engine_mode_changed`, `settings_changed`, `threshold_changed`, `threshold_breached` |
| **Auth** | `login`, `password_changed`, `username_changed`, `api_key_generated` |
| **Integration** | `integration_added`, `integration_updated`, `integration_removed`, `integration_test`, `integration_test_failed` |
| **Approval** | `approval_approved`, `approval_rejected`, `approval_unsnoozed`, `approval_bulk_unsnoozed`, `approval_orphans_recovered` |
| **Deletion** | `deletion_success`, `deletion_failed`, `deletion_dry_run`, `deletion_batch_complete` |
| **Rules** | `rule_created`, `rule_updated`, `rule_deleted`, `rules_exported`, `rules_imported` |
| **Notifications** | `notification_channel_added`, `notification_channel_updated`, `notification_channel_removed`, `notification_sent`, `notification_delivery_failed` |
| **Data** | `data_reset` |
| **System** | `server_started`, `update_available` |

### SSE (Server-Sent Events)

The frontend connects to `GET /api/v1/events` (authenticated, long-lived HTTP connection) to receive real-time updates. This replaces the previous polling-based approach.

```
HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive

id: 1741199820-001
event: engine_start
data: {"message":"Engine run started in approval mode","executionMode":"approval"}

id: 1741199825-002
event: engine_complete
data: {"message":"Engine run completed: evaluated 97, flagged 12","evaluated":97,"flagged":12}
```

Key features:
- Auto-increment event IDs for replay support via `Last-Event-ID` header
- In-memory ring buffer (last 100 events) for reconnection replay
- Keepalive comments every 30 seconds to prevent proxy timeouts
- Auto-reconnect with exponential backoff on the client side

## Database Schema

The database uses two purpose-specific tables instead of a single overloaded table:

### Approval Queue

Active approval queue items with a state machine (`pending` → `approved`/`rejected`):

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER | Primary key |
| `media_name` | TEXT | Item title |
| `media_type` | TEXT | `movie`, `show`, `season`, `episode`, `artist`, `album`, `book` |
| `reason` | TEXT | Score explanation |
| `score_details` | TEXT | JSON-encoded score breakdown |
| `size_bytes` | INTEGER | File size |
| `integration_id` | INTEGER | FK to `integration_configs` (required) |
| `external_id` | TEXT | External ID in the integration |
| `status` | TEXT | `pending`, `approved`, `rejected` |
| `snoozed_until` | DATETIME | When snooze expires (rejected items) |
| `created_at` | DATETIME | Row creation |
| `updated_at` | DATETIME | Last state transition |

### Audit Log

Permanent, append-only history of deletions and dry-runs:

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER | Primary key |
| `media_name` | TEXT | Item title |
| `media_type` | TEXT | Media type |
| `reason` | TEXT | Score explanation |
| `score_details` | TEXT | JSON-encoded score breakdown |
| `action` | TEXT | `deleted`, `dry_run`, `dry_delete` |
| `size_bytes` | INTEGER | File size |
| `integration_id` | INTEGER | FK to `integration_configs` (nullable — preserved on integration delete) |
| `created_at` | DATETIME | Row creation |

### Activity Events

Transient dashboard feed with 7-day retention:

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER | Primary key |
| `event_type` | TEXT | One of 39 event types |
| `message` | TEXT | Human-readable message |
| `metadata` | TEXT | Optional JSON payload |
| `created_at` | DATETIME | Row creation |

## Frontend Architecture

### SSE Integration

The frontend uses a singleton `useEventStream` composable that maintains a single `EventSource` connection shared across all components:

```mermaid
flowchart LR
    SSE_ENDPOINT["GET /api/v1/events"]
    EVENT_SOURCE["EventSource<br/>(singleton)"]
    ENGINE_CTL["useEngineControl<br/>Engine state updates"]
    APPROVAL["useApprovalQueue<br/>Queue refresh"]
    DASHBOARD["index.vue<br/>Activity feed + stats"]
    BANNER["ConnectionBanner<br/>Connection state"]

    SSE_ENDPOINT --> EVENT_SOURCE
    EVENT_SOURCE --> ENGINE_CTL
    EVENT_SOURCE --> APPROVAL
    EVENT_SOURCE --> DASHBOARD
    EVENT_SOURCE --> BANNER
```

- `app.vue` initializes the SSE connection on mount when authenticated
- Components subscribe to specific event types and react accordingly
- Engine state, approval queue, and activity feed update in real-time without polling
- `ConnectionBanner.vue` uses SSE connection state as the primary health indicator

### Page Structure

| Page | Route | Purpose |
|------|-------|---------|
| Dashboard | `/` | Disk groups, approval queue, activity feed, engine controls, sparklines |
| Audit Log | `/audit` | History-only view (deleted, dry-run, dry-delete) |
| Rules | `/rules` | Cascading rule builder + disk threshold configuration |
| Settings | `/settings` | Preferences, integrations, notifications, authentication |
| Help | `/help` | Scoring guide, FAQ, about section |
| Login | `/login` | Authentication |

## Project Structure

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
│   │   ├── notifications/          # Discord, Slack, in-app senders + HTTP client
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
│   │   ├── types/                  # TypeScript type definitions
│   │   └── assets/css/             # Tailwind CSS + theme variables
│   └── nuxt.config.ts              # Nuxt configuration
├── site/                           # Project marketing site (Nuxt UI Pro)
├── docs/                           # Documentation
│   ├── api/                        # OpenAPI spec, examples, workflows
│   └── plans/                      # Internal plan documents
├── docker-compose.yml              # Development/deployment compose file
├── Dockerfile                      # Multi-stage build (Node → Go → Alpine)
└── Makefile                        # CI/CD targets (lint, test, security, build)
```
