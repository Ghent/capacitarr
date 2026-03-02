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
    <UiTabs
      default-value="general"
      class="w-full"
    >
      <UiTabsList class="mb-6">
        <UiTabsTrigger value="general">
          {{ $t('settings.general') }}
        </UiTabsTrigger>
        <UiTabsTrigger value="integrations">
          {{ $t('settings.integrations') }}
        </UiTabsTrigger>
        <UiTabsTrigger value="security">
          {{ $t('settings.security') }}
        </UiTabsTrigger>
        <UiTabsTrigger value="advanced">
          {{ $t('settings.advanced') }}
        </UiTabsTrigger>
        <UiTabsTrigger value="notifications">
          {{ $t('settings.notifications') }}
        </UiTabsTrigger>
      </UiTabsList>

      <!-- ═══════════════════════════════════════════════════════
           GENERAL TAB
           ═══════════════════════════════════════════════════════ -->
      <UiTabsContent
        value="general"
        class="space-y-6"
      >
        <!-- Display Preferences Section -->
        <UiCard
          v-motion
          :initial="{ opacity: 0, y: 12 }"
          :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 200 } }"
          class="overflow-hidden"
        >
          <UiCardHeader class="border-b border-border">
            <div class="flex items-center gap-3">
              <div class="w-10 h-10 rounded-lg bg-purple-500 flex items-center justify-center">
                <component
                  :is="MonitorIcon"
                  class="w-5 h-5 text-white"
                />
              </div>
              <div>
                <UiCardTitle class="text-base">
                  {{ $t('settings.display') }}
                </UiCardTitle>
                <UiCardDescription>{{ $t('settings.displayDesc') }}</UiCardDescription>
              </div>
            </div>
          </UiCardHeader>
          <UiCardContent class="pt-5 space-y-5">
            <!-- Timezone -->
            <div class="space-y-1.5">
              <UiLabel>{{ $t('settings.timezone') }}</UiLabel>
              <UiSelect
                :model-value="displayTimezone"
                @update:model-value="(v: string) => setTimezone(String(v))"
              >
                <UiSelectTrigger class="w-full max-w-xs">
                  <UiSelectValue placeholder="Select timezone" />
                </UiSelectTrigger>
                <UiSelectContent>
                  <UiSelectItem value="local">
                    Local (Browser)
                  </UiSelectItem>
                  <UiSelectItem value="UTC">
                    Remote (Server / UTC)
                  </UiSelectItem>
                </UiSelectContent>
              </UiSelect>
            </div>

            <!-- Clock Format -->
            <div class="space-y-1.5">
              <UiLabel>{{ $t('settings.clockFormat') }}</UiLabel>
              <div class="flex gap-2">
                <UiButton
                  :variant="displayClockFormat === '12h' ? 'default' : 'outline'"
                  size="sm"
                  @click="setClockFormat('12h')"
                >
                  {{ $t('settings.clock12h') }}
                </UiButton>
                <UiButton
                  :variant="displayClockFormat === '24h' ? 'default' : 'outline'"
                  size="sm"
                  @click="setClockFormat('24h')"
                >
                  {{ $t('settings.clock24h') }}
                </UiButton>
              </div>
            </div>

            <!-- Theme -->
            <div class="space-y-2">
              <UiLabel>{{ $t('settings.theme') }}</UiLabel>
              <div class="grid grid-cols-3 sm:grid-cols-6 gap-2">
                <button
                  v-for="t in themeList"
                  :key="t.id"
                  class="flex flex-col items-center gap-1.5 rounded-lg border-2 px-3 py-2.5 transition-colors"
                  :class="currentTheme === t.id ? 'border-primary bg-primary/5' : 'border-transparent hover:bg-accent'"
                  @click="setTheme(t.id)"
                >
                  <span
                    class="w-6 h-6 rounded-full"
                    :style="{ backgroundColor: `oklch(0.6 0.2 ${t.hue})` }"
                  />
                  <span class="text-xs font-medium">{{ t.label }}</span>
                </button>
              </div>
            </div>

            <!-- Language -->
            <div class="space-y-1.5">
              <UiLabel>{{ $t('settings.language') }}</UiLabel>
              <p class="text-xs text-muted-foreground mb-1">
                {{ $t('settings.languageDesc') }}
              </p>
              <UiSelect
                :model-value="currentLocale"
                @update:model-value="(v: string) => setLocale(v)"
              >
                <UiSelectTrigger class="w-full max-w-xs">
                  <UiSelectValue :placeholder="$t('settings.language')" />
                </UiSelectTrigger>
                <UiSelectContent>
                  <UiSelectItem
                    v-for="loc in availableLocales"
                    :key="loc.code"
                    :value="loc.code"
                  >
                    {{ loc.name }}
                  </UiSelectItem>
                </UiSelectContent>
              </UiSelect>
            </div>

            <p class="text-xs text-muted-foreground/70">
              {{ $t('settings.themeHint') }}
            </p>
          </UiCardContent>
        </UiCard>

        <!-- Engine Behavior Section -->
        <UiCard
          v-motion
          :initial="{ opacity: 0, y: 12 }"
          :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 300 } }"
          class="overflow-hidden"
        >
          <UiCardHeader class="border-b border-border">
            <div class="flex items-center gap-3">
              <div class="w-10 h-10 rounded-lg bg-primary flex items-center justify-center">
                <component
                  :is="CogIcon"
                  class="w-5 h-5 text-white"
                />
              </div>
              <div>
                <UiCardTitle class="text-base">
                  {{ $t('settings.engineBehavior') }}
                </UiCardTitle>
                <UiCardDescription>{{ $t('settings.engineBehaviorDesc') }}</UiCardDescription>
              </div>
            </div>
          </UiCardHeader>
          <UiCardContent class="pt-5 space-y-6">
            <!-- Execution Mode -->
            <div class="space-y-3">
              <div class="flex items-center gap-2">
                <UiLabel>{{ $t('settings.executionMode') }}</UiLabel>
                <SaveIndicator :status="saveStatus.executionMode" />
              </div>
              <div class="grid grid-cols-1 sm:grid-cols-3 gap-3">
                <button
                  v-for="mode in executionModes"
                  :key="mode.value"
                  data-slot="execution-mode-card"
                  :data-active="engineExecutionMode === mode.value"
                  class="px-4 py-3 rounded-xl border-2 text-left transition-all"
                  :class="engineExecutionMode === mode.value
                    ? 'border-primary bg-primary/5 shadow-sm ring-1 ring-primary/20'
                    : 'border-border hover:border-border'"
                  @click="setExecutionMode(mode.value)"
                >
                  <div
                    class="text-sm font-medium"
                    :class="engineExecutionMode === mode.value ? 'text-primary' : ''"
                  >
                    {{ mode.label }}
                  </div>
                  <div class="text-xs text-muted-foreground mt-0.5">
                    {{ mode.description }}
                  </div>
                </button>
              </div>
            </div>

            <!-- Score Tiebreaker -->
            <div class="space-y-1.5">
              <div class="flex items-center gap-2">
                <UiLabel>{{ $t('settings.scoreTiebreaker') }}</UiLabel>
                <SaveIndicator :status="saveStatus.tiebreaker" />
              </div>
              <p class="text-xs text-muted-foreground mb-1">
                When items have the same score, how should they be ordered?
              </p>
              <UiSelect v-model="engineTiebreakerMethod">
                <UiSelectTrigger class="w-full max-w-xs">
                  <UiSelectValue placeholder="Select tiebreaker" />
                </UiSelectTrigger>
                <UiSelectContent>
                  <UiSelectItem value="size_desc">
                    Largest first (free more space)
                  </UiSelectItem>
                  <UiSelectItem value="size_asc">
                    Smallest first
                  </UiSelectItem>
                  <UiSelectItem value="name_asc">
                    Alphabetical (A → Z)
                  </UiSelectItem>
                  <UiSelectItem value="oldest_first">
                    Oldest in library first
                  </UiSelectItem>
                  <UiSelectItem value="newest_first">
                    Newest in library first
                  </UiSelectItem>
                </UiSelectContent>
              </UiSelect>
            </div>
          </UiCardContent>
        </UiCard>
      </UiTabsContent>

      <!-- ═══════════════════════════════════════════════════════
           INTEGRATIONS TAB
           ═══════════════════════════════════════════════════════ -->
      <UiTabsContent value="integrations">
        <div class="flex justify-end mb-6">
          <UiButton @click="openAddModal">
            <component
              :is="PlusIcon"
              class="w-4 h-4"
            />
            {{ $t('settings.addIntegration') }}
          </UiButton>
        </div>

        <!-- Loading -->
        <div
          v-if="loading"
          class="flex justify-center py-16"
        >
          <component
            :is="LoaderCircleIcon"
            class="w-8 h-8 text-primary animate-spin"
          />
        </div>

        <!-- Empty state -->
        <div
          v-else-if="integrations.length === 0"
          v-motion
          :initial="{ opacity: 0, y: 8 }"
          :enter="{ opacity: 1, y: 0 }"
          class="text-center py-20"
        >
          <component
            :is="HardDriveIcon"
            class="w-16 h-16 text-muted-foreground/40 mx-auto mb-4"
          />
          <h3 class="text-lg font-medium text-foreground mb-2">
            {{ $t('settings.noIntegrations') }}
          </h3>
          <p class="text-muted-foreground mb-6">
            {{ $t('settings.noIntegrationsHelp') }}
          </p>
          <UiButton
            size="lg"
            @click="openAddModal"
          >
            <component
              :is="PlusIcon"
              class="w-4 h-4"
            />
            {{ $t('settings.addFirstIntegration') }}
          </UiButton>
        </div>

        <!-- Integration Cards Grid -->
        <div
          v-else
          class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5"
        >
          <UiCard
            v-for="(integration, idx) in integrations"
            :key="integration.id"
            v-motion
            :initial="{ opacity: 0, y: 12 }"
            :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 80 * idx } }"
            class="overflow-hidden"
          >
            <!-- Card Header -->
            <UiCardHeader class="border-b border-border">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-3">
                  <div :class="['w-10 h-10 rounded-lg flex items-center justify-center', typeColor(integration.type)]">
                    <component
                      :is="typeIcon(integration.type)"
                      class="w-5 h-5 text-white"
                    />
                  </div>
                  <div>
                    <UiCardTitle class="text-base">
                      {{ integration.name }}
                    </UiCardTitle>
                    <span
                      class="text-xs uppercase tracking-wider font-medium"
                      :class="typeTextColor(integration.type)"
                    >
                      {{ integration.type }}
                    </span>
                  </div>
                </div>
                <UiBadge :variant="integration.enabled ? 'default' : 'secondary'">
                  {{ integration.enabled ? $t('common.active') : $t('common.disabled') }}
                </UiBadge>
              </div>
            </UiCardHeader>

            <!-- Card Body -->
            <UiCardContent class="pt-4 space-y-2 text-sm text-muted-foreground">
              <div class="flex items-center gap-2">
                <component
                  :is="LinkIcon"
                  class="w-3.5 h-3.5 shrink-0"
                />
                <span class="truncate">{{ integration.url }}</span>
              </div>
              <div class="flex items-center gap-2">
                <component
                  :is="KeyIcon"
                  class="w-3.5 h-3.5 shrink-0"
                />
                <span class="font-mono text-xs">{{ integration.apiKey }}</span>
              </div>
              <div
                v-if="integration.lastSync"
                class="flex items-center gap-2"
              >
                <component
                  :is="ClockIcon"
                  class="w-3.5 h-3.5 shrink-0"
                />
                <span>Synced {{ formatRelativeTime(integration.lastSync) }}</span>
              </div>
              <div
                v-if="integration.lastError"
                class="flex items-center gap-2 text-red-500"
              >
                <component
                  :is="AlertTriangleIcon"
                  class="w-3.5 h-3.5 shrink-0"
                />
                <span class="text-xs">{{ integration.lastError }}</span>
              </div>
            </UiCardContent>

            <!-- Card Footer -->
            <UiCardFooter class="border-t border-border flex items-center justify-between">
              <div class="flex gap-2">
                <UiButton
                  variant="outline"
                  size="sm"
                  @click="testConnection(integration)"
                >
                  {{ $t('common.test') }}
                </UiButton>
                <UiButton
                  variant="outline"
                  size="sm"
                  @click="openEditModal(integration)"
                >
                  {{ $t('common.edit') }}
                </UiButton>
              </div>
              <UiButton
                variant="destructive"
                size="sm"
                @click="deleteIntegration(integration)"
              >
                {{ $t('common.delete') }}
              </UiButton>
            </UiCardFooter>
          </UiCard>
        </div>
      </UiTabsContent>

      <!-- ═══════════════════════════════════════════════════════
           SECURITY TAB
           ═══════════════════════════════════════════════════════ -->
      <UiTabsContent
        value="security"
        class="space-y-6"
      >
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
                <component
                  :is="UserIcon"
                  class="w-5 h-5 text-white"
                />
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
            <UiAlert
              v-if="usernameError"
              variant="destructive"
            >
              <UiAlertDescription>{{ usernameError }}</UiAlertDescription>
            </UiAlert>
            <div>
              <UiButton
                :disabled="savingUsername"
                @click="changeUsername"
              >
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
                <component
                  :is="ShieldIcon"
                  class="w-5 h-5 text-white"
                />
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
            <UiAlert
              v-if="passwordError"
              variant="destructive"
            >
              <UiAlertDescription>{{ passwordError }}</UiAlertDescription>
            </UiAlert>
            <div>
              <UiButton
                :disabled="savingPassword"
                @click="changePassword"
              >
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
                <component
                  :is="KeyIcon"
                  class="w-5 h-5 text-white"
                />
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
            <div
              v-if="apiKey"
              class="flex items-center gap-2"
            >
              <code class="flex-1 px-3 py-2 bg-muted rounded-lg text-sm font-mono break-all">{{ apiKey }}</code>
              <UiButton
                variant="outline"
                size="sm"
                @click="copyApiKey"
              >
                {{ $t('common.copy') }}
              </UiButton>
            </div>
            <div
              v-else
              class="text-sm text-muted-foreground"
            >
              {{ $t('settings.noApiKey') }}
            </div>
            <div>
              <UiButton
                :disabled="generatingApiKey"
                @click="generateApiKey"
              >
                {{ apiKey ? $t('settings.regenerateApiKey') : $t('settings.generateApiKey') }}
              </UiButton>
            </div>
          </UiCardContent>
        </UiCard>
      </UiTabsContent>

      <!-- ═══════════════════════════════════════════════════════
           ADVANCED TAB
           ═══════════════════════════════════════════════════════ -->
      <UiTabsContent
        value="advanced"
        class="space-y-6"
      >
        <!-- Poll Interval -->
        <UiCard
          v-motion
          :initial="{ opacity: 0, y: 12 }"
          :enter="{ opacity: 1, y: 0 }"
          class="overflow-hidden"
        >
          <UiCardHeader class="border-b border-border">
            <div class="flex items-center gap-3">
              <div class="w-10 h-10 rounded-lg bg-blue-500 flex items-center justify-center">
                <component
                  :is="TimerIcon"
                  class="w-5 h-5 text-white"
                />
              </div>
              <div>
                <UiCardTitle class="text-base">
                  {{ $t('settings.pollInterval') }}
                </UiCardTitle>
                <UiCardDescription>{{ $t('settings.pollIntervalDesc') }}</UiCardDescription>
              </div>
            </div>
          </UiCardHeader>
          <UiCardContent class="pt-5">
            <div class="space-y-1.5">
              <div class="flex items-center gap-2">
                <UiLabel>{{ $t('settings.interval') }}</UiLabel>
                <SaveIndicator :status="saveStatus.pollInterval" />
              </div>
              <UiSelect v-model="pollIntervalStr">
                <UiSelectTrigger class="w-full max-w-xs">
                  <UiSelectValue placeholder="Select interval" />
                </UiSelectTrigger>
                <UiSelectContent>
                  <UiSelectItem value="30">
                    30 seconds
                  </UiSelectItem>
                  <UiSelectItem value="60">
                    1 minute
                  </UiSelectItem>
                  <UiSelectItem value="300">
                    5 minutes (default)
                  </UiSelectItem>
                  <UiSelectItem value="900">
                    15 minutes
                  </UiSelectItem>
                  <UiSelectItem value="1800">
                    30 minutes
                  </UiSelectItem>
                  <UiSelectItem value="3600">
                    1 hour
                  </UiSelectItem>
                </UiSelectContent>
              </UiSelect>
              <p class="text-xs text-muted-foreground/70">
                {{ $t('settings.pollIntervalHint') }}
              </p>
            </div>
          </UiCardContent>
        </UiCard>

        <!-- Log Level -->
        <UiCard
          v-motion
          :initial="{ opacity: 0, y: 12 }"
          :enter="{ opacity: 1, y: 0, transition: { delay: 50 } }"
          class="overflow-hidden"
        >
          <UiCardHeader class="border-b border-border">
            <div class="flex items-center gap-3">
              <div class="w-10 h-10 rounded-lg bg-emerald-600 flex items-center justify-center">
                <component
                  :is="TerminalIcon"
                  class="w-5 h-5 text-white"
                />
              </div>
              <div>
                <UiCardTitle class="text-base">
                  {{ $t('settings.logLevel') }}
                </UiCardTitle>
                <UiCardDescription>{{ $t('settings.logLevelDesc') }}</UiCardDescription>
              </div>
            </div>
          </UiCardHeader>
          <UiCardContent class="pt-5">
            <div class="space-y-1.5">
              <div class="flex items-center gap-2">
                <UiLabel>{{ $t('settings.logLevel') }}</UiLabel>
                <SaveIndicator :status="saveStatus.logLevel" />
              </div>
              <UiSelect v-model="logLevel">
                <UiSelectTrigger class="w-full max-w-xs">
                  <UiSelectValue placeholder="Select log level" />
                </UiSelectTrigger>
                <UiSelectContent>
                  <UiSelectItem value="debug">
                    Debug
                  </UiSelectItem>
                  <UiSelectItem value="info">
                    Info (default)
                  </UiSelectItem>
                  <UiSelectItem value="warn">
                    Warn
                  </UiSelectItem>
                  <UiSelectItem value="error">
                    Error
                  </UiSelectItem>
                </UiSelectContent>
              </UiSelect>
              <p class="text-xs text-muted-foreground/70">
                {{ $t('settings.logLevelHint') }}
              </p>
            </div>
          </UiCardContent>
        </UiCard>

        <!-- Data Management -->
        <UiCard
          v-motion
          :initial="{ opacity: 0, y: 12 }"
          :enter="{ opacity: 1, y: 0, transition: { delay: 150 } }"
          class="overflow-hidden"
        >
          <UiCardHeader class="border-b border-border">
            <div class="flex items-center gap-3">
              <div class="w-10 h-10 rounded-lg bg-primary flex items-center justify-center">
                <component
                  :is="DatabaseIcon"
                  class="w-5 h-5 text-white"
                />
              </div>
              <div>
                <UiCardTitle class="text-base">
                  {{ $t('settings.dataManagement') }}
                </UiCardTitle>
                <UiCardDescription>{{ $t('settings.dataManagementDesc') }}</UiCardDescription>
              </div>
            </div>
          </UiCardHeader>
          <UiCardContent class="pt-5 space-y-4">
            <div class="space-y-1.5">
              <div class="flex items-center gap-2">
                <UiLabel>{{ $t('settings.auditRetention') }}</UiLabel>
                <SaveIndicator :status="saveStatus.retention" />
              </div>
              <UiSelect v-model="retentionStr">
                <UiSelectTrigger class="w-full max-w-xs">
                  <UiSelectValue placeholder="Select retention" />
                </UiSelectTrigger>
                <UiSelectContent>
                  <UiSelectItem value="7">
                    7 days
                  </UiSelectItem>
                  <UiSelectItem value="14">
                    14 days
                  </UiSelectItem>
                  <UiSelectItem value="30">
                    30 days (default)
                  </UiSelectItem>
                  <UiSelectItem value="60">
                    60 days
                  </UiSelectItem>
                  <UiSelectItem value="90">
                    90 days
                  </UiSelectItem>
                  <UiSelectItem value="180">
                    180 days
                  </UiSelectItem>
                  <UiSelectItem value="365">
                    365 days
                  </UiSelectItem>
                  <UiSelectItem value="0">
                    Indefinite
                  </UiSelectItem>
                </UiSelectContent>
              </UiSelect>
            </div>
            <UiAlert
              v-if="retentionDays === 0"
              variant="destructive"
            >
              <UiAlertTitle>{{ $t('common.warning') }}</UiAlertTitle>
              <UiAlertDescription>
                {{ $t('settings.retentionWarning') }}
              </UiAlertDescription>
            </UiAlert>
          </UiCardContent>
        </UiCard>

        <!-- Default Disk Group Thresholds -->
        <UiCard
          v-motion
          :initial="{ opacity: 0, y: 12 }"
          :enter="{ opacity: 1, y: 0, transition: { delay: 200 } }"
          class="overflow-hidden"
        >
          <UiCardHeader class="border-b border-border">
            <div class="flex items-center gap-3">
              <div class="w-10 h-10 rounded-lg bg-amber-500 flex items-center justify-center">
                <component
                  :is="HardDriveIcon"
                  class="w-5 h-5 text-white"
                />
              </div>
              <div>
                <UiCardTitle class="text-base">
                  Default Disk Group Thresholds
                </UiCardTitle>
                <UiCardDescription>Applied when new disk groups are discovered</UiCardDescription>
              </div>
            </div>
          </UiCardHeader>
          <UiCardContent class="pt-5 space-y-4">
            <div class="grid grid-cols-2 gap-4 max-w-sm">
              <div class="space-y-1.5">
                <div class="flex items-center gap-2">
                  <UiLabel>Threshold %</UiLabel>
                  <SaveIndicator :status="saveStatus.defaultThreshold" />
                </div>
                <UiInput
                  v-model.number="defaultThreshold"
                  type="number"
                  min="50"
                  max="99"
                  @change="autoSavePreference('defaultThreshold', 'defaultThresholdPct', defaultThreshold)"
                />
              </div>
              <div class="space-y-1.5">
                <div class="flex items-center gap-2">
                  <UiLabel>Target %</UiLabel>
                  <SaveIndicator :status="saveStatus.defaultTarget" />
                </div>
                <UiInput
                  v-model.number="defaultTarget"
                  type="number"
                  min="50"
                  max="98"
                  @change="autoSavePreference('defaultTarget', 'defaultTargetPct', defaultTarget)"
                />
              </div>
            </div>
            <p class="text-xs text-muted-foreground/70">
              Threshold triggers cleanup. Target is the desired usage level after cleanup.
              Threshold must be greater than target.
            </p>
          </UiCardContent>
        </UiCard>

        <!-- Danger Zone -->
        <UiCard
          v-motion
          :initial="{ opacity: 0, y: 12 }"
          :enter="{ opacity: 1, y: 0, transition: { delay: 300 } }"
          class="overflow-hidden border-destructive/50"
        >
          <UiCardHeader class="border-b border-destructive/30">
            <div class="flex items-center gap-3">
              <div class="w-10 h-10 rounded-lg bg-destructive flex items-center justify-center">
                <component
                  :is="AlertTriangleIcon"
                  class="w-5 h-5 text-white"
                />
              </div>
              <div>
                <UiCardTitle class="text-base text-destructive">
                  Danger Zone
                </UiCardTitle>
                <UiCardDescription>
                  Destructive actions that cannot be easily undone.
                </UiCardDescription>
              </div>
            </div>
          </UiCardHeader>
          <UiCardContent class="pt-5 space-y-6">
            <!-- Reset Scraped Data -->
            <div class="space-y-2">
              <p class="text-sm font-medium text-foreground">
                Reset Scraped Data
              </p>
              <p class="text-sm text-muted-foreground">
                Clear all audit logs, capacity history, engine stats, and disk group data.
                Integration configurations, preferences, and custom rules are preserved.
                Data will be re-populated on the next engine run.
              </p>
              <UiButton
                variant="destructive"
                :disabled="resettingData"
                @click="showResetDialog = true"
              >
                {{ resettingData ? 'Clearing…' : 'Clear All Scraped Data' }}
              </UiButton>
            </div>

            <UiSeparator />

            <!-- Deletion Safety -->
            <div class="space-y-3">
              <p class="text-sm font-medium text-foreground">
                {{ $t('settings.deletionSafety') }}
              </p>
              <p class="text-sm text-muted-foreground">
                {{ $t('settings.deletionSafetyExplain') }}
              </p>
              <UiAlert
                v-if="deletionsEnabled"
                variant="destructive"
              >
                <component
                  :is="Trash2Icon"
                  class="w-4 h-4"
                />
                <UiAlertTitle>{{ $t('settings.deletionsActiveAlert') }}</UiAlertTitle>
                <UiAlertDescription>
                  {{ $t('settings.deletionsActiveAlertDesc') }}
                </UiAlertDescription>
              </UiAlert>
              <!-- Toggle with label -->
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-3">
                  <div @click.prevent="onDeletionToggle(!deletionsEnabled)">
                    <UiSwitch
                      :checked="deletionsEnabled"
                      aria-label="Enable actual file deletion"
                      :class="deletionsEnabled ? '[&[data-state=checked]]:bg-destructive' : ''"
                    />
                  </div>
                  <div>
                    <span class="text-sm font-medium">
                      {{ $t('settings.enableDeletions') }}
                    </span>
                    <p
                      v-if="deletionsEnabled"
                      class="text-xs font-medium text-red-600 dark:text-red-400"
                    >
                      Current status: Deletions are active!
                    </p>
                    <p
                      v-else
                      class="text-xs font-medium text-amber-600 dark:text-amber-400"
                    >
                      Current status: All deletions are logged and simulated
                    </p>
                  </div>
                </div>
                <SaveIndicator :status="saveStatus.deletionsEnabled" />
              </div>
            </div>
          </UiCardContent>
        </UiCard>
      </UiTabsContent>

      <!-- ═══════════════════════════════════════════════════════
           NOTIFICATIONS TAB
           ═══════════════════════════════════════════════════════ -->
      <UiTabsContent value="notifications">
        <div class="flex justify-end mb-6">
          <UiButton @click="openAddChannelModal">
            <component
              :is="PlusIcon"
              class="w-4 h-4"
            />
            Add Channel
          </UiButton>
        </div>

        <!-- Loading -->
        <div
          v-if="channelsLoading"
          class="flex justify-center py-16"
        >
          <component
            :is="LoaderCircleIcon"
            class="w-8 h-8 text-primary animate-spin"
          />
        </div>

        <!-- Empty state -->
        <div
          v-else-if="channels.length === 0"
          v-motion
          :initial="{ opacity: 0, y: 8 }"
          :enter="{ opacity: 1, y: 0 }"
          class="text-center py-20"
        >
          <component
            :is="BellIcon"
            class="w-16 h-16 text-muted-foreground/40 mx-auto mb-4"
          />
          <h3 class="text-lg font-medium text-foreground mb-2">
            No notification channels configured
          </h3>
          <p class="text-muted-foreground mb-6">
            Set up Discord, Slack, or in-app notifications for engine events.
          </p>
          <UiButton
            size="lg"
            @click="openAddChannelModal"
          >
            <component
              :is="PlusIcon"
              class="w-4 h-4"
            />
            Add Your First Channel
          </UiButton>
        </div>

        <!-- Channel Cards Grid -->
        <div
          v-else
          class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5"
        >
          <UiCard
            v-for="(channel, idx) in channels"
            :key="channel.id"
            v-motion
            :initial="{ opacity: 0, y: 12 }"
            :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 80 * idx } }"
            class="overflow-hidden"
          >
            <!-- Card Header -->
            <UiCardHeader class="border-b border-border">
              <div class="flex items-center justify-between">
                <div class="flex items-center gap-3">
                  <div :class="['w-10 h-10 rounded-lg flex items-center justify-center', channelTypeColor(channel.type)]">
                    <component
                      :is="channelTypeIcon(channel.type)"
                      class="w-5 h-5 text-white"
                    />
                  </div>
                  <div>
                    <UiCardTitle class="text-base">
                      {{ channel.name }}
                    </UiCardTitle>
                    <UiBadge
                      :variant="channel.type === 'inapp' ? 'secondary' : 'outline'"
                      class="mt-1"
                    >
                      {{ channelTypeLabel(channel.type) }}
                    </UiBadge>
                  </div>
                </div>
                <UiSwitch
                  :checked="channel.enabled"
                  @update:checked="(val: boolean) => toggleChannelEnabled(channel, val)"
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
                    :checked="channel.onThresholdBreach"
                    size="sm"
                    @update:checked="(val: boolean) => updateChannelEvent(channel, 'onThresholdBreach', val)"
                  />
                  <span>Threshold Breach</span>
                </label>
                <label class="flex items-center gap-2 text-sm">
                  <UiSwitch
                    :checked="channel.onDeletionExecuted"
                    size="sm"
                    @update:checked="(val: boolean) => updateChannelEvent(channel, 'onDeletionExecuted', val)"
                  />
                  <span>Deletion Executed</span>
                </label>
                <label class="flex items-center gap-2 text-sm">
                  <UiSwitch
                    :checked="channel.onEngineError"
                    size="sm"
                    @update:checked="(val: boolean) => updateChannelEvent(channel, 'onEngineError', val)"
                  />
                  <span>Engine Error</span>
                </label>
                <label class="flex items-center gap-2 text-sm">
                  <UiSwitch
                    :checked="channel.onEngineComplete"
                    size="sm"
                    @update:checked="(val: boolean) => updateChannelEvent(channel, 'onEngineComplete', val)"
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
                <UiButton
                  variant="outline"
                  size="sm"
                  @click="openEditChannelModal(channel)"
                >
                  Edit
                </UiButton>
              </div>
              <UiButton
                variant="destructive"
                size="sm"
                @click="deleteChannel(channel)"
              >
                Delete
              </UiButton>
            </UiCardFooter>
          </UiCard>
        </div>
      </UiTabsContent>
    </UiTabs>

    <!-- Data Reset Confirmation Dialog -->
    <UiDialog
      :open="showResetDialog"
      @update:open="(val: boolean) => { showResetDialog = val }"
    >
      <UiDialogContent class="max-w-md">
        <UiDialogHeader>
          <UiDialogTitle>Are you sure?</UiDialogTitle>
          <UiDialogDescription>
            This will permanently delete all audit logs, capacity history, and engine statistics. This action cannot be undone.
          </UiDialogDescription>
        </UiDialogHeader>
        <UiDialogFooter class="flex gap-2 justify-end">
          <UiButton
            variant="outline"
            @click="showResetDialog = false"
          >
            Cancel
          </UiButton>
          <UiButton
            variant="destructive"
            :disabled="resettingData"
            @click="confirmResetData"
          >
            {{ resettingData ? 'Clearing…' : 'Yes, clear all data' }}
          </UiButton>
        </UiDialogFooter>
      </UiDialogContent>
    </UiDialog>

    <!-- Deletion Confirmation Dialog -->
    <UiDialog
      :open="showDeletionConfirmDialog"
      @update:open="(val: boolean) => { if (!val) cancelEnableDeletions() }"
    >
      <UiDialogContent class="max-w-md">
        <UiDialogHeader>
          <UiDialogTitle>Enable Actual Deletions?</UiDialogTitle>
          <UiDialogDescription>
            This will allow Capacitarr to permanently delete media files from your storage.
            Deleted files cannot be recovered. Make sure you have backups before proceeding.
          </UiDialogDescription>
        </UiDialogHeader>
        <div class="py-2">
          <UiAlert variant="destructive">
            <component
              :is="AlertTriangleIcon"
              class="w-4 h-4"
            />
            <UiAlertTitle>Warning</UiAlertTitle>
            <UiAlertDescription>
              Once enabled, any media flagged by the scoring engine will be permanently removed from disk. This action cannot be undone. Make sure your scoring rules and thresholds are configured correctly before enabling.
            </UiAlertDescription>
          </UiAlert>
        </div>
        <UiDialogFooter class="flex gap-2 justify-end">
          <UiButton
            variant="outline"
            @click="cancelEnableDeletions"
          >
            Cancel
          </UiButton>
          <UiButton
            variant="destructive"
            @click="confirmEnableDeletions"
          >
            Enable Deletions
          </UiButton>
        </UiDialogFooter>
      </UiDialogContent>
    </UiDialog>

    <!-- Notification Channel Modal -->
    <UiDialog
      :open="showChannelModal"
      @update:open="(val: boolean) => { showChannelModal = val }"
    >
      <UiDialogContent class="max-w-md">
        <UiDialogHeader>
          <UiDialogTitle>
            {{ editingChannel ? 'Edit Channel' : 'Add Notification Channel' }}
          </UiDialogTitle>
        </UiDialogHeader>

        <form
          class="space-y-4"
          @submit.prevent="onChannelSubmit"
        >
          <div class="space-y-1.5">
            <UiLabel>Type</UiLabel>
            <UiSelect
              v-model="channelForm.type"
              :disabled="!!editingChannel"
            >
              <UiSelectTrigger class="w-full">
                <UiSelectValue placeholder="Select type" />
              </UiSelectTrigger>
              <UiSelectContent>
                <UiSelectItem value="discord">
                  Discord
                </UiSelectItem>
                <UiSelectItem value="slack">
                  Slack
                </UiSelectItem>
                <UiSelectItem value="inapp">
                  In-App
                </UiSelectItem>
              </UiSelectContent>
            </UiSelect>
          </div>

          <div class="space-y-1.5">
            <UiLabel>Name</UiLabel>
            <UiInput
              v-model="channelForm.name"
              type="text"
              placeholder="e.g. My Discord Alerts"
            />
          </div>

          <div
            v-if="channelForm.type !== 'inapp'"
            class="space-y-1.5"
          >
            <UiLabel>Webhook URL</UiLabel>
            <UiInput
              v-model="channelForm.webhookUrl"
              type="text"
              :placeholder="channelForm.type === 'discord'
                ? 'https://discord.com/api/webhooks/...'
                : 'https://hooks.slack.com/services/...'"
            />
          </div>

          <div class="space-y-3">
            <p class="text-xs font-medium text-muted-foreground uppercase tracking-wider">
              Event Subscriptions
            </p>
            <label class="flex items-center gap-2 text-sm">
              <UiSwitch
                :checked="channelForm.onThresholdBreach"
                @update:checked="(val: boolean) => { channelForm.onThresholdBreach = val }"
              />
              <span>Threshold Breach</span>
            </label>
            <label class="flex items-center gap-2 text-sm">
              <UiSwitch
                :checked="channelForm.onDeletionExecuted"
                @update:checked="(val: boolean) => { channelForm.onDeletionExecuted = val }"
              />
              <span>Deletion Executed</span>
            </label>
            <label class="flex items-center gap-2 text-sm">
              <UiSwitch
                :checked="channelForm.onEngineError"
                @update:checked="(val: boolean) => { channelForm.onEngineError = val }"
              />
              <span>Engine Error</span>
            </label>
            <label class="flex items-center gap-2 text-sm">
              <UiSwitch
                :checked="channelForm.onEngineComplete"
                @update:checked="(val: boolean) => { channelForm.onEngineComplete = val }"
              />
              <span>Engine Complete</span>
            </label>
          </div>

          <!-- Error -->
          <UiAlert
            v-if="channelFormError"
            variant="destructive"
          >
            <UiAlertDescription>{{ channelFormError }}</UiAlertDescription>
          </UiAlert>
        </form>

        <UiDialogFooter class="flex gap-2 justify-end">
          <UiButton
            variant="ghost"
            @click="showChannelModal = false"
          >
            Cancel
          </UiButton>
          <UiButton
            :disabled="savingChannel"
            @click="onChannelSubmit"
          >
            {{ editingChannel ? 'Save' : 'Add' }}
          </UiButton>
        </UiDialogFooter>
      </UiDialogContent>
    </UiDialog>

    <!-- Integration Modal -->
    <UiDialog
      :open="showModal"
      @update:open="(val: boolean) => { showModal = val }"
    >
      <UiDialogContent class="max-w-md">
        <UiDialogHeader>
          <UiDialogTitle>
            {{ editingIntegration ? 'Edit Integration' : 'Add Integration' }}
          </UiDialogTitle>
        </UiDialogHeader>

        <form
          class="space-y-4"
          @submit.prevent="onSubmit"
        >
          <div class="space-y-1.5">
            <UiLabel>Type</UiLabel>
            <UiSelect
              v-model="formState.type"
              :disabled="!!editingIntegration"
            >
              <UiSelectTrigger class="w-full">
                <UiSelectValue placeholder="Select type" />
              </UiSelectTrigger>
              <UiSelectContent>
                <UiSelectItem value="sonarr">
                  Sonarr
                </UiSelectItem>
                <UiSelectItem value="radarr">
                  Radarr
                </UiSelectItem>
                <UiSelectItem value="lidarr">
                  Lidarr
                </UiSelectItem>
                <UiSelectItem value="readarr">
                  Readarr
                </UiSelectItem>
                <UiSelectItem value="plex">
                  Plex
                </UiSelectItem>
                <UiSelectItem value="jellyfin">
                  Jellyfin
                </UiSelectItem>
                <UiSelectItem value="emby">
                  Emby
                </UiSelectItem>
                <UiSelectItem value="tautulli">
                  Tautulli
                </UiSelectItem>
                <UiSelectItem value="overseerr">
                  Overseerr
                </UiSelectItem>
              </UiSelectContent>
            </UiSelect>
          </div>

          <div class="space-y-1.5">
            <UiLabel>Name</UiLabel>
            <UiInput
              v-model="formState.name"
              type="text"
              :placeholder="namePlaceholder"
            />
          </div>

          <div class="space-y-1.5">
            <UiLabel>URL</UiLabel>
            <UiInput
              v-model="formState.url"
              type="text"
              :placeholder="urlPlaceholder"
            />
            <p class="text-xs text-muted-foreground/70">
              {{ urlHelp }}
            </p>
          </div>

          <div class="space-y-1.5">
            <UiLabel>
              {{ formState.type === 'plex' ? 'Plex Token' : 'API Key' }}
            </UiLabel>
            <UiInput
              v-model="formState.apiKey"
              :type="editingIntegration && formState.apiKey.includes('•') ? 'text' : 'password'"
              :placeholder="editingIntegration ? 'Enter new API key to change, or leave as-is' : 'Enter API key or token'"
              @focus="onApiKeyFocus"
            />

            <!-- Plex OAuth Sign-in Button -->
            <template v-if="formState.type === 'plex'">
              <div class="pt-1 space-y-2">
                <UiButton
                  type="button"
                  class="w-full text-black font-semibold"
                  style="background-color: #E5A00D;"
                  :disabled="plexAuthLoading"
                  @click="startPlexAuth"
                >
                  <template v-if="plexAuthLoading">
                    <component
                      :is="LoaderCircleIcon"
                      class="w-4 h-4 animate-spin"
                    />
                    Waiting for Plex authorization…
                  </template>
                  <template v-else>
                    <component
                      :is="LogInIcon"
                      class="w-4 h-4"
                    />
                    Sign in with Plex
                  </template>
                </UiButton>
                <p class="text-xs text-muted-foreground/70">
                  Opens Plex in a new window to authorize Capacitarr
                </p>
              </div>

              <UiSeparator class="my-1" />

              <p class="text-xs text-muted-foreground/70">
                Or enter your token manually: open any library item in Plex Web → Get Info → View XML → look for <code class="font-mono text-[11px]">X-Plex-Token</code> in the URL.
              </p>
            </template>
          </div>

          <!-- Error -->
          <UiAlert
            v-if="formError"
            variant="destructive"
          >
            <UiAlertDescription>{{ formError }}</UiAlertDescription>
          </UiAlert>
        </form>

        <UiDialogFooter class="flex items-center justify-between">
          <UiButton
            variant="outline"
            @click="testFormConnection"
          >
            Test Connection
          </UiButton>
          <div class="flex gap-2">
            <UiButton
              variant="ghost"
              @click="showModal = false"
            >
              Cancel
            </UiButton>
            <UiButton
              :disabled="saving"
              @click="onSubmit"
            >
              {{ editingIntegration ? 'Save' : 'Add' }}
            </UiButton>
          </div>
        </UiDialogFooter>
      </UiDialogContent>
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
import {
  PlusIcon, HardDriveIcon, LoaderCircleIcon,
  LinkIcon, KeyIcon, ClockIcon, AlertTriangleIcon,
  TvIcon, FilmIcon, PlayCircleIcon, ServerIcon,
  DatabaseIcon, MonitorIcon, ActivityIcon,
  InboxIcon, MusicIcon, TimerIcon, ShieldIcon,
  CheckIcon, UserIcon, BookOpenIcon, MonitorPlayIcon,
  CogIcon, Trash2Icon, LogInIcon,
  BellIcon, MessageSquareIcon, HashIcon, TerminalIcon
} from 'lucide-vue-next'
import { formatRelativeTime } from '~/utils/format'
import type { IntegrationConfig, NotificationChannel, PreferenceSet, ConnectionTestResult, ApiKeyResponse, ApiError } from '~/types/api'

