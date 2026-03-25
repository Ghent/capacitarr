<template>
  <div v-if="factors.length > 0" class="flex flex-col gap-1">
    <div class="flex items-center gap-2">
      <span
        :class="[
          'font-semibold tabular-nums text-foreground',
          size === 'sm' ? 'text-xs' : 'text-sm',
        ]"
      >
        {{ scoreDisplay }}
      </span>
      <div
        v-if="weightFactors.length > 0"
        :class="[
          'flex rounded-full overflow-hidden bg-muted flex-1 min-w-0',
          size === 'sm' ? 'h-1.5' : 'h-2',
        ]"
      >
        <div
          v-for="f in weightFactors"
          :key="f.name"
          class="h-full transition-all duration-300"
          :style="{
            width: totalContrib > 0 ? `${(f.contribution / totalContrib) * 100}%` : '0%',
            backgroundColor: factorColor(f.key),
            minWidth: f.contribution > 0 ? '2px' : '0px',
          }"
          :title="`${f.name}: ${f.contribution.toFixed(2)}`"
        />
      </div>
    </div>
    <div v-if="weightFactors.length > 0" class="flex flex-wrap gap-x-2 gap-y-0.5">
      <span
        v-for="f in visibleWeightFactors"
        :key="f.name"
        class="inline-flex items-center gap-1 text-[10px] text-zinc-500"
        :title="`${f.name}: ${f.contribution.toFixed(2)} (raw: ${f.rawScore.toFixed(2)}, weight: ${f.weight})`"
      >
        <span
          class="w-1.5 h-1.5 rounded-full flex-shrink-0"
          :style="{ backgroundColor: factorColor(f.key) }"
        />
        <span>{{ factorAbbr(f.key) }}{{ f.contribution.toFixed(2) }}</span>
      </span>
    </div>
    <div v-if="skippedFactors.length > 0" class="flex flex-wrap gap-x-2 gap-y-0.5">
      <span
        v-for="f in skippedFactors"
        :key="f.name"
        class="inline-flex items-center gap-1 text-[10px] text-amber-500/70"
        :title="`${f.name}: ${f.skipReason}`"
      >
        <span>⚠</span>
        <span class="line-through">{{ factorAbbr(f.key) }}</span>
        <span class="no-underline">{{ f.skipReason }}</span>
      </span>
    </div>
    <div v-if="ruleFactors.length > 0" class="flex flex-wrap gap-1 mt-0.5">
      <span
        v-for="f in ruleFactors"
        :key="f.name"
        class="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-medium"
        :class="
          f.name.includes('Protect')
            ? 'bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400'
            : 'bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400'
        "
        :title="f.name"
      >
        {{ f.name }}
      </span>
    </div>
  </div>
  <div v-else-if="legacyParsed.score" class="flex flex-col gap-1">
    <div class="flex items-center gap-2">
      <span :class="['font-semibold tabular-nums', size === 'sm' ? 'text-xs' : 'text-sm']">
        {{ legacyParsed.score }}
      </span>
      <div
        v-if="legacyParsed.factors.length > 0"
        :class="[
          'flex rounded-full overflow-hidden bg-muted flex-1 min-w-0',
          size === 'sm' ? 'h-1.5' : 'h-2',
        ]"
      >
        <div
          v-for="f in legacyParsed.factors"
          :key="f.name"
          class="h-full transition-all duration-300"
          :style="{
            width: legacyTotalContrib > 0 ? `${(f.value / legacyTotalContrib) * 100}%` : '0%',
            backgroundColor: factorColor(f.name),
            minWidth: f.value > 0 ? '2px' : '0px',
          }"
          :title="`${f.label}: ${f.value.toFixed(2)}`"
        />
      </div>
    </div>
    <div v-if="legacyParsed.factors.length > 0" class="flex flex-wrap gap-x-2 gap-y-0.5">
      <span
        v-for="f in legacyVisibleFactors"
        :key="f.name"
        class="inline-flex items-center gap-1 text-[10px] text-zinc-500"
        :title="`${f.label}: ${f.value.toFixed(2)}`"
      >
        <span
          class="w-1.5 h-1.5 rounded-full flex-shrink-0"
          :style="{ backgroundColor: factorColor(f.name) }"
        />
        <span>{{ f.abbr }}{{ f.value.toFixed(2) }}</span>
      </span>
    </div>
    <span
      v-if="legacyParsed.factors.length === 0 && legacyParsed.rawBreakdown"
      class="text-[10px] text-zinc-400 truncate max-w-[200px]"
      :title="legacyParsed.rawBreakdown"
    >
      {{ legacyParsed.rawBreakdown }}
    </span>
  </div>
  <span v-else class="text-xs text-zinc-500 max-w-xs truncate block" :title="reason">
    {{ reason }}
  </span>
