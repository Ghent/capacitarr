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

      <!-- Two-Column Slider Grid -->
      <div class="grid grid-cols-1 md:grid-cols-2 gap-x-8 gap-y-5">
        <div v-for="slider in sliders" :key="slider.key" class="space-y-1.5">
          <div class="flex justify-between text-sm">
            <span class="inline-flex items-center gap-1.5 font-medium text-foreground">
              {{ slider.label }}
              <UiTooltipProvider v-if="slider.tooltip">
                <UiTooltip>
                  <UiTooltipTrigger as-child>
                    <InfoIcon class="w-3.5 h-3.5 text-muted-foreground cursor-help" />
                  </UiTooltipTrigger>
                  <UiTooltipContent side="top" class="max-w-xs text-xs">
                    {{ slider.tooltip }}
                  </UiTooltipContent>
                </UiTooltip>
              </UiTooltipProvider>
            </span>
            <span class="text-muted-foreground font-mono tabular-nums"
              >{{ preferences[slider.key as keyof WeightKeys] }} / 10</span
            >
          </div>
          <UiSlider
            :model-value="[Number(preferences[slider.key as keyof WeightKeys])]"
            :min="0"
            :max="10"
            :step="1"
            class="w-full"
            @update:model-value="
              (v: number[] | undefined) => {
                if (v && v[0] != null) $emit('update:preference', slider.key, v[0]);
              }
            "
          />
          <p class="text-xs text-muted-foreground">
            {{ slider.description }}
          </p>
        </div>
      </div>
    </UiCardContent>
  </UiCard>
</template>

<script setup lang="ts">
import { InfoIcon } from 'lucide-vue-next';

const { t } = useI18n();

export interface WeightKeys {
  watchHistoryWeight: number;
  lastWatchedWeight: number;
  fileSizeWeight: number;
  ratingWeight: number;
  timeInLibraryWeight: number;
  seriesStatusWeight: number;
}

const props = defineProps<{
  preferences: WeightKeys;
}>();

const emit = defineEmits<{
  save: [];
  'update:preference': [key: string, value: number];
  'apply-preset': [values: Record<string, number>];
}>();

const sliders = [
  {
    key: 'watchHistoryWeight',
    label: 'Watch History (Play Count)',
    description: 'Unwatched items score much higher.',
  },
  {
    key: 'lastWatchedWeight',
    label: 'Days Since Last Watched',
    description: 'Media not watched in a long time scores higher.',
  },
  {
    key: 'fileSizeWeight',
    label: 'File Size',
    description: 'Larger files score higher to free more space.',
  },
  {
    key: 'ratingWeight',
    label: 'Rating',
    description: 'Low-rated content scores higher for deletion.',
    tooltip: t('rules.ratingTooltip'),
  },
  {
    key: 'timeInLibraryWeight',
    label: 'Time in Library',
    description: 'Older content may be less valuable.',
  },
  {
    key: 'seriesStatusWeight',
    label: 'Series Status',
    description:
      'Ended or canceled shows score higher for removal since no new episodes are expected.',
  },
];

const presets = [
  {
    name: 'Balanced',
    values: {
      watchHistoryWeight: 8,
      lastWatchedWeight: 7,
      fileSizeWeight: 6,
      ratingWeight: 5,
      timeInLibraryWeight: 4,
      seriesStatusWeight: 3,
    },
  },
  {
    name: 'Space Saver',
    values: {
      watchHistoryWeight: 3,
      lastWatchedWeight: 3,
      fileSizeWeight: 10,
      ratingWeight: 2,
      timeInLibraryWeight: 8,
      seriesStatusWeight: 5,
    },
  },
  {
    name: 'Hoarder',
    values: {
      watchHistoryWeight: 10,
      lastWatchedWeight: 10,
      fileSizeWeight: 2,
      ratingWeight: 8,
      timeInLibraryWeight: 2,
      seriesStatusWeight: 2,
    },
  },
  {
    name: 'Watch-Based',
    values: {
      watchHistoryWeight: 10,
      lastWatchedWeight: 9,
      fileSizeWeight: 4,
      ratingWeight: 3,
      timeInLibraryWeight: 3,
      seriesStatusWeight: 5,
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
  return Object.entries(values).every(
    ([key, val]) => props.preferences[key as keyof WeightKeys] === val,
  );
}

const activePresetDescription = computed(() => {
  const active = presets.find((p) => isActivePreset(p.values));
  return active
    ? (presetDescriptions[active.name] ?? '')
    : 'Custom configuration — adjust sliders to fine-tune scoring.';
});

function applyPreset(values: Record<string, number>) {
  emit('apply-preset', values);
}
</script>
