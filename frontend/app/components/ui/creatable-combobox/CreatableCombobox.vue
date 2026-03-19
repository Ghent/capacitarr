<!--
  CreatableCombobox — a combobox that supports creating new values.

  Built on Reka-UI ComboboxRoot primitives (same foundation as shadcn-vue Combobox).
  Key features:
  - Synthetic "Create: {typed value}" option appears when the typed text doesn't
    match any existing option
  - modelValue is authoritative: the component displays the current value even if
    it isn't in the suggestions list (fixes prepopulation of saved values)
  - Emits 'update:modelValue' with the selected or created value
  - Emits 'create' when a new value is created (for parent components that need
    to persist the new value server-side)
-->
<script setup lang="ts">
import { ref, computed, watch } from 'vue';
import { cn } from '@/lib/utils';
import {
  ComboboxRoot,
  ComboboxAnchor,
  ComboboxInput,
  ComboboxContent,
  ComboboxItem,
  ComboboxItemIndicator,
  ComboboxEmpty,
  ComboboxViewport,
} from 'reka-ui';
import { CheckIcon, PlusCircleIcon } from 'lucide-vue-next';

export interface CreatableComboboxOption {
  label: string;
  value: string;
}

const props = withDefaults(
  defineProps<{
    /** Current selected value (v-model). Authoritative — always displayed even if not in options. */
    modelValue: string;
    /** Available options to choose from. */
    options: CreatableComboboxOption[];
    /** Placeholder text for the input. */
    placeholder?: string;
    /** Whether the combobox is disabled. */
    disabled?: boolean;
    /** Additional CSS classes for the root wrapper. */
    class?: string;
  }>(),
  {
    placeholder: 'Search or create...',
    disabled: false,
    class: '',
  },
);

const emit = defineEmits<{
  /** Emitted when the value changes (selected from list or created). */
  'update:modelValue': [value: string];
  /** Emitted when a new value is created (not in the existing options list). */
  create: [value: string];
}>();

// Internal search term — tracks what the user types
const searchTerm = ref('');

// Track whether the dropdown is open
const open = ref(false);

// Initialize search term from modelValue on mount
watch(
  () => props.modelValue,
  (val) => {
    if (!open.value) {
      searchTerm.value = val || '';
    }
  },
  { immediate: true },
);

// Filtered options based on search term
const filteredOptions = computed(() => {
  const term = searchTerm.value.toLowerCase().trim();
  if (!term) return props.options;
  return props.options.filter(
    (opt) => opt.label.toLowerCase().includes(term) || opt.value.toLowerCase().includes(term),
  );
});

// Whether to show the "Create: ..." synthetic option
const showCreateOption = computed(() => {
  const term = searchTerm.value.trim();
  if (!term) return false;
  // Don't show create if an exact match exists (case-insensitive)
  return !props.options.some(
    (opt) =>
      opt.value.toLowerCase() === term.toLowerCase() ||
      opt.label.toLowerCase() === term.toLowerCase(),
  );
});

// Handle selection from the dropdown (either existing option or create)
function handleSelect(value: string) {
  if (value === '__create__') {
    const newValue = searchTerm.value.trim();
    emit('update:modelValue', newValue);
    emit('create', newValue);
    searchTerm.value = newValue;
  } else {
    emit('update:modelValue', value);
    // Update search term to show the selected label
    const opt = props.options.find((o) => o.value === value);
    searchTerm.value = opt ? opt.label : value;
  }
  open.value = false;
}
</script>

<template>
  <ComboboxRoot
    v-model:open="open"
    :model-value="modelValue"
    :display-value="() => searchTerm"
    :filter-function="() => filteredOptions.map((o) => o.value)"
    data-slot="creatable-combobox"
    :class="cn('relative', props.class)"
    :disabled="disabled"
    @update:model-value="handleSelect"
  >
    <ComboboxAnchor
      :class="
        cn(
          'flex h-9 w-full items-center rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm transition-colors',
          'focus-within:ring-1 focus-within:ring-ring',
          disabled && 'cursor-not-allowed opacity-50',
        )
      "
    >
      <ComboboxInput
        :class="
          cn(
            'flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground',
            disabled && 'cursor-not-allowed',
          )
        "
        :placeholder="placeholder"
        :disabled="disabled"
        @update:model-value="searchTerm = $event"
      />
    </ComboboxAnchor>

    <ComboboxContent
      :class="
        cn(
          'absolute z-50 mt-1 max-h-[200px] w-full overflow-y-auto rounded-md border bg-popover text-popover-foreground shadow-md',
          'animate-in fade-in-0 zoom-in-95',
        )
      "
      position="popper"
      :side-offset="4"
    >
      <ComboboxViewport class="p-1">
        <!-- Existing options -->
        <ComboboxItem
          v-for="option in filteredOptions"
          :key="option.value"
          :value="option.value"
          :class="
            cn(
              'relative flex cursor-pointer select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none',
              'data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground',
              'data-[disabled]:pointer-events-none data-[disabled]:opacity-50',
            )
          "
        >
          <ComboboxItemIndicator class="mr-2 flex h-3.5 w-3.5 items-center justify-center">
            <CheckIcon class="h-4 w-4" />
          </ComboboxItemIndicator>
          <span>{{ option.label }}</span>
        </ComboboxItem>

        <!-- Synthetic "Create: ..." option -->
        <ComboboxItem
          v-if="showCreateOption"
          value="__create__"
          :class="
            cn(
              'relative flex cursor-pointer select-none items-center rounded-sm px-2 py-1.5 text-sm outline-none',
              'data-[highlighted]:bg-accent data-[highlighted]:text-accent-foreground',
              'text-primary font-medium',
            )
          "
        >
          <PlusCircleIcon class="mr-2 h-4 w-4" />
          <span>{{ $t('common.create') }}: {{ searchTerm.trim() }}</span>
        </ComboboxItem>

        <!-- Empty state (no options and create not shown) -->
        <ComboboxEmpty
          v-if="filteredOptions.length === 0 && !showCreateOption"
          class="px-2 py-1.5 text-sm text-muted-foreground"
        >
          {{ $t('common.noResults') }}
        </ComboboxEmpty>
      </ComboboxViewport>
    </ComboboxContent>
  </ComboboxRoot>
</template>
