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
      class="mb-8 flex flex-col md:flex-row md:items-center justify-between gap-4"
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
        <UiSelect v-model="chartMode">
          <UiSelectTrigger class="h-9 w-[130px]">
            <UiSelectValue placeholder="Chart mode" />
          </UiSelectTrigger>
          <UiSelectContent>
            <UiSelectItem v-for="opt in chartModeOptions" :key="opt.value" :value="opt.value">
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

    <!-- Engine Activity (prominent, first card) -->
    <UiCard
      v-if="engineStats"
      v-motion
      :initial="{ opacity: 0, y: 12 }"
      :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24 } }"
      class="mb-8"
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
            <apexchart
              type="area"
              height="120"
              :options="sparklineOptions"
              :series="sparklineSeries"
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

        <!-- Mini sparklines: duration + freed bytes -->
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
              <apexchart
                type="area"
                height="70"
                :options="durationSparklineOptions"
                :series="durationSparklineSeries"
              />
            </ClientOnly>
          </div>

          <!-- Recent Activity -->
          <div class="rounded-lg bg-muted px-3 py-2 flex flex-col">
            <div class="text-[11px] text-muted-foreground mb-1 flex items-center gap-1">
              <span>{{ $t('dashboard.recentActivity') }}</span>
              <span class="text-muted-foreground/40">·</span>
              <NuxtLink to="/audit" class="text-primary hover:text-primary/80 font-medium">
                {{ $t('dashboard.viewAll') }}
              </NuxtLink>
            </div>
            <UiScrollArea
              v-if="recentActivity.length > 0"
              class="flex-1 pr-3"
              style="height: 160px"
            >
              <div
                v-for="entry in recentActivity"
                :key="entry.id"
                class="flex items-center gap-1.5 py-0.5 text-[11px] leading-tight"
              >
                <component
                  :is="eventIcon(entry.eventType)"
                  class="w-3 h-3 shrink-0"
                  :class="eventIconClass(entry.eventType)"
                />
                <span class="truncate line-clamp-1 flex-1 min-w-0 text-foreground">
                  {{ entry.message }}
                </span>
                <span class="text-muted-foreground/70 shrink-0 whitespace-nowrap ml-auto">
                  <DateDisplay :date="entry.createdAt" />
                </span>
              </div>
            </UiScrollArea>
            <div
              v-else
              class="flex-1 flex items-center justify-center text-[11px] text-muted-foreground/60"
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

    <!-- Per-Disk-Group Sections -->
    <div v-if="diskGroups.length > 0" class="space-y-6 mb-8">
      <DiskGroupSection
        v-for="group in diskGroups"
        :key="group.id"
        :group="group"
        :chart-mode="chartMode"
        :date-range="dateRange"
        :date-range-label="dateRangeLabel"
        :refresh-key="refreshKey"
        @updated="fetchDashboardData"
      />
    </div>

    <!-- Summary Cards (informational, at the bottom) -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-5 mb-8" data-stagger>
      <!-- Total Storage -->
      <UiCard
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: { type: 'spring', stiffness: 260, damping: 24, delay: 100 },
        }"
        data-slot="stat-card"
      >
        <UiCardContent class="pt-5">
          <div class="flex items-center gap-3 font-medium text-sm mb-3">
            <div data-slot="stat-icon">
              <component :is="ServerIcon" class="w-4 h-4" />
            </div>
            <span class="text-primary">{{ $t('dashboard.totalStorage') }}</span>
          </div>
          <div class="text-3xl font-bold tabular-nums">
            {{ formatBytes(totalCapacity) }}
          </div>
          <p class="text-sm text-muted-foreground mt-1">
            {{ $t('dashboard.diskGroups', { count: diskGroups.length }, diskGroups.length) }}
          </p>
        </UiCardContent>
      </UiCard>

      <!-- Used Capacity -->
      <UiCard
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: { type: 'spring', stiffness: 260, damping: 24, delay: 160 },
        }"
        data-slot="stat-card"
      >
        <UiCardContent class="pt-5">
          <div class="flex items-center gap-3 font-medium text-sm mb-3">
            <div data-slot="stat-icon">
              <component :is="ChartPieIcon" class="w-4 h-4" />
            </div>
            <span class="text-primary">{{ $t('dashboard.usedCapacity') }}</span>
          </div>
          <div class="text-3xl font-bold tabular-nums">
            {{ formatBytes(totalUsed) }}
          </div>
          <p class="text-sm text-muted-foreground mt-1">
            {{
              $t('dashboard.utilization', {
                pct: totalCapacity > 0 ? Math.round((totalUsed / totalCapacity) * 100) : 0,
              })
            }}
          </p>
        </UiCardContent>
      </UiCard>

      <!-- Integrations -->
      <UiCard
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: { type: 'spring', stiffness: 260, damping: 24, delay: 220 },
        }"
        data-slot="stat-card"
      >
        <UiCardContent class="pt-5">
          <div class="flex items-center gap-3 font-medium text-sm mb-3">
            <div data-slot="stat-icon">
              <component :is="HardDriveIcon" class="w-4 h-4" />
            </div>
            <span class="text-primary">{{ $t('dashboard.integrations') }}</span>
          </div>
          <div class="text-3xl font-bold tabular-nums">
            {{ enabledIntegrations.length }}
          </div>
          <p class="text-sm text-muted-foreground mt-1">
            {{
              $t('dashboard.syncedRecently', {
                count: enabledIntegrations.filter((i) => i.lastSync).length,
              })
            }}
          </p>
        </UiCardContent>
      </UiCard>
    </div>

    <!-- Lifetime Stats Cards (Row 2) -->
    <div class="grid grid-cols-1 md:grid-cols-3 gap-5 mb-8" data-stagger>
      <!-- Total Space Reclaimed -->
      <UiCard
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: { type: 'spring', stiffness: 260, damping: 24, delay: 280 },
        }"
        data-slot="stat-card"
      >
        <UiCardContent class="pt-5">
          <div class="flex items-center gap-3 font-medium text-sm mb-3">
            <div data-slot="stat-icon">
              <component :is="Trash2Icon" class="w-4 h-4" />
            </div>
            <span class="text-primary">{{ $t('dashboard.spaceReclaimed') }}</span>
          </div>
          <div class="text-3xl font-bold tabular-nums">
            {{ formatBytes(dashboardStats?.totalBytesReclaimed ?? 0) }}
          </div>
          <p class="text-sm text-muted-foreground mt-1">
            {{ $t('dashboard.itemsRemoved', { count: dashboardStats?.totalItemsRemoved ?? 0 }) }}
          </p>
        </UiCardContent>
      </UiCard>

      <!-- Protected Items -->
      <UiCard
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: { type: 'spring', stiffness: 260, damping: 24, delay: 340 },
        }"
        data-slot="stat-card"
      >
        <UiCardContent class="pt-5">
          <div class="flex items-center gap-3 font-medium text-sm mb-3">
            <div data-slot="stat-icon">
              <component :is="ShieldCheckIcon" class="w-4 h-4" />
            </div>
            <span class="text-primary">{{ $t('dashboard.protectedItems') }}</span>
          </div>
          <div class="text-3xl font-bold tabular-nums">
            {{ dashboardStats?.protectedCount ?? 0 }}
          </div>
          <p class="text-sm text-muted-foreground mt-1">
            {{ $t('dashboard.protectedByRules') }}
          </p>
        </UiCardContent>
      </UiCard>

      <!-- Library Growth Rate -->
      <UiCard
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: { type: 'spring', stiffness: 260, damping: 24, delay: 400 },
        }"
        data-slot="stat-card"
      >
        <UiCardContent class="pt-5">
          <div class="flex items-center gap-3 font-medium text-sm mb-3">
            <div data-slot="stat-icon">
              <component :is="TrendingUpIcon" class="w-4 h-4" />
            </div>
            <span class="text-primary">{{ $t('dashboard.growthRate') }}</span>
          </div>
          <div class="text-3xl font-bold tabular-nums">
            {{ formattedGrowthRate }}
          </div>
          <p class="text-sm text-muted-foreground mt-1">
            {{
              dashboardStats?.hasGrowthData
                ? $t('dashboard.overLastWeek')
                : $t('dashboard.notEnoughData')
            }}
          </p>
        </UiCardContent>
      </UiCard>
    </div>

    <!-- Empty State -->
    <div
      v-if="!engineStats && !loading"
      v-motion
      :initial="{ opacity: 0, y: 8 }"
      :enter="{ opacity: 1, y: 0 }"
      class="rounded-xl border-2 border-dashed border-border p-12 text-center mb-8"
    >
      <component :is="HardDriveIcon" class="w-12 h-12 text-muted-foreground/40 mx-auto mb-4" />
      <h3 class="text-muted-foreground font-medium mb-1.5">
        {{ $t('dashboard.noDiskGroups') }}
      </h3>
      <p class="text-sm text-muted-foreground/70 mb-4 max-w-md mx-auto">
        {{ $t('dashboard.noDiskGroupsHelp') }}
      </p>
    </div>

    <!-- Skeleton Loading State -->
    <template v-if="loading">
      <div class="grid grid-cols-1 md:grid-cols-3 gap-5 mb-8">
        <UiCard v-for="i in 3" :key="i" class="animate-pulse">
          <UiCardContent class="pt-5">
            <div class="flex items-center gap-2 mb-3">
              <div class="w-4 h-4 rounded bg-muted" />
              <div class="h-3 w-24 rounded bg-muted" />
            </div>
            <div class="h-8 w-28 rounded bg-muted mb-2" />
            <div class="h-3 w-32 rounded bg-muted/50" />
          </UiCardContent>
        </UiCard>
      </div>
      <SkeletonCard :show-chart="true" />
    </template>
  </div>
