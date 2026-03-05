<script setup lang="ts">
import { formatRelativeTime } from '~/utils/format';

const props = defineProps<{
  date: string;
  alwaysExact?: boolean;
}>();

const { showExactDates, formatTimestamp } = useDisplayPrefs();
const localOverride = ref<boolean | null>(null); // null = follow global pref

const showExact = computed(() => {
  if (props.alwaysExact) return true;
  if (localOverride.value !== null) return localOverride.value;
  return showExactDates.value;
});

function toggleLocal() {
  if (props.alwaysExact) return; // don't toggle if always exact
  localOverride.value = localOverride.value === null ? !showExact.value : !localOverride.value;
}

const displayText = computed(() => {
  if (!props.date) return '';
  return showExact.value ? formatTimestamp(props.date) : formatRelativeTime(props.date);
});

const tooltipText = computed(() => {
  if (!props.date) return '';
  return showExact.value ? formatRelativeTime(props.date) : formatTimestamp(props.date);
});
</script>

<template>
  <span
    class="cursor-pointer border-b border-dotted border-muted-foreground/40 hover:border-muted-foreground"
    :title="tooltipText"
    @click="toggleLocal"
  >
    {{ displayText }}
  </span>
</template>
