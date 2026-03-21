<template>
  <div class="space-y-8">
    <!-- Export Section -->
    <UiCard v-motion v-bind="cardEntrance">
      <UiCardHeader>
        <UiCardTitle>{{ $t('settings.export') }}</UiCardTitle>
        <UiCardDescription>{{ $t('settings.exportDesc') }}</UiCardDescription>
      </UiCardHeader>
      <UiCardContent class="space-y-4">
        <UiLabel class="text-sm font-medium">{{ $t('settings.sectionsToExport') }}</UiLabel>
        <div class="space-y-3">
          <div class="flex items-start gap-3">
            <UiCheckbox id="export-preferences" v-model="exportSections.preferences" />
            <div class="grid gap-0.5 leading-none">
              <UiLabel for="export-preferences" class="cursor-pointer">
                {{ $t('settings.sectionPreferences') }}
              </UiLabel>
              <p class="text-xs text-muted-foreground">
                {{ $t('settings.sectionPreferencesDesc') }}
              </p>
            </div>
          </div>
          <div class="flex items-start gap-3">
            <UiCheckbox id="export-rules" v-model="exportSections.rules" />
            <UiLabel for="export-rules" class="cursor-pointer">
              {{ $t('settings.sectionRules') }}
            </UiLabel>
          </div>
          <div class="flex items-start gap-3">
            <UiCheckbox id="export-integrations" v-model="exportSections.integrations" />
            <div class="grid gap-0.5 leading-none">
              <UiLabel for="export-integrations" class="cursor-pointer">
                {{ $t('settings.sectionIntegrations') }}
              </UiLabel>
              <p class="text-xs text-muted-foreground">
                {{ $t('settings.sectionIntegrationsDesc') }}
              </p>
            </div>
          </div>
          <div class="flex items-start gap-3">
            <UiCheckbox id="export-diskgroups" v-model="exportSections.diskGroups" />
            <UiLabel for="export-diskgroups" class="cursor-pointer">
              {{ $t('settings.sectionDiskGroups') }}
            </UiLabel>
          </div>
          <div class="flex items-start gap-3">
            <UiCheckbox id="export-notifications" v-model="exportSections.notificationChannels" />
            <div class="grid gap-0.5 leading-none">
              <UiLabel for="export-notifications" class="cursor-pointer">
                {{ $t('settings.sectionNotifications') }}
              </UiLabel>
              <p class="text-xs text-muted-foreground">
                {{ $t('settings.sectionNotificationsDesc') }}
              </p>
            </div>
          </div>
        </div>
        <UiButton :disabled="!anyExportSelected || exporting" @click="doExport">
          <component :is="DownloadIcon" class="w-4 h-4" />
          {{ exporting ? $t('common.loading') : $t('settings.exportButton') }}
        </UiButton>
      </UiCardContent>
    </UiCard>

    <!-- Import Section -->
    <UiCard v-motion v-bind="cardEntrance">
      <UiCardHeader>
        <UiCardTitle>{{ $t('settings.import') }}</UiCardTitle>
        <UiCardDescription>{{ $t('settings.importDesc') }}</UiCardDescription>
      </UiCardHeader>
      <UiCardContent class="space-y-4">
        <!-- File upload area -->
        <div
          class="relative flex flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed border-muted-foreground/25 p-8 transition-colors hover:border-muted-foreground/50 cursor-pointer"
          :class="{ 'border-primary bg-primary/5': isDragOver }"
          @click="fileInputRef?.click()"
          @dragover.prevent="isDragOver = true"
          @dragleave="isDragOver = false"
          @drop.prevent="onFileDrop"
        >
          <component :is="UploadIcon" class="w-8 h-8 text-muted-foreground/50" />
          <p class="text-sm text-muted-foreground">
            {{ $t('settings.importDragDrop') }}
          </p>
          <input
            ref="fileInputRef"
            type="file"
            accept=".json"
            class="hidden"
            @change="onFileSelected"
          />
        </div>

        <!-- Preview after file is loaded -->
        <template v-if="parsedPayload">
          <UiCard class="bg-muted/30">
            <UiCardHeader class="pb-3">
              <UiCardTitle class="text-sm">{{ $t('settings.importPreview') }}</UiCardTitle>
            </UiCardHeader>
            <UiCardContent class="space-y-1 text-sm text-muted-foreground">
              <p v-if="parsedPayload.preferences">✓ {{ $t('settings.sectionPreferences') }}</p>
              <p v-if="parsedPayload.rules?.length">
                ✓
                {{
                  $t(
                    'settings.importResultRules',
                    { count: parsedPayload.rules.length },
                    parsedPayload.rules.length,
                  )
                }}
              </p>
              <p v-if="parsedPayload.integrations?.length">
                ✓
                {{
                  $t(
                    'settings.importResultIntegrations',
                    { count: parsedPayload.integrations.length },
                    parsedPayload.integrations.length,
                  )
                }}
              </p>
              <p v-if="parsedPayload.diskGroups?.length">
                ✓
                {{
                  $t(
                    'settings.importResultDiskGroups',
                    { count: parsedPayload.diskGroups.length },
                    parsedPayload.diskGroups.length,
                  )
                }}
              </p>
              <p v-if="parsedPayload.notificationChannels?.length">
                ✓
                {{
                  $t(
                    'settings.importResultNotifications',
                    { count: parsedPayload.notificationChannels.length },
                    parsedPayload.notificationChannels.length,
                  )
                }}
              </p>
            </UiCardContent>
          </UiCard>

          <!-- Import section checkboxes -->
          <UiLabel class="text-sm font-medium">{{ $t('settings.sectionsToImport') }}</UiLabel>
          <div class="space-y-3">
            <div v-if="parsedPayload.preferences" class="flex items-center gap-3">
              <UiCheckbox id="import-preferences" v-model="importSections.preferences" />
              <UiLabel for="import-preferences" class="cursor-pointer">
                {{ $t('settings.sectionPreferences') }}
              </UiLabel>
            </div>
            <div v-if="parsedPayload.rules?.length" class="flex items-center gap-3">
              <UiCheckbox id="import-rules" v-model="importSections.rules" />
              <UiLabel for="import-rules" class="cursor-pointer">
                {{ $t('settings.sectionRules') }}
              </UiLabel>
            </div>
            <div v-if="parsedPayload.integrations?.length" class="flex items-center gap-3">
              <UiCheckbox id="import-integrations" v-model="importSections.integrations" />
              <UiLabel for="import-integrations" class="cursor-pointer">
                {{ $t('settings.sectionIntegrations') }}
              </UiLabel>
            </div>
            <div v-if="parsedPayload.diskGroups?.length" class="flex items-center gap-3">
              <UiCheckbox id="import-diskgroups" v-model="importSections.diskGroups" />
              <UiLabel for="import-diskgroups" class="cursor-pointer">
                {{ $t('settings.sectionDiskGroups') }}
              </UiLabel>
            </div>
            <div v-if="parsedPayload.notificationChannels?.length" class="flex items-center gap-3">
              <UiCheckbox id="import-notifications" v-model="importSections.notificationChannels" />
              <UiLabel for="import-notifications" class="cursor-pointer">
                {{ $t('settings.sectionNotifications') }}
              </UiLabel>
            </div>
          </div>

          <!-- Import mode selector -->
          <div class="space-y-2">
            <UiLabel class="text-sm font-medium">{{ $t('settings.importModeLabel') }}</UiLabel>
            <UiRadioGroup v-model="importMode" class="space-y-2">
              <div class="flex items-start gap-3">
                <UiRadioGroupItem id="mode-append" value="append" />
                <div class="grid gap-0.5 leading-none">
                  <UiLabel for="mode-append" class="cursor-pointer">
                    {{ $t('settings.importModeAppend') }}
                  </UiLabel>
                  <p class="text-xs text-muted-foreground">
                    {{ $t('settings.importModeAppendDesc') }}
                  </p>
                </div>
              </div>
              <div class="flex items-start gap-3">
                <UiRadioGroupItem id="mode-replace" value="replace" />
                <div class="grid gap-0.5 leading-none">
                  <UiLabel for="mode-replace" class="cursor-pointer text-destructive">
                    {{ $t('settings.importModeReplace') }}
                  </UiLabel>
                  <p class="text-xs text-muted-foreground">
                    {{ $t('settings.importModeReplaceDesc') }}
                  </p>
                </div>
              </div>
            </UiRadioGroup>
          </div>

          <!-- Replace mode warning -->
          <UiAlert v-if="importMode === 'replace'" variant="destructive">
            <component :is="AlertTriangleIcon" class="w-4 h-4" />
            <UiAlertTitle>{{ $t('common.warning') }}</UiAlertTitle>
            <UiAlertDescription>
              {{ $t('settings.importModeReplaceConfirm') }}
            </UiAlertDescription>
          </UiAlert>

          <!-- Credential warning -->
          <UiAlert variant="default">
            <component :is="AlertTriangleIcon" class="w-4 h-4" />
            <UiAlertTitle>{{ $t('common.warning') }}</UiAlertTitle>
            <UiAlertDescription>
              {{ $t('settings.importCredentialWarning') }}
            </UiAlertDescription>
          </UiAlert>

          <!-- Import button -->
          <div class="flex items-center gap-3">
            <UiButton :disabled="!anyImportSelected || importing" @click="doImport">
              <component :is="UploadIcon" class="w-4 h-4" />
              {{ importing ? $t('common.loading') : $t('settings.importButton') }}
            </UiButton>
            <UiButton variant="outline" @click="clearImport">
              {{ $t('common.cancel') }}
            </UiButton>
          </div>
        </template>

        <!-- Import result -->
        <UiAlert v-if="importResult" variant="default">
          <component :is="CheckCircleIcon" class="w-4 h-4" />
          <UiAlertTitle>{{ $t('settings.importResult') }}</UiAlertTitle>
          <UiAlertDescription class="space-y-1">
            <p v-if="importResult.preferencesImported">
              ✓ {{ $t('settings.importResultPreferences') }}
            </p>
            <p v-if="importResult.rulesImported > 0">
              ✓
              {{
                $t(
                  'settings.importResultRules',
                  { count: importResult.rulesImported },
                  importResult.rulesImported,
                )
              }}
            </p>
            <p v-if="importResult.integrationsImported > 0">
              ✓
              {{
                $t(
                  'settings.importResultIntegrations',
                  { count: importResult.integrationsImported },
                  importResult.integrationsImported,
                )
              }}
            </p>
            <p v-if="importResult.diskGroupsImported > 0">
              ✓
              {{
                $t(
                  'settings.importResultDiskGroups',
                  { count: importResult.diskGroupsImported },
                  importResult.diskGroupsImported,
                )
              }}
            </p>
            <p v-if="importResult.notificationChannelsImported > 0">
              ✓
              {{
                $t(
                  'settings.importResultNotifications',
                  { count: importResult.notificationChannelsImported },
                  importResult.notificationChannelsImported,
                )
              }}
            </p>
          </UiAlertDescription>
        </UiAlert>

        <!-- Unmatched rules warning -->
        <UiAlert v-if="importResult && importResult.rulesUnmatched > 0" variant="destructive">
          <component :is="AlertTriangleIcon" class="w-4 h-4" />
          <UiAlertDescription>
            {{
              $t(
                'settings.importResultRulesUnmatched',
                { count: importResult.rulesUnmatched },
                importResult.rulesUnmatched,
              )
            }}
          </UiAlertDescription>
        </UiAlert>
      </UiCardContent>
    </UiCard>

    <!-- Resolution Dialog -->
    <SettingsImportResolution
      v-model:open="showResolutionDialog"
      :resolutions="previewResolutions"
      @confirm="onResolutionConfirmed"
    />
  </div>
