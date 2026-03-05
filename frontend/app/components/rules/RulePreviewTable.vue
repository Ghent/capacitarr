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
          <UiCardTitle>{{ $t('rules.deletionPriority') }}</UiCardTitle>
          <UiCardDescription class="mt-1">
            {{ $t('rules.deletionPriorityDesc') }}
          </UiCardDescription>
        </div>
        <UiButton variant="outline" size="sm" @click="$emit('refresh')">
          <component
            :is="loading ? LoaderCircleIcon : RefreshCwIcon"
            :class="{ 'animate-spin': loading }"
            class="w-3.5 h-3.5"
          />
          {{ $t('common.refresh') }}
        </UiButton>
      </div>
    </UiCardHeader>
    <UiCardContent>
      <!-- Disk below threshold banner -->
      <div
        v-if="!loading && preview.length > 0 && diskContext && diskContext.bytesToFree === 0"
        class="mb-4 rounded-md border border-emerald-500/30 bg-emerald-500/5 px-4 py-3 text-sm text-emerald-600 dark:text-emerald-400 flex items-center gap-2"
      >
        <CheckIcon class="w-4 h-4 shrink-0" />
        {{ $t('rules.diskBelowThreshold') }}
      </div>

      <div v-if="loading" class="flex items-center justify-center py-12">
        <component :is="LoaderCircleIcon" class="w-6 h-6 text-primary animate-spin" />
      </div>

      <div v-else-if="preview.length === 0" class="text-center py-8 text-muted-foreground text-sm">
        {{ $t('rules.noItemsToEvaluate') }}
      </div>

      <div v-else>
        <!-- Search & Filters -->
        <div class="flex flex-col sm:flex-row gap-3 mb-4">
          <div class="relative flex-1">
            <SearchIcon
              class="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground pointer-events-none"
            />
            <UiInput
              v-model="previewSearch"
              aria-label="Search deletion priority by title"
              placeholder="Search by title…"
              class="pl-8"
            />
          </div>
          <div class="flex items-center gap-1.5 flex-wrap">
            <UiButton
              v-for="mt in previewMediaTypes"
              :key="mt"
              :variant="previewTypeFilter === mt ? 'default' : 'outline'"
              size="sm"
              class="rounded-full h-7 px-3 text-xs capitalize"
              @click="previewTypeFilter = previewTypeFilter === mt ? null : mt"
            >
              {{ mt }}
            </UiButton>
            <UiSeparator orientation="vertical" class="h-5 mx-1" />
            <UiButton
              :variant="previewStatusFilter === 'protected' ? 'default' : 'outline'"
              size="sm"
              class="rounded-full h-7 px-3 text-xs"
              @click="
                previewStatusFilter = previewStatusFilter === 'protected' ? 'all' : 'protected'
              "
            >
              <ShieldCheckIcon class="w-3 h-3 mr-1" />
              Protected
            </UiButton>
            <UiButton
              :variant="previewStatusFilter === 'unprotected' ? 'default' : 'outline'"
              size="sm"
              class="rounded-full h-7 px-3 text-xs"
              @click="
                previewStatusFilter = previewStatusFilter === 'unprotected' ? 'all' : 'unprotected'
              "
            >
              Unprotected
            </UiButton>
          </div>
        </div>

        <!-- Results count -->
        <div class="text-xs text-muted-foreground mb-2">
          <template v-if="previewSearch || previewTypeFilter || previewStatusFilter !== 'all'">
            {{ filteredGroupedPreview.length }} of {{ groupedPreview.length }} items
          </template>
          <template v-else> {{ groupedPreview.length }} items </template>
        </div>

        <div
          v-if="filteredGroupedPreview.length === 0"
          class="text-center py-8 text-muted-foreground text-sm"
        >
          No items match filters.
        </div>

        <div
          v-else
          ref="tableScrollRef"
          class="overflow-x-auto max-h-[600px] overflow-y-auto relative"
        >
          <UiTable>
            <UiTableHeader class="sticky top-0 z-10 bg-background">
              <UiTableRow>
                <UiTableHead
                  v-for="col in tableColumns"
                  :key="col.key"
                  :class="[col.class, 'cursor-pointer select-none group']"
                  @click="togglePreviewSort(col.key)"
                >
                  <span
                    :class="[
                      'inline-flex items-center gap-1',
                      col.key === 'size' ? 'justify-end' : '',
                    ]"
                  >
                    {{ col.label }}
                    <ArrowUpIcon
                      v-if="previewSortBy === col.key && previewSortDir === 'asc'"
                      class="w-3 h-3"
                    />
                    <ArrowDownIcon
                      v-else-if="previewSortBy === col.key && previewSortDir === 'desc'"
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
              <template v-for="(group, groupIdx) in renderedGroups" :key="group.key">
                <!-- Deletion line: inserted before the first item that falls below the cutoff -->
                <UiTableRow
                  v-if="deletionLineIndex !== null && deletionLineIndex === groupIdx"
                  class="pointer-events-none"
                >
                  <UiTableCell :colspan="5" class="!p-0">
                    <div
                      class="flex items-center gap-2 px-4 py-1.5 bg-destructive/10 border-y border-destructive/30"
                    >
                      <div class="flex-1 h-px bg-destructive/40" />
                      <span class="text-xs font-medium text-destructive whitespace-nowrap"
                        >Engine stops here (target reached)</span
                      >
                      <div class="flex-1 h-px bg-destructive/40" />
                    </div>
                  </UiTableCell>
                </UiTableRow>
                <UiTableRow
                  class="cursor-pointer"
                  :class="
                    deletionLineIndex !== null && groupIdx >= deletionLineIndex ? 'opacity-40' : ''
                  "
                  @click="
                    selectPreviewItem(group.entry);
                    group.seasons.length > 0 && togglePreviewGroup(group.key);
                  "
                >
                  <UiTableCell class="w-12 text-center">
                    <span class="text-xs font-mono tabular-nums text-muted-foreground">{{
                      groupIdx + 1
                    }}</span>
                  </UiTableCell>
                  <UiTableCell>
                    <span
                      class="text-xs font-mono tabular-nums font-semibold"
                      :class="group.entry.isProtected ? 'text-emerald-500' : 'text-primary'"
                    >
                      {{ group.entry.isProtected ? 'Protected' : group.entry.score.toFixed(2) }}
                    </span>
                  </UiTableCell>
                  <UiTableCell class="font-medium">
                    <div class="flex items-center gap-2">
                      <span class="truncate">{{ group.entry.item.title }}</span>
                      <button
                        v-if="group.seasons.length > 0"
                        class="text-muted-foreground hover:text-foreground transition-colors shrink-0 inline-flex items-center gap-0.5"
                        @click.stop="togglePreviewGroup(group.key)"
                      >
                        <ChevronRightIcon
                          class="w-3.5 h-3.5 transition-transform duration-200"
                          :class="{ 'rotate-90': expandedPreviewGroups.has(group.key) }"
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
                      {{ group.entry.item.type }}
                    </UiBadge>
                  </UiTableCell>
                  <UiTableCell class="text-right font-mono text-xs tabular-nums">
                    {{ formatBytes(group.entry.item.sizeBytes) }}
                  </UiTableCell>
                </UiTableRow>
                <template v-if="expandedPreviewGroups.has(group.key)">
                  <UiTableRow
                    v-for="(season, sIdx) in group.seasons"
                    :key="`${group.key}-s${sIdx}`"
                    class="bg-muted/30 cursor-pointer"
                    :class="
                      deletionLineIndex !== null && groupIdx >= deletionLineIndex
                        ? 'opacity-40'
                        : ''
                    "
                    @click.stop="selectPreviewItem(season)"
                  >
                    <UiTableCell class="w-12" />
                    <UiTableCell>
                      <span
                        class="text-xs font-mono tabular-nums font-semibold"
                        :class="season.isProtected ? 'text-emerald-500' : 'text-primary'"
                      >
                        {{ season.isProtected ? 'Protected' : season.score.toFixed(2) }}
                      </span>
                    </UiTableCell>
                    <UiTableCell class="text-muted-foreground pl-8">
                      <span class="inline-flex items-center gap-1.5">
                        <UiSeparator orientation="horizontal" class="w-3" />
                        {{ extractPreviewSeasonLabel(season.item.title) }}
                      </span>
                    </UiTableCell>
                    <UiTableCell>
                      <UiBadge variant="secondary" class="capitalize">
                        {{ season.item.type }}
                      </UiBadge>
                    </UiTableCell>
                    <UiTableCell
                      class="text-right font-mono text-xs tabular-nums text-muted-foreground"
                    >
                      {{ formatBytes(season.item.sizeBytes) }}
                    </UiTableCell>
                  </UiTableRow>
                </template>
              </template>
            </UiTableBody>
          </UiTable>
          <!-- Progressive rendering indicator -->
          <div
            v-if="renderedGroups.length < filteredGroupedPreview.length"
            class="flex items-center justify-center py-3 text-xs text-muted-foreground gap-2"
          >
            <component :is="LoaderCircleIcon" class="w-3.5 h-3.5 animate-spin" />
            Showing {{ renderedGroups.length }} of {{ filteredGroupedPreview.length }} — scroll for
            more
          </div>
        </div>
      </div>
    </UiCardContent>
  </UiCard>

  <ScoreDetailModal
    v-if="selectedPreviewItem"
    :visible="!!selectedPreviewItem"
    :media-name="selectedPreviewItem.mediaName"
    :media-type="selectedPreviewItem.mediaType"
    :score="selectedPreviewItem._score ?? 0"
    :score-details="selectedPreviewItem.scoreDetails || ''"
    :size-bytes="selectedPreviewItem.sizeBytes"
    :action="selectedPreviewItem.action || 'Preview'"
    :created-at="selectedPreviewItem.createdAt"
    @close="selectedPreviewItem = null"
  />
