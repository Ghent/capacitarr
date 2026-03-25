# Misc UI Issues — Comprehensive Plan

**Status:** 🔵 In Progress
**Issue:** [starshadow/software/capacitarr#9](https://gitlab.com/starshadow/software/capacitarr/-/work_items/9) (expanded scope)
**Branch:** `fix/misc-ui-issues`
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

## Remaining Items

### Phase 1: Bug Fixes

#### Step 1: Artist/Book filter buttons appearing without integrations

**Bug:** Media type filter buttons (artist, book) appear even when no Lidarr/Readarr integrations are configured.

**Root cause:** `mediaTypes` in `LibraryTable.vue` derives from `props.items` (raw items pre-dedup). If the backend preview data contains stale cached items from previously-configured integrations, or if the Sonarr/Radarr APIs return unexpected types, those filter buttons appear incorrectly.

**Fix:** Extract the dedup logic into a `dedupedItems` computed, then derive `mediaTypes` from the deduped + unfiltered items (before search/type/integration filters, but after dedup). This ensures only types actually present in the current data set generate buttons.

Additionally, the `mediaTypes` computed should exclude types that have no configured integration. Pass the `integrations` list to `LibraryTable` and cross-reference: only show a type button if at least one enabled integration provides that type.

**Files:**
- `frontend/app/components/LibraryTable.vue`

#### Step 2: Shows filter empty when showLevelOnly=true

**Bug:** When Sonarr has `showLevelOnly=true`, clicking the "Show" filter in the Library shows "no items match filters."

**Root cause:** The Shows filter logic does `result.filter(e => e.item.type === 'season')`. With `showLevelOnly=true`, the backend sends only `type=show` items (no seasons), so the filter matches nothing.

**Fix:** Change the Shows filter to match both types: `result.filter(e => e.item.type === 'season' || e.item.type === 'show')`.

When items are purely `show` type (showLevelOnly mode), the group header logic should still work — each show gets a header with "1 show" instead of "N seasons", and the show item itself appears below the header. Or simpler: skip group headers entirely when items are already show-level (no grouping needed since each show IS the atomic unit).

**Files:**
- `frontend/app/components/LibraryTable.vue` — `filteredItems` and `displayRows` computed

#### Step 3: Selection not working for show cards in poster/grid view

**Bug:** In grid view with Shows filter active, clicking a show card does nothing in selection mode.

**Root cause:** The grid template explicitly disables selection for shows: `:selectable="selectionMode && !isShowsFilter"`. This was intentional because aggregated show cards don't map to a single deletable item, but the UX is broken.

**Fix:** When a show card is selected/deselected in grid view, toggle all of its underlying season items in `selectedIds`. Map the show title back to its season items via `filteredItems.filter(e => e.item.showTitle === showTitle)`.

**Files:**
- `frontend/app/components/LibraryTable.vue` — grid view selection handlers

#### Step 4: Show group header select-all checkbox in table view

**Bug:** The show group header row in table view has an empty cell where the selection checkbox should be.

**Fix:** Add a checkbox in the group header that toggles all seasons belonging to that show. When all seasons are selected, show a filled checkbox. When some are selected, show an indeterminate state. When none are selected, show an empty checkbox.

**Files:**
- `frontend/app/components/LibraryTable.vue` — group header row template

### Phase 2: Score/Factor Issues

#### Step 5: Score detail colors gray

**Bug:** In the score detail breakdown, many factor lines are gray instead of colored (green/red).

**Investigation needed:** Check `ScoreBreakdown.vue` or `ScoreDetailModal.vue` for how factor contribution colors are determined. Likely: factors from broken integrations have zero contribution, falling into a "neutral" color bucket.

**Files:**
- `frontend/app/components/ScoreBreakdown.vue`
- `frontend/app/components/ScoreDetailModal.vue`

#### Step 6: Broken integrations affecting score weights

**Issue:** When an integration (e.g., Seerr, Tautulli) has a connection error, its scoring factors still participate in the calculation with zero/default values, which can unfairly bias scores.

**Design decision:** When a scoring factor's data source has a connection error, that factor should be excluded from the score calculation entirely (as if its weight were 0), rather than contributing with zero data. The `EvaluationContext` already tracks which integration types are configured — it should also track which are currently healthy vs errored.

**Fix approach:**
1. Backend: Pass connection-error integration IDs into `EvaluationContext`
2. Engine: Skip factors whose required integration has a connection error
3. Frontend: Factor weight UI already shows `integrationError` flag — no change needed

**Files:**
- `backend/internal/engine/evaluator.go`
- `backend/internal/engine/factors.go`
- `backend/internal/poller/evaluate.go` or `backend/internal/services/preview.go`

### Phase 3: View Simplification

#### Step 7: Remove filters from RulePreviewTable (Deletion Priority)

**Change:** The "Deletion Priority" card on the Rules page (`RulePreviewTable.vue`) currently has search, type filters, integration dropdown, sort controls, and view mode toggle. Remove everything except the view mode toggle (table/grid) and the refresh button. The list shows the full scored ranking with the "engine stops here" line. No filtering, no searching, no sorting (always sorted by deletion score).

**Rationale:** The deletion priority view must reflect exactly what the engine sees. Filtering the view makes the "engine stops here" line position misleading. Users who want to browse/filter should use the Library page.

**Files:**
- `frontend/app/components/rules/RulePreviewTable.vue`

### Phase 4: Audits

#### Step 8: Collection deletion end-to-end audit

**Verification:** Trace the collection deletion feature through the entire stack:
1. Frontend toggle → API call → DB field update
2. Backend: `CollectionResolver` interface implementation per integration
3. Poller: `evaluate.go` expanding collection members when enabled
4. Deletion: `deletion.go` handling collection groups
5. Audit log: collection group recorded
6. Notifications: collection deletion mentioned

Document any gaps or incomplete wiring.

**Files to audit:**
- `backend/internal/integrations/registry.go` — CollectionResolver lookup
- `backend/internal/poller/evaluate.go` — collection expansion
- `backend/internal/services/deletion.go` — collection deletion execution
- `backend/internal/services/approval.go` — collection group in approval queue

#### Step 9: Verify "requested" filter when Seerr is healthy

**Verification:** The "Requested" smart filter in `library.vue` checks `e.item.isRequested`. This field is populated by the Seerr enrichment pipeline. Verify:
1. When Seerr is connected and healthy, `isRequested` is correctly set on items
2. The filter button shows the correct count
3. Clicking the filter shows only requested items

This can only be verified when the Seerr integration is actually connected. If the connection is currently broken, note it as a deferred verification.

---

## Implementation Order

1. Step 1 — Artist/book filter buttons (quick fix)
2. Step 2 — Shows filter empty with showLevelOnly (quick fix)
3. Step 3 — Show card selection in grid view (medium)
4. Step 4 — Group header select-all checkbox (medium)
5. Step 5 — Score detail colors investigation (investigation)
6. Step 6 — Broken integration score weights (design + backend)
7. Step 7 — Remove filters from RulePreviewTable (frontend cleanup)
8. Step 8 — Collection deletion audit (verification)
9. Step 9 — Requested filter verification (deferred if Seerr is down)

## Acceptance Criteria

- [ ] No artist/book filter buttons when Lidarr/Readarr are not configured
- [ ] Shows filter works correctly with showLevelOnly=true (shows show-level items)
- [ ] Show cards in grid view can be selected (selects all underlying seasons)
- [ ] Show group headers in table view have a select-all checkbox
- [ ] Score detail breakdown uses correct colors for all factors
- [ ] Broken integrations do not contribute zero-value factors to scores
- [ ] Deletion Priority on Rules page has no search/filter/sort controls
- [ ] Collection deletion feature is fully wired end-to-end
- [ ] Requested filter works when Seerr is connected
- [ ] `make ci` passes
- [ ] Visual review confirms all changes
