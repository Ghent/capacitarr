<script setup lang="ts">
import {
  CheckIcon,
  AlarmClockIcon,
  ClipboardListIcon,
  ChevronRightIcon,
  Trash2Icon,
  XIcon,
} from 'lucide-vue-next';
import { formatBytes } from '~/utils/format';
import type { ApprovalGroup } from '~/composables/useApprovalQueue';

const { t } = useI18n();
const { listItem } = useMotionPresets();
const { viewMode } = useDisplayPrefs();
const {
  pendingItems,
  loading,
  approveGroup,
  rejectGroup,
  approveSeason,
  snoozeSeason,
  dismissGroup,
  dismissSeason,
  clearQueue,
} = useApprovalQueue();

const totalCount = computed(() => pendingItems.value.length);

const scrollAreaRef = ref<InstanceType<
  typeof import('~/components/ui/scroll-area/ScrollArea.vue').default
> | null>(null);

/** Expanded group keys (for show groups with seasons) */
const expandedKeys = ref<Set<string>>(new Set());

function toggleExpand(key: string) {
  const next = new Set(expandedKeys.value);
  if (next.has(key)) {
    next.delete(key);
  } else {
    next.add(key);
  }
  expandedKeys.value = next;
}

/** Selected item for score detail modal (group or season) */
const selectedGroup = ref<ApprovalGroup | null>(null);
const selectedSeason = ref<ApprovalGroup['seasons'][number] | null>(null);

/** Format subtitle for a group: season count + total size */
function groupSubtitle(group: { seasonCount: number; totalSizeBytes: number }): string {
  const size = formatBytes(group.totalSizeBytes);
  if (group.seasonCount > 0) {
    return `${group.seasonCount} ${group.seasonCount === 1 ? 'season' : 'seasons'} · ${size}`;
  }
  return size;
}

/** Open score detail modal for a group */
function showDetail(group: ApprovalGroup) {
  selectedSeason.value = null;
  selectedGroup.value = group;
}

/** Open score detail modal for a specific season */
function showSeasonDetail(season: ApprovalGroup['seasons'][number]) {
  selectedGroup.value = null;
  selectedSeason.value = season;
}

/** Extract "Season N" from a full title like "Show Name - Season 3" */
function extractPreviewSeasonLabel(title: string): string {
  const parts = title.split(' - Season ');
  return parts.length > 1 ? `Season ${parts[parts.length - 1]}` : title;
}

// --- 3-second confirmation timer for approve button ---
const confirmingKey = ref<string | null>(null);
const confirmCountdown = ref(3);
let confirmTimer: ReturnType<typeof setInterval> | null = null;

function startApproveConfirm(group: ApprovalGroup) {
  // If already confirming this group and they click again, cancel
  if (confirmingKey.value === group.key) {
    clearConfirmTimer();
    return;
  }

  // Cancel any existing confirm for a different group
  clearConfirmTimer();

  confirmingKey.value = group.key;
  confirmCountdown.value = 3;

  confirmTimer = setInterval(() => {
    confirmCountdown.value--;
    if (confirmCountdown.value <= 0) {
      clearConfirmTimer();
      approveGroup(group);
    }
  }, 1000);
}

function clearConfirmTimer() {
  if (confirmTimer) {
    clearInterval(confirmTimer);
    confirmTimer = null;
  }
  confirmingKey.value = null;
  confirmCountdown.value = 3;
}

onUnmounted(() => {
  clearConfirmTimer();
});

// --- Batch selection ---
const selectedKeys = ref<Set<string>>(new Set());

/** Build a selectable key for a season within a group */
function seasonKey(groupKey: string, seasonTitle: string): string {
  return `${groupKey}::${seasonTitle}`;
}

/** Get all selectable keys for a group (seasons if it has them, otherwise the group itself) */
function selectableKeysForGroup(group: ApprovalGroup): string[] {
  if (group.seasonCount > 0) {
    return group.seasons.map((s) => seasonKey(group.key, s.title));
  }
  return [group.key];
}

/** Check if a group is fully selected (all its seasons or the group itself) */
function isGroupFullySelected(group: ApprovalGroup): boolean {
  return selectableKeysForGroup(group).every((k) => selectedKeys.value.has(k));
}

/** Check if a group is partially selected (some but not all seasons) */
function isGroupPartiallySelected(group: ApprovalGroup): boolean {
  const keys = selectableKeysForGroup(group);
  const count = keys.filter((k) => selectedKeys.value.has(k)).length;
  return count > 0 && count < keys.length;
}

