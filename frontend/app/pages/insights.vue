<template>
  <div class="container mx-auto px-4 py-6 max-w-7xl">
    <h1 class="text-2xl font-bold mb-6">{{ $t('insights.title') }}</h1>

    <!-- Tabs -->
    <UiTabs v-model="activeTab" class="w-full">
      <UiTabsList class="mb-6">
        <UiTabsTrigger value="overview">
          <BarChart3Icon class="w-4 h-4 mr-1.5" />
          {{ $t('insights.tabs.overview') }}
        </UiTabsTrigger>
        <UiTabsTrigger value="quality">
          <SparklesIcon class="w-4 h-4 mr-1.5" />
          {{ $t('insights.tabs.quality') }}
        </UiTabsTrigger>
        <UiTabsTrigger value="watch">
          <EyeIcon class="w-4 h-4 mr-1.5" />
          {{ $t('insights.tabs.watch') }}
        </UiTabsTrigger>
      </UiTabsList>

      <!-- Tab 1: Overview -->
      <UiTabsContent value="overview">
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <!-- Quality Profile Distribution (donut) -->
          <DashboardCard :title="$t('insights.qualityProfile')" :icon="PieChartIcon">
            <div class="h-64">
              <ClientOnly>
                <v-chart
                  v-if="compositionData"
                  :option="qualityDonutOption"
                  autoresize
                  class="h-full w-full"
                  @click="onQualityDonutClick"
                />
                <template #fallback>
                  <div class="h-full flex items-center justify-center">
                    <LoaderCircleIcon class="w-5 h-5 animate-spin text-muted-foreground" />
                  </div>
                </template>
              </ClientOnly>
            </div>
          </DashboardCard>

          <!-- Genre Distribution (horizontal bar) -->
          <DashboardCard :title="$t('insights.genreDistribution')" :icon="BarChart3Icon">
            <div class="h-64">
              <ClientOnly>
                <v-chart
                  v-if="compositionData"
                  :option="genreBarOption"
                  autoresize
                  class="h-full w-full"
                />
                <template #fallback>
                  <div class="h-full flex items-center justify-center">
                    <LoaderCircleIcon class="w-5 h-5 animate-spin text-muted-foreground" />
                  </div>
                </template>
              </ClientOnly>
            </div>
          </DashboardCard>

          <!-- Integration Contribution (treemap) -->
          <DashboardCard :title="$t('insights.integrationContribution')" :icon="LayersIcon">
            <div class="h-64">
              <ClientOnly>
                <v-chart
                  v-if="compositionData"
                  :option="integrationTreemapOption"
                  autoresize
                  class="h-full w-full"
                />
                <template #fallback>
                  <div class="h-full flex items-center justify-center">
                    <LoaderCircleIcon class="w-5 h-5 animate-spin text-muted-foreground" />
                  </div>
                </template>
              </ClientOnly>
            </div>
          </DashboardCard>

          <!-- Growth Over Time (line chart) — full width -->
          <DashboardCard
            :title="$t('insights.growthOverTime')"
            :icon="TrendingUpIcon"
            class="md:col-span-2"
          >
            <div class="h-64">
              <ClientOnly>
                <v-chart
                  v-if="metricsData && metricsData.length >= 3"
                  :option="growthLineOption"
                  autoresize
                  class="h-full w-full"
                />
                <div
                  v-else
                  class="h-full flex items-center justify-center text-muted-foreground text-sm"
                >
                  {{ $t('insights.noGrowthData') }}
                </div>
                <template #fallback>
                  <div class="h-full flex items-center justify-center">
                    <LoaderCircleIcon class="w-5 h-5 animate-spin text-muted-foreground" />
                  </div>
                </template>
              </ClientOnly>
            </div>
          </DashboardCard>
        </div>
      </UiTabsContent>

      <!-- Tab 2: Quality -->
      <UiTabsContent value="quality">
        <div class="grid grid-cols-1 gap-4">
          <!-- Quality Distribution (stacked bar: count + storage) -->
          <DashboardCard :title="$t('insights.qualityBreakdown')" :icon="SparklesIcon">
            <div class="h-72">
              <ClientOnly>
                <v-chart
                  v-if="qualityData"
                  :option="qualityStackedBarOption"
                  autoresize
                  class="h-full w-full"
                  @click="onQualityBarClick"
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

          <!-- Size Anomalies / Bloat Detection -->
          <DashboardCard :title="$t('insights.sizeAnomalies')" :icon="AlertTriangleIcon">
            <div v-if="bloatData && bloatData.length > 0" class="overflow-x-auto">
              <UiTable>
                <UiTableHeader>
                  <UiTableRow>
                    <UiTableHead>{{ $t('insights.bloatTitle') }}</UiTableHead>
                    <UiTableHead>{{ $t('insights.bloatQuality') }}</UiTableHead>
                    <UiTableHead class="text-right">{{ $t('insights.bloatSize') }}</UiTableHead>
                    <UiTableHead class="text-right">{{ $t('insights.bloatMedian') }}</UiTableHead>
                    <UiTableHead class="text-right">{{ $t('insights.bloatRatio') }}</UiTableHead>
                  </UiTableRow>
                </UiTableHeader>
                <UiTableBody>
                  <UiTableRow v-for="(item, idx) in bloatData.slice(0, 20)" :key="idx">
                    <UiTableCell class="font-medium max-w-[200px] truncate">
                      {{ item.title }}
                    </UiTableCell>
                    <UiTableCell>
                      <NuxtLink
                        :to="`/library?quality=${encodeURIComponent(item.qualityProfile)}`"
                        class="text-primary hover:underline"
                      >
                        {{ item.qualityProfile }}
                      </NuxtLink>
                    </UiTableCell>
                    <UiTableCell class="text-right font-mono text-xs tabular-nums">
                      {{ formatBytes(item.sizeBytes) }}
                    </UiTableCell>
                    <UiTableCell class="text-right font-mono text-xs tabular-nums">
                      {{ formatBytes(item.medianBytes) }}
                    </UiTableCell>
                    <UiTableCell class="text-right">
                      <UiBadge variant="destructive" class="text-xs tabular-nums">
                        {{ item.ratio }}x
                      </UiBadge>
                    </UiTableCell>
                  </UiTableRow>
                </UiTableBody>
              </UiTable>
            </div>
            <div
              v-else
              class="min-h-[120px] flex items-center justify-center text-muted-foreground text-sm"
            >
              {{ $t('insights.noBloat') }}
            </div>
          </DashboardCard>
        </div>
      </UiTabsContent>

      <!-- Tab 3: Watch Intelligence -->
      <UiTabsContent value="watch">
        <!-- Empty state: no media server configured -->
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

        <!-- Watch Intelligence cards -->
        <div v-else class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <!-- Dead Content -->
          <DashboardCard :title="$t('insights.deadContent')" :icon="SkullIcon">
            <div class="py-4 text-center">
              <div class="text-4xl font-bold tabular-nums">
                {{ deadData?.totalCount ?? '—' }}
              </div>
              <div class="text-sm text-muted-foreground mt-1">
                {{ $t('insights.deadContentItems') }}
              </div>
              <div v-if="deadData?.totalSize" class="text-xs text-muted-foreground mt-0.5">
                {{ formatBytes(deadData.totalSize) }} {{ $t('insights.reclaimable') }}
              </div>
            </div>
            <template #footer>
              <NuxtLink to="/library?filter=dead" class="text-primary hover:underline text-xs">
                {{ $t('insights.viewInLibrary') }} →
              </NuxtLink>
            </template>
          </DashboardCard>

          <!-- Stale Content -->
          <DashboardCard :title="$t('insights.staleContent')" :icon="ClockIcon">
            <div class="py-4 text-center">
              <div class="text-4xl font-bold tabular-nums">
                {{ staleData?.totalCount ?? '—' }}
              </div>
              <div class="text-sm text-muted-foreground mt-1">
                {{ $t('insights.staleContentItems') }}
              </div>
              <div v-if="staleData?.totalSize" class="text-xs text-muted-foreground mt-0.5">
                {{ formatBytes(staleData.totalSize) }} {{ $t('insights.reclaimable') }}
              </div>
            </div>
            <template #footer>
              <NuxtLink to="/library?filter=stale" class="text-primary hover:underline text-xs">
                {{ $t('insights.viewInLibrary') }} →
              </NuxtLink>
            </template>
          </DashboardCard>

          <!-- Top 20 / Bottom 20 ranked lists -->
          <DashboardCard v-if="popularityData" :title="$t('insights.topItems')" :icon="TrophyIcon">
            <div
              v-if="popularityData.topItems.length > 0"
              class="max-h-64 overflow-y-auto space-y-1"
            >
              <div
                v-for="(item, idx) in popularityData.topItems"
                :key="idx"
                class="flex items-center justify-between px-2 py-1 rounded hover:bg-muted/50 text-sm"
              >
                <div class="flex items-center gap-2 truncate">
                  <span class="text-xs text-muted-foreground font-mono tabular-nums w-5"
                    >{{ idx + 1 }}.</span
                  >
                  <span class="truncate">{{ item.title }}</span>
                </div>
                <UiBadge variant="secondary" class="text-xs tabular-nums shrink-0 ml-2">
                  {{ item.playCount }} {{ $t('insights.plays') }}
                </UiBadge>
              </div>
            </div>
            <div v-else class="h-32 flex items-center justify-center text-muted-foreground text-sm">
              {{ $t('insights.noData') }}
            </div>
          </DashboardCard>

          <DashboardCard
            v-if="popularityData"
            :title="$t('insights.leastPopular')"
            :icon="ThumbsDownIcon"
          >
            <div
              v-if="popularityData.lowItems.length > 0"
              class="max-h-64 overflow-y-auto space-y-1"
            >
              <div
                v-for="(item, idx) in popularityData.lowItems"
                :key="idx"
                class="flex items-center justify-between px-2 py-1 rounded hover:bg-muted/50 text-sm"
              >
                <div class="flex items-center gap-2 truncate">
                  <span class="text-xs text-muted-foreground font-mono tabular-nums w-5"
                    >{{ idx + 1 }}.</span
                  >
                  <span class="truncate">{{ item.title }}</span>
                </div>
                <UiBadge variant="outline" class="text-xs tabular-nums shrink-0 ml-2">
                  {{ item.playCount }} {{ $t('insights.plays') }}
                </UiBadge>
              </div>
            </div>
            <div v-else class="h-32 flex items-center justify-center text-muted-foreground text-sm">
              {{ $t('insights.noData') }}
            </div>
          </DashboardCard>

          <!-- Request Fulfillment -->
          <DashboardCard
            :title="$t('insights.requestFulfillment')"
            :icon="CheckCircleIcon"
            class="md:col-span-2"
          >
            <div v-if="requestData" class="space-y-4">
              <!-- Headline stats -->
              <div class="flex items-center gap-6 justify-center py-2">
                <div class="text-center">
                  <div class="text-3xl font-bold tabular-nums">
                    {{ requestData.fulfillmentPct }}%
                  </div>
                  <div class="text-xs text-muted-foreground">
                    {{ $t('insights.fulfillmentRate') }}
                  </div>
                </div>
                <div class="text-center">
                  <div class="text-2xl font-semibold tabular-nums">
                    {{ requestData.fulfilled }}/{{ requestData.totalRequested }}
                  </div>
                  <div class="text-xs text-muted-foreground">
                    {{ $t('insights.requestsFulfilled') }}
                  </div>
                </div>
              </div>

              <!-- Unfulfilled table -->
              <div
                v-if="requestData.unfulfilledItems && requestData.unfulfilledItems.length > 0"
                class="overflow-x-auto"
              >
                <h4 class="text-sm font-medium mb-2">
                  {{ $t('insights.unfulfilledRequests') }}
                </h4>
                <UiTable>
                  <UiTableHeader>
                    <UiTableRow>
                      <UiTableHead>{{ $t('insights.requestTitle') }}</UiTableHead>
                      <UiTableHead>{{ $t('insights.requestedBy') }}</UiTableHead>
                      <UiTableHead class="text-right">{{ $t('insights.requestSize') }}</UiTableHead>
                    </UiTableRow>
                  </UiTableHeader>
                  <UiTableBody>
                    <UiTableRow
                      v-for="(item, idx) in requestData.unfulfilledItems.slice(0, 15)"
                      :key="idx"
                    >
                      <UiTableCell class="font-medium max-w-[200px] truncate">
                        {{ item.title }}
                      </UiTableCell>
                      <UiTableCell class="text-muted-foreground">
                        {{ item.requestedBy || '—' }}
                      </UiTableCell>
                      <UiTableCell class="text-right font-mono text-xs tabular-nums">
                        {{ formatBytes(item.sizeBytes) }}
                      </UiTableCell>
                    </UiTableRow>
                  </UiTableBody>
                </UiTable>
              </div>
            </div>
            <div
              v-else
              class="min-h-[120px] flex items-center justify-center text-muted-foreground text-sm"
            >
              {{ $t('insights.noRequestData') }}
            </div>
          </DashboardCard>
        </div>
      </UiTabsContent>
    </UiTabs>
  </div>
