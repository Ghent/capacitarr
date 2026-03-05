import type { CustomRule } from '~/types/api';

// ---------------------------------------------------------------------------
// Label / display maps for rule fields, operators, and effects
// ---------------------------------------------------------------------------

export const operatorLabelMap: Record<string, string> = {
  '==': 'is',
  '!=': 'is not',
  contains: 'contains',
  '!contains': 'does not contain',
  '>': 'more than',
  '>=': 'at least',
  '<': 'less than',
  '<=': 'at most',
  in_last: 'in the last',
  over_ago: 'over…ago',
  never: 'never',
};

export const effectLabelMap: Record<string, string> = {
  always_keep: 'Always keep',
  prefer_keep: 'Prefer to keep',
  lean_keep: 'Lean toward keeping',
  lean_remove: 'Lean toward removing',
  prefer_remove: 'Prefer to remove',
  always_remove: 'Always remove',
};

export const effectBadgeClassMap: Record<string, string> = {
  always_keep: 'bg-transparent border-emerald-600/60 text-emerald-700 dark:text-emerald-300',
  prefer_keep: 'bg-transparent border-green-500/50 text-green-600 dark:text-green-400',
  lean_keep: 'bg-transparent border-sky-500/50 text-sky-600 dark:text-sky-400',
  lean_remove: 'bg-transparent border-amber-500/50 text-amber-700 dark:text-amber-400',
  prefer_remove: 'bg-transparent border-orange-500/50 text-orange-600 dark:text-orange-400',
  always_remove: 'bg-transparent border-red-600/60 text-red-700 dark:text-red-400',
};

export const effectIconMap: Record<string, string> = {
  always_keep: '🛡️',
  prefer_keep: '🟢',
  lean_keep: '🔵',
  lean_remove: '🟡',
  prefer_remove: '🟠',
  always_remove: '🔴',
};

export const fieldLabelMap: Record<string, string> = {
  title: 'Title',
  quality: 'Quality Profile',
  tag: 'Tags',
  genre: 'Genre',
  rating: 'Rating',
  sizebytes: 'Size',
  timeinlibrary: 'Time in Library',
  monitored: 'Monitored',
  year: 'Year',
  language: 'Language',
  seriesstatus: 'Series Status',
  seasoncount: 'Season Count',
  episodecount: 'Episode Count',
  playcount: 'Play Count',
  lastplayed: 'Last Watched',
  requested: 'Is Requested',
  requestcount: 'Request Count',
  requestedby: 'Requested By',
  incollection: 'In Collection',
  watchedbyreq: 'Watched by Requestor',
  type: 'Media Type',
};

// ---------------------------------------------------------------------------
// Display helper functions
// ---------------------------------------------------------------------------

export function effectLabel(effect: string): string {
  return effectLabelMap[effect] ?? effect;
}

export function effectBadgeClass(effect: string): string {
  return effectBadgeClassMap[effect] ?? 'bg-muted text-foreground';
}

export function operatorLabel(op: string): string {
  return operatorLabelMap[op] ?? op;
}

export function fieldLabel(field: string): string {
  return fieldLabelMap[field] ?? field;
}

/** Show "days" suffix for date-aware operators in rule cards */
export function ruleValueSuffix(rule: { field: string; operator: string }): string {
  if (
    ['in_last', 'over_ago'].includes(rule.operator) &&
    ['lastplayed', 'timeinlibrary'].includes(rule.field)
  ) {
    return ' days';
  }
  return '';
}

/** Convert legacy type+intensity to new effect (for display of pre-migration rules) */
export function legacyEffect(type: string, intensity: string): string {
  if (type === 'protect') {
    if (intensity === 'absolute') return 'always_keep';
    if (intensity === 'strong') return 'prefer_keep';
    return 'lean_keep';
  }
  if (type === 'target') {
    if (intensity === 'absolute') return 'always_remove';
    if (intensity === 'strong') return 'prefer_remove';
    return 'lean_remove';
  }
  return 'lean_keep';
}

// ---------------------------------------------------------------------------
// Conflict Detection — O(n²) computed once per rules change, not per render
// ---------------------------------------------------------------------------

const keepEffects = new Set(['always_keep', 'prefer_keep', 'lean_keep']);
const removeEffects = new Set(['lean_remove', 'prefer_remove', 'always_remove']);

/** Fields that use numeric values */
const numericFields = new Set([
  'rating',
  'sizebytes',
  'timeinlibrary',
  'year',
  'seasoncount',
  'episodecount',
  'playcount',
  'requestcount',
  'lastplayed',
]);

/** Fields that use boolean values */
const booleanFields = new Set(['monitored', 'requested']);

