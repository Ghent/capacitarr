<script setup lang="ts">
interface Screenshot {
  src: string
  title: string
  description: string
}

const config = useRuntimeConfig()
const base = config.app.baseURL?.replace(/\/$/, '') || ''

const screenshots: Screenshot[] = [
  {
    src: `${base}/screenshots/dashboard.webp`,
    title: 'Dashboard',
    description: 'Real-time overview of your media library capacity across all disk groups',
  },
  {
    src: `${base}/screenshots/weights.webp`,
    title: 'Scoring Weights',
    description: 'Fine-tune scoring dimensions with intuitive weight sliders',
  },
  {
    src: `${base}/screenshots/custom-rules.webp`,
    title: 'Custom Rules',
    description: 'Build sophisticated cascading rules with conditions, weights, and overrides',
  },
  {
    src: `${base}/screenshots/deletion-priority.webp`,
    title: 'Deletion Priority',
    description: 'Transparent priority rankings with full score breakdowns',
  },
  {
    src: `${base}/screenshots/library.webp`,
    title: 'Library Management',
    description: 'Per-library threshold management with independent disk usage triggers',
  },
  {
    src: `${base}/screenshots/scorecard-keep.webp`,
    title: 'Score Detail — Protected',
    description: 'See exactly why an item is protected, with matched rule and override explanation',
  },
  {
    src: `${base}/screenshots/scorecard-rules.webp`,
    title: 'Score Detail — Breakdown',
    description: 'Full weighted score breakdown with every factor, custom rule modifiers, and final calculation',
  },
]

const activeIndex = ref(0)
const lightboxOpen = ref(false)
const galleryRef = ref<HTMLElement | null>(null)

function openLightbox(index: number) {
  activeIndex.value = index
  lightboxOpen.value = true
}

function closeLightbox() {
  lightboxOpen.value = false
}

function nextImage() {
  activeIndex.value = (activeIndex.value + 1) % screenshots.length
}

function prevImage() {
  activeIndex.value = (activeIndex.value - 1 + screenshots.length) % screenshots.length
}

function handleKeydown(e: KeyboardEvent) {
  if (!lightboxOpen.value) return
  if (e.key === 'Escape') closeLightbox()
  if (e.key === 'ArrowRight') nextImage()
  if (e.key === 'ArrowLeft') prevImage()
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})

// Intersection observer for scroll animation
const isVisible = ref(false)
onMounted(() => {
  if (!galleryRef.value) return
  const observer = new IntersectionObserver(
    ([entry]) => {
      if (entry.isIntersecting) {
        isVisible.value = true
        observer.disconnect()
      }
    },
    { threshold: 0.1 },
  )
  observer.observe(galleryRef.value)
})
</script>

<template>
  <div ref="galleryRef" class="screenshot-gallery">
    <!-- Featured screenshot (large) -->
    <div
      class="featured-screenshot"
      :class="{ 'animate-in': isVisible }"
      @click="openLightbox(activeIndex)"
    >
      <div class="screenshot-frame">
        <div class="screenshot-chrome">
          <div class="chrome-dots">
            <span class="dot dot-red" />
            <span class="dot dot-yellow" />
            <span class="dot dot-green" />
          </div>
          <span class="chrome-title">{{ screenshots[activeIndex].title }}</span>
        </div>
        <div class="screenshot-image-wrapper">
          <img
            :src="screenshots[activeIndex].src"
            :alt="screenshots[activeIndex].title"
            class="screenshot-image"
            width="2880"
            height="1800"
            loading="lazy"
          >
        </div>
      </div>
      <p class="featured-description">{{ screenshots[activeIndex].description }}</p>
    </div>

    <!-- Thumbnail strip -->
    <div class="thumbnail-strip" :class="{ 'animate-in': isVisible }">
      <button
        v-for="(shot, index) in screenshots"
        :key="shot.src"
        class="thumbnail"
        :class="{ active: index === activeIndex }"
        @click="activeIndex = index"
      >
        <img :src="shot.src" :alt="shot.title" loading="lazy">
        <span class="thumbnail-label">{{ shot.title }}</span>
      </button>
    </div>

    <!-- Lightbox -->
    <Teleport to="body">
      <Transition name="lightbox">
        <div v-if="lightboxOpen" class="lightbox-overlay" @click.self="closeLightbox">
          <button class="lightbox-close" @click="closeLightbox" aria-label="Close">
            <UIcon name="i-lucide-x" class="size-6" />
          </button>
          <button class="lightbox-nav lightbox-prev" @click="prevImage" aria-label="Previous">
            <UIcon name="i-lucide-chevron-left" class="size-8" />
          </button>
          <div class="lightbox-content">
            <img
              :src="screenshots[activeIndex].src"
              :alt="screenshots[activeIndex].title"
              class="lightbox-image"
            >
            <div class="lightbox-caption">
              <h3>{{ screenshots[activeIndex].title }}</h3>
              <p>{{ screenshots[activeIndex].description }}</p>
            </div>
          </div>
          <button class="lightbox-nav lightbox-next" @click="nextImage" aria-label="Next">
            <UIcon name="i-lucide-chevron-right" class="size-8" />
          </button>
          <div class="lightbox-counter">
            {{ activeIndex + 1 }} / {{ screenshots.length }}
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.screenshot-gallery {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
  max-width: 64rem;
  margin: 0 auto;
}

/* Featured screenshot */
.featured-screenshot {
  cursor: pointer;
  opacity: 0;
  transform: translateY(2rem);
  transition: opacity 0.8s ease, transform 0.8s ease;
}

.featured-screenshot.animate-in {
  opacity: 1;
  transform: translateY(0);
}

