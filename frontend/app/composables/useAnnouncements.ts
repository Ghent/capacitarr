/**
 * Composable for managing repo-based announcements.
 *
 * Reads the build-time bundled announcement data from the virtual module
 * and provides reactive state for the banner and Help page archive.
 * Dismissal state is persisted in localStorage.
 */
import rawAnnouncements from 'virtual:announcements';
import type { Announcement } from 'virtual:announcements';
import { DISMISSED_PREFIX } from '~/utils/storageKeys';

function isDismissed(id: string): boolean {
  if (!import.meta.client) return false;
  return localStorage.getItem(`${DISMISSED_PREFIX}${id}`) === 'true';
}

function isExpired(announcement: Announcement): boolean {
  if (!announcement.expires) return false;
  return new Date(announcement.expires) < new Date();
}

/** Severity order for sorting: critical > warning > info */
const SEVERITY_ORDER: Record<string, number> = { critical: 0, warning: 1, info: 2 };

export function useAnnouncements() {
  // Reactive dismissed set — triggers re-computation when an announcement is dismissed
  const dismissedIds = ref(new Set<string>());

  // Initialize dismissed state from localStorage on client
  if (import.meta.client) {
    for (const a of rawAnnouncements) {
      if (isDismissed(a.id)) {
        dismissedIds.value.add(a.id);
      }
    }
  }

  /** All announcements sorted by date descending (for Help page archive). */
  const allAnnouncements = computed<Announcement[]>(() => rawAnnouncements);

  /** Active, non-expired announcements that the user has NOT dismissed. */
  const activeBannerAnnouncements = computed<Announcement[]>(() =>
    rawAnnouncements
      .filter((a) => a.active && !isExpired(a) && !dismissedIds.value.has(a.id))
      .sort((a, b) => (SEVERITY_ORDER[a.type] ?? 2) - (SEVERITY_ORDER[b.type] ?? 2)),
  );

  /** Dismiss an announcement — persists to localStorage and updates reactive state. */
  function dismiss(id: string): void {
    if (import.meta.client) {
      localStorage.setItem(`${DISMISSED_PREFIX}${id}`, 'true');
    }
    dismissedIds.value = new Set([...dismissedIds.value, id]);
  }

  /** All announcement IDs in the current build bundle (for cleanup composable). */
  const activeIds = rawAnnouncements.map((a) => a.id);

  return {
    allAnnouncements,
    activeBannerAnnouncements,
    dismiss,
    activeIds,
  };
}
