<template>
  <!-- Username Change -->
  <UiCard
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{ opacity: 1, y: 0 }"
    class="overflow-hidden"
  >
    <UiCardHeader class="border-b border-border">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg bg-purple-500 flex items-center justify-center">
          <component :is="UserIcon" class="w-5 h-5 text-white" />
        </div>
        <div>
          <UiCardTitle class="text-base">
            {{ $t('settings.changeUsername') }}
          </UiCardTitle>
          <UiCardDescription>{{ $t('settings.changeUsernameDesc') }}</UiCardDescription>
        </div>
      </div>
    </UiCardHeader>
    <UiCardContent class="pt-5 space-y-4 max-w-md">
      <div class="space-y-1.5">
        <UiLabel for="new-username">
          {{ $t('settings.newUsername') }}
        </UiLabel>
        <UiInput
          id="new-username"
          v-model="usernameForm.newUsername"
          type="text"
          placeholder="Enter new username"
        />
      </div>
      <div class="space-y-1.5">
        <UiLabel for="username-password">
          {{ $t('settings.currentPassword') }}
        </UiLabel>
        <UiInput
          id="username-password"
          v-model="usernameForm.password"
          type="password"
          placeholder="Confirm with current password"
        />
      </div>
      <UiAlert v-if="usernameError" variant="destructive">
        <UiAlertDescription>{{ usernameError }}</UiAlertDescription>
      </UiAlert>
      <div>
        <UiButton :disabled="savingUsername" @click="changeUsername">
          {{ savingUsername ? $t('settings.changingUsername') : $t('settings.changeUsername') }}
        </UiButton>
      </div>
    </UiCardContent>
  </UiCard>

  <!-- Password Change -->
  <UiCard
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{ opacity: 1, y: 0 }"
    class="overflow-hidden"
  >
    <UiCardHeader class="border-b border-border">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg bg-red-500 flex items-center justify-center">
          <component :is="ShieldIcon" class="w-5 h-5 text-white" />
        </div>
        <div>
          <UiCardTitle class="text-base">
            {{ $t('settings.changePassword') }}
          </UiCardTitle>
          <UiCardDescription>{{ $t('settings.changePasswordDesc') }}</UiCardDescription>
        </div>
      </div>
    </UiCardHeader>
    <UiCardContent class="pt-5 space-y-4 max-w-md">
      <div class="space-y-1.5">
        <UiLabel for="current-password">
          {{ $t('settings.currentPassword') }}
        </UiLabel>
        <UiInput
          id="current-password"
          v-model="passwordForm.currentPassword"
          type="password"
          placeholder="Enter current password"
        />
      </div>
      <div class="space-y-1.5">
        <UiLabel for="new-password">
          {{ $t('settings.newPassword') }}
        </UiLabel>
        <UiInput
          id="new-password"
          v-model="passwordForm.newPassword"
          type="password"
          placeholder="Enter new password"
        />
      </div>
      <div class="space-y-1.5">
        <UiLabel for="confirm-password">
          {{ $t('settings.confirmPassword') }}
        </UiLabel>
        <UiInput
          id="confirm-password"
          v-model="passwordForm.confirmPassword"
          type="password"
          placeholder="Confirm new password"
        />
      </div>
      <UiAlert v-if="passwordError" variant="destructive">
        <UiAlertDescription>{{ passwordError }}</UiAlertDescription>
      </UiAlert>
      <div>
        <UiButton :disabled="savingPassword" @click="changePassword">
          {{ savingPassword ? $t('settings.changingPassword') : $t('settings.changePassword') }}
        </UiButton>
      </div>
    </UiCardContent>
  </UiCard>

  <!-- API Key -->
  <UiCard
    v-motion
    :initial="{ opacity: 0, y: 12 }"
    :enter="{ opacity: 1, y: 0, transition: { delay: 100 } }"
    class="overflow-hidden"
  >
    <UiCardHeader class="border-b border-border">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-lg bg-amber-500 flex items-center justify-center">
          <component :is="KeyIcon" class="w-5 h-5 text-white" />
        </div>
        <div>
          <UiCardTitle class="text-base">
            {{ $t('settings.apiKey') }}
          </UiCardTitle>
          <UiCardDescription>{{ $t('settings.apiKeyDesc') }}</UiCardDescription>
        </div>
      </div>
    </UiCardHeader>
    <UiCardContent class="pt-5 space-y-4">
      <div v-if="apiKey" class="space-y-2">
        <div class="flex items-center gap-2">
          <code class="flex-1 px-3 py-2 bg-muted rounded-lg text-sm font-mono break-all">{{
            apiKeyDisplay
          }}</code>
          <UiButton v-if="!apiKey.startsWith('••')" variant="outline" size="sm" @click="copyApiKey">
            {{ $t('common.copy') }}
          </UiButton>
        </div>
        <p v-if="apiKey.startsWith('••')" class="text-xs text-muted-foreground">
          {{ $t('settings.apiKeyHashedHint') }}
        </p>
      </div>
      <div v-else class="text-sm text-muted-foreground">
        {{ $t('settings.noApiKey') }}
      </div>
      <div>
        <UiButton :disabled="generatingApiKey" @click="generateApiKey">
          {{ apiKey ? $t('settings.regenerateApiKey') : $t('settings.generateApiKey') }}
        </UiButton>
      </div>
    </UiCardContent>
  </UiCard>
