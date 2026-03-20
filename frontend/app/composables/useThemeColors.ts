/**
 * Provides chart-safe hex colors for ECharts and other canvas-based libraries.
 *
 * Uses a static hex lookup table per theme — no DOM manipulation, no
 * getComputedStyle, no oklch-to-hex conversion. The hex values are
 * pre-computed from the oklch values in main.css and are guaranteed to
 * be valid #rrggbb strings that ECharts can consume directly.
 *
 * Reactively updates when the theme changes via useTheme().
 */
import type { ThemeId } from './useTheme';

interface ChartColors {
  chart1: string;
  chart2: string;
  chart3: string;
  chart4: string;
  primary: string;
  destructive: string;
  success: string;
}

/**
 * Hand-picked hex chart palettes per theme. Each palette uses
 * complementary/analogous hue relationships for maximum visual contrast.
 */
const THEME_COLORS: Record<ThemeId, ChartColors> = {
  violet: {
    chart1: '#8b5cf6', // violet (primary)
    chart2: '#06b6d4', // cyan (complementary cool)
    chart3: '#f59e0b', // amber (warm accent)
    chart4: '#10b981', // emerald (natural contrast)
    primary: '#8b5cf6',
    destructive: '#ef4444',
    success: '#10b981',
  },
  ocean: {
    chart1: '#0ea5e9', // sky blue (primary)
    chart2: '#8b5cf6', // violet (warm accent)
    chart3: '#f59e0b', // amber (complementary warm)
    chart4: '#10b981', // emerald (analogous cool)
    primary: '#0ea5e9',
    destructive: '#ef4444',
    success: '#10b981',
  },
  emerald: {
    chart1: '#10b981', // emerald (primary)
    chart2: '#0ea5e9', // sky blue (cool analogous)
    chart3: '#f59e0b', // amber (complementary warm)
    chart4: '#8b5cf6', // violet (contrast)
    primary: '#10b981',
    destructive: '#ef4444',
    success: '#10b981',
  },
  sunset: {
    chart1: '#f59e0b', // amber (primary)
    chart2: '#ef4444', // red (warm analogous)
    chart3: '#0ea5e9', // sky blue (complementary cool)
    chart4: '#8b5cf6', // violet (cool contrast)
    primary: '#f59e0b',
    destructive: '#ef4444',
    success: '#10b981',
  },
  rose: {
    chart1: '#ec4899', // pink (primary)
    chart2: '#0ea5e9', // sky blue (complementary cool)
    chart3: '#f59e0b', // amber (warm contrast)
    chart4: '#10b981', // emerald (natural contrast)
    primary: '#ec4899',
    destructive: '#ef4444',
    success: '#10b981',
  },
  slate: {
    chart1: '#64748b', // slate (primary)
    chart2: '#94a3b8', // light slate
    chart3: '#475569', // dark slate
    chart4: '#cbd5e1', // very light slate
    primary: '#64748b',
    destructive: '#ef4444',
    success: '#10b981',
  },
};

export function useThemeColors() {
  const { theme } = useTheme();

  const colors = computed<ChartColors>(() => THEME_COLORS[theme.value] ?? THEME_COLORS.violet);

  const primaryColor = computed(() => colors.value.primary);
  const destructiveColor = computed(() => colors.value.destructive);
  const successColor = computed(() => colors.value.success);
  const chart1Color = computed(() => colors.value.chart1);
  const chart2Color = computed(() => colors.value.chart2);
  const chart3Color = computed(() => colors.value.chart3);
  const chart4Color = computed(() => colors.value.chart4);

  return {
    primaryColor,
    destructiveColor,
    successColor,
    chart1Color,
    chart2Color,
    chart3Color,
    chart4Color,
    /** @deprecated No-op — colors are now static per theme. Kept for API compatibility. */
    refresh: () => {},
  };
}
