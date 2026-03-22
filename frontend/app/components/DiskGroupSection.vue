<template>
  <UiCard
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24 } }"
    class="overflow-hidden"
  >
    <UiCardContent class="pt-5 pb-4">
      <!-- Top row: icon + path + badges -->
      <div class="flex items-start gap-3 mb-4">
        <div
          class="w-10 h-10 rounded-lg flex items-center justify-center shrink-0"
          :class="statusBgColor"
        >
          <component :is="HardDriveIcon" class="w-5 h-5 text-white" />
        </div>
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-1.5 flex-wrap">
            <h3 class="font-semibold text-sm truncate" :title="group.mountPath">
              {{ group.mountPath }}
            </h3>
            <UiBadge
              v-for="integ in group.integrations || []"
              :key="integ.id"
              variant="outline"
              class="text-[10px] px-1.5 py-0"
            >
              {{ integ.type }}
            </UiBadge>
          </div>
          <span class="text-xs text-muted-foreground">
            {{ formatBytes(group.usedBytes) }} / {{ formatBytes(effectiveTotalBytes) }}
            <UiBadge
              v-if="hasOverride"
              variant="outline"
              class="ml-1 text-amber-600 dark:text-amber-400 border-amber-300 dark:border-amber-600"
              :title="`Detected: ${formatBytes(group.totalBytes)}, Custom: ${formatBytes(effectiveTotalBytes)}`"
            >
              📌 Custom
            </UiBadge>
          </span>
        </div>
      </div>

      <!-- Gauge + Stats layout -->
      <div class="flex items-center gap-4">
        <!-- ECharts Gauge Arc -->
        <div
          class="shrink-0"
          :class="{ 'gauge-critical': rawUsagePct >= (group.thresholdPct || 85) }"
          style="width: 160px; height: 120px"
        >
          <ClientOnly>
            <VChart :option="gaugeOption" :autoresize="true" class="h-full w-full" />
          </ClientOnly>
        </div>

        <!-- Stats panel (right side) -->
        <div class="flex-1 min-w-0 space-y-2">
          <!-- Free space -->
          <div class="flex items-center gap-2">
            <component :is="HardDriveIcon" class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
            <span class="text-sm text-muted-foreground">
              {{ formatBytes(effectiveTotalBytes - group.usedBytes) }} free
            </span>
          </div>

          <!-- Growth rate -->
          <div
            v-if="forecastData && forecastData.growthRatePerDay !== 0"
            class="flex items-center gap-2"
          >
            <component
              :is="forecastData.growthRatePerDay > 0 ? TrendingUpIcon : TrendingDownIcon"
              class="w-3.5 h-3.5 shrink-0"
              :class="forecastData.growthRatePerDay > 0 ? 'text-destructive' : 'text-emerald-500'"
            />
            <span class="text-sm text-muted-foreground">
              {{ forecastData.growthRatePerDay > 0 ? '+' : ''
              }}{{ formatBytes(Math.abs(forecastData.growthRatePerDay)) }}/day
            </span>
          </div>

          <!-- Days until full -->
          <div
            v-if="forecastData && forecastData.daysUntilFull > 0"
            class="flex items-center gap-2"
          >
            <component :is="ClockIcon" class="w-3.5 h-3.5 text-muted-foreground shrink-0" />
            <span
              class="text-sm"
              :class="
                forecastData.daysUntilFull <= 7
                  ? 'text-destructive font-medium'
                  : 'text-muted-foreground'
              "
            >
              Full in ~{{ forecastData.daysUntilFull }}d
            </span>
          </div>

          <!-- Override note -->
          <div v-if="hasOverride" class="text-[10px] text-muted-foreground/50">
            (detected: {{ formatBytes(group.totalBytes) }})
          </div>
        </div>
      </div>
    </UiCardContent>
  </UiCard>
</template>

<script setup lang="ts">
import { HardDriveIcon, TrendingUpIcon, TrendingDownIcon, ClockIcon } from 'lucide-vue-next';
import { formatBytes, diskStatusBgClass } from '~/utils/format';
import type { DiskGroup } from '~/types/api';

const props = defineProps<{
  group: DiskGroup;
  /** Date range from the dashboard dropdown (e.g. '1h', '6h', '24h', '7d', '30d', 'all') */
  dateRange?: string;
}>();

const api = useApi();

const { destructiveColor, successColor, colorAlpha } = useEChartsDefaults();

/** Effective total bytes — override if set, otherwise API-detected. */
const effectiveTotalBytes = computed(() => {
  const override = props.group.totalBytesOverride;
  if (override && override > 0) return override;
  return props.group.totalBytes;
});

/** Whether this disk group has an active user-defined size override. */
const hasOverride = computed(() => {
  return !!(props.group.totalBytesOverride && props.group.totalBytesOverride > 0);
});

/** Raw (unrounded) usage percentage — used for color zone comparisons. */
const rawUsagePct = computed(() => {
  if (effectiveTotalBytes.value === 0) return 0;
  return (props.group.usedBytes / effectiveTotalBytes.value) * 100;
});

const statusBgColor = computed(() =>
  diskStatusBgClass(rawUsagePct.value, props.group.targetPct, props.group.thresholdPct),
);

