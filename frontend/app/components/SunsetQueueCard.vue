<script setup lang="ts">
import {
  HourglassIcon,
  XCircleIcon,
  CalendarClockIcon,
  Trash2Icon,
  ShieldCheckIcon,
} from 'lucide-vue-next';
import { formatBytes } from '~/utils/format';
import type { SunsetQueueItem } from '~/types/api';

const { hasSunsetMode = false } = defineProps<{
  hasSunsetMode?: boolean;
}>();

const { t } = useI18n();
const { listItem } = useMotionPresets();
const { viewMode } = useDisplayPrefs();
const { sunsetItems, fetchSunsetItems, cancelItem, rescheduleItem, clearAll } = useSunsetQueue();
const api = useApi();

// Overlay display style preference: "countdown" (default) or "simple"
const overlayStyle = useState<string>('sunsetOverlayStyle', () => 'countdown');

// Fetch on mount
onMounted(async () => {
  fetchSunsetItems();
  try {
    const prefs = await api.GET('/preferences');
    if (prefs?.posterOverlayStyle) {
      overlayStyle.value = prefs.posterOverlayStyle;
    }
  } catch {
    // Fall back to default "countdown" style
  }
});

/** Selected item for the score detail modal */
const selectedItem = ref<SunsetQueueItem | null>(null);

function showDetail(item: SunsetQueueItem) {
  selectedItem.value = item;
}

/**
 * Format days remaining as a human-readable countdown.
 * e.g. "30 days", "1 day", "Last day"
 */
function formatDaysRemaining(days: number): string {
  if (overlayStyle.value === 'simple') return t('sunset.leavingSoon');
  if (days <= 0) return t('sunset.lastDay');
  if (days === 1) return t('sunset.leavingTomorrow');
  return t('sunset.leavingInDays', { days });
}

const showClearAllDialog = ref(false);

function confirmClearAll() {
  showClearAllDialog.value = true;
}

function executeClearAll() {
  clearAll();
  showClearAllDialog.value = false;
}

/** Reschedule an item by adding days to its current deletion date. */
function rescheduleByDays(itemId: number, currentDate: string, addDays: number) {
  const date = new Date(currentDate + 'T00:00:00Z');
  date.setUTCDate(date.getUTCDate() + addDays);
  const iso = date.toISOString().split('T')[0]!;
  rescheduleItem(itemId, iso);
}
</script>

