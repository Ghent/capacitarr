<script setup lang="ts">
interface FaqItem {
  question: string
  answer: string
}

const faqs: FaqItem[] = [
  {
    question: 'Will Capacitarr delete my media without asking?',
    answer: 'No. Capacitarr starts in dry-run mode by default — it shows you exactly what would be deleted without taking any action. You can switch to approval mode (requires manual confirmation) or auto mode when you\'re confident in your configuration.',
  },
  {
    question: 'What integrations are supported?',
    answer: 'Sonarr, Radarr, Lidarr, Readarr (content managers), Plex, Jellyfin, Emby (media servers), Tautulli, Jellystat, Tracearr (watch analytics), and Seerr (media requests). All integrations are optional — use as many or as few as you need.',
  },
  {
    question: 'Can I protect specific content from deletion?',
    answer: 'Yes. The cascading rule builder lets you create rules that mark content as "always keep" based on any combination of quality, tags, genre, year, network, or custom properties. Protected content is never scored for deletion.',
  },
  {
    question: 'How is deletion priority calculated?',
    answer: 'Capacitarr uses a weighted scoring engine with 6 dimensions: age, file size, popularity, recency, rating, and series status. You control the weight of each factor. Items with the highest cleanup scores are removed first.',
  },
  {
    question: 'Does it work with multiple disk groups?',
    answer: 'Yes. Capacitarr automatically detects disk groups from the root folders reported by your *arr integrations. If your movies and TV shows live on different drives, they appear as separate disk groups on the dashboard — each with their own capacity tracking, thresholds, and targets.',
  },
  {
    question: 'Is there an API?',
    answer: 'Yes. Capacitarr has a full REST API (documented with OpenAPI/Swagger) that you can use for automation, monitoring, or integration with other tools. API keys are supported for programmatic access.',
  },
]

const openIndex = ref<number | null>(null)

function toggle(index: number) {
  openIndex.value = openIndex.value === index ? null : index
}
</script>

<template>
  <div class="faq-list">
    <div
      v-for="(faq, index) in faqs"
      :key="index"
      class="faq-item"
      :class="{ open: openIndex === index }"
    >
      <button
        class="faq-trigger"
        @click="toggle(index)"
        :aria-expanded="openIndex === index"
      >
        <span class="faq-question">{{ faq.question }}</span>
        <UIcon
          name="i-lucide-chevron-down"
          class="faq-chevron size-4"
          :class="{ rotated: openIndex === index }"
        />
      </button>
      <Transition name="faq-expand">
        <div v-if="openIndex === index" class="faq-answer-wrapper">
          <p class="faq-answer">{{ faq.answer }}</p>
        </div>
      </Transition>
    </div>
  </div>
</template>

<style scoped>
.faq-list {
  max-width: 40rem;
  margin: 0 auto;
  display: flex;
  flex-direction: column;
}

.faq-item {
  border-bottom: 1px solid var(--color-neutral-200);
}

:root.dark .faq-item {
  border-bottom-color: var(--color-neutral-800);
}

.faq-item:first-child {
  border-top: 1px solid var(--color-neutral-200);
}

:root.dark .faq-item:first-child {
  border-top-color: var(--color-neutral-800);
}

.faq-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 1rem 0;
  text-align: left;
  cursor: pointer;
  background: none;
  border: none;
  gap: 1rem;
}

.faq-question {
  font-size: 0.9375rem;
  font-weight: 500;
  color: var(--color-neutral-900);
  transition: color 0.2s;
}

:root.dark .faq-question {
  color: var(--color-neutral-100);
}

.faq-trigger:hover .faq-question {
  color: var(--color-violet-600);
}

:root.dark .faq-trigger:hover .faq-question {
  color: var(--color-violet-400);
}

.faq-chevron {
  color: var(--color-neutral-400);
  transition: transform 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
  flex-shrink: 0;
}

.faq-chevron.rotated {
  transform: rotate(180deg);
}

.faq-answer-wrapper {
  overflow: hidden;
}

.faq-answer {
  font-size: 0.875rem;
  line-height: 1.6;
  color: var(--color-neutral-500);
  padding-bottom: 1rem;
}

/* Accordion transition */
.faq-expand-enter-active {
  transition: all 0.3s ease;
  max-height: 300px;
}

.faq-expand-leave-active {
  transition: all 0.2s ease;
  max-height: 300px;
}

.faq-expand-enter-from,
.faq-expand-leave-to {
  opacity: 0;
  max-height: 0;
}
</style>
