<template>
  <div>
    <div class="mb-8 flex flex-col md:flex-row md:items-center justify-between gap-4">
      <div>
        <h1 class="text-3xl font-bold tracking-tight text-slate-900 dark:text-white">Dashboard</h1>
        <p class="text-slate-500 dark:text-slate-400 mt-2">Welcome to your Capacitarr capacity overview.</p>
      </div>

      <div class="flex items-center gap-2">
         <USelectMenu
          v-model="resolution"
          :options="resolutionOptions"
          class="w-40"
          value-attribute="value"
          option-attribute="label"
        />
      </div>
    </div>

    <div class="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
      <UCard>
        <template #header>
          <div class="flex items-center gap-2 text-indigo-500 font-medium">
            <UIcon name="i-heroicons-server" />
            Total Storage Active
          </div>
        </template>
        <div class="text-3xl font-bold text-slate-900 dark:text-white">1,024 GB</div>
        <p class="text-sm text-slate-500 dark:text-slate-400 mt-1">Capacity mapped</p>
      </UCard>

      <UCard>
        <template #header>
          <div class="flex items-center gap-2 text-amber-500 font-medium">
            <UIcon name="i-heroicons-chart-pie" />
            Used Capacity
          </div>
        </template>
        <div class="text-3xl font-bold text-slate-900 dark:text-white">650 GB</div>
        <p class="text-sm text-slate-500 dark:text-slate-400 mt-1">63% utilization</p>
      </UCard>

      <UCard>
        <template #header>
          <div class="flex items-center gap-2 text-emerald-500 font-medium">
            <UIcon name="i-heroicons-chart-bar" />
            Growth Rate
          </div>
        </template>
        <div class="text-3xl font-bold text-slate-900 dark:text-white">+12.5%</div>
        <p class="text-sm text-slate-500 dark:text-slate-400 mt-1">Over last 30 days</p>
      </UCard>
    </div>

    <UCard class="h-96" :ui="{ body: { padding: 'p-0 sm:p-0' } }">
      <CapacityChart :resolution="resolution" />
    </UCard>
  </div>
</template>

<script setup lang="ts">
const token = useCookie('jwt')
const router = useRouter()

const resolutionOptions = [
  { label: 'Real-time (Raw)', value: 'raw' },
  { label: 'Hourly', value: 'hourly' },
  { label: 'Daily', value: 'daily' },
  { label: 'Weekly', value: 'weekly' }
]

const resolution = ref('raw')

// Require authentication for dashboard
onMounted(() => {
  if (!token.value) {
    router.push('/login')
  }
})
</script>
