<template>
  <div class="flex justify-end mb-6">
    <UiButton @click="openAddModal">
      <component :is="PlusIcon" class="w-4 h-4" />
      {{ $t('settings.addIntegration') }}
    </UiButton>
  </div>

  <!-- Loading -->
  <div v-if="loading" class="flex justify-center py-16">
    <component :is="LoaderCircleIcon" class="w-8 h-8 text-primary animate-spin" />
  </div>

  <!-- Empty state -->
  <div
    v-else-if="integrations.length === 0"
    v-motion
    :initial="{ opacity: 0, y: 8 }"
    :enter="{ opacity: 1, y: 0 }"
    class="text-center py-20"
  >
    <component :is="HardDriveIcon" class="w-16 h-16 text-muted-foreground/40 mx-auto mb-4" />
    <h3 class="text-lg font-medium text-foreground mb-2">
      {{ $t('settings.noIntegrations') }}
    </h3>
    <p class="text-muted-foreground mb-6">
      {{ $t('settings.noIntegrationsHelp') }}
    </p>
    <UiButton size="lg" @click="openAddModal">
      <component :is="PlusIcon" class="w-4 h-4" />
      {{ $t('settings.addFirstIntegration') }}
    </UiButton>
  </div>

  <!-- Integration Cards Grid -->
  <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
    <UiCard
      v-for="(integration, idx) in integrations"
      :key="integration.id"
      v-motion
      :initial="{ opacity: 0, y: 12 }"
      :enter="{
        opacity: 1,
        y: 0,
        transition: { type: 'spring', stiffness: 260, damping: 24, delay: 80 * idx },
      }"
      class="overflow-hidden"
    >
      <UiCardHeader class="border-b border-border">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div
              :class="[
                'w-10 h-10 rounded-lg flex items-center justify-center',
                typeColor(integration.type),
              ]"
            >
              <component :is="typeIcon(integration.type)" class="w-5 h-5 text-white" />
            </div>
            <div>
              <UiCardTitle class="text-base">
                {{ integration.name }}
              </UiCardTitle>
              <span
                class="text-xs uppercase tracking-wider font-medium"
                :class="typeTextColor(integration.type)"
              >
                {{ integration.type }}
              </span>
            </div>
          </div>
          <div class="flex items-center gap-2">
            <UiBadge v-if="getWeightState(integration.id).enabled" variant="outline" class="gap-1">
              <component :is="SlidersHorizontalIcon" class="w-3 h-3" />
              {{ $t('settings.customWeightsBadge') }}
            </UiBadge>
            <UiBadge :variant="integration.enabled ? 'default' : 'secondary'">
              {{ integration.enabled ? $t('common.active') : $t('common.disabled') }}
            </UiBadge>
          </div>
        </div>
      </UiCardHeader>

      <UiCardContent class="pt-4 space-y-2 text-sm text-muted-foreground">
        <div class="flex items-center gap-2">
          <component :is="LinkIcon" class="w-3.5 h-3.5 shrink-0" />
          <span class="truncate">{{ integration.url }}</span>
        </div>
        <div class="flex items-center gap-2">
          <component :is="KeyIcon" class="w-3.5 h-3.5 shrink-0" />
          <span
            class="font-mono text-xs truncate max-w-[180px] inline-block align-bottom"
            :title="integration.apiKey"
          >
            {{
              integration.apiKey.length > 16
                ? integration.apiKey.slice(0, 8) + '••••' + integration.apiKey.slice(-4)
                : integration.apiKey
            }}
          </span>
        </div>
        <div v-if="integration.lastSync" class="flex items-center gap-2">
          <component :is="ClockIcon" class="w-3.5 h-3.5 shrink-0" />
          <span>Synced <DateDisplay :date="integration.lastSync" /></span>
        </div>
        <div v-if="integration.lastError" class="flex items-center gap-2 text-red-500">
          <component :is="AlertTriangleIcon" class="w-3.5 h-3.5 shrink-0" />
          <span class="text-xs">{{ integration.lastError }}</span>
        </div>
      </UiCardContent>

      <!-- Custom Scoring Weights Toggle & Panel -->
      <div class="border-t border-border px-6 py-3">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <UiSwitch
              :model-value="getWeightState(integration.id).enabled"
              @update:model-value="(val: boolean) => toggleCustomWeights(integration.id, val)"
            />
            <UiLabel class="text-sm font-medium cursor-pointer">
              {{ $t('settings.customScoringWeights') }}
            </UiLabel>
          </div>
          <UiTooltipProvider>
            <UiTooltip>
              <UiTooltipTrigger as-child>
                <UiBadge variant="outline" class="text-xs text-muted-foreground">
                  {{ $t('settings.customWeightsPreview') }}
                </UiBadge>
              </UiTooltipTrigger>
              <UiTooltipContent>
                <p>{{ $t('settings.customWeightsPreviewDesc') }}</p>
              </UiTooltipContent>
            </UiTooltip>
          </UiTooltipProvider>
        </div>

        <!-- Collapsible Weight Sliders -->
        <Transition
          enter-active-class="transition-all duration-300 ease-out"
          leave-active-class="transition-all duration-200 ease-in"
          enter-from-class="opacity-0 max-h-0"
          enter-to-class="opacity-100 max-h-[500px]"
          leave-from-class="opacity-100 max-h-[500px]"
          leave-to-class="opacity-0 max-h-0"
        >
          <div v-if="getWeightState(integration.id).enabled" class="mt-4 space-y-3 overflow-hidden">
            <div v-for="slider in weightSliders" :key="slider.key" class="space-y-1">
              <div class="flex justify-between text-sm">
                <span class="font-medium text-foreground">{{ slider.label }}</span>
                <span class="text-muted-foreground font-mono tabular-nums">
                  {{ getWeightValue(integration.id, slider.key) }} / 10
                </span>
              </div>
              <UiSlider
                :model-value="[getWeightValue(integration.id, slider.key)]"
                :min="0"
                :max="10"
                :step="1"
                class="w-full"
                @update:model-value="
                  (v: number[] | undefined) => {
                    if (v && v[0] != null) updateWeight(integration.id, slider.key, v[0]);
                  }
                "
              />
              <p class="text-xs text-muted-foreground">
                {{ slider.description }}
              </p>
            </div>
          </div>
        </Transition>
      </div>

      <!-- Per-Integration Threshold Overrides -->
      <div class="border-t border-border px-6 py-3">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-2">
            <UiSwitch
              :model-value="getThresholdState(integration.id).enabled"
              @update:model-value="(val: boolean) => toggleThresholdOverride(integration.id, val)"
            />
            <UiLabel class="text-sm font-medium cursor-pointer">
              {{ $t('settings.overrideThresholds') }}
            </UiLabel>
          </div>
        </div>

        <!-- Collapsible Threshold Sliders -->
        <Transition
          enter-active-class="transition-all duration-300 ease-out"
          leave-active-class="transition-all duration-200 ease-in"
          enter-from-class="opacity-0 max-h-0"
          enter-to-class="opacity-100 max-h-[300px]"
          leave-from-class="opacity-100 max-h-[300px]"
          leave-to-class="opacity-0 max-h-0"
        >
          <div
            v-if="getThresholdState(integration.id).enabled"
            class="mt-4 space-y-3 overflow-hidden"
          >
            <div class="space-y-1">
              <div class="flex justify-between text-sm">
                <span class="font-medium text-foreground">{{ $t('settings.thresholdPct') }}</span>
                <span class="text-muted-foreground font-mono tabular-nums">
                  {{ getThresholdState(integration.id).thresholdPct }}%
                </span>
              </div>
              <UiSlider
                :model-value="[getThresholdState(integration.id).thresholdPct]"
                :min="50"
                :max="99"
                :step="1"
                class="w-full"
                @update:model-value="
                  (v: number[] | undefined) => {
                    if (v && v[0] != null) updateThreshold(integration.id, 'thresholdPct', v[0]);
                  }
                "
              />
              <p class="text-xs text-muted-foreground">
                {{ $t('settings.thresholdPctDesc') }}
              </p>
            </div>

            <div class="space-y-1">
              <div class="flex justify-between text-sm">
                <span class="font-medium text-foreground">{{ $t('settings.targetPct') }}</span>
                <span class="text-muted-foreground font-mono tabular-nums">
                  {{ getThresholdState(integration.id).targetPct }}%
                </span>
              </div>
              <UiSlider
                :model-value="[getThresholdState(integration.id).targetPct]"
                :min="50"
                :max="99"
                :step="1"
                class="w-full"
                @update:model-value="
                  (v: number[] | undefined) => {
                    if (v && v[0] != null) updateThreshold(integration.id, 'targetPct', v[0]);
                  }
                "
              />
              <p class="text-xs text-muted-foreground">
                {{ $t('settings.targetPctDesc') }}
              </p>
            </div>
          </div>
        </Transition>
      </div>

      <UiCardFooter class="border-t border-border flex items-center justify-between">
        <div class="flex gap-2">
          <UiButton variant="outline" size="sm" @click="testConnection(integration)">
            {{ $t('common.test') }}
          </UiButton>
          <UiButton variant="outline" size="sm" @click="openEditModal(integration)">
            {{ $t('common.edit') }}
          </UiButton>
        </div>
        <UiButton variant="destructive" size="sm" @click="deleteIntegration(integration)">
          {{ $t('common.delete') }}
        </UiButton>
      </UiCardFooter>
    </UiCard>
  </div>

  <!-- Integration Modal -->
  <UiDialog
    :open="showModal"
    @update:open="
      (val: boolean) => {
        showModal = val;
      }
    "
  >
    <UiDialogContent class="max-w-md">
      <UiDialogHeader>
        <UiDialogTitle>
          {{ editingIntegration ? 'Edit Integration' : 'Add Integration' }}
        </UiDialogTitle>
      </UiDialogHeader>

      <form class="space-y-4" @submit.prevent="onSubmit">
        <div class="space-y-1.5">
          <UiLabel>Type</UiLabel>
          <UiSelect v-model="formState.type" :disabled="!!editingIntegration">
            <UiSelectTrigger class="w-full">
              <UiSelectValue placeholder="Select type" />
            </UiSelectTrigger>
            <UiSelectContent>
              <UiSelectItem value="sonarr">Sonarr</UiSelectItem>
              <UiSelectItem value="radarr">Radarr</UiSelectItem>
              <UiSelectItem value="lidarr">Lidarr</UiSelectItem>
              <UiSelectItem value="readarr">Readarr</UiSelectItem>
              <UiSelectItem value="plex">Plex</UiSelectItem>
              <UiSelectItem value="jellyfin">Jellyfin</UiSelectItem>
              <UiSelectItem value="emby">Emby</UiSelectItem>
              <UiSelectItem value="tautulli">Tautulli</UiSelectItem>
              <UiSelectItem value="overseerr">Overseerr</UiSelectItem>
            </UiSelectContent>
          </UiSelect>
        </div>

        <div class="space-y-1.5">
          <UiLabel>Name</UiLabel>
          <UiInput v-model="formState.name" type="text" :placeholder="namePlaceholder" />
        </div>

        <div class="space-y-1.5">
          <UiLabel>URL</UiLabel>
          <UiInput v-model="formState.url" type="text" :placeholder="urlPlaceholder" />
          <p class="text-xs text-muted-foreground/70">{{ urlHelp }}</p>
        </div>

        <div class="space-y-1.5">
          <UiLabel>{{ formState.type === 'plex' ? 'Plex Token' : 'API Key' }}</UiLabel>
          <UiInput
            v-model="formState.apiKey"
            :type="editingIntegration && formState.apiKey.includes('•') ? 'text' : 'password'"
            :placeholder="
              editingIntegration
                ? 'Enter new API key to change, or leave as-is'
                : 'Enter API key or token'
            "
            @focus="onApiKeyFocus"
          />
          <!-- Plex OAuth Sign-in Button -->
          <template v-if="formState.type === 'plex'">
            <div class="pt-1 space-y-2">
              <UiButton
                type="button"
                class="w-full bg-[#e5a00d] text-black font-semibold hover:bg-[#c98c0b]"
                :disabled="plexAuthLoading"
                @click="startPlexAuth"
              >
                <template v-if="plexAuthLoading">
                  <component :is="LoaderCircleIcon" class="w-4 h-4 animate-spin" />
                  Waiting for Plex authorization…
                </template>
                <template v-else>
                  <component :is="LogInIcon" class="w-4 h-4" />
                  Sign in with Plex
                </template>
              </UiButton>
              <p class="text-xs text-muted-foreground/70">
                Opens Plex in a new window to authorize Capacitarr
              </p>
            </div>
            <UiSeparator class="my-1" />
            <p class="text-xs text-muted-foreground/70">
              Or enter your token manually: open any library item in Plex Web → Get Info → View XML
              → look for <code class="font-mono text-[11px]">X-Plex-Token</code> in the URL.
            </p>
          </template>
        </div>

        <UiAlert v-if="formError" variant="destructive">
          <UiAlertDescription>{{ formError }}</UiAlertDescription>
        </UiAlert>
      </form>

      <UiDialogFooter class="flex items-center justify-between">
        <UiButton variant="outline" @click="testFormConnection"> Test Connection </UiButton>
        <div class="flex gap-2">
          <UiButton variant="ghost" @click="showModal = false">Cancel</UiButton>
          <UiButton :disabled="saving" @click="onSubmit">
            {{ editingIntegration ? 'Save' : 'Add' }}
          </UiButton>
        </div>
      </UiDialogFooter>
    </UiDialogContent>
  </UiDialog>
