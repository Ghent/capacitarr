<template>
  <div>
    <!-- Pull-to-refresh indicator -->
    <PullToRefreshIndicator
      :pull-distance="pullDistance"
      :pull-progress="pullProgress"
      :is-refreshing="isRefreshing"
    />

    <!-- Header -->
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
          <span
            v-if="lastUpdated"
            class="inline-flex items-center gap-1 ml-2 text-xs text-muted-foreground/70"
          >
            <component
              :is="RefreshCwIcon"
              class="w-3 h-3"
              :class="{ 'animate-spin': isAutoRefreshing }"
            />
            Updated <DateDisplay :date="lastUpdated.toISOString()" />
          </span>
        </p>
      </div>
      <div class="flex items-center gap-2">
        <UiSelect v-model="dateRange">
          <UiSelectTrigger class="h-9 w-[130px]">
            <UiSelectValue placeholder="Time range" />
          </UiSelectTrigger>
          <UiSelectContent>
            <UiSelectItem v-for="opt in dateRangeOptions" :key="opt.value" :value="opt.value">
              {{ opt.label }}
            </UiSelectItem>
          </UiSelectContent>
        </UiSelect>
        <UiSelect v-model="refreshIntervalStr">
          <UiSelectTrigger class="h-9 w-[110px]">
            <UiSelectValue placeholder="Refresh" />
          </UiSelectTrigger>
          <UiSelectContent>
            <UiSelectItem v-for="opt in refreshOptions" :key="opt.value" :value="String(opt.value)">
              {{ opt.label }}
            </UiSelectItem>
          </UiSelectContent>
        </UiSelect>
      </div>
    </div>

    <!-- Integration error banner (below page title) -->
    <IntegrationErrorBanner :integrations="allIntegrations" />

    <!-- Empty state (when no disk groups and not loading) -->
    <DashboardEmptyState
      v-if="diskGroups.length === 0 && !loading"
      :integrations="allIntegrations"
    />

    <!-- Engine Activity (prominent, first card) -->
    <UiCard
      v-if="engineStats"
      v-motion
      :initial="{ opacity: 0, y: 12 }"
      :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24 } }"
      class="mb-6"
      :class="engineIsRunning ? 'engine-running-glow' : ''"
    >
      <UiCardContent class="pt-5">
        <!-- Status banner -->
        <div
          aria-live="polite"
          class="rounded-lg px-3 py-2 mb-4 flex items-center gap-2 text-sm font-medium"
          :class="engineStatusBannerClass"
        >
          <LoaderCircleIcon v-if="engineIsRunning" class="w-4 h-4 animate-spin shrink-0" />
          <component
            :is="engineIsRunning ? ActivityIcon : CheckCircle2Icon"
            v-else
            class="w-4 h-4 shrink-0"
          />
          <span v-if="engineIsRunning">{{ t('dashboard.engineRunningDetail') }}</span>
          <span v-else-if="!engineLastRunEpoch">{{ t('dashboard.engineIdleNoRuns') }}</span>
          <i18n-t v-else keypath="dashboard.engineIdleLastRun" tag="span">
            <template #time>
              <DateDisplay :date="new Date(engineLastRunEpoch * 1000).toISOString()" />
            </template>
          </i18n-t>
          <span
            v-if="!engineIsRunning && countdownText"
            class="ml-auto text-xs font-normal text-muted-foreground"
          >
            {{ countdownText }}
          </span>
        </div>

        <!-- Top row: title, run now, mode badge, evaluated/flagged -->
        <div class="flex flex-wrap items-center gap-2 mb-3">
          <div class="flex items-center gap-1.5 text-primary font-medium text-sm">
            <component :is="ActivityIcon" class="w-4 h-4" />
            {{ $t('dashboard.engineActivity') }}
          </div>
          <UiButton
            variant="outline"
            size="sm"
            :disabled="engineRunNowLoading"
            @click="engineTriggerRunNow"
          >
            <LoaderCircleIcon v-if="engineRunNowLoading" class="w-3.5 h-3.5 animate-spin" />
            <PlayIcon v-else class="w-3.5 h-3.5" />
            {{ $t('dashboard.runNow') }}
          </UiButton>
          <span class="text-xs text-muted-foreground">
            <template v-if="engineIsRunning"> 🔄 {{ $t('dashboard.engineRunning') }} </template>
            <template v-else-if="engineLastRunEpoch">
              <i18n-t keypath="dashboard.lastRun" tag="span">
                <template #time>
                  <DateDisplay :date="new Date(engineLastRunEpoch * 1000).toISOString()" />
                </template>
              </i18n-t>
            </template>
            <template v-else>
              {{ $t('dashboard.noRunsYet') }}
            </template>
          </span>
          <UiBadge
            :variant="
              engineExecutionMode === 'auto'
                ? 'destructive'
                : engineExecutionMode === 'approval'
                  ? 'outline'
                  : 'secondary'
            "
            class="ml-auto"
          >
            {{ engineModeLabel(engineExecutionMode) }}
          </UiBadge>
          <span class="text-xs text-muted-foreground">
            {{ $t('dashboard.evaluated') }} {{ engineLastRunEvaluated?.toLocaleString() ?? 0 }} ·
            {{ $t('dashboard.flagged') }} {{ engineLastRunFlagged?.toLocaleString() ?? 0 }}
          </span>
        </div>

        <!-- Sparkline: items flagged + deleted per engine run -->
        <div v-if="flaggedSeries.length > 0 || deletedSeries.length > 0" class="mb-3">
          <div class="flex items-center gap-3 mb-1">
            <span class="text-[11px] text-muted-foreground/70">
              {{ $t('dashboard.engineActivityTitle') }} · {{ dateRangeLabel }}
            </span>
            <span class="inline-flex items-center gap-1 text-[11px] text-muted-foreground">
              <span class="w-2 h-2 rounded-full bg-primary" />
              {{ $t('dashboard.flagged') }}
            </span>
            <span class="inline-flex items-center gap-1 text-[11px] text-muted-foreground">
              <span class="w-2 h-2 rounded-full bg-destructive" />
              {{ $t('dashboard.deleted') }}
            </span>
          </div>
          <ClientOnly>
            <VChart
              :option="sparklineEChartsOption"
              :autoresize="true"
              style="height: 120px; width: 100%"
            />
          </ClientOnly>
        </div>

        <!-- Toggle for mini sparklines -->
        <button
          v-if="engineHistoryData.length > 0"
          class="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors mb-2"
          @click="showMiniSparklines = !showMiniSparklines"
        >
          <component
            :is="showMiniSparklines ? ChevronUpIcon : ChevronDownIcon"
            class="w-3.5 h-3.5"
          />
          {{ showMiniSparklines ? $t('dashboard.hideDetails') : $t('dashboard.showDetails') }}
        </button>

        <!-- Mini sparklines: duration + recent activity (matched heights) -->
        <div
          v-if="showMiniSparklines && engineHistoryData.length > 0"
          class="grid grid-cols-2 gap-3 mb-3"
        >
          <!-- Run Duration -->
          <div class="rounded-lg bg-muted px-3 py-2">
            <div class="text-[11px] text-muted-foreground mb-0.5">
              {{ $t('dashboard.runDuration') }} · {{ dateRangeLabel }}
            </div>
            <div class="text-[11px] text-muted-foreground/70 mb-1">
              {{ $t('dashboard.avgDuration', { avg: avgDurationMs + 'ms' }) }} ·
              {{ $t('dashboard.maxDuration', { max: maxDurationMs + 'ms' }) }}
            </div>
            <ClientOnly>
              <VChart
                :option="durationSparklineEChartsOption"
                :autoresize="true"
                style="height: 70px; width: 100%"
              />
            </ClientOnly>
          </div>

          <!-- Recent Activity -->
          <div class="rounded-lg bg-muted px-3 py-2">
            <div class="text-[11px] text-muted-foreground mb-1 flex items-center gap-1">
              <span>{{ $t('dashboard.recentActivity') }}</span>
              <span class="text-muted-foreground/40">·</span>
              <NuxtLink to="/audit" class="text-primary hover:text-primary/80 font-medium">
                {{ $t('dashboard.viewAll') }}
              </NuxtLink>
            </div>
            <div
              v-if="recentActivity.length > 0"
              ref="activityScrollRef"
              class="h-[86px] overflow-auto pr-3"
            >
              <div
                :style="{ height: `${activityVirtualizer.getTotalSize()}px`, position: 'relative' }"
              >
                <div
                  v-for="virtualRow in activityVirtualItems"
                  :key="virtualRow.index"
                  :style="{
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    width: '100%',
                    height: `${virtualRow.size}px`,
                    transform: `translateY(${virtualRow.start}px)`,
                  }"
                >
                  <div class="flex items-center gap-1.5 py-0.5 text-[11px] leading-tight">
                    <component
                      :is="eventIcon(virtualRow.entry.eventType)"
                      class="w-3 h-3 shrink-0"
                      :class="eventIconClass(virtualRow.entry.eventType)"
                    />
                    <span class="truncate line-clamp-1 flex-1 min-w-0 text-foreground">
                      {{ virtualRow.entry.message }}
                    </span>
                    <span class="text-muted-foreground/70 shrink-0 whitespace-nowrap ml-auto">
                      <DateDisplay :date="virtualRow.entry.createdAt" />
                    </span>
                  </div>
                </div>
              </div>
            </div>
            <div
              v-else
              class="flex items-center justify-center text-[11px] text-muted-foreground/60 h-[86px]"
            >
              {{ $t('dashboard.noActivityYet') }}
            </div>
          </div>
        </div>

        <!-- Stats row: 3 compact boxes -->
        <div class="grid grid-cols-3 gap-3 mb-3">
          <!-- Would Free / Freed -->
          <div class="rounded-lg bg-muted px-3 py-2">
            <div class="text-[11px] text-muted-foreground mb-0.5">
              {{
                engineExecutionMode === 'auto' ? $t('dashboard.freed') : $t('dashboard.wouldFree')
              }}
            </div>
            <div class="text-sm font-bold tabular-nums">
              {{ formatBytes(engineStats.lastRunFreedBytes ?? 0) }}
            </div>
          </div>

          <!-- Queue -->
          <div class="rounded-lg bg-muted px-3 py-2">
            <div class="text-[11px] text-muted-foreground mb-0.5">
              {{ $t('dashboard.queue') }}
            </div>
            <div class="flex items-center gap-1.5">
              <span
                class="w-2 h-2 rounded-full shrink-0"
                :class="(engineStats.queueDepth ?? 0) > 0 ? 'bg-warning' : 'bg-success'"
              />
              <span class="text-sm font-bold tabular-nums">{{ engineStats.queueDepth ?? 0 }}</span>
              <span class="text-[11px] text-muted-foreground">{{ $t('common.items') }}</span>
            </div>
          </div>

          <!-- Active Delete -->
          <div class="rounded-lg bg-muted px-3 py-2">
            <div class="text-[11px] text-muted-foreground mb-0.5">
              {{ $t('dashboard.activeDelete') }}
            </div>
            <div class="text-sm">
              <template v-if="engineStats.currentlyDeleting">
                <span class="inline-flex items-center gap-1.5">
                  <span class="w-2 h-2 rounded-full bg-primary animate-pulse shrink-0" />
                  <span
                    class="font-medium truncate max-w-[120px]"
                    :title="engineStats.currentlyDeleting"
                  >
                    {{ engineStats.currentlyDeleting }}
                  </span>
                </span>
              </template>
              <template v-else-if="engineExecutionMode === 'dry-run'">
                <span class="text-muted-foreground text-xs">{{
                  $t('dashboard.dryRunNoDelete')
                }}</span>
              </template>
              <template v-else-if="(engineStats.queueDepth ?? 0) === 0">
                <span class="text-muted-foreground">{{ $t('common.idle') }}</span>
              </template>
              <template v-else>
                <span class="text-muted-foreground">{{ $t('dashboard.waiting') }}</span>
              </template>
            </div>
          </div>
        </div>

        <!-- Footer link -->
        <NuxtLink
          to="/audit"
          class="text-xs text-primary hover:text-primary/80 font-medium transition-colors"
        >
          {{ $t('dashboard.viewAuditLog') }}
        </NuxtLink>
      </UiCardContent>
    </UiCard>

    <!-- Approval Queue (only in approval mode) -->
    <ApprovalQueueCard v-if="approvalQueueVisible" />

    <!-- Snoozed Items (visible in all modes when snoozed items exist) -->
    <SnoozedItemsCard />

    <!-- Deletion Queue (always visible) -->
    <DeletionQueueCard />

    <!-- Per-Disk-Group Sections -->
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

    <!-- Skeleton Loading State -->
    <template v-if="loading">
      <SkeletonCard :show-chart="true" />
    </template>
  </div>
