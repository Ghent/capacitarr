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

    <FetchErrorBanner v-if="loadError" @retry="loadRulesPage" />

    <!-- Integration error banner (below page title) -->
    <IntegrationErrorBanner :integrations="allIntegrations" />

    <!-- Disk Thresholds -->
    <RulesRuleDiskThresholds :disk-groups="diskGroups" @update:disk-group="onDiskGroupUpdated" />

    <!-- Preference Weights (dynamically populated from API) -->
    <RulesRuleWeightEditor
      :factors="factorWeights"
      @save="saveFactorWeights"
      @update:weight="onWeightUpdate"
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
import type { DiskGroup, IntegrationConfig, CustomRule, ScoringFactorWeight } from '~/types/api';
import { toast } from 'vue-sonner';

const api = useApi();
const { t } = useI18n();
const { loadError, markSuccess, markFailure } = useFetchStatus();
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
  diskGroups.value = (await api('/api/v1/disk-groups')) as DiskGroup[];
}

function onDiskGroupUpdated(updated: DiskGroup) {
  const idx = diskGroups.value.findIndex((g) => g.id === updated.id);
  if (idx !== -1) {
    diskGroups.value[idx] = updated;
  }
}

// ---------------------------------------------------------------------------
// Scoring Factor Weights (dynamic — fetched from dedicated API)
// ---------------------------------------------------------------------------
const factorWeights = ref<ScoringFactorWeight[]>([]);

async function fetchFactorWeights() {
  factorWeights.value = (await api('/api/v1/scoring-factor-weights')) as ScoringFactorWeight[];
}

async function saveFactorWeights() {
  try {
    // Build weight map from current state
    const weightMap: Record<string, number> = {};
    for (const f of factorWeights.value) {
      weightMap[f.key] = f.weight;
    }
    const updated = (await api('/api/v1/scoring-factor-weights', {
      method: 'PUT',
      body: weightMap,
    })) as ScoringFactorWeight[];
    factorWeights.value = updated;
    toast.success(t('rules.weightsSaved'));
  } catch {
    toast.error(t('rules.weightsSaveFailed'));
  }
}

function onWeightUpdate(key: string, value: number) {
  const factor = factorWeights.value.find((f) => f.key === key);
  if (factor) {
    factor.weight = value;
  }
}

function onApplyPreset(values: Record<string, number>) {
  for (const f of factorWeights.value) {
    const v = values[f.key];
    if (v !== undefined) {
      f.weight = v;
    }
  }
}

// ---------------------------------------------------------------------------
// Custom Rules
// ---------------------------------------------------------------------------
const rules = ref<CustomRule[]>([]);
const allIntegrations = ref<IntegrationConfig[]>([]);

async function fetchIntegrations() {
  allIntegrations.value = (await api('/api/v1/integrations')) as IntegrationConfig[];
}

async function fetchRules() {
  rules.value = (await api('/api/v1/custom-rules')) as CustomRule[];
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
    toast.success(t('rules.ruleAdded'));
    await fetchRules();
  } catch {
    toast.error(t('rules.ruleAddFailed'));
  }
}

async function deleteRule(id: number) {
  try {
    await api(`/api/v1/custom-rules/${id}`, { method: 'DELETE' });
    toast.success(t('rules.ruleRemoved'));
    await fetchRules();
  } catch {
    toast.error(t('rules.ruleRemoveFailed'));
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
    toast.success(t('rules.ruleUpdated'));
    await fetchRules();
  } catch {
    toast.error(t('rules.ruleUpdateFailed'));
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
    toast.success(enabled ? t('rules.ruleEnabled') : t('rules.ruleDisabled'));
  } catch {
    // Revert on failure
    rule.enabled = !enabled;
    toast.error(t('rules.ruleUpdateFailed'));
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
    toast.success(t('rules.rulesReordered'));
  } catch {
    // Revert — re-fetch from server
    await fetchRules();
    toast.error(t('rules.rulesReorderFailed'));
  }
}

async function loadRulesPage() {
  try {
    await Promise.all([
      fetchFactorWeights(),
      fetchRules(),
      previewRefresh(),
      fetchDiskGroups(),
      fetchIntegrations(),
    ]);
    markSuccess();
  } catch (err) {
    console.warn('[Rules] loadRulesPage failed:', err);
    markFailure();
  }
}

onMounted(() => {
  loadRulesPage();
});
</script>
