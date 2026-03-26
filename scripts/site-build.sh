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

set -eu

echo "=== Site Build ==="

# Enable corepack to make pnpm available (ships with Node.js)
corepack enable

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
