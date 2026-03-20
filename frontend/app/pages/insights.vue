<template>
  <div class="container mx-auto px-4 py-6 max-w-7xl">
    <h1 class="text-2xl font-bold mb-6">{{ $t('insights.title') }}</h1>

    <div class="space-y-4">
      <!-- Watch Intelligence: Empty state — no media server configured -->
      <div
        v-if="noWatchProviders"
        class="flex flex-col items-center justify-center py-16 text-center"
      >
        <EyeOffIcon class="w-12 h-12 text-muted-foreground/40 mb-4" />
        <h3 class="text-lg font-medium text-foreground mb-2">
          {{ $t('insights.noWatchProviders') }}
        </h3>
        <p class="text-muted-foreground max-w-md">
          {{ $t('insights.noWatchProvidersDesc') }}
        </p>
        <UiButton class="mt-4" as-child>
          <NuxtLink to="/settings?tab=integrations">
            {{ $t('insights.configureMediaServer') }}
          </NuxtLink>
        </UiButton>
      </div>

      <template v-else>
        <!-- Row 1: Capacity Gauge + Forecast (side by side) -->
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <!-- Capacity Gauge -->
          <DashboardCard
            v-motion
            :initial="{ opacity: 0, y: 12 }"
            :enter="{
              opacity: 1,
              y: 0,
              transition: { type: 'spring', stiffness: 260, damping: 24, delay: 0 },
            }"
            :title="$t('insights.capacityGauge')"
            :icon="GaugeIcon"
          >
            <div class="h-72">
              <ClientOnly>
                <v-chart
                  v-if="latestMetrics"
                  :option="capacityGaugeOption"
                  autoresize
                  class="h-full w-full"
                />
                <div
                  v-else
                  class="h-full flex items-center justify-center text-muted-foreground text-sm"
                >
                  <LoaderCircleIcon class="w-5 h-5 animate-spin" />
                </div>
                <template #fallback>
                  <div class="h-full flex items-center justify-center">
                    <LoaderCircleIcon class="w-5 h-5 animate-spin text-muted-foreground" />
                  </div>
                </template>
              </ClientOnly>
            </div>
          </DashboardCard>

          <!-- Capacity Forecast -->
          <DashboardCard
            v-motion
            :initial="{ opacity: 0, y: 12 }"
            :enter="{
              opacity: 1,
              y: 0,
              transition: { type: 'spring', stiffness: 260, damping: 24, delay: 60 },
            }"
            :title="$t('insights.capacityForecast')"
            :icon="TrendingUpIcon"
          >
            <div class="py-6 space-y-4">
              <template v-if="forecastData">
                <!-- Shrinking -->
                <div v-if="forecastData.growthRatePerDay < 0" class="text-center py-8">
                  <div class="text-2xl font-bold text-green-500">
                    {{ $t('insights.capacityDecreasing') }} ↓
                  </div>
                  <div class="text-sm text-muted-foreground mt-2">
                    {{ formatBytes(Math.abs(forecastData.growthRatePerDay)) }}/day
                  </div>
                </div>

                <!-- Growing or stable -->
                <template v-else>
                  <div class="grid grid-cols-1 gap-3">
                    <!-- Growth rate -->
                    <div class="flex items-center justify-between p-3 rounded-lg bg-muted/40">
                      <span class="text-sm text-muted-foreground">
                        {{ $t('insights.growthRate') }}
                      </span>
                      <span class="text-lg font-bold tabular-nums">
                        {{ formatGrowthRate(forecastData.growthRatePerDay) }}
                      </span>
                    </div>

                    <!-- Days until threshold -->
                    <div class="flex items-center justify-between p-3 rounded-lg bg-muted/40">
                      <span class="text-sm text-muted-foreground">
                        {{ $t('insights.daysUntilThreshold') }}
                      </span>
                      <span class="text-lg font-bold tabular-nums">
                        <template v-if="forecastData.daysUntilThreshold === -1">—</template>
                        <template v-else-if="forecastData.daysUntilThreshold === 0">
                          <UiBadge variant="destructive">Now</UiBadge>
                        </template>
                        <template v-else>
                          {{ forecastData.daysUntilThreshold }} {{ $t('insights.days') }}
                        </template>
                      </span>
                    </div>

                    <!-- Days until full -->
                    <div class="flex items-center justify-between p-3 rounded-lg bg-muted/40">
                      <span class="text-sm text-muted-foreground">
                        {{ $t('insights.daysUntilFull') }}
                      </span>
                      <span class="text-lg font-bold tabular-nums">
                        <template v-if="forecastData.daysUntilFull === -1">—</template>
                        <template v-else-if="forecastData.daysUntilFull === 0">
                          <UiBadge variant="destructive">Now</UiBadge>
                        </template>
                        <template v-else>
                          {{ forecastData.daysUntilFull }} {{ $t('insights.days') }}
                        </template>
                      </span>
                    </div>
                  </div>
                </template>
              </template>

              <!-- Loading state -->
              <div
                v-else
                class="h-40 flex items-center justify-center text-muted-foreground text-sm"
              >
                <LoaderCircleIcon class="w-5 h-5 animate-spin" />
              </div>
            </div>
          </DashboardCard>
        </div>

        <!-- Row 2: Storage Map — full width -->
        <DashboardCard
          v-motion
          :initial="{ opacity: 0, y: 12 }"
          :enter="{
            opacity: 1,
            y: 0,
            transition: { type: 'spring', stiffness: 260, damping: 24, delay: 120 },
          }"
          title="Storage Map"
          :icon="LayoutGridIcon"
        >
          <div class="h-[500px]">
            <ClientOnly>
              <v-chart
                v-if="statusData && statusData.statuses.some((s: StatusGroup) => s.totalCount > 0)"
                :option="treemapOption"
                autoresize
                class="h-full w-full"
              />
              <div
                v-else-if="statusData"
                class="h-full flex items-center justify-center text-muted-foreground text-sm"
              >
                No status breakdown data available — run an engine cycle to populate.
              </div>
              <div
                v-else
                class="h-full flex items-center justify-center text-muted-foreground text-sm"
              >
                <LoaderCircleIcon class="w-5 h-5 animate-spin" />
              </div>
              <template #fallback>
                <div class="h-full flex items-center justify-center">
                  <LoaderCircleIcon class="w-5 h-5 animate-spin text-muted-foreground" />
                </div>
              </template>
            </ClientOnly>
          </div>
        </DashboardCard>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  EyeOffIcon,
  LoaderCircleIcon,
  TrendingUpIcon,
  GaugeIcon,
  LayoutGridIcon,
} from 'lucide-vue-next';
import { DashboardCard } from '~/components/ui/dashboard-card';
import { formatBytes } from '~/utils/format';
import type {
  IntegrationConfig,
  MetricsHistoryResponse,
  LibraryHistoryRow,
  DiskGroup,
} from '~/types/api';

