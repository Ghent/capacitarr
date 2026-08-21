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
      :class="['overflow-hidden transition-opacity', { 'opacity-50': !integration.enabled }]"
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
            <UiLabel class="text-xs text-muted-foreground">
              {{
                integration.enabled
                  ? $t('settings.integrationEnabled')
                  : $t('settings.integrationDisabled')
              }}
            </UiLabel>
            <UiSwitch
              :model-value="integration.enabled"
              @update:model-value="(val: boolean) => toggleEnabled(integration, val)"
            />
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
            {{ formatApiKeyForDisplay(integration.apiKey) }}
          </span>
        </div>
        <div v-if="integration.lastSync" class="flex items-center gap-2">
          <component :is="ClockIcon" class="w-3.5 h-3.5 shrink-0" />
          <span>Synced <DateDisplay :date="integration.lastSync" /></span>
        </div>
        <div v-if="integration.lastError" class="space-y-1">
          <div class="flex items-start gap-2 text-red-500 min-w-0">
            <component :is="AlertTriangleIcon" class="w-3.5 h-3.5 shrink-0 mt-0.5" />
            <span class="text-xs break-all">{{ integration.lastError }}</span>
          </div>
          <p class="text-xs text-muted-foreground/70 pl-5.5">
            {{ $t('settings.integrationErrorHint') }}
          </p>
          <div v-if="integration.consecutiveFailures > 0" class="flex items-center gap-1.5 pl-5.5">
            <span class="relative flex h-2 w-2 shrink-0">
              <span
                class="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75"
              />
              <span class="relative inline-flex rounded-full h-2 w-2 bg-amber-500" />
            </span>
            <span class="text-xs text-amber-500 font-medium">
              Attempting recovery ({{ integration.consecutiveFailures }}
              {{ integration.consecutiveFailures === 1 ? 'failure' : 'failures' }})
            </span>
          </div>
        </div>

        <!-- Inline feature toggles (issue #9) -->
        <div
          v-if="
            integration.type === 'sonarr' ||
            collectionDeletionTypes.has(integration.type) ||
            importExclusionTypes.has(integration.type)
          "
          class="pt-2 space-y-2 border-t border-border mt-2"
        >
          <!-- Show-Level Evaluation (Sonarr only) -->
          <div v-if="integration.type === 'sonarr'" class="space-y-1">
            <div class="flex items-center justify-between gap-2">
              <div class="flex items-center gap-1.5">
                <TvIcon class="w-3.5 h-3.5 text-blue-500 shrink-0" />
                <span class="text-xs">Show-Level Only</span>
              </div>
              <UiSwitch
                :model-value="integration.showLevelOnlyOverride || integration.showLevelOnly"
                :disabled="integration.showLevelOnlyOverride"
                @update:model-value="
                  (val: boolean) => toggleCardSetting(integration, 'showLevelOnly', val)
                "
              />
            </div>
            <p v-if="integration.showLevelOnlyOverride" class="text-xs text-amber-500">
              {{ integration.showLevelOnlyOverrideReason }}
            </p>
          </div>
          <!-- Collection Deletion (supported types) -->
          <div
            v-if="collectionDeletionTypes.has(integration.type)"
            class="flex items-center justify-between gap-2"
          >
            <div class="flex items-center gap-1.5">
              <LayersIcon
                class="w-3.5 h-3.5 shrink-0"
                :class="integration.collectionDeletion ? 'text-destructive' : 'text-indigo-500'"
              />
              <span
                class="text-xs"
                :class="integration.collectionDeletion ? 'text-destructive font-medium' : ''"
                >Collection Deletion</span
              >
            </div>
            <UiSwitch
              :model-value="integration.collectionDeletion"
              @update:model-value="
                (val: boolean) => requestCollectionDeletionToggle(integration, val, 'card')
              "
            />
          </div>
          <!-- Import Exclusion (*arr types only) -->
          <div
            v-if="importExclusionTypes.has(integration.type)"
            class="flex items-center justify-between gap-2"
          >
            <div class="flex items-center gap-1.5">
              <ShieldBanIcon class="w-3.5 h-3.5 text-amber-500 shrink-0" />
              <span class="text-xs">Import Exclusion on Delete</span>
            </div>
            <UiSwitch
              :model-value="integration.addImportExclusion"
              @update:model-value="
                (val: boolean) => toggleCardSetting(integration, 'addImportExclusion', val)
              "
            />
          </div>
        </div>
      </UiCardContent>

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
              <UiSelectItem value="jellystat">Jellystat</UiSelectItem>
              <UiSelectItem value="tracearr">Tracearr</UiSelectItem>
              <UiSelectItem value="seerr">Seerr</UiSelectItem>
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
            :type="editingIntegration && isMaskedApiKey(formState.apiKey) ? 'text' : 'password'"
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

        <!-- Collection Deletion toggle (only for supported types) -->
        <div v-if="collectionDeletionTypes.has(formState.type)" class="space-y-2 pt-1">
          <UiSeparator />
          <div class="flex items-center justify-between gap-3">
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-1.5">
                <LayersIcon
                  class="w-4 h-4 shrink-0"
                  :class="formState.collectionDeletion ? 'text-destructive' : 'text-indigo-500'"
                />
                <UiLabel
                  class="cursor-pointer"
                  :class="formState.collectionDeletion ? 'text-destructive font-medium' : ''"
                  >Collection Deletion</UiLabel
                >
              </div>
              <p class="text-xs text-muted-foreground mt-1">
                {{ collectionDeletionDescriptions[formState.type] || '' }}
              </p>
            </div>
            <UiSwitch
              :model-value="formState.collectionDeletion"
              @update:model-value="
                (val: boolean) => requestCollectionDeletionToggle(null, val, 'modal')
              "
            />
          </div>
          <NuxtLink
            to="/help#collection-deletion"
            class="text-xs text-primary hover:underline inline-flex items-center gap-1"
          >
            <InfoIcon class="w-3 h-3" />
            Learn more about collection deletion
          </NuxtLink>
        </div>

        <!-- Show-Level Evaluation toggle (Sonarr only) -->
        <div v-if="formState.type === 'sonarr'" class="space-y-2 pt-1">
          <UiSeparator />
          <div class="flex items-center justify-between gap-3">
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-1.5">
                <TvIcon class="w-4 h-4 text-blue-500 shrink-0" />
                <UiLabel class="cursor-pointer">Show-Level Evaluation</UiLabel>
              </div>
              <p
                v-if="editingIntegration?.showLevelOnlyOverride"
                class="text-xs text-amber-500 mt-1"
              >
                {{ editingIntegration.showLevelOnlyOverrideReason }}
              </p>
              <p v-else class="text-xs text-muted-foreground mt-1">
                Evaluate and delete entire shows instead of individual seasons. When a show is
                selected for deletion, all seasons and episodes are removed.
              </p>
            </div>
            <UiSwitch
              :model-value="editingIntegration?.showLevelOnlyOverride || formState.showLevelOnly"
              :disabled="!!editingIntegration?.showLevelOnlyOverride"
              @update:model-value="(val: boolean) => (formState.showLevelOnly = val)"
            />
          </div>
          <NuxtLink
            to="/help#show-level-evaluation"
            class="text-xs text-primary hover:underline inline-flex items-center gap-1"
          >
            <InfoIcon class="w-3 h-3" />
            Learn more about show-level evaluation
          </NuxtLink>
        </div>

        <!-- Import Exclusion toggle (*arr types only) -->
        <div v-if="importExclusionTypes.has(formState.type)" class="space-y-2 pt-1">
          <UiSeparator />
          <div class="flex items-center justify-between gap-3">
            <div class="flex-1 min-w-0">
              <div class="flex items-center gap-1.5">
                <ShieldBanIcon class="w-4 h-4 text-amber-500 shrink-0" />
                <UiLabel class="cursor-pointer">Import Exclusion on Delete</UiLabel>
              </div>
              <p class="text-xs text-muted-foreground mt-1">
                When Capacitarr deletes media, add it to the import exclusion list to prevent import
                lists from automatically re-adding it.
              </p>
            </div>
            <UiSwitch
              :model-value="formState.addImportExclusion"
              @update:model-value="(val: boolean) => (formState.addImportExclusion = val)"
            />
          </div>
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

  <!-- Collection Deletion Confirmation Dialog -->
  <UiDialog
    :open="showCollectionDeleteConfirm"
    @update:open="
      (val: boolean) => {
        if (!val) cancelCollectionDeletionToggle();
      }
    "
  >
    <UiDialogContent class="max-w-md">
      <UiDialogHeader>
        <UiDialogTitle class="text-destructive">Enable Collection Deletion?</UiDialogTitle>
        <UiDialogDescription class="space-y-3">
          <p>
            When a movie gets selected for deletion,
            <strong>every other movie in its collection</strong>
            will be deleted too. One low-scoring movie in a large franchise could trigger the
            deletion of dozens of files.
          </p>
          <p>
            This is permanent and cannot be undone. Consider using
            <strong>dry-run mode</strong> first while you review how this feature behaves.
          </p>
          <NuxtLink
            to="/help#collection-deletion"
            class="text-xs text-primary hover:underline inline-flex items-center gap-1"
          >
            <InfoIcon class="w-3 h-3" />
            Learn more about collection deletion and safety features
          </NuxtLink>
        </UiDialogDescription>
      </UiDialogHeader>
      <UiDialogFooter class="flex gap-2 justify-end">
        <UiButton variant="outline" @click="cancelCollectionDeletionToggle()"> Cancel </UiButton>
        <UiButton variant="destructive" @click="confirmCollectionDeletionToggle()">
          Yes, enable collection deletion
        </UiButton>
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
  LayersIcon,
  TvIcon,
  InfoIcon,
  ShieldBanIcon,
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
import { toast } from 'vue-sonner';
import { formatApiKeyForDisplay, isMaskedApiKey } from '~/utils/apiKeyDisplay';

const api = useApi();
const { t } = useI18n();

const loading = ref(true);
const integrations = ref<IntegrationConfig[]>([]);
const showModal = ref(false);
const editingIntegration = ref<IntegrationConfig | null>(null);
const saving = ref(false);
const formError = ref('');

const formState = reactive({
  type: 'sonarr',
  name: '',
  url: '',
  apiKey: '',
  collectionDeletion: false,
  showLevelOnly: false,
  addImportExclusion: true,
});

/** Integration types that support collection deletion */
const collectionDeletionTypes = new Set(['radarr', 'plex', 'jellyfin', 'emby']);

/** Integration types that support import exclusion (*arr media managers only) */
const importExclusionTypes = new Set(['sonarr', 'radarr', 'lidarr', 'readarr']);

/** Description text for the collection deletion toggle per integration type */
const collectionDeletionDescriptions: Record<string, string> = {
  radarr:
    'Uses TMDb movie collections — curated franchise groupings like "The Lord of the Rings Collection".',
  plex: 'Uses Plex library collections. Includes automatic and user-created collections. Custom collections can group unrelated media.',
  jellyfin:
    'Uses Jellyfin Box Sets — groups of related movies that were auto-detected or manually organized.',
  emby: 'Uses Emby Box Sets — groups of related movies that were auto-detected or manually organized.',
};

// ─── Enable/Disable toggle ──────────────────────────────────────────────────
async function toggleEnabled(integration: IntegrationConfig, enabled: boolean) {
  // Optimistic update — toggle immediately in the UI
  const previous = integration.enabled;
  integration.enabled = enabled;

  try {
    await api(`/api/v1/integrations/${integration.id}`, {
      method: 'PUT',
      body: { enabled },
    });
    toast.success(
      t('settings.integrationToggled', {
        action: enabled ? t('common.enabled') : t('common.disabled'),
      }),
    );
  } catch {
    // Revert on failure
    integration.enabled = previous;
    toast.error(t('settings.integrationToggleFailed'));
  }
}

// ─── Card-level feature toggle (showLevelOnly, collectionDeletion, addImportExclusion) ─
async function toggleCardSetting(
  integration: IntegrationConfig,
  key: 'showLevelOnly' | 'collectionDeletion' | 'addImportExclusion',
  value: boolean,
) {
  const previous = integration[key];
  integration[key] = value;

  try {
    await api(`/api/v1/integrations/${integration.id}`, {
      method: 'PUT',
      body: { [key]: value },
    });
    toast.success(
      t('settings.integrationToggled', {
        action: value ? t('common.enabled') : t('common.disabled'),
      }),
    );
  } catch {
    integration[key] = previous;
    toast.error(t('settings.integrationToggleFailed'));
  }
}

// ─── Collection Deletion confirmation dialog ─────────────────────────────────
const showCollectionDeleteConfirm = ref(false);
// Store the pending toggle context so we can apply it after confirmation
const pendingCollectionToggle = ref<{
  integration: IntegrationConfig | null;
  source: 'card' | 'modal';
} | null>(null);

/**
 * Intercept collection deletion toggle. If enabling, show confirmation dialog.
 * If disabling, proceed immediately (no confirmation needed to turn OFF).
 */
function requestCollectionDeletionToggle(
  integration: IntegrationConfig | null,
  value: boolean,
  source: 'card' | 'modal',
) {
  if (!value) {
    // Turning OFF — no confirmation needed
    if (source === 'card' && integration) {
      toggleCardSetting(integration, 'collectionDeletion', false);
    } else {
      formState.collectionDeletion = false;
    }
    return;
  }
  // Turning ON — show confirmation dialog
  pendingCollectionToggle.value = { integration, source };
  showCollectionDeleteConfirm.value = true;
}

function confirmCollectionDeletionToggle() {
  const ctx = pendingCollectionToggle.value;
  if (!ctx) return;

  if (ctx.source === 'card' && ctx.integration) {
    toggleCardSetting(ctx.integration, 'collectionDeletion', true);
  } else {
    formState.collectionDeletion = true;
  }

  showCollectionDeleteConfirm.value = false;
  pendingCollectionToggle.value = null;
}

function cancelCollectionDeletionToggle() {
  showCollectionDeleteConfirm.value = false;
  pendingCollectionToggle.value = null;
}

// ─── Plex OAuth ──────────────────────────────────────────────────────────────
const plexAuthLoading = ref(false);
let plexOAuth: PlexOAuth | null = null;

async function startPlexAuth() {
  plexAuthLoading.value = true;
  try {
    plexOAuth = new PlexOAuth();
    const authToken = await plexOAuth.login();
    formState.apiKey = authToken;
    toast.success('Plex authorized successfully!');
  } catch (e) {
    const msg = e instanceof Error ? e.message : 'Unknown error';
    if (msg.includes('closed')) {
      toast.info('Plex authorization cancelled');
    } else {
      toast.error('Failed to start Plex authorization: ' + msg);
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
async function fetchIntegrations(showSpinner = true) {
  if (showSpinner) loading.value = true;
  try {
    integrations.value = (await api('/api/v1/integrations')) as IntegrationConfig[];
  } catch {
    toast.error('Failed to load integrations');
  } finally {
    loading.value = false;
  }
}

function openAddModal() {
  editingIntegration.value = null;
  Object.assign(formState, {
    type: 'sonarr',
    name: '',
    url: '',
    apiKey: '',
    collectionDeletion: false,
    showLevelOnly: false,
    addImportExclusion: true,
  });
  formError.value = '';
  showModal.value = true;
}

function onApiKeyFocus() {
  if (isMaskedApiKey(formState.apiKey)) formState.apiKey = '';
}

function openEditModal(integration: IntegrationConfig) {
  editingIntegration.value = integration;
  Object.assign(formState, {
    type: integration.type,
    name: integration.name,
    url: integration.url,
    apiKey: integration.apiKey,
    collectionDeletion: integration.collectionDeletion ?? false,
    showLevelOnly: integration.showLevelOnly ?? false,
    addImportExclusion: integration.addImportExclusion ?? true,
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
    toast.success('Integration saved');
    await fetchIntegrations();
  } catch (e: unknown) {
    formError.value = (e as ApiError)?.data?.error || 'Failed to save integration';
    toast.error(formError.value);
  } finally {
    saving.value = false;
  }
}

async function deleteIntegration(integration: IntegrationConfig) {
  if (!confirm(`Delete ${integration.name}? This cannot be undone.`)) return;
  try {
    await api(`/api/v1/integrations/${integration.id}`, { method: 'DELETE' });
    toast.success('Integration deleted');
    await fetchIntegrations();
  } catch {
    toast.error('Failed to delete integration');
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
    if (result.success) {
      toast.success('Connection successful!');
    } else {
      toast.error(`Connection failed: ${result.error}`);
    }
    // Silently refetch to reflect updated lastError / lastSync status
    await fetchIntegrations(false);
  } catch {
    toast.error('Connection test failed');
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
      toast.success('Connection successful!');
    } else {
      formError.value = result.error || 'Connection failed';
      toast.error(formError.value);
    }
  } catch {
    formError.value = 'Connection test failed';
    toast.error('Connection test failed');
  }
}

onMounted(() => {
  fetchIntegrations();
});
</script>
