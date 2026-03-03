<script setup lang="ts">
import type { ContentNavigationItem } from '@nuxt/content'

const navigation = inject<Ref<ContentNavigationItem[]>>('navigation')
const { header } = useAppConfig()
</script>

<template>
  <UHeader :to="header?.to || '/'">
    <template #left>
      <NuxtLink :to="header?.to || '/'" class="header-logo">
        <span class="logo-icon">
          <UIcon name="i-lucide-hard-drive" class="size-5" />
        </span>
        <span class="logo-text">{{ header?.title || 'Capacitarr' }}</span>
      </NuxtLink>
    </template>

    <UContentSearchButton
      v-if="header?.search"
      :collapsed="false"
      class="w-full"
    />

    <template #right>
      <UContentSearchButton
        v-if="header?.search"
        class="lg:hidden"
      />

      <UColorModeButton v-if="header?.colorMode" />

      <template v-if="header?.links">
        <UButton
          v-for="(link, index) of header.links"
          :key="index"
          v-bind="{ color: 'neutral', variant: 'ghost', ...link }"
        />
      </template>
    </template>

    <template #body>
      <UContentNavigation
        highlight
        :navigation="navigation"
      />
    </template>
  </UHeader>
</template>

<style scoped>
.header-logo {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  text-decoration: none;
  transition: opacity 0.2s;
}

.header-logo:hover {
  opacity: 0.8;
}

.logo-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.75rem;
  height: 1.75rem;
  border-radius: 0.375rem;
  background: linear-gradient(135deg, var(--color-violet-500), var(--color-violet-600));
  color: white;
  flex-shrink: 0;
}

.logo-text {
  font-weight: 700;
  font-size: 1.0625rem;
  letter-spacing: -0.01em;
}
</style>
