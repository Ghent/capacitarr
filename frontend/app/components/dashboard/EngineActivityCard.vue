<template>
  <UiCard
    v-if="engineStats"
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24 } }"
    class="mb-6"
    :class="engineIsRunning ? 'engine-running-glow' : ''"
  >
    <UiCardContent class="pt-5">
      <div
        aria-live="polite"
        class="rounded-lg px-3 py-2 mb-4 flex items-center gap-2 text-sm font-medium"
        :class="engineStatusBannerClass"
      >
        <LoaderCircleIcon v-if="engineIsRunning" class="w-4 h-4 animate-spin shrink-0" />
        <component
          :is="engineIsRunning ? ActivityIcon : CheckCircle2Icon"
          v-else
          class="w-4 h-4 shrink-0"
        />
        <span v-if="engineIsRunning">{{ t('dashboard.engineRunningDetail') }}</span>
        <span v-else-if="!engineLastRunEpoch">{{ t('dashboard.engineIdleNoRuns') }}</span>
        <i18n-t v-else keypath="dashboard.engineIdleLastRun" tag="span">
          <template #time>
            <DateDisplay :date="new Date(engineLastRunEpoch * 1000).toISOString()" />
          </template>
        </i18n-t>
        <span
          v-if="!engineIsRunning && countdownText"
          class="ml-auto text-xs font-normal text-muted-foreground"
        >
          {{ countdownText }}
        </span>
      </div>

      <div class="flex flex-wrap items-center gap-2 mb-3">
        <div class="flex items-center gap-1.5 text-primary font-medium text-sm">
          <ActivityIcon class="w-4 h-4" />
          {{ $t('dashboard.engineActivity') }}
        </div>
        <UiButton
          variant="outline"
          size="sm"
          :disabled="engineRunNowLoading"
          @click="engineTriggerRunNow"
        >
          <LoaderCircleIcon v-if="engineRunNowLoading" class="w-3.5 h-3.5 animate-spin" />
          <PlayIcon v-else class="w-3.5 h-3.5" />
          {{ $t('dashboard.runNow') }}
        </UiButton>
        <span class="text-xs text-muted-foreground">
          <template v-if="engineLastRunEpoch">
            <i18n-t keypath="dashboard.lastRun" tag="span">
              <template #time>
                <DateDisplay :date="new Date(engineLastRunEpoch * 1000).toISOString()" />
              </template>
            </i18n-t>
          </template>
          <template v-else>
            {{ $t('dashboard.noRunsYet') }}
          </template>
        </span>
        <UiBadge
          :variant="
            effectiveMode === MODE_AUTO
              ? 'destructive'
              : effectiveMode === MODE_APPROVAL
                ? 'outline'
                : 'secondary'
          "
          class="ml-auto"
        >
          {{ engineModeLabel(effectiveMode) }}
        </UiBadge>
        <span class="text-xs text-muted-foreground">
          {{ $t('dashboard.evaluated') }} {{ engineLastRunEvaluated?.toLocaleString() ?? 0 }} ·
          {{ $t('dashboard.candidates') }} {{ engineLastRunCandidates?.toLocaleString() ?? 0 }}
        </span>
      </div>

      <div v-if="history.length > 0" class="mb-3">
        <div class="flex items-center gap-3 mb-1">
          <span class="text-[11px] text-muted-foreground/70">
            {{ $t('dashboard.engineActivityTitle') }} · {{ dateRangeLabel }}
          </span>
          <span class="inline-flex items-center gap-1 text-[11px] text-muted-foreground">
            <span class="w-2 h-2 rounded-full bg-primary" />
            {{ $t('dashboard.candidates') }}
          </span>
          <span
            class="inline-flex items-center gap-1 text-[11px] text-muted-foreground"
            :class="{ 'opacity-40': !hasQueuedData }"
          >
            <span class="w-2 h-2 rounded-full bg-amber-500" />
            {{ $t('dashboard.wouldDelete') }}
          </span>
          <span
            class="inline-flex items-center gap-1 text-[11px] text-muted-foreground"
            :class="{ 'opacity-40': !hasDeletedData }"
          >
            <span class="w-2 h-2 rounded-full bg-destructive" />
            {{ $t('dashboard.deleted') }}
          </span>
        </div>
        <ClientOnly>
          <div class="h-[120px] w-full">
            <VChart :option="sparklineEChartsOption" :autoresize="true" class="h-full w-full" />
          </div>
        </ClientOnly>
      </div>

      <UiButton
        v-if="history.length > 0"
        variant="ghost"
        class="h-auto p-0 flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors mb-2"
        @click="showMiniSparklines = !showMiniSparklines"
      >
        <component :is="showMiniSparklines ? ChevronUpIcon : ChevronDownIcon" class="w-3.5 h-3.5" />
        {{ showMiniSparklines ? $t('dashboard.hideDetails') : $t('dashboard.showDetails') }}
      </UiButton>

      <div v-if="showMiniSparklines && history.length > 0" class="grid grid-cols-2 gap-3 mb-3">
        <div class="rounded-lg bg-muted px-3 py-2">
          <div class="text-[11px] text-muted-foreground mb-0.5">
            {{ $t('dashboard.runDuration') }} · {{ dateRangeLabel }}
          </div>
          <div class="text-[11px] text-muted-foreground/70 mb-1">
            {{ $t('dashboard.avgDuration', { avg: avgDurationMs + 'ms' }) }} ·
            {{ $t('dashboard.maxDuration', { max: maxDurationMs + 'ms' }) }}
          </div>
          <ClientOnly>
            <div class="h-[70px] w-full">
              <VChart
                :option="durationSparklineEChartsOption"
                :autoresize="true"
                class="h-full w-full"
              />
            </div>
          </ClientOnly>
        </div>

        <div class="rounded-lg bg-muted px-3 py-2">
          <div class="text-[11px] text-muted-foreground mb-1 flex items-center gap-1">
            <span>{{ $t('dashboard.recentActivity') }}</span>
            <span class="text-muted-foreground/40">·</span>
            <NuxtLink to="/audit" class="text-primary hover:text-primary/80 font-medium">
              {{ $t('dashboard.viewAll') }}
            </NuxtLink>
          </div>
          <div
            v-if="recentActivity.length > 0"
            ref="activityScrollRef"
            class="h-[86px] overflow-auto pr-3"
          >
            <div
              :style="{ height: `${activityVirtualizer.getTotalSize()}px`, position: 'relative' }"
            >
              <div
                v-for="virtualRow in activityVirtualItems"
                :key="virtualRow.index"
                :style="{
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  width: '100%',
                  height: `${virtualRow.size}px`,
                  transform: `translateY(${virtualRow.start}px)`,
                }"
              >
                <div class="flex items-center gap-1.5 py-0.5 text-[11px] leading-tight">
                  <component
                    :is="eventIcon(virtualRow.entry.eventType)"
                    class="w-3 h-3 shrink-0"
                    :class="eventIconClass(virtualRow.entry.eventType)"
                  />
                  <span class="truncate line-clamp-1 flex-1 min-w-0 text-foreground">
                    {{ virtualRow.entry.message }}
                  </span>
                  <span class="text-muted-foreground/70 shrink-0 whitespace-nowrap ml-auto">
                    <DateDisplay :date="virtualRow.entry.createdAt" />
                  </span>
                </div>
              </div>
            </div>
          </div>
          <div
            v-else
            class="flex items-center justify-center text-[11px] text-muted-foreground/60 h-[86px]"
          >
            {{ $t('dashboard.noActivityYet') }}
          </div>
        </div>
      </div>

      <div class="grid grid-cols-3 gap-3 mb-3">
        <div class="rounded-lg bg-muted px-3 py-2">
          <div class="text-[11px] text-muted-foreground mb-0.5">
            {{ anyAutoMode ? $t('dashboard.freed') : $t('dashboard.wouldFree') }}
          </div>
          <div class="text-sm font-bold tabular-nums">
            {{ formatBytes(engineStats.lastRunFreedBytes ?? 0) }}
          </div>
        </div>
        <div class="rounded-lg bg-muted px-3 py-2">
          <div class="text-[11px] text-muted-foreground mb-0.5">
            {{ $t('dashboard.queue') }}
          </div>
          <div class="flex items-center gap-1.5">
            <span
              class="w-2 h-2 rounded-full shrink-0"
              :class="(engineStats.queueDepth ?? 0) > 0 ? 'bg-warning' : 'bg-success'"
            />
            <span class="text-sm font-bold tabular-nums">{{ engineStats.queueDepth ?? 0 }}</span>
            <span class="text-xs text-muted-foreground">{{ $t('common.items') }}</span>
          </div>
          <p
            v-if="(engineStats.queueFullRejections ?? 0) > 0"
            class="text-[11px] text-destructive mt-1"
          >
            {{ $t('deletion.queueFullRejections', { count: engineStats.queueFullRejections }) }}
          </p>
        </div>
        <div class="rounded-lg bg-muted px-3 py-2">
          <div class="text-[11px] text-muted-foreground mb-0.5">
            {{ $t('dashboard.activeDelete') }}
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
            <template v-else-if="allDryRun">
              <span class="text-muted-foreground text-xs">{{
                $t('dashboard.dryRunNoDelete')
              }}</span>
            </template>
            <template v-else-if="(engineStats.queueDepth ?? 0) === 0">
              <span class="text-muted-foreground">{{ $t('common.idle') }}</span>
            </template>
            <template v-else>
              <span class="text-muted-foreground">{{ $t('dashboard.waiting') }}</span>
            </template>
          </div>
        </div>
      </div>

      <NuxtLink
        to="/audit"
        class="text-xs text-primary hover:text-primary/80 font-medium transition-colors"
      >
        {{ $t('dashboard.viewAuditLog') }}
      </NuxtLink>
    </UiCardContent>
  </UiCard>
