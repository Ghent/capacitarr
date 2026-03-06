<script setup lang="ts">
import type { HTMLAttributes } from 'vue';
import { NumberFieldIncrement, type NumberFieldIncrementProps } from 'reka-ui';
import { cn } from '@/lib/utils';

const props = defineProps<NumberFieldIncrementProps & { class?: HTMLAttributes['class'] }>();
</script>

<template>
  <NumberFieldIncrement
    data-slot="number-field-increment"
    :class="
      cn(
        'flex items-center justify-center border-l border-input px-2 text-muted-foreground transition-colors hover:bg-accent hover:text-primary disabled:pointer-events-none disabled:opacity-50',
        props.class,
      )
    "
  >
    <slot>
      <svg
        xmlns="http://www.w3.org/2000/svg"
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path d="m18 15-6-6-6 6" />
      </svg>
    </slot>
  </NumberFieldIncrement>
</template>
