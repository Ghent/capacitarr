<template>
  <div class="h-full w-full">
    <div v-if="loading" class="h-full flex items-center justify-center">
      <UIcon name="i-heroicons-arrow-path" class="w-8 h-8 text-indigo-500 animate-spin" />
    </div>
    <div v-else-if="error" class="h-full flex flex-col items-center justify-center text-red-500">
      <UIcon name="i-heroicons-exclamation-triangle" class="w-8 h-8 mb-2" />
      <span>Error loading metrics</span>
    </div>
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
const props = defineProps<{
  resolution: string
}>()

const api = useApi()
const loading = ref(true)
const error = ref(false)

const colorMode = useColorMode()

const series = ref([{
  name: 'Used Capacity',
  data: [] as [number, number][]
}, {
  name: 'Total Capacity',
  data: [] as [number, number][]
}])

const chartOptions = computed(() => {
  const isDark = colorMode.value === 'dark'
  const textColor = isDark ? '#94a3b8' : '#64748b' // slate-400 / slate-500
  const gridColor = isDark ? '#334155' : '#e2e8f0' // slate-700 / slate-200

  return {
    chart: {
      type: 'area',
      height: '100%',
      toolbar: {
        show: false
      },
      zoom: {
        enabled: false
      },
      background: 'transparent',
      fontFamily: 'inherit'
    },
    colors: ['#f59e0b', '#6366f1'], // amber-500, indigo-500
    fill: {
      type: 'gradient',
      gradient: {
        shadeIntensity: 1,
        opacityFrom: 0.4,
        opacityTo: 0.1,
        stops: [0, 90, 100]
      }
    },
    dataLabels: {
      enabled: false
    },
    stroke: {
      curve: 'smooth',
      width: 2
    },
    xaxis: {
      type: 'datetime',
      labels: {
        style: {
          colors: textColor
        }
      },
      axisBorder: {
        show: false
      },
      axisTicks: {
        show: false
      }
    },
    yaxis: {
      labels: {
        style: {
          colors: textColor
        },
        formatter: (value: number) => {
          return `${(value / (1024 * 1024 * 1024)).toFixed(0)} GB`
        }
      }
    },
    grid: {
      borderColor: gridColor,
      strokeDashArray: 4,
      xaxis: {
        lines: {
          show: true
        }
      },
      yaxis: {
        lines: {
          show: true
        }
      }
    },
    theme: {
      mode: isDark ? 'dark' : 'light'
    },
    tooltip: {
      theme: isDark ? 'dark' : 'light',
      y: {
        formatter: (value: number) => {
          return `${(value / (1024 * 1024 * 1024)).toFixed(2)} GB`
        }
      }
    },
    legend: {
      position: 'top',
      horizontalAlign: 'right',
      labels: {
        colors: textColor
      }
    }
  }
})

async function fetchMetrics() {
  loading.value = true
  error.value = false
  try {
    const res = await api('/api/v1/metrics/history', {
      query: { resolution: props.resolution }
    })
    
    if (res.status === 'success' && res.data) {
      const usedData = res.data.map((row: any) => [new Date(row.Timestamp).getTime(), row.UsedCapacity])
      const totalData = res.data.map((row: any) => [new Date(row.Timestamp).getTime(), row.TotalCapacity])
      
      series.value = [
        { name: 'Used Capacity', data: usedData },
        { name: 'Total Capacity', data: totalData }
      ]
    }
  } catch (err) {
    console.error('Failed to grab history data:', err)
    error.value = true
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchMetrics()
})

watch(() => props.resolution, () => {
  fetchMetrics()
})
</script>