function ruleEffectDirection(rule: CustomRule): 'keep' | 'remove' | 'unknown' {
  const eff = rule.effect || legacyEffect(rule.type ?? '', rule.intensity ?? '');
  if (keepEffects.has(eff)) return 'keep';
  if (removeEffects.has(eff)) return 'remove';
  return 'unknown';
}

/**
 * Check if two rules targeting the same field could ever match the same item.
 * Returns true if the rules' conditions could overlap, false if they're mutually exclusive.
 */
function rulesCouldOverlap(a: CustomRule, b: CustomRule): boolean {
  if (a.field !== b.field) return false;

  const field = a.field;

  // Boolean fields: same value = overlap, different values = no overlap
  if (booleanFields.has(field)) {
    return a.value === b.value;
  }

  // Numeric fields: check range intersection
  if (numericFields.has(field)) {
    return numericRangesOverlap(a.operator, Number(a.value), b.operator, Number(b.value));
  }

  // String fields with exact match operators and different values = no overlap
  if (a.operator === '==' && b.operator === '==') {
    return a.value === b.value;
  }

  // Mutual exclusion: positive match vs negation of the same value
  const aVal = a.value.toLowerCase();
  const bVal = b.value.toLowerCase();

  if (a.operator === 'contains' && b.operator === '!contains' && aVal === bVal) return false;
  if (a.operator === '!contains' && b.operator === 'contains' && aVal === bVal) return false;
  if (a.operator === '==' && b.operator === '!=' && aVal === bVal) return false;
  if (a.operator === '!=' && b.operator === '==' && aVal === bVal) return false;
  if (a.operator === '==' && b.operator === '!contains' && aVal.includes(bVal)) return false;
  if (a.operator === '!contains' && b.operator === '==' && bVal.includes(aVal)) return false;

  // For all other mixed operators, conservatively assume overlap
  return true;
}

/**
 * Determine if two numeric operator+value pairs describe overlapping ranges.
 */
function numericRangesOverlap(opA: string, valA: number, opB: string, valB: number): boolean {
  const dateOpMap: Record<string, string> = { in_last: '<', over_ago: '>' };
  opA = dateOpMap[opA] ?? opA;
  opB = dateOpMap[opB] ?? opB;

  if (isNaN(valA) || isNaN(valB)) return true;

  const rangeA = numericToRange(opA, valA);
  const rangeB = numericToRange(opB, valB);

  return rangeA[0] <= rangeB[1] && rangeA[1] >= rangeB[0];
}

/** Convert an operator+value into a [min, max] range tuple. */
function numericToRange(op: string, val: number): [number, number] {
  const INF = Number.MAX_SAFE_INTEGER;
  const NEG_INF = Number.MIN_SAFE_INTEGER;
  switch (op) {
    case '==':
      return [val, val];
    case '!=':
      return [NEG_INF, INF];
    case '>':
      return [val + 0.001, INF];
    case '>=':
      return [val, INF];
    case '<':
      return [NEG_INF, val - 0.001];
    case '<=':
      return [NEG_INF, val];
    default:
      return [NEG_INF, INF];
  }
}

/**
 * Compute all rule conflicts for a list of rules — O(n²) but runs once per rules change.
 * Returns a Map from rule.id → array of conflict description strings.
 */
export function computeAllRuleConflicts(rules: CustomRule[]): Map<number, string[]> {
  const result = new Map<number, string[]>();

  for (const rule of rules) {
    const direction = ruleEffectDirection(rule);
    if (direction === 'unknown') {
      result.set(rule.id, []);
      continue;
    }
    const eff = rule.effect || legacyEffect(rule.type ?? '', rule.intensity ?? '');

    const conflicts: string[] = [];
    for (const other of rules) {
      if (other.id === rule.id) continue;
      const otherDirection = ruleEffectDirection(other);
      if (otherDirection === 'unknown' || otherDirection === direction) continue;

      const sameScope =
        (!rule.integrationId && !other.integrationId) ||
        !rule.integrationId ||
        !other.integrationId ||
        rule.integrationId === other.integrationId;

      if (!sameScope) continue;
      if (!rulesCouldOverlap(rule, other)) continue;

      const otherEff = other.effect || legacyEffect(other.type ?? '', other.intensity ?? '');
      const otherName = `${fieldLabel(other.field)} ${operatorLabel(other.operator)} "${other.value}" → ${effectLabel(otherEff)}`;

      if (eff === 'always_keep' || otherEff === 'always_keep') {
        conflicts.push(`Conflicts with "${otherName}". When both match, "Always keep" wins.`);
      } else {
        conflicts.push(
          `Conflicts with "${otherName}". When both match, effects multiply together.`,
        );
      }
    }
    result.set(rule.id, conflicts);
  }

  return result;
}
