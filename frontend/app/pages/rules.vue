<template>
  <div>
    <!-- Header -->
    <div
      data-slot="page-header"
      class="mb-8"
    >
      <h1 class="text-3xl font-bold tracking-tight">
        {{ $t('rules.title') }}
      </h1>
      <p class="text-muted-foreground mt-1.5">
        {{ $t('rules.subtitle') }}
      </p>
    </div>

    <!-- Disk Thresholds — Editable -->
    <UiCard
      v-if="diskGroups.length > 0"
      v-motion
      :initial="{ opacity: 0, y: 12 }"
      :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24 } }"
      class="mb-6"
    >
      <UiCardHeader>
        <UiCardTitle>{{ $t('rules.diskThresholds') }}</UiCardTitle>
        <UiCardDescription>
          {{ $t('rules.diskThresholdsDesc') }}
        </UiCardDescription>
      </UiCardHeader>
      <UiCardContent class="space-y-5">
        <div
          v-for="dg in diskGroups"
          :key="dg.id"
          class="rounded-lg border border-border bg-muted/50 p-5 space-y-4"
        >
          <!-- Mount path & current usage -->
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-3">
              <div
                class="w-9 h-9 rounded-lg flex items-center justify-center shrink-0"
                :class="diskStatusBgClass(diskUsagePct(dg), thresholdEdits[dg.id]?.target ?? dg.targetPct, thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct)"
              >
                <component
                  :is="HardDriveIcon"
                  class="w-4.5 h-4.5 text-white"
                />
              </div>
              <div>
                <div
                  class="text-sm font-medium text-foreground truncate"
                  :title="dg.mountPath"
                >
                  {{ dg.mountPath }}
                </div>
                <span class="text-xs text-muted-foreground">
                  {{ formatBytes(dg.usedBytes) }} / {{ formatBytes(dg.totalBytes) }}
                </span>
              </div>
            </div>
            <span
              class="text-2xl font-bold tabular-nums"
              :class="diskStatusTextClass(diskUsagePct(dg), thresholdEdits[dg.id]?.target ?? dg.targetPct, thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct)"
            >
              {{ Math.round(diskUsagePct(dg)) }}%
            </span>
          </div>

          <!-- Progress bar with segmented zone background + triangle markers -->
          <div class="relative w-full mt-8 mb-6">
            <!-- Bar container -->
            <div class="relative w-full h-3 rounded-full overflow-hidden">
              <!-- Segmented background zones -->
              <div class="absolute inset-0 flex">
                <div
                  class="h-full"
                  :style="{ width: (thresholdEdits[dg.id]?.target ?? dg.targetPct) + '%', backgroundColor: 'oklch(0.648 0.2 160 / 0.2)' }"
                />
                <div
                  class="h-full"
                  :style="{ width: ((thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct) - (thresholdEdits[dg.id]?.target ?? dg.targetPct)) + '%', backgroundColor: 'oklch(0.75 0.183 55.934 / 0.2)' }"
                />
                <div
                  class="h-full"
                  :style="{ width: (100 - (thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct)) + '%', backgroundColor: 'oklch(0.577 0.245 27.325 / 0.2)' }"
                />
              </div>
              <!-- Usage fill bar -->
              <div
                data-slot="progress-bar-fill"
                role="progressbar"
                :aria-valuenow="Math.round(diskUsagePct(dg))"
                aria-valuemin="0"
                aria-valuemax="100"
                :aria-label="`Disk usage: ${Math.round(diskUsagePct(dg))}%`"
                :data-status="diskUsageStatus(diskUsagePct(dg), thresholdEdits[dg.id]?.target ?? dg.targetPct, thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct)"
                class="relative h-full rounded-full transition-all duration-700 ease-out z-10"
                :style="{ width: Math.min(diskUsagePct(dg), 100) + '%', backgroundColor: diskStatusFillColor(diskUsagePct(dg), thresholdEdits[dg.id]?.target ?? dg.targetPct, thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct) }"
              />
            </div>

            <!-- Target marker ABOVE the bar -->
            <div
              class="absolute bottom-3 flex flex-col items-center z-20"
              :style="{ left: (thresholdEdits[dg.id]?.target ?? dg.targetPct) + '%', transform: 'translateX(-50%)' }"
            >
              <span class="text-[10px] font-medium text-emerald-600 dark:text-emerald-400 whitespace-nowrap mb-0.5">
                Target {{ thresholdEdits[dg.id]?.target ?? dg.targetPct }}%
              </span>
              <span class="text-emerald-500 text-[10px] leading-none mb-0.5">▼</span>
            </div>
            <!-- Threshold marker BELOW the bar -->
            <div
              class="absolute top-3 flex flex-col items-center z-20"
              :style="{ left: (thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct) + '%', transform: 'translateX(-50%)' }"
            >
              <span class="text-red-500 text-[10px] leading-none mt-0.5">▲</span>
              <span class="text-[10px] font-medium text-red-500 dark:text-red-400 whitespace-nowrap mt-0.5">
                Threshold {{ thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct }}%
              </span>
            </div>
          </div>

          <!-- Free space info -->
          <div class="text-xs text-muted-foreground/70">
            <span>{{ formatBytes(dg.totalBytes - dg.usedBytes) }} free</span>
          </div>

          <!-- Editable inputs -->
          <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div class="space-y-1.5">
              <UiLabel>{{ $t('rules.cleanupThreshold') }}</UiLabel>
              <div class="flex items-center gap-2">
                <UiInput
                  :model-value="String(thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct)"
                  type="number"
                  min="1"
                  max="99"
                  @update:model-value="(v: string | number) => updateThresholdEdit(dg.id, 'threshold', Number(v), dg)"
                />
                <span class="w-2 h-2 rounded-full bg-red-400 shrink-0" />
              </div>
              <p class="text-[11px] text-muted-foreground">
                {{ $t('rules.cleanupThresholdDesc') }}
              </p>
            </div>
            <div class="space-y-1.5">
              <UiLabel>{{ $t('rules.cleanupTarget') }}</UiLabel>
              <div class="flex items-center gap-2">
                <UiInput
                  :model-value="String(thresholdEdits[dg.id]?.target ?? dg.targetPct)"
                  type="number"
                  min="1"
                  max="99"
                  @update:model-value="(v: string | number) => updateThresholdEdit(dg.id, 'target', Number(v), dg)"
                />
                <span class="w-2 h-2 rounded-full bg-emerald-500 shrink-0" />
              </div>
              <p class="text-[11px] text-muted-foreground">
                {{ $t('rules.cleanupTargetDesc') }}
              </p>
            </div>
          </div>

          <!-- Validation error -->
          <p
            v-if="thresholdValidation(dg.id, dg)"
            class="text-xs text-red-500"
          >
            {{ thresholdValidation(dg.id, dg) }}
          </p>

          <!-- Auto-save status indicator -->
          <div class="flex items-center gap-2 h-5">
            <Transition
              enter-active-class="transition-all duration-300 ease-out"
              leave-active-class="transition-all duration-300 ease-in"
              enter-from-class="opacity-0 translate-y-1"
              enter-to-class="opacity-100 translate-y-0"
              leave-from-class="opacity-100 translate-y-0"
              leave-to-class="opacity-0 translate-y-1"
            >
              <span
                v-if="thresholdEdits[dg.id]?.saving"
                class="inline-flex items-center gap-1.5 text-xs text-muted-foreground"
              >
                <component
                  :is="LoaderCircleIcon"
                  class="w-3.5 h-3.5 animate-spin"
                />
                {{ $t('common.saving') }}
              </span>
              <span
                v-else-if="thresholdEdits[dg.id]?.success && thresholdEdits[dg.id]?.message"
                class="inline-flex items-center gap-1.5 text-xs text-emerald-500"
              >
                <component
                  :is="CheckIcon"
                  class="w-3.5 h-3.5"
                />
                {{ $t('common.saved') }}
              </span>
              <span
                v-else-if="thresholdEdits[dg.id]?.message && !thresholdEdits[dg.id]?.success"
                class="inline-flex items-center gap-1.5 text-xs text-red-500"
              >
                {{ thresholdEdits[dg.id]?.message }}
              </span>
            </Transition>
          </div>
        </div>
      </UiCardContent>
    </UiCard>

    <!-- Preference Weights -->
    <UiCard
      v-motion
      :initial="{ opacity: 0, y: 12 }"
      :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24 } }"
      class="mb-6"
    >
      <UiCardHeader>
        <div class="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div>
            <UiCardTitle>{{ $t('rules.preferenceWeights') }}</UiCardTitle>
            <UiCardDescription>
              {{ $t('rules.preferenceWeightsDesc') }}
            </UiCardDescription>
          </div>
          <UiButton
            size="sm"
            @click="savePreferences"
          >
            {{ $t('rules.saveWeights') }}
          </UiButton>
        </div>
      </UiCardHeader>
      <UiCardContent>
        <!-- Preset Chips -->
        <div class="flex flex-wrap gap-2 mb-2">
          <UiButton
            v-for="preset in presets"
            :key="preset.name"
            :variant="isActivePreset(preset.values) ? 'default' : 'outline'"
            size="sm"
            class="rounded-full h-7 px-3 text-xs"
            @click="applyPreset(preset.values)"
          >
            {{ preset.name }}
          </UiButton>
        </div>

        <!-- Preset Description -->
        <Transition
          enter-active-class="transition-all duration-300 ease-out"
          leave-active-class="transition-all duration-200 ease-in"
          enter-from-class="opacity-0 -translate-y-1"
          enter-to-class="opacity-100 translate-y-0"
          leave-from-class="opacity-100 translate-y-0"
          leave-to-class="opacity-0 -translate-y-1"
          mode="out-in"
        >
          <p
            :key="activePresetDescription"
            class="text-xs text-muted-foreground/70 mb-6 leading-relaxed"
          >
            {{ activePresetDescription }}
          </p>
        </Transition>

        <!-- Two-Column Slider Grid -->
        <div class="grid grid-cols-1 md:grid-cols-2 gap-x-8 gap-y-5">
          <div
            v-for="slider in sliders"
            :key="slider.key"
            class="space-y-1.5"
          >
            <div class="flex justify-between text-sm">
              <span class="font-medium text-foreground">{{ slider.label }}</span>
              <span class="text-muted-foreground font-mono tabular-nums">{{ prefs[slider.key as keyof typeof prefs] }} / 10</span>
            </div>
            <UiSlider
              :model-value="[Number(prefs[slider.key as keyof typeof prefs])]"
              :min="0"
              :max="10"
              :step="1"
              class="w-full"
              @update:model-value="(v: number[] | undefined) => { if (v) Object.assign(prefs, { [slider.key]: v[0] }) }"
            />
            <p class="text-xs text-muted-foreground">
              {{ slider.description }}
            </p>
          </div>
        </div>
      </UiCardContent>
    </UiCard>

    <!-- Custom Rules -->
    <UiCard
      v-motion
      :initial="{ opacity: 0, y: 12 }"
      :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 100 } }"
      class="mb-6"
    >
      <UiCardHeader>
        <div class="flex items-center justify-between">
          <div>
            <UiCardTitle>{{ $t('rules.customRules') }}</UiCardTitle>
            <UiCardDescription class="mt-1">
              {{ $t('rules.customRulesDesc') }}
            </UiCardDescription>
            <p class="text-xs text-muted-foreground mt-1">
              {{ $t('rules.orderDisclaimer') }}
            </p>
          </div>
          <UiButton
            size="sm"
            @click="showAddRule = !showAddRule"
          >
            <component
              :is="PlusIcon"
              class="w-3.5 h-3.5"
            />
            {{ $t('rules.addRule') }}
          </UiButton>
        </div>
      </UiCardHeader>
      <UiCardContent>
        <!-- Add Rule Form — Cascading Rule Builder -->
        <RuleBuilder
          v-if="showAddRule"
          :integrations="allIntegrations"
          class="mb-4"
          @save="addRule"
          @cancel="showAddRule = false"
        />

        <!-- Rules List — Natural Language Display with Conflict Indicators -->
        <div
          v-if="rules.length === 0 && !showAddRule"
          class="text-center py-6 text-muted-foreground text-sm"
        >
          {{ $t('rules.noRules') }}
        </div>
        <div
          v-else
          class="space-y-2"
        >
          <div
            v-for="(rule, ruleIdx) in rules"
            :key="rule.id"
            draggable="true"
            class="flex items-center justify-between px-4 py-2.5 rounded-lg border bg-muted/50 transition-opacity duration-200"
            :class="[
              ruleConflicts(rule).length > 0 ? 'border-amber-400/50' : 'border-border',
              rule.enabled === false ? 'opacity-50' : '',
              dragOverIdx === ruleIdx ? 'border-primary border-dashed' : '',
              dragSourceIdx === ruleIdx ? 'opacity-30' : ''
            ]"
            @dragstart="onDragStart($event, ruleIdx)"
            @dragover.prevent="onDragOver($event, ruleIdx)"
            @dragleave="onDragLeave"
            @drop.prevent="onDrop($event, ruleIdx)"
            @dragend="onDragEnd"
          >
            <div class="flex items-center gap-2 text-sm flex-wrap">
              <!-- Drag handle -->
              <span
                role="button"
                aria-label="Drag to reorder"
                class="inline-flex items-center shrink-0 cursor-grab active:cursor-grabbing text-muted-foreground/50 hover:text-muted-foreground transition-colors"
              >
                <GripVerticalIcon class="w-4 h-4" />
              </span>
              <!-- Rule number -->
              <span class="text-xs font-mono tabular-nums text-muted-foreground w-5 shrink-0">{{ ruleIdx + 1 }}.</span>
              <!-- Enable/Disable toggle -->
              <UiSwitch
                :model-value="rule.enabled !== false"
                :aria-label="rule.enabled !== false ? 'Disable rule' : 'Enable rule'"
                class="shrink-0"
                @update:model-value="(v: boolean) => toggleRuleEnabled(rule, v)"
              />
              <!-- Conflict indicator -->
              <UiTooltipProvider v-if="ruleConflicts(rule).length > 0">
                <UiTooltip>
                  <UiTooltipTrigger as-child>
                    <span class="inline-flex items-center shrink-0 cursor-help">
                      <component
                        :is="AlertTriangleIcon"
                        class="w-4 h-4 text-amber-500"
                      />
                    </span>
                  </UiTooltipTrigger>
                  <UiTooltipContent
                    side="top"
                    class="max-w-xs text-xs"
                  >
                    <p
                      v-for="(conflict, idx) in ruleConflicts(rule)"
                      :key="idx"
                      class="mb-1 last:mb-0"
                    >
                      {{ conflict }}
                    </p>
                  </UiTooltipContent>
                </UiTooltip>
              </UiTooltipProvider>
              <!-- Service name -->
              <span
                v-if="rule.integrationId"
                class="text-muted-foreground"
              >
                {{ integrationName(rule.integrationId) }} ·
              </span>
              <!-- Human-readable condition -->
              <span :class="rule.enabled === false ? 'text-muted-foreground' : 'text-foreground'">{{ fieldLabel(rule.field) }}</span>
              <span class="text-muted-foreground">{{ operatorLabel(rule.operator) }}</span>
              <span :class="rule.enabled === false ? 'text-muted-foreground' : 'font-medium'">"{{ rule.value }}"</span>
            </div>
            <div class="flex items-center gap-2 shrink-0">
              <!-- Effect badge -->
              <UiBadge
                variant="outline"
                :class="effectBadgeClass(rule.effect || legacyEffect(rule.type, rule.intensity))"
                class="shrink-0"
              >
                <span class="inline-flex items-center gap-1">
                  <span class="text-xs">{{ effectIconMap[rule.effect || legacyEffect(rule.type, rule.intensity)] || '' }}</span>
                  {{ effectLabel(rule.effect || legacyEffect(rule.type, rule.intensity)) }}
                </span>
              </UiBadge>
              <UiButton
                variant="ghost"
                size="icon-sm"
                aria-label="Delete rule"
                class="text-muted-foreground hover:text-red-500 shrink-0"
                @click="deleteRule(rule.id)"
              >
                <component
                  :is="XIcon"
                  class="w-4 h-4"
                />
              </UiButton>
            </div>
          </div>
        </div>
      </UiCardContent>
    </UiCard>

    <!-- Live Preview -->
    <UiCard
      v-motion
      :initial="{ opacity: 0, y: 12 }"
      :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 200 } }"
    >
      <UiCardHeader>
        <div class="flex items-center justify-between">
          <div>
            <UiCardTitle>{{ $t('rules.deletionPriority') }}</UiCardTitle>
            <UiCardDescription class="mt-1">
              {{ $t('rules.deletionPriorityDesc') }}
            </UiCardDescription>
          </div>
          <UiButton
            variant="outline"
            size="sm"
            @click="fetchPreview"
          >
            <component
              :is="previewLoading ? LoaderCircleIcon : RefreshCwIcon"
              :class="{ 'animate-spin': previewLoading }"
              class="w-3.5 h-3.5"
            />
            {{ $t('common.refresh') }}
          </UiButton>
        </div>
      </UiCardHeader>
      <UiCardContent>
        <!-- Disk below threshold banner -->
        <div
          v-if="!previewLoading && preview.length > 0 && diskContext && diskContext.bytesToFree === 0"
          class="mb-4 rounded-md border border-emerald-500/30 bg-emerald-500/5 px-4 py-3 text-sm text-emerald-600 dark:text-emerald-400 flex items-center gap-2"
        >
          <CheckIcon class="w-4 h-4 shrink-0" />
          {{ $t('rules.diskBelowThreshold') }}
        </div>

        <div
          v-if="previewLoading"
          class="flex items-center justify-center py-12"
        >
          <component
            :is="LoaderCircleIcon"
            class="w-6 h-6 text-primary animate-spin"
          />
        </div>

        <div
          v-else-if="preview.length === 0"
          class="text-center py-8 text-muted-foreground text-sm"
        >
          {{ $t('rules.noItemsToEvaluate') }}
        </div>

        <div v-else>
          <!-- Search & Filters -->
          <div class="flex flex-col sm:flex-row gap-3 mb-4">
            <div class="relative flex-1">
              <SearchIcon class="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground pointer-events-none" />
              <UiInput
                v-model="previewSearch"
                aria-label="Search deletion priority by title"
                placeholder="Search by title…"
                class="pl-8"
              />
            </div>
            <div class="flex items-center gap-1.5 flex-wrap">
              <UiButton
                v-for="mt in previewMediaTypes"
                :key="mt"
                :variant="previewTypeFilter === mt ? 'default' : 'outline'"
                size="sm"
                class="rounded-full h-7 px-3 text-xs capitalize"
                @click="previewTypeFilter = previewTypeFilter === mt ? null : mt"
              >
                {{ mt }}
              </UiButton>
              <UiSeparator
                orientation="vertical"
                class="h-5 mx-1"
              />
              <UiButton
                :variant="previewStatusFilter === 'protected' ? 'default' : 'outline'"
                size="sm"
                class="rounded-full h-7 px-3 text-xs"
                @click="previewStatusFilter = previewStatusFilter === 'protected' ? 'all' : 'protected'"
              >
                <ShieldCheckIcon class="w-3 h-3 mr-1" />
                Protected
              </UiButton>
              <UiButton
                :variant="previewStatusFilter === 'unprotected' ? 'default' : 'outline'"
                size="sm"
                class="rounded-full h-7 px-3 text-xs"
                @click="previewStatusFilter = previewStatusFilter === 'unprotected' ? 'all' : 'unprotected'"
              >
                Unprotected
              </UiButton>
            </div>
          </div>

          <!-- Results count -->
          <div class="text-xs text-muted-foreground mb-2">
            <template v-if="previewSearch || previewTypeFilter || previewStatusFilter !== 'all'">
              {{ filteredGroupedPreview.length }} of {{ groupedPreview.length }} items
            </template>
            <template v-else>
              {{ groupedPreview.length }} items
            </template>
          </div>

          <div
            v-if="filteredGroupedPreview.length === 0"
            class="text-center py-8 text-muted-foreground text-sm"
          >
            No items match filters.
          </div>

          <div
            v-else
            ref="tableScrollRef"
            class="overflow-x-auto max-h-[600px] overflow-y-auto relative"
          >
            <UiTable>
              <UiTableHeader class="sticky top-0 z-10 bg-background">
                <UiTableRow>
                  <UiTableHead
                    class="w-12 cursor-pointer select-none group"
                    @click="togglePreviewSort('rank')"
                  >
                    <span class="inline-flex items-center gap-1">
                      #
                      <ArrowUpIcon
                        v-if="previewSortBy === 'rank' && previewSortDir === 'asc'"
                        class="w-3 h-3"
                      />
                      <ArrowDownIcon
                        v-else-if="previewSortBy === 'rank' && previewSortDir === 'desc'"
                        class="w-3 h-3"
                      />
                      <ArrowUpDownIcon
                        v-else
                        class="w-3 h-3 opacity-0 group-hover:opacity-50 transition-opacity"
                      />
                    </span>
                  </UiTableHead>
                  <UiTableHead
                    class="cursor-pointer select-none group"
                    @click="togglePreviewSort('score')"
                  >
                    <span class="inline-flex items-center gap-1">
                      Score
                      <ArrowUpIcon
                        v-if="previewSortBy === 'score' && previewSortDir === 'asc'"
                        class="w-3 h-3"
                      />
                      <ArrowDownIcon
                        v-else-if="previewSortBy === 'score' && previewSortDir === 'desc'"
                        class="w-3 h-3"
                      />
                      <ArrowUpDownIcon
                        v-else
                        class="w-3 h-3 opacity-0 group-hover:opacity-50 transition-opacity"
                      />
                    </span>
                  </UiTableHead>
                  <UiTableHead
                    class="cursor-pointer select-none group"
                    @click="togglePreviewSort('title')"
                  >
                    <span class="inline-flex items-center gap-1">
                      Title
                      <ArrowUpIcon
                        v-if="previewSortBy === 'title' && previewSortDir === 'asc'"
                        class="w-3 h-3"
                      />
                      <ArrowDownIcon
                        v-else-if="previewSortBy === 'title' && previewSortDir === 'desc'"
                        class="w-3 h-3"
                      />
                      <ArrowUpDownIcon
                        v-else
                        class="w-3 h-3 opacity-0 group-hover:opacity-50 transition-opacity"
                      />
                    </span>
                  </UiTableHead>
                  <UiTableHead
                    class="cursor-pointer select-none group"
                    @click="togglePreviewSort('type')"
                  >
                    <span class="inline-flex items-center gap-1">
                      Type
                      <ArrowUpIcon
                        v-if="previewSortBy === 'type' && previewSortDir === 'asc'"
                        class="w-3 h-3"
                      />
                      <ArrowDownIcon
                        v-else-if="previewSortBy === 'type' && previewSortDir === 'desc'"
                        class="w-3 h-3"
                      />
                      <ArrowUpDownIcon
                        v-else
                        class="w-3 h-3 opacity-0 group-hover:opacity-50 transition-opacity"
                      />
                    </span>
                  </UiTableHead>
                  <UiTableHead
                    class="text-right cursor-pointer select-none group"
                    @click="togglePreviewSort('size')"
                  >
                    <span class="inline-flex items-center gap-1 justify-end">
                      Size
                      <ArrowUpIcon
                        v-if="previewSortBy === 'size' && previewSortDir === 'asc'"
                        class="w-3 h-3"
                      />
                      <ArrowDownIcon
                        v-else-if="previewSortBy === 'size' && previewSortDir === 'desc'"
                        class="w-3 h-3"
                      />
                      <ArrowUpDownIcon
                        v-else
                        class="w-3 h-3 opacity-0 group-hover:opacity-50 transition-opacity"
                      />
                    </span>
                  </UiTableHead>
                </UiTableRow>
              </UiTableHeader>
              <UiTableBody>
                <template
                  v-for="(group, groupIdx) in renderedGroups"
                  :key="group.key"
                >
                  <!-- Deletion line: inserted before the first item that falls below the cutoff -->
                  <UiTableRow
                    v-if="deletionLineIndex !== null && deletionLineIndex === groupIdx"
                    class="pointer-events-none"
                  >
                    <UiTableCell
                      :colspan="5"
                      class="!p-0"
                    >
                      <div class="flex items-center gap-2 px-4 py-1.5 bg-destructive/10 border-y border-destructive/30">
                        <div class="flex-1 h-px bg-destructive/40" />
                        <span class="text-xs font-medium text-destructive whitespace-nowrap">Engine stops here (target reached)</span>
                        <div class="flex-1 h-px bg-destructive/40" />
                      </div>
                    </UiTableCell>
                  </UiTableRow>
                  <UiTableRow
                    class="cursor-pointer"
                    :class="{ 'opacity-40': deletionLineIndex !== null && groupIdx >= deletionLineIndex }"
                    @click="selectPreviewItem(group.entry); group.seasons.length > 0 && togglePreviewGroup(group.key)"
                  >
                    <UiTableCell class="w-12 text-center">
                      <span class="text-xs font-mono tabular-nums text-muted-foreground">{{ groupIdx + 1 }}</span>
                    </UiTableCell>
                    <UiTableCell>
                      <span
                        class="text-xs font-mono tabular-nums font-semibold"
                        :class="group.entry.isProtected ? 'text-emerald-500' : 'text-primary'"
                      >
                        {{ group.entry.isProtected ? 'Protected' : group.entry.score.toFixed(2) }}
                      </span>
                    </UiTableCell>
                    <UiTableCell class="font-medium">
                      <div class="flex items-center gap-2">
                        <span class="truncate">{{ group.entry.item.title }}</span>
                        <button
                          v-if="group.seasons.length > 0"
                          class="text-muted-foreground hover:text-foreground transition-colors shrink-0 inline-flex items-center gap-0.5"
                          @click.stop="togglePreviewGroup(group.key)"
                        >
                          <ChevronRightIcon
                            class="w-3.5 h-3.5 transition-transform duration-200"
                            :class="{ 'rotate-90': expandedPreviewGroups.has(group.key) }"
                          />
                          <span class="text-xs text-muted-foreground font-normal whitespace-nowrap">({{ group.seasons.length }} season{{ group.seasons.length !== 1 ? 's' : '' }})</span>
                        </button>
                      </div>
                    </UiTableCell>
                    <UiTableCell>
                      <UiBadge
                        variant="secondary"
                        class="capitalize"
                      >
                        {{ group.entry.item.type }}
                      </UiBadge>
                    </UiTableCell>
                    <UiTableCell class="text-right font-mono text-xs tabular-nums">
                      {{ formatBytes(group.entry.item.sizeBytes) }}
                    </UiTableCell>
                  </UiTableRow>
                  <template v-if="expandedPreviewGroups.has(group.key)">
                    <UiTableRow
                      v-for="(season, sIdx) in group.seasons"
                      :key="`${group.key}-s${sIdx}`"
                      class="bg-muted/30 cursor-pointer"
                      :class="{ 'opacity-40': deletionLineIndex !== null && groupIdx >= deletionLineIndex }"
                      @click.stop="selectPreviewItem(season)"
                    >
                      <UiTableCell class="w-12" />
                      <UiTableCell>
                        <span
                          class="text-xs font-mono tabular-nums font-semibold"
                          :class="season.isProtected ? 'text-emerald-500' : 'text-primary'"
                        >
                          {{ season.isProtected ? 'Protected' : season.score.toFixed(2) }}
                        </span>
                      </UiTableCell>
                      <UiTableCell class="text-muted-foreground pl-8">
                        <span class="inline-flex items-center gap-1.5">
                          <UiSeparator
                            orientation="horizontal"
                            class="w-3"
                          />
                          {{ extractPreviewSeasonLabel(season.item.title) }}
                        </span>
                      </UiTableCell>
                      <UiTableCell>
                        <UiBadge
                          variant="secondary"
                          class="capitalize"
                        >
                          {{ season.item.type }}
                        </UiBadge>
                      </UiTableCell>
                      <UiTableCell class="text-right font-mono text-xs tabular-nums text-muted-foreground">
                        {{ formatBytes(season.item.sizeBytes) }}
                      </UiTableCell>
                    </UiTableRow>
                  </template>
                </template>
              </UiTableBody>
            </UiTable>
            <!-- Progressive rendering indicator -->
            <div
              v-if="renderedGroups.length < filteredGroupedPreview.length"
              class="flex items-center justify-center py-3 text-xs text-muted-foreground gap-2"
            >
              <component
                :is="LoaderCircleIcon"
                class="w-3.5 h-3.5 animate-spin"
              />
              Showing {{ renderedGroups.length }} of {{ filteredGroupedPreview.length }} — scroll for more
            </div>
          </div>
        </div>
      </UiCardContent>
    </UiCard>

    <ScoreDetailModal
      v-if="selectedPreviewItem"
      :visible="!!selectedPreviewItem"
      :media-name="selectedPreviewItem.mediaName"
      :media-type="selectedPreviewItem.mediaType"
      :score="selectedPreviewItem._score ?? 0"
      :score-details="selectedPreviewItem.scoreDetails || ''"
      :size-bytes="selectedPreviewItem.sizeBytes"
      :action="selectedPreviewItem.action || 'Preview'"
      :created-at="selectedPreviewItem.createdAt"
      @close="selectedPreviewItem = null"
    />
  </div>
