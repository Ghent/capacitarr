<template>
  <!-- Poll Interval -->
  <UiCard
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{ opacity: 1, y: 0 }"
    class="overflow-hidden"
  >
    <UiCardHeader class="border-b border-border">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg bg-blue-500 flex items-center justify-center">
          <component :is="TimerIcon" class="w-5 h-5 text-white" />
        </div>
        <div>
          <UiCardTitle class="text-base">
            {{ $t('settings.pollInterval') }}
          </UiCardTitle>
          <UiCardDescription>{{ $t('settings.pollIntervalDesc') }}</UiCardDescription>
        </div>
      </div>
    </UiCardHeader>
    <UiCardContent class="pt-5">
      <div class="space-y-1.5">
        <div class="flex items-center gap-2">
          <UiLabel>{{ $t('settings.interval') }}</UiLabel>
          <SaveIndicator :status="saveStatus.pollInterval" />
        </div>
        <UiSelect v-model="pollIntervalStr">
          <UiSelectTrigger class="w-full max-w-xs">
            <UiSelectValue placeholder="Select interval" />
          </UiSelectTrigger>
          <UiSelectContent>
            <UiSelectItem value="30"> 30 seconds </UiSelectItem>
            <UiSelectItem value="60"> 1 minute </UiSelectItem>
            <UiSelectItem value="300"> 5 minutes (default) </UiSelectItem>
            <UiSelectItem value="900"> 15 minutes </UiSelectItem>
            <UiSelectItem value="1800"> 30 minutes </UiSelectItem>
            <UiSelectItem value="3600"> 1 hour </UiSelectItem>
          </UiSelectContent>
        </UiSelect>
        <p class="text-xs text-muted-foreground/70">
          {{ $t('settings.pollIntervalHint') }}
        </p>
      </div>
    </UiCardContent>
  </UiCard>

  <!-- Log Level -->
  <UiCard
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{ opacity: 1, y: 0, transition: { delay: 50 } }"
    class="overflow-hidden"
  >
    <UiCardHeader class="border-b border-border">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg bg-emerald-600 flex items-center justify-center">
          <component :is="TerminalIcon" class="w-5 h-5 text-white" />
        </div>
        <div>
          <UiCardTitle class="text-base">
            {{ $t('settings.logLevel') }}
          </UiCardTitle>
          <UiCardDescription>{{ $t('settings.logLevelDesc') }}</UiCardDescription>
        </div>
      </div>
    </UiCardHeader>
    <UiCardContent class="pt-5">
      <div class="space-y-1.5">
        <div class="flex items-center gap-2">
          <UiLabel>{{ $t('settings.logLevel') }}</UiLabel>
          <SaveIndicator :status="saveStatus.logLevel" />
        </div>
        <UiSelect v-model="logLevel">
          <UiSelectTrigger class="w-full max-w-xs">
            <UiSelectValue placeholder="Select log level" />
          </UiSelectTrigger>
          <UiSelectContent>
            <UiSelectItem value="debug"> Debug </UiSelectItem>
            <UiSelectItem value="info"> Info (default) </UiSelectItem>
            <UiSelectItem value="warn"> Warn </UiSelectItem>
            <UiSelectItem value="error"> Error </UiSelectItem>
          </UiSelectContent>
        </UiSelect>
        <p class="text-xs text-muted-foreground/70">
          {{ $t('settings.logLevelHint') }}
        </p>
      </div>
    </UiCardContent>
  </UiCard>

  <!-- Data Management -->
  <UiCard
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{ opacity: 1, y: 0, transition: { delay: 150 } }"
    class="overflow-hidden"
  >
    <UiCardHeader class="border-b border-border">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg bg-primary flex items-center justify-center">
          <component :is="DatabaseIcon" class="w-5 h-5 text-white" />
        </div>
        <div>
          <UiCardTitle class="text-base">
            {{ $t('settings.dataManagement') }}
          </UiCardTitle>
          <UiCardDescription>{{ $t('settings.dataManagementDesc') }}</UiCardDescription>
        </div>
      </div>
    </UiCardHeader>
    <UiCardContent class="pt-5 space-y-4">
      <div class="space-y-1.5">
        <div class="flex items-center gap-2">
          <UiLabel>{{ $t('settings.auditRetention') }}</UiLabel>
          <SaveIndicator :status="saveStatus.retention" />
        </div>
        <UiSelect v-model="retentionStr">
          <UiSelectTrigger class="w-full max-w-xs">
            <UiSelectValue placeholder="Select retention" />
          </UiSelectTrigger>
          <UiSelectContent>
            <UiSelectItem value="7"> 7 days </UiSelectItem>
            <UiSelectItem value="14"> 14 days </UiSelectItem>
            <UiSelectItem value="30"> 30 days (default) </UiSelectItem>
            <UiSelectItem value="60"> 60 days </UiSelectItem>
            <UiSelectItem value="90"> 90 days </UiSelectItem>
            <UiSelectItem value="180"> 180 days </UiSelectItem>
            <UiSelectItem value="365"> 365 days </UiSelectItem>
            <UiSelectItem value="0"> Indefinite </UiSelectItem>
          </UiSelectContent>
        </UiSelect>
      </div>
      <UiAlert v-if="retentionDays === 0" variant="destructive">
        <UiAlertTitle>{{ $t('common.warning') }}</UiAlertTitle>
        <UiAlertDescription>
          {{ $t('settings.retentionWarning') }}
        </UiAlertDescription>
      </UiAlert>
    </UiCardContent>
  </UiCard>

  <!-- Default Disk Group Thresholds -->
  <UiCard
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{ opacity: 1, y: 0, transition: { delay: 200 } }"
    class="overflow-hidden"
  >
    <UiCardHeader class="border-b border-border">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg bg-amber-500 flex items-center justify-center">
          <component :is="HardDriveIcon" class="w-5 h-5 text-white" />
        </div>
        <div>
          <UiCardTitle class="text-base"> Default Disk Group Thresholds </UiCardTitle>
          <UiCardDescription>Applied when new disk groups are discovered</UiCardDescription>
        </div>
      </div>
    </UiCardHeader>
    <UiCardContent class="pt-5 space-y-4">
      <div class="grid grid-cols-2 gap-4 max-w-sm">
        <div class="space-y-1.5">
          <div class="flex items-center gap-2">
            <UiLabel>Threshold %</UiLabel>
            <SaveIndicator :status="saveStatus.defaultThreshold" />
          </div>
          <UiInput
            v-model.number="defaultThreshold"
            type="number"
            min="50"
            max="99"
            @change="
              autoSavePreference('defaultThreshold', 'defaultThresholdPct', defaultThreshold)
            "
          />
        </div>
        <div class="space-y-1.5">
          <div class="flex items-center gap-2">
            <UiLabel>Target %</UiLabel>
            <SaveIndicator :status="saveStatus.defaultTarget" />
          </div>
          <UiInput
            v-model.number="defaultTarget"
            type="number"
            min="50"
            max="98"
            @change="autoSavePreference('defaultTarget', 'defaultTargetPct', defaultTarget)"
          />
        </div>
      </div>
      <p class="text-xs text-muted-foreground/70">
        Threshold triggers cleanup. Target is the desired usage level after cleanup. Threshold must
        be greater than target.
      </p>
    </UiCardContent>
  </UiCard>

  <!-- Danger Zone -->
  <UiCard
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{ opacity: 1, y: 0, transition: { delay: 300 } }"
    class="overflow-hidden border-destructive/50"
  >
    <UiCardHeader class="border-b border-destructive/30">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg bg-destructive flex items-center justify-center">
          <component :is="AlertTriangleIcon" class="w-5 h-5 text-white" />
        </div>
        <div>
          <UiCardTitle class="text-base text-destructive"> Danger Zone </UiCardTitle>
          <UiCardDescription> Destructive actions that cannot be easily undone. </UiCardDescription>
        </div>
      </div>
    </UiCardHeader>
    <UiCardContent class="pt-5 space-y-6">
      <!-- Reset Scraped Data -->
      <div class="space-y-2">
        <p class="text-sm font-medium text-foreground">Reset Scraped Data</p>
        <p class="text-sm text-muted-foreground">
          Clear all audit logs, capacity history, and engine stats. Disk group thresholds and targets
          will be preserved. Integration configurations, preferences, and custom rules are preserved.
          Data will be re-populated on the next engine run.
        </p>
        <UiButton variant="destructive" :disabled="resettingData" @click="showResetDialog = true">
          {{ resettingData ? 'Clearing…' : 'Clear All Scraped Data' }}
        </UiButton>
      </div>

      <UiSeparator />

      <!-- Deletion Safety -->
      <div class="space-y-3">
        <p class="text-sm font-medium text-foreground">
          {{ $t('settings.deletionSafety') }}
        </p>
        <p class="text-sm text-muted-foreground">
          {{ $t('settings.deletionSafetyExplain') }}
        </p>
        <UiAlert v-if="deletionsEnabled" variant="destructive">
          <component :is="Trash2Icon" class="w-4 h-4" />
          <UiAlertTitle>{{ $t('settings.deletionsActiveAlert') }}</UiAlertTitle>
          <UiAlertDescription>
            {{ $t('settings.deletionsActiveAlertDesc') }}
          </UiAlertDescription>
        </UiAlert>
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <UiSwitch
              :model-value="deletionsEnabled"
              aria-label="Enable actual file deletion"
              :class="deletionsEnabled ? '[&[data-state=checked]]:bg-destructive' : ''"
              @update:model-value="(v: boolean) => onDeletionToggle(v)"
            />
            <div>
              <span class="text-sm font-medium">
                {{ $t('settings.enableDeletions') }}
              </span>
              <p v-if="deletionsEnabled" class="text-xs font-medium text-red-600 dark:text-red-400">
                Current status: Deletions are active!
              </p>
              <p v-else class="text-xs font-medium text-amber-600 dark:text-amber-400">
                Current status: All deletions are logged and simulated
              </p>
            </div>
          </div>
          <SaveIndicator :status="saveStatus.deletionsEnabled" />
        </div>
      </div>
    </UiCardContent>
  </UiCard>

  <!-- Data Reset Confirmation Dialog -->
  <UiDialog
    :open="showResetDialog"
    @update:open="
      (val: boolean) => {
        showResetDialog = val;
      }
    "
  >
    <UiDialogContent class="max-w-md">
      <UiDialogHeader>
        <UiDialogTitle>Are you sure?</UiDialogTitle>
        <UiDialogDescription>
          This will permanently delete all audit logs, capacity history, and engine statistics. Disk
          group thresholds and targets will be preserved. This action cannot be undone.
        </UiDialogDescription>
      </UiDialogHeader>
      <UiDialogFooter class="flex gap-2 justify-end">
        <UiButton variant="outline" @click="showResetDialog = false"> Cancel </UiButton>
        <UiButton variant="destructive" :disabled="resettingData" @click="confirmResetData">
          {{ resettingData ? 'Clearing…' : 'Yes, clear all data' }}
        </UiButton>
      </UiDialogFooter>
    </UiDialogContent>
  </UiDialog>

  <!-- Deletion Confirmation Dialog -->
  <UiDialog
    :open="showDeletionConfirmDialog"
    @update:open="
      (val: boolean) => {
        if (!val) cancelEnableDeletions();
      }
    "
  >
    <UiDialogContent class="max-w-md">
      <UiDialogHeader>
        <UiDialogTitle>Enable Actual Deletions?</UiDialogTitle>
        <UiDialogDescription>
          This will allow Capacitarr to permanently delete media files from your storage. Deleted
          files cannot be recovered. Make sure you have backups before proceeding.
        </UiDialogDescription>
      </UiDialogHeader>
      <div class="py-2">
        <UiAlert variant="destructive">
          <component :is="AlertTriangleIcon" class="w-4 h-4" />
          <UiAlertTitle>Warning</UiAlertTitle>
          <UiAlertDescription>
            Once enabled, any media flagged by the scoring engine will be permanently removed from
            disk. This action cannot be undone. Make sure your scoring rules and thresholds are
            configured correctly before enabling.
          </UiAlertDescription>
        </UiAlert>
      </div>
      <UiDialogFooter class="flex gap-2 justify-end">
        <UiButton variant="outline" @click="cancelEnableDeletions"> Cancel </UiButton>
        <UiButton variant="destructive" @click="confirmEnableDeletions">
          Enable Deletions
        </UiButton>
      </UiDialogFooter>
    </UiDialogContent>
  </UiDialog>
