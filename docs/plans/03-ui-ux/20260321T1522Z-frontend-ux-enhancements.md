# Frontend UX Enhancements: v-motion, Virtual Scrolling, Disk Group Redesign

**Status:** ✅ Complete
**Branch:** `feature/frontend-ux-enhancements` (from `feature/2.0`)
**Created:** 2026-03-21T15:22Z

## Overview

Three frontend enhancements to improve polish, performance, and information density:

1. **v-motion everywhere** — Fill gaps in animation coverage, extract shared presets
2. **Virtual scrolling** — Virtualize the two remaining large-list components
3. **Disk group display redesign** — Replace the thermometer bar with a clean bar + sparkline combo using ECharts dual-grid layout

## Decisions

| Topic | Decision |
|-------|----------|
| v-motion | ✅ Fill all gaps + extract presets composable |
| Virtual scrolling | ✅ AuditLogPanel + RulePreviewTable only (small lists not worth it) |
| Queue sparklines (approval/snoozed/deletion counts) | ❌ Dropped — ephemeral queues have no meaningful trend signal |
| Disk group display | ✅ Option 3: ECharts slim bar + area sparkline combo in dual-grid layout |

---

## Phase 1: v-motion Presets Composable

Create a shared composable to eliminate the repeated motion config scattered across 47+ instances.

### Step 1.1: Create `composables/useMotionPresets.ts`

Create `frontend/app/composables/useMotionPresets.ts` exporting preset objects:

- `cardEntrance` — the standard `{ opacity: 0, y: 12 }` → `{ opacity: 1, y: 0, spring }` pattern
- `listItem(delay?)` — per-item entrance with optional stagger delay
- `slideInX(direction?)` — horizontal slide-in (for banners, sidebar elements)
- `fadeIn` — simple opacity fade
- `scaleIn` — opacity + slight scale (for cards, modals)

All presets should use the same spring config: `{ type: 'spring', stiffness: 260, damping: 24 }`.

### Step 1.2: Apply v-motion to `SettingsBackupRestore.vue`

Add `v-motion` with `cardEntrance` preset to all 5 `<UiCard>` elements in `SettingsBackupRestore.vue`. Every other settings page already has entrance animations — this is a consistency fix.

### Step 1.3: Migrate existing v-motion usages to presets (optional, low priority)

Gradually replace inline `v-motion` props across the codebase with the preset composable. This is a refactor-only change — no visual difference. Can be done incrementally.

---

## Phase 2: v-motion Gap Coverage

Apply v-motion animations to components that currently lack them.

### Step 2.1: `MediaPosterCard.vue` — Staggered grid entrance

Add `v-motion` with `scaleIn` preset to `MediaPosterCard.vue`. The parent component (`ApprovalQueueCard`, `LibraryTable` grid mode) should pass an `index` prop so each card can compute a stagger `:delay` (e.g., `index * 30ms`, capped at 300ms to avoid long waits on large grids).

### Step 2.2: `ApprovalQueueCard.vue` — Per-item entrance

Add `v-motion` with `listItem` preset to individual approval group items and season rows within the card content area. Follow the same pattern used in `SnoozedItemsCard.vue` (which already animates individual items).

### Step 2.3: `DeletionQueueCard.vue` — Per-item entrance

Add `v-motion` with `listItem` preset to individual queued and completed item rows.

### Step 2.4: `ScoreDetailModal.vue` — Factor row stagger

Add `v-motion` with `listItem(index * 40)` to each factor row inside the score detail modal body.

### Step 2.5: `ScoreBreakdown.vue` — Animated bar segments

Add `v-motion` to the stacked bar segments so they animate their width on initial render.

### Step 2.6: `ConnectionBanner.vue` — Migrate to v-motion

Replace the Vue `<Transition>` CSS class-based animation with `v-motion` spring physics for the reconnecting/disconnected/reconnected banners. Use `slideInX` or a vertical slide preset.

### Step 2.7: `BottomToolbar.vue` — Mount entrance

Add `v-motion` with a slide-up-from-bottom entrance to the toolbar footer.

### Step 2.8: `Navbar.vue` — Mount entrance

Add subtle `v-motion` fade-in on the navbar.

### Step 2.9: `EngineControlPopover.vue` — Content entrance

Add `v-motion` with `scaleIn` or `fadeIn` to the popover content when it opens.

---

## Phase 3: Virtual Scrolling — AuditLogPanel

### Step 3.1: Add `useVirtualizer` to `AuditLogPanel.vue`

Import `useVirtualizer` from `@tanstack/vue-virtual`. Create a virtualizer for the audit log table rows:

```ts
const auditScrollRef = ref<HTMLElement | null>(null);
const auditVirtualizer = useVirtualizer(computed(() => ({
  count: filteredLogs.value.length,
  getScrollElement: () => auditScrollRef.value,
  estimateSize: () => 44, // approximate row height
  overscan: 10,
})));
```

### Step 3.2: Implement spacer-row pattern

Follow the pattern established in `LibraryTable.vue`:

