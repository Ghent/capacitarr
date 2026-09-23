# Plans

`00-active/` is the inbox: open work, backlog, and in-progress plans.
`ls docs/plans/00-active` is “what is in flight.”

## Lifecycle

1. New work → `docs/plans/00-active/<timestamp>-<slug>.md` on Capacitarr.
2. Ship → copy any still-true constraint into `docs/reference/`, `docs/development.md`, or a code comment. Do not leave the journal as the only home of a product invariant.
3. Close → mark Status Complete, copy the file to the private archive in the matching category folder, then delete it from this repo.
4. Supersede → write a new active plan. Do not edit an archived journal to “keep it current.”

## Archive

Historical plans (Complete / Closed / Superseded / Won’t Do / Rolled back) live in the private repo [Ghent/capacitarr-plans](https://github.com/Ghent/capacitarr-plans). That repo is **private, not a product, and not licensed for use**. The docs site does not publish this directory.

Do not add finished journals back onto `main`.

## Marketing screenshots

New marketing shots are authored as WebP under `site/public/screenshots/`. Do not commit a root `screenshots/` PNG tree.