</template>

<script setup lang="ts">
import {
  LoaderCircleIcon,
  RefreshCwIcon,
  ActivityIcon,
  PlayIcon,
  CheckCircle2Icon,
  Trash2Icon,
  ChevronDownIcon,
  ChevronUpIcon,
  SettingsIcon,
  UserIcon,
  PlugIcon,
  AlertCircleIcon,
  XCircleIcon,
  PlusCircleIcon,
  PencilIcon,
  PowerIcon,
  KeyIcon,
  SlidersHorizontalIcon,
  AlarmClockOffIcon,
  DatabaseIcon,
  BellIcon,
  BellRingIcon,
  BellOffIcon,
  ArrowUpCircleIcon,
} from 'lucide-vue-next';
import { useVirtualizer } from '@tanstack/vue-virtual';
import { formatBytes } from '~/utils/format';
import type { ActivityEvent, DeletionProgress, DiskGroup, IntegrationConfig } from '~/types/api';

const { t } = useI18n();
const api = useApi();
const {
  chart1Color,
  chart3Color,
  destructiveColor,
  glowLineStyle,
  gradientArea,
  tooltipConfig,
  emphasisConfig,
} = useEChartsDefaults();

// Use shared engine control composable for isRunning detection + toast on completion
const {
  workerStats: engineControlStats,
  executionMode: engineExecutionMode,
  lastRunEpoch: engineLastRunEpoch,
  lastRunEvaluated: engineLastRunEvaluated,
  lastRunFlagged: engineLastRunFlagged,
  isRunning: engineIsRunning,
  pollIntervalSeconds: enginePollInterval,
  runNowLoading: engineRunNowLoading,
  runCompletionCounter: engineRunCompletionCounter,
  modeLabel: engineModeLabel,
  fetchStats: engineFetchStats,
  triggerRunNow: engineTriggerRunNow,
} = useEngineControl();

