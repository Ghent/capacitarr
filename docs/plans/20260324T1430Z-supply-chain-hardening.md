# Supply Chain Hardening — Pin Docker Images & Eliminate curl-pipe-to-shell

**Status:** ✅ Complete
**Created:** 2026-03-24T14:30Z
**Triggered by:** Trivy breach investigation — no compromise found, but audit revealed 8 unpinned `:latest` Docker images and 1 curl-pipe-to-shell install across CI and Makefile

## Background

On 2026-03-24, a Trivy breach announcement prompted an investigation into our supply chain exposure. The investigation found **no evidence of compromise** (see exposure check results below), but revealed that 8 of our CI/Makefile Docker image references use `:latest` tags, meaning any upstream supply chain attack would silently propagate into our pipeline.

### Exposure Investigation Results (2026-03-24)

| Check | Result |
|-------|--------|
| Local Trivy image | ✅ v0.69.3, digest `bcc376de8d77`, official OCI labels |
| Local Docker containers | ✅ Only 3 expected containers running |
| CI `security:trivy` job | ✅ Clean output, 0 vulnerabilities |
| CI `security:trivy-image` job | ✅ Installed v0.69.3, clean scan, 0 vulnerabilities |
| GitLab Container Registry | ✅ No unexpected tags |
| GitLab Releases | ✅ Sequential semver v1.5.0 → v2.0.0, all by expected author |

### Current State of Image References

**Unpinned — `:latest` tag:**

| Image | Files | Lines |
|-------|-------|-------|
| `golangci/golangci-lint:latest` | Makefile, .gitlab-ci.yml | Makefile:24,47,58; CI:34 |
| `ghcr.io/aquasecurity/trivy:latest` | Makefile, .gitlab-ci.yml | Makefile:100,103,141; CI:115 |
| `zricethezav/gitleaks:latest` | Makefile, .gitlab-ci.yml | Makefile:105; CI:141 |
| `semgrep/semgrep:latest` | Makefile, .gitlab-ci.yml | Makefile:108; CI:152 |
| `orhunp/git-cliff:latest` | .gitlab-ci.yml | CI:164 |
| `goreleaser/goreleaser:latest` | .gitlab-ci.yml | CI:185 |
| `docker:latest` | .gitlab-ci.yml | CI:80,126 |
| `alpine:latest` | .gitlab-ci.yml | CI:222,239,257 |

**Already properly pinned:**

| Image | Pin Strategy |
|-------|-------------|
| `node:22-alpine` | Minor version pin |
| `golang:1.26-alpine` | Minor version pin |
| `alpine:3.21@sha256:c3f8e7...` | Digest pin in Dockerfile |
| `ghcr.io/zaproxy/zaproxy:stable` | Channel pin |

**curl-pipe-to-shell — highest risk:**

```yaml
# .gitlab-ci.yml:134-136 — downloads from GitHub main branch
curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin
```

## Target Pinned Versions

Versions captured from local `:latest` images on 2026-03-24:

| Image | Current Tag | Pinned To | Pin Strategy | Notes |
|-------|-------------|-----------|-------------|-------|
| `ghcr.io/aquasecurity/trivy` | `:latest` | `:0.69.3` | Semver tag | Verified via `trivy version` |
| `golangci/golangci-lint` | `:latest` | `:v2.11.4` | Semver tag | Built 2026-03-22 |
| `zricethezav/gitleaks` | `:latest` | `:v8.30.1` | Semver tag | |
| `semgrep/semgrep` | `:latest` | `:1.155.0` | Semver tag | |
| `orhunp/git-cliff` | `:latest` | `:2.12.0` | Semver tag | |
| `goreleaser/goreleaser` | `:latest` | `:v2.14.1` | Semver tag | Built 2026-02-25 |
| `docker` | `:latest` | `:27` | Major version pin | Matches Docker Engine 27.x |
| `docker` dind service | `:dind` | `:27-dind` | Major version pin | Must match `docker:27` |
| `alpine` | `:latest` | `:3.21` | Minor version pin | Matches Dockerfile runtime base |

