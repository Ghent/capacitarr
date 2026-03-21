<template>
  <div>
    <!-- Header -->
    <div data-slot="page-header" class="mb-8">
      <h1 class="text-3xl font-bold tracking-tight">
        {{ $t('rules.title') }}
      </h1>
      <p class="text-muted-foreground mt-1.5">
        {{ $t('rules.subtitle') }}
      </p>
    </div>

    <!-- Integration error banner (below page title) -->
    <IntegrationErrorBanner :integrations="allIntegrations" />

    <!-- Disk Thresholds -->
    <RulesRuleDiskThresholds :disk-groups="diskGroups" @update:disk-group="onDiskGroupUpdated" />

    <!-- Preference Weights -->
    <RulesRuleWeightEditor
      :preferences="weightPreferences"
      @save="savePreferences"
      @update:preference="onPreferenceUpdate"
      @apply-preset="onApplyPreset"
    />

    <!-- Custom Rules -->
    <RulesRuleCustomList
      :rules="rules"
      :integrations="allIntegrations"
      @add-rule="addRule"
      @edit-rule="editRule"
      @delete-rule="deleteRule"
      @toggle-enabled="toggleRuleEnabled"
      @reorder="reorderRules"
    />

    <!-- Live Preview -->
    <RulesRulePreviewTable
      :preview="previewItems"
      :loading="previewLoading"
      :fetched-at="previewFetchedAt"
      :disk-context="previewDiskContext"
      :rules="rules"
      @refresh="previewRefresh(true)"
    />
  </div>
</template>

<script setup lang="ts">
import type { DiskGroup, IntegrationConfig, CustomRule, PreferenceSet } from '~/types/api';
import type { WeightKeys } from '~/components/rules/RuleWeightEditor.vue';

const api = useApi();
const { addToast } = useToast();
const {
  items: previewItems,
  diskContext: previewDiskContext,
  loading: previewLoading,
  refresh: previewRefresh,
} = usePreview();
const previewFetchedAt = ref<string>('');
watch(previewItems, () => {
  previewFetchedAt.value = new Date().toISOString();
});

// ---------------------------------------------------------------------------
// Disk Groups
// ---------------------------------------------------------------------------
const diskGroups = ref<DiskGroup[]>([]);

async function fetchDiskGroups() {
  try {
    diskGroups.value = (await api('/api/v1/disk-groups')) as DiskGroup[];
  } catch (err) {
    console.warn('[Rules] fetchDiskGroups failed:', err);
  }
}

function onDiskGroupUpdated(updated: DiskGroup) {
  const idx = diskGroups.value.findIndex((g) => g.id === updated.id);
  if (idx !== -1) {
    diskGroups.value[idx] = updated;
  }
}

// ---------------------------------------------------------------------------
// Preferences
// ---------------------------------------------------------------------------
const prefs = reactive({
  watchHistoryWeight: 10,
  lastWatchedWeight: 8,
  fileSizeWeight: 6,
  ratingWeight: 5,
  timeInLibraryWeight: 4,
  seriesStatusWeight: 3,
  executionMode: 'dry-run',
  tiebreakerMethod: 'size_desc',
  logLevel: 'info',
  auditLogRetentionDays: 30,
});

/** Weight-only view of prefs for the weight editor component */
const weightPreferences = computed<WeightKeys>(() => ({
  watchHistoryWeight: prefs.watchHistoryWeight,
  lastWatchedWeight: prefs.lastWatchedWeight,
  fileSizeWeight: prefs.fileSizeWeight,
  ratingWeight: prefs.ratingWeight,
  timeInLibraryWeight: prefs.timeInLibraryWeight,
  seriesStatusWeight: prefs.seriesStatusWeight,
}));

async function fetchPreferences() {
  try {
    const data = (await api('/api/v1/preferences')) as PreferenceSet;
    if (data?.id) {
      Object.assign(prefs, data);
    }
  } catch (err) {
    console.warn('[Rules] fetchPreferences failed:', err);
  }
}

async function savePreferences() {
  try {
    await api('/api/v1/preferences', { method: 'PUT', body: { ...prefs, id: 1 } });
    addToast('Settings saved', 'success');
  } catch {
    addToast('Failed to save preferences', 'error');
  }
}

function onPreferenceUpdate(key: string, value: number) {
  Object.assign(prefs, { [key]: value });
}

function onApplyPreset(values: Record<string, number>) {
  Object.assign(prefs, values);
}

// ---------------------------------------------------------------------------
// Custom Rules
// ---------------------------------------------------------------------------
const rules = ref<CustomRule[]>([]);
const allIntegrations = ref<IntegrationConfig[]>([]);

async function fetchIntegrations() {
  try {
    allIntegrations.value = (await api('/api/v1/integrations')) as IntegrationConfig[];
  } catch (err) {
    console.warn('[Rules] fetchIntegrations failed:', err);
  }
}

async function fetchRules() {
  try {
    rules.value = (await api('/api/v1/custom-rules')) as CustomRule[];
  } catch (err) {
    console.warn('[Rules] fetchRules failed:', err);
  }
}

async function addRule(rule: {
  integrationId: number;
  field: string;
  operator: string;
  value: string;
  effect: string;
}) {
  try {
    await api('/api/v1/custom-rules', { method: 'POST', body: rule });
    addToast('Rule added', 'success');
    await fetchRules();
  } catch {
    addToast('Failed to add rule', 'error');
  }
}

async function deleteRule(id: number) {
  try {
    await api(`/api/v1/custom-rules/${id}`, { method: 'DELETE' });
    addToast('Rule removed', 'success');
    await fetchRules();
  } catch {
    addToast('Failed to delete rule', 'error');
  }
}

async function editRule(
  id: number,
  rule: { integrationId: number; field: string; operator: string; value: string; effect: string },
) {
  try {
    await api(`/api/v1/custom-rules/${id}`, {
      method: 'PUT',
      body: { ...rule, id },
    });
    addToast('Rule updated', 'success');
    await fetchRules();
  } catch {
    addToast('Failed to update rule', 'error');
  }
}

async function toggleRuleEnabled(rule: CustomRule, enabled: boolean) {
  // Optimistically update local state
  rule.enabled = enabled;
  try {
    await api(`/api/v1/custom-rules/${rule.id}`, {
      method: 'PUT',
      body: { ...rule, enabled },
    });
    addToast(enabled ? 'Rule enabled' : 'Rule disabled', 'success');
  } catch {
    // Revert on failure
    rule.enabled = !enabled;
    addToast('Failed to update rule', 'error');
  }
}

async function reorderRules(order: number[]) {
  // Optimistically reorder local array
  const reordered = order
    .map((id) => rules.value.find((r) => r.id === id))
    .filter(Boolean) as CustomRule[];
  rules.value = reordered;

  try {
    await api('/api/v1/custom-rules/reorder', {
      method: 'PUT',
      body: { order },
    });
    addToast('Rules reordered', 'success');
  } catch {
    // Revert — re-fetch from server
    await fetchRules();
    addToast('Failed to reorder rules', 'error');
  }
}

// ---------------------------------------------------------------------------
// Lifecycle — fetch all data on mount
// ---------------------------------------------------------------------------
onMounted(async () => {
  await Promise.all([
    fetchPreferences(),
    fetchRules(),
    previewRefresh(),
    fetchDiskGroups(),
    fetchIntegrations(),
  ]);
});
</script>