</template>

<script setup lang="ts">
import { useInfiniteScroll } from '@vueuse/core'
import { PlusIcon, XIcon, RefreshCwIcon, LoaderCircleIcon, CheckIcon, ChevronRightIcon, HardDriveIcon, AlertTriangleIcon, SearchIcon, ShieldCheckIcon, ArrowUpIcon, ArrowDownIcon, ArrowUpDownIcon, GripVerticalIcon } from 'lucide-vue-next'
import {
  formatBytes,
  diskUsageStatus,
  diskStatusBgClass,
  diskStatusTextClass,
  diskStatusFillColor
} from '~/utils/format'
import type { DiskGroup, IntegrationConfig, CustomRule, PreferenceSet, EvaluatedItem, PreviewResponse, SelectedDetailItem, ApiError } from '~/types/api'

const api = useApi()
const { addToast } = useToast()

// Disk Groups
const diskGroups = ref<DiskGroup[]>([])

// Per-disk-group threshold editing state
const thresholdEdits = reactive<Record<number, {
  threshold: number
  target: number
  saving: boolean
  message: string
  success: boolean
}>>({})

function diskUsagePct(dg: DiskGroup): number {
  if (!dg.totalBytes || dg.totalBytes === 0) return 0
  return (dg.usedBytes / dg.totalBytes) * 100
}

