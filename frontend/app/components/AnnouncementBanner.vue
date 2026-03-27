<template>
  <Transition
    enter-active-class="transition-all duration-300 ease-out"
    leave-active-class="transition-all duration-200 ease-in"
    enter-from-class="opacity-0 -translate-y-full"
    enter-to-class="opacity-100 translate-y-0"
    leave-from-class="opacity-100 translate-y-0"
    leave-to-class="opacity-0 -translate-y-full"
  >
    <div
      v-if="topAnnouncement"
      data-slot="announcement-banner"
      class="fixed top-0 left-0 right-0 w-full z-[99] flex items-center justify-center gap-2 px-4 py-2.5 text-sm font-medium shadow-lg"
      :class="bannerClasses"
    >
      <component :is="bannerIcon" class="w-4 h-4 shrink-0" />
      <span class="truncate">{{ topAnnouncement.title }}</span>
      <NuxtLink
        v-if="extraCount > 0"
        to="/help"
        class="shrink-0 text-xs opacity-80 hover:opacity-100 underline underline-offset-2 transition-opacity"
      >
        {{ $t('announcements.moreCount', { count: extraCount }) }}
      </NuxtLink>
      <button
        class="ml-2 shrink-0 opacity-70 hover:opacity-100 transition-opacity"
        :title="$t('announcements.dismiss')"
        @click="dismiss(topAnnouncement.id)"
      >
        <XIcon class="w-4 h-4" />
      </button>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { InfoIcon, AlertTriangleIcon, AlertCircleIcon, XIcon } from 'lucide-vue-next';
import { useAnnouncements } from '~/composables/useAnnouncements';

const { activeBannerAnnouncements, dismiss } = useAnnouncements();

const topAnnouncement = computed(() => activeBannerAnnouncements.value[0] ?? null);
const extraCount = computed(() => Math.max(0, activeBannerAnnouncements.value.length - 1));

const bannerClasses = computed(() => {
  switch (topAnnouncement.value?.type) {
    case 'critical':
      return 'bg-destructive text-destructive-foreground';
    case 'warning':
      return 'bg-warning text-warning-foreground';
    default:
      return 'bg-primary text-primary-foreground';
  }
});

const bannerIcon = computed(() => {
  switch (topAnnouncement.value?.type) {
    case 'critical':
      return AlertCircleIcon;
    case 'warning':
      return AlertTriangleIcon;
    default:
      return InfoIcon;
  }
});
</script>
