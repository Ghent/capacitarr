# Nuxt UI v4 App Migration — Replace shadcn-vue

**Created:** 2026-03-03T05:39Z
**Updated:** 2026-03-03T22:15Z
**Status:** ❌ Won't Do
**Scope:** Migrate `capacitarr/frontend/` from shadcn-vue + @vueuse/motion to Nuxt UI v4

> **Decision (2026-03-03):** After a full codebase audit, this migration was rejected. The ROI is poor — ~50–80 hours of effort to swap a working, polished component library for marginal developer-experience gains. The primary blockers are: (1) the existing 6-theme oklch color system (1,076 lines of custom CSS) would need to be rebuilt or abandoned, since Nuxt UI v4's theming expects preset palettes; (2) all `data-slot`-based CSS styling would break; (3) the complex audit table with expandable row groups, custom sorting, and badge rendering would require a full TanStack ColumnDef rewrite; (4) ~50+ lucide-vue-next icon imports would need migration. The current shadcn-vue setup works well, the wrapper files are never manually edited, and the template verbosity is not a meaningful bottleneck at this project's scale (6 pages, ~12 components).

## Overview

Replace all shadcn-vue components in the Capacitarr frontend with Nuxt UI v4 equivalents. This unifies the component library across both the app and the project site, eliminates 60+ copied component files under `components/ui/`, and gains access to Nuxt UI v4's dashboard layout components, TanStack-powered tables, and built-in toast system.

## Why Nuxt UI v4 (not v3 / Pro)

In Nuxt UI v4 (`@nuxt/ui@^4.5`), all previously-Pro components are **included in the base package**. The separate `@nuxt/ui-pro` module is legacy v3 only.

- **No license required** — the `NUXT_UI_PRO_LICENSE` env var is a v3 concept
- **One package:** `@nuxt/ui` provides all 100+ components (base + dashboard + page layout)
- **TanStack Table:** `UTable` is powered by `@tanstack/vue-table` with full sorting, filtering, pagination
- **Modern stack:** Reka UI primitives, Tailwind CSS v4, Tailwind Variants, `motion-v` for animations

## Current State

### shadcn-vue Components in Use (287 instances across the app)

| shadcn-vue Component | Usage Count | Nuxt UI v4 Equivalent |
|---------------------|-------------|----------------------|
| `UiButton` | ~40 | `UButton` |
| `UiCard` / `UiCardHeader` / `UiCardContent` / `UiCardFooter` | ~50 | `UCard` |
| `UiSelect` / `UiSelectTrigger` / `UiSelectContent` / `UiSelectItem` | ~40 | `USelect` or `USelectMenu` |
| `UiInput` | ~15 | `UInput` |
| `UiLabel` | ~20 | `UFormField` (wraps label + input) |
| `UiDialog` / `UiDialogContent` / `UiDialogHeader` / `UiDialogFooter` | ~20 | `UModal` |
| `UiTable` / `UiTableHeader` / `UiTableBody` / `UiTableRow` / `UiTableCell` | ~30 | `UTable` (TanStack-powered) |
| `UiBadge` | ~15 | `UBadge` |
| `UiAlert` / `UiAlertTitle` / `UiAlertDescription` | ~8 | `UAlert` |
| `UiSwitch` | ~10 | `USwitch` |
| `UiSlider` | ~2 | `USlider` |
| `UiTabs` / `UiTabsList` / `UiTabsTrigger` / `UiTabsContent` | ~8 | `UTabs` |
| `UiPopover` / `UiPopoverTrigger` / `UiPopoverContent` | ~6 | `UPopover` |
| `UiDropdownMenu` / `UiDropdownMenuTrigger` / `UiDropdownMenuContent` / `UiDropdownMenuItem` | ~12 | `UDropdownMenu` |
| `UiTooltip` / `UiTooltipTrigger` / `UiTooltipContent` | ~4 | `UTooltip` |
| `UiCommand` / `UiCommandInput` / `UiCommandList` / `UiCommandItem` | ~5 | `UCommandPalette` |
| `UiSeparator` | ~4 | `USeparator` |
| `UiProgress` | ~1 | `UProgress` |
| `UiSkeleton` | ~2 | `USkeleton` |
| `UiCollapsible` | ~1 | `UCollapsible` |
| `UiScrollArea` | ~2 | `UScrollArea` |

### Other Libraries to Remove

