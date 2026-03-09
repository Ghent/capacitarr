/**
 * Shared show/season grouping logic for preview data.
 * Used by both rules.vue (deletion preview table) and useApprovalQueue.ts (approval queue).
 */
import type { EvaluatedItem } from '~/types/api';

export interface PreviewGroup {
  key: string;
  entry: EvaluatedItem;
  seasons: EvaluatedItem[];
}

/**
 * Group evaluated items by show, attaching seasons under their parent show.
 * Non-show/season items (movies, artists, books) are standalone groups.
 *
 * Two-pass approach:
 *   Pass 1: collect show entries and create groups for them.
 *   Pass 2: attach seasons to their parent show, or create synthetic groups for orphan seasons.
 *   Filter: remove show-level entries with no seasons (nothing actionable).
 */
export function groupEvaluatedItems(items: EvaluatedItem[]): PreviewGroup[] {
  const groups: PreviewGroup[] = [];
  // Map from show name → index in groups array
  const showMap = new Map<string, number>();

  // Pass 1: identify all show entries and create groups for them
  for (const item of items) {
    if (item.item?.type === 'show') {
      const key = `preview-${item.item.title}-show`;
      showMap.set(item.item.title, groups.length);
      groups.push({ key, entry: item, seasons: [] });
    }
  }

  // Pass 2: attach seasons to their parent show, or create a synthetic show group
  for (const item of items) {
    const title = item.item?.title ?? '';
    if (item.item?.type === 'season' && title.includes(' - Season ')) {
      const showName = title.split(' - Season ')[0]!;
      const groupIdx = showMap.get(showName);
      if (groupIdx !== undefined && groups[groupIdx]) {
        groups[groupIdx]!.seasons.push(item);
      } else {
        // Season without a parent show entry — create a synthetic group using the season as the parent
        const syntheticKey = `preview-${showName}-show-synthetic`;
        if (!showMap.has(showName)) {
          showMap.set(showName, groups.length);
          // Use the first season as the group entry but display the show name
          const syntheticEntry: EvaluatedItem = {
            ...item,
            item: { ...item.item, title: showName, type: 'show' },
          };
          groups.push({ key: syntheticKey, entry: syntheticEntry, seasons: [item] });
        } else {
          // Already created a synthetic group, just add the season
          const existingIdx = showMap.get(showName)!;
          groups[existingIdx]!.seasons.push(item);
        }
      }
    } else if (item.item?.type !== 'show') {
      // Non-show, non-season items (movies, artists, books, etc.)
      const key = `preview-${title}-${item.item?.type}`;
      groups.push({ key, entry: item, seasons: [] });
    }
    // Shows already handled in pass 1
  }

  // Filter out show-level entries with no seasons — they're only useful as grouping parents
  // A show with 0 seasons in the preview has nothing actionable to display
  return groups
    .filter((g) => !(g.entry.item?.type === 'show' && g.seasons.length === 0))
    .map((g) => {
      if (g.seasons.length <= 1) return g;
      // Sort seasons by title with numeric awareness so
      // "Season 2" sorts before "Season 10"
      return {
        ...g,
        seasons: [...g.seasons].sort((a, b) =>
          (a.item?.title ?? '').localeCompare(b.item?.title ?? '', undefined, { numeric: true }),
        ),
      };
    });
}
