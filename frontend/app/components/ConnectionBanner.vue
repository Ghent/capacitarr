<template>
  <Transition
    enter-active-class="transition-all duration-300 ease-out"
    leave-active-class="transition-all duration-200 ease-in"
    enter-from-class="opacity-0 -translate-y-full"
    enter-to-class="opacity-100 translate-y-0"
    leave-from-class="opacity-100 translate-y-0"
    leave-to-class="opacity-0 -translate-y-full"
  >
    <div
      v-if="!isConnected"
      data-slot="connection-banner"
      class="fixed top-0 left-0 right-0 w-full z-[100] flex items-center justify-center gap-2 px-4 py-2.5 text-sm font-medium bg-destructive text-destructive-foreground shadow-lg"
    >
      <WifiOffIcon class="w-4 h-4 shrink-0" />
      <span>{{ $t('connection.lost') }}</span>
      <LoaderCircleIcon class="w-3.5 h-3.5 animate-spin shrink-0 ml-1" />
    </div>
  </Transition>
  <Transition
    enter-active-class="transition-all duration-300 ease-out"
    leave-active-class="transition-all duration-200 ease-in"
    enter-from-class="opacity-0 -translate-y-full"
    enter-to-class="opacity-100 translate-y-0"
    leave-from-class="opacity-100 translate-y-0"
    leave-to-class="opacity-0 -translate-y-full"
  >
    <div
      v-if="isReconnected"
      data-slot="connection-banner"
      class="fixed top-0 left-0 right-0 w-full z-[100] flex items-center justify-center gap-2 px-4 py-2.5 text-sm font-medium bg-emerald-500 text-white shadow-lg"
    >
      <WifiIcon class="w-4 h-4 shrink-0" />
      <span>{{ $t('connection.restored') }}</span>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { WifiOffIcon, WifiIcon, LoaderCircleIcon } from 'lucide-vue-next';

const { isConnected, isReconnected } = useConnectionHealth();
</script>
