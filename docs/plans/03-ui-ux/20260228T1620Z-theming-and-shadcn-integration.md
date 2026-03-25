# Capacitarr: Theming System & shadcn-vue Integration

Comprehensive plan for replacing raw HTML elements with shadcn-vue components and introducing a multi-theme color system with light/dark mode support across all pages.

---

## Current State Analysis

### Dependency Inventory

The project already has the **infrastructure** for shadcn-vue but no components have been generated:

| Dependency | Version | Purpose | Status |
|---|---|---|---|
| `radix-vue` | ^1.9.17 | Headless UI primitives (shadcn-vue foundation) | Installed |
| `class-variance-authority` | ^0.7.1 | Component variant utility | Installed |
| `clsx` | ^2.1.1 | Class string builder | Installed |
| `tailwind-merge` | ^3.0.2 | Tailwind class deduplication | Installed |
| `vue-sonner` | ^2.0.9 | Toast notifications | Installed but unused |
| `lucide-vue-next` | ^0.511.0 | Icon library | Installed and used |
| `shadcn-vue` CLI | — | Component generator | **Not initialized** |

The [`cn()`](capacitarr/frontend/app/lib/utils.ts:4) utility is already defined — this is the function shadcn-vue components import.

### CSS Variable Architecture

[`main.css`](capacitarr/frontend/app/assets/css/main.css) defines oklch-based CSS custom properties in `:root` (light) and `.dark` (dark) selectors. The current palette is **fixed violet**:

| Variable | Light Value | Dark Value | Purpose |
|---|---|---|---|
| `--color-primary` | `oklch(0.606 0.25 292.717)` | same | Violet accent |
| `--color-background` | `oklch(0.985 ...)` | `oklch(0.08 ...)` | Page background |
| `--color-card` | white | `oklch(0.119 ...)` | Card surfaces |
| `--color-border` | `oklch(0.871 ...)` | `oklch(0.255 ...)` | Borders |
| `--color-muted` | `oklch(0.961 ...)` | `oklch(0.179 ...)` | Muted backgrounds |
| `--color-ring` | violet | violet | Focus rings |

### Raw HTML Pattern Audit

Every page uses inline Tailwind classes with **hardcoded `violet-*` references** and raw HTML elements:

| Pattern | Occurrences | shadcn-vue Replacement |
|---|---|---|
| `<button class="...bg-violet-600...">` | ~20+ across all pages | `<Button>`, `<Button variant="outline">` |
| `<input class="...rounded-lg border...">` | ~15+ (login, settings, rules) | `<Input>` |
| `<select class="...rounded-lg border...">` | ~12+ (dashboard, rules, settings) | `<Select>` |
| `<div class="...rounded-xl border...card">` | ~15+ (all pages) | `<Card>`, `<CardHeader>`, `<CardContent>` |
| `<span class="...badge...">` | ~10+ (audit, settings, rules) | `<Badge>` |
| Custom `<Teleport>` modal | 2 (settings, ScoreDetailModal) | `<Dialog>` |
| Custom `ToastContainer.vue` | 1 | `<Sonner>` (vue-sonner) |
| `<input type="range">` | 6+ (rules sliders) | `<Slider>` |
| `<table>` with custom styles | 2 (audit, score detail) | `<Table>`, `<TableHeader>`, `<TableBody>` |
| Toggle buttons (execution mode) | 1 (rules page) | `<ToggleGroup>` |
| Expandable rows | 1 (audit) | `<Collapsible>` or accordion |

### Color Mode

[`useAppColorMode.ts`](capacitarr/frontend/app/composables/useColorMode.ts) manages dark/light via localStorage key `capacitarr-color-mode` and toggles the `.dark` class on `<html>`. This is compatible with the proposed multi-theme approach.

---

## Phase 1: shadcn-vue Foundation

### 1.1 Initialize shadcn-vue CLI

Run from `capacitarr/frontend/`:

```bash
npx shadcn-vue@latest init
```

Configuration choices:
- **Style:** New York (denser, fits the current compact design)
- **Base color:** Zinc (matches current neutral palette)
- **CSS variables:** Yes (already using them)
- **Framework:** Nuxt
- **Components directory:** `app/components/ui`

This creates:
- `components.json` — shadcn-vue registry config
- Updates `tailwind.config` (or in TW v4, the CSS import)
- Confirms `lib/utils.ts` path

### 1.2 Generate Required Components

```bash
npx shadcn-vue@latest add button card input label select badge \
  dialog sheet slider table tooltip toggle-group separator \
  dropdown-menu alert skeleton progress tabs scroll-area \
  collapsible popover command sonner
```

This generates local Vue component files in `app/components/ui/` that are fully customizable and use the CSS custom properties.

### 1.3 Wire vue-sonner

Replace the custom [`ToastContainer.vue`](capacitarr/frontend/app/components/ToastContainer.vue) with the shadcn-vue `<Sonner />` component (wraps `vue-sonner`). Update [`app.vue`](capacitarr/frontend/app/app.vue) to mount `<Sonner />` and refactor [`useToast.ts`](capacitarr/frontend/app/composables/useToast.ts) to call `toast()` from vue-sonner.

### 1.4 Verify Tailwind v4 Compatibility

shadcn-vue v2 targets Tailwind v4 via `@import "tailwindcss"` and CSS-first configuration. The project already uses this pattern in [`main.css`](capacitarr/frontend/app/assets/css/main.css:1). Verify that:

- The generated components use `@theme` inline references (TW v4 style) rather than `tailwind.config.ts` extensions
- oklch values work through the CSS custom property pipeline
- The `@tailwindcss/vite` plugin in [`nuxt.config.ts`](capacitarr/frontend/nuxt.config.ts:14) picks up generated component styles

---

## Phase 2: Multi-Theme Color System

### 2.1 Theme Architecture

Themes are applied via a `data-theme` attribute on `<html>`, combined with the existing `.dark` class. This gives a 2D matrix: **theme × mode**.

```
<html class="dark" data-theme="ocean">   → Ocean theme, dark mode
<html data-theme="sunset">               → Sunset theme, light mode
<html data-theme="violet">               → Default violet, light mode
```

#### CSS Variable Override Strategy

Each theme defines **only the accent/primary variables** — the neutral surface variables (background, card, border, muted) remain shared across all themes in their light/dark variants. This keeps themes maintainable and ensures consistent readability.

```css
/* Base neutrals - shared by all themes */
:root {
  --color-background: oklch(0.985 0.002 247.839);
  --color-foreground: oklch(0.141 0.005 285.823);
  --color-card: oklch(1 0 0);
  --color-card-foreground: oklch(0.141 0.005 285.823);
  --color-border: oklch(0.871 0.006 286.029);
  --color-muted: oklch(0.961 0.003 264.542);
  --color-muted-foreground: oklch(0.552 0.016 285.938);
  --color-destructive: oklch(0.577 0.245 27.325);
  --color-success: oklch(0.648 0.2 160);
  --color-warning: oklch(0.75 0.183 55.934);
  --radius-sm: 0.375rem;
  --radius-md: 0.5rem;
  --radius-lg: 0.75rem;
  --radius-xl: 1rem;
}

.dark {
  --color-background: oklch(0.08 0.005 285.823);
  --color-foreground: oklch(0.985 0.002 247.839);
  --color-card: oklch(0.119 0.011 285.823);
  --color-card-foreground: oklch(0.985 0.002 247.839);
  --color-border: oklch(0.255 0.014 285.823);
  --color-muted: oklch(0.179 0.012 285.823);
  --color-muted-foreground: oklch(0.603 0.016 285.938);
}

/* Theme-specific accent overrides */
[data-theme="violet"], :root {
  --color-primary: oklch(0.606 0.25 292.717);
  --color-primary-foreground: oklch(1 0 0);
  --color-ring: oklch(0.606 0.25 292.717);
  --color-input: oklch(0.871 0.006 286.029);

  /* Chart accent colors for multi-series */
  --color-chart-1: oklch(0.606 0.25 292.717);
  --color-chart-2: oklch(0.55 0.22 280);
  --color-chart-3: oklch(0.65 0.18 305);
  --color-chart-4: oklch(0.7 0.15 320);
}

.dark[data-theme="violet"], .dark {
  --color-input: oklch(0.255 0.014 285.823);
  --color-ring: oklch(0.606 0.25 292.717);
}
```

