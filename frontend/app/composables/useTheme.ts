/**
 * Theme composable for multi-theme color system.
 * Manages 6 theme palettes via data-theme attribute on <html>,
 * persisted in localStorage under 'capacitarr-theme'.
 */

export type ThemeId = 'violet' | 'ocean' | 'emerald' | 'sunset' | 'rose' | 'slate';

export interface ThemeMeta {
  id: ThemeId;
  label: string;
  hue: number;
  type: 'analogous' | 'complementary' | 'monochrome';
  /** Actual light-mode primary color (oklch) for accurate swatch display */
  primaryColor: string;
}

/** All available themes with display metadata */
export const THEMES: ThemeMeta[] = [
  {
    id: 'violet',
    label: 'Violet',
    hue: 293,
    type: 'analogous',
    primaryColor: 'oklch(0.606 0.25 292.717)',
  },
  { id: 'ocean', label: 'Ocean', hue: 230, type: 'analogous', primaryColor: 'oklch(0.55 0.2 230)' },
  {
    id: 'emerald',
    label: 'Emerald',
    hue: 160,
    type: 'analogous',
    primaryColor: 'oklch(0.55 0.2 160)',
  },
  {
    id: 'sunset',
    label: 'Sunset',
    hue: 55,
    type: 'complementary',
    primaryColor: 'oklch(0.70 0.17 55)',
  },
  {
    id: 'rose',
    label: 'Rose',
    hue: 350,
    type: 'complementary',
    primaryColor: 'oklch(0.60 0.22 350)',
  },
  {
    id: 'slate',
    label: 'Slate',
    hue: 260,
    type: 'monochrome',
    primaryColor: 'oklch(0.45 0.03 260)',
  },
];

const STORAGE_KEY = 'capacitarr-theme';
const DEFAULT_THEME: ThemeId = 'violet';
const VALID_THEMES = new Set<string>(THEMES.map((t) => t.id));

export const useTheme = () => {
  const theme = useState<ThemeId>('appTheme', () => {
    if (import.meta.client) {
      const stored = localStorage.getItem(STORAGE_KEY);
      if (stored && VALID_THEMES.has(stored)) return stored as ThemeId;
    }
    return DEFAULT_THEME;
  });

  function setTheme(id: ThemeId) {
    theme.value = id;
    if (import.meta.client) {
      document.documentElement.setAttribute('data-theme', id);
      localStorage.setItem(STORAGE_KEY, id);
    }
  }

  // Apply on first client load
  if (import.meta.client) {
    document.documentElement.setAttribute('data-theme', theme.value);
  }

  return { theme: readonly(theme), setTheme, themes: THEMES };
};
