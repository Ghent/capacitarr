# Nuxt UI Pro App Migration — Replace shadcn-vue

**Created:** 2026-03-03T05:39Z
**Scope:** Migrate `capacitarr/frontend/` from shadcn-vue + @vueuse/motion to Nuxt UI (Pro)
**Prerequisite:** Nuxt UI Pro license (same license as the site plan)

## Overview

Replace all shadcn-vue components in the Capacitarr frontend with Nuxt UI equivalents. This unifies the component library across both the app and the project site, eliminates 60+ copied component files under `components/ui/`, and gains access to Nuxt UI Pro's dashboard layout components.

## Current State

### shadcn-vue Components in Use (287 instances across the app)

| shadcn-vue Component | Usage Count | Nuxt UI Equivalent |
|---------------------|-------------|-------------------|
| `UiButton` | ~40 | `UButton` |
| `UiCard` / `UiCardHeader` / `UiCardContent` / `UiCardFooter` | ~50 | `UCard` |
| `UiSelect` / `UiSelectTrigger` / `UiSelectContent` / `UiSelectItem` | ~40 | `USelect` or `USelectMenu` |
| `UiInput` | ~15 | `UInput` |
| `UiLabel` | ~20 | `UFormField` (wraps label + input) |
| `UiDialog` / `UiDialogContent` / `UiDialogHeader` / `UiDialogFooter` | ~20 | `UModal` |
| `UiTable` / `UiTableHeader` / `UiTableBody` / `UiTableRow` / `UiTableCell` | ~30 | `UTable` |
| `UiBadge` | ~15 | `UBadge` |
| `UiAlert` / `UiAlertTitle` / `UiAlertDescription` | ~8 | `UAlert` |
| `UiSwitch` | ~10 | `USwitch` |
| `UiSlider` | ~2 | `USlider` |
| `UiTabs` / `UiTabsList` / `UiTabsTrigger` / `UiTabsContent` | ~8 | `UTabs` |
| `UiPopover` / `UiPopoverTrigger` / `UiPopoverContent` | ~6 | `UPopover` |
| `UiDropdownMenu` / `UiDropdownMenuTrigger` / `UiDropdownMenuContent` / `UiDropdownMenuItem` | ~12 | `UDropdownMenu` |
| `UiTooltip` / `UiTooltipTrigger` / `UiTooltipContent` | ~4 | `UTooltip` |
| `UiCommand` / `UiCommandInput` / `UiCommandList` / `UiCommandItem` | ~5 | `UCommandPalette` |
| `UiSeparator` | ~4 | `UDivider` |
| `UiProgress` | ~1 | `UProgress` |
| `UiSkeleton` | ~2 | `USkeleton` |
| `UiCollapsible` | ~1 | native `<details>` or custom |
| `UiScrollArea` | ~2 | native CSS `overflow-y: auto` |

### Other Libraries to Remove

| Library | Current Use | Nuxt UI Replacement |
|---------|------------|-------------------|
| `@vueuse/motion` (`v-motion`) | Card entrance animations | Nuxt UI's built-in transitions or CSS `@keyframes` |
| `reka-ui` | Primitive headless components (used by shadcn-vue internally) | Reka UI is already used by Nuxt UI internally — no change |
| `@radix-icons/vue` | Icons | `@nuxt/icon` module with Iconify (bundled with Nuxt UI) |
| `tailwind-variants` / `class-variance-authority` | Component variant styling | Nuxt UI handles variants internally via `app.config.ts` |

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
- `frontend/app/lib/utils.ts` (shadcn's `cn()` utility — Nuxt UI doesn't need it)
- `frontend/components.json` (shadcn-vue config)

## Migration Approach

### API Differences

Nuxt UI components are **simpler** than shadcn-vue's compound component pattern. For example:

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

**Nuxt UI (single component with props):**
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

**Nuxt UI Modal:**
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

**Nuxt UI Table (data-driven):**
```vue
<UTable
  :columns="[
    { key: 'name', label: 'Name' },
    { key: 'score', label: 'Score' },
  ]"
  :rows="items"
/>
```

This means the migration generally **reduces** template complexity, but requires restructuring data into the format Nuxt UI expects (arrays of objects for selects/tables instead of inline `<Item>` elements).

## Phase 1: Infrastructure

### Step 1.1: Create Branch

```bash
git checkout main && git pull
git checkout -b feature/nuxt-ui-migration
```

