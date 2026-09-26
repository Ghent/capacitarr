<template>
  <div class="space-y-8">
    <!-- Export Section -->
    <UiCard v-motion v-bind="cardEntrance" class="overflow-hidden">
      <UiCardHeader class="border-b border-border">
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-lg bg-blue-500 flex items-center justify-center">
            <component :is="DownloadIcon" class="w-5 h-5 text-white" />
          </div>
          <div>
            <UiCardTitle class="text-base">{{ $t('settings.export') }}</UiCardTitle>
            <UiCardDescription>{{ $t('settings.exportDesc') }}</UiCardDescription>
          </div>
        </div>
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
    <UiCard v-motion v-bind="cardEntrance" class="overflow-hidden">
      <UiCardHeader class="border-b border-border">
        <div class="flex items-center gap-3">
          <div class="w-10 h-10 rounded-lg bg-green-500 flex items-center justify-center">
            <component :is="UploadIcon" class="w-5 h-5 text-white" />
          </div>
          <div>
            <UiCardTitle class="text-base">{{ $t('settings.import') }}</UiCardTitle>
            <UiCardDescription>{{ $t('settings.importDesc') }}</UiCardDescription>
          </div>
        </div>
      </UiCardHeader>
      <UiCardContent class="space-y-4">
        <!-- Step indicator -->
        <div v-if="importStep > 0" class="flex items-center gap-2 text-xs text-muted-foreground">
          <template v-for="s in totalSteps" :key="s">
            <div
              class="flex items-center justify-center rounded-full w-6 h-6 text-xs font-medium border"
              :class="{
                'bg-primary text-primary-foreground border-primary': s === importStep,
                'bg-muted text-muted-foreground border-border': s !== importStep && s > importStep,
                'bg-primary/20 text-primary border-primary/40': s < importStep,
              }"
            >
              {{ s }}
            </div>
            <div
              v-if="s < totalSteps"
              class="flex-1 h-px"
              :class="s < importStep ? 'bg-primary/40' : 'bg-border'"
            />
          </template>
        </div>

        <!-- Step 1: Upload -->
        <template v-if="importStep === 0 || importStep === 1">
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

          <UiCard v-if="parsedPayload" class="bg-muted/30">
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

          <div v-if="parsedPayload" class="flex items-center gap-3 pt-2">
            <UiButton @click="importStep = 2">
              {{ $t('common.next') }}
            </UiButton>
            <UiButton variant="outline" @click="clearImport">
              {{ $t('common.cancel') }}
            </UiButton>
          </div>
        </template>

        <!-- Step 2: Select Sections & Mode -->
        <template v-if="importStep === 2 && parsedPayload">
          <UiLabel class="text-sm font-medium">
            {{ $t('settings.sectionsToImport') }}
          </UiLabel>
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

          <div class="space-y-2">
            <UiLabel class="text-sm font-medium">
              {{ $t('settings.importModeLabel') }}
            </UiLabel>
            <UiRadioGroup v-model="importMode" class="space-y-2">
              <div class="flex items-start gap-3">
                <UiRadioGroupItem id="mode-merge" value="merge" />
                <div class="grid gap-0.5 leading-none">
                  <UiLabel for="mode-merge" class="cursor-pointer">
                    {{ $t('settings.importModeMerge') }}
                  </UiLabel>
                  <p class="text-xs text-muted-foreground">
                    {{ $t('settings.importModeMergeDesc') }}
                  </p>
                </div>
              </div>
              <div class="flex items-start gap-3">
                <UiRadioGroupItem id="mode-sync" value="sync" />
                <div class="grid gap-0.5 leading-none">
                  <UiLabel for="mode-sync" class="cursor-pointer text-destructive">
                    {{ $t('settings.importModeSync') }}
                  </UiLabel>
                  <p class="text-xs text-muted-foreground">
                    {{ $t('settings.importModeSyncDesc') }}
                  </p>
                </div>
              </div>
            </UiRadioGroup>
          </div>

          <UiAlert v-if="importMode === 'sync'" variant="destructive">
            <component :is="AlertTriangleIcon" class="w-4 h-4" />
            <UiAlertTitle>{{ $t('common.warning') }}</UiAlertTitle>
            <UiAlertDescription>
              {{ $t('settings.importModeSyncConfirm') }}
            </UiAlertDescription>
          </UiAlert>

          <UiAlert variant="default">
            <component :is="AlertTriangleIcon" class="w-4 h-4" />
            <UiAlertTitle>{{ $t('common.warning') }}</UiAlertTitle>
            <UiAlertDescription>
              {{ $t('settings.importCredentialWarning') }}
            </UiAlertDescription>
          </UiAlert>

          <div class="flex items-center gap-3 pt-2">
            <UiButton @click="importStep = 1">
              {{ $t('common.back') }}
            </UiButton>
            <UiButton :disabled="!anyImportSelected || loadingPreview" @click="loadPreview">
              {{ loadingPreview ? $t('common.loading') : $t('settings.importReviewChanges') }}
            </UiButton>
            <UiButton variant="outline" @click="clearImport">
              {{ $t('common.cancel') }}
            </UiButton>
          </div>
        </template>

        <!-- Step 3: Review Changes (Diff) -->
        <template v-if="importStep === 3 && importPreviewData">
          <SettingsImportDiff
            :preview="importPreviewData"
            @update:overrides="ruleOverrides = $event"
          />

          <div class="flex items-center gap-3 pt-2">
            <UiButton @click="importStep = 2">
              {{ $t('common.back') }}
            </UiButton>
            <UiButton :disabled="importing" @click="doConfirmImport">
              <component :is="UploadIcon" class="w-4 h-4" />
              {{ importing ? $t('common.loading') : $t('settings.importConfirmButton') }}
            </UiButton>
            <UiButton variant="outline" @click="clearImport">
              {{ $t('common.cancel') }}
            </UiButton>
          </div>
        </template>

        <!-- Step 4: Results -->
        <template v-if="importStep === 4 && importResult">
          <UiAlert variant="default">
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

          <UiAlert v-if="importResult.itemsDeleted > 0" variant="default">
            <component :is="AlertTriangleIcon" class="w-4 h-4" />
            <UiAlertDescription>
              {{
                $t(
                  'settings.importResultItemsDeleted',
                  { count: importResult.itemsDeleted },
                  importResult.itemsDeleted,
                )
              }}
            </UiAlertDescription>
          </UiAlert>

          <UiAlert v-if="importResult.rulesUnmatched > 0" variant="destructive">
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

          <div
            v-if="importResult.preImportSnapshot && !autoDownloadedSnapshot"
            class="flex items-center gap-2"
          >
            <UiButton
              variant="outline"
              size="sm"
              @click="downloadSnapshot(importResult.preImportSnapshot!)"
            >
              <component :is="DownloadIcon" class="w-4 h-4" />
              {{ $t('settings.importDownloadBackup') }}
            </UiButton>
            <span class="text-xs text-muted-foreground">
              {{ $t('settings.importDownloadBackupHint') }}
            </span>
          </div>

          <UiButton variant="outline" @click="clearImport">
            {{ $t('common.done') }}
          </UiButton>
        </template>
      </UiCardContent>
    </UiCard>
  </div>
