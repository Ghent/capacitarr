# v1.x Feature Batch — Implementation Plan

**Date:** 2026-03-09
**Status:** 📋 Planning
**Branch:** `feature/v1x-feature-batch`

---

## Overview

This plan covers seven features and two audit phases, derived from the codebase audit and roadmap discussion (`docs/plans/20260309T0015Z-codebase-audit-and-roadmap.md`). All work happens on a single feature branch.

### Guiding Principles

- **No backwards compatibility concerns.** There are no production users yet. However, we keep database migrations additive and non-destructive so existing databases upgrade cleanly (no data loss for early adopters).
- **Service layer compliance.** All data access goes through services. No direct DB access from routes, middleware, orchestrators, or subscribers.
- **Clean removal.** When removing features (Slack, rules portability), delete all related code, tests, validation, constants, and documentation. No dead code, no stubs, no "deprecated" markers.
- **Conventional commits.** Every commit follows the conventional commits format for git-cliff changelog generation.

### Features

| Phase | Feature | Summary |
|-------|---------|---------|
| 1 | Deletion worker progress (SSE) | Wire existing atomic counters into SSE events, update sparkline in real-time, remove dead REST-only polling code |
| 2 | Apprise notification support + Slack removal | New `"apprise"` channel type, remove Slack sender and all related code |
| 3 | Settings export/import | Full config backup with section-level granularity, supersedes rules-only portability |
| 4 | Watchlist enrichment | Plex/Jellyfin/Emby watchlist → `OnWatchlist` field + custom rule support |
| 5 | Collection autocomplete | Populate rule builder value field from actual Plex collections |
| 6 | Mobile PWA | manifest.json + service worker via Nuxt PWA module |
| 7 | Per-integration scoring overrides (frontend mockup) | Non-functional UI toggle + inline sliders for UX evaluation |
| 8 | Test audit | Audit all tests, remove dead/wasteful tests, ensure coverage |
| 9 | Documentation audit | Update all project and site docs to reflect new functionality |

### Migration Strategy

All existing migrations (`00001` through `00006`) remain untouched. New schema changes are added as new migration files. Migrations are additive and non-destructive:
- `ALTER TABLE ... ADD COLUMN` with defaults for new columns
- `DELETE FROM ... WHERE` for rows referencing removed features (e.g., Slack channels)
- Never modify or drop existing tables that contain user data

---

## Phase 0: Branch Setup

### Step 0.1: Create feature branch

```bash
git checkout main
git pull
git checkout -b feature/v1x-feature-batch
```

Verify clean working tree with `git status` before branching.

---

## Phase 1: Deletion Worker Progress (SSE)

### Context

`DeletionService` already tracks `currentlyDeleting`, `batchExpected`, `batchProcessed`, `batchSucceeded`, and `batchFailed` via atomics. The frontend currently polls `GET /worker/stats` via REST. This phase wires those counters into the SSE event stream and uses the SSE data to update both a progress indicator and the dashboard sparkline in real-time.

**Sparkline note:** The engine history sparklines are fed by `GET /engine/history`, which reads `EngineRunStats` rows from the database. The `Deleted` count on each row is incrementally updated by the deletion worker via `IncrementDeletedStats()`. However, the frontend only fetches this data on page load or when an `engine_complete` SSE event triggers a refresh — so during an active deletion batch, the sparkline's "Deleted" count is stale. This phase fixes that by having the frontend patch the sparkline's most recent data point with the `succeeded` count from the `DeletionProgressEvent` SSE stream.

### Steps

#### Step 1.1: Add `DeletionProgressEvent` to `events/types.go`

Add a new event type following the established pattern in `events/types.go`:

```go
type DeletionProgressEvent struct {
    CurrentItem string `json:"currentItem"`
    QueueDepth  int    `json:"queueDepth"`
    Processed   int    `json:"processed"`
    Succeeded   int    `json:"succeeded"`
    Failed      int    `json:"failed"`
    BatchTotal  int    `json:"batchTotal"`
}
```

Implement `EventType()` returning `"deletion_progress"` and `EventMessage()` with a descriptive string.

#### Step 1.2: Publish `DeletionProgressEvent` from the deletion worker

In `DeletionService.processJob()` (in `services/deletion.go`), after each job completes (success or failure), publish the new event via `s.bus.Publish()`. The event payload is built from the existing atomic counters plus `len(s.queue)` for queue depth.

