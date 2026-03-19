<template>
  <UiCard data-slot="dashboard-card" class="overflow-hidden">
    <!-- Header -->
    <div
      v-if="$slots.header || title"
      class="flex items-center justify-between px-4 pt-4 pb-2"
      data-slot="dashboard-card-header"
    >
      <div class="flex items-center gap-2">
        <slot name="icon">
          <component :is="icon" v-if="icon" class="w-4 h-4 text-muted-foreground" />
        </slot>
        <h3 class="text-sm font-semibold text-foreground">
          <slot name="header">{{ title }}</slot>
        </h3>
      </div>
      <div class="flex items-center gap-1">
        <slot name="actions" />
      </div>
    </div>

    <!-- Content -->
    <div
      class="px-4 pb-4"
      :class="[contentClass, !$slots.header && !title ? 'pt-4' : 'pt-0']"
      data-slot="dashboard-card-content"
    >
      <slot />
    </div>

    <!-- Footer -->
    <div
      v-if="$slots.footer"
      class="px-4 py-2 border-t border-border bg-muted/30 text-xs text-muted-foreground"
      data-slot="dashboard-card-footer"
    >
      <slot name="footer" />
    </div>
  </UiCard>
</template>

<script setup lang="ts">
import type { Component } from 'vue';

defineProps<{
  /** Card title text (alternative to #header slot) */
  title?: string;
  /** Optional Lucide icon component */
  icon?: Component;
  /** Additional CSS classes for the content area */
  contentClass?: string;
}>();
</script>
