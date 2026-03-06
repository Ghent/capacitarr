<script setup lang="ts">
import type { HTMLAttributes } from 'vue';
import { reactiveOmit } from '@vueuse/core';
import { ComboboxEmpty, type ComboboxEmptyProps, useForwardProps } from 'reka-ui';
import { cn } from '@/lib/utils';

const props = defineProps<ComboboxEmptyProps & { class?: HTMLAttributes['class'] }>();

const delegatedProps = reactiveOmit(props, 'class');
const forwardedProps = useForwardProps(delegatedProps);
</script>

<template>
  <ComboboxEmpty
    v-bind="forwardedProps"
    data-slot="combobox-empty"
    :class="cn('px-3 py-2 text-sm text-muted-foreground', props.class)"
  >
    <slot />
  </ComboboxEmpty>
</template>
