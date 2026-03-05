/**
 * Shared formatting utilities for Capacitarr.
 * Single source of truth — no more copy-pasting formatBytes across components.
 */

export function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
  const i = Math.floor(Math.log(Math.abs(bytes)) / Math.log(1024));
  const val = bytes / Math.pow(1024, i);
  return `${val.toFixed(val >= 100 ? 0 : 1)} ${units[i]}`;
}

export function formatTime(dateStr: string | null | undefined): string {
  if (!dateStr) return 'Never';
  return new Date(dateStr).toLocaleString();
}

// ---------------------------------------------------------------------------
// Disk usage color helpers
// ---------------------------------------------------------------------------
// Single source of truth for disk-usage → color mapping used by both
// DiskGroupSection (dashboard) and rules.vue (scoring engine).
//
// Zones:
//   Green  — usage < targetPct   (safe zone)
//   Amber  — usage ≥ targetPct AND usage < thresholdPct  (warning zone)
//   Red    — usage ≥ thresholdPct (danger zone — cleanup should trigger)

export type DiskStatus = 'ok' | 'warning' | 'danger';

/**
 * Determine the disk usage status zone.
 * @param usagePct  Current usage percentage (0-100, NOT rounded)
 * @param targetPct Cleanup target percentage (stop cleanup at this level)
 * @param thresholdPct Cleanup threshold percentage (start cleanup above this)
 */
export function diskUsageStatus(
  usagePct: number,
  targetPct: number,
  thresholdPct: number,
): DiskStatus {
  if (usagePct >= thresholdPct) return 'danger';
  if (usagePct >= targetPct) return 'warning';
  return 'ok';
}

/** Tailwind background class for the disk-group icon badge. */
export function diskStatusBgClass(
  usagePct: number,
  targetPct: number,
  thresholdPct: number,
): string {
  const s = diskUsageStatus(usagePct, targetPct, thresholdPct);
  if (s === 'danger') return 'bg-red-500';
  if (s === 'warning') return 'bg-amber-500';
  return 'bg-green-500';
}

/** Tailwind text-color class for the percentage label. */
export function diskStatusTextClass(
  usagePct: number,
  targetPct: number,
  thresholdPct: number,
): string {
  const s = diskUsageStatus(usagePct, targetPct, thresholdPct);
  if (s === 'danger') return 'text-red-500';
  if (s === 'warning') return 'text-amber-500';
  return 'text-green-500';
}

/** Hex color for the progress-bar fill (inline style). */
export function diskStatusFillColor(
  usagePct: number,
  targetPct: number,
  thresholdPct: number,
): string {
  const s = diskUsageStatus(usagePct, targetPct, thresholdPct);
  if (s === 'danger') return '#ef4444';
  if (s === 'warning') return '#eab308';
  return '#22c55e';
}

// ---------------------------------------------------------------------------
// Relative time formatting
// ---------------------------------------------------------------------------

export function formatRelativeTime(dateStr: string | null | undefined): string {
  if (!dateStr) return 'Never';
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);
  const diffMin = Math.floor(diffSec / 60);
  const diffHour = Math.floor(diffMin / 60);
  const diffDay = Math.floor(diffHour / 24);

  if (diffSec < 60) return 'just now';
  if (diffMin < 60) return `${diffMin}m ago`;
  if (diffHour < 24) return `${diffHour}h ago`;
  if (diffDay < 7) return `${diffDay}d ago`;
  return date.toLocaleDateString();
}