</template>

<script setup lang="ts">
import {
  PlusIcon,
  HardDriveIcon,
  LoaderCircleIcon,
  LinkIcon,
  KeyIcon,
  ClockIcon,
  AlertTriangleIcon,
  LogInIcon,
  SlidersHorizontalIcon,
} from 'lucide-vue-next';
import type { IntegrationConfig, ConnectionTestResult, ApiError } from '~/types/api';
import { PlexOAuth } from '~/utils/plexOAuth';
import {
  typeIcon,
  typeColor,
  typeTextColor,
  namePlaceholders,
  urlPlaceholders,
  urlHelpTexts,
} from '~/utils/integrationHelpers';

const api = useApi();
const { addToast } = useToast();
const { t } = useI18n();

const loading = ref(true);
const integrations = ref<IntegrationConfig[]>([]);
const showModal = ref(false);
const editingIntegration = ref<IntegrationConfig | null>(null);
const saving = ref(false);
const formError = ref('');

// ─── Per-integration custom weight overrides (local state only) ──────────────
interface WeightOverrides {
  enabled: boolean;
  watchHistoryWeight: number;
  lastWatchedWeight: number;
  fileSizeWeight: number;
  ratingWeight: number;
  timeInLibraryWeight: number;
  seriesStatusWeight: number;
}

