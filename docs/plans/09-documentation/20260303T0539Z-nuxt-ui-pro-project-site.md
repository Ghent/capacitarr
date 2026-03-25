# Nuxt UI v4 Project Site for Capacitarr

**Created:** 2026-03-03T05:39Z
**Updated:** 2026-03-03T13:55Z
**Status:** ✅ Complete
**Scope:** Replace Astro + Starlight site (`capacitarr/site/`) with Nuxt UI v4
**Deploys to:** GitLab Pages via CI pipeline

## Overview

Replace the current Astro + Starlight project site with a Nuxt UI v4 site. This eliminates the Tailwind/Starlight CSS conflict that prevents the sidebar from rendering, and provides a docs + landing page experience using the same stack as the Capacitarr app (Nuxt + Tailwind CSS v4).

## Why Nuxt UI v4 (not v3 / Pro)

In Nuxt UI v4 (`@nuxt/ui@^4.5`), all previously-Pro components — `UPageHero`, `UPageSection`, `UContentNavigation`, `UContentToc`, `UHeader`, `UFooter`, etc. — are **included in the base `@nuxt/ui` package**. The separate `@nuxt/ui-pro` module is a legacy v3 package that is no longer needed.

- **No separate Pro license required** — the `NUXT_UI_PRO_LICENSE` env var is a v3 concept
- **One package:** `@nuxt/ui` provides all 100+ components (base + previously-Pro)
- **Templates available:** Official `nuxt-ui-templates/docs` template provides a complete docs site reference

## Why Replace Astro + Starlight

1. **CSS conflict** — `@tailwindcss/vite` and Starlight's `@layer` system are incompatible, breaking the sidebar
2. **Stack mismatch** — Astro is a different framework from the Nuxt app, requiring separate tooling and knowledge
3. **Limited customization** — Starlight's docs pages resist visual customization due to its opinionated CSS pipeline
4. **Nuxt UI v4 is purpose-built** — provides landing page components, docs layout with sidebar, search, and TOC

## Technology Stack

| Component | Technology |
|-----------|-----------|
| Framework | Nuxt 4 (`nuxt@^4.3`) |
| UI Library | Nuxt UI v4 (`@nuxt/ui@^4.5`) |
| Styling | Tailwind CSS v4 + oklch design tokens |
| Docs | Nuxt Content v3 (`@nuxt/content`) with collections API |
| Search | `UContentSearch` + `UContentSearchButton` (built-in search over content collections) |
| Fonts | Geist Sans + Geist Mono via `@fontsource` |
| CI | GitLab CI `pages` job |
| Hosting | GitLab Pages |

## Phase 1: Project Setup

### Step 1.1: Create Branch

```bash
cd capacitarr
git checkout main && git pull
git checkout -b feature/nuxt-ui-site
```

### Step 1.2: Remove Astro + Starlight

```bash
rm -rf site/
```

### Step 1.3: Initialize Nuxt Site

```bash
pnpm dlx nuxi@latest init site
cd site
pnpm add @nuxt/ui @nuxt/content tailwindcss @fontsource/geist-sans @fontsource/geist-mono
```

> **Note:** `@nuxt/ui` v4 pulls in `@nuxtjs/color-mode`, `@nuxt/icon`, `@nuxt/fonts`, `@tailwindcss/vite`, `reka-ui`, `tailwind-variants`, and `fuse.js` as transitive dependencies. Do not install them separately.

### Step 1.4: Project Structure

