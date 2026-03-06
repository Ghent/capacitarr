<script setup lang="ts">
import type { HTMLAttributes } from 'vue';
import { reactiveOmit } from '@vueuse/core';
import {
  ComboboxRoot,
  type ComboboxRootEmits,
  type ComboboxRootProps,
  useForwardPropsEmits,
} from 'reka-ui';
import { cn } from '@/lib/utils';

const props = defineProps<ComboboxRootProps & { class?: HTMLAttributes['class'] }>();
const emits = defineEmits<ComboboxRootEmits>();

const delegatedProps = reactiveOmit(props, 'class');
const forwarded = useForwardPropsEmits(delegatedProps, emits);
</script>

<template>
  <ComboboxRoot v-bind="forwarded" data-slot="combobox" :class="cn('relative', props.class)">
    <slot />
  </ComboboxRoot>
</template>
