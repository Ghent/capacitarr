<template>
  <header class="border-b border-slate-200 dark:border-slate-800 bg-white/75 dark:bg-slate-900/75 backdrop-blur-md sticky top-0 z-50">
    <UContainer>
      <div class="flex items-center justify-between h-16">
        <div class="flex items-center gap-2">
          <UIcon name="i-heroicons-circle-stack" class="w-8 h-8 text-indigo-500" />
          <span class="text-xl font-bold tracking-tight text-slate-900 dark:text-white">Capacitarr</span>
        </div>
        
        <div class="flex items-center gap-4">
          <UButton variant="ghost" color="gray" icon="i-heroicons-moon" v-if="!isDark" @click="isDark = true" aria-label="Dark mode" />
          <UButton variant="ghost" color="gray" icon="i-heroicons-sun" v-else @click="isDark = false" aria-label="Light mode" />
          
          <UDropdown :items="userDropdownItems">
            <UAvatar src="https://avatars.githubusercontent.com/u/10?v=4" alt="Avatar" size="sm" />
          </UDropdown>
        </div>
      </div>
    </UContainer>
  </header>
</template>

<script setup lang="ts">
const colorMode = useColorMode()
const isDark = computed({
  get() {
    return colorMode.value === 'dark'
  },
  set() {
    colorMode.preference = colorMode.value === 'dark' ? 'light' : 'dark'
  }
})

const router = useRouter()
const token = useCookie('jwt')

const logout = () => {
  token.value = null
  router.push('/login')
}

const userDropdownItems = [
  [{
    label: 'Profile',
    icon: 'i-heroicons-user'
  }, {
    label: 'Settings',
    icon: 'i-heroicons-cog-8-tooth'
  }],
  [{
    label: 'Logout',
    icon: 'i-heroicons-arrow-right-on-rectangle',
    click: logout
  }]
]
</script>