</template>

<script setup lang="ts">
import {
  BarChart3Icon,
  SparklesIcon,
  EyeIcon,
  EyeOffIcon,
  PieChartIcon,
  AlertTriangleIcon,
  SkullIcon,
  ClockIcon,
  CheckCircleIcon,
  LoaderCircleIcon,
  LayersIcon,
  TrendingUpIcon,
  TrophyIcon,
  ThumbsDownIcon,
} from 'lucide-vue-next';
import { DashboardCard } from '~/components/ui/dashboard-card';
import { formatBytes } from '~/utils/format';
import type { IntegrationConfig, MetricsHistoryResponse, LibraryHistoryRow } from '~/types/api';

// ─── API response types ─────────────────────────────────────────────────────

interface NameCount {
  name: string;
  count: number;
  sizeBytes: number;
}

interface CompositionResponse {
  qualityDistribution: NameCount[];
  genreDistribution: NameCount[];
  yearDistribution: NameCount[];
  typeDistribution: NameCount[];
  totalItems: number;
  totalSizeBytes: number;
}

interface QualityProfile {
  name: string;
  count: number;
  sizeBytes: number;
}

interface QualityDistributionResponse {
  profiles: QualityProfile[];
}

interface SizeAnomaly {
  title: string;
  qualityProfile: string;
  sizeBytes: number;
  medianBytes: number;
  ratio: number;
  integrationId: number;
}

