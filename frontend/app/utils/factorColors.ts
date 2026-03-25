/**
 * Factor color and abbreviation mapping.
 * Single source of truth — keyed on stable machine identifiers from the backend.
 */

// Primary map: keyed on ScoringFactor.Key() values
const FACTOR_COLORS: Record<string, string> = {
  watch_history: '#8b5cf6',
  last_watched: '#3b82f6',
  file_size: '#f59e0b',
  rating: '#10b981',
  time_in_library: '#f97316',
  series_status: '#ec4899',
  request_popularity: '#06b6d4',
};

const FACTOR_ABBRS: Record<string, string> = {
  watch_history: 'P:',
  last_watched: 'LP:',
  file_size: 'S:',
  rating: 'Rt:',
  time_in_library: 'A:',
  series_status: 'Sh:',
  request_popularity: 'Rq:',
};

const DEFAULT_COLOR = '#6b7280';

/** Resolve factor color by key. Returns default gray if key is unknown or absent. */
export function factorColor(key?: string): string {
  if (key && FACTOR_COLORS[key]) return FACTOR_COLORS[key];
  return DEFAULT_COLOR;
}

/** Resolve factor abbreviation by key. Falls back to first two chars of key. */
export function factorAbbr(key?: string): string {
  if (key && FACTOR_ABBRS[key]) return FACTOR_ABBRS[key];
  return (key ?? '').slice(0, 2) + ':';
}
