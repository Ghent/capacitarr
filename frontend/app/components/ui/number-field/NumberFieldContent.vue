<script setup lang="ts">
import type { HTMLAttributes } from 'vue';
import { cn } from '@/lib/utils';

const props = defineProps<{
  class?: HTMLAttributes['class'];
}>();
</script>

<template>
  <div
    data-slot="number-field-content"
    :class="
      cn(
        'flex items-center rounded-md border border-input shadow-xs',
        'has-[:focus-visible]:border-ring has-[:focus-visible]:ring-ring/50 has-[:focus-visible]:ring-[3px]',
        'has-[:disabled]:opacity-50 has-[:disabled]:cursor-not-allowed',
        props.class,
      )
    "
  >
    <slot />
  </div>
</template>