interface DeadContentReport {
  items: { title: string; type: string; sizeBytes: number; daysInLibrary: number }[];
  totalCount: number;
  totalSize: number;
}

interface StaleContentReport {
  items: {
    title: string;
    type: string;
    sizeBytes: number;
    daysSinceWatched: number;
    playCount: number;
    stalenessScore: number;
  }[];
  totalCount: number;
  totalSize: number;
}

interface PopularityEntry {
  genre: string;
  year: string;
  playCount: number;
  itemCount: number;
}

interface RankedItem {
  title: string;
  playCount: number;
  sizeBytes: number;
}

interface PopularityData {
  heatmap: PopularityEntry[];
  topItems: RankedItem[];
  lowItems: RankedItem[];
}

interface RequestFulfillmentData {
  totalRequested: number;
  fulfilled: number;
  unfulfilled: number;
  fulfillmentPct: number;
  unfulfilledItems: {
    title: string;
    requestedBy: string;
    watchedByRequestor: boolean;
    sizeBytes: number;
  }[];
}

// ─── Reactive state ─────────────────────────────────────────────────────────

const activeTab = ref('overview');
const api = useApi();
const router = useRouter();
const { isDark } = useAppColorMode();
const {
  chart1Color,
  chart2Color,
  chart3Color,
  chart4Color,
  glowLineStyle,
  gradientArea,
  gradientBar,
  tooltipConfig,
  emphasisConfig,
  generatePalette,
} = useEChartsDefaults();
const { on, off } = useEventStream();

