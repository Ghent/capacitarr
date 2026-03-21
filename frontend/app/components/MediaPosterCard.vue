<script setup lang="ts">
import {
  Film,
  Tv,
  Music,
  BookOpen,
  CheckSquare,
  Square,
  ShieldCheck,
  ClockIcon,
  CheckIcon,
  ZapIcon,
  LoaderCircleIcon,
} from 'lucide-vue-next';

const { gridItem } = useMotionPresets();

const props = defineProps<{
  title: string;
  posterUrl?: string;
  year?: number;
  mediaType: string;
  score?: number;
  sizeBytes?: number;
  isProtected?: boolean;
  isFlagged?: boolean;
  selectable?: boolean;
  selected?: boolean;
  seasonCount?: number;
  queueStatus?: 'pending' | 'approved' | 'user_initiated' | 'deleting';
  /** Animation stagger delay in ms (e.g., index * 30). Defaults to 0. */
  animationDelay?: number;
}>();

const motionProps = computed(() => gridItem(props.animationDelay ?? 0));

defineEmits<{
  click: [];
  select: [];
}>();

const imageError = ref(false);
const imageLoaded = ref(false);

/** Reset image state when posterUrl changes */
watch(
  () => props.posterUrl,
  () => {
    imageError.value = false;
    imageLoaded.value = false;
  },
);

/** Per-media-type object-fit class */
const objectFitClass = computed(() => {
  switch (props.mediaType) {
    case 'artist':
      return 'object-contain';
    default:
      return 'object-cover';
  }
});

/** Media type icon component */
const mediaTypeIcon = computed(() => {
  switch (props.mediaType) {
    case 'movie':
      return Film;
    case 'show':
    case 'season':
      return Tv;
    case 'artist':
      return Music;
    case 'book':
      return BookOpen;
    default:
      return Film;
  }
});

/** Media-type accent color for fallback gradient */
const accentClass = computed(() => {
  switch (props.mediaType) {
    case 'movie':
      return 'from-blue-900/40 to-blue-950/80';
    case 'show':
    case 'season':
      return 'from-purple-900/40 to-purple-950/80';
    case 'artist':
      return 'from-emerald-900/40 to-emerald-950/80';
    case 'book':
      return 'from-amber-900/40 to-amber-950/80';
    default:
      return 'from-slate-900/40 to-slate-950/80';
  }
});

/** Score color badge class */
const scoreBadgeClass = computed(() => {
  if (props.score == null) return 'bg-muted text-muted-foreground';
  if (props.score >= 0.7) return 'bg-red-500/90 text-white';
  if (props.score >= 0.4) return 'bg-amber-500/90 text-white';
  return 'bg-emerald-500/90 text-white';
});

const showImage = computed(() => props.posterUrl && !imageError.value);
const showFallback = computed(() => !props.posterUrl || imageError.value || !imageLoaded.value);

/** Human-readable label for queue status banner */
const queueStatusLabel = computed(() => {
  switch (props.queueStatus) {
    case 'pending':
      return 'Pending';
    case 'approved':
      return 'Approved';
    case 'user_initiated':
      return 'Delete';
    case 'deleting':
      return 'Deleting…';
    default:
      return '';
  }
});
</script>