// ─── API response types ─────────────────────────────────────────────────────

interface TreeNode {
  name: string;
  value?: number;
  children?: TreeNode[];
}

interface StatusGroup {
  name: string;
  totalSize: number;
  totalCount: number;
  children: TreeNode[];
}

interface StatusBreakdown {
  statuses: StatusGroup[];
}

interface CapacityForecast {
  currentUsedPct: number;
  growthRatePerDay: number;
  daysUntilThreshold: number;
  daysUntilFull: number;
  totalCapacity: number;
  usedCapacity: number;
}

// ─── Reactive state ─────────────────────────────────────────────────────────

const api = useApi();
const { isDark } = useAppColorMode();
const { tooltipConfig, colorAlpha, successColor, destructiveColor } = useEChartsDefaults();
const { primaryColor } = useThemeColors();
const { on, off } = useEventStream();

const metricsData = ref<LibraryHistoryRow[]>([]);
const integrations = ref<IntegrationConfig[]>([]);
const forecastData = ref<CapacityForecast | null>(null);
const diskGroups = ref<DiskGroup[]>([]);
const statusData = ref<StatusBreakdown | null>(null);

// ─── Data fetching ──────────────────────────────────────────────────────────

async function fetchMetrics() {
  try {
    const resp = (await api('/api/v1/metrics/history')) as MetricsHistoryResponse;
    metricsData.value = resp?.data ?? [];
  } catch {
    // Silent
  }
}

async function fetchIntegrations() {
  try {
    integrations.value = (await api('/api/v1/integrations')) as IntegrationConfig[];
  } catch {
    // Silent
  }
}

async function fetchForecast() {
  try {
    forecastData.value = (await api('/api/v1/analytics/forecast')) as CapacityForecast;
  } catch {
    // Silent
  }
}

async function fetchDiskGroups() {
  try {
    diskGroups.value = (await api('/api/v1/disk-groups')) as DiskGroup[];
  } catch {
    // Silent
  }
}