</template>

<script setup lang="ts">
import {
  TimerIcon,
  TerminalIcon,
  DatabaseIcon,
  HardDriveIcon,
  AlertTriangleIcon,
  Trash2Icon,
} from 'lucide-vue-next';
import type { PreferenceSet, ApiError } from '~/types/api';
import SaveIndicator from '~/components/settings/SaveIndicator.vue';

const api = useApi();
const { addToast } = useToast();
const { saveStatus, initFields, autoSavePreference } = useAutoSave();

initFields([
  'pollInterval',
  'retention',
  'defaultThreshold',
  'defaultTarget',
  'deletionsEnabled',
  'logLevel',
]);

// ─── Settings state ──────────────────────────────────────────────────────────
const retentionDays = ref(30);
const pollIntervalSeconds = ref(300);
const logLevel = ref('info');
const defaultThreshold = ref(85);
const defaultTarget = ref(75);
const deletionsEnabled = ref(true);
const showDeletionConfirmDialog = ref(false);
const showResetDialog = ref(false);
const resettingData = ref(false);

const pollIntervalStr = computed({
  get: () => String(pollIntervalSeconds.value),
  set: (val: string) => {
    pollIntervalSeconds.value = Number(val);
  },
});

const retentionStr = computed({
  get: () => String(retentionDays.value),
  set: (val: string) => {
    retentionDays.value = Number(val);
  },
});

