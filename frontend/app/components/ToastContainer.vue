<template>
  <div class="fixed bottom-16 right-4 z-[100] flex flex-col space-y-2 max-w-sm">
    <TransitionGroup
      enter-active-class="transition-all duration-300 ease-out"
      enter-from-class="translate-x-full opacity-0"
      enter-to-class="translate-x-0 opacity-100"
      leave-active-class="transition-all duration-200 ease-in"
      leave-from-class="translate-x-0 opacity-100"
      leave-to-class="translate-x-full opacity-0"
      move-class="transition-all duration-300 ease-in-out"
    >
      <div
        v-for="toast in toasts"
        :key="toast.id"
        :class="[
          'flex items-start gap-3 rounded-lg border px-4 py-3 shadow-lg',
          colorClasses[toast.type],
        ]"
      >
        <component :is="iconMap[toast.type]" class="mt-0.5 h-5 w-5 shrink-0" />
        <span class="flex-1 text-sm">{{ toast.message }}</span>
        <button
          class="shrink-0 rounded p-0.5 opacity-60 hover:opacity-100 transition-opacity"
          @click="removeToast(toast.id)"
        >
          <XIcon class="h-4 w-4" />
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>

<script setup lang="ts">
import { AlertCircleIcon, CheckCircleIcon, InfoIcon, XIcon } from 'lucide-vue-next';

const { toasts, removeToast } = useToast();

const colorClasses: Record<string, string> = {
  error:
    'bg-red-50 dark:bg-red-950/80 border-red-200 dark:border-red-800 text-red-800 dark:text-red-200',
  success:
    'bg-green-50 dark:bg-green-950/80 border-green-200 dark:border-green-800 text-green-800 dark:text-green-200',
  info: 'bg-blue-50 dark:bg-blue-950/80 border-blue-200 dark:border-blue-800 text-blue-800 dark:text-blue-200',
};

const iconMap: Record<string, typeof AlertCircleIcon> = {
  error: AlertCircleIcon,
  success: CheckCircleIcon,
  info: InfoIcon,
};
</script>
