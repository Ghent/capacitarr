<script setup lang="ts">
import type { HTMLAttributes } from 'vue';
import { reactiveOmit } from '@vueuse/core';
import { ComboboxAnchor, type ComboboxAnchorProps, useForwardProps } from 'reka-ui';
import { cn } from '@/lib/utils';

const props = defineProps<ComboboxAnchorProps & { class?: HTMLAttributes['class'] }>();

const delegatedProps = reactiveOmit(props, 'class');
const forwardedProps = useForwardProps(delegatedProps);
</script>

<template>
  <ComboboxAnchor
    v-bind="forwardedProps"
    data-slot="combobox-anchor"
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
  </ComboboxAnchor>
</template>