</template>

<script setup lang="ts">
import { DownloadIcon, UploadIcon, AlertTriangleIcon, CheckCircleIcon } from 'lucide-vue-next';
import type {
  SettingsExportEnvelope,
  ExportSections,
  ImportSections,
  ImportResult,
  ImportPreview,
  RuleResolution,
  RuleOverride,
} from '~/types/api';

const { cardEntrance } = useMotionPresets();
const api = useApi();
const { addToast } = useToast();
const { t } = useI18n();
const route = useRoute();

// ─── Export State ────────────────────────────────────────────────────────────

const exporting = ref(false);

// Pre-select sections based on query param (e.g., ?section=rules from Custom Rules page)
const preselectedSection = route.query.section as string | undefined;

const exportSections = reactive<ExportSections>({
  preferences: preselectedSection ? preselectedSection === 'preferences' : true,
  rules: preselectedSection ? preselectedSection === 'rules' : true,
  integrations: preselectedSection ? preselectedSection === 'integrations' : true,
  diskGroups: preselectedSection ? preselectedSection === 'diskGroups' : true,
  notificationChannels: preselectedSection ? preselectedSection === 'notifications' : true,
});

const anyExportSelected = computed(
  () =>
    exportSections.preferences ||
    exportSections.rules ||
    exportSections.integrations ||
    exportSections.diskGroups ||
    exportSections.notificationChannels,
);

