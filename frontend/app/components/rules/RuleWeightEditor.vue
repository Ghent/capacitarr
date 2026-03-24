<template>
  <UiCard
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24 } }"
    class="mb-6"
  >
    <UiCardHeader>
      <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <UiCardTitle>{{ $t('rules.preferenceWeights') }}</UiCardTitle>
          <UiCardDescription>
            {{ $t('rules.preferenceWeightsDesc') }}
          </UiCardDescription>
        </div>
        <UiButton size="sm" @click="$emit('save')">
          {{ $t('rules.saveWeights') }}
        </UiButton>
      </div>
    </UiCardHeader>
    <UiCardContent>
      <!-- Preset Chips -->
      <div class="flex flex-wrap gap-2 mb-2">
        <UiButton
          v-for="preset in presets"
          :key="preset.name"
          :variant="isActivePreset(preset.values) ? 'default' : 'outline'"
          size="sm"
          class="rounded-full h-7 px-3 text-xs"
          @click="applyPreset(preset.values)"
        >
          {{ preset.name }}
        </UiButton>
      </div>

      <!-- Preset Description -->
      <Transition
        enter-active-class="transition-all duration-300 ease-out"
        leave-active-class="transition-all duration-200 ease-in"
        enter-from-class="opacity-0 -translate-y-1"
        enter-to-class="opacity-100 translate-y-0"
        leave-from-class="opacity-100 translate-y-0"
        leave-to-class="opacity-0 -translate-y-1"
        mode="out-in"
      >
        <p
          :key="activePresetDescription"
          class="text-xs text-muted-foreground/70 mb-6 leading-relaxed"
        >
          {{ activePresetDescription }}
        </p>
      </Transition>

      <!-- Two-Column Slider Grid — dynamically rendered from API response -->
      <div class="grid grid-cols-1 md:grid-cols-2 gap-x-8 gap-y-5">
        <div v-for="factor in factors" :key="factor.key" class="space-y-1.5">
          <div class="flex justify-between text-sm">
            <span class="inline-flex items-center gap-1.5 font-medium text-foreground">
              {{ factor.name }}
              <UiTooltipProvider v-if="factor.key === 'rating'">
                <UiTooltip>
                  <UiTooltipTrigger as-child>
                    <InfoIcon class="w-3.5 h-3.5 text-muted-foreground cursor-help" />
                  </UiTooltipTrigger>
                  <UiTooltipContent side="top" class="max-w-xs text-xs">
                    {{ $t('rules.ratingTooltip') }}
                  </UiTooltipContent>
                </UiTooltip>
              </UiTooltipProvider>
              <UiTooltipProvider v-if="factor.integrationError">
                <UiTooltip>
                  <UiTooltipTrigger as-child>
                    <AlertTriangleIcon class="w-3.5 h-3.5 text-amber-500 cursor-help" />
                  </UiTooltipTrigger>
                  <UiTooltipContent side="top" class="max-w-xs text-xs">
                    {{ $t('rules.integrationErrorTooltip') }}
                  </UiTooltipContent>
                </UiTooltip>
              </UiTooltipProvider>
            </span>
            <span class="text-muted-foreground font-mono tabular-nums"
              >{{ factor.weight }} / 10</span
            >
          </div>
          <UiSlider
            :model-value="[factor.weight]"
            :min="0"
            :max="10"
            :step="1"
            class="w-full"
            @update:model-value="
              (v: number[] | undefined) => {
                if (v && v[0] != null) $emit('update:weight', factor.key, v[0]);
              }
            "
          />
          <p class="text-xs text-muted-foreground">
            {{ factor.description }}
          </p>
        </div>
      </div>
    </UiCardContent>
  </UiCard>
</template>

<script setup lang="ts">
import { AlertTriangleIcon, InfoIcon } from 'lucide-vue-next';
import type { ScoringFactorWeight } from '~/types/api';

const props = defineProps<{
  factors: ScoringFactorWeight[];
}>();

const emit = defineEmits<{
  save: [];
  'update:weight': [key: string, value: number];
  'apply-preset': [values: Record<string, number>];
}>();

// Presets use factor keys as property names so they work with any factor set.
const presets = [
  {
    name: 'Balanced',
    values: {
      watch_history: 8,
      last_watched: 7,
      file_size: 6,
      rating: 5,
      time_in_library: 4,
      series_status: 3,
      request_popularity: 2,
    },
  },
  {
    name: 'Space Saver',
    values: {
      watch_history: 3,
      last_watched: 3,
      file_size: 10,
      rating: 2,
      time_in_library: 8,
      series_status: 5,
      request_popularity: 1,
    },
  },
  {
    name: 'Hoarder',
    values: {
      watch_history: 10,
      last_watched: 10,
      file_size: 2,
      rating: 8,
      time_in_library: 2,
      series_status: 2,
      request_popularity: 5,
    },
  },
  {
    name: 'Watch-Based',
    values: {
      watch_history: 10,
      last_watched: 9,
      file_size: 4,
      rating: 3,
      time_in_library: 3,
      series_status: 5,
      request_popularity: 3,
    },
  },
];

const presetDescriptions: Record<string, string> = {
  Balanced: 'A general-purpose profile that weighs all factors evenly. Good starting point.',
  'Space Saver': 'Prioritizes freeing disk space. Targets large, old media with low ratings.',
  Hoarder:
    "Strongly resists deletion. Only removes media that's never been watched and poorly rated.",
  'Watch-Based': 'Focuses on watch history. Unwatched and stale media is removed first.',
};

function isActivePreset(values: Record<string, number>): boolean {
  // Only compare keys that exist in both the preset AND the current factor set.
  // Presets may include keys for inapplicable factors (e.g. series_status without
  // Sonarr) — those are ignored when determining if the preset is active.
  return Object.entries(values).every(([key, val]) => {
    const factor = props.factors.find((f) => f.key === key);
    return factor ? factor.weight === val : true;
  });
}

const activePresetDescription = computed(() => {
  const active = presets.find((p) => isActivePreset(p.values));
  return active
    ? (presetDescriptions[active.name] ?? '')
    : 'Custom configuration — adjust sliders to fine-tune scoring.';
});

function applyPreset(values: Record<string, number>) {
  // Only emit keys that exist in the current factor set. Preset values for
  // inapplicable factors (e.g. request_popularity without Seerr) are filtered
  // out so the API doesn't receive keys for hidden factors.
  const filtered = Object.fromEntries(
    Object.entries(values).filter(([key]) => props.factors.some((f) => f.key === key)),
  );
  emit('apply-preset', filtered);
}
</script>
