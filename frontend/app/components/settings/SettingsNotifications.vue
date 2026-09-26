<template>
  <div class="flex justify-end mb-6">
    <UiButton @click="openAddChannelModal">
      <component :is="PlusIcon" class="w-4 h-4" />
      Add Channel
    </UiButton>
  </div>

  <!-- Loading -->
  <div v-if="channelsLoading" class="flex justify-center py-16">
    <component :is="LoaderCircleIcon" class="w-8 h-8 text-primary animate-spin" />
  </div>

  <!-- Empty state -->
  <div
    v-else-if="channels.length === 0"
    v-motion
    :initial="{ opacity: 0, y: 8 }"
    :enter="{ opacity: 1, y: 0 }"
    class="text-center py-20"
  >
    <component :is="BellIcon" class="w-16 h-16 text-muted-foreground/40 mx-auto mb-4" />
    <h3 class="text-lg font-medium text-foreground mb-2">No notification channels configured</h3>
    <p class="text-muted-foreground mb-6">
      Set up Discord or Apprise notifications for engine events.
    </p>
    <UiButton size="lg" @click="openAddChannelModal">
      <component :is="PlusIcon" class="w-4 h-4" />
      Add Your First Channel
    </UiButton>
  </div>

  <!-- Channel Cards Grid -->
  <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
    <UiCard
      v-for="(channel, idx) in channels"
      :key="channel.id"
      v-motion
      :initial="{ opacity: 0, y: 12 }"
      :enter="{
        opacity: 1,
        y: 0,
        transition: { type: 'spring', stiffness: 260, damping: 24, delay: 80 * idx },
      }"
      class="overflow-hidden"
    >
      <!-- Card Header -->
      <UiCardHeader class="border-b border-border">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-3">
            <div
              :class="[
                'w-10 h-10 rounded-lg flex items-center justify-center',
                channelTypeColor(channel.type),
              ]"
            >
              <component :is="channelTypeIcon(channel.type)" class="w-5 h-5 text-white" />
            </div>
            <div>
              <UiCardTitle class="text-base">
                {{ channel.name }}
              </UiCardTitle>
              <UiBadge variant="outline" class="mt-1">
                {{ channelTypeLabel(channel.type) }}
              </UiBadge>
            </div>
          </div>
          <UiSwitch
            :model-value="channel.enabled"
            @update:model-value="(val: boolean) => toggleChannelEnabled(channel, val)"
          />
        </div>
      </UiCardHeader>

      <!-- Card Body -->
      <UiCardContent class="pt-4 space-y-3">
        <div class="flex items-center justify-between">
          <p class="text-xs font-medium text-muted-foreground uppercase tracking-wider">
            {{ $t('notifications.levelLabel') }}
          </p>
          <UiBadge variant="outline" class="capitalize">{{ channel.notificationLevel }}</UiBadge>
        </div>
        <p class="text-xs text-muted-foreground">
          {{ levelDescription(channel.notificationLevel) }}
        </p>
        <p v-if="activeOverrideCount(channel) > 0" class="text-xs text-muted-foreground">
          {{ $t('notifications.overridesActive', { count: activeOverrideCount(channel) }) }}
        </p>
      </UiCardContent>

      <!-- Card Footer -->
      <UiCardFooter class="border-t border-border flex items-center justify-between">
        <div class="flex gap-2">
          <UiButton
            variant="outline"
            size="sm"
            :disabled="testingChannelId === channel.id"
            @click="testChannel(channel)"
          >
            {{ testingChannelId === channel.id ? 'Sending…' : 'Test' }}
          </UiButton>
          <UiButton variant="outline" size="sm" @click="openEditChannelModal(channel)">
            Edit
          </UiButton>
        </div>
        <UiButton variant="destructive" size="sm" @click="deleteChannel(channel)">
          Delete
        </UiButton>
      </UiCardFooter>
    </UiCard>
  </div>

  <!-- Notification Channel Modal -->
  <UiDialog
    :open="showChannelModal"
    @update:open="
      (val: boolean) => {
        showChannelModal = val;
      }
    "
  >
    <UiDialogContent class="max-w-md">
      <UiDialogHeader>
        <UiDialogTitle>
          {{ editingChannel ? 'Edit Channel' : 'Add Notification Channel' }}
        </UiDialogTitle>
      </UiDialogHeader>

      <form class="space-y-4" @submit.prevent="onChannelSubmit">
        <div class="space-y-1.5">
          <UiLabel>Type</UiLabel>
          <UiSelect v-model="channelForm.type" :disabled="!!editingChannel">
            <UiSelectTrigger class="w-full">
              <UiSelectValue placeholder="Select type" />
            </UiSelectTrigger>
            <UiSelectContent>
              <UiSelectItem value="discord"> Discord </UiSelectItem>
              <UiSelectItem value="apprise"> Apprise </UiSelectItem>
            </UiSelectContent>
          </UiSelect>
        </div>

        <div class="space-y-1.5">
          <UiLabel>Name</UiLabel>
          <UiInput v-model="channelForm.name" type="text" placeholder="e.g. My Discord Alerts" />
        </div>

        <div class="space-y-1.5">
          <UiLabel>
            {{ channelForm.type === 'apprise' ? 'Apprise Server URL' : 'Discord Webhook URL' }}
          </UiLabel>
          <UiInput
            v-model="channelForm.webhookUrl"
            type="text"
            :placeholder="
              channelForm.type === 'apprise'
                ? 'http://apprise:8000/api/notify/mykey/'
                : 'https://discord.com/api/webhooks/...'
            "
          />
        </div>

        <div v-if="channelForm.type === 'apprise'" class="space-y-1.5">
          <UiLabel>Tags (optional)</UiLabel>
          <UiInput v-model="channelForm.appriseTags" type="text" placeholder="discord,email" />
          <p class="text-xs text-muted-foreground">
            Comma-separated Apprise tags to route notifications to specific services.
          </p>
        </div>

        <div class="space-y-3">
          <div class="space-y-1.5">
            <UiLabel>{{ $t('notifications.levelLabel') }}</UiLabel>
            <UiSelect v-model="channelForm.notificationLevel">
              <UiSelectTrigger class="w-full">
                <UiSelectValue :placeholder="$t('notifications.levelPlaceholder')" />
              </UiSelectTrigger>
              <UiSelectContent>
                <UiSelectItem value="off">{{ $t('notifications.levelOff') }}</UiSelectItem>
                <UiSelectItem value="critical">{{
                  $t('notifications.levelCritical')
                }}</UiSelectItem>
                <UiSelectItem value="important">{{
                  $t('notifications.levelImportant')
                }}</UiSelectItem>
                <UiSelectItem value="normal">{{ $t('notifications.levelNormal') }}</UiSelectItem>
                <UiSelectItem value="verbose">{{ $t('notifications.levelVerbose') }}</UiSelectItem>
              </UiSelectContent>
            </UiSelect>
            <p class="text-xs text-muted-foreground">
              {{ levelDescription(channelForm.notificationLevel) }}
            </p>
          </div>

          <UiCollapsible v-model:open="showAdvanced">
            <UiCollapsibleTrigger
              class="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground"
            >
              <component :is="showAdvanced ? ChevronDownIcon : ChevronRightIcon" class="w-4 h-4" />
              {{ $t('notifications.advancedOverrides') }}
            </UiCollapsibleTrigger>
            <UiCollapsibleContent class="mt-3 space-y-2">
              <p class="text-xs text-muted-foreground mb-3">
                {{ $t('notifications.advancedOverridesDesc') }}
              </p>
              <div
                v-for="override in overrideOptions"
                :key="override.key"
                class="flex items-center justify-between"
              >
                <span class="text-sm">{{ override.label }}</span>
                <UiSelect
                  :model-value="triStateValue(getOverrideValue(override.key))"
                  @update:model-value="(val) => setTriState(override.key, String(val))"
                >
                  <UiSelectTrigger class="w-24 h-8">
                    <UiSelectValue />
                  </UiSelectTrigger>
                  <UiSelectContent>
                    <UiSelectItem value="auto">{{ $t('notifications.triStateAuto') }}</UiSelectItem>
                    <UiSelectItem value="on">{{ $t('notifications.triStateOn') }}</UiSelectItem>
                    <UiSelectItem value="off">{{ $t('notifications.triStateOff') }}</UiSelectItem>
                  </UiSelectContent>
                </UiSelect>
              </div>
            </UiCollapsibleContent>
          </UiCollapsible>
        </div>

        <!-- Error -->
        <UiAlert v-if="channelFormError" variant="destructive">
          <UiAlertDescription>{{ channelFormError }}</UiAlertDescription>
        </UiAlert>
      </form>

      <UiDialogFooter class="flex gap-2 justify-end">
        <UiButton variant="ghost" @click="showChannelModal = false"> Cancel </UiButton>
        <UiButton :disabled="savingChannel" @click="onChannelSubmit">
          {{ editingChannel ? 'Save' : 'Add' }}
        </UiButton>
      </UiDialogFooter>
    </UiDialogContent>
  </UiDialog>
