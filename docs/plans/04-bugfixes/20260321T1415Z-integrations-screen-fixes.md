# Integrations Screen Fixes

**Created:** 2026-03-21T14:15Z
**Status:** ✅ Complete

## Overview

Fix several issues with the integrations settings screen:

1. Error messages not wrapping properly in cards
2. Errors never clear (no way to tell the app a service is back up)
3. Remove unfinished custom scoring weights and threshold override features
4. Add enable/disable toggle to integration cards
5. Fix bug where threshold override saves accidentally disable integrations

## Phases

### Phase 1: Fix PUT handler partial updates

**Problem:** The PUT `/api/v1/integrations/:id` handler binds the entire request body to `db.IntegrationConfig`. When only `thresholdPct`/`targetPct` are sent, Go's zero-value for `bool` (`false`) overwrites `Enabled`, silently disabling integrations.

**Fix:** Guard the `Enabled` field assignment — only update it when the request explicitly includes it. Since we're removing thresholds in Phase 4, this is a safety fix for the general PUT handler.

**Files:**
- `backend/routes/integrations.go` — guard `existing.Enabled = update.Enabled`

### Phase 2: Fix error text wrapping

**Problem:** Long error messages (containing URLs) overflow the card because the `<span>` has no word-break rules and the flex parent doesn't constrain width.

**Fix:** Add `break-all min-w-0` classes to the error display elements.

**Files:**
- `frontend/app/components/settings/SettingsIntegrations.vue` — error display div/span

### Phase 3: Clear lastError on successful Test + hint text

**Problem:** Clicking "Test" on a card calls `TestConnection()` which publishes events but never calls `UpdateSyncStatus()` to clear `lastError` in the DB.

**Fix:**
- In `IntegrationService.TestConnection()`, when `integrationID` is non-nil and test succeeds, call `UpdateSyncStatus(id, &now, "")`.
- Add a small hint below the error in the card: "Click Test to retry"

**Files:**
- `backend/internal/services/integration.go` — `TestConnection()` method
- `frontend/app/components/settings/SettingsIntegrations.vue` — hint text below error

### Phase 4: Remove custom weights + threshold overrides

**Problem:** Neither feature is ready. Custom weights are local-only state (never persisted). Threshold overrides have the Enabled bug. Both confuse users.

**Frontend removal:**
- Template: Remove "Custom Scoring Weights" section (lines 112–173) and "Threshold Overrides" section (lines 175–251)
- Script: Remove `WeightOverrides`, `defaultWeights`, `customWeightsState`, all weight helpers, `ThresholdOverride`, `thresholdState`, all threshold helpers, `saveThresholds()`, debounce timer
- Header badge: Remove custom weights badge (lines 73–76)
- Import: Remove `SlidersHorizontalIcon`
- i18n: Remove ~25 keys from all 22 locale files
- Types: Remove `thresholdPct`/`targetPct` from `IntegrationConfig` interface

**Backend removal:**
- Model: Remove `ThresholdPct` and `TargetPct` from `IntegrationConfig` struct
- Route: Remove threshold assignment in PUT handler
- Tests: Update affected tests

**Note:** Keep `ThresholdPct`/`TargetPct` on the `Library` model — that's a separate feature.

### Phase 5: Add enable/disable toggle to integration cards

**Implementation:**
- Replace the static Active/Disabled badge with a `UiSwitch` + label in the card header
- Toggle calls `PUT /api/v1/integrations/:id` with `{ enabled: true/false }`
- Disabled cards get visual dimming (`opacity-60`)
- Clear error state when disabling (no point showing connection errors for intentionally disabled integrations)

### Phase 6: Update baseline SQL

Since this is based on `feature/2.0`, no migration needed — update the baseline SQL to remove `threshold_pct` and `target_pct` columns from the `integration_configs` table.

**Files:**
- `backend/internal/db/migrations/00001_v2_baseline.sql`

### Phase 7: Add integration error/recovery notifications

**Problem:** Integration connection failures and recoveries are not sent to Discord/Apprise. The events (`IntegrationTestEvent`, `IntegrationTestFailedEvent`) exist but the notification dispatch doesn't subscribe to them.

**Implementation:**

1. Add a new `IntegrationRecoveredEvent` to `events/types.go` — published when an integration transitions from error to healthy
2. Add a new notification toggle `OnIntegrationStatus` to `NotificationConfig` model and baseline SQL
3. Subscribe to `IntegrationTestFailedEvent` and `IntegrationRecoveredEvent` in the notification dispatch handler
4. Publish `IntegrationRecoveredEvent` in the poller when `lastError` transitions from non-empty to empty
5. Publish `IntegrationRecoveredEvent` in `TestConnection()` when test succeeds and `lastError` was previously set
6. Add the toggle to the notification channel settings UI
7. Add i18n keys for the new notification type

**Files:**
- `backend/internal/events/types.go` — new `IntegrationRecoveredEvent`
- `backend/internal/db/models.go` — add `OnIntegrationStatus` to `NotificationConfig`
- `backend/internal/db/migrations/00001_v2_baseline.sql` — add column
- `backend/internal/services/notification_dispatch.go` — handle new events
- `backend/internal/services/integration.go` — publish recovery event in `TestConnection()`
- `backend/internal/poller/fetch.go` — publish recovery event on error→healthy transition
- `frontend/app/components/settings/SettingsNotifications.vue` — add toggle
- `frontend/app/types/api.ts` — add field to `NotificationChannel`
- `frontend/app/locales/*.json` — add i18n keys
