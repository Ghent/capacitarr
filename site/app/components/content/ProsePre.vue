<script setup lang="ts">
/**
 * ProseCode — overrides @nuxt/ui's default code block component.
 *
 * - For `language === 'mermaid'`: renders diagrams client-side via mermaid.render()
 *   with violet-branded dark/light themes and the ELK layout engine.
 *   Diagrams use a breakout layout to extend beyond the content column
 *   for improved readability of complex diagrams.
 * - For all other languages: delegates to Nuxt UI's built-in ProsePre
 *   component for proper themed styling, copy button, and syntax highlighting.
 */
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import NuxtUIPre from '@nuxt/ui/components/prose/Pre.vue'

const props = defineProps<{
  code?: string
  language?: string
  filename?: string
  highlights?: number[]
  meta?: string
}>()

const isMermaid = computed(() => props.language === 'mermaid')

// ─── Mermaid state (client-only) ─────────────────────────────────
const mermaidSvg = ref('')
const renderError = ref('')
let renderCount = 0
let elkRegistered = false

// ─── Theme palettes ──────────────────────────────────────────────
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

// ─── Mermaid rendering ───────────────────────────────────────────
async function renderDiagram() {
  if (!props.code) return

  try {
    const mermaid = (await import('mermaid')).default

    // Register ELK layout engine once
    if (!elkRegistered) {
      const elkModule = await import('@mermaid-js/layout-elk')
      mermaid.registerLayoutLoaders(elkModule.default || elkModule)
      elkRegistered = true
    }

    const colorMode = useColorMode()
    const isDark = colorMode.value === 'dark'

    mermaid.initialize({
      startOnLoad: false,
      theme: 'base',
      themeVariables: isDark ? darkTheme : lightTheme,
      flowchart: {
        defaultRenderer: 'elk',
        nodeSpacing: 60,
        rankSpacing: 70,
        padding: 20,
        curve: 'basis',
      },
      sequence: {
        actorMargin: 80,
      },
      fontFamily: '\'Geist Sans\', \'Geist\', ui-sans-serif, system-ui, sans-serif',
      themeCSS: `
        .node rect, .node polygon, .node circle, .node ellipse { rx: 8; ry: 8; }
        .cluster rect { rx: 12; ry: 12; }
        .edgeLabel { font-size: 12px; }
        .edgeLabel rect {
          rx: 10;
          ry: 10;
          stroke: ${isDark ? darkTheme.lineColor : lightTheme.lineColor};
          stroke-width: 1px;
          fill-opacity: 0.95;
        }
      `,
    })

    // Use a unique ID per render pass to avoid mermaid ID collisions
    renderCount++
    const { svg } = await mermaid.render(`mermaid-${renderCount}-${Date.now()}`, props.code)

    // Strip inline width/height attributes so the SVG scales responsively
    // via its viewBox. Mermaid sets explicit pixel dimensions that prevent
    // CSS-based responsive sizing.
    mermaidSvg.value = svg
      .replace(/(<svg[^>]*?)\swidth="[^"]*"/, '$1')
      .replace(/(<svg[^>]*?)\sheight="[^"]*"/, '$1')
    renderError.value = ''
  }
  catch (err) {
    renderError.value = String(err)
    console.error('[Mermaid] Render error:', err)
  }
}

// ─── Lifecycle ───────────────────────────────────────────────────
if (isMermaid.value) {
  onMounted(async () => {
    await renderDiagram()

    // Re-render when color mode changes
    const colorMode = useColorMode()
    watch(() => colorMode.value, async () => {
      await nextTick()
      await renderDiagram()
    })
  })
}
</script>

<template>
  <!-- Mermaid diagram: client-only rendering with breakout layout -->
  <ClientOnly v-if="isMermaid">
    <div class="mermaid-wrapper">
      <div
        v-if="mermaidSvg"
        class="mermaid-diagram"
        v-html="mermaidSvg"
      />
      <div v-else-if="renderError" class="mermaid-error">
        <p><strong>Diagram render error:</strong></p>
        <pre>{{ renderError }}</pre>
      </div>
      <div v-else class="mermaid-loading">
        <UIcon name="i-lucide-loader-2" class="size-5 animate-spin" />
        <span>Rendering diagram…</span>
      </div>
    </div>
    <template #fallback>
      <div class="mermaid-wrapper mermaid-fallback">
        <pre><code>{{ code }}</code></pre>
      </div>
    </template>
  </ClientOnly>

  <!-- Non-mermaid code: delegate to Nuxt UI's themed ProsePre component -->
  <NuxtUIPre
    v-else
    :code="code"
    :language="language"
    :filename="filename"
    :highlights="highlights"
    :meta="meta"
  >
    <slot />
  </NuxtUIPre>
</template>

<style scoped>
/* ─── Mermaid breakout layout ─────────────────────────────────────
   The diagram breaks out of the content column to use more horizontal
   space. On large screens it extends ~8rem beyond each side of the
   content area. On small screens it stays within the viewport. */
.mermaid-wrapper {
  display: flex;
  justify-content: center;
  margin: 2rem -8rem;
  padding: 1.5rem 1rem;
  overflow-x: auto;
}

/* On screens narrower than 1280px (lg breakpoint), don't break out —
   stay within the content column to avoid horizontal overflow. */
@media (max-width: 1279px) {
  .mermaid-wrapper {
    margin-left: 0;
    margin-right: 0;
  }
}

.mermaid-diagram {
  width: 100%;
  max-width: 1100px;
}

.mermaid-diagram :deep(svg) {
  width: 100%;
  height: auto;
}

.mermaid-loading {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--color-neutral-500);
  font-size: 0.875rem;
  padding: 2rem;
}

.mermaid-error {
  color: var(--color-red-600);
  font-size: 0.875rem;
  padding: 1rem;
}

:root.dark .mermaid-error {
  color: var(--color-red-400);
}

.mermaid-error pre {
  margin-top: 0.5rem;
  white-space: pre-wrap;
  font-size: 0.75rem;
}

.mermaid-fallback {
  opacity: 0.7;
}

.mermaid-fallback pre {
  font-size: 0.8125rem;
  white-space: pre-wrap;
}
</style>
