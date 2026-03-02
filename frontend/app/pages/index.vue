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
          Dashboard
        </h1>
        <p class="text-muted-foreground mt-1.5">
          Capacity overview across your media storage.
          <span
            v-if="lastUpdated"
            class="inline-flex items-center gap-1 ml-2 text-xs text-muted-foreground/70"
          >
            <component
              :is="RefreshCwIcon"
              class="w-3 h-3"
              :class="{ 'animate-spin': isAutoRefreshing }"
            />
            Updated {{ formatRelativeTime(lastUpdated.toISOString()) }}
          </span>
        </p>
      </div>
      <div class="flex items-center gap-2">
        <UiSelect v-model="dateRange">
          <UiSelectTrigger class="h-9 w-[130px]">
            <UiSelectValue placeholder="Time range" />
          </UiSelectTrigger>
          <UiSelectContent>
            <UiSelectItem
              v-for="opt in dateRangeOptions"
              :key="opt.value"
              :value="opt.value"
            >
              {{ opt.label }}
            </UiSelectItem>
          </UiSelectContent>
        </UiSelect>
        <UiSelect v-model="chartMode">
          <UiSelectTrigger class="h-9 w-[130px]">
            <UiSelectValue placeholder="Chart mode" />
          </UiSelectTrigger>
          <UiSelectContent>
            <UiSelectItem
              v-for="opt in chartModeOptions"
              :key="opt.value"
              :value="opt.value"
            >
              {{ opt.label }}
            </UiSelectItem>
          </UiSelectContent>
        </UiSelect>
        <UiSelect v-model="refreshIntervalStr">
          <UiSelectTrigger class="h-9 w-[110px]">
            <UiSelectValue placeholder="Refresh" />
          </UiSelectTrigger>
          <UiSelectContent>
            <UiSelectItem
              v-for="opt in refreshOptions"
              :key="opt.value"
              :value="String(opt.value)"
            >
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
          class="rounded-lg px-3 py-2 mb-4 flex items-center gap-2 text-sm font-medium"
          :class="engineStatusBannerClass"
        >
          <LoaderCircleIcon
            v-if="engineIsRunning"
            class="w-4 h-4 animate-spin shrink-0"
          />
          <component
            :is="engineIsRunning ? ActivityIcon : CheckCircle2Icon"
            v-else
            class="w-4 h-4 shrink-0"
          />
          <span>{{ engineStatusText }}</span>
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
            <component
              :is="ActivityIcon"
              class="w-4 h-4"
            />
            Engine Activity
          </div>
          <UiButton
            variant="outline"
            size="sm"
            :disabled="engineRunNowLoading"
            @click="engineTriggerRunNow"
          >
            <LoaderCircleIcon
              v-if="engineRunNowLoading"
              class="w-3.5 h-3.5 animate-spin"
            />
            <PlayIcon
              v-else
              class="w-3.5 h-3.5"
            />
            Run Now
          </UiButton>
          <span class="text-xs text-muted-foreground">
            <template v-if="engineIsRunning">
              🔄 Engine running…
            </template>
            <template v-else>
              {{ engineLastRunEpoch ? `Last run: ${formatRelativeTime(new Date(engineLastRunEpoch * 1000).toISOString())}` : 'No runs yet' }}
            </template>
          </span>
          <UiBadge
            :variant="engineExecutionMode === 'auto' ? 'destructive' : engineExecutionMode === 'approval' ? 'outline' : 'secondary'"
            class="ml-auto"
          >
            {{ engineModeLabel(engineExecutionMode) }}
          </UiBadge>
          <span class="text-xs text-muted-foreground">
            Evaluated {{ engineLastRunEvaluated?.toLocaleString() ?? 0 }} · Flagged {{ engineLastRunFlagged?.toLocaleString() ?? 0 }}
          </span>
        </div>

        <!-- Sparkline: items flagged + deleted per engine run -->
        <div
          v-if="flaggedSeries.length > 0 || deletedSeries.length > 0"
          class="mb-3"
        >
          <div class="flex items-center gap-3 mb-1">
            <span class="text-[11px] text-muted-foreground/70">
              Engine Activity · {{ dateRangeLabel }}
            </span>
            <span class="inline-flex items-center gap-1 text-[11px] text-muted-foreground">
              <span class="w-2 h-2 rounded-full bg-primary" /> Flagged
            </span>
            <span class="inline-flex items-center gap-1 text-[11px] text-muted-foreground">
              <span class="w-2 h-2 rounded-full bg-destructive" /> Deleted
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

        <!-- Stats row: 3 compact boxes -->
        <div class="grid grid-cols-3 gap-3 mb-3">
          <!-- Would Free / Freed -->
          <div class="rounded-lg bg-muted px-3 py-2">
            <div class="text-[11px] text-muted-foreground mb-0.5">
              {{ engineExecutionMode === 'auto' ? 'Freed' : 'Would free' }}
            </div>
            <div class="text-sm font-bold tabular-nums">
              {{ formatBytes(engineStats.lastRunFreedBytes ?? 0) }}
            </div>
          </div>

          <!-- Queue -->
          <div class="rounded-lg bg-muted px-3 py-2">
            <div class="text-[11px] text-muted-foreground mb-0.5">
              Queue
            </div>
            <div class="flex items-center gap-1.5">
              <span
                class="w-2 h-2 rounded-full shrink-0"
                :class="(engineStats.queueDepth ?? 0) > 0 ? 'bg-warning' : 'bg-success'"
              />
              <span class="text-sm font-bold tabular-nums">{{ engineStats.queueDepth ?? 0 }}</span>
              <span class="text-[11px] text-muted-foreground">items</span>
            </div>
          </div>

          <!-- Active Delete -->
          <div class="rounded-lg bg-muted px-3 py-2">
            <div class="text-[11px] text-muted-foreground mb-0.5">
              Active Delete
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
              <template v-else-if="engineExecutionMode === 'dry_run' || engineExecutionMode === 'dry-run'">
                <span class="text-muted-foreground text-xs">Dry-Run — no deletions</span>
              </template>
              <template v-else-if="(engineStats.queueDepth ?? 0) === 0">
                <span class="text-muted-foreground">Idle</span>
              </template>
              <template v-else>
                <span class="text-muted-foreground">Waiting…</span>
              </template>
            </div>
          </div>
        </div>

        <!-- Footer link -->
        <NuxtLink
          to="/audit"
          class="text-xs text-primary hover:text-primary/80 font-medium transition-colors"
        >
          View full audit log →
        </NuxtLink>
      </UiCardContent>
    </UiCard>

    <!-- Per-Disk-Group Sections -->
    <div
      v-if="diskGroups.length > 0"
      class="space-y-6 mb-8"
    >
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
    <div
      class="grid grid-cols-1 md:grid-cols-3 gap-5 mb-8"
      data-stagger
    >
      <!-- Total Storage -->
      <UiCard
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 100 } }"
        data-slot="stat-card"
      >
        <UiCardContent class="pt-5">
          <div class="flex items-center gap-3 font-medium text-sm mb-3">
            <div data-slot="stat-icon">
              <component
                :is="ServerIcon"
                class="w-4 h-4"
              />
            </div>
            <span class="text-primary">Total Storage</span>
          </div>
          <div class="text-3xl font-bold tabular-nums">
            {{ formatBytes(totalCapacity) }}
          </div>
          <p class="text-sm text-muted-foreground mt-1">
            {{ diskGroups.length }} disk group{{ diskGroups.length !== 1 ? 's' : '' }} mapped
          </p>
        </UiCardContent>
      </UiCard>

      <!-- Used Capacity -->
      <UiCard
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 160 } }"
        data-slot="stat-card"
      >
        <UiCardContent class="pt-5">
          <div class="flex items-center gap-3 font-medium text-sm mb-3">
            <div data-slot="stat-icon">
              <component
                :is="ChartPieIcon"
                class="w-4 h-4"
              />
            </div>
            <span class="text-primary">Used Capacity</span>
          </div>
          <div class="text-3xl font-bold tabular-nums">
            {{ formatBytes(totalUsed) }}
          </div>
          <p class="text-sm text-muted-foreground mt-1">
            {{ totalCapacity > 0 ? Math.round((totalUsed / totalCapacity) * 100) : 0 }}% utilization
          </p>
        </UiCardContent>
      </UiCard>

      <!-- Integrations -->
      <UiCard
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 220 } }"
        data-slot="stat-card"
      >
        <UiCardContent class="pt-5">
          <div class="flex items-center gap-3 font-medium text-sm mb-3">
            <div data-slot="stat-icon">
              <component
                :is="HardDriveIcon"
                class="w-4 h-4"
              />
            </div>
            <span class="text-primary">Integrations</span>
          </div>
          <div class="text-3xl font-bold tabular-nums">
            {{ enabledIntegrations.length }}
          </div>
          <p class="text-sm text-muted-foreground mt-1">
            {{ enabledIntegrations.filter(i => i.lastSync).length }} synced recently
          </p>
        </UiCardContent>
      </UiCard>
    </div>

    <!-- Lifetime Stats Cards (Row 2) -->
    <div
      class="grid grid-cols-1 md:grid-cols-3 gap-5 mb-8"
      data-stagger
    >
      <!-- Total Space Reclaimed -->
      <UiCard
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 280 } }"
        data-slot="stat-card"
      >
        <UiCardContent class="pt-5">
          <div class="flex items-center gap-3 font-medium text-sm mb-3">
            <div data-slot="stat-icon">
              <component
                :is="Trash2Icon"
                class="w-4 h-4"
              />
            </div>
            <span class="text-primary">Total Space Reclaimed</span>
          </div>
          <div class="text-3xl font-bold tabular-nums">
            {{ formatBytes(dashboardStats?.totalBytesReclaimed ?? 0) }}
          </div>
          <p class="text-sm text-muted-foreground mt-1">
            {{ dashboardStats?.totalItemsRemoved ?? 0 }} items removed lifetime
          </p>
        </UiCardContent>
      </UiCard>

      <!-- Protected Items -->
      <UiCard
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 340 } }"
        data-slot="stat-card"
      >
        <UiCardContent class="pt-5">
          <div class="flex items-center gap-3 font-medium text-sm mb-3">
            <div data-slot="stat-icon">
              <component
                :is="ShieldCheckIcon"
                class="w-4 h-4"
              />
            </div>
            <span class="text-primary">Protected Items</span>
          </div>
          <div class="text-3xl font-bold tabular-nums">
            {{ dashboardStats?.protectedCount ?? 0 }}
          </div>
          <p class="text-sm text-muted-foreground mt-1">
            items protected by your rules
          </p>
        </UiCardContent>
      </UiCard>

      <!-- Library Growth Rate -->
      <UiCard
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 400 } }"
        data-slot="stat-card"
      >
        <UiCardContent class="pt-5">
          <div class="flex items-center gap-3 font-medium text-sm mb-3">
            <div data-slot="stat-icon">
              <component
                :is="TrendingUpIcon"
                class="w-4 h-4"
              />
            </div>
            <span class="text-primary">Library Growth Rate</span>
          </div>
          <div class="text-3xl font-bold tabular-nums">
            {{ formattedGrowthRate }}
          </div>
          <p class="text-sm text-muted-foreground mt-1">
            {{ dashboardStats?.hasGrowthData ? 'over the last 7 days' : 'not enough data yet' }}
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
      <component
        :is="HardDriveIcon"
        class="w-12 h-12 text-muted-foreground/40 mx-auto mb-4"
      />
      <h3 class="text-muted-foreground font-medium mb-1.5">
        No disk groups yet
      </h3>
      <p class="text-sm text-muted-foreground/70 mb-4 max-w-md mx-auto">
        Add integrations in
        <NuxtLink
          to="/settings"
          class="text-primary hover:underline"
        >Settings</NuxtLink>
        and data will appear on the next poll cycle.
      </p>
    </div>

    <!-- Skeleton Loading State -->
    <template v-if="loading">
      <div class="grid grid-cols-1 md:grid-cols-3 gap-5 mb-8">
        <UiCard
          v-for="i in 3"
          :key="i"
          class="animate-pulse"
        >
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
import { ServerIcon, ChartPieIcon, HardDriveIcon, LoaderCircleIcon, RefreshCwIcon, ActivityIcon, PlayIcon, CheckCircle2Icon, Trash2Icon, ShieldCheckIcon, TrendingUpIcon } from 'lucide-vue-next'
import { formatBytes, formatRelativeTime } from '~/utils/format'
import type { DiskGroup, IntegrationConfig, DashboardStats, SparklineTooltipOpts } from '~/types/api'