.screenshot-frame {
  border-radius: 0.75rem;
  overflow: hidden;
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.25),
    0 0 0 1px var(--color-neutral-200);
  transition: transform 0.4s cubic-bezier(0.34, 1.56, 0.64, 1), box-shadow 0.4s ease;
}

:root.dark .screenshot-frame {
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.5),
    0 0 0 1px var(--color-neutral-800),
    0 0 80px -20px rgba(139, 92, 246, 0.15);
}

.featured-screenshot:hover .screenshot-frame {
  transform: translateY(-4px) scale(1.005);
  box-shadow:
    0 30px 60px -15px rgba(0, 0, 0, 0.3),
    0 0 0 1px var(--color-neutral-200),
    0 0 40px -10px rgba(139, 92, 246, 0.2);
}

:root.dark .featured-screenshot:hover .screenshot-frame {
  box-shadow:
    0 30px 60px -15px rgba(0, 0, 0, 0.6),
    0 0 0 1px var(--color-neutral-700),
    0 0 80px -20px rgba(139, 92, 246, 0.25);
}

/* Chrome bar */
.screenshot-chrome {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.625rem 1rem;
  background: var(--color-neutral-100);
  border-bottom: 1px solid var(--color-neutral-200);
}

:root.dark .screenshot-chrome {
  background: var(--color-neutral-900);
  border-bottom-color: var(--color-neutral-800);
}

.chrome-dots {
  display: flex;
  gap: 0.375rem;
}

.dot {
  width: 0.625rem;
  height: 0.625rem;
  border-radius: 50%;
}

.dot-red { background: #ff5f57; }
.dot-yellow { background: #febc2e; }
.dot-green { background: #28c840; }

.chrome-title {
  font-size: 0.75rem;
  font-weight: 500;
  color: var(--color-neutral-500);
}

.screenshot-image-wrapper {
  background: var(--color-neutral-950);
  aspect-ratio: 16 / 10;
  overflow: hidden;
}

.screenshot-image {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: cover;
}

.featured-description {
  text-align: center;
  margin-top: 1rem;
  color: var(--color-neutral-500);
  font-size: 0.875rem;
}

/* Thumbnail strip */
.thumbnail-strip {
  display: flex;
  gap: 0.75rem;
  justify-content: center;
  flex-wrap: wrap;
  opacity: 0;
  transform: translateY(1rem);
  transition: opacity 0.8s ease 0.3s, transform 0.8s ease 0.3s;
}

.thumbnail-strip.animate-in {
  opacity: 1;
  transform: translateY(0);
}

.thumbnail {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.375rem;
  padding: 0.375rem;
  border-radius: 0.5rem;
  border: 2px solid transparent;
  background: var(--color-neutral-50);
  transition: all 0.25s ease;
  cursor: pointer;
}

:root.dark .thumbnail {
  background: var(--color-neutral-900);
}

.thumbnail:hover {
  border-color: var(--color-neutral-300);
  transform: translateY(-2px);
}

:root.dark .thumbnail:hover {
  border-color: var(--color-neutral-700);
}

.thumbnail.active {
  border-color: var(--color-violet-500);
  box-shadow: 0 0 12px -3px rgba(139, 92, 246, 0.4);
}

.thumbnail img {
  width: 7rem;
  height: auto;
  border-radius: 0.25rem;
}

.thumbnail-label {
  font-size: 0.6875rem;
  font-weight: 500;
  color: var(--color-neutral-600);
}

:root.dark .thumbnail-label {
  color: var(--color-neutral-400);
}

.thumbnail.active .thumbnail-label {
  color: var(--color-violet-600);
}

:root.dark .thumbnail.active .thumbnail-label {
  color: var(--color-violet-400);
}

/* Lightbox */
.lightbox-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.9);
  backdrop-filter: blur(8px);
}

.lightbox-close {
  position: absolute;
  top: 1rem;
  right: 1rem;
  z-index: 10;
  padding: 0.5rem;
  border-radius: 0.5rem;
  color: white;
  background: rgba(255, 255, 255, 0.1);
  transition: background 0.2s;
  cursor: pointer;
}

.lightbox-close:hover {
  background: rgba(255, 255, 255, 0.2);
}

.lightbox-nav {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  z-index: 10;
  padding: 0.75rem;
  border-radius: 0.5rem;
  color: white;
  background: rgba(255, 255, 255, 0.1);
  transition: background 0.2s;
  cursor: pointer;
}

.lightbox-nav:hover {
  background: rgba(255, 255, 255, 0.2);
}

.lightbox-prev { left: 1rem; }
.lightbox-next { right: 1rem; }

.lightbox-content {
  max-width: 90vw;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 1rem;
}

.lightbox-image {
  max-width: 100%;
  max-height: 80vh;
  object-fit: contain;
  border-radius: 0.5rem;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
}

.lightbox-caption {
  text-align: center;
  color: white;
}

.lightbox-caption h3 {
  font-size: 1.125rem;
  font-weight: 600;
  margin-bottom: 0.25rem;
}

.lightbox-caption p {
  font-size: 0.875rem;
  color: rgba(255, 255, 255, 0.7);
}

.lightbox-counter {
  position: absolute;
  bottom: 1rem;
  left: 50%;
  transform: translateX(-50%);
  color: rgba(255, 255, 255, 0.5);
  font-size: 0.875rem;
  font-variant-numeric: tabular-nums;
}

/* Lightbox transitions */
.lightbox-enter-active {
  transition: opacity 0.3s ease;
}

.lightbox-leave-active {
  transition: opacity 0.2s ease;
}

.lightbox-enter-from,
.lightbox-leave-to {
  opacity: 0;
}

@media (max-width: 640px) {
  .thumbnail img {
    width: 4rem;
  }

  .thumbnail-label {
    display: none;
  }
}
</style>
