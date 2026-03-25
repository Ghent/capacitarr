# CI/CD Polish and Cleanup

**Status:** ✅ Complete (except Phase 4 — manual GitLab UI cleanup)

**Prerequisite:** [Build, CI/CD, and Publishing Overhaul](20260303T1551Z-build-cicd-publishing.md) — ✅ Complete

---

## Overview

Follow-up items discovered during the CI/CD overhaul implementation. These are polish, optimization, and documentation fixes — the core pipeline is functional.

---

## Phase 1: CI Pipeline Fixes

### 1.1 Fix `build:docker` warning — ✅ Complete

The `build:docker` smoke test job produces: `WARNING: No output specified with docker-container driver`. Add `--output=type=cacheonly` to explicitly declare the build is verification-only.

**File:** [`.gitlab-ci.yml`](../../.gitlab-ci.yml)

```yaml
build:docker:
  script:
    - docker buildx build --platform linux/amd64,linux/arm64 --output=type=cacheonly .
```

### 1.2 Add `interruptible: true` to non-release CI jobs — ✅ Complete

When multiple pushes happen quickly, older pipelines waste CI minutes. Adding `interruptible: true` to lint/test/build/security jobs allows GitLab to auto-cancel superseded pipelines.

**File:** [`.gitlab-ci.yml`](../../.gitlab-ci.yml)

### 1.3 Fix duplicate pipeline on tag+branch push — ✅ Complete

`git push origin main --tags` creates two pipelines (branch + tag). Implemented both options:
- **Option A:** Documented two-step push in `docs/releasing.md`
- **Option B:** Added `workflow.auto_cancel.on_new_commit: interruptible` to `.gitlab-ci.yml`

**Files:** [`.gitlab-ci.yml`](../../.gitlab-ci.yml), [`docs/releasing.md`](../releasing.md)

---

## Phase 2: Changelog Improvements

### 2.1 Fix broken changelog link on release page — ✅ Complete

The GoReleaser release header links to `https://capacitarr.app/changelog`, but the actual site route is `/docs/changelog` (the sync script writes `CHANGELOG.md` into `content/docs/changelog.md`).

**Fix:** Update the URL in `.goreleaser.yml` from `/changelog` to `/docs/changelog`.

**File:** [`.goreleaser.yml`](../../.goreleaser.yml)

### 2.2 Slim down changelog output — ✅ Complete

The changelog currently includes every conventional commit type. Trimmed to user-facing changes only.

**Keep in changelog:**
- 🚀 Features (`feat`)
- 🐛 Bug Fixes (`fix`)
- ⚡ Performance (`perf`)
- 🛡️ Security (commits with "security" in body)
- ◀️ Revert (`revert`)

**Skip from changelog (add `skip = true`):**
- 📚 Documentation (`docs`)
- 🚜 Refactor (`refactor`)
- ⚙️ Miscellaneous Tasks (`chore`)

**File:** [`cliff.toml`](../../cliff.toml) — update `commit_parsers` to add `skip = true` for `docs`, `refactor`, and `chore`

---

## Phase 3: Documentation Updates

### 3.1 Fix inaccurate stack references across documentation — ✅ Complete

The project uses Nuxt 4 (`nuxt: ^4.3.1`), Vue 3 (`vue: ^3.5.29`), Tailwind CSS 4 (`tailwindcss: ^4.1.18`). Multiple docs reference outdated stack info.

**Audit results:**

[`README.md`](../../README.md):
- Line 19: `Nuxt 3 frontend` → `Nuxt 4 frontend`
- Line 108: `Nuxt 3 (Vue 3) frontend` → `Nuxt 4 (Vue 3) frontend`
- Line 113 (Mermaid): `Nuxt 3 Frontend` → `Nuxt 4 Frontend`, `Tailwind CSS` → `Tailwind CSS 4`
- Line 151 (table): `Nuxt 3` → `Nuxt 4`
- Line 200: `Nuxt 3 frontend` → `Nuxt 4 frontend`
- Line 217: `# Nuxt 3 frontend` → `# Nuxt 4 frontend`
- Line 224: `docs/  # Documentation (MkDocs)` → `docs/  # Documentation`
- Line 242: `powered by MkDocs` → `powered by Nuxt Content` (site is now Nuxt Content + Nuxt UI Pro)

[`docs/index.md`](../index.md) — ✅ No inaccurate references

### 3.2 Update releasing docs for two-step push — ✅ Complete

Documented that `git push origin main --tags` creates duplicate pipelines, and updated to recommend the two-step approach:

```bash
git push origin main        # branch pipeline (lint/test/build/security/pages)
git push origin vX.Y.Z      # tag pipeline (full release)
```

**File:** [`docs/releasing.md`](../releasing.md)

---

## Phase 4: Cleanup

### 4.1 Clean up stale tags and container images

Delete via GitLab web UI:
- **Tags:** `v0.1.0`, `v0.1.1` (created during CI debugging, point to intermediate commits)
- **Container images:** Any images tagged `0.1.0`, `0.1.1`, `0.1.2` (from failed/partial release pipelines)
- **Releases:** Delete any auto-created releases from the old release job or failed goreleaser runs

---

## Files to Modify

| File | Phase | Description |
|------|-------|-------------|
| `.gitlab-ci.yml` | 1.1, 1.2, 1.3 | Add `--output=type=cacheonly`, `interruptible: true`, pipeline dedup |
| `.goreleaser.yml` | 2.1 | Fix broken changelog link (`/changelog` → `/docs/changelog`) |
| `cliff.toml` | 2.2 | Skip docs/refactor/chore from changelog |
| `README.md` | 3.1 | Fix Nuxt 3 → Nuxt 4 (6 occurrences), MkDocs → Nuxt Content (2 occurrences) |
| `docs/releasing.md` | 3.2 | Document two-step push workflow |
