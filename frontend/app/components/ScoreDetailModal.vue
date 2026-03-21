<template>
  <UiDialog
    :open="visible"
    @update:open="
      (val: boolean) => {
        if (!val) $emit('close');
      }
    "
  >
    <UiDialogContent class="max-w-lg flex flex-col max-h-[85vh]">
      <!-- Header -->
      <UiDialogHeader class="shrink-0">
        <span class="text-[10px] font-medium uppercase tracking-widest text-muted-foreground"
          >Score Detail</span
        >
        <div class="flex items-center justify-between gap-2">
          <UiDialogTitle class="truncate flex-1" :title="mediaName">
            {{ mediaName }}
          </UiDialogTitle>
          <UiBadge variant="secondary" class="capitalize shrink-0">
            {{ mediaType }}
          </UiBadge>
        </div>
      </UiDialogHeader>

      <!-- Body -->
      <div class="flex-1 min-h-0 overflow-y-auto space-y-4">
        <!-- Protected Item (always_keep) -->
        <div
          v-if="isProtected"
          class="rounded-lg border border-emerald-500/30 bg-emerald-500/5 p-4 space-y-3"
        >
          <div class="flex items-center gap-2 text-emerald-600 dark:text-emerald-400">
            <span class="text-sm">🛡️</span>
            <span class="text-sm font-medium">{{ protectedRuleName }}</span>
          </div>
          <p
            v-if="protectedMatchedValue"
            class="text-[11px] text-muted-foreground truncate"
            :title="protectedMatchedValue"
          >
            Actual: {{ protectedMatchedValue }}
          </p>
          <p class="text-xs text-muted-foreground">Score overridden — item is immune to deletion</p>
          <div class="flex items-center gap-2 pt-2 border-t border-emerald-500/20">
            <span class="text-xs text-muted-foreground">Final Score:</span>
            <span class="text-lg font-bold text-emerald-500 tabular-nums font-mono">Protected</span>
          </div>
        </div>

        <!-- Weighted Score Section -->
        <div
          v-if="!isProtected && weightFactors.length > 0"
          class="rounded-lg border border-border/50 bg-card/80 p-4 space-y-3"
        >
          <h3 class="text-xs font-medium uppercase tracking-wider text-muted-foreground">
            Weighted Score
          </h3>
          <!-- Factor rows -->
          <div class="space-y-1.5">
            <div
              v-for="(f, idx) in weightFactors"
              :key="f.name"
              v-motion
              v-bind="listItem(idx * 40)"
              class="flex items-center justify-between text-sm"
            >
              <span class="flex items-center gap-2">
                <span
                  class="w-2 h-2 rounded-full shrink-0"
                  :style="{ backgroundColor: factorColor(f.name) }"
                />
                <span class="text-muted-foreground">{{ f.name }}</span>
              </span>
              <span class="font-mono tabular-nums text-foreground">
                <span class="text-muted-foreground"
                  >{{ f.rawScore.toFixed(2) }} × {{ f.weight }}</span
                >
                <span class="mx-1.5 text-muted-foreground/50">=</span>
                <span class="font-semibold">{{ f.contribution.toFixed(3) }}</span>
              </span>
            </div>
          </div>
          <!-- Base score total -->
          <div class="flex items-center justify-between pt-2 border-t border-border/50">
            <span class="text-xs font-medium text-muted-foreground">Base Score</span>
            <span class="font-mono tabular-nums font-bold text-foreground">{{
              baseScore.toFixed(3)
            }}</span>
          </div>
          <!-- Normalization note -->
          <p class="text-[10px] text-muted-foreground/60 leading-relaxed">
            Each slider value determines how much that factor matters. Your slider values add up to
            {{ totalWeight }}, so each factor's share of the score is its slider value divided by
            {{ totalWeight }}.
          </p>
        </div>

        <!-- Custom Rules Section -->
        <div
          v-if="!isProtected && ruleFactors.length > 0"
          class="rounded-lg border border-border/50 bg-card/80 p-4 space-y-3"
        >
          <h3 class="text-xs font-medium uppercase tracking-wider text-muted-foreground">
            Custom Rules
          </h3>
          <!-- Rule modifier rows -->
          <div class="space-y-2.5">
            <div v-for="(f, idx) in ruleFactors" :key="f.name" v-motion v-bind="listItem(idx * 40)">
              <div class="flex items-center justify-between text-sm">
                <span class="flex items-center gap-2 min-w-0">
                  <span class="text-xs shrink-0">{{ ruleIcon(f.name) }}</span>
                  <span class="text-muted-foreground truncate">{{ f.name }}</span>
                </span>
                <span class="font-mono tabular-nums font-semibold text-foreground shrink-0 ml-2"
                  >× {{ f.rawScore.toFixed(2) }}</span
                >
              </div>
              <!-- Matched value -->
              <p
                v-if="f.matchedValue"
                class="text-[11px] text-muted-foreground/70 ml-6 mt-0.5 truncate"
                :title="f.matchedValue"
              >
                Actual: {{ f.matchedValue }}
              </p>
            </div>
          </div>
          <!-- Combined modifier -->
          <div
            v-if="ruleFactors.length > 1"
            class="flex items-center justify-between pt-2 border-t border-border/50"
          >
            <span class="text-xs font-medium text-muted-foreground">Combined Modifier</span>
            <span class="font-mono tabular-nums text-muted-foreground">
              {{ ruleFactors.map((f) => f.rawScore.toFixed(2)).join(' × ') }} =
              <span class="font-bold text-foreground">{{ ruleModifier.toFixed(3) }}</span>
            </span>
          </div>
          <div v-else class="flex items-center justify-between pt-2 border-t border-border/50">
            <span class="text-xs font-medium text-muted-foreground">Rule Modifier</span>
            <span class="font-mono tabular-nums font-bold text-foreground">{{
              ruleModifier.toFixed(3)
            }}</span>
          </div>
        </div>

        <!-- Final Score -->
        <div
          v-if="!isProtected"
          class="rounded-lg border border-primary/20 bg-primary/5 p-4 space-y-2"
        >
          <h3 class="text-xs font-medium uppercase tracking-wider text-muted-foreground">
            Final Score
          </h3>
          <div
            v-if="ruleFactors.length > 0"
            class="font-mono text-sm tabular-nums text-muted-foreground"
          >
            {{ baseScore.toFixed(3) }} <span class="text-[10px]">(base)</span> ×
            {{ ruleModifier.toFixed(3) }} <span class="text-[10px]">(modifier)</span> =
            <span class="font-bold text-lg" :class="scoreColorClass">{{ score.toFixed(2) }}</span>
          </div>
          <div v-else class="flex items-center gap-2">
            <span class="text-xs text-muted-foreground">No rules applied</span>
            <span class="mx-1 text-muted-foreground/50">→</span>
            <span class="text-primary font-medium text-xs">Score:</span>
            <span class="font-mono tabular-nums font-bold text-lg" :class="scoreColorClass">
              {{ score.toFixed(2) }}
            </span>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <UiDialogFooter
        class="shrink-0 flex-row items-center justify-between border-t border-primary/10 dark:border-primary/15 pt-3"
      >
        <div class="flex items-center gap-3">
          <span class="text-sm text-muted-foreground font-mono tabular-nums">
            {{ formatBytes(sizeBytes) }}
          </span>
          <UiBadge v-if="action" :variant="actionBadgeVariant">
            {{ action }}
          </UiBadge>
        </div>
        <span v-if="createdAt" class="inline-flex items-center gap-1 text-xs text-muted-foreground">
          <ClockIcon class="w-3 h-3" />
          <DateDisplay :date="createdAt" />
        </span>
      </UiDialogFooter>
    </UiDialogContent>
  </UiDialog>
