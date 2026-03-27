/**
 * Vite virtual module plugin for repo-based announcements.
 *
 * Reads `.md` files from `frontend/announcements/`, parses frontmatter
 * with gray-matter, and pre-renders markdown bodies to HTML at build time.
 * Exposes the result as `virtual:announcements` which can be imported
 * anywhere in the frontend.
 *
 * During dev, the plugin watches the announcements directory for changes
 * and triggers HMR updates.
 */
import { readdirSync, readFileSync, existsSync } from 'node:fs';
import { resolve, join } from 'node:path';
import matter from 'gray-matter';
import type { Plugin, ViteDevServer } from 'vite';

const VIRTUAL_MODULE_ID = 'virtual:announcements';
const RESOLVED_ID = '\0' + VIRTUAL_MODULE_ID;

/**
 * Minimal markdown-to-HTML converter for announcement bodies.
 * Handles the subset of markdown likely used in short announcements:
 * bold, italic, inline code, links, paragraphs, and line breaks.
 * No external dependency required.
 */
function renderMarkdown(md: string): string {
  return md
    .split(/\n{2,}/)
    .map((block) => {
      const html = block
        .trim()
        .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
        .replace(/\*(.+?)\*/g, '<em>$1</em>')
        .replace(/`(.+?)`/g, '<code>$1</code>')
        .replace(/\[(.+?)\]\((.+?)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>')
        .replace(/\n/g, '<br/>');
      return `<p>${html}</p>`;
    })
    .join('\n');
}

interface AnnouncementFrontmatter {
  id: string;
  title: string;
  type: 'info' | 'warning' | 'critical';
  /** gray-matter auto-parses YAML dates into Date objects */
  date: string | Date;
  expires?: string | Date;
  active: boolean;
}

/** Convert a value that may be a Date (from gray-matter) or a string to YYYY-MM-DD. */
function toDateString(value: string | Date): string {
  if (value instanceof Date) return value.toISOString().split('T')[0]!;
  return String(value);
}

function loadAnnouncements(dir: string) {
  if (!existsSync(dir)) return [];

  const files = readdirSync(dir).filter((f) => f.endsWith('.md'));
  const announcements = [];

  for (const file of files) {
    const raw = readFileSync(join(dir, file), 'utf-8');
    const { data, content } = matter(raw);
    const fm = data as AnnouncementFrontmatter;

    if (!fm.id || !fm.title || !fm.type || !fm.date) {
      console.warn(`[announcements] Skipping ${file}: missing required frontmatter fields`);
      continue;
    }

    announcements.push({
      id: fm.id,
      title: fm.title,
      type: fm.type,
      date: toDateString(fm.date),
      expires: fm.expires ? toDateString(fm.expires) : undefined,
      active: fm.active !== false,
      body: renderMarkdown(content.trim()),
    });
  }

  // Sort by date descending (newest first)
  announcements.sort((a, b) => b.date.localeCompare(a.date));
  return announcements;
}

export default function viteAnnouncements(): Plugin {
  const announcementsDir = resolve(__dirname, '../announcements');

  return {
    name: 'vite-announcements',
    resolveId(id: string) {
      if (id === VIRTUAL_MODULE_ID) return RESOLVED_ID;
    },
    load(id: string) {
      if (id === RESOLVED_ID) {
        const announcements = loadAnnouncements(announcementsDir);
        return `export default ${JSON.stringify(announcements, null, 2)};`;
      }
    },
    configureServer(server: ViteDevServer) {
      // Watch the announcements directory for HMR in dev mode
      if (existsSync(announcementsDir)) {
        server.watcher.add(announcementsDir);
        server.watcher.on('change', (changedPath: string) => {
          if (changedPath.startsWith(announcementsDir)) {
            const mod = server.moduleGraph.getModuleById(RESOLVED_ID);
            if (mod) {
              server.moduleGraph.invalidateModule(mod);
              server.ws.send({ type: 'full-reload' });
            }
          }
        });
      }
    },
  };
}
