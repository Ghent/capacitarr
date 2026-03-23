# Pre-Release Versioning Support

**Status:** ✅ Complete

**Prerequisite:** [Build, CI/CD, and Publishing Overhaul](20260303T1551Z-build-cicd-publishing.md) — ✅ Complete

---

## Overview

Add support for SemVer pre-release versions (`alpha`, `beta`, `rc`) to the release pipeline. This enables a standard progression from development to stable release:

```
v0.2.0-alpha.1 → v0.2.0-alpha.2 → v0.2.0-beta.1 → v0.2.0-rc.1 → v0.2.0
```

### What Already Works

GoReleaser and git-cliff natively support SemVer pre-release tags. If you tag `v0.2.0-rc.1` today:

- GoReleaser creates a GitLab release marked as pre-release
- git-cliff includes it in the changelog
- Binary archives are built and uploaded normally

### What Needs Fixing

The `release:docker` CI job currently tags **every** release as `:latest`, `:MAJOR`, and `:MINOR`. For pre-releases, this is wrong — `v1.0.0-rc.1` should NOT become `:latest`.

---

## Phase 1: CI Docker Tag Logic

### 1.1 Detect pre-release tags in `release:docker`

Update the `release:docker` job in [`.gitlab-ci.yml`](../../.gitlab-ci.yml) to detect pre-release suffixes and adjust Docker tags accordingly:

```yaml
script:
  - |
    VERSION=${CI_COMMIT_TAG#v}

    # Always tag with the full version
    TAGS="--tag ${IMAGE}:${VERSION}"

    # Only add :latest, :MAJOR, :MINOR for stable releases (no hyphen in version)
    if echo "$VERSION" | grep -qv '-'; then
      MAJOR=$(echo "$VERSION" | cut -d. -f1)
      MINOR=$(echo "$VERSION" | cut -d. -f1-2)
      TAGS="$TAGS --tag ${IMAGE}:latest --tag ${IMAGE}:${MAJOR} --tag ${IMAGE}:${MINOR}"
    fi

    docker buildx build \
      --platform linux/amd64,linux/arm64 \
      --build-arg APP_VERSION="${CI_COMMIT_TAG}" \
      --build-arg BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      --build-arg COMMIT_SHA="${CI_COMMIT_SHORT_SHA}" \
      $TAGS \
      --push \
      .
```

**Behavior:**

| Tag | Docker Tags |
|-----|-------------|
| `v0.2.0` | `0.2.0`, `0.2`, `0`, `latest` |
| `v0.2.0-rc.1` | `0.2.0-rc.1` only |
| `v0.2.0-beta.1` | `0.2.0-beta.1` only |
| `v0.2.0-alpha.1` | `0.2.0-alpha.1` only |

### 1.2 Update GoReleaser pre-release detection

GoReleaser already detects pre-release tags automatically. Verify that `.goreleaser.yml` marks pre-releases correctly in GitLab releases. No changes expected — this is a verification step.

---

## Phase 2: Release Workflow Updates

### 2.1 Update release prep workflow

Update [`docs/releasing.md`](../releasing.md) to document the pre-release workflow:

```bash
# Pre-release
git cliff --bump -o CHANGELOG.md
VERSION="v0.2.0-rc.1"  # manually specify pre-release version
SEMVER=${VERSION#v}
npm version "$SEMVER" --no-git-tag-version
cd frontend && npm version "$SEMVER" --no-git-tag-version && cd ..
git add CHANGELOG.md package.json frontend/package.json
git commit -m "chore(release): $VERSION"
git tag "$VERSION"
git push origin main --tags

# Stable release (after RC validation)
git cliff --bump -o CHANGELOG.md
VERSION="v0.2.0"
# ... same workflow
```

### 2.2 Update `package.json` release script

The root `package.json` `release` script uses `git cliff --bumped-version` which always produces stable versions. For pre-releases, the version must be specified manually. Consider adding a `release:pre` script or documenting that pre-releases are manual-only.

---

## Phase 3: Documentation

### 3.1 Update releasing docs

Add a "Pre-Release Workflow" section to [`docs/releasing.md`](../releasing.md) covering:

- When to use alpha/beta/rc
- How Docker tags differ for pre-releases
- How to promote an RC to stable
- Example progression for a major release

### 3.2 Update README

Add a note about pre-release Docker image availability:

```markdown
## Pre-Release Images

Pre-release images are available with version-specific tags only:

```bash
docker pull registry.gitlab.com/starshadow/software/capacitarr:0.2.0-rc.1
```

The `:latest` tag always points to the most recent stable release.
```

---

## Files to Create or Modify

| File | Action | Description |
|------|--------|-------------|
| `.gitlab-ci.yml` | **Modify** | Add pre-release detection to `release:docker` job |
| `docs/releasing.md` | **Modify** | Add pre-release workflow documentation |
| `README.md` | **Modify** | Add pre-release image availability note |
| `package.json` | **Modify** | Consider adding `release:pre` script (optional) |