function ensureThresholdEdit(dgId: number, dg: DiskGroup) {
  if (!thresholdEdits[dgId]) {
    thresholdEdits[dgId] = {
      threshold: dg.thresholdPct,
      target: dg.targetPct,
      saving: false,
      message: '',
      success: false
    }
  }
}

// Debounce timers for auto-save per disk group
const debounceTimers: Record<number, ReturnType<typeof setTimeout>> = {}

function updateThresholdEdit(dgId: number, field: 'threshold' | 'target', value: number, dg: DiskGroup) {
  ensureThresholdEdit(dgId, dg)
  const edit = thresholdEdits[dgId]!
  edit[field] = value
  edit.message = ''
  edit.success = false

  // Cancel any pending debounce for this disk group
  if (debounceTimers[dgId]) {
    clearTimeout(debounceTimers[dgId])
  }

  // Auto-save after 1 second debounce (skip if validation fails)
  debounceTimers[dgId] = setTimeout(() => {
    if (!thresholdValidation(dgId, dg)) {
      saveThresholds(dg)
    }
  }, 1000)
}

function thresholdValidation(dgId: number, dg: DiskGroup): string {
  const edit = thresholdEdits[dgId]
  const t = edit?.threshold ?? dg.thresholdPct
  const g = edit?.target ?? dg.targetPct
  if (t == null || g == null) return 'Both values are required'
  if (t < 1 || t > 99 || g < 1 || g > 99) return 'Values must be between 1 and 99'
  if (t <= g) return 'Threshold must be greater than target'
  return ''
}