#### Step 1.3: Add frontend SSE handler for `deletion_progress`

In the frontend's `useEventStream` composable, register a handler for the `deletion_progress` event type. Create a reactive store (or extend the existing engine status store) to hold the deletion progress state.

#### Step 1.4: Add deletion progress indicator to the dashboard

Add a subtle progress indicator (small progress bar or text) below the engine status card that appears when `batchTotal > 0` and `processed < batchTotal`. Auto-hides when the batch completes.

#### Step 1.5: Update sparkline with real-time deletion data

When the frontend receives a `deletion_progress` SSE event, patch the most recent data point in the sparkline array:
- Set `lastPoint.deleted = event.succeeded`
- The sparkline re-renders with the updated count

When the batch completes (signaled by `engine_complete` or `deletion_batch_complete` SSE event), trigger a full re-fetch from `GET /engine/history` to reconcile with the authoritative DB values.

#### Step 1.6: Remove dead REST-only worker stats polling code

Identify and remove frontend polling code that was replaced by the SSE approach:
- Any frontend `setInterval` or polling composable that fetches `GET /worker/stats` for deletion progress display
- Any unused helper methods on `DeletionService` that only existed to serve the REST endpoint

**Note:** Keep the `GET /worker/stats` REST endpoint itself — it's still useful for API consumers and the engine status card. Only remove the frontend *polling* of it for deletion progress, since SSE now handles that.

#### Step 1.7: Write tests for `DeletionProgressEvent`

- Unit test in `events/` verifying `EventType()` and `EventMessage()` return correct values
- Unit test in `services/deletion_test.go` verifying that `processJob()` publishes `DeletionProgressEvent` with correct counters
- Follow the existing in-memory SQLite test pattern used throughout `services/*_test.go`

#### Step 1.8: Run `make ci` and fix any issues

---

## Phase 2: Apprise Notification Support + Slack Removal

### Context

The notification system uses a `Sender` interface with `SendDigest()` and `SendAlert()` methods. `DiscordSender` and `SlackSender` currently implement this. This phase:
1. **Adds Apprise** as a new channel type, letting users reach 80+ notification services (Telegram, Matrix, Pushover, Ntfy, Gotify, Email, etc.) through a single Apprise server.
2. **Removes Slack** entirely. Slack is primarily enterprise/workplace and has minimal usage in the selfhosted community. Apprise supports Slack natively, so users who need Slack can route through Apprise.
3. **Keeps Discord** with its native rich embed formatting, which is significantly better than what Apprise produces for Discord.

### Steps

#### Step 2.1: Remove all Slack code

Delete the following files and code:
- `notifications/slack.go` — SlackSender implementation
- `notifications/slack_test.go` — SlackSender tests
- `"slack"` entry from `ValidNotificationChannelTypes` in `db/validation.go`
- `"slack"` entry from the sender map in `NewNotificationDispatchService()` in `services/notification_dispatch.go`
- `notifTypeSlack` constant from `routes/notifications.go`
- Any Slack-specific validation logic in `routes/notifications.go`
- Any Slack references in frontend notification channel forms

#### Step 2.2: Add `"apprise"` to `ValidNotificationChannelTypes`

In `db/validation.go`, update the `ValidNotificationChannelTypes` map:

```go
var ValidNotificationChannelTypes = map[string]bool{
    "discord": true,
    "apprise": true,
}
```

#### Step 2.3: Add Apprise-specific fields to `NotificationConfig` model

Add a new field to `NotificationConfig` in `db/models.go`:

```go
AppriseTags string `gorm:"default:''" json:"appriseTags,omitempty"` // Comma-separated Apprise tags for routing
```

Update the model comment to reflect the new channel types:

```go
// NotificationConfig stores a configured notification channel (Discord or Apprise).
```

#### Step 2.4: Create database migration

Create `00007_apprise_and_remove_slack.sql`:

```sql
-- +goose Up
ALTER TABLE notification_configs ADD COLUMN apprise_tags TEXT NOT NULL DEFAULT '';
DELETE FROM notification_configs WHERE type = 'slack';

-- +goose Down
ALTER TABLE notification_configs DROP COLUMN apprise_tags;
```

#### Step 2.5: Create `AppriseSender` in `notifications/apprise.go`

Implement the `Sender` interface. The Apprise API accepts:

