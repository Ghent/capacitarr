<template>
  <UiCard
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
      <!-- Empty placeholder when no disk groups exist -->
      <div
        v-if="diskGroups.length === 0"
        class="rounded-lg border-2 border-dashed border-border p-8 text-center"
      >
        <HardDriveIcon class="w-8 h-8 text-muted-foreground/40 mx-auto mb-3" />
        <p class="text-sm text-muted-foreground/70 max-w-sm mx-auto">
          {{ $t('rules.diskThresholdsEmpty') }}
        </p>
      </div>

      <!-- Per-disk-group threshold controls -->
      <template v-else>
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
                :class="diskStatusBgClass(diskUsagePct(dg), editTarget(dg), editThreshold(dg))"
              >
                <component :is="HardDriveIcon" class="w-4.5 h-4.5 text-white" />
              </div>
              <div>
                <div class="flex items-center gap-1.5 flex-wrap">
                  <span class="text-sm font-medium text-foreground truncate" :title="dg.mountPath">
                    {{ dg.mountPath }}
                  </span>
                  <UiBadge
                    v-for="integ in dg.integrations || []"
                    :key="integ.id"
                    variant="outline"
                    class="text-[10px] px-1.5 py-0"
                  >
                    {{ integ.type }}
                  </UiBadge>
                </div>
                <div class="text-xs text-muted-foreground flex items-center gap-1">
                  {{ formatBytes(dg.usedBytes) }} /
                  <!-- Inline edit for total size -->
                  <template v-if="overrideEditing[dg.id]">
                    <UiInput
                      :ref="(el) => setOverrideInputRef(dg.id, el)"
                      type="text"
                      inputmode="decimal"
                      :model-value="editOverrideDisplay(dg)"
                      :placeholder="formatBytes(dg.totalBytes)"
                      class="w-16 h-5 text-xs px-1 inline-flex"
                      @update:model-value="(v: string | number) => onOverrideInput(dg, v)"
                      @keydown.enter="applyInlineOverride(dg)"
                      @keydown.escape="cancelInlineOverride(dg)"
                    />
                    <UiSelect
                      :model-value="editOverrideUnit(dg)"
                      @update:model-value="(v) => onOverrideUnitChange(dg, String(v))"
                    >
                      <UiSelectTrigger class="w-14 h-5 text-xs px-1">
                        <UiSelectValue />
                      </UiSelectTrigger>
                      <UiSelectContent>
                        <UiSelectItem value="GB">GB</UiSelectItem>
                        <UiSelectItem value="TB">TB</UiSelectItem>
                        <UiSelectItem value="PB">PB</UiSelectItem>
                      </UiSelectContent>
                    </UiSelect>
                    <button
                      class="text-emerald-500 hover:text-emerald-400 transition-colors"
                      title="Apply"
                      @click="applyInlineOverride(dg)"
                    >
                      <component :is="CheckIcon" class="w-3.5 h-3.5" />
                    </button>
                    <button
                      class="text-muted-foreground hover:text-foreground transition-colors"
                      title="Cancel"
                      @click="cancelInlineOverride(dg)"
                    >
                      <component :is="XIcon" class="w-3.5 h-3.5" />
                    </button>
                  </template>
                  <template v-else>
                    <span
                      :class="
                        hasActiveOverride(dg)
                          ? 'text-amber-600 dark:text-amber-400 underline decoration-dotted underline-offset-2'
                          : ''
                      "
                      :title="
                        hasActiveOverride(dg)
                          ? `Custom size (detected: ${formatBytes(dg.totalBytes)})`
                          : 'Click pencil to set custom size'
                      "
                    >
                      {{ formatBytes(effectiveTotal(dg)) }}
                    </span>
                    <button
                      class="text-muted-foreground/40 hover:text-foreground transition-colors"
                      title="Edit disk size"
                      @click="startInlineOverride(dg)"
                    >
                      <component :is="PencilIcon" class="w-3 h-3" />
                    </button>
                    <button
                      v-if="hasActiveOverride(dg)"
                      class="text-muted-foreground/40 hover:text-destructive transition-colors"
                      title="Clear custom size"
                      @click="clearAndSaveOverride(dg)"
                    >
                      <component :is="XIcon" class="w-3 h-3" />
                    </button>
                  </template>
                </div>
              </div>
            </div>
            <span
              class="text-2xl font-bold tabular-nums"
              :class="diskStatusTextClass(diskUsagePct(dg), editTarget(dg), editThreshold(dg))"
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
                    width: editTarget(dg) + '%',
                    backgroundColor: 'oklch(0.648 0.2 160 / 0.2)',
                  }"
                />
                <div
                  class="h-full"
                  :style="{
                    width: editThreshold(dg) - editTarget(dg) + '%',
                    backgroundColor: 'oklch(0.75 0.183 55.934 / 0.2)',
                  }"
                />
                <div
                  class="h-full"
                  :style="{
                    width: 100 - editThreshold(dg) + '%',
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
                :data-status="diskUsageStatus(diskUsagePct(dg), editTarget(dg), editThreshold(dg))"
                class="relative h-full rounded-full transition-all duration-700 ease-out z-10"
                :style="{
                  width: Math.min(diskUsagePct(dg), 100) + '%',
                  backgroundColor: diskStatusFillColor(
                    diskUsagePct(dg),
                    editTarget(dg),
                    editThreshold(dg),
                  ),
                }"
              />
            </div>

            <!-- Target marker ABOVE the bar -->
            <div
              class="absolute bottom-3 flex flex-col items-center z-20"
              :style="{ left: editTarget(dg) + '%', transform: 'translateX(-50%)' }"
            >
              <span
                class="text-[10px] font-medium text-emerald-600 dark:text-emerald-400 whitespace-nowrap mb-0.5"
              >
                Target {{ editTarget(dg) }}%
              </span>
              <span class="text-emerald-500 text-[10px] leading-none mb-0.5">▼</span>
            </div>
            <!-- Threshold marker BELOW the bar -->
            <div
              class="absolute top-3 flex flex-col items-center z-20"
              :style="{ left: editThreshold(dg) + '%', transform: 'translateX(-50%)' }"
            >
              <span class="text-red-500 text-[10px] leading-none mt-0.5">▲</span>
              <span
                class="text-[10px] font-medium text-red-500 dark:text-red-400 whitespace-nowrap mt-0.5"
              >
                Threshold {{ editThreshold(dg) }}%
              </span>
            </div>
          </div>

          <!-- Free space info -->
          <div class="text-xs text-muted-foreground/70">
            <span>{{ formatBytes(effectiveTotal(dg) - dg.usedBytes) }} free</span>
            <span v-if="hasActiveOverride(dg)" class="ml-1 text-muted-foreground/50">
              (detected: {{ formatBytes(dg.totalBytes) }})
            </span>
          </div>

          <!-- Dual-thumb range slider -->
          <div class="space-y-2">
            <div data-slot="threshold-slider" class="relative">
              <UiSlider
                :model-value="[editTarget(dg), editThreshold(dg)]"
                :min="1"
                :max="99"
                :step="1"
                :min-steps-between-thumbs="1"
                @update:model-value="
                  (v: number[] | undefined) => {
                    if (v) onSliderChange(dg, v);
                  }
                "
              />
              <!-- Zone color overlays on the slider track -->
              <div
                class="absolute top-1/2 -translate-y-1/2 h-2.5 rounded-l-full pointer-events-none z-1"
                :style="{
                  left: '0%',
                  width: editTarget(dg) + '%',
                  background: 'oklch(0.648 0.2 160 / 0.5)',
                }"
              />
              <div
                class="absolute top-1/2 -translate-y-1/2 h-2.5 pointer-events-none z-1"
                :style="{
                  left: editTarget(dg) + '%',
                  width: editThreshold(dg) - editTarget(dg) + '%',
                  background: 'oklch(0.75 0.183 55.934 / 0.5)',
                }"
              />
              <div
                class="absolute top-1/2 -translate-y-1/2 h-2.5 rounded-r-full pointer-events-none z-1"
                :style="{
                  left: editThreshold(dg) + '%',
                  width: 100 - editThreshold(dg) + '%',
                  background: 'oklch(0.577 0.245 27.325 / 0.5)',
                }"
              />
            </div>

            <!-- Labels below the slider -->
            <div class="flex items-center justify-between text-[11px]">
              <span class="text-emerald-600 dark:text-emerald-400 font-medium">
                ● Target {{ editTarget(dg) }}%
              </span>
              <span class="text-red-500 dark:text-red-400 font-medium">
                ● Threshold {{ editThreshold(dg) }}%
              </span>
            </div>
          </div>

          <!-- Validation error + Save button row -->
          <div class="flex items-center justify-between">
            <p v-if="thresholdValidation(dg.id, dg)" class="text-xs text-red-500">
              {{ thresholdValidation(dg.id, dg) }}
            </p>
            <span v-else />
            <UiButton
              size="sm"
              :disabled="!hasChanges(dg) || !!thresholdValidation(dg.id, dg) || isSaving(dg.id)"
              @click="saveThresholds(dg)"
            >
              <component
                :is="isSaving(dg.id) ? LoaderCircleIcon : SaveIcon"
                class="w-3.5 h-3.5 mr-1.5"
                :class="{ 'animate-spin': isSaving(dg.id) }"
              />
              {{ isSaving(dg.id) ? $t('common.saving') : $t('common.save') }}
            </UiButton>
          </div>
        </div>
      </template>
    </UiCardContent>
  </UiCard>
