/**
 * Sync documentation from ../docs/ and root-level project files into
 * content/docs/ for Nuxt Content.
 *
 * Run from the site/ directory: node scripts/sync-docs.mjs
 *
 * Features:
 * - Auto-discovers all .md files in docs/ (no hardcoded file lists)
 * - Excludes docs/plans/ (internal development documents)
 * - Syncs root-level project files (SECURITY.md, CONTRIBUTING.md, etc.)
 * - Rewrites relative markdown links to absolute Nuxt Content paths
 * - Injects navigation ordering for sidebar display
 * - Injects CHANGELOG.md with frontmatter
 */
import { existsSync, mkdirSync, readdirSync, readFileSync, writeFileSync } from 'node:fs'
import { basename, dirname, join, relative } from 'node:path'

const ROOT = join(import.meta.dirname, '..')
const DOCS_SRC = join(ROOT, '..', 'docs')
const PROJECT_ROOT = join(ROOT, '..')
const CONTENT_DOCS = join(ROOT, 'content', 'docs')

// Directories within docs/ to exclude from sync.
// docs/plans/ contains internal development documents (per .kilocoderules).
const EXCLUDED_DIRS = new Set(['plans'])

// ── Navigation ordering ────────────────────────────────────────────
//
// Controls sidebar display order on the site. Lower numbers appear first.
// Files not listed here retain their title but get no explicit order
// (Nuxt Content falls back to alphabetical).
//
// For files within directories (api/, security/), the order is relative
// to siblings in that directory. The directory's position among top-level
// pages is controlled by its _dir.yml file (generated below).

/** Navigation metadata for top-level docs files (relative to docs/). */
const NAV_META = {
  'quick-start.md': { order: 1 },
  'configuration.md': { order: 2 },
  'deployment.md': { order: 3 },
  'scoring.md': { order: 4 },
  'notifications.md': { order: 5 },
  'troubleshooting.md': { order: 6 },
  'architecture.md': { order: 7 },
  // api/ and security/ are directories — positioned via _dir.yml
  'releasing.md': { order: 10 },
}

/** Navigation metadata for files within subdirectories. */
const NAV_META_SUB = {
  'api/README.md': { order: 1 },
  'api/examples.md': { order: 2 },
  'api/workflows.md': { order: 3 },
  'api/versioning.md': { order: 4 },
  'security/zap-baseline-20260310.md': { order: 2 },
  'security/zap-baseline-20260316.md': { order: 3 },
  'security/zap-baseline-20260323.md': { order: 4 },
}

/** Navigation metadata for root-level project files synced to the site. */
const NAV_META_ROOT = {
  'contributing.md': { order: 11 },
  'contributors.md': { order: 12 },
}

/**
 * Directory navigation config. Each entry generates a _dir.yml file
 * that positions the directory section in the sidebar.
 */
const DIR_NAV = {
  api: { title: 'API Reference', order: 8 },
  security: { title: 'Security', order: 9 },
}

// ── File discovery ─────────────────────────────────────────────────

/**
 * Recursively discover all .md files in a directory, excluding specified
 * top-level subdirectories.
 */
function discoverMarkdownFiles(dir) {
  const results = []

  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const fullPath = join(dir, entry.name)

    if (entry.isDirectory()) {
      // Check top-level exclusions relative to DOCS_SRC
      const relDir = relative(DOCS_SRC, fullPath)
      if (EXCLUDED_DIRS.has(relDir)) continue

      results.push(...discoverMarkdownFiles(fullPath))
    } else if (entry.isFile() && entry.name.endsWith('.md')) {
      results.push(fullPath)
    }
  }

  return results
}

// ── Link rewriting ─────────────────────────────────────────────────

/**
 * Rewrite relative markdown links within docs/ files to absolute /docs/ paths.
 *
 * Matches: [text](file.md) or [text](file.md#anchor)
 * Converts: (file.md) → (/docs/{subdir}/file)
 *
 * Parent-directory links (e.g., ../architecture.md from api/) resolve
 * naturally via URL path normalization in the browser.
 */