```json
{
  "title": "string",
  "body": "string",
  "type": "info|success|warning|failure",
  "tag": "optional,comma,separated"
}
```

Methods:
- `SendDigest(webhookURL string, digest CycleDigest) error` — Format the digest as a Markdown message (title, stats, disk bar, update banner) and POST to `{webhookURL}/api/notify/`
- `SendAlert(webhookURL string, alert Alert) error` — Format the alert as Markdown and POST with `type` mapped from `AlertType`

**Tags handling:** The `Sender` interface passes only `webhookURL`. For Apprise, we also need tags. The cleanest approach without changing the interface: have `AppriseSender` store tags at construction time. The `NotificationDispatchService` creates a per-channel `AppriseSender` instance with the channel's tags, or we extend the approach slightly.

**Decision:** Evaluate during implementation whether to:
1. Change the `Sender` interface to accept a `SenderConfig` struct (URL + optional fields) — requires updating `DiscordSender` too
2. Have `AppriseSender` accept tags via constructor and create per-channel instances in the dispatch loop
3. Encode tags in the URL string and parse in `AppriseSender`

Option 1 is cleanest long-term. Option 2 is least invasive. Choose during implementation based on what feels right in the code.

Use the existing `sendWebhookRequest()` from `notifications/httpclient.go` for the HTTP POST with retry logic.

#### Step 2.6: Register `AppriseSender` in `NotificationDispatchService`

In `NewNotificationDispatchService()` in `services/notification_dispatch.go`, update the sender map:

```go
senders := map[string]notifications.Sender{
    "discord": notifications.NewDiscordSender(),
    "apprise": notifications.NewAppriseSender(),
}
```

#### Step 2.7: Update route validation for Apprise channels

