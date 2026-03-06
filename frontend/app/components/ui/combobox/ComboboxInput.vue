<script setup lang="ts">
import type { HTMLAttributes } from 'vue';
import { reactiveOmit } from '@vueuse/core';
import { ComboboxInput, type ComboboxInputProps, useForwardProps } from 'reka-ui';
import { cn } from '@/lib/utils';

const props = defineProps<ComboboxInputProps & { class?: HTMLAttributes['class'] }>();

const delegatedProps = reactiveOmit(props, 'class');
const forwardedProps = useForwardProps(delegatedProps);
</script>

<template>
  <ComboboxInput
    v-bind="forwardedProps"
    data-slot="combobox-input"
    :class="
      cn(
        'flex h-9 w-full min-w-0 bg-transparent px-3 py-1 text-base text-foreground placeholder:text-muted-foreground outline-none disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 md:text-sm',
        props.class,
      )
    "
  />
</template>