Based on the official [nuxt-ui-templates/docs](https://github.com/nuxt-ui-templates/docs) template:

```
capacitarr/site/
├── nuxt.config.ts           # Nuxt config with @nuxt/ui + @nuxt/content modules
├── content.config.ts         # Content collections definition
├── package.json
├── pnpm-lock.yaml
├── app/
│   ├── app.vue               # Root: UApp + AppHeader + UMain + AppFooter + UContentSearch
│   ├── app.config.ts          # UI color theme + header/footer/toc config
│   ├── assets/
│   │   └── css/
│   │       └── main.css       # Tailwind + Nuxt UI imports, theme tokens, fonts
│   ├── components/
│   │   ├── AppHeader.vue      # UHeader with search, nav, color mode
│   │   ├── AppFooter.vue      # UFooter with links
│   │   └── AppLogo.vue        # Logo component
│   ├── layouts/
│   │   └── docs.vue           # Docs layout: UPage + UPageAside + UContentNavigation
│   └── pages/
│       ├── index.vue          # Landing page (renders content/index.md MDC)
│       └── [...slug].vue      # Docs catch-all route
├── content/
│   ├── index.md               # Landing page content (MDC components)
│   └── docs/                  # CI copies from ../docs/
│       └── .gitkeep
├── public/
│   ├── favicon.ico
│   └── screenshots/
└── tsconfig.json
```

## Phase 2: Configuration

### Step 2.1: Nuxt Config

```ts
// nuxt.config.ts
export default defineNuxtConfig({
  modules: ['@nuxt/ui', '@nuxt/content'],

  css: ['~/assets/css/main.css'],

  app: {
    baseURL: '/software/capacitarr/',
  },

  content: {
    build: {
      markdown: {
        toc: {
          searchDepth: 1,
        },
      },
    },
  },

  nitro: {
    prerender: {
      routes: ['/'],
      crawlLinks: true,
      autoSubfolderIndex: false,
    },
  },

  icon: {
    provider: 'iconify',
  },

  compatibilityDate: '2024-07-11',
})
```

### Step 2.2: Content Collections Config

Nuxt Content v3 uses a collections API defined in `content.config.ts`:

```ts
// content.config.ts
import { defineContentConfig, defineCollection, z } from '@nuxt/content'

export default defineContentConfig({
  collections: {
    landing: defineCollection({
      type: 'page',
      source: 'index.md',
    }),
    docs: defineCollection({
      type: 'page',
      source: {
        include: '**',
        exclude: ['index.md'],
      },
      schema: z.object({
        links: z.array(z.object({
          label: z.string(),
          icon: z.string(),
          to: z.string(),
          target: z.string().optional(),
        })).optional(),
      }),
    }),
  },
})
```

### Step 2.3: App Config — Theme + Site Config

```ts
// app/app.config.ts
export default defineAppConfig({
  ui: {
    colors: {
      primary: 'violet',
      neutral: 'zinc',
    },
    footer: {
      slots: {
        root: 'border-t border-default',
        left: 'text-sm text-muted',
      },
    },
  },
  seo: {
    siteName: 'Capacitarr',
  },
  header: {
    title: 'Capacitarr',
    to: '/',
    search: true,
    colorMode: true,
    links: [{
      icon: 'i-simple-icons-gitlab',
      to: 'https://gitlab.com/starshadow/software/capacitarr',
      target: '_blank',
      'aria-label': 'GitLab',
    }],
  },
  footer: {
    credits: `© ${new Date().getFullYear()} Capacitarr`,
    colorMode: false,
    links: [{
      icon: 'i-simple-icons-gitlab',
      to: 'https://gitlab.com/starshadow/software/capacitarr',
      target: '_blank',
      'aria-label': 'Capacitarr on GitLab',
    }],
  },
  toc: {
    title: 'On this page',
    bottom: {
      title: 'Resources',
      links: [{
        icon: 'i-lucide-book-open',
        label: 'Nuxt UI Docs',
        to: 'https://ui.nuxt.com/docs/getting-started/installation/nuxt',
        target: '_blank',
      }],
    },
  },
})
```

### Step 2.4: CSS — Theme Tokens + Fonts

```css
/* app/assets/css/main.css */
@import "tailwindcss";
@import "@nuxt/ui";

/* Ensure content files are scanned for Tailwind classes */
@source "../../../content/**/*";

/* Geist Sans — body text (matches the Capacitarr app) */
@import '@fontsource/geist-sans/400.css';
@import '@fontsource/geist-sans/500.css';
@import '@fontsource/geist-sans/600.css';
@import '@fontsource/geist-sans/700.css';

/* Geist Mono — code blocks, monospace text */
@import '@fontsource/geist-mono/400.css';

@theme static {
  --font-sans: 'Geist Sans', 'Geist', ui-sans-serif, system-ui, sans-serif;
  --font-mono: 'Geist Mono', ui-monospace, SFMono-Regular, monospace;
}
```

> **Key difference from v3:** The CSS imports are `@import "tailwindcss"` and `@import "@nuxt/ui"` — NOT `@import "@nuxt/ui-pro"`. Font families are set via Tailwind CSS v4's `@theme` directive, not `app.config.ts`.

## Phase 3: App Shell

### Step 3.1: App Root

```vue
<!-- app/app.vue -->
<script setup lang="ts">
import type { ContentNavigationItem } from '@nuxt/content'

const { seo } = useAppConfig()

const { data: navigation } = await useAsyncData('navigation', () =>
  queryCollectionNavigation('docs'),
)
const { data: files } = useLazyAsyncData('search', () =>
  queryCollectionSearchSections('docs'),
  { server: false },
)

useHead({
  meta: [{ name: 'viewport', content: 'width=device-width, initial-scale=1' }],
  link: [{ rel: 'icon', href: '/favicon.ico' }],
  htmlAttrs: { lang: 'en' },
})

useSeoMeta({
  titleTemplate: `%s - ${seo?.siteName}`,
  ogSiteName: seo?.siteName,
})

provide('navigation', navigation)
</script>

<template>
  <UApp>
    <NuxtLoadingIndicator />

    <AppHeader />

    <UMain>
      <NuxtLayout>
        <NuxtPage />
      </NuxtLayout>
    </UMain>

    <AppFooter />

    <ClientOnly>
      <LazyUContentSearch
        :files="files"
        :navigation="navigation"
      />
    </ClientOnly>
  </UApp>
</template>
```

### Step 3.2: Header Component

```vue
<!-- app/components/AppHeader.vue -->
<script setup lang="ts">
import type { ContentNavigationItem } from '@nuxt/content'

const navigation = inject<Ref<ContentNavigationItem[]>>('navigation')
const { header } = useAppConfig()
</script>

<template>
  <UHeader :to="header?.to || '/'">
    <template #left>
      <NuxtLink :to="header?.to || '/'">
        <span class="font-bold text-lg">{{ header?.title || 'Capacitarr' }}</span>
      </NuxtLink>
    </template>

    <UContentSearchButton
      v-if="header?.search"
      :collapsed="false"
      class="w-full"
    />

    <template #right>
      <UContentSearchButton
        v-if="header?.search"
        class="lg:hidden"
      />

      <UColorModeButton v-if="header?.colorMode" />

      <template v-if="header?.links">
        <UButton
          v-for="(link, index) of header.links"
          :key="index"
          v-bind="{ color: 'neutral', variant: 'ghost', ...link }"
        />
      </template>
    </template>

    <template #body>
      <UContentNavigation
        highlight
        :navigation="navigation"
      />
    </template>
  </UHeader>
</template>
```

### Step 3.3: Footer Component

```vue
<!-- app/components/AppFooter.vue -->
<script setup lang="ts">
const { footer } = useAppConfig()
</script>

<template>
  <UFooter>
    <template #left>
      {{ footer.credits }}
    </template>

    <template #right>
      <UColorModeButton v-if="footer?.colorMode" />

      <template v-if="footer?.links">
        <UButton
          v-for="(link, index) of footer?.links"
          :key="index"
          v-bind="{ color: 'neutral', variant: 'ghost', ...link }"
        />
      </template>
    </template>
  </UFooter>
</template>
```

## Phase 4: Documentation Pages

### Step 4.1: Docs Layout

The docs layout provides the left sidebar with `UContentNavigation`:

```vue
<!-- app/layouts/docs.vue -->
<script setup lang="ts">
import type { ContentNavigationItem } from '@nuxt/content'

const navigation = inject<Ref<ContentNavigationItem[]>>('navigation')
</script>

<template>
  <UContainer>
    <UPage>
      <template #left>
        <UPageAside>
          <UContentNavigation
            highlight
            :navigation="navigation"
          />
        </UPageAside>
      </template>

      <slot />
    </UPage>
  </UContainer>
</template>
```

This provides:
- **Left sidebar** — collapsible navigation groups via `UContentNavigation`
- **Mobile responsive** — hamburger menu + drawer handled automatically
- **Scroll tracking** — `highlight` prop tracks current page in sidebar

### Step 4.2: Docs Catch-All Page

```vue
<!-- app/pages/[...slug].vue -->
<script setup lang="ts">
import type { ContentNavigationItem } from '@nuxt/content'
import { findPageHeadline } from '@nuxt/content/utils'

definePageMeta({
  layout: 'docs',
})

const route = useRoute()
const { toc } = useAppConfig()
const navigation = inject<Ref<ContentNavigationItem[]>>('navigation')

const { data: page } = await useAsyncData(route.path, () =>
  queryCollection('docs').path(route.path).first(),
)
if (!page.value) {
  throw createError({ statusCode: 404, statusMessage: 'Page not found', fatal: true })
}

const { data: surround } = await useAsyncData(`${route.path}-surround`, () =>
  queryCollectionItemSurroundings('docs', route.path, {
    fields: ['description'],
  }),
)

const title = page.value.seo?.title || page.value.title
const description = page.value.seo?.description || page.value.description

useSeoMeta({ title, ogTitle: title, description, ogDescription: description })

const headline = computed(() => findPageHeadline(navigation?.value, page.value?.path))

const links = computed(() => {
  const result = []
  if (toc?.bottom?.edit) {
    result.push({
      icon: 'i-lucide-external-link',
      label: 'Edit this page',
      to: `${toc.bottom.edit}/${page?.value?.stem}.${page?.value?.extension}`,
      target: '_blank',
    })
  }
  return [...result, ...(toc?.bottom?.links || [])].filter(Boolean)
})
</script>

<template>
  <UPage v-if="page">
    <UPageHeader
      :title="page.title"
      :description="page.description"
      :headline="headline"
    />

    <UPageBody>
      <ContentRenderer
        v-if="page"
        :value="page"
      />

      <USeparator v-if="surround?.length" />

      <UContentSurround :surround="surround" />
    </UPageBody>

    <template
      v-if="page?.body?.toc?.links?.length"
      #right
    >
      <UContentToc
        :title="toc?.title"
        :links="page.body?.toc?.links"
      >
        <template
          v-if="toc?.bottom"
          #bottom
        >
          <div class="hidden lg:block space-y-6">
            <USeparator
              v-if="page.body?.toc?.links?.length"
              type="dashed"
            />

            <UPageLinks
              :title="toc.bottom.title"
              :links="links"
            />
          </div>
        </template>
      </UContentToc>
    </template>
  </UPage>
</template>
```

This gives:
- **Right sidebar** — table of contents ("On this page") via `UContentToc`
- **Prev/next navigation** — via `UContentSurround`
- **Search** — via `UContentSearch` in `app.vue` (triggered by `UContentSearchButton` in header)
- **Breadcrumbs** — `headline` computed from navigation
- **SEO** — automatic title/description from frontmatter

### Step 4.3: Content Sync

Same strategy as before — `docs/*.md` stays as source of truth, CI copies into `content/docs/` at build time:

```json
{
  "scripts": {
    "sync-docs": "node scripts/sync-docs.mjs"
  }
}
```

### Step 4.4: Markdown Compatibility

Nuxt Content v3 supports standard Markdown plus MDC (Markdown Components). The existing docs should work with minimal changes:

- Starlight `:::caution` admonitions → Nuxt Content's `::callout` or MDC `::u-alert` syntax
- Standard headings, tables, code blocks, links → work as-is
- Mermaid diagrams → add `nuxt-mermaid` module or render client-side with a custom MDC component

### Step 4.5: Navigation Order

Navigation order is controlled by numeric prefixes in filenames or `_dir.yml` / `.navigation.yml` files. Since the docs are synced at build time, either:
1. Add numeric prefixes during the CI copy step
2. Place `.navigation.yml` files in `content/docs/` with ordering

## Phase 5: Landing Page

### Step 5.1: Landing Page Route

The landing page renders MDC content from `content/index.md`:

```vue
<!-- app/pages/index.vue -->
<script setup lang="ts">
const { data: page } = await useAsyncData('index', () =>
  queryCollection('landing').path('/').first(),
)
if (!page.value) {
  throw createError({ statusCode: 404, statusMessage: 'Page not found', fatal: true })
}

const title = page.value.seo?.title || page.value.title
const description = page.value.seo?.description || page.value.description

useSeoMeta({
  titleTemplate: '',
  title,
  ogTitle: title,
  description,
  ogDescription: description,
})
</script>

<template>
  <ContentRenderer
    v-if="page"
    :value="page"
    :prose="false"
  />
</template>
```

### Step 5.2: Landing Page Content (MDC)

The landing page is written in MDC markup using Nuxt UI components directly in markdown. This is the pattern used by the official Nuxt UI templates:

```markdown
<!-- content/index.md -->
---
seo:
  title: Capacitarr
  description: Intelligent media library management — automatically score, evaluate, and clean up your *arr stack
---

::u-page-hero{class="dark:bg-gradient-to-b from-neutral-900 to-neutral-950"}
---
orientation: horizontal
---

#title
Intelligent Media Library [Management]{.text-primary}.

#description
Automatically score, evaluate, and clean up your *arr stack.
Connect Sonarr, Radarr, Lidarr, Readarr, Plex, Jellyfin, Emby, Tautulli, and Overseerr.

#links
  :::u-button
  ---
  to: /getting-started
  size: xl
  trailing-icon: i-lucide-arrow-right
  ---
  Get Started
  :::

  :::u-button
  ---
  icon: i-simple-icons-gitlab
  color: neutral
  variant: outline
  size: xl
  to: https://gitlab.com/starshadow/software/capacitarr
  target: _blank
  ---
  View on GitLab
  :::

#default
  ![Capacitarr Dashboard](/screenshots/dashboard.png)
::

::u-page-section
#title
Features

#features
  :::u-page-feature
  ---
  icon: i-lucide-bar-chart-3
  ---
  #title
  Smart Scoring
  #description
  Score media items across multiple dimensions — age, size, popularity, watch history.
  :::

  :::u-page-feature
  ---
  icon: i-lucide-puzzle
  ---
  #title
  Multi-Integration
  #description
  Connect Sonarr, Radarr, Lidarr, Readarr, Plex, Jellyfin, Emby, Tautulli, Overseerr.
  :::

  :::u-page-feature
  ---
  icon: i-lucide-sliders-horizontal
  ---
  #title
  Configurable Rules
  #description
  Build cascading rules with custom conditions, weights, and thresholds.
  :::

  :::u-page-feature
  ---
  icon: i-lucide-shield-check
  ---
  #title
  Safe Cleanup
  #description
  Preview what would be removed before any action — safety guards prevent accidents.
  :::
::
```

This approach uses MDC syntax to embed Nuxt UI components (`::u-page-hero`, `::u-page-section`, `:::u-page-feature`, etc.) directly in markdown content. Custom CSS effects (gradient text, glass-morphism, glow) can be applied via Tailwind classes in the MDC markup.

## Phase 6: CI/CD

### Step 6.1: Update Pages Job

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
    # Build static site
    - pnpm generate
    - mv .output/public/ ../public/
  artifacts:
    paths:
      - public
  rules:
    - if: $CI_COMMIT_BRANCH == "main"
```

> **No license key needed** — unlike `@nuxt/ui-pro` v3 which required `NUXT_UI_PRO_LICENSE`, Nuxt UI v4 has no license requirement. The CI job does not need any special environment variables.

## Phase 7: Testing

- Landing page renders with MDC components (hero, features, integrations, quick start)
- `UHeader` navigation works (mobile hamburger, desktop nav)
- `UContentSearchButton` opens search and indexes docs content via `queryCollectionSearchSections`
- Docs sidebar (`UContentNavigation`) is visible and navigable
- Table of contents (`UContentToc`) renders for each doc page
- Prev/next navigation (`UContentSurround`) appears at bottom of doc pages
- All doc pages render correctly (markdown, code blocks, tables)
- Mobile responsive layout works (hamburger menu, drawer sidebar)
- Dark theme is consistent with the Capacitarr app's violet palette
- `UColorModeButton` toggles light/dark
- Build produces correct static output for GitLab Pages at `/software/capacitarr/`

## Migration Reference

Quick reference mapping all previous plan versions to the correct Nuxt UI v4 patterns:

| Previous Plan | Correct (v4) | Notes |
|--------------|-------------|-------|
| `@nuxt/ui-pro` module | `@nuxt/ui@^4.5` | Pro components merged into base package |
| `extends: ['@nuxt/ui-pro']` | `modules: ['@nuxt/ui']` | Module, not layer |
| `NUXT_UI_PRO_LICENSE` env var | *(not needed)* | No license required in v4 |
| `nuxt@^3` | `nuxt@^4.3` | Nuxt 4 |
| `@import "@nuxt/ui-pro"` in CSS | `@import "@nuxt/ui"` | Different CSS import |
| `ULandingHero` | `UPageHero` (via MDC: `::u-page-hero`) | Component renamed |
| `ULandingSection` | `UPageSection` (via MDC: `::u-page-section`) | Component renamed |
| `ULandingGrid` | `UPageGrid` (via MDC: `::u-page-grid`) | Component renamed |
| `ULandingCard` | `UPageCard` (via MDC: `::u-page-card`) | Component renamed |
| `DocsLayout` | `layouts/docs.vue` with `UPage` + `UPageAside` + `UContentNavigation` | Explicit layout |
| `queryContent(path).findOne()` | `queryCollection('docs').path(path).first()` | Content v3 collections API |
| `fetchContentNavigation()` | `queryCollectionNavigation('docs')` | Content v3 collections API |
| `content: { sources: {} }` in nuxt.config | `content.config.ts` with `defineCollection()` | Content v3 config file |
| `app.config.ts` `ui.fonts` | `@theme { --font-sans: ... }` in CSS | Tailwind CSS v4 |
| `ui: { colorMode: false }` | *(handled via app.config `header.colorMode`)* | Color mode via app config |

## Notes

- The `site/` directory remains independent from the app's `frontend/` directory
- This plan can be executed independently from the app migration plan (`20260303T0539Z-nuxt-ui-pro-app-migration.md`)
- The existing Nuxt UI Pro license is for `@nuxt/ui-pro` v3 — it is not needed for `@nuxt/ui` v4
- The landing page uses MDC (Markdown Components) to embed Nuxt UI components in content files, which is the pattern used by all official Nuxt UI v4 templates
- The official reference template is [nuxt-ui-templates/docs](https://github.com/nuxt-ui-templates/docs)