async function saveThresholds(dg: DiskGroup) {
  ensureThresholdEdit(dg.id, dg)
  const edit = thresholdEdits[dg.id]!
  if (thresholdValidation(dg.id, dg)) return

  edit.saving = true
  edit.message = ''
  edit.success = false

  try {
    const updated = await api(`/api/v1/disk-groups/${dg.id}`, {
      method: 'PUT',
      body: {
        thresholdPct: edit.threshold,
        targetPct: edit.target
      }
    }) as DiskGroup

    edit.success = true
    edit.message = 'Saved'

    // Update local diskGroups array with canonical values from the API response
    const idx = diskGroups.value.findIndex(g => g.id === dg.id)
    if (idx !== -1 && updated) {
      diskGroups.value[idx] = { ...diskGroups.value[idx], ...updated }
    } else if (idx !== -1) {
      diskGroups.value[idx].thresholdPct = edit.threshold
      diskGroups.value[idx].targetPct = edit.target
    }

    setTimeout(() => {
      edit.message = ''
      edit.success = false
    }, 2500)
  } catch (err: unknown) {
    edit.success = false
    const errMsg = (err as ApiError)?.message || 'Failed to save thresholds'
    edit.message = errMsg
    addToast('Failed to save: ' + errMsg, 'error')
  } finally {
    edit.saving = false
  }
}

