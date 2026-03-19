<template>
  <div
    data-slot="app-shell"
    class="min-h-screen bg-background text-foreground transition-colors duration-300"
  >
    <Navbar v-if="isAuthenticated" />
    <main
      data-slot="page-content"
      class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-5"
      :class="isAuthenticated ? 'pb-20' : 'pb-6'"
    >
      <NuxtPage />
    </main>
    <BottomToolbar v-if="isAuthenticated" />
  </div>
  <ClientOnly>
    <ConnectionBanner />
    <ToastContainer />
  </ClientOnly>
</template>

<script setup lang="ts">
import type { IntegrationConfig } from '~/types/api';

const authenticated = useAuthCookie();
const isAuthenticated = computed(() => !!authenticated.value);

// Fetch integrations and provide to child pages for error banners
const api = useApi();
const appIntegrations = ref<IntegrationConfig[]>([]);
provide('appIntegrations', appIntegrations);

async function fetchAppIntegrations() {
  try {
    appIntegrations.value = (await api('/api/v1/integrations')) as IntegrationConfig[];
  } catch {
    // Silently fail — banner just won't show
  }
}

// Fetch on auth state change and periodically
watch(isAuthenticated, (authed) => {
  if (authed) fetchAppIntegrations();
  else appIntegrations.value = [];
});

// Initialize color mode and theme on client
if (import.meta.client) {
  useAppColorMode();
  useTheme();
}

// Initialize SSE event stream when authenticated (client-only).
// The connection persists for the app lifetime and reconnects automatically.
const { connect: connectSSE, disconnect: disconnectSSE, on: sseOn, off: sseOff } = useEventStream();

// Re-fetch integrations when any integration is added, updated, or removed
// so the error banner and integration cards reflect the current state.
const integrationEventTypes = ['integration_added', 'integration_updated', 'integration_removed'];
onMounted(() => {
  for (const evt of integrationEventTypes) {
    sseOn(evt, fetchAppIntegrations);
  }
});
onUnmounted(() => {
  for (const evt of integrationEventTypes) {
    sseOff(evt, fetchAppIntegrations);
  }
});

watch(isAuthenticated, (authed) => {
  if (import.meta.client) {
    if (authed) {
      connectSSE();
    } else {
      disconnectSSE();
    }
  }
});

// Remove splash screen on mount, start SSE if already authenticated
onMounted(() => {
  const splash = document.getElementById('capacitarr-splash');
  if (splash) {
    splash.classList.add('fade-out');
    setTimeout(() => splash.remove(), 300);
  }

  if (isAuthenticated.value) {
    connectSSE();
    fetchAppIntegrations();
  }
});
</script>
