# Site & Documentation Polish

**Date:** 2026-03-05
**Branch:** `docs/site-polish-plan`
**Scope:** Pages site accuracy, documentation corrections, credits update, GitLab repo stats widget
**Status:** ✅ Complete

---

## Executive Summary

This plan addresses a set of content accuracy issues across the Capacitarr pages site (`site/`) and source documentation (`docs/`). The site was built with hardcoded content that has drifted from the actual application behavior. The sync-docs script copies source docs to the site, so most documentation fixes must be made in `docs/` (the source of truth) and will propagate to the site automatically.

Additionally, this plan adds a Maintainerr credit callout, a GitLab repo stats widget in the site header, and performs a full regression audit to catch any remaining inaccuracies.

---

## Findings Summary

| # | Issue | Severity | Location(s) |
|---|-------|----------|-------------|
| F-01 | "Nuxt 3" should be "Nuxt 4" | 🔴 Wrong | `site/app/pages/index.vue:252`, `frontend/app/pages/help.vue:654` |
| F-02 | Quick Start says users need to "Create a Disk Group" manually | 🔴 Wrong | `docs/quick-start.md:56-61` → synced to `site/content/docs/quick-start.md` |
| F-03 | FAQ says "You can define multiple disk groups" with user-created groups | 🔴 Wrong | `site/app/components/FaqSection.vue:25-27` |
| F-04 | FAQ uses "availability" instead of "series status" | 🟡 Outdated | `site/app/components/FaqSection.vue:22` |
| F-05 | Quick Start uses "Availability" for scoring factor | 🟡 Outdated | `docs/quick-start.md:72` |
| F-06 | README intro uses "availability" in scoring dimension list | 🟡 Outdated | `README.md:7` |
| F-07 | No Maintainerr credit in CONTRIBUTORS.md or help page | 🟢 Missing | `CONTRIBUTORS.md`, `frontend/app/pages/help.vue:660-666` |
| F-08 | No GitLab repo stats in site header | 🟢 Enhancement | `site/app/components/AppHeader.vue` |
| F-09 | About section on site links to "Nuxt UI Docs" in TOC bottom | 🟡 Unnecessary | `site/app/app.config.ts:44-48` |
| F-10 | `docs/index.md` step 2 wording suggests user action for disk groups | 🟡 Misleading | `docs/index.md:8` |

---

## Phase 1: Content Accuracy Fixes (Site + Docs)

**Goal:** Fix all factual inaccuracies in documentation and site content.
**Risk:** 🟢 Low — text-only changes, no logic changes.
**Status:** ✅ Complete

### Step 1.1: Fix "Nuxt 3" → "Nuxt 4" References

The app uses Nuxt 4. The README already says "Nuxt 4" but two files still say "Nuxt 3":

- [x] `site/app/pages/index.vue:252` — Change `Built with Go, Nuxt 3, and SQLite` → `Built with Go, Nuxt 4, and SQLite`
- [x] `frontend/app/pages/help.vue:654` — Change `'Nuxt 3'` → `'Nuxt 4'` in `techStack.frontend` array

### Step 1.2: Fix Quick Start "Create a Disk Group" Section

Disk groups are determined automatically from integrations — users don't create them manually. The entire Step 5 in quick start needs rewriting.

- [x] `docs/quick-start.md:56-61` — Rewrite "## 5. Create a Disk Group" section. Replace with explanation that disk groups are automatically detected from integration root folders. Users configure **thresholds** and **targets** on the auto-detected groups via Settings → General (or wherever the threshold UI lives).
- [x] Renumber remaining steps if needed (Step 6→5, Step 7→6), or keep step count and just update the content of Step 5. — Kept as Step 5, renamed to "Configure Disk Thresholds".

**Suggested replacement for Step 5:**

```markdown
## 5. Configure Disk Thresholds

Capacitarr automatically detects disk groups from the root folders reported by your
*arr integrations. No manual setup is needed — disk groups appear on the Dashboard
as soon as integrations are connected and the engine runs.

To configure when cleanup triggers:

1. Navigate to the **Rules** page → **Disk Thresholds** section
2. Set a **threshold** — the disk usage percentage that triggers cleanup evaluation (e.g., 85%)
3. Set a **target** — the disk usage percentage the engine tries to reach (e.g., 75%)
```

