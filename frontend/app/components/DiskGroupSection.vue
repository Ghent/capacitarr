<template>
  <UiCard
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24 } }"
    class="overflow-hidden"
  >
    <!-- Header with progress -->
    <UiCardContent class="pt-5 pb-0">
      <div class="flex items-center justify-between mb-3">
        <div class="flex items-center gap-3">
          <div
            class="w-10 h-10 rounded-lg flex items-center justify-center shrink-0"
            :class="statusBgColor"
          >
            <component :is="HardDriveIcon" class="w-5 h-5 text-white" />
          </div>
          <div>
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
        <span class="text-2xl font-bold tabular-nums" :class="statusTextColor">
          {{ usagePercent }}%
        </span>
      </div>
    </UiCardContent>

    <!-- Thermometer Bar -->
    <UiCardContent class="pt-3 pb-3">
      <div
        class="h-20"
        :class="{ 'thermometer-critical': rawUsagePct >= (group.thresholdPct || 85) }"
      >
        <ClientOnly>
          <VChart :option="thermometerOption" :autoresize="true" class="h-full w-full" />
        </ClientOnly>
      </div>

      <!-- Free space + link -->
      <div class="flex items-center justify-between mt-1">
        <span class="text-xs text-muted-foreground">
          {{ formatBytes(effectiveTotalBytes - group.usedBytes) }} free
          <span v-if="hasOverride" class="ml-1 text-muted-foreground/50">
            (detected: {{ formatBytes(group.totalBytes) }})
          </span>
        </span>
      </div>
    </UiCardContent>
  </UiCard>
</template>

<script setup lang="ts">
import { HardDriveIcon } from 'lucide-vue-next';
import { formatBytes, diskStatusBgClass, diskStatusTextClass } from '~/utils/format';
import type { DiskGroup } from '~/types/api';

const props = defineProps<{
  group: DiskGroup;
}>();

const api = useApi();

const { successColor, destructiveColor, tooltipConfig, colorAlpha } = useEChartsDefaults();

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

/** Rounded display percentage. */
const usagePercent = computed(() => Math.round(rawUsagePct.value));

const statusBgColor = computed(() =>
  diskStatusBgClass(rawUsagePct.value, props.group.targetPct, props.group.thresholdPct),
);

const statusTextColor = computed(() =>
  diskStatusTextClass(rawUsagePct.value, props.group.targetPct, props.group.thresholdPct),
);

// --- Forecast data for tooltip ---
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
    forecastData.value = (await api('/api/v1/analytics/forecast')) as CapacityForecast;
  } catch {
    // Non-critical — tooltip will just show usage without forecast
  }
}

onMounted(() => {
  fetchForecast();
});

// --- Zone colors ---
const targetPct = computed(() => props.group.targetPct || 75);
const thresholdPct = computed(() => props.group.thresholdPct || 85);

/** Determine the zone color pair (lighter, saturated) for the gradient fill. */
const zoneGradient = computed(() => {
  const pct = rawUsagePct.value;
  if (pct >= thresholdPct.value) {
    // Red zone
    return {
      light: '#fca5a5', // red-300
      saturated: '#ef4444', // red-500
      glow: destructiveColor.value,
    };
  }
  if (pct >= targetPct.value) {
    // Amber zone
    return {
      light: '#fcd34d', // amber-300
      saturated: '#f59e0b', // amber-500
      glow: '#f59e0b',
    };
  }
  // Green zone
  return {
    light: '#6ee7b7', // emerald-300
    saturated: '#10b981', // emerald-500
    glow: successColor.value,
  };
});

