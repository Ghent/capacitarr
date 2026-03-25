# Documentation Site Enhancements

**Created:** 2026-03-09T00:26Z
**Status:** ✅ Complete

## Problems

1. **Mermaid diagrams do not render on the published site** — they display as raw code blocks. The site has no Mermaid dependency or rendering component.
2. **Diagrams use Mermaid's default theme** — no brand alignment with the violet design system, poor dark mode contrast.
3. **Layout issues in complex diagrams** — the default `dagre` engine produces crossing arrows and cramped spacing, especially on the 13-node service layer diagram.
4. **No documentation search** — the site has no search functionality. Users must browse or use browser find.
5. **Screenshots need refresh** — updated screenshots are available in `capacitarr/screenshots/` including a new Custom Rules screenshot. The gallery component lacks consistent aspect-ratio handling, causing layout shifts between images.

## Solution

Three independent workstreams, all targeting the documentation site:

- **Mermaid:** Client-side rendering via a custom `ProseCode.vue` component, with dark/light themed palettes and the ELK layout engine.
- **Search:** Enable `UContentSearch` from `@nuxt/ui` for full-text documentation search with `Cmd+K` / `Ctrl+K` shortcut.
- **Screenshots:** Deploy refreshed screenshots, add a new Custom Rules entry to the gallery, fix CSS for consistent sizing.

## Scope

- 6 diagrams across 2 published docs files (`architecture.md`, `scoring.md`)
- 8 screenshots in `capacitarr/screenshots/` → `site/public/screenshots/`
- Plan files are **excluded** from diagram refactoring (per user request)

---

## Part 1: Mermaid Rendering

### Step 1: Install Mermaid dependencies

Add `mermaid` and `@mermaid-js/layout-elk` to `site/package.json`:

```bash
cd site && pnpm add mermaid @mermaid-js/layout-elk
```

### Step 2: Create `ProseCode.vue` component

Create `site/app/components/content/ProseCode.vue` that:

- Intercepts code blocks where `language === 'mermaid'`
- For mermaid blocks: renders the diagram client-side using `mermaid.render()`
- For all other languages: delegates to the default Nuxt Content code block rendering
- Wraps mermaid rendering in `<ClientOnly>` for SSG compatibility
- Watches `useColorMode()` and re-initializes Mermaid with the appropriate theme on toggle

### Step 3: Define theme palettes

Define two theme objects in a shared composable or inline in the component:

**Dark theme — Violet Depths:**
- Node backgrounds: `#1e1b4b` (indigo-950)
- Node borders: `#8b5cf6` (violet-500)
- Text: `#e9d5ff` (violet-100)
- Edges: `#a78bfa` (violet-400)
- Subgraph backgrounds: `#110f24`
- Subgraph borders: `#4c1d95` (violet-800)

**Light theme — Violet Mist:**
- Node backgrounds: `#ede9fe` (violet-100)
- Node borders: `#8b5cf6` (violet-500)
- Text: `#1e1b4b` (indigo-950)
- Edges: `#6d28d9` (violet-700)
- Subgraph backgrounds: `#f5f3ff` (violet-50)
- Subgraph borders: `#c4b5fd` (violet-300)

### Step 4: Configure Mermaid for readability

Apply these settings in `mermaid.initialize()`:

- **Layout engine:** ELK (`defaultRenderer: 'elk'`) — reduces crossing arrows
- **Node spacing:** 60px (default 50)
- **Rank spacing:** 70px (default 50)
- **Node padding:** 20px (default 15)
- **Font:** Geist Sans (matching the site)
- **Curve style:** `basis` (smooth)
- **Rounded corners:** via `themeCSS` injection (`rx: 8; ry: 8` on nodes, `rx: 12; ry: 12` on subgraphs)
- **Edge label font size:** 12px

### Step 5: Add Mermaid container CSS to main.css

Add styles to `site/app/assets/css/main.css` for the Mermaid container wrapper:

- Light background + border in light mode
- Dark background + border in dark mode
- Rounded corners, center alignment, padding
- Responsive max-width

### Step 6: Refactor `architecture.md` diagrams — 4 diagrams

**Diagram 1 — High-Level Overview, line 8:**
- Add `direction LR` inside the container subgraph so internal components flow horizontally
- External subgraphs for arr apps, media servers, enrichment stay in TD below
- Use dashed edges `-.->` for async/event flows vs solid for data flows

**Diagram 2 — Service Layer, line 70:**
- Group the 13 services into domain subgraphs:
  - **Core:** ApprovalService, DeletionService, EngineService, SettingsService
  - **Data:** AuditLogService, DataService, MetricsService, RulesService
  - **External:** IntegrationService, AuthService, NotificationChannelService, NotificationDispatchService, VersionService
- Each domain subgraph uses `direction LR`
- Overall flow remains TD from HTTP layer through services to events to data

**Diagram 3 — Notification Dispatch, line 172:**
- Already clean with 5 nodes, LR. Apply theme only.
- Use dashed edges for the gate arrows to convey waits-for semantics

**Diagram 4 — SSE Frontend, line 289:**
- Already clean with 6 nodes, LR. Apply theme only.

### Step 7: Refactor `scoring.md` diagrams — 2 diagrams

**Diagram 5 — Scoring Overview, line 9:**
- Simplify the `RULE_CHECK` to `ABS_KEEP` decision chain into a single annotated decision node
- Use thick edges `==>` for the primary scoring path
- Use dashed edges for the rule-override path