async function doExport() {
  exporting.value = true;
  try {
    const sectionParams: string[] = [];
    if (exportSections.preferences) sectionParams.push('preferences');
    if (exportSections.rules) sectionParams.push('rules');
    if (exportSections.integrations) sectionParams.push('integrations');
    if (exportSections.diskGroups) sectionParams.push('diskGroups');
    if (exportSections.notificationChannels) sectionParams.push('notifications');

    const query = sectionParams.join(',');
    const data = (await api(`/api/v1/settings/export?sections=${query}`)) as SettingsExportEnvelope;

    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const date = new Date().toISOString().slice(0, 10);
    const a = document.createElement('a');
    a.href = url;
    a.download = `capacitarr-settings-${date}.json`;
    a.click();
    URL.revokeObjectURL(url);

    addToast(t('settings.exportSuccess'), 'success');
  } catch {
    addToast(t('settings.exportError'), 'error');
  } finally {
    exporting.value = false;
  }
}

// ─── Import State ────────────────────────────────────────────────────────────

const fileInputRef = ref<HTMLInputElement | null>(null);
const parsedPayload = ref<SettingsExportEnvelope | null>(null);
const importing = ref(false);
const isDragOver = ref(false);
const importResult = ref<ImportResult | null>(null);
const importMode = ref<'append' | 'replace'>('append');

