<template>
  <UiCard
    v-if="diskGroups.length > 0"
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24 } }"
    class="mb-6"
  >
    <UiCardHeader>
      <UiCardTitle>{{ $t('rules.diskThresholds') }}</UiCardTitle>
      <UiCardDescription>
        {{ $t('rules.diskThresholdsDesc') }}
      </UiCardDescription>
    </UiCardHeader>
    <UiCardContent class="space-y-5">
      <div
        v-for="dg in diskGroups"
        :key="dg.id"
        class="rounded-lg border border-border bg-muted/50 p-5 space-y-4"
      >
        <!-- Mount path & current usage -->
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div
              class="w-9 h-9 rounded-lg flex items-center justify-center shrink-0"
              :class="
                diskStatusBgClass(
                  diskUsagePct(dg),
                  thresholdEdits[dg.id]?.target ?? dg.targetPct,
                  thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct,
                )
              "
            >
              <component :is="HardDriveIcon" class="w-4.5 h-4.5 text-white" />
            </div>
            <div>
              <div class="text-sm font-medium text-foreground truncate" :title="dg.mountPath">
                {{ dg.mountPath }}
              </div>
              <span class="text-xs text-muted-foreground">
                {{ formatBytes(dg.usedBytes) }} / {{ formatBytes(dg.totalBytes) }}
              </span>
            </div>
          </div>
          <span
            class="text-2xl font-bold tabular-nums"
            :class="
              diskStatusTextClass(
                diskUsagePct(dg),
                thresholdEdits[dg.id]?.target ?? dg.targetPct,
                thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct,
              )
            "
          >
            {{ Math.round(diskUsagePct(dg)) }}%
          </span>
        </div>

        <!-- Progress bar with segmented zone background + triangle markers -->
        <div class="relative w-full mt-8 mb-6">
          <!-- Bar container -->
          <div class="relative w-full h-3 rounded-full overflow-hidden">
            <!-- Segmented background zones -->
            <div class="absolute inset-0 flex">
              <div
                class="h-full"
                :style="{
                  width: (thresholdEdits[dg.id]?.target ?? dg.targetPct) + '%',
                  backgroundColor: 'oklch(0.648 0.2 160 / 0.2)',
                }"
              />
              <div
                class="h-full"
                :style="{
                  width:
                    (thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct) -
                    (thresholdEdits[dg.id]?.target ?? dg.targetPct) +
                    '%',
                  backgroundColor: 'oklch(0.75 0.183 55.934 / 0.2)',
                }"
              />
              <div
                class="h-full"
                :style="{
                  width: 100 - (thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct) + '%',
                  backgroundColor: 'oklch(0.577 0.245 27.325 / 0.2)',
                }"
              />
            </div>
            <!-- Usage fill bar -->
            <div
              data-slot="progress-bar-fill"
              role="progressbar"
              :aria-valuenow="Math.round(diskUsagePct(dg))"
              aria-valuemin="0"
              aria-valuemax="100"
              :aria-label="`Disk usage: ${Math.round(diskUsagePct(dg))}%`"
              :data-status="
                diskUsageStatus(
                  diskUsagePct(dg),
                  thresholdEdits[dg.id]?.target ?? dg.targetPct,
                  thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct,
                )
              "
              class="relative h-full rounded-full transition-all duration-700 ease-out z-10"
              :style="{
                width: Math.min(diskUsagePct(dg), 100) + '%',
                backgroundColor: diskStatusFillColor(
                  diskUsagePct(dg),
                  thresholdEdits[dg.id]?.target ?? dg.targetPct,
                  thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct,
                ),
              }"
            />
          </div>

          <!-- Target marker ABOVE the bar -->
          <div
            class="absolute bottom-3 flex flex-col items-center z-20"
            :style="{
              left: (thresholdEdits[dg.id]?.target ?? dg.targetPct) + '%',
              transform: 'translateX(-50%)',
            }"
          >
            <span
              class="text-[10px] font-medium text-emerald-600 dark:text-emerald-400 whitespace-nowrap mb-0.5"
            >
              Target {{ thresholdEdits[dg.id]?.target ?? dg.targetPct }}%
            </span>
            <span class="text-emerald-500 text-[10px] leading-none mb-0.5">▼</span>
          </div>
          <!-- Threshold marker BELOW the bar -->
          <div
            class="absolute top-3 flex flex-col items-center z-20"
            :style="{
              left: (thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct) + '%',
              transform: 'translateX(-50%)',
            }"
          >
            <span class="text-red-500 text-[10px] leading-none mt-0.5">▲</span>
            <span
              class="text-[10px] font-medium text-red-500 dark:text-red-400 whitespace-nowrap mt-0.5"
            >
              Threshold {{ thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct }}%
            </span>
          </div>
        </div>

        <!-- Free space info -->
        <div class="text-xs text-muted-foreground/70">
          <span>{{ formatBytes(dg.totalBytes - dg.usedBytes) }} free</span>
        </div>

        <!-- Editable inputs -->
        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div class="space-y-1.5">
            <UiLabel>{{ $t('rules.cleanupThreshold') }}</UiLabel>
            <div class="flex items-center gap-2">
              <UiInput
                :model-value="String(thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct)"
                type="number"
                min="1"
                max="99"
                @update:model-value="
                  (v: string | number) => updateThresholdEdit(dg.id, 'threshold', Number(v), dg)
                "
              />
              <span class="w-2 h-2 rounded-full bg-red-400 shrink-0" />
            </div>
            <p class="text-[11px] text-muted-foreground">
              {{ $t('rules.cleanupThresholdDesc') }}
            </p>
          </div>
          <div class="space-y-1.5">
            <UiLabel>{{ $t('rules.cleanupTarget') }}</UiLabel>
            <div class="flex items-center gap-2">
              <UiInput
                :model-value="String(thresholdEdits[dg.id]?.target ?? dg.targetPct)"
                type="number"
                min="1"
                max="99"
                @update:model-value="
                  (v: string | number) => updateThresholdEdit(dg.id, 'target', Number(v), dg)
                "
              />
              <span class="w-2 h-2 rounded-full bg-emerald-500 shrink-0" />
            </div>
            <p class="text-[11px] text-muted-foreground">
              {{ $t('rules.cleanupTargetDesc') }}
            </p>
          </div>
        </div>

        <!-- Validation error -->
        <p v-if="thresholdValidation(dg.id, dg)" class="text-xs text-red-500">
          {{ thresholdValidation(dg.id, dg) }}
        </p>

        <!-- Auto-save status indicator -->
        <div class="flex items-center gap-2 h-5">
          <Transition
            enter-active-class="transition-all duration-300 ease-out"
            leave-active-class="transition-all duration-300 ease-in"
            enter-from-class="opacity-0 translate-y-1"
            enter-to-class="opacity-100 translate-y-0"
            leave-from-class="opacity-100 translate-y-0"
            leave-to-class="opacity-0 translate-y-1"
          >
            <span
              v-if="thresholdEdits[dg.id]?.saving"
              class="inline-flex items-center gap-1.5 text-xs text-muted-foreground"
            >
              <component :is="LoaderCircleIcon" class="w-3.5 h-3.5 animate-spin" />
              {{ $t('common.saving') }}
            </span>
            <span
              v-else-if="thresholdEdits[dg.id]?.success && thresholdEdits[dg.id]?.message"
              class="inline-flex items-center gap-1.5 text-xs text-emerald-500"
            >
              <component :is="CheckIcon" class="w-3.5 h-3.5" />
              {{ $t('common.saved') }}
            </span>
            <span
              v-else-if="thresholdEdits[dg.id]?.message && !thresholdEdits[dg.id]?.success"
              class="inline-flex items-center gap-1.5 text-xs text-red-500"
            >
              {{ thresholdEdits[dg.id]?.message }}
            </span>
          </Transition>
        </div>
      </div>
    </UiCardContent>
  </UiCard>
