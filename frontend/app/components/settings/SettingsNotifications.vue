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
      Set up Discord, Slack, or in-app notifications for engine events.
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
              <UiBadge :variant="channel.type === 'inapp' ? 'secondary' : 'outline'" class="mt-1">
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
        <p class="text-xs font-medium text-muted-foreground uppercase tracking-wider">
          Event Subscriptions
        </p>
        <div class="space-y-2">
          <label class="flex items-center gap-2 text-sm">
            <UiSwitch
              :model-value="channel.onThresholdBreach"
              size="sm"
              @update:model-value="
                (val: boolean) => updateChannelEvent(channel, 'onThresholdBreach', val)
              "
            />
            <span>Threshold Breach</span>
          </label>
          <label class="flex items-center gap-2 text-sm">
            <UiSwitch
              :model-value="channel.onDeletionExecuted"
              size="sm"
              @update:model-value="
                (val: boolean) => updateChannelEvent(channel, 'onDeletionExecuted', val)
              "
            />
            <span>Deletion Executed</span>
          </label>
          <label class="flex items-center gap-2 text-sm">
            <UiSwitch
              :model-value="channel.onEngineError"
              size="sm"
              @update:model-value="
                (val: boolean) => updateChannelEvent(channel, 'onEngineError', val)
              "
            />
            <span>Engine Error</span>
          </label>
          <label class="flex items-center gap-2 text-sm">
            <UiSwitch
              :model-value="channel.onEngineComplete"
              size="sm"
              @update:model-value="
                (val: boolean) => updateChannelEvent(channel, 'onEngineComplete', val)
              "
            />
            <span>Engine Complete</span>
          </label>
        </div>
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
              <UiSelectItem value="slack"> Slack </UiSelectItem>
              <UiSelectItem value="inapp"> In-App </UiSelectItem>
            </UiSelectContent>
          </UiSelect>
        </div>

        <div class="space-y-1.5">
          <UiLabel>Name</UiLabel>
          <UiInput v-model="channelForm.name" type="text" placeholder="e.g. My Discord Alerts" />
        </div>

        <div v-if="channelForm.type !== 'inapp'" class="space-y-1.5">
          <UiLabel>Webhook URL</UiLabel>
          <UiInput
            v-model="channelForm.webhookUrl"
            type="text"
            :placeholder="
              channelForm.type === 'discord'
                ? 'https://discord.com/api/webhooks/...'
                : 'https://hooks.slack.com/services/...'
            "
          />
        </div>

        <div class="space-y-3">
          <p class="text-xs font-medium text-muted-foreground uppercase tracking-wider">
            Event Subscriptions
          </p>
          <label class="flex items-center gap-2 text-sm">
            <UiSwitch
              :model-value="channelForm.onThresholdBreach"
              @update:model-value="
                (val: boolean) => {
                  channelForm.onThresholdBreach = val;
                }
              "
            />
            <span>Threshold Breach</span>
          </label>
          <label class="flex items-center gap-2 text-sm">
            <UiSwitch
              :model-value="channelForm.onDeletionExecuted"
              @update:model-value="
                (val: boolean) => {
                  channelForm.onDeletionExecuted = val;
                }
              "
            />
            <span>Deletion Executed</span>
          </label>
          <label class="flex items-center gap-2 text-sm">
            <UiSwitch
              :model-value="channelForm.onEngineError"
              @update:model-value="
                (val: boolean) => {
                  channelForm.onEngineError = val;
                }
              "
            />
            <span>Engine Error</span>
          </label>
          <label class="flex items-center gap-2 text-sm">
            <UiSwitch
              :model-value="channelForm.onEngineComplete"
              @update:model-value="
                (val: boolean) => {
                  channelForm.onEngineComplete = val;
                }
              "
            />
            <span>Engine Complete</span>
          </label>
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
import { PlusIcon, LoaderCircleIcon, BellIcon, MessageSquareIcon, HashIcon } from 'lucide-vue-next';
import type { NotificationChannel, ApiError } from '~/types/api';

const api = useApi();
const { addToast } = useToast();

// ─── Notification Channels state ─────────────────────────────────────────────
const channels = ref<NotificationChannel[]>([]);
const channelsLoading = ref(false);
const showChannelModal = ref(false);
const editingChannel = ref<NotificationChannel | null>(null);
const savingChannel = ref(false);
const channelFormError = ref('');
const testingChannelId = ref<number | null>(null);

const channelForm = reactive({
  type: 'discord' as 'discord' | 'slack' | 'inapp',
  name: '',
  webhookUrl: '',
  onThresholdBreach: true,
  onDeletionExecuted: true,
  onEngineError: true,
  onEngineComplete: false,
});

