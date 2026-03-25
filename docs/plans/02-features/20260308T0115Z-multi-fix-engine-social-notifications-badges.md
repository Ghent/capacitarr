# Multi-Fix: Engine Mode, Social Links, Notification Triggers, Badges

**Created:** 2026-03-08T01:15Z
**Status:** ✅ Complete

## Overview

This plan covers four distinct improvements bundled into a single feature branch:

1. **Engine control mode switching bug** — Mode reverts after changing
2. **Social media icons** — Add Discord and Reddit links
3. **Notification trigger context** — Show what action triggered Discord/Slack alerts
4. **Shields.io badges** — Add project badges to README and docs

## Changes

### 1. Fix: Engine Mode Switching Reverts Immediately

**Root Cause:**
- `GetStats()` in `engine.go` reads `executionMode` from the last `EngineRunStats` record (the mode at time of last engine run), not from the `PreferenceSet` table (the user's current setting)
- `GetWorkerMetrics()` in `metrics.go` passes this stale value to the frontend
- The frontend's `useEngineControl` composable doesn't subscribe to the `engine_mode_changed` SSE event

**Backend fix (`metrics.go`):**
- Override `executionMode` in `GetWorkerMetrics()` with the value from `SettingsService.GetPreferences()`, which reflects the user's current preference

**Frontend fix (`useEngineControl.ts`):**
- Add SSE handler for `engine_mode_changed` event to update local `workerStats.executionMode` immediately

**Tests:**
- `metrics_test.go`: Added `TestMetricsService_GetWorkerMetrics_ExecutionModeFromPreferences` — creates an engine run with mode A, changes preference to mode B, verifies worker metrics returns mode B
- `useEngineControl.test.ts`: Added `SSE engine_mode_changed` test suite

### 2. Feature: Social Media Icons (Discord + Reddit)

**`help.vue`:**
- Added "Community" row in the About Capacitarr grid with Discord and Reddit links
- Used inline SVG icons from Simple Icons for brand recognition

**`README.md`:**
- Added "Community" section with Discord and Reddit links
- Added Discord and Reddit shields.io badges to the badge row

### 3. Feature: Notification Trigger Labels

**`sender.go`:**
- Added `TriggerLabel(AlertType) string` — maps alert types to human-readable labels (e.g., `AlertUpdateAvailable` → "Update Available")

**`discord.go`:**
- Changed alert author line from `"⚡ Capacitarr v1.4.0"` to `"⚡ Capacitarr v1.4.0 • Update Available"` (or whatever the trigger type is)

**`slack.go`:**
- Same change to Slack alert header

**Tests:**
- `sender_test.go`: Added `TestTriggerLabel` covering all alert types
- `discord_test.go`: Verify author line includes trigger label for `SendAlert`
- `slack_test.go`: Verify header includes trigger label for `SendAlert`

### 4. Feature: Shields.io Badges

**`README.md`:**
- Pipeline status, release version, license, container registry, Discord, Reddit badges

**`docs/index.md`:**
- Same badges for the GitLab Pages documentation site

## Files Modified

| File | Change |
|------|--------|
| `backend/internal/services/metrics.go` | Read executionMode from preferences |
| `backend/internal/services/metrics_test.go` | Add executionMode preference test |
| `backend/internal/notifications/sender.go` | Add `TriggerLabel()` function |
| `backend/internal/notifications/sender_test.go` | Add `TriggerLabel` tests |
| `backend/internal/notifications/discord.go` | Add trigger label to alert author |
| `backend/internal/notifications/discord_test.go` | Verify trigger label in author |
| `backend/internal/notifications/slack.go` | Add trigger label to alert header |
| `backend/internal/notifications/slack_test.go` | Verify trigger label in header |
| `frontend/app/composables/useEngineControl.ts` | Add `engine_mode_changed` SSE handler |
| `frontend/app/composables/useEngineControl.test.ts` | Add SSE handler tests |
| `frontend/app/pages/help.vue` | Add Discord + Reddit community links |
| `README.md` | Add badges and community section |
| `docs/index.md` | Add badges |
