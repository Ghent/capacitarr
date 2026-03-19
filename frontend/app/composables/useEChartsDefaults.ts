/**
 * Shared ECharts styling utilities providing glow lines, gradient fills,
 * frosted-glass tooltips, and palette generation.
 *
 * Uses theme-aware chart colors from `useThemeColors()` and dark-mode
 * detection from `useAppColorMode()`.
 */

/* ---------- private color-space helpers ---------- */

interface HSL {
  h: number;
  s: number;
  l: number;
}

function hexToHSL(hex: string): HSL {
  let r = 0;
  let g = 0;
  let b = 0;

  const stripped = hex.replace('#', '');
  // Validate hex input — return neutral gray if not a valid hex color
  if (!/^[0-9a-fA-F]{3}$|^[0-9a-fA-F]{6}$/.test(stripped)) {
    return { h: 0, s: 0, l: 50 };
  }
  if (stripped.length === 3) {
    r = parseInt(stripped[0]! + stripped[0]!, 16);
    g = parseInt(stripped[1]! + stripped[1]!, 16);
    b = parseInt(stripped[2]! + stripped[2]!, 16);
  } else {
    r = parseInt(stripped.substring(0, 2), 16);
    g = parseInt(stripped.substring(2, 4), 16);
    b = parseInt(stripped.substring(4, 6), 16);
  }

  r /= 255;
  g /= 255;
  b /= 255;

  const max = Math.max(r, g, b);
  const min = Math.min(r, g, b);
  const l = (max + min) / 2;

  let h = 0;
  let s = 0;

  if (max !== min) {
    const d = max - min;
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
    if (max === r) h = ((g - b) / d + (g < b ? 6 : 0)) / 6;
    else if (max === g) h = ((b - r) / d + 2) / 6;
    else h = ((r - g) / d + 4) / 6;
  }

  return { h: h * 360, s: s * 100, l: l * 100 };
}

function hslToHex(h: number, s: number, l: number): string {
  const sNorm = s / 100;
  const lNorm = l / 100;

  const c = (1 - Math.abs(2 * lNorm - 1)) * sNorm;
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1));
  const m = lNorm - c / 2;

  let r = 0;
  let g = 0;
  let b = 0;

  if (h < 60) {
    r = c;
    g = x;
  } else if (h < 120) {
    r = x;
    g = c;
  } else if (h < 180) {
    g = c;
    b = x;
  } else if (h < 240) {
    g = x;
    b = c;
  } else if (h < 300) {
    r = x;
    b = c;
  } else {
    r = c;
    b = x;
  }

  const toHex = (v: number) =>
    Math.round((v + m) * 255)
      .toString(16)
      .padStart(2, '0');

  return `#${toHex(r)}${toHex(g)}${toHex(b)}`;
}

/**
 * Convert a hex color + alpha (0–1) to an rgba() string.
 * ECharts does not reliably support 8-digit hex (#RRGGBBAA),
 * so we must use rgba() for any color with transparency.
 *
 * Defensively validates the input: if the string is not a valid
 * hex color (e.g. if an oklch/color() string leaked through),
 * returns a transparent black fallback to avoid Canvas NaN errors.
 */
function hexToRgba(hex: string, alpha: number): string {
  const stripped = hex.replace('#', '');
  // Validate that we have a 3- or 6-digit hex string
  if (!/^[0-9a-fA-F]{3}$|^[0-9a-fA-F]{6}$/.test(stripped)) {
    return `rgba(0,0,0,${alpha})`;
  }
  let r: number, g: number, b: number;
  if (stripped.length === 3) {
    r = parseInt(stripped[0]! + stripped[0]!, 16);
    g = parseInt(stripped[1]! + stripped[1]!, 16);
    b = parseInt(stripped[2]! + stripped[2]!, 16);
  } else {
    r = parseInt(stripped.substring(0, 2), 16);
    g = parseInt(stripped.substring(2, 4), 16);
    b = parseInt(stripped.substring(4, 6), 16);
  }
  return `rgba(${r},${g},${b},${alpha})`;
}

/* ---------- composable ---------- */

export function useEChartsDefaults() {
  const { isDark } = useAppColorMode();
  const { chart1Color, chart2Color, chart3Color, chart4Color, destructiveColor, successColor } =
    useThemeColors();

  /** Line style with glow shadow. */
  function glowLineStyle(color: string, width = 2) {
    return { width, color, shadowBlur: 8, shadowColor: hexToRgba(color, 0.5) };
  }

  /** 3-stop vertical gradient fill for area charts. */
  function gradientArea(color: string) {
    return {
      color: {
        type: 'linear',
        x: 0,
        y: 0,
        x2: 0,
        y2: 1,
        colorStops: [
          { offset: 0, color: hexToRgba(color, 0.4) },
          { offset: 0.6, color: hexToRgba(color, 0.15) },
          { offset: 1, color: hexToRgba(color, 0.02) },
        ],
      },
    };
  }

  /** Horizontal gradient for bar charts. */
  function gradientBar(color: string) {
    return {
      type: 'linear',
      x: 0,
      y: 0,
      x2: 1,
      y2: 0,
      colorStops: [
        { offset: 0, color: color },
        { offset: 1, color: hexToRgba(color, 0.6) },
      ],
    };
  }

  /** Frosted glass tooltip configuration. */
  function tooltipConfig() {
    return {
      backgroundColor: isDark.value ? 'rgba(24,24,27,0.85)' : 'rgba(255,255,255,0.92)',
      borderColor: isDark.value ? 'rgba(63,63,70,0.6)' : 'rgba(228,228,231,0.8)',
      textStyle: {
        color: isDark.value ? '#fafafa' : '#18181b',
        fontSize: 12,
      },
      extraCssText:
        'backdrop-filter: blur(8px); border-radius: 8px; box-shadow: 0 4px 12px rgba(0,0,0,0.15);',
    };
  }

  /** Emphasis focus on series hover. */
  function emphasisConfig() {
    return { focus: 'series' as const, blurScope: 'coordinateSystem' as const };
  }

  /**
   * Generate N harmonious colors from a base hex color.
   * Spreads hues across a ~120° analogous arc.
   */
  function generatePalette(baseHex: string, count: number): string[] {
    const hsl = hexToHSL(baseHex);
    const arc = Math.min(120, count * 25);
    return Array.from({ length: count }, (_, i) => {
      const hue = (hsl.h + (i * arc) / Math.max(count, 1)) % 360;
      const lightness = isDark.value ? 55 + (i % 3) * 8 : 45 + (i % 3) * 8;
      return hslToHex(hue, hsl.s, lightness);
    });
  }

  /** Convert hex color + alpha (0–1) to rgba() string for ECharts. */
  function colorAlpha(hex: string, alpha: number): string {
    return hexToRgba(hex, alpha);
  }

  return {
    chart1Color,
    chart2Color,
    chart3Color,
    chart4Color,
    destructiveColor,
    successColor,
    glowLineStyle,
    gradientArea,
    gradientBar,
    tooltipConfig,
    emphasisConfig,
    generatePalette,
    colorAlpha,
  };
}
