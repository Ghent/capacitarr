<template>
  <div>
    <!-- Header -->
    <div data-slot="page-header" class="mb-8">
      <h1 class="text-3xl font-bold tracking-tight">
        {{ $t('library.title') }}
      </h1>
      <p class="text-muted-foreground mt-1.5">
        {{ $t('library.subtitle') }}
      </p>
    </div>

    <!-- Integration error banner -->
    <IntegrationErrorBanner :integrations="integrations" />

    <!-- Library Table -->
    <LibraryTable
      ref="libraryTableRef"
      :items="flatItems"
      :integrations="enabledIntegrations"
      :loading="loading"
      @refresh="fetchPreview"
      @force-delete="handleForceDelete"
    />
  </div>
</template>

<script setup lang="ts">
import type { IntegrationConfig, EvaluatedItem, PreviewResponse } from '~/types/api';

const api = useApi();
const { addToast } = useToast();
const { t } = useI18n();

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
// Preview Data (flat items — no grouping)
// ---------------------------------------------------------------------------
const preview = ref<EvaluatedItem[]>([]);
const loading = ref(false);

/** Flat list: every item is its own row (no show→season grouping) */
const flatItems = computed(() => preview.value);

async function fetchPreview() {
  loading.value = true;
  try {
    const data = (await api('/api/v1/preview')) as PreviewResponse;
    preview.value = data?.items ?? [];
  } catch (err) {
    console.warn('[Library] fetchPreview failed:', err);
    preview.value = [];
  } finally {
    loading.value = false;
  }
}

// ---------------------------------------------------------------------------
// Force Delete
// ---------------------------------------------------------------------------
const libraryTableRef = ref<InstanceType<
  typeof import('~/components/LibraryTable.vue').default
> | null>(null);

async function handleForceDelete(items: EvaluatedItem[]) {
  try {
    const body = items.map((e) => ({
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
    await fetchPreview();
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
  await Promise.all([fetchIntegrations(), fetchPreview()]);
});
</script>
