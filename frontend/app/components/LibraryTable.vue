<script setup lang="ts">
import {
  SearchIcon,
  RefreshCwIcon,
  LoaderCircleIcon,
  ArrowUpIcon,
  ArrowDownIcon,
  ArrowUpDownIcon,
  ShieldCheckIcon,
  CheckSquareIcon,
  SquareIcon,
  AlertTriangleIcon,
  Trash2Icon,
  XIcon,
  ClockIcon,
  CheckIcon,
  ZapIcon,
} from 'lucide-vue-next';
import { useVirtualizer } from '@tanstack/vue-virtual';
import type { EvaluatedItem, IntegrationConfig } from '~/types/api';
import { formatBytes } from '~/utils/format';

const props = defineProps<{
  items: EvaluatedItem[];
  integrations: IntegrationConfig[];
  loading: boolean;
}>();

const emit = defineEmits<{
  refresh: [];
  delete: [items: EvaluatedItem[]];
}>();

const { t } = useI18n();
const { viewMode } = useDisplayPrefs();

// ---------------------------------------------------------------------------
// Search & Filters
// ---------------------------------------------------------------------------
const search = ref('');
const typeFilter = ref<string | null>(null);
const integrationFilter = ref<number | null>(null);

const mediaTypes = computed(() => {
  const types = new Set(props.items.map((e) => e.item.type));
  return [...types].sort();
});

const integrationMap = computed(() => {
  // Build map from all integrations for name lookups
  const fullMap = new Map<number, string>();
  for (const integ of props.integrations) {
    fullMap.set(integ.id, integ.name);
  }
  return fullMap;
});

/** Integrations that actually have media items (excludes enrichment-only integrations). */
const mediaIntegrations = computed(() => {
  const idsWithMedia = new Set(props.items.map((e) => e.item.integrationId));
  return props.integrations.filter((i) => idsWithMedia.has(i.id));
});

// ---------------------------------------------------------------------------
// Sorting
// ---------------------------------------------------------------------------
type SortKey = 'title' | 'size' | 'score' | 'type' | 'integration';
const sortBy = ref<SortKey>('title');
const sortDir = ref<'asc' | 'desc'>('asc');

function toggleSort(key: SortKey) {
  if (sortBy.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc';
  } else {
    sortBy.value = key;
    sortDir.value = key === 'score' || key === 'size' ? 'desc' : 'asc';
  }
}

// ---------------------------------------------------------------------------
// Filtered + Sorted Items
// ---------------------------------------------------------------------------
const filteredItems = computed(() => {
  // Dedup: when season entries exist for a show, skip the show-level entry.
  // Season entries allow granular per-season actions (same logic as poller evaluate.go).
  const showsWithSeasons = new Set<string>();
  for (const e of props.items) {
    if (e.item.type === 'season' && e.item.showTitle) {
      showsWithSeasons.add(e.item.showTitle);
    }
  }
  let result = props.items.filter(
    (e) => !(e.item.type === 'show' && showsWithSeasons.has(e.item.title)),
  );

  if (search.value) {
    const q = search.value.toLowerCase();
    result = result.filter((e) => e.item.title.toLowerCase().includes(q));
  }

  if (typeFilter.value) {
    result = result.filter((e) => e.item.type === typeFilter.value);
  }

  if (integrationFilter.value !== null) {
    result = result.filter((e) => e.item.integrationId === integrationFilter.value);
  }

  // Sort
  const dir = sortDir.value === 'asc' ? 1 : -1;
  result = [...result].sort((a, b) => {
    switch (sortBy.value) {
      case 'title':
        return dir * a.item.title.localeCompare(b.item.title);
      case 'size':
        return dir * (a.item.sizeBytes - b.item.sizeBytes);
      case 'score':
        return dir * (a.score - b.score);
      case 'type':
        return dir * a.item.type.localeCompare(b.item.type);
      case 'integration': {
        const aName = integrationMap.value.get(a.item.integrationId) ?? '';
        const bName = integrationMap.value.get(b.item.integrationId) ?? '';
        return dir * aName.localeCompare(bName);
      }
      default:
        return 0;
    }
  });

  return result;
});