</template>

<script setup lang="ts">
import {
  PlusIcon,
  LoaderCircleIcon,
  BellIcon,
  SendIcon,
  ChevronDownIcon,
  ChevronRightIcon,
} from 'lucide-vue-next';
import DiscordIcon from '~/components/icons/DiscordIcon.vue';
import type { NotificationChannel, ApiError } from '~/types/api';
import { toast } from 'vue-sonner';

const api = useApi();

// ─── Notification Channels state ─────────────────────────────────────────────
const channels = ref<NotificationChannel[]>([]);
const channelsLoading = ref(false);
const showChannelModal = ref(false);
const editingChannel = ref<NotificationChannel | null>(null);
const savingChannel = ref(false);
const channelFormError = ref('');
const testingChannelId = ref<number | null>(null);

const channelForm = reactive({
  type: 'discord' as 'discord' | 'apprise',
  name: '',
  webhookUrl: '',
  appriseTags: '',
  notificationLevel: 'normal' as NotificationChannel['notificationLevel'],
  overrideCycleDigest: null as boolean | null,
  overrideError: null as boolean | null,
  overrideModeChanged: null as boolean | null,
  overrideServerStarted: null as boolean | null,
  overrideThresholdBreach: null as boolean | null,
  overrideUpdateAvailable: null as boolean | null,
  overrideApprovalActivity: null as boolean | null,
  overrideIntegrationStatus: null as boolean | null,
});

