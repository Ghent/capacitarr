<script setup lang="ts">
import { AlarmClockIcon, Undo2Icon } from 'lucide-vue-next';
import { formatBytes } from '~/utils/format';

const { t } = useI18n();
const { snoozedItems, fetchSnoozedItems, unsnooze } = useSnoozedItems();

// Fetch on mount
onMounted(() => {
  fetchSnoozedItems();
});

/**
 * Format a snooze expiration as a human-readable countdown.
 * e.g. "18h 30m", "2h 15m", "45m", "< 1m"
 */
function formatCountdown(snoozedUntil: string): string {
  const now = Date.now();
  const expiry = new Date(snoozedUntil).getTime();
  const diffMs = expiry - now;

  if (diffMs <= 0) return '< 1m';

  const totalMinutes = Math.floor(diffMs / 60000);
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;

  if (hours > 0) {
    return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`;
  }
  return minutes > 0 ? `${minutes}m` : '< 1m';
}
</script>

<template>
  <UiCard
    v-if="snoozedItems.length > 0"
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24 } }"
    class="mb-6"
  >
    <UiCardHeader>
      <div class="flex items-center justify-between">
        <div>
          <UiCardTitle class="flex items-center gap-2">
            <AlarmClockIcon class="w-4.5 h-4.5" />
            {{ t('snoozed.title') }}
          </UiCardTitle>
          <UiCardDescription class="mt-1">
            {{ t('snoozed.subtitle') }}
          </UiCardDescription>
        </div>
        <UiBadge variant="secondary" class="text-xs">
          {{ t('snoozed.count', { count: snoozedItems.length }) }}
        </UiBadge>
      </div>
    </UiCardHeader>
    <UiCardContent>
      <div class="space-y-1.5">
        <div
          v-for="item in snoozedItems"
          :key="item.id"
          v-motion
          :initial="{ opacity: 0, x: -8 }"
          :enter="{
            opacity: 1,
            x: 0,
            transition: { type: 'spring', stiffness: 260, damping: 24 },
          }"
          :leave="{ opacity: 0, x: 8 }"
          class="flex items-center gap-3 rounded-lg border border-border bg-muted/30 px-3 py-2"
        >
          <div class="flex-1 min-w-0">
            <span class="text-sm font-medium truncate block">{{ item.mediaName }}</span>
            <span class="text-xs text-muted-foreground">
              {{ item.mediaType }} · {{ formatBytes(item.sizeBytes) }}
              <span class="ml-1 text-amber-500 dark:text-amber-400">
                · {{ t('snoozed.expiresIn', { time: formatCountdown(item.snoozedUntil) }) }}
              </span>
            </span>
          </div>
          <UiButton
            variant="ghost"
            size="sm"
            class="h-7 p-0 px-2 text-muted-foreground hover:text-foreground shrink-0"
            :aria-label="t('snoozed.unsnooze')"
            :title="t('snoozed.unsnooze')"
            @click="unsnooze(item.id)"
          >
            <Undo2Icon class="h-3.5 w-3.5 mr-1" />
            <span class="text-xs">{{ t('snoozed.unsnooze') }}</span>
          </UiButton>
        </div>
      </div>
    </UiCardContent>
  </UiCard>
</template>
