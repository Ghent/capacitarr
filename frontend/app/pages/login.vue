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
      class="w-full max-w-sm"
    >
      <UiCard data-slot="login-card" class="overflow-hidden">
        <!-- Header -->
        <UiCardHeader class="pb-2 text-center">
          <div
            data-slot="login-icon"
            class="w-14 h-14 rounded-2xl bg-primary flex items-center justify-center mx-auto mb-5"
          >
            <component :is="LockKeyholeIcon" class="w-7 h-7 text-primary-foreground" />
          </div>
          <UiCardTitle class="text-2xl">
            {{ isSetupMode ? $t('login.setupTitle') : $t('login.title') }}
          </UiCardTitle>
          <UiCardDescription>{{
            isSetupMode ? $t('login.setupSubtitle') : $t('login.subtitle')
          }}</UiCardDescription>
        </UiCardHeader>

        <!-- Form -->
        <UiCardContent>
          <form class="space-y-5" @submit.prevent="onSubmit">
            <div class="space-y-2">
              <UiLabel for="username">
                {{ $t('login.username') }}
              </UiLabel>
              <UiInput
                id="username"
                v-model="state.username"
                type="text"
                autocomplete="username"
                :placeholder="isSetupMode ? 'Choose a username' : ''"
                autofocus
              />
            </div>

            <div class="space-y-2">
              <UiLabel for="password">
                {{ $t('login.password') }}
              </UiLabel>
              <UiInput
                id="password"
                v-model="state.password"
                type="password"
                autocomplete="current-password"
              />
            </div>

            <!-- Setup mode hint -->
            <p v-if="isSetupMode" class="text-sm text-muted-foreground">
              {{ $t('login.setupHint') }}
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

            <UiButton
              type="submit"
              :disabled="loading"
              class="w-full shadow-lg shadow-primary/25 hover:shadow-primary/40"
            >
              <span v-if="loading" class="flex items-center justify-center gap-2">
                <component :is="LoaderCircleIcon" class="w-4 h-4 animate-spin" />
                {{ $t('login.signingIn') }}
              </span>
              <span v-else>{{ isSetupMode ? $t('login.createAccount') : $t('login.signIn') }}</span>
            </UiButton>
          </form>
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
import { LockKeyholeIcon, LoaderCircleIcon } from 'lucide-vue-next';
import { ofetch } from 'ofetch';

const { t } = useI18n();
const config = useRuntimeConfig();

const state = reactive({
  username: '',
  password: '',
});

const loading = ref(false);
const errorMsg = ref('');
const isSetupMode = ref(false);
const isLoading = ref(true);

onMounted(async () => {
  try {
    // Check if a 1.x database exists — redirect to migration page if so
    try {
      const migrationStatus = await ofetch<{ available: boolean }>(
        `${config.public.apiBaseUrl}/api/v1/migration/status`,
      );
      if (migrationStatus.available) {
        const router = useRouter();
        router.replace('/migrate');
        return;
      }
    } catch {
      // Migration check failed — continue to normal login flow
    }

    const data = await ofetch(`${config.public.apiBaseUrl}/api/v1/auth/status`);
    isSetupMode.value = !data.initialized;
  } catch {
    // If the status check fails, default to login mode
    isSetupMode.value = false;
  } finally {
    isLoading.value = false;
  }
});

async function onSubmit() {
  errorMsg.value = '';
  loading.value = true;

  // Browser autofill can populate DOM inputs without triggering Vue's input
  // events, leaving v-model state empty. Read values directly from the DOM
  // as a fallback so autofilled credentials aren't silently discarded.
  const username =
    state.username || (document.getElementById('username') as HTMLInputElement | null)?.value || '';
  const password =
    state.password || (document.getElementById('password') as HTMLInputElement | null)?.value || '';

  try {
    const response = await ofetch(`${config.public.apiBaseUrl}/api/v1/auth/login`, {
      method: 'POST',
      credentials: 'include',
      body: {
        username,
        password,
      },
    });

    if (response.message === 'success') {
      // The server sets both an HttpOnly 'jwt' cookie and a non-HttpOnly
      // 'authenticated' cookie via Set-Cookie headers. No need to set
      // cookies manually from JS.
      // Full page reload to ensure all components pick up the auth state.
      // Use config.app.baseURL to respect subdirectory deployments (BASE_URL).
      window.location.href = config.app.baseURL || '/';
    } else {
      errorMsg.value = t('login.authFailed');
    }
  } catch (e) {
    const err = e as { response?: { status?: number } };
    if (err.response?.status === 401) {
      errorMsg.value = t('login.invalidCredentials');
    } else {
      errorMsg.value = t('login.networkError');
    }
  } finally {
    loading.value = false;
  }
}
</script>
