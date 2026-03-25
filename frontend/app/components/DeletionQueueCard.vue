<script setup lang="ts">
import { MODE_DRY_RUN, MODE_APPROVAL } from '~/constants';
import {
  Trash2Icon,
  XIcon,
  CheckCircle2Icon,
  XCircleIcon,
  BanIcon,
  LayersIcon,
  LoaderCircleIcon,
  ClockIcon,
  Trash2Icon as ClearAllIcon,
} from 'lucide-vue-next';
import { formatBytes } from '~/utils/format';

const { t } = useI18n();
const { listItem } = useMotionPresets();
const {
  deletionProgress: engineDeletionProgress,
  isDeletionActive: engineIsDeletionActive,
  executionMode,
} = useEngineControl();
const { queuedItems, completedItems, countdown, fetchQueue, cancelItem, snoozeItem, clearAll } =
  useDeletionQueue();

// Fetch queue on mount
onMounted(() => {
  fetchQueue();
});

const hasContent = computed(
  () =>
    engineIsDeletionActive.value || queuedItems.value.length > 0 || completedItems.value.length > 0,
);

/** Mode-specific empty state message key */
const emptyStateMessage = computed(() => {
  switch (executionMode.value) {
    case MODE_APPROVAL:
      return t('deletion.emptyInApproval');
    case MODE_DRY_RUN:
      return t('deletion.emptyInDryRun');
    default:
      return t('deletion.noItems');
  }
});

const progressPercent = computed(() => {
  if (!engineDeletionProgress.value) return 0;
  const { processed, batchTotal } = engineDeletionProgress.value;
  return batchTotal > 0 ? Math.round((processed / batchTotal) * 100) : 0;
});
</script>

