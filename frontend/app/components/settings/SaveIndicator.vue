<script setup lang="ts">
import { CheckIcon } from 'lucide-vue-next';

defineProps<{
  status: 'idle' | 'saving' | 'saved' | 'error';
}>();
</script>

<template>
  <span
    v-if="status === 'saving'"
    class="inline-flex items-center gap-1 text-xs text-muted-foreground animate-pulse"
  >
    …saving
  </span>
  <span
    v-else-if="status === 'saved'"
    class="inline-flex items-center gap-1 text-xs text-emerald-500 font-medium transition-opacity"
  >
    <CheckIcon class="w-3 h-3" />
    Saved
  </span>
  <span
    v-else-if="status === 'error'"
    class="inline-flex items-center gap-1 text-xs text-red-500 font-medium"
  >
    ✕ Failed
  </span>
</template>