// ─── i18n ─────────────────────────────────────────────────────────────────────
const { locale: currentLocale, setLocale, locales } = useI18n()
const availableLocales = computed(() =>
  (locales.value as Array<{ code: string; name: string }>).map(l => ({ code: l.code, name: l.name }))
)

// ─── SaveIndicator functional component ──────────────────────────────────────
const SaveIndicator = defineComponent({
  props: {
    status: { type: String as () => 'idle' | 'saving' | 'saved' | 'error', default: 'idle' }
  },
  setup(props) {
    return () => {
      if (props.status === 'idle') return null
      if (props.status === 'saving') {
        return h('span', { class: 'inline-flex items-center gap-1 text-xs text-muted-foreground animate-pulse' }, '…saving')
      }
      if (props.status === 'saved') {
        return h('span', {
          class: 'inline-flex items-center gap-1 text-xs text-emerald-500 font-medium transition-opacity'
        }, [
          h(CheckIcon, { class: 'w-3 h-3' }),
          'Saved'
        ])
      }
      if (props.status === 'error') {
        return h('span', { class: 'inline-flex items-center gap-1 text-xs text-red-500 font-medium' }, '✕ Failed')
      }
      return null
    }
  }
})

const api = useApi()
const { timezone: displayTimezone, clockFormat: displayClockFormat, setTimezone, setClockFormat } = useDisplayPrefs()
const { theme: currentTheme, setTheme, themes: themeList } = useTheme()