// ─── Type display helpers ────────────────────────────────────────────────────
function channelTypeIcon(type: string) {
  switch (type) {
    case 'discord':
      return MessageSquareIcon;
    case 'slack':
      return HashIcon;
    case 'inapp':
      return BellIcon;
    default:
      return BellIcon;
  }
}

function channelTypeColor(type: string) {
  switch (type) {
    case 'discord':
      return 'bg-indigo-500';
    case 'slack':
      return 'bg-green-500';
    case 'inapp':
      return 'bg-blue-500';
    default:
      return 'bg-muted-foreground';
  }
}

function channelTypeLabel(type: string) {
  switch (type) {
    case 'discord':
      return 'Discord';
    case 'slack':
      return 'Slack';
    case 'inapp':
      return 'In-App';
    default:
      return type;
  }
}

// ─── CRUD operations ─────────────────────────────────────────────────────────
async function fetchChannels() {
  channelsLoading.value = true;
  try {
    channels.value = (await api('/api/v1/notifications/channels')) as NotificationChannel[];
  } catch {
    addToast('Failed to load notification channels', 'error');
  } finally {
    channelsLoading.value = false;
  }
}

function openAddChannelModal() {
  editingChannel.value = null;
  channelForm.type = 'discord';
  channelForm.name = '';
  channelForm.webhookUrl = '';
  channelForm.onThresholdBreach = true;
  channelForm.onDeletionExecuted = true;
  channelForm.onEngineError = true;
  channelForm.onEngineComplete = false;
  channelFormError.value = '';
  showChannelModal.value = true;
}

function openEditChannelModal(channel: NotificationChannel) {
  editingChannel.value = channel;
  channelForm.type = channel.type;
  channelForm.name = channel.name;
  channelForm.webhookUrl = channel.webhookUrl || '';
  channelForm.onThresholdBreach = channel.onThresholdBreach;
  channelForm.onDeletionExecuted = channel.onDeletionExecuted;
  channelForm.onEngineError = channel.onEngineError;
  channelForm.onEngineComplete = channel.onEngineComplete;
  channelFormError.value = '';
  showChannelModal.value = true;
}

async function onChannelSubmit() {
  savingChannel.value = true;
  channelFormError.value = '';
  try {
    const body = {
      type: channelForm.type,
      name: channelForm.name,
      webhookUrl: channelForm.type !== 'inapp' ? channelForm.webhookUrl : undefined,
      enabled: editingChannel.value ? editingChannel.value.enabled : true,
      onThresholdBreach: channelForm.onThresholdBreach,
      onDeletionExecuted: channelForm.onDeletionExecuted,
      onEngineError: channelForm.onEngineError,
      onEngineComplete: channelForm.onEngineComplete,
    };
    if (editingChannel.value) {
      await api(`/api/v1/notifications/channels/${editingChannel.value.id}`, {
        method: 'PUT',
        body,
      });
    } else {
      await api('/api/v1/notifications/channels', {
        method: 'POST',
        body,
      });
    }
    showChannelModal.value = false;
    addToast('Notification channel saved', 'success');
    await fetchChannels();
  } catch (e: unknown) {
    channelFormError.value = (e as ApiError)?.data?.error || 'Failed to save channel';
    addToast(channelFormError.value, 'error');
  } finally {
    savingChannel.value = false;
  }
}

async function deleteChannel(channel: NotificationChannel) {
  if (!confirm(`Delete "${channel.name}"? This cannot be undone.`)) return;
  try {
    await api(`/api/v1/notifications/channels/${channel.id}`, { method: 'DELETE' });
    addToast('Channel deleted', 'success');
    await fetchChannels();
  } catch {
    addToast('Failed to delete channel', 'error');
  }
}

async function toggleChannelEnabled(channel: NotificationChannel, enabled: boolean) {
  try {
    await api(`/api/v1/notifications/channels/${channel.id}`, {
      method: 'PUT',
      body: { ...channel, enabled },
    });
    channel.enabled = enabled;
    addToast(`Channel ${enabled ? 'enabled' : 'disabled'}`, 'success');
  } catch {
    addToast('Failed to update channel', 'error');
  }
}

async function updateChannelEvent(channel: NotificationChannel, field: string, value: boolean) {
  try {
    const updated = { ...channel, [field]: value };
    await api(`/api/v1/notifications/channels/${channel.id}`, {
      method: 'PUT',
      body: updated,
    });
    (channel as unknown as Record<string, unknown>)[field] = value;
  } catch {
    addToast('Failed to update channel', 'error');
  }
}

async function testChannel(channel: NotificationChannel) {
  testingChannelId.value = channel.id;
  try {
    await api(`/api/v1/notifications/channels/${channel.id}/test`, { method: 'POST' });
    addToast('Test notification sent!', 'success');
  } catch {
    addToast('Failed to send test notification', 'error');
  } finally {
    testingChannelId.value = null;
  }
}

onMounted(() => {
  fetchChannels();
});
</script>
