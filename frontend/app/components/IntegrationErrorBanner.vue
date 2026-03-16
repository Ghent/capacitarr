<template>
  <UiCard
    v-if="failingIntegrations.length > 0 && !dismissed"
    v-motion
    :initial="{ opacity: 0, y: -8 }"
    :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 300, damping: 26 } }"
    data-slot="integration-error-banner"
    class="border-amber-500/40 bg-amber-500/5"
  >
    <UiCardContent class="pt-4 pb-3">
      <div class="flex items-start justify-between gap-3">
        <div class="flex items-start gap-2.5 min-w-0">
          <AlertTriangleIcon class="w-4 h-4 text-amber-500 shrink-0 mt-0.5" />
          <div class="min-w-0 space-y-2">
            <p class="text-sm font-medium text-foreground">
              {{ failingIntegrations.length }} integration{{
                failingIntegrations.length > 1 ? 's' : ''
              }}
              failed to connect
            </p>
            <div
              v-for="integration in failingIntegrations"
              :key="integration.id"
              class="flex items-baseline gap-2 text-sm"
            >
              <span class="font-medium text-foreground shrink-0">{{ integration.name }}</span>
              <UiBadge variant="outline" class="text-[10px] shrink-0">{{
                integration.type
              }}</UiBadge>
              <span class="text-muted-foreground text-xs truncate" :title="integration.lastError">
                — {{ integration.lastError }}
              </span>
            </div>
            <NuxtLink
              to="/settings"
              class="inline-flex items-center gap-1 text-xs text-amber-600 dark:text-amber-400 hover:text-amber-500 font-medium transition-colors"
            >
              <SettingsIcon class="w-3 h-3" />
              Fix in Settings
            </NuxtLink>
          </div>
        </div>
        <button
          class="text-muted-foreground/60 hover:text-foreground transition-colors shrink-0"
          title="Dismiss"
          @click="dismissed = true"
        >
          <XIcon class="w-4 h-4" />
        </button>
      </div>
    </UiCardContent>
  </UiCard>
</template>

<script setup lang="ts">
import { AlertTriangleIcon, SettingsIcon, XIcon } from 'lucide-vue-next';
import type { IntegrationConfig } from '~/types/api';

const props = defineProps<{
  integrations: IntegrationConfig[];
}>();

const failingIntegrations = computed(() => props.integrations.filter((i) => i.lastError));

const dismissed = ref(false);
</script>