async function fetchStatusBreakdown() {
  try {
    statusData.value = (await api('/api/v1/analytics/status-breakdown')) as StatusBreakdown;
  } catch {
    // Silent
  }
}

function fetchAllAnalytics() {
  fetchMetrics();
  fetchForecast();
  fetchStatusBreakdown();
}

// ─── Watch providers detection ──────────────────────────────────────────────

const WATCH_PROVIDER_TYPES = new Set(['plex', 'jellyfin', 'emby', 'tautulli']);

const noWatchProviders = computed(() => {
  const enabled = integrations.value.filter((i) => i.enabled);
  return !enabled.some((i) => WATCH_PROVIDER_TYPES.has(i.type));
});

// ─── Derived data ───────────────────────────────────────────────────────────

/** Latest metrics data point for the capacity gauge. */
const latestMetrics = computed(() => {
  if (!metricsData.value.length) return null;
  const sorted = [...metricsData.value].sort(
    (a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime(),
  );
  return sorted[0] ?? null;
});

/** Disk group threshold and target (from first disk group). */
const diskThreshold = computed(() => diskGroups.value[0]?.thresholdPct ?? 90);
const diskTarget = computed(() => diskGroups.value[0]?.targetPct ?? 80);

// ─── Formatting helpers ─────────────────────────────────────────────────────

function formatGrowthRate(bytesPerDay: number): string {
  if (bytesPerDay === 0) return '0 B/day';
  return `${formatBytes(bytesPerDay)}/day`;
}

// ─── SSE: analytics_updated → refetch ───────────────────────────────────────

function handleAnalyticsUpdated() {
  fetchAllAnalytics();
}

// ─── Chart options ──────────────────────────────────────────────────────────

// Status → color mapping (semantic)
const STATUS_COLORS: Record<string, string> = {
  dead: '#ef4444',
  stale: '#f59e0b',
  protected: '', // filled dynamically from primaryColor
  active: '#10b981',
};

function getStatusColor(name: string): string {
  if (name === 'protected') return primaryColor.value;
  return STATUS_COLORS[name] ?? '#64748b';
}

// Capacity Gauge — liquid fill tank with zone-aware color
const capacityGaugeOption = computed(() => {
  const latest = latestMetrics.value;
  if (!latest) return {};

  const usedPct = latest.totalCapacity > 0 ? (latest.usedCapacity / latest.totalCapacity) * 100 : 0;
  const usedTB = (latest.usedCapacity / 1e12).toFixed(1);
  const totalTB = (latest.totalCapacity / 1e12).toFixed(1);
  const threshold = diskThreshold.value;

  // Determine tank color based on zone
  let tankColor = successColor.value;
  if (usedPct >= threshold) {
    tankColor = destructiveColor.value;
  } else if (usedPct >= diskTarget.value) {
    tankColor = '#f59e0b'; // amber
  }

  return {
    backgroundColor: 'transparent',
    series: [
      {
        type: 'liquidFill',
        data: [usedPct / 100],
        radius: '80%',
        center: ['50%', '50%'],
        color: [tankColor],
        backgroundStyle: {
          color: isDark.value ? 'rgba(63,63,70,0.3)' : 'rgba(228,228,231,0.4)',
          borderColor: isDark.value ? 'rgba(63,63,70,0.6)' : 'rgba(228,228,231,0.8)',
          borderWidth: 2,
        },
        outline: {
          show: true,
          borderDistance: 4,
          itemStyle: {
            borderColor: colorAlpha(tankColor, 0.3),
            borderWidth: 3,
          },
        },
        label: {
          formatter: () => `${usedPct.toFixed(1)}%\n${usedTB} / ${totalTB} TB`,
          fontSize: 20,
          fontWeight: 'bold',
          color: isDark.value ? '#e4e4e7' : '#18181b',
        },
        shape: 'circle',
        waveAnimation: true,
        animationDuration: 2000,
        animationDurationUpdate: 1000,
      },
    ],
  };
});

// ─── Treemap option ─────────────────────────────────────────────────────────

const treemapOption = computed(() => {
  if (!statusData.value) return {};

  const cardBg = isDark.value ? '#18181b' : '#ffffff';

  // Recursively color tree nodes — children inherit lighter variations of parent color
  function colorTree(
    nodes: TreeNode[],
    baseColor: string,
    depth: number,
  ): Record<string, unknown>[] {
    return nodes.map((node, i) => {
      const nodeColor = depth === 0 ? baseColor : colorAlpha(baseColor, 0.5 + (i % 6) * 0.08);

      if (node.children && node.children.length > 0) {
        return {
          name: node.name,
          itemStyle: { color: nodeColor },
          children: colorTree(node.children, nodeColor, depth + 1),
        };
      }
      return {
        name: node.name,
        value: node.value ?? 0,
        itemStyle: { color: nodeColor },
      };
    });
  }

  // Build treemap data from the recursive StatusBreakdown tree
  const data = statusData.value.statuses
    .filter((s) => s.totalCount > 0)
    .map((status) => {
      const baseColor = getStatusColor(status.name);
      return {
        name: capitalize(status.name),
        itemStyle: { color: baseColor },
        children: colorTree(status.children ?? [], baseColor, 0),
      };
    });

  return {
    backgroundColor: 'transparent',
    animationDuration: 1000,
    animationEasing: 'cubicOut',
    tooltip: {
      ...tooltipConfig(),
      formatter: (params: { name?: string; value?: number; treePathInfo?: { name: string }[] }) => {
        const name = params.name ?? '';
        const gb = params.value ? (params.value / 1e9).toFixed(1) : '0';
        // Show breadcrumb path in tooltip
        const path = params.treePathInfo
          ?.map((p) => p.name)
          .filter(Boolean)
          .join(' › ');
        return `<div style="font-size:11px;color:#999;margin-bottom:2px">${path ?? ''}</div><strong>${name}</strong><br/>${gb} GB`;
      },
    },
    series: [
      {
        type: 'treemap',
        data,
        leafDepth: 2,
        drillDownIcon: '▶',
        breadcrumb: {
          show: true,
          itemStyle: {
            color: isDark.value ? '#27272a' : '#f4f4f5',
            borderColor: isDark.value ? '#3f3f46' : '#e4e4e7',
            textStyle: {
              color: isDark.value ? '#a1a1aa' : '#71717a',
            },
          },
        },
        roam: false,
        visibleMin: 200,
        upperLabel: {
          show: true,
          height: 28,
          color: isDark.value ? '#fafafa' : '#18181b',
          fontSize: 12,
          fontWeight: 'bold',
          backgroundColor: 'transparent',
        },
        itemStyle: {
          borderColor: cardBg,
          borderWidth: 2,
          gapWidth: 2,
          borderRadius: 4,
        },
        emphasis: {
          itemStyle: {
            shadowBlur: 12,
            shadowColor: isDark.value ? 'rgba(255,255,255,0.15)' : 'rgba(0,0,0,0.2)',
          },
          upperLabel: {
            show: true,
            color: isDark.value ? '#ffffff' : '#000000',
          },
        },
        label: {
          show: true,
          formatter: (params: { name?: string; value?: number }) => {
            const name = params.name ?? '';
            const gb = params.value ? (params.value / 1e9).toFixed(1) : '0';
            return `${name}\n${gb} GB`;
          },
          color: isDark.value ? '#e4e4e7' : '#18181b',
          fontSize: 11,
          lineHeight: 16,
        },
        levels: [
          {
            // Level 0: top status groups (Dead, Stale, Protected, Active)
            itemStyle: {
              borderColor: cardBg,
              borderWidth: 4,
              gapWidth: 4,
              borderRadius: 6,
            },
          },
          {
            // Level 1: shows / movies (containers or leaves)
            itemStyle: {
              borderColor: cardBg,
              borderWidth: 2,
              gapWidth: 2,
              borderRadius: 4,
            },
            colorSaturation: [0.3, 0.7],
          },
          {
            // Level 2: seasons within shows (deepest leaves)
            itemStyle: {
              borderColor: cardBg,
              borderWidth: 1,
              gapWidth: 1,
              borderRadius: 3,
            },
            colorSaturation: [0.25, 0.6],
          },
        ],
      },
    ],
  };
});

// ─── Helpers ────────────────────────────────────────────────────────────────

function capitalize(str: string): string {
  return str.charAt(0).toUpperCase() + str.slice(1);
}

// ─── Lifecycle ───────────────────────────────────────────────────────────────

onMounted(async () => {
  await fetchIntegrations();
  await fetchDiskGroups();
  fetchAllAnalytics();
  on('analytics_updated', handleAnalyticsUpdated);
});

onUnmounted(() => {
  off('analytics_updated', handleAnalyticsUpdated);
});
</script>