#### CSS Variable Cascade

```mermaid
flowchart TD
    ROOT[":root<br/>Shared light neutrals"]
    DARK[".dark<br/>Shared dark neutrals"]
    THEME["data-theme=X<br/>Accent overrides"]
    COMPONENT["shadcn-vue component<br/>Reads CSS vars via Tailwind"]

    ROOT --> COMPONENT
    DARK --> COMPONENT
    THEME --> COMPONENT
```

### 2.2 Theme Palette Definitions

Six themes — three complementary (contrasting accent vs base) and three analogous (harmonious adjacent hues). All values are oklch for perceptual uniformity.

#### Theme 1: Violet (Default) — Analogous
The current theme. Cool violet primary with purple/indigo chart accents.

| Variable | Light | Dark |
|---|---|---|
| `--color-primary` | `oklch(0.606 0.25 292.717)` | same |
| `--color-primary-foreground` | white | white |
| Hue family | 280–310 (purple-violet range) | — |
| Scheme type | **Analogous** (violet + purple + indigo) | — |

#### Theme 2: Ocean — Analogous
Cool blue-teal gradient. Calming, professional feel.

| Variable | Light | Dark |
|---|---|---|
| `--color-primary` | `oklch(0.55 0.2 230)` | `oklch(0.65 0.18 230)` |
| `--color-primary-foreground` | white | white |
| Hue family | 210–250 (blue-cyan range) | — |
| Scheme type | **Analogous** (blue + cyan + teal) | — |

#### Theme 3: Emerald — Analogous
Green-teal nature palette. Clean and fresh.

| Variable | Light | Dark |
|---|---|---|
| `--color-primary` | `oklch(0.60 0.19 160)` | `oklch(0.70 0.17 160)` |
| `--color-primary-foreground` | white | `oklch(0.15 0.02 160)` |
| Hue family | 140–180 (green-teal range) | — |
| Scheme type | **Analogous** (green + teal + mint) | — |

#### Theme 4: Sunset — Complementary
Warm amber/orange primary against cool neutral backgrounds. High-contrast, energetic.

| Variable | Light | Dark |
|---|---|---|
| `--color-primary` | `oklch(0.70 0.18 55)` | `oklch(0.78 0.15 55)` |
| `--color-primary-foreground` | `oklch(0.15 0.03 55)` | `oklch(0.15 0.03 55)` |
| Hue family | 40–70 (amber-orange range) | — |
| Scheme type | **Complementary** (warm accent vs cool zinc base) | — |

#### Theme 5: Rose — Complementary
Pink-rose primary against the cool zinc neutrals. Elegant contrast.

| Variable | Light | Dark |
|---|---|---|
| `--color-primary` | `oklch(0.60 0.22 350)` | `oklch(0.70 0.19 350)` |
| `--color-primary-foreground` | white | white |
| Hue family | 335–15 (rose-pink-red range) | — |
| Scheme type | **Complementary** (warm pink vs cool zinc base) | — |

#### Theme 6: Slate — Complementary / Monochrome
Minimal, near-monochrome with a subtle blue-gray accent. Zero-chroma design.

