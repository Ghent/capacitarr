#!/bin/sh
# scripts/docker-build.sh — Build and push multi-arch Docker image to GHCR.
#
# This script builds the Capacitarr Docker image for linux/amd64 and linux/arm64,
# then pushes it to GHCR (the source of truth). External registries (Docker Hub)
# are populated by the separate docker-mirror.sh script.
#
# Required environment variables:
#   CI_COMMIT_TAG       — Git tag (e.g., v1.0.0)
#   CI_COMMIT_SHORT_SHA — Short commit SHA
#   CI_REGISTRY_IMAGE   — Registry image path (e.g., ghcr.io/ghent/capacitarr)
#
# Usage: scripts/docker-build.sh
#
# NOTE: Uses /bin/sh (not bash) for Alpine compatibility in GitHub Actions.

set -eu

# ── Validate required environment variables ──────────────────────────────────

: "${CI_COMMIT_TAG:?CI_COMMIT_TAG is required (e.g., v1.0.0)}"
: "${CI_COMMIT_SHORT_SHA:?CI_COMMIT_SHORT_SHA is required}"
: "${CI_REGISTRY_IMAGE:?CI_REGISTRY_IMAGE is required}"

# ── Compute version and tags ─────────────────────────────────────────────────

VERSION="${CI_COMMIT_TAG#v}"
IMAGE="${CI_REGISTRY_IMAGE}"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# Every release: exact version + :latest
TAGS="--tag ${IMAGE}:${VERSION} --tag ${IMAGE}:latest"

# Stable releases (no hyphen = no pre-release suffix): add floating tags
case "$VERSION" in
    *-*) ;; # pre-release, skip floating tags
    *)
        MAJOR="${VERSION%%.*}"
        MINOR="${VERSION%.*}"
        TAGS="$TAGS --tag ${IMAGE}:stable --tag ${IMAGE}:${MAJOR} --tag ${IMAGE}:${MINOR}"
        ;;
esac

# ── Build and push ───────────────────────────────────────────────────────────

echo "=== Docker Build ==="
echo "Version:  ${VERSION}"
echo "Image:    ${IMAGE}"
echo "Date:     ${BUILD_DATE}"
echo "Commit:   ${CI_COMMIT_SHORT_SHA}"
echo "Tags:     ${TAGS}"
echo "===================="

# shellcheck disable=SC2086  # Intentional word splitting of $TAGS
docker buildx build \
    --platform linux/amd64,linux/arm64 \
    --provenance=false \
    --build-arg APP_VERSION="${CI_COMMIT_TAG}" \
    --build-arg BUILD_DATE="${BUILD_DATE}" \
    --build-arg COMMIT_SHA="${CI_COMMIT_SHORT_SHA}" \
    ${TAGS} \
    --push \
    .