</template>

<script setup lang="ts">
import type { ScoreFactor } from '~/types/api';
import { factorColor, factorAbbr } from '~/utils/factorColors';

interface Props {
  reason?: string;
  score?: number;
  scoreDetails?: string;
  factors?: ScoreFactor[];
  size?: string;
}
const props = withDefaults(defineProps<Props>(), {
  reason: '',
  size: 'md',
  score: undefined,
  scoreDetails: '',
  factors: () => [],
});

// Parse structured factors from scoreDetails JSON or direct factors prop
const factors = computed<ScoreFactor[]>(() => {
  // Prefer direct factors prop (from preview API)
  if (props.factors && props.factors.length > 0) {
    return props.factors;
  }
  // Try parsing scoreDetails JSON string (from audit API)
  if (props.scoreDetails) {
    try {
      const parsed = JSON.parse(props.scoreDetails);
      if (Array.isArray(parsed) && parsed.length > 0) {
        return parsed as ScoreFactor[];
      }
    } catch (err) {
      console.warn('[ScoreBreakdown] parseScoreDetails failed:', err);
    }
  }
  return [];
});

const weightFactors = computed(() =>
  factors.value.filter((f) => f.type === 'weight' && !f.skipped),
);
const skippedFactors = computed(() =>
  factors.value.filter((f) => f.type === 'weight' && f.skipped),
);
const ruleFactors = computed(() => factors.value.filter((f) => f.type === 'rule'));

const totalContrib = computed(() =>
  weightFactors.value.reduce((sum, f) => sum + f.contribution, 0),
);

const visibleWeightFactors = computed(() =>
  weightFactors.value.filter((f) => f.contribution > 0.01),
);

// Prefer the dedicated score prop; fall back to parsing the reason string for backward compat
const scoreDisplay = computed(() => {
  if (props.score !== undefined && props.score !== null) {
    return props.score.toFixed(2);
  }
  const match = props.reason.match(/^Score:\s*([\d.]+)/);
  return match ? match[1] : factors.value.reduce((s, f) => s + f.contribution, 0).toFixed(2);
});

// Legacy parsing for backward compatibility with old audit logs
interface LegacyFactor {
  name: string;
  label: string;
  abbr: string;
  value: number;
}

interface LegacyParsed {
  score: string | null;
  factors: LegacyFactor[];
  rawBreakdown: string;
}

function parseLegacyReason(reason: string): LegacyParsed {
  const scoreMatch = reason.match(/^Score:\s*([\d.]+)\s*\((.+)\)$/);
  if (!scoreMatch) {
    return { score: null, factors: [], rawBreakdown: reason };
  }

  const score = scoreMatch[1];
  const breakdownStr = scoreMatch[2];

  const factorPattern = /(\w+):([\d.]+)/g;
  const legacyFactors: LegacyFactor[] = [];
  let match: RegExpExecArray | null;

  while ((match = factorPattern.exec(breakdownStr!)) !== null) {
    const name = match[1]!;
    const value = parseFloat(match[2]!);
    legacyFactors.push({
      name,
      label: name,
      abbr: name.slice(0, 1).toUpperCase() + ':',
      value,
    });
  }

  return { score: score ?? null, factors: legacyFactors, rawBreakdown: breakdownStr ?? '' };
}

const legacyParsed = computed(() => parseLegacyReason(props.reason));

const legacyTotalContrib = computed(() =>
  legacyParsed.value.factors.reduce((sum, f) => sum + f.value, 0),
);

const legacyVisibleFactors = computed(() =>
  legacyParsed.value.factors.filter((f) => f.value > 0.01),
);
</script>