</template>

<script setup lang="ts">
import {
  HardDriveIcon,
  LoaderCircleIcon,
  SaveIcon,
  PencilIcon,
  XIcon,
  CheckIcon,
} from 'lucide-vue-next';
import {
  formatBytes,
  diskUsageStatus,
  diskStatusBgClass,
  diskStatusTextClass,
  diskStatusFillColor,
} from '~/utils/format';
import type { DiskGroup, ApiError } from '~/types/api';

const GB = 1_073_741_824;
const TB = 1_099_511_627_776;
const PB = 1_125_899_906_842_624;

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
      overrideDisplay: string;
      overrideUnit: string;
      overrideBytes: number | null;
      saving: boolean;
    }
  >
>({});

/** Returns the effective total bytes (override if set, otherwise detected). */
function effectiveTotal(dg: DiskGroup): number {
  const edit = thresholdEdits[dg.id];
  if (edit && edit.overrideBytes != null && edit.overrideBytes > 0) return edit.overrideBytes;
  if (dg.totalBytesOverride && dg.totalBytesOverride > 0) return dg.totalBytesOverride;
  return dg.totalBytes;
}

/** Check if the disk group has an active override (saved or in-progress edit). */
function hasActiveOverride(dg: DiskGroup): boolean {
  return !!(dg.totalBytesOverride && dg.totalBytesOverride > 0);
}

