<template>
  <div>
    <PullToRefreshIndicator
      :pull-distance="pullDistance"
      :pull-progress="pullProgress"
      :is-refreshing="isRefreshing"
    />

    <div
      data-slot="page-header"
      class="mb-6 flex flex-col md:flex-row md:items-center justify-between gap-4"
    >
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          {{ $t('dashboard.title') }}
        </h1>
        <p class="text-muted-foreground mt-1.5">
          {{ $t('dashboard.subtitle') }}
          <i18n-t
            v-if="lastUpdated"
            keypath="dashboard.updated"
            tag="span"
            class="inline-flex items-center gap-1 ml-2 text-xs text-muted-foreground/70"
          >
            <RefreshCwIcon class="w-3 h-3" />
            <template #time>
              <DateDisplay :date="lastUpdated.toISOString()" />
            </template>
          </i18n-t>
        </p>
      </div>
      <div class="flex items-center gap-2">
        <UiSelect v-model="dateRange">
          <UiSelectTrigger class="h-9 w-[130px]">
            <UiSelectValue :placeholder="$t('dashboard.timeRange')" />
          </UiSelectTrigger>
          <UiSelectContent>
            <UiSelectItem v-for="opt in dateRangeOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </UiSelectItem>
          </UiSelectContent>
        </UiSelect>
      </div>
    </div>

    <FetchErrorBanner v-if="loadError" @retry="fetchDashboardData()" />

    <IntegrationErrorBanner :integrations="allIntegrations" />

    <DashboardEmptyState
      v-if="lastFetchOk && diskGroups.length === 0 && !loading"
      :integrations="allIntegrations"
    />

    <DashboardEngineActivityCard
      :recent-activity="recentActivity"
      :effective-mode="effectiveMode"
      :any-auto-mode="anyAutoMode"
      :all-dry-run="allDryRun"
    />

    <DeletionQueueCard :effective-mode="effectiveMode" />
    <SnoozedItemsCard />
    <SunsetQueueCard :has-sunset-mode="diskGroups.some((g) => g.mode === 'sunset')" />
    <ApprovalQueueCard v-if="approvalQueueVisible" />

    <div
      v-if="diskGroups.length > 0"
      class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 mb-6"
    >
      <DiskGroupSection
        v-for="group in diskGroups"
        :key="group.id"
        :group="group"
        :date-range="dateRange"
      />
    </div>

    <template v-if="loading">
      <SkeletonCard :show-chart="true" />
    </template>
  </div>
</template>

<script setup lang="ts">
import { RefreshCwIcon } from 'lucide-vue-next';
import type { ActivityEvent, DiskGroup, IntegrationConfig } from '~/types/api';
import {
  ACTIVITY_FEED_EVENT_TYPES,
  EVENT_DATA_RESET,
  EVENT_DELETION_BATCH_COMPLETE,
  EVENT_DELETION_PROGRESS,
  EVENT_DELETION_SUCCESS,
  EVENT_INTEGRATION_ADDED,
  EVENT_INTEGRATION_RECOVERED,
  EVENT_INTEGRATION_RECOVERY_ATTEMPT,
  EVENT_INTEGRATION_REMOVED,
  EVENT_INTEGRATION_UPDATED,
  EVENT_SETTINGS_CHANGED,
  EVENT_SETTINGS_IMPORTED,
  MODE_DRY_RUN,
} from '~/constants';
import { mostAggressiveMode } from '~/utils/diskGroupMode';

const api = useApi();
const { fetchStats: engineFetchStats, runCompletionCounter: engineRunCompletionCounter } =
  useEngineControl();
const { on: sseOn } = useEventStream();
const { hasQueueItems, fetchQueue: fetchApprovalQueue } = useApprovalQueue();
const { dateRange, dateRangeOptions, fetchHistory, patchDeletionProgress } = useEngineHistory();
const { lastFetchOk, loadError, markSuccess, markFailure } = useFetchStatus();
const { replayGapCounter } = useAppDataRefresh();

const { isRefreshing, pullProgress, pullDistance } = usePullToRefresh(async () => {
  await fetchDashboardData(true);
  fetchHistory();
});

const diskGroups = ref<DiskGroup[]>([]);
const allIntegrations = ref<IntegrationConfig[]>([]);
const recentActivity = ref<ActivityEvent[]>([]);
const loading = ref(true);
const lastUpdated = ref<Date | null>(null);

const approvalQueueVisible = computed(
  () => diskGroups.value.some((g) => g.mode === 'approval') || hasQueueItems.value,
);

