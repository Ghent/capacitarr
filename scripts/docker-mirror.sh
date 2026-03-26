#!/bin/sh
# scripts/docker-mirror.sh — Mirror Docker image from GHCR to a target registry.
#
# Uses `crane copy` to replicate multi-arch manifest lists between registries
# at the API level — no layers are re-pulled or re-pushed.
#
# Required environment variables:
#   CI_COMMIT_TAG     — Git tag (e.g., v1.0.0)
#   CI_REGISTRY_IMAGE — GHCR image path (source, e.g., ghcr.io/ghent/capacitarr)
#
# Arguments:
#   $1 — Target registry image path (e.g., docker.io/ghentstarshadow/capacitarr)
#
# Usage: scripts/docker-mirror.sh docker.io/ghentstarshadow/capacitarr
#
# NOTE: Uses /bin/sh (not bash) for compatibility in GitHub Actions.

set -eu

# ── Validate inputs ──────────────────────────────────────────────────────────

TARGET="${1:?Usage: docker-mirror.sh <target-registry-image>}"

: "${CI_COMMIT_TAG:?CI_COMMIT_TAG is required (e.g., v1.0.0)}"
: "${CI_REGISTRY_IMAGE:?CI_REGISTRY_IMAGE is required}"

SOURCE="${CI_REGISTRY_IMAGE}"
VERSION="${CI_COMMIT_TAG#v}"

echo "=== Docker Mirror ==="
echo "Source:   ${SOURCE}"
echo "Target:   ${TARGET}"
echo "Version:  ${VERSION}"
echo "====================="

# ── Copy tags ────────────────────────────────────────────────────────────────

# Every release: exact version + :latest
echo "Copying ${SOURCE}:${VERSION} → ${TARGET}:${VERSION}"
crane copy "${SOURCE}:${VERSION}" "${TARGET}:${VERSION}"

echo "Copying ${SOURCE}:latest → ${TARGET}:latest"
crane copy "${SOURCE}:latest" "${TARGET}:latest"

# Stable releases (no hyphen = no pre-release suffix): copy floating tags
case "$VERSION" in
    *-*) ;; # pre-release, skip floating tags
    *)
        MAJOR="${VERSION%%.*}"
        MINOR="${VERSION%.*}"

        echo "Copying ${SOURCE}:stable → ${TARGET}:stable"
        crane copy "${SOURCE}:stable" "${TARGET}:stable"

        echo "Copying ${SOURCE}:${MAJOR} → ${TARGET}:${MAJOR}"
        crane copy "${SOURCE}:${MAJOR}" "${TARGET}:${MAJOR}"

        echo "Copying ${SOURCE}:${MINOR} → ${TARGET}:${MINOR}"
        crane copy "${SOURCE}:${MINOR}" "${TARGET}:${MINOR}"
        ;;
esac

echo "Mirror complete: ${TARGET}"
