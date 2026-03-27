/**
 * Client-side localStorage cleanup composable.
 *
 * Runs on every app mount to:
 * 1. Migrate legacy key names to the current `capacitarr:` convention.
 * 2. Prune orphaned announcement dismissal keys whose announcement IDs
 *    are no longer present in the bundled announcement list.
 *
 * Both operations are idempotent and safe to run on every page load.
 */
import { LEGACY_KEY_MAP, DISMISSED_PREFIX } from '~/utils/storageKeys';

/**
 * Migrate legacy localStorage keys to the current naming convention.
 * For each old key found, copies the value to the new key (if the new
 * key doesn't already exist), then removes the old key.
 */
function migrateKeys(): void {
  for (const [oldKey, newKey] of Object.entries(LEGACY_KEY_MAP)) {
    const oldValue = localStorage.getItem(oldKey);
    if (oldValue === null) continue;

    // Only copy if the new key is not already set (avoids overwriting
    // a value the user may have already set under the new key name).
    if (localStorage.getItem(newKey) === null) {
      localStorage.setItem(newKey, oldValue);
    }
    localStorage.removeItem(oldKey);
  }
}

/**
 * Remove announcement dismissal keys from localStorage whose
 * announcement ID is no longer in the bundled list.
 *
 * @param activeIds - IDs of announcements currently in the build bundle.
 *                    Pass an empty array to skip pruning.
 */
function pruneAnnouncementDismissals(activeIds: string[]): void {
  if (activeIds.length === 0) return;

  const activeSet = new Set(activeIds);
  const keysToRemove: string[] = [];

  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i);
    if (key && key.startsWith(DISMISSED_PREFIX)) {
      const id = key.slice(DISMISSED_PREFIX.length);
      if (!activeSet.has(id)) {
        keysToRemove.push(key);
      }
    }
  }

  for (const key of keysToRemove) {
    localStorage.removeItem(key);
  }
}

/**
 * Run all localStorage cleanup tasks.
 *
 * @param activeAnnouncementIds - IDs of all announcements in the current
 *   build bundle. When omitted or empty, dismissal pruning is skipped.
 */
export function runStorageCleanup(activeAnnouncementIds: string[] = []): void {
  migrateKeys();
  pruneAnnouncementDismissals(activeAnnouncementIds);
}