In `routes/notifications.go`, update the channel creation/update validation:
- Accept `"discord"` and `"apprise"` as valid types
- Require `WebhookURL` for both types (for Apprise, it's the Apprise server URL)
- `AppriseTags` is optional
- Remove all Slack-specific validation

#### Step 2.8: Update frontend notification channel form

- Remove Slack as a channel type option
- Add `"apprise"` as a channel type option
- When Apprise is selected: show URL field (labeled "Apprise Server URL"), show optional "Tags" text input
- When Discord is selected: show URL field (labeled "Discord Webhook URL"), no tags field

#### Step 2.9: Write tests

- `notifications/apprise_test.go` — Unit tests for `AppriseSender.SendDigest()` and `SendAlert()` using an httptest server
- Update `services/notification_dispatch_test.go` — Remove Slack tests, add Apprise dispatch tests
- Update `routes/notifications_test.go` — Remove Slack validation tests, add Apprise validation tests
- Follow the existing test patterns in `notifications/discord_test.go`

#### Step 2.10: Run `make ci` and fix any issues

---

## Phase 3: Settings Export/Import

### Context

This supersedes the existing rules-only import/export (`routes/rules_portability.go`, `services/rules.go` Export/Import methods). The new settings export includes preferences, rules, integration configs (minus credentials), disk group thresholds, and notification channels (minus webhook URLs). Users can select which sections to export/import, so rules-only backup is still possible as a subset.

### Steps

#### Step 3.1: Design the export envelope format

```go
type SettingsExportEnvelope struct {
    Version              int                       `json:"version"`    // 1
    ExportedAt           string                    `json:"exportedAt"` // ISO 8601
    AppVersion           string                    `json:"appVersion"` // Capacitarr version that created the export
    Preferences          *PreferencesExport        `json:"preferences,omitempty"`
    Rules                []RuleExport              `json:"rules,omitempty"`
    Integrations         []IntegrationExport       `json:"integrations,omitempty"`
    DiskGroups           []DiskGroupExport         `json:"diskGroups,omitempty"`
    NotificationChannels []NotificationExport      `json:"notificationChannels,omitempty"`
}
```

Each export struct strips sensitive fields:
- `IntegrationExport`: includes `Name`, `Type`, `URL`, `Enabled` — excludes `APIKey`
- `NotificationExport`: includes `Name`, `Type`, `Enabled`, event subscriptions, `AppriseTags` — excludes `WebhookURL`
- `PreferencesExport`: all fields from `PreferenceSet` except `ID`, `UpdatedAt`
- `RuleExport`: reuse the existing export format (field, operator, value, effect, enabled, integration name/type)
- `DiskGroupExport`: includes `MountPath`, `ThresholdPct`, `TargetPct` — excludes transient `TotalBytes`, `UsedBytes`

#### Step 3.2: Create `BackupService` in `services/backup.go`

New service following the established pattern:

```go
type BackupService struct {
    db  *gorm.DB
    bus *events.EventBus
}
```

Methods:
- `Export(sections ExportSections) (*SettingsExportEnvelope, error)` — Reads config tables via their respective services, builds the envelope with only the requested sections
- `Import(envelope SettingsExportEnvelope, sections ImportSections) (*ImportResult, error)` — Selectively imports sections based on user choice

`ExportSections` and `ImportSections` are structs with boolean flags:
```go
type ExportSections struct {
    Preferences          bool
    Rules                bool
    Integrations         bool
    DiskGroups           bool
    NotificationChannels bool
}
```

`ImportResult` reports what was imported:
```go
type ImportResult struct {
    PreferencesImported          bool
    RulesImported                int
    IntegrationsImported         int
    DiskGroupsImported           int
    NotificationChannelsImported int
}
```

**Service layer compliance:** `BackupService` must access data through existing services where possible:
- Export: `SettingsService.GetPreferences()`, `RulesService.List()`, `IntegrationService.List()`, `NotificationChannelService.List()`, etc.
- Import: `RulesService.Create()`, `SettingsService.UpdatePreferences()`, etc.
- If bulk import methods don't exist on the services, create them.

#### Step 3.3: Register `BackupService` on `services.Registry`

Add `Backup *BackupService` to the `Registry` struct and wire it in `NewRegistry()`.

#### Step 3.4: Create export/import route handlers

In a new `routes/backup.go`:

- `GET /api/v1/settings/export` — Accepts optional `sections` query param (comma-separated: `preferences,rules,integrations,diskGroups,notifications`). Defaults to all sections. Calls `reg.Backup.Export()`, returns JSON with `Content-Disposition` header for download.
- `POST /api/v1/settings/import` — Accepts the envelope JSON + `ImportSections`, calls `reg.Backup.Import()`, returns `ImportResult`.

#### Step 3.5: Remove old rules portability endpoints and all related code

Delete completely:
- `routes/rules_portability.go`
- `routes/rules_portability_test.go`
- `RuleExportEnvelope`, `ExportRule`, `ImportRule` types from `services/rules.go`
- `Export()` and `Import()` methods from `RulesService`
- Route registration call in `routes/api.go` for the old endpoints
- All frontend code that calls `/custom-rules/export` or `/custom-rules/import`

The rule export format is absorbed into `BackupService` — the `RuleExport` struct in `backup.go` replaces the one in `rules.go`.

#### Step 3.6: Publish events for export/import actions

Add `SettingsExportedEvent` and `SettingsImportedEvent` to `events/types.go` for audit trail and SSE notification.

#### Step 3.7: Update frontend

- Replace the rules import/export UI with a settings import/export UI
- Export: "Export Settings" button with section checkboxes (Preferences, Rules, Integrations, Disk Groups, Notifications). Default: all checked. Downloads the JSON file.
- Import: file upload → preview/summary step showing what sections are in the file → checkboxes for which sections to import → confirmation step → result summary
- Remove all UI references to the old rules-only import/export
- Add a "Backup & Restore" button on the Custom Rules page that navigates to the Settings > Backup & Restore section. When arriving from the rules page, pre-select the "Rules" section checkbox via a query param (e.g., `?section=rules`) for one-click rules-only export.

#### Step 3.8: Write tests

- `services/backup_test.go` — Unit tests for Export and Import using in-memory SQLite
- `routes/backup_test.go` — Integration tests for the HTTP endpoints
- Test that sensitive fields (API keys, webhook URLs) are excluded from export
- Test selective export/import (e.g., export only rules, import only preferences)
- Test version validation (reject unknown versions)
- Test that rules-only export/import works as a subset of the full settings export

#### Step 3.9: Run `make ci` and fix any issues

---

## Phase 4: Watchlist Enrichment

### Context

The enrichment pipeline in `integrations/enrich.go` already follows a priority chain: Tautulli > Plex > Jellyfin > Emby for watch data, plus Overseerr for request data. This phase adds watchlist data from Plex, Jellyfin, and Emby as a new enrichment dimension, and wires it into the custom rules engine.

### Steps

#### Step 4.1: Add `OnWatchlist` field to `MediaItem`

In `integrations/types.go`, add to the `MediaItem` struct:

```go
OnWatchlist bool `json:"onWatchlist,omitempty"` // Item is on a user's watchlist or favorited
```

No database migration needed — `MediaItem` is an in-memory struct, not a DB model.

#### Step 4.2: Implement Plex watchlist fetching

In `integrations/plex.go`, add a method:

```go
func (p *PlexClient) GetWatchlistItems() (map[string]bool, error)
```

This calls the Plex watchlist API endpoint and returns a map of title keys (using the same `normalizedTitleKey` pattern from `enrich.go`) to boolean.

**Research required:** Determine the correct Plex API endpoint for watchlist data. Options include:
- `/library/sections/{id}/unwatched` — items not yet watched
- Plex Discover watchlist API — user-curated "want to watch" list
- Per-user watchlist via managed user tokens

Document the approach chosen and any limitations.

#### Step 4.3: Implement Jellyfin favorites fetching

In `integrations/jellyfin.go`, add a method:

```go
func (j *JellyfinClient) GetFavoritedItems(userID string) (map[string]bool, error)
```

Jellyfin exposes favorites via the Items API with `IsFavorite=true` filter. This serves as the Jellyfin equivalent of a watchlist.

#### Step 4.4: Implement Emby favorites fetching

In `integrations/emby.go`, add a method:

```go
func (e *EmbyClient) GetFavoritedItems(userID string) (map[string]bool, error)
```

Same pattern as Jellyfin — Emby's API is nearly identical for favorites.

#### Step 4.5: Add watchlist enrichment pass to `EnrichItems()`

In `integrations/enrich.go`, add a new enrichment section after the existing watch data and request data enrichment. Follow the same pattern:

```go
// ─── Enrichment: Watchlist / Favorites ──────────────────────────────────
```

Priority: Plex watchlist > Jellyfin favorites > Emby favorites (first source wins, same as watch data).

For each item, if the title key matches a watchlist entry, set `item.OnWatchlist = true`.

#### Step 4.6: Add `watchlist` as a rule field in the engine

In `engine/rules.go`, add support for a `watchlist` field in the rule evaluation switch statement. This reads `item.OnWatchlist` and supports the `==` operator with boolean values (`true`/`false`), following the same pattern as the existing `requested` and `incollection` fields.

#### Step 4.7: Add `watchlist` to the rule fields API

In `routes/rulefields.go`, update `appendEnrichmentFields()` to include the watchlist field when a media server integration is enabled:

```go
if p.hasMedia {
    fields = append(fields,
        map[string]any{"field": "incollection", "label": "In Collection", "type": "boolean", "operators": []string{"=="}},
        map[string]any{"field": "watchlist", "label": "On Watchlist", "type": "boolean", "operators": []string{"=="}},
    )
}
```

#### Step 4.8: Update frontend rule builder

Add `watchlist` to the field dropdown in the rule builder UI. When selected, show a true/false value selector (same UX as the existing "Is Requested" and "In Collection" boolean fields).

**Example rule:** "On Watchlist == true → always_keep" — protects any item on a user's watchlist from deletion.

#### Step 4.9: Write tests

- `integrations/plex_test.go` — Test `GetWatchlistItems()` with mock HTTP responses
- `integrations/jellyfin_test.go` — Test `GetFavoritedItems()` with mock HTTP responses
- `integrations/emby_test.go` — Test `GetFavoritedItems()` with mock HTTP responses
- `integrations/enrich_test.go` — Test watchlist enrichment pass (verify `OnWatchlist` is set correctly, verify priority chain)
- `engine/rules_test.go` — Test `watchlist` field evaluation in rules (both `true` and `false` values, with `always_keep` and `prefer_remove` effects)

#### Step 4.10: Run `make ci` and fix any issues

---

## Phase 5: Collection Autocomplete in Rule Builder

### Context

Plex collections are already fetched and stored on `MediaItem.Collections []string`. The `incollection` rule field already works for boolean checks. This phase adds a `collection` rule field with autocomplete populated from actual collection data, allowing rules like "collection contains Favorites → always_keep".

### Steps

#### Step 5.1: Add collection value caching to `IntegrationService`

The `IntegrationService` already has a TTL cache for rule values (quality profiles, tags, languages). Add a method to cache and return collection names:

```go
func (s *IntegrationService) FetchCollectionValues() ([]integrations.NameValue, error)
```

This aggregates unique collection names from Plex libraries. The data source is the Plex client's library data — collections are already parsed in `plexMetadataToMediaItem()`.

**Approach:** During the fetch, call the Plex API to get all items with their collections, extract unique collection names, and cache them in the existing TTL cache pattern used by `FetchRuleValues()`.

#### Step 5.2: Add `collection` rule field with string operators

In `routes/rulefields.go`, add a `collection` field (distinct from the existing boolean `incollection`) that supports string operators:

```go
map[string]any{"field": "collection", "label": "Collection Name", "type": "string", "operators": []string{"==", "!=", "contains", "!contains"}}
```

In `engine/rules.go`, add evaluation logic that checks `item.Collections` against the rule value using the specified operator.

#### Step 5.3: Add API endpoint for collection values

In `routes/rulefields.go`, extend the existing rule values endpoint to return collection names when the field is `collection`. This populates the autocomplete dropdown in the rule builder.

#### Step 5.4: Update frontend rule builder autocomplete

When the rule field is `collection`, fetch collection values from the API and populate the value input as an autocomplete/dropdown instead of a free-text field.

#### Step 5.5: Write tests

- Test collection value fetching and caching in `services/integration_test.go`
- Test the API endpoint returns correct collection names
- Test `collection` field evaluation in `engine/rules_test.go` with various operators
- Follow existing patterns

#### Step 5.6: Run `make ci` and fix any issues

---

## Phase 6: Mobile PWA

### Context

The Nuxt 3 frontend is embedded into the Go binary via `go:embed`. Adding PWA support gives users "Add to Home Screen" capability, full-screen experience, and cached assets for faster loads.

### Steps

#### Step 6.1: Research and select Nuxt PWA module

Evaluate `@vite-pwa/nuxt` vs `@kevinmarrec/nuxt-pwa` vs manual implementation. Select the one that:
- Works with Nuxt 3
- Supports manifest generation
- Supports service worker for asset caching only (not API caching)
- Has minimal bundle size impact

#### Step 6.2: Install and configure the PWA module

Add the selected module to the Nuxt config. Configure:
- `manifest.json`: app name ("Capacitarr"), short name, description, theme color (matching the app's primary color), background color, display mode (`standalone`), start URL, icons
- Service worker: cache strategy for static assets (JS, CSS, fonts, images) only — do NOT cache API responses
- Appropriate `<meta>` tags for iOS (apple-mobile-web-app-capable, status-bar-style) and Android

#### Step 6.3: Create app icons

Generate PWA icon set (192x192, 512x512 minimum) from the existing Capacitarr logo/branding. Include maskable icon variant for Android adaptive icons.

#### Step 6.4: Verify PWA behavior

Use the Puppeteer browser tool to:
- Verify the manifest is served correctly
- Check that the service worker registers
- Confirm the "install" prompt appears in Chrome
- Verify the app works in standalone mode

#### Step 6.5: Ensure `go:embed` includes PWA assets

Verify that the generated `manifest.json`, service worker file, and icon files are included in the `go:embed` directive so they're served from the Go binary.

#### Step 6.6: Run `make ci` and fix any issues

---

## Phase 7: Per-Integration Scoring Overrides (Frontend Mockup Only)

### Context

This is a **frontend-only mockup** to evaluate the UX before committing to backend changes. The sliders will be visually present but non-functional — they don't save to any backend endpoint.

### Steps

#### Step 7.1: Add toggle to integration card

On each integration card in the Settings > Integrations page, add a small toggle/switch labeled "Custom scoring weights" (or similar). Default: off.

#### Step 7.2: Add collapsible weight sliders panel

When the toggle is enabled, show a collapsible panel inline on the integration card with the 6 weight sliders (Watch History, Last Watched, File Size, Rating, Time in Library, Series Status). Pre-populate with the current global weight values.

#### Step 7.3: Add visual indicator

When custom weights are enabled for an integration, show a small badge or icon on the integration card to indicate it has custom scoring.

#### Step 7.4: Ensure sliders are non-functional

The sliders should be interactive (draggable) but should NOT make any API calls. They exist purely for UX evaluation. Add a subtle "Preview — not yet functional" indicator or tooltip.

#### Step 7.5: Evaluate and gather feedback

After implementation, use the app and evaluate whether the UX feels cluttered or intuitive. This step is manual — no code changes.

---

## Phase 8: Test Audit

### Context

After all feature work is complete, audit the entire test suite to ensure quality, remove dead tests, and verify coverage.

### Steps

#### Step 8.1: Inventory all test files

List all `*_test.go` files and their test functions. Identify:
- Tests that cover removed code (Slack sender, rules portability)
- Tests with redundant coverage (testing the same behavior multiple ways)
- Tests that are flaky or depend on timing
- Tests that use outdated patterns (not following the current in-memory SQLite pattern)

#### Step 8.2: Remove dead tests

Delete test files and test functions that cover code removed in earlier phases:
- `notifications/slack_test.go` (removed in Phase 2)
- `routes/rules_portability_test.go` (removed in Phase 3)
- Any test functions that reference removed types, methods, or constants

#### Step 8.3: Verify test coverage for new code

Ensure every new service method, route handler, event type, and sender has corresponding tests:
- `DeletionProgressEvent` tests (Phase 1)
- `AppriseSender` tests (Phase 2)
- `BackupService` tests (Phase 3)
- Watchlist enrichment tests (Phase 4)
- Collection autocomplete tests (Phase 5)

#### Step 8.4: Check for test anti-patterns

Review tests for:
- Hardcoded sleep/timing dependencies (replace with channels or condition variables)
- Tests that don't clean up resources
- Tests that depend on execution order
- Missing error case coverage (only testing happy paths)

#### Step 8.5: Run full test suite with coverage

Run `make ci` and review the coverage report. Identify any significant gaps in coverage for the new code.

#### Step 8.6: Fix any issues found

Address all issues identified in steps 8.1-8.5.

---

## Phase 9: Documentation Audit

### Context

After all features are implemented and tests pass, update all documentation to reflect the new functionality and remove outdated information.

### Steps

#### Step 9.1: Update `README.md`

- Add Apprise to the notification integrations list
- Remove Slack from the notification integrations list
- Add watchlist enrichment to the enrichment integrations list
- Update the feature list to include settings export/import, PWA support
- Remove any references to rules-only import/export

#### Step 9.2: Update `docs/notifications.md`

- Add Apprise section with setup instructions (deploying Apprise container, configuring the channel, using tags)
- Remove all Slack documentation
- Keep Discord documentation as-is

#### Step 9.3: Update `docs/scoring.md`

- Add `watchlist` to the list of available rule fields
- Add `collection` (string-type) to the list of available rule fields
- Document the watchlist enrichment behavior (priority chain, which services provide watchlist data)
- Document collection autocomplete in the rule builder

#### Step 9.4: Update `docs/configuration.md`

- Add settings export/import documentation with section-level granularity
- Remove rules-only import/export documentation
- Document the export format and what's included/excluded (credentials stripped)
- Document that rules-only backup is possible by selecting only the Rules section

#### Step 9.5: Update `docs/architecture.md`

- Add `BackupService` to the service layer documentation
- Update the notification system section: remove Slack, add Apprise
- Update the enrichment pipeline section to include watchlist data
- Add `DeletionProgressEvent` to the event types documentation

#### Step 9.6: Update `docs/api/openapi.yaml`

- Add new endpoints: `GET /settings/export`, `POST /settings/import`
- Remove old endpoints: `GET /custom-rules/export`, `POST /custom-rules/import`
- Update notification channel type enum: remove `"slack"`, add `"apprise"`
- Add `appriseTags` field to notification channel schema
- Add `onWatchlist` field to media item schema
- Add `deletion_progress` to SSE event types
- Add `collection` rule field
- Update any response schemas that changed

#### Step 9.7: Update `docs/api/examples.md` and `docs/api/workflows.md`

- Update notification channel examples: remove Slack, add Apprise
- Update settings management examples to use the new export/import endpoints
- Remove rules-only import/export examples

#### Step 9.8: Update `CHANGELOG.md`

This will be handled by git-cliff from conventional commit messages. Ensure all commits in the feature branch follow the conventional commits format so the changelog is auto-generated correctly.

#### Step 9.9: Update `docs/quick-start.md`

- Mention PWA "Add to Home Screen" capability
- Mention settings export as a backup mechanism

#### Step 9.10: Review `docs/troubleshooting.md`

- Add troubleshooting for Apprise connection issues
- Remove any Slack-related troubleshooting
- Add troubleshooting for settings import failures

#### Step 9.11: Run `make ci` one final time

Ensure everything passes after all documentation updates.

---

## Architecture Diagrams

### Notification System (After Phase 2)

```mermaid
flowchart TD
    subgraph EVENT_BUS["Event Bus"]
        EVENTS["Engine, Deletion, Settings,<br/>Approval, System events"]
    end

    subgraph DISPATCH["NotificationDispatchService"]
        ACCUMULATOR["Cycle Accumulator<br/>Two-gate flush pattern"]
        SENDER_MAP["Sender Map<br/>discord, apprise"]
    end

    subgraph SENDERS["Sender Implementations"]
        DISCORD["DiscordSender<br/>Rich embeds, colors"]
        APPRISE["AppriseSender<br/>Markdown → Apprise API"]
    end

    subgraph TARGETS["Notification Targets"]
        DISCORD_WH["Discord Webhooks"]
        APPRISE_SVC["Apprise Server<br/>80+ services incl. Slack"]
    end

    EVENTS --> DISPATCH
    DISPATCH --> DISCORD
    DISPATCH --> APPRISE
    DISCORD --> DISCORD_WH
    APPRISE --> APPRISE_SVC
```

### Enrichment Pipeline (After Phase 4)

```mermaid
flowchart TD
    subgraph FETCH["Fetch Phase"]
        ARR["*arr APIs<br/>Sonarr, Radarr, Lidarr, Readarr"]
    end

    subgraph ENRICH["Enrichment Phase - Priority Order"]
        TAUTULLI["1. Tautulli<br/>Watch history + users"]
        PLEX_WATCH["2. Plex<br/>Play counts, collections"]
        JELLYFIN_WATCH["3. Jellyfin<br/>Play counts"]
        EMBY_WATCH["4. Emby<br/>Play counts"]
        OVERSEERR["5. Overseerr<br/>Request data"]
        WATCHLIST["6. Watchlist 🆕<br/>Plex watchlist,<br/>Jellyfin/Emby favorites"]
        CROSS_REF["7. Cross-reference<br/>Requestor watched?"]
    end

    subgraph SCORE["Scoring Phase"]
        ENGINE["EvaluateMedia<br/>Weights + Rules"]
    end

    ARR --> ENRICH
    TAUTULLI --> PLEX_WATCH
    PLEX_WATCH --> JELLYFIN_WATCH
    JELLYFIN_WATCH --> EMBY_WATCH
    EMBY_WATCH --> OVERSEERR
    OVERSEERR --> WATCHLIST
    WATCHLIST --> CROSS_REF
    CROSS_REF --> SCORE
```

### Settings Export/Import (Phase 3)

```mermaid
flowchart LR
    subgraph EXPORT["Export Flow"]
        EXPORT_BTN["Export Settings<br/>Section checkboxes"]
        BACKUP_SVC["BackupService.Export<br/>Selected sections only"]
        STRIP["Strip credentials<br/>API keys, webhook URLs"]
        JSON_FILE["capacitarr-settings.json"]
    end

    subgraph IMPORT["Import Flow"]
        UPLOAD["Upload JSON file"]
        VALIDATE["Validate version +<br/>schema integrity"]
        PREVIEW["Show import preview<br/>Section checkboxes"]
        APPLY["BackupService.Import<br/>Selected sections only"]
    end

    EXPORT_BTN --> BACKUP_SVC
    BACKUP_SVC --> STRIP
    STRIP --> JSON_FILE

    JSON_FILE -.->|"user transfers file"| UPLOAD
    UPLOAD --> VALIDATE
    VALIDATE --> PREVIEW
    PREVIEW --> APPLY
```

### Deletion Progress SSE Flow (Phase 1)

```mermaid
flowchart LR
    subgraph BACKEND["Backend"]
        WORKER["DeletionService.processJob"]
        ATOMICS["Atomic counters<br/>succeeded, failed,<br/>batchTotal"]
        BUS["EventBus"]
        SSE["SSE Broadcaster"]
    end

    subgraph FRONTEND["Frontend"]
        HANDLER["useEventStream<br/>deletion_progress handler"]
        PROGRESS["Progress indicator<br/>below engine card"]
        SPARKLINE["Sparkline<br/>patch lastPoint.deleted"]
    end

    WORKER --> ATOMICS
    ATOMICS --> BUS
    BUS --> SSE
    SSE --> HANDLER
    HANDLER --> PROGRESS
    HANDLER --> SPARKLINE
```
