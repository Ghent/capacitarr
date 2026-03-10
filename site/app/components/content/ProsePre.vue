<script setup lang="ts">
/**
 * ProseCode — overrides @nuxt/ui's default code block component.
 *
 * - For `language === 'mermaid'`: renders diagrams client-side via the
 *   useMermaid composable (singleton init + serialised render queue).
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

// ─── Mermaid rendering ───────────────────────────────────────────
async function renderDiagram() {
  if (!props.code) return

  try {
    const { render } = useMermaid()
    const colorMode = useColorMode()
    const isDark = colorMode.value === 'dark'

    const svg = await render(props.code, isDark)
    mermaidSvg.value = svg
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
    const { reinitialize } = useMermaid()
    watch(() => colorMode.value, async () => {
      await nextTick()
      const isDark = colorMode.value === 'dark'
      await reinitialize(isDark)
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
/* ─── Mermaid diagram layout ──────────────────────────────────────
   Diagrams render within the content column at full width. Per project
   conventions, complex diagrams should be split into smaller focused
   diagrams (~15 nodes max) that render clearly at content column width. */
.mermaid-wrapper {
  display: flex;
  justify-content: center;
  margin: 2rem 0;
  padding: 1rem 0;
}

.mermaid-diagram {
  width: 100%;
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
