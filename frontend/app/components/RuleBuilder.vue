<template>
  <div class="p-4 rounded-lg border border-border bg-muted space-y-4 relative">
    <!-- Loading overlay during edit initialization -->
    <div
      v-if="isInitializing"
      class="absolute inset-0 flex items-center justify-center bg-background/80 rounded-lg z-10"
    >
      <LoaderCircleIcon class="w-5 h-5 animate-spin text-muted-foreground" />
    </div>
    <div
      class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3"
      :class="{ 'opacity-50 pointer-events-none': isInitializing }"
    >
      <!-- ① Service Instance -->
      <div class="space-y-1.5">
        <UiLabel class="text-xs text-muted-foreground"> Service </UiLabel>
        <UiSelect v-model="form.integrationId" @update:model-value="onServiceChange">
          <UiSelectTrigger>
            <UiSelectValue placeholder="Select service…" />
          </UiSelectTrigger>
          <UiSelectContent>
            <UiSelectItem v-for="svc in arrIntegrations" :key="svc.id" :value="String(svc.id)">
              {{ capitalize(svc.type) }}: {{ svc.name }}
            </UiSelectItem>
          </UiSelectContent>
        </UiSelect>
      </div>

      <!-- ② Action (Field) -->
      <div class="space-y-1.5">
        <UiLabel class="text-xs text-muted-foreground"> Action </UiLabel>
        <UiSelect
          v-model="form.field"
          :disabled="!form.integrationId"
          @update:model-value="onFieldChange"
        >
          <UiSelectTrigger>
            <UiSelectValue placeholder="Select field…" />
          </UiSelectTrigger>
          <UiSelectContent>
            <UiSelectItem v-for="f in fields" :key="f.field" :value="f.field">
              {{ f.label }}
            </UiSelectItem>
          </UiSelectContent>
        </UiSelect>
      </div>

      <!-- ③ Operator -->
      <div class="space-y-1.5">
        <UiLabel class="text-xs text-muted-foreground"> Operator </UiLabel>
        <UiSelect v-model="form.operator" :disabled="!form.field">
          <UiSelectTrigger>
            <UiSelectValue placeholder="Select…" />
          </UiSelectTrigger>
          <UiSelectContent>
            <UiSelectItem v-for="op in availableOperators" :key="op.value" :value="op.value">
              {{ op.label }}
            </UiSelectItem>
          </UiSelectContent>
        </UiSelect>
      </div>

      <!-- ④ Value — Dynamic input based on action type -->
      <div class="space-y-1.5">
        <UiLabel class="text-xs text-muted-foreground"> Value </UiLabel>

        <!-- "never" operator: no value needed -->
        <div
          v-if="form.operator === 'never'"
          class="flex items-center h-9 px-3 rounded-md border border-input bg-muted/50"
        >
          <span class="text-xs text-muted-foreground italic">No value required</span>
        </div>

        <!-- Loading state -->
        <div
          v-else-if="valueLoading"
          class="flex items-center justify-center h-9 rounded-md border border-input bg-background px-3"
        >
          <span class="text-xs text-muted-foreground animate-pulse">Loading…</span>
        </div>

        <!-- Boolean toggle (monitored, is_requested) -->
        <template v-else-if="valueInputMode === 'boolean'">
          <div class="flex items-center gap-3 h-9">
            <UiSwitch
              :model-value="form.value === 'true'"
              :disabled="!form.operator"
              aria-label="Rule value toggle"
              @update:model-value="(v: boolean) => (form.value = String(v))"
            />
            <span class="text-sm text-muted-foreground">{{
              form.value === 'true' ? 'Yes' : 'No'
            }}</span>
          </div>
        </template>

        <!-- Closed-set select (quality profiles, languages, show status, media type) -->
        <template v-else-if="valueInputMode === 'closed'">
          <UiSelect v-model="form.value" :disabled="!form.operator">
            <UiSelectTrigger>
              <UiSelectValue placeholder="Select…" />
            </UiSelectTrigger>
            <UiSelectContent>
              <UiSelectItem v-for="opt in closedOptions" :key="opt.value" :value="opt.value">
                {{ opt.label }}
              </UiSelectItem>
            </UiSelectContent>
          </UiSelect>
        </template>

        <!-- Free-text input (numbers and text) with optional suffix and suggestions -->
        <template v-else>
          <!-- Combobox mode: free-text input with CreatableCombobox for suggestion + custom values -->
          <div v-if="hasSuggestions" class="flex items-center gap-2">
            <CreatableCombobox
              v-model="form.value"
              :options="filteredSuggestions"
              :placeholder="freeInputPlaceholder"
              :disabled="!form.operator"
              class="flex-1"
            />
            <span
              v-if="freeInputSuffix"
              class="text-xs text-muted-foreground whitespace-nowrap shrink-0"
            >
              {{ freeInputSuffix }}
            </span>
          </div>
          <!-- Plain input mode: no suggestions available -->
          <div v-else class="flex items-center gap-2">
            <UiInput
              v-model="form.value"
              :disabled="!form.operator"
              :type="freeInputType"
              :placeholder="freeInputPlaceholder"
              class="flex-1"
            />
            <span
              v-if="freeInputSuffix"
              class="text-xs text-muted-foreground whitespace-nowrap shrink-0"
            >
              {{ freeInputSuffix }}
            </span>
          </div>
          <!-- Validation warning for size field: show GB equivalent -->
          <p
            v-if="form.field === 'sizebytes' && form.value"
            class="text-[11px] text-muted-foreground mt-1"
          >
            ≈ {{ (Number(form.value) / 1073741824).toFixed(2) }} GB
          </p>
          <!-- Non-blocking warning when free-text value doesn't match known options -->
          <p v-if="showFreeTextWarning" class="text-[11px] text-amber-500 mt-1">
            ⚠ Value not found in known options
          </p>
        </template>
      </div>

      <!-- ⑤ Effect -->
      <div class="space-y-1.5">
        <UiLabel class="text-xs text-muted-foreground"> Effect </UiLabel>
        <UiSelect v-model="form.effect" :disabled="!form.value && !valueNotRequired">
          <UiSelectTrigger>
            <UiSelectValue placeholder="Select effect…" />
          </UiSelectTrigger>
          <UiSelectContent>
            <UiSelectItem v-for="eff in effectOptions" :key="eff.value" :value="eff.value">
              <span class="inline-flex items-center gap-2">
                <span class="text-sm shrink-0">{{ eff.icon }}</span>
                {{ eff.label }}
              </span>
            </UiSelectItem>
          </UiSelectContent>
        </UiSelect>
      </div>
    </div>

    <div class="flex items-center gap-3">
      <UiButton size="sm" :disabled="!isFormValid || isInitializing" @click="submitRule">
        {{ isEditMode ? $t('rules.updateRule') : $t('rules.saveRule') }}
      </UiButton>
      <UiButton variant="ghost" size="sm" @click="$emit('cancel')">
        {{ $t('common.cancel') }}
      </UiButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import { LoaderCircleIcon } from 'lucide-vue-next';