// SSE event stream — subscribe for real-time dashboard updates
const { on: sseOn, off: sseOff } = useEventStream();

// Approval queue (shown when execution mode is "approval")
const { isApprovalMode, fetchQueue: fetchApprovalQueue } = useApprovalQueue();
const approvalQueueVisible = computed(() => isApprovalMode.value);

// Pull-to-refresh for touch devices
const { isRefreshing, pullProgress, pullDistance } = usePullToRefresh(async () => {
  await fetchDashboardData(true);
  refreshKey.value++;
});

const dateRangeOptions = [
  { label: 'Last Hour', value: '1h' },
  { label: 'Last 6h', value: '6h' },
  { label: 'Last 24h', value: '24h' },
  { label: 'Last 7 Days', value: '7d' },
  { label: 'Last 30 Days', value: '30d' },
  { label: 'All Time', value: 'all' },
];

const refreshOptions = [
  { label: '⏸ Paused', value: 0 },
  { label: '↻ 1s', value: 1000 },
  { label: '↻ 2s', value: 2000 },
  { label: '↻ 5s', value: 5000 },
  { label: '↻ 10s', value: 10000 },
  { label: '↻ 15s', value: 15000 },
  { label: '↻ 30s', value: 30000 },
  { label: '↻ 1m', value: 60000 },
  { label: '↻ 5m', value: 300000 },
];