</template>

<script setup lang="ts">
import { DownloadIcon, UploadIcon, AlertTriangleIcon, CheckCircleIcon } from 'lucide-vue-next';
import SettingsImportDiff from '~/components/settings/SettingsImportDiff.vue';
import type {
  SettingsExportEnvelope,
  ExportSections,
  ImportSections,
  ImportResult,
  ImportPreview,
  RuleOverride,
} from '~/types/api';
import { toast } from 'vue-sonner';

const { cardEntrance } = useMotionPresets();
const api = useApi();
const { t } = useI18n();
const route = useRoute();

// ─── Export State ────────────────────────────────────────────────────────────

const exporting = ref(false);

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
    const data = await api.GET('/settings/export', { query: { sections: query } });

    const blob = new Blob([JSON.stringify(data, null, 2)], {
      type: 'application/json',
    });
    const url = URL.createObjectURL(blob);
    const date = new Date().toISOString().slice(0, 10);
    const a = document.createElement('a');
    a.href = url;
    a.download = `capacitarr-settings-${date}.json`;
    a.click();
    URL.revokeObjectURL(url);

    toast.success(t('settings.exportSuccess'));
  } catch {
    toast.error(t('settings.exportError'));
  } finally {
    exporting.value = false;
  }
}

// ─── Import State ────────────────────────────────────────────────────────────

