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
 * Pre-computed hex equivalents of the oklch chart colors from main.css.
 * Generated via oklch→sRGB→hex conversion. If you change the oklch values
 * in main.css, regenerate these with the conversion script in the commit
 * that introduced this table.
 */
const THEME_COLORS: Record<ThemeId, ChartColors> = {
  violet: {
    chart1: '#8e51ff',
    chart2: '#6054ec',
    chart3: '#a96de6',
    chart4: '#c77dd8',
    primary: '#8e51ff',
    destructive: '#e7000b',
    success: '#00bc7d',
  },
  ocean: {
    chart1: '#0080ce',
    chart2: '#0077a2',
    chart3: '#0086d8',
    chart4: '#00a7b1',
    primary: '#0080ce',
    destructive: '#e7000b',
    success: '#00bc7d',
  },
  emerald: {
    chart1: '#00a056',
    chart2: '#098926',
    chart3: '#00ab8a',
    chart4: '#83ae53',
    primary: '#00a056',
    destructive: '#e7000b',
    success: '#00bc7d',
  },
  sunset: {
    chart1: '#f07900',
    chart2: '#dd6836',
    chart3: '#e79b3d',
    chart4: '#dd4115',
    primary: '#f07900',
    destructive: '#e7000b',
    success: '#00bc7d',
  },
  rose: {
    chart1: '#d72f92',
    chart2: '#b4319b',
    chart3: '#e45580',
    chart4: '#f16f7e',
    primary: '#d72f92',
    destructive: '#e7000b',
    success: '#00bc7d',
  },
  slate: {
    chart1: '#4c5666',
    chart2: '#3e4955',
    chart3: '#6c7181',
    chart4: '#77828c',
    primary: '#4c5666',
    destructive: '#e7000b',
    success: '#00bc7d',
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
