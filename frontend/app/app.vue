<template>
  <div class="min-h-screen bg-slate-50 dark:bg-slate-900 transition-colors duration-300">
    <LazyNavbar v-if="isAuthenticated" />
    <UContainer class="py-8">
      <NuxtPage />
    </UContainer>
  </div>
</template>

<script setup lang="ts">
const isAuthenticated = useCookie('jwt')
</script>
