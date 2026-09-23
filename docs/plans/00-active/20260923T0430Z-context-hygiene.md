# Context Hygiene — Archive Plans, Ignore Noise, Drop PNG Duplicates

**Status:** Planned
**Priority:** High (agent correctness + clone/index noise; no product behavior change)
**Estimated Effort:** S–M (mostly mechanical; one human gate to create the archive repo)
**Origin:** Repo-size / token review (2026-09-23). Implements item #1 (context hygiene) only.
**Branch:** `cursor/context-hygiene-plan-210c`

---

## Summary

Finished design journals currently live on `main` next to the product. There are **143** plan files (~47k lines, ~550k tokens). Only **6** sit in `docs/plans/00-active/`. The rest are Complete / Closed / Superseded / Won’t Do / Rolled back, but every clone and every agent still treats them as current spec.

This plan does three things, and only these three:

1. **Move completed plans to a separate, private, all-rights-reserved archive repo.** Not an orphan branch. Not a public repo. Not Capacitarr’s PolyForm Noncommercial license.
2. **Add `.cursorignore`** so lockfiles, extra locales, changelog, screenshots, and the marketing site stop entering the default agent index.
3. **Delete the unused root `screenshots/*.png` duplicates** (the site and README already use `site/public/screenshots/*.webp`).

No scoring, deletion, API, or UI behavior changes.

---

## Locked decisions

These are already decided. Do not re-litigate them in implementation.

| Decision | Choice | Why |
|----------|--------|-----|
| Archive location | **Separate GitHub repo**, not an orphan branch | Stronger wall: a Capacitarr clone never fetches the journals unless someone also clones the archive |
| Archive visibility | **Private** | Internal work journals. Not a second public product |
| Archive licensing | **All rights reserved. Not open source. Not free to use.** | Do **not** copy `LICENSE`, `CONTRIBUTING.md` (CLA), or PolyForm Noncommercial from Capacitarr. Do **not** add MIT / Apache / GPL / PolyForm / CC. GitHub “Add a license” must stay unchecked |
| What stays on `main` | `docs/plans/00-active/` + a short `docs/plans/README.md` | Inbox only. `ls docs/plans/00-active` is “what is in flight” |
| What moves | Category folders `01`–`10` (137 files) | Finished journals |
| History rewrite | **No** | Old commits on `capacitarr` keep the files. That is fine. We only change the current tree |
| This plan’s scope | Context hygiene only | Type generation, preview pagination, auth roles, etc. are other items |

### Archive repo constraints (non-negotiable)

When the archive repo is created (human step — agents in this environment cannot create GitHub repos):

- Owner: `Ghent` (same as Capacitarr).
- Proposed name: `capacitarr-plans` (change the name if it collides; do not change privacy or license).
- **Private.** If the create UI or `gh repo create` defaults to public, stop and fix it before the first push.
- **No license file. No license template.** Root `LICENSE` in the archive should be a short proprietary notice, not an OSI/PolyForm license. Suggested text:

  ```text
  Copyright (c) Starshadow Studios / Ghent Starshadow. All rights reserved.

  This repository is private. It is not open source. It is not licensed
  for use, copy, modification, or distribution by anyone other than the
  copyright holder. No license is granted, including Capacitarr's
  PolyForm Noncommercial 1.0.0 license.
  ```

- **Do not** enable GitHub Pages, a public website, or “Use this template.”
- **Do not** copy Capacitarr’s `LICENSE`, `CONTRIBUTING.md`, `FUNDING.yml`, or user-facing docs into the archive.
- Issues / wiki / discussions: off unless you later want them for private tracking. Default off.
- Description on GitHub: “Private plan archive for Capacitarr. Not a product. Not licensed for use.” — not “open source media manager.”

A public or OSS-licensed archive is a failed implementation of this plan, even if every file moved correctly.

---

## Current state

### Plan tree on `main` (2026-09-23)