const api = useApi()
const { primaryColor, destructiveColor } = useThemeColors()

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
  modeLabel: engineModeLabel,
  fetchStats: engineFetchStats,
  triggerRunNow: engineTriggerRunNow
} = useEngineControl()

// Pull-to-refresh for touch devices
const { isRefreshing, pullProgress, pullDistance } = usePullToRefresh(async () => {
  await fetchDashboardData(true)
  refreshKey.value++
})

const chartModeOptions = [
  { label: 'Percentage', value: 'percentage' },
  { label: 'Raw (GB)', value: 'raw' }
]

const dateRangeOptions = [
  { label: 'Last Hour', value: '1h' },
  { label: 'Last 6h', value: '6h' },
  { label: 'Last 24h', value: '24h' },
  { label: 'Last 7 Days', value: '7d' },
  { label: 'Last 30 Days', value: '30d' },
  { label: 'All Time', value: 'all' }
]

const refreshOptions = [
  { label: '⏸ Paused', value: 0 },
  { label: '↻ 1s', value: 1000 },
  { label: '↻ 2s', value: 2000 },
  { label: '↻ 5s', value: 5000 },
  { label: '↻ 10s', value: 10000 },
  { label: '↻ 15s', value: 15000 },
  { label: '↻ 30s', value: 30000 },
  { label: '↻ 1m', value: 60000 },
  { label: '↻ 5m', value: 300000 }
]

