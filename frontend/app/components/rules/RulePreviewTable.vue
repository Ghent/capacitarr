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
          <ViewModeToggle />
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
            <!-- Rule filter -->
            <template v-if="enabledRules.length > 0">
              <UiSeparator orientation="vertical" class="h-5 mx-1" />
              <UiPopover v-model:open="ruleFilterOpen">
                <UiPopoverTrigger as-child>
                  <UiButton
                    :variant="hasRuleFilter ? 'default' : 'outline'"
                    size="sm"
                    class="rounded-full h-7 px-3 text-xs"
                  >
                    <FilterIcon class="w-3 h-3 mr-1" />
                    {{ t('rules.filterByRule') }}
                    <span v-if="hasRuleFilter" class="ml-1 text-[10px] opacity-80"
                      >({{ selectedRuleIds.length }})</span
                    >
                  </UiButton>
                </UiPopoverTrigger>
                <UiPopoverContent class="w-72 p-0" side="bottom" align="start">
                  <div class="px-3 py-2 border-b flex items-center justify-between">
                    <p class="text-sm font-medium">{{ t('rules.filterByRule') }}</p>
                    <div class="flex items-center gap-1">
                      <UiButton
                        :variant="ruleFilterMode === 'any' ? 'default' : 'outline'"
                        size="sm"
                        class="h-5 px-2 text-[10px]"
                        @click="ruleFilterMode = 'any'"
                      >
                        {{ t('rules.filterModeAny') }}
                      </UiButton>
                      <UiButton
                        :variant="ruleFilterMode === 'all' ? 'default' : 'outline'"
                        size="sm"
                        class="h-5 px-2 text-[10px]"
                        @click="ruleFilterMode = 'all'"
                      >
                        {{ t('rules.filterModeAll') }}
                      </UiButton>
                    </div>
                  </div>
                  <div class="max-h-60 overflow-y-auto">
                    <div
                      v-for="rule in enabledRules"
                      :key="rule.id"
                      class="flex items-center gap-2 px-3 py-1.5 hover:bg-muted/50 transition-colors cursor-pointer"
                      @click="toggleRuleFilter(rule.id)"
                    >
                      <UiCheckbox
                        :model-value="selectedRuleIds.includes(rule.id)"
                        class="pointer-events-none"
                      />
                      <span class="text-xs truncate flex-1">
                        {{ formatRuleLabel(rule) }}
                      </span>
                      <UiBadge variant="secondary" class="text-[10px] shrink-0 capitalize">
                        {{ rule.effect.replace('_', ' ') }}
                      </UiBadge>
                    </div>
                  </div>
                  <div v-if="hasRuleFilter" class="px-3 py-2 border-t">
                    <UiButton
                      variant="ghost"
                      size="sm"
                      class="w-full h-7 text-xs"
                      @click="clearRuleFilter"
                    >
                      <XIcon class="w-3 h-3 mr-1" />
                      {{ t('rules.clearRuleFilter') }}
                    </UiButton>
                  </div>
                </UiPopoverContent>
              </UiPopover>
            </template>
          </div>
        </div>

        <!-- Results count -->
        <div class="text-xs text-muted-foreground mb-2">
          <template
            v-if="
              previewSearch || previewTypeFilter || previewStatusFilter !== 'all' || hasRuleFilter
            "
          >
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

        <!-- Grid View -->
        <div
          v-else-if="viewMode === 'grid'"
          ref="gridScrollRef"
          class="max-h-[600px] overflow-y-auto"
        >
          <div
            class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-3"
          >
            <template v-for="(group, groupIdx) in renderedGroups" :key="group.key">
              <!-- Deletion line: full-width divider -->
              <div
                v-if="deletionLineIndex !== null && deletionLineIndex === groupIdx"
                class="col-span-full flex items-center gap-2 py-1"
              >
                <div class="flex-1 h-px bg-destructive/40" />
                <span class="text-xs font-medium text-destructive whitespace-nowrap"
                  >Engine stops here (target reached)</span
                >
                <div class="flex-1 h-px bg-destructive/40" />
              </div>
              <!-- Show groups: popover with individual seasons -->
              <UiPopover v-if="group.seasons.length > 0">
                <UiPopoverTrigger as-child>
                  <MediaPosterCard
                    :title="group.entry.item.title"
                    :poster-url="group.entry.item.posterUrl"
                    :year="group.entry.item.year"
                    :media-type="group.entry.item.type"
                    :score="group.entry.isProtected ? undefined : group.entry.score"
                    :size-bytes="group.entry.item.sizeBytes"
                    :is-protected="group.entry.isProtected"
                    :is-flagged="deletionLineIndex !== null && groupIdx >= deletionLineIndex"
                    :season-count="group.seasons.length"
                  />
                </UiPopoverTrigger>
                <UiPopoverContent class="w-72 p-0" side="bottom" align="start">
                  <div class="px-3 py-2 border-b">
                    <p class="text-sm font-medium truncate">
                      {{ group.entry.item.title }}
                    </p>
                    <p class="text-xs text-muted-foreground">
                      {{ group.seasons.length }} season{{ group.seasons.length !== 1 ? 's' : '' }}
                    </p>
                  </div>
                  <div
                    class="max-h-60 overflow-y-auto"
                    :class="{
                      'opacity-50': deletionLineIndex !== null && groupIdx >= deletionLineIndex,
                    }"
                  >
                    <div
                      v-for="season in group.seasons"
                      :key="season.item.title"
                      class="flex items-center gap-2 px-3 py-1.5 hover:bg-muted/50 transition-colors cursor-pointer"
                      @click="selectPreviewItem(season)"
                    >
                      <span
                        class="text-xs font-mono tabular-nums font-semibold w-10 text-right shrink-0"
                        :class="season.isProtected ? 'text-emerald-500' : 'text-primary'"
                      >
                        {{ season.isProtected ? '✓' : season.score.toFixed(2) }}
                      </span>
                      <span class="text-xs truncate flex-1">
                        {{ extractPreviewSeasonLabel(season.item.title) }}
                      </span>
                      <span class="text-xs text-muted-foreground tabular-nums shrink-0">
                        {{ formatBytes(season.item.sizeBytes) }}
                      </span>
                    </div>
                  </div>
                </UiPopoverContent>
              </UiPopover>
              <!-- Non-show items: direct click to detail -->
              <MediaPosterCard
                v-else
                :title="group.entry.item.title"
                :poster-url="group.entry.item.posterUrl"
                :year="group.entry.item.year"
                :media-type="group.entry.item.type"
                :score="group.entry.isProtected ? undefined : group.entry.score"
                :size-bytes="group.entry.item.sizeBytes"
                :is-protected="group.entry.isProtected"
                :is-flagged="deletionLineIndex !== null && groupIdx >= deletionLineIndex"
                @click="selectPreviewItem(group.entry)"
              />
            </template>
          </div>
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

        <!-- List/Table View -->
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
              <!-- Top spacer for virtual scroll -->
              <tr :style="{ height: `${previewVirtualRows[0]?.start ?? 0}px` }" />
              <template v-for="vRow in previewVirtualRows" :key="vRow.index">
                <!-- Deletion line row -->
                <UiTableRow v-if="vRow.entry.type === 'deletion-line'" class="pointer-events-none">
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
                <!-- Group header row -->
                <UiTableRow
                  v-else-if="vRow.entry.type === 'group'"
                  class="cursor-pointer"
                  :class="
                    deletionLineIndex !== null && vRow.entry.groupIdx >= deletionLineIndex
                      ? 'opacity-40'
                      : ''
                  "
                  @click="
                    selectPreviewItem(vRow.entry.group.entry);
                    vRow.entry.group.seasons.length > 0 && togglePreviewGroup(vRow.entry.group.key);
                  "
                >
                  <UiTableCell class="w-12 text-center">
                    <span class="text-xs font-mono tabular-nums text-muted-foreground">{{
                      vRow.entry.groupIdx + 1
                    }}</span>
                  </UiTableCell>
                  <UiTableCell>
                    <span
                      class="text-xs font-mono tabular-nums font-semibold"
                      :class="
                        vRow.entry.group.entry.isProtected ? 'text-emerald-500' : 'text-primary'
                      "
                    >
                      {{
                        vRow.entry.group.entry.isProtected
                          ? 'Protected'
                          : vRow.entry.group.entry.score.toFixed(2)
                      }}
                    </span>
                  </UiTableCell>
                  <UiTableCell class="font-medium">
                    <div class="flex items-center gap-2">
                      <span class="truncate">{{ vRow.entry.group.entry.item.title }}</span>
                      <button
                        v-if="vRow.entry.group.seasons.length > 0"
                        class="text-muted-foreground hover:text-foreground transition-colors shrink-0 inline-flex items-center gap-0.5"
                        @click.stop="togglePreviewGroup(vRow.entry.group.key)"
                      >
                        <ChevronRightIcon
                          class="w-3.5 h-3.5 transition-transform duration-200"
                          :class="{
                            'rotate-90': expandedPreviewGroups.has(vRow.entry.group.key),
                          }"
                        />
                        <span class="text-xs text-muted-foreground font-normal whitespace-nowrap"
                          >({{ vRow.entry.group.seasons.length }} season{{
                            vRow.entry.group.seasons.length !== 1 ? 's' : ''
                          }})</span
                        >
                      </button>
                    </div>
                  </UiTableCell>
                  <UiTableCell>
                    <UiBadge variant="secondary" class="capitalize">
                      {{ vRow.entry.group.entry.item.type }}
                    </UiBadge>
                  </UiTableCell>
                  <UiTableCell class="text-right font-mono text-xs tabular-nums">
                    {{ formatBytes(vRow.entry.group.entry.item.sizeBytes) }}
                  </UiTableCell>
                </UiTableRow>
                <!-- Expanded season row -->
                <UiTableRow
                  v-else
                  class="bg-muted/30 cursor-pointer"
                  :class="
                    deletionLineIndex !== null && vRow.entry.groupIdx >= deletionLineIndex
                      ? 'opacity-40'
                      : ''
                  "
                  @click.stop="selectPreviewItem(vRow.entry.season)"
                >
                  <UiTableCell class="w-12" />
                  <UiTableCell>
                    <span
                      class="text-xs font-mono tabular-nums font-semibold"
                      :class="vRow.entry.season.isProtected ? 'text-emerald-500' : 'text-primary'"
                    >
                      {{
                        vRow.entry.season.isProtected
                          ? 'Protected'
                          : vRow.entry.season.score.toFixed(2)
                      }}
                    </span>
                  </UiTableCell>
                  <UiTableCell class="text-muted-foreground pl-8">
                    <span class="inline-flex items-center gap-1.5">
                      <UiSeparator orientation="horizontal" class="w-3" />
                      {{ extractPreviewSeasonLabel(vRow.entry.season.item.title) }}
                    </span>
                  </UiTableCell>
                  <UiTableCell>
                    <UiBadge variant="secondary" class="capitalize">
                      {{ vRow.entry.season.item.type }}
                    </UiBadge>
                  </UiTableCell>
                  <UiTableCell
                    class="text-right font-mono text-xs tabular-nums text-muted-foreground"
                  >
                    {{ formatBytes(vRow.entry.season.item.sizeBytes) }}
                  </UiTableCell>
                </UiTableRow>
              </template>
              <!-- Bottom spacer for virtual scroll -->
              <tr
                :style="{
                  height: `${previewTableVirtualizer.getTotalSize() - (previewVirtualRows.at(-1)?.end ?? 0)}px`,
                }"
              />
            </UiTableBody>
          </UiTable>
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
import { useVirtualizer } from '@tanstack/vue-virtual';
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
  FilterIcon,
  XIcon,
} from 'lucide-vue-next';
import { formatBytes } from '~/utils/format';
import { groupEvaluatedItems } from '~/utils/groupPreview';
import type { PreviewGroup } from '~/utils/groupPreview';
import type { EvaluatedItem, SelectedDetailItem, CustomRule } from '~/types/api';

