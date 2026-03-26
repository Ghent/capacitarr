<script setup lang="ts">
interface Integration {
  name: string
  icon: string
  color: string
}

const integrations: Integration[] = [
  { name: 'Sonarr', icon: 'i-simple-icons-sonarr', color: '#35c5f4' },
  { name: 'Radarr', icon: 'i-simple-icons-radarr', color: '#ffc230' },
  { name: 'Lidarr', icon: 'i-lucide-music', color: '#00bc8c' },
  { name: 'Readarr', icon: 'i-lucide-book-open', color: '#8B5CF6' },
  { name: 'Plex', icon: 'i-simple-icons-plex', color: '#e5a00d' },
  { name: 'Jellyfin', icon: 'i-simple-icons-jellyfin', color: '#00a4dc' },
  { name: 'Emby', icon: 'i-simple-icons-emby', color: '#52b54b' },
  { name: 'Tautulli', icon: 'i-lucide-activity', color: '#e5a00d' },
  { name: 'Jellystat', icon: 'i-lucide-bar-chart-3', color: '#00a4dc' },
  { name: 'Tracearr', icon: 'i-lucide-scan-line', color: '#4ade80' },
  { name: 'Seerr', icon: 'i-lucide-ticket', color: '#7b68ee' },
]

// Double the list for seamless infinite scroll
const marqueeItems = [...integrations, ...integrations]
</script>

<template>
  <div class="marquee-container">
    <div class="marquee-track">
      <div
        v-for="(integration, index) in marqueeItems"
        :key="`${integration.name}-${index}`"
        class="marquee-item"
        :style="{ '--accent': integration.color }"
      >
        <div class="marquee-icon-wrapper">
          <UIcon :name="integration.icon" class="size-6" />
        </div>
        <span class="marquee-name">{{ integration.name }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.marquee-container {
  overflow: hidden;
  mask-image: linear-gradient(to right, transparent 0%, black 10%, black 90%, transparent 100%);
  padding: 0.5rem 0;
}

.marquee-track {
  display: flex;
  gap: 2rem;
  animation: marquee 30s linear infinite;
  width: max-content;
}

.marquee-container:hover .marquee-track {
  animation-play-state: paused;
}

@keyframes marquee {
  0% { transform: translateX(0); }
  100% { transform: translateX(-50%); }
}

.marquee-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 1rem;
  border-radius: 0.5rem;
  background: var(--color-neutral-100);
  border: 1px solid var(--color-neutral-200);
  transition: all 0.3s cubic-bezier(0.34, 1.56, 0.64, 1);
  cursor: default;
  white-space: nowrap;
}

:root.dark .marquee-item {
  background: var(--color-neutral-900);
  border-color: var(--color-neutral-800);
}

.marquee-item:hover {
  transform: translateY(-2px) scale(1.05);
  border-color: color-mix(in srgb, var(--accent) 40%, transparent);
  box-shadow: 0 4px 15px -3px color-mix(in srgb, var(--accent) 20%, transparent);
}

.marquee-icon-wrapper {
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--accent);
}

.marquee-name {
  font-size: 0.8125rem;
  font-weight: 500;
  color: var(--color-neutral-600);
}

:root.dark .marquee-name {
  color: var(--color-neutral-400);
}
</style>