// --- Forecast data ---
interface CapacityForecast {
  currentUsedPct: number;
  growthRatePerDay: number;
  daysUntilThreshold: number;
  daysUntilFull: number;
  totalCapacity: number;
  usedCapacity: number;
}

const forecastData = ref<CapacityForecast | null>(null);

async function fetchForecast() {
  try {
    forecastData.value = (await api(
      `/api/v1/analytics/forecast?disk_group_id=${props.group.id}`,
    )) as CapacityForecast;
  } catch {
    // Non-critical
  }
}

onMounted(() => {
  fetchForecast();
});

// --- Zone colors ---
const targetPct = computed(() => props.group.targetPct || 75);
const thresholdPct = computed(() => props.group.thresholdPct || 85);

/** Determine the zone color pair for the gradient fill. */
const zoneGradient = computed(() => {
  const pct = rawUsagePct.value;
  if (pct >= thresholdPct.value) {
    return {
      light: '#fca5a5',
      saturated: '#ef4444',
      glow: destructiveColor.value,
    };
  }
  if (pct >= targetPct.value) {
    return {
      light: '#fcd34d',
      saturated: '#f59e0b',
      glow: '#f59e0b',
    };
  }
  return {
    light: '#6ee7b7',
    saturated: '#10b981',
    glow: successColor.value,
  };
});

// --- ECharts Gauge Option ---
const gaugeOption = computed(() => {
  const usage = Math.round(rawUsagePct.value * 10) / 10;
  const tgtPct = targetPct.value;
  const thrPct = thresholdPct.value;
  const grad = zoneGradient.value;

  return {
    animation: true,
    animationDuration: 2000,
    animationEasing: 'elasticOut',
    animationDurationUpdate: 1000,
    animationEasingUpdate: 'cubicOut',
    series: [
      // Main gauge: progress arc + background zones
      {
        type: 'gauge',
        startAngle: 210,
        endAngle: -30,
        min: 0,
        max: 100,
        center: ['50%', '60%'],
        radius: '90%',

        // Background track with zone-colored segments
        axisLine: {
          lineStyle: {
            width: 14,
            color: [
              [tgtPct / 100, colorAlpha('#10b981', 0.12)],
              [thrPct / 100, colorAlpha('#f59e0b', 0.12)],
              [1, colorAlpha('#ef4444', 0.12)],
            ],
          },
          roundCap: true,
        },

        // Progress arc (the filled portion)
        progress: {
          show: true,
          width: 14,
          roundCap: true,
          itemStyle: {
            color: {
              type: 'linear',
              x: 0,
              y: 0,
              x2: 1,
              y2: 0,
              colorStops: [
                { offset: 0, color: grad.light },
                { offset: 1, color: grad.saturated },
              ],
            },
            shadowBlur: 16,
            shadowColor: colorAlpha(grad.glow, 0.6),
          },
        },

        pointer: { show: false },
        axisTick: { show: false },
        splitLine: { show: false },
        axisLabel: { show: false },
        title: { show: false },

        // Center detail: the big percentage number
        detail: {
          valueAnimation: true,
          formatter: '{value}%',
          fontSize: 24,
          fontWeight: 'bold',
          fontFamily: 'var(--font-geist-mono, monospace)',
          color: grad.saturated,
          offsetCenter: [0, '-5%'],
        },

        data: [{ value: usage }],
      },
      // Target marker: small green triangle on INNER edge of arc
      {
        type: 'gauge',
        startAngle: 210,
        endAngle: -30,
        min: 0,
        max: 100,
        center: ['50%', '60%'],
        radius: '90%',
        z: 3,
        axisLine: { show: false },
        progress: { show: false },
        axisTick: { show: false },
        splitLine: { show: false },
        axisLabel: { show: false },
        detail: { show: false },
        title: { show: false },
        pointer: {
          show: true,
          icon: 'triangle',
          length: '12%',
          width: 6,
          offsetCenter: [0, '-62%'],
          itemStyle: { color: '#10b981', opacity: 0.85 },
        },
        data: [{ value: tgtPct }],
        animation: false,
        silent: true,
      },
      // Threshold marker: small red triangle on OUTER edge of arc
      {
        type: 'gauge',
        startAngle: 210,
        endAngle: -30,
        min: 0,
        max: 100,
        center: ['50%', '60%'],
        radius: '90%',
        z: 3,
        axisLine: { show: false },
        progress: { show: false },
        axisTick: { show: false },
        splitLine: { show: false },
        axisLabel: { show: false },
        detail: { show: false },
        title: { show: false },
        pointer: {
          show: true,
          icon: 'triangle',
          length: '12%',
          width: 6,
          offsetCenter: [0, '-92%'],
          itemStyle: { color: '#ef4444', opacity: 0.85 },
        },
        data: [{ value: thrPct }],
        animation: false,
        silent: true,
      },
    ],
  };
});
</script>

<style scoped>
@keyframes gauge-pulse {
  0%,
  100% {
    filter: drop-shadow(0 0 6px var(--destructive));
  }
  50% {
    filter: drop-shadow(0 0 18px var(--destructive));
  }
}

.gauge-critical {
  animation: gauge-pulse 2s ease-in-out infinite;
}
</style>