const chartMode = ref('percentage')
const dateRange = ref('24h')
const refreshIntervalStr = ref('15000')
const refreshInterval = computed(() => Number(refreshIntervalStr.value))
const diskGroups = ref<DiskGroup[]>([])
const allIntegrations = ref<IntegrationConfig[]>([])
const flaggedSeries = ref<{ x: string, y: number }[]>([])
const deletedSeries = ref<{ x: string, y: number }[]>([])
const dashboardStats = ref<DashboardStats | null>(null)
const loading = ref(true)
const lastUpdated = ref<Date | null>(null)
const isAutoRefreshing = ref(false)
const refreshKey = ref(0)

// Engine stats — alias from shared composable
const engineStats = computed(() => engineControlStats.value)

const enabledIntegrations = computed(() => allIntegrations.value.filter(i => i.enabled))

const dateRangeLabel = computed(() => {
  const match = dateRangeOptions.find(o => o.value === dateRange.value)
  return match?.label ?? dateRange.value
})

const totalCapacity = computed(() =>
  diskGroups.value.reduce((sum, g) => sum + (g.totalBytes || 0), 0)
)

const totalUsed = computed(() =>
  diskGroups.value.reduce((sum, g) => sum + (g.usedBytes || 0), 0)
)

const formattedGrowthRate = computed(() => {
  if (!dashboardStats.value?.hasGrowthData) return '—'
  const bytes = dashboardStats.value.growthBytesPerWeek
  const prefix = bytes >= 0 ? '+' : ''
  return `${prefix}${formatBytes(Math.abs(bytes))} / week`
})