</template>

<script setup lang="ts">
import { HardDriveIcon, LoaderCircleIcon, CheckIcon } from 'lucide-vue-next';
import {
  formatBytes,
  diskUsageStatus,
  diskStatusBgClass,
  diskStatusTextClass,
  diskStatusFillColor,
} from '~/utils/format';
import type { DiskGroup, ApiError } from '~/types/api';

const props = defineProps<{
  diskGroups: DiskGroup[];
}>();

const emit = defineEmits<{
  'update:diskGroup': [diskGroup: DiskGroup];
}>();

const api = useApi();
const { addToast } = useToast();

// Per-disk-group threshold editing state
const thresholdEdits = reactive<
  Record<
    number,
    {
      threshold: number;
      target: number;
      saving: boolean;
      message: string;
      success: boolean;
    }
  >
>({});

function diskUsagePct(dg: DiskGroup): number {
  if (!dg.totalBytes || dg.totalBytes === 0) return 0;
  return (dg.usedBytes / dg.totalBytes) * 100;
}

function ensureThresholdEdit(dgId: number, dg: DiskGroup) {
  if (!thresholdEdits[dgId]) {
    thresholdEdits[dgId] = {
      threshold: dg.thresholdPct,
      target: dg.targetPct,
      saving: false,
      message: '',
      success: false,
    };
  }
}