const hasFilters = computed(
  () => search.value || typeFilter.value || integrationFilter.value !== null,
);

// ---------------------------------------------------------------------------
// Virtual Scrolling
// ---------------------------------------------------------------------------
const TABLE_ROW_HEIGHT = 41;
const GRID_ROW_HEIGHT = 280;
const GRID_ROW_GAP = 12;

/** Type-safe index access into filteredItems. The virtualizer guarantees valid indices. */
function getEntry(index: number): EvaluatedItem {
  const entry = filteredItems.value[index];
  if (!entry) throw new Error(`Invalid virtualizer index: ${index}`);
  return entry;
}

const tableScrollRef = ref<HTMLElement | null>(null);
const gridScrollRef = ref<HTMLElement | null>(null);

const tableVirtualizer = useVirtualizer(
  computed(() => ({
    count: filteredItems.value.length,
    getScrollElement: () => tableScrollRef.value,
    estimateSize: () => TABLE_ROW_HEIGHT,
    overscan: 10,
  })),
);

/** Responsive column count mirrors the Tailwind grid breakpoints. */
const gridCols = ref(2);
function updateGridCols() {
  const w = window.innerWidth;
  if (w >= 1280) gridCols.value = 6;
  else if (w >= 1024) gridCols.value = 5;
  else if (w >= 768) gridCols.value = 4;
  else if (w >= 640) gridCols.value = 3;
  else gridCols.value = 2;
}
onMounted(() => {
  updateGridCols();
  window.addEventListener('resize', updateGridCols);
});
onUnmounted(() => {
  window.removeEventListener('resize', updateGridCols);
});

const gridRowCount = computed(() => Math.ceil(filteredItems.value.length / gridCols.value));

const gridVirtualizer = useVirtualizer(
  computed(() => ({
    count: gridRowCount.value,
    getScrollElement: () => gridScrollRef.value,
    estimateSize: () => GRID_ROW_HEIGHT + GRID_ROW_GAP,
    overscan: 3,
  })),
);

// Reset scroll position when filters / sort change
watch([search, typeFilter, integrationFilter, sortBy, sortDir], () => {
  tableVirtualizer.value.scrollToIndex(0);
  gridVirtualizer.value.scrollToIndex(0);
});

// ---------------------------------------------------------------------------
// Selection Mode
// ---------------------------------------------------------------------------
const selectionMode = ref(false);
const selectedIds = ref(new Set<string>());
const lastClickedIndex = ref<number | null>(null);

function itemKey(e: EvaluatedItem): string {
  return `${e.item.integrationId}:${e.item.externalId}`;
}

function enterSelectionMode() {
  selectionMode.value = true;
  selectedIds.value = new Set();
  lastClickedIndex.value = null;
}

function exitSelectionMode() {
  selectionMode.value = false;
  selectedIds.value = new Set();
  lastClickedIndex.value = null;
}

function toggleItem(e: EvaluatedItem, index: number, event?: MouseEvent) {
  if (e.isProtected) return;

  const key = itemKey(e);

  // Shift-click range selection
  if (event?.shiftKey && lastClickedIndex.value !== null) {
    const start = Math.min(lastClickedIndex.value, index);
    const end = Math.max(lastClickedIndex.value, index);
    for (let i = start; i <= end; i++) {
      const rangeItem = filteredItems.value[i];
      if (rangeItem && !rangeItem.isProtected) {
        selectedIds.value.add(itemKey(rangeItem));
      }
    }
    // Trigger reactivity
    selectedIds.value = new Set(selectedIds.value);
  } else {
    if (selectedIds.value.has(key)) {
      selectedIds.value.delete(key);
    } else {
      selectedIds.value.add(key);
    }
    selectedIds.value = new Set(selectedIds.value);
  }

  lastClickedIndex.value = index;
}