</template>

<script setup lang="ts">
import { ClockIcon } from 'lucide-vue-next';
import { formatBytes } from '~/utils/format';
import type { ScoreFactor } from '~/types/api';

const { listItem } = useMotionPresets();

interface Props {
  visible: boolean;
  mediaName: string;
  mediaType: string;
  score: number;
  scoreDetails: string;
  sizeBytes: number;
  action: string;
  createdAt?: string;
}

const props = defineProps<Props>();
defineEmits<{ close: [] }>();

const FACTOR_COLORS: Record<string, string> = {
  'Watch History': '#8b5cf6',
  'Last Watched': '#3b82f6',
  'File Size': '#f59e0b',
  Rating: '#10b981',
  'Time in Library': '#f97316',
  'Series Status': '#ec4899',
};

function factorColor(name: string): string {
  return FACTOR_COLORS[name] || '#6b7280';
}

function ruleIcon(name: string): string {
  if (name.includes('Always keep')) return '🛡️';
  if (name.includes('Prefer keep')) return '🟢';
  if (name.includes('Lean keep')) return '🔵';
  if (name.includes('Lean remove')) return '🟡';
  if (name.includes('Prefer remove')) return '🟠';
  if (name.includes('Always remove')) return '🔴';
  return '📋';
}

// Parse factors
const parsedFactors = computed<ScoreFactor[]>(() => {
  if (!props.scoreDetails) return [];
  try {
    const parsed = JSON.parse(props.scoreDetails);
    if (Array.isArray(parsed)) return parsed as ScoreFactor[];
  } catch (err) {
    console.warn('[ScoreDetailModal] parseFactors failed:', err);
  }
  return [];
});

const weightFactors = computed(() => parsedFactors.value.filter((f) => f.type === 'weight'));
const ruleFactors = computed(() => parsedFactors.value.filter((f) => f.type === 'rule'));

const totalWeight = computed(() => weightFactors.value.reduce((sum, f) => sum + f.weight, 0));

const baseScore = computed(() => weightFactors.value.reduce((sum, f) => sum + f.contribution, 0));

const ruleModifier = computed(() => ruleFactors.value.reduce((mod, f) => mod * f.rawScore, 1.0));

const isProtected = computed(() => {
  return ruleFactors.value.some((f) => f.name.includes('Always keep')) && props.score === 0;
});

const protectedRuleName = computed(() => {
  const factor = ruleFactors.value.find((f) => f.name.includes('Always keep'));
  return factor?.name || 'Protected by rule';
});

const protectedMatchedValue = computed(() => {
  const factor = ruleFactors.value.find((f) => f.name.includes('Always keep'));
  return factor?.matchedValue || '';
});

// Score color class
const scoreColorClass = computed(() => {
  if (props.score >= 0.7) return 'text-destructive';
  if (props.score >= 0.4) return 'text-warning';
  return 'text-success';
});

// Action badge variant
const actionBadgeVariant = computed<'destructive' | 'outline' | 'secondary' | 'default'>(() => {
  if (props.action === 'Deleted') return 'destructive';
  if (props.action === 'Dry-Run' || props.action === 'Dry-Delete') return 'secondary';
  if (props.action === 'Pending' || props.action === 'Snoozed') return 'outline';
  if (props.action === 'Approved') return 'default';
  return 'default';
});
</script>