</template>

<script setup lang="ts">
import { useInfiniteScroll } from '@vueuse/core';
import {
  RefreshCwIcon,
  LoaderCircleIcon,
  CheckIcon,
  ChevronRightIcon,
  SearchIcon,
  ShieldCheckIcon,
  ArrowUpIcon,
  ArrowDownIcon,
  ArrowUpDownIcon,
} from 'lucide-vue-next';
import { formatBytes } from '~/utils/format';
import { groupEvaluatedItems } from '~/utils/groupPreview';
import type { PreviewGroup } from '~/utils/groupPreview';
import type { EvaluatedItem, SelectedDetailItem } from '~/types/api';

const props = defineProps<{
  preview: EvaluatedItem[];
  loading: boolean;
  fetchedAt: string;
  diskContext: {
    totalBytes: number;
    usedBytes: number;
    targetPct: number;
    thresholdPct: number;
    bytesToFree: number;
  } | null;
}>();

defineEmits<{
  refresh: [];
}>();

// Preview filters & sorting
const previewSearch = ref('');
const previewTypeFilter = ref<string | null>(null);
const previewStatusFilter = ref<'all' | 'protected' | 'unprotected'>('all');
type PreviewSortColumn = 'rank' | 'score' | 'title' | 'type' | 'size';
const previewSortBy = ref<PreviewSortColumn>('rank');
const previewSortDir = ref<'asc' | 'desc'>('asc');