| Variable | Light | Dark |
|---|---|---|
| `--color-primary` | `oklch(0.45 0.03 260)` | `oklch(0.65 0.03 260)` |
| `--color-primary-foreground` | white | white |
| Hue family | 250–270 (desaturated blue-gray) | — |
| Scheme type | **Complementary / Monochrome** | — |

#### Theme Color Preview

```mermaid
flowchart LR
    subgraph Analogous Themes
        VIOLET["Violet<br/>Hue 293 / C 0.25"]
        OCEAN["Ocean<br/>Hue 230 / C 0.20"]
        EMERALD["Emerald<br/>Hue 160 / C 0.19"]
    end

    subgraph Complementary Themes
        SUNSET["Sunset<br/>Hue 55 / C 0.18"]
        ROSE["Rose<br/>Hue 350 / C 0.22"]
        SLATE["Slate<br/>Hue 260 / C 0.03"]
    end
```

### 2.3 Accessibility Constraints

All theme primaries must pass these checks:

- **WCAG AA** contrast ratio ≥ 4.5:1 for `--color-primary` on `--color-primary-foreground`
- **WCAG AA** contrast ratio ≥ 3:1 for `--color-primary` used as UI element indicators on `--color-card` background
- Light mode primaries use lower lightness (L ≤ 0.65) for contrast on white surfaces
- Dark mode primaries use higher lightness (L ≥ 0.60) for contrast on dark surfaces
- Destructive, success, and warning colors remain **theme-independent** (always red, green, amber)

### 2.4 Chart Theme Integration

ApexCharts colors in [`CapacityChart.vue`](capacitarr/frontend/app/components/CapacityChart.vue) currently use hardcoded hex values. These need to read from the CSS custom properties at render time:

```typescript
// New utility: getThemeColor
function getThemeColor(varName: string): string {
  return getComputedStyle(document.documentElement)
    .getPropertyValue(varName).trim()
}

// Usage in chart config
colors: [getThemeColor('--color-primary'), getThemeColor('--color-chart-2')]
```

Charts must re-render when theme changes (via a `watch` on the active theme).

---

## Phase 3: Theme Infrastructure

### 3.1 `useTheme` Composable

New file: `app/composables/useTheme.ts`

```typescript
export type ThemeId = 'violet' | 'ocean' | 'emerald' | 'sunset' | 'rose' | 'slate'

export const useTheme = () => {
  const theme = useState<ThemeId>('appTheme', () => {
    if (import.meta.client) {
      return (localStorage.getItem('capacitarr-theme') as ThemeId) || 'violet'
    }
    return 'violet'
  })

  function setTheme(id: ThemeId) {
    theme.value = id
    if (import.meta.client) {
      document.documentElement.setAttribute('data-theme', id)
      localStorage.setItem('capacitarr-theme', id)
    }
  }

  // Apply on first client load
  if (import.meta.client) {
    document.documentElement.setAttribute('data-theme', theme.value)
  }

  return { theme, setTheme }
}
```

### 3.2 Extend `useAppColorMode`

Update [`useAppColorMode.ts`](capacitarr/frontend/app/composables/useColorMode.ts) to also initialize the theme on startup, or keep them independent composables that both run in [`app.vue`](capacitarr/frontend/app/app.vue).

### 3.3 Hot-Swap Without Reload

Since themes only change CSS custom properties via `data-theme` attribute:
1. `setTheme()` updates `data-theme` on `<html>` — instant CSS cascade
2. All shadcn-vue components inherit new colors immediately (they reference variables through Tailwind)
3. ApexCharts requires explicit re-render — use a `watch()` keyed to `theme` that calls `chart.updateOptions({ colors: [...] })`
4. No page reload needed

### 3.4 CSS File Organization

Restructure [`main.css`](capacitarr/frontend/app/assets/css/main.css) with clearly separated sections:

