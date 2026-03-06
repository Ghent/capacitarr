<script setup lang="ts">
import type { HTMLAttributes } from 'vue';
import { NumberFieldDecrement, type NumberFieldDecrementProps } from 'reka-ui';
import { cn } from '@/lib/utils';

const props = defineProps<NumberFieldDecrementProps & { class?: HTMLAttributes['class'] }>();
</script>

<template>
  <NumberFieldDecrement
    data-slot="number-field-decrement"
    :class="
      cn(
        'flex items-center justify-center border-r border-input px-2 text-muted-foreground transition-colors hover:bg-accent hover:text-primary disabled:pointer-events-none disabled:opacity-50',
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
        <path d="m6 9 6 6 6-6" />
      </svg>
    </slot>
  </NumberFieldDecrement>
</template>