const compositionData = ref<CompositionResponse | null>(null);
const qualityData = ref<QualityDistributionResponse | null>(null);
const bloatData = ref<SizeAnomaly[] | null>(null);
const deadData = ref<DeadContentReport | null>(null);
const staleData = ref<StaleContentReport | null>(null);
const popularityData = ref<PopularityData | null>(null);
const requestData = ref<RequestFulfillmentData | null>(null);
const metricsData = ref<LibraryHistoryRow[]>([]);
const integrations = ref<IntegrationConfig[]>([]);

// ─── Data fetching ──────────────────────────────────────────────────────────

async function fetchComposition() {
  try {
    compositionData.value = (await api('/api/v1/analytics/composition')) as CompositionResponse;
  } catch {
    // Silent — charts show placeholder
  }
}

async function fetchQuality() {
  try {
    qualityData.value = (await api('/api/v1/analytics/quality')) as QualityDistributionResponse;
  } catch {
    // Silent
  }
}

async function fetchBloat() {
  try {
    bloatData.value = (await api('/api/v1/analytics/bloat')) as SizeAnomaly[];
  } catch {
    // Silent
  }
}

async function fetchDeadContent() {
  try {
    deadData.value = (await api('/api/v1/analytics/dead-content')) as DeadContentReport;
  } catch {
    // Silent
  }
}

