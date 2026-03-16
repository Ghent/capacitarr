<template>
  <div
    data-slot="app-shell"
    class="min-h-screen bg-background text-foreground transition-colors duration-300"
  >
    <Navbar v-if="isAuthenticated" />
    <main data-slot="page-content" class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pt-5 pb-6">
      <IntegrationErrorBanner v-if="isAuthenticated" :integrations="appIntegrations" />
      <NuxtPage />
    </main>
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

// Fetch integrations for the global error banner
const api = useApi();
const appIntegrations = ref<IntegrationConfig[]>([]);

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
const { connect: connectSSE, disconnect: disconnectSSE } = useEventStream();

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