</template>

<script setup lang="ts">
import {
  ActivityIcon,
  CheckCircle2Icon,
  ChevronDownIcon,
  ChevronUpIcon,
  LoaderCircleIcon,
  PlayIcon,
} from 'lucide-vue-next';
import { useVirtualizer } from '@tanstack/vue-virtual';
import { MODE_APPROVAL, MODE_AUTO } from '~/constants';
import type { ActivityEvent } from '~/types/api';
import { formatBytes } from '~/utils/format';
import { eventIcon, eventIconClass } from '~/utils/eventIcons';
import { STORAGE_KEYS } from '~/utils/storageKeys';

const props = defineProps<{
  recentActivity: ActivityEvent[];
  effectiveMode: string;
  anyAutoMode: boolean;
  allDryRun: boolean;
}>();

const { t } = useI18n();
const {
  workerStats: engineStats,
  lastRunEpoch: engineLastRunEpoch,
  lastRunEvaluated: engineLastRunEvaluated,
  lastRunCandidates: engineLastRunCandidates,
  isRunning: engineIsRunning,
  pollIntervalSeconds: enginePollInterval,
  runNowLoading: engineRunNowLoading,
  modeLabel: engineModeLabel,
  triggerRunNow: engineTriggerRunNow,
} = useEngineControl();