const loading = ref(true)
const integrations = ref<IntegrationConfig[]>([])
const showModal = ref(false)
const editingIntegration = ref<IntegrationConfig | null>(null)
const saving = ref(false)
const formError = ref('')
const { addToast } = useToast()

// Engine behavior state
const engineExecutionMode = ref('dry-run')
const engineTiebreakerMethod = ref('size_desc')

const executionModes = [
  { value: 'dry-run', label: 'Dry Run', description: 'Log only, no deletions' },
  { value: 'approval', label: 'Approval', description: 'Queue for manual approval' },
  { value: 'auto', label: 'Automatic', description: 'Delete automatically' }
]

// General settings state
const retentionDays = ref(30)
const pollIntervalSeconds = ref(300)
const logLevel = ref('info')

// String wrappers for UiSelect (which requires string values)
const pollIntervalStr = computed({
  get: () => String(pollIntervalSeconds.value),
  set: (val: string) => { pollIntervalSeconds.value = Number(val) }
})

const retentionStr = computed({
  get: () => String(retentionDays.value),
  set: (val: string) => { retentionDays.value = Number(val) }
})

// Per-field save status for inline feedback
const saveStatus = reactive<Record<string, 'idle' | 'saving' | 'saved' | 'error'>>({
  pollInterval: 'idle',
  retention: 'idle',
  defaultThreshold: 'idle',
  defaultTarget: 'idle',
  executionMode: 'idle',
  tiebreaker: 'idle',
  deletionsEnabled: 'idle',
  logLevel: 'idle'
})