// Debounce timers for auto-save per disk group
const debounceTimers: Record<number, ReturnType<typeof setTimeout>> = {};

function updateThresholdEdit(
  dgId: number,
  field: 'threshold' | 'target',
  value: number,
  dg: DiskGroup,
) {
  ensureThresholdEdit(dgId, dg);
  const edit = thresholdEdits[dgId]!;
  edit[field] = value;
  edit.message = '';
  edit.success = false;

  // Cancel any pending debounce for this disk group
  if (debounceTimers[dgId]) {
    clearTimeout(debounceTimers[dgId]);
  }

  // Auto-save after 1 second debounce (skip if validation fails)
  debounceTimers[dgId] = setTimeout(() => {
    if (!thresholdValidation(dgId, dg)) {
      saveThresholds(dg);
    }
  }, 1000);
}

function thresholdValidation(dgId: number, dg: DiskGroup): string {
  const edit = thresholdEdits[dgId];
  const t = edit?.threshold ?? dg.thresholdPct;
  const g = edit?.target ?? dg.targetPct;
  if (t == null || g == null) return 'Both values are required';
  if (t < 1 || t > 99 || g < 1 || g > 99) return 'Values must be between 1 and 99';
  if (t <= g) return 'Threshold must be greater than target';
  return '';
}

async function saveThresholds(dg: DiskGroup) {
  ensureThresholdEdit(dg.id, dg);
  const edit = thresholdEdits[dg.id]!;
  if (thresholdValidation(dg.id, dg)) return;

  edit.saving = true;
  edit.message = '';
  edit.success = false;

  try {
    const updated = (await api(`/api/v1/disk-groups/${dg.id}`, {
      method: 'PUT',
      body: {
        thresholdPct: edit.threshold,
        targetPct: edit.target,
      },
    })) as DiskGroup;

    edit.success = true;
    edit.message = 'Saved';

    // Emit updated disk group to parent for sync
    if (updated) {
      const idx = props.diskGroups.findIndex((g) => g.id === dg.id);
      if (idx !== -1) {
        emit('update:diskGroup', { ...props.diskGroups[idx], ...updated });
      }
    } else {
      emit('update:diskGroup', { ...dg, thresholdPct: edit.threshold, targetPct: edit.target });
    }

    setTimeout(() => {
      edit.message = '';
      edit.success = false;
    }, 2500);
  } catch (err: unknown) {
    edit.success = false;
    const errMsg = (err as ApiError)?.message || 'Failed to save thresholds';
    edit.message = errMsg;
    addToast('Failed to save: ' + errMsg, 'error');
  } finally {
    edit.saving = false;
  }
}
</script>
