/**
 * Fetch GitHub repo stats at build time.
 * Writes stats to app/repo-stats.json for the RepoStats component.
 *
 * Run from the site/ directory: node scripts/fetch-repo-stats.mjs
 * Called automatically via the "pregenerate" script in package.json.
 *
 * Set GITHUB_TOKEN in the environment for higher rate limits (optional).
 */
import { writeFileSync } from 'node:fs'
import { join } from 'node:path'

const ROOT = join(import.meta.dirname, '..')
const OUTPUT = join(ROOT, 'app', 'repo-stats.json')

const API_BASE = 'https://api.github.com'
const REPO = 'Ghent/capacitarr'

async function fetchJSON(url) {
  const headers = {
    Accept: 'application/vnd.github+json',
    'User-Agent': 'capacitarr-site-build',
  }

  // Use GITHUB_TOKEN if available for higher rate limits
  if (process.env.GITHUB_TOKEN) {
    headers.Authorization = `Bearer ${process.env.GITHUB_TOKEN}`
  }

  const res = await fetch(url, { headers })
  if (!res.ok) {
    console.warn(`⚠ Failed to fetch ${url}: ${res.status} ${res.statusText}`)
    return null
  }
  return res.json()
}

async function main() {
  const stats = {
    stars: 0,
    forks: 0,
    version: null,
    fetchedAt: new Date().toISOString(),
  }

  // Fetch repository metadata (stars, forks)
  const repo = await fetchJSON(`${API_BASE}/repos/${REPO}`)
  if (repo) {
    stats.stars = repo.stargazers_count ?? 0
    stats.forks = repo.forks_count ?? 0
  }

  // Fetch latest release tag
  const release = await fetchJSON(`${API_BASE}/repos/${REPO}/releases/latest`)
  if (release) {
    stats.version = release.tag_name ?? null
  }

  writeFileSync(OUTPUT, JSON.stringify(stats, null, 2))
  console.log(`✓ Repo stats written to app/repo-stats.json:`)
  console.log(`  Stars: ${stats.stars} | Forks: ${stats.forks} | Version: ${stats.version ?? 'none'}`)
}

main().catch((err) => {
  console.warn('⚠ Failed to fetch repo stats (non-fatal):', err.message)
  // Write fallback stats so the component still works
  writeFileSync(
    OUTPUT,
    JSON.stringify({ stars: 0, forks: 0, version: null, fetchedAt: new Date().toISOString() }, null, 2),
  )
})