const dateRange = ref('24h');
const refreshIntervalStr = ref('15000');
const refreshInterval = computed(() => Number(refreshIntervalStr.value));
const diskGroups = ref<DiskGroup[]>([]);
const allIntegrations = ref<IntegrationConfig[]>([]);
const engineHistoryData = ref<
  Array<{
    timestamp: string;
    evaluated: number;
    flagged: number;
    deleted: number;
    freedBytes: number;
    durationMs: number;
  }>
>([]);
const showMiniSparklines = ref(
  typeof localStorage !== 'undefined'
    ? localStorage.getItem('capacitarr:showMiniSparklines') !== 'false'
    : true,
);
watch(showMiniSparklines, (val) => {
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem('capacitarr:showMiniSparklines', String(val));
  }
});
const recentActivity = ref<ActivityEvent[]>([]);

// Activity feed virtual scroller
const activityScrollRef = ref<HTMLElement | null>(null);
const activityVirtualizer = useVirtualizer(
  computed(() => ({
    count: recentActivity.value.length,
    getScrollElement: () => activityScrollRef.value,
    estimateSize: () => 20,
    overscan: 5,
  })),
);
const activityVirtualItems = computed(() =>
  activityVirtualizer.value.getVirtualItems().map((row) => ({
    ...row,
    entry: recentActivity.value[row.index]!,
  })),
);
const loading = ref(true);
const lastUpdated = ref<Date | null>(null);
const isAutoRefreshing = ref(false);
const refreshKey = ref(0);

