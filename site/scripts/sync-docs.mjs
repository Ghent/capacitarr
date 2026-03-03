/**
 * Sync documentation from ../docs/ into content/docs/ for Nuxt Content.
 * Run from the site/ directory: node scripts/sync-docs.mjs
 *
 * Rewrites relative markdown links (e.g. `(deployment.md)`) to absolute
 * Nuxt Content paths (e.g. `(/docs/deployment)`) so prerender crawling works.
 */
import { cpSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { basename, dirname, join } from 'node:path'

const ROOT = join(import.meta.dirname, '..')
const DOCS_SRC = join(ROOT, '..', 'docs')
const CONTENT_DOCS = join(ROOT, 'content', 'docs')

// Ensure target directories exist
mkdirSync(join(CONTENT_DOCS, 'api'), { recursive: true })

/**
 * Rewrite relative markdown links to absolute /docs/ paths.
 * Matches: [text](file.md) or [text](file.md#anchor)
 * Converts: (file.md) → (/docs/{dir}/file) where {dir} is the content subdirectory.
 */
function rewriteLinks(content, contentSubdir) {
  const prefix = contentSubdir ? `/docs/${contentSubdir}` : '/docs'
  return content.replace(
    /\]\(([^)]+?)\.md(#[^)]*?)?\)/g,
    (_match, file, anchor = '') => {
      // Skip absolute URLs and already-absolute paths
      if (file.startsWith('http') || file.startsWith('/')) return _match
      // Handle README → index
      const name = file === 'README' ? 'index' : file
      return `](${prefix}/${name}${anchor})`
    },
  )
}

/**
 * Copy + rewrite a markdown file.
 */
function syncFile(src, dest, contentSubdir = '') {
  let content = readFileSync(src, 'utf-8')
  content = rewriteLinks(content, contentSubdir)
  writeFileSync(dest, content)
}

// Copy main docs
const mainDocs = ['index.md', 'quick-start.md', 'deployment.md', 'configuration.md', 'scoring.md', 'releasing.md']
for (const file of mainDocs) {
  syncFile(join(DOCS_SRC, file), join(CONTENT_DOCS, file))
}

// Copy API docs (README.md becomes index.md)
syncFile(join(DOCS_SRC, 'api', 'README.md'), join(CONTENT_DOCS, 'api', 'index.md'), 'api')
const apiDocs = ['examples.md', 'workflows.md', 'versioning.md']
for (const file of apiDocs) {
  syncFile(join(DOCS_SRC, 'api', file), join(CONTENT_DOCS, 'api', file), 'api')
}

// Inject changelog
const changelogSrc = join(ROOT, '..', 'CHANGELOG.md')
const changelogContent = readFileSync(changelogSrc, 'utf-8')
const changelogMd = `---\ntitle: Changelog\n---\n\n${changelogContent}`
writeFileSync(join(CONTENT_DOCS, 'changelog.md'), changelogMd)

// Sync screenshots from ../screenshots/ to public/screenshots/
const SCREENSHOTS_SRC = join(ROOT, '..', 'screenshots')
const SCREENSHOTS_DEST = join(ROOT, 'public', 'screenshots')
mkdirSync(SCREENSHOTS_DEST, { recursive: true })

import { readdirSync } from 'node:fs'
const screenshotFiles = readdirSync(SCREENSHOTS_SRC).filter(f => /\.(png|jpe?g|webp|gif|svg)$/i.test(f))
for (const file of screenshotFiles) {
  cpSync(join(SCREENSHOTS_SRC, file), join(SCREENSHOTS_DEST, file))
  // Also create a clean-name alias (e.g. 01_dashboard_20260303.png → dashboard.png)
  const match = file.match(/^\d+_(.+?)_\d{8}\.(.+)$/)
  if (match) {
    const cleanName = `${match[1].replace(/_/g, '-')}.${match[2]}`
    cpSync(join(SCREENSHOTS_SRC, file), join(SCREENSHOTS_DEST, cleanName))
  }
}

console.log(`✓ ${screenshotFiles.length} screenshots synced to public/screenshots/`)
console.log('✓ Docs synced to content/docs/')