const defaultWeights: Omit<WeightOverrides, 'enabled'> = {
  watchHistoryWeight: 5,
  lastWatchedWeight: 5,
  fileSizeWeight: 5,
  ratingWeight: 5,
  timeInLibraryWeight: 5,
  seriesStatusWeight: 5,
};

const customWeightsState = reactive<Record<number, WeightOverrides>>({});

function getWeightState(integrationId: number): WeightOverrides {
  if (!customWeightsState[integrationId]) {
    customWeightsState[integrationId] = {
      enabled: false,
      ...defaultWeights,
    };
  }
  // Non-null: we just ensured the entry exists above
  return customWeightsState[integrationId]!;
}

function toggleCustomWeights(integrationId: number, enabled: boolean) {
  const state = getWeightState(integrationId);
  state.enabled = enabled;
}

function updateWeight(integrationId: number, key: string, value: number) {
  (getWeightState(integrationId) as unknown as Record<string, number>)[key] = value;
}

function getWeightValue(integrationId: number, key: string): number {
  return Number(
    (getWeightState(integrationId) as unknown as Record<string, number | undefined>)[key] ?? 5,
  );
}

const weightSliders = computed(() => [
  {
    key: 'watchHistoryWeight',
    label: t('settings.weightWatchHistory'),
    description: t('settings.weightWatchHistoryDesc'),
  },
  {
    key: 'lastWatchedWeight',
    label: t('settings.weightLastWatched'),
    description: t('settings.weightLastWatchedDesc'),
  },
  {
    key: 'fileSizeWeight',
    label: t('settings.weightFileSize'),
    description: t('settings.weightFileSizeDesc'),
  },
  {
    key: 'ratingWeight',
    label: t('settings.weightRating'),
    description: t('settings.weightRatingDesc'),
  },
  {
    key: 'timeInLibraryWeight',
    label: t('settings.weightTimeInLibrary'),
    description: t('settings.weightTimeInLibraryDesc'),
  },
  {
    key: 'seriesStatusWeight',
    label: t('settings.weightSeriesStatus'),
    description: t('settings.weightSeriesStatusDesc'),
  },
]);

