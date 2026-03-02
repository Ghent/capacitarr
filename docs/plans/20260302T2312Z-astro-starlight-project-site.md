# Astro + Starlight Project Site for Capacitarr

**Created:** 2026-03-02T23:12Z
**Scope:** Single repo (`capacitarr/site/`)
**Deploys to:** GitLab Pages via existing CI pipeline

## Overview

Create a professional project website for Capacitarr using Astro + Starlight, hosted on GitLab Pages. The site consists of two parts:

1. **Custom landing page** (`/`) — A flashy, dark-themed marketing page showcasing features, integrations, and quick start
2. **Documentation section** (`/docs/`) — Starlight-powered docs, auto-synced from `capacitarr/docs/` in CI

The site uses the same violet dark theme as the Capacitarr app, built with Tailwind CSS v4 and the oklch color tokens from `frontend/app/assets/css/main.css`.

## Technology Choices

| Component | Technology | Why |
|-----------|-----------|-----|
| Framework | [Astro](https://astro.build/) v5+ | Static-first, zero JS by default, component system, Vite-based |
| Docs | [Starlight](https://starlight.astro.build/) | Astro's official docs plugin — search, sidebar, dark mode, i18n |
| Styling | Tailwind CSS v4 | Same as the app; shares oklch design tokens |
| Diagrams | `starlight-mermaid` plugin | Renders Mermaid diagrams in docs (matches existing docs) |
| Search | Pagefind (built into Starlight) | Client-side full-text search, no external service |
| CI | GitLab CI `pages` job | Replaces existing MkDocs pages job |
| Hosting | GitLab Pages | Already configured in `.gitlab-ci.yml` |

## Phase 1: Project Scaffolding

### Step 1.1: Create Branch

```bash
cd capacitarr
git checkout main
git pull
git checkout -b feature/project-site
```

### Step 1.2: Initialize Astro + Starlight Project

Create the Astro project in `capacitarr/site/`:

```bash
cd capacitarr
mkdir site
cd site
pnpm create astro@latest . -- --template starlight --typescript strict --install --no-git
```

If the interactive installer doesn't support all flags, run:

```bash
pnpm create astro@latest .
# Choose: Starlight template
# Choose: TypeScript strict
# Choose: Install dependencies
# Choose: No git init (already in a git repo)
```

### Step 1.3: Install Additional Dependencies

```bash
cd site
pnpm add @astrojs/tailwind tailwindcss @tailwindcss/vite starlight-mermaid
```

### Step 1.4: Project Structure

After scaffolding, the directory should look like:

```
capacitarr/site/
├── astro.config.mjs          # Astro + Starlight + Tailwind config
├── package.json
├── pnpm-lock.yaml
├── public/
│   ├── favicon.ico            # Copy from frontend/public/favicon.ico
│   └── screenshots/           # CI copies from ../screenshots/
├── src/
│   ├── assets/
│   │   └── capacitarr-hero.png  # App screenshot for hero section
│   ├── components/
│   │   ├── Hero.astro           # Landing page hero
│   │   ├── FeatureGrid.astro    # Feature cards section
│   │   ├── Integrations.astro   # Integration logos section
│   │   ├── HowItWorks.astro     # Three-step visual
│   │   ├── QuickStart.astro     # Docker Compose code block
│   │   └── Footer.astro         # Site footer
│   ├── content/
│   │   └── docs/                # Starlight docs (CI-populated)
│   │       └── .gitkeep         # Placeholder — CI copies real docs here
│   ├── layouts/
│   │   └── Landing.astro        # Full-width layout for landing page
│   ├── pages/
│   │   └── index.astro          # Custom landing page (NOT Starlight)
│   └── styles/
│       └── theme.css            # Violet dark theme tokens
└── tsconfig.json
```

## Phase 2: Theme & Design Tokens

### Step 2.1: Create Theme CSS

Create `site/src/styles/theme.css` with the Capacitarr violet dark theme tokens. These are extracted from `frontend/app/assets/css/main.css`:

```css
/* Capacitarr Violet Dark Theme for Project Site
   Source: frontend/app/assets/css/main.css */

:root {
  /* Force dark mode — the site is always dark */
  color-scheme: dark;

  /* ---- Dark Neutrals ---- */
  --color-background: oklch(0.14 0.025 280);
  --color-foreground: oklch(0.93 0.008 270);
  --color-card: oklch(0.18 0.035 280);
  --color-card-foreground: oklch(0.93 0.008 270);
  --color-border: oklch(0.35 0.06 280);
  --color-muted: oklch(0.22 0.035 280);
  --color-muted-foreground: oklch(0.62 0.02 280);
  --color-secondary: oklch(0.23 0.035 280);
  --color-secondary-foreground: oklch(0.90 0.008 270);

  /* ---- Violet Primary ---- */
  --color-primary: oklch(0.606 0.25 292.717);
  --color-primary-foreground: oklch(1 0 0);
  --color-ring: oklch(0.606 0.25 292.717);

  /* ---- Semantic ---- */
  --color-destructive: oklch(0.577 0.245 27.325);
  --color-success: oklch(0.648 0.2 160);
  --color-warning: oklch(0.75 0.183 55.934);
}
```

### Step 2.2: Configure Starlight Theme

In `astro.config.mjs`, override Starlight's CSS custom properties to match the violet dark theme. Starlight uses its own CSS variable naming convention (`--sl-color-*`), so map them:

```js
// astro.config.mjs
import { defineConfig } from 'astro/config'
import starlight from '@astrojs/starlight'
import tailwindcss from '@tailwindcss/vite'
import starlightMermaid from 'starlight-mermaid'

export default defineConfig({
  site: 'https://your-gitlab-namespace.gitlab.io/capacitarr',
  vite: {
    plugins: [tailwindcss()],
  },
  integrations: [
    starlight({
      title: 'Capacitarr',
      description: 'Intelligent media library capacity manager',
      plugins: [starlightMermaid()],
      customCss: ['./src/styles/starlight-overrides.css'],
      sidebar: [
        { label: 'Home', link: '/' },
        {
          label: 'Getting Started',
          items: [
            { label: 'Deployment Guide', slug: 'deployment' },
            { label: 'Configuration', slug: 'configuration' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'Scoring Algorithm', slug: 'scoring' },
            { label: 'API Overview', slug: 'api' },
            { label: 'API Examples', slug: 'api/examples' },
            { label: 'API Workflows', slug: 'api/workflows' },
            { label: 'API Versioning', slug: 'api/versioning' },
          ],
        },
        {
          label: 'Project',
          items: [
            { label: 'Release Workflow', slug: 'releasing' },
            { label: 'Changelog', slug: 'changelog' },
          ],
        },
      ],
      social: {
        gitlab: 'https://gitlab.com/your-namespace/capacitarr',
      },
    }),
  ],
})
```

### Step 2.3: Starlight CSS Overrides

Create `site/src/styles/starlight-overrides.css` to map Starlight's variables to the violet dark theme:

```css
/* Override Starlight's default colors to match Capacitarr's violet dark theme */
:root {
  --sl-color-accent-low: oklch(0.22 0.06 292);
  --sl-color-accent: oklch(0.606 0.25 292.717);
  --sl-color-accent-high: oklch(0.85 0.08 292);
  --sl-color-white: oklch(0.93 0.008 270);
  --sl-color-gray-1: oklch(0.80 0.01 280);
  --sl-color-gray-2: oklch(0.62 0.02 280);
  --sl-color-gray-3: oklch(0.45 0.03 280);
  --sl-color-gray-4: oklch(0.35 0.06 280);
  --sl-color-gray-5: oklch(0.22 0.035 280);
  --sl-color-gray-6: oklch(0.18 0.035 280);
  --sl-color-black: oklch(0.14 0.025 280);
}
```

## Phase 3: Landing Page

### Step 3.1: Landing Layout

Create `site/src/layouts/Landing.astro` — a full-width layout that does NOT use Starlight's docs layout:

```astro
---
interface Props {
  title: string
  description: string
}
const { title, description } = Astro.props
---

<!doctype html>
<html lang="en" class="dark">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <meta name="description" content={description} />
    <title>{title}</title>
    <link rel="icon" href="/favicon.ico" />
  </head>
  <body class="bg-background text-foreground min-h-screen">
    <slot />
  </body>
</html>
```

### Step 3.2: Landing Page Sections

Create `site/src/pages/index.astro` with these sections:

#### Section 1: Hero

- Full-width dark gradient background: `linear-gradient(135deg, oklch(0.10 0.02 280), oklch(0.16 0.04 292))`
- Large "Capacitarr" heading with animated gradient text (violet → purple → indigo shift)
- Tagline: "Intelligent media library capacity manager for the *arr ecosystem"
- Two CTA buttons: "Get Started" → `/docs/deployment/`, "View Source" → GitLab repo
- Large app screenshot in a browser-frame mockup with a violet glow `box-shadow`
- Version badge reading from build-time env var

#### Section 2: Feature Grid

- 3×2 grid of glass-morphism cards (`backdrop-filter: blur(12px)`, semi-transparent `oklch(0.18 0.035 280 / 0.6)` background, violet border glow)
- Features to highlight:
  1. 🎯 **Intelligent Scoring** — Six weighted factors rank every media item
  2. 🔧 **Cascading Rule Builder** — Visual rules with always_keep, prefer_keep, prefer_delete, always_delete
  3. 🔌 **9 Integrations** — Sonarr, Radarr, Lidarr, Readarr, Plex, Jellyfin, Emby, Overseerr, Tautulli
  4. 💾 **Disk Group Monitoring** — Track capacity across multiple disk groups
  5. 📊 **Score Transparency** — Full per-item breakdowns showing each factor
  6. 📋 **Audit Trail** — Complete history of every engine action

#### Section 3: Integration Logos

- Horizontal row of integration logos/icons
- Grayscale by default, color on hover (CSS `filter: grayscale(1)` → `grayscale(0)` transition)
- Services: Sonarr, Radarr, Lidarr, Readarr, Plex, Jellyfin, Emby, Tautulli, Overseerr

#### Section 4: How It Works

- Three-step horizontal layout with connecting lines/arrows:
  1. **Connect** — Link your *arr apps and media servers
  2. **Configure** — Set thresholds, weights, and protection rules
  3. **Relax** — Capacitarr handles capacity automatically
- Each step has an icon and brief description
- Subtle fade-in animation on scroll (IntersectionObserver)

#### Section 5: Quick Start

- Dark code block with the Docker Compose snippet from README.md
- Copy-to-clipboard button
- Version badge: `v{version}` pulled from `CAPACITARR_VERSION` env var
- "Full deployment guide →" link to `/docs/deployment/`

#### Section 6: Footer

- Links: Documentation, GitLab, License, Changelog
- "Made by Ghent Starshadow"
- Current version number

### Step 3.3: Visual Effects

Implement these CSS effects for the "flashy but tasteful" aesthetic:

1. **Animated gradient text** on the hero heading:
   ```css
   .gradient-text {
     background: linear-gradient(135deg, oklch(0.606 0.25 292.717), oklch(0.65 0.22 310), oklch(0.55 0.20 270));
     background-size: 200% 200%;
     -webkit-background-clip: text;
     -webkit-text-fill-color: transparent;
     animation: gradient-shift 6s ease infinite;
   }
   @keyframes gradient-shift {
     0%, 100% { background-position: 0% 50%; }
     50% { background-position: 100% 50%; }
   }
   ```

2. **Glass-morphism cards**:
   ```css
   .glass-card {
     backdrop-filter: blur(12px);
     background: oklch(0.18 0.035 280 / 0.6);
     border: 1px solid oklch(0.606 0.25 292.717 / 0.15);
     border-radius: 0.75rem;
     transition: border-color 0.3s ease, box-shadow 0.3s ease;
   }
   .glass-card:hover {
     border-color: oklch(0.606 0.25 292.717 / 0.4);
     box-shadow: 0 0 20px oklch(0.606 0.25 292.717 / 0.1);
   }
   ```

3. **Hero screenshot glow**:
   ```css
   .hero-screenshot {
     border-radius: 0.75rem;
     box-shadow:
       0 0 40px oklch(0.606 0.25 292.717 / 0.2),
       0 0 80px oklch(0.606 0.25 292.717 / 0.1);
   }
   ```

4. **Scroll-triggered fade-in** (via IntersectionObserver in a `<script>` tag — Astro ships this as zero-cost since it's a tiny inline script):
   ```js
   const observer = new IntersectionObserver((entries) => {
     entries.forEach(entry => {
       if (entry.isIntersecting) {
         entry.target.classList.add('visible')
         observer.unobserve(entry.target)
       }
     })
   }, { threshold: 0.1 })
   document.querySelectorAll('.fade-in').forEach(el => observer.observe(el))
   ```

## Phase 4: Documentation Setup

### Step 4.1: Docs Content Strategy

The docs in `capacitarr/docs/` are the **source of truth**. They are NOT duplicated into `site/src/content/docs/` in the repo. Instead, CI copies them at build time.

For local development, create a symlink or copy script:

```bash
# For local dev — symlink docs into Starlight content dir
cd site
ln -s ../../docs src/content/docs
```

Or use a dev script in `site/package.json`:

```json
{
  "scripts": {
    "dev": "pnpm sync-docs && astro dev",
    "sync-docs": "rm -rf src/content/docs && cp -r ../docs src/content/docs && cp ../CHANGELOG.md src/content/docs/changelog.md",
    "build": "astro build"
  }
}
```

### Step 4.2: Markdown Compatibility

The existing docs use standard markdown with some MkDocs-specific syntax. Check and convert:

| Pattern | Found In | Starlight Equivalent | Action |
|---------|----------|---------------------|--------|
| `!!! note "Title"` | Check all docs | `:::note[Title]` | Find-and-replace if present |
| `!!! warning` | Check all docs | `:::caution` | Find-and-replace if present |
| ` ```mermaid ` | scoring.md, README.md | Same (via starlight-mermaid plugin) | No change needed |
| `=== "Tab 1"` | Check all docs | Starlight `<Tabs>` component | Rewrite if present |
| Relative links `[text](file.md)` | All docs | Same | No change needed |

Review each doc file and make any necessary syntax adjustments. Most of the existing docs (`configuration.md`, `deployment.md`, `scoring.md`, `releasing.md`) are standard markdown with tables and code blocks — they should work with zero changes.

### Step 4.3: Frontmatter

Starlight requires frontmatter on each doc page. Add frontmatter to each doc file (or have CI prepend it). Example for `deployment.md`:

```yaml
---
title: Deployment Guide
description: Docker setup, reverse proxy configuration, and authentication for Capacitarr
---
```

**Decision:** Either modify the source docs in `capacitarr/docs/` to include Starlight-compatible frontmatter (preferred — keeps them self-contained), or have CI inject frontmatter during the copy step.

### Step 4.4: API Documentation

The existing `docs/api/` directory contains:
- `README.md` — API overview
- `examples.md` — API usage examples
- `workflows.md` — Common API workflows
- `versioning.md` — API versioning policy
- `openapi.yaml` — Full OpenAPI 3.1 spec

For the OpenAPI spec, consider using the `starlight-openapi` plugin to render interactive API docs from `openapi.yaml`. Alternatively, link to a Swagger UI instance or just include the YAML as a downloadable file.

### Step 4.5: Changelog Page

CI injects `CHANGELOG.md` as a docs page with frontmatter:

```bash
# In CI script
echo '---' > src/content/docs/changelog.md
echo 'title: Changelog' >> src/content/docs/changelog.md
echo 'description: Release history for Capacitarr' >> src/content/docs/changelog.md
echo '---' >> src/content/docs/changelog.md
echo '' >> src/content/docs/changelog.md
cat ../CHANGELOG.md >> src/content/docs/changelog.md
```

## Phase 5: CI/CD Integration

### Step 5.1: Update `.gitlab-ci.yml`

Replace the existing `pages` job (currently MkDocs) with the Astro build:

```yaml
pages:
  stage: pages
  image: node:22-alpine
  before_script:
    - corepack enable
    - cd site && pnpm install --frozen-lockfile
  script:
    # Sync docs from source of truth into Starlight content directory
    - mkdir -p src/content/docs/api
    - cp ../docs/index.md src/content/docs/
    - cp ../docs/deployment.md src/content/docs/
    - cp ../docs/configuration.md src/content/docs/
    - cp ../docs/scoring.md src/content/docs/
    - cp ../docs/releasing.md src/content/docs/
    - cp ../docs/api/README.md src/content/docs/api/index.md
    - cp ../docs/api/examples.md src/content/docs/api/
    - cp ../docs/api/workflows.md src/content/docs/api/
    - cp ../docs/api/versioning.md src/content/docs/api/
    # Inject changelog with frontmatter
    - |
      echo '---' > src/content/docs/changelog.md
      echo 'title: Changelog' >> src/content/docs/changelog.md
      echo 'description: Release history for Capacitarr' >> src/content/docs/changelog.md
      echo '---' >> src/content/docs/changelog.md
      echo '' >> src/content/docs/changelog.md
      cat ../CHANGELOG.md >> src/content/docs/changelog.md
    # Copy screenshots
    - mkdir -p public/screenshots
    - cp -r ../screenshots/* public/screenshots/ 2>/dev/null || true
    # Copy favicon
    - cp ../frontend/public/favicon.ico public/favicon.ico 2>/dev/null || true
    # Inject version as build-time env var
    - export CAPACITARR_VERSION=$(node -p "require('../package.json').version")
    - echo "Building site for Capacitarr v$CAPACITARR_VERSION"
    # Build
    - pnpm build
    - mv dist/ ../public/
  artifacts:
    paths:
      - public
  rules:
    - if: $CI_COMMIT_BRANCH == "main"
```

### Step 5.2: Version Injection

In the Astro landing page, read the version from an environment variable at build time:

```astro
---
// In index.astro or a component
const version = import.meta.env.CAPACITARR_VERSION || '0.0.0'
---
<span class="version-badge">v{version}</span>
```

In `astro.config.mjs`, expose the env var:

```js
export default defineConfig({
  vite: {
    define: {
      'import.meta.env.CAPACITARR_VERSION': JSON.stringify(process.env.CAPACITARR_VERSION || '0.0.0'),
    },
  },
  // ...
})
```

### Step 5.3: Remove MkDocs Config

After the Astro site is working:

1. Delete `capacitarr/mkdocs.yml`
2. Remove `pip install mkdocs-material mkdocs-exclude` from any CI references
3. The `docs/` directory stays — it's still the source of truth, just rendered by Starlight now

## Phase 6: Screenshots & Assets

### Step 6.1: Take App Screenshots

The site needs high-quality screenshots of the Capacitarr UI. Currently only `screenshots/login-styled.png` exists. Additional screenshots needed:

1. **Dashboard** — Main dashboard showing disk groups, capacity bars, and media items
2. **Rule Builder** — The cascading rule builder with example rules
3. **Score Breakdown** — A score detail modal showing factor contributions
4. **Settings** — Integration configuration page
5. **Audit Log** — Audit trail showing engine actions

These should be taken in the violet dark theme at a consistent viewport size (e.g., 1280×800). Save to `capacitarr/screenshots/`.

### Step 6.2: Integration Logos

Source SVG logos for the supported integrations. These can be:
- Downloaded from each project's official branding/assets
- Stored in `site/src/assets/logos/`
- Referenced in the Integrations component

Services needing logos: Sonarr, Radarr, Lidarr, Readarr, Plex, Jellyfin, Emby, Tautulli, Overseerr.

## Phase 7: Testing & Polish

### Step 7.1: Local Development

```bash
cd capacitarr/site
pnpm sync-docs   # Copy docs into content dir
pnpm dev          # Start Astro dev server (usually port 4321)
```

### Step 7.2: Verify

- [ ] Landing page loads at `/`
- [ ] All six sections render correctly
- [ ] Animated gradient text works
- [ ] Glass-morphism cards have proper blur and glow
- [ ] Hero screenshot has violet glow shadow
- [ ] Scroll animations trigger on scroll
- [ ] "Get Started" links to `/docs/deployment/`
- [ ] Version badge shows correct version
- [ ] Docker Compose code block has copy button
- [ ] Documentation loads at `/docs/`
- [ ] All doc pages render (deployment, configuration, scoring, releasing, API)
- [ ] Changelog page shows release history
- [ ] Sidebar navigation works
- [ ] Search works (Pagefind)
- [ ] Dark theme is consistent between landing page and docs
- [ ] Mermaid diagrams render in docs
- [ ] Mobile responsive layout works
- [ ] Favicon displays correctly

### Step 7.3: Build Test

```bash
cd capacitarr/site
pnpm build
# Verify output in dist/
# Check that dist/ contains both the landing page and docs
```

### Step 7.4: Lighthouse Audit

Run a Lighthouse audit on the built site to verify:
- Performance score ≥ 95 (static site should be near-perfect)
- Accessibility score ≥ 90
- SEO score ≥ 90

## File Checklist

Files to **create**:
- [ ] `site/package.json`
- [ ] `site/astro.config.mjs`
- [ ] `site/tsconfig.json`
- [ ] `site/src/styles/theme.css`
- [ ] `site/src/styles/starlight-overrides.css`
- [ ] `site/src/layouts/Landing.astro`
- [ ] `site/src/pages/index.astro`
- [ ] `site/src/components/Hero.astro`
- [ ] `site/src/components/FeatureGrid.astro`
- [ ] `site/src/components/Integrations.astro`
- [ ] `site/src/components/HowItWorks.astro`
- [ ] `site/src/components/QuickStart.astro`
- [ ] `site/src/components/Footer.astro`
- [ ] `site/src/content/docs/.gitkeep`
- [ ] `site/public/.gitkeep`

Files to **modify**:
- [ ] `.gitlab-ci.yml` — Replace MkDocs `pages` job with Astro build
- [ ] `docs/*.md` — Add Starlight-compatible frontmatter to each doc file
- [ ] `.gitignore` — Add `site/node_modules/`, `site/dist/`, `site/src/content/docs/` (CI-generated)

Files to **delete** (after Astro site is confirmed working):
- [ ] `mkdocs.yml`

## Notes for the Implementing Agent

1. **Start by creating the branch**: `git checkout -b feature/project-site` from `main`
2. **Do NOT modify files outside `site/`** until Phase 5 (CI update) and Phase 4.3 (doc frontmatter)
3. **The `site/src/content/docs/` directory should be mostly empty in git** — CI populates it. Only a `.gitkeep` file should be committed. For local dev, use the `sync-docs` script.
4. **Force dark mode on the landing page** — do not implement a light/dark toggle. The site identity IS the dark violet theme.
5. **Use `pnpm`** as the package manager (consistent with the frontend).
6. **The site URL** will be something like `https://namespace.gitlab.io/capacitarr/` — configure `base` in `astro.config.mjs` accordingly if the repo is not at the root of the GitLab namespace.
7. **Screenshots**: If `screenshots/` only has `login-styled.png`, use it for the hero. Additional screenshots can be added later — design the hero section to work with a single screenshot.
8. **Integration logos**: If official SVGs aren't readily available, use text labels with colored backgrounds as placeholders. The logos can be swapped in later.
9. **Commit frequently** with conventional commit messages: `feat(site): scaffold Astro + Starlight project`, `feat(site): add landing page hero section`, etc.
10. **Do not run `pnpm dev` or `pnpm build` directly** — use `docker compose` if testing the full app, but for the static site specifically, running `pnpm dev` in the `site/` directory is acceptable since it's a standalone static site generator, not the Capacitarr application itself.