| Library | Current Use | Nuxt UI v4 Replacement |
|---------|------------|----------------------|
| `@vueuse/motion` (`v-motion`) | Card entrance animations | `motion-v` (bundled with Nuxt UI v4) or CSS `@keyframes` |
| `reka-ui` | Primitive headless components (used by shadcn-vue internally) | Reka UI is already used by Nuxt UI v4 internally — no change |
| `@radix-icons/vue` | Icons | `@nuxt/icon` module with Iconify (bundled with Nuxt UI v4) |
| `tailwind-variants` / `class-variance-authority` | Component variant styling | `tailwind-variants` is used by Nuxt UI v4 internally — can keep or let Nuxt UI manage it |

### Files to Delete

All 60+ files under `frontend/app/components/ui/`:
- `alert/` (3 files)
- `badge/` (2 files)
- `button/` (2 files)
- `card/` (7 files)
- `collapsible/` (4 files)
- `command/` (10 files)
- `dialog/` (10 files)
- `dropdown-menu/` (14 files)
- `input/` (2 files)
- `label/` (2 files)
- `popover/` (5 files)
- `progress/` (2 files)
- `scroll-area/` (3 files)
- `select/` (12 files)
- `separator/` (2 files)
- `sheet/` (10 files)
- `skeleton/` (2 files)
- `slider/` (2 files)
- `sonner/` (2 files)
- `switch/` (2 files)
- `table/` (10 files)
- `tabs/` (5 files)
- `toggle/` (2 files)
- `toggle-group/` (3 files)
- `tooltip/` (5 files)

