# Complete Remaining Plan Items

**Date:** 2026-03-02
**Status:** ✅ Complete — All 8 phases implemented on branch `feature/complete-remaining-plan-items`.
**Branch:** `feature/complete-remaining-plan-items`

---

## Overview

This plan consolidates all truly incomplete items identified during the comprehensive plan audit on 2026-03-02. It covers rule builder polish, test coverage expansion, code quality improvements, new features (notifications, Plex OAuth), and internationalization.

---

## Phase 1: Quick Wins (XS-S effort)

### 1.1 Add `exhaustive` Linter to golangci-lint
- Add `exhaustive` to the `enable` list in `backend/.golangci.yml`

### 1.2 Rule Builder: Free-Text Value Validation
- In `RuleBuilder.vue`, when the value input is free-text and the field has known options from the API, show a warning badge if the entered value doesn't match any known option
- Non-blocking — the rule can still be saved

---

## Phase 2: Rule Builder Polish (S-M effort)

### 2.1 Bulk Rule Enable/Disable
- Add `Enabled` boolean field to `ProtectionRule` model (default: true)
- Create Goose migration `00009_add_rule_enabled.sql`
- Update `applyRules()` in `engine/rules.go` to skip disabled rules
- Add toggle switch per rule in the rules list UI
- Include `enabled` field in rule CRUD API

### 2.2 Rule Drag-to-Reorder
- Add `SortOrder` integer field to `ProtectionRule` model
- Create Goose migration `00010_add_rule_sort_order.sql`
- Add drag handle to each rule card in the rules list
- Use a lightweight drag library (e.g., `vuedraggable` or `@vueuse/integrations/useSortable`)
- Update sort order via `PUT /api/v1/protections/reorder` endpoint

---

## Phase 3: Test Coverage Expansion (M effort)

### 3.1 Integration Client Tests
Create test files for remaining integration clients using `httptest.NewServer` pattern:
- `backend/internal/integrations/lidarr_test.go`
- `backend/internal/integrations/readarr_test.go`
- `backend/internal/integrations/emby_test.go`
- `backend/internal/integrations/tautulli_test.go`
- `backend/internal/integrations/overseerr_test.go`

### 3.2 Route Handler Tests
- `backend/routes/rules_test.go` — CRUD for protection rules, rule-fields, rule-values
- `backend/routes/preview_test.go` — preview endpoint with mock data
- `backend/routes/data_test.go` — data reset/clear endpoints

### 3.3 Poller Full-Cycle Test
- `backend/internal/poller/poller_test.go` — test poll cycle with mock integration server

---

## Phase 4: Go Doc Comments (M effort)

Add doc comments to all exported functions across:
- `backend/routes/*.go`
- `backend/internal/integrations/*.go`
- `backend/internal/poller/*.go`
- `backend/internal/engine/*.go`
- `backend/internal/db/*.go`
- `backend/internal/cache/*.go`
- `backend/internal/jobs/*.go`

---

## Phase 5: Accessibility Improvements (M effort)

### 5.1 ARIA Labels
- Audit all interactive elements for proper `aria-label` attributes
- Add `role` attributes where semantic HTML isn't sufficient
- Ensure all icon-only buttons have accessible names

### 5.2 Keyboard Navigation
- Verify tab order is logical on all pages
- Ensure all interactive elements are keyboard-accessible
- Add focus-visible styles where missing

### 5.3 Focus Management
- Trap focus in modals/dialogs (shadcn-vue Dialog should handle this)
- Return focus to trigger element when modal closes
- Manage focus on page transitions

---

## Phase 6: Plex OAuth PIN Flow (S-M effort)

### 6.1 Backend
- New endpoint `POST /api/v1/integrations/plex/auth/pin` — creates a Plex PIN via `POST https://plex.tv/api/v2/pins`
- New endpoint `GET /api/v1/integrations/plex/auth/pin/:id` — polls Plex for PIN claim status
- On successful claim, auto-create/update the Plex integration with the received auth token

### 6.2 Frontend
- "Sign in with Plex" button on the integration add/edit form when type is Plex
- Opens Plex auth URL in a popup window
- Polls the backend PIN status endpoint until claimed or timeout
- On success, closes popup and populates the integration form

---

## Phase 7: Notification Channels (L effort)

### 7.1 Backend Infrastructure
- New `NotificationConfig` model (type, webhook URL, enabled, events)
- Goose migration for notification_configs table
- `backend/internal/notifications/` package with dispatcher
- Event types: `threshold_breach`, `deletion_executed`, `engine_error`, `engine_complete`

### 7.2 Discord Webhook
- `backend/internal/notifications/discord.go`
- Rich embed format with color-coded severity
- Media item details in embed fields

### 7.3 Slack Webhook
- `backend/internal/notifications/slack.go`
- Block Kit format for rich messages

### 7.4 In-App Notifications
- `backend/internal/notifications/inapp.go`
- Store in SQLite, expose via `GET /api/v1/notifications`
- Bell icon in navbar with unread count badge
- Notification dropdown/panel

### 7.5 Frontend Settings
- Notifications tab in Settings page
- Add/edit/delete notification channels
- Test notification button
- Event subscription checkboxes per channel

---

## Phase 8: Internationalization (L effort)

### 8.1 Infrastructure
- Install `@nuxtjs/i18n`
- Configure in `nuxt.config.ts` with lazy loading
- Create `frontend/app/locales/en.json` with all ~205 keys

### 8.2 String Extraction
- Extract strings from all pages and components
- Replace with `$t()` calls

### 8.3 Language Selector
- Add to Settings > General tab
- Store in localStorage

### 8.4 Translations
- `locales/es.json` — Spanish
- `locales/de.json` — German
- `locales/fr.json` — French

---

## Execution Order

```
Phase 1 (quick wins) → Phase 2 (rule builder) → Phase 3 (tests) → Phase 4 (doc comments) → Phase 5 (a11y) → Phase 6 (Plex OAuth) → Phase 7 (notifications) → Phase 8 (i18n)
```

Phases 3-4 can run in parallel with Phase 2. Phase 5 can run in parallel with Phase 6.
