<template>
  <div class="rounded-xl border border-border bg-card shadow-sm overflow-hidden animate-pulse">
    <!-- Table header -->
    <div class="flex items-center gap-4 px-4 py-3 border-b border-border/50">
      <div v-for="w in columnWidths" :key="w" class="h-3 rounded bg-muted" :style="{ width: w }" />
    </div>
    <!-- Table rows -->
    <div
      v-for="i in rows"
      :key="i"
      class="flex items-center gap-4 px-4 py-3 border-b border-zinc-50 dark:border-zinc-800/30 last:border-b-0"
    >
      <div
        v-for="(w, j) in columnWidths"
        :key="j"
        class="h-3 rounded"
        :class="j === 0 ? 'bg-muted' : 'bg-muted'"
        :style="{ width: w }"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    rows?: number;
    columnWidths?: string[];
  }>(),
  {
    rows: 5,
    columnWidths: () => ['30%', '15%', '12%', '18%', '10%'],
  },
);
</script>
