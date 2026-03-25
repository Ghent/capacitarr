# Deletion Priority Poster View — Dimming Bug Investigation

**Status:** 📋 Planned
**Branch:** `fix/misc-ui-issues` (or `debug/poster-dimming`)
**Reported by:** User — items below the "engine stops here" line are not dimmed in poster/grid view on the Rules page deletion priority, though they ARE dimmed in list/table view.

## Symptoms

- **List view:** Items below the deletion line are correctly dimmed (`opacity-40`)
- **Poster/grid view:** Items below the deletion line appear at full opacity — no dimming
- User confirms this worked before the `fix/misc-ui-issues` changes

## Code Path

### Table View (WORKING)
```
RulePreviewTable.vue line 192-193:
  deletionLineIndex !== null && vRow.entry.groupIdx >= deletionLineIndex
    → 'opacity-40'
```

### Grid/Poster View (BROKEN)
```
RulePreviewTable.vue line 82 (show groups):
  :is-flagged="deletionLineIndex !== null && groupIdx >= deletionLineIndex"

RulePreviewTable.vue line 133 (non-show items):
  :is-flagged="deletionLineIndex !== null && groupIdx >= deletionLineIndex"

MediaPosterCard.vue line 136:
  'opacity-40': isFlagged && !isProtected
```

### Deletion Line Calculation
```
RulePreviewTable.vue deletionLineIndex computed:
  Iterates groupedPreview.value, cumulates sizeBytes
  Returns the index where cumulative >= bytesToFree
```

## Investigation Steps

### Step 1: Verify `deletionLineIndex` is not null
- Open browser dev tools → Vue DevTools
- Inspect `RulePreviewTable` component
- Check `deletionLineIndex` computed value
- If `null`: the disk context `bytesToFree` is 0 — no deletion needed, no dimming expected
- If a valid number: proceed to Step 2

### Step 2: Verify `groupIdx` comparison in grid template
- The grid uses `v-for="(group, groupIdx) in renderedGroups"`
- `renderedGroups = groupedPreview.value.slice(0, gridVisibleCount.value)`
- Check if `groupIdx` values are being compared correctly to `deletionLineIndex`
- The issue might be that `renderedGroups` is a slice so `groupIdx` starts at 0, matching `deletionLineIndex` correctly

### Step 3: Check if the deletion line divider renders
- In the grid template, line 62: `v-if="deletionLineIndex !== null && deletionLineIndex === groupIdx"`
- This should render a full-width "Engine stops here" divider
- If the divider is visible but dimming isn't → the `isFlagged` prop is the issue
- If neither is visible → `deletionLineIndex` is null or out of range

### Step 4: Check `isFlagged` prop on MediaPosterCard
- In Vue DevTools, inspect a poster card that should be below the deletion line
- Check its `isFlagged` prop value
- If `false`: the condition `groupIdx >= deletionLineIndex` is not being met
- If `true`: the CSS class `opacity-40` should be applied — check if Tailwind purge removed it

### Step 5: Compare with previous working state
- Check out the commit BEFORE the filter removal changes
- Verify the poster dimming works
- If it works → the filter removal broke something
- If it doesn't work → this was a pre-existing bug

### Step 6: Potential root cause — `filteredGroupedPreview` removal
The old code had `filteredGroupedPreview` which:
1. Applied search/type/status/rule filters
2. Applied sorting (score, title, type, size)

The `deletionLineIndex` was computed against `filteredGroupedPreview.value` (now renamed to `groupedPreview`). The old `filteredGroupedPreview` included sorting that could reorder items. Without the sorting logic, items might be in a different order than expected, causing the deletion line to be at an unexpected position.

However, `groupedPreview` feeds from `props.preview` which comes from the backend already sorted by score. The `groupEvaluatedItems()` function preserves order for non-show items and groups shows. So the order should be correct.

**More likely cause:** The old code's sort logic (when `sortBy === 'rank'` and `sortDir === 'asc'`) returned the natural order — which is what we now have. So removing the sort shouldn't change behavior for the default rank-ascending view. But the old code also had a `previewSortBy` watcher that reset scroll position — maybe the deletion line calculation depended on scroll position?

## Fix Approaches

### If `deletionLineIndex` is null:
- Check `props.diskContext` — is it being passed correctly?
- Check `diskContext.bytesToFree` — is it > 0?
- If the backend isn't sending disk context for the preview, this is a backend issue

### If `isFlagged` is false when it should be true:
- Debug the `groupIdx >= deletionLineIndex` comparison
- Check types: `groupIdx` (number from v-for) vs `deletionLineIndex` (number from computed)

### If CSS class not applied:
- Check if `opacity-40` is in the Tailwind safelist or used elsewhere
- The class IS used in the table view (`opacity-40` at line 193) so it shouldn't be purged

## Files

| File | Relevance |
|------|-----------|
| `frontend/app/components/rules/RulePreviewTable.vue` | Grid template, deletionLineIndex, renderedGroups |
| `frontend/app/components/MediaPosterCard.vue` | isFlagged prop → opacity-40 class |
| `frontend/app/utils/groupPreview.ts` | groupEvaluatedItems ordering |
