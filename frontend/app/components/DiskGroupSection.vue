<template>
  <UiCard
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24 } }"
    class="overflow-hidden"
  >
    <!-- Header with progress -->
    <UiCardContent class="pt-5 pb-0 border-b border-border">
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

      <!-- Progress Bar with segmented zone background + triangle markers -->
      <div class="relative w-full mt-8 mb-6">
        <!-- Bar container -->
        <div class="relative w-full h-3 rounded-full overflow-hidden">
          <!-- Segmented background zones -->
          <div class="absolute inset-0 flex">
            <!-- Green zone: 0% → target% -->
            <div
              class="h-full"
              :style="{
                width: (group.targetPct || 75) + '%',
                backgroundColor: 'oklch(0.648 0.2 160 / 0.2)',
              }"
            />
            <!-- Amber zone: target% → threshold% -->
            <div
              class="h-full"
              :style="{
                width: (group.thresholdPct || 85) - (group.targetPct || 75) + '%',
                backgroundColor: 'oklch(0.75 0.183 55.934 / 0.2)',
              }"
            />
            <!-- Red zone: threshold% → 100% -->
            <div
              class="h-full"
              :style="{
                width: 100 - (group.thresholdPct || 85) + '%',
                backgroundColor: 'oklch(0.577 0.245 27.325 / 0.2)',
              }"
            />
          </div>
          <!-- Usage fill bar (on top of zones) -->
          <div
            data-slot="progress-bar-fill"
            role="progressbar"
            :aria-valuenow="usagePercent"
            aria-valuemin="0"
            aria-valuemax="100"
            :aria-label="`Disk usage: ${usagePercent}%`"
            :data-status="diskUsageStatus(rawUsagePct, group.targetPct, group.thresholdPct)"
            class="relative h-full rounded-full transition-all duration-700 ease-out z-10"
            :style="{ width: usagePercent + '%', backgroundColor: barFillColor }"
          />
        </div>

        <!-- Target marker ABOVE the bar -->
        <div
          v-if="group.targetPct"
          class="absolute bottom-3 flex flex-col items-center z-20"
          :style="{ left: group.targetPct + '%', transform: 'translateX(-50%)' }"
        >
          <span class="text-[10px] font-medium text-emerald-500 whitespace-nowrap mb-0.5">
            Target {{ group.targetPct }}%
          </span>
          <span class="text-emerald-500 text-[10px] leading-none mb-0.5">▼</span>
        </div>
        <!-- Threshold marker BELOW the bar -->
        <div
          v-if="group.thresholdPct"
          class="absolute top-3 flex flex-col items-center z-20"
          :style="{ left: group.thresholdPct + '%', transform: 'translateX(-50%)' }"
        >
          <span class="text-red-500 text-[10px] leading-none mt-0.5">▲</span>
          <span class="text-[10px] font-medium text-red-500 whitespace-nowrap mt-0.5">
            Threshold {{ group.thresholdPct }}%
          </span>
        </div>
      </div>

      <!-- Free space info -->
      <div class="text-xs text-muted-foreground pb-4">
        <span>{{ formatBytes(effectiveTotalBytes - group.usedBytes) }} free</span>
        <span v-if="hasOverride" class="ml-1 text-muted-foreground/50">
          (detected: {{ formatBytes(group.totalBytes) }})
        </span>
      </div>
    </UiCardContent>

    <!-- Chart Area -->
    <UiCardContent class="pt-3 pb-1">
      <span class="text-xs font-medium text-muted-foreground">
        Capacity · {{ dateRangeLabel }}
      </span>
      <div class="h-64 pt-1">
        <CapacityChart
          :key="`chart-${group.id}-${refreshKey || 0}`"
          :mode="chartMode"
          :disk-group-id="group.id"
          :since="dateRange"
        />
      </div>
    </UiCardContent>
  </UiCard>
</template>

<script setup lang="ts">
import { HardDriveIcon } from 'lucide-vue-next';
import {
  formatBytes,
  diskUsageStatus,
  diskStatusBgClass,
  diskStatusTextClass,
  diskStatusFillColor,
} from '~/utils/format';
import type { DiskGroup } from '~/types/api';

const props = defineProps<{
  group: DiskGroup;
  chartMode: string;
  dateRange: string;
  dateRangeLabel: string;
  refreshKey?: number;
}>();

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

/** Inline fill color for the progress bar (bypasses Tailwind alpha issues). */
const barFillColor = computed(() =>
  diskStatusFillColor(rawUsagePct.value, props.group.targetPct, props.group.thresholdPct),
);
</script>