## Goals

1. Pin all `:latest` Docker images to specific versions in both `Makefile` and `.gitlab-ci.yml`
2. Eliminate the curl-pipe-to-shell Trivy install pattern
3. Maintain the Makefile ↔ CI parity invariant
4. Document the update procedure for future version bumps

## Plan

### Phase 1: Pin Security Tooling Images

These are the highest-priority pins — security scanners with access to source code and, in one case, the Docker socket.

#### Step 1.1: Determine current versions of all unpinned images

Run each image locally to capture the exact version that `:latest` currently resolves to:

```bash
docker run --rm golangci/golangci-lint:latest golangci-lint version
docker run --rm ghcr.io/aquasecurity/trivy:latest version
docker run --rm zricethezav/gitleaks:latest version
docker run --rm semgrep/semgrep:latest semgrep --version
docker run --rm orhunp/git-cliff:latest --version
docker run --rm goreleaser/goreleaser:latest --version
docker version --format '{{.Client.Version}}'
docker run --rm alpine:latest cat /etc/alpine-release
```

Record each version for use in subsequent steps.

#### Step 1.2: Pin Trivy in `.gitlab-ci.yml` and `Makefile`

Replace all occurrences of `ghcr.io/aquasecurity/trivy:latest` with the specific version tag, e.g. `ghcr.io/aquasecurity/trivy:0.69.3`.

**Files to update:**
- `.gitlab-ci.yml` line 115: `security:trivy` job image
- `Makefile` lines 100, 103: `security:ci` target
- `Makefile` line 141: `security:image` target

#### Step 1.3: Eliminate curl-pipe-to-shell in `security:trivy-image`

Replace the current pattern in `.gitlab-ci.yml` `security:trivy-image` job (lines 123-137):

**Current approach (unsafe):**
```yaml
security:trivy-image:
  stage: security
  image: docker:latest
  services:
    - docker:dind
  script:
    - docker build -t capacitarr:ci-scan .
    - |
      apk add --no-cache curl
      curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin
      trivy image --exit-code 1 --severity HIGH,CRITICAL --scanners vuln capacitarr:ci-scan
```

**New approach — use Trivy Docker image with socket mount:**
```yaml
security:trivy-image:
  stage: security
  image: docker:27
  services:
    - docker:27-dind
  script:
    - docker build -t capacitarr:ci-scan .
    - docker run --rm -v /var/run/docker.sock:/var/run/docker.sock
        ghcr.io/aquasecurity/trivy:0.69.3
        image --exit-code 1 --severity HIGH,CRITICAL --scanners vuln capacitarr:ci-scan
```

This eliminates the curl-pipe-to-shell entirely and uses the same pinned Trivy image as the filesystem scan job. No external script is downloaded at runtime.

#### Step 1.4: Pin golangci-lint

Replace `golangci/golangci-lint:latest` with the captured version tag.

**Files to update:**
- `.gitlab-ci.yml` line 34: `lint:go` job image
- `Makefile` lines 24, 47, 58: `lint`, `check`, `lint:ci` targets

#### Step 1.5: Pin gitleaks

Replace `zricethezav/gitleaks:latest` with the captured version tag.

**Files to update:**
- `.gitlab-ci.yml` line 141: `security:gitleaks` job image
- `Makefile` line 105: `security:ci` target

#### Step 1.6: Pin semgrep

Replace `semgrep/semgrep:latest` with the captured version tag.

**Files to update:**
- `.gitlab-ci.yml` line 152: `security:semgrep` job image
- `Makefile` line 108: `security:ci` target

### Phase 2: Pin Release and Infrastructure Images

Lower priority since these only run on tag pushes and have less direct access to source code.

#### Step 2.1: Pin git-cliff

