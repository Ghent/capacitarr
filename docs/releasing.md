# Release Workflow

Capacitarr uses a tag-triggered release pipeline powered by [git-cliff](https://git-cliff.org/), [GoReleaser](https://goreleaser.com/), and GitHub Actions. Releases follow [Semantic Versioning](https://semver.org/) and are driven by [Conventional Commits](https://www.conventionalcommits.org/).

## How It Works

### Tag-Triggered Releases

Releases are created when you push a `v*` tag to the repository. The CI pipeline then:

1. **Extracts release notes** — `git cliff --latest --strip header` generates notes for the tagged version
2. **Builds cross-compiled binaries** — GoReleaser compiles `linux/amd64` and `linux/arm64` binaries with the frontend SPA embedded
3. **Creates a GitHub release** — with binary archives and checksums attached as downloadable assets
4. **Pushes Docker images** — multi-arch images (`linux/amd64` + `linux/arm64`) to GHCR
5. **Rebuilds the project site** — the `pages` job picks up the committed changelog

### On Every Push and MR

The standard CI pipeline runs on every push and merge request:

- **Lint** — `golangci-lint` + ESLint
- **Test** — Go tests + frontend tests
- **Security** — `govulncheck` + `pnpm audit` + Trivy (filesystem + image) + Gitleaks + Semgrep
- **Build** — Docker multi-arch smoke test (build only, no push)

## Release Workflow

### Step-by-Step

```bash
# 1. Run the full CI pipeline locally (must pass before releasing)
make ci

# 2. Determine the next version
git cliff --bumped-version                    # e.g., v0.2.0

# 3. Generate the full changelog
git cliff --bump -o CHANGELOG.md

# 4. Update package.json version (strip the 'v' prefix)
VERSION=$(git cliff --bumped-version)
SEMVER=${VERSION#v}
npm version "$SEMVER" --no-git-tag-version
cd frontend && npm version "$SEMVER" --no-git-tag-version && cd ..

# 5. Commit and tag
git add CHANGELOG.md package.json package-lock.json frontend/package.json
git commit -m "chore(release): $VERSION"
git tag "$VERSION"

# 6. Push (two-step to avoid duplicate pipelines)
git push origin main        # branch pipeline: lint/test/build/security/pages
git push origin "$VERSION"  # tag pipeline: full release (binaries + Docker + GitLab release)
```

> **Why two steps?** Running `git push origin main --tags` in a single command causes GitLab to create **two separate pipelines** — one for the branch push and one for the tag push — which wastes CI minutes. Pushing them separately ensures only one pipeline runs at a time.

### Convenience Script

There is a convenience script in the root `package.json`:

```bash
npm run release
```

This runs the full CI pipeline (`make ci`) first, then performs the release flow (changelog generation, version bump, commit, and tag). If CI fails, the release is aborted. You still need to push afterward:

```bash
git push origin main
git push origin vX.Y.Z
```

## Semantic Versioning

Version numbers follow the `MAJOR.MINOR.PATCH` format. The version bump is determined automatically from commit messages:

| Commit Type | Version Bump | Example |
|-------------|-------------|---------|
| `feat:` | **MINOR** (0.1.0 → 0.2.0) | `feat: add disk group filtering` |
| `fix:` | **PATCH** (0.1.0 → 0.1.1) | `fix: correct capacity calculation` |
| `docs:` | **PATCH** (0.1.0 → 0.1.1) | `docs: update deployment guide` |
| `refactor:` | **PATCH** (0.1.0 → 0.1.1) | `refactor: simplify poller logic` |
| `perf:` | **PATCH** (0.1.0 → 0.1.1) | `perf: optimize database queries` |
| `chore:` | **PATCH** (0.1.0 → 0.1.1) | `chore: update dependencies` |
| Any type with `BREAKING CHANGE:` footer or `!` | **MAJOR** (0.1.0 → 1.0.0) | `feat!: redesign API endpoints` |

### Skipped Types

The following commit types are excluded from the changelog but still count toward version determination:

- `docs` — documentation changes
- `refactor` — code refactoring
- `chore` — maintenance tasks (including `chore(release)`)
- `test` — test additions/changes
- `ci` — CI/CD pipeline changes
- `style` — code style/formatting changes
- `build` — build system changes

## Version Display

Version information flows to the UI through two paths:

1. **Backend (API version)** — injected via `-ldflags` at build time into `main.go` variables (`version`, `commit`, `buildDate`). Exposed at `GET /api/v1/version`. Displayed in the navbar as "API vX.Y.Z" and on the help page.

2. **Frontend (UI version)** — read from `frontend/package.json` at build time via `nuxt.config.ts` → `runtimeConfig.public.appVersion`. Displayed in the navbar as "UI vX.Y.Z" and on the help page.

Both `package.json` files must be updated during the release prep step for the UI to display the correct version.

## git-cliff Configuration

The changelog is configured in [`cliff.toml`](../cliff.toml) at the project root. Key settings:

- **Conventional commits parsing** — only conventional commit messages are included
- **Commit grouping** — user-facing commits are organized by type with emoji prefixes:
  - 🚀 Features (`feat`)
  - 🐛 Bug Fixes (`fix`)
  - ⚡ Performance (`perf`)
  - 🛡️ Security (commits with "security" in the body)
  - ◀️ Revert (`revert`)
- **Skipped from changelog** — `docs`, `refactor`, `chore`, `test`, `ci`, `style`, `build`
- **Commit links** — each changelog entry links to the commit on GitHub
- **Sorted oldest-first** — commits within each group appear in chronological order

## CI Pipeline Jobs

### On Every Push and MR

| Job | Stage | Image | Purpose |
|-----|-------|-------|---------|
| `lint:go` | lint | `golangci/golangci-lint:latest` | Go linting |
| `lint:frontend` | lint | `node:22-alpine` | ESLint |
| `test:go` | test | `golang:1.26-alpine` | Go tests with race detector |
| `test:frontend` | test | `node:22-alpine` | Frontend tests |
| `build:docker` | build | `docker:latest` | Multi-arch Docker smoke test (no push) |
| `security:govulncheck` | security | `golang:1.26-alpine` | Go vulnerability check |
| `security:pnpm-audit` | security | `node:22-alpine` | npm dependency audit |
| `security:trivy` | security | `aquasec/trivy:latest` | Filesystem CVE scan (Go modules + pnpm) |
| `security:trivy-image` | security | `docker:latest` | Docker image CVE scan (Alpine + Go binary) |
| `security:gitleaks` | security | `zricethezav/gitleaks:latest` | Git secrets detection |
| `security:semgrep` | security | `semgrep/semgrep:latest` | Static analysis (SAST) |

### On Tag Push (`v*`)

| Job | Stage | Image | Purpose |
|-----|-------|-------|---------|
| `changelog` | release | `orhunp/git-cliff:latest` | Extract release notes for the tagged version |
| `release:goreleaser` | release | `goreleaser/goreleaser:latest` | Cross-compile binaries, create GitHub release with assets |
| `release:docker:build` | release | `docker:latest` | Build and push multi-arch Docker images to GHCR |
| `release:docker:dockerhub` | release | `alpine` + `crane` | Mirror image from GHCR to Docker Hub |
| `notify:discord` | notify | `alpine` | Send release notification to Discord |
| `pages` | pages | `node:22-alpine` | Rebuild project site with latest changelog |

## Release Artifacts

Each release produces:

| Artifact | Description |
|----------|-------------|
| `capacitarr_X.Y.Z_linux_amd64.tar.gz` | Linux x86_64 binary + README + LICENSE + CHANGELOG |
| `capacitarr_X.Y.Z_linux_arm64.tar.gz` | Linux ARM64 binary + README + LICENSE + CHANGELOG |
| `checksums.txt` | SHA-256 checksums for all archives |
| Docker image (multi-arch) | Published to GHCR and Docker Hub (see below) |

### Docker Registries

Docker images are published to two registries using a build-then-mirror pipeline:

| Registry | Image Path | Role |
|----------|-----------|------|
| GHCR | `ghcr.io/ghent/capacitarr` | Source of truth (built + pushed first) |
| Docker Hub | `ghentstarshadow/capacitarr` | Mirrored via `crane copy` |

The build job pushes to GHCR. A mirror job then uses [`crane copy`](https://github.com/google/go-containerregistry/tree/main/cmd/crane) to replicate the multi-arch manifest to Docker Hub. The mirror job has `allow_failure: true` — if Docker Hub is down, the pipeline shows a warning but does not fail.

### Docker Image Tags

Every release (including pre-releases) is tagged as `:latest` along with the full version. Stable releases additionally get `:stable`, `:MAJOR`, and `:MINOR` convenience tags:

| Tag | Applied When | Meaning |
|-----|-------------|---------|
| `:1.0.0` or `:1.0.0-rc.1` | Every release | Immutable, pinned to exact version |
| `:latest` | Every release | Most recently built image, may include pre-releases |
| `:stable` | Stable releases only | Most recent non-pre-release version (recommended) |
| `:1`, `:1.0` | Stable releases only | Floating within stable release line |

All tags are available on both registries:

```
# GHCR (recommended)
docker pull ghcr.io/ghent/capacitarr:stable

# Docker Hub (no registry prefix needed)
docker pull ghentstarshadow/capacitarr:stable
```

## Prerequisites

For the release pipeline to work correctly:

1. **Use Conventional Commits** — all commits on `main` must follow the [Conventional Commits](https://www.conventionalcommits.org/) format. Non-conventional commits are filtered out.
2. **Tag from `main`** — releases are triggered by `v*` tags. Create tags only from the `main` branch.
3. **Commit changelog and version before tagging** — the release prep step (see workflow above) must be committed before creating the tag. The CI pipeline reads from the committed files.
4. **Repository secrets** — the following secrets must be configured in GitHub (Settings → Secrets and variables → Actions):

| Secret | Purpose |
|--------|---------|
| `GHCR_TOKEN` | GitHub PAT with `write:packages` scope (or use `GITHUB_TOKEN` for GHCR) |
| `DOCKERHUB_USERNAME` | Docker Hub login username |
| `DOCKERHUB_TOKEN` | Docker Hub access token (Read & Write) |
