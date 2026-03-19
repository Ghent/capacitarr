<template>
  <div class="h-full w-full">
    <!-- Loading -->
    <div v-if="loading" class="h-full flex items-center justify-center">
      <component :is="LoaderCircleIcon" class="w-6 h-6 text-primary animate-spin" />
    </div>

    <!-- Error -->
    <div v-else-if="error" class="h-full flex flex-col items-center justify-center text-red-500">
      <component :is="AlertTriangleIcon" class="w-6 h-6 mb-2" />
      <span class="text-sm">Error loading metrics</span>
    </div>

    <!-- No data -->
    <div
      v-else-if="noData"
      class="h-full flex flex-col items-center justify-center text-muted-foreground/70"
    >
      <component :is="BarChart3Icon" class="w-6 h-6 mb-2" />
      <span class="text-sm">Waiting for data…</span>
    </div>

    <!-- Chart -->
    <ClientOnly v-else>
      <v-chart :option="chartOption" autoresize class="h-full w-full" />
    </ClientOnly>
  </div>
</template>

<script setup lang="ts">
import { LoaderCircleIcon, AlertTriangleIcon, BarChart3Icon } from 'lucide-vue-next';
import { formatBytes } from '~/utils/format';
import type { MetricsHistoryResponse, LibraryHistoryRow } from '~/types/api';

const props = defineProps<{
  mode: string;
  diskGroupId?: number;
  since?: string;
}>();

const api = useApi();
const loading = ref(true);
const error = ref(false);
const noData = ref(false);

const { isDark } = useAppColorMode();
const { primaryColor, successColor } = useThemeColors();

interface ChartSeries {
  name: string;
  data: [number, number][];
}

const seriesData = ref<ChartSeries[]>([]);
const totalCapacity = ref<number | undefined>(undefined);

const chartOption = computed(() => {
  const dark = isDark.value;
  const textColor = dark ? '#a1a1aa' : '#71717a';
  const gridColor = dark ? 'rgba(63,63,70,0.5)' : '#e4e4e7';
  const isPercent = props.mode === 'percentage';
  const isRaw = props.mode === 'raw';

  const colors =
    isPercent || isRaw ? [primaryColor.value] : [primaryColor.value, successColor.value];

  return {
    backgroundColor: 'transparent',
    textStyle: { fontFamily: 'Geist Sans, Geist, system-ui, sans-serif' },
    color: colors,
    tooltip: {
      trigger: 'axis',
      backgroundColor: dark ? 'rgba(30,30,30,0.9)' : 'rgba(255,255,255,0.96)',
      borderColor: dark ? '#333' : '#e3e3e3',
      textStyle: { color: dark ? '#fff' : '#333' },
      valueFormatter: (value: number) => (isPercent ? `${value.toFixed(1)}%` : formatBytes(value)),
    },
    legend: {
      show: !isPercent && !isRaw,
      top: 0,
      right: 0,
      textStyle: { color: textColor },
    },
    grid: { top: 30, right: 10, bottom: 30, left: 60, containLabel: false },
    xAxis: {
      type: 'time',
      axisLabel: { color: textColor, fontSize: 11 },
      axisLine: { show: false },
      axisTick: { show: false },
      splitLine: { show: false },
    },
    yAxis: {
      type: 'value',
      min: isPercent ? 0 : undefined,
      max: isPercent ? 100 : isRaw && totalCapacity.value ? totalCapacity.value : undefined,
      axisLabel: {
        color: textColor,
        fontSize: 11,
        formatter: (value: number) => (isPercent ? `${value.toFixed(0)}%` : formatBytes(value)),
      },
      splitLine: { lineStyle: { color: gridColor, type: 'dashed' as const } },
    },
    series: seriesData.value.map((s, idx) => ({
      name: s.name,
      type: 'line',
      smooth: true,
      symbol: 'none',
      lineStyle: { width: 2 },
      areaStyle: {
        opacity: 0.35,
        color: {
          type: 'linear',
          x: 0,
          y: 0,
          x2: 0,
          y2: 1,
          colorStops: [
            { offset: 0, color: colors[idx] || colors[0] },
            { offset: 1, color: 'transparent' },
          ],
        },
      },
      data: s.data,
    })),
    animationDuration: 400,
    animationEasing: 'cubicInOut',
  };
});

async function fetchMetrics() {
  loading.value = true;
  error.value = false;
  noData.value = false;
  try {
    const query: Record<string, string | number> = { resolution: 'raw' };
    if (props.diskGroupId) query.disk_group_id = props.diskGroupId;
    if (props.since && props.since !== 'all') query.since = props.since;

    const res = (await api('/api/v1/metrics/history', { query })) as MetricsHistoryResponse;

    if (res.status === 'success' && res.data?.length > 0) {
      const isPercent = props.mode === 'percentage';
      const isRaw = props.mode === 'raw';

      // Capture total capacity from the last data point for y-axis max
      const lastRow = res.data[res.data.length - 1];
      if (lastRow?.totalCapacity) {
        totalCapacity.value = lastRow.totalCapacity;
      }

      const usedData: [number, number][] = res.data.map((row: LibraryHistoryRow) => {
        const ts = new Date(row.timestamp).getTime();
        if (isPercent) {
          const pct = row.totalCapacity > 0 ? (row.usedCapacity / row.totalCapacity) * 100 : 0;
          return [ts, Math.round(pct * 10) / 10] as [number, number];
        }
        return [ts, row.usedCapacity] as [number, number];
      });

      if (isPercent) {
        seriesData.value = [{ name: 'Used %', data: usedData }];
      } else if (isRaw) {
        seriesData.value = [{ name: 'Used', data: usedData }];
      } else {
        const totalData: [number, number][] = res.data.map(
          (row: LibraryHistoryRow) =>
            [new Date(row.timestamp).getTime(), row.totalCapacity] as [number, number],
        );
        seriesData.value = [
          { name: 'Used', data: usedData },
          { name: 'Total', data: totalData },
        ];
      }
    } else {
      noData.value = true;
    }
  } catch {
    error.value = true;
  } finally {
    loading.value = false;
  }
}

onMounted(() => fetchMetrics());

watch(
  () => [props.mode, props.diskGroupId, props.since],
  () => fetchMetrics(),
);
</script>