const activeModes = computed(() => new Set(diskGroups.value.map((g) => g.mode)));
const { executionMode: engineExecutionMode } = useEngineControl();
const effectiveMode = computed(() => {
  if (diskGroups.value.length === 0) return engineExecutionMode.value;
  return mostAggressiveMode(activeModes.value);
});
const allDryRun = computed(() =>
  diskGroups.value.length === 0
    ? engineExecutionMode.value === MODE_DRY_RUN
    : diskGroups.value.every((g) => g.mode === MODE_DRY_RUN),
);
const anyAutoMode = computed(() => diskGroups.value.some((g) => g.mode === 'auto'));

watch(engineRunCompletionCounter, () => {
  fetchDashboardData(true);
  fetchHistory();
});

watch(replayGapCounter, () => {
  fetchDashboardData(true);
  fetchRecentActivity();
});

function handleActivityEvent(eventType: string) {
  return (data: unknown) => {
    const payload = data as Record<string, unknown>;
    const entry: ActivityEvent = {
      id: Date.now(),
      eventType,
      message: (payload.message as string) || eventType.replace(/_/g, ' '),
      metadata: JSON.stringify(payload),
      createdAt: new Date().toISOString(),
    };
    recentActivity.value = [entry, ...recentActivity.value].slice(0, 100);
  };
}

function handleDeletionBatchCompleteRefresh() {
  fetchDashboardData(true);
  fetchHistory();
}

function handleIntegrationChange() {
  api('/api/v1/integrations')
    .then((data) => {
      allIntegrations.value = data as IntegrationConfig[];
      lastUpdated.value = new Date();
    })
    .catch((err) => console.warn('[Dashboard] integration refresh failed:', err));
}

function handleDataReset() {
  fetchDashboardData(true);
  fetchHistory();
  fetchRecentActivity();
}

function handleSettingsChange() {
  api('/api/v1/disk-groups')
    .then((data) => {
      diskGroups.value = data as DiskGroup[];
      lastUpdated.value = new Date();
    })
    .catch((err) => console.warn('[Dashboard] settings refresh failed:', err));
}

onMounted(async () => {
  await fetchDashboardData();
  fetchHistory();
  fetchRecentActivity();

  const scope = { onUnmounted };
  for (const eventType of ACTIVITY_FEED_EVENT_TYPES) {
    sseOn(eventType, handleActivityEvent(eventType), scope);
  }
  sseOn(EVENT_DELETION_SUCCESS, () => fetchApprovalQueue(), scope);
  sseOn(EVENT_DELETION_PROGRESS, patchDeletionProgress, scope);
  sseOn(EVENT_DELETION_BATCH_COMPLETE, handleDeletionBatchCompleteRefresh, scope);
  sseOn(EVENT_INTEGRATION_ADDED, handleIntegrationChange, scope);
  sseOn(EVENT_INTEGRATION_UPDATED, handleIntegrationChange, scope);
  sseOn(EVENT_INTEGRATION_REMOVED, handleIntegrationChange, scope);
  sseOn(EVENT_INTEGRATION_RECOVERED, handleIntegrationChange, scope);
  sseOn(EVENT_INTEGRATION_RECOVERY_ATTEMPT, handleIntegrationChange, scope);
  sseOn(EVENT_SETTINGS_CHANGED, handleSettingsChange, scope);
  sseOn(EVENT_SETTINGS_IMPORTED, handleDataReset, scope);
  sseOn(EVENT_DATA_RESET, handleDataReset, scope);
});

async function fetchDashboardData(silent = false) {
  if (!silent) loading.value = true;
  try {
    const [groups, integrations] = await Promise.all([
      api('/api/v1/disk-groups'),
      api('/api/v1/integrations'),
    ]);
    await engineFetchStats();
    diskGroups.value = groups as DiskGroup[];
    allIntegrations.value = integrations as IntegrationConfig[];
    fetchApprovalQueue();
    lastUpdated.value = new Date();
    markSuccess();
  } catch (err) {
    console.warn('[Dashboard] fetchDashboardData failed:', err);
    markFailure();
  } finally {
    if (!silent) loading.value = false;
  }
}

async function fetchRecentActivity() {
  try {
    const data = (await api('/api/v1/activity/recent?limit=100')) as ActivityEvent[];
    recentActivity.value = data || [];
  } catch (err) {
    console.warn('[Dashboard] fetchRecentActivity failed:', err);
  }
}
</script>
