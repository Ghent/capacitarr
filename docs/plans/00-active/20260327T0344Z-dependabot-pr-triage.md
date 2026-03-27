# Dependabot PR Triage — First Post-Migration Batch

**Status:** 🟡 Planned
**Created:** 2026-03-27
**Scope:** Dependencies (Go backend, docs site)

## Overview

Two Dependabot PRs opened immediately after the GitHub migration. Both need rebasing (they target stale `main` SHAs from before the migration cleanup commits) and CI verification before merging.

## PR #2 — Go Backend Dependencies (Low Risk)

**PR:** [chore(deps): bump the go-minor-patch group in /backend with 3 updates](https://github.com/Ghent/capacitarr/pull/2)

| Package | From | To | Type |
|---------|------|----|------|
| `golang.org/x/crypto` | 0.48.0 | 0.49.0 | minor |
| `golang.org/x/sync` | 0.19.0 | 0.20.0 | minor |
| `golang.org/x/time` | 0.14.0 | 0.15.0 | minor |

**Risk:** Low — Go standard library extensions, minor version bumps only.

### Steps

1. [ ] Comment `@dependabot rebase` on PR #2 to update against current `main`
2. [ ] Wait for CI to run on the rebased PR
3. [ ] Review CI results — all lint, test, build, and security jobs must pass
4. [ ] Merge PR #2

## PR #11 — Docs Site Dependencies (Medium Risk)

**PR:** [chore(deps): bump the site-all group across 1 directory with 7 updates](https://github.com/Ghent/capacitarr/pull/11)

| Package | From | To | Type | Risk |
|---------|------|----|------|------|
| `@nuxt/ui` | 4.5.1 | 4.6.0 | minor | ⚠️ Has breaking change |
| `@nuxtjs/sitemap` | 8.0.6 | 8.0.7 | patch | Low |
| `mermaid` | 11.12.3 | 11.13.0 | minor | Low |
| `nuxt` | 4.3.1 | 4.4.2 | minor | Low |
| `tailwindcss` | 4.2.1 | 4.2.2 | patch | Low |
| `@types/node` | 25.3.3 | 25.5.0 | minor | Low |
| `better-sqlite3` | 12.6.2 | 12.8.0 | minor | Low |

**Risk:** Medium — `@nuxt/ui` 4.6.0 includes a breaking change: `module: use moduleDependencies to manipulate options` ([#5384](https://github.com/nuxt/ui/issues/5384)). The other 6 packages are safe minor/patch bumps.

### Steps

1. [ ] Review the `@nuxt/ui` 4.6.0 breaking change — check if the site's `nuxt.config.ts` or any component uses the old `moduleDependencies` pattern
2. [ ] Comment `@dependabot rebase` on PR #11 to update against current `main`
3. [ ] Wait for CI to run on the rebased PR
4. [ ] Check the Cloudflare Pages preview deployment — verify the site renders correctly with the new dependencies
5. [ ] If the `@nuxt/ui` breaking change causes build failures:
   - Close PR #11
   - Manually update the 6 safe packages in a separate branch
   - Address `@nuxt/ui` 4.6.0 migration in a dedicated PR
6. [ ] If everything passes, merge PR #11

## Merge Order

1. **PR #2 first** — Go deps have no site impact; merging first keeps the dependency graph clean
2. **PR #11 second** — Site deps need the `@nuxt/ui` breaking change review

## Security Context

The `make ci` security stage currently fails due to `node-forge` (4 HIGH CVEs) and `happy-dom` (1 HIGH CVE) in the frontend. These are transitive dependencies — `node-forge` comes from `nuxt > @nuxt/cli > listhen`, and `happy-dom` is a dev dependency. Neither PR addresses these directly, but PR #11's `nuxt` 4.3.1 → 4.4.2 bump may resolve the `node-forge` issue if `listhen` was updated in the dependency tree.