// Icon component for activity events — covers all 39 typed event types
function eventIcon(eventType: string) {
  switch (eventType) {
    // Engine
    case 'engine_start':
      return PlayIcon;
    case 'engine_complete':
      return CheckCircle2Icon;
    case 'engine_error':
      return AlertCircleIcon;
    case 'engine_mode_changed':
      return SettingsIcon;
    case 'manual_run_triggered':
      return PlayIcon;
    // Settings
    case 'settings_changed':
      return SettingsIcon;
    case 'threshold_changed':
      return SlidersHorizontalIcon;
    // Auth
    case 'login':
      return UserIcon;
    case 'password_changed':
      return KeyIcon;
    case 'username_changed':
      return UserIcon;
    case 'api_key_generated':
      return KeyIcon;
    // Integrations
    case 'integration_added':
    case 'integration_updated':
    case 'integration_removed':
    case 'integration_test':
    case 'integration_test_failed':
      return PlugIcon;
    // Approval
    case 'approval_approved':
      return CheckCircle2Icon;
    case 'approval_rejected':
      return XCircleIcon;
    case 'approval_unsnoozed':
    case 'approval_bulk_unsnoozed':
      return AlarmClockOffIcon;
    case 'approval_orphans_recovered':
      return RefreshCwIcon;
    // Deletion
    case 'deletion_queued':
    case 'deletion_success':
    case 'deletion_dry_run':
      return Trash2Icon;
    case 'deletion_failed':
      return AlertCircleIcon;
    case 'deletion_batch_complete':
      return CheckCircle2Icon;
    case 'deletion_progress':
      return Trash2Icon;
    // Disk
    case 'threshold_breached':
      return AlertCircleIcon;
    // Version
    case 'update_available':
      return ArrowUpCircleIcon;
    // Rules
    case 'rule_created':
      return PlusCircleIcon;
    case 'rule_updated':
      return PencilIcon;
    case 'rule_deleted':
      return Trash2Icon;
    // Notifications
    case 'notification_channel_added':
    case 'notification_channel_updated':
    case 'notification_channel_removed':
      return BellIcon;
    case 'notification_sent':
      return BellRingIcon;
    case 'notification_delivery_failed':
      return BellOffIcon;
    // Data
    case 'data_reset':
      return DatabaseIcon;
    // System
    case 'server_started':
      return PowerIcon;
    default:
      return ActivityIcon;
  }
}

// Color class for activity event icons — covers all 39 typed event types
function eventIconClass(eventType: string): string {
  switch (eventType) {
    case 'engine_start':
    case 'engine_mode_changed':
    case 'manual_run_triggered':
    case 'threshold_changed':
    case 'approval_unsnoozed':
    case 'approval_bulk_unsnoozed':
    case 'rule_created':
    case 'update_available':
      return 'text-primary';
    case 'engine_complete':
    case 'approval_approved':
    case 'server_started':
    case 'integration_added':
    case 'integration_test':
    case 'deletion_success':
    case 'deletion_batch_complete':
    case 'notification_channel_added':
    case 'notification_sent':
      return 'text-success';
    case 'engine_error':
    case 'approval_rejected':
    case 'rule_deleted':
    case 'integration_test_failed':
    case 'integration_removed':
    case 'deletion_failed':
    case 'threshold_breached':
    case 'data_reset':
    case 'notification_channel_removed':
    case 'notification_delivery_failed':
      return 'text-destructive';
    case 'deletion_queued':
    case 'deletion_dry_run':
    case 'deletion_progress':
    case 'approval_orphans_recovered':
      return 'text-warning';
    case 'rule_updated':
    case 'password_changed':
    case 'username_changed':
    case 'api_key_generated':
    case 'login':
    case 'settings_changed':
    case 'integration_updated':
    case 'notification_channel_updated':
      return 'text-muted-foreground';
    default:
      return 'text-muted-foreground';
  }
}

// Engine stats — alias from shared composable
const engineStats = computed(() => engineControlStats.value);

const dateRangeLabel = computed(() => {
  const match = dateRangeOptions.find((o) => o.value === dateRange.value);
  return match?.label ?? dateRange.value;
});

// --- Status banner ---
const engineStatusBannerClass = computed(() => {
  if (engineIsRunning.value) {
    return 'bg-primary/10 text-primary border border-primary/20';
  }
  return 'bg-muted text-muted-foreground';
});

// engineStatusText removed — now rendered inline with <DateDisplay> component

// --- Countdown to next run ---
const nowEpoch = ref(Math.floor(Date.now() / 1000));
let countdownTimer: ReturnType<typeof setInterval> | null = null;

