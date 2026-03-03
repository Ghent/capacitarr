# Nuxt UI Pro Project Site for Capacitarr

**Created:** 2026-03-03T05:39Z
**Scope:** Replace Astro + Starlight site (`capacitarr/site/`) with Nuxt UI Pro
**Deploys to:** GitLab Pages via CI pipeline
**Prerequisite:** Nuxt UI Pro license ($249/dev)

## Overview

Replace the current Astro + Starlight project site with a Nuxt UI Pro site. This eliminates the Tailwind/Starlight CSS conflict that prevents the sidebar from rendering, and provides a premium docs + landing page experience using the same stack as the Capacitarr app (Nuxt + Tailwind CSS v4).

## Why Replace Astro + Starlight

1. **CSS conflict** — `@tailwindcss/vite` and Starlight's `@layer` system are incompatible, breaking the sidebar
2. **Stack mismatch** — Astro is a different framework from the Nuxt app, requiring separate tooling and knowledge
3. **Limited customization** — Starlight's docs pages resist visual customization due to its opinionated CSS pipeline
4. **Nuxt UI Pro is purpose-built** — provides both landing page templates AND docs layout with sidebar, search, TOC

## Technology Stack

| Component | Technology |
|-----------|-----------|
| Framework | Nuxt 3 |
| UI Library | Nuxt UI Pro |
| Styling | Tailwind CSS v4 + oklch design tokens |
| Docs | Nuxt Content v3 (markdown rendering) |
| Search | Nuxt UI Pro command palette (local) |
| Fonts | Geist Sans + Geist Mono via @fontsource |
| CI | GitLab CI `pages` job |
| Hosting | GitLab Pages |

## Phase 1: Project Setup

### Step 1.1: Create Branch

```bash
cd capacitarr
git checkout main && git pull
git checkout -b feature/nuxt-ui-pro-site
```

### Step 1.2: Remove Astro + Starlight

```bash
rm -rf site/
```

### Step 1.3: Initialize Nuxt UI Pro Site

```bash
npx nuxi init site -t ui-pro
cd site
pnpm install
pnpm add @fontsource/geist-sans @fontsource/geist-mono
```

### Step 1.4: Project Structure

```
capacitarr/site/
├── nuxt.config.ts           # Nuxt config with UI Pro module
├── app.config.ts            # UI theme configuration
├── package.json
├── pnpm-lock.yaml
├── content/
│   └── docs/                # CI copies from ../docs/
│       └── .gitkeep
├── pages/
│   ├── index.vue            # Custom landing page
│   └── docs/
│       └── [...slug].vue    # Docs catch-all route
├── components/
│   ├── Hero.vue             # Landing page hero section
│   ├── FeatureGrid.vue      # Feature cards
│   ├── Integrations.vue     # Integration logos
│   ├── HowItWorks.vue       # Three-step visual
│   ├── QuickStart.vue       # Docker Compose block
│   └── AppFooter.vue        # Site footer
├── layouts/
│   ├── default.vue          # Docs layout with sidebar
│   └── landing.vue          # Full-width landing layout
├── assets/
│   └── css/
│       └── main.css         # Violet dark theme tokens
├── public/
│   ├── favicon.ico
│   └── screenshots/
└── tsconfig.json
```

## Phase 2: Theme Configuration

### Step 2.1: Nuxt Config

```ts
// nuxt.config.ts
export default defineNuxtConfig({
  extends: ['@nuxt/ui-pro'],
  modules: ['@nuxt/content', '@nuxt/ui'],
  ui: {
    // Force dark mode
    colorMode: false,
  },
  content: {
    sources: {
      docs: {
        driver: 'fs',
        base: './content/docs',
      },
    },
  },
  app: {
    baseURL: '/software/capacitarr/',
  },
  css: ['~/assets/css/main.css'],
})
```

### Step 2.2: Theme Tokens

Transfer the oklch violet dark theme from the existing site's `theme.css` into `assets/css/main.css`. Nuxt UI uses CSS variables for theming — map the Capacitarr tokens to Nuxt UI's variable names.

### Step 2.3: Geist Fonts

The site uses the same Geist Sans and Geist Mono typefaces as the Capacitarr app (matching `frontend/app/assets/css/main.css`).

**Import in `assets/css/main.css`:**

```css
/* Geist Sans — body text (matches the Capacitarr app) */
@import '@fontsource/geist-sans/400.css';
@import '@fontsource/geist-sans/500.css';
@import '@fontsource/geist-sans/600.css';
@import '@fontsource/geist-sans/700.css';

/* Geist Mono — code blocks, monospace text */
@import '@fontsource/geist-mono/400.css';
```

