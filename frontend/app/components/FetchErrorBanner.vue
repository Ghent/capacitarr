<template>
  <UiAlert variant="destructive" data-slot="fetch-error-banner" class="mb-6">
    <AlertTriangleIcon class="w-4 h-4" />
    <UiAlertTitle>{{ $t('errors.fetchFailedTitle') }}</UiAlertTitle>
    <UiAlertDescription class="flex flex-col sm:flex-row sm:items-center gap-3">
      <span>{{ message || $t('errors.fetchFailed') }}</span>
      <UiButton variant="outline" size="sm" class="shrink-0" @click="$emit('retry')">
        {{ $t('errors.retry') }}
      </UiButton>
    </UiAlertDescription>
  </UiAlert>
</template>

<script setup lang="ts">
import { AlertTriangleIcon } from 'lucide-vue-next';

defineProps<{
  message?: string;
}>();

defineEmits<{
  retry: [];
}>();
</script>
