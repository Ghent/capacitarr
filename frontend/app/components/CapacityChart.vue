<template>
  <div class="h-full w-full">
    <!-- Loading -->
    <div
      v-if="loading"
      class="h-full flex items-center justify-center"
    >
      <component
        :is="LoaderCircleIcon"
        class="w-6 h-6 text-primary animate-spin"
      />
    </div>

    <!-- Error -->
    <div
      v-else-if="error"
      class="h-full flex flex-col items-center justify-center text-red-500"
    >
      <component
        :is="AlertTriangleIcon"
        class="w-6 h-6 mb-2"
      />
      <span class="text-sm">Error loading metrics</span>
    </div>

    <!-- No data -->
    <div
      v-else-if="noData"
      class="h-full flex flex-col items-center justify-center text-muted-foreground/70"
    >
      <component
        :is="BarChart3Icon"
        class="w-6 h-6 mb-2"
      />
      <span class="text-sm">Waiting for data…</span>
    </div>

    <!-- Chart -->
    <ClientOnly v-else>
      <apexchart
        type="area"
        height="100%"
        :options="chartOptions"
        :series="series"
      />
    </ClientOnly>
  </div>
</template>

<script setup lang="ts">
import { LoaderCircleIcon, AlertTriangleIcon, BarChart3Icon } from 'lucide-vue-next'
import { formatBytes } from '~/utils/format'
import type { MetricsHistoryResponse, LibraryHistoryRow } from '~/types/api'

const props = defineProps<{
  mode: string
  diskGroupId?: number
  since?: string
}>()

const api = useApi()
const loading = ref(true)
const error = ref(false)
const noData = ref(false)

const { isDark } = useAppColorMode()
const { primaryColor, successColor } = useThemeColors()

interface ChartSeries {
  name: string
  data: [number, number][]
}

const series = ref<ChartSeries[]>([])
const totalCapacity = ref<number | undefined>(undefined)

const chartOptions = computed(() => {
  const dark = isDark.value
  const textColor = dark ? '#a1a1aa' : '#71717a'
  const gridColor = dark ? 'rgba(63,63,70,0.5)' : '#e4e4e7'
  const isPercent = props.mode === 'percentage'
  const isRaw = props.mode === 'raw'

  return {
    chart: {
      type: 'area',
      height: '100%',
      toolbar: { show: false },
      zoom: { enabled: false },
      background: 'transparent',
      fontFamily: 'Geist Sans, Geist, system-ui, sans-serif',
      animations: { enabled: true, easing: 'easeinout', speed: 400 }
    },
    // Theme-aware colors: primary for used, success green for total
    // Raw mode only has one series (Used), so only one color needed
    colors: isPercent || isRaw
      ? [primaryColor.value]
      : [primaryColor.value, successColor.value],
    fill: {
      type: 'gradient',
      gradient: { shadeIntensity: 1, opacityFrom: 0.35, opacityTo: 0.05, stops: [0, 90, 100] }
    },
    dataLabels: { enabled: false },
    stroke: { curve: 'smooth', width: 2 },
    xaxis: {
      type: 'datetime',
      labels: { style: { colors: textColor, fontSize: '11px' }, datetimeUTC: false },
      axisBorder: { show: false },
      axisTicks: { show: false }
    },
    yaxis: {
      min: isPercent ? 0 : undefined,
      max: isPercent ? 100 : (isRaw && totalCapacity.value ? totalCapacity.value : undefined),
      labels: {
        style: { colors: textColor, fontSize: '11px' },
        formatter: (value: number) => isPercent ? `${value.toFixed(0)}%` : formatBytes(value)
      }
    },
    grid: {
      borderColor: gridColor,
      strokeDashArray: 4,
      xaxis: { lines: { show: false } },
      yaxis: { lines: { show: true } }
    },
    theme: { mode: dark ? 'dark' : 'light' },
    tooltip: {
      theme: dark ? 'dark' : 'light',
      x: { format: 'MMM dd, yyyy HH:mm' },
      y: {
        formatter: (value: number) => isPercent ? `${value.toFixed(1)}%` : formatBytes(value)
      }
    },
    legend: {
      show: !isPercent && !isRaw,
      position: 'top',
      horizontalAlign: 'right',
      labels: { colors: textColor }
    }
  }
})

async function fetchMetrics() {
  loading.value = true
  error.value = false
  noData.value = false
  try {
    const query: Record<string, string | number> = { resolution: 'raw' }
    if (props.diskGroupId) query.disk_group_id = props.diskGroupId
    if (props.since && props.since !== 'all') query.since = props.since

    const res = await api('/api/v1/metrics/history', { query }) as MetricsHistoryResponse

    if (res.status === 'success' && res.data?.length > 0) {
      const isPercent = props.mode === 'percentage'
      const isRaw = props.mode === 'raw'

      // Capture total capacity from the last data point for y-axis max
      const lastRow = res.data[res.data.length - 1]
      if (lastRow?.TotalCapacity) {
        totalCapacity.value = lastRow.TotalCapacity
      }

      const usedData = res.data.map((row: LibraryHistoryRow) => {
        const ts = new Date(row.Timestamp).getTime()
        if (isPercent) {
          const pct = row.TotalCapacity > 0 ? (row.UsedCapacity / row.TotalCapacity) * 100 : 0
          return [ts, Math.round(pct * 10) / 10]
        }
        return [ts, row.UsedCapacity]
      })

      if (isPercent) {
        series.value = [{ name: 'Used %', data: usedData }]
      } else if (isRaw) {
        // Raw mode: only show Used series; y-axis max is set to totalCapacity
        series.value = [{ name: 'Used', data: usedData }]
      } else {
        const totalData = res.data.map((row: LibraryHistoryRow) => [
          new Date(row.Timestamp).getTime(),
          row.TotalCapacity
        ])
        series.value = [
          { name: 'Used', data: usedData },
          { name: 'Total', data: totalData }
        ]
      }
    } else {
      noData.value = true
    }
  } catch {
    error.value = true
  } finally {
    loading.value = false
  }
}

onMounted(() => fetchMetrics())

watch(
  () => [props.mode, props.diskGroupId, props.since],
  () => fetchMetrics()
)
</script>
