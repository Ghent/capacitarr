# UX Fixes, Enrichment Rule Fields, Approval Queue, and Plex OAuth

**Date:** 2026-03-04
**Status:** ✅ Complete
**Branch:** `feature/ux-polish-enrichments`
**Base:** `main`

---

## Overview

This plan covers a focused set of UX improvements, bug fixes, and feature additions
identified during a review session. The work is organized into 7 phases that can be
tackled incrementally within a single feature branch.

---

## Phase 1: Quick Wins (API Key Truncation + Effect Badge Position)

**Estimated effort:** 30 minutes

### 1.1 — API Key Truncation on Integration Cards

**File:** `frontend/app/pages/settings.vue` (line ~369)

**Problem:** Long API keys (especially Overseerr's 40+ char keys) overflow the
integration card width.

**Solution:** Mask and truncate the displayed key:

```vue
<!-- Before -->
<span class="font-mono text-xs">{{ integration.apiKey }}</span>

<!-- After -->
<span
  class="font-mono text-xs truncate max-w-[180px] inline-block align-bottom"
  :title="integration.apiKey"
>
  {{ integration.apiKey.length > 16
    ? integration.apiKey.slice(0, 8) + '••••' + integration.apiKey.slice(-4)
    : integration.apiKey }}
</span>
```

**Changes:**
- [x] Update `settings.vue` integration card body (line ~369) with truncation logic

### 1.2 — Move Effect Badge to Right Side in Custom Rules

**File:** `frontend/app/pages/rules.vue` (lines ~348–443)

**Problem:** The effect badge (e.g., "🔴 Always remove") sits inline with the
condition text on the left, making it hard to scan rules at a glance.

**Current layout:**
```
[Drag] [#] [Toggle] [⚠️] [Effect Badge] [Service] [Condition]     [X]
```

**Target layout:**
```
[Drag] [#] [Toggle] [⚠️] [Service] [Condition]     [Effect Badge] [X]
```

**Changes:**
- [x] Move the `<UiBadge>` (lines ~409-418) from the left `<div>` to a new right-side wrapper
- [x] Wrap the right side: `<div class="flex items-center gap-2 shrink-0">[Badge] [X]</div>`

---

## Phase 2: First Login UX

**Estimated effort:** 2–3 hours

**Problem:** On first launch, the login page shows "Welcome Back" and "Sign In" — implying an account already exists. The first login actually _creates_ the admin account
via bootstrap logic in `backend/routes/auth.go` (lines 46–79), but there's zero UI
indication of this.

### 2.1 — Backend: Auth Status Endpoint

**File:** `backend/routes/auth.go`

Add a public endpoint that indicates whether any user account exists:

```go
public.GET("/auth/status", func(c echo.Context) error {
    var count int64
    database.Model(&db.AuthConfig{}).Count(&count)
    return c.JSON(http.StatusOK, map[string]interface{}{
        "initialized": count > 0,
    })
})
```

**Changes:**
- [x] Add `GET /api/v1/auth/status` route (public, no auth required)
- [x] Add test in `auth_test.go`

### 2.2 — Frontend: Conditional Login/Setup Mode

**File:** `frontend/app/pages/login.vue`

On mount, fetch `/api/v1/auth/status`. If `initialized === false`:
- Title: **"Set Up Capacitarr"** (not "Welcome Back")
- Subtitle: **"Create your admin account to get started"**
- Button: **"Create Account"** (not "Sign In")
- Helper text below password: "Choose a username and password for your admin account"
- Remove the `placeholder="admin"` (or change to `placeholder="Choose a username"`)

If `initialized === true`: show the existing login form unchanged.

**Changes:**
- [x] Add `onMounted` fetch to `/api/v1/auth/status`
- [x] Add `isSetupMode` ref
- [x] Conditional template rendering for setup vs login mode
- [x] Add i18n keys to `en.json` (and locale stubs):
  - `login.setupTitle`: "Set Up Capacitarr"
  - `login.setupSubtitle`: "Create your admin account to get started"
  - `login.createAccount`: "Create Account"
  - `login.setupHint`: "Choose a username and password — this will be your admin account"

---

## Phase 3: Date Toggle (Relative ↔ Exact)

**Estimated effort:** 4–6 hours

**Problem:** Dates throughout the app show only relative time ("3d ago"). Users
sometimes need the exact date/time.

### 3.1 — Display Preferences Enhancement

**File:** `frontend/app/composables/useDisplayPrefs.ts`

Add `showExactDates` state (boolean, persisted to localStorage):

```typescript
const showExactDates = useState('displayExactDates', () => {
  if (import.meta.client) {
    return localStorage.getItem('capacitarr_exactDates') === 'true'
  }
  return false
})

function setShowExactDates(val: boolean) {
  showExactDates.value = val
  if (import.meta.client) localStorage.setItem('capacitarr_exactDates', String(val))
}
```

**Changes:**
- [x] Add `showExactDates` + `setShowExactDates` to `useDisplayPrefs()`
- [x] Export from the composable's return

### 3.2 — DateDisplay Component

**File:** `frontend/app/components/DateDisplay.vue` (new)

A component that wraps all date rendering. Shows relative by default, toggles to
exact on click/touch. Respects the global `showExactDates` preference.

Props:
- `date: string` — ISO 8601 date string
- `alwaysExact?: boolean` — override to always show exact (for audit log)

Behavior:
- Default: relative time via `formatRelativeTime()`
- Click/touch: toggle to exact via `formatTimestamp()`
- If `showExactDates` preference is true, always show exact
- Subtle underline/cursor hint that it's interactive
- Tooltip showing the alternate format

**Changes:**
- [x] Create `DateDisplay.vue` component
- [x] Add i18n keys if needed

### 3.3 — Replace All Date Rendering Call Sites

Replace direct `formatRelativeTime()` / `formatTime()` calls with `<DateDisplay>`:

| File | Location | Current |
|------|----------|---------|
| `pages/index.vue` | Line ~30 | `formatRelativeTime(lastUpdated.toISOString())` |
| `pages/index.vue` | Line ~144 | `formatRelativeTime(...)` |
| `pages/index.vue` | Line ~587 | `formatRelativeTime(...)` |
| `pages/settings.vue` | Line ~379 | `formatRelativeTime(integration.lastSync)` |
| `pages/audit.vue` | Lines ~192, ~243 | `formatTimestamp(...)` |
| `components/Navbar.vue` | Line ~150 | `formatRelativeTime(notif.createdAt)` |
| `components/EngineControlPopover.vue` | Line ~184 | `formatRelativeTime(...)` |
| `components/ScoreDetailModal.vue` | Line ~195 | `formatTime(createdAt)` |

**Changes:**
- [x] Replace ~10 call sites across 6 files with `<DateDisplay :date="..." />`

### 3.4 — Settings UI Toggle

**File:** `frontend/app/pages/settings.vue` — General tab

Add a toggle switch: "Always show exact dates" underneath the clock format setting.

**Changes:**
- [x] Add toggle to Settings → General tab
- [x] Wire to `setShowExactDates()`
- [x] Add i18n keys: `settings.exactDates`, `settings.exactDatesDesc`

---

## Phase 4: Plex OAuth — Proper Reimplementation

**Estimated effort:** 5–6 hours

**Problem:** The current Plex OAuth implementation has fundamental architectural
issues. When a user is already logged into Plex, the popup loads the full Plex webapp
instead of the auth flow, and the token is never retrieved.

### Reference Implementation

Studied Maintainerr's working implementation at
`maintainerr/apps/ui/src/utils/PlexAuth.ts` — a battle-tested approach used in
production by thousands of users.

### Root Cause Analysis (8 issues)

| # | Issue | Capacitarr (broken) | Maintainerr (working) |
|---|-------|---------------------|----------------------|
| 1 | PIN creation | Via backend proxy (`POST /api/v1/.../pin`) | Direct browser → `plex.tv/api/v2/pins` |
| 2 | PIN polling | Via backend proxy (`GET /api/v1/.../pin/:id`) | Direct browser → `plex.tv/api/v2/pins/:id` |
| 3 | Auth URL format | `https://app.plex.tv/auth#?...` | `https://app.plex.tv/auth/#!?...` |
| 4 | Device headers | 2 headers only | 12 full device context headers |
| 5 | `forwardUrl` | `forwardUrl=close` (unreliable) | Not used |
| 6 | Popup strategy | Opens directly to Plex auth URL | Opens `about:blank` first, then redirects |
| 7 | Context params | Only `product` | Full device context (9 fields) |
| 8 | Client ID | Static `"capacitarr"` | Persistent UUID per browser |

**The core issue:** Plex's auth page requires proper device context headers and the
`#!` URL format to render the auth flow. Without them, it falls back to the full
webapp. Proxying through the backend loses the browser's device context entirely.

### 4.1 — Create Client-Side PlexOAuth Class

**File:** `frontend/app/utils/plexOAuth.ts` (new)

Port the pattern from Maintainerr's `PlexAuth.ts`, adapted for Nuxt/Vue:

**Key design points:**
- PIN creation and polling happen **client-side** (browser → `plex.tv` directly)
- Full 12-field device headers sent with every Plex API request
- URL uses `auth/#!?` (exclamation mark forces Plex SPA to handle the auth route)
- Popup opens to `about:blank` first, then redirects (avoids popup blockers)
- 1-second polling interval (not 2 seconds)
- Client ID is a persistent UUID per browser (stored in localStorage)
- Lightweight UA detection (no external dependency like Bowser — use
  `navigator.userAgentData` API with fallbacks)

**Class API:**

```typescript
export class PlexOAuth {
  constructor()             // builds device headers from browser UA
  login(): Promise<string>  // full flow: create PIN → open popup → poll → return token
  abort(): void             // cancel the flow and close popup
}
```

**Flow:**
1. Create PIN via `POST https://plex.tv/api/v2/pins?strong=true` with full headers
2. Open popup to `about:blank`, then redirect to
   `https://app.plex.tv/auth/#!?clientID=...&code=...&context[device][...]`
3. Poll `GET https://plex.tv/api/v2/pins/{id}` every 1 second with same headers
4. If `authToken` returned → resolve with token, close popup
5. If no token and popup still open → continue polling
6. If popup closed without token → reject (user cancelled)

**Changes:**
- [x] Create `frontend/app/utils/plexOAuth.ts` — full PlexOAuth class
- [x] Implement lightweight UA detection helpers (platform, browser, version)
- [x] Implement persistent device ID via `localStorage`

### 4.2 — Update Frontend Integration

**File:** `frontend/app/pages/settings.vue`

Replace the entire `startPlexAuth()` function (lines ~1648–1708) with the new class:

```typescript
import { PlexOAuth } from '~/utils/plexOAuth'

let plexOAuth: PlexOAuth | null = null

async function startPlexAuth() {
  plexAuthLoading.value = true
  try {
    plexOAuth = new PlexOAuth()
    const authToken = await plexOAuth.login()
    formState.apiKey = authToken
    addToast('Plex authorized successfully!', 'success')
  } catch (e) {
    const msg = e instanceof Error ? e.message : 'Unknown error'
    if (msg.includes('closed')) {
      addToast('Plex authorization cancelled', 'info')
    } else {
      addToast('Failed to start Plex authorization: ' + msg, 'error')
    }
  } finally {
    plexAuthLoading.value = false
    plexOAuth = null
  }
}
```

**Changes:**
- [x] Replace inline Plex auth logic with `PlexOAuth` class
- [x] Update abort logic to call `plexOAuth.abort()`
- [x] Remove all references to old backend Plex auth API calls

### 4.3 — Backend Cleanup

**File:** `backend/routes/plex_auth.go`

The backend PIN proxy routes are no longer needed — PIN operations now happen
client-side (browser → plex.tv directly). This eliminates an unnecessary
intermediary and ensures Plex receives the correct device context.

**Changes:**
- [x] Remove `routes/plex_auth.go` entirely
- [x] Remove `RegisterPlexAuthRoutes()` call from `routes/api.go`
- [x] Remove any related test files/functions

---

## Phase 5: Enrichment Rule Fields

**Estimated effort:** 3–4 hours

**Problem:** Several valuable data points from Plex, Tautulli, and Overseerr are
fetched and stored on `MediaItem` during enrichment, but not exposed as custom rule
fields. Users cannot create rules like "last watched > 180 days ago" or
"requested by specific user".

### Currently Available vs Missing

| Field | Source | Status | Rule Use Case |
|-------|--------|:------:|---------------|
| `playcount` | Plex/Tautulli | ✅ Exists | "Play count == 0 → lean remove" |
| `requested` | Overseerr | ✅ Exists | "Is requested == true → always keep" |
| `requestcount` | Overseerr | ✅ Exists | "Request count > 0 → prefer keep" |
| `lastplayed` | Plex/Tautulli | ❌ **New** | "Last watched over 180 days ago → lean remove" |
| `requestedby` | Overseerr | ❌ **New** | "Requested by == 'alice' → always keep" |
| `incollection` | Plex | ❌ **New** | "In collection == true → prefer keep" |
| `watchedbyreq` | Overseerr + Tautulli | ❌ **New** | "Watched by requestor == true → lean remove" |

### 5.1 — Add `lastplayed` Rule Field with Date-Aware Operators

**Backend files:** `routes/rules.go`, `internal/engine/rules.go`

New field: `lastplayed` — date-based field for when the item was last played. Uses
`MediaItem.LastPlayed` which is already populated by Plex, Tautulli, Jellyfin,
and Emby during enrichment.

**Date-aware operators** (new operator type for date fields):

| Operator | Frontend Label | Meaning | Example |
|----------|---------------|---------|---------|
| `in_last` | "in the last" | Last played within X days | "Last Played in the last 30 days" |
| `over_ago` | "over...ago" | Last played more than X days ago | "Last Played over 180 days ago" |
| `never` | "never" | Item has never been played | "Last Played never" |

These operators are friendlier than raw `>` / `<` for date-based decisions. The
backend converts them internally:
- `in_last 30` → days since last played < 30
- `over_ago 180` → days since last played > 180
- `never` → lastPlayed is nil or zero

```go
// In matchesRuleWithValue():
case "lastplayed":
    if item.LastPlayed == nil || item.LastPlayed.IsZero() {
        if cond == "never" { return true, "never played" }
        if cond == "over_ago" { return true, "never played" }
        return false, "never played"
    }
    if cond == "never" { return false, formatDaysAgo(item.LastPlayed) }
    ruleNum, err := strconv.ParseFloat(val, 64)
    if err != nil { return false, "" }
    days := time.Since(*item.LastPlayed).Hours() / 24.0
    switch cond {
    case "in_last":
        return days <= ruleNum, fmt.Sprintf("%.0f days ago", days)
    case "over_ago":
        return days > ruleNum, fmt.Sprintf("%.0f days ago", days)
    }
    return false, ""
```

**Also update `timeinlibrary` operators.** The existing `timeinlibrary` field (already
implemented with `>`, `>=`, `<`, `<=` operators) will be updated to use the same
date-aware `in_last` / `over_ago` operators for consistency. The backend must accept
both the old operators (backward compat for existing rules) and the new date-aware
operators. The frontend dropdown will show only the friendlier labels going forward.

**Changes:**
- [x] Add `lastplayed` to rule-fields endpoint (type: `date`, operators: `in_last`, `over_ago`, `never`)
- [x] Add `case "lastplayed"` to `matchesRuleWithValue()` with date-aware operator handling
- [x] Add operator labels in frontend: `in_last: 'in the last'`, `over_ago: 'over...ago'`, `never: 'never'`
- [x] Add suffix hint in value input: "days" (hidden for `never` operator)
- [x] Update `timeinlibrary` to support `in_last` / `over_ago` operators (keep old `>` / `<` for backward compat)
- [x] Update `timeinlibrary` frontend operator dropdown to show date-aware labels
- [x] Add tests in `rules_test.go` (including edge cases: never played, zero days)
- [x] Add frontend label in `rules.vue` fieldLabel map: `lastplayed: 'Last Watched'`

### 5.2 — Add `requestedby` Rule Field

New field: `requestedby` — string, the username of who requested the item via
Overseerr. Uses `MediaItem.RequestedBy` already populated during enrichment.

**Changes:**
- [x] Add `requestedby` to rule-fields endpoint (type: string, operators: `==`, `!=`, `contains`, `!contains`)
- [x] Add `case "requestedby"` to `matchesRuleWithValue()`
- [x] Add tests
- [x] Frontend label: `requestedby: 'Requested By'`

### 5.3 — Add `incollection` Rule Field (Plex Collections)

**New data needed:** Plex provides collection membership via the `/library/sections/{key}/all`
endpoint's `Collection` field in the metadata. This requires:

1. Adding `Collections []string` to `MediaItem` in `types.go`
2. Parsing `Collection` array from Plex metadata in `plex.go`
3. Adding a boolean rule field `incollection` or a string field `collection`

For simplicity, start with a boolean `incollection` and iterate:

**Changes:**
- [x] Add `Collections []string` field to `MediaItem` struct in `types.go`
- [x] Parse `Collection` from `plexMetadata` in `plex.go` (similar to Genre parsing)
- [x] Add `incollection` boolean rule field (true if `len(item.Collections) > 0`)
- [x] Future: add `collection` string field for matching specific collection names
- [x] Tests + frontend label

### 5.4 — Add `watchedbyreq` Rule Field (Watched By Requestor)

**Concept:** Cross-reference Overseerr's `RequestedBy` with Tautulli's `Users` list
to determine if the person who requested the content has actually watched it.
This is powerful for cleanup: "the user asked for this movie, watched it, so it's
safe to remove."

This requires enrichment to set a new boolean field:

```go
// In enrichItems(), after both Overseerr and Tautulli enrichment:
// Cross-reference: did the requestor watch it?
if ec.overseerr != nil && ec.tautulli != nil {
    for i := range items {
        item := &items[i]
        if item.IsRequested && item.RequestedBy != "" {
            // Check if requestor is in the Tautulli users list
            // (need to fetch/store users per item during Tautulli enrichment)
        }
    }
}
```

**Important:** Plex alone cannot provide per-user watch data — it only reports
aggregate `viewCount` and `lastViewedAt` across all users. **Tautulli is required**
for per-user history (the `user` field in its history API). Jellyfin and Emby have
similar limitations — they report admin-user watch data only. So `watchedbyreq`
specifically requires Tautulli + Overseerr.

**Challenge:** Currently Tautulli `Users` data is fetched per-item but NOT stored on
`MediaItem`. The `TautulliWatchData.Users` is consumed during enrichment but only
`PlayCount` and `LastPlayed` are persisted to the item.

**Solution:** Add `WatchedByUsers []string` to `MediaItem`, populated during Tautulli
enrichment, then cross-reference with `RequestedBy`.

**Changes:**
- [x] Add `WatchedByUsers []string` to `MediaItem` in `types.go`
- [x] Store `watchData.Users` during Tautulli enrichment in `fetch.go`
- [x] Add cross-reference logic after both enrichments
- [x] Add `WatchedByRequestor bool` to `MediaItem`
- [x] Add `watchedbyreq` boolean rule field
- [x] Tests + frontend label

---

## Phase 6: Approval Queue

**Estimated effort:** 8–10 hours

### Design Decision: No Separate Table

Rather than a separate `approval_queue` table (which would duplicate data), we'll:

1. **Extend the audit log** with the fields needed to reconstruct a delete job
   (integration ID, external ID)
2. **Use the existing deletion priority list** on the Rules/Scoring Engine page as the
   approval UI — add an approve/reject column after the Size column
3. Only show the approve/reject column when execution mode is "approval"

This approach:
- ✅ No data duplication
- ✅ Reuses existing UI (deletion priority table)
- ✅ Contextually appropriate (you see scores alongside approve/reject)
- ✅ Minimal new backend routes (just approve/reject endpoints)

### 6.1 — Database Migration: Extend Audit Log

**File:** `backend/internal/db/migrations/00013_add_approval_fields.sql` (new)

```sql
-- Add fields to audit_logs for reconstructing delete jobs from approval entries
ALTER TABLE audit_logs ADD COLUMN integration_id INTEGER DEFAULT NULL;
ALTER TABLE audit_logs ADD COLUMN external_id TEXT DEFAULT '';
```

**Changes:**
- [x] Create migration `00013_add_approval_fields.sql`
- [x] Update `AuditLog` model in `models.go` to include `IntegrationID` and `ExternalID`
- [x] Update `evaluate.go` approval path to store these fields in audit entries

### 6.2 — Backend: Approve/Reject Endpoints

**File:** `backend/routes/audit.go`

```
POST /api/v1/audit/:id/approve  — finds "Queued for Approval" entry, pushes to deleteQueue
POST /api/v1/audit/:id/reject   — updates action to "Rejected"
```

Approve logic:
1. Find audit entry by ID, verify action is "Queued for Approval"
2. Look up the integration client by `integration_id`
3. Construct a `deleteJob` from the stored data
4. Push to `deleteQueue` channel (same path as auto mode)
5. Update audit entry action to "Approved"

Reject logic:
1. Find audit entry by ID, verify action is "Queued for Approval"
2. Update action to "Rejected"

**Changes:**
- [x] Add `POST /api/v1/audit/:id/approve` route
- [x] Add `POST /api/v1/audit/:id/reject` route
- [x] Add `GET /api/v1/audit?action=Queued+for+Approval` filtering (may already work)
- [x] Export `deleteQueue` or add a `QueueDeletion()` function in the poller package
- [x] Tests in `audit_test.go`

### 6.3 — Frontend: Approval Column in Deletion Priority Table

**File:** `frontend/app/pages/rules.vue`

When execution mode is "approval", add a rightmost column to the deletion priority
table with approve/reject buttons:

**Table header addition** (after Size column):
```vue
<UiTableHead v-if="isApprovalMode" class="w-24 text-center">
  Action
</UiTableHead>
```

**Table cell addition** (after Size cell, for each row):
```vue
<UiTableCell v-if="isApprovalMode" class="text-center">
  <div v-if="!group.entry.isProtected" class="flex items-center justify-center gap-1">
    <UiButton
      variant="ghost"
      size="icon-sm"
      class="text-green-600 hover:text-green-700 hover:bg-green-50"
      @click="approveItem(group.entry)"
    >
      <CheckIcon class="w-4 h-4" />
    </UiButton>
    <UiButton
      variant="ghost"
      size="icon-sm"
      class="text-red-500 hover:text-red-600 hover:bg-red-50"
      @click="rejectItem(group.entry)"
    >
      <XIcon class="w-4 h-4" />
    </UiButton>
  </div>
</UiTableCell>
```

**Also needed:**
- Fetch current execution mode from the engine stats endpoint
- Add `approveItem()` and `rejectItem()` methods that call the new API routes
- Pending approval count badge somewhere visible (dashboard or nav)

**Changes:**
- [x] Add conditional "Action" column header and cells
- [x] Wire approve/reject to API endpoints
- [x] Add loading states for approve/reject buttons
- [x] Add approval count to dashboard queue display
- [x] Add i18n keys for approve/reject button labels
- [x] Same treatment for season-level rows

---

## Phase 7: Testing and Polish

**Estimated effort:** 2–3 hours

- [x] Run full backend test suite (`go test ./...`)
- [x] Verify all new API endpoints with manual testing via Docker Compose
- [x] Test Plex OAuth flow with an already-logged-in Plex account
- [x] Test first-login setup flow (fresh database)
- [x] Test approval queue workflow end-to-end
- [x] Verify all new rule fields appear in the rule builder dropdown
- [x] Verify date toggle works on all pages
- [x] Update locale files for any new i18n keys
- [x] Verify no regressions on existing functionality

---

## Summary

| Phase | Description | Effort | Files Touched |
|:-----:|-------------|:------:|:-------------:|
| 1 | Quick wins (API key + badge) | 30 min | 2 |
| 2 | First login UX | 2–3h | 4 |
| 3 | Date toggle | 4–6h | 10+ |
| 4 | Plex OAuth reimplementation | 5–6h | 3 |
| 5 | Enrichment rule fields | 3–4h | 6 |
| 6 | Approval queue | 8–10h | 6 |
| 7 | Testing & polish | 2–3h | — |
| **Total** | | **~25–33h** | |

---

## Completion Notes

**Completed:** 2026-03-04
**Branch:** `feature/ux-polish-enrichments`
**Commits:** 8 (7 feature commits + 1 fix commit)

All 7 phases implemented successfully:
- Phase 1: API key truncation and effect badge repositioning
- Phase 2: Auth status endpoint and first-login setup UX
- Phase 3: DateDisplay component with global/local toggle and settings control
- Phase 4: Client-side PlexOAuth class replacing backend proxy
- Phase 5: 4 new rule fields (lastplayed, requestedby, incollection, watchedbyreq) with date-aware operators
- Phase 6: Approval queue with DB migration, approve/reject API routes, and conditional UI column
- Phase 7: Full test suite passing, TypeScript strict mode fixes

Backend: All 7 test packages pass, go vet clean, go build clean.
Frontend: TypeScript issues from this branch resolved; pre-existing strict mode issues noted but not in scope.