// Preferences
const prefs = reactive({
  watchHistoryWeight: 10,
  lastWatchedWeight: 8,
  fileSizeWeight: 6,
  ratingWeight: 5,
  timeInLibraryWeight: 4,
  seriesStatusWeight: 3,
  executionMode: 'dry-run',
  tiebreakerMethod: 'size_desc',
  logLevel: 'info',
  auditLogRetentionDays: 30
})

const sliders = [
  { key: 'watchHistoryWeight', label: 'Watch History (Play Count)', description: 'Unwatched items score much higher.' },
  { key: 'lastWatchedWeight', label: 'Days Since Last Watched', description: 'Media not watched in a long time scores higher.' },
  { key: 'fileSizeWeight', label: 'File Size', description: 'Larger files score higher to free more space.' },
  { key: 'ratingWeight', label: 'Rating', description: 'Low-rated content scores higher for deletion.' },
  { key: 'timeInLibraryWeight', label: 'Time in Library', description: 'Older content may be less valuable.' },
  { key: 'seriesStatusWeight', label: 'Series Status', description: 'Ended or canceled shows score higher for removal since no new episodes are expected.' }
]

const presets = [
  { name: 'Balanced', values: { watchHistoryWeight: 8, lastWatchedWeight: 7, fileSizeWeight: 6, ratingWeight: 5, timeInLibraryWeight: 4, seriesStatusWeight: 3 } },
  { name: 'Space Saver', values: { watchHistoryWeight: 3, lastWatchedWeight: 3, fileSizeWeight: 10, ratingWeight: 2, timeInLibraryWeight: 8, seriesStatusWeight: 5 } },
  { name: 'Hoarder', values: { watchHistoryWeight: 10, lastWatchedWeight: 10, fileSizeWeight: 2, ratingWeight: 8, timeInLibraryWeight: 2, seriesStatusWeight: 2 } },
  { name: 'Watch-Based', values: { watchHistoryWeight: 10, lastWatchedWeight: 9, fileSizeWeight: 4, ratingWeight: 3, timeInLibraryWeight: 3, seriesStatusWeight: 5 } }
]

function isActivePreset(values: Record<string, number>): boolean {
  return Object.entries(values).every(
    ([key, val]) => prefs[key as keyof typeof prefs] === val
  )
}

const presetDescriptions: Record<string, string> = {
  'Balanced': 'A general-purpose profile that weighs all factors evenly. Good starting point.',
  'Space Saver': 'Prioritizes freeing disk space. Targets large, old media with low ratings.',
  'Hoarder': 'Strongly resists deletion. Only removes media that\'s never been watched and poorly rated.',
  'Watch-Based': 'Focuses on watch history. Unwatched and stale media is removed first.'
}

const activePresetDescription = computed(() => {
  const active = presets.find(p => isActivePreset(p.values))
  return active ? presetDescriptions[active.name] ?? '' : 'Custom configuration — adjust sliders to fine-tune scoring.'
})

// ---------------------------------------------------------------------------
// Custom Rules (Cascading Rule Builder)
// ---------------------------------------------------------------------------
const rules = ref<CustomRule[]>([])
const showAddRule = ref(false)
const allIntegrations = ref<IntegrationConfig[]>([])

// Operator label mapping for natural-language display
const operatorLabelMap: Record<string, string> = {
  '==': 'is',
  '!=': 'is not',
  'contains': 'contains',
  '!contains': 'does not contain',
  '>': 'more than',
  '>=': 'at least',
  '<': 'less than',
  '<=': 'at most'
}

// Effect label and badge style helpers
const effectLabelMap: Record<string, string> = {
  always_keep: 'Always keep',
  prefer_keep: 'Prefer to keep',
  lean_keep: 'Lean toward keeping',
  lean_remove: 'Lean toward removing',
  prefer_remove: 'Prefer to remove',
  always_remove: 'Always remove'
}