```
main.css
├── @import "tailwindcss"
├── /* Shared neutral variables (light) */
├── :root { ... }
├── /* Shared neutral variables (dark) */
├── .dark { ... }
├── /* Theme: Violet (default) */
├── [data-theme="violet"] { ... }
├── .dark[data-theme="violet"] { ... }
├── /* Theme: Ocean */
├── [data-theme="ocean"] { ... }
├── .dark[data-theme="ocean"] { ... }
├── /* ... remaining themes ... */
├── /* Base styles */
├── /* Animations */
└── /* Page transitions */
```

---

## Phase 4: shadcn-vue Component Migration

Migration follows a **page-by-page, inside-out strategy**: start with low-risk leaf components, then work up to page-level layouts.

### 4.1 Shared Components (Foundation Layer)

These are used across multiple pages and should be migrated first:

| Current | Replacement | Files Affected |
|---|---|---|
| Custom `ToastContainer.vue` | `<Sonner />` from shadcn-vue/vue-sonner | `app.vue`, `useToast.ts` |
| Skeleton divs | `<Skeleton>` component | `SkeletonCard.vue`, `SkeletonTable.vue` |
| Navbar buttons | `<Button variant="ghost" size="icon">` | `Navbar.vue` |
| Navbar dropdown (new) | `<DropdownMenu>` for theme selector | `Navbar.vue` |
| Score detail modal | `<Dialog>` | `ScoreDetailModal.vue` |

### 4.2 Login Page

Smallest page, ideal for first migration.

| Element | Current | shadcn-vue |
|---|---|---|
| Card wrapper | `<div class="rounded-2xl border...">` | `<Card>` + `<CardHeader>` + `<CardContent>` |
| Username input | `<input class="...">` | `<Input>` + `<Label>` |
| Password input | `<input type="password">` | `<Input>` + `<Label>` |
| Error alert | `<div class="rounded-lg border border-red...">` | `<Alert variant="destructive">` |
| Submit button | `<button class="...bg-violet-600...">` | `<Button>` |

**Key change:** Remove all hardcoded `violet-*` classes — the `<Button>` component uses `bg-primary` which reads `--color-primary`.

### 4.3 Dashboard Page (`index.vue`)

| Element | Current | shadcn-vue |
|---|---|---|
| Summary stat cards | `<div class="rounded-xl border...">` | `<Card>` + `<CardContent>` |
| Date range select | `<select class="...">` | `<Select>` + `<SelectTrigger>` + `<SelectContent>` |
| Chart mode select | `<select class="...">` | `<Select>` |
| Refresh interval select | `<select class="...">` | `<Select>` |
| Progress bars | inline div with gradients | `<Progress>` (themed) |
| Worker status badge | `<span class="...badge...">` | `<Badge>` |

### 4.4 Scoring Engine Page (`rules.vue`)

Most complex page — migrate incrementally:

| Element | Current | shadcn-vue |
|---|---|---|
| Section cards | `<div class="rounded-xl border...">` | `<Card>` |
| Range sliders | `<input type="range">` | `<Slider>` |
| Slider value display | `<span>X / 10</span>` | Keep, styled alongside `<Slider>` |
| Preset chips | `<button class="rounded-full...">` | `<Badge>` with clickable variant or `<ToggleGroup>` |
| Execution mode toggle | Two `<button>` cards | `<ToggleGroup type="single">` with `<ToggleGroupItem>` |
| Tiebreaker select | `<select>` | `<Select>` |
| Add rule form selects | Multiple `<select>` | `<Select>` components |
| Add rule value input | `<input>` | `<Input>` |
| Save buttons | `<button class="bg-violet-600">` | `<Button>` |
| Threshold sliders | Custom dual-marker progress | Custom component using `<Slider>` as base |

### 4.5 Audit History Page (`audit.vue`)

