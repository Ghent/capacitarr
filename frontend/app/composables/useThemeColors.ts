/**
 * Resolves CSS custom property oklch values to hex colors for use in
 * libraries that don't support oklch (e.g., ApexCharts).
 *
 * Creates a hidden element, applies the CSS variable as background-color,
 * then reads the computed rgb value from the browser.
 */
export function useThemeColors() {
  const primaryColor = ref('#8b5cf6')
  const destructiveColor = ref('#ef4444')
  const successColor = ref('#10b981')

  function resolveColor(cssVar: string, fallback: string): string {
    if (typeof document === 'undefined') return fallback
    const el = document.createElement('div')
    el.style.display = 'none'
    el.style.color = `var(${cssVar})`
    document.body.appendChild(el)
    const computed = getComputedStyle(el).color
    document.body.removeChild(el)
    // computed returns "rgb(r, g, b)" or "oklch(...)" depending on browser
    if (computed && computed !== '' && computed !== 'rgba(0, 0, 0, 0)') {
      return rgbToHex(computed)
    }
    return fallback
  }

  function rgbToHex(rgb: string): string {
    // Handle "rgb(r, g, b)" or "rgba(r, g, b, a)"
    const match = rgb.match(/rgba?\((\d+),\s*(\d+),\s*(\d+)/)
    if (!match) return rgb // Return as-is if not parseable
    const r = parseInt(match[1]!)
    const g = parseInt(match[2]!)
    const b = parseInt(match[3]!)
    return `#${((1 << 24) + (r << 16) + (g << 8) + b).toString(16).slice(1)}`
  }

  function refresh() {
    primaryColor.value = resolveColor('--color-primary', '#8b5cf6')
    destructiveColor.value = resolveColor('--color-destructive', '#ef4444')
    successColor.value = resolveColor('--color-success', '#10b981')
  }

  // Resolve on mount
  onMounted(() => {
    refresh()
  })

  // Re-resolve when theme changes (watch for data-theme attribute changes)
  if (typeof window !== 'undefined') {
    const observer = new MutationObserver(() => {
      nextTick(() => refresh())
    })
    onMounted(() => {
      observer.observe(document.documentElement, {
        attributes: true,
        attributeFilter: ['data-theme', 'class']
      })
    })
    onBeforeUnmount(() => {
      observer.disconnect()
    })
  }

  return {
    primaryColor: readonly(primaryColor),
    destructiveColor: readonly(destructiveColor),
    successColor: readonly(successColor),
    refresh
  }
}
