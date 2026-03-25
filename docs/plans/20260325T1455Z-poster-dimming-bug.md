# Deletion Priority Poster View — Dimming Bug Investigation

**Status:** ✅ Complete
**Branch:** `fix/poster-dimming`
**Reported by:** User — items below the "engine stops here" line are not dimmed in poster/grid view on the Rules page deletion priority, though they ARE dimmed in list/table view.

## Symptoms

- **List view:** Items below the deletion line are correctly dimmed (`opacity-40`)
- **Poster/grid view:** Items below the deletion line appear at full opacity — no dimming
- User confirms this worked before the `fix/misc-ui-issues` changes

## Root Cause

**`v-motion` inline `opacity: 1` overrides Tailwind `opacity-40` class.**

The `MediaPosterCard.vue` root `<div>` has both:
- `v-motion v-bind="motionProps"` — which sets `style="opacity: 1"` after animation
- `:class="{ 'opacity-40': isFlagged && !isProtected }"` — Tailwind class

CSS inline styles (`style="opacity: 1"`) have higher specificity than class-based
styles (`opacity-40` → `opacity: 0.4`), so the Tailwind class is permanently
overridden after the entrance animation completes.

### Why table view works but grid doesn't

- **Table view:** `opacity-40` is applied on `<UiTableRow>` elements which do NOT
  have `v-motion`. The only `v-motion` in `RulePreviewTable.vue` is on the outer
  `<UiCard>` wrapper.
- **Grid view:** `opacity-40` is applied inside `MediaPosterCard.vue` on the same
  `<div>` that has `v-motion` — so the inline style wins.

### When the bug was introduced

Commit `0076e13` (`feat(ui): v-motion presets, virtual scrolling`) added `v-motion`
+ `gridItem()` to `MediaPosterCard.vue`. This is BEFORE the `fix/misc-ui-issues`
commit — meaning the poster dimming was already broken before the filter removal.
The filter removal just made it more noticeable.

## Hypotheses Evaluated

| # | Hypothesis | Verdict |
|---|-----------|---------|
| 1 | `v-motion` inline `opacity: 1` overriding Tailwind `opacity-40` | ⭐ ROOT CAUSE |
| 2 | `deletionLineIndex` is null (diskContext not passed) | ❌ Table view works |
| 3 | `deletionLineIndex` exceeds `gridVisibleCount` (100) | ❌ General bug |
| 4 | `groupIdx` type coercion (v-for index vs computed number) | ❌ Both plain JS numbers |
| 5 | Tailwind purged `opacity-40` | ❌ Used in table view |
| 6 | CSS `transition-all` delays the change | ❌ Would delay, not prevent |
| 7 | `deletionLineIndex` double-counting show+season sizes | ❌ Wouldn't cause "no dimming" |

## Fix Applied

Removed `opacity` from the `gridItem()` v-motion preset in
`frontend/app/composables/useMotionPresets.ts`. The entrance animation now uses
scale-only (0.95 → 1.0), which was already the dominant visual cue. This allows
Tailwind classes to freely control opacity without inline-style interference.

```diff
 function gridItem(delay = 0) {
   return {
-    initial: { opacity: 0, scale: 0.95 },
+    initial: { scale: 0.95 },
     enter: {
-      opacity: 1,
       scale: 1,
       transition: { ...spring, delay: Math.min(delay, 300) },
     },
   };
 }
```

### Why this approach over alternatives

- **Fixes all callers** of `MediaPosterCard` at once (library, approval queue,
  deletion priority)
- **No DOM changes** — CSS grid layout stays intact
- **Future-proof** — prevents the same class of bug for any component using
  opacity classes with `MediaPosterCard`
- A wrapper `<div>` alternative would break CSS grid `aspect-[2/3]` sizing and
  need duplication in both the show-group popover path and the non-show path

## Files Changed

| File | Change |
|------|--------|
| `frontend/app/composables/useMotionPresets.ts` | Remove opacity from `gridItem()` preset |

## General Lesson

Animation libraries (`@vueuse/motion`, GSAP, Framer Motion) work by manipulating
inline styles for per-frame control. Many leave the final animation state as an
inline style after completion. This creates a specificity conflict with utility-class
CSS frameworks like Tailwind. **Avoid animating properties that Tailwind classes also
control on the same element**, or use transform-only animations that don't conflict.