const showAdvanced = ref(false);

const overrideOptions = [
  { key: 'overrideCycleDigest', label: 'Cycle Digest' },
  { key: 'overrideError', label: 'Errors' },
  { key: 'overrideThresholdBreach', label: 'Threshold Breach' },
  { key: 'overrideModeChanged', label: 'Mode Changed' },
  { key: 'overrideApprovalActivity', label: 'Approval Activity' },
  { key: 'overrideIntegrationStatus', label: 'Integration Status' },
  { key: 'overrideServerStarted', label: 'Server Started' },
  { key: 'overrideUpdateAvailable', label: 'Update Available' },
];

const { t } = useI18n();

function levelDescription(level: string): string {
  switch (level) {
    case 'off':
      return t('notifications.levelOffDesc');
    case 'critical':
      return t('notifications.levelCriticalDesc');
    case 'important':
      return t('notifications.levelImportantDesc');
    case 'normal':
      return t('notifications.levelNormalDesc');
    case 'verbose':
      return t('notifications.levelVerboseDesc');
    default:
      return '';
  }
}

function getOverrideValue(key: string): boolean | null {
  return (channelForm as unknown as Record<string, boolean | null>)[key] ?? null;
}

function triStateValue(val: boolean | null): string {
  if (val === null || val === undefined) return 'auto';
  return val ? 'on' : 'off';
}

function setTriState(key: string, val: string) {
  (channelForm as Record<string, unknown>)[key] = val === 'auto' ? null : val === 'on';
}

function activeOverrideCount(channel: NotificationChannel): number {
  const overrideKeys = [
    'overrideCycleDigest',
    'overrideError',
    'overrideModeChanged',
    'overrideServerStarted',
    'overrideThresholdBreach',
    'overrideUpdateAvailable',
    'overrideApprovalActivity',
    'overrideIntegrationStatus',
  ] as const;
  return overrideKeys.filter((k) => channel[k] !== null && channel[k] !== undefined).length;
}

// ─── Type display helpers ────────────────────────────────────────────────────
function channelTypeIcon(type: string) {
  switch (type) {
    case 'discord':
      return DiscordIcon;
    case 'apprise':
      return SendIcon;
    default:
      return BellIcon;
  }
}

function channelTypeColor(type: string) {
  switch (type) {
    case 'discord':
      return 'bg-indigo-500';
    case 'apprise':
      return 'bg-amber-500';
    default:
      return 'bg-muted-foreground';
  }
}

function channelTypeLabel(type: string) {
  switch (type) {
    case 'discord':
      return 'Discord';
    case 'apprise':
      return 'Apprise';
    default:
      return type;
  }
}

// ─── CRUD operations ─────────────────────────────────────────────────────────
async function fetchChannels() {
  channelsLoading.value = true;
  try {
    channels.value = (await api.GET('/notifications/channels')) ?? [];
  } catch {
    toast.error('Failed to load notification channels');
  } finally {
    channelsLoading.value = false;
  }
}