const effectBadgeClassMap: Record<string, string> = {
  always_keep: 'bg-transparent border-emerald-600/60 text-emerald-700 dark:text-emerald-300',
  prefer_keep: 'bg-transparent border-green-500/50 text-green-600 dark:text-green-400',
  lean_keep: 'bg-transparent border-sky-500/50 text-sky-600 dark:text-sky-400',
  lean_remove: 'bg-transparent border-amber-500/50 text-amber-700 dark:text-amber-400',
  prefer_remove: 'bg-transparent border-orange-500/50 text-orange-600 dark:text-orange-400',
  always_remove: 'bg-transparent border-red-600/60 text-red-700 dark:text-red-400'
}

const effectIconMap: Record<string, string> = {
  always_keep: '🛡️',
  prefer_keep: '🟢',
  lean_keep: '🔵',
  lean_remove: '🟡',
  prefer_remove: '🟠',
  always_remove: '🔴'
}

// Field label mapping for human-readable display
const fieldLabelMap: Record<string, string> = {
  title: 'Title',
  quality: 'Quality Profile',
  tag: 'Tags',
  genre: 'Genre',
  rating: 'Rating',
  sizebytes: 'Size',
  timeinlibrary: 'Time in Library',
  monitored: 'Monitored',
  year: 'Year',
  language: 'Language',
  seriesstatus: 'Series Status',
  seasoncount: 'Season Count',
  episodecount: 'Episode Count',
  playcount: 'Play Count',
  requested: 'Is Requested',
  requestcount: 'Request Count',
  type: 'Media Type'
}

function effectLabel(effect: string): string {
  return effectLabelMap[effect] ?? effect
}

function effectBadgeClass(effect: string): string {
  return effectBadgeClassMap[effect] ?? 'bg-muted text-foreground'
}

function operatorLabel(op: string): string {
  return operatorLabelMap[op] ?? op
}

function fieldLabel(field: string): string {
  return fieldLabelMap[field] ?? field
}

function integrationName(id: number): string {
  const svc = allIntegrations.value.find(i => i.id === id)
  if (!svc) return `Integration #${id}`
  const typeName = svc.type ? svc.type.charAt(0).toUpperCase() + svc.type.slice(1) : ''
  return typeName ? `${typeName}: ${svc.name}` : svc.name
}

// Convert legacy type+intensity to new effect (for display of pre-migration rules)
function legacyEffect(type: string, intensity: string): string {
  if (type === 'protect') {
    if (intensity === 'absolute') return 'always_keep'
    if (intensity === 'strong') return 'prefer_keep'
    return 'lean_keep'
  }
  if (type === 'target') {
    if (intensity === 'absolute') return 'always_remove'
    if (intensity === 'strong') return 'prefer_remove'
    return 'lean_remove'
  }
  return 'lean_keep'
}

// ─── Conflict Detection (Phase 3) ──────────────────────────────────────────────
// Determines if a rule has opposing-direction rules on the same integration instance.
// Uses value-aware overlap analysis to avoid false positives (e.g., non-overlapping
// numeric ranges on the same field are not flagged as conflicts).
// Returns an array of conflict description strings for the tooltip.
const keepEffects = new Set(['always_keep', 'prefer_keep', 'lean_keep'])
const removeEffects = new Set(['lean_remove', 'prefer_remove', 'always_remove'])

// Fields that use numeric values
const numericFields = new Set(['rating', 'sizebytes', 'timeinlibrary', 'year', 'seasoncount', 'episodecount', 'playcount', 'requestcount'])
// Fields that use boolean values
const booleanFields = new Set(['monitored', 'requested'])

function ruleEffectDirection(rule: CustomRule): 'keep' | 'remove' | 'unknown' {
  const eff = rule.effect || legacyEffect(rule.type, rule.intensity)
  if (keepEffects.has(eff)) return 'keep'
  if (removeEffects.has(eff)) return 'remove'
  return 'unknown'
}

/**
 * Check if two rules targeting the same field could ever match the same item.
 * Returns true if the rules' conditions could overlap, false if they're mutually exclusive.
 */
function rulesCouldOverlap(a: CustomRule, b: CustomRule): boolean {
  // Different fields target independent dimensions — a single media item can
  // match both (e.g., old AND well-rated), so they're not in conflict.
  if (a.field !== b.field) return false

  const field = a.field

  // Boolean fields: same value = overlap, different values = no overlap
  if (booleanFields.has(field)) {
    return a.value === b.value
  }

  // Numeric fields: check range intersection
  if (numericFields.has(field)) {
    return numericRangesOverlap(a.operator, Number(a.value), b.operator, Number(b.value))
  }

  // String fields with exact match operators and different values = no overlap
  if (a.operator === '==' && b.operator === '==') {
    return a.value === b.value
  }

  // Mutual exclusion: positive match vs negation of the same value
  const aVal = a.value.toLowerCase()
  const bVal = b.value.toLowerCase()

  // contains X vs !contains X → mutually exclusive
  if (a.operator === 'contains' && b.operator === '!contains' && aVal === bVal) return false
  if (a.operator === '!contains' && b.operator === 'contains' && aVal === bVal) return false

  // == X vs != X → mutually exclusive
  if (a.operator === '==' && b.operator === '!=' && aVal === bVal) return false
  if (a.operator === '!=' && b.operator === '==' && aVal === bVal) return false

  // == X vs !contains X → mutually exclusive (exact value always contains itself)
  if (a.operator === '==' && b.operator === '!contains' && aVal.includes(bVal)) return false
  if (a.operator === '!contains' && b.operator === '==' && bVal.includes(aVal)) return false

  // For all other mixed operators, conservatively assume overlap
  return true
}

/**
 * Determine if two numeric operator+value pairs describe overlapping ranges.
 * Each rule defines a half-open range. We check if the intersection is non-empty.
 */
function numericRangesOverlap(opA: string, valA: number, opB: string, valB: number): boolean {
  if (isNaN(valA) || isNaN(valB)) return true // can't determine — assume overlap

  // Convert operator+value into [min, max] ranges (inclusive)
  const rangeA = numericToRange(opA, valA)
  const rangeB = numericToRange(opB, valB)

  // Two ranges overlap if rangeA.min <= rangeB.max AND rangeA.max >= rangeB.min
  return rangeA[0] <= rangeB[1] && rangeA[1] >= rangeB[0]
}

/** Convert an operator+value into a [min, max] range tuple. */
function numericToRange(op: string, val: number): [number, number] {
  const INF = Number.MAX_SAFE_INTEGER
  const NEG_INF = Number.MIN_SAFE_INTEGER
  switch (op) {
    case '==': return [val, val]
    case '!=': return [NEG_INF, INF] // matches everything except val — conservatively full range
    case '>': return [val + 0.001, INF] // exclusive: just above val
    case '>=': return [val, INF]
    case '<': return [NEG_INF, val - 0.001] // exclusive: just below val
    case '<=': return [NEG_INF, val]
    default: return [NEG_INF, INF]
  }
}

function ruleConflicts(rule: CustomRule): string[] {
  const direction = ruleEffectDirection(rule)
  if (direction === 'unknown') return []
  const eff = rule.effect || legacyEffect(rule.type, rule.intensity)

  const conflicts: string[] = []
  for (const other of rules.value) {
    if (other.id === rule.id) continue
    const otherDirection = ruleEffectDirection(other)
    if (otherDirection === 'unknown' || otherDirection === direction) continue

    // Check if they could overlap — same integration or one is global
    const sameScope
      = (!rule.integrationId && !other.integrationId)
        || (!rule.integrationId || !other.integrationId)
        || (rule.integrationId === other.integrationId)

    if (!sameScope) continue

    // Check if the rules' conditions could ever match the same item
    if (!rulesCouldOverlap(rule, other)) continue

    const otherEff = other.effect || legacyEffect(other.type, other.intensity)
    const otherName = `${fieldLabel(other.field)} ${operatorLabel(other.operator)} "${other.value}" → ${effectLabel(otherEff)}`

    // Determine which wins
    if (eff === 'always_keep' || otherEff === 'always_keep') {
      conflicts.push(`Conflicts with "${otherName}". When both match, "Always keep" wins.`)
    } else {
      conflicts.push(`Conflicts with "${otherName}". When both match, effects multiply together.`)
    }
  }
  return conflicts
}