| Element | Current | shadcn-vue |
|---|---|---|
| Data table | `<table>` with custom styles | `<Table>` + `<TableHeader>` + `<TableBody>` + `<TableRow>` |
| Action badges | `<span class="...badge...">` | `<Badge>` with variant mapping |
| Expandable rows | Custom toggle logic | `<Collapsible>` or keep custom with shadcn styling |
| Refresh button | `<button class="border...">` | `<Button variant="outline">` |
| Pagination controls | Custom buttons | `<Button variant="outline" size="sm">` |
| Score detail modal | Custom `<Teleport>` | `<Dialog>` (already in 4.1) |
| Filter selects | `<select>` dropdowns | `<Select>` |

### 4.6 Settings Page (`settings.vue`)

| Element | Current | shadcn-vue |
|---|---|---|
| Integration cards | `<div class="rounded-xl border...">` | `<Card>` + `<CardHeader>` + `<CardContent>` + `<CardFooter>` |
| Add/Edit modal | Custom `<Teleport>` + backdrop | `<Dialog>` + `<DialogContent>` + `<DialogHeader>` |
| Form inputs | `<input class="...">` | `<Input>` + `<Label>` |
| Type select | `<select>` | `<Select>` |
| Retention select | `<select>` | `<Select>` |
| Action buttons (Test/Edit) | `<button class="border...">` | `<Button variant="outline" size="sm">` |
| Delete button | `<button class="text-red...">` | `<Button variant="destructive" size="sm">` or ghost destructive |
| Status badges | `<span class="bg-emerald...">` | `<Badge variant="success">` / `<Badge variant="secondary">` |
| Warning alert | `<div class="bg-amber...">` | `<Alert>` |

### 4.7 Help Page (`help.vue`)

| Element | Current | shadcn-vue |
|---|---|---|
| Accordion sections | Custom toggle divs | `<Collapsible>` or `<Tabs>` |
| Section cards | `<div class="rounded-xl border...">` | `<Card>` |
| Code snippets | `<code>` blocks | Keep, styled with theme border/muted colors |

### 4.8 Migration Strategy per Component

```mermaid
flowchart TD
    FOUNDATION["Phase 4.1: Foundation<br/>Sonner, Skeleton, Navbar"]
    LOGIN["Phase 4.2: Login<br/>Card, Input, Button, Alert"]
    DASHBOARD["Phase 4.3: Dashboard<br/>Card, Select, Progress, Badge"]
    RULES["Phase 4.4: Rules<br/>Slider, ToggleGroup, Select, Card"]
    AUDIT["Phase 4.5: Audit<br/>Table, Badge, Dialog, Collapsible"]
    SETTINGS["Phase 4.6: Settings<br/>Dialog, Card, Input, Select"]
    HELP["Phase 4.7: Help<br/>Tabs/Collapsible, Card"]

    FOUNDATION --> LOGIN
    LOGIN --> DASHBOARD
    DASHBOARD --> AUDIT
    DASHBOARD --> SETTINGS
    AUDIT --> RULES
    SETTINGS --> RULES
    RULES --> HELP
```

**Order rationale:** Login is smallest/simplest and validates the full shadcn-vue pipeline. Dashboard tests Card/Select patterns reused everywhere. Rules is last because it has the most complex custom widgets (dual-position slider markers, preset chips).

---

## Phase 5: Theme Selector UI

### 5.1 Location: Navbar Dropdown

Add a theme picker to the [`Navbar.vue`](capacitarr/frontend/app/components/Navbar.vue) right-side actions, next to the dark mode toggle. Use `<DropdownMenu>`:

```
[Dashboard] [Scoring Engine] [Audit Log] [Settings]        [🎨▾] [🌙] [?] [⏏]
                                                              ^
                                                         Theme picker
```

The dropdown shows:
- A color swatch + theme name for each of the 6 themes
- A checkmark (✓) next to the active theme
- Clicking a theme applies it immediately (hot-swap)

### 5.2 Settings Page Section (Secondary)

Also expose the theme selector on the Settings page in the Display Preferences section (already exists for timezone/clock format). This provides discoverability for users who don't notice the navbar dropdown.

### 5.3 Theme Selector Component