// ─── Per-integration threshold overrides (local state, wired to API in Phase 6) ─
interface ThresholdOverride {
  enabled: boolean;
  thresholdPct: number;
  targetPct: number;
}

const thresholdState = reactive<Record<number, ThresholdOverride>>({});

function getThresholdState(integrationId: number): ThresholdOverride {
  if (!thresholdState[integrationId]) {
    // Initialize from integration's stored thresholds if available
    const integration = integrations.value.find((i) => i.id === integrationId);
    const hasOverride = integration?.thresholdPct != null || integration?.targetPct != null;
    thresholdState[integrationId] = {
      enabled: hasOverride,
      thresholdPct: integration?.thresholdPct ?? 85,
      targetPct: integration?.targetPct ?? 80,
    };
  }
  return thresholdState[integrationId]!;
}

function toggleThresholdOverride(integrationId: number, enabled: boolean) {
  const state = getThresholdState(integrationId);
  state.enabled = enabled;
}

function updateThreshold(integrationId: number, key: 'thresholdPct' | 'targetPct', value: number) {
  const state = getThresholdState(integrationId);
  state[key] = value;
}

const formState = reactive({ type: 'sonarr', name: '', url: '', apiKey: '' });

// ─── Plex OAuth ──────────────────────────────────────────────────────────────
const plexAuthLoading = ref(false);
let plexOAuth: PlexOAuth | null = null;