// Preview
const preview = ref<EvaluatedItem[]>([])
const previewLoading = ref(false)
const previewFetchedAt = ref<string>('')
const selectedPreviewItem = ref<SelectedDetailItem | null>(null)
const diskContext = ref<{ totalBytes: number, usedBytes: number, targetPct: number, thresholdPct: number, bytesToFree: number } | null>(null)

// Preview filters
const previewSearch = ref('')
const previewTypeFilter = ref<string | null>(null)
const previewStatusFilter = ref<'all' | 'protected' | 'unprotected'>('all')

// Preview sorting
type PreviewSortColumn = 'rank' | 'score' | 'title' | 'type' | 'size'
const previewSortBy = ref<PreviewSortColumn>('rank')
const previewSortDir = ref<'asc' | 'desc'>('asc')

function togglePreviewSort(column: PreviewSortColumn) {
  if (previewSortBy.value === column) {
    previewSortDir.value = previewSortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    previewSortBy.value = column
    previewSortDir.value = column === 'score' || column === 'size' ? 'desc' : 'asc'
  }
}

const previewMediaTypes = ['movie', 'show', 'season', 'artist', 'book'] as const

function selectPreviewItem(entry: EvaluatedItem) {
  // Preview API returns `factors` as a JSON array; ScoreDetailModal expects `scoreDetails` as a JSON string
  let scoreDetails = ''
  if (entry.factors && Array.isArray(entry.factors)) {
    scoreDetails = JSON.stringify(entry.factors)
  } else if (typeof entry.scoreDetails === 'string') {
    scoreDetails = entry.scoreDetails
  }
  selectedPreviewItem.value = {
    mediaName: entry.item?.title || 'Unknown',
    mediaType: entry.item?.type || 'unknown',
    _score: entry.score ?? 0,
    scoreDetails,
    sizeBytes: entry.item?.sizeBytes || 0,
    action: entry.isProtected ? 'Protected' : 'Preview',
    createdAt: previewFetchedAt.value || new Date().toISOString()
  }
}

onMounted(async () => {
  await Promise.all([fetchPreferences(), fetchRules(), fetchPreview(), fetchDiskGroups(), fetchIntegrations()])
})

async function fetchDiskGroups() {
  try {
    diskGroups.value = await api('/api/v1/disk-groups') as DiskGroup[]
  } catch {
    // Silently ignored — UI has no further handling
  }
}

async function fetchIntegrations() {
  try {
    allIntegrations.value = await api('/api/v1/integrations') as IntegrationConfig[]
  } catch {
    // Silently ignored — UI has no further handling
  }
}

async function fetchPreferences() {
  try {
    const data = await api('/api/v1/preferences') as PreferenceSet
    if (data?.id) {
      Object.assign(prefs, data)
    }
  } catch {
    // Silently ignored — UI has no further handling
  }
}

async function savePreferences() {
  try {
    await api('/api/v1/preferences', { method: 'PUT', body: { ...prefs, id: 1 } })
    addToast('Settings saved', 'success')
  } catch {
    addToast('Failed to save preferences', 'error')
  }
}

function applyPreset(values: Record<string, number>) {
  Object.assign(prefs, values)
  // Preset populates sliders but does NOT auto-save; user clicks Save
}

async function fetchRules() {
  try {
    rules.value = await api('/api/v1/custom-rules') as CustomRule[]
  } catch {
    // Silently ignored — UI has no further handling
  }
}

async function addRule(rule: { integrationId: number, field: string, operator: string, value: string, effect: string }) {
  try {
    await api('/api/v1/custom-rules', { method: 'POST', body: rule })
    showAddRule.value = false
    addToast('Rule added', 'success')
    await fetchRules()
    await fetchPreview()
  } catch {
    addToast('Failed to add rule', 'error')
  }
}

async function deleteRule(id: number) {
  try {
    await api(`/api/v1/custom-rules/${id}`, { method: 'DELETE' })
    addToast('Rule removed', 'success')
    await fetchRules()
    await fetchPreview()
  } catch {
    addToast('Failed to delete rule', 'error')
  }
}

// ─── Rule Enable/Disable Toggle ────────────────────────────────────────────────
async function toggleRuleEnabled(rule: CustomRule, enabled: boolean) {
  // Optimistically update local state
  rule.enabled = enabled
  try {
    await api(`/api/v1/custom-rules/${rule.id}`, {
      method: 'PUT',
      body: { ...rule, enabled }
    })
    addToast(enabled ? 'Rule enabled' : 'Rule disabled', 'success')
  } catch {
    // Revert on failure
    rule.enabled = !enabled
    addToast('Failed to update rule', 'error')
  }
}

// ─── Drag-to-Reorder ───────────────────────────────────────────────────────────
const dragSourceIdx = ref<number | null>(null)
const dragOverIdx = ref<number | null>(null)

function onDragStart(event: DragEvent, idx: number) {
  dragSourceIdx.value = idx
  if (event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', String(idx))
  }
}

function onDragOver(_event: DragEvent, idx: number) {
  dragOverIdx.value = idx
}

function onDragLeave() {
  dragOverIdx.value = null
}

function onDragEnd() {
  dragSourceIdx.value = null
  dragOverIdx.value = null
}

async function onDrop(_event: DragEvent, targetIdx: number) {
  const sourceIdx = dragSourceIdx.value
  dragSourceIdx.value = null
  dragOverIdx.value = null

  if (sourceIdx === null || sourceIdx === targetIdx) return

  // Reorder local array
  const reordered = [...rules.value]
  const [moved] = reordered.splice(sourceIdx, 1)
  reordered.splice(targetIdx, 0, moved)
  rules.value = reordered

  // Send new order to backend
  const order = reordered.map(r => r.id)
  try {
    await api('/api/v1/custom-rules/reorder', {
      method: 'PUT',
      body: { order }
    })
    addToast('Rules reordered', 'success')
  } catch {
    // Revert — re-fetch from server
    await fetchRules()
    addToast('Failed to reorder rules', 'error')
  }
}

async function fetchPreview() {
  previewLoading.value = true
  try {
    const data = await api('/api/v1/preview') as PreviewResponse
    preview.value = data.items || []
    diskContext.value = data.diskContext || null
    previewFetchedAt.value = new Date().toISOString()
  } catch {
    // Silently ignored — UI has no further handling
  } finally {
    previewLoading.value = false
  }
}

// ─── Preview Show/Season Grouping ─────────────────────────────────────────────
interface PreviewGroup {
  key: string
  entry: EvaluatedItem
  seasons: EvaluatedItem[]
}