// Password change state
const passwordForm = reactive({
  currentPassword: '',
  newPassword: '',
  confirmPassword: ''
})
const passwordError = ref('')
const savingPassword = ref(false)

// Username change state
const usernameForm = reactive({
  newUsername: '',
  password: ''
})
const usernameError = ref('')
const savingUsername = ref(false)

// Default threshold state
const defaultThreshold = ref(85)
const defaultTarget = ref(75)

// Deletion safety state
const deletionsEnabled = ref(true)
const showDeletionConfirmDialog = ref(false)

// Data reset state
const showResetDialog = ref(false)
const resettingData = ref(false)

// API Key state
const apiKey = ref('')
const generatingApiKey = ref(false)

const formState = reactive({
  type: 'sonarr',
  name: '',
  url: '',
  apiKey: ''
})

// ─── Plex OAuth PIN Auth ─────────────────────────────────────────────────────
const plexAuthLoading = ref(false)
let plexAuthPopup: Window | null = null
let plexAuthAborted = false

async function startPlexAuth() {
  plexAuthLoading.value = true
  plexAuthAborted = false

  try {
    // 1. Create PIN
    const pinResponse = await api('/api/v1/integrations/plex/auth/pin', {
      method: 'POST'
    }) as { id: number, code: string }

    // 2. Open popup
    plexAuthPopup = window.open(
      `https://app.plex.tv/auth#?clientID=capacitarr&code=${pinResponse.code}&forwardUrl=close&context%5Bdevice%5D%5Bproduct%5D=Capacitarr`,
      'PlexAuth',
      'width=600,height=700,scrollbars=yes'
    )

    // 3. Poll for claim (150 attempts × 2s = 5 min timeout)
    const maxAttempts = 150
    for (let i = 0; i < maxAttempts; i++) {
      await new Promise(resolve => setTimeout(resolve, 2000))

      // Check if aborted or popup closed by user
      if (plexAuthAborted) {
        break
      }

      if (plexAuthPopup?.closed) {
        addToast('Plex authorization cancelled', 'info')
        break
      }

      try {
        const result = await api(`/api/v1/integrations/plex/auth/pin/${pinResponse.id}`) as {
          claimed: boolean
          authToken?: string
        }

        if (result.claimed && result.authToken) {
          plexAuthPopup?.close()
          formState.apiKey = result.authToken
          addToast('Plex authorized successfully!', 'success')
          break
        }
      } catch {
        // Polling error — continue trying
      }

      // Timeout on last attempt
      if (i === maxAttempts - 1) {
        plexAuthPopup?.close()
        addToast('Plex authorization timed out — please try again', 'error')
      }
    }
  } catch {
    addToast('Failed to start Plex authorization', 'error')
  } finally {
    plexAuthLoading.value = false
    plexAuthPopup = null
  }
}