async function startPlexAuth() {
  plexAuthLoading.value = true;
  try {
    plexOAuth = new PlexOAuth();
    const authToken = await plexOAuth.login();
    formState.apiKey = authToken;
    addToast('Plex authorized successfully!', 'success');
  } catch (e) {
    const msg = e instanceof Error ? e.message : 'Unknown error';
    if (msg.includes('closed')) {
      addToast('Plex authorization cancelled', 'info');
    } else {
      addToast('Failed to start Plex authorization: ' + msg, 'error');
    }
  } finally {
    plexAuthLoading.value = false;
    plexOAuth = null;
  }
}

onBeforeUnmount(() => {
  plexOAuth?.abort();
});

// ─── Computed placeholders ───────────────────────────────────────────────────
const namePlaceholder = computed(() => namePlaceholders[formState.type] || 'Integration Name');
const urlPlaceholder = computed(() => urlPlaceholders[formState.type] || 'http://localhost:8080');
const urlHelp = computed(() => urlHelpTexts[formState.type] || 'The base URL of your integration.');

// ─── CRUD operations ─────────────────────────────────────────────────────────
async function fetchIntegrations() {
  loading.value = true;
  try {
    integrations.value = (await api('/api/v1/integrations')) as IntegrationConfig[];
  } catch {
    addToast('Failed to load integrations', 'error');
  } finally {
    loading.value = false;
  }
}

function openAddModal() {
  editingIntegration.value = null;
  Object.assign(formState, { type: 'sonarr', name: '', url: '', apiKey: '' });
  formError.value = '';
  showModal.value = true;
}

function onApiKeyFocus() {
  if (formState.apiKey.includes('•')) formState.apiKey = '';
}

function openEditModal(integration: IntegrationConfig) {
  editingIntegration.value = integration;
  Object.assign(formState, {
    type: integration.type,
    name: integration.name,
    url: integration.url,
    apiKey: integration.apiKey,
  });
  formError.value = '';
  showModal.value = true;
}

async function onSubmit() {
  saving.value = true;
  formError.value = '';
  try {
    if (editingIntegration.value) {
      await api(`/api/v1/integrations/${editingIntegration.value.id}`, {
        method: 'PUT',
        body: { ...formState, enabled: editingIntegration.value.enabled },
      });
    } else {
      await api('/api/v1/integrations', { method: 'POST', body: formState });
    }
    showModal.value = false;
    addToast('Integration saved', 'success');
    await fetchIntegrations();
  } catch (e: unknown) {
    formError.value = (e as ApiError)?.data?.error || 'Failed to save integration';
    addToast(formError.value, 'error');
  } finally {
    saving.value = false;
  }
}

async function deleteIntegration(integration: IntegrationConfig) {
  if (!confirm(`Delete ${integration.name}? This cannot be undone.`)) return;
  try {
    await api(`/api/v1/integrations/${integration.id}`, { method: 'DELETE' });
    addToast('Integration deleted', 'success');
    await fetchIntegrations();
  } catch {
    addToast('Failed to delete integration', 'error');
  }
}

async function testConnection(integration: IntegrationConfig) {
  try {
    const result = (await api('/api/v1/integrations/test', {
      method: 'POST',
      body: {
        type: integration.type,
        url: integration.url,
        apiKey: integration.apiKey,
        integrationId: integration.id,
      },
    })) as ConnectionTestResult;
    addToast(
      result.success ? 'Connection successful!' : `Connection failed: ${result.error}`,
      result.success ? 'success' : 'error',
    );
  } catch {
    addToast('Connection test failed', 'error');
  }
}

async function testFormConnection() {
  try {
    const body: Record<string, unknown> = {
      type: formState.type,
      url: formState.url,
      apiKey: formState.apiKey,
    };
    if (editingIntegration.value) body.integrationId = editingIntegration.value.id;
    const result = (await api('/api/v1/integrations/test', {
      method: 'POST',
      body,
    })) as ConnectionTestResult;
    if (result.success) {
      formError.value = '';
      addToast('Connection successful!', 'success');
    } else {
      formError.value = result.error || 'Connection failed';
      addToast(formError.value, 'error');
    }
  } catch {
    formError.value = 'Connection test failed';
    addToast('Connection test failed', 'error');
  }
}

onMounted(() => {
  fetchIntegrations();
});
</script>
