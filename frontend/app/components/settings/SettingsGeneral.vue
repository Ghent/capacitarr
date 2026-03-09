<template>
  <!-- Display Preferences Section -->
  <UiCard
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{
      opacity: 1,
      y: 0,
      transition: { type: 'spring', stiffness: 260, damping: 24, delay: 200 },
    }"
    class="overflow-hidden"
  >
    <UiCardHeader class="border-b border-border">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg bg-purple-500 flex items-center justify-center">
          <component :is="MonitorIcon" class="w-5 h-5 text-white" />
        </div>
        <div>
          <UiCardTitle class="text-base">
            {{ $t('settings.display') }}
          </UiCardTitle>
          <UiCardDescription>{{ $t('settings.displayDesc') }}</UiCardDescription>
        </div>
      </div>
    </UiCardHeader>
    <UiCardContent class="pt-5 space-y-5">
      <div class="grid grid-cols-1 sm:grid-cols-3 gap-5">
        <!-- Timezone -->
        <div class="space-y-1.5">
          <UiLabel>{{ $t('settings.timezone') }}</UiLabel>
          <div class="flex gap-1">
            <UiButton
              :variant="displayTimezone === 'local' ? 'default' : 'outline'"
              size="sm"
              class="flex-1"
              @click="setTimezone('local')"
            >
              {{ $t('settings.timezoneLocal') }}
            </UiButton>
            <UiButton
              :variant="displayTimezone === 'UTC' ? 'default' : 'outline'"
              size="sm"
              class="flex-1"
              @click="setTimezone('UTC')"
            >
              {{ $t('settings.timezoneUTC') }}
            </UiButton>
          </div>
        </div>

        <!-- Clock Format -->
        <div class="space-y-1.5">
          <UiLabel>{{ $t('settings.clockFormat') }}</UiLabel>
          <div class="flex gap-1">
            <UiButton
              :variant="displayClockFormat === '12h' ? 'default' : 'outline'"
              size="sm"
              class="flex-1"
              @click="setClockFormat('12h')"
            >
              {{ $t('settings.clock12h') }}
            </UiButton>
            <UiButton
              :variant="displayClockFormat === '24h' ? 'default' : 'outline'"
              size="sm"
              class="flex-1"
              @click="setClockFormat('24h')"
            >
              {{ $t('settings.clock24h') }}
            </UiButton>
          </div>
        </div>

        <!-- Date Style -->
        <div class="space-y-1.5">
          <UiLabel>{{ $t('settings.exactDates') }}</UiLabel>
          <div class="flex gap-1">
            <UiButton
              :variant="!showExactDates ? 'default' : 'outline'"
              size="sm"
              class="flex-1"
              @click="setShowExactDates(false)"
            >
              {{ $t('settings.dateRelative') }}
            </UiButton>
            <UiButton
              :variant="showExactDates ? 'default' : 'outline'"
              size="sm"
              class="flex-1"
              @click="setShowExactDates(true)"
            >
              {{ $t('settings.dateExact') }}
            </UiButton>
          </div>
        </div>
      </div>
    </UiCardContent>
  </UiCard>

  <!-- Engine Behavior Section -->
  <UiCard
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{
      opacity: 1,
      y: 0,
      transition: { type: 'spring', stiffness: 260, damping: 24, delay: 300 },
    }"
    class="overflow-hidden"
  >
    <UiCardHeader class="border-b border-border">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg bg-primary flex items-center justify-center">
          <component :is="CogIcon" class="w-5 h-5 text-white" />
        </div>
        <div>
          <UiCardTitle class="text-base">
            {{ $t('settings.engineBehavior') }}
          </UiCardTitle>
          <UiCardDescription>{{ $t('settings.engineBehaviorDesc') }}</UiCardDescription>
        </div>
      </div>
    </UiCardHeader>
    <UiCardContent class="pt-5 space-y-6">
      <!-- Execution Mode -->
      <div class="space-y-3">
        <div class="flex items-center gap-2">
          <UiLabel>{{ $t('settings.executionMode') }}</UiLabel>
          <SaveIndicator :status="saveStatus.executionMode ?? 'idle'" />
        </div>
        <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <button
            v-for="mode in executionModes"
            :key="mode.value"
            data-slot="execution-mode-card"
            :data-active="engineExecutionMode === mode.value"
            class="px-4 py-3 rounded-xl border-2 text-left transition-all"
            :class="
              engineExecutionMode === mode.value
                ? 'border-primary bg-primary/5 shadow-sm ring-1 ring-primary/20'
                : 'border-border hover:border-border'
            "
            @click="setExecutionMode(mode.value)"
          >
            <div
              class="text-sm font-medium"
              :class="engineExecutionMode === mode.value ? 'text-primary' : ''"
            >
              {{ mode.label }}
            </div>
            <div class="text-xs text-muted-foreground mt-0.5">
              {{ mode.description }}
            </div>
          </button>
        </div>
      </div>

      <!-- Score Tiebreaker + Snooze Duration — side by side -->
      <div class="grid grid-cols-1 sm:grid-cols-2 gap-5">
        <!-- Score Tiebreaker -->
        <div class="space-y-1.5">
          <div class="flex items-center gap-2">
            <UiLabel>{{ $t('settings.scoreTiebreaker') }}</UiLabel>
            <SaveIndicator :status="saveStatus.tiebreaker ?? 'idle'" />
          </div>
          <p class="text-xs text-muted-foreground mb-1">
            When items have the same score, how should they be ordered?
          </p>
          <UiSelect v-model="engineTiebreakerMethod">
            <UiSelectTrigger class="w-full">
              <UiSelectValue placeholder="Select tiebreaker" />
            </UiSelectTrigger>
            <UiSelectContent>
              <UiSelectItem value="size_desc"> Largest first (free more space) </UiSelectItem>
              <UiSelectItem value="size_asc"> Smallest first </UiSelectItem>
              <UiSelectItem value="name_asc"> Alphabetical (A → Z) </UiSelectItem>
              <UiSelectItem value="oldest_first"> Oldest in library first </UiSelectItem>
              <UiSelectItem value="newest_first"> Newest in library first </UiSelectItem>
            </UiSelectContent>
          </UiSelect>
        </div>

        <!-- Snooze Duration -->
        <div class="space-y-1.5">
          <div class="flex items-center gap-2">
            <UiLabel>{{ $t('settings.snoozeDurationHours') }}</UiLabel>
            <SaveIndicator :status="saveStatus.snoozeDuration ?? 'idle'" />
          </div>
          <p class="text-xs text-muted-foreground mb-1">
            {{ $t('settings.snoozeDurationDesc') }}
          </p>
          <UiSelect
            :model-value="String(snoozeDurationHours)"
            @update:model-value="
              (v: AcceptableValue) => {
                snoozeDurationHours = Number(v);
                autoSavePreference('snoozeDuration', 'snoozeDurationHours', Number(v));
              }
            "
          >
            <UiSelectTrigger class="w-full">
              <UiSelectValue placeholder="Select duration" />
            </UiSelectTrigger>
            <UiSelectContent>
              <UiSelectItem value="1"> 1 hour </UiSelectItem>
              <UiSelectItem value="6"> 6 hours </UiSelectItem>
              <UiSelectItem value="12"> 12 hours </UiSelectItem>
              <UiSelectItem value="24"> 24 hours (1 day) </UiSelectItem>
              <UiSelectItem value="48"> 48 hours (2 days) </UiSelectItem>
              <UiSelectItem value="72"> 72 hours (3 days) </UiSelectItem>
              <UiSelectItem value="168"> 1 week </UiSelectItem>
              <UiSelectItem value="336"> 2 weeks </UiSelectItem>
              <UiSelectItem value="720"> 30 days </UiSelectItem>
            </UiSelectContent>
          </UiSelect>
        </div>
      </div>
    </UiCardContent>
  </UiCard>
