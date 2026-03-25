# Safety Guard Advanced Setting + Internationalization

**Date:** 2026-03-01
**Status:** ✅ Phase 1 Complete — Safety guard implemented as DB-backed `DeletionsEnabled` setting with UI toggle in Settings > Advanced. Phase 2 (i18n) superseded by `20260302T1527Z-internationalization-i18n.md`.
**Branch:** `feature/ux-refinement`

---

## Phase 1: Safety Guard as Advanced Setting

**Effort:** S
**Current state:** Deletions are disabled via a hardcoded guard in `poller.go` (line ~498). The `DeleteMediaItem` call is commented out and all actions are logged as "Dry-Delete" regardless of execution mode.

**Goal:** Replace the hardcoded guard with a database-backed setting exposed in Settings > Advanced, with a prominent warning.

### UI Design

Settings > Advanced tab, new card at the top:

```
┌─────────────────────────────────────────────────────────────────┐
│ ⚠️  Deletion Safety                                            │
│ ────────────────────────────────────────────────────────────── │
│                                                                 │
│  Enable actual file deletion     [  OFF  ]                     │
│                                                                 │
│  ⚠️ WARNING: When enabled, Capacitarr will permanently         │
│  delete media files from your storage when the scoring          │
│  engine flags items for removal. Files cannot be recovered.     │
│                                                                 │
│  Leave this OFF while setting up and testing your scoring       │
│  rules. Only enable when you're confident in your               │
│  configuration.                                                 │
│                                                                 │
│  Current status: All deletions are simulated (Dry-Delete)       │
└─────────────────────────────────────────────────────────────────┘
```

### Implementation

1. Add `DeletionsEnabled` field to `PreferenceSet` model (default: `false`)
2. Read this field in the deletion worker before calling `DeleteMediaItem`
3. If disabled: log as "Dry-Delete", skip actual deletion (current behavior)
4. If enabled: proceed with actual deletion (original behavior)
5. Add the toggle to Settings > Advanced with a destructive-styled warning card
6. Require a confirmation dialog when enabling ("Type DELETE to confirm")

### Files

- `backend/internal/db/models.go` — add `DeletionsEnabled` bool
- `backend/internal/poller/poller.go` — read setting in deletion worker
- `backend/routes/rules.go` — include in preferences API
- `frontend/app/pages/settings.vue` — add safety toggle card in Advanced tab
- `backend/internal/db/migrations/` — migration to add column

---

## Phase 2: Internationalization (i18n)

**Effort:** M-L
**Current state:** All user-facing strings are hardcoded in English across Vue templates and Go backend error messages.

### Approach

Use `@nuxtjs/i18n` (Nuxt's official i18n module) for the frontend. Backend error messages stay in English (API consumers expect English errors).

### Steps

1. **Install `@nuxtjs/i18n`** and configure in `nuxt.config.ts`
2. **Extract all hardcoded strings** from Vue templates into message files
3. **Create message files:**
   - `frontend/app/locales/en.json` — English (source of truth)
   - `frontend/app/locales/` — additional locales added by community
4. **Replace hardcoded text** with `$t('key')` calls across all pages/components
5. **Add language selector** in Settings > General (alongside theme/clock)
6. **Lazy-load locales** — only load the selected language

### String categories to extract

| Category | Approx. count | Examples |
|----------|---------------|---------|
| Page titles & descriptions | ~15 | "Dashboard", "Capacity overview across your media storage" |
| Button labels | ~20 | "Run Now", "Save", "Cancel", "Test Connection" |
| Card titles | ~15 | "Poll Interval", "Change Password", "API Key" |
| Status messages | ~10 | "Synced 2m ago", "No disk groups yet", "Loading..." |
| Error messages (frontend) | ~15 | "Invalid credentials", "Failed to save", "Connection failed" |
| Tooltips & help text | ~10 | Plex token instructions, URL guidance |

### File structure

```
frontend/app/
├── locales/
│   ├── en.json         # English (complete)
│   ├── es.json         # Spanish (community)
│   ├── de.json         # German (community)
│   └── fr.json         # French (community)
└── plugins/
    └── i18n.ts         # Nuxt i18n config
```

### Priority

Low — functional completeness and stability come first. i18n is a post-launch enhancement when community demand warrants it. The string extraction is the most tedious part but doesn't require design decisions.

---

## Implementation Summary

| Phase | Item | Effort | Priority |
|-------|------|--------|----------|
| 1 | Safety guard Advanced setting | S | High (before production testing) |
| 2 | Internationalization | M-L | Low (post-launch) |