const namePlaceholder = computed(() => {
  const defaults: Record<string, string> = {
    sonarr: 'My Sonarr', radarr: 'My Radarr', lidarr: 'My Lidarr',
    readarr: 'My Readarr', plex: 'My Plex', jellyfin: 'My Jellyfin',
    emby: 'My Emby', tautulli: 'My Tautulli', overseerr: 'My Overseerr'
  }
  return defaults[formState.type] || 'Integration Name'
})

const urlPlaceholder = computed(() => {
  const defaults: Record<string, string> = {
    sonarr: 'http://localhost:8989',
    radarr: 'http://localhost:7878',
    lidarr: 'http://localhost:8686',
    readarr: 'http://localhost:8787',
    plex: 'http://192.168.1.100:32400',
    jellyfin: 'http://localhost:8096',
    emby: 'http://localhost:8096',
    tautulli: 'http://localhost:8181',
    overseerr: 'http://localhost:5055'
  }
  return defaults[formState.type] || 'http://localhost:8080'
})

const urlHelp = computed(() => {
  const help: Record<string, string> = {
    sonarr: 'Your Sonarr instance URL (IP or hostname + port).',
    radarr: 'Your Radarr instance URL (IP or hostname + port).',
    lidarr: 'Your Lidarr instance URL (IP or hostname + port).',
    readarr: 'Your Readarr instance URL (IP or hostname + port).',
    plex: 'Your Plex Media Server URL. Use the direct server address, not app.plex.tv.',
    jellyfin: 'Your Jellyfin server URL (IP or hostname + port).',
    emby: 'Your Emby server URL (IP or hostname + port).',
    tautulli: 'Your Tautulli instance URL (IP or hostname + port).',
    overseerr: 'Full URL including any subpath (e.g., https://example.com/requests/).'
  }
  return help[formState.type] || 'The base URL of your integration.'
})

