# Release Workflow

Capacitarr uses a tag-triggered release pipeline powered by [git-cliff](https://git-cliff.org/), [GoReleaser](https://goreleaser.com/), and GitHub Actions. Releases follow [Semantic Versioning](https://semver.org/) and are driven by [Conventional Commits](https://www.conventionalcommits.org/).

## How It Works

### Tag-Triggered Releases

Releases are created when you push a `v*` tag to the repository. The CI pipeline then:

1. **Extracts release notes** — `git cliff --latest --strip header` generates notes for the tagged version
2. **Builds cross-compiled binaries** — GoReleaser compiles `linux/amd64` and `linux/arm64` binaries with the frontend SPA embedded
3. **Creates a GitHub release** — with binary archives and checksums attached as downloadable assets
4. **Pushes Docker images** — multi-arch images (`linux/amd64` + `linux/arm64`) to GHCR and Docker Hub
5. **Sends release notification** — posts to Discord via webhook

GoReleaser is configured with `prerelease: auto`. Tags with a SemVer pre-release suffix (`-rc.1`, `-beta`, `-alpha`) become GitHub pre-releases and are not marked Latest. Stable tags remain full releases.

The Release workflow also uses a per-tag concurrency group (`release-${{ github.ref }}`, `cancel-in-progress: true`). A tag push can be delivered twice; without this, two workflows race and GoReleaser fails uploading assets that already exist.

### On Every Push and PR

The standard CI pipeline runs on every push and pull request:

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

# 6. Push
git push origin main        # CI pipeline: lint/test/build/security
git push origin "$VERSION"  # Release pipeline: changelog + GoReleaser + Docker + Discord
```

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

### On Every Push and PR

These jobs are defined in `.github/workflows/ci.yml`. Lint and test jobs run in parallel; build and security jobs run after lint+test pass.

| Job | Group | Tool / Action | Purpose |
|-----|-------|---------------|---------|
| `lint-go` | lint | `golangci/golangci-lint:v2.11.4` Docker image | Go linting |
| `lint-frontend` | lint | pnpm + Node.js 24 | ESLint + Prettier + TypeScript typecheck |
| `test-go` | test | Go 1.26 | Go unit tests |
| `test-frontend` | test | pnpm + Vitest | Frontend unit tests |
| `build-docker` | build | `docker/setup-buildx-action` | Multi-arch Docker smoke test (no push) |
| `security-govulncheck` | security | `govulncheck` | Go vulnerability check |
| `security-pnpm-audit` | security | `pnpm audit --ignore-registry-errors` | npm dependency audit (see `SECURITY.md` for registry 410 workaround) |
| `security-trivy` | security | `aquasecurity/trivy-action@v0.35.0` (Trivy v0.69.3) | Filesystem CVE scan (backend + frontend) |
| `security-trivy-image` | security | `aquasecurity/trivy-action@v0.35.0` (Trivy v0.69.3) | Docker image CVE scan |
| `security-gitleaks` | security | `gitleaks/gitleaks-action` | Git secrets detection |
| `security-semgrep` | security | `semgrep/semgrep:1.155.0` | Static analysis (SAST) |

### On Tag Push (`v*`)

These jobs are defined in `.github/workflows/release.yml`.

| Job | Tool / Action | Purpose |
|-----|---------------|---------|
| `changelog` | `orhunp/git-cliff:2.12.0` | Extract release notes for the tagged version |
| `goreleaser` | `goreleaser/goreleaser-action@v6` (GoReleaser v2.14.1) | Cross-compile binaries, create GitHub Release with assets |
| `docker-build` | `docker/setup-buildx-action` + GHCR login | Build and push multi-arch Docker images to GHCR |
| `docker-mirror-dockerhub` | `crane copy` | Mirror from GHCR to Docker Hub (`continue-on-error`) |
| `discord-notify` | `scripts/discord-release-notify.sh` | Send release notification to Discord (`continue-on-error`) |

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

The build job pushes to GHCR. A mirror job then uses [`crane copy`](https://github.com/google/go-containerregistry/tree/main/cmd/crane) to replicate the multi-arch manifest to Docker Hub. The mirror job has `continue-on-error: true` — if Docker Hub is down, the workflow shows a warning but does not fail.

### Docker Image Tags

Every release is tagged with the exact version and `:latest`. Pre-release channels (alpha, beta) get a rolling channel tag. Stable releases get `:stable`, `:MAJOR`, and `:MINOR` convenience tags:

| Tag | Applied When | Meaning |
|-----|-------------|---------|
| `:1.0.0` or `:1.0.0-beta` | Every release | Immutable, pinned to exact version |
| `:latest` | Every release | Most recently built image, may include pre-releases |
| `:alpha` | Alpha releases (`*-alpha*`) | Rolling tag — always the latest alpha build |
| `:beta` | Beta releases (`*-beta*`) | Rolling tag — always the latest beta build |
| `:stable` | Stable releases only | Most recent non-pre-release version (recommended for production) |
| `:1`, `:1.0` | Stable releases only | Floating within stable release line |

Other pre-release suffixes (e.g., `-rc.1`) receive only the exact version and `:latest` tags.

All tags are available on both registries:

```
# GHCR (recommended)
docker pull ghcr.io/ghent/capacitarr:stable
docker pull ghcr.io/ghent/capacitarr:beta

# Docker Hub (no registry prefix needed)
docker pull ghentstarshadow/capacitarr:stable
docker pull ghentstarshadow/capacitarr:beta
```

## Prerequisites

For the release pipeline to work correctly:

1. **Use Conventional Commits** — all commits on `main` must follow the [Conventional Commits](https://www.conventionalcommits.org/) format. Non-conventional commits are filtered out.
2. **Tag from `main`** — releases are triggered by `v*` tags. Create tags only from the `main` branch.
3. **Commit changelog and version before tagging** — the release prep step (see workflow above) must be committed before creating the tag. The CI pipeline reads from the committed files.
4. **Repository secrets** — the following secrets must be configured in GitHub (Settings → Secrets and variables → Actions):

| Secret | Purpose |
|--------|---------|
| `DOCKERHUB_USERNAME` | Docker Hub login username (for mirroring) |
| `DOCKERHUB_TOKEN` | Docker Hub access token (Read & Write) |
| `DISCORD_WEBHOOK_URL` | Discord webhook URL for release notifications |

`GITHUB_TOKEN` is automatically provided by GitHub Actions — it handles GHCR authentication and GoReleaser GitHub Releases. No separate GHCR token is needed.