const { viewMode } = useDisplayPrefs();
const { t } = useI18n();

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
  rules?: CustomRule[];
}>();

defineEmits<{
  refresh: [];
}>();

// Preview filters & sorting
const previewSearch = ref('');
const previewTypeFilter = ref<string | null>(null);
const previewStatusFilter = ref<'all' | 'protected' | 'unprotected'>('all');

// Rule filter state
const selectedRuleIds = ref<number[]>([]);
const ruleFilterMode = ref<'any' | 'all'>('any');
const ruleFilterOpen = ref(false);

/** Enabled rules available for filtering */
const enabledRules = computed(() => (props.rules ?? []).filter((r) => r.enabled));

/** Whether any rule filters are active */
const hasRuleFilter = computed(() => selectedRuleIds.value.length > 0);

function toggleRuleFilter(ruleId: number) {
  const idx = selectedRuleIds.value.indexOf(ruleId);
  if (idx === -1) {
    selectedRuleIds.value = [...selectedRuleIds.value, ruleId];
  } else {
    selectedRuleIds.value = selectedRuleIds.value.filter((id) => id !== ruleId);
  }
}

function clearRuleFilter() {
  selectedRuleIds.value = [];
}

/** Format a rule for display in the filter dropdown */
function formatRuleLabel(rule: CustomRule): string {
  return `${rule.field} ${rule.operator} ${rule.value}`;
}

