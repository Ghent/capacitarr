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
            <template v-if="engineLastRunEpoch">
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
              effectiveMode === MODE_AUTO
                ? 'destructive'
                : effectiveMode === MODE_APPROVAL
                  ? 'outline'
                  : 'secondary'
            "
            class="ml-auto"
          >
            {{ engineModeLabel(effectiveMode) }}
          </UiBadge>
          <span class="text-xs text-muted-foreground">
            {{ $t('dashboard.evaluated') }} {{ engineLastRunEvaluated?.toLocaleString() ?? 0 }} ·
            {{ $t('dashboard.candidates') }} {{ engineLastRunCandidates?.toLocaleString() ?? 0 }}
          </span>
        </div>

        <!-- Sparkline: candidates + would-delete + deleted per engine run -->
        <div v-if="engineHistoryData.length > 0" class="mb-3">
          <div class="flex items-center gap-3 mb-1">
            <span class="text-[11px] text-muted-foreground/70">
              {{ $t('dashboard.engineActivityTitle') }} · {{ dateRangeLabel }}
            </span>
            <span class="inline-flex items-center gap-1 text-[11px] text-muted-foreground">
              <span class="w-2 h-2 rounded-full bg-primary" />
              {{ $t('dashboard.candidates') }}
            </span>
            <span
              class="inline-flex items-center gap-1 text-[11px] text-muted-foreground"
              :class="{ 'opacity-40': !hasQueuedData }"
            >
              <span class="w-2 h-2 rounded-full bg-amber-500" />
              {{ $t('dashboard.wouldDelete') }}
            </span>
            <span
              class="inline-flex items-center gap-1 text-[11px] text-muted-foreground"
              :class="{ 'opacity-40': !hasDeletedData }"
            >
              <span class="w-2 h-2 rounded-full bg-destructive" />
              {{ $t('dashboard.deleted') }}
            </span>
          </div>
          <ClientOnly>
            <!-- Explicit height wrapper: vue-echarts v8 sets height:100% on the
                 <x-vue-echarts> custom element via adoptedStyleSheets, which
                 overrides Tailwind utility classes. The wrapper provides the
                 fixed reference height that 100% resolves against. -->
            <div class="h-[120px] w-full">
              <VChart :option="sparklineEChartsOption" :autoresize="true" class="h-full w-full" />
            </div>
          </ClientOnly>
        </div>

        <!-- Toggle for mini sparklines -->
        <UiButton
          v-if="engineHistoryData.length > 0"
          variant="ghost"
          class="h-auto p-0 flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors mb-2"
          @click="showMiniSparklines = !showMiniSparklines"
        >
          <component
            :is="showMiniSparklines ? ChevronUpIcon : ChevronDownIcon"
            class="w-3.5 h-3.5"
          />
          {{ showMiniSparklines ? $t('dashboard.hideDetails') : $t('dashboard.showDetails') }}
        </UiButton>

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
              <div class="h-[70px] w-full">
                <VChart
                  :option="durationSparklineEChartsOption"
                  :autoresize="true"
                  class="h-full w-full"
                />
              </div>
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
              {{ anyAutoMode ? $t('dashboard.freed') : $t('dashboard.wouldFree') }}
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
              <template v-else-if="allDryRun">
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
    <DeletionQueueCard :effective-mode="effectiveMode" />

    <!-- Snoozed Items (visible in all modes when snoozed items exist) -->
    <SnoozedItemsCard />

    <!-- Sunset Queue (visible when any disk group is in sunset mode or sunset items exist) -->
    <SunsetQueueCard :has-sunset-mode="diskGroups.some((g) => g.mode === 'sunset')" />

    <!-- Approval Queue (visible when any disk group is in approval mode or queue has items) -->
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
  MODE_SUNSET,
  EVENT_DELETION_SUCCESS,
  EVENT_DELETION_DRY_RUN,
  EVENT_DELETION_FAILED,
  EVENT_DELETION_QUEUED,
  EVENT_DELETION_PROGRESS,
  EVENT_DELETION_BATCH_COMPLETE,
  EVENT_APPROVAL_APPROVED,
  EVENT_APPROVAL_REJECTED,
  EVENT_APPROVAL_UNSNOOZED,
  EVENT_APPROVAL_BULK_UNSNOOZED,
  EVENT_APPROVAL_ORPHANS_RECOVERED,
  EVENT_APPROVAL_RETURNED_TO_PENDING,
  EVENT_INTEGRATION_ADDED,
  EVENT_INTEGRATION_UPDATED,
  EVENT_INTEGRATION_REMOVED,
  EVENT_INTEGRATION_RECOVERED,
  EVENT_INTEGRATION_RECOVERY_ATTEMPT,
  EVENT_SETTINGS_CHANGED,
  EVENT_SETTINGS_IMPORTED,
  EVENT_DATA_RESET,
} from '~/constants';
import { STORAGE_KEYS } from '~/utils/storageKeys';

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
const { on: sseOn } = useEventStream();