function rewriteDocsLinks(content, contentSubdir) {
  const prefix = contentSubdir ? `/docs/${contentSubdir}` : '/docs'
  return content.replace(
    /\]\(([^)]+?)\.md(#[^)]*?)?\)/g,
    (_match, file, anchor = '') => {
      // Skip absolute URLs and already-absolute paths
      if (file.startsWith('http') || file.startsWith('/')) return _match
      // README → index in any directory
      const name = file.replace(/(?:^|(?<=\/))README$/, 'index')
      return `](${prefix}/${name}${anchor})`
    },
  )
}

/**
 * Rewrite markdown links within root-level project files.
 *
 * Root files reference paths relative to the project root:
 * - (CONTRIBUTORS.md)                      → (/docs/contributors)
 * - (docs/architecture.md)                 → (/docs/architecture)
 * - (docs/security/zap-baseline-20260310.md) → (/docs/security/zap-baseline-20260310)
 */
function rewriteRootLinks(content) {
  // Map of root-level file basenames (without .md) to their site paths
  const ROOT_FILE_MAP = {
    SECURITY: 'security',
    CONTRIBUTING: 'contributing',
    CONTRIBUTORS: 'contributors',
    CHANGELOG: 'changelog',
    README: 'index',
    LICENSE: null, // Not synced — leave link unchanged
  }

  return content.replace(
    /\]\(([^)]+?)\.md(#[^)]*?)?\)/g,
    (_match, file, anchor = '') => {
      if (file.startsWith('http') || file.startsWith('/')) return _match

      let path = file

      // Strip docs/ prefix — root files reference docs/ relative to project root
      if (path.startsWith('docs/')) {
        path = path.slice(5)
      } else {
        // Check if this is a known root file
        const mapped = ROOT_FILE_MAP[path]
        if (mapped === null) return _match // Skip unmapped files (e.g., LICENSE)
        if (mapped !== undefined) path = mapped
      }

      // README → index in any directory
      path = path.replace(/(?:^|(?<=\/))README$/, 'index')

      return `](/docs/${path}${anchor})`
    },
  )
}

// ── Navigation frontmatter injection ───────────────────────────────

/**
 * Inject navigation ordering into file content via YAML frontmatter.
 *
 * If the file already has frontmatter (starts with ---), the navigation
 * block is inserted before the closing ---. Otherwise, a new frontmatter
 * block is created.
 *
 * @param {string} content - File content (after link rewriting)
 * @param {{ order: number }} navMeta - Navigation metadata
 * @returns {string} Content with navigation frontmatter
 */
function injectNavigation(content, navMeta) {
  if (!navMeta) return content

  const navYaml = `navigation:\n  order: ${navMeta.order}`

  if (content.startsWith('---\n')) {
    // Has existing frontmatter — inject before the closing ---
    const endIndex = content.indexOf('\n---', 4)
    if (endIndex !== -1) {
      return content.slice(0, endIndex) + '\n' + navYaml + content.slice(endIndex)
    }
  }

  // No frontmatter — create a new block
  return `---\n${navYaml}\n---\n\n${content}`
}

// ── Heading extraction ──────────────────────────────────────────────

/**
 * Extract the leading `# Heading` from markdown content and return it
 * separately as a title. This prevents duplicate headings when Nuxt
 * Content renders both UPageHeader (from frontmatter title) and the
 * markdown `# Heading`.
 *
 * Only extracts if the file has no existing frontmatter (files with
 * frontmatter already control their title via YAML).
 *
 * @param {string} content - Raw markdown content
 * @returns {{ title: string|null, content: string }}
 */