</template>

<script setup lang="ts">
import { UserIcon, ShieldIcon, KeyIcon } from 'lucide-vue-next';
import type { ApiKeyResponse, ApiError } from '~/types/api';

const api = useApi();
const { addToast } = useToast();

// ─── Password change state ───────────────────────────────────────────────────
const passwordForm = reactive({
  currentPassword: '',
  newPassword: '',
  confirmPassword: '',
});
const passwordError = ref('');
const savingPassword = ref(false);

// ─── Username change state ───────────────────────────────────────────────────
const usernameForm = reactive({
  newUsername: '',
  password: '',
});
const usernameError = ref('');
const savingUsername = ref(false);

// ─── API Key state ───────────────────────────────────────────────────────────
const apiKey = ref('');
const apiKeyHint = ref('');
const generatingApiKey = ref(false);

const apiKeyDisplay = computed(() => {
  if (!apiKey.value) return '';
  if (!apiKey.value.startsWith('••')) return apiKey.value;
  if (apiKeyHint.value) return '••••••••••••••••••••••••••••' + apiKeyHint.value;
  return apiKey.value;
});

// ─── Password Change ─────────────────────────────────────────────────────────
async function changePassword() {
  passwordError.value = '';

  if (!passwordForm.currentPassword || !passwordForm.newPassword) {
    passwordError.value = 'All fields are required';
    return;
  }
  if (passwordForm.newPassword !== passwordForm.confirmPassword) {
    passwordError.value = 'New passwords do not match';
    return;
  }
  if (passwordForm.newPassword.length < 8) {
    passwordError.value = 'New password must be at least 8 characters';
    return;
  }

  savingPassword.value = true;
  try {
    await api('/api/v1/auth/password', {
      method: 'PUT',
      body: {
        currentPassword: passwordForm.currentPassword,
        newPassword: passwordForm.newPassword,
      },
    });
    addToast('Password changed — please log in again', 'success');
    passwordForm.currentPassword = '';
    passwordForm.newPassword = '';
    passwordForm.confirmPassword = '';
    setTimeout(() => {
      navigateTo('/login');
    }, 1500);
  } catch (e: unknown) {
    passwordError.value = (e as ApiError)?.data?.error || 'Failed to change password';
    addToast(passwordError.value, 'error');
  } finally {
    savingPassword.value = false;
  }
}

// ─── Username Change ─────────────────────────────────────────────────────────
async function changeUsername() {
  usernameError.value = '';

  if (!usernameForm.newUsername || !usernameForm.password) {
    usernameError.value = 'All fields are required';
    return;
  }
  if (usernameForm.newUsername.length < 3) {
    usernameError.value = 'Username must be at least 3 characters';
    return;
  }
  if (/\s/.test(usernameForm.newUsername)) {
    usernameError.value = 'Username cannot contain spaces';
    return;
  }

  savingUsername.value = true;
  try {
    await api('/api/v1/auth/username', {
      method: 'PUT',
      body: {
        newUsername: usernameForm.newUsername,
        currentPassword: usernameForm.password,
      },
    });
    addToast('Username changed — please log in again', 'success');
    usernameForm.newUsername = '';
    usernameForm.password = '';
    setTimeout(() => {
      navigateTo('/login');
    }, 1500);
  } catch (e: unknown) {
    usernameError.value = (e as ApiError)?.data?.error || 'Failed to change username';
    addToast(usernameError.value, 'error');
  } finally {
    savingUsername.value = false;
  }
}

// ─── API Key ─────────────────────────────────────────────────────────────────
async function generateApiKey() {
  generatingApiKey.value = true;
  try {
    const result = (await api('/api/v1/auth/apikey', { method: 'POST' })) as ApiKeyResponse;
    apiKey.value = result.api_key;
    addToast('API key generated', 'success');
  } catch {
    addToast('Failed to generate API key', 'error');
  } finally {
    generatingApiKey.value = false;
  }
}

async function fetchApiKey() {
  try {
    const result = (await api('/api/v1/auth/apikey')) as {
      has_key?: boolean;
      api_key?: string;
      hint?: string;
    };
    if (result?.api_key) {
      apiKey.value = result.api_key;
    } else if (result?.has_key) {
      apiKey.value = '••••••••••••••••••••••••••••••••';
      if (result.hint) apiKeyHint.value = result.hint;
    }
  } catch {
    // Silently fail — no API key yet
  }
}

function copyApiKey() {
  navigator.clipboard.writeText(apiKey.value);
  addToast('API key copied to clipboard', 'success');
}

onMounted(() => {
  fetchApiKey();
});
</script>