</template>

<script setup lang="ts">
import {
  ServerIcon,
  ChartPieIcon,
  HardDriveIcon,
  LoaderCircleIcon,
  RefreshCwIcon,
  ActivityIcon,
  PlayIcon,
  CheckCircle2Icon,
  Trash2Icon,
  ShieldCheckIcon,
  TrendingUpIcon,
  ChevronDownIcon,
  ChevronUpIcon,
  SettingsIcon,
  UserIcon,
  PlugIcon,
  AlertCircleIcon,
  PauseIcon,
  XCircleIcon,
  PlusCircleIcon,
  PencilIcon,
  PowerIcon,
  KeyIcon,
} from 'lucide-vue-next';
import { formatBytes } from '~/utils/format';
import type {
  ActivityEvent,
  DiskGroup,
  IntegrationConfig,
  DashboardStats,
  SparklineTooltipOpts,
} from '~/types/api';

const { t } = useI18n();
const api = useApi();
const { primaryColor, destructiveColor, successColor } = useThemeColors();

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

// Approval queue (shown when execution mode is "approval")
const { isApprovalMode, fetchQueue: fetchApprovalQueue } = useApprovalQueue();
const approvalQueueVisible = computed(() => isApprovalMode.value);

// Pull-to-refresh for touch devices
const { isRefreshing, pullProgress, pullDistance } = usePullToRefresh(async () => {
  await fetchDashboardData(true);
  refreshKey.value++;
});