const isAllSelected = computed(() => {
  const actionable = pendingItems.value.filter((g) => g.auditIds.length > 0);
  return actionable.length > 0 && actionable.every((g) => isGroupFullySelected(g));
});

const selectedCount = computed(() => selectedKeys.value.size);

function toggleSelect(key: string) {
  const next = new Set(selectedKeys.value);
  if (next.has(key)) {
    next.delete(key);
  } else {
    next.add(key);
  }
  selectedKeys.value = next;
}

/** Toggle all selectable keys for a group (select all seasons or deselect all) */
function toggleGroupSelect(group: ApprovalGroup) {
  const next = new Set(selectedKeys.value);
  const keys = selectableKeysForGroup(group);
  if (isGroupFullySelected(group)) {
    for (const k of keys) next.delete(k);
  } else {
    for (const k of keys) next.add(k);
  }
  selectedKeys.value = next;
}

function toggleSelectAll() {
  if (isAllSelected.value) {
    selectedKeys.value = new Set();
  } else {
    const next = new Set<string>();
    for (const g of pendingItems.value) {
      if (g.auditIds.length > 0) {
        for (const k of selectableKeysForGroup(g)) {
          next.add(k);
        }
      }
    }
    selectedKeys.value = next;
  }
}

// Batch approve: 3-second confirmation for batch too
const batchConfirming = ref(false);
const batchCountdown = ref(3);
let batchTimer: ReturnType<typeof setInterval> | null = null;

function startBatchApprove() {
  if (batchConfirming.value) {
    // Second click: cancel
    clearBatchTimer();
    return;
  }

  batchConfirming.value = true;
  batchCountdown.value = 3;

  batchTimer = setInterval(() => {
    batchCountdown.value--;
    if (batchCountdown.value <= 0) {
      clearBatchTimer();
      executeBatchApprove();
    }
  }, 1000);
}

function clearBatchTimer() {
  if (batchTimer) {
    clearInterval(batchTimer);
    batchTimer = null;
  }
  batchConfirming.value = false;
  batchCountdown.value = 3;
}

async function executeBatchApprove() {
  // Find groups that have any selected keys (group-level or season-level)
  const selected = pendingItems.value.filter((g) =>
    selectableKeysForGroup(g).some((k) => selectedKeys.value.has(k)),
  );
  for (const group of selected) {
    await approveGroup(group);
  }
  selectedKeys.value = new Set();
}