Replace `orhunp/git-cliff:latest` with the captured version tag.

**Files to update:**
- `.gitlab-ci.yml` line 164: `changelog` job image

#### Step 2.2: Pin goreleaser

Replace `goreleaser/goreleaser:latest` with the captured version tag.

**Files to update:**
- `.gitlab-ci.yml` line 185: `release:goreleaser` job image

#### Step 2.3: Pin docker and alpine in CI jobs

Replace `docker:latest` with `docker:27` and `alpine:latest` with `alpine:3.21`.

**Files to update:**
- `.gitlab-ci.yml` line 80: `build:docker` job image
- `.gitlab-ci.yml` line 126: `security:trivy-image` job image (already updated in Step 1.3)
- `.gitlab-ci.yml` lines 222, 239: `release:docker:dockerhub`, `release:docker:ghcr` job images
- `.gitlab-ci.yml` line 257: `notify:discord` job image

### Phase 3: Remove `--pull always` from Makefile

Currently the Makefile uses `--pull always` on every `docker run`, which re-pulls the image every time. With pinned version tags, this is less risky but still bypasses local image integrity if an upstream tag is reassigned.

#### Step 3.1: Change `--pull always` to `--pull missing`

Update all `docker run` commands in the Makefile to use `--pull missing` instead of `--pull always`. This ensures the local image is used if already present, and only pulls on first use or after a version bump.

**Files to update:**
- `Makefile` lines 23, 46, 57, 60, 74, 77, 89, 94, 100, 103, 105, 108, 141

### Phase 4: Verification

#### Step 4.1: Run `make ci` with pinned images

Execute the full CI pipeline locally to verify all pinned images work correctly:

```bash
cd capacitarr && make ci
```

All stages must pass: lint, test, security.

#### Step 4.2: Verify CI parity

Confirm that every image reference in `.gitlab-ci.yml` has a matching reference in the `Makefile`, and vice versa. Both must use identical version tags.

#### Step 4.3: Update `CONTRIBUTING.md`

Add a section documenting the image pinning policy and the procedure for updating pinned versions:

1. Pull the new version locally
2. Verify it works with `make ci`
3. Update the version tag in both `Makefile` and `.gitlab-ci.yml`
4. Commit with `chore(deps): bump <tool> to v<version>`

### Phase 5: Future Automation (Optional)

#### Step 5.1: Evaluate Renovate Bot for Docker image updates

Consider adding a `renovate.json` config to automatically propose MRs when new versions of pinned Docker images are released. This prevents version staleness while maintaining the pin-and-review model.

Scope: evaluate only — do not implement without separate approval.

## Pre-requisites (completed)

These documentation changes were made ahead of the implementation plan:

- ✅ **`SECURITY.md`** — Added "Supply Chain Security — Docker Image Pinning" section with pinning policy, 30-day re-evaluation cycle, and currently pinned images table
- ✅ **`.kilocoderules`** — Added rule: "Re-evaluate pinned Docker image versions every 30 days"

## Files Modified

| File | Changes |
|------|---------|
| `.gitlab-ci.yml` | Pin all 8 `:latest` images, rewrite `security:trivy-image` job |
| `Makefile` | Pin all `:latest` images, change `--pull always` to `--pull missing` |
| `SECURITY.md` | Add supply chain security section (✅ done as pre-requisite) |
| `.kilocoderules` | Add 30-day re-evaluation rule (✅ done as pre-requisite) |
| `CONTRIBUTING.md` | Add image pinning policy and update procedure |

## Invariants

- **Makefile ↔ CI parity:** Every Docker image version in `.gitlab-ci.yml` must match the corresponding image in the `Makefile`
- **No `:latest` tags:** After this plan completes, zero Docker image references should use `:latest`
- **No curl-pipe-to-shell:** No CI job should download and execute scripts from external URLs at runtime
- **No `--pull always` with version pins:** Local builds should use cached images to avoid TOCTOU races