function diskUsagePct(dg: DiskGroup): number {
  const total = effectiveTotal(dg);
  if (!total || total === 0) return 0;
  return (dg.usedBytes / total) * 100;
}

/** Get the current edit target value or fall back to saved value. */
function editTarget(dg: DiskGroup): number {
  return thresholdEdits[dg.id]?.target ?? dg.targetPct;
}

/** Get the current edit threshold value or fall back to saved value. */
function editThreshold(dg: DiskGroup): number {
  return thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct;
}

/** Whether the user has unsaved changes for this disk group. */
function hasChanges(dg: DiskGroup): boolean {
  const edit = thresholdEdits[dg.id];
  if (!edit) return false;
  const savedOverrideBytes = dg.totalBytesOverride ?? null;
  const editOverrideBytes = edit.overrideBytes;
  const overrideChanged = editOverrideBytes !== savedOverrideBytes;
  return edit.target !== dg.targetPct || edit.threshold !== dg.thresholdPct || overrideChanged;
}

/** Whether this disk group is currently saving. */
function isSaving(dgId: number): boolean {
  return thresholdEdits[dgId]?.saving ?? false;
}

/** Convert bytes to a display value and unit pair. */
function bytesToDisplayUnit(bytes: number | null | undefined): { value: string; unit: string } {
  if (!bytes || bytes <= 0) return { value: '', unit: 'GB' };
  if (bytes >= PB && bytes % PB === 0) return { value: String(bytes / PB), unit: 'PB' };
  if (bytes >= PB) return { value: String(+(bytes / PB).toFixed(2)), unit: 'PB' };
  if (bytes >= TB && bytes % TB === 0) return { value: String(bytes / TB), unit: 'TB' };
  if (bytes >= TB) return { value: String(+(bytes / TB).toFixed(2)), unit: 'TB' };
  return { value: String(+(bytes / GB).toFixed(2)), unit: 'GB' };
}