// Watch poll interval
watch(pollIntervalSeconds, (newVal, oldVal) => {
  if (oldVal !== undefined && newVal !== oldVal) {
    autoSavePreference('pollInterval', 'pollIntervalSeconds', newVal);
  }
});

// Watch retention days
watch(retentionDays, (newVal, oldVal) => {
  if (oldVal !== undefined && newVal !== oldVal) {
    autoSavePreference('retention', 'auditLogRetentionDays', newVal);
  }
});

// Watch log level
watch(logLevel, (newVal, oldVal) => {
  if (oldVal !== undefined && newVal !== oldVal) {
    autoSavePreference('logLevel', 'logLevel', newVal);
  }
});

// ─── Deletion Safety Toggle ──────────────────────────────────────────────────
function onDeletionToggle(checked: boolean) {
  if (checked) {
    showDeletionConfirmDialog.value = true;
  } else {
    deletionsEnabled.value = false;
    autoSavePreference('deletionsEnabled', 'deletionsEnabled', false);
    addToast('File deletions disabled — all actions are now simulated', 'success');
  }
}

function confirmEnableDeletions() {
  deletionsEnabled.value = true;
  showDeletionConfirmDialog.value = false;
  autoSavePreference('deletionsEnabled', 'deletionsEnabled', true);
  addToast('File deletions enabled — flagged items will be permanently removed', 'error');
}