function togglePreviewSort(column: PreviewSortColumn) {
  if (previewSortBy.value === column) {
    previewSortDir.value = previewSortDir.value === 'asc' ? 'desc' : 'asc';
  } else {
    previewSortBy.value = column;
    previewSortDir.value = column === 'score' || column === 'size' ? 'desc' : 'asc';
  }
}

const previewMediaTypes = ['movie', 'show', 'season', 'artist', 'book'] as const;

const tableColumns: { key: PreviewSortColumn; label: string; class?: string }[] = [
  { key: 'rank', label: '#', class: 'w-12' },
  { key: 'score', label: 'Score' },
  { key: 'title', label: 'Title' },
  { key: 'type', label: 'Type' },
  { key: 'size', label: 'Size', class: 'text-right' },
];

const selectedPreviewItem = ref<SelectedDetailItem | null>(null);

function selectPreviewItem(entry: EvaluatedItem) {
  let scoreDetails = '';
  if (entry.factors && Array.isArray(entry.factors)) {
    scoreDetails = JSON.stringify(entry.factors);
  } else if (typeof entry.scoreDetails === 'string') {
    scoreDetails = entry.scoreDetails;
  }
  selectedPreviewItem.value = {
    mediaName: entry.item?.title || 'Unknown',
    mediaType: entry.item?.type || 'unknown',
    _score: entry.score ?? 0,
    scoreDetails,
    sizeBytes: entry.item?.sizeBytes || 0,
    action: entry.isProtected ? 'Protected' : 'Preview',
    createdAt: props.fetchedAt || new Date().toISOString(),
  };
}

const groupedPreview = computed<PreviewGroup[]>(() => groupEvaluatedItems(props.preview));