function typeIcon(type: string) {
  switch (type) {
    case 'sonarr': return TvIcon
    case 'radarr': return FilmIcon
    case 'lidarr': return MusicIcon
    case 'readarr': return BookOpenIcon
    case 'plex': return PlayCircleIcon
    case 'jellyfin': return MonitorPlayIcon
    case 'emby': return MonitorPlayIcon
    case 'tautulli': return ActivityIcon
    case 'overseerr': return InboxIcon
    default: return ServerIcon
  }
}

function typeColor(type: string) {
  switch (type) {
    case 'sonarr': return 'bg-sky-500'
    case 'radarr': return 'bg-amber-500'
    case 'lidarr': return 'bg-green-500'
    case 'readarr': return 'bg-emerald-600'
    case 'plex': return 'bg-orange-500'
    case 'jellyfin': return 'bg-purple-500'
    case 'emby': return 'bg-emerald-500'
    case 'tautulli': return 'bg-teal-500'
    case 'overseerr': return 'bg-indigo-500'
    default: return 'bg-muted-foreground'
  }
}

function typeTextColor(type: string) {
  switch (type) {
    case 'sonarr': return 'text-sky-500'
    case 'radarr': return 'text-amber-500'
    case 'lidarr': return 'text-green-500'
    case 'readarr': return 'text-emerald-600'
    case 'plex': return 'text-orange-500'
    case 'jellyfin': return 'text-purple-500'
    case 'emby': return 'text-emerald-500'
    case 'tautulli': return 'text-teal-500'
    case 'overseerr': return 'text-indigo-500'
    default: return 'text-muted-foreground'
  }
}