function extractLeadingHeading(content) {
  // Skip files that already have frontmatter
  if (content.startsWith('---\n')) {
    return { title: null, content }
  }

  // Match a leading `# Title` (with optional blank lines before it)
  const match = content.match(/^\s*#\s+(.+)\n+/)
  if (match) {
    return {
      title: match[1].trim(),
      content: content.slice(match[0].length),
    }
  }

  return { title: null, content }
}

// ── File sync ──────────────────────────────────────────────────────

/**
 * Sync a single file: read, rewrite links, extract heading, optionally
 * add frontmatter and navigation ordering, write.
 */
function syncFile(src, dest, { rewriter = rewriteDocsLinks, rewriterArg = '', frontmatter = null, navMeta = null } = {}) {
  mkdirSync(dirname(dest), { recursive: true })

  let content = readFileSync(src, 'utf-8')
  content = rewriter(content, rewriterArg)

  // Extract leading # heading before adding frontmatter (prevents duplicate headings).
  // If frontmatter is explicitly provided (root files), still strip the heading
  // since the title is already in the provided frontmatter.
  const { title: extractedTitle, content: bodyContent } = extractLeadingHeading(content)
  content = bodyContent

  if (frontmatter) {
    // Root files: use provided frontmatter (title already included)
    content = `---\n${frontmatter}\n---\n\n${content}`
  } else if (extractedTitle) {
    // Docs files without frontmatter: use extracted heading as title
    content = `---\ntitle: "${extractedTitle}"\n---\n\n${content}`
  }

  // Inject navigation ordering into frontmatter
  content = injectNavigation(content, navMeta)

  writeFileSync(dest, content)
}

// ── Auto-discover and sync docs/ ───────────────────────────────────

const discoveredFiles = discoverMarkdownFiles(DOCS_SRC)
let docsCount = 0

for (const srcPath of discoveredFiles) {
  const relPath = relative(DOCS_SRC, srcPath)
  const dir = dirname(relPath)
  const file = basename(relPath)

  // README.md → index.md in any directory
  const destName = file === 'README.md' ? 'index.md' : file
  const destPath = join(CONTENT_DOCS, dir === '.' ? '' : dir, destName)

  // Content subdir for link rewriting (e.g., 'api' for docs/api/*.md)
  const contentSubdir = dir === '.' ? '' : dir

  // Look up navigation metadata for this file
  const navMeta = NAV_META[relPath] || NAV_META_SUB[relPath] || null

  syncFile(srcPath, destPath, { rewriter: rewriteDocsLinks, rewriterArg: contentSubdir, navMeta })
  docsCount++
}

// ── Generate _dir.yml for directory sections ───────────────────────

for (const [dirName, meta] of Object.entries(DIR_NAV)) {
  const dirPath = join(CONTENT_DOCS, dirName)
  mkdirSync(dirPath, { recursive: true })
  const yml = `title: "${meta.title}"\nnavigation:\n  order: ${meta.order}\n`
  writeFileSync(join(dirPath, '_dir.yml'), yml)
}

// ── Sync root-level project files ──────────────────────────────────
//
// SECURITY.md is routed to security/index.md to avoid a naming conflict
// with the docs/security/ directory (which contains the ZAP baseline).

const rootFiles = [
  { src: 'SECURITY.md', dest: 'security/index.md', title: 'Security Policy', navMeta: { order: 1 } },
  { src: 'CONTRIBUTING.md', dest: 'contributing.md', title: 'Contributing', navMeta: NAV_META_ROOT['contributing.md'] },
  { src: 'CONTRIBUTORS.md', dest: 'contributors.md', title: 'Contributors', navMeta: NAV_META_ROOT['contributors.md'] },
]

let rootCount = 0

for (const { src, dest, title, navMeta } of rootFiles) {
  const srcPath = join(PROJECT_ROOT, src)
  if (!existsSync(srcPath)) {
    console.warn(`⚠ Root file not found, skipping: ${src}`)
    continue
  }
  syncFile(srcPath, join(CONTENT_DOCS, dest), {
    rewriter: rewriteRootLinks,
    frontmatter: `title: ${title}`,
    navMeta,
  })
  rootCount++
}

// ── Inject changelog ───────────────────────────────────────────────

const changelogSrc = join(PROJECT_ROOT, 'CHANGELOG.md')
let changelogSynced = false
if (existsSync(changelogSrc)) {
  const changelogContent = readFileSync(changelogSrc, 'utf-8')
  const changelogMd = `---\ntitle: Changelog\nnavigation:\n  order: 13\n---\n\n${changelogContent}`
  mkdirSync(CONTENT_DOCS, { recursive: true })
  writeFileSync(join(CONTENT_DOCS, 'changelog.md'), changelogMd)
  changelogSynced = true
}

// Screenshots are managed as lossless WebP files in public/screenshots/.
// They are converted once from ../screenshots/*.png using sharp.
// The sync script does not manage them — see docs/plans/ for the conversion process.

console.log(
  `✓ Docs synced to content/docs/ (${docsCount} files from docs/`
  + `, ${rootCount} root files`
  + `, ${Object.keys(DIR_NAV).length} _dir.yml`
  + `${changelogSynced ? ', changelog' : ''})`,
)
