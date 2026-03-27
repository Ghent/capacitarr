#!/bin/sh
# scripts/site-build.sh — Build the documentation site for deployment.
#
# Used by:
#   - Cloudflare Pages (build command: scripts/site-build.sh)
#   - GitHub Actions pages.yml workflow
#
# Output: site/.output/public/ (static HTML/CSS/JS)
#
# NOTE: Uses /bin/sh (not bash) for compatibility with Cloudflare's build environment.

set -eux

echo "=== Site Build ==="

# pnpm is provided by the build environment:
#   - Cloudflare Pages v3: built-in (set PNPM_VERSION env var in dashboard)
#   - GitHub Actions: installed via pnpm/action-setup or corepack
#   - Local: installed via corepack enable
echo "pnpm version: $(pnpm --version)"

# Install dependencies
cd site
pnpm install --frozen-lockfile

# Sync documentation from docs/ and root-level project files into content/
node scripts/sync-docs.mjs

# Copy favicon from frontend
cp ../frontend/public/favicon.ico public/favicon.ico 2>/dev/null || true

# Generate static site
pnpm generate

echo "=== Site build complete → site/.output/public/ ==="
