<script setup lang="ts">
const { footer } = useAppConfig()
</script>

<template>
  <UFooter>
    <template #left>
      <div class="footer-brand">
        <span class="footer-logo-icon">
          <UIcon name="i-lucide-hard-drive" class="size-3.5" />
        </span>
        <span class="footer-credits">{{ footer.credits }}</span>
        <span class="footer-separator">·</span>
        <span class="footer-tagline">
          Built with
          <UIcon name="i-lucide-heart" class="size-3 text-rose-500 inline-block" />
          for the self-hosted community
        </span>
      </div>
    </template>

    <template #right>
      <UColorModeButton v-if="footer?.colorMode" />

      <template v-if="footer?.links">
        <UButton
          v-for="(link, index) of footer?.links"
          :key="index"
          v-bind="{ color: 'neutral', variant: 'ghost', ...link }"
        />
      </template>
    </template>
  </UFooter>
</template>

<style scoped>
.footer-brand {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.footer-logo-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 1.375rem;
  height: 1.375rem;
  border-radius: 0.25rem;
  background: linear-gradient(135deg, var(--color-violet-500), var(--color-violet-600));
  color: white;
  flex-shrink: 0;
}

.footer-credits {
  font-size: 0.8125rem;
  color: var(--color-neutral-500);
}

.footer-separator {
  color: var(--color-neutral-300);
}

:root.dark .footer-separator {
  color: var(--color-neutral-700);
}

.footer-tagline {
  font-size: 0.75rem;
  color: var(--color-neutral-400);
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
}
</style>
