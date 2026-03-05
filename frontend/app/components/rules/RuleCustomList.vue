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
        <UiButton size="sm" @click="showAddRule = !showAddRule">
          <component :is="PlusIcon" class="w-3.5 h-3.5" />
          {{ $t('rules.addRule') }}
        </UiButton>
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

      <!-- Rules List — Natural Language Display with Conflict Indicators -->
      <div
        v-if="rules.length === 0 && !showAddRule"
        class="text-center py-6 text-muted-foreground text-sm"
      >
        {{ $t('rules.noRules') }}
      </div>
      <div v-else class="space-y-2">
        <div
          v-for="(rule, ruleIdx) in rules"
          :key="rule.id"
          draggable="true"
          class="flex items-center justify-between px-4 py-2.5 rounded-lg border bg-muted/50 transition-opacity duration-200"
          :class="[
            (conflictsMap.get(rule.id)?.length ?? 0) > 0 ? 'border-amber-400/50' : 'border-border',
            rule.enabled === false ? 'opacity-50' : '',
            dragOverIdx === ruleIdx ? 'border-primary border-dashed' : '',
            dragSourceIdx === ruleIdx ? 'opacity-30' : '',
          ]"
          @dragstart="onDragStart($event, ruleIdx)"
          @dragover.prevent="onDragOver($event, ruleIdx)"
          @dragleave="onDragLeave"
          @drop.prevent="onDrop($event, ruleIdx)"
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
            <!-- Rule number -->
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
            <!-- Service name -->
            <span v-if="rule.integrationId" class="text-muted-foreground">
              {{ integrationName(rule.integrationId) }} ·
            </span>
            <!-- Human-readable condition -->
            <span :class="rule.enabled === false ? 'text-muted-foreground' : 'text-foreground'">{{
              fieldLabel(rule.field)
            }}</span>
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
                effectBadgeClass(rule.effect || legacyEffect(rule.type ?? '', rule.intensity ?? ''))
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
                  effectLabel(rule.effect || legacyEffect(rule.type ?? '', rule.intensity ?? ''))
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
    </UiCardContent>
  </UiCard>
</template>

<script setup lang="ts">
import { PlusIcon, XIcon, AlertTriangleIcon, GripVerticalIcon } from 'lucide-vue-next';
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
}>();

const showAddRule = ref(false);

// Compute rule conflicts as a Map — runs once per rules change, not per render
const conflictsMap = computed(() => computeAllRuleConflicts(props.rules));

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

// ─── Drag-to-Reorder ───────────────────────────────────────────────────────────
const dragSourceIdx = ref<number | null>(null);
const dragOverIdx = ref<number | null>(null);

function onDragStart(event: DragEvent, idx: number) {
  dragSourceIdx.value = idx;
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move';
    event.dataTransfer.setData('text/plain', String(idx));
  }
}

function onDragOver(_event: DragEvent, idx: number) {
  dragOverIdx.value = idx;
}

function onDragLeave() {
  dragOverIdx.value = null;
}

function onDragEnd() {
  dragSourceIdx.value = null;
  dragOverIdx.value = null;
}

function onDrop(_event: DragEvent, targetIdx: number) {
  const sourceIdx = dragSourceIdx.value;
  dragSourceIdx.value = null;
  dragOverIdx.value = null;

  if (sourceIdx === null || sourceIdx === targetIdx) return;

  // Compute new order and emit to parent
  const reordered = [...props.rules];
  const [moved] = reordered.splice(sourceIdx, 1);
  reordered.splice(targetIdx, 0, moved);
  emit(
    'reorder',
    reordered.map((r) => r.id),
  );
}
</script>