onMounted(() => {
  countdownTimer = setInterval(() => {
    nowEpoch.value = Math.floor(Date.now() / 1000);
  }, 1000);
});

onUnmounted(() => {
  if (countdownTimer) clearInterval(countdownTimer);
});

const countdownText = computed(() => {
  if (engineIsRunning.value) return '';
  if (!engineLastRunEpoch.value || !enginePollInterval.value) return '';

  const nextRunEpoch = engineLastRunEpoch.value + enginePollInterval.value;
  const remaining = nextRunEpoch - nowEpoch.value;

  if (remaining <= 0) return t('dashboard.nextRunImminent');
  if (remaining < 60) return t('dashboard.nextRunSeconds', { seconds: remaining });
  if (remaining < 3600) {
    const mins = Math.floor(remaining / 60);
    const secs = remaining % 60;
    return t('dashboard.nextRunMinSec', { min: mins, sec: secs });
  }
  const hours = Math.floor(remaining / 3600);
  const mins = Math.floor((remaining % 3600) / 60);
  return t('dashboard.nextRunHourMin', { hour: hours, min: mins });
});

// --- Auto refresh (non-event data: disk groups, integrations, dashboard stats) ---
// Engine stats (workerStats) are updated in real-time via SSE events:
//   engine_start, engine_complete, engine_error, engine_mode_changed, deletion_progress.
// The periodic fetchDashboardData() still calls engineFetchStats() as a reconciliation
// fallback for fields not covered by SSE (e.g. lastRunFreedBytes, protectedCount).
let autoRefreshTimer: ReturnType<typeof setInterval> | null = null;

function startAutoRefresh() {
  stopAutoRefresh();
  if (refreshInterval.value > 0) {
    autoRefreshTimer = setInterval(async () => {
      isAutoRefreshing.value = true;
      await fetchDashboardData(true);
      refreshKey.value++;
      isAutoRefreshing.value = false;
    }, refreshInterval.value);
  }
}

function stopAutoRefresh() {
  if (autoRefreshTimer) {
    clearInterval(autoRefreshTimer);
    autoRefreshTimer = null;
  }
}

watch(refreshInterval, () => {
  startAutoRefresh();
});

// When the engine finishes a run (detected via SSE engine_complete event),
// immediately refresh all dashboard data so the UI reflects the latest state.
watch(engineRunCompletionCounter, () => {
  fetchDashboardData(true);
  fetchEngineHistory();
  refreshKey.value++;
});

// --- SSE event subscriptions for real-time dashboard updates ---

// Handler: prepend any activity event to the recent activity feed in real-time.
// The SSE data payload includes { message, ... }; we construct an ActivityEvent
// from the SSE event type + data.
function handleActivityEvent(eventType: string) {
  return (data: unknown) => {
    const payload = data as Record<string, unknown>;
    const entry: ActivityEvent = {
      id: Date.now(), // Temporary client-side ID for key uniqueness
      eventType,
      message: (payload.message as string) || eventType.replace(/_/g, ' '),
      metadata: JSON.stringify(payload),
      createdAt: new Date().toISOString(),
    };
    // Prepend to feed, cap at 100 entries
    recentActivity.value = [entry, ...recentActivity.value].slice(0, 100);
  };
}

// All event types that should prepend to the activity feed
const activityEventTypes = [
  'engine_start',
  'engine_complete',
  'engine_error',
  'engine_mode_changed',
  'manual_run_triggered',
  'settings_changed',
  'threshold_changed',
  'login',
  'password_changed',
  'username_changed',
  'api_key_generated',
  'integration_added',
  'integration_updated',
  'integration_removed',
  'integration_test',
  'integration_test_failed',
  'approval_approved',
  'approval_rejected',
  'approval_unsnoozed',
  'approval_bulk_unsnoozed',
  'approval_orphans_recovered',
  'deletion_queued',
  'deletion_success',
  'deletion_failed',
  'deletion_dry_run',
  'deletion_batch_complete',
  'deletion_progress',
  'threshold_breached',
  'update_available',
  'rule_created',
  'rule_updated',
  'rule_deleted',
  'notification_channel_added',
  'notification_channel_updated',
  'notification_channel_removed',
  'notification_sent',
  'notification_delivery_failed',
  'data_reset',
  'server_started',
] as const;

// Keep handler refs so we can unsubscribe on unmount
const _activityHandlers = new Map<string, (data: unknown) => void>();