function unitMultiplier(unit: string): number {
  if (unit === 'PB') return PB;
  if (unit === 'TB') return TB;
  return GB;
}

function ensureThresholdEdit(dgId: number, dg: DiskGroup) {
  if (!thresholdEdits[dgId]) {
    const { value, unit } = bytesToDisplayUnit(dg.totalBytesOverride);
    thresholdEdits[dgId] = {
      threshold: dg.thresholdPct,
      target: dg.targetPct,
      overrideDisplay: value,
      overrideUnit: unit,
      overrideBytes: dg.totalBytesOverride ?? null,
      saving: false,
    };
  }
}

/** Get the current display value for the override input. */
function editOverrideDisplay(dg: DiskGroup): string {
  ensureThresholdEdit(dg.id, dg);
  return thresholdEdits[dg.id]!.overrideDisplay;
}

/** Get the current unit for the override input. */
function editOverrideUnit(dg: DiskGroup): string {
  ensureThresholdEdit(dg.id, dg);
  return thresholdEdits[dg.id]!.overrideUnit;
}

/** Handle override value input. */
function onOverrideInput(dg: DiskGroup, value: string | number) {
  ensureThresholdEdit(dg.id, dg);
  const edit = thresholdEdits[dg.id]!;
  const strVal = String(value).trim();
  edit.overrideDisplay = strVal;
  if (!strVal || strVal === '0') {
    edit.overrideBytes = null;
  } else {
    const num = parseFloat(strVal);
    if (!isNaN(num) && num > 0) {
      edit.overrideBytes = Math.round(num * unitMultiplier(edit.overrideUnit));
    }
  }
}

/** Handle override unit change. */
function onOverrideUnitChange(dg: DiskGroup, unit: string) {
  ensureThresholdEdit(dg.id, dg);
  const edit = thresholdEdits[dg.id]!;
  edit.overrideUnit = unit;
  // Recalculate bytes from the display value with new unit
  if (edit.overrideDisplay && edit.overrideDisplay !== '0') {
    const num = parseFloat(edit.overrideDisplay);
    if (!isNaN(num) && num > 0) {
      edit.overrideBytes = Math.round(num * unitMultiplier(unit));
    }
  }
}

/** Clear the override (local state only). */
function clearOverride(dg: DiskGroup) {
  ensureThresholdEdit(dg.id, dg);
  const edit = thresholdEdits[dg.id]!;
  edit.overrideDisplay = '';
  edit.overrideUnit = 'GB';
  edit.overrideBytes = null;
}

/** Clear the override and immediately save to the backend. */
async function clearAndSaveOverride(dg: DiskGroup) {
  clearOverride(dg);
  // Reset thresholds to current saved values so only the override changes
  ensureThresholdEdit(dg.id, dg);
  const edit = thresholdEdits[dg.id]!;
  edit.threshold = dg.thresholdPct;
  edit.target = dg.targetPct;
  await saveThresholds(dg);
}

/** Whether the override value has unsaved changes (for the Apply button). */
function overrideHasChanges(dg: DiskGroup): boolean {
  const edit = thresholdEdits[dg.id];
  if (!edit) return false;
  const savedOverrideBytes = dg.totalBytesOverride ?? null;
  return edit.overrideBytes !== savedOverrideBytes;
}

// --- Inline override editing ---
const overrideEditing = reactive<Record<number, boolean>>({});
const overrideInputRefs = reactive<Record<number, HTMLInputElement | null>>({});

