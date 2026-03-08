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
      <RepoStats class="hidden lg:flex mr-2" />

      <UContentSearchButton
        v-if="header?.search"
        class="lg:hidden"
      />

      <UColorModeButton v-if="header?.colorMode" />

      <!-- Donation / Support popover -->
      <UPopover>
        <UButton
          icon="i-lucide-heart-handshake"
          color="neutral"
          variant="ghost"
          aria-label="Support & Donate"
        />

        <template #content>
          <div class="donate-popover">
            <div class="donate-header">
              <UIcon name="i-lucide-paw-print" class="size-4 text-amber-500" />
              <span class="donate-title">Support Animal Rescue</span>
            </div>

            <p class="donate-message">
              Capacitarr is free software. If it saves you time, we'd love for you to donate to animal rescue instead of supporting us directly.
            </p>

            <div class="donate-links">
              <a
                href="https://uanimals.org/en/"
                target="_blank"
                rel="noopener noreferrer"
                class="donate-link"
              >
                <UIcon name="i-lucide-heart" class="size-4 text-amber-500" />
                <div>
                  <span class="donate-link-name">UAnimals</span>
                  <span class="donate-link-desc">Rescuing animals in Ukraine 🇺🇦</span>
                </div>
                <UIcon name="i-lucide-external-link" class="size-3 donate-link-external" />
              </a>

              <a
                href="https://www.aspca.org/ways-to-help"
                target="_blank"
                rel="noopener noreferrer"
                class="donate-link"
              >
                <UIcon name="i-lucide-paw-print" class="size-4 text-orange-500" />
                <div>
                  <span class="donate-link-name">ASPCA</span>
                  <span class="donate-link-desc">Preventing cruelty to animals</span>
                </div>
                <UIcon name="i-lucide-external-link" class="size-3 donate-link-external" />
              </a>
            </div>

            <div class="donate-separator" />

            <p class="donate-dev-heading">Support the Developer</p>

            <div class="donate-dev-links">
              <a
                href="https://github.com/sponsors/ghent"
                target="_blank"
                rel="noopener noreferrer"
                class="donate-dev-link"
              >
                <UIcon name="i-simple-icons-githubsponsors" class="size-3.5" />
                GitHub Sponsors
              </a>
              <a
                href="https://ko-fi.com/ghent"
                target="_blank"
                rel="noopener noreferrer"
                class="donate-dev-link"
              >
                <UIcon name="i-simple-icons-kofi" class="size-3.5" />
                Ko-fi
              </a>
              <a
                href="https://buymeacoffee.com/ghentgames"
                target="_blank"
                rel="noopener noreferrer"
                class="donate-dev-link"
              >
                <UIcon name="i-simple-icons-buymeacoffee" class="size-3.5" />
                Buy Me a Coffee
              </a>
            </div>
          </div>
        </template>
      </UPopover>

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

/* Donate popover */
.donate-popover {
  padding: 1rem;
  width: 18rem;
}

.donate-header {
  display: flex;
  align-items: center;
  gap: 0.375rem;
  margin-bottom: 0.5rem;
}

.donate-title {
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--color-neutral-900);
}

:root.dark .donate-title {
  color: var(--color-neutral-100);
}

.donate-message {
  font-size: 0.75rem;
  line-height: 1.5;
  color: var(--color-neutral-500);
  margin-bottom: 0.75rem;
}

.donate-links {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.donate-link {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem;
  border-radius: 0.375rem;
  text-decoration: none;
  transition: background 0.2s;
}

.donate-link:hover {
  background: var(--color-neutral-100);
}

:root.dark .donate-link:hover {
  background: var(--color-neutral-800);
}

.donate-link-name {
  display: block;
  font-size: 0.8125rem;
  font-weight: 500;
  color: var(--color-neutral-900);
}

:root.dark .donate-link-name {
  color: var(--color-neutral-100);
}

.donate-link-desc {
  display: block;
  font-size: 0.6875rem;
  color: var(--color-neutral-400);
}

.donate-link-external {
  margin-left: auto;
  color: var(--color-neutral-300);
}

:root.dark .donate-link-external {
  color: var(--color-neutral-600);
}

.donate-separator {
  height: 1px;
  background: var(--color-neutral-200);
  margin: 0.75rem 0;
}

:root.dark .donate-separator {
  background: var(--color-neutral-800);
}

.donate-dev-heading {
  font-size: 0.6875rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--color-neutral-400);
  margin-bottom: 0.375rem;
}

.donate-dev-links {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.donate-dev-link {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  font-size: 0.75rem;
  color: var(--color-neutral-500);
  text-decoration: none;
  transition: color 0.2s;
}

.donate-dev-link:hover {
  color: var(--color-primary-500);
}
</style>