### Step 1.3: Fix FAQ — Disk Group Question

The FAQ answer says "You can define multiple disk groups, each with their own integrations, thresholds, and targets." This is inaccurate.

- [x] `site/app/components/FaqSection.vue:25-27` — Rewrite the disk groups Q&A:

**Suggested replacement:**

```typescript
{
  question: 'Does it work with multiple disk groups?',
  answer: 'Yes. Capacitarr automatically detects disk groups from the root folders reported by your *arr integrations. If your movies and TV shows live on different drives, they will appear as separate disk groups on the dashboard — each with their own capacity tracking, thresholds, and targets.',
},
```

### Step 1.4: Fix "Availability" → "Series Status" Wording

The scoring factor was renamed from "Availability" to "Series Status." Update all remaining references:

- [x] `site/app/components/FaqSection.vue:22` — Change `age, file size, popularity, recency, rating, and availability` → `age, file size, popularity, recency, rating, and series status`
- [x] `docs/quick-start.md:72` — Change `**Availability** — Content available from more sources scores higher` → `**Series Status** — Ended shows score higher for deletion than continuing shows`
- [x] `README.md:7` — Change `watch history, recency, file size, ratings, age, and availability` → `watch history, recency, file size, ratings, age, and series status`

### Step 1.5: Fix `docs/index.md` Disk Group Wording

- [x] `docs/index.md:8` — Change `Group services by shared disk` to wording that makes clear disk groups are automatic. E.g.: `Disk groups are auto-detected — Capacitarr tracks capacity per root folder across your integrations.`

---

## Phase 2: Credits & Acknowledgments

**Goal:** Add Maintainerr as a special callout in credits.
**Risk:** 🟢 Low — additive content only.
**Status:** ✅ Complete

### Step 2.1: Update CONTRIBUTORS.md

Add a "Credits & Acknowledgments" section that calls out the *arr community and Maintainerr specifically.

- [x] `CONTRIBUTORS.md` — Add a new section after "Contributors":

```markdown
## Credits & Acknowledgments

### The *arr Community

Capacitarr exists because of the incredible *arr ecosystem. Special thanks to:

- **[Maintainerr](https://github.com/jorenn92/maintainerr)** — A major source of inspiration for Capacitarr's approach to media library management. The Plex OAuth implementation was adapted from Maintainerr's battle-tested `PlexAuth.ts`. Thank you to [@jorenn92](https://github.com/jorenn92) and all Maintainerr contributors.

### Open Source Libraries

- [shadcn-vue](https://www.shadcn-vue.com/) — Component library
- [Tailwind CSS](https://tailwindcss.com/) — Utility-first CSS framework
- [Nuxt](https://nuxt.com/) — Vue meta-framework
- [Geist](https://vercel.com/font) — Typography (Vercel)
```

### Step 2.2: Update Frontend Help Page Credits

- [x] `frontend/app/pages/help.vue:660-666` — Add Maintainerr to the `credits` array with special positioning (before "The *arr community" or as an expanded entry):

```typescript
const credits = [
  { name: 'Maintainerr', desc: 'Inspiration & Plex OAuth reference implementation' },
  { name: 'shadcn-vue', desc: 'Component library' },
  { name: 'Tailwind CSS', desc: 'Utility-first CSS framework' },
  { name: 'Nuxt', desc: 'Vue meta-framework' },
  { name: 'Geist', desc: 'Typography (Vercel)' },
  { name: 'The *arr community', desc: 'Inspiration and ecosystem' },
];
```

---

## Phase 3: GitLab Repo Stats Widget

**Goal:** Add a compact repo stats display in the site header, inspired by Maintainerr's GitHub widget.
**Risk:** 🟡 Medium — new component, API integration, needs fallback handling.
**Status:** ✅ Complete

### Design

Maintainerr shows: `repo-name | ◇ v3.0.1 | ☆ 1.7k | ⑂ 68` in the upper right of their docs site.

For Capacitarr on GitLab, the equivalent useful stats are:

| Stat | GitLab API | Icon | Example |
|------|-----------|------|---------|
| Latest version | Releases API or package.json | `i-lucide-tag` | `v0.5.0` |
| Stars | Project API `star_count` | `i-lucide-star` | `42` |
| Forks | Project API `forks_count` | `i-lucide-git-fork` | `3` |
| Pipeline status | Badges API | `i-lucide-circle-check` | `passing` |

**Alternative approach (recommended):** Rather than live API calls (which add complexity, CORS issues, and rate limits), use **static GitLab badges** or **build-time fetched stats** injected during the site build via the CI pipeline. The simplest approach:

1. **Option A: GitLab Badge Images** — Use `shields.io` or GitLab's native badge URLs in the header. Zero JS, zero API calls. Works everywhere.
2. **Option B: Build-time stats** — Add a prebuild script that fetches stats from the GitLab API and writes them to a JSON file consumed by the site at build time. Stats update on every site deploy.
3. **Option C: Client-side fetch** — Fetch from `https://gitlab.com/api/v4/projects/{id}` on page load. Simple but has CORS considerations and rate limits.

**Recommendation:** Option B (build-time) gives the best balance — stats are always current on deploy, no client-side API calls, no CORS issues, no rate limits for visitors.

### Step 3.1: Create Build-Time Stats Fetcher

- [x] Create `site/scripts/fetch-repo-stats.mjs` — Script that fetches project stats from the GitLab public API and writes to `site/app/repo-stats.json`:

```javascript
// Fetches: star_count, forks_count, latest release tag_name
// Writes JSON: { stars: N, forks: N, version: "vX.Y.Z", pipeline: "success" }
// Called during site build (prebuild hook in package.json)
```

- [x] Update `site/package.json` — Add `"prebuild": "node scripts/fetch-repo-stats.mjs"` before the generate script

### Step 3.2: Create RepoStats Component

- [x] Create `site/app/components/RepoStats.vue` — Compact inline stats display:
  - GitLab icon + "starshadow/capacitarr" (linked to repo)
  - Tag icon + version (from build-time JSON)
  - Star icon + count
  - Fork icon + count
  - Styled to match header (small, muted text, hover effects)
  - Graceful fallback if stats file is missing (component renders nothing)

### Step 3.3: Integrate into Header

- [x] `site/app/components/AppHeader.vue` — Add `<RepoStats />` in the `#right` template slot, before the theme toggle and GitLab link button. Replace the existing standalone GitLab icon button since the stats widget already links to the repo.

---

## Phase 4: Miscellaneous Site Cleanup

**Goal:** Fix minor issues found during audit.
**Risk:** 🟢 Low
**Status:** ✅ Complete

### Step 4.1: Remove "Nuxt UI Docs" Link from TOC

The TOC sidebar bottom links to "Nuxt UI Docs" — this is a leftover from the template and not useful for Capacitarr users.

- [x] `site/app/app.config.ts:42-49` — Remove or replace the `toc.bottom.links` entry. Replace with a link to the GitLab repo or remove entirely.

### Step 4.2: Verify Synced Docs Accuracy

The `site/scripts/sync-docs.mjs` copies these docs to the site:
- `index.md`, `quick-start.md`, `deployment.md`, `configuration.md`, `scoring.md`, `releasing.md`
- `api/README.md`, `api/examples.md`, `api/workflows.md`, `api/versioning.md`
- `CHANGELOG.md`

After Phase 1 changes, run `node scripts/sync-docs.mjs` (or the build) to verify all synced docs reflect the fixes.

- [x] Run site build/sync and verify `content/docs/quick-start.md` has the updated disk group and series status wording
- [x] Spot-check other synced docs for any remaining inaccuracies

---

## Phase 5: Full Regression Audit

**Goal:** Systematic review of every content surface on the site to catch any remaining issues.
**Risk:** 🟢 Low — read-only audit that may produce additional fix items.
**Status:** ✅ Complete — No additional issues found beyond those already fixed in Phases 1–4.

### Step 5.1: Site Landing Page Audit

Review every text string on `site/app/pages/index.vue`:

