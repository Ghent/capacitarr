<script setup lang="ts">
interface Stat {
  value: number
  suffix?: string
  label: string
  icon: string
}

const stats: Stat[] = [
  { value: 9, suffix: '+', label: 'Integrations', icon: 'i-lucide-plug' },
  { value: 6, label: 'Scoring Dimensions', icon: 'i-lucide-sliders-horizontal' },
  { value: 100, suffix: '%', label: 'Open Source', icon: 'i-lucide-code-2' },
  { value: 0, suffix: '', label: 'Required Cloud Services', icon: 'i-lucide-cloud-off' },
]

const containerRef = ref<HTMLElement | null>(null)
const isVisible = ref(false)
const displayValues = ref(stats.map(() => 0))

function animateCount(index: number, target: number, duration = 1500) {
  const start = performance.now()
  const step = (timestamp: number) => {
    const progress = Math.min((timestamp - start) / duration, 1)
    const eased = 1 - Math.pow(1 - progress, 3) // easeOutCubic
    displayValues.value[index] = Math.round(eased * target)
    if (progress < 1) {
      requestAnimationFrame(step)
    }
  }
  requestAnimationFrame(step)
}

onMounted(() => {
  if (!containerRef.value) return
  const observer = new IntersectionObserver(
    ([entry]) => {
      if (entry.isIntersecting) {
        isVisible.value = true
        stats.forEach((stat, i) => {
          setTimeout(() => animateCount(i, stat.value), i * 150)
        })
        observer.disconnect()
      }
    },
    { threshold: 0.3 },
  )
  observer.observe(containerRef.value)
})
</script>

<template>
  <div ref="containerRef" class="stats-grid">
    <div
      v-for="(stat, index) in stats"
      :key="stat.label"
      class="stat-card"
      :class="{ visible: isVisible }"
      :style="{ '--delay': `${index * 100}ms` }"
    >
      <div class="stat-icon">
        <UIcon :name="stat.icon" class="size-5" />
      </div>
      <div class="stat-value">
        {{ displayValues[index] }}{{ stat.suffix }}
      </div>
      <div class="stat-label">{{ stat.label }}</div>
    </div>
  </div>
</template>

<style scoped>
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 1.5rem;
  max-width: 56rem;
  margin: 0 auto;
}

@media (max-width: 768px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

.stat-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  padding: 1.5rem 1rem;
  border-radius: 0.75rem;
  background: var(--color-neutral-50);
  border: 1px solid var(--color-neutral-200);
  opacity: 0;
  transform: translateY(1.5rem);
  transition: all 0.6s cubic-bezier(0.34, 1.56, 0.64, 1) var(--delay);
}

:root.dark .stat-card {
  background: var(--color-neutral-900);
  border-color: var(--color-neutral-800);
}

.stat-card.visible {
  opacity: 1;
  transform: translateY(0);
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 25px -5px rgba(139, 92, 246, 0.15);
  border-color: var(--color-violet-300);
}

:root.dark .stat-card:hover {
  border-color: var(--color-violet-800);
  box-shadow: 0 8px 25px -5px rgba(139, 92, 246, 0.2);
}

.stat-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.5rem;
  height: 2.5rem;
  border-radius: 0.5rem;
  background: var(--color-violet-100);
  color: var(--color-violet-600);
}

:root.dark .stat-icon {
  background: var(--color-violet-950);
  color: var(--color-violet-400);
}

.stat-value {
  font-size: 2rem;
  font-weight: 700;
  letter-spacing: -0.025em;
  font-variant-numeric: tabular-nums;
  background: linear-gradient(135deg, var(--color-violet-600), var(--color-violet-400));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.stat-label {
  font-size: 0.8125rem;
  font-weight: 500;
  color: var(--color-neutral-500);
  text-align: center;
}
</style>