// ─── Auto-save helpers ───────────────────────────────────────────────────────
const saveTimers: Record<string, ReturnType<typeof setTimeout>> = {}

function showSaveStatus(field: string, status: 'saving' | 'saved' | 'error') {
  saveStatus[field] = status
  if (status === 'saved') {
    if (saveTimers[field]) clearTimeout(saveTimers[field])
    saveTimers[field] = setTimeout(() => {
      saveStatus[field] = 'idle'
    }, 2000)
  }
}

async function autoSavePreference(field: string, key: string, value: string | number | boolean) {
  showSaveStatus(field, 'saving')
  try {
    const currentPrefs = await api('/api/v1/preferences') as PreferenceSet
    await api('/api/v1/preferences', {
      method: 'PUT',
      body: { ...currentPrefs, [key]: value }
    })
    showSaveStatus(field, 'saved')
  } catch {
    showSaveStatus(field, 'error')
    addToast(`Failed to save ${field} setting`, 'error')
  }
}

// Watch poll interval — immediate save on select change
watch(pollIntervalSeconds, (newVal, oldVal) => {
  if (oldVal !== undefined && newVal !== oldVal) {
    autoSavePreference('pollInterval', 'pollIntervalSeconds', newVal)
  }
})

// Watch retention days — immediate save on select change
watch(retentionDays, (newVal, oldVal) => {
  if (oldVal !== undefined && newVal !== oldVal) {
    autoSavePreference('retention', 'auditLogRetentionDays', newVal)
  }
})

// Watch log level — immediate save on select change
watch(logLevel, (newVal, oldVal) => {
  if (oldVal !== undefined && newVal !== oldVal) {
    autoSavePreference('logLevel', 'logLevel', newVal)
  }
})

// ─── Integrations ────────────────────────────────────────────────────────────
async function fetchIntegrations() {
  loading.value = true
  try {
    integrations.value = await api('/api/v1/integrations') as IntegrationConfig[]
  } catch {
    addToast('Failed to load integrations', 'error')
  } finally {
    loading.value = false
  }
}

function openAddModal() {
  editingIntegration.value = null
  formState.type = 'sonarr'
  formState.name = ''
  formState.url = ''
  formState.apiKey = ''
  formError.value = ''
  showModal.value = true
}

function onApiKeyFocus() {
  // When user clicks into the API key field while it shows a masked value,
  // clear it so they can type a new key. If they leave it empty, the backend
  // will preserve the existing key.
  if (formState.apiKey.includes('•')) {
    formState.apiKey = ''
  }
}

function openEditModal(integration: IntegrationConfig) {
  editingIntegration.value = integration
  formState.type = integration.type
  formState.name = integration.name
  formState.url = integration.url
  formState.apiKey = integration.apiKey
  formError.value = ''
  showModal.value = true
}

async function onSubmit() {
  saving.value = true
  formError.value = ''
  try {
    if (editingIntegration.value) {
      await api(`/api/v1/integrations/${editingIntegration.value.id}`, {
        method: 'PUT',
        body: { ...formState, enabled: editingIntegration.value.enabled }
      })
    } else {
      await api('/api/v1/integrations', {
        method: 'POST',
        body: formState
      })
    }
    showModal.value = false
    addToast('Integration saved', 'success')
    await fetchIntegrations()
  } catch (e: unknown) {
    formError.value = (e as ApiError)?.data?.error || 'Failed to save integration'
    addToast(formError.value, 'error')
  } finally {
    saving.value = false
  }
}

async function deleteIntegration(integration: IntegrationConfig) {
  if (!confirm(`Delete ${integration.name}? This cannot be undone.`)) return
  try {
    await api(`/api/v1/integrations/${integration.id}`, { method: 'DELETE' })
    addToast('Integration deleted', 'success')
    await fetchIntegrations()
  } catch {
    addToast('Failed to delete integration', 'error')
  }
}

async function testConnection(integration: IntegrationConfig) {
  try {
    const result = await api('/api/v1/integrations/test', {
      method: 'POST',
      body: { type: integration.type, url: integration.url, apiKey: integration.apiKey, integrationId: integration.id }
    }) as ConnectionTestResult
    addToast(result.success ? 'Connection successful!' : `Connection failed: ${result.error}`, result.success ? 'success' : 'error')
  } catch {
    addToast('Connection test failed', 'error')
  }
}

async function testFormConnection() {
  try {
    const body: Record<string, unknown> = { type: formState.type, url: formState.url, apiKey: formState.apiKey }
    if (editingIntegration.value) {
      body.integrationId = editingIntegration.value.id
    }
    const result = await api('/api/v1/integrations/test', {
      method: 'POST',
      body
    }) as ConnectionTestResult
    if (result.success) {
      formError.value = ''
      addToast('Connection successful!', 'success')
    } else {
      formError.value = result.error || 'Connection failed'
      addToast(formError.value, 'error')
    }
  } catch {
    formError.value = 'Connection test failed'
    addToast('Connection test failed', 'error')
  }
}

// ─── General Settings ────────────────────────────────────────────────────────
async function fetchPreferences() {
  try {
    const prefs = await api('/api/v1/preferences') as PreferenceSet
    if (prefs?.auditLogRetentionDays !== undefined) {
      retentionDays.value = prefs.auditLogRetentionDays
    }
    if (prefs?.pollIntervalSeconds !== undefined && prefs.pollIntervalSeconds >= 30) {
      pollIntervalSeconds.value = prefs.pollIntervalSeconds
    }
    if (prefs?.executionMode) {
      engineExecutionMode.value = prefs.executionMode
    }
    if (prefs?.tiebreakerMethod) {
      engineTiebreakerMethod.value = prefs.tiebreakerMethod
    }
    if (prefs?.deletionsEnabled !== undefined) {
      deletionsEnabled.value = prefs.deletionsEnabled
    }
    if (prefs?.logLevel) {
      logLevel.value = prefs.logLevel
    }
  } catch {
    // Silently ignored — UI has no further handling
  }
}

function setExecutionMode(mode: string) {
  engineExecutionMode.value = mode
  autoSavePreference('executionMode', 'executionMode', mode)
}

// ─── Deletion Safety Toggle ──────────────────────────────────────────────────
function onDeletionToggle(checked: boolean) {
  if (checked) {
    // Show confirmation dialog when enabling
    showDeletionConfirmDialog.value = true
  } else {
    // Disable immediately without confirmation
    deletionsEnabled.value = false
    autoSavePreference('deletionsEnabled', 'deletionsEnabled', false)
    addToast('File deletions disabled — all actions are now simulated', 'success')
  }
}

function confirmEnableDeletions() {
  deletionsEnabled.value = true
  showDeletionConfirmDialog.value = false
  autoSavePreference('deletionsEnabled', 'deletionsEnabled', true)
  addToast('File deletions enabled — flagged items will be permanently removed', 'warning')
}

function cancelEnableDeletions() {
  showDeletionConfirmDialog.value = false
}

// Watch tiebreaker — immediate save on select change
watch(engineTiebreakerMethod, (newVal, oldVal) => {
  if (oldVal !== undefined && newVal !== oldVal) {
    autoSavePreference('tiebreaker', 'tiebreakerMethod', newVal)
  }
})

