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
          <UiButton variant="outline" size="sm" @click="goToBackupRestore">
            <component :is="ArchiveIcon" class="w-3.5 h-3.5" />
            {{ $t('rules.backupRestore') }}
          </UiButton>
          <UiButton
            size="sm"
            @click="
              viewMode = 'add';
              editingRule = null;
            "
          >
            <component :is="PlusIcon" class="w-3.5 h-3.5" />
            {{ $t('rules.addRule') }}
          </UiButton>
        </div>
      </div>
    </UiCardHeader>
    <UiCardContent>
      <!-- Editor Mode (Add or Edit) — state-swap with list -->
      <div
        v-if="viewMode !== 'list'"
        v-motion
        :initial="{ opacity: 0, x: 12 }"
        :enter="{ opacity: 1, x: 0, transition: { duration: 200 } }"
      >
        <!-- Back link + title -->
        <div class="flex items-center gap-2 mb-4">
          <UiButton
            variant="ghost"
            size="sm"
            @click="
              viewMode = 'list';
              editingRule = null;
            "
          >
            <ArrowLeftIcon class="w-4 h-4" />
            {{ $t('rules.backToRules') }}
          </UiButton>
          <span class="text-sm font-medium">
            {{ viewMode === 'edit' ? $t('rules.editRule') : $t('rules.addRule') }}
          </span>
        </div>

        <!-- RuleBuilder form -->
        <RuleBuilder
          :key="editingRule?.id ?? 'new'"
          :integrations="integrations"
          :initial-rule="
            editingRule
              ? {
                  id: editingRule.id,
                  integrationId: editingRule.integrationId ?? 0,
                  field: editingRule.field,
                  operator: editingRule.operator,
                  value: editingRule.value,
                  effect: editingRule.effect,
                }
              : undefined
          "
          @save="onAddRule"
          @update="onUpdateRule"
          @cancel="
            viewMode = 'list';
            editingRule = null;
          "
        />
      </div>

      <!-- List Mode -->
      <div
        v-else
        v-motion
        :initial="{ opacity: 0, x: -12 }"
        :enter="{ opacity: 1, x: 0, transition: { duration: 200 } }"
      >
        <!-- Empty state -->
        <div v-if="rules.length === 0" class="text-center py-6 text-muted-foreground text-sm">
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
                    <!-- Impact badge — shows how many items this rule affects -->
                    <NuxtLink
                      v-if="rule.enabled !== false"
                      :to="`/library?ruleId=${rule.id}`"
                      class="no-underline"
                      @click.stop
                    >
                      <UiBadge
                        variant="secondary"
                        class="text-xs tabular-nums cursor-pointer hover:bg-accent transition-colors gap-1"
                      >
                        <component :is="BarChart3Icon" class="w-3 h-3" />
                        <span>{{ impactCounts.get(rule.id) ?? '—' }}</span>
                      </UiBadge>
                    </NuxtLink>
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
                      aria-label="Edit rule"
                      class="text-muted-foreground hover:text-foreground shrink-0"
                      @click="
                        editingRule = rule;
                        viewMode = 'edit';
                      "
                    >
                      <PencilIcon class="w-4 h-4" />
                    </UiButton>
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
      </div>
      <!-- end list mode -->
    </UiCardContent>
  </UiCard>
</template>

<script setup lang="ts">
import {
  PlusIcon,
  XIcon,
  AlertTriangleIcon,
  GripVerticalIcon,
  ChevronRightIcon,
  ArchiveIcon,
  BarChart3Icon,
  PencilIcon,
  ArrowLeftIcon,
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
import type { CustomRule, IntegrationConfig } from '~/types/api';

interface RuleGroup {
  integrationId: number;
  name: string;
  rules: CustomRule[];
}

const router = useRouter();
const api = useApi();

const props = defineProps<{
  rules: CustomRule[];
  integrations: IntegrationConfig[];
}>();

const emit = defineEmits<{
  'add-rule': [
    rule: { integrationId: number; field: string; operator: string; value: string; effect: string },
  ];
  'edit-rule': [
    id: number,
    rule: { integrationId: number; field: string; operator: string; value: string; effect: string },
  ];
  'delete-rule': [id: number];
  'toggle-enabled': [rule: CustomRule, enabled: boolean];
  reorder: [order: number[]];
}>();

type ViewMode = 'list' | 'add' | 'edit';
const viewMode = ref<ViewMode>('list');
const editingRule = ref<CustomRule | null>(null);

function goToBackupRestore() {
  router.push({ path: '/settings', query: { tab: 'backup', section: 'rules' } });
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
  viewMode.value = 'list';
  emit('add-rule', rule);
}

function onUpdateRule(
  id: number,
  rule: {
    integrationId: number;
    field: string;
    operator: string;
    value: string;
    effect: string;
  },
) {
  viewMode.value = 'list';
  editingRule.value = null;
  emit('edit-rule', id, rule);
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
  const moved = reordered.splice(source.idx, 1)[0];
  if (!moved) return;
  reordered.splice(targetIdx, 0, moved);

  // Emit the full reorder with all rule IDs (non-group rules keep their position)
  const groupIds = new Set(group.rules.map((r) => r.id));
  const fullOrder: number[] = [];
  let reorderedIdx = 0;
  for (const rule of props.rules) {
    if (groupIds.has(rule.id)) {
      const reorderedRule = reordered[reorderedIdx];
      if (reorderedRule) fullOrder.push(reorderedRule.id);
      reorderedIdx++;
    } else {
      fullOrder.push(rule.id);
    }
  }

  emit('reorder', fullOrder);
}

// ─── Rule impact counts (fetched from API) ────────────────────────────────
interface RuleImpact {
  ruleId: number;
  affectedCount: number;
  totalItems: number;
}

const impactCounts = ref<Map<number, number>>(new Map());

async function fetchRuleImpacts() {
  const enabledRules = props.rules.filter((r) => r.enabled !== false);
  const results = await Promise.allSettled(
    enabledRules.map((rule) =>
      (api(`/api/v1/custom-rules/${rule.id}/impact`) as Promise<RuleImpact>).then((resp) => ({
        id: rule.id,
        count: resp.affectedCount,
      })),
    ),
  );

  const newCounts = new Map<number, number>();
  for (const result of results) {
    if (result.status === 'fulfilled') {
      newCounts.set(result.value.id, result.value.count);
    }
  }
  impactCounts.value = newCounts;
}

// Fetch impacts when rules change
watch(
  () => props.rules,
  () => {
    fetchRuleImpacts();
  },
  { immediate: true },
);
</script>
