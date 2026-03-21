<template>
  <div>
    <!-- Pull-to-refresh indicator -->
    <PullToRefreshIndicator
      :pull-distance="pullDistance"
      :pull-progress="pullProgress"
      :is-refreshing="isRefreshing"
    />

    <!-- Header (only shown in standalone mode, not when embedded in Library) -->
    <div
      v-if="showHeader"
      data-slot="page-header"
      class="mb-6 flex flex-col md:flex-row md:items-center justify-between gap-4"
    >
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          {{ $t('audit.title') }}
        </h1>
        <p class="text-muted-foreground mt-1.5">
          {{ $t('audit.subtitle') }}
        </p>
      </div>
      <UiButton variant="outline" @click="fetchLogs">
        <LoaderCircleIcon v-if="pending" class="w-4 h-4 animate-spin" />
        <RefreshCwIcon v-else class="w-4 h-4" />
        {{ $t('common.refresh') }}
      </UiButton>
    </div>

    <UiCard
      v-motion
      :initial="{ opacity: 0, y: 8 }"
      :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24 } }"
      class="overflow-hidden"
    >
      <!-- Search & Action Filters -->
      <div class="px-5 pt-5 pb-3 space-y-3 border-b border-border">
        <div class="flex flex-col sm:flex-row gap-3">
          <div class="relative flex-1">
            <SearchIcon
              class="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground pointer-events-none"
            />
            <UiInput
              :model-value="auditSearch"
              aria-label="Search audit logs by media name"
              :placeholder="$t('audit.searchPlaceholder')"
              class="pl-8"
              @update:model-value="onSearchInput"
            />
          </div>
          <div class="flex items-center gap-1.5 flex-wrap">
            <UiButton
              v-for="action in auditActionTypes"
              :key="action.value"
              :variant="auditActionFilter === action.value ? 'default' : 'outline'"
              size="sm"
              class="rounded-full h-7 px-3 text-xs"
              @click="toggleActionFilter(action.value)"
            >
              {{ action.label }}
            </UiButton>
          </div>

          <!-- Refresh button (embedded mode) -->
          <UiButton v-if="!showHeader" variant="outline" size="sm" @click="fetchLogs">
            <LoaderCircleIcon v-if="pending" class="w-4 h-4 animate-spin" />
            <RefreshCwIcon v-else class="w-4 h-4" />
            {{ $t('common.refresh') }}
          </UiButton>
        </div>
      </div>

      <div v-if="pending && logs.length === 0" class="p-4">
        <SkeletonTable :rows="8" :column-widths="['28%', '10%', '10%', '15%', '22%', '8%']" />
      </div>

      <div
        v-else-if="!pending && logs.length === 0"
        class="flex flex-col items-center justify-center py-20 text-muted-foreground"
      >
        <ClockIcon class="w-10 h-10 mb-3" />
        <span class="text-sm font-medium">{{ $t('audit.noHistory') }}</span>
      </div>

      <div
        v-else
        ref="auditScrollRef"
        class="overflow-x-auto max-h-[600px] overflow-y-auto relative"
      >
        <UiTable>
          <UiTableHeader class="sticky top-0 z-10 bg-background">
            <UiTableRow>
              <UiTableHead
                class="cursor-pointer select-none group"
                @click="toggleAuditSort('created_at')"
              >
                <span class="inline-flex items-center gap-1">
                  {{ $t('audit.timestamp') }}
                  <ArrowUpIcon
                    v-if="auditSortBy === 'created_at' && auditSortDir === 'asc'"
                    class="w-3 h-3"
                  />
                  <ArrowDownIcon
                    v-else-if="auditSortBy === 'created_at' && auditSortDir === 'desc'"
                    class="w-3 h-3"
                  />
                  <ArrowUpDownIcon
                    v-else
                    class="w-3 h-3 opacity-0 group-hover:opacity-50 transition-opacity"
                  />
                </span>
              </UiTableHead>
              <UiTableHead
                class="cursor-pointer select-none group"
                @click="toggleAuditSort('media_name')"
              >
                <span class="inline-flex items-center gap-1">
                  {{ $t('audit.mediaTitle') }}
                  <ArrowUpIcon
                    v-if="auditSortBy === 'media_name' && auditSortDir === 'asc'"
                    class="w-3 h-3"
                  />
                  <ArrowDownIcon
                    v-else-if="auditSortBy === 'media_name' && auditSortDir === 'desc'"
                    class="w-3 h-3"
                  />
                  <ArrowUpDownIcon
                    v-else
                    class="w-3 h-3 opacity-0 group-hover:opacity-50 transition-opacity"
                  />
                </span>
              </UiTableHead>
              <UiTableHead>{{ $t('audit.type') }}</UiTableHead>
              <UiTableHead
                class="cursor-pointer select-none group"
                @click="toggleAuditSort('action')"
              >
                <span class="inline-flex items-center gap-1">
                  {{ $t('audit.result') }}
                  <ArrowUpIcon
                    v-if="auditSortBy === 'action' && auditSortDir === 'asc'"
                    class="w-3 h-3"
                  />
                  <ArrowDownIcon
                    v-else-if="auditSortBy === 'action' && auditSortDir === 'desc'"
                    class="w-3 h-3"
                  />
                  <ArrowUpDownIcon
                    v-else
                    class="w-3 h-3 opacity-0 group-hover:opacity-50 transition-opacity"
                  />
                </span>
              </UiTableHead>
              <UiTableHead>{{ $t('audit.score') }}</UiTableHead>
              <UiTableHead
                class="text-right cursor-pointer select-none group"
                @click="toggleAuditSort('size_bytes')"
              >
                <span class="inline-flex items-center gap-1 justify-end">
                  {{ $t('audit.space') }}
                  <ArrowUpIcon
                    v-if="auditSortBy === 'size_bytes' && auditSortDir === 'asc'"
                    class="w-3 h-3"
                  />
                  <ArrowDownIcon
                    v-else-if="auditSortBy === 'size_bytes' && auditSortDir === 'desc'"
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
          <UiTableBody>
            <template v-for="group in renderedAuditGroups" :key="group.key">
              <UiTableRow
                class="cursor-pointer"
                @click="
                  selectItem(group.entry);
                  group.seasons.length > 0 && toggleGroup(group.key);
                "
              >
                <UiTableCell class="text-xs text-muted-foreground whitespace-nowrap">
                  <DateDisplay :date="group.entry.createdAt" :always-exact="true" />
                </UiTableCell>
                <UiTableCell class="font-medium whitespace-nowrap">
                  <div class="flex items-center gap-2">
                    <span class="truncate">{{ group.entry.mediaName }}</span>
                    <button
                      v-if="group.seasons.length > 0"
                      :aria-label="
                        expandedGroups.has(group.key) ? 'Collapse seasons' : 'Expand seasons'
                      "
                      :aria-expanded="expandedGroups.has(group.key)"
                      class="text-muted-foreground hover:text-foreground transition-colors shrink-0 inline-flex items-center gap-0.5"
                      @click.stop="toggleGroup(group.key)"
                    >
                      <ChevronRightIcon
                        class="w-3.5 h-3.5 transition-transform duration-200"
                        :class="{ 'rotate-90': expandedGroups.has(group.key) }"
                      />
                      <span class="text-xs text-muted-foreground font-normal whitespace-nowrap"
                        >({{ group.seasons.length }} season{{
                          group.seasons.length !== 1 ? 's' : ''
                        }})</span
                      >
                    </button>
                  </div>
                </UiTableCell>
                <UiTableCell>
                  <UiBadge variant="secondary" class="capitalize">
                    {{ group.entry.mediaType }}
                  </UiBadge>
                </UiTableCell>
                <UiTableCell>
                  <UiBadge :variant="actionBadgeVariant(group.entry.action)">
                    {{ actionLabel(group.entry.action) }}
                  </UiBadge>
                </UiTableCell>
                <UiTableCell>
                  <ScoreBreakdown
                    :score="group.entry.score"
                    :score-details="group.entry.scoreDetails || ''"
                  />
                </UiTableCell>
                <UiTableCell class="text-right font-mono text-xs tabular-nums">
                  {{ (group.entry.sizeBytes / 1024 / 1024 / 1024).toFixed(2) }} GB
                </UiTableCell>
              </UiTableRow>
              <template v-if="expandedGroups.has(group.key)">
                <UiTableRow
                  v-for="season in group.seasons"
                  :key="season.id"
                  class="bg-muted/30 cursor-pointer"
                  @click.stop="selectItem(season)"
                >
                  <UiTableCell class="text-xs text-muted-foreground whitespace-nowrap pl-8">
                    <DateDisplay :date="season.createdAt" :always-exact="true" />
                  </UiTableCell>
                  <UiTableCell class="text-muted-foreground whitespace-nowrap pl-8">
                    <span class="inline-flex items-center gap-1.5">
                      <span class="w-3 h-px bg-border inline-block" />
                      {{ extractSeasonLabel(season.mediaName) }}
                    </span>
                  </UiTableCell>
                  <UiTableCell>
                    <UiBadge variant="secondary" class="capitalize">
                      {{ season.mediaType }}
                    </UiBadge>
                  </UiTableCell>
                  <UiTableCell>
                    <UiBadge :variant="actionBadgeVariant(season.action)">
                      {{ actionLabel(season.action) }}
                    </UiBadge>
                  </UiTableCell>
                  <UiTableCell>
                    <ScoreBreakdown
                      :score="season.score"
                      :score-details="season.scoreDetails || ''"
                      size="sm"
                    />
                  </UiTableCell>
                  <UiTableCell
                    class="text-right font-mono text-xs tabular-nums text-muted-foreground"
                  >
                    {{ (season.sizeBytes / 1024 / 1024 / 1024).toFixed(2) }} GB
                  </UiTableCell>
                </UiTableRow>
              </template>
            </template>
          </UiTableBody>
        </UiTable>
        <!-- Progressive rendering indicator -->
        <div
          v-if="renderedAuditGroups.length < groupedLogs.length"
          class="flex items-center justify-center py-3 text-xs text-muted-foreground gap-2"
        >
          <LoaderCircleIcon class="w-3.5 h-3.5 animate-spin" />
          Showing {{ renderedAuditGroups.length }} of {{ groupedLogs.length }} — scroll for more
        </div>
        <!-- Load more from server indicator -->
        <div
          v-else-if="logs.length < total && !loadingMore"
          class="flex items-center justify-center py-3 text-xs text-muted-foreground gap-2"
        >
          <LoaderCircleIcon class="w-3.5 h-3.5 animate-spin" />
          Loading more…
        </div>
      </div>

      <div
        v-if="logs.length > 0"
        class="flex items-center justify-between px-5 py-3 border-t border-border"
      >
        <span class="text-xs text-muted-foreground"
          >{{ groupedLogs.length }} groups from {{ logs.length }} of {{ total }} entries</span
        >
      </div>
    </UiCard>

    <ScoreDetailModal
      v-if="selectedItem"
      :visible="!!selectedItem"
      :media-name="selectedItem.mediaName"
      :media-type="selectedItem.mediaType"
      :score="selectedItem._score ?? 0"
      :score-details="selectedItem.scoreDetails || ''"
      :size-bytes="selectedItem.sizeBytes"
      :action="selectedItem.action"
      :created-at="selectedItem.createdAt"
      @close="selectedItem = null"
    />
  </div>
