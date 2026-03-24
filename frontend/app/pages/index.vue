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
            <component :is="RefreshCwIcon" class="w-3 h-3" />
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

        <!-- Top row: title, run now, mode badge, evaluated/candidates -->
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
              engineExecutionMode === MODE_AUTO
                ? 'destructive'
                : engineExecutionMode === MODE_APPROVAL
                  ? 'outline'
                  : 'secondary'
            "
            class="ml-auto"
          >
            {{ engineModeLabel(engineExecutionMode) }}
          </UiBadge>
          <span class="text-xs text-muted-foreground">
            {{ $t('dashboard.evaluated') }} {{ engineLastRunEvaluated?.toLocaleString() ?? 0 }} ·
            {{ $t('dashboard.candidates') }} {{ engineLastRunCandidates?.toLocaleString() ?? 0 }}
          </span>
        </div>

        <!-- Sparkline: candidates + deleted/would-delete per engine run -->
        <div v-if="engineHistoryData.length > 0" class="mb-3">
          <div class="flex items-center gap-3 mb-1">
            <span class="text-[11px] text-muted-foreground/70">
              {{ $t('dashboard.engineActivityTitle') }} · {{ dateRangeLabel }}
            </span>
            <span class="inline-flex items-center gap-1 text-[11px] text-muted-foreground">
              <span class="w-2 h-2 rounded-full bg-primary" />
              {{ $t('dashboard.candidates') }}
            </span>
            <span class="inline-flex items-center gap-1 text-[11px] text-muted-foreground">
              <span
                class="w-2 h-2 rounded-full"
                :class="isDryRunMode ? 'bg-amber-500' : 'bg-destructive'"
              />
              {{ isDryRunMode ? $t('dashboard.wouldDelete') : $t('dashboard.deleted') }}
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
                engineExecutionMode === MODE_AUTO
                  ? $t('dashboard.freed')
                  : $t('dashboard.wouldFree')
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
              <template v-else-if="engineExecutionMode === MODE_DRY_RUN">
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

    <!-- Deletion Queue (always visible) -->
    <DeletionQueueCard />

    <!-- Snoozed Items (visible in all modes when snoozed items exist) -->
    <SnoozedItemsCard />

    <!-- Approval Queue (only in approval mode) -->
    <ApprovalQueueCard v-if="approvalQueueVisible" />

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
import {
  MODE_DRY_RUN,
  MODE_AUTO,
  MODE_APPROVAL,
  EVENT_DELETION_SUCCESS,
  EVENT_DELETION_DRY_RUN,
  EVENT_DELETION_FAILED,
  EVENT_DELETION_QUEUED,
  EVENT_DELETION_PROGRESS,
  EVENT_DELETION_BATCH_COMPLETE,
} from '~/constants';

const { t } = useI18n();
const api = useApi();
const {
  chart1Color,
  chart3Color,
  destructiveColor,
  successColor,
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
  lastRunCandidates: engineLastRunCandidates,
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
  fetchEngineHistory();
});

const dateRangeOptions = [
  { label: 'Last Hour', value: '1h' },
  { label: 'Last 6h', value: '6h' },
  { label: 'Last 24h', value: '24h' },
  { label: 'Last 7 Days', value: '7d' },
  { label: 'Last 30 Days', value: '30d' },
  { label: 'All Time', value: 'all' },
];

