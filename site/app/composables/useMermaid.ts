/**
 * useMermaid — singleton composable for rendering Mermaid diagrams.
 *
 * Solves the "mermaid is a global singleton" problem by:
 *   1. Initialising mermaid + ELK layout loader exactly once.
 *   2. Serialising render() calls through a promise queue so only one
 *      mermaid.render() is in-flight at a time — preventing async ELK
 *      layout interleaving that causes diagrams to overlap.
 *   3. Exposing a reinitialise path for colour-mode changes that
 *      doesn't race with in-progress renders.
 */

// ─── Module-level singletons (shared across all component instances) ─
let mermaidInstance: typeof import('mermaid').default | null = null
let initPromise: Promise<void> | null = null
let renderCounter = 0

// Promise-based serial queue: each render waits for the previous one.
let renderChain: Promise<void> = Promise.resolve()

// ─── Theme palettes ──────────────────────────────────────────────────
const darkTheme = {
  primaryColor: '#1e1b4b',
  primaryBorderColor: '#8b5cf6',
  primaryTextColor: '#e9d5ff',
  lineColor: '#a78bfa',
  secondaryColor: '#110f24',
  tertiaryColor: '#110f24',
  mainBkg: '#1e1b4b',
  nodeBorder: '#8b5cf6',
  clusterBkg: '#110f24',
  clusterBorder: '#4c1d95',
  titleColor: '#e9d5ff',
  edgeLabelBackground: '#110f24',
  textColor: '#e9d5ff',
}

const lightTheme = {
  primaryColor: '#ede9fe',
  primaryBorderColor: '#8b5cf6',
  primaryTextColor: '#1e1b4b',
  lineColor: '#6d28d9',
  secondaryColor: '#f5f3ff',
  tertiaryColor: '#f5f3ff',
  mainBkg: '#ede9fe',
  nodeBorder: '#8b5cf6',
  clusterBkg: '#f5f3ff',
  clusterBorder: '#c4b5fd',
  titleColor: '#1e1b4b',
  edgeLabelBackground: '#f5f3ff',
  textColor: '#1e1b4b',
}

// ─── Internal helpers ────────────────────────────────────────────────

function buildConfig(isDark: boolean) {
  const theme = isDark ? darkTheme : lightTheme
  return {
    startOnLoad: false,
    theme: 'base' as const,
    themeVariables: theme,
    flowchart: {
      defaultRenderer: 'elk' as const,
      nodeSpacing: 60,
      rankSpacing: 70,
      padding: 20,
      curve: 'basis' as const,
    },
    sequence: {
      actorMargin: 80,
    },
    fontFamily: '\'Geist Sans\', \'Geist\', ui-sans-serif, system-ui, sans-serif',
    themeCSS: `
      .node rect, .node polygon, .node circle, .node ellipse { rx: 8; ry: 8; }
      .cluster rect { rx: 12; ry: 12; }
      .edgeLabel { font-size: 12px; }
      .edgeLabel span {
        padding: 2px 8px;
        border-radius: 10px;
        border: 1px solid ${theme.lineColor};
      }
    `,
  }
}

/**
 * Ensures mermaid + ELK are loaded and initialised exactly once.
 * Subsequent calls return the same promise.
 */
async function ensureInit(isDark: boolean): Promise<void> {
  if (initPromise) return initPromise

  initPromise = (async () => {
    const mermaid = (await import('mermaid')).default
    const elkModule = await import('@mermaid-js/layout-elk')
    mermaid.registerLayoutLoaders(elkModule.default || elkModule)
    mermaid.initialize(buildConfig(isDark))
    mermaidInstance = mermaid
  })()

  return initPromise
}

/**
 * Strip inline width/height attributes so the SVG scales responsively
 * via its viewBox. Mermaid sets explicit pixel dimensions that prevent
 * CSS-based responsive sizing.
 */
function stripSvgDimensions(svg: string): string {
  return svg
    .replace(/(<svg[^>]*?)\swidth="[^"]*"/, '$1')
    .replace(/(<svg[^>]*?)\sheight="[^"]*"/, '$1')
}

// ─── Public API ──────────────────────────────────────────────────────

export function useMermaid() {
  /**
   * Render a mermaid diagram. Calls are serialised so only one
   * mermaid.render() runs at a time, preventing ELK layout interleaving.
   *
   * @returns The rendered SVG string with responsive dimensions.
   * @throws  Re-throws mermaid render errors for the caller to display.
   */
  async function render(code: string, isDark: boolean): Promise<string> {
    // Wait for initialisation (no-ops after the first call)
    await ensureInit(isDark)

    // Enqueue this render after all previously queued renders.
    // We capture the current chain tail and extend it.
    return new Promise<string>((resolve, reject) => {
      renderChain = renderChain.then(async () => {
        try {
          renderCounter++
          const id = `mermaid-${renderCounter}-${Date.now()}`
          const { svg } = await mermaidInstance!.render(id, code)
          resolve(stripSvgDimensions(svg))
        }
        catch (err) {
          reject(err)
        }
      })
    })
  }

  /**
   * Re-initialise mermaid with the opposite colour-mode theme and
   * return. Callers should re-render all active diagrams after this.
   *
   * Waits for any in-flight renders to finish before reinitialising
   * so we never mutate global state mid-render.
   */
  async function reinitialize(isDark: boolean): Promise<void> {
    // Drain the render queue first
    await renderChain

    if (mermaidInstance) {
      mermaidInstance.initialize(buildConfig(isDark))
    }
  }

  return { render, reinitialize }
}
