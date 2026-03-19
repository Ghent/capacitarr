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
          <DashboardCard title="Quality Distribution" :icon="PieChartIcon">
            <div class="h-64 flex items-center justify-center text-muted-foreground text-sm">
              <ClientOnly>
                <v-chart
                  v-if="compositionData"
                  :option="qualityDonutOption"
                  autoresize
                  class="h-full w-full"
                />
                <template #fallback>
                  <LoaderCircleIcon class="w-5 h-5 animate-spin" />
                </template>
              </ClientOnly>
            </div>
          </DashboardCard>

          <DashboardCard title="Genre Distribution" :icon="BarChart3Icon">
            <div class="h-64 flex items-center justify-center text-muted-foreground text-sm">
              <ClientOnly>
                <v-chart
                  v-if="compositionData"
                  :option="genreBarOption"
                  autoresize
                  class="h-full w-full"
                />
                <template #fallback>
                  <LoaderCircleIcon class="w-5 h-5 animate-spin" />
                </template>
              </ClientOnly>
            </div>
          </DashboardCard>

          <DashboardCard title="Year Distribution" :icon="CalendarIcon" class="md:col-span-2">
            <div class="h-64 flex items-center justify-center text-muted-foreground text-sm">
              <ClientOnly>
                <v-chart
                  v-if="compositionData"
                  :option="yearAreaOption"
                  autoresize
                  class="h-full w-full"
                />
                <template #fallback>
                  <LoaderCircleIcon class="w-5 h-5 animate-spin" />
                </template>
              </ClientOnly>
            </div>
          </DashboardCard>
        </div>
      </UiTabsContent>

      <!-- Tab 2: Quality -->
      <UiTabsContent value="quality">
        <div class="grid grid-cols-1 gap-4">
          <DashboardCard title="Quality Profile Breakdown" :icon="SparklesIcon">
            <div class="h-64 flex items-center justify-center text-muted-foreground text-sm">
              Quality distribution visualization (coming soon)
            </div>
          </DashboardCard>

          <DashboardCard title="Size Anomalies" :icon="AlertTriangleIcon">
            <div
              class="min-h-[200px] flex items-center justify-center text-muted-foreground text-sm"
            >
              Bloat detection table (coming soon)
            </div>
          </DashboardCard>
        </div>
      </UiTabsContent>

      <!-- Tab 3: Watch Intelligence -->
      <UiTabsContent value="watch">
        <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
          <DashboardCard title="Dead Content" :icon="SkullIcon">
            <div class="h-32 flex items-center justify-center text-muted-foreground text-sm">
              Never-watched items report (coming soon)
            </div>
            <template #footer>
              <NuxtLink to="/library?filter=dead" class="text-primary hover:underline">
                View in Library →
              </NuxtLink>
            </template>
          </DashboardCard>

          <DashboardCard title="Stale Content" :icon="ClockIcon">
            <div class="h-32 flex items-center justify-center text-muted-foreground text-sm">
              Watched long ago items report (coming soon)
            </div>
            <template #footer>
              <NuxtLink to="/library?filter=stale" class="text-primary hover:underline">
                View in Library →
              </NuxtLink>
            </template>
          </DashboardCard>

          <DashboardCard title="Popularity Heatmap" :icon="FlameIcon" class="md:col-span-2">
            <div class="h-64 flex items-center justify-center text-muted-foreground text-sm">
              Genre × Year heatmap (coming soon)
            </div>
          </DashboardCard>

          <DashboardCard title="Request Fulfillment" :icon="CheckCircleIcon" class="md:col-span-2">
            <div class="h-32 flex items-center justify-center text-muted-foreground text-sm">
              Request fulfillment tracking (coming soon)
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
  PieChartIcon,
  CalendarIcon,
  AlertTriangleIcon,
  SkullIcon,
  ClockIcon,
  FlameIcon,
  CheckCircleIcon,
  LoaderCircleIcon,
} from 'lucide-vue-next';
import { DashboardCard } from '~/components/ui/dashboard-card';

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

const activeTab = ref('overview');
const api = useApi();
const { isDark } = useAppColorMode();
const { primaryColor } = useThemeColors();

const compositionData = ref<CompositionResponse | null>(null);

onMounted(async () => {
  try {
    compositionData.value = (await api('/api/v1/analytics/composition')) as CompositionResponse;
  } catch {
    // Charts show placeholder — no error toast needed
  }
});

// ─── Chart options ──────────────────────────────────────────────────────────

const qualityDonutOption = computed(() => {
  const data = compositionData.value?.qualityDistribution ?? [];
  const dark = isDark.value;
  return {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'item', formatter: '{b}: {c} ({d}%)' },
    series: [
      {
        type: 'pie',
        radius: ['40%', '70%'],
        avoidLabelOverlap: true,
        label: { color: dark ? '#a1a1aa' : '#71717a', fontSize: 11 },
        data: data.map((d) => ({ name: d.name, value: d.count })),
      },
    ],
  };
});

const genreBarOption = computed(() => {
  const data = (compositionData.value?.genreDistribution ?? []).slice(0, 10);
  const dark = isDark.value;
  const textColor = dark ? '#a1a1aa' : '#71717a';
  return {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis' },
    grid: { top: 10, right: 10, bottom: 30, left: 80 },
    xAxis: { type: 'value', axisLabel: { color: textColor, fontSize: 11 } },
    yAxis: {
      type: 'category',
      data: data.map((d) => d.name).reverse(),
      axisLabel: { color: textColor, fontSize: 11 },
    },
    series: [
      {
        type: 'bar',
        data: data.map((d) => d.count).reverse(),
        itemStyle: { color: primaryColor.value },
      },
    ],
  };
});

const yearAreaOption = computed(() => {
  const data = compositionData.value?.yearDistribution ?? [];
  const dark = isDark.value;
  const textColor = dark ? '#a1a1aa' : '#71717a';
  const sorted = [...data].sort((a, b) => a.name.localeCompare(b.name));
  return {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis' },
    grid: { top: 10, right: 10, bottom: 30, left: 50 },
    xAxis: {
      type: 'category',
      data: sorted.map((d) => d.name),
      axisLabel: { color: textColor, fontSize: 11 },
    },
    yAxis: {
      type: 'value',
      axisLabel: { color: textColor, fontSize: 11 },
    },
    series: [
      {
        type: 'line',
        smooth: true,
        symbol: 'none',
        areaStyle: { opacity: 0.3 },
        data: sorted.map((d) => d.count),
        itemStyle: { color: primaryColor.value },
      },
    ],
  };
});
</script>