// Handler: deletion_progress SSE event — patch last sparkline data point in real-time
function handleDeletionProgressSparkline(data: unknown) {
  const event = data as DeletionProgress;
  const history = engineHistoryData.value;
  const last = history.length > 0 ? history[history.length - 1] : undefined;
  if (last) {
    // Mutate + reassign to trigger Vue reactivity on the sparkline computed properties
    engineHistoryData.value = [...history.slice(0, -1), { ...last, deleted: event.succeeded }];
  }
}

// Handler: deletion_batch_complete SSE event — re-fetch engine history for authoritative data
function handleDeletionBatchCompleteRefresh() {
  fetchDashboardData(true);
  fetchEngineHistory();
  refreshKey.value++;
}

// Handler: approval queue changes — refresh the queue
function handleApprovalChange() {
  fetchApprovalQueue();
}

onMounted(async () => {
  await fetchDashboardData();
  startAutoRefresh();

  // Subscribe to all activity event types for the real-time feed
  for (const eventType of activityEventTypes) {
    const handler = handleActivityEvent(eventType);
    _activityHandlers.set(eventType, handler);
    sseOn(eventType, handler);
  }

  // Subscribe to approval-related events to refresh the queue
  sseOn('approval_approved', handleApprovalChange);
  sseOn('approval_rejected', handleApprovalChange);
  sseOn('approval_unsnoozed', handleApprovalChange);
  sseOn('approval_bulk_unsnoozed', handleApprovalChange);
  sseOn('approval_orphans_recovered', handleApprovalChange);
  sseOn('deletion_success', handleApprovalChange);

  // When a deletion completes, patch the most recent sparkline data point in real-time
  sseOn('deletion_progress', handleDeletionProgressSparkline);

  // When all deletions for a cycle finish, refresh dashboard stats — the numbers are now final
  sseOn('deletion_batch_complete', handleDeletionBatchCompleteRefresh);
});

onUnmounted(() => {
  stopAutoRefresh();

  // Unsubscribe all activity event handlers
  for (const [eventType, handler] of _activityHandlers) {
    sseOff(eventType, handler);
  }
  _activityHandlers.clear();

  // Unsubscribe deletion progress handlers
  sseOff('deletion_progress', handleDeletionProgressSparkline);
  sseOff('deletion_batch_complete', handleDeletionBatchCompleteRefresh);

  // Unsubscribe approval handlers
  sseOff('approval_approved', handleApprovalChange);
  sseOff('approval_rejected', handleApprovalChange);
  sseOff('approval_unsnoozed', handleApprovalChange);
  sseOff('approval_bulk_unsnoozed', handleApprovalChange);
  sseOff('approval_orphans_recovered', handleApprovalChange);
  sseOff('deletion_success', handleApprovalChange);
});

async function fetchDashboardData(silent = false) {
  if (!silent) loading.value = true;
  try {
    const [groups, integrations] = await Promise.all([
      api('/api/v1/disk-groups'),
      api('/api/v1/integrations'),
    ]);
    // Fetch engine stats via the shared composable (handles toast on completion).
    // Must be awaited so workerStats is populated before fetchApprovalQueue()
    // checks isApprovalMode — otherwise executionMode defaults to 'dry-run'
    // and the approval queue guard clears the list on initial load.
    await engineFetchStats();
    // Fetch approval queue (non-blocking, only runs in approval mode)
    fetchApprovalQueue();
    // Fetch sparkline engine history data in parallel (non-blocking)
    fetchEngineHistory();
    // Fetch recent activity for the mini feed (non-blocking)
    fetchRecentActivity();
    diskGroups.value = groups as DiskGroup[];
    allIntegrations.value = integrations as IntegrationConfig[];
    lastUpdated.value = new Date();
  } catch (err) {
    console.warn('[Dashboard] fetchDashboardData failed:', err);
  } finally {
    if (!silent) loading.value = false;
  }
}

// --- Sparkline: engine history (flagged + deleted per engine run) ---

