<template>
    <div data-slot="app-shell" class="min-h-screen bg-background text-foreground transition-colors duration-300">
    <Navbar v-if="isAuthenticated" />
    <main data-slot="page-content" class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-6 pb-8">
      <NuxtPage />
    </main>
  </div>
  <ClientOnly>
    <ToastContainer />
  </ClientOnly>
</template>

<script setup lang="ts">
const token = useCookie('jwt')
const isAuthenticated = computed(() => !!token.value)

// Initialize color mode and theme on client
if (import.meta.client) {
  useAppColorMode()
  useTheme()
}
</script>