</template>

<script setup lang="ts">
import { useInfiniteScroll } from '@vueuse/core';
import {
  RefreshCwIcon,
  LoaderCircleIcon,
  ClockIcon,
  ChevronRightIcon,
  SearchIcon,
  ArrowUpIcon,
  ArrowDownIcon,
  ArrowUpDownIcon,
} from 'lucide-vue-next';
import type { AuditLogEntry, AuditResponse, SelectedDetailItem } from '~/types/api';

withDefaults(
  defineProps<{
    /** Show the standalone header with title and subtitle */
    showHeader?: boolean;
  }>(),
  { showHeader: true },
);

const api = useApi();

// Pull-to-refresh for touch devices
const { isRefreshing, pullProgress, pullDistance } = usePullToRefresh(async () => {
  await resetAndFetch();
});

const logs = ref<AuditLogEntry[]>([]);
const total = ref(0);
const pending = ref(false);
const loadingMore = ref(false);
const batchSize = 100;
const selectedItem = ref<SelectedDetailItem | null>(null);

// Audit filters
const auditSearch = ref('');
const auditActionFilter = ref<string | null>(null);
// Action values must match the backend db.Action* constants exactly
// (deleted, dry_delete, cancelled) — sent as ?action= query param.
const auditActionTypes = [
  { value: 'deleted', label: 'Deleted' },
  { value: 'dry_delete', label: 'Dry-Delete' },
] as const;
let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null;