### Step 1.2: Install Nuxt UI Pro

```bash
cd frontend
pnpm remove @radix-icons/vue class-variance-authority tailwind-variants @vueuse/motion
pnpm add @nuxt/ui-pro
```

### Step 1.3: Update nuxt.config.ts

Replace shadcn-related config with Nuxt UI module:

```ts
export default defineNuxtConfig({
  extends: ['@nuxt/ui-pro'],
  modules: ['@nuxt/ui', '@nuxtjs/i18n', /* other existing modules */],
  // ... rest of config
})
```

### Step 1.4: Theme Configuration

Create `app.config.ts` with the violet dark theme mapped to Nuxt UI's color system:

```ts
export default defineAppConfig({
  ui: {
    colors: {
      primary: 'violet',
      neutral: 'zinc',
    },
    // Component-level customization for the violet dark aesthetic
  },
})
```

### Step 1.5: Update CSS

The existing `main.css` with oklch tokens can largely stay. Remove shadcn-specific CSS variables and replace with Nuxt UI's variable names.

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
- `v-motion` directives → CSS `@keyframes` or `Transition` component

This is the largest page — 80+ component instances.

### Step 2.4: Rules Page (`pages/rules.vue`)

**Components to replace:**
- `UiCard` / `UiCardHeader` / `UiCardContent` → `UCard`
- `UiSelect` (many instances) → `USelectMenu`
- `UiSlider` → `USlider`
- `UiSwitch` → `USwitch`
- `UiTable` (preview table) → `UTable`
- `UiTooltip` → `UTooltip`
- `UiInput` → `UInput`
- `UiSeparator` → `UDivider`
- `UiButton` → `UButton`

### Step 2.5: Audit Page (`pages/audit.vue`)

**Components to replace:**
- `UiTable` (full audit table) → `UTable`
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
- `UiSeparator` → `UDivider`

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
- `ToastContainer.vue` — Replace with `UNotification` toast system

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
pnpm remove reka-ui class-variance-authority tailwind-variants @vueuse/motion @radix-icons/vue
```

### Step 3.3: Update Imports

Remove all dead imports referencing `@/components/ui/`, `@/lib/utils`, or removed packages.

### Step 3.4: Update CSS

Remove shadcn-specific CSS variables and utility classes from `main.css`. Keep the oklch design tokens and theme-specific styles.

## Phase 4: Animation Migration

Replace `@vueuse/motion` (`v-motion`) directives with CSS or Vue transitions:

**Current pattern (v-motion):**
```vue
<UiCard
  v-motion
  :initial="{ opacity: 0, y: 20 }"
  :enter="{ opacity: 1, y: 0, transition: { delay: 100 } }"
>
```

**Replacement (CSS):**
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

Or use Vue's `<Transition>` component for route-level transitions.

## Phase 5: Testing

- All pages render correctly with Nuxt UI components
- Dark/light mode toggle works (if re-enabled)
- Theme colors match the violet dark aesthetic
- Mobile responsive layout works
- All form interactions (select, input, switch, slider) work
- Modals open/close correctly
- Tables sort and filter correctly
- Toast notifications work
- I18n translations still display correctly
- No console errors or warnings
- Build produces correct output

## Risk Considerations

1. **Nuxt UI Table API differs significantly** — the shadcn-vue tables use manual `<TableRow>` / `<TableCell>` with custom rendering. Nuxt UI's `UTable` is data-driven with column slots. The audit and rules preview tables have complex rendering (expandable rows, color-coded badges, score breakdowns) that will need custom column slots.

2. **Command/Combobox in RuleBuilder** — The current implementation uses shadcn-vue's Command + Popover combo for the combobox. Nuxt UI's `UCommandPalette` has a different API — may need `USelectMenu` with `searchable` prop instead.

3. **Select component API change** — shadcn-vue uses inline `<SelectItem>` children. Nuxt UI uses an `items` array prop. Every select usage needs to extract the options into a data array. This is the highest-volume change.

4. **Motion animations** — The `v-motion` fade-in-up animations on cards are used extensively on the dashboard, rules, and settings pages. CSS replacement is straightforward but needs testing for timing/feel parity.

5. **Custom toast system** — The current `ToastContainer.vue` + `useToast` composable would be replaced by Nuxt UI's built-in notification system. Toast triggering throughout the app would need updating.