const totalSteps = 4;
const importStep = ref(0); // 0=hidden, 1=upload, 2=sections, 3=review, 4=results
const fileInputRef = ref<HTMLInputElement | null>(null);
const parsedPayload = ref<SettingsExportEnvelope | null>(null);
const importing = ref(false);
const loadingPreview = ref(false);
const isDragOver = ref(false);
const importResult = ref<ImportResult | null>(null);
const importMode = ref<'merge' | 'sync'>('merge');
const autoDownloadedSnapshot = ref(false);
const importPreviewData = ref<ImportPreview | null>(null);
const ruleOverrides = ref<RuleOverride[]>([]);

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
  importPreviewData.value = null;

  const reader = new FileReader();
  reader.onload = () => {
    try {
      const data = JSON.parse(reader.result as string) as SettingsExportEnvelope;

      if (!data || typeof data.version !== 'number') {
        toast.error(t('settings.importInvalidFile'));
        return;
      }
      if (data.version !== 1) {
        toast.error(t('settings.importInvalidVersion'));
        return;
      }

      parsedPayload.value = data;
      importStep.value = 1;

      // Pre-check sections
      importSections.preferences = !!data.preferences;
      importSections.rules = (data.rules?.length ?? 0) > 0;
      importSections.integrations = (data.integrations?.length ?? 0) > 0;
      importSections.diskGroups = (data.diskGroups?.length ?? 0) > 0;
      importSections.notificationChannels = (data.notificationChannels?.length ?? 0) > 0;
    } catch {
      toast.error(t('settings.importInvalidFile'));
    }
  };
  reader.readAsText(file);
}

function clearImport() {
  parsedPayload.value = null;
  importResult.value = null;
  importPreviewData.value = null;
  ruleOverrides.value = [];
  importMode.value = 'merge';
  autoDownloadedSnapshot.value = false;
  importStep.value = 0;
}

function downloadSnapshot(snapshot: SettingsExportEnvelope) {
  const blob = new Blob([JSON.stringify(snapshot, null, 2)], {
    type: 'application/json',
  });
  const url = URL.createObjectURL(blob);
  const date = new Date().toISOString().slice(0, 10);
  const a = document.createElement('a');
  a.href = url;
  a.download = `capacitarr-pre-import-backup-${date}.json`;
  a.click();
  URL.revokeObjectURL(url);
}

async function loadPreview() {
  if (!parsedPayload.value) return;
  loadingPreview.value = true;
  try {
    const preview = await api.POST('/settings/import/preview', {
      body: {
        payload: parsedPayload.value,
        sections: { ...importSections, mode: importMode.value },
      },
    });

    importPreviewData.value = preview;
    ruleOverrides.value = [];
    importStep.value = 3;
  } catch {
    toast.error(t('settings.importError'));
  } finally {
    loadingPreview.value = false;
  }
}

async function doConfirmImport() {
  if (!parsedPayload.value) return;
  importing.value = true;
  try {
    const hasOverrides = ruleOverrides.value.length > 0;
    const sections = { ...importSections, mode: importMode.value };
    const result = hasOverrides
      ? await api.POST('/settings/import/commit', {
          body: {
            payload: parsedPayload.value,
            sections,
            overrides: ruleOverrides.value,
          },
        })
      : await api.POST('/settings/import', {
          body: {
            payload: parsedPayload.value,
            sections,
          },
        });

    importResult.value = result;
    parsedPayload.value = null;
    importStep.value = 4;

    // Auto-download safety backup in sync mode
    if (importMode.value === 'sync' && result.preImportSnapshot) {
      downloadSnapshot(result.preImportSnapshot);
      autoDownloadedSnapshot.value = true;
      toast.info(t('settings.importSnapshotDownloaded'));
    }

    toast.success(t('settings.importSuccess'));
  } catch {
    toast.error(t('settings.importError'));
  } finally {
    importing.value = false;
  }
}
</script>