function setOverrideInputRef(dgId: number, el: unknown) {
  if (el && typeof el === 'object' && '$el' in el) {
    overrideInputRefs[dgId] = (el as { $el: HTMLInputElement }).$el;
  }
}

function startInlineOverride(dg: DiskGroup) {
  ensureThresholdEdit(dg.id, dg);
  overrideEditing[dg.id] = true;
  nextTick(() => {
    overrideInputRefs[dg.id]?.focus();
  });
}

function cancelInlineOverride(dg: DiskGroup) {
  overrideEditing[dg.id] = false;
  // Reset to saved value
  const { value, unit } = bytesToDisplayUnit(dg.totalBytesOverride);
  const edit = thresholdEdits[dg.id];
  if (edit) {
    edit.overrideDisplay = value;
    edit.overrideUnit = unit;
    edit.overrideBytes = dg.totalBytesOverride ?? null;
  }
}

async function applyInlineOverride(dg: DiskGroup) {
  overrideEditing[dg.id] = false;
  if (overrideHasChanges(dg)) {
    await saveThresholds(dg);
  }
}

/** Handle slider value changes — array is [target, threshold]. */
function onSliderChange(dg: DiskGroup, values: number[]) {
  ensureThresholdEdit(dg.id, dg);
  const edit = thresholdEdits[dg.id]!;
  edit.target = values[0] ?? dg.targetPct;
  edit.threshold = values[1] ?? dg.thresholdPct;
}

function thresholdValidation(dgId: number, dg: DiskGroup): string {
  const t = editThreshold(dg);
  const g = editTarget(dg);
  if (t == null || g == null) return 'Both values are required';
  if (t < 1 || t > 99 || g < 1 || g > 99) return 'Values must be between 1 and 99';
  if (t <= g) return 'Threshold must be greater than target';
  // Suppress false positives when dgId is used only for lookup
  void dgId;
  return '';
}

function findDgIndex(dg: DiskGroup): number {
  return props.diskGroups.findIndex((g) => g.id === dg.id);
}

async function saveThresholds(dg: DiskGroup) {
  ensureThresholdEdit(dg.id, dg);
  const edit = thresholdEdits[dg.id]!;
  if (thresholdValidation(dg.id, dg)) return;

  edit.saving = true;

  try {
    const updated = (await api(`/api/v1/disk-groups/${dg.id}`, {
      method: 'PUT',
      body: {
        thresholdPct: edit.threshold,
        targetPct: edit.target,
        totalBytesOverride: edit.overrideBytes,
      },
    })) as DiskGroup;

    // Emit updated disk group to parent for sync.
    // Explicitly handle totalBytesOverride: when the API omits it (omitempty),
    // the spread would keep the old value. Set it to undefined to clear it.
    if (updated) {
      const merged = { ...props.diskGroups[findDgIndex(dg)], ...updated };
      if (!('totalBytesOverride' in updated)) {
        merged.totalBytesOverride = undefined;
      }
      emit('update:diskGroup', merged);
    } else {
      emit('update:diskGroup', {
        ...dg,
        thresholdPct: edit.threshold,
        targetPct: edit.target,
        totalBytesOverride: edit.overrideBytes ?? undefined,
      });
    }

    // Reset local edit state so it picks up the new prop values on next access
    const dgId = dg.id;
    if (thresholdEdits[dgId]) {
      const updatedDg = updated ?? dg;
      const { value, unit } = bytesToDisplayUnit(updatedDg.totalBytesOverride);
      thresholdEdits[dgId] = {
        threshold: updatedDg.thresholdPct,
        target: updatedDg.targetPct,
        overrideDisplay: value,
        overrideUnit: unit,
        overrideBytes: updatedDg.totalBytesOverride ?? null,
        saving: false,
      };
    }

    addToast(`Thresholds saved for ${dg.mountPath}`, 'success');
  } catch (err: unknown) {
    const errMsg = (err as ApiError)?.message || 'Failed to save thresholds';
    addToast('Failed to save: ' + errMsg, 'error');
  } finally {
    edit.saving = false;
  }
}
</script>
