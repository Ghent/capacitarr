<script setup lang="ts">
const props = defineProps<{
  src: string
  alt?: string
  width?: string | number
  height?: string | number
}>()

const isBadge = computed(() =>
  props.src?.includes('shields.io')
  || props.src?.includes('img.shields'),
)
</script>

<template>
  <img
    v-if="isBadge"
    :src="src"
    :alt="alt || ''"
    class="badge-inline"
  >
  <img
    v-else
    :src="src"
    :alt="alt || ''"
    :width="width"
    :height="height"
    class="rounded-md w-full will-change-transform cursor-zoom-in"
  >
</template>

<style scoped>
.badge-inline {
  display: inline;
  height: 20px;
  width: auto;
  max-width: none;
  vertical-align: middle;
  border-radius: 3px;
}
</style>
