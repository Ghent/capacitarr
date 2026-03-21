<template>
  <header data-slot="navbar" class="sticky top-0 z-50 relative">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
      <div class="flex items-center justify-between h-16">
        <!-- Brand -->
        <div class="flex items-center gap-6">
          <NuxtLink to="/" class="flex items-center gap-2.5 group">
            <div
              data-slot="brand-icon"
              class="w-8 h-8 rounded-lg bg-primary flex items-center justify-center"
            >
              <component :is="DatabaseIcon" class="w-4.5 h-4.5 text-primary-foreground" />
            </div>
            <div class="flex flex-col">
              <span class="text-lg font-bold tracking-tight text-foreground leading-tight">
                Capacitarr
              </span>
              <span class="text-[10px] text-muted-foreground/50 leading-none font-mono">
                UI v{{ uiVersion }} · API {{ apiVersion || '···' }}
              </span>
              <span
                class="text-[10px] text-muted-foreground/40 leading-none italic mt-0.5 inline-flex items-center gap-1"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  viewBox="0 0 24 24"
                  fill="currentColor"
                  class="inline-block w-3.5 h-3.5"
                  aria-hidden="true"
                >
                  <path d="M12 3 L14 7 L14 14 L13 16 L12 18 L11 16 L10 14 L10 7 Z" />
                  <path d="M10 9 L7 11 L6 14 L7 15 L8 14 L10 12 Z" />
                  <path d="M14 9 L17 11 L18 14 L17 15 L16 14 L14 12 Z" />
                  <path d="M10 14 L5 17 L6 18 L10 16 Z" />
                  <path d="M14 14 L19 17 L18 18 L14 16 Z" />
                  <path d="M11 18 L12 21 L13 18 Z" />
                </svg>
                {{ $t('nav.slogan') }}
              </span>
            </div>
          </NuxtLink>
        </div>

        <!-- Nav Links (right-aligned) -->
        <nav aria-label="Main navigation" class="hidden sm:flex items-center gap-1">
          <NuxtLink
            v-for="link in navLinks"
            :key="link.to"
            :to="link.to"
            class="px-3 py-1.5 rounded-lg text-sm font-medium transition-all duration-200"
            :class="[
              isActive(link.to)
                ? 'text-primary bg-primary/10'
                : 'text-muted-foreground hover:text-foreground hover:bg-accent',
            ]"
            :data-slot="isActive(link.to) ? 'nav-link-active' : undefined"
          >
            {{ link.label }}
          </NuxtLink>
        </nav>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import { DatabaseIcon } from 'lucide-vue-next';

const { uiVersion, apiVersion } = useVersion();
const route = useRoute();
const { t } = useI18n();

const navLinks = computed(() => [
  { to: '/', label: t('nav.dashboard') },
  { to: '/library', label: t('nav.library') },
  { to: '/rules', label: t('nav.rules') },
  { to: '/settings', label: t('nav.settings') },
  { to: '/help', label: t('nav.help') },
]);

/** Check if a nav link matches the current route */
function isActive(to: string): boolean {
  if (to === '/') return route.path === '/';
  return route.path.startsWith(to);
}
</script>