const filteredGroupedPreview = computed<PreviewGroup[]>(() => {
  let groups = groupedPreview.value;
  const search = previewSearch.value.trim().toLowerCase();
  const typeFilter = previewTypeFilter.value;
  const statusFilter = previewStatusFilter.value;

  // Apply filters
  if (search || typeFilter || statusFilter !== 'all') {
    groups = groups.reduce<PreviewGroup[]>((result, group) => {
      const entry = group.entry;
      const entryType = entry.item?.type;
      const entryTitle = (entry.item?.title || '').toLowerCase();
      const entryProtected = !!entry.isProtected;

      // For show groups, also check if any seasons match
      if (group.seasons.length > 0) {
        const filteredSeasons = group.seasons.filter((s) => {
          const sTitle = (s.item?.title || '').toLowerCase();
          const sType = s.item?.type;
          const sProtected = !!s.isProtected;
          const matchSearch = !search || sTitle.includes(search) || entryTitle.includes(search);
          const matchType = !typeFilter || sType === typeFilter || entryType === typeFilter;
          const matchStatus =
            statusFilter === 'all' || (statusFilter === 'protected' ? sProtected : !sProtected);
          return matchSearch && matchType && matchStatus;
        });

        // Also check if the parent entry matches
        const parentMatchSearch = !search || entryTitle.includes(search);
        const parentMatchType = !typeFilter || entryType === typeFilter;
        const parentMatchStatus =
          statusFilter === 'all' ||
          (statusFilter === 'protected' ? entryProtected : !entryProtected);

        if (filteredSeasons.length > 0) {
          result.push({ ...group, seasons: filteredSeasons });
        } else if (parentMatchSearch && parentMatchType && parentMatchStatus) {
          result.push({ ...group, seasons: [] });
        }
      } else {
        // Non-grouped entries (movies, artists, books, etc.)
        const matchSearch = !search || entryTitle.includes(search);
        const matchType = !typeFilter || entryType === typeFilter;
        const matchStatus =
          statusFilter === 'all' ||
          (statusFilter === 'protected' ? entryProtected : !entryProtected);
        if (matchSearch && matchType && matchStatus) {
          result.push(group);
        }
      }
      return result;
    }, []);
  }

  // Apply sorting
  const sortBy = previewSortBy.value;
  const sortDir = previewSortDir.value;
  if (sortBy === 'rank' && sortDir === 'asc') return groups; // natural order

  const sorted = [...groups];
  const dir = sortDir === 'asc' ? 1 : -1;

  sorted.sort((a, b) => {
    switch (sortBy) {
      case 'rank':
        return dir * (groupedPreview.value.indexOf(a) - groupedPreview.value.indexOf(b));
      case 'score': {
        const scoreA = a.entry.isProtected ? Infinity : (a.entry.score ?? 0);
        const scoreB = b.entry.isProtected ? Infinity : (b.entry.score ?? 0);
        return dir * (scoreA - scoreB);
      }
      case 'title': {
        const titleA = (a.entry.item?.title || '').toLowerCase();
        const titleB = (b.entry.item?.title || '').toLowerCase();
        return dir * titleA.localeCompare(titleB);
      }
      case 'type': {
        const typeA = (a.entry.item?.type || '').toLowerCase();
        const typeB = (b.entry.item?.type || '').toLowerCase();
        return dir * typeA.localeCompare(typeB);
      }
      case 'size': {
        const sizeA = a.entry.item?.sizeBytes ?? 0;
        const sizeB = b.entry.item?.sizeBytes ?? 0;
        return dir * (sizeA - sizeB);
      }
      default:
        return 0;
    }
  });

  return sorted;
});
const deletionLineIndex = computed<number | null>(() => {
  const ctx = props.diskContext;
  if (!ctx || ctx.bytesToFree <= 0) return null;

  const groups = filteredGroupedPreview.value;
  let cumulative = 0;
  for (let i = 0; i < groups.length; i++) {
    const group = groups[i];
    if (!group) continue;
    if (group.entry.isProtected) continue;
    cumulative += group.entry.item?.sizeBytes ?? 0;
    if (group.seasons.length > 0) {
      for (const season of group.seasons) {
        if (!season.isProtected) {
          cumulative += season.item?.sizeBytes ?? 0;
        }
      }
    }
    if (cumulative >= ctx.bytesToFree) {
      return i + 1;
    }
  }
  return null;
});

// Progressive rendering
const tableScrollRef = ref<HTMLElement | null>(null);
const visibleCount = ref(100);
const renderedGroups = computed(() => filteredGroupedPreview.value.slice(0, visibleCount.value));

useInfiniteScroll(
  tableScrollRef,
  () => {
    if (visibleCount.value < filteredGroupedPreview.value.length) {
      visibleCount.value = Math.min(visibleCount.value + 100, filteredGroupedPreview.value.length);
    }
  },
  { distance: 200, canLoadMore: () => visibleCount.value < filteredGroupedPreview.value.length },
);

watch(
  [
    previewSearch,
    previewTypeFilter,
    previewStatusFilter,
    previewSortBy,
    previewSortDir,
    () => props.preview,
  ],
  () => {
    visibleCount.value = 100;
  },
);

const expandedPreviewGroups = ref(new Set<string>());
function togglePreviewGroup(key: string) {
  const next = new Set(expandedPreviewGroups.value);
  if (next.has(key)) {
    next.delete(key);
  } else {
    next.add(key);
  }
  expandedPreviewGroups.value = next;
}
function extractPreviewSeasonLabel(title: string): string {
  const parts = title.split(' - Season ');
  return parts.length > 1 ? `Season ${parts[parts.length - 1]}` : title;
}
</script>