onUnmounted(() => {
  clearBatchTimer();
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
            <ClipboardListIcon class="w-4.5 h-4.5" />
            {{ t('approval.title') }}
          </UiCardTitle>
          <UiCardDescription class="mt-1">
            {{ t('approval.subtitle') }}
          </UiCardDescription>
        </div>
        <div class="flex items-center gap-2 text-xs text-muted-foreground">
          <ViewModeToggle />
          <UiBadge v-if="pendingItems.length > 0" variant="default" class="text-xs">
            {{ t('approval.pendingCount', { count: pendingItems.length }) }}
          </UiBadge>
          <UiButton
            v-if="pendingItems.length > 0"
            variant="ghost"
            size="sm"
            class="h-6 px-2 text-xs text-muted-foreground hover:text-destructive hover:bg-destructive/10 dark:hover:bg-destructive/20"
            @click="clearQueue()"
          >
            <Trash2Icon class="h-3 w-3 mr-1" />
            Clear All
          </UiButton>
        </div>
      </div>
    </UiCardHeader>
    <UiCardContent>
      <!-- Empty state -->
      <div v-if="totalCount === 0" class="text-center py-6 text-muted-foreground text-sm">
        {{ t('approval.noPending') }}
      </div>

      <UiScrollArea v-else ref="scrollAreaRef" class="h-[480px] pr-4">
        <div class="space-y-4">
          <!-- Pending Approval -->
          <div v-if="pendingItems.length > 0">
            <div class="flex items-center justify-between mb-2">
              <h4 class="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                {{ t('approval.pending') }}
              </h4>
              <div class="flex items-center gap-2">
                <!-- Select All toggle -->
                <UiButton
                  v-if="pendingItems.some((g) => g.auditIds.length > 0)"
                  variant="ghost"
                  size="sm"
                  class="h-6 px-2 text-xs text-muted-foreground hover:text-foreground"
                  @click="toggleSelectAll"
                >
                  {{ isAllSelected ? 'Deselect All' : 'Select All' }}
                </UiButton>
                <!-- Batch approve button (only visible when items are selected) -->
                <UiButton
                  v-if="selectedCount > 0"
                  :variant="batchConfirming ? 'destructive' : 'default'"
                  size="sm"
                  class="h-6 px-2 text-xs gap-1"
                  @click="startBatchApprove"
                >
                  <Trash2Icon class="h-3 w-3" />
                  <template v-if="batchConfirming">
                    {{ batchCountdown }}s — click to cancel
                  </template>
                  <template v-else> Approve {{ selectedCount }} Selected </template>
                </UiButton>
              </div>
            </div>
            <!-- Grid view for pending items -->
            <div
              v-if="viewMode === 'grid'"
              class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-3"
            >
              <template v-for="group in pendingItems" :key="group.key">
                <!-- Show groups: popover with individual seasons -->
                <UiPopover v-if="group.seasonCount > 0">
                  <UiPopoverTrigger as-child>
                    <MediaPosterCard
                      :title="group.showTitle"
                      :poster-url="group.posterUrl"
                      :media-type="group.type"
                      :score="group.score"
                      :size-bytes="group.totalSizeBytes"
                      :selectable="group.auditIds.length > 0"
                      :selected="isGroupFullySelected(group)"
                      :season-count="group.seasonCount"
                      @select="toggleGroupSelect(group)"
                    />
                  </UiPopoverTrigger>
                  <UiPopoverContent class="w-72 p-0" side="bottom" align="start">
                    <div class="px-3 py-2 border-b">
                      <p class="text-sm font-medium truncate">
                        {{ group.showTitle }}
                      </p>
                      <p class="text-xs text-muted-foreground">
                        {{ group.seasonCount }} season{{ group.seasonCount !== 1 ? 's' : '' }}
                      </p>
                    </div>
                    <div class="max-h-60 overflow-y-auto">
                      <div
                        v-for="season in group.seasons"
                        :key="season.title"
                        class="flex items-center gap-2 px-3 py-1.5 hover:bg-muted/50 transition-colors"
                      >
                        <span
                          class="text-xs font-mono tabular-nums font-semibold w-10 text-right shrink-0 cursor-pointer text-primary hover:text-primary/80"
                          @click="showSeasonDetail(season)"
                        >
                          {{ season.score.toFixed(2) }}
                        </span>
                        <span
                          class="text-xs truncate flex-1 cursor-pointer"
                          @click="showSeasonDetail(season)"
                        >
                          {{ extractPreviewSeasonLabel(season.title) }}
                        </span>
                        <div v-if="season.auditId" class="flex items-center gap-0.5 shrink-0">
                          <UiButton
                            variant="ghost"
                            size="sm"
                            class="h-6 w-6 p-0 text-green-600 hover:text-green-700 hover:bg-green-100 dark:hover:bg-green-900/30"
                            :aria-label="t('approval.approve')"
                            @click="approveSeason(season.auditId!)"
                          >
                            <CheckIcon class="h-3.5 w-3.5" />
                          </UiButton>
                          <UiButton
                            variant="ghost"
                            size="sm"
                            class="h-6 w-6 p-0 text-amber-500 hover:text-amber-600 hover:bg-amber-100 dark:hover:bg-amber-900/30"
                            :aria-label="t('approval.snooze')"
                            @click="snoozeSeason(season.auditId!)"
                          >
                            <AlarmClockIcon class="h-3.5 w-3.5" />
                          </UiButton>
                          <UiButton
                            variant="ghost"
                            size="sm"
                            class="h-6 w-6 p-0 text-muted-foreground hover:text-destructive hover:bg-destructive/10 dark:hover:bg-destructive/20"
                            aria-label="Dismiss"
                            @click="dismissSeason(season.auditId!)"
                          >
                            <XIcon class="h-3.5 w-3.5" />
                          </UiButton>
                        </div>
                      </div>
                    </div>
                  </UiPopoverContent>
                </UiPopover>
                <!-- Non-show items: direct click to detail -->
                <MediaPosterCard
                  v-else
                  :title="group.showTitle"
                  :poster-url="group.posterUrl"
                  :media-type="group.type"
                  :score="group.score"
                  :size-bytes="group.totalSizeBytes"
                  :selectable="group.auditIds.length > 0"
                  :selected="isGroupFullySelected(group)"
                  @click="showDetail(group)"
                  @select="toggleGroupSelect(group)"
                />
              </template>
            </div>
            <!-- List view for pending items -->
            <div v-else class="space-y-1.5">
              <div
                v-for="(group, idx) in pendingItems"
                :key="group.key"
                v-motion
                v-bind="listItem(idx * 30)"
              >
                <div
                  class="flex items-center gap-3 rounded-lg border border-border bg-muted/30 px-3 py-2"
                  :class="
                    isGroupFullySelected(group) || isGroupPartiallySelected(group)
                      ? 'ring-1 ring-primary/30 bg-primary/5'
                      : ''
                  "
                >
                  <!-- Score (clickable for detail) -->
                  <span
                    class="text-xs font-mono tabular-nums font-semibold text-primary shrink-0 w-12 text-right cursor-pointer hover:text-primary/80"
                    @click="showDetail(group)"
                  >
                    {{ group.score.toFixed(2) }}
                  </span>

                  <!-- Title + type badge + expand toggle + subtitle -->
                  <div class="flex-1 min-w-0 cursor-pointer" @click="showDetail(group)">
                    <span class="inline-flex items-center gap-1.5">
                      <span class="text-sm font-medium truncate">{{ group.showTitle }}</span>
                      <button
                        v-if="group.seasonCount > 0"
                        class="text-muted-foreground hover:text-foreground transition-colors shrink-0 inline-flex items-center gap-0.5"
                        :aria-label="
                          expandedKeys.has(group.key) ? 'Collapse seasons' : 'Expand seasons'
                        "
                        :aria-expanded="expandedKeys.has(group.key)"
                        @click.stop="toggleExpand(group.key)"
                      >
                        <ChevronRightIcon
                          class="w-3.5 h-3.5 transition-transform duration-200"
                          :class="{ 'rotate-90': expandedKeys.has(group.key) }"
                        />
                        <span class="text-xs text-muted-foreground font-normal whitespace-nowrap"
                          >({{ group.seasonCount }} season{{
                            group.seasonCount !== 1 ? 's' : ''
                          }})</span
                        >
                      </button>
                      <UiBadge
                        variant="secondary"
                        class="capitalize text-[10px] px-1.5 py-0 shrink-0"
                      >
                        {{ group.type }}
                      </UiBadge>
                    </span>
                    <span class="text-xs text-muted-foreground block">{{
                      groupSubtitle(group)
                    }}</span>
                  </div>

                  <!-- Actions -->
                  <div class="flex items-center gap-1 shrink-0">
                    <!-- Approve button with 3-second confirmation countdown -->
                    <UiButton
                      v-if="group.auditIds.length > 0"
                      variant="ghost"
                      size="sm"
                      :class="[
                        'h-7 p-0 transition-all',
                        confirmingKey === group.key
                          ? 'w-auto px-2 bg-destructive/10 text-destructive hover:bg-destructive/20 dark:bg-destructive/20 dark:hover:bg-destructive/30'
                          : 'w-7 text-green-600 hover:text-green-700 hover:bg-green-100 dark:hover:bg-green-900/30',
                      ]"
                      :disabled="!!loading[group.key]"
                      :aria-label="
                        confirmingKey === group.key ? 'Cancel approval' : t('approval.approve')
                      "
                      @click="startApproveConfirm(group)"
                    >
                      <template v-if="confirmingKey === group.key">
                        <span class="text-[11px] font-medium tabular-nums"
                          >{{ confirmCountdown }}s</span
                        >
                      </template>
                      <template v-else>
                        <CheckIcon class="h-4 w-4" />
                      </template>
                    </UiButton>
                    <UiButton
                      v-if="group.auditIds.length > 0"
                      variant="ghost"
                      size="sm"
                      class="h-7 w-7 p-0 text-amber-500 hover:text-amber-600 hover:bg-amber-100 dark:hover:bg-amber-900/30"
                      :disabled="!!loading[group.key]"
                      :aria-label="t('approval.snooze')"
                      :title="t('approval.snooze')"
                      @click="rejectGroup(group)"
                    >
                      <AlarmClockIcon class="h-4 w-4" />
                    </UiButton>
                    <UiButton
                      v-if="group.auditIds.length > 0"
                      variant="ghost"
                      size="sm"
                      class="h-7 w-7 p-0 text-muted-foreground hover:text-destructive hover:bg-destructive/10 dark:hover:bg-destructive/20"
                      :disabled="!!loading[group.key]"
                      aria-label="Dismiss"
                      title="Dismiss"
                      @click="dismissGroup(group)"
                    >
                      <XIcon class="h-4 w-4" />
                    </UiButton>
                    <!-- Checkbox for batch selection (after snooze icon) -->
                    <UiCheckbox
                      v-if="group.auditIds.length > 0"
                      :model-value="
                        isGroupPartiallySelected(group)
                          ? 'indeterminate'
                          : isGroupFullySelected(group)
                      "
                      class="h-3.5 w-3.5 shrink-0 cursor-pointer"
                      @click.stop="toggleGroupSelect(group)"
                    />
                    <span
                      v-if="group.auditIds.length === 0"
                      class="text-[10px] text-muted-foreground italic"
                      >{{ t('approval.awaitingEngine') }}</span
                    >
                  </div>
                </div>

                <!-- Expanded seasons -->
                <div
                  v-if="group.seasonCount > 0 && expandedKeys.has(group.key)"
                  class="ml-6 mt-0.5 space-y-0.5"
                >
                  <div
                    v-for="season in group.seasons"
                    :key="season.title"
                    class="flex items-center gap-3 rounded border border-border/50 bg-muted/20 px-3 py-1.5 text-xs hover:bg-muted/40 transition-colors"
                    :class="
                      selectedKeys.has(seasonKey(group.key, season.title))
                        ? 'ring-1 ring-primary/30 bg-primary/5'
                        : ''
                    "
                  >
                    <span
                      class="font-mono tabular-nums font-medium text-primary/80 shrink-0 w-12 text-right cursor-pointer"
                      @click="showSeasonDetail(season)"
                    >
                      {{ season.score.toFixed(2) }}
                    </span>
                    <span
                      class="flex-1 min-w-0 truncate text-muted-foreground cursor-pointer"
                      @click="showSeasonDetail(season)"
                      >{{ season.title }}</span
                    >
                    <!-- Size (right-aligned before action icons) -->
                    <span class="text-muted-foreground shrink-0 tabular-nums w-16 text-right">{{
                      formatBytes(season.sizeBytes)
                    }}</span>
                    <!-- Season-level actions (uses season auditId if available, otherwise group actions) -->
                    <div class="flex items-center gap-1 shrink-0">
                      <UiButton
                        v-if="group.auditIds.length > 0"
                        variant="ghost"
                        size="sm"
                        class="h-6 w-6 p-0 text-green-600 hover:text-green-700 hover:bg-green-100 dark:hover:bg-green-900/30"
                        :aria-label="t('approval.approve')"
                        @click.stop="
                          season.auditId !== null
                            ? approveSeason(season.auditId)
                            : approveGroup(group)
                        "
                      >
                        <CheckIcon class="h-3.5 w-3.5" />
                      </UiButton>
                      <UiButton
                        v-if="group.auditIds.length > 0"
                        variant="ghost"
                        size="sm"
                        class="h-6 w-6 p-0 text-amber-500 hover:text-amber-600 hover:bg-amber-100 dark:hover:bg-amber-900/30"
                        :aria-label="t('approval.snooze')"
                        :title="t('approval.snooze')"
                        @click.stop="
                          season.auditId !== null
                            ? snoozeSeason(season.auditId)
                            : rejectGroup(group)
                        "
                      >
                        <AlarmClockIcon class="h-3.5 w-3.5" />
                      </UiButton>
                      <UiButton
                        v-if="group.auditIds.length > 0"
                        variant="ghost"
                        size="sm"
                        class="h-6 w-6 p-0 text-muted-foreground hover:text-destructive hover:bg-destructive/10 dark:hover:bg-destructive/20"
                        aria-label="Dismiss"
                        @click.stop="
                          season.auditId !== null
                            ? dismissSeason(season.auditId)
                            : dismissGroup(group)
                        "
                      >
                        <XIcon class="h-3.5 w-3.5" />
                      </UiButton>
                      <!-- Season-level checkbox -->
                      <UiCheckbox
                        :model-value="selectedKeys.has(seasonKey(group.key, season.title))"
                        class="h-3.5 w-3.5 shrink-0 cursor-pointer"
                        @click.stop="toggleSelect(seasonKey(group.key, season.title))"
                      />
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </UiScrollArea>
    </UiCardContent>
  </UiCard>

  <!-- Score Detail Modal (group level) -->
  <ScoreDetailModal
    v-if="selectedGroup"
    :visible="!!selectedGroup"
    :media-name="selectedGroup.showTitle"
    :media-type="selectedGroup.type"
    :score="selectedGroup.score"
    :score-details="selectedGroup.scoreDetails"
    :size-bytes="selectedGroup.totalSizeBytes"
    :action="selectedGroup.state === 'approved' ? 'Approved' : 'Pending'"
    @close="selectedGroup = null"
  />

  <!-- Score Detail Modal (season level) -->
  <ScoreDetailModal
    v-if="selectedSeason"
    :visible="!!selectedSeason"
    :media-name="selectedSeason.title"
    :media-type="selectedSeason.type"
    :score="selectedSeason.score"
    :score-details="selectedSeason.scoreDetails"
    :size-bytes="selectedSeason.sizeBytes"
    action="Pending"
    @close="selectedSeason = null"
  />
</template>