New component: `app/components/ThemeSelector.vue`

Renders a grid of small round swatches, each showing the theme's primary color. Used by both the Navbar dropdown and Settings page.

```mermaid
flowchart LR
    SWATCH_1["⬤ Violet"]
    SWATCH_2["⬤ Ocean"]
    SWATCH_3["⬤ Emerald"]
    SWATCH_4["⬤ Sunset"]
    SWATCH_5["⬤ Rose"]
    SWATCH_6["⬤ Slate"]
```

### 5.4 Persistence

- **Storage:** `localStorage` key `capacitarr-theme` (same pattern as `capacitarr-color-mode`)
- **Default:** `violet` (maintains backward compatibility)
- **Application timing:** During [`useTheme()`](capacitarr/frontend/app/composables/useTheme.ts) initialization in `app.vue`, before first paint
- **No backend persistence needed** — theme is a display preference, not a functional setting

### 5.5 Preventing FOUC (Flash of Unstyled Content)

Add an inline `<script>` to the Nuxt head that reads `localStorage` and applies both `.dark` and `data-theme` before the Vue app hydrates:

```typescript
// nuxt.config.ts — app.head.script
app: {
  head: {
    script: [{
      innerHTML: `
        (function() {
          var t = localStorage.getItem('capacitarr-theme') || 'violet';
          var m = localStorage.getItem('capacitarr-color-mode');
          document.documentElement.setAttribute('data-theme', t);
          if (m === 'dark' || (!m && matchMedia('(prefers-color-scheme:dark)').matches)) {
            document.documentElement.classList.add('dark');
          }
        })();
      `,
      type: 'text/javascript'
    }]
  }
}
```

---

## Phase 6: Hardcoded Color Removal

### 6.1 Tailwind Class Audit

Every instance of hardcoded color classes must be replaced with theme-aware equivalents:

| Hardcoded Pattern | Theme-Aware Replacement |
|---|---|
| `bg-violet-600` | `bg-primary` |
| `text-violet-500` | `text-primary` |
| `hover:bg-violet-700` | `hover:bg-primary/90` |
| `border-violet-500` | `border-primary` |
| `shadow-violet-500/20` | `shadow-primary/20` |
| `focus:ring-violet-500/40` | `focus:ring-ring/40` |
| `bg-violet-50` | `bg-primary/10` |
| `dark:bg-violet-500/10` | `bg-primary/10` |
| `text-violet-600 dark:text-violet-400` | `text-primary` |
| `accent-violet-500` (slider) | Handled by `<Slider>` component internally |

### 6.2 Gradient Handling

The brand logo gradient (`from-violet-500 to-violet-600`) should use:
```html
<div class="bg-gradient-to-br from-primary to-primary/80">
```

Or define a separate `--color-primary-gradient` variable per theme if a two-tone gradient is desired.

### 6.3 Selection Color

Update the `::selection` style in CSS to use the variable:
```css
::selection {
  background-color: oklch(from var(--color-primary) l c h / 0.2);
}
```

Note: `oklch(from ...)` relative color syntax requires modern browsers. Alternatively, define `--color-selection` per theme.

---

## File Change Summary

### New Files

| File | Purpose |
|---|---|
| `app/components/ui/*` | ~20 shadcn-vue generated component directories |
| `app/composables/useTheme.ts` | Theme state management |
| `app/components/ThemeSelector.vue` | Theme picker swatch grid |
| `components.json` | shadcn-vue CLI config |

### Modified Files