function selectAll() {
  for (const item of filteredItems.value) {
    if (!item.isProtected) {
      selectedIds.value.add(itemKey(item));
    }
  }
  selectedIds.value = new Set(selectedIds.value);
}

function deselectAll() {
  selectedIds.value = new Set();
}

const selectedItems = computed(() => props.items.filter((e) => selectedIds.value.has(itemKey(e))));

const selectedTotalBytes = computed(() =>
  selectedItems.value.reduce((sum, e) => sum + e.item.sizeBytes, 0),
);

// ---------------------------------------------------------------------------
// Force-Delete Dialog
// ---------------------------------------------------------------------------
const showDeleteDialog = ref(false);
const deleteLoading = ref(false);

function openDeleteDialog() {
  if (selectedItems.value.length === 0) return;
  showDeleteDialog.value = true;
}

async function confirmDelete() {
  deleteLoading.value = true;
  emit('delete', selectedItems.value);
  // Parent handles the API call and will close selection mode on success
}

function onDeleteComplete() {
  deleteLoading.value = false;
  showDeleteDialog.value = false;
  exitSelectionMode();
}

defineExpose({ onDeleteComplete });

// ---------------------------------------------------------------------------
// Detail Modal
// ---------------------------------------------------------------------------
const selectedDetail = ref<EvaluatedItem | null>(null);

function openDetail(e: EvaluatedItem) {
  if (selectionMode.value) return;
  selectedDetail.value = e;
}

// ---------------------------------------------------------------------------
// Table Columns
// ---------------------------------------------------------------------------
const tableColumns = computed(() => [
  { key: 'title' as SortKey, label: t('library.sortTitle'), class: 'font-medium' },
  { key: 'type' as SortKey, label: t('library.sortType'), class: 'w-24' },
  { key: 'integration' as SortKey, label: t('library.sortIntegration'), class: 'w-32' },
  { key: 'size' as SortKey, label: t('library.sortSize'), class: 'w-28 text-right' },
  { key: 'score' as SortKey, label: t('library.sortScore'), class: 'w-24 text-right' },
]);
</script>

