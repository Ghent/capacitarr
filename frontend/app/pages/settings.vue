<template>
  <div>
    <!-- Header -->
    <div
      data-slot="page-header"
      class="mb-8 flex flex-col md:flex-row md:items-center justify-between gap-4"
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
    <UiTabs default-value="general" class="w-full">
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
import SettingsSecurity from '~/components/settings/SettingsSecurity.vue';
import SettingsAdvanced from '~/components/settings/SettingsAdvanced.vue';
</script>