1. Add a `ref="auditScrollRef"` to the scrollable container (the `div` with `max-h-[600px] overflow-y-auto`)
2. Add a top spacer `<tr>` with `height: firstVirtualItem.start`
3. Render only the virtual items in the loop
4. Add a bottom spacer `<tr>` with `height: totalSize - lastVirtualItem.end`

### Step 3.3: Preserve sticky header

The sticky `<UiTableHeader>` must remain outside the virtualized area. The scroll container should wrap only the `<UiTableBody>`, not the header.

### Step 3.4: Test with large datasets

Verify scrolling performance with 1000+ audit log entries. Ensure sort and filter changes reset the virtualizer scroll position.

---

## Phase 4: Virtual Scrolling — RulePreviewTable

### Step 4.1: Add `useVirtualizer` to `RulePreviewTable.vue`

Same pattern as Phase 3. The preview table can contain items from the entire library.

### Step 4.2: Implement spacer-row pattern

Follow the same spacer-row approach. The preview table already has sort and filter controls — ensure they call `virtualizer.scrollToIndex(0)` on change (same pattern as `LibraryTable.vue`).

### Step 4.3: Test with large preview datasets

Simulate or test with 500+ preview items to verify smooth scrolling.

---

## Phase 5: Disk Group Display Redesign

Replace the current thermometer chart with a dual-grid ECharts layout: slim bar at top + area sparkline below.

### Step 5.1: Add per-disk-group history fetch to `DiskGroupSection.vue`

Add an `onMounted` fetch to `/api/v1/metrics/history?disk_group_id=${group.id}&resolution=hourly&since=7d` to get historical usage data for the sparkline. Store in a local `ref<LibraryHistory[]>`.

The API endpoint already exists at `routes/metrics.go` and accepts `disk_group_id` + `since` params. No backend changes needed.

### Step 5.2: Compute derived sparkline data

From the `LibraryHistory` entries:

- **Usage series:** `usedCapacity / totalCapacity * 100` per timestamp — plots disk usage %
- **Growth rate series (optional):** Compute the delta between consecutive data points, smoothed. This shows "are we gaining or losing capacity?"

### Step 5.3: Redesign `thermometerOption` to dual-grid layout

Replace the current single-grid bar chart with a dual-grid ECharts config:

**Grid 0 (bar, top 20px):**
- Same gradient fill bar and zone backgrounds as current
- Remove all `markLine` properties from the usage series (the ugly dashed lines)
- The zone backgrounds already communicate where target/threshold are

**Grid 1 (sparkline, bottom 60px):**
- `type: 'line'` series with `smooth: true`, `showSymbol: false`
- Area fill with gradient (using existing `gradientArea` from `useEChartsDefaults`)
- Line color matches the zone color (green/amber/red based on current usage)
- Horizontal `markLine` reference lines at `yAxis: targetPct` (green, dashed) and `yAxis: thresholdPct` (red, dashed) — these are the natural, clean way to show thresholds on a time-series chart

**Shared tooltip:**
- Hovering the bar shows current capacity stats (already implemented)
- Hovering the sparkline shows date + usage % at that point

### Step 5.4: Handle empty history gracefully

If no history data exists (fresh install, new disk group), fall back to showing only the bar without the sparkline. The chart height should adapt.

### Step 5.5: Update the `thermometer-critical` CSS animation

The existing `thermometer-pulse` animation should apply to the entire chart container (not just the bar area) so the sparkline also pulses when usage is above threshold.

### Step 5.6: Test with multiple disk groups

Each disk group card fetches its own history independently. Verify:
- Multiple concurrent fetches don't race or conflict
- Different disk groups with different history lengths render correctly
- Disk groups with no history show a clean fallback

---

## Files Changed

| File | Phase | Change Type |
|------|-------|-------------|
| `frontend/app/composables/useMotionPresets.ts` | 1 | **New file** |
| `frontend/app/components/settings/SettingsBackupRestore.vue` | 1 | Add v-motion |
| `frontend/app/components/MediaPosterCard.vue` | 2 | Add v-motion + index prop |
| `frontend/app/components/ApprovalQueueCard.vue` | 2 | Add v-motion to items |
| `frontend/app/components/DeletionQueueCard.vue` | 2 | Add v-motion to items |
| `frontend/app/components/ScoreDetailModal.vue` | 2 | Add v-motion to factor rows |
| `frontend/app/components/ScoreBreakdown.vue` | 2 | Add v-motion to bar segments |
| `frontend/app/components/ConnectionBanner.vue` | 2 | Migrate `<Transition>` to v-motion |
| `frontend/app/components/BottomToolbar.vue` | 2 | Add v-motion entrance |
| `frontend/app/components/Navbar.vue` | 2 | Add v-motion entrance |
| `frontend/app/components/EngineControlPopover.vue` | 2 | Add v-motion entrance |
| `frontend/app/components/AuditLogPanel.vue` | 3 | Add virtual scrolling |
| `frontend/app/components/rules/RulePreviewTable.vue` | 4 | Add virtual scrolling |
| `frontend/app/components/DiskGroupSection.vue` | 5 | Redesign chart to dual-grid bar + sparkline |

## Backend Changes

None required. All data needed is already available through existing API endpoints.