// ─── Password Change ─────────────────────────────────────────────────────────
async function changePassword() {
  passwordError.value = ''

  if (!passwordForm.currentPassword || !passwordForm.newPassword) {
    passwordError.value = 'All fields are required'
    return
  }
  if (passwordForm.newPassword !== passwordForm.confirmPassword) {
    passwordError.value = 'New passwords do not match'
    return
  }
  if (passwordForm.newPassword.length < 8) {
    passwordError.value = 'New password must be at least 8 characters'
    return
  }

  savingPassword.value = true
  try {
    await api('/api/v1/auth/password', {
      method: 'PUT',
      body: {
        currentPassword: passwordForm.currentPassword,
        newPassword: passwordForm.newPassword
      }
    })
    addToast('Password changed — please log in again', 'success')
    passwordForm.currentPassword = ''
    passwordForm.newPassword = ''
    passwordForm.confirmPassword = ''
    // Redirect to login after short delay
    setTimeout(() => {
      navigateTo('/login')
    }, 1500)
  } catch (e: unknown) {
    passwordError.value = (e as ApiError)?.data?.error || 'Failed to change password'
    addToast(passwordError.value, 'error')
  } finally {
    savingPassword.value = false
  }
}

// ─── Username Change ─────────────────────────────────────────────────────────
async function changeUsername() {
  usernameError.value = ''

  if (!usernameForm.newUsername || !usernameForm.password) {
    usernameError.value = 'All fields are required'
    return
  }
  if (usernameForm.newUsername.length < 3) {
    usernameError.value = 'Username must be at least 3 characters'
    return
  }
  if (/\s/.test(usernameForm.newUsername)) {
    usernameError.value = 'Username cannot contain spaces'
    return
  }

  savingUsername.value = true
  try {
    await api('/api/v1/auth/username', {
      method: 'PUT',
      body: {
        newUsername: usernameForm.newUsername,
        currentPassword: usernameForm.password
      }
    })
    addToast('Username changed — please log in again', 'success')
    usernameForm.newUsername = ''
    usernameForm.password = ''
    setTimeout(() => {
      navigateTo('/login')
    }, 1500)
  } catch (e: unknown) {
    usernameError.value = (e as ApiError)?.data?.error || 'Failed to change username'
    addToast(usernameError.value, 'error')
  } finally {
    savingUsername.value = false
  }
}

// ─── API Key ─────────────────────────────────────────────────────────────────
async function generateApiKey() {
  generatingApiKey.value = true
  try {
    const result = await api('/api/v1/auth/apikey', { method: 'POST' }) as ApiKeyResponse
    apiKey.value = result.api_key
    addToast('API key generated', 'success')
  } catch {
    addToast('Failed to generate API key', 'error')
  } finally {
    generatingApiKey.value = false
  }
}

async function fetchApiKey() {
  try {
    const result = await api('/api/v1/auth/apikey') as ApiKeyResponse
    if (result?.api_key) {
      apiKey.value = result.api_key
    }
  } catch {
    // Silently fail — no API key yet
  }
}

function copyApiKey() {
  navigator.clipboard.writeText(apiKey.value)
  addToast('API key copied to clipboard', 'success')
}

// ─── Data Reset ──────────────────────────────────────────────────────────────
async function confirmResetData() {
  resettingData.value = true
  try {
    await api('/api/v1/data/reset', { method: 'DELETE' })
    showResetDialog.value = false
    addToast('All scraped data has been cleared', 'success')
    // Refresh page data so the UI reflects the cleared state
    await fetchIntegrations()
  } catch (e: unknown) {
    addToast((e as ApiError)?.data?.error || 'Failed to clear data', 'error')
  } finally {
    resettingData.value = false
  }
}

// ─── Notification Channels ───────────────────────────────────────────────────
const channels = ref<NotificationChannel[]>([])
const channelsLoading = ref(false)
const showChannelModal = ref(false)
const editingChannel = ref<NotificationChannel | null>(null)
const savingChannel = ref(false)
const channelFormError = ref('')
const testingChannelId = ref<number | null>(null)

const channelForm = reactive({
  type: 'discord' as 'discord' | 'slack' | 'inapp',
  name: '',
  webhookUrl: '',
  onThresholdBreach: true,
  onDeletionExecuted: true,
  onEngineError: true,
  onEngineComplete: false
})

function channelTypeIcon(type: string) {
  switch (type) {
    case 'discord': return MessageSquareIcon
    case 'slack': return HashIcon
    case 'inapp': return BellIcon
    default: return BellIcon
  }
}

function channelTypeColor(type: string) {
  switch (type) {
    case 'discord': return 'bg-indigo-500'
    case 'slack': return 'bg-green-500'
    case 'inapp': return 'bg-blue-500'
    default: return 'bg-muted-foreground'
  }
}

function channelTypeLabel(type: string) {
  switch (type) {
    case 'discord': return 'Discord'
    case 'slack': return 'Slack'
    case 'inapp': return 'In-App'
    default: return type
  }
}

async function fetchChannels() {
  channelsLoading.value = true
  try {
    channels.value = await api('/api/v1/notifications/channels') as NotificationChannel[]
  } catch {
    addToast('Failed to load notification channels', 'error')
  } finally {
    channelsLoading.value = false
  }
}

function openAddChannelModal() {
  editingChannel.value = null
  channelForm.type = 'discord'
  channelForm.name = ''
  channelForm.webhookUrl = ''
  channelForm.onThresholdBreach = true
  channelForm.onDeletionExecuted = true
  channelForm.onEngineError = true
  channelForm.onEngineComplete = false
  channelFormError.value = ''
  showChannelModal.value = true
}

function openEditChannelModal(channel: NotificationChannel) {
  editingChannel.value = channel
  channelForm.type = channel.type
  channelForm.name = channel.name
  channelForm.webhookUrl = channel.webhookUrl || ''
  channelForm.onThresholdBreach = channel.onThresholdBreach
  channelForm.onDeletionExecuted = channel.onDeletionExecuted
  channelForm.onEngineError = channel.onEngineError
  channelForm.onEngineComplete = channel.onEngineComplete
  channelFormError.value = ''
  showChannelModal.value = true
}

async function onChannelSubmit() {
  savingChannel.value = true
  channelFormError.value = ''
  try {
    const body = {
      type: channelForm.type,
      name: channelForm.name,
      webhookUrl: channelForm.type !== 'inapp' ? channelForm.webhookUrl : undefined,
      enabled: editingChannel.value ? editingChannel.value.enabled : true,
      onThresholdBreach: channelForm.onThresholdBreach,
      onDeletionExecuted: channelForm.onDeletionExecuted,
      onEngineError: channelForm.onEngineError,
      onEngineComplete: channelForm.onEngineComplete
    }
    if (editingChannel.value) {
      await api(`/api/v1/notifications/channels/${editingChannel.value.id}`, {
        method: 'PUT',
        body
      })
    } else {
      await api('/api/v1/notifications/channels', {
        method: 'POST',
        body
      })
    }
    showChannelModal.value = false
    addToast('Notification channel saved', 'success')
    await fetchChannels()
  } catch (e: unknown) {
    channelFormError.value = (e as ApiError)?.data?.error || 'Failed to save channel'
    addToast(channelFormError.value, 'error')
  } finally {
    savingChannel.value = false
  }
}

async function deleteChannel(channel: NotificationChannel) {
  if (!confirm(`Delete "${channel.name}"? This cannot be undone.`)) return
  try {
    await api(`/api/v1/notifications/channels/${channel.id}`, { method: 'DELETE' })
    addToast('Channel deleted', 'success')
    await fetchChannels()
  } catch {
    addToast('Failed to delete channel', 'error')
  }
}

async function toggleChannelEnabled(channel: NotificationChannel, enabled: boolean) {
  try {
    await api(`/api/v1/notifications/channels/${channel.id}`, {
      method: 'PUT',
      body: { ...channel, enabled }
    })
    channel.enabled = enabled
    addToast(`Channel ${enabled ? 'enabled' : 'disabled'}`, 'success')
  } catch {
    addToast('Failed to update channel', 'error')
  }
}

async function updateChannelEvent(channel: NotificationChannel, field: string, value: boolean) {
  try {
    const updated = { ...channel, [field]: value }
    await api(`/api/v1/notifications/channels/${channel.id}`, {
      method: 'PUT',
      body: updated
    })
    // Update local state
    ;(channel as Record<string, unknown>)[field] = value
  } catch {
    addToast('Failed to update channel', 'error')
  }
}

async function testChannel(channel: NotificationChannel) {
  testingChannelId.value = channel.id
  try {
    await api(`/api/v1/notifications/channels/${channel.id}/test`, { method: 'POST' })
    addToast('Test notification sent!', 'success')
  } catch {
    addToast('Failed to send test notification', 'error')
  } finally {
    testingChannelId.value = null
  }
}

onMounted(() => {
  fetchIntegrations()
  fetchPreferences()
  fetchApiKey()
  fetchChannels()
})
</script>
