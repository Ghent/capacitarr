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
    <UiCardContent class="pt-5 max-w-md">
      <form class="space-y-4" @submit.prevent="changeUsername">
        <div class="space-y-1.5">
          <UiLabel for="new-username">
            {{ $t('settings.newUsername') }}
          </UiLabel>
          <UiInput
            id="new-username"
            v-model="usernameForm.newUsername"
            type="text"
            autocomplete="username"
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
            autocomplete="current-password"
            placeholder="Confirm with current password"
          />
        </div>
        <UiAlert v-if="usernameError" variant="destructive">
          <UiAlertDescription>{{ usernameError }}</UiAlertDescription>
        </UiAlert>
        <div>
          <UiButton type="submit" :disabled="savingUsername">
            {{ savingUsername ? $t('settings.changingUsername') : $t('settings.changeUsername') }}
          </UiButton>
        </div>
      </form>
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
    <UiCardContent class="pt-5 max-w-md">
      <form class="space-y-4" @submit.prevent="changePassword">
        <!-- Hidden username field for password-manager association and accessibility -->
        <input
          type="text"
          autocomplete="username"
          class="sr-only"
          tabindex="-1"
          aria-hidden="true"
        />
        <div class="space-y-1.5">
          <UiLabel for="current-password">
            {{ $t('settings.currentPassword') }}
          </UiLabel>
          <UiInput
            id="current-password"
            v-model="passwordForm.currentPassword"
            type="password"
            autocomplete="current-password"
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
            autocomplete="new-password"
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
            autocomplete="new-password"
            placeholder="Confirm new password"
          />
        </div>
        <UiAlert v-if="passwordError" variant="destructive">
          <UiAlertDescription>{{ passwordError }}</UiAlertDescription>
        </UiAlert>
        <div>
          <UiButton type="submit" :disabled="savingPassword">
            {{ savingPassword ? $t('settings.changingPassword') : $t('settings.changePassword') }}
          </UiButton>
        </div>
      </form>
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
import type { ApiError } from '~/types/api';
import { toast } from 'vue-sonner';

const api = useApi();

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
    await api.PUT('/auth/password', {
      body: {
        currentPassword: passwordForm.currentPassword,
        newPassword: passwordForm.newPassword,
      },
    });
    toast.success('Password changed — please log in again');
    passwordForm.currentPassword = '';
    passwordForm.newPassword = '';
    passwordForm.confirmPassword = '';
    setTimeout(() => {
      navigateTo('/login');
    }, 1500);
  } catch (e: unknown) {
    passwordError.value = (e as ApiError)?.data?.error || 'Failed to change password';
    toast.error(passwordError.value);
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
    await api.PUT('/auth/username', {
      body: {
        newUsername: usernameForm.newUsername,
        currentPassword: usernameForm.password,
      },
    });
    toast.success('Username changed — please log in again');
    usernameForm.newUsername = '';
    usernameForm.password = '';
    setTimeout(() => {
      navigateTo('/login');
    }, 1500);
  } catch (e: unknown) {
    usernameError.value = (e as ApiError)?.data?.error || 'Failed to change username';
    toast.error(usernameError.value);
  } finally {
    savingUsername.value = false;
  }
}

// ─── API Key ─────────────────────────────────────────────────────────────────
async function generateApiKey() {
  generatingApiKey.value = true;
  try {
    const result = await api.POST('/auth/apikey');
    apiKey.value = result.api_key;
    toast.success('API key generated');
  } catch {
    toast.error('Failed to generate API key');
  } finally {
    generatingApiKey.value = false;
  }
}

async function fetchApiKey() {
  try {
    const result = await api.GET('/auth/apikey');
    if (result?.has_key) {
      apiKey.value = '••••••••••••••••••••••••••••••••';
      if (result.hint) apiKeyHint.value = result.hint;
    }
  } catch (err) {
    console.warn('[SettingsSecurity] fetchApiKey failed:', err);
  }
}

function copyApiKey() {
  navigator.clipboard.writeText(apiKey.value);
  toast.success('API key copied to clipboard');
}

onMounted(() => {
  fetchApiKey();
});
</script>