function openAddChannelModal() {
  editingChannel.value = null;
  channelForm.type = 'discord';
  channelForm.name = '';
  channelForm.webhookUrl = '';
  channelForm.appriseTags = '';
  channelForm.notificationLevel = 'normal';
  channelForm.overrideCycleDigest = null;
  channelForm.overrideError = null;
  channelForm.overrideModeChanged = null;
  channelForm.overrideServerStarted = null;
  channelForm.overrideThresholdBreach = null;
  channelForm.overrideUpdateAvailable = null;
  channelForm.overrideApprovalActivity = null;
  channelForm.overrideIntegrationStatus = null;
  channelFormError.value = '';
  showAdvanced.value = false;
  showChannelModal.value = true;
}

function openEditChannelModal(channel: NotificationChannel) {
  editingChannel.value = channel;
  channelForm.type = channel.type;
  channelForm.name = channel.name;
  channelForm.webhookUrl = channel.webhookUrl || '';
  channelForm.appriseTags = channel.appriseTags || '';
  channelForm.notificationLevel = channel.notificationLevel;
  channelForm.overrideCycleDigest = channel.overrideCycleDigest ?? null;
  channelForm.overrideError = channel.overrideError ?? null;
  channelForm.overrideModeChanged = channel.overrideModeChanged ?? null;
  channelForm.overrideServerStarted = channel.overrideServerStarted ?? null;
  channelForm.overrideThresholdBreach = channel.overrideThresholdBreach ?? null;
  channelForm.overrideUpdateAvailable = channel.overrideUpdateAvailable ?? null;
  channelForm.overrideApprovalActivity = channel.overrideApprovalActivity ?? null;
  channelForm.overrideIntegrationStatus = channel.overrideIntegrationStatus ?? null;
  channelFormError.value = '';
  showAdvanced.value = activeOverrideCount(channel) > 0;
  showChannelModal.value = true;
}

async function onChannelSubmit() {
  savingChannel.value = true;
  channelFormError.value = '';
  try {
    const body = {
      type: channelForm.type,
      name: channelForm.name,
      webhookUrl: channelForm.webhookUrl,
      enabled: editingChannel.value ? editingChannel.value.enabled : true,
      notificationLevel: channelForm.notificationLevel,
      overrideCycleDigest: channelForm.overrideCycleDigest,
      overrideError: channelForm.overrideError,
      overrideModeChanged: channelForm.overrideModeChanged,
      overrideServerStarted: channelForm.overrideServerStarted,
      overrideThresholdBreach: channelForm.overrideThresholdBreach,
      overrideUpdateAvailable: channelForm.overrideUpdateAvailable,
      overrideApprovalActivity: channelForm.overrideApprovalActivity,
      overrideIntegrationStatus: channelForm.overrideIntegrationStatus,
      ...(channelForm.type === 'apprise' ? { appriseTags: channelForm.appriseTags } : {}),
    };
    if (editingChannel.value) {
      await api.PUT('/notifications/channels/{id}', {
        path: { id: editingChannel.value.id },
        body,
      });
    } else {
      await api.POST('/notifications/channels', { body });
    }
    showChannelModal.value = false;
    toast.success('Notification channel saved');
    await fetchChannels();
  } catch (e: unknown) {
    channelFormError.value = (e as ApiError)?.data?.error || 'Failed to save channel';
    toast.error(channelFormError.value);
  } finally {
    savingChannel.value = false;
  }
}

async function deleteChannel(channel: NotificationChannel) {
  if (!confirm(`Delete "${channel.name}"? This cannot be undone.`)) return;
  try {
    await api.DELETE('/notifications/channels/{id}', { path: { id: channel.id } });
    toast.success('Channel deleted');
    await fetchChannels();
  } catch {
    toast.error('Failed to delete channel');
  }
}

async function toggleChannelEnabled(channel: NotificationChannel, enabled: boolean) {
  try {
    await api.PUT('/notifications/channels/{id}', {
      path: { id: channel.id },
      body: { ...channel, enabled },
    });
    channel.enabled = enabled;
    toast.success(`Channel ${enabled ? 'enabled' : 'disabled'}`);
  } catch {
    toast.error('Failed to update channel');
  }
}

async function testChannel(channel: NotificationChannel) {
  testingChannelId.value = channel.id;
  try {
    await api.POST('/notifications/channels/{id}/test', { path: { id: channel.id } });
    toast.success('Test notification sent!');
  } catch {
    toast.error('Failed to send test notification');
  } finally {
    testingChannelId.value = null;
  }
}

onMounted(() => {
  fetchChannels();
});
</script>