| Path | Files | Role after this plan |
|------|------:|----------------------|
| `docs/plans/00-active/` | 6 | **Stays** (inbox; curate — see below) |
| `docs/plans/01-architecture/` | 16 | Move to archive |
| `docs/plans/02-features/` | 51 | Move to archive |
| `docs/plans/03-ui-ux/` | 15 | Move to archive |
| `docs/plans/04-bugfixes/` | 16 | Move to archive |
| `docs/plans/05-integrations/` | 3 | Move to archive |
| `docs/plans/06-infrastructure/` | 13 | Move to archive |
| `docs/plans/07-audits/` | 11 | Move to archive |
| `docs/plans/08-backend/` | 4 | Move to archive |
| `docs/plans/09-documentation/` | 7 | Move to archive |
| `docs/plans/10-wont-do/` | 1 | Move to archive |

Site publish already excludes `docs/plans/` (`site/scripts/sync-docs.mjs`). Users never see these files. Agents and clones always do. There is no `docs/plans/README.md`.

### `00-active/` inbox (curate in Phase 1)

| File | Header today | Action |
|------|--------------|--------|
| `20260414T0153Z-sportarr-integration.md` | Planned | Stay |
| `20260408T1335Z-context-propagation.md` | Planned | Stay |
| `20260408T1336Z-frontend-architecture-polish.md` | Planned | Stay |
| `20260413T1843Z-v4-schema-fixup-removal.md` | Pending (v4 prerequisite) | Stay |
| `20260923T0430Z-context-hygiene.md` | Planned (this file) | Stay until this work is done, then archive |
| `20260413T0205Z-approval-queue-visibility-fix.md` | ✅ Complete | **Move to archive** (`04-bugfixes/`) |
| `20260317T1323Z-absolute-byte-thresholds.md` | ⏸️ Deferred to post-2.0 | **Decision:** product is well past 2.0. Either retitle to `Backlog` and keep, or archive as deferred. Default if nobody picks: **keep as Backlog** so the inbox stays honest |

### Citations that will break if we only delete

| Location | What to do |
|----------|------------|
| `backend/internal/db/validation.go` (~L45) | Replace plan path with one sentence: Overseerr was renamed to Seerr in 2.0 |
| `SECURITY.md` Gitleaks table | After the move, `docs/plans/` on `main` is inbox-only. Keep the allowlist (active plans can still have example credentials). Add a line that the private archive is out of this repo’s scan |
| `.gitleaks.toml` | Keep `docs/plans/` allowlist for the inbox |
| `site/scripts/sync-docs.mjs` | Keep excluding `plans`. Update the “see docs/plans/ for conversion” comment to point at the README, not a conversion process that does not exist |

Do **not** leave 20 comments that 404. One real citation is enough to fix. Intra-plan “Supersedes:” links live in the archive and can stay relative.

### Screenshots

| Path | Size | Used by |
|------|------|---------|
| `screenshots/*.png` (7 files) | ~21 MB | **Nothing** in product or README. Only `SECURITY.md` Semgrep skip table |
| `site/public/screenshots/*.webp` (7 files) | ~8 MB | README hero, `site/app/components/ScreenshotGallery.vue`, site content |

Root PNGs are authoring leftovers from the WebP conversion. Delete them. Do not recreate `screenshots/` on `main`. New marketing shots go straight to `site/public/screenshots/*.webp`.

### Token / clone noise this plan is meant to cut

Naive tokenize-the-checkout is ~2.0–2.3M tokens. Plans are ~550k of that. Lockfiles ~233k. Extra locales ~185k. Root PNGs are 21 MB on disk (not tokens) and make the repo *feel* huge.

After this plan, the default working tree should drop the 137 journals and the PNGs. `.cursorignore` hides the rest from agents without deleting files the build still needs.

---

## Non-goals