// --- Status banner ---
const engineStatusBannerClass = computed(() => {
  if (engineIsRunning.value) {
    return 'bg-primary/10 text-primary border border-primary/20'
  }
  return 'bg-muted text-muted-foreground'
})

const engineStatusText = computed(() => {
  if (engineIsRunning.value) {
    return 'Engine is running — evaluating media items…'
  }
  if (!engineLastRunEpoch.value) return 'Engine idle — no runs yet'
  return `Engine idle — last run ${formatRelativeTime(new Date(engineLastRunEpoch.value * 1000).toISOString())}`
})

// --- Countdown to next run ---
const nowEpoch = ref(Math.floor(Date.now() / 1000))
let countdownTimer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  countdownTimer = setInterval(() => {
    nowEpoch.value = Math.floor(Date.now() / 1000)
  }, 1000)
})

onUnmounted(() => {
  if (countdownTimer) clearInterval(countdownTimer)
})

const countdownText = computed(() => {
  if (engineIsRunning.value) return ''
  if (!engineLastRunEpoch.value || !enginePollInterval.value) return ''

  const nextRunEpoch = engineLastRunEpoch.value + enginePollInterval.value
  const remaining = nextRunEpoch - nowEpoch.value

  if (remaining <= 0) return 'Next run imminent'
  if (remaining < 60) return `Next run in ${remaining}s`
  if (remaining < 3600) {
    const mins = Math.floor(remaining / 60)
    const secs = remaining % 60
    return `Next run in ${mins}m ${secs}s`
  }
  const hours = Math.floor(remaining / 3600)
  const mins = Math.floor((remaining % 3600) / 60)
  return `Next run in ${hours}h ${mins}m`
})

