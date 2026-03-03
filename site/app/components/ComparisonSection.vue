<script setup lang="ts">
const containerRef = ref<HTMLElement | null>(null)
const isVisible = ref(false)

onMounted(() => {
  if (!containerRef.value) return
  const observer = new IntersectionObserver(
    ([entry]) => {
      if (entry.isIntersecting) {
        isVisible.value = true
        observer.disconnect()
      }
    },
    { threshold: 0.15 },
  )
  observer.observe(containerRef.value)
})

const before = [
  'Manually check disk space across servers',
  'Guess which shows nobody watches anymore',
  'Delete random files when storage is full',
  'Hope you didn\'t remove something important',
  'No history of what was removed or why',
]

const after = [
  'Automatic monitoring across all disk groups',
  'Data-driven scoring from Plex/Jellyfin watch history',
  'Intelligent priority ranking across 6 dimensions',
  'Preview mode and safety guards prevent accidents',
  'Full audit log of every decision and action',
]
</script>

<template>
  <div ref="containerRef" class="comparison">
    <div class="comparison-grid" :class="{ visible: isVisible }">
      <!-- Before -->
      <div class="comparison-card comparison-before">
        <div class="comparison-header comparison-header-before">
          <UIcon name="i-lucide-frown" class="size-5" />
          <h3>Without Capacitarr</h3>
        </div>
        <ul class="comparison-list">
          <li
            v-for="(item, i) in before"
            :key="i"
            class="comparison-item"
            :style="{ '--delay': `${i * 80 + 200}ms` }"
          >
            <UIcon name="i-lucide-x" class="size-4 text-red-500 shrink-0 mt-0.5" />
            <span>{{ item }}</span>
          </li>
        </ul>
      </div>

      <!-- After -->
      <div class="comparison-card comparison-after">
        <div class="comparison-header comparison-header-after">
          <UIcon name="i-lucide-sparkles" class="size-5" />
          <h3>With Capacitarr</h3>
        </div>
        <ul class="comparison-list">
          <li
            v-for="(item, i) in after"
            :key="i"
            class="comparison-item"
            :style="{ '--delay': `${i * 80 + 200}ms` }"
          >
            <UIcon name="i-lucide-check" class="size-4 text-emerald-500 shrink-0 mt-0.5" />
            <span>{{ item }}</span>
          </li>
        </ul>
      </div>
    </div>
  </div>
</template>

<style scoped>
.comparison-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1.5rem;
  max-width: 56rem;
  margin: 0 auto;
}

@media (max-width: 768px) {
  .comparison-grid {
    grid-template-columns: 1fr;
  }
}

.comparison-card {
  padding: 1.5rem;
  border-radius: 0.75rem;
  border: 1px solid var(--color-neutral-200);
  opacity: 0;
  transform: translateY(1.5rem);
  transition: opacity 0.6s ease, transform 0.6s ease;
}

:root.dark .comparison-card {
  border-color: var(--color-neutral-800);
}

.comparison-grid.visible .comparison-card {
  opacity: 1;
  transform: translateY(0);
}

.comparison-before {
  background: var(--color-neutral-50);
  transition-delay: 0ms;
}

:root.dark .comparison-before {
  background: var(--color-neutral-900);
}

.comparison-after {
  background: linear-gradient(135deg, color-mix(in srgb, var(--color-violet-50) 50%, white), var(--color-neutral-50));
  border-color: var(--color-violet-200);
  transition-delay: 150ms;
}

:root.dark .comparison-after {
  background: linear-gradient(135deg, color-mix(in srgb, var(--color-violet-950) 50%, var(--color-neutral-950)), var(--color-neutral-900));
  border-color: var(--color-violet-900);
}

.comparison-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 1rem;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid var(--color-neutral-200);
}

:root.dark .comparison-header {
  border-bottom-color: var(--color-neutral-800);
}

.comparison-header h3 {
  font-size: 1rem;
  font-weight: 600;
}

.comparison-header-before {
  color: var(--color-neutral-500);
}

.comparison-header-after {
  color: var(--color-violet-600);
}

:root.dark .comparison-header-after {
  color: var(--color-violet-400);
}

.comparison-list {
  display: flex;
  flex-direction: column;
  gap: 0.625rem;
}

.comparison-item {
  display: flex;
  gap: 0.5rem;
  font-size: 0.875rem;
  line-height: 1.5;
  color: var(--color-neutral-600);
  opacity: 0;
  transform: translateX(-0.5rem);
  transition: opacity 0.4s ease var(--delay), transform 0.4s ease var(--delay);
}

:root.dark .comparison-item {
  color: var(--color-neutral-400);
}

.comparison-grid.visible .comparison-item {
  opacity: 1;
  transform: translateX(0);
}
</style>