function cancelEnableDeletions() {
  showDeletionConfirmDialog.value = false;
}

// ─── Data Reset ──────────────────────────────────────────────────────────────
async function confirmResetData() {
  resettingData.value = true;
  try {
    await api('/api/v1/data/reset', { method: 'DELETE' });
    showResetDialog.value = false;
    addToast('All scraped data has been cleared', 'success');
  } catch (e: unknown) {
    addToast((e as ApiError)?.data?.error || 'Failed to clear data', 'error');
  } finally {
    resettingData.value = false;
  }
}

// ─── Fetch preferences on mount ──────────────────────────────────────────────
async function fetchPreferences() {
  try {
    const prefs = (await api('/api/v1/preferences')) as PreferenceSet;
    if (prefs?.auditLogRetentionDays !== undefined) {
      retentionDays.value = prefs.auditLogRetentionDays;
    }
    if (prefs?.pollIntervalSeconds !== undefined && prefs.pollIntervalSeconds >= 30) {
      pollIntervalSeconds.value = prefs.pollIntervalSeconds;
    }
    if (prefs?.deletionsEnabled !== undefined) {
      deletionsEnabled.value = prefs.deletionsEnabled;
    }
    if (prefs?.logLevel) {
      logLevel.value = prefs.logLevel;
    }
  } catch {
    // Silently ignored — UI has no further handling
  }
}

onMounted(() => {
  fetchPreferences();
});
</script>