const chartModeOptions = [
  { label: 'Percentage', value: 'percentage' },
  { label: 'Raw (GB)', value: 'raw' },
];

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

const chartMode = ref('percentage');
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
const dashboardStats = ref<DashboardStats | null>(null);
const recentActivity = ref<ActivityEvent[]>([]);
const loading = ref(true);
const lastUpdated = ref<Date | null>(null);
const isAutoRefreshing = ref(false);
const refreshKey = ref(0);

// Icon component for activity events
function eventIcon(eventType: string) {
  switch (eventType) {
    case 'engine_start':
      return PlayIcon;
    case 'engine_complete':
      return CheckCircle2Icon;
    case 'engine_error':
      return AlertCircleIcon;
    case 'engine_paused':
      return PauseIcon;
    case 'engine_resumed':
      return PlayIcon;
    case 'engine_mode_changed':
      return SettingsIcon;
    case 'approval_approved':
      return CheckCircle2Icon;
    case 'approval_rejected':
      return XCircleIcon;
    case 'rule_created':
      return PlusCircleIcon;
    case 'rule_updated':
      return PencilIcon;
    case 'rule_deleted':
      return Trash2Icon;
    case 'integration_test_failed':
      return AlertCircleIcon;
    case 'server_started':
      return PowerIcon;
    case 'password_changed':
      return KeyIcon;
    case 'login':
      return UserIcon;
    case 'settings_changed':
      return SettingsIcon;
    case 'integration_added':
      return PlugIcon;
    case 'integration_removed':
      return PlugIcon;
    case 'integration_test':
      return PlugIcon;
    default:
      return ActivityIcon;
  }
}

// Color class for activity event icons
function eventIconClass(eventType: string): string {
  switch (eventType) {
    case 'engine_start':
    case 'engine_mode_changed':
    case 'rule_created':
      return 'text-primary';
    case 'engine_complete':
    case 'engine_resumed':
    case 'approval_approved':
    case 'server_started':
    case 'integration_added':
    case 'integration_test':
      return 'text-success';
    case 'engine_error':
    case 'approval_rejected':
    case 'rule_deleted':
    case 'integration_test_failed':
    case 'integration_removed':
      return 'text-destructive';
    case 'engine_paused':
      return 'text-warning';
    case 'rule_updated':
    case 'password_changed':
    case 'login':
    case 'settings_changed':
      return 'text-muted-foreground';
    default:
      return 'text-muted-foreground';
  }
}

// Engine stats — alias from shared composable
const engineStats = computed(() => engineControlStats.value);

const enabledIntegrations = computed(() => allIntegrations.value.filter((i) => i.enabled));

const dateRangeLabel = computed(() => {
  const match = dateRangeOptions.find((o) => o.value === dateRange.value);
  return match?.label ?? dateRange.value;
});

