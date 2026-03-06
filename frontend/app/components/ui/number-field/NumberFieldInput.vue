<script setup lang="ts">
import type { HTMLAttributes } from 'vue';
import { NumberFieldInput, type NumberFieldInputProps } from 'reka-ui';
import { cn } from '@/lib/utils';

const props = defineProps<NumberFieldInputProps & { class?: HTMLAttributes['class'] }>();
</script>

<template>
  <NumberFieldInput
    data-slot="number-field-input"
    :class="
      cn(
        'flex h-9 w-full min-w-0 bg-transparent px-3 py-1 text-base tabular-nums text-foreground placeholder:text-muted-foreground outline-none disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 md:text-sm',
        '[appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none',
        props.class,
      )
    "
  />
</template>