**Diagram 6 — End-to-End Flow, line 215:**
- Differentiate the three mode paths with edge styles:
  - `==>` for Auto — critical/active path
  - `-->` for Approval — standard
  - `-.->` for Dry Run — passive/no-action

### Step 8: Add edge style legend pattern

Add a small legend box to the architecture overview diagram explaining the edge convention:

- Solid = data flow
- Dashed = event/async
- Thick = critical path

### Step 9: Verify Mermaid rendering

- Build the site locally and verify all 6 diagrams render in both light and dark mode
- Verify the same Markdown source still renders correctly on GitLab's Markdown preview
- Check that non-mermaid code blocks are unaffected

---

## Part 2: Screenshot Refresh

### Step 10: Copy, rename, and convert screenshots to lossless WebP

Copy the updated screenshots from `capacitarr/screenshots/` to `site/public/screenshots/`, renaming to web-friendly kebab-case names and converting to lossless WebP using Node.js `sharp`:

| Source file | Destination |
|------------|-------------|
| `01_dashboard_20260307.png` | `dashboard.webp` |
| `02_weights_20260307.png` | `weights.webp` |
| `03_custom_rules_20260307.png` | `custom-rules.webp` |
| `04_deletion_priority_20260307.png` | `deletion-priority.webp` |
| `05_audit_log_20260307.png` | `audit-log.webp` |
| `06_scorecard_keep_20260307.png` | `scorecard-keep.webp` |
| `07_scorecard_rules_20260307.png` | `scorecard-rules.webp` |
| `08_settings_20260307.png` | `settings.webp` |

**Conversion method:** Use `sharp` with `lossless: true` for zero quality loss:

```bash
cd site && node -e "
const sharp = require('sharp');
const fs = require('fs');
const mapping = {
  '01_dashboard_20260307': 'dashboard',
  '02_weights_20260307': 'weights',
  '03_custom_rules_20260307': 'custom-rules',
  '04_deletion_priority_20260307': 'deletion-priority',
  '05_audit_log_20260307': 'audit-log',
  '06_scorecard_keep_20260307': 'scorecard-keep',
  '07_scorecard_rules_20260307': 'scorecard-rules',
  '08_settings_20260307': 'settings',
};
for (const [src, dest] of Object.entries(mapping)) {
  sharp('../screenshots/' + src + '.png')
    .webp({ lossless: true })
    .toFile('public/screenshots/' + dest + '.webp')
    .then(() => console.log(dest + '.webp done'));
}
"
```

Remove old `.png` files from `site/public/screenshots/` after conversion is verified.

### Step 11: Add Custom Rules entry to ScreenshotGallery

Update `site/app/components/ScreenshotGallery.vue`:

1. Change all `.png` references to `.webp` in the `screenshots` array
2. Add the new Custom Rules screenshot entry, inserted between Weights and Deletion Priority:

```typescript
{
  src: `${base}/screenshots/custom-rules.webp`,
  title: 'Custom Rules',
  description: 'Build sophisticated cascading rules with conditions, weights, and overrides',
}
```

### Step 12: Fix screenshot gallery CSS for consistent sizing

Update `site/app/components/ScreenshotGallery.vue` styles:

1. **Add `aspect-ratio: 16/10`** to `.screenshot-image-wrapper` — prevents layout shifts between images by reserving a consistent frame
2. **Add `object-fit: cover`** to `.screenshot-image` — ensures images fill the frame consistently even if source dimensions vary slightly
3. **Add explicit `width` and `height` attributes** on `<img>` tags to prevent CLS — use `width="2880" height="1800"` matching the 16:10 capture size

---

## Part 3: Documentation Search

### Step 13: Enable UContentSearch in Nuxt config

Update `site/nuxt.config.ts` to enable content search:

```typescript
content: {
  build: {
    markdown: {
      toc: { searchDepth: 1 },
    },
  },
  // Enable full-text search index generation
  search: {
    enabled: true,
  },
},
```

### Step 14: Add search button to site header

Add `UContentSearchButton` to the site's header configuration. This is typically done via `app.config.ts` header config or a layout slot. The button:

- Renders a search icon/bar in the navigation header
- Opens the search dialog on click
- Responds to `Cmd+K` / `Ctrl+K` keyboard shortcut automatically
- Searches across all content pages with fuzzy matching

### Step 15: Verify search functionality

- Build the site and verify the search index is generated
- Test search across documentation pages
- Verify keyboard shortcut works
- Confirm search results link to the correct pages

---

## Final Verification

### Step 16: Full site build and review

- Run `nuxt generate` and verify the complete site builds without errors
- Test all three features: Mermaid diagrams, screenshots, and search
- Verify in both light and dark mode
- Check mobile responsiveness

---

## Files Changed

| File | Action |
|------|--------|
| `site/package.json` | Add `mermaid`, `@mermaid-js/layout-elk` |
| `site/nuxt.config.ts` | Enable `content.search` |
| `site/app/components/content/ProseCode.vue` | **New** — Mermaid rendering component |
| `site/app/components/ScreenshotGallery.vue` | Add Custom Rules entry, fix aspect-ratio/object-fit CSS |
| `site/app/assets/css/main.css` | Add Mermaid container styles |
| `site/public/screenshots/*.png` | Replace with refreshed screenshots, add `custom-rules.png` |
| `docs/architecture.md` | Refactor 4 diagrams |
| `docs/scoring.md` | Refactor 2 diagrams |
| `site/app.config.ts` or layout | Add `UContentSearchButton` to header |
