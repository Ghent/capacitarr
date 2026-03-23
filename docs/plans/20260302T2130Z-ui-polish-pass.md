# UI Polish Pass

**Created:** 2026-03-02T21:30Z
**Status:** ✅ Complete
**Branch:** `feature/ui-polish-pass`
**Based on:** `feature/complete-remaining-plan-items`

## Overview

A collection of UX improvements, bug fixes, and polish items to refine the Capacitarr frontend experience before release.

---

## Items

### 1. Document Plex Service Dropdown Behavior (docs only)

**Status:** Planned
**Effort:** Trivial

The rule builder's service dropdown intentionally only shows *arr integrations (Sonarr, Radarr, Lidarr, Readarr) because rules operate on *arr library items. Plex, Jellyfin, Emby, Tautulli, and Overseerr are "enrichment" services — they add data (play count, request status) to arr items. When an enrichment service is active, its fields (e.g., "Play Count") automatically appear in the rule builder for any *arr integration.

**Action:** Add a clear note in the scoring documentation explaining this design decision.

### 2. Investigate Rule Enabled Toggle for New Rules

**Status:** Needs Investigation
**Effort:** Small (depends on root cause)

Reported: toggle for newly added rules shows as disabled. The backend model `ProtectionRule.Enabled` is `bool` with `gorm:"default:true;not null"` and the POST handler explicitly sets `Enabled = true`. JSON serialization includes the field (no `omitempty`).

**Investigation needed:**
- Create a fresh rule via the UI and inspect the raw API response in browser DevTools
- Check if old/migrated rules have `enabled: false` due to missing backfill
- Check if the UiSwitch component has a visual rendering issue on initial mount

### 3. Smarter Rule Conflict Detection

**Status:** Planned
**Effort:** Medium

Current conflict detection only checks effect direction (keep vs remove) and scope (same integration). It flags false positives when rules target the same field but with non-overlapping numeric ranges. For example:
- Rule A: `timeinlibrary > 180` → prefer_remove
- Rule B: `timeinlibrary < 30` → always_keep

These ranges don't overlap, so they can never both match the same item.

**Implementation:**
- For rules on the **same field** with opposing effects:
  - **Numeric fields:** Compute the effective range from operator + value, check for intersection
  - **String `==` / `!=`:** Compare exact values — different values with `==` = no overlap
  - **String `contains` / `!contains`:** Flag as potential conflict (substring overlap is hard to determine)
  - **Boolean fields:** Same value = conflict, different values = no conflict
- Only flag conflicts when ranges/values could plausibly overlap

### 4. Advanced Tab → Last Position + Red Styling

**Status:** Planned
**Effort:** Trivial

Reorder settings tabs: General → Integrations → Notifications → Security → **Advanced**

Style the Advanced tab trigger with a persistent red/destructive accent to signal it contains dangerous settings (enable deletions, reset data).

### 5. Backend Connection Loss/Recovery Banner

**Status:** Planned
**Effort:** Medium

Add a global connection health monitor:
- Detect network errors / timeouts in `useApi` via `onRequestError` handler
- Show a persistent on-screen banner when connection is lost
- Start polling a health endpoint (e.g., `GET /api/v1/health` or `/api/v1/preferences`)
- Dismiss the banner and briefly show "Connection restored" when the backend responds again

### 6. Complete i18n Locale Files

**Status:** Planned
**Effort:** Large

Audit all 21 non-English locale files against `en.json` and add complete translations for every missing key. This ensures switching languages shows actual translations rather than raw i18n key labels.

**Languages:** es, de, fr, pt-BR, nl, it, pl, sv, da, nb, fi, ru, uk, cs, ro, hu, tr, ja, ko, zh-CN, zh-TW

### 7. Deletion Enable Toast → Red/Error Variant

**Status:** Planned
**Effort:** Trivial

Change the toast variant for "File deletions enabled" from `'warning'` (amber) to `'error'` (red) to match the severity of the action.

---

## Priority Order

| Priority | Item | Effort |
|----------|------|--------|
| P1 | 7 — Deletion toast → red | Trivial |
| P1 | 4 — Advanced tab last + red | Trivial |
| P2 | 2 — Rule enabled toggle investigation | Small |
| P2 | 3 — Smarter conflict detection | Medium |
| P2 | 5 — Connection loss/recovery banner | Medium |
| P3 | 1 — Document Plex dropdown behavior | Trivial |
| P3 | 6 — Complete i18n locale files | Large |