async function fetchStaleContent() {
  try {
    staleData.value = (await api('/api/v1/analytics/stale-content')) as StaleContentReport;
  } catch {
    // Silent
  }
}

async function fetchPopularity() {
  try {
    popularityData.value = (await api('/api/v1/analytics/popularity')) as PopularityData;
  } catch {
    // Silent
  }
}

async function fetchRequestFulfillment() {
  try {
    requestData.value = (await api(
      '/api/v1/analytics/request-fulfillment',
    )) as RequestFulfillmentData;
  } catch {
    // Silent
  }
}

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

function fetchAllAnalytics() {
  fetchComposition();
  fetchQuality();
  fetchBloat();
  fetchDeadContent();
  fetchStaleContent();
  fetchPopularity();
  fetchRequestFulfillment();
  fetchMetrics();
}

// ─── Watch providers detection ──────────────────────────────────────────────

const WATCH_PROVIDER_TYPES = new Set(['plex', 'jellyfin', 'emby', 'tautulli']);

const noWatchProviders = computed(() => {
  const enabled = integrations.value.filter((i) => i.enabled);
  return !enabled.some((i) => WATCH_PROVIDER_TYPES.has(i.type));
});

// ─── SSE: analytics_updated → refetch ───────────────────────────────────────

function handleAnalyticsUpdated() {
  fetchAllAnalytics();
}

// ─── Click handlers for chart→Library navigation ────────────────────────────

function onQualityDonutClick(params: { name?: string }) {
  if (params.name) {
    router.push(`/library?quality=${encodeURIComponent(params.name)}`);
  }
}

function onQualityBarClick(params: { name?: string }) {
  if (params.name) {
    router.push(`/library?quality=${encodeURIComponent(params.name)}`);
  }
}

// ─── Chart options ──────────────────────────────────────────────────────────

const textColor = computed(() => (isDark.value ? '#a1a1aa' : '#71717a'));

const qualityDonutOption = computed(() => {
  const data = compositionData.value?.qualityDistribution ?? [];
  const palette = generatePalette(chart1Color.value, data.length);
  return {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)', ...tooltipConfig() },
    series: [
      {
        type: 'pie',
        radius: ['40%', '70%'],
        avoidLabelOverlap: true,
        animationType: 'scale',
        label: { color: textColor.value, fontSize: 11 },
        itemStyle: { shadowBlur: 6, shadowColor: 'rgba(0,0,0,0.15)' },
        emphasis: { itemStyle: { shadowBlur: 12, shadowColor: 'rgba(0,0,0,0.3)' } },
        data: data.map((d, i) => ({
          name: d.name,
          value: d.count,
          itemStyle: { color: palette[i] },
        })),
      },
    ],
  };
});

const genreBarOption = computed(() => {
  const data = (compositionData.value?.genreDistribution ?? []).slice(0, 10);
  return {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', ...tooltipConfig() },
    grid: { top: 10, right: 10, bottom: 30, left: 80 },
    xAxis: {
      type: 'value',
      axisLabel: { color: textColor.value, fontSize: 11 },
      splitLine: { lineStyle: { type: 'dashed', opacity: 0.15 } },
    },
    yAxis: {
      type: 'category',
      data: data.map((d) => d.name).reverse(),
      axisLabel: { color: textColor.value, fontSize: 11 },
    },
    series: [
      {
        type: 'bar',
        data: data.map((d) => d.count).reverse(),
        itemStyle: { color: gradientBar(chart1Color.value), borderRadius: [0, 4, 4, 0] },
        emphasis: { itemStyle: { shadowBlur: 8 } },
      },
    ],
  };
});

