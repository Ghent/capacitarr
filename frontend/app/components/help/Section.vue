<template>
  <details
    :id="id || undefined"
    v-motion
    :open="defaultOpen || undefined"
    :initial="{ opacity: 0, y: 12 }"
    :enter="{
      opacity: 1,
      y: 0,
      transition: { type: 'spring', stiffness: 260, damping: 24, delay },
    }"
    data-slot="card"
    class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
  >
    <summary
      class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors"
    >
      <ChevronRightIcon
        class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90"
      />
      <component :is="icon" v-if="icon" class="w-4.5 h-4.5 text-primary" />
      <h3 class="font-semibold text-primary">{{ title }}</h3>
    </summary>
    <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed" :class="contentClass">
      <slot />
    </div>
  </details>
</template>

<script setup lang="ts">
import type { Component } from 'vue';
import { ChevronRightIcon } from 'lucide-vue-next';

withDefaults(
  defineProps<{
    title: string;
    icon?: Component;
    defaultOpen?: boolean;
    id?: string;
    delay?: number;
    contentClass?: string;
  }>(),
  {
    defaultOpen: false,
    delay: 0,
    contentClass: 'space-y-3',
  },
);
</script>
