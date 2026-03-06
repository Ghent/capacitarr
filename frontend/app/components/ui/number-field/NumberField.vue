<script setup lang="ts">
import type { HTMLAttributes } from 'vue';
import { reactiveOmit } from '@vueuse/core';
import {
  NumberFieldRoot,
  type NumberFieldRootEmits,
  type NumberFieldRootProps,
  useForwardPropsEmits,
} from 'reka-ui';
import { cn } from '@/lib/utils';

const props = defineProps<NumberFieldRootProps & { class?: HTMLAttributes['class'] }>();
const emits = defineEmits<NumberFieldRootEmits>();

const delegatedProps = reactiveOmit(props, 'class');
const forwarded = useForwardPropsEmits(delegatedProps, emits);
</script>

<template>
  <NumberFieldRoot
    v-bind="forwarded"
    data-slot="number-field"
    :class="cn('grid gap-1', props.class)"
  >
    <slot />
  </NumberFieldRoot>
</template>