Also delete:
- `frontend/app/lib/utils.ts` (shadcn's `cn()` utility — Nuxt UI v4 uses Tailwind Variants internally)
- `frontend/components.json` (shadcn-vue config)

## Migration Approach

### API Differences

Nuxt UI v4 components are **simpler** than shadcn-vue's compound component pattern. For example:

**shadcn-vue (verbose compound components):**
```vue
<UiSelect v-model="value">
  <UiSelectTrigger>
    <UiSelectValue placeholder="Select..." />
  </UiSelectTrigger>
  <UiSelectContent>
    <UiSelectItem value="a">Option A</UiSelectItem>
    <UiSelectItem value="b">Option B</UiSelectItem>
  </UiSelectContent>
</UiSelect>
```

**Nuxt UI v4 (single component with props):**
```vue
<USelectMenu
  v-model="value"
  :items="[
    { label: 'Option A', value: 'a' },
    { label: 'Option B', value: 'b' },
  ]"
  placeholder="Select..."
/>
```

**shadcn-vue Dialog:**
```vue
<UiDialog :open="show" @update:open="show = $event">
  <UiDialogContent>
    <UiDialogHeader>
      <UiDialogTitle>Title</UiDialogTitle>
      <UiDialogDescription>Description</UiDialogDescription>
    </UiDialogHeader>
    <p>Content</p>
    <UiDialogFooter>
      <UiButton @click="show = false">Close</UiButton>
    </UiDialogFooter>
  </UiDialogContent>
</UiDialog>
```

**Nuxt UI v4 Modal:**
```vue
<UModal v-model:open="show" title="Title" description="Description">
  <p>Content</p>
  <template #footer>
    <UButton @click="show = false">Close</UButton>
  </template>
</UModal>
```

**shadcn-vue Table (manual rows/cells):**
```vue
<UiTable>
  <UiTableHeader>
    <UiTableRow>
      <UiTableHead>Name</UiTableHead>
      <UiTableHead>Score</UiTableHead>
    </UiTableRow>
  </UiTableHeader>
  <UiTableBody>
    <UiTableRow v-for="item in items">
      <UiTableCell>{{ item.name }}</UiTableCell>
      <UiTableCell>{{ item.score }}</UiTableCell>
    </UiTableRow>
  </UiTableBody>
</UiTable>
```

**Nuxt UI v4 Table (TanStack-powered with ColumnDef):**
```vue
<script setup lang="ts">
import type { ColumnDef } from '@tanstack/vue-table'

const columns: ColumnDef<Item>[] = [
  { accessorKey: 'name', header: 'Name' },
  { accessorKey: 'score', header: 'Score' },
]
</script>

<template>
  <UTable :columns="columns" :data="items" />
</template>
```

The v4 `UTable` uses TanStack Table's `ColumnDef` API, which supports:
- `accessorKey` / `accessorFn` for data access
- `header` (string or render function) for column headers
- `cell` render function for custom cell rendering
- Built-in sorting, filtering, pagination, row selection, column pinning
- Row expansion via TanStack's `ExpandedState`

This means the migration **reduces** template verbosity, but requires restructuring data into TanStack Table format (column definitions + data arrays).

**shadcn-vue Toast (custom composable):**
```vue
<!-- Using custom ToastContainer.vue + useToast composable -->
```

**Nuxt UI v4 Toast:**
```vue
<script setup lang="ts">
const toast = useToast()

function showSuccess() {
  toast.add({
    title: 'Success',
    description: 'Action completed',
    color: 'success',
  })
}
</script>

<!-- UToaster is global — add once in app.vue -->
<template>
  <UToaster />
</template>
```

## Phase 1: Infrastructure

### Step 1.1: Create Branch

```bash
git checkout main && git pull
git checkout -b feature/nuxt-ui-migration
```

### Step 1.2: Install Nuxt UI v4

```bash
cd frontend
pnpm remove @radix-icons/vue class-variance-authority @vueuse/motion
pnpm add @nuxt/ui tailwindcss
```

> **Note:** `@nuxt/ui` v4 includes `reka-ui`, `tailwind-variants`, `@nuxt/icon`, `@nuxtjs/color-mode`, `@nuxt/fonts`, `@tailwindcss/vite`, `@tanstack/vue-table`, `@tanstack/vue-virtual`, `motion-v`, and `fuse.js` as dependencies. Do not install them separately.

### Step 1.3: Update nuxt.config.ts

Replace shadcn-related config with Nuxt UI v4 module:

```ts
export default defineNuxtConfig({
  modules: ['@nuxt/ui', '@nuxtjs/i18n', /* other existing modules */],
  css: ['~/assets/css/main.css'],
  // ... rest of config
})
```

> **No `extends`** — Nuxt UI v4 is a module, not a layer. Do NOT use `extends: ['@nuxt/ui-pro']`.

### Step 1.4: Update CSS

The existing `main.css` needs to be updated to use Nuxt UI v4's CSS imports. Add at the top:

```css
@import "tailwindcss";
@import "@nuxt/ui";
```

Remove shadcn-specific CSS variables (`--radius`, `--background`, `--foreground`, `--card`, `--primary`, etc.) and replace with Nuxt UI v4's theme system. The oklch design tokens can be mapped to Tailwind CSS v4's `@theme` directive.

### Step 1.5: Theme Configuration

Update `app.config.ts` with the violet dark theme mapped to Nuxt UI v4's color system:

```ts
export default defineAppConfig({
  ui: {
    colors: {
      primary: 'violet',
      neutral: 'zinc',
    },
    // Component-level customization can be done here
    // See: https://ui.nuxt.com/docs/getting-started/theme
  },
})
```

### Step 1.6: Add UToaster to app.vue

Add the global toast container to the app root:

```vue
<!-- app.vue -->
<template>
  <NuxtPage />
  <UToaster />
</template>
```

## Phase 2: Component Migration (by page)

Migrate one page at a time, testing after each:

### Step 2.1: Login Page (`pages/login.vue`)

**Components to replace:**
- `UiCard` → `UCard`
- `UiCardHeader` / `UiCardTitle` / `UiCardDescription` → `UCard` with `#header` slot
- `UiCardContent` → `UCard` default slot
- `UiLabel` → `UFormField`
- `UiInput` → `UInput`
- `UiButton` → `UButton`

This is the simplest page — good starting point for establishing the migration pattern.

### Step 2.2: Navbar (`components/Navbar.vue`)

**Components to replace:**
- `UiPopover` → `UPopover`
- `UiButton` → `UButton`
- `UiDropdownMenu` → `UDropdownMenu`
- `UiBadge` → `UBadge`

### Step 2.3: Dashboard (`pages/index.vue`)

**Components to replace:**
- `UiCard` / `UiCardContent` → `UCard`
- `UiSelect` (3 instances) → `USelectMenu`
- `UiButton` → `UButton`
- `UiBadge` → `UBadge`
- `v-motion` directives → `motion-v` or CSS `@keyframes` (see Phase 4)

This is the largest page — 80+ component instances.

### Step 2.4: Rules Page (`pages/rules.vue`)

**Components to replace:**
- `UiCard` / `UiCardHeader` / `UiCardContent` → `UCard`
- `UiSelect` (many instances) → `USelectMenu`
- `UiSlider` → `USlider`
- `UiSwitch` → `USwitch`
- `UiTable` (preview table) → `UTable` (TanStack-powered)
- `UiTooltip` → `UTooltip`
- `UiInput` → `UInput`
- `UiSeparator` → `USeparator`
- `UiButton` → `UButton`

### Step 2.5: Audit Page (`pages/audit.vue`)

**Components to replace:**
- `UiTable` (full audit table) → `UTable` (TanStack-powered)
- `UiInput` → `UInput`
- `UiButton` → `UButton`
- `UiBadge` → `UBadge`
- `UiCard` → `UCard`

### Step 2.6: Settings Page (`pages/settings.vue`)

**Components to replace:**
- `UiTabs` → `UTabs`
- `UiCard` (many instances) → `UCard`
- `UiSelect` (many instances) → `USelectMenu`
- `UiDialog` (3 modals) → `UModal`
- `UiAlert` → `UAlert`
- `UiSwitch` → `USwitch`
- `UiInput` → `UInput`
- `UiLabel` → `UFormField`
- `UiButton` → `UButton`
- `UiSeparator` → `USeparator`

This is the most complex page with the most diverse component usage.

### Step 2.7: Help Page (`pages/help.vue`)

**Components to replace:**
- `UiBadge` → `UBadge`

Minimal changes needed.

### Step 2.8: Shared Components

- `ScoreDetailModal.vue` — `UiDialog` → `UModal`, `UiBadge` → `UBadge`
- `EngineControlPopover.vue` — `UiPopover` → `UPopover`, `UiDialog` → `UModal`, `UiButton` → `UButton`, `UiBadge` → `UBadge`
- `RuleBuilder.vue` — `UiSelect` → `USelectMenu`, `UiSwitch` → `USwitch`, `UiInput` → `UInput`, `UiCommand` → `UCommandPalette`, `UiPopover` → `UPopover`, `UiButton` → `UButton`
- `DiskGroupSection.vue` — `UiCard` → `UCard`
- `ToastContainer.vue` — Delete and replace with global `UToaster` + `useToast()` composable

## Phase 3: Cleanup

### Step 3.1: Delete shadcn-vue Components

```bash
rm -rf frontend/app/components/ui/
rm frontend/app/lib/utils.ts
rm frontend/components.json
```

### Step 3.2: Remove Unused Dependencies

```bash
cd frontend
pnpm remove reka-ui class-variance-authority @vueuse/motion @radix-icons/vue
```

> **Note:** `tailwind-variants` is kept as a dependency of `@nuxt/ui` v4 — do not remove it.

### Step 3.3: Update Imports

Remove all dead imports referencing `@/components/ui/`, `@/lib/utils`, or removed packages.

### Step 3.4: Update CSS

Remove shadcn-specific CSS variables and utility classes from `main.css`. Keep the oklch design tokens and theme-specific styles. Ensure the file starts with:

```css
@import "tailwindcss";
@import "@nuxt/ui";
```

## Phase 4: Animation Migration

Nuxt UI v4 includes `motion-v` as a dependency. This provides a Vue animation API similar to `@vueuse/motion`. Two options:

### Option A: Use motion-v (bundled with Nuxt UI v4)

`motion-v` is already installed. Use its directive or component API:

```vue
<script setup lang="ts">
import { Motion } from 'motion-v'
</script>

<template>
  <Motion
    :initial="{ opacity: 0, y: 20 }"
    :animate="{ opacity: 1, y: 0 }"
    :transition="{ delay: 0.1 }"
  >
    <UCard>...</UCard>
  </Motion>
</template>
```

### Option B: CSS @keyframes (simpler, no JS dependency)

```css
.card-enter {
  animation: fadeInUp 0.4s ease both;
  animation-delay: var(--delay, 0ms);
}
@keyframes fadeInUp {
  from { opacity: 0; transform: translateY(12px); }
  to { opacity: 1; transform: translateY(0); }
}
```

```vue
<UCard class="card-enter" :style="{ '--delay': `${idx * 100}ms` }">
```

Option B is recommended for simplicity. Option A provides more control and matches the current `v-motion` behavior more closely.

## Phase 5: Testing

- All pages render correctly with Nuxt UI v4 components
- Dark/light mode toggle works (via `UColorModeButton` or existing toggle)
- Theme colors match the violet dark aesthetic
- Mobile responsive layout works
- All form interactions (select, input, switch, slider) work
- Modals open/close correctly
- TanStack-powered tables sort, filter, and paginate correctly
- Toast notifications work via `useToast()` + `UToaster`
- I18n translations still display correctly
- No console errors or warnings
- Build produces correct Docker image output

## Risk Considerations

1. **TanStack Table API in UTable** — Nuxt UI v4's `UTable` uses `@tanstack/vue-table` with `ColumnDef<T>[]` column definitions. The shadcn-vue tables use manual `<TableRow>` / `<TableCell>` with inline rendering. The audit and rules preview tables have complex rendering (expandable rows, color-coded badges, score breakdowns) that will need TanStack `cell` render functions or scoped slots. This is the most complex migration item. The upside is that TanStack Table provides built-in sorting, filtering, pagination, and row selection that the current manual implementation lacks.

2. **Command/Combobox in RuleBuilder** — The current implementation uses shadcn-vue's Command + Popover combo for the combobox. Nuxt UI v4's `UCommandPalette` has a different API. Alternatively, `USelectMenu` with `searchable` prop or `UInputMenu` may be more appropriate replacements.

3. **Select component API change** — shadcn-vue uses inline `<SelectItem>` children. Nuxt UI v4 uses an `items` array prop. Every select usage needs to extract the options into a data array. This is the highest-volume change (~40 instances).

4. **Motion animations** — The `v-motion` fade-in-up animations on cards are used extensively on the dashboard, rules, and settings pages. Nuxt UI v4 bundles `motion-v` which provides a similar (but not identical) API. CSS `@keyframes` is the simpler path. Either approach needs testing for timing/feel parity.

5. **Toast system** — The current `ToastContainer.vue` + `useToast` composable would be replaced by Nuxt UI v4's `UToaster` + `useToast()`. The composable API is similar but the options shape differs — each call site that triggers a toast needs updating.

6. **CSS variable names** — shadcn-vue uses CSS variables like `--background`, `--foreground`, `--card`, `--primary`, `--radius`, etc. Nuxt UI v4 uses a different variable naming scheme integrated with Tailwind CSS v4's `@theme` system. The `main.css` file will need significant cleanup to remove shadcn variables and map oklch tokens to the new system.

## Migration Reference

| shadcn-vue | Nuxt UI v4 | Notes |
|-----------|-----------|-------|
| `UiButton` | `UButton` | Props: `variant`, `color`, `size`, `icon`, `loading` |
| `UiCard` + compounds | `UCard` | Single component with `#header`, `#footer` slots |
| `UiSelect` + compounds | `USelectMenu` | Data-driven: `:items="[...]"` prop |
| `UiInput` | `UInput` | Similar API |
| `UiLabel` | `UFormField` | Wraps label + input + error |
| `UiDialog` + compounds | `UModal` | `v-model:open`, `title`, `description` props |
| `UiTable` + compounds | `UTable` | TanStack-powered: `:columns` + `:data` |
| `UiBadge` | `UBadge` | Similar API |
| `UiAlert` + compounds | `UAlert` | Single component with props |
| `UiSwitch` | `USwitch` | Similar API |
| `UiSlider` | `USlider` | Similar API |
| `UiTabs` + compounds | `UTabs` | Data-driven: `:items="[...]"` with `#content` slot |
| `UiPopover` + compounds | `UPopover` | `#trigger` + `#content` slots |
| `UiDropdownMenu` + compounds | `UDropdownMenu` | Data-driven: `:items="[...]"` |
| `UiTooltip` + compounds | `UTooltip` | `text` prop or `#content` slot |
| `UiCommand` + compounds | `UCommandPalette` | Data-driven: `:groups="[...]"` |
| `UiSeparator` | `USeparator` | Similar API |
| `UiProgress` | `UProgress` | Similar API |
| `UiSkeleton` | `USkeleton` | Similar API |
| `UiCollapsible` | `UCollapsible` | Similar API (v4 has this built-in) |
| `UiScrollArea` | `UScrollArea` | Similar API (v4 has this built-in) |
| `ToastContainer.vue` | `UToaster` + `useToast()` | Global toaster + composable |
| `@radix-icons/vue` icons | `UIcon` with Iconify | `icon="i-lucide-*"` or `icon="i-heroicons-*"` |
| `class-variance-authority` | Tailwind Variants (internal) | Nuxt UI handles variants — use `app.config.ts` |
| `@vueuse/motion` | `motion-v` (bundled) or CSS | `motion-v` already in node_modules |

## Notes

- Nuxt UI v4 does **not** require a license — all components are included in `@nuxt/ui@^4.5`
- The `@nuxt/ui-pro` package is legacy (v3 only) and should NOT be installed
- This plan can be executed independently from the site plan (`20260303T0539Z-nuxt-ui-pro-project-site.md`)
- The official Nuxt UI v4 docs are at [ui.nuxt.com](https://ui.nuxt.com)
