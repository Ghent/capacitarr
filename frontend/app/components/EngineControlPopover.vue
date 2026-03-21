<template>
  <UiPopover>
    <UiPopoverTrigger as-child>
      <UiButton
        variant="ghost"
        size="icon"
        aria-label="Engine controls"
        class="relative"
        :class="isRunning ? 'text-primary animate-pulse' : ''"
      >
        <!-- Mode icon: shield=dry-run, hand=approval, zap=auto -->
        <component
          :is="isRunning ? LoaderCircleIcon : modeIcon"
          :class="['w-5 h-5', isRunning ? 'animate-spin' : '']"
        />
        <!-- Health status dot -->
        <span
          aria-live="polite"
          :aria-label="isRunning ? 'Engine running' : 'Engine idle'"
          class="absolute top-1 right-1 w-2.5 h-2.5 rounded-full border-2 border-background"
          :class="[statusDotColor, isRunning || runNowLoading ? 'animate-pulse' : '']"
        />
      </UiButton>
    </UiPopoverTrigger>
    <UiPopoverContent align="center" :side="props.side" class="w-72">
      <div v-motion v-bind="scaleIn" class="space-y-4">
        <!-- Header -->
        <div class="flex items-center justify-between">
          <h4 class="font-semibold text-sm">
            {{ $t('engine.control') }}
          </h4>
          <UiBadge
            :variant="
              executionMode === 'auto'
                ? 'destructive'
                : executionMode === 'approval'
                  ? 'outline'
                  : 'secondary'
            "
          >
            {{ modeLabel(executionMode) }}
          </UiBadge>
        </div>

        <!-- Mode Toggle -->
        <div class="space-y-1.5">
          <span class="text-xs text-muted-foreground font-medium">{{
            $t('engine.executionMode')
          }}</span>
          <div class="flex gap-1">
            <UiButton
              v-for="mode in modes"
              :key="mode.value"
              :variant="executionMode === mode.value ? 'default' : 'outline'"
              size="sm"
              class="flex-1 text-xs"
              :disabled="changingMode"
              @click="handleModeChange(mode.value)"
            >
              {{ mode.label }}
            </UiButton>
          </div>
        </div>

        <!-- Stats -->
        <div class="grid grid-cols-2 gap-2 text-xs">
          <div class="rounded-lg bg-muted px-2.5 py-1.5">
            <div class="text-muted-foreground">
              {{ $t('engine.lastRun') }}
            </div>
            <div class="font-medium">
              <DateDisplay
                v-if="lastRunEpoch"
                :date="new Date(lastRunEpoch * 1000).toISOString()"
              />
              <span v-else>Never</span>
            </div>
          </div>
          <div class="rounded-lg bg-muted px-2.5 py-1.5">
            <div class="text-muted-foreground">
              {{ $t('engine.queue') }}
            </div>
            <div class="font-medium">{{ queueDepth }} items</div>
          </div>
          <div class="rounded-lg bg-muted px-2.5 py-1.5">
            <div class="text-muted-foreground">
              {{ $t('engine.evaluated') }}
            </div>
            <div class="font-medium">
              {{ lastRunEvaluated }}
            </div>
          </div>
          <div class="rounded-lg bg-muted px-2.5 py-1.5">
            <div class="text-muted-foreground">
              {{ $t('engine.flagged') }}
            </div>
            <div class="font-medium">
              {{ lastRunFlagged }}
            </div>
          </div>
        </div>

        <!-- Run Now -->
        <UiButton class="w-full" :disabled="runNowLoading" @click="triggerRunNow">
          <LoaderCircleIcon v-if="runNowLoading" class="w-4 h-4 animate-spin" />
          <PlayIcon v-else class="w-4 h-4" />
          {{ executionMode === 'dry-run' ? $t('engine.dryRun') : $t('engine.runNow') }}
        </UiButton>
      </div>
    </UiPopoverContent>
  </UiPopover>

  <!-- Auto mode confirmation dialog -->
  <UiDialog :open="showAutoConfirm" @update:open="(v: boolean) => (showAutoConfirm = v)">
    <UiDialogContent class="max-w-sm">
      <UiDialogHeader>
        <UiDialogTitle class="text-destructive">
          {{ $t('engine.enableAutoMode') }}
        </UiDialogTitle>
      </UiDialogHeader>
      <p class="text-sm text-muted-foreground">
        {{ $t('engine.autoModeWarning') }}
      </p>
      <UiDialogFooter>
        <UiButton variant="ghost" @click="showAutoConfirm = false">
          {{ $t('common.cancel') }}
        </UiButton>
        <UiButton variant="destructive" @click="confirmAutoMode">
          {{ $t('engine.enableAutoModeConfirm') }}
        </UiButton>
      </UiDialogFooter>
    </UiDialogContent>
  </UiDialog>
</template>

<script setup lang="ts">
import { ShieldIcon, HandIcon, ZapIcon, PlayIcon, LoaderCircleIcon } from 'lucide-vue-next';

const { scaleIn } = useMotionPresets();

const props = withDefaults(
  defineProps<{
    /** Which side the popover opens toward. Defaults to 'bottom'. */
    side?: 'top' | 'bottom' | 'left' | 'right';
  }>(),
  { side: 'bottom' },
);

const {
  executionMode,
  lastRunEpoch,
  lastRunEvaluated,
  lastRunFlagged,
  queueDepth,
  isRunning,
  runNowLoading,
  changingMode,
  modeLabel,
  fetchStats,
  setMode,
  triggerRunNow,
} = useEngineControl();

const showAutoConfirm = ref(false);

const modes = [
  { value: 'dry-run', label: 'Dry-Run' },
  { value: 'approval', label: 'Approval' },
  { value: 'auto', label: 'Auto' },
];

// Mode icon — distinct shape per mode, NOT color-coded
const modeIcon = computed(() => {
  switch (executionMode.value) {
    case 'auto':
      return ZapIcon; // ⚡ auto = lightning bolt
    case 'approval':
      return HandIcon; // ✋ manual review
    default:
      return ShieldIcon; // 🛡️ dry-run = protected/safe
  }
});

// Health status dot — green=healthy, green+pulse=running, amber=loading
const statusDotColor = computed(() => {
  if (isRunning.value) return 'bg-primary';
  if (runNowLoading.value) return 'bg-amber-500';
  return 'bg-green-500';
});

function handleModeChange(mode: string) {
  if (mode === 'auto') {
    showAutoConfirm.value = true;
  } else {
    setMode(mode);
  }
}

async function confirmAutoMode() {
  showAutoConfirm.value = false;
  await setMode('auto');
}

// Fetch stats on mount for initial hydration.
// Ongoing updates arrive via SSE (engine_start / engine_complete / engine_error).
onMounted(() => {
  fetchStats();
});
</script>