// Audit sorting (server-side)
type AuditSortColumn = 'created_at' | 'media_name' | 'size_bytes' | 'action';
const auditSortBy = ref<AuditSortColumn>('created_at');
const auditSortDir = ref<'asc' | 'desc'>('desc');

function toggleAuditSort(column: AuditSortColumn) {
  if (auditSortBy.value === column) {
    auditSortDir.value = auditSortDir.value === 'asc' ? 'desc' : 'asc';
  } else {
    auditSortBy.value = column;
    auditSortDir.value = column === 'created_at' || column === 'size_bytes' ? 'desc' : 'asc';
  }
  resetAndFetch();
}

function selectItem(entry: AuditLogEntry) {
  const score = entry.score ?? 0;
  selectedItem.value = {
    mediaName: entry.mediaName,
    mediaType: entry.mediaType,
    _score: score,
    scoreDetails: entry.scoreDetails || '',
    sizeBytes: entry.sizeBytes,
    action: entry.action,
    createdAt: entry.createdAt,
  };
}

// ─── Data Fetching (Infinite Scroll) ──────────────────────────────────────────
async function fetchLogs(append = false) {
  if (append) {
    loadingMore.value = true;
  } else {
    pending.value = true;
  }
  try {
    const params = new URLSearchParams({
      limit: String(batchSize),
      offset: String(append ? logs.value.length : 0),
    });
    if (auditSearch.value.trim()) {
      params.set('search', auditSearch.value.trim());
    }
    if (auditActionFilter.value) {
      params.set('action', auditActionFilter.value);
    }
    params.set('sort_by', auditSortBy.value);
    params.set('sort_dir', auditSortDir.value);
    const data = (await api(`/api/v1/audit-log?${params.toString()}`)) as AuditResponse;
    if (data?.data) {
      if (append) {
        logs.value = [...logs.value, ...data.data];
      } else {
        logs.value = data.data;
      }
      total.value = data.total;
    }
  } catch (err) {
    console.warn('[Audit] fetchLogs failed:', err);
  } finally {
    pending.value = false;
    loadingMore.value = false;
  }
}

