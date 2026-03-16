<template>
  <div>
    <!-- Header -->
    <div
      data-slot="page-header"
      class="mb-6 flex flex-col md:flex-row md:items-center justify-between gap-4"
    >
      <div>
        <h1 class="text-3xl font-bold tracking-tight">
          {{ $t('settings.title') }}
        </h1>
        <p class="text-muted-foreground mt-1.5">
          {{ $t('settings.subtitle') }}
        </p>
      </div>
    </div>

    <!-- Tabs -->
    <UiTabs v-model="activeTab" class="w-full">
      <UiTabsList class="mb-6">
        <UiTabsTrigger value="general">
          {{ $t('settings.general') }}
        </UiTabsTrigger>
        <UiTabsTrigger value="integrations">
          {{ $t('settings.integrations') }}
        </UiTabsTrigger>
        <UiTabsTrigger value="notifications">
          {{ $t('settings.notifications') }}
        </UiTabsTrigger>
        <UiTabsTrigger value="backup">
          {{ $t('settings.backupRestore') }}
        </UiTabsTrigger>
        <UiTabsTrigger value="security">
          {{ $t('settings.security') }}
        </UiTabsTrigger>
        <UiTabsTrigger
          value="advanced"
          class="border-destructive/40 bg-destructive/5 text-destructive hover:bg-destructive/10 data-[state=active]:bg-destructive data-[state=active]:text-white data-[state=active]:border-destructive"
        >
          {{ $t('settings.advanced') }}
        </UiTabsTrigger>
      </UiTabsList>

      <UiTabsContent value="general" class="space-y-6">
        <SettingsGeneral />
      </UiTabsContent>

      <UiTabsContent value="integrations">
        <SettingsIntegrations />
      </UiTabsContent>

      <UiTabsContent value="notifications">
        <SettingsNotifications />
      </UiTabsContent>

      <UiTabsContent value="backup" class="space-y-6">
        <SettingsBackupRestore />
      </UiTabsContent>

      <UiTabsContent value="security" class="space-y-6">
        <SettingsSecurity />
      </UiTabsContent>

      <UiTabsContent value="advanced" class="space-y-6">
        <SettingsAdvanced />
      </UiTabsContent>
    </UiTabs>
  </div>
</template>

<script setup lang="ts">
import SettingsGeneral from '~/components/settings/SettingsGeneral.vue';
import SettingsIntegrations from '~/components/settings/SettingsIntegrations.vue';
import SettingsNotifications from '~/components/settings/SettingsNotifications.vue';
import SettingsBackupRestore from '~/components/settings/SettingsBackupRestore.vue';
import SettingsSecurity from '~/components/settings/SettingsSecurity.vue';
import SettingsAdvanced from '~/components/settings/SettingsAdvanced.vue';

const VALID_TABS = [
  'general',
  'integrations',
  'notifications',
  'backup',
  'security',
  'advanced',
] as const;
type SettingsTab = (typeof VALID_TABS)[number];

const route = useRoute();
const router = useRouter();

// Read the initial tab from the URL query param, validated against known values.
// Falls back to 'general' for missing or invalid values.
const queryTab = route.query.tab as string | undefined;
const activeTab = ref<SettingsTab>(
  VALID_TABS.includes(queryTab as SettingsTab) ? (queryTab as SettingsTab) : 'general',
);

// Sync tab changes back to the URL so reloads and deep-links work.
watch(activeTab, (tab) => {
  router.replace({ query: { ...route.query, tab } });
});
</script>
