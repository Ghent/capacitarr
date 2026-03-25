# Misc UI Issues — Comprehensive Plan

**Status:** ✅ Complete
**Issue:** [starshadow/software/capacitarr#9](https://gitlab.com/starshadow/software/capacitarr/-/work_items/9) (expanded scope)
**Branch:** `fix/misc-ui-issues` (Steps 1–4, 7), `fix/media-type-filter-integration-check` (Step 1 integration cross-reference)
**Supersedes:** `20260325T1245Z-issue-9-shows-filter.md`, `20260325T1320Z-issue-9-expanded-ux.md`

## Already Completed (prior commits on this branch)

- [x] Shows filter with group headers in table view
- [x] Shows filter with aggregated show cards in grid view
- [x] Selection checkbox moved to right side of table rows
- [x] Integration card toggles (showLevelOnly, collectionDeletion)
- [x] showLevelOnly bug: DB-persisted cache cleared on invalidation
- [x] Search matches against showTitle
- [x] Season count badge format updated to "N seasons"

---

## Completed Items

### Phase 1: Bug Fixes

#### Step 1: Artist/Book filter buttons appearing without integrations ✅

**Bug:** Media type filter buttons (artist, book) appear even when no Lidarr/Readarr integrations are configured.

**Root cause:** `mediaTypes` in `LibraryTable.vue` derives from `props.items` (raw items pre-dedup). If the backend preview data contains stale cached items from previously-configured integrations, or if the Sonarr/Radarr APIs return unexpected types, those filter buttons appear incorrectly.

**Fix:** Extract the dedup logic into a `dedupedItems` computed, then derive `mediaTypes` from the deduped + unfiltered items (before search/type/integration filters, but after dedup). This ensures only types actually present in the current data set generate buttons.

Additionally, the `mediaTypes` computed excludes types that have no configured integration. A `MEDIA_TYPE_TO_INTEGRATION` map cross-references each media type against `props.integrations`: only types backed by at least one configured integration generate filter buttons.

**Files:**
- `frontend/app/components/LibraryTable.vue`

#### Step 2: Shows filter empty when showLevelOnly=true ✅

**Bug:** When Sonarr has `showLevelOnly=true`, clicking the "Show" filter in the Library shows "no items match filters."

**Root cause:** The Shows filter logic does `result.filter(e => e.item.type === 'season')`. With `showLevelOnly=true`, the backend sends only `type=show` items (no seasons), so the filter matches nothing.

**Fix:** Changed the Shows filter to match both types: `result.filter(e => e.item.type === 'season' || e.item.type === 'show')`.

When items are purely `show` type (showLevelOnly mode), the group header logic still works — each show gets a header with "1 show" instead of "N seasons", and the show item itself appears below the header.

**Files:**
- `frontend/app/components/LibraryTable.vue` — `filteredItems` and `displayRows` computed

#### Step 3: Selection not working for show cards in poster/grid view ✅

**Bug:** In grid view with Shows filter active, clicking a show card does nothing in selection mode.

**Root cause:** The grid template explicitly disables selection for shows: `:selectable="selectionMode && !isShowsFilter"`. This was intentional because aggregated show cards don't map to a single deletable item, but the UX is broken.

**Fix:** Added `toggleShowSeasons()` function that toggles all underlying season items when a show card is selected/deselected. Grid view click handlers now call `toggleShowSeasons()` for show cards and `toggleItem()` for non-show items. The `:selectable` prop is now always `selectionMode` (no longer disabled for shows).

**Files:**
- `frontend/app/components/LibraryTable.vue` — grid view selection handlers

#### Step 4: Show group header select-all checkbox in table view ✅

**Bug:** The show group header row in table view has an empty cell where the selection checkbox should be.

**Fix:** Added a clickable checkbox cell in the group header row that calls `toggleShowSeasons()`. Shows `CheckSquareIcon` when all seasons are selected, `SquareIcon` when none are selected.

**Note:** Indeterminate state (some-but-not-all selected) was mentioned in the plan but not implemented — the checkbox shows fully-selected or not-selected only.

**Files:**
- `frontend/app/components/LibraryTable.vue` — group header row template

### Phase 3: View Simplification

#### Step 7: Remove filters from RulePreviewTable (Deletion Priority) ✅

**Change:** Removed search, type filters, integration dropdown, and sort controls from the "Deletion Priority" card on the Rules page. Only the view mode toggle (table/grid) and the refresh button remain. The list shows the full scored ranking with the "engine stops here" line. No filtering, no searching, no sorting (always sorted by deletion score).

**Rationale:** The deletion priority view must reflect exactly what the engine sees. Filtering the view makes the "engine stops here" line position misleading. Users who want to browse/filter should use the Library page.

**Files:**
- `frontend/app/components/rules/RulePreviewTable.vue`

---

## Spun Out to Dedicated Plans

The following steps were extracted into their own plan files for independent tracking:

### Step 5: Score detail colors gray → `20260325T1444Z-factor-color-key-refactor.md`

**Investigation:** Score detail breakdown shows gray instead of colored (green/red) for many factor lines. The `FACTOR_COLORS` map uses display names as keys, which is brittle.

### Step 6: Broken integrations affecting score weights → `20260325T1444Z-broken-integration-scoring.md`

**Issue:** When an integration has a connection error, its scoring factors still participate with zero/default values, biasing scores. Requires backend `EvaluationContext` changes.

### Step 8: Collection deletion end-to-end audit → `20260325T1444Z-collection-deletion-audit.md`

**Verification:** Trace the collection deletion feature through the entire stack and document gaps.

### Step 9: Verify "requested" filter when Seerr is healthy → `20260325T1444Z-seerr-requested-filter.md`

**Verification:** Blocked on Seerr connection availability.

---

## Implementation Order

1. ~~Step 1 — Artist/book filter buttons (quick fix)~~ ✅
2. ~~Step 2 — Shows filter empty with showLevelOnly (quick fix)~~ ✅
3. ~~Step 3 — Show card selection in grid view (medium)~~ ✅
4. ~~Step 4 — Group header select-all checkbox (medium)~~ ✅
5. ~~Step 5 — Score detail colors investigation~~ → Spun out
6. ~~Step 6 — Broken integration score weights~~ → Spun out
7. ~~Step 7 — Remove filters from RulePreviewTable (frontend cleanup)~~ ✅
8. ~~Step 8 — Collection deletion audit~~ → Spun out
9. ~~Step 9 — Requested filter verification~~ → Spun out

## Acceptance Criteria

- [x] No artist/book filter buttons when Lidarr/Readarr are not configured
- [x] Shows filter works correctly with showLevelOnly=true (shows show-level items)
- [x] Show cards in grid view can be selected (selects all underlying seasons)
- [x] Show group headers in table view have a select-all checkbox
- [ ] Score detail breakdown uses correct colors for all factors → `20260325T1444Z-factor-color-key-refactor.md`
- [ ] Broken integrations do not contribute zero-value factors to scores → `20260325T1444Z-broken-integration-scoring.md`
- [x] Deletion Priority on Rules page has no search/filter/sort controls
- [ ] Collection deletion feature is fully wired end-to-end → `20260325T1444Z-collection-deletion-audit.md`
- [ ] Requested filter works when Seerr is connected → `20260325T1444Z-seerr-requested-filter.md`
- [x] `make ci` passes
- [ ] Visual review confirms all changes