<template>
  <UiCard
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{
      opacity: 1,
      y: 0,
      transition: { type: 'spring', stiffness: 260, damping: 24, delay: 200 },
    }"
  >
    <UiCardHeader>
      <div class="flex items-center justify-between">
        <div>
          <UiCardTitle>{{ t('library.title') }}</UiCardTitle>
          <UiCardDescription class="mt-1">
            {{ t('library.subtitle') }}
          </UiCardDescription>
        </div>
        <div class="flex items-center gap-2">
          <UiButton v-if="!selectionMode" variant="outline" size="sm" @click="enterSelectionMode">
            <SquareIcon class="w-3.5 h-3.5 mr-1" />
            {{ t('library.select') }}
          </UiButton>
          <template v-else>
            <UiButton variant="outline" size="sm" @click="selectAll">
              {{ t('library.selectAll') }}
            </UiButton>
            <UiButton variant="outline" size="sm" @click="deselectAll">
              {{ t('library.deselectAll') }}
            </UiButton>
            <UiButton variant="ghost" size="sm" @click="exitSelectionMode">
              <XIcon class="w-3.5 h-3.5 mr-1" />
              {{ t('library.cancel') }}
            </UiButton>
          </template>
          <UiButton variant="outline" size="sm" @click="$emit('refresh')">
            <component
              :is="loading ? LoaderCircleIcon : RefreshCwIcon"
              :class="{ 'animate-spin': loading }"
              class="w-3.5 h-3.5"
            />
          </UiButton>
        </div>
      </div>
    </UiCardHeader>
    <UiCardContent>
      <div v-if="loading" class="flex flex-col items-center justify-center py-12 gap-3">
        <LoaderCircleIcon class="w-6 h-6 text-primary animate-spin" />
        <p class="text-muted-foreground text-sm">{{ t('library.loadingHint') }}</p>
      </div>

      <div v-else-if="items.length === 0" class="text-center py-12 space-y-2">
        <p class="text-muted-foreground text-sm font-medium">{{ t('library.noItems') }}</p>
        <p class="text-muted-foreground/60 text-xs">{{ t('library.noItemsDesc') }}</p>
      </div>

      <div v-else>
        <!-- Search & Filters -->
        <div class="flex flex-col sm:flex-row gap-3 mb-4">
          <ViewModeToggle />
          <div class="relative flex-1">
            <SearchIcon
              class="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground pointer-events-none"
            />
            <UiInput
              v-model="search"
              :aria-label="t('library.sortTitle')"
              placeholder="Search by title…"
              class="pl-8"
            />
          </div>
          <div class="flex items-center gap-1.5 flex-wrap">
            <UiButton
              v-for="mt in mediaTypes"
              :key="mt"
              :variant="typeFilter === mt ? 'default' : 'outline'"
              size="sm"
              class="rounded-full h-7 px-3 text-xs capitalize"
              @click="typeFilter = typeFilter === mt ? null : mt"
            >
              {{ mt }}
            </UiButton>
            <template v-if="integrations.length > 1">
              <UiSeparator orientation="vertical" class="h-5 mx-1" />
              <UiSelect
                :model-value="integrationFilter === null ? 'all' : String(integrationFilter)"
                @update:model-value="
                  (v) => (integrationFilter = String(v) === 'all' ? null : Number(v))
                "
              >
                <UiSelectTrigger class="h-7 text-xs w-40">
                  <UiSelectValue :placeholder="t('library.allIntegrations')" />
                </UiSelectTrigger>
                <UiSelectContent>
                  <UiSelectItem value="all">{{ t('library.allIntegrations') }}</UiSelectItem>
                  <UiSelectItem
                    v-for="integ in mediaIntegrations"
                    :key="integ.id"
                    :value="String(integ.id)"
                  >
                    {{ integ.name }}
                  </UiSelectItem>
                </UiSelectContent>
              </UiSelect>
            </template>
            <!-- Sort control (always visible — supplements table headers in table mode) -->
            <UiSeparator orientation="vertical" class="h-5 mx-1" />
            <UiSelect :model-value="sortBy" @update:model-value="(v) => toggleSort(v as SortKey)">
              <UiSelectTrigger class="h-7 text-xs w-32">
                <UiSelectValue :placeholder="t('library.sortBy')" />
              </UiSelectTrigger>
              <UiSelectContent>
                <UiSelectItem v-for="col in tableColumns" :key="col.key" :value="col.key">
                  {{ col.label }}
                </UiSelectItem>
              </UiSelectContent>
            </UiSelect>
            <UiButton
              variant="ghost"
              size="sm"
              class="h-7 w-7 p-0"
              :aria-label="sortDir === 'asc' ? t('library.sortAsc') : t('library.sortDesc')"
              @click="sortDir = sortDir === 'asc' ? 'desc' : 'asc'"
            >
              <ArrowUpIcon v-if="sortDir === 'asc'" class="w-3.5 h-3.5" />
              <ArrowDownIcon v-else class="w-3.5 h-3.5" />
            </UiButton>
          </div>
        </div>

        <!-- Results count -->
        <div class="text-xs text-muted-foreground mb-2">
          <template v-if="hasFilters">
            {{
              t('library.filteredCount', { filtered: filteredItems.length, total: items.length })
            }}
          </template>
          <template v-else>
            {{ t('library.itemCount', { count: items.length }) }}
          </template>
        </div>

        <div
          v-if="filteredItems.length === 0"
          class="text-center py-8 text-muted-foreground text-sm"
        >
          {{ t('library.noFilterMatch') }}
        </div>

        <!-- Grid View (virtualized) -->
        <div
          v-else-if="viewMode === 'grid'"
          ref="gridScrollRef"
          class="library-scroll max-h-[calc(100vh-16rem)] overflow-y-auto"
        >
          <div
            :style="{
              height: `${gridVirtualizer.getTotalSize()}px`,
              position: 'relative',
              width: '100%',
            }"
          >
            <div
              v-for="vRow in gridVirtualizer.getVirtualItems()"
              :key="vRow.index"
              :style="{
                position: 'absolute',
                top: 0,
                left: 0,
                width: '100%',
                height: `${vRow.size - GRID_ROW_GAP}px`,
                transform: `translateY(${vRow.start}px)`,
              }"
              class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-3"
            >
              <template v-for="col in gridCols" :key="col">
                <MediaPosterCard
                  v-if="vRow.index * gridCols + (col - 1) < filteredItems.length"
                  :title="getEntry(vRow.index * gridCols + (col - 1)).item.title"
                  :poster-url="getEntry(vRow.index * gridCols + (col - 1)).item.posterUrl"
                  :year="getEntry(vRow.index * gridCols + (col - 1)).item.year"
                  :media-type="getEntry(vRow.index * gridCols + (col - 1)).item.type"
                  :score="
                    getEntry(vRow.index * gridCols + (col - 1)).isProtected
                      ? undefined
                      : getEntry(vRow.index * gridCols + (col - 1)).score
                  "
                  :size-bytes="getEntry(vRow.index * gridCols + (col - 1)).item.sizeBytes"
                  :is-protected="getEntry(vRow.index * gridCols + (col - 1)).isProtected"
                  :queue-status="getEntry(vRow.index * gridCols + (col - 1)).queueStatus"
                  :selectable="selectionMode"
                  :selected="selectedIds.has(itemKey(getEntry(vRow.index * gridCols + (col - 1))))"
                  @click="
                    selectionMode
                      ? toggleItem(
                          getEntry(vRow.index * gridCols + (col - 1)),
                          vRow.index * gridCols + (col - 1),
                          $event,
                        )
                      : openDetail(getEntry(vRow.index * gridCols + (col - 1)))
                  "
                  @select="
                    toggleItem(
                      getEntry(vRow.index * gridCols + (col - 1)),
                      vRow.index * gridCols + (col - 1),
                    )
                  "
                />
              </template>
            </div>
          </div>
        </div>

        <!-- Table View (virtualized) -->
        <div v-else class="overflow-x-auto relative">
          <UiTable>
            <UiTableHeader class="sticky top-0 z-10 bg-background">
              <UiTableRow>
                <UiTableHead v-if="selectionMode" class="w-10" />
                <UiTableHead
                  v-for="col in tableColumns"
                  :key="col.key"
                  :class="[col.class, 'cursor-pointer select-none group']"
                  @click="toggleSort(col.key)"
                >
                  <span
                    :class="[
                      'inline-flex items-center gap-1',
                      col.key === 'size' || col.key === 'score' ? 'justify-end' : '',
                    ]"
                  >
                    {{ col.label }}
                    <ArrowUpIcon v-if="sortBy === col.key && sortDir === 'asc'" class="w-3 h-3" />
                    <ArrowDownIcon
                      v-else-if="sortBy === col.key && sortDir === 'desc'"
                      class="w-3 h-3"
                    />
                    <ArrowUpDownIcon
                      v-else
                      class="w-3 h-3 opacity-0 group-hover:opacity-50 transition-opacity"
                    />
                  </span>
                </UiTableHead>
              </UiTableRow>
            </UiTableHeader>
          </UiTable>
          <div
            ref="tableScrollRef"
            class="library-scroll max-h-[calc(100vh-20rem)] overflow-y-auto"
          >
            <UiTable>
              <UiTableBody>
                <tr :style="{ height: `${tableVirtualizer.getVirtualItems()[0]?.start ?? 0}px` }" />
                <UiTableRow
                  v-for="vRow in tableVirtualizer.getVirtualItems()"
                  :key="itemKey(getEntry(vRow.index))"
                  class="cursor-pointer"
                  @click="
                    selectionMode
                      ? toggleItem(getEntry(vRow.index), vRow.index, $event)
                      : openDetail(getEntry(vRow.index))
                  "
                >
                  <UiTableCell v-if="selectionMode" class="w-10 text-center">
                    <component
                      :is="
                        getEntry(vRow.index).isProtected
                          ? ShieldCheckIcon
                          : selectedIds.has(itemKey(getEntry(vRow.index)))
                            ? CheckSquareIcon
                            : SquareIcon
                      "
                      class="w-4 h-4"
                      :class="
                        getEntry(vRow.index).isProtected
                          ? 'text-emerald-500 cursor-not-allowed'
                          : 'text-muted-foreground'
                      "
                      :title="
                        getEntry(vRow.index).isProtected ? t('library.protectedTooltip') : undefined
                      "
                    />
                  </UiTableCell>
                  <UiTableCell class="font-medium">
                    <div class="flex items-center gap-1.5">
                      <span class="truncate">{{ getEntry(vRow.index).item.title }}</span>
                      <UiBadge
                        v-if="getEntry(vRow.index).queueStatus === 'pending'"
                        variant="outline"
                        class="text-[10px] border-amber-500/50 bg-amber-500/10 text-amber-600 dark:text-amber-400 shrink-0"
                      >
                        <ClockIcon class="w-3 h-3" />
                        {{ t('library.queuePending') }}
                      </UiBadge>
                      <UiBadge
                        v-else-if="getEntry(vRow.index).queueStatus === 'approved'"
                        variant="outline"
                        class="text-[10px] border-emerald-500/50 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 shrink-0"
                      >
                        <CheckIcon class="w-3 h-3" />
                        {{ t('library.queueApproved') }}
                      </UiBadge>
                      <UiBadge
                        v-else-if="getEntry(vRow.index).queueStatus === 'user_initiated'"
                        variant="destructive"
                        class="text-[10px] shrink-0"
                      >
                        <ZapIcon class="w-3 h-3" />
                        {{ t('library.queueDelete') }}
                      </UiBadge>
                      <UiBadge
                        v-else-if="getEntry(vRow.index).queueStatus === 'deleting'"
                        variant="destructive"
                        class="text-[10px] shrink-0 animate-pulse"
                      >
                        <LoaderCircleIcon class="w-3 h-3 animate-spin" />
                        {{ t('library.queueDeleting') }}
                      </UiBadge>
                    </div>
                  </UiTableCell>
                  <UiTableCell>
                    <UiBadge variant="secondary" class="text-[10px] capitalize">
                      {{ getEntry(vRow.index).item.type }}
                    </UiBadge>
                  </UiTableCell>
                  <UiTableCell>
                    <span class="text-xs text-muted-foreground">
                      {{ integrationMap.get(getEntry(vRow.index).item.integrationId) ?? '—' }}
                    </span>
                  </UiTableCell>
                  <UiTableCell class="text-right">
                    <span class="text-xs tabular-nums text-muted-foreground">
                      {{ formatBytes(getEntry(vRow.index).item.sizeBytes) }}
                    </span>
                  </UiTableCell>
                  <UiTableCell class="text-right">
                    <span
                      class="text-xs font-mono tabular-nums font-semibold"
                      :class="
                        getEntry(vRow.index).isProtected ? 'text-emerald-500' : 'text-primary'
                      "
                    >
                      {{
                        getEntry(vRow.index).isProtected
                          ? 'Protected'
                          : getEntry(vRow.index).score.toFixed(2)
                      }}
                    </span>
                  </UiTableCell>
                </UiTableRow>
                <tr
                  :style="{
                    height: `${tableVirtualizer.getTotalSize() - (tableVirtualizer.getVirtualItems().at(-1)?.end ?? 0)}px`,
                  }"
                />
              </UiTableBody>
            </UiTable>
          </div>
        </div>
      </div>
    </UiCardContent>
  </UiCard>

  <!-- Floating Action Bar -->
  <Teleport to="body">
    <Transition
      enter-active-class="transition-all duration-200 ease-out"
      enter-from-class="translate-y-full opacity-0"
      enter-to-class="translate-y-0 opacity-100"
      leave-active-class="transition-all duration-150 ease-in"
      leave-from-class="translate-y-0 opacity-100"
      leave-to-class="translate-y-full opacity-0"
    >
      <div
        v-if="selectionMode && selectedIds.size > 0"
        class="fixed bottom-6 left-1/2 -translate-x-1/2 z-50 flex items-center gap-4 rounded-xl border bg-background/95 backdrop-blur-sm shadow-lg px-5 py-3"
      >
        <span class="text-sm font-medium">
          {{ t('library.selected', { count: selectedIds.size }) }}
        </span>
        <UiSeparator orientation="vertical" class="h-5" />
        <span class="text-sm text-muted-foreground tabular-nums">
          {{ formatBytes(selectedTotalBytes) }}
        </span>
        <UiButton variant="ghost" size="sm" @click="exitSelectionMode">
          {{ t('library.cancel') }}
        </UiButton>
        <UiButton variant="destructive" size="sm" @click="openDeleteDialog">
          <Trash2Icon class="w-3.5 h-3.5 mr-1" />
          {{ t('library.delete') }}
        </UiButton>
      </div>
    </Transition>
  </Teleport>

  <!-- Delete Confirmation Dialog -->
  <UiDialog
    :open="showDeleteDialog"
    @update:open="
      (val: boolean) => {
        if (!val) showDeleteDialog = false;
      }
    "
  >
    <UiDialogContent class="max-w-lg">
      <UiDialogHeader>
        <UiDialogTitle class="flex items-center gap-2">
          <AlertTriangleIcon class="w-5 h-5 text-destructive" />
          {{ t('library.deleteTitle', { count: selectedItems.length }) }}
        </UiDialogTitle>
        <UiDialogDescription>
          {{ t('library.deleteDesc') }}
        </UiDialogDescription>
      </UiDialogHeader>

      <div class="max-h-60 overflow-y-auto my-2 space-y-1">
        <div
          v-for="entry in selectedItems"
          :key="itemKey(entry)"
          class="flex items-center justify-between text-sm px-2 py-1 rounded hover:bg-muted/50"
        >
          <span class="truncate flex-1 mr-2">{{ entry.item.title }}</span>
          <span class="text-muted-foreground tabular-nums shrink-0">
            {{ formatBytes(entry.item.sizeBytes) }}
          </span>
        </div>
      </div>

      <div class="flex items-center justify-between text-sm font-medium px-2 pt-2 border-t">
        <span>{{ t('library.deleteTotal', { size: formatBytes(selectedTotalBytes) }) }}</span>
      </div>

      <UiDialogFooter>
        <UiButton variant="outline" :disabled="deleteLoading" @click="showDeleteDialog = false">
          {{ t('library.cancel') }}
        </UiButton>
        <UiButton variant="destructive" :disabled="deleteLoading" @click="confirmDelete">
          <LoaderCircleIcon v-if="deleteLoading" class="w-3.5 h-3.5 mr-1 animate-spin" />
          <Trash2Icon v-else class="w-3.5 h-3.5 mr-1" />
          {{ t('library.delete') }}
        </UiButton>
      </UiDialogFooter>
    </UiDialogContent>
  </UiDialog>

  <!-- Score Detail Modal -->
  <ScoreDetailModal
    v-if="selectedDetail"
    :visible="!!selectedDetail"
    :media-name="selectedDetail.item.title"
    :media-type="selectedDetail.item.type"
    :score="selectedDetail.score"
    :score-details="JSON.stringify(selectedDetail.factors)"
    :size-bytes="selectedDetail.item.sizeBytes"
    :action="selectedDetail.isProtected ? 'protected' : ''"
    @close="selectedDetail = null"
  />
</template>