<template>
  <UiCard
    v-if="sunsetItems.length > 0 || hasSunsetMode"
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24 } }"
    class="mb-6"
  >
    <UiCardHeader>
      <div class="flex items-center justify-between">
        <div>
          <UiCardTitle class="flex items-center gap-2">
            <HourglassIcon class="w-4.5 h-4.5" />
            {{ t('sunset.title') }}
          </UiCardTitle>
          <UiCardDescription class="mt-1">
            {{ t('sunset.subtitle') }}
          </UiCardDescription>
        </div>
        <div class="flex items-center gap-2 text-xs text-muted-foreground">
          <ViewModeToggle />
          <UiBadge variant="secondary" class="text-xs">
            {{ t('sunset.count', { count: sunsetItems.length }) }}
          </UiBadge>
          <UiButton
            variant="ghost"
            size="sm"
            class="h-7 px-2 text-xs text-muted-foreground hover:text-destructive"
            :title="t('sunset.clearAll')"
            :aria-label="t('sunset.clearAll')"
            @click="confirmClearAll()"
          >
            <Trash2Icon class="h-3.5 w-3.5 mr-1" />
            {{ t('sunset.clearAll') }}
          </UiButton>
        </div>
      </div>
    </UiCardHeader>
    <UiCardContent>
      <!-- Empty state -->
      <div
        v-if="sunsetItems.length === 0"
        class="rounded-lg border-2 border-dashed border-border p-8 text-center"
      >
        <HourglassIcon class="w-8 h-8 text-muted-foreground/40 mx-auto mb-3" />
        <p class="text-sm text-muted-foreground">
          {{ t('deletion.emptyInSunset') }}
        </p>
      </div>

      <!-- Grid / poster view -->
      <div
        v-if="viewMode === 'grid'"
        class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-3"
      >
        <MediaPosterCard
          v-for="(item, idx) in sunsetItems"
          :key="item.id"
          :title="item.mediaName"
          :poster-url="item.posterUrl"
          :media-type="item.mediaType"
          :score="item.score"
          :size-bytes="item.sizeBytes"
          :sunset-days-remaining="item.status !== 'saved' ? item.daysRemaining : undefined"
          :overlay-style="overlayStyle"
          :animation-delay="idx * 30"
          :queue-status="item.status === 'saved' ? 'saved' : undefined"
          @click="showDetail(item)"
        />
      </div>

      <!-- List view -->
      <div v-else class="space-y-1.5">
        <div v-for="(item, idx) in sunsetItems" :key="item.id" v-motion v-bind="listItem(idx * 30)">
          <div
            :class="[
              'flex items-center gap-3 rounded-lg border px-3 py-2',
              item.status === 'saved'
                ? 'border-emerald-500/30 bg-emerald-500/5'
                : 'border-border bg-muted/30',
            ]"
          >
            <!-- Score (clickable) -->
            <span
              :class="[
                'text-xs font-mono tabular-nums font-semibold shrink-0 w-12 text-right cursor-pointer',
                item.status === 'saved'
                  ? 'text-emerald-600 dark:text-emerald-400 hover:text-emerald-500'
                  : 'text-primary hover:text-primary/80',
              ]"
              @click="showDetail(item)"
            >
              {{ item.score.toFixed(2) }}
            </span>

            <!-- Title + metadata (clickable) -->
            <div class="flex-1 min-w-0 cursor-pointer" @click="showDetail(item)">
              <span class="text-sm font-medium truncate block">{{ item.mediaName }}</span>
              <span
                v-if="item.status === 'saved'"
                class="text-xs text-emerald-600 dark:text-emerald-400"
              >
                <ShieldCheckIcon class="inline h-3 w-3 mr-0.5 -mt-px" />
                {{ t('sunset.savedByPopularDemand') }}
                <span v-if="item.savedReason" class="ml-1 text-muted-foreground">
                  · {{ item.savedReason }}
                </span>
              </span>
              <span v-else class="text-xs text-muted-foreground">
                {{ item.mediaType }} · {{ formatBytes(item.sizeBytes) }}
                <span class="ml-1 text-orange-500 dark:text-orange-400">
                  · {{ formatDaysRemaining(item.daysRemaining) }}
                </span>
              </span>
            </div>

            <!-- Saved badge (for saved items) -->
            <UiBadge
              v-if="item.status === 'saved'"
              class="shrink-0 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20"
            >
              {{ t('sunset.saved') }}
            </UiBadge>

            <!-- Actions (only for pending items) -->
            <div v-if="item.status !== 'saved'" class="flex items-center gap-1 shrink-0">
              <!-- Reschedule dropdown -->
              <UiDropdownMenu>
                <UiDropdownMenuTrigger as-child>
                  <UiButton
                    variant="ghost"
                    size="sm"
                    class="h-7 p-0 px-2 text-muted-foreground hover:text-foreground"
                    :title="t('sunset.reschedule')"
                  >
                    <CalendarClockIcon class="h-3.5 w-3.5 mr-1" />
                    <span class="text-xs">{{ t('sunset.reschedule') }}</span>
                  </UiButton>
                </UiDropdownMenuTrigger>
                <UiDropdownMenuContent align="end">
                  <UiDropdownMenuItem @click="rescheduleByDays(item.id, item.deletionDate, 7)">
                    + 7 {{ t('sunset.days') }}
                  </UiDropdownMenuItem>
                  <UiDropdownMenuItem @click="rescheduleByDays(item.id, item.deletionDate, 14)">
                    + 14 {{ t('sunset.days') }}
                  </UiDropdownMenuItem>
                  <UiDropdownMenuItem @click="rescheduleByDays(item.id, item.deletionDate, 30)">
                    + 30 {{ t('sunset.days') }}
                  </UiDropdownMenuItem>
                </UiDropdownMenuContent>
              </UiDropdownMenu>
              <!-- Cancel button -->
              <UiButton
                variant="ghost"
                size="sm"
                class="h-7 p-0 px-2 text-muted-foreground hover:text-foreground"
                :aria-label="t('sunset.cancel')"
                :title="t('sunset.cancel')"
                @click="cancelItem(item.id)"
              >
                <XCircleIcon class="h-3.5 w-3.5 mr-1" />
                <span class="text-xs">{{ t('sunset.cancel') }}</span>
              </UiButton>
            </div>
            <!-- Cancel button for saved items (still allows removal) -->
            <UiButton
              v-if="item.status === 'saved'"
              variant="ghost"
              size="sm"
              class="h-7 p-0 px-2 text-muted-foreground hover:text-foreground shrink-0"
              :aria-label="t('sunset.cancel')"
              :title="t('sunset.cancel')"
              @click="cancelItem(item.id)"
            >
              <XCircleIcon class="h-3.5 w-3.5 mr-1" />
              <span class="text-xs">{{ t('sunset.cancel') }}</span>
            </UiButton>
          </div>
        </div>
      </div>
    </UiCardContent>
  </UiCard>

  <!-- Clear All Confirmation Dialog -->
  <UiDialog
    :open="showClearAllDialog"
    @update:open="
      (val: boolean) => {
        showClearAllDialog = val;
      }
    "
  >
    <UiDialogContent class="max-w-md">
      <UiDialogHeader>
        <UiDialogTitle>{{ t('sunset.clearAllDialogTitle') }}</UiDialogTitle>
        <UiDialogDescription>
          {{ t('sunset.clearAllConfirm') }}
        </UiDialogDescription>
      </UiDialogHeader>
      <UiDialogFooter class="flex gap-2 justify-end">
        <UiButton variant="outline" @click="showClearAllDialog = false">
          {{ t('common.cancel') }}
        </UiButton>
        <UiButton variant="destructive" @click="executeClearAll">
          {{ t('sunset.clearAll') }}
        </UiButton>
      </UiDialogFooter>
    </UiDialogContent>
  </UiDialog>

  <!-- Score Detail Modal -->
  <ScoreDetailModal
    v-if="selectedItem"
    :visible="!!selectedItem"
    :media-name="selectedItem.mediaName"
    :media-type="selectedItem.mediaType"
    :score="selectedItem.score"
    :score-details="selectedItem.scoreDetails ?? '[]'"
    :size-bytes="selectedItem.sizeBytes"
    :action="
      selectedItem.status === 'saved'
        ? t('sunset.savedByPopularDemand')
        : formatDaysRemaining(selectedItem.daysRemaining)
    "
    :created-at="selectedItem.createdAt"
    @close="selectedItem = null"
  />
</template>