const dateRange = ref('24h');
const diskGroups = ref<DiskGroup[]>([]);
const allIntegrations = ref<IntegrationConfig[]>([]);
const engineHistoryData = ref<
  Array<{
    timestamp: string;
    evaluated: number;
    candidates: number;
    queued: number;
    deleted: number;
    freedBytes: number;
    durationMs: number;
    executionMode: string;
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
    case EVENT_DELETION_QUEUED:
    case EVENT_DELETION_SUCCESS:
    case EVENT_DELETION_DRY_RUN:
      return Trash2Icon;
    case EVENT_DELETION_FAILED:
      return AlertCircleIcon;
    case EVENT_DELETION_BATCH_COMPLETE:
      return CheckCircle2Icon;
    case EVENT_DELETION_PROGRESS:
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
    case EVENT_DELETION_SUCCESS:
    case EVENT_DELETION_BATCH_COMPLETE:
    case 'notification_channel_added':
    case 'notification_sent':
      return 'text-success';
    case 'engine_error':
    case 'approval_rejected':
    case 'rule_deleted':
    case 'integration_test_failed':
    case 'integration_removed':
    case EVENT_DELETION_FAILED:
    case 'threshold_breached':
    case 'data_reset':
    case 'notification_channel_removed':
    case 'notification_delivery_failed':
      return 'text-destructive';
    case EVENT_DELETION_QUEUED:
    case EVENT_DELETION_DRY_RUN:
    case EVENT_DELETION_PROGRESS:
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

// --- SSE-driven data refresh ---
// All dashboard data is updated via SSE events. The auto-refresh timer was
// removed because SSE covers all real-time state. Disk groups and integrations
// are re-fetched on engine_complete (they change once per engine cycle).

// When the engine finishes a run (detected via SSE engine_complete event),
// refresh disk groups, engine stats, and sparkline history.
watch(engineRunCompletionCounter, () => {
  fetchDashboardData(true);
  fetchEngineHistory();
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
  EVENT_DELETION_QUEUED,
  EVENT_DELETION_SUCCESS,
  EVENT_DELETION_FAILED,
  EVENT_DELETION_DRY_RUN,
  EVENT_DELETION_BATCH_COMPLETE,
  EVENT_DELETION_PROGRESS,
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

// Handler: deletion_progress SSE event — patch last sparkline data point in real-time.
// In dry-run mode, the sparkline shows the "queued" series (would-delete count),
// so we patch that field. In auto/approval mode, it shows "deleted".
function handleDeletionProgressSparkline(data: unknown) {
  const event = data as DeletionProgress;
  const history = engineHistoryData.value;
  const last = history.length > 0 ? history[history.length - 1] : undefined;
  if (last) {
    const patchField = isDryRunMode.value ? 'queued' : 'deleted';
    engineHistoryData.value = [...history.slice(0, -1), { ...last, [patchField]: event.succeeded }];
  }
}

// Handler: deletion_batch_complete SSE event — re-fetch engine history for authoritative data
function handleDeletionBatchCompleteRefresh() {
  fetchDashboardData(true);
  fetchEngineHistory();
}

// Handler: integration changes — refresh the integration list
function handleIntegrationChange() {
  api('/api/v1/integrations')
    .then((data) => {
      allIntegrations.value = data as IntegrationConfig[];
      lastUpdated.value = new Date();
    })
    .catch((err) => console.warn('[Dashboard] integration refresh failed:', err));
}

// Handler: settings changes — refresh disk groups (threshold may have changed)
function handleSettingsChange() {
  api('/api/v1/disk-groups')
    .then((data) => {
      diskGroups.value = data as DiskGroup[];
      lastUpdated.value = new Date();
    })
    .catch((err) => console.warn('[Dashboard] settings refresh failed:', err));
}

// Handler: approval queue changes — refresh the queue
function handleApprovalChange() {
  fetchApprovalQueue();
}

onMounted(async () => {
  // Initial hydration — fetch all data once
  await fetchDashboardData();
  // Fetch sparkline history and recent activity (non-blocking, after initial data)
  fetchEngineHistory();
  fetchRecentActivity();

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
  sseOn(EVENT_DELETION_SUCCESS, handleApprovalChange);

  // When a deletion completes, patch the most recent sparkline data point in real-time
  sseOn(EVENT_DELETION_PROGRESS, handleDeletionProgressSparkline);

  // When all deletions for a cycle finish, refresh dashboard stats — the numbers are now final
  sseOn(EVENT_DELETION_BATCH_COMPLETE, handleDeletionBatchCompleteRefresh);

  // SSE-driven data refresh: integration and settings changes
  sseOn('integration_added', handleIntegrationChange);
  sseOn('integration_updated', handleIntegrationChange);
  sseOn('integration_removed', handleIntegrationChange);
  sseOn('settings_changed', handleSettingsChange);
});

onUnmounted(() => {
  // Unsubscribe all activity event handlers
  for (const [eventType, handler] of _activityHandlers) {
    sseOff(eventType, handler);
  }
  _activityHandlers.clear();

  // Unsubscribe deletion progress handlers
  sseOff(EVENT_DELETION_PROGRESS, handleDeletionProgressSparkline);
  sseOff(EVENT_DELETION_BATCH_COMPLETE, handleDeletionBatchCompleteRefresh);

  // Unsubscribe approval handlers
  sseOff('approval_approved', handleApprovalChange);
  sseOff('approval_rejected', handleApprovalChange);
  sseOff('approval_unsnoozed', handleApprovalChange);
  sseOff('approval_bulk_unsnoozed', handleApprovalChange);
  sseOff('approval_orphans_recovered', handleApprovalChange);
  sseOff(EVENT_DELETION_SUCCESS, handleApprovalChange);

  // Unsubscribe SSE-driven data refresh handlers
  sseOff('integration_added', handleIntegrationChange);
  sseOff('integration_updated', handleIntegrationChange);
  sseOff('integration_removed', handleIntegrationChange);
  sseOff('settings_changed', handleSettingsChange);
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
    // Note: fetchEngineHistory() and fetchRecentActivity() are NOT called here.
    // They are fetched once on mount and updated via SSE events to avoid
    // replacing the data array (which causes ECharts to replay animations).
    diskGroups.value = groups as DiskGroup[];
    allIntegrations.value = integrations as IntegrationConfig[];
    lastUpdated.value = new Date();
  } catch (err) {
    console.warn('[Dashboard] fetchDashboardData failed:', err);
  } finally {
    if (!silent) loading.value = false;
  }
}

// --- Sparkline: engine history (candidates + deleted/would-delete per engine run) ---

// Bucket data points into hourly groups, summing values within each hour.
// This reduces dense per-run data (hundreds of points) into a smaller set of
// points that produce visually meaningful curves with visible gradient fill.
// Only used for 7d+ ranges where point density is high.
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

// Prepare series data with range-aware bucketing strategy.
// For 24h and below, use raw data points to preserve individual engine runs.
// For 7d+, bucket into hourly groups to reduce noise.
function prepareSeriesData(
  data: Array<{ timestamp: string }>,
  valueKey: string,
  range: string,
): Array<{ x: number; y: number }> {
  if (range === '1h' || range === '6h' || range === '24h' || data.length <= 24) {
    return data.map((point) => ({
      x: new Date(point.timestamp).getTime(),
      y: (point as Record<string, unknown>)[valueKey] as number,
    }));
  }
  return bucketHourly(data, valueKey);
}

const candidatesSeries = computed(() =>
  prepareSeriesData(engineHistoryData.value, 'candidates', dateRange.value),
);
const queuedSeries = computed(() =>
  prepareSeriesData(engineHistoryData.value, 'queued', dateRange.value),
);
const deletedSeries = computed(() =>
  prepareSeriesData(engineHistoryData.value, 'deleted', dateRange.value),
);
const isDryRunMode = computed(() => engineExecutionMode.value === MODE_DRY_RUN);

// --- ECharts sparkline options ---

const sparklineEChartsOption = computed(() => {
  const candidates = candidatesSeries.value;
  const isDryRun = isDryRunMode.value;
  const secondSeriesData = isDryRun ? queuedSeries.value : deletedSeries.value;
  const ghostSeriesData = isDryRun ? deletedSeries.value : queuedSeries.value;
  const secondName = isDryRun ? t('dashboard.wouldDelete') : t('dashboard.deleted');
  const secondColor = isDryRun ? chart3Color.value : destructiveColor.value;
  const ghostName = isDryRun ? t('dashboard.deleted') : t('dashboard.wouldDelete');
  const ghostColor = isDryRun ? destructiveColor.value : chart3Color.value;
  const series: Array<Record<string, unknown>> = [];

  // Show symbols when data is sparse (≤ 3 points) so single points are visible
  const sparseSymbol = (len: number) => (len <= 3 ? 'circle' : 'none');
  const sparseSymbolSize = (len: number) => (len <= 3 ? 6 : 0);

  // Primary series: Candidates (always shown)
  if (candidates.length > 0) {
    series.push({
      name: t('dashboard.candidates'),
      type: 'line',
      smooth: true,
      symbol: sparseSymbol(candidates.length),
      symbolSize: sparseSymbolSize(candidates.length),
      itemStyle: { color: chart1Color.value },
      lineStyle: glowLineStyle(chart1Color.value),
      areaStyle: gradientArea(chart1Color.value),
      emphasis: emphasisConfig(),
      data: candidates.map((d) => [d.x, d.y]),
    });
  }

  // Active second series: Would Delete (dry-run) or Deleted (auto/approval)
  if (secondSeriesData.length > 0) {
    const lastPoint = secondSeriesData[secondSeriesData.length - 1];
    series.push({
      name: secondName,
      type: 'line',
      smooth: true,
      symbol: sparseSymbol(secondSeriesData.length),
      symbolSize: sparseSymbolSize(secondSeriesData.length),
      itemStyle: { color: secondColor },
      lineStyle: glowLineStyle(secondColor),
      areaStyle: gradientArea(secondColor),
      emphasis: emphasisConfig(),
      data: secondSeriesData.map((d) => [d.x, d.y]),
      // Animated pulse on rightmost point while engine is running
      markPoint:
        engineIsRunning.value && lastPoint
          ? {
              symbol: 'circle',
              symbolSize: 8,
              data: [{ coord: [lastPoint.x, lastPoint.y] }],
              itemStyle: { color: secondColor },
              animation: true,
              animationDuration: 1200,
              animationEasingUpdate: 'sinusoidalInOut',
            }
          : undefined,
    });
  }

  // Ghost series: inactive mode's data (faint dashed, no interaction)
  if (ghostSeriesData.length > 0 && ghostSeriesData.some((d) => d.y > 0)) {
    series.push({
      name: ghostName + ' (historical)',
      type: 'line',
      smooth: true,
      symbol: 'none',
      lineStyle: { color: ghostColor, width: 1, type: 'dashed', opacity: 0.15 },
      areaStyle: undefined,
      emphasis: { disabled: true },
      silent: true,
      data: ghostSeriesData.map((d) => [d.x, d.y]),
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
        label: {
          formatter: (p: { value: number }) => new Date(p.value).toLocaleString(),
        },
      },
    },
    yAxis: {
      type: 'value',
      show: false,
      minInterval: 1,
      axisPointer: {
        label: { formatter: (p: { value: number }) => Math.round(p.value).toString() },
      },
    },
    tooltip: {
      trigger: 'axis',
      axisPointer: {
        type: 'cross',
        crossStyle: { color: chart1Color.value, opacity: 0.3 },
      },
      ...tooltipConfig(),
      formatter: (
        params: Array<{ seriesName: string; value: [number, number]; marker: string }>,
      ) => {
        if (!params.length) return '';
        const ts = new Date(params[0]!.value[0]).toLocaleString();
        let html = `<div style="font-weight:600">${ts}</div>`;
        for (const p of params) {
          // Skip ghost series in tooltip
          if (p.seriesName.includes('(historical)')) continue;
          html += `<div>${p.marker} ${p.seriesName}: <b>${Math.round(p.value[1])}</b></div>`;
        }
        if (isDryRun) {
          html += `<div style="opacity:0.6;font-size:11px;margin-top:2px">dry-run — no deletions</div>`;
        }
        return html;
      },
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

// Duration sparkline ECharts option — uses successColor (green) as the base
// with a visualMap gradient from green → amber → red for
// low → medium → high duration values.
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
      inRange: { color: [successColor.value, chart3Color.value, destructiveColor.value] },
    },
  ],
  series: [
    {
      name: 'Duration',
      type: 'line',
      smooth: true,
      symbol: 'none',
      lineStyle: glowLineStyle(successColor.value),
      areaStyle: gradientArea(successColor.value),
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
      candidates: number;
      queued: number;
      deleted: number;
      freedBytes: number;
      durationMs: number;
      executionMode: string;
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