const importSections = reactive<ImportSections>({
  preferences: false,
  rules: false,
  integrations: false,
  diskGroups: false,
  notificationChannels: false,
});

const anyImportSelected = computed(
  () =>
    importSections.preferences ||
    importSections.rules ||
    importSections.integrations ||
    importSections.diskGroups ||
    importSections.notificationChannels,
);

// ─── Resolution Dialog State ─────────────────────────────────────────────────

const showResolutionDialog = ref(false);
const previewResolutions = ref<RuleResolution[]>([]);
const pendingPayload = ref<SettingsExportEnvelope | null>(null);

function onFileSelected(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  parseFile(file);
  input.value = '';
}

function onFileDrop(event: DragEvent) {
  isDragOver.value = false;
  const file = event.dataTransfer?.files?.[0];
  if (!file) return;
  parseFile(file);
}

function parseFile(file: File) {
  importResult.value = null;

  const reader = new FileReader();
  reader.onload = () => {
    try {
      const data = JSON.parse(reader.result as string) as SettingsExportEnvelope;

      if (!data || typeof data.version !== 'number') {
        addToast(t('settings.importInvalidFile'), 'error');
        return;
      }
      if (data.version !== 1) {
        addToast(t('settings.importInvalidVersion'), 'error');
        return;
      }

      parsedPayload.value = data;

      // Pre-check sections based on what's in the file
      importSections.preferences = !!data.preferences;
      importSections.rules = (data.rules?.length ?? 0) > 0;
      importSections.integrations = (data.integrations?.length ?? 0) > 0;
      importSections.diskGroups = (data.diskGroups?.length ?? 0) > 0;
      importSections.notificationChannels = (data.notificationChannels?.length ?? 0) > 0;
    } catch {
      addToast(t('settings.importInvalidFile'), 'error');
    }
  };
  reader.readAsText(file);
}

function clearImport() {
  parsedPayload.value = null;
  importResult.value = null;
  importMode.value = 'append';
}

async function doImport() {
  if (!parsedPayload.value) return;

  // If rules are being imported, run preview first to check for unmatched rules
  if (importSections.rules && (parsedPayload.value.rules?.length ?? 0) > 0) {
    importing.value = true;
    try {
      const preview = (await api('/api/v1/settings/import/preview', {
        method: 'POST',
        body: {
          payload: parsedPayload.value,
          sections: { ...importSections, mode: importMode.value },
        },
      })) as ImportPreview;

      const hasUnresolved = preview.rules.some(
        (r) => r.resolution === 'unmatched' || r.resolution === 'type_fallback',
      );

      if (hasUnresolved) {
        // Show resolution dialog
        previewResolutions.value = preview.rules;
        pendingPayload.value = parsedPayload.value;
        showResolutionDialog.value = true;
        importing.value = false;
        return;
      }

      // All rules matched — proceed with direct import (no overrides needed)
    } catch {
      addToast(t('settings.importError'), 'error');
      importing.value = false;
      return;
    }
  }

  // Direct import (no resolution needed)
  await executeImport();
}

async function executeImport(overrides?: RuleOverride[]) {
  if (!parsedPayload.value) return;
  importing.value = true;
  try {
    const endpoint = overrides ? '/api/v1/settings/import/commit' : '/api/v1/settings/import';
    const body: Record<string, unknown> = {
      payload: parsedPayload.value,
      sections: { ...importSections, mode: importMode.value },
    };
    if (overrides) {
      body.overrides = overrides;
    }

    const result = (await api(endpoint, {
      method: 'POST',
      body,
    })) as ImportResult;

    importResult.value = result;
    parsedPayload.value = null;
    addToast(t('settings.importSuccess'), 'success');
  } catch {
    addToast(t('settings.importError'), 'error');
  } finally {
    importing.value = false;
  }
}

function onResolutionConfirmed(overrides: RuleOverride[]) {
  if (pendingPayload.value) {
    parsedPayload.value = pendingPayload.value;
  }
  executeImport(overrides);
}
</script>