// Approval queue — visible when any disk group is in approval mode, or when
// the queue already contains items (e.g. user switched away from approval mode
// while items were still queued).
const { hasQueueItems, fetchQueue: fetchApprovalQueue } = useApprovalQueue();
const approvalQueueVisible = computed(
  () => diskGroups.value.some((g) => g.mode === 'approval') || hasQueueItems.value,
);

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

// ---------------------------------------------------------------------------
// Per-disk-group mode helpers
// ---------------------------------------------------------------------------
// Since v3.0, execution mode is per-disk-group. These computeds derive
// dashboard-level indicators from the actual disk group modes instead of
// the global defaultDiskGroupMode preference.

/** Set of unique modes across all disk groups. */
const activeModes = computed(() => new Set(diskGroups.value.map((g) => g.mode)));

/**
 * Effective mode for dashboard indicators — the "most aggressive" mode
 * across all disk groups. Priority: auto > approval > sunset > dry-run.
 * Falls back to engineExecutionMode (global default) when no disk groups exist.
 */
const effectiveMode = computed(() => {
  const modes = activeModes.value;
  if (modes.has(MODE_AUTO)) return MODE_AUTO;
  if (modes.has(MODE_APPROVAL)) return MODE_APPROVAL;
  if (modes.has(MODE_SUNSET)) return MODE_SUNSET;
  if (modes.has(MODE_DRY_RUN)) return MODE_DRY_RUN;
  return engineExecutionMode.value;
});

/** True only when ALL disk groups are in dry-run (or no groups exist and global default is dry-run). */
const allDryRun = computed(() =>
  diskGroups.value.length === 0
    ? engineExecutionMode.value === MODE_DRY_RUN
    : diskGroups.value.every((g) => g.mode === MODE_DRY_RUN),
);

/** True when any disk group is in auto mode (actual deletions can happen). */
const anyAutoMode = computed(() => diskGroups.value.some((g) => g.mode === MODE_AUTO));
const engineHistoryData = ref<
  Array<{
    timestamp: string;
    evaluated: number;
    candidates: number;
    queued: number;
    deleted: number;
    freedBytes: number;
    durationMs: number;
    diskGroupModes: string;
  }>
