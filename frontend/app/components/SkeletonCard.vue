<template>
  <div
    class="rounded-xl border border-border bg-card shadow-sm overflow-hidden animate-pulse"
  >
    <div class="p-5 space-y-3">
      <!-- Header row -->
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-lg bg-muted" />
          <div class="space-y-2">
            <div class="h-3.5 w-32 rounded bg-muted" />
            <div class="h-3 w-20 rounded bg-muted" />
          </div>
        </div>
        <div class="h-7 w-14 rounded bg-muted" />
      </div>

      <!-- Progress bar -->
      <div class="h-3 w-full rounded-full bg-muted mt-4" />

      <!-- Subtitle -->
      <div class="h-3 w-24 rounded bg-muted" />
    </div>

    <!-- Chart area placeholder -->
    <div
      v-if="showChart"
      class="px-5 pb-4"
    >
      <div class="h-3 w-28 rounded bg-muted mb-2" />
      <div class="h-48 rounded-lg bg-muted" />
    </div>
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  showChart?: boolean
}>(), {
  showChart: false
})
</script>
