<template>
  <UiCard
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{
      opacity: 1,
      y: 0,
      transition: { type: 'spring', stiffness: 260, damping: 24, delay: 100 },
    }"
    class="mb-6"
  >
    <UiCardHeader>
      <div class="flex items-center justify-between">
        <div>
          <UiCardTitle>{{ $t('rules.customRules') }}</UiCardTitle>
          <UiCardDescription class="mt-1">
            {{ $t('rules.customRulesDesc') }}
          </UiCardDescription>
          <p class="text-xs text-muted-foreground mt-1">
            {{ $t('rules.orderDisclaimer') }}
          </p>
        </div>
        <div class="flex items-center gap-2">
          <UiButton variant="outline" size="sm" @click="triggerFileInput">
            <component :is="UploadIcon" class="w-3.5 h-3.5" />
            {{ $t('rules.importRules') }}
          </UiButton>
          <UiButton
            variant="outline"
            size="sm"
            :disabled="rules.length === 0"
            @click="$emit('export-rules')"
          >
            <component :is="DownloadIcon" class="w-3.5 h-3.5" />
            {{ $t('rules.exportRules') }}
          </UiButton>
          <UiButton size="sm" @click="showAddRule = !showAddRule">
            <component :is="PlusIcon" class="w-3.5 h-3.5" />
            {{ $t('rules.addRule') }}
          </UiButton>
        </div>
      </div>
    </UiCardHeader>
    <UiCardContent>
      <!-- Add Rule Form — Cascading Rule Builder -->
      <RuleBuilder
        v-if="showAddRule"
        :integrations="integrations"
        class="mb-4"
        @save="onAddRule"
        @cancel="showAddRule = false"
      />

      <!-- Empty state -->
      <div
        v-if="rules.length === 0 && !showAddRule"
        class="text-center py-6 text-muted-foreground text-sm"
      >
        {{ $t('rules.noRules') }}
      </div>

      <!-- Grouped rules by integration — collapsible sections -->
      <div v-else class="space-y-3">
        <UiCollapsible
          v-for="group in groupedRules"
          :key="group.integrationId"
          :default-open="true"
        >
          <!-- Section header / trigger -->
          <UiCollapsibleTrigger
            class="flex w-full items-center justify-between rounded-lg px-3 py-2 text-sm font-medium text-foreground hover:bg-muted/80 transition-colors group"
          >
            <div class="flex items-center gap-2">
              <ChevronRightIcon
                class="w-4 h-4 text-muted-foreground transition-transform duration-200 group-data-[state=open]:rotate-90"
              />
              <span>{{ group.name }}</span>
              <UiBadge variant="secondary" class="text-xs tabular-nums">
                {{ $t('rules.ruleCount', { count: group.rules.length }, group.rules.length) }}
              </UiBadge>
            </div>
          </UiCollapsibleTrigger>

          <!-- Collapsible rule list -->
          <UiCollapsibleContent>
            <div class="space-y-2 mt-2">
              <div
                v-for="(rule, ruleIdx) in group.rules"
                :key="rule.id"
                draggable="true"
                class="flex items-center justify-between px-4 py-2.5 rounded-lg border bg-muted/50 transition-opacity duration-200"
                :class="[
                  (conflictsMap.get(rule.id)?.length ?? 0) > 0
                    ? 'border-amber-400/50'
                    : 'border-border',
                  rule.enabled === false ? 'opacity-50' : '',
                  dragOverKey === ruleKey(group.integrationId, ruleIdx)
                    ? 'border-primary border-dashed'
                    : '',
                  dragSourceKey === ruleKey(group.integrationId, ruleIdx) ? 'opacity-30' : '',
                ]"
                @dragstart="onDragStart($event, group.integrationId, ruleIdx)"
                @dragover.prevent="onDragOver($event, group.integrationId, ruleIdx)"
                @dragleave="onDragLeave"
                @drop.prevent="onDrop($event, group.integrationId, ruleIdx)"
                @dragend="onDragEnd"
              >
                <div class="flex items-center gap-2 text-sm flex-wrap">
                  <!-- Drag handle -->
                  <span
                    role="button"
                    aria-label="Drag to reorder"
                    class="inline-flex items-center shrink-0 cursor-grab active:cursor-grabbing text-muted-foreground/50 hover:text-muted-foreground transition-colors"
                  >
                    <GripVerticalIcon class="w-4 h-4" />
                  </span>
                  <!-- Rule number (per-group) -->
                  <span class="text-xs font-mono tabular-nums text-muted-foreground w-5 shrink-0"
                    >{{ ruleIdx + 1 }}.</span
                  >
                  <!-- Enable/Disable toggle -->
                  <UiSwitch
                    :model-value="rule.enabled !== false"
                    :aria-label="rule.enabled !== false ? 'Disable rule' : 'Enable rule'"
                    class="shrink-0"
                    @update:model-value="(v: boolean) => $emit('toggle-enabled', rule, v)"
                  />
                  <!-- Conflict indicator -->
                  <UiTooltipProvider v-if="(conflictsMap.get(rule.id)?.length ?? 0) > 0">
                    <UiTooltip>
                      <UiTooltipTrigger as-child>
                        <span class="inline-flex items-center shrink-0 cursor-help">
                          <component :is="AlertTriangleIcon" class="w-4 h-4 text-amber-500" />
                        </span>
                      </UiTooltipTrigger>
                      <UiTooltipContent side="top" class="max-w-xs text-xs">
                        <p
                          v-for="(conflict, idx) in conflictsMap.get(rule.id)"
                          :key="idx"
                          class="mb-1 last:mb-0"
                        >
                          {{ conflict }}
                        </p>
                      </UiTooltipContent>
                    </UiTooltip>
                  </UiTooltipProvider>
                  <!-- Human-readable condition (no service name — it's in the section header) -->
                  <span
                    :class="rule.enabled === false ? 'text-muted-foreground' : 'text-foreground'"
                    >{{ fieldLabel(rule.field) }}</span
                  >
                  <span class="text-muted-foreground">{{ operatorLabel(rule.operator) }}</span>
                  <span
                    v-if="rule.operator !== 'never'"
                    :class="rule.enabled === false ? 'text-muted-foreground' : 'font-medium'"
                    >"{{ rule.value }}"{{ ruleValueSuffix(rule) }}</span
                  >
                </div>
                <div class="flex items-center gap-2 shrink-0">
                  <!-- Effect badge -->
                  <UiBadge
                    variant="outline"
                    :class="
                      effectBadgeClass(
                        rule.effect || legacyEffect(rule.type ?? '', rule.intensity ?? ''),
                      )
                    "
                    class="shrink-0"
                  >
                    <span class="inline-flex items-center gap-1">
                      <span class="text-xs">{{
                        effectIconMap[
                          rule.effect || legacyEffect(rule.type ?? '', rule.intensity ?? '')
                        ] || ''
                      }}</span>
                      {{
                        effectLabel(
                          rule.effect || legacyEffect(rule.type ?? '', rule.intensity ?? ''),
                        )
                      }}
                    </span>
                  </UiBadge>
                  <UiButton
                    variant="ghost"
                    size="icon-sm"
                    aria-label="Delete rule"
                    class="text-muted-foreground hover:text-red-500 shrink-0"
                    @click="$emit('delete-rule', rule.id)"
                  >
                    <component :is="XIcon" class="w-4 h-4" />
                  </UiButton>
                </div>
              </div>
            </div>
          </UiCollapsibleContent>
        </UiCollapsible>
      </div>
    </UiCardContent>
  </UiCard>

  <!-- Hidden file input for import -->
  <input ref="fileInputRef" type="file" accept=".json" class="hidden" @change="onFileSelected" />

  <!-- Import confirmation dialog (no mapping needed) -->
  <UiDialog :open="showConfirmDialog" @update:open="(v: boolean) => (showConfirmDialog = v)">
    <UiDialogContent class="max-w-sm">
      <UiDialogHeader>
        <UiDialogTitle>{{ $t('rules.importConfirm') }}</UiDialogTitle>
      </UiDialogHeader>
      <p class="text-sm text-muted-foreground">
        {{
          $t(
            'rules.importConfirmDesc',
            { count: parsedPayload?.rules?.length ?? 0 },
            parsedPayload?.rules?.length ?? 0,
          )
        }}
      </p>
      <UiDialogFooter>
        <UiButton variant="outline" @click="showConfirmDialog = false">
          {{ $t('common.cancel') }}
        </UiButton>
        <UiButton @click="confirmImport">
          {{ $t('rules.importConfirm') }}
        </UiButton>
      </UiDialogFooter>
    </UiDialogContent>
  </UiDialog>

  <!-- Import mapping dialog (unmapped integrations) -->
  <UiDialog :open="showMappingDialog" @update:open="(v: boolean) => (showMappingDialog = v)">
    <UiDialogScrollContent class="max-w-md">
      <UiDialogHeader>
        <UiDialogTitle>{{ $t('rules.importMappingRequired') }}</UiDialogTitle>
      </UiDialogHeader>
      <p class="text-sm text-muted-foreground mb-4">
        {{ $t('rules.importMappingDesc') }}
      </p>
      <div class="space-y-4">
        <div v-for="key in unmappedKeys" :key="key" class="space-y-1.5">
          <UiLabel class="text-sm font-medium">{{ key }}</UiLabel>
          <UiSelect
            :model-value="mappingSelections[key] ?? ''"
            @update:model-value="(v: string) => (mappingSelections[key] = v)"
          >
            <UiSelectTrigger class="w-full">
              <UiSelectValue :placeholder="$t('rules.importSelectIntegration')" />
            </UiSelectTrigger>
            <UiSelectContent>
              <UiSelectItem value="__skip__">
                {{ $t('rules.importSkipRules') }}
              </UiSelectItem>
              <UiSelectItem
                v-for="integration in integrations"
                :key="integration.id"
                :value="String(integration.id)"
              >
                {{ integrationDisplayName(integration) }}
              </UiSelectItem>
            </UiSelectContent>
          </UiSelect>
        </div>
      </div>
      <UiDialogFooter class="mt-4">
        <UiButton variant="outline" @click="showMappingDialog = false">
          {{ $t('common.cancel') }}
        </UiButton>
        <UiButton :disabled="!allMapped" @click="confirmMappedImport">
          {{ $t('rules.importConfirm') }}
        </UiButton>
      </UiDialogFooter>
    </UiDialogScrollContent>
  </UiDialog>
</template>

<script setup lang="ts">
import {
  PlusIcon,
  XIcon,
  AlertTriangleIcon,
  GripVerticalIcon,
  ChevronRightIcon,
  UploadIcon,
  DownloadIcon,
} from 'lucide-vue-next';
import {
  fieldLabel,
  operatorLabel,
  effectLabel,
  effectBadgeClass,
  effectIconMap,
  legacyEffect,
  ruleValueSuffix,
  computeAllRuleConflicts,
} from '~/utils/ruleFieldMaps';
import type { CustomRule, IntegrationConfig, RuleExportEnvelope } from '~/types/api';

interface RuleGroup {
  integrationId: number;
  name: string;
  rules: CustomRule[];
}

const { t } = useI18n();
const { addToast } = useToast();

const props = defineProps<{
  rules: CustomRule[];
  integrations: IntegrationConfig[];
}>();

const emit = defineEmits<{
  'add-rule': [
    rule: { integrationId: number; field: string; operator: string; value: string; effect: string },
  ];
  'delete-rule': [id: number];
  'toggle-enabled': [rule: CustomRule, enabled: boolean];
  reorder: [order: number[]];
  'export-rules': [];
  'import-rules': [
    data: { payload: RuleExportEnvelope; integrationMapping?: Record<string, number> },
  ];
}>();

const showAddRule = ref(false);

// ─── Import State ───────────────────────────────────────────────────────────
const fileInputRef = ref<HTMLInputElement | null>(null);
const parsedPayload = ref<RuleExportEnvelope | null>(null);
const showConfirmDialog = ref(false);
const showMappingDialog = ref(false);
const unmappedKeys = ref<string[]>([]);
const mappingSelections = ref<Record<string, string>>({});

function triggerFileInput() {
  fileInputRef.value?.click();
}

function onFileSelected(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;

  const reader = new FileReader();
  reader.onload = () => {
    // Reset the input so the same file can be re-selected
    input.value = '';

    try {
      const data = JSON.parse(reader.result as string) as RuleExportEnvelope;

      // Validate structure
      if (!data || !Array.isArray(data.rules)) {
        addToast(t('rules.importInvalidFile'), 'error');
        return;
      }
      if (data.version !== 1) {
        addToast(t('rules.importInvalidVersion'), 'error');
        return;
      }

      parsedPayload.value = data;

      // Check for unmapped integrations
      const localLookup = new Set(props.integrations.map((i) => `${i.type}:${i.name}`));
      const needsMapping = new Set<string>();
      for (const rule of data.rules) {
        if (rule.integrationName && rule.integrationType) {
          const key = `${rule.integrationType}:${rule.integrationName}`;
          if (!localLookup.has(key)) {
            needsMapping.add(key);
          }
        }
      }

      if (needsMapping.size === 0) {
        // All integrations match or no integrations referenced — show simple confirm
        showConfirmDialog.value = true;
      } else {
        // Some integrations need mapping
        unmappedKeys.value = [...needsMapping];
        mappingSelections.value = {};
        showMappingDialog.value = true;
      }
    } catch {
      addToast(t('rules.importInvalidFile'), 'error');
    }
  };
  reader.readAsText(file);
}

function confirmImport() {
  if (!parsedPayload.value) return;
  showConfirmDialog.value = false;
  emit('import-rules', { payload: parsedPayload.value });
  parsedPayload.value = null;
}

const allMapped = computed(() => {
  return unmappedKeys.value.every((key) => !!mappingSelections.value[key]);
});

function confirmMappedImport() {
  if (!parsedPayload.value) return;
  showMappingDialog.value = false;

  // Build the integration mapping: "type:name" → integration ID
  const mapping: Record<string, number> = {};
  for (const key of unmappedKeys.value) {
    const val = mappingSelections.value[key];
    if (val && val !== '__skip__') {
      mapping[key] = Number(val);
    }
    // If __skip__, we don't add it to the mapping — the backend will skip those rules
  }

  emit('import-rules', {
    payload: parsedPayload.value,
    integrationMapping: Object.keys(mapping).length > 0 ? mapping : undefined,
  });
  parsedPayload.value = null;
}

function integrationDisplayName(integration: IntegrationConfig): string {
  const typeName = integration.type
    ? integration.type.charAt(0).toUpperCase() + integration.type.slice(1)
    : '';
  return typeName ? `${typeName}: ${integration.name}` : integration.name;
}

// Compute rule conflicts as a Map — runs once per rules change, not per render
const conflictsMap = computed(() => computeAllRuleConflicts(props.rules));

// Group rules by integrationId, preserving relative order within each group
const groupedRules = computed<RuleGroup[]>(() => {
  const map = new Map<number, CustomRule[]>();
  for (const rule of props.rules) {
    const id = rule.integrationId ?? 0;
    if (!map.has(id)) map.set(id, []);
    map.get(id)!.push(rule);
  }

  const groups: RuleGroup[] = [];
  for (const [integrationId, rules] of map) {
    groups.push({
      integrationId,
      name: integrationName(integrationId),
      rules,
    });
  }
  return groups;
});

function integrationName(id: number): string {
  const svc = props.integrations.find((i) => i.id === id);
  if (!svc) return `Integration #${id}`;
  const typeName = svc.type ? svc.type.charAt(0).toUpperCase() + svc.type.slice(1) : '';
  return typeName ? `${typeName}: ${svc.name}` : svc.name;
}

function onAddRule(rule: {
  integrationId: number;
  field: string;
  operator: string;
  value: string;
  effect: string;
}) {
  showAddRule.value = false;
  emit('add-rule', rule);
}

// ─── Drag-to-Reorder (scoped per integration group) ────────────────────────────
const dragSourceKey = ref<string | null>(null);
const dragOverKey = ref<string | null>(null);

function ruleKey(integrationId: number, idx: number): string {
  return `${integrationId}:${idx}`;
}

function onDragStart(event: DragEvent, integrationId: number, idx: number) {
  dragSourceKey.value = ruleKey(integrationId, idx);
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move';
    event.dataTransfer.setData('text/plain', JSON.stringify({ integrationId, idx }));
  }
}

