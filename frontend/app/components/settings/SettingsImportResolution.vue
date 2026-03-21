<template>
  <UiDialog :open="open" @update:open="$emit('update:open', $event)">
    <UiDialogContent class="max-w-3xl max-h-[80vh] overflow-y-auto">
      <UiDialogHeader>
        <UiDialogTitle>{{ $t('settings.importResolveTitle') }}</UiDialogTitle>
        <UiDialogDescription>{{ $t('settings.importResolveDesc') }}</UiDialogDescription>
      </UiDialogHeader>

      <div class="space-y-4 py-4">
        <div
          v-for="rule in resolutions"
          :key="rule.index"
          class="rounded-lg border p-4 space-y-2"
          :class="{
            'border-green-500/30 bg-green-500/5': rule.resolution === 'matched',
            'border-yellow-500/30 bg-yellow-500/5': rule.resolution === 'type_fallback',
            'border-red-500/30 bg-red-500/5': rule.resolution === 'unmatched',
          }"
        >
          <!-- Rule summary -->
          <div class="flex items-center justify-between gap-2">
            <div class="flex items-center gap-2 text-sm font-medium">
              <span class="font-mono text-xs text-muted-foreground">#{{ rule.index + 1 }}</span>
              <UiBadge variant="outline" class="text-xs">{{ rule.rule.field }}</UiBadge>
              <span class="text-muted-foreground">{{ rule.rule.operator }}</span>
              <span>{{ rule.rule.value }}</span>
              <UiBadge :variant="effectVariant(rule.rule.effect)" class="text-xs">
                {{ rule.rule.effect }}
              </UiBadge>
            </div>
            <UiBadge :variant="statusVariant(rule.resolution)" class="text-xs">
              {{ statusLabel(rule.resolution) }}
            </UiBadge>
          </div>

          <!-- Original integration info -->
          <p
            v-if="rule.rule.integrationName || rule.rule.integrationType"
            class="text-xs text-muted-foreground"
          >
            {{
              $t('settings.importResolveOriginal', {
                name: rule.rule.integrationName || '?',
                type: rule.rule.integrationType || '?',
              })
            }}
          </p>

          <!-- Assignment controls for unmatched / type_fallback rules -->
          <div v-if="rule.resolution !== 'matched'" class="flex items-center gap-3 pt-1">
            <UiSelect
              :model-value="getOverrideValue(rule.index)"
              @update:model-value="(v) => setOverride(rule.index, String(v))"
            >
              <UiSelectTrigger class="w-[280px]">
                <UiSelectValue />
              </UiSelectTrigger>
              <UiSelectContent>
                <UiSelectItem
                  v-if="rule.matchedIntegrationId != null"
                  :value="String(rule.matchedIntegrationId)"
                >
                  {{ rule.matchedIntegrationName }}
                  <span class="text-muted-foreground ml-1">(auto-matched)</span>
                </UiSelectItem>
                <UiSelectItem
                  v-for="c in rule.candidates.filter(
                    (c: IntCandidate) => c.id !== rule.matchedIntegrationId,
                  )"
                  :key="c.id"
                  :value="String(c.id)"
                >
                  {{ c.name }}
                </UiSelectItem>
                <UiSelectItem value="skip">
                  {{ $t('settings.importResolveSkip') }}
                </UiSelectItem>
              </UiSelectContent>
            </UiSelect>
          </div>
        </div>
      </div>

      <UiDialogFooter>
        <UiButton variant="outline" @click="$emit('update:open', false)">
          {{ $t('settings.importResolveCancel') }}
        </UiButton>
        <UiButton @click="confirm">
          {{ $t('settings.importResolveConfirm') }}
        </UiButton>
      </UiDialogFooter>
    </UiDialogContent>
  </UiDialog>
</template>

<script setup lang="ts">
import type { RuleResolution, RuleOverride, IntCandidate } from '~/types/api';

const { t } = useI18n();

const props = defineProps<{
  open: boolean;
  resolutions: RuleResolution[];
}>();

const emit = defineEmits<{
  'update:open': [value: boolean];
  confirm: [overrides: RuleOverride[]];
}>();

// Track user decisions for each rule
const decisions = reactive<Record<number, string>>({});

// Initialize decisions based on pre-matched results
watch(
  () => props.resolutions,
  (rules) => {
    for (const r of rules) {
      if (r.resolution === 'matched' && r.matchedIntegrationId != null) {
        decisions[r.index] = String(r.matchedIntegrationId);
      } else if (r.resolution === 'type_fallback' && r.matchedIntegrationId != null) {
        decisions[r.index] = String(r.matchedIntegrationId);
      } else {
        decisions[r.index] =
          r.candidates.length === 1 ? String(r.candidates[0]?.id ?? 'skip') : 'skip';
      }
    }
  },
  { immediate: true },
);

function getOverrideValue(index: number): string {
  return decisions[index] ?? 'skip';
}

function setOverride(index: number, value: string) {
  decisions[index] = value;
}

function effectVariant(effect: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  if (effect.includes('keep')) return 'default';
  if (effect.includes('remove')) return 'destructive';
  return 'secondary';
}

function statusVariant(resolution: string): 'default' | 'secondary' | 'destructive' | 'outline' {
  if (resolution === 'matched') return 'default';
  if (resolution === 'type_fallback') return 'secondary';
  return 'destructive';
}

function statusLabel(resolution: string): string {
  if (resolution === 'matched') return t('settings.importResolveMatched');
  if (resolution === 'type_fallback') return t('settings.importResolveFallback');
  return t('settings.importResolveUnmatched');
}

function confirm() {
  const overrides: RuleOverride[] = [];

  for (const r of props.resolutions) {
    const decision = decisions[r.index];

    // For fully matched rules, no override needed
    if (r.resolution === 'matched' && decision === String(r.matchedIntegrationId)) {
      continue;
    }

    if (decision === 'skip' || decision === 'global') {
      // Every rule must belong to an integration — skip rules without one
      overrides.push({ index: r.index, integrationId: null, skip: true });
    } else {
      overrides.push({ index: r.index, integrationId: Number(decision), skip: false });
    }
  }

  emit('confirm', overrides);
  emit('update:open', false);
}
</script>