/** Check if an EvaluatedItem matches the active rule filters */
function itemMatchesRuleFilter(entry: EvaluatedItem): boolean {
  if (selectedRuleIds.value.length === 0) return true;

  const matchedRuleIds = (entry.factors ?? [])
    .filter((f) => f.type === 'rule' && f.ruleId != null)
    .map((f) => f.ruleId!);

  if (ruleFilterMode.value === 'any') {
    return selectedRuleIds.value.some((id) => matchedRuleIds.includes(id));
  }
  return selectedRuleIds.value.every((id) => matchedRuleIds.includes(id));
}
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

  // Apply rule filter
  if (hasRuleFilter.value) {
    groups = groups.filter((group) => {
      // Check if the main entry matches
      if (itemMatchesRuleFilter(group.entry)) return true;
      // For show groups, check if any season matches
      if (group.seasons.length > 0) {
        return group.seasons.some((s) => itemMatchesRuleFilter(s));
      }
      return false;
    });
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

// ─── Virtual Scrolling (Table View) ──────────────────────────────────────────
const tableScrollRef = ref<HTMLElement | null>(null);

/** Row types for the flattened virtual table */
type PreviewFlatRow =
  | { type: 'group'; group: PreviewGroup; groupIdx: number }
  | { type: 'season'; season: EvaluatedItem; groupIdx: number }
  | { type: 'deletion-line' };

/** Flatten groups + expanded seasons + deletion line into a single row list */
const previewFlatRows = computed<PreviewFlatRow[]>(() => {
  const rows: PreviewFlatRow[] = [];
  const groups = filteredGroupedPreview.value;
  const delIdx = deletionLineIndex.value;

  for (let i = 0; i < groups.length; i++) {
    const group = groups[i]!;
    if (delIdx !== null && delIdx === i) {
      rows.push({ type: 'deletion-line' });
    }
    rows.push({ type: 'group', group, groupIdx: i });
    if (expandedPreviewGroups.value.has(group.key)) {
      for (const season of group.seasons) {
        rows.push({ type: 'season', season, groupIdx: i });
      }
    }
  }
  return rows;
});

const previewTableVirtualizer = useVirtualizer(
  computed(() => ({
    count: previewFlatRows.value.length,
    getScrollElement: () => tableScrollRef.value,
    estimateSize: () => 44,
    overscan: 15,
  })),
);

const previewVirtualRows = computed(() =>
  previewTableVirtualizer.value.getVirtualItems().map((vRow) => ({
    ...vRow,
    entry: previewFlatRows.value[vRow.index]!,
  })),
);

// ─── Progressive Rendering (Grid View) ──────────────────────────────────────
const gridScrollRef = ref<HTMLElement | null>(null);
const gridVisibleCount = ref(100);
const renderedGroups = computed(() =>
  filteredGroupedPreview.value.slice(0, gridVisibleCount.value),
);

function gridLoadMore() {
  if (gridVisibleCount.value < filteredGroupedPreview.value.length) {
    gridVisibleCount.value = Math.min(
      gridVisibleCount.value + 100,
      filteredGroupedPreview.value.length,
    );
  }
}

useInfiniteScroll(gridScrollRef, gridLoadMore, {
  distance: 200,
  canLoadMore: () => gridVisibleCount.value < filteredGroupedPreview.value.length,
});

// Reset scroll/visible count when filters change
watch(
  [
    previewSearch,
    previewTypeFilter,
    previewStatusFilter,
    previewSortBy,
    previewSortDir,
    selectedRuleIds,
    ruleFilterMode,
    () => props.preview,
  ],
  () => {
    gridVisibleCount.value = 100;
    previewTableVirtualizer.value.scrollToIndex(0);
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