// --- Auto refresh ---
let autoRefreshTimer: ReturnType<typeof setInterval> | null = null

function startAutoRefresh() {
  stopAutoRefresh()
  if (refreshInterval.value > 0) {
    autoRefreshTimer = setInterval(async () => {
      isAutoRefreshing.value = true
      await fetchDashboardData(true)
      refreshKey.value++
      isAutoRefreshing.value = false
    }, refreshInterval.value)
  }
}

function stopAutoRefresh() {
  if (autoRefreshTimer) {
    clearInterval(autoRefreshTimer)
    autoRefreshTimer = null
  }
}

watch(refreshInterval, () => {
  startAutoRefresh()
})

onMounted(async () => {
  await fetchDashboardData()
  startAutoRefresh()
})

onUnmounted(() => {
  stopAutoRefresh()
})

async function fetchDashboardData(silent = false) {
  if (!silent) loading.value = true
  try {
    const [groups, integrations, dStats] = await Promise.all([
      api('/api/v1/disk-groups'),
      api('/api/v1/integrations'),
      api('/api/v1/dashboard-stats').catch(() => null)
    ])
    // Fetch engine stats via the shared composable (handles toast on completion)
    engineFetchStats()
    // Fetch sparkline activity data in parallel (non-blocking)
    fetchActivityData()
    diskGroups.value = groups as DiskGroup[]
    allIntegrations.value = integrations as IntegrationConfig[]
    if (dStats) dashboardStats.value = dStats as DashboardStats
    lastUpdated.value = new Date()
  } catch {
    // Silently ignored — UI has no further handling
  } finally {
    if (!silent) loading.value = false
  }
}

// --- Sparkline: audit activity (flagged + deleted per time bucket) ---

const sparklineSeries = computed(() => {
  const series = []
  if (flaggedSeries.value.length > 0) {
    series.push({ name: 'Flagged', data: flaggedSeries.value })
  }
  if (deletedSeries.value.length > 0) {
    series.push({ name: 'Deleted', data: deletedSeries.value })
  }
  return series
})

const sparklineOptions = computed(() => ({
  chart: {
    type: 'area' as const,
    sparkline: { enabled: true },
    animations: { enabled: true, easing: 'easeinout', speed: 400 }
  },
  stroke: { curve: 'smooth' as const, width: 2 },
  colors: [primaryColor.value, destructiveColor.value],
  fill: {
    type: 'gradient',
    gradient: {
      shadeIntensity: 1,
      opacityFrom: 0.45,
      opacityTo: 0.05,
      stops: [0, 100]
    }
  },
  tooltip: {
    enabled: true,
    shared: true,
    x: { show: true },
    y: {
      formatter: (val: number, opts: SparklineTooltipOpts) => {
        const label = opts?.seriesIndex === 1 ? 'deleted' : 'flagged'
        return `${val} ${label}`
      }
    },
    theme: 'dark'
  },
  xaxis: { type: 'category' as const }
}))

// Re-fetch activity sparkline when time range changes
watch(dateRange, () => {
  fetchActivityData()
})

async function fetchActivityData() {
  const since = dateRange.value === 'all' ? '30d' : dateRange.value
  try {
    const data = await api(`/api/v1/audit/activity?since=${since}`) as { timestamp: string, flagged: number, deleted: number }[]
    flaggedSeries.value = (data || []).map(p => ({ x: p.timestamp, y: p.flagged }))
    deletedSeries.value = (data || []).map(p => ({ x: p.timestamp, y: p.deleted }))
  } catch {
    // silently ignore — sparkline is a nice-to-have
  }
}
</script>