</template>

<script setup lang="ts">
import { MonitorIcon, CogIcon } from 'lucide-vue-next';
import type { PreferenceSet } from '~/types/api';
import type { AcceptableValue } from 'reka-ui';
import SaveIndicator from '~/components/settings/SaveIndicator.vue';

const api = useApi();
const {
  timezone: displayTimezone,
  clockFormat: displayClockFormat,
  showExactDates,
  setTimezone,
  setClockFormat,
  setShowExactDates,
} = useDisplayPrefs();
const { saveStatus, initFields, autoSavePreference } = useAutoSave();

initFields(['executionMode', 'tiebreaker', 'snoozeDuration']);

// Engine behavior state
const engineExecutionMode = ref('dry-run');
const engineTiebreakerMethod = ref('size_desc');
const snoozeDurationHours = ref(24);

const executionModes = [
  { value: 'dry-run', label: 'Dry Run', description: 'Log only, no deletions' },
  { value: 'approval', label: 'Approval', description: 'Queue for manual approval' },
  { value: 'auto', label: 'Automatic', description: 'Delete automatically' },
];

function setExecutionMode(mode: string) {
  engineExecutionMode.value = mode;
  autoSavePreference('executionMode', 'executionMode', mode);
}

// Watch tiebreaker — immediate save on select change
watch(engineTiebreakerMethod, (newVal, oldVal) => {
  if (oldVal !== undefined && newVal !== oldVal) {
    autoSavePreference('tiebreaker', 'tiebreakerMethod', newVal);
  }
});

// ─── Fetch preferences on mount ──────────────────────────────────────────────
async function fetchPreferences() {
  try {
    const prefs = (await api('/api/v1/preferences')) as PreferenceSet;
    if (prefs?.executionMode) {
      engineExecutionMode.value = prefs.executionMode;
    }
    if (prefs?.tiebreakerMethod) {
      engineTiebreakerMethod.value = prefs.tiebreakerMethod;
    }
    if (prefs?.snoozeDurationHours !== undefined) {
      snoozeDurationHours.value = prefs.snoozeDurationHours;
    }
  } catch (err) {
    console.warn('[SettingsGeneral] fetchPreferences failed:', err);
  }
}

onMounted(() => {
  fetchPreferences();
});
</script>