import { CreatableCombobox } from '~/components/ui/creatable-combobox';

interface Integration {
  id: number;
  type: string;
  name: string;
  enabled: boolean;
}

interface FieldDef {
  field: string;
  label: string;
  type: string;
  operators: string[];
}

interface NameValue {
  value: string;
  label: string;
}

interface RuleValuesResponse {
  type: 'closed' | 'combobox' | 'free';
  options?: NameValue[];
  suggestions?: NameValue[];
  inputType?: string;
  placeholder?: string;
  suffix?: string;
}

interface RuleContextResponse {
  rule: {
    id: number;
    integrationId: number;
    field: string;
    operator: string;
    value: string;
    effect: string;
  };
  fields: FieldDef[];
  values: RuleValuesResponse | null;
}

const props = defineProps<{
  integrations: Integration[];
  /** When provided, the builder enters edit mode and prepopulates the form. */
  initialRule?: {
    id: number;
    integrationId: number;
    field: string;
    operator: string;
    value: string;
    effect: string;
  };
}>();

const isEditMode = computed(() => !!props.initialRule);

const emit = defineEmits<{
  (
    e: 'save',
    rule: { integrationId: number; field: string; operator: string; value: string; effect: string },
  ): void;
  (
    e: 'update',
    id: number,
    rule: { integrationId: number; field: string; operator: string; value: string; effect: string },
  ): void;
  (e: 'cancel'): void;
}>();

const api = useApi();

// Only show *arr integrations (not enrichment services like Plex/Tautulli/Seerr)
const arrTypes = ['sonarr', 'radarr', 'lidarr', 'readarr'];
const arrIntegrations = computed(() =>
  props.integrations.filter((i) => i.enabled && arrTypes.includes(i.type)),
);

function capitalize(str: string): string {
  if (!str) return '';
  return str.charAt(0).toUpperCase() + str.slice(1);
}

