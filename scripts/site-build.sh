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

# Make pnpm available.
# Cloudflare Pages uses asdf for Node version management, which doesn't shim
# corepack for custom-installed versions (exit code 126). Use npm as the
# universal fallback that works in all environments.
if command -v pnpm >/dev/null 2>&1; then
  echo "pnpm already available"
elif corepack enable 2>/dev/null; then
  echo "pnpm enabled via corepack"
else
  echo "Installing pnpm via npm..."
  npm install -g pnpm
fi

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