function onDragOver(_event: DragEvent, integrationId: number, idx: number) {
  // Only allow drop within the same integration group
  if (dragSourceKey.value?.startsWith(`${integrationId}:`)) {
    dragOverKey.value = ruleKey(integrationId, idx);
  }
}

function onDragLeave() {
  dragOverKey.value = null;
}

function onDragEnd() {
  dragSourceKey.value = null;
  dragOverKey.value = null;
}

function onDrop(event: DragEvent, targetIntegrationId: number, targetIdx: number) {
  dragSourceKey.value = null;
  dragOverKey.value = null;

  const raw = event.dataTransfer?.getData('text/plain');
  if (!raw) return;

  let source: { integrationId: number; idx: number };
  try {
    source = JSON.parse(raw);
  } catch {
    return;
  }

  // Only allow reorder within the same integration group
  if (source.integrationId !== targetIntegrationId) return;
  if (source.idx === targetIdx) return;

  // Find the group and compute new order
  const group = groupedRules.value.find((g) => g.integrationId === targetIntegrationId);
  if (!group) return;

  const reordered = [...group.rules];
  const [moved] = reordered.splice(source.idx, 1);
  reordered.splice(targetIdx, 0, moved);

  // Emit the full reorder with all rule IDs (non-group rules keep their position)
  const groupIds = new Set(group.rules.map((r) => r.id));
  const fullOrder: number[] = [];
  let reorderedIdx = 0;
  for (const rule of props.rules) {
    if (groupIds.has(rule.id)) {
      fullOrder.push(reordered[reorderedIdx].id);
      reorderedIdx++;
    } else {
      fullOrder.push(rule.id);
    }
  }

  emit('reorder', fullOrder);
}
</script>