>([]);
const showMiniSparklines = ref(
  import.meta.client ? localStorage.getItem(STORAGE_KEYS.sparklines) !== 'false' : true,
);
watch(showMiniSparklines, (val) => {
  if (import.meta.client) {
    localStorage.setItem(STORAGE_KEYS.sparklines, String(val));
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
    case 'manual_run_triggered':
      return PlayIcon;
    // Settings
    case 'settings_changed':
    case 'settings_imported':
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
    case 'integration_recovered':
    case 'integration_recovery_attempt':
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
    case 'approval_returned_to_pending':
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
    case 'integration_recovered':
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
    case 'approval_returned_to_pending':
    case 'integration_recovery_attempt':
      return 'text-warning';
    case 'rule_updated':
    case 'password_changed':
    case 'username_changed':
    case 'api_key_generated':
    case 'login':
    case 'settings_changed':
    case 'settings_imported':
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
  'integration_recovered',
  'integration_recovery_attempt',
  'approval_approved',
  'approval_rejected',
  'approval_unsnoozed',
  'approval_bulk_unsnoozed',
  'approval_orphans_recovered',
  'approval_returned_to_pending',
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
  'settings_imported',
  'server_started',
] as const;

// Handler refs for activity events — the auto-cleanup scope handles
// unsubscription, but we keep the map to hold stable references to
// the closures created by handleActivityEvent().
const _activityHandlers = new Map<string, (data: unknown) => void>();

// Handler: deletion_progress SSE event — patch last sparkline data point in real-time.
// The deletion_progress event only fires during actual deletions (auto/approval
// groups), so we always patch the "deleted" field. The "queued" count for dry-run
// groups is finalized by UpdateRunStats() at engine run completion and arrives
// via the engine_run_complete SSE event.
function handleDeletionProgressSparkline(data: unknown) {
  const event = data as DeletionProgress;
  const history = engineHistoryData.value;
  const last = history.length > 0 ? history[history.length - 1] : undefined;
  if (last) {
    engineHistoryData.value = [...history.slice(0, -1), { ...last, deleted: event.succeeded }];
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

// Handler: data reset — full dashboard refresh since all scraped data was wiped
function handleDataReset() {
  fetchDashboardData(true);
  fetchEngineHistory();
  fetchRecentActivity();
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

  // Subscribe to all activity event types for the real-time feed.
  // The { onUnmounted } scope auto-cleans up handlers when the component unmounts.
  const scope = { onUnmounted };
  for (const eventType of activityEventTypes) {
    const handler = handleActivityEvent(eventType);
    _activityHandlers.set(eventType, handler);
    sseOn(eventType, handler, scope);
  }

  // Subscribe to approval-related events to refresh the queue
  sseOn(EVENT_APPROVAL_APPROVED, handleApprovalChange, scope);
  sseOn(EVENT_APPROVAL_REJECTED, handleApprovalChange, scope);
  sseOn(EVENT_APPROVAL_UNSNOOZED, handleApprovalChange, scope);
  sseOn(EVENT_APPROVAL_BULK_UNSNOOZED, handleApprovalChange, scope);
  sseOn(EVENT_APPROVAL_ORPHANS_RECOVERED, handleApprovalChange, scope);
  sseOn(EVENT_APPROVAL_RETURNED_TO_PENDING, handleApprovalChange, scope);
  sseOn(EVENT_DELETION_SUCCESS, handleApprovalChange, scope);

  // When a deletion completes, patch the most recent sparkline data point in real-time
  sseOn(EVENT_DELETION_PROGRESS, handleDeletionProgressSparkline, scope);

  // When all deletions for a cycle finish, refresh dashboard stats — the numbers are now final
  sseOn(EVENT_DELETION_BATCH_COMPLETE, handleDeletionBatchCompleteRefresh, scope);

  // SSE-driven data refresh: integration and settings changes
  sseOn(EVENT_INTEGRATION_ADDED, handleIntegrationChange, scope);
  sseOn(EVENT_INTEGRATION_UPDATED, handleIntegrationChange, scope);
  sseOn(EVENT_INTEGRATION_REMOVED, handleIntegrationChange, scope);
  sseOn(EVENT_INTEGRATION_RECOVERED, handleIntegrationChange, scope);
  sseOn(EVENT_INTEGRATION_RECOVERY_ATTEMPT, handleIntegrationChange, scope);
  sseOn(EVENT_SETTINGS_CHANGED, handleSettingsChange, scope);
  sseOn(EVENT_SETTINGS_IMPORTED, handleDataReset, scope); // Full refresh — import may change everything
  sseOn(EVENT_DATA_RESET, handleDataReset, scope);
});

async function fetchDashboardData(silent = false) {
  if (!silent) loading.value = true;
  try {
    const [groups, integrations] = await Promise.all([
      api('/api/v1/disk-groups'),
      api('/api/v1/integrations'),
    ]);
    // Fetch engine stats via the shared composable (handles toast on completion).
    await engineFetchStats();
    // Assign disk groups before fetching the approval queue so
    // approvalQueueVisible (which checks diskGroups) is accurate.
    diskGroups.value = groups as DiskGroup[];
    allIntegrations.value = integrations as IntegrationConfig[];
    // Fetch approval queue (non-blocking)
    fetchApprovalQueue();
    // Note: fetchEngineHistory() and fetchRecentActivity() are NOT called here.
    // They are fetched once on mount and updated via SSE events to avoid
    // replacing the data array (which causes ECharts to replay animations).
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
// --- ECharts sparkline options ---

// Helper computeds: whether each action series has non-zero data in the current range.
// Used by the legend to dim labels when a series carries no data.
const hasQueuedData = computed(() => queuedSeries.value.some((d) => d.y > 0));
const hasDeletedData = computed(() => deletedSeries.value.some((d) => d.y > 0));

const sparklineEChartsOption = computed(() => {
  const candidates = candidatesSeries.value;
  const qData = queuedSeries.value;
  const dData = deletedSeries.value;
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

  // "Would Delete" series (amber) — always rendered from queued field.
  // With per-disk-group modes, dry-run groups contribute to queued while
  // auto/approval groups contribute to deleted. Both can be non-zero in
  // the same engine run, so both series are always present.
  if (qData.length > 0) {
    series.push({
      name: t('dashboard.wouldDelete'),
      type: 'line',
      smooth: true,
      symbol: sparseSymbol(qData.length),
      symbolSize: sparseSymbolSize(qData.length),
      itemStyle: { color: chart3Color.value },
      lineStyle: glowLineStyle(chart3Color.value),
      areaStyle: gradientArea(chart3Color.value),
      emphasis: emphasisConfig(),
      data: qData.map((d) => [d.x, d.y]),
    });
  }

  // "Deleted" series (red) — always rendered from deleted field.
  // Includes animated pulse on rightmost point while engine is running,
  // since real-time deletion progress only applies to actual deletions.
  if (dData.length > 0) {
    const lastPoint = dData[dData.length - 1];
    series.push({
      name: t('dashboard.deleted'),
      type: 'line',
      smooth: true,
      symbol: sparseSymbol(dData.length),
      symbolSize: sparseSymbolSize(dData.length),
      itemStyle: { color: destructiveColor.value },
      lineStyle: glowLineStyle(destructiveColor.value),
      areaStyle: gradientArea(destructiveColor.value),
      emphasis: emphasisConfig(),
      data: dData.map((d) => [d.x, d.y]),
      markPoint:
        engineIsRunning.value && lastPoint
          ? {
              symbol: 'circle',
              symbolSize: 8,
              data: [{ coord: [lastPoint.x, lastPoint.y] }],
              itemStyle: { color: destructiveColor.value },
              animation: true,
              animationDuration: 1200,
              animationEasingUpdate: 'sinusoidalInOut',
            }
          : undefined,
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
        let pointHasQueued = false;
        let pointHasDeleted = false;
        for (const p of params) {
          if (p.value[1] === 0) continue; // skip zero-value series
          html += `<div>${p.marker} ${p.seriesName}: <b>${Math.round(p.value[1])}</b></div>`;
          if (p.seriesName === t('dashboard.wouldDelete')) pointHasQueued = true;
          if (p.seriesName === t('dashboard.deleted')) pointHasDeleted = true;
        }
        if (pointHasQueued && pointHasDeleted) {
          html += `<div style="opacity:0.6;font-size:11px;margin-top:2px">mixed modes</div>`;
        } else if (pointHasQueued) {
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
      diskGroupModes: string;
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
