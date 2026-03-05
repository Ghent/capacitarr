/**
 * Color mode composable supporting light, dark, and system preferences.
 * Persists user preference in localStorage and applies 'dark' class to <html>.
 * When set to 'system', follows the OS/browser prefers-color-scheme media query.
 */
export type ColorModePreference = 'light' | 'dark' | 'system';

export const useAppColorMode = () => {
  const preference = useState<ColorModePreference>('colorModePreference', () => {
    if (import.meta.client) {
      const stored = localStorage.getItem('capacitarr-color-mode');
      if (stored === 'dark' || stored === 'light' || stored === 'system') return stored;
      return 'system'; // Default to system preference
    }
    return 'dark'; // Default for SSR/initial
  });

  const isDark = computed(() => {
    if (preference.value === 'system') {
      if (import.meta.client) {
        return window.matchMedia('(prefers-color-scheme: dark)').matches;
      }
      return true; // Assume dark for SSR
    }
    return preference.value === 'dark';
  });

  function setMode(newMode: ColorModePreference) {
    preference.value = newMode;
    apply();
  }

  function apply() {
    if (!import.meta.client) return;
    document.documentElement.classList.toggle('dark', isDark.value);
    localStorage.setItem('capacitarr-color-mode', preference.value);
  }

  // Apply on first client-side load
  if (import.meta.client) {
    apply();

    // Listen for OS theme changes when in system mode
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
      if (preference.value === 'system') {
        apply();
      }
    });
  }

  return { mode: preference, isDark, setMode };
};