const groupedPreview = computed<PreviewGroup[]>(() => {
  const items = preview.value
  const groups: PreviewGroup[] = []
  // Map from show name → index in groups array
  const showMap = new Map<string, number>()

  // Two-pass approach: first pass collects shows, second pass groups seasons
  // Pass 1: identify all show entries and create groups for them
  for (const item of items) {
    if (item.item?.type === 'show') {
      const key = `preview-${item.item.title}-show`
      showMap.set(item.item.title, groups.length)
      groups.push({ key, entry: item, seasons: [] })
    }
  }

  // Pass 2: attach seasons to their parent show, or create a synthetic show group
  for (const item of items) {
    if (item.item?.type === 'season' && item.item?.title?.includes(' - Season ')) {
      const showName = item.item.title.split(' - Season ')[0]
      const groupIdx = showMap.get(showName)
      if (groupIdx !== undefined && groups[groupIdx]) {
        groups[groupIdx].seasons.push(item)
      } else {
        // Season without a parent show entry — create a synthetic group using the season as the parent
        const syntheticKey = `preview-${showName}-show-synthetic`
        if (!showMap.has(showName)) {
          showMap.set(showName, groups.length)
          // Use the first season as the group entry but display the show name
          const syntheticEntry = {
            ...item,
            item: { ...item.item, title: showName, type: 'show' }
          }
          groups.push({ key: syntheticKey, entry: syntheticEntry, seasons: [item] })
        } else {
          // Already created a synthetic group, just add the season
          const existingIdx = showMap.get(showName)!
          groups[existingIdx].seasons.push(item)
        }
      }
    } else if (item.item?.type !== 'show') {
      // Non-show, non-season items (movies, artists, books, etc.)
      const key = `preview-${item.item?.title}-${item.item?.type}`
      groups.push({ key, entry: item, seasons: [] })
    }
    // Shows already handled in pass 1
  }

  // Filter out show-level entries with no seasons — they're only useful as grouping parents
  // A show with 0 seasons in the preview has nothing actionable to display
  return groups.filter(g => !(g.entry.item?.type === 'show' && g.seasons.length === 0))
})

// Filtered preview: applies search, type, status filters and sorting to groupedPreview
const filteredGroupedPreview = computed<PreviewGroup[]>(() => {
  let groups = groupedPreview.value
  const search = previewSearch.value.trim().toLowerCase()
  const typeFilter = previewTypeFilter.value
  const statusFilter = previewStatusFilter.value

  // Apply filters
  if (search || typeFilter || statusFilter !== 'all') {
    groups = groups.reduce<PreviewGroup[]>((result, group) => {
      const entry = group.entry
      const entryType = entry.item?.type
      const entryTitle = (entry.item?.title || '').toLowerCase()
      const entryProtected = !!entry.isProtected

      // For show groups, also check if any seasons match
      if (group.seasons.length > 0) {
        const filteredSeasons = group.seasons.filter((s) => {
          const sTitle = (s.item?.title || '').toLowerCase()
          const sType = s.item?.type
          const sProtected = !!s.isProtected
          const matchSearch = !search || sTitle.includes(search) || entryTitle.includes(search)
          const matchType = !typeFilter || sType === typeFilter || entryType === typeFilter
          const matchStatus = statusFilter === 'all' || (statusFilter === 'protected' ? sProtected : !sProtected)
          return matchSearch && matchType && matchStatus
        })

        // Also check if the parent entry matches
        const parentMatchSearch = !search || entryTitle.includes(search)
        const parentMatchType = !typeFilter || entryType === typeFilter
        const parentMatchStatus = statusFilter === 'all' || (statusFilter === 'protected' ? entryProtected : !entryProtected)

        if (filteredSeasons.length > 0) {
          result.push({ ...group, seasons: filteredSeasons })
        } else if (parentMatchSearch && parentMatchType && parentMatchStatus) {
          result.push({ ...group, seasons: [] })
        }
      } else {
        // Non-grouped entries (movies, artists, books, etc.)
        const matchSearch = !search || entryTitle.includes(search)
        const matchType = !typeFilter || entryType === typeFilter
        const matchStatus = statusFilter === 'all' || (statusFilter === 'protected' ? entryProtected : !entryProtected)
        if (matchSearch && matchType && matchStatus) {
          result.push(group)
        }
      }
      return result
    }, [])
  }

  // Apply sorting
  const sortBy = previewSortBy.value
  const sortDir = previewSortDir.value
  if (sortBy === 'rank' && sortDir === 'asc') return groups // natural order

  const sorted = [...groups]
  const dir = sortDir === 'asc' ? 1 : -1

  sorted.sort((a, b) => {
    switch (sortBy) {
      case 'rank':
        // Rank is the natural index from groupedPreview; use indexOf
        return dir * (groupedPreview.value.indexOf(a) - groupedPreview.value.indexOf(b))
      case 'score': {
        const scoreA = a.entry.isProtected ? Infinity : (a.entry.score ?? 0)
        const scoreB = b.entry.isProtected ? Infinity : (b.entry.score ?? 0)
        return dir * (scoreA - scoreB)
      }
      case 'title': {
        const titleA = (a.entry.item?.title || '').toLowerCase()
        const titleB = (b.entry.item?.title || '').toLowerCase()
        return dir * titleA.localeCompare(titleB)
      }
      case 'type': {
        const typeA = (a.entry.item?.type || '').toLowerCase()
        const typeB = (b.entry.item?.type || '').toLowerCase()
        return dir * typeA.localeCompare(typeB)
      }
      case 'size': {
        const sizeA = a.entry.item?.sizeBytes ?? 0
        const sizeB = b.entry.item?.sizeBytes ?? 0
        return dir * (sizeA - sizeB)
      }
      default:
        return 0
    }
  })

  return sorted
})

// Deletion line: index in filteredGroupedPreview where cumulative size exceeds bytesToFree
const deletionLineIndex = computed<number | null>(() => {
  const ctx = diskContext.value
  if (!ctx || ctx.bytesToFree <= 0) return null

  const groups = filteredGroupedPreview.value
  let cumulative = 0
  for (let i = 0; i < groups.length; i++) {
    const group = groups[i]
    // Skip protected items — the engine wouldn't delete them
    if (group.entry.isProtected) continue
    // Accumulate group entry size plus any season sizes
    cumulative += group.entry.item?.sizeBytes ?? 0
    if (group.seasons.length > 0) {
      for (const season of group.seasons) {
        if (!season.isProtected) {
          cumulative += season.item?.sizeBytes ?? 0
        }
      }
    }
    if (cumulative >= ctx.bytesToFree) {
      return i + 1 // Line goes *after* this item
    }
  }
  return null // Not enough items to reach the target
})

// ─── Progressive Rendering (Virtual Scroll) ─────────────────────────────────
// Render rows incrementally as user scrolls to avoid DOM overload with 1000+ groups.
const tableScrollRef = ref<HTMLElement | null>(null)
const visibleCount = ref(100)

const renderedGroups = computed(() => filteredGroupedPreview.value.slice(0, visibleCount.value))

useInfiniteScroll(tableScrollRef, () => {
  if (visibleCount.value < filteredGroupedPreview.value.length) {
    visibleCount.value = Math.min(visibleCount.value + 100, filteredGroupedPreview.value.length)
  }
}, { distance: 200, canLoadMore: () => visibleCount.value < filteredGroupedPreview.value.length })

// Reset visible count when filters or data change
watch([previewSearch, previewTypeFilter, previewStatusFilter, previewSortBy, previewSortDir, preview], () => {
  visibleCount.value = 100
})

// Seasons collapsed by default — user clicks to expand
const expandedPreviewGroups = ref(new Set<string>())

function togglePreviewGroup(key: string) {
  const next = new Set(expandedPreviewGroups.value)
  if (next.has(key)) {
    next.delete(key)
  } else {
    next.add(key)
  }
  expandedPreviewGroups.value = next
}

function extractPreviewSeasonLabel(title: string): string {
  const parts = title.split(' - Season ')
  return parts.length > 1 ? `Season ${parts[parts.length - 1]}` : title
}
</script>