- Orphan branch (rejected).
- Public archive, dual-license, or “same license as Capacitarr.”
- Rewriting 137 journals into ADRs.
- Rewriting git history on `capacitarr`.
- Generating TS types from OpenAPI (item #4).
- Preview pagination, event-bus, or auth-role work (items #2–#3).
- Adding `docs/plans/` to `.gitignore` (the inbox must stay tracked).
- Deleting `CHANGELOG.md` or `site/` from git (ignore for agents; keep for humans and the docs site).

---

## Design

### Two repos, two jobs

```text
Ghent/capacitarr          (public product — unchanged license)
  docs/plans/README.md
  docs/plans/00-active/   ← only open / backlog work
  .cursorignore
  site/public/screenshots/*.webp
  (no root screenshots/, no 01–10 plan folders)

Ghent/capacitarr-plans    (PRIVATE, all rights reserved, not a product)
  README.md               ← “private archive, not licensed for use”
  LICENSE                 ← proprietary notice only
  01-architecture/
  02-features/
  …
  10-wont-do/
  (optional) 04-bugfixes/20260413T0205Z-approval-queue-visibility-fix.md
```

`main` never vendors the archive. A README link is enough. Agents working in Capacitarr will not see the journals unless someone also adds that private repo to the workspace.

### Lifecycle after this ships

1. New work → `docs/plans/00-active/<timestamp>-<slug>.md` on Capacitarr.
2. Ship → copy any *still-true* constraint into `docs/reference/`, `docs/development.md`, or a code comment.
3. Close → mark Status Complete, `git mv` is **not** enough. Copy the file to the private archive in the matching category folder, then delete it from Capacitarr.
4. Supersede → write a new active plan. Do not edit the archived journal to “keep it current.”

### `.cursorignore`

Gitignore syntax. Proposed contents:

```gitignore
# Lockfiles — needed by CI, useless as source
**/pnpm-lock.yaml
**/package-lock.json
**/go.sum

# i18n copies — en.json is the source of truth
frontend/app/locales/*
!frontend/app/locales/en.json

# Release journal — @-mention when doing a release
CHANGELOG.md

# Unused after Phase 5; keep the rule so they do not come back
screenshots/

# Marketing site is a separate app
site/
```

After category folders leave `main`, there is nothing extra to ignore under `docs/plans/`. Do not ignore `docs/reference/`, `docs/guides/`, `SECURITY.md`, or `00-active`.

---

## Implementation

### Phase 0 — Create the private archive repo (human)

Blocked on Ghent. Agents in this environment cannot create GitHub repos.

- [ ] Create `Ghent/capacitarr-plans` as **private**
- [ ] Confirm the GitHub UI shows **Private** (lock) before any push
- [ ] Do **not** add a license template, `.gitignore` template that implies OSS, or public README badges
- [ ] Push an initial commit: proprietary `LICENSE` (text above) + `README.md` that states private / not open source / not licensed for use / not the Capacitarr product
- [ ] Grant access only to people who should read internal plans (default: just you)
- [ ] Record the canonical URL in Capacitarr’s `docs/plans/README.md` in Phase 3 (use the private GitHub URL; do not advertise it in the public README root unless you want it visible — **prefer linking only from `docs/plans/README.md`**, which the site already does not publish)

### Phase 1 — Curate `00-active` on Capacitarr

- [ ] Move `20260413T0205Z-approval-queue-visibility-fix.md` with the category folders (Complete; does not belong in the inbox)
- [ ] Absolute byte thresholds: keep as `Backlog` **or** include in the archive move (default: keep)
- [ ] Leave the four still-open plans + this hygiene plan in `00-active`

### Phase 2 — Promote still-true facts; fix citations

Do this **before** deleting files from `main`, so we do not need the archive to understand the product.

- [ ] `validation.go`: drop the plan path; keep the Seerr rename in one sentence
- [ ] Skim `docs/development.md` and `docs/reference/architecture.md` — if a closed plan is the only place a still-true invariant lives, lift that paragraph now. Do not lift implementation checklists
- [ ] Known candidate already lifted: per-disk-group execution mode vs `DefaultDiskGroupMode` (already in `docs/development.md`). No action unless something else is similarly stranded

### Phase 3 — Seed the archive, then remove from `main`

- [ ] Copy folders `01`–`10` (and the Complete approval-queue plan) into the private repo, **preserving relative paths** so intra-plan links keep working
- [ ] Do **not** copy `00-active/` as a whole (those files are still live). Exception: the Complete approval-queue file goes to `04-bugfixes/`
- [ ] Do **not** copy Capacitarr `LICENSE`, `CONTRIBUTING.md`, screenshots, or user docs
- [ ] Archive `README.md` lists the category folders and repeats the proprietary notice
- [ ] Push to the **private** remote; re-check the repo is still private
- [ ] On Capacitarr: `git rm -r` the moved folders (`01`–`10`)
- [ ] Add `docs/plans/README.md`:

  - What `00-active` is
  - How to open / close a plan (lifecycle above)
  - That historical plans live in the private archive (name + that it is private / not a product license)
  - Do not paste plan bodies into the public README

### Phase 4 — `.cursorignore`

- [ ] Add the file as specified under Design
- [ ] Confirm `frontend/app/locales/en.json` is un-ignored (`!` exception)
- [ ] Confirm `docs/`, `backend/`, `frontend/app/` (except extra locales) stay indexed

### Phase 5 — Drop root PNG duplicates

- [ ] `git rm screenshots/*.png` (and the directory if empty)
- [ ] Update `SECURITY.md` Semgrep skip table: remove the `screenshots/*.png` rows; the 1 MB skip no longer applies to those files
- [ ] Confirm README and site gallery still point at `site/public/screenshots/*.webp` only
- [ ] One-line note in `docs/plans/README.md` or `docs/development.md`: new marketing shots are authored as WebP under `site/public/screenshots/`. Do not commit a root `screenshots/` PNG tree

### Phase 6 — Housekeeping on Capacitarr

- [ ] `.gitleaks.toml`: keep `docs/plans/` allowlist; comment that it covers the inbox only
- [ ] `SECURITY.md`: Gitleaks row stays; Semgrep screenshot row goes (Phase 5)
- [ ] `site/scripts/sync-docs.mjs`: keep `EXCLUDED_DIRS = plans`; fix the stale “conversion process” / `.kilocoderules` comments
- [ ] `CONTRIBUTING.md`: short “Internal plans” note — active plans in `docs/plans/00-active/`, completed plans go to the private archive, do not add finished journals back onto `main`
- [ ] Mark this plan Complete only after Phases 0–6 and Verify. Then copy it to the archive and remove it from `00-active` (dogfood the lifecycle)

### Verify

- [ ] `Ghent/capacitarr-plans` (or chosen name) shows **Private** on GitHub
- [ ] Archive `LICENSE` is proprietary / all rights reserved — grep the archive for `PolyForm`, `MIT`, `Apache`, `GPL` and expect no license grants
- [ ] Capacitarr `main` working tree has no `docs/plans/01-*` … `10-*`
- [ ] `docs/plans/00-active/` still has the open plans
- [ ] `git grep docs/plans/01` (and `02`–`10`) on Capacitarr `HEAD` returns no product-code citations (comments already rewritten)
- [ ] Root `screenshots/` is gone; `site/public/screenshots/*.webp` remain; README image still renders
- [ ] `.cursorignore` exists and does not exclude `backend/` or `docs/reference/`
- [ ] Site sync still skips `docs/plans/`
- [ ] `make ci` (or at least lint/test that do not need a full frontend embed) still passes — this change is docs + ignore + image delete
- [ ] Optional: re-run the token breakdown. Expect plans category on `main` to fall from ~550k tokens to `00-active` + README only

---

## Risk

| Risk | Mitigation |
|------|------------|
| Archive created as public by default | Phase 0 checklist; abort if the lock icon is missing |
| Archive inherits Capacitarr’s LICENSE via copy-paste | Explicit do-not-copy; proprietary LICENSE in the first commit |
| Public Capacitarr README advertises the private repo | Link only from `docs/plans/README.md` (unpublished by the site) |
| Agent implements a stale audit because someone added the private repo to the workspace | Archive README: “historical journals, not current spec.” Capacitarr `.cursorignore` cannot help a second checkout |
| Intra-plan links break | Preserve directory layout `01`–`10` |
| Still-true invariant only lived in a deleted plan | Phase 2 lift before `git rm` |
| Contributors put completed plans back on `main` | Lifecycle in `docs/plans/README.md` + `CONTRIBUTING.md` |

---

## Suggested implementation order

Phase 0 (human, private repo) can happen in parallel with Phase 2 (citation cleanup) and Phase 4–5 (ignore + PNGs), which do not need the archive. Phase 3 (`git rm` of journals) waits on Phase 0 + 1 + 2.

Prefer one Capacitarr PR for Phases 1–6 once the private repo exists, or two PRs: (A) ignore + PNG + citation + README, (B) `git rm` of `01`–`10` after the archive push is verified private.
