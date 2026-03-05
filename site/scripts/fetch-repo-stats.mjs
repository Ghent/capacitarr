/**
 * Fetch GitLab repo stats at build time.
 * Writes stats to app/repo-stats.json for the RepoStats component.
 *
 * Run from the site/ directory: node scripts/fetch-repo-stats.mjs
 * Called automatically via the "pregenerate" script in package.json.
 */
import { writeFileSync } from 'node:fs'
import { join } from 'node:path'

const ROOT = join(import.meta.dirname, '..')
const OUTPUT = join(ROOT, 'app', 'repo-stats.json')

const PROJECT_PATH = 'starshadow%2Fsoftware%2Fcapacitarr'
const API_BASE = 'https://gitlab.com/api/v4'

async function fetchJSON(url) {
  const res = await fetch(url)
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

  // Fetch project metadata (stars, forks)
  const project = await fetchJSON(`${API_BASE}/projects/${PROJECT_PATH}`)
  if (project) {
    stats.stars = project.star_count ?? 0
    stats.forks = project.forks_count ?? 0
  }

  // Fetch latest release tag
  const releases = await fetchJSON(
    `${API_BASE}/projects/${PROJECT_PATH}/releases?per_page=1&order_by=released_at&sort=desc`,
  )
  if (releases && releases.length > 0) {
    stats.version = releases[0].tag_name ?? null
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
