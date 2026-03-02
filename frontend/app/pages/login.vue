<template>
  <div class="flex items-center justify-center min-h-[calc(100vh-100px)]">
    <div
      v-motion
      :initial="{ opacity: 0, scale: 0.96, y: 10 }"
      :enter="{ opacity: 1, scale: 1, y: 0, transition: { type: 'spring', stiffness: 300, damping: 25 } }"
      class="w-full max-w-sm"
    >
      <UiCard
        data-slot="login-card"
        class="overflow-hidden"
      >
        <!-- Header -->
        <UiCardHeader class="pb-2 text-center">
          <div
            data-slot="login-icon"
            class="w-14 h-14 rounded-2xl bg-primary flex items-center justify-center mx-auto mb-5"
          >
            <component
              :is="LockKeyholeIcon"
              class="w-7 h-7 text-primary-foreground"
            />
          </div>
          <UiCardTitle class="text-2xl">
            Welcome Back
          </UiCardTitle>
          <UiCardDescription>Sign in to Capacitarr</UiCardDescription>
        </UiCardHeader>

        <!-- Form -->
        <UiCardContent>
          <form
            class="space-y-5"
            @submit.prevent="onSubmit"
          >
            <div class="space-y-2">
              <UiLabel for="username">
                Username
              </UiLabel>
              <UiInput
                id="username"
                v-model="state.username"
                type="text"
                placeholder="admin"
                autofocus
              />
            </div>

            <div class="space-y-2">
              <UiLabel for="password">
                Password
              </UiLabel>
              <UiInput
                id="password"
                v-model="state.password"
                type="password"
                placeholder="••••••••"
              />
            </div>

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
              <span
                v-if="loading"
                class="flex items-center justify-center gap-2"
              >
                <component
                  :is="LoaderCircleIcon"
                  class="w-4 h-4 animate-spin"
                />
                Signing in...
              </span>
              <span v-else>Sign In</span>
            </UiButton>
          </form>
        </UiCardContent>
      </UiCard>

      <!-- Subtle branding footer -->
      <p class="text-center text-xs text-muted-foreground mt-4">
        Capacitarr — Intelligent Media Capacity Management
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { LockKeyholeIcon, LoaderCircleIcon } from 'lucide-vue-next'
import { ofetch } from 'ofetch'

const config = useRuntimeConfig()

const state = reactive({
  username: '',
  password: ''
})

const loading = ref(false)
const errorMsg = ref('')

async function onSubmit() {
  errorMsg.value = ''
  loading.value = true

  try {
    const response = await ofetch(`${config.public.apiBaseUrl}/api/v1/auth/login`, {
      method: 'POST',
      credentials: 'include',
      body: {
        username: state.username,
        password: state.password
      }
    })

    if (response.message === 'success') {
      // The server sets both an HttpOnly 'jwt' cookie and a non-HttpOnly
      // 'authenticated' cookie via Set-Cookie headers. No need to set
      // cookies manually from JS.
      // Full page reload to ensure all components pick up the auth state
      window.location.href = '/'
    } else {
      errorMsg.value = 'Authentication failed'
    }
  } catch (e) {
    const err = e as { response?: { status?: number } }
    if (err.response?.status === 401) {
      errorMsg.value = 'Invalid username or password'
    } else {
      errorMsg.value = 'Network error connecting to backend'
    }
  } finally {
    loading.value = false
  }
}
</script>