// --- ECharts thermometer option ---
const thermometerOption = computed(() => {
  const usage = Math.round(rawUsagePct.value * 10) / 10;
  const tgtPct = targetPct.value;
  const thrPct = thresholdPct.value;
  const grad = zoneGradient.value;
  const usedBytes = props.group.usedBytes;
  const totalBytes = effectiveTotalBytes.value;
  const forecast = forecastData.value;

  // Build tooltip content
  const tooltipFormatter = () => {
    let html = `<div style="font-size:12px">`;
    html += `<strong>${formatBytes(usedBytes)} / ${formatBytes(totalBytes)}</strong> · ${usage}%`;
    if (forecast) {
      if (forecast.growthRatePerDay > 0) {
        html += `<br/><span style="opacity:0.7">Growth: +${formatBytes(forecast.growthRatePerDay)}/day</span>`;
      } else if (forecast.growthRatePerDay < 0) {
        html += `<br/><span style="opacity:0.7">Shrinking: ${formatBytes(Math.abs(forecast.growthRatePerDay))}/day</span>`;
      }
      if (forecast.daysUntilFull > 0) {
        html += `<br/><span style="opacity:0.7">Full in ~${forecast.daysUntilFull} days</span>`;
      }
    }
    html += `</div>`;
    return html;
  };

  return {
    animation: true,
    animationDuration: 1500,
    animationEasing: 'elasticOut',
    animationDurationUpdate: 800,
    animationEasingUpdate: 'cubicOut',
    tooltip: {
      trigger: 'item' as const,
      ...tooltipConfig(),
      formatter: tooltipFormatter,
    },
    grid: {
      top: 28,
      right: 16,
      bottom: 16,
      left: 16,
    },
    xAxis: {
      type: 'value' as const,
      min: 0,
      max: 100,
      show: false,
    },
    yAxis: {
      type: 'category' as const,
      data: ['usage'],
      show: false,
    },
    series: [
      // Background zone segments (behind the fill bar)
      {
        name: 'zones',
        type: 'bar',
        stack: 'bg',
        barWidth: 24,
        silent: true,
        barGap: '-100%',
        z: 1,
        data: [tgtPct],
        itemStyle: {
          color: colorAlpha('#10b981', 0.08),
          borderRadius: [0, 0, 0, 0],
        },
      },
      {
        name: 'zones-amber',
        type: 'bar',
        stack: 'bg',
        barWidth: 24,
        silent: true,
        barGap: '-100%',
        z: 1,
        data: [thrPct - tgtPct],
        itemStyle: {
          color: colorAlpha('#f59e0b', 0.08),
          borderRadius: [0, 0, 0, 0],
        },
      },
      {
        name: 'zones-red',
        type: 'bar',
        stack: 'bg',
        barWidth: 24,
        silent: true,
        barGap: '-100%',
        z: 1,
        data: [100 - thrPct],
        itemStyle: {
          color: colorAlpha('#ef4444', 0.08),
          borderRadius: [0, 6, 6, 0],
        },
      },
      // Usage fill bar
      {
        name: 'usage',
        type: 'bar',
        barWidth: 24,
        z: 2,
        data: [usage],
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
          borderRadius: [0, 6, 6, 0],
          shadowBlur: 12,
          shadowColor: colorAlpha(grad.glow, 0.5),
          shadowOffsetX: 4,
        },
        markLine: {
          silent: true,
          symbol: ['none', 'triangle'],
          symbolSize: [8, 6],
          animation: false,
          data: [
            {
              xAxis: tgtPct,
              lineStyle: { color: '#10b981', type: 'dashed', width: 1 },
              label: {
                show: true,
                formatter: `${tgtPct}%`,
                fontSize: 9,
                color: '#10b981',
                position: 'start',
                offset: Math.abs(tgtPct - thrPct) < 8 ? [-12, 0] : [0, 0],
              },
            },
            {
              xAxis: thrPct,
              lineStyle: { color: '#ef4444', type: 'dashed', width: 1 },
              label: {
                show: true,
                formatter: `${thrPct}%`,
                fontSize: 9,
                color: '#ef4444',
                position: 'start',
                offset: Math.abs(tgtPct - thrPct) < 8 ? [12, 0] : [0, 0],
              },
            },
          ],
        },
      },
    ],
  };
});
</script>

<style scoped>
@keyframes thermometer-pulse {
  0%,
  100% {
    filter: drop-shadow(0 0 6px var(--destructive));
  }
  50% {
    filter: drop-shadow(0 0 14px var(--destructive));
  }
}

.thermometer-critical {
  animation: thermometer-pulse 2s ease-in-out infinite;
}
</style>