// Operator labels mapping
const operatorLabels: Record<string, string> = {
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

// Effect options with color coding
const effectOptions = [
  { value: 'always_keep', label: 'Always keep', colorClass: 'bg-emerald-500', icon: '🛡️' },
  { value: 'prefer_keep', label: 'Prefer to keep', colorClass: 'bg-teal-400', icon: '🟢' },
  { value: 'lean_keep', label: 'Lean toward keeping', colorClass: 'bg-sky-400', icon: '🔵' },
  { value: 'lean_remove', label: 'Lean toward removing', colorClass: 'bg-amber-400', icon: '🟡' },
  { value: 'prefer_remove', label: 'Prefer to remove', colorClass: 'bg-orange-500', icon: '🟠' },
  { value: 'always_remove', label: 'Always remove', colorClass: 'bg-red-500', icon: '🔴' },
];

// Form state
const form = reactive({
  integrationId: '',
  field: '',
  operator: '',
  value: '',
  effect: '',
});

// Dynamic fields fetched based on selected service type
const fields = ref<FieldDef[]>([]);

// Rule values response from API
const ruleValues = ref<RuleValuesResponse | null>(null);
const valueLoading = ref(false);

// Edit mode initialization guard — prevents cascade functions from
// clearing downstream values during sequential prepopulation.
const isInitializing = ref(false);

// Combobox search term (synced with the combobox input)
const comboboxSearch = ref('');

// Get the service type from the selected integration
const selectedServiceType = computed(() => {
  if (!form.integrationId) return '';
  const svc = arrIntegrations.value.find((i) => String(i.id) === form.integrationId);
  return svc?.type ?? '';
});

// Get the selected field definition
const selectedField = computed(() => fields.value.find((f) => f.field === form.field));

const selectedFieldType = computed(() => selectedField.value?.type ?? 'string');

// Available operators for the selected field with friendly labels
const availableOperators = computed(() => {
  if (!selectedField.value) return [];
  return selectedField.value.operators.map((op) => ({
    value: op,
    label: operatorLabels[op] ?? op,
  }));
});

// Determine value input mode based on API response
const valueInputMode = computed((): 'boolean' | 'closed' | 'combobox' | 'free' => {
  // Boolean fields always use toggle
  if (selectedFieldType.value === 'boolean') return 'boolean';
  // After API response, use the type from the response
  if (ruleValues.value) {
    if (ruleValues.value.type === 'closed') return 'closed';
    if (ruleValues.value.type === 'combobox') return 'free';
    return 'free';
  }
  // Default to free input
  return 'free';
});

// Closed-set options from API
const closedOptions = computed((): NameValue[] => {
  return ruleValues.value?.options ?? [];
});

// Whether the current field has suggestions available (determines combobox vs plain input)
const hasSuggestions = computed((): boolean => {
  if (!ruleValues.value) return false;
  return (ruleValues.value.suggestions ?? []).length > 0;
});

// Filtered suggestions for the combobox dropdown (from API suggestions)
const filteredSuggestions = computed((): NameValue[] => {
  if (!ruleValues.value) return [];
  const all = ruleValues.value.suggestions ?? [];
  if (all.length === 0) return [];
  if (!comboboxSearch.value) return all;
  const needle = comboboxSearch.value.toLowerCase();
  return all.filter(
    (s) => s.label.toLowerCase().includes(needle) || s.value.toLowerCase().includes(needle),
  );
});

// Free input metadata from API
const freeInputType = computed(() => {
  if (ruleValues.value?.inputType === 'number') return 'number';
  if (selectedFieldType.value === 'number') return 'number';
  return 'text';
});

const freeInputPlaceholder = computed(() => {
  if (ruleValues.value?.placeholder) return ruleValues.value.placeholder;
  if (!form.field) return 'Value';
  switch (form.field) {
    case 'title':
      return 'e.g., Breaking Bad';
    case 'quality':
      return 'e.g., HD-1080p';
    case 'tag':
      return 'e.g., anime';
    case 'genre':
      return 'e.g., Action';
    case 'rating':
      return 'e.g., 7.5';
    case 'sizebytes':
      return 'e.g., 5368709120';
    case 'timeinlibrary':
      return 'e.g., 30';
    case 'year':
      return 'e.g., 2020';
    case 'language':
      return 'e.g., English';
    case 'seasoncount':
      return 'e.g., 5';
    case 'episodecount':
      return 'e.g., 100';
    case 'playcount':
      return 'e.g., 0';
    case 'requestcount':
      return 'e.g., 3';
    case 'lastplayed':
      return 'e.g., 30';
    case 'requestedby':
      return 'e.g., john';
    default:
      return 'Value';
  }
});

const freeInputSuffix = computed(() => {
  if (ruleValues.value?.suffix) return ruleValues.value.suffix;
  if (form.field === 'timeinlibrary') return 'days';
  if (form.field === 'lastplayed' && ['in_last', 'over_ago'].includes(form.operator)) return 'days';
  if (form.field === 'sizebytes') return 'bytes (≈ GB)';
  return '';
});

// Show a non-blocking warning when free-text input doesn't match any known option/suggestion
const showFreeTextWarning = computed(() => {
  if (valueInputMode.value !== 'free') return false;
  if (!form.value) return false;
  if (!ruleValues.value) return false;

  const knownOptions = [
    ...(ruleValues.value.options ?? []),
    ...(ruleValues.value.suggestions ?? []),
  ];
  if (knownOptions.length === 0) return false;

  const lower = form.value.toLowerCase();
  return !knownOptions.some((opt) => opt.value.toLowerCase() === lower);
});

// "never" operator needs no value — the backend ignores the value field for it
const valueNotRequired = computed(() => form.operator === 'never');

const isFormValid = computed(
  () =>
    form.integrationId !== '' &&
    form.field !== '' &&
    form.operator !== '' &&
    (form.value !== '' || valueNotRequired.value) &&
    form.effect !== '',
);

// Cascade: when service changes, reset downstream fields and fetch field definitions
async function onServiceChange() {
  if (isInitializing.value) return; // Guard: skip during edit initialization
  form.field = '';
  form.operator = '';
  form.value = '';
  form.effect = '';
  ruleValues.value = null;

  if (!form.integrationId) {
    fields.value = [];
    return;
  }

  try {
    const serviceType = selectedServiceType.value;
    fields.value = (await api(`/api/v1/rule-fields?service_type=${serviceType}`)) as FieldDef[];
  } catch {
    fields.value = [];
  }
}

// Cascade: when action (field) changes, reset operator and value, fetch value options
async function onFieldChange() {
  if (isInitializing.value) return; // Guard: skip during edit initialization
  form.operator = '';
  form.value = '';
  form.effect = '';
  ruleValues.value = null;
  comboboxSearch.value = '';

  if (!form.field || !form.integrationId) return;

  // For boolean fields, auto-set the operator to '==' and value to 'true'
  if (selectedFieldType.value === 'boolean') {
    form.operator = '==';
    form.value = 'true';
  }

  // Fetch value options from the API
  valueLoading.value = true;
  try {
    const data = (await api(
      `/api/v1/rule-values?integration_id=${form.integrationId}&action=${form.field}`,
    )) as RuleValuesResponse;
    ruleValues.value = data;
  } catch {
    ruleValues.value = null;
  } finally {
    valueLoading.value = false;
  }
}

// Note: comboboxSearch is still used for filteredSuggestions computed.
// CreatableCombobox manages the interaction internally via v-model.

// Initialize the form for editing an existing rule. Uses the combined
// /context endpoint to fetch fields + values in a single round-trip,
// then populates all form fields with cascade guards active.
async function initializeForEdit() {
  if (!props.initialRule) return;

  isInitializing.value = true;
  try {
    const ctx = (await api(
      `/api/v1/custom-rules/${props.initialRule.id}/context`,
    )) as RuleContextResponse;

    // Populate field definitions and value options directly from context
    if (ctx.fields) {
      fields.value = ctx.fields;
    }
    if (ctx.values) {
      ruleValues.value = ctx.values;
    }

    // Set all form fields at once — cascade guards prevent downstream resets
    form.integrationId = String(props.initialRule.integrationId);
    form.field = props.initialRule.field;
    form.operator = props.initialRule.operator;
    form.value = props.initialRule.value;
    form.effect = props.initialRule.effect;
  } catch (err) {
    console.warn('[RuleBuilder] Failed to initialize for edit:', err);
  } finally {
    isInitializing.value = false;
  }
}

onMounted(() => {
  if (props.initialRule) {
    initializeForEdit();
  }
});

function submitRule() {
  if (!isFormValid.value) return;
  const ruleData = {
    integrationId: Number(form.integrationId),
    field: form.field,
    operator: form.operator,
    value: valueNotRequired.value ? 'true' : String(form.value),
    effect: form.effect,
  };

  if (isEditMode.value && props.initialRule) {
    emit('update', props.initialRule.id, ruleData);
  } else {
    emit('save', ruleData);
  }

  // Only reset form on create (edit mode will unmount when returning to list)
  if (!isEditMode.value) {
    form.integrationId = '';
    form.field = '';
    form.operator = '';
    form.value = '';
    form.effect = '';
    fields.value = [];
    ruleValues.value = null;
    comboboxSearch.value = '';
  }
}
</script>