const integrationTreemapOption = computed(() => {
  const data = compositionData.value?.typeDistribution ?? [];
  const colors = [chart1Color.value, chart2Color.value, chart3Color.value, chart4Color.value];
  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'item',
      ...tooltipConfig(),
      formatter: (params: { name: string; value: number }) =>
        `${params.name}: ${params.value} items`,
    },
    series: [
      {
        type: 'treemap',
        data: data.map((d, i) => ({
          name: d.name,
          value: d.count,
          itemStyle: { color: colors[i % colors.length] },
        })),
        itemStyle: {
          borderWidth: 2,
          borderColor: isDark.value ? '#18181b' : '#fafafa',
        },
        label: {
          show: true,
          color: '#fff',
          fontSize: 12,
          formatter: '{b}\n{c}',
          textShadowBlur: 2,
          textShadowColor: 'rgba(0,0,0,0.5)',
        },
        breadcrumb: { show: false },
      },
    ],
  };
});

const growthLineOption = computed(() => {
  const rows = [...metricsData.value].sort(
    (a, b) => new Date(a.timestamp).getTime() - new Date(b.timestamp).getTime(),
  );
  const labels = rows.map((r) => {
    const d = new Date(r.timestamp);
    return `${d.getMonth() + 1}/${d.getDate()}`;
  });
  const used = rows.map((r) => Math.round(r.usedCapacity / 1e9));
  const total = rows.map((r) => Math.round(r.totalCapacity / 1e9));

  return {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis', ...tooltipConfig() },
    legend: {
      data: ['Used (GB)', 'Total (GB)'],
      textStyle: { color: textColor.value },
    },
    grid: { top: 30, right: 10, bottom: 30, left: 50 },
    xAxis: {
      type: 'category',
      data: labels,
      axisLabel: { color: textColor.value, fontSize: 11 },
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: textColor.value, fontSize: 11 },
    },
    series: [
      {
        name: 'Used (GB)',
        type: 'line',
        smooth: true,
        symbol: 'none',
        lineStyle: glowLineStyle(chart1Color.value),
        areaStyle: gradientArea(chart1Color.value),
        data: used,
        itemStyle: { color: chart1Color.value },
        emphasis: emphasisConfig(),
      },
      {
        name: 'Total (GB)',
        type: 'line',
        smooth: true,
        symbol: 'none',
        lineStyle: { type: 'dashed', width: 1, color: chart2Color.value },
        data: total,
        itemStyle: { color: chart2Color.value },
      },
    ],
  };
});

const qualityStackedBarOption = computed(() => {
  const profiles = qualityData.value?.profiles ?? [];
  const names = profiles.map((p) => p.name);
  const counts = profiles.map((p) => p.count);
  const sizes = profiles.map((p) => Math.round(p.sizeBytes / 1e9)); // GB

  return {
    backgroundColor: 'transparent',
    tooltip: {
      trigger: 'axis',
      axisPointer: { type: 'shadow' },
      ...tooltipConfig(),
    },
    legend: {
      data: ['Items', 'Storage (GB)'],
      textStyle: { color: textColor.value },
    },
    grid: { top: 30, right: 10, bottom: 30, left: 50 },
    xAxis: {
      type: 'category',
      data: names,
      axisLabel: { color: textColor.value, fontSize: 11, rotate: names.length > 5 ? 30 : 0 },
    },
    yAxis: [
      {
        type: 'value',
        name: 'Items',
        axisLabel: { color: textColor.value, fontSize: 11 },
        nameTextStyle: { color: textColor.value },
      },
      {
        type: 'value',
        name: 'GB',
        axisLabel: { color: textColor.value, fontSize: 11 },
        nameTextStyle: { color: textColor.value },
      },
    ],
    series: [
      {
        name: 'Items',
        type: 'bar',
        data: counts,
        itemStyle: { color: chart1Color.value, borderRadius: [4, 4, 0, 0] },
        emphasis: emphasisConfig(),
      },
      {
        name: 'Storage (GB)',
        type: 'bar',
        yAxisIndex: 1,
        data: sizes,
        itemStyle: { color: chart3Color.value, borderRadius: [4, 4, 0, 0] },
        emphasis: emphasisConfig(),
      },
    ],
  };
});

// ─── Lifecycle ───────────────────────────────────────────────────────────────

onMounted(async () => {
  await fetchIntegrations();
  fetchAllAnalytics();
  on('analytics_updated', handleAnalyticsUpdated);
});

onUnmounted(() => {
  off('analytics_updated', handleAnalyticsUpdated);
});
</script>
