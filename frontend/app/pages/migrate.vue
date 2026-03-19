<template>
  <div class="flex items-center justify-center min-h-[calc(100vh-100px)]">
    <div
      v-if="!isLoading"
      v-motion
      :initial="{ opacity: 0, scale: 0.96, y: 10 }"
      :enter="{
        opacity: 1,
        scale: 1,
        y: 0,
        transition: { type: 'spring', stiffness: 300, damping: 25 },
      }"
      class="w-full max-w-lg"
    >
      <!-- Migration Available -->
      <UiCard v-if="migrationAvailable && !migrationComplete" data-slot="migration-card">
        <UiCardHeader class="text-center">
          <div
            data-slot="migration-icon"
            class="w-14 h-14 rounded-2xl bg-primary flex items-center justify-center mx-auto mb-5"
          >
            <component :is="ArrowUpCircleIcon" class="w-7 h-7 text-primary-foreground" />
          </div>
          <UiCardTitle class="text-2xl">
            {{ $t('migration.title') }}
          </UiCardTitle>
          <UiCardDescription>
            {{ $t('migration.subtitle') }}
          </UiCardDescription>
        </UiCardHeader>
        <UiCardContent class="space-y-4">
          <p class="text-sm text-muted-foreground">
            {{ $t('migration.description') }}
          </p>

          <UiAlert>
            <component :is="InfoIcon" class="h-4 w-4" />
            <UiAlertTitle>{{ $t('migration.whatMigrates') }}</UiAlertTitle>
            <UiAlertDescription>
              <ul class="list-disc list-inside mt-1 space-y-0.5 text-xs">
                <li>{{ $t('migration.migratesIntegrations') }}</li>
                <li>{{ $t('migration.migratesRules') }}</li>
                <li>{{ $t('migration.migratesPreferences') }}</li>
                <li>{{ $t('migration.migratesNotifications') }}</li>
                <li>{{ $t('migration.migratesAuth') }}</li>
              </ul>
            </UiAlertDescription>
          </UiAlert>

          <p class="text-xs text-muted-foreground">
            {{ $t('migration.transientNote') }}
          </p>

          <!-- Error message -->
          <div
            v-if="errorMsg"
            v-motion
            :initial="{ opacity: 0, y: -4 }"
            :enter="{ opacity: 1, y: 0 }"
            class="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive"
          >
            {{ errorMsg }}
          </div>

          <div class="flex flex-col gap-3 pt-2">
            <UiButton
              :disabled="executing"
              class="w-full shadow-lg shadow-primary/25 hover:shadow-primary/40"
              @click="executeMigration"
            >
              <span v-if="executing" class="flex items-center justify-center gap-2">
                <component :is="LoaderCircleIcon" class="w-4 h-4 animate-spin" />
                {{ $t('migration.importing') }}
              </span>
              <span v-else>{{ $t('migration.importButton') }}</span>
            </UiButton>

            <UiButton variant="outline" :disabled="executing" class="w-full" @click="startFresh">
              {{ $t('migration.startFresh') }}
            </UiButton>
          </div>

          <p class="text-xs text-muted-foreground text-center">
            {{ $t('migration.cliNote') }}
          </p>
        </UiCardContent>
      </UiCard>

      <!-- Migration Complete -->
      <UiCard v-if="migrationComplete" data-slot="migration-complete-card">
        <UiCardHeader class="text-center">
          <div
            data-slot="migration-success-icon"
            class="w-14 h-14 rounded-2xl bg-green-500 flex items-center justify-center mx-auto mb-5"
          >
            <component :is="CheckCircle2Icon" class="w-7 h-7 text-white" />
          </div>
          <UiCardTitle class="text-2xl">
            {{ $t('migration.completeTitle') }}
          </UiCardTitle>
          <UiCardDescription>
            {{ $t('migration.completeSubtitle') }}
          </UiCardDescription>
        </UiCardHeader>
        <UiCardContent class="space-y-4">
          <div v-if="migrationResult" class="space-y-2 text-sm">
            <div class="flex justify-between">
              <span class="text-muted-foreground">{{ $t('migration.resultIntegrations') }}</span>
              <span class="font-medium">{{ migrationResult.integrationsImported }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-muted-foreground">{{ $t('migration.resultRules') }}</span>
              <span class="font-medium">{{ migrationResult.rulesImported }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-muted-foreground">{{ $t('migration.resultPreferences') }}</span>
              <span class="font-medium">{{ migrationResult.preferencesImported ? '✓' : '—' }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-muted-foreground">{{ $t('migration.resultNotifications') }}</span>
              <span class="font-medium">{{ migrationResult.notificationsImported }}</span>
            </div>
            <div class="flex justify-between">
              <span class="text-muted-foreground">{{ $t('migration.resultAuth') }}</span>
              <span class="font-medium">{{ migrationResult.authImported ? '✓' : '—' }}</span>
            </div>
          </div>

          <UiButton class="w-full mt-4" @click="goToLogin">
            {{ $t('migration.continueToLogin') }}
          </UiButton>
        </UiCardContent>
      </UiCard>

      <!-- No migration available (shouldn't normally appear — redirect) -->
      <UiCard
        v-if="!migrationAvailable && !migrationComplete && !isLoading"
        data-slot="no-migration-card"
      >
        <UiCardHeader class="text-center">
          <UiCardTitle>{{ $t('migration.notAvailableTitle') }}</UiCardTitle>
          <UiCardDescription>{{ $t('migration.notAvailableDesc') }}</UiCardDescription>
        </UiCardHeader>
        <UiCardContent>
          <UiButton class="w-full" @click="goToLogin">
            {{ $t('migration.continueToLogin') }}
          </UiButton>
        </UiCardContent>
      </UiCard>

      <!-- Subtle branding footer -->
      <p class="text-center text-xs text-muted-foreground mt-4">
        {{ $t('login.branding') }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ArrowUpCircleIcon, InfoIcon, LoaderCircleIcon, CheckCircle2Icon } from 'lucide-vue-next';
import { ofetch } from 'ofetch';

interface MigrationResultData {
  success: boolean;
  integrationsImported: number;
  rulesImported: number;
  preferencesImported: boolean;
  notificationsImported: number;
  authImported: boolean;
  error?: string;
}

const config = useRuntimeConfig();
const router = useRouter();

const isLoading = ref(true);
const migrationAvailable = ref(false);
const migrationComplete = ref(false);
const executing = ref(false);
const errorMsg = ref('');
const migrationResult = ref<MigrationResultData | null>(null);

onMounted(async () => {
  try {
    const data = await ofetch<{ available: boolean }>(
      `${config.public.apiBaseUrl}/api/v1/migration/status`,
    );
    migrationAvailable.value = data.available;

    // If no 1.x DB exists, redirect to login
    if (!data.available) {
      router.replace('/login');
      return;
    }
  } catch {
    // If the check fails, redirect to login
    router.replace('/login');
    return;
  } finally {
    isLoading.value = false;
  }
});

async function executeMigration() {
  errorMsg.value = '';
  executing.value = true;

  try {
    const result = await ofetch<MigrationResultData>(
      `${config.public.apiBaseUrl}/api/v1/migration/execute`,
      {
        method: 'POST',
        credentials: 'include',
      },
    );

    if (result.success) {
      migrationResult.value = result;
      migrationComplete.value = true;
    } else {
      errorMsg.value = result.error || 'Migration failed';
    }
  } catch (e) {
    const err = e as { data?: { error?: string } };
    errorMsg.value = err.data?.error || 'Migration failed — check server logs for details.';
  } finally {
    executing.value = false;
  }
}

function startFresh() {
  // Skip migration and go straight to login/setup
  router.replace('/login');
}

function goToLogin() {
  window.location.href = config.app.baseURL || '/';
}
</script>