// Bucket data points into hourly groups, summing values within each hour.
// This reduces dense per-run data (hundreds of points) into a smaller set of
// points that produce visually meaningful curves with visible gradient fill.
function bucketHourly(
  data: Array<{ timestamp: string }>,
  valueKey: string,
): Array<{ x: number; y: number }> {
  const buckets = new Map<number, { ts: number; sum: number }>();
  for (const point of data) {
    const ts = new Date(point.timestamp).getTime();
    const hourKey = Math.floor(ts / 3_600_000);
    const existing = buckets.get(hourKey);
    const value = (point as Record<string, unknown>)[valueKey] as number;
    if (existing) {
      existing.sum += value;
    } else {
      // Use the midpoint of the hour as the representative timestamp
      buckets.set(hourKey, { ts: hourKey * 3_600_000 + 1_800_000, sum: value });
    }
  }
  return Array.from(buckets.values())
    .sort((a, b) => a.ts - b.ts)
    .map((b) => ({ x: b.ts, y: b.sum }));
}

const flaggedSeries = computed(() => bucketHourly(engineHistoryData.value, 'flagged'));

const deletedSeries = computed(() => bucketHourly(engineHistoryData.value, 'deleted'));

// --- ECharts sparkline options ---

const sparklineEChartsOption = computed(() => {
  const flagged = flaggedSeries.value;
  const deleted = deletedSeries.value;
  const series: Array<Record<string, unknown>> = [];

  if (flagged.length > 0) {
    series.push({
      name: 'Flagged',
      type: 'line',
      smooth: true,
      symbol: 'none',
      lineStyle: glowLineStyle(chart1Color.value),
      areaStyle: gradientArea(chart1Color.value),
      emphasis: emphasisConfig(),
      data: flagged.map((d) => [d.x, d.y]),
    });
  }
  if (deleted.length > 0) {
    series.push({
      name: 'Deleted',
      type: 'line',
      smooth: true,
      symbol: 'none',
      lineStyle: glowLineStyle(destructiveColor.value),
      areaStyle: gradientArea(destructiveColor.value),
      emphasis: emphasisConfig(),
      data: deleted.map((d) => [d.x, d.y]),
    });
  }

  return {
    animation: true,
    animationDelay: (idx: number) => idx * 10,
    grid: { top: 4, right: 4, bottom: 4, left: 4 },
    xAxis: {
      type: 'time',
      show: false,
      axisPointer: {
        type: 'cross',
        lineStyle: { color: chart1Color.value, opacity: 0.3 },
      },
    },
    yAxis: { type: 'value', show: false },
    tooltip: {
      trigger: 'axis',
      ...tooltipConfig(),
    },
    series,
  };
});

// --- Mini sparklines: duration + freed bytes ---

const avgDurationMs = computed(() => {
  const data = engineHistoryData.value;
  if (data.length === 0) return 0;
  const sum = data.reduce((acc, p) => acc + p.durationMs, 0);
  return Math.round(sum / data.length);
});

const maxDurationMs = computed(() => {
  const data = engineHistoryData.value;
  if (data.length === 0) return 0;
  return Math.max(...data.map((p) => p.durationMs));
});

// Duration sparkline ECharts option
const durationSparklineEChartsOption = computed(() => ({
  animation: true,
  grid: { top: 4, right: 4, bottom: 4, left: 4 },
  xAxis: { type: 'time' as const, show: false },
  yAxis: { type: 'value' as const, show: false },
  tooltip: {
    trigger: 'axis' as const,
    ...tooltipConfig(),
    formatter: (params: Array<{ value: [number, number] }>) => {
      if (!params[0]) return '';
      const [ts, val] = params[0].value;
      const date = new Date(ts).toLocaleString();
      return `${date}<br/>${val}ms`;
    },
  },
  visualMap: [
    {
      show: false,
      min: 0,
      max: maxDurationMs.value || 1,
      inRange: { color: [chart3Color.value, '#f59e0b', destructiveColor.value] },
    },
  ],
  series: [
    {
      name: 'Duration',
      type: 'line',
      smooth: true,
      symbol: 'none',
      lineStyle: glowLineStyle(chart3Color.value),
      areaStyle: gradientArea(chart3Color.value),
      emphasis: emphasisConfig(),
      data: engineHistoryData.value.map((p) => [new Date(p.timestamp).getTime(), p.durationMs]),
    },
  ],
}));

// Re-fetch engine history when time range changes
watch(dateRange, () => {
  fetchEngineHistory();
});

async function fetchEngineHistory() {
  try {
    const range = dateRange.value || '7d';
    const data = (await api(`/api/v1/engine/history?range=${range}`)) as Array<{
      timestamp: string;
      evaluated: number;
      flagged: number;
      deleted: number;
      freedBytes: number;
      durationMs: number;
    }>;
    engineHistoryData.value = data || [];
  } catch (err) {
    console.warn('[Dashboard] fetchEngineHistory failed:', err);
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
