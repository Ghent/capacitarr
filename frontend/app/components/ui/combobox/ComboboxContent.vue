<script setup lang="ts">
import type { HTMLAttributes } from 'vue';
import { reactiveOmit } from '@vueuse/core';
import { ComboboxContent, type ComboboxContentProps, useForwardProps } from 'reka-ui';
import { cn } from '@/lib/utils';

const props = withDefaults(
  defineProps<ComboboxContentProps & { class?: HTMLAttributes['class'] }>(),
  {
    position: 'popper',
    side: 'bottom',
    sideOffset: 4,
    align: 'start',
  },
);

const delegatedProps = reactiveOmit(props, 'class');
const forwardedProps = useForwardProps(delegatedProps);
</script>

<template>
  <ComboboxContent
    v-bind="forwardedProps"
    data-slot="combobox-content"
    :class="
      cn(
        'z-50 max-h-48 w-[var(--reka-popper-anchor-width)] overflow-auto rounded-md border border-border bg-popover text-popover-foreground shadow-md',
        'data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95',
        'data-[side=bottom]:slide-in-from-top-2 data-[side=left]:slide-in-from-right-2 data-[side=right]:slide-in-from-left-2 data-[side=top]:slide-in-from-bottom-2',
        props.class,
      )
    "
  >
    <slot />
  </ComboboxContent>
</template>
