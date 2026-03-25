# Visual Polish Fix Plan — Comprehensive Overhaul

**Date:** 2026-03-01
**Status:** ✅ Complete — All 7 steps implemented.
**Branch:** `feature/visual-polish-apexcharts` (then cherry-pick to `feature/visual-polish-uplot`)

## Root Cause Analysis

The previous visual polish attempt failed because it made CSS-level changes to components that aren't actually used, and didn't address the fundamental issues. Here's what's actually broken and why:

### Issue 1: Sliders Are Native HTML, Not shadcn

**Root cause:** `rules.vue` line 218 uses `<input type="range">` — a native HTML element. All fixes to `Slider.vue` (the shadcn component) had zero effect because the rules page doesn't use it.

**Fix:** Replace `<input type="range">` with `<UiSlider>` in `rules.vue`, AND add proper CSS styling for the shadcn slider track/thumb in `main.css`.

### Issue 2: Score Bars Are Invisible

**Root cause:** The score bars in Live Preview use `bg-primary` as a Tailwind class, but the `--color-primary` CSS variable contains an oklch value. Tailwind v4 should resolve this, but the bar container is only `h-1.5` (6px) tall and `w-16` (64px) wide — very small. Combined with the dark background, they're nearly invisible.

**Fix:** Make score bars taller (h-2.5), wider (w-24), and use inline `style` with `backgroundColor` reading from the computed CSS variable for guaranteed rendering.

### Issue 3: No Visual Depth or Theme Color

**Root cause:** Cards use `border-border` which in dark mode is a very subtle gray. There's no primary color accent anywhere except the navbar. The design looks flat because there's no gradient, glow, or color variation.

**Fix:** Add primary-tinted card borders in dark mode, gradient section headers, primary-colored accents on interactive elements, and subtle background gradients.

### Issue 4: uPlot Chart Broken

**Root cause:** The uPlot implementation uses dynamic import but may have issues with the container sizing or the data format. Need to debug the actual error.

**Fix:** Debug and fix the uPlot rendering. Ensure the container has explicit dimensions before uPlot initializes.

### Issue 5: Sparkline Not Themed

**Root cause:** ApexCharts `colors` array receives `var(--color-primary)` as a string, but ApexCharts can't resolve CSS custom properties — it needs actual color values. The fix attempted to use `getComputedStyle` but it's inside a `computed()` which may not have access to `document` during SSR.

**Fix:** Use a reactive ref that reads the computed color on mount, and pass that to ApexCharts.

---

## Implementation Plan

### Step 1: Replace Native Sliders with UiSlider

**File:** `frontend/app/pages/rules.vue` lines 218-224

Replace:
```html
<input type="range" :value="..." min="0" max="10" class="..." @input="..." />
```

With:
```html
<UiSlider :model-value="[prefs[slider.key]]" :min="0" :max="10" :step="1"
  @update:model-value="(v) => { prefs[slider.key] = v[0] }" />
```

### Step 2: Style the Shadcn Slider

**File:** `frontend/app/assets/css/main.css`

Add slider thumb glow, track primary fill, and hover effects using `data-slot` selectors from the original visual polish plan.

### Step 3: Fix Score Bars

**File:** `frontend/app/pages/rules.vue` lines 398-404

Make the score bar container larger and use inline style with a helper function that resolves the primary color to a usable value.

### Step 4: Add Visual Depth to Cards

**File:** `frontend/app/assets/css/main.css`

Enhance the dark mode card styles with:
- Primary-tinted border glow (already partially there but too subtle)
- Increase border opacity from 0.12 to 0.2
- Add subtle primary gradient to card headers
- Add hover lift with primary glow intensification

### Step 5: Fix Sparkline Theme Colors

**File:** `frontend/app/pages/index.vue`

Create a composable or reactive ref that resolves CSS custom properties to actual color strings on mount, and use those in the ApexCharts config.

### Step 6: Fix uPlot Chart

**File:** `frontend/app/components/CapacityChart.vue` (uplot branch only)

Ensure the chart container has explicit height before uPlot initializes. Add error handling and fallback.

### Step 7: Add Theme Color Accents Throughout

**File:** `frontend/app/assets/css/main.css`

- Section headings with subtle primary gradient text
- Table header rows with primary-tinted background
- Active/selected states with primary ring
- Progress bars with primary gradient fill
- Badge/chip borders with primary tint