<template>
  <div
    v-motion
    v-bind="motionProps"
    class="group relative aspect-[2/3] overflow-hidden rounded-lg border cursor-pointer transition-all hover:ring-2 hover:ring-primary/50"
    :class="{
      'ring-2 ring-emerald-500/50': isProtected,
      'opacity-40': isFlagged && !isProtected,
      'ring-2 ring-primary bg-primary/5': selected,
    }"
    @click="$emit('click')"
  >
    <!-- Poster image (absolute positioned — no layout shift) -->
    <img
      v-if="showImage"
      :src="posterUrl"
      :alt="title"
      loading="lazy"
      decoding="async"
      :class="[objectFitClass, { 'opacity-100': imageLoaded, 'opacity-0': !imageLoaded }]"
      class="absolute inset-0 h-full w-full transition-opacity duration-300"
      @error="imageError = true"
      @load="imageLoaded = true"
    />

    <!-- Fallback / loading shimmer (also absolute — same space) -->
    <div
      v-if="showFallback"
      class="absolute inset-0 flex flex-col items-center justify-center bg-gradient-to-b p-3"
      :class="[accentClass, { 'animate-pulse': !!posterUrl && !imageError && !imageLoaded }]"
    >
      <!-- Album art letterbox background for artist type -->
      <div v-if="mediaType === 'artist'" class="absolute inset-0 bg-muted/30" />
      <component :is="mediaTypeIcon" class="relative z-10 w-10 h-10 text-white/60 mb-2" />
      <span class="relative z-10 text-xs text-white/70 text-center line-clamp-2 font-medium">
        {{ title }}
      </span>
      <span v-if="year" class="relative z-10 text-[10px] text-white/50 mt-0.5">
        {{ year }}
      </span>
    </div>

    <!-- Bottom gradient overlay for text readability (over real posters) -->
    <div
      v-if="showImage && imageLoaded"
      class="absolute inset-x-0 bottom-0 h-1/3 bg-gradient-to-t from-black/80 to-transparent"
    />

    <!-- Title + Year (bottom) -->
    <div v-if="showImage && imageLoaded" class="absolute inset-x-0 bottom-0 p-2">
      <p class="text-xs font-medium text-white line-clamp-2 leading-tight">
        {{ title }}
      </p>
      <p v-if="year" class="text-[10px] text-white/70 mt-0.5">
        {{ year }}
      </p>
    </div>

    <!-- Score badge or Protected shield (top-right) -->
    <div
      v-if="isProtected"
      class="absolute top-1.5 right-1.5 rounded-full bg-emerald-500/90 px-1.5 py-0.5"
    >
      <ShieldCheck class="w-3 h-3 text-white" />
    </div>
    <div
      v-else-if="score != null"
      class="absolute top-1.5 right-1.5 rounded-full px-1.5 py-0.5 text-[10px] font-bold tabular-nums"
      :class="scoreBadgeClass"
    >
      {{ score.toFixed(2) }}
    </div>

    <!-- Top-left: Selection checkbox (when selectable) or Media type chip -->
    <button
      v-if="selectable"
      class="absolute top-1.5 left-1.5 z-20 rounded bg-black/40 backdrop-blur-sm p-0.5 transition-colors hover:bg-black/60"
      :class="{ 'bg-primary/80 hover:bg-primary/90': selected }"
      @click.stop="$emit('select')"
    >
      <component :is="selected ? CheckSquare : Square" class="w-4 h-4 text-white" />
    </button>
    <div
      v-else
      class="absolute top-1.5 left-1.5 rounded-full bg-black/50 backdrop-blur-sm px-2 py-0.5 text-[10px] font-medium text-white/80 capitalize"
    >
      {{ mediaType }}
    </div>

    <!-- Bottom-right: Seasons badge -->
    <div
      v-if="seasonCount && seasonCount > 0"
      class="absolute bottom-1.5 right-1.5 rounded-full bg-black/50 backdrop-blur-sm px-1.5 py-0.5 text-[10px] font-medium text-white/80"
    >
      ×{{ seasonCount }}
    </div>

    <!-- Queue status banner (above title gradient) -->
    <div
      v-if="queueStatus"
      class="absolute inset-x-0 bottom-10 z-10 flex items-center justify-center gap-1 py-1 text-[10px] font-semibold uppercase tracking-wider backdrop-blur-sm"
      :class="{
        'bg-amber-500/70 text-white': queueStatus === 'pending',
        'bg-emerald-500/70 text-white': queueStatus === 'approved',
        'bg-red-500/70 text-white': queueStatus === 'user_initiated',
        'bg-red-500/70 text-white animate-pulse': queueStatus === 'deleting',
      }"
    >
      <ClockIcon v-if="queueStatus === 'pending'" class="w-3 h-3" />
      <CheckIcon v-else-if="queueStatus === 'approved'" class="w-3 h-3" />
      <ZapIcon v-else-if="queueStatus === 'user_initiated'" class="w-3 h-3" />
      <LoaderCircleIcon v-else-if="queueStatus === 'deleting'" class="w-3 h-3 animate-spin" />
      <span>{{ queueStatusLabel }}</span>
    </div>
  </div>
</template>
