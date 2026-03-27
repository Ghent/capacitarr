/**
 * Canonical localStorage key constants.
 *
 * All keys use the `capacitarr:<camelCase>` convention.
 * Any code that reads or writes localStorage MUST import from here
 * instead of hardcoding key strings.
 *
 * IMPORTANT: The nuxt.config.ts inline head script (FOUC prevention)
 * cannot import this module — it duplicates the `theme` and `colorMode`
 * key strings as raw literals. If you rename those keys, update the
 * inline script as well.
 */
export const STORAGE_KEYS = {
  theme: 'capacitarr:theme',
  colorMode: 'capacitarr:colorMode',
  timezone: 'capacitarr:timezone',
  clockFormat: 'capacitarr:clockFormat',
  viewMode: 'capacitarr:viewMode',
  exactDates: 'capacitarr:exactDates',
  sparklines: 'capacitarr:sparklines',
  plexClientId: 'capacitarr:plexClientId',
} as const;

/**
 * Map of legacy key names → current key names.
 * Used by useStorageCleanup to migrate values on first load after upgrade.
 */
export const LEGACY_KEY_MAP: Record<string, string> = {
  'capacitarr-theme': STORAGE_KEYS.theme,
  'capacitarr-color-mode': STORAGE_KEYS.colorMode,
  capacitarr_timezone: STORAGE_KEYS.timezone,
  capacitarr_clockFormat: STORAGE_KEYS.clockFormat,
  capacitarr_viewMode: STORAGE_KEYS.viewMode,
  capacitarr_exactDates: STORAGE_KEYS.exactDates,
  'capacitarr:showMiniSparklines': STORAGE_KEYS.sparklines,
  capacitarr_plexClientId: STORAGE_KEYS.plexClientId,
};

/** Prefix used for announcement dismissal keys in localStorage. */
export const DISMISSED_PREFIX = 'capacitarr:dismissed:';