const totalCapacity = computed(() =>
  diskGroups.value.reduce((sum, g) => sum + (g.totalBytes || 0), 0),
);

const totalUsed = computed(() => diskGroups.value.reduce((sum, g) => sum + (g.usedBytes || 0), 0));

const formattedGrowthRate = computed(() => {
  if (!dashboardStats.value?.hasGrowthData) return '—';
  const bytes = dashboardStats.value.growthBytesPerWeek;
  const prefix = bytes >= 0 ? '+' : '';
  return `${prefix}${formatBytes(Math.abs(bytes))} / week`;
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

// --- Auto refresh ---
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

// When the engine finishes a run (detected via navbar or dashboard Run Now, or scheduled),
// immediately refresh all dashboard data so the UI reflects the latest state.
watch(engineRunCompletionCounter, () => {
  fetchDashboardData(true);
  refreshKey.value++;
});

onMounted(async () => {
  await fetchDashboardData();
  startAutoRefresh();
});

onUnmounted(() => {
  stopAutoRefresh();
});

async function fetchDashboardData(silent = false) {
  if (!silent) loading.value = true;
  try {
    const [groups, integrations, dStats] = await Promise.all([
      api('/api/v1/disk-groups'),
      api('/api/v1/integrations'),
      api('/api/v1/dashboard-stats').catch(() => null),
    ]);
    // Fetch engine stats via the shared composable (handles toast on completion)
    engineFetchStats();
    // Fetch approval queue (non-blocking, only runs in approval mode)
    fetchApprovalQueue();
    // Fetch sparkline engine history data in parallel (non-blocking)
    fetchEngineHistory();
    // Fetch recent activity for the mini feed (non-blocking)
    fetchRecentActivity();
    diskGroups.value = groups as DiskGroup[];
    allIntegrations.value = integrations as IntegrationConfig[];
    if (dStats) dashboardStats.value = dStats as DashboardStats;
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

const sparklineSeries = computed(() => {
  const series = [];
  if (flaggedSeries.value.length > 0) {
    series.push({ name: 'Flagged', data: flaggedSeries.value });
  }
  if (deletedSeries.value.length > 0) {
    series.push({ name: 'Deleted', data: deletedSeries.value });
  }
  return series;
});

const sparklineOptions = computed(() => ({
  chart: {
    type: 'area' as const,
    sparkline: { enabled: true },
    animations: { enabled: true, easing: 'easeinout', speed: 400 },
  },
  stroke: { curve: 'smooth' as const, width: 2 },
  colors: [primaryColor.value, destructiveColor.value],
  fill: {
    type: 'gradient',
    gradient: {
      shadeIntensity: 1,
      opacityFrom: 0.45,
      opacityTo: 0.05,
      stops: [0, 100],
    },
  },
  tooltip: {
    enabled: true,
    shared: true,
    x: { show: true, format: 'MMM dd, yyyy HH:mm' },
    y: {
      formatter: (val: number, opts: SparklineTooltipOpts) => {
        const label = opts?.seriesIndex === 1 ? 'deleted' : 'flagged';
        return `${val} ${label}`;
      },
    },
    theme: 'dark',
  },
  xaxis: { type: 'datetime' as const, labels: { datetimeUTC: false } },
}));

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

const durationSparklineSeries = computed(() => [
  {
    name: 'Duration',
    data: engineHistoryData.value.map((p) => ({
      x: new Date(p.timestamp).getTime(),
      y: p.durationMs,
    })),
  },
]);

// Base sparkline chart options shared by both mini sparklines
function miniSparklineBase(color: string) {
  return {
    chart: {
      type: 'area' as const,
      sparkline: { enabled: true },
      animations: { enabled: true, easing: 'easeinout', speed: 400 },
    },
    stroke: { curve: 'smooth' as const, width: 2 },
    colors: [color],
    fill: {
      type: 'gradient',
      gradient: {
        shadeIntensity: 1,
        opacityFrom: 0.45,
        opacityTo: 0.05,
        stops: [0, 100],
      },
    },
    xaxis: { type: 'datetime' as const, labels: { datetimeUTC: false } },
  };
}

// Duration sparkline: tooltip shows milliseconds
const durationSparklineOptions = computed(() => ({
  ...miniSparklineBase(successColor.value),
  tooltip: {
    enabled: true,
    x: { show: true, format: 'MMM dd, yyyy HH:mm' },
    y: { formatter: (val: number) => `${val}ms` },
    theme: 'dark',
  },
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
    const data = (await api('/api/v1/activity/recent?limit=10')) as ActivityEvent[];
    recentActivity.value = data || [];
  } catch (err) {
    console.warn('[Dashboard] fetchRecentActivity failed:', err);
  }
}
</script>