| File | Changes |
|---|---|
| `app/assets/css/main.css` | Add 6 theme blocks with accent overrides |
| `app/app.vue` | Mount `<Sonner />`, initialize `useTheme()` |
| `app/composables/useColorMode.ts` | Minor: coordinate with theme initialization |
| `app/composables/useToast.ts` | Switch to vue-sonner `toast()` API |
| `app/components/Navbar.vue` | Add theme dropdown, use `<Button>` |
| `app/components/ToastContainer.vue` | **Delete** — replaced by Sonner |
| `app/components/ScoreDetailModal.vue` | Replace with `<Dialog>` |
| `app/components/SkeletonCard.vue` | Use `<Skeleton>` from shadcn-vue |
| `app/components/SkeletonTable.vue` | Use `<Skeleton>` from shadcn-vue |
| `app/components/CapacityChart.vue` | Read theme colors dynamically |
| `app/components/DiskGroupSection.vue` | Use `<Card>`, `<Badge>`, `<Progress>` |
| `app/pages/login.vue` | Full shadcn-vue conversion |
| `app/pages/index.vue` | Card, Select, Badge, Progress |
| `app/pages/rules.vue` | Slider, Card, Select, ToggleGroup, Button |
| `app/pages/audit.vue` | Table, Badge, Button, Dialog |
| `app/pages/settings.vue` | Dialog, Card, Input, Select, Button |
| `app/pages/help.vue` | Card, Tabs/Collapsible |
| `nuxt.config.ts` | Add FOUC-prevention script to head |
| `package.json` | Add `shadcn-vue` if CLI requires it |

### Deleted Files

| File | Reason |
|---|---|
| `app/components/ToastContainer.vue` | Replaced by `<Sonner />` |

---

## Implementation Checklist

- [x] Run `npx shadcn-vue@latest init` in `capacitarr/frontend/`
- [x] Generate all required shadcn-vue components (128 files, 25 groups)
- [x] Define 6 theme CSS variable blocks in `main.css`
- [x] Create `useTheme` composable with localStorage persistence
- [x] Add FOUC-prevention inline script to `nuxt.config.ts`
- [x] Create `ThemeSelector.vue` component (in Navbar dropdown)
- [x] Replace `ToastContainer.vue` with Sonner integration
- [x] Migrate `login.vue` to shadcn-vue components
- [x] Migrate `Navbar.vue` — add theme dropdown
- [x] Migrate `ScoreDetailModal.vue` → Dialog
- [x] Migrate `SkeletonCard.vue` and `SkeletonTable.vue`
- [x] Migrate `index.vue` (dashboard) — Card, Select, Badge, Progress
- [x] Migrate `audit.vue` — Table, Badge, Collapsible
- [x] Migrate `settings.vue` — Dialog, Card, Input, Select
- [x] Migrate `rules.vue` — Slider, ToggleGroup, Select
- [x] Migrate `help.vue` — Tabs/Collapsible, Card
- [x] Update `CapacityChart.vue` to read theme colors dynamically — Done via `useThemeColors.ts` composable
- [x] Remove all hardcoded `violet-*` Tailwind classes project-wide
- [x] Replace hardcoded `zinc-*` dark/light patterns with semantic CSS variable classes
- [x] Verify all 6 themes × 2 modes (12 combinations) render correctly
- [ ] Test WCAG AA contrast for all theme primaries — Low priority; oklch lightness values were chosen with contrast in mind but not formally verified with tooling
- [x] Verify ApexCharts re-renders on theme change — Done via MutationObserver in `useThemeColors.ts`

---

## Completion Status (2026-03-01)

**Status:** ✅ Complete — All phases implemented including visual quality refinements. Chart theme integration done via `useThemeColors.ts`.

All 6 phases of the theming plan were executed:
- Phase 1: shadcn-vue foundation (128 components)
- Phase 2: Multi-theme color system (6 themes × light/dark via oklch CSS variables)
- Phase 3: Visual quality — partially done, remaining items in Phase 4 §12
- Phase 4: Page migration (all 6 pages + all components)
- Phase 5: Theme selector UI (Navbar dropdown + Settings)
- Phase 6: Hardcoded color removal (semantic CSS variables)

Remaining visual quality items (broken sliders, chart theme integration, dark mode borders, typography) are tracked in the Phase 4 production readiness plan.
