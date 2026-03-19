<template>
  <div>
    <!-- Redirect notice: audit log is now under Library → History -->
    <div class="flex flex-col items-center justify-center py-20 text-muted-foreground">
      <ClockIcon class="w-10 h-10 mb-3" />
      <span class="text-sm font-medium mb-2">{{ $t('audit.movedToLibrary') }}</span>
      <NuxtLink to="/library?tab=history" class="text-primary hover:underline text-sm">
        {{ $t('audit.goToLibraryHistory') }}
      </NuxtLink>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ClockIcon } from 'lucide-vue-next';

// Auto-redirect to Library → History tab
const router = useRouter();
onMounted(() => {
  router.replace('/library?tab=history');
});
</script>