async function resetAndFetch() {
  logs.value = [];
  visibleCount.value = 100;
  await fetchLogs(false);
}

function onSearchInput(value: string | number) {
  auditSearch.value = String(value);
  if (searchDebounceTimer) clearTimeout(searchDebounceTimer);
  searchDebounceTimer = setTimeout(() => {
    resetAndFetch();
  }, 400);
}

function toggleActionFilter(action: string) {
  auditActionFilter.value = auditActionFilter.value === action ? null : action;
  resetAndFetch();
}

onMounted(() => fetchLogs(false));

// ─── Show/Season Grouping ─────────────────────────────────────────────────────
interface AuditGroupItem {
  key: string;
  entry: AuditLogEntry;
  seasons: AuditLogEntry[];
}

const groupedLogs = computed<AuditGroupItem[]>(() => {
  const groups: AuditGroupItem[] = [];
  const showMap = new Map<string, number>();

  for (const log of logs.value) {
    // Try to group season entries under their parent show
    if (log.mediaType === 'season' && log.mediaName.includes(' - Season ')) {
      const showName = log.mediaName.split(' - Season ')[0] ?? log.mediaName;
      const groupIdx = showMap.get(showName);
      const existingGroup = groupIdx !== undefined ? groups[groupIdx] : undefined;
      if (existingGroup) {
        existingGroup.seasons.push(log);
        continue;
      }
      // Orphan season — create a virtual show group for it
      showMap.set(showName, groups.length);
      groups.push({
        key: `show-${showName}`,
        entry: { ...log, mediaName: showName, mediaType: 'show' },
        seasons: [log],
      });
      continue;
    }

    const key = `${log.id}-${log.mediaName}`;
    if (log.mediaType === 'show') {
      showMap.set(log.mediaName, groups.length);
    }
    groups.push({ key, entry: log, seasons: [] });
  }

  return groups;
});

// ─── Progressive Rendering (Virtual Scroll) ─────────────────────────────────
// Render groups incrementally and fetch more from server as user scrolls.
const auditScrollRef = ref<HTMLElement | null>(null);
const visibleCount = ref(100);

const renderedAuditGroups = computed(() => groupedLogs.value.slice(0, visibleCount.value));

useInfiniteScroll(
  auditScrollRef,
  async () => {
    // First: render more already-fetched groups
    if (visibleCount.value < groupedLogs.value.length) {
      visibleCount.value = Math.min(visibleCount.value + 100, groupedLogs.value.length);
      return;
    }
    // Second: if all fetched groups are rendered, load more from server
    if (logs.value.length < total.value && !loadingMore.value) {
      await fetchLogs(true);
      visibleCount.value = groupedLogs.value.length;
    }
  },
  {
    distance: 200,
    canLoadMore: () =>
      visibleCount.value < groupedLogs.value.length || logs.value.length < total.value,
  },
);

// ─── Expand/Collapse state ────────────────────────────────────────────────────
const expandedGroups = ref(new Set<string>());

function toggleGroup(key: string) {
  const next = new Set(expandedGroups.value);
  if (next.has(key)) {
    next.delete(key);
  } else {
    next.add(key);
  }
  expandedGroups.value = next;
}

function extractSeasonLabel(mediaName: string): string {
  const parts = mediaName.split(' - Season ');
  return parts.length > 1 ? `Season ${parts[parts.length - 1]}` : mediaName;
}

// ─── Action badge variant mapping (uses raw backend db.Action* values) ────────
function actionBadgeVariant(action: string): 'destructive' | 'outline' | 'secondary' | 'default' {
  if (action === 'deleted') return 'destructive';
  if (action === 'dry_delete') return 'outline';
  return 'default';
}

/** Human-readable label for raw backend action values */
function actionLabel(action: string): string {
  switch (action) {
    case 'deleted':
      return 'Deleted';
    case 'dry_delete':
      return 'Dry-Delete';
    case 'cancelled':
      return 'Cancelled';
    default:
      return action;
  }
}
</script>
