<script setup lang="ts">
import type { HTMLAttributes } from 'vue';
import { reactiveOmit } from '@vueuse/core';
import { ComboboxItem, type ComboboxItemProps, useForwardProps } from 'reka-ui';
import { cn } from '@/lib/utils';

const props = defineProps<ComboboxItemProps & { class?: HTMLAttributes['class'] }>();

const delegatedProps = reactiveOmit(props, 'class');
const forwardedProps = useForwardProps(delegatedProps);
</script>

<template>
  <ComboboxItem
    v-bind="forwardedProps"
    data-slot="combobox-item"
    :class="
      cn(
        'relative flex w-full cursor-pointer select-none items-center px-3 py-1.5 text-sm outline-none',
        'data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground',
        'data-[disabled]:pointer-events-none data-[disabled]:opacity-50',
        props.class,
      )
    "
  >
    <slot />
  </ComboboxItem>
</template>