- [x] Hero section — tagline, description, button labels
- [x] Features section (6 cards) — titles and descriptions match actual capabilities
- [x] Stats section (`AnimatedStats.vue`) — "9+ Integrations", "6 Scoring Dimensions", "100% Open Source", "0 Required Cloud Services" — verify all numbers are current
- [x] Comparison section (`ComparisonSection.vue`) — "before/after" claims are accurate
- [x] FAQ section (`FaqSection.vue`) — all 6 Q&A pairs are factually correct (disk groups and availability already fixed in Phase 1)
- [x] About section — license, tech stack (already fixed in Phase 1)
- [x] How-it-works section — 3 steps are accurate
- [x] Terminal animation — commands shown are correct
- [x] CTA section — links work

### Step 5.2: Documentation Content Audit

Review each doc file for accuracy against the current codebase:

- [x] `docs/index.md` — overview, steps, links
- [x] `docs/quick-start.md` — all steps match actual UI flow (already partially fixed in Phase 1)
- [x] `docs/configuration.md` — all env vars are current, defaults are correct
- [x] `docs/deployment.md` — reverse proxy examples, env var table
- [x] `docs/scoring.md` — factor names, raw score tables, rule effects, tiebreaker methods
- [x] `docs/api/README.md` — API overview
- [x] `docs/api/examples.md` — API examples match current endpoints
- [x] `docs/api/workflows.md` — workflow descriptions match current behavior
- [x] `docs/api/versioning.md` — versioning policy

### Step 5.3: README Audit

- [x] `README.md` — features list, configuration table, architecture diagram, scoring factors table, project structure, all links

---

## File Change Matrix

| File | Phase | Action | Description |
|------|-------|--------|-------------|
| `site/app/pages/index.vue` | 1.1 | Modify | "Nuxt 3" → "Nuxt 4" |
| `frontend/app/pages/help.vue` | 1.1, 2.2 | Modify | "Nuxt 3" → "Nuxt 4" + Maintainerr credit |
| `docs/quick-start.md` | 1.2, 1.4 | Modify | Rewrite disk group step + "Availability" → "Series Status" |
| `site/app/components/FaqSection.vue` | 1.3, 1.4 | Modify | Fix disk groups Q&A + "availability" → "series status" |
| `README.md` | 1.4 | Modify | "availability" → "series status" in intro |
| `docs/index.md` | 1.5 | Modify | Reword disk group step |
| `CONTRIBUTORS.md` | 2.1 | Modify | Add Credits & Acknowledgments section with Maintainerr |
| `site/scripts/fetch-repo-stats.mjs` | 3.1 | **Create** | Build-time GitLab stats fetcher |
| `site/package.json` | 3.1 | Modify | Add prebuild script |
| `site/app/components/RepoStats.vue` | 3.2 | **Create** | GitLab repo stats display component |
| `site/app/components/AppHeader.vue` | 3.3 | Modify | Integrate RepoStats component |
| `site/app/app.config.ts` | 4.1 | Modify | Remove/replace "Nuxt UI Docs" TOC link |

---

## Execution Order

| Phase | Risk | Effort | Priority | Status | Description |
|-------|------|--------|----------|--------|-------------|
| Phase 1 | 🟢 Low | 🟢 Low | **P1** | ✅ | Content accuracy fixes |
| Phase 2 | 🟢 Low | 🟢 Low | **P2** | ✅ | Credits & Maintainerr acknowledgment |
| Phase 4 | 🟢 Low | 🟢 Low | **P3** | ✅ | Misc site cleanup |
| Phase 3 | 🟡 Medium | 🟡 Medium | **P4** | ✅ | GitLab repo stats widget |
| Phase 5 | 🟢 Low | 🟡 Medium | **P5** | ✅ | Full regression audit |

---

## Success Criteria

- [x] No "Nuxt 3" references in project source files (excluding node_modules, plan docs)
- [x] No instructions telling users to "create" disk groups
- [x] No "availability" as a scoring factor name (all converted to "series status")
- [x] Maintainerr credited in CONTRIBUTORS.md and frontend help page
- [x] GitLab repo stats visible in site header
- [x] Site build completes successfully after all changes
- [x] All synced docs reflect the updated content