**Configure Nuxt UI to use Geist in `app.config.ts`:**

```ts
export default defineAppConfig({
  ui: {
    fonts: {
      sans: "'Geist Sans', 'Geist', ui-sans-serif, system-ui, sans-serif",
      mono: "'Geist Mono', ui-monospace, SFMono-Regular, monospace",
    },
  },
})
```

**Or set via CSS variables in `main.css`:**

```css
body {
  font-family: 'Geist Sans', 'Geist', ui-sans-serif, system-ui, sans-serif;
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}

code, kbd, pre, .font-mono {
  font-family: 'Geist Mono', ui-monospace, SFMono-Regular, monospace;
}
```

The exact Nuxt UI integration method depends on the version — check Nuxt UI's theming docs for the preferred approach. Both CSS and `app.config.ts` methods work.

## Phase 3: Landing Page

Recreate the landing page from the Astro site using Nuxt UI Pro components:

- **Hero** — Use `ULandingHero` with gradient text, version badge, CTA buttons, screenshot
- **Feature Grid** — Use `ULandingGrid` + `ULandingCard` with glass-morphism styling
- **Integrations** — Custom component with grayscale-to-color hover logos
- **How It Works** — Use `ULandingSection` with three-step layout
- **Quick Start** — Code block with Docker Compose snippet and copy button
- **Footer** — Use `UFooter` with navigation links

The visual effects (gradient text, glass-morphism, glow) are pure CSS and transfer directly from the Astro implementation.

## Phase 4: Documentation

### Step 4.1: Docs Layout

Use Nuxt UI Pro's `DocsLayout` component which provides:
- Left sidebar with collapsible navigation groups
- Right-side table of contents (On this page)
- Search via command palette
- Breadcrumbs
- Prev/next navigation
- Mobile responsive hamburger menu

### Step 4.2: Content Sync

Same strategy as before — `docs/*.md` stays as source of truth, CI copies into `content/docs/` at build time. The `sync-docs` script in `package.json` handles local dev.

### Step 4.3: Markdown Compatibility

Nuxt Content supports standard markdown plus MDC (Markdown Components). The existing docs should work with minimal changes. The `:::caution` Starlight syntax would need to become Nuxt Content's callout syntax.

### Step 4.4: Frontmatter

Nuxt Content uses YAML frontmatter (same as Starlight). The existing frontmatter from Phase 4 of the original plan works as-is.

## Phase 5: CI/CD

### Step 5.1: Update Pages Job

Replace the Astro build with a Nuxt generate:

```yaml
pages:
  stage: pages
  image: node:22-alpine
  before_script:
    - corepack enable
    - cd site && pnpm install --frozen-lockfile
  script:
    # Sync docs (same file-by-file copy as before)
    - mkdir -p content/docs/api
    - cp ../docs/index.md content/docs/
    - cp ../docs/deployment.md content/docs/
    - cp ../docs/configuration.md content/docs/
    - cp ../docs/scoring.md content/docs/
    - cp ../docs/releasing.md content/docs/
    - cp ../docs/api/README.md content/docs/api/index.md
    - cp ../docs/api/examples.md content/docs/api/
    - cp ../docs/api/workflows.md content/docs/api/
    - cp ../docs/api/versioning.md content/docs/api/
    # Inject changelog
    - |
      echo '---' > content/docs/changelog.md
      echo 'title: Changelog' >> content/docs/changelog.md
      echo '---' >> content/docs/changelog.md
      echo '' >> content/docs/changelog.md
      cat ../CHANGELOG.md >> content/docs/changelog.md
    # Copy assets
    - mkdir -p public/screenshots
    - cp -r ../screenshots/* public/screenshots/ 2>/dev/null || true
    - cp ../frontend/public/favicon.ico public/favicon.ico 2>/dev/null || true
    # Build
    - pnpm generate
    - mv .output/public/ ../public/
  artifacts:
    paths:
      - public
  rules:
    - if: $CI_COMMIT_BRANCH == "main"
```

## Phase 6: Testing

- Landing page renders with all visual effects
- Docs sidebar is visible and navigable  
- Search (command palette) works
- All doc pages render correctly
- Mobile responsive layout works
- Dark theme is consistent
- Build produces correct static output for GitLab Pages

## Notes

- The Nuxt UI Pro license must be activated before development begins
- The `site/` directory remains independent from the app's `frontend/` directory
- This plan can be executed independently from Plan 2 (app migration)
