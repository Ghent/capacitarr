<template>
  <div>
    <!-- Integration error banner -->
    <IntegrationErrorBanner :integrations="integrations" />

    <!-- Tabs -->
    <UiTabs v-model="activeTab" class="w-full">
      <UiTabsList class="mb-6">
        <UiTabsTrigger value="browse">
          <LibraryIcon class="w-4 h-4 mr-1.5" />
          {{ $t('library.tabBrowse') }}
        </UiTabsTrigger>
        <UiTabsTrigger value="history">
          <ClockIcon class="w-4 h-4 mr-1.5" />
          {{ $t('library.tabHistory') }}
        </UiTabsTrigger>
      </UiTabsList>

      <!-- Tab 1: Browse -->
      <UiTabsContent value="browse">
        <!-- Smart Filter Presets -->
        <div class="flex flex-wrap items-center gap-2 mb-4">
          <span class="text-xs text-muted-foreground font-medium mr-1">{{
            $t('library.quickFilters')
          }}</span>
          <UiButton
            v-for="preset in filterPresets"
            :key="preset.key"
            :variant="activeFilter === preset.key ? 'default' : 'outline'"
            size="sm"
            class="rounded-full h-7 px-3 text-xs gap-1.5"
            @click="toggleFilter(preset.key)"
          >
            <component :is="preset.icon" class="w-3.5 h-3.5" />
            {{ preset.label }}
          </UiButton>

          <!-- Active filter indicator -->
          <div
            v-if="activeFilter"
            class="flex items-center gap-1 text-xs text-muted-foreground ml-2"
          >
            <span>{{ $t('library.filterActive', { name: activeFilterLabel }) }}</span>
            <UiButton variant="ghost" size="icon" class="h-5 w-5" @click="clearFilter">
              <XIcon class="w-3 h-3" />
            </UiButton>
          </div>
        </div>

        <!-- Stale data indicator -->
        <div
          v-if="stale"
          data-slot="stale-indicator"
          class="bg-muted text-muted-foreground mb-4 flex items-center gap-2 rounded-md px-4 py-2 text-sm"
        >
          <svg
            class="size-4 animate-spin"
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
          >
            <circle
              class="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              stroke-width="4"
            />
            <path
              class="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            />
          </svg>
          {{ $t('preview.stale') }}
        </div>

        <!-- Library Table -->
        <LibraryTable
          ref="libraryTableRef"
          :items="items"
          :integrations="enabledIntegrations"
          :loading="loading"
          @refresh="refresh(true)"
          @force-delete="handleForceDelete"
        />
      </UiTabsContent>

      <!-- Tab 2: History (merged Audit Log) -->
      <UiTabsContent value="history">
        <AuditLogPanel :show-header="false" />
      </UiTabsContent>
    </UiTabs>
  </div>
</template>

<script setup lang="ts">
import {
  LibraryIcon,
  ClockIcon,
  SkullIcon,
  TimerIcon,
  ExpandIcon,
  StarIcon,
  ShieldIcon,
  XIcon,
} from 'lucide-vue-next';
import type { IntegrationConfig, EvaluatedItem } from '~/types/api';

const api = useApi();
const { addToast } = useToast();
const { t } = useI18n();
const { items, loading, stale, refresh } = usePreview();
const route = useRoute();
const router = useRouter();

// ---------------------------------------------------------------------------
// Tabs — sync with query param
// ---------------------------------------------------------------------------
const VALID_TABS = ['browse', 'history'] as const;
type LibraryTab = (typeof VALID_TABS)[number];

const queryTab = route.query.tab as string | undefined;
const activeTab = ref<LibraryTab>(
  VALID_TABS.includes(queryTab as LibraryTab) ? (queryTab as LibraryTab) : 'browse',
);

watch(activeTab, (tab) => {
  router.replace({ query: { ...route.query, tab } });
});

// ---------------------------------------------------------------------------
// Smart Filter Presets
// ---------------------------------------------------------------------------
const filterPresets = [
  { key: 'dead', label: t('library.filterDead'), icon: SkullIcon },
  { key: 'stale', label: t('library.filterStale'), icon: TimerIcon },
  { key: 'bloated', label: t('library.filterBloated'), icon: ExpandIcon },
  { key: 'requested', label: t('library.filterRequested'), icon: StarIcon },
  { key: 'protected', label: t('library.filterProtected'), icon: ShieldIcon },
];

const activeFilter = ref<string | null>((route.query.filter as string) || null);
const activeFilterLabel = computed(() => {
  const preset = filterPresets.find((p) => p.key === activeFilter.value);
  return preset?.label ?? '';
});

function toggleFilter(key: string) {
  if (activeFilter.value === key) {
    clearFilter();
  } else {
    activeFilter.value = key;
    router.replace({ query: { ...route.query, filter: key, tab: 'browse' } });
  }
}

function clearFilter() {
  activeFilter.value = null;
  const query = { ...route.query };
  delete query.filter;
  router.replace({ query });
}

// Re-read filter from URL on route changes (e.g. Insights → Library links)
watch(
  () => route.query.filter,
  (newFilter) => {
    activeFilter.value = (newFilter as string) || null;
  },
);

// ---------------------------------------------------------------------------
// Integrations
// ---------------------------------------------------------------------------
const integrations = ref<IntegrationConfig[]>([]);

const enabledIntegrations = computed(() => integrations.value.filter((i) => i.enabled));

async function fetchIntegrations() {
  try {
    integrations.value = (await api('/api/v1/integrations')) as IntegrationConfig[];
  } catch (err) {
    console.warn('[Library] fetchIntegrations failed:', err);
  }
}

// ---------------------------------------------------------------------------
// Force Delete
// ---------------------------------------------------------------------------
const libraryTableRef = ref<InstanceType<
  typeof import('~/components/LibraryTable.vue').default
> | null>(null);

async function handleForceDelete(selectedItems: EvaluatedItem[]) {
  try {
    const body = selectedItems.map((e) => ({
      mediaName: e.item.title,
      mediaType: e.item.type,
      integrationId: e.item.integrationId,
      externalId: e.item.externalId,
      sizeBytes: e.item.sizeBytes,
      reason: e.reason || `Score: ${e.score.toFixed(2)}`,
      scoreDetails: JSON.stringify(e.factors),
      posterUrl: e.item.posterUrl ?? '',
    }));

    const result = (await api('/api/v1/force-delete', {
      method: 'POST',
      body,
    })) as { queued: number; total: number };

    addToast(t('library.forceDeleteSuccess', { count: result.queued }), 'success');
    libraryTableRef.value?.onDeleteComplete();

    // Refresh to reflect changes
    await refresh(true);
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : String(err);
    addToast(`${t('library.forceDeleteError')}: ${message}`, 'error');
    libraryTableRef.value?.onDeleteComplete();
  }
}

// ---------------------------------------------------------------------------
// Init
// ---------------------------------------------------------------------------
onMounted(async () => {
  await Promise.all([fetchIntegrations(), refresh()]);
});
</script>
