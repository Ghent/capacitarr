<script setup lang="ts">
interface RepoStatsData {
  stars: number
  forks: number
  version: string | null
  fetchedAt: string
}

let stats: RepoStatsData | null = null

try {
  stats = await import('~/repo-stats.json').then(m => m.default) as RepoStatsData
} catch {
  // Stats file not available — component gracefully hides itself
}

function formatCount(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`
  return String(n)
}

const repoUrl = 'https://github.com/Ghent/capacitarr'
</script>

<template>
  <div v-if="stats" class="repo-stats">
    <NuxtLink :to="repoUrl" target="_blank" class="repo-stats-link" aria-label="Capacitarr on GitHub">
      <UIcon name="i-simple-icons-github" class="size-3.5 repo-stats-icon" />
      <span class="repo-stats-name">Ghent/capacitarr</span>
    </NuxtLink>

    <span v-if="stats.version" class="repo-stats-item" :title="`Latest release: ${stats.version}`">
      <UIcon name="i-lucide-tag" class="size-3" />
      <span>{{ stats.version }}</span>
    </span>

    <span class="repo-stats-item" :title="`${stats.stars} stars`">
      <UIcon name="i-lucide-star" class="size-3" />
      <span>{{ formatCount(stats.stars) }}</span>
    </span>

    <span class="repo-stats-item" :title="`${stats.forks} forks`">
      <UIcon name="i-lucide-git-fork" class="size-3" />
      <span>{{ formatCount(stats.forks) }}</span>
    </span>
  </div>
</template>

<style scoped>
.repo-stats {
  display: flex;
  align-items: center;
  gap: 0.625rem;
  font-size: 0.75rem;
  font-weight: 500;
  color: var(--color-neutral-500);
}

.repo-stats-link {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  text-decoration: none;
  color: var(--color-neutral-600);
  transition: color 0.2s;
}

:root.dark .repo-stats-link {
  color: var(--color-neutral-400);
}

.repo-stats-link:hover {
  color: var(--color-violet-600);
}

:root.dark .repo-stats-link:hover {
  color: var(--color-violet-400);
}

.repo-stats-icon {
  color: var(--color-orange-500);
}

.repo-stats-name {
  display: none;
}

@media (min-width: 1024px) {
  .repo-stats-name {
    display: inline;
  }
}

.repo-stats-item {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  color: var(--color-neutral-400);
}

:root.dark .repo-stats-item {
  color: var(--color-neutral-500);
}
</style>