<template>
  <UiCard
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24 } }"
    class="mb-6"
  >
    <UiCardHeader>
      <div class="flex items-center justify-between">
        <div>
          <UiCardTitle class="flex items-center gap-2">
            <Trash2Icon class="w-4.5 h-4.5" />
            {{ t('deletion.title') }}
          </UiCardTitle>
          <UiCardDescription class="mt-1">
            {{ t('deletion.subtitle') }}
          </UiCardDescription>
        </div>
        <div class="flex items-center gap-2 text-xs text-muted-foreground">
          <UiBadge v-if="queuedItems.length > 0" variant="default" class="text-xs">
            {{ queuedItems.length }} {{ t('deletion.queued').toLowerCase() }}
          </UiBadge>
          <UiBadge v-if="completedItems.length > 0" variant="secondary" class="text-xs">
            {{ completedItems.length }} done
          </UiBadge>
          <UiButton
            v-if="queuedItems.length > 0"
            variant="ghost"
            size="sm"
            class="h-7 px-2 text-xs text-muted-foreground hover:text-destructive hover:bg-destructive/10 dark:hover:bg-destructive/20"
            :title="t('deletion.clearAll')"
            @click="clearAll()"
          >
            <ClearAllIcon class="h-3.5 w-3.5 mr-1" />
            {{ t('deletion.clearAll') }}
          </UiButton>
        </div>
      </div>
    </UiCardHeader>
    <UiCardContent>
      <!-- Grace period countdown -->
      <div
        v-if="countdown > 0 && queuedItems.length > 0 && !engineIsDeletionActive"
        class="mb-4 flex items-center gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2"
      >
        <ClockIcon class="w-4 h-4 text-amber-500 shrink-0" />
        <span class="text-sm text-amber-700 dark:text-amber-400">
          {{ t('deletion.gracePeriod', { seconds: countdown }) }}
        </span>
      </div>

      <!-- Batch progress bar -->
      <div v-if="engineDeletionProgress" class="mb-4">
        <div class="flex items-center justify-between text-xs text-muted-foreground mb-1">
          <span>{{ t('deletion.batchProgress') }}</span>
          <span
            >{{ engineDeletionProgress.processed }}/{{ engineDeletionProgress.batchTotal }}</span
          >
        </div>
        <UiProgress :model-value="progressPercent" class="h-2" />
      </div>

      <!-- Currently deleting -->
      <div v-if="engineIsDeletionActive && engineDeletionProgress" class="mb-3">
        <h4 class="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">
          {{ t('deletion.currentlyDeleting') }}
        </h4>
        <div class="flex items-center gap-3 rounded-lg border border-border bg-muted/30 px-3 py-2">
          <LoaderCircleIcon class="w-4 h-4 animate-spin text-destructive shrink-0" />
          <div class="flex-1 min-w-0">
            <span class="text-sm font-medium truncate block">{{
              engineDeletionProgress.currentItem
            }}</span>
          </div>
        </div>
      </div>

      <!-- Queued items -->
      <div v-if="queuedItems.length > 0" class="mb-3">
        <h4 class="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">
          {{ t('deletion.queued') }}
        </h4>
        <div class="space-y-1.5">
          <div
            v-for="(item, idx) in queuedItems"
            :key="`${item.mediaName}:${item.mediaType}`"
            v-motion
            v-bind="listItem(idx * 30)"
            class="flex items-center gap-3 rounded-lg border border-border bg-muted/30 px-3 py-2"
          >
            <div class="flex-1 min-w-0">
              <span class="inline-flex items-center gap-1.5">
                <span class="text-sm font-medium truncate">{{ item.mediaName }}</span>
                <UiBadge
                  v-if="item.collectionGroup"
                  variant="outline"
                  class="text-[10px] border-indigo-500/50 bg-indigo-500/10 text-indigo-600 dark:text-indigo-400 shrink-0"
                  :title="item.collectionGroup"
                >
                  <LayersIcon class="w-3 h-3" />
                </UiBadge>
              </span>
              <span class="text-xs text-muted-foreground block">
                {{ item.mediaType }} · {{ formatBytes(item.sizeBytes) }}
              </span>
            </div>
            <div class="flex items-center gap-1 shrink-0">
              <UiButton
                variant="ghost"
                size="sm"
                class="h-7 w-7 p-0 text-muted-foreground hover:text-amber-500 hover:bg-amber-500/10 dark:hover:bg-amber-500/20"
                :aria-label="t('deletion.snoozeItem')"
                :title="t('deletion.snoozeItem')"
                @click="snoozeItem(item.mediaName, item.mediaType)"
              >
                <ClockIcon class="h-4 w-4" />
              </UiButton>
              <UiButton
                variant="ghost"
                size="sm"
                class="h-7 w-7 p-0 text-muted-foreground hover:text-destructive hover:bg-destructive/10 dark:hover:bg-destructive/20"
                :aria-label="t('deletion.cancelItem')"
                :title="t('deletion.cancelItem')"
                @click="cancelItem(item.mediaName, item.mediaType)"
              >
                <XIcon class="h-4 w-4" />
              </UiButton>
            </div>
          </div>
        </div>
      </div>

      <!-- Completed items -->
      <div v-if="completedItems.length > 0">
        <h4 class="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">
          {{ t('deletion.completed') }}
        </h4>
        <div class="space-y-1.5">
          <div
            v-for="(item, idx) in completedItems"
            :key="idx"
            v-motion
            v-bind="listItem(idx * 30)"
            class="flex items-center gap-3 rounded-lg border border-border bg-muted/30 px-3 py-2 opacity-75"
          >
            <CheckCircle2Icon
              v-if="item.status === 'success'"
              class="w-4 h-4 text-green-500 shrink-0"
            />
            <XCircleIcon
              v-else-if="item.status === 'failed'"
              class="w-4 h-4 text-red-500 shrink-0"
            />
            <BanIcon v-else class="w-4 h-4 text-amber-500 shrink-0" />
            <div class="flex-1 min-w-0">
              <span class="text-sm font-medium truncate block">{{ item.mediaName }}</span>
              <span class="text-xs text-muted-foreground">
                {{ item.mediaType }}
                <template v-if="item.sizeBytes > 0"> · {{ formatBytes(item.sizeBytes) }} </template>
              </span>
            </div>
            <span
              class="text-xs capitalize shrink-0"
              :class="{
                'text-green-500': item.status === 'success',
                'text-red-500': item.status === 'failed',
                'text-amber-500': item.status === 'cancelled',
              }"
            >
              {{ item.status === 'cancelled' ? t('deletion.cancelled') : item.status }}
            </span>
          </div>
        </div>
      </div>

      <!-- Empty state — always shown when no items are queued -->
      <div v-if="!hasContent" class="text-center py-6 text-muted-foreground text-sm">
        {{ emptyStateMessage }}
      </div>
    </UiCardContent>
  </UiCard>
</template>