const {
  history,
  dateRangeLabel,
  hasQueuedData,
  hasDeletedData,
  avgDurationMs,
  maxDurationMs,
  sparklineEChartsOption,
  durationSparklineEChartsOption,
} = useEngineHistory();

const engineStatusBannerClass = computed(() => {
  if (engineIsRunning.value) {
    return 'bg-primary/10 text-primary border border-primary/20';
  }
  return 'bg-muted text-muted-foreground';
});

const nowEpoch = ref(Math.floor(Date.now() / 1000));
let countdownTimer: ReturnType<typeof setInterval> | null = null;
onMounted(() => {
  countdownTimer = setInterval(() => {
    nowEpoch.value = Math.floor(Date.now() / 1000);
  }, 1000);
});
onUnmounted(() => {
  if (countdownTimer) clearInterval(countdownTimer);
});

const countdownText = computed(() => {
  if (engineIsRunning.value) return '';
  if (!engineLastRunEpoch.value || !enginePollInterval.value) return '';
  const nextRunEpoch = engineLastRunEpoch.value + enginePollInterval.value;
  const remaining = nextRunEpoch - nowEpoch.value;
  if (remaining <= 0) return t('dashboard.nextRunImminent');
  if (remaining < 60) return t('dashboard.nextRunSeconds', { seconds: remaining });
  if (remaining < 3600) {
    const mins = Math.floor(remaining / 60);
    const secs = remaining % 60;
    return t('dashboard.nextRunMinSec', { min: mins, sec: secs });
  }
  const hours = Math.floor(remaining / 3600);
  const mins = Math.floor((remaining % 3600) / 60);
  return t('dashboard.nextRunHourMin', { hour: hours, min: mins });
});

const showMiniSparklines = ref(
  import.meta.client ? localStorage.getItem(STORAGE_KEYS.sparklines) !== 'false' : true,
);
watch(showMiniSparklines, (val) => {
  if (import.meta.client) {
    localStorage.setItem(STORAGE_KEYS.sparklines, String(val));
  }
});

const activityScrollRef = ref<HTMLElement | null>(null);
const activityVirtualizer = useVirtualizer(
  computed(() => ({
    count: props.recentActivity.length,
    getScrollElement: () => activityScrollRef.value,
    estimateSize: () => 20,
    overscan: 5,
  })),
);
const activityVirtualItems = computed(() =>
  activityVirtualizer.value.getVirtualItems().map((row) => ({
    ...row,
    entry: props.recentActivity[row.index]!,
  })),
);
</script>
