<template>
  <div
    v-if="pullDistance > 0 || isRefreshing"
    class="fixed top-0 left-0 right-0 z-[60] flex justify-center pointer-events-none transition-transform duration-200"
    :style="{ transform: `translateY(${Math.min(pullDistance, 60)}px)` }"
  >
    <div
      class="flex items-center justify-center w-10 h-10 rounded-full bg-card border border-border shadow-lg"
      :class="{ 'animate-spin': isRefreshing }"
    >
      <component
        :is="isRefreshing ? LoaderCircleIcon : ArrowDownIcon"
        class="w-5 h-5 text-primary transition-transform duration-200"
        :style="
          !isRefreshing
            ? { transform: `rotate(${pullProgress * 180}deg)`, opacity: pullProgress }
            : {}
        "
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { LoaderCircleIcon, ArrowDownIcon } from 'lucide-vue-next';

defineProps<{
  pullDistance: number;
  pullProgress: number;
  isRefreshing: boolean;
}>();
</script>
