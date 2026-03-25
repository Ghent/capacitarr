# Issue #9 Expanded: Shows/Seasons UX Improvements

**Status:** ✅ Complete
**Issue:** [starshadow/software/capacitarr#9](https://gitlab.com/starshadow/software/capacitarr/-/work_items/9) (expanded scope)
**Branch:** `fix/shows-filter`
**Depends on:** Initial fix already committed on this branch

## Items

### Item 1: Selection Checkbox Positioning

**Problem:** The selection checkbox column appears on the left side of the table. When entering selection mode, the new column shifts all other columns right, causing a jarring layout jump.

**Fix:** Move the checkbox `<UiTableCell>` from the first column to the last column in the table row. Also move the `<UiTableHead>` for the selection column to the end of the header row. This way entering selection mode adds a narrow column at the right edge without affecting existing column positions.

**Files:**
- `frontend/app/components/LibraryTable.vue` — table header and body selection cells

### Item 2: Poster/Grid View Differentiation for Shows

**Problem:** In grid/poster view, the Shows filter and Seasons filter look identical — both show individual season poster cards with no visual distinction.

**Fix:** When the Shows filter is active in grid view, display show-level cards that aggregate seasons. Each card would show the show title, show poster, season count badge, and total size. This mirrors the group header concept from the table view but adapted for the poster grid.

**Approach options:**
- **A) Show-level aggregate cards:** Render one poster card per unique show. Click expands to show individual seasons. More complex.
- **B) Section dividers:** Insert a full-width show title divider above each show group in the grid. Simpler, consistent with table view.

**Recommendation:** Option B — section dividers in grid view, consistent with the table group header approach.

**Files:**
- `frontend/app/components/LibraryTable.vue` — grid view rendering
- Possibly `frontend/app/components/MediaPosterCard.vue` if show-level cards are needed

### Item 3: Integration Toggles on Card

**Problem:** The `showLevelOnly` and `collectionDeletion` toggles are only accessible inside the Edit modal. Users must click Edit to see or change these settings.

**Fix:** Surface these toggles directly on the integration card body, below the existing URL/API key info. Show them only when applicable (showLevelOnly for Sonarr only, collectionDeletion for supported types).

**Files:**
- `frontend/app/components/settings/SettingsIntegrations.vue` — card content section (lines 88-119)

### Item 4: showLevelOnly Bug — Seasons Still Appearing

**Problem:** When a Sonarr integration has `showLevelOnly=true`, season-type items still appear in the Library and deletion priority. The Season filter button should not appear when there are no season items.

**Root Cause Investigation:**
The backend filters seasons in both paths:
- `poller/fetch.go` lines 70-80: Drops seasons during poll cycle
- `services/preview.go` lines 309-317: Drops seasons during cold-start preview build

Possible causes of the bug:
1. **DB-persisted preview cache:** If stale data was persisted before `showLevelOnly` was enabled, the restored cache could contain seasons. When the preview cache is invalidated (line 251-252 in preview.go), it sets cache to nil, which triggers `buildPreviewFromScratch` on next request. But if the DB-persisted cache is loaded before the invalidation happens...
2. **Race between cache restore and invalidation:** On startup, the persisted cache is loaded from DB. If the user toggled `showLevelOnly` while the app was down, the stale cached data loads first.
3. **Frontend dedup logic adding seasons back:** The frontend LibraryTable.vue dedup logic at lines 84-95 removes show entries when seasons exist, but when showLevelOnly=true and the backend sends only show items (no seasons), the dedup logic should leave show items alone. Need to verify the frontend isn't synthetically creating issues.

**Fix approach:**
- Add `showLevelOnly` filtering to the preview DB restoration path
- Ensure the persisted cache is invalidated/re-filtered when integration config changes
- On the frontend, the `mediaTypes` computed already derives from actual items — if no seasons exist, no Season button appears. No frontend change needed for this.

**Files:**
- `backend/internal/services/preview.go` — DB cache restore path

---

## Implementation Plan

### Step 1: Item 1 — Move selection checkbox to right side
1. In the table header, move the `<UiTableHead v-if="selectionMode">` from before the column loop to after it
2. In the table body, move the `<UiTableCell v-if="selectionMode">` (both for group headers and item rows) from the first position to the last
3. Verify the floating action bar at bottom still works

### Step 2: Item 3 — Surface integration toggles on card
1. In `SettingsIntegrations.vue`, add showLevelOnly and collectionDeletion toggles to the `<UiCardContent>` section (lines 88-119)
2. Show showLevelOnly only for Sonarr integrations
3. Show collectionDeletion only for supported types
4. Wire the toggles to call the update API directly (same as the edit modal does)

### Step 3: Item 4 — Fix showLevelOnly bug
1. Investigate the DB cache restore path in `preview.go`
2. Add showLevelOnly filtering when restoring from DB
3. Or: Clear persisted cache when integration config changes
4. Reproduce and verify the fix

### Step 4: Item 2 — Grid view show grouping
1. When Shows filter is active and viewMode is grid, insert full-width section headers between show groups
2. Each section header shows the show title, season count, and total size
3. Adapt grid virtualizer to handle mixed row types (section headers vs poster rows)

### Step 5: Verify all items
1. Run `make ci`
2. Visual review in browser for all 4 items
3. Commit with appropriate message

## Commit Message

```
fix(library): improve shows/seasons UX across library management

- Move selection checkbox to right side of table to prevent column shift
- Add show group headers in grid/poster view for visual grouping
- Surface showLevelOnly and collectionDeletion toggles on integration cards
- Fix showLevelOnly bug where seasons appeared despite being enabled

Reported-by: @tomislavf
Closes #9
```
