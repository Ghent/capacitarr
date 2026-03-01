<template>
  <div>
    <!-- Header -->
    <div data-slot="page-header" class="mb-8">
      <h1 class="text-3xl font-bold tracking-tight">Scoring Engine</h1>
      <p class="text-muted-foreground mt-1.5">
        Adjust preference weights and set custom rules.
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
        <UiCardTitle>Disk Thresholds</UiCardTitle>
        <UiCardDescription>
          Set when cleanup begins (threshold) and when it stops (target) for each disk.
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
                  :class="diskUsagePct(dg) >= (thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct) ? 'bg-red-500' : diskUsagePct(dg) >= (thresholdEdits[dg.id]?.target ?? dg.targetPct) ? 'bg-amber-500' : 'bg-green-500'"
              >
                <component :is="HardDriveIcon" class="w-4.5 h-4.5 text-white" />
              </div>
              <div>
                <div class="text-sm font-medium text-foreground truncate" :title="dg.mountPath">
                  {{ dg.mountPath }}
                </div>
                <span class="text-xs text-muted-foreground">
                  {{ formatBytes(dg.usedBytes) }} / {{ formatBytes(dg.totalBytes) }}
                </span>
              </div>
            </div>
            <span class="text-2xl font-bold tabular-nums" :class="diskUsagePct(dg) >= (thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct) ? 'text-red-500' : diskUsagePct(dg) >= (thresholdEdits[dg.id]?.target ?? dg.targetPct) ? 'text-amber-500' : 'text-green-500'">
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
                :data-status="diskUsagePct(dg) >= (thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct) ? 'danger' : diskUsagePct(dg) >= (thresholdEdits[dg.id]?.target ?? dg.targetPct) ? 'warning' : 'ok'"
                class="relative h-full rounded-full transition-all duration-700 ease-out z-10"
                :style="{ width: Math.min(diskUsagePct(dg), 100) + '%', backgroundColor: diskUsagePct(dg) >= (thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct) ? '#ef4444' : diskUsagePct(dg) >= (thresholdEdits[dg.id]?.target ?? dg.targetPct) ? '#eab308' : '#22c55e' }"
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
              <UiLabel>Cleanup Threshold %</UiLabel>
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
              <p class="text-[11px] text-muted-foreground">Begin cleanup when usage exceeds this %</p>
            </div>
            <div class="space-y-1.5">
              <UiLabel>Cleanup Target %</UiLabel>
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
              <p class="text-[11px] text-muted-foreground">Stop cleanup when usage drops to this %</p>
            </div>
          </div>

          <!-- Validation error -->
          <p v-if="thresholdValidation(dg.id, dg)" class="text-xs text-red-500">
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
              <span v-if="thresholdEdits[dg.id]?.saving" class="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
                <component :is="LoaderCircleIcon" class="w-3.5 h-3.5 animate-spin" />
                Saving…
              </span>
              <span v-else-if="thresholdEdits[dg.id]?.success && thresholdEdits[dg.id]?.message" class="inline-flex items-center gap-1.5 text-xs text-emerald-500">
                <component :is="CheckIcon" class="w-3.5 h-3.5" />
                Saved
              </span>
              <span v-else-if="thresholdEdits[dg.id]?.message && !thresholdEdits[dg.id]?.success" class="inline-flex items-center gap-1.5 text-xs text-red-500">
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
            <UiCardTitle>Preference Weights</UiCardTitle>
            <UiCardDescription>
              Higher weights increase the attribute's influence on deletion score.
            </UiCardDescription>
          </div>
          <UiButton size="sm" @click="savePreferences">
            Save Weights
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
          <p :key="activePresetDescription" class="text-xs text-muted-foreground/70 mb-6 leading-relaxed">
            {{ activePresetDescription }}
          </p>
        </Transition>

        <!-- Two-Column Slider Grid -->
        <div class="grid grid-cols-1 md:grid-cols-2 gap-x-8 gap-y-5">
          <div v-for="slider in sliders" :key="slider.key" class="space-y-1.5">
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
              @update:model-value="(v: number[] | undefined) => { if (v) (prefs as any)[slider.key] = v[0] }"
            />
            <p class="text-xs text-muted-foreground">{{ slider.description }}</p>
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
            <UiCardTitle>Custom Rules</UiCardTitle>
            <UiCardDescription class="mt-1">
              Rules take effect on the next engine run, not immediately.
              When multiple rules match an item, their effects multiply together.
              "Always keep" is an absolute override and cannot be outweighed by any other rule.
            </UiCardDescription>
          </div>
          <UiButton size="sm" @click="showAddRule = !showAddRule">
            <component :is="PlusIcon" class="w-3.5 h-3.5" />
            Add Rule
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
        <div v-if="rules.length === 0 && !showAddRule" class="text-center py-6 text-muted-foreground text-sm">
          No rules configured. Media will be ranked purely by preference weights.
        </div>
        <div v-else class="space-y-2">
          <div
            v-for="(rule, ruleIdx) in rules"
            :key="rule.id"
            class="flex items-center justify-between px-4 py-2.5 rounded-lg border bg-muted/50"
            :class="ruleConflicts(rule).length > 0 ? 'border-amber-400/50' : 'border-border'"
          >
            <div class="flex items-center gap-2 text-sm flex-wrap">
              <!-- Rule number -->
              <span class="text-xs font-mono tabular-nums text-muted-foreground w-5 shrink-0">{{ ruleIdx + 1 }}.</span>
              <!-- Conflict indicator -->
              <UiTooltipProvider v-if="ruleConflicts(rule).length > 0">
                <UiTooltip>
                  <UiTooltipTrigger as-child>
                    <span class="inline-flex items-center shrink-0 cursor-help">
                      <component :is="AlertTriangleIcon" class="w-4 h-4 text-amber-500" />
                    </span>
                  </UiTooltipTrigger>
                  <UiTooltipContent side="top" class="max-w-xs text-xs">
                    <p v-for="(conflict, idx) in ruleConflicts(rule)" :key="idx" class="mb-1 last:mb-0">
                      {{ conflict }}
                    </p>
                  </UiTooltipContent>
                </UiTooltip>
              </UiTooltipProvider>
              <!-- Effect badge -->
              <UiBadge
                :class="effectBadgeClass(rule.effect || legacyEffect(rule.type, rule.intensity))"
                class="shrink-0"
              >
                <span class="inline-flex items-center gap-1">
                  <span class="text-xs">{{ effectIconMap[rule.effect || legacyEffect(rule.type, rule.intensity)] || '' }}</span>
                  {{ effectLabel(rule.effect || legacyEffect(rule.type, rule.intensity)) }}
                </span>
              </UiBadge>
              <!-- Service name -->
              <span v-if="rule.integrationId" class="text-muted-foreground">
                {{ integrationName(rule.integrationId) }} ·
              </span>
              <!-- Human-readable condition -->
              <span class="text-foreground">{{ fieldLabel(rule.field) }}</span>
              <span class="text-muted-foreground">{{ operatorLabel(rule.operator) }}</span>
              <span class="font-medium">"{{ rule.value }}"</span>
            </div>
            <UiButton
              variant="ghost"
              size="icon-sm"
              class="text-muted-foreground hover:text-red-500 shrink-0"
              @click="deleteRule(rule.id)"
            >
              <component :is="XIcon" class="w-4 h-4" />
            </UiButton>
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
          <UiCardTitle>Live Preview — What Would Be Deleted</UiCardTitle>
          <UiButton variant="outline" size="sm" @click="fetchPreview">
            <component :is="previewLoading ? LoaderCircleIcon : RefreshCwIcon" :class="{ 'animate-spin': previewLoading }" class="w-3.5 h-3.5" />
            Refresh
          </UiButton>
        </div>
      </UiCardHeader>
      <UiCardContent>
        <div v-if="previewLoading" class="flex items-center justify-center py-12">
          <component :is="LoaderCircleIcon" class="w-6 h-6 text-primary animate-spin" />
        </div>

        <div v-else-if="preview.length === 0" class="text-center py-8 text-muted-foreground text-sm">
          No items to evaluate. Connect integrations and ensure media exists.
        </div>

        <div v-else>
          <!-- Search & Filters -->
          <div class="flex flex-col sm:flex-row gap-3 mb-4">
            <div class="relative flex-1">
              <SearchIcon class="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground pointer-events-none" />
              <UiInput
                v-model="previewSearch"
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
              <UiSeparator orientation="vertical" class="h-5 mx-1" />
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
          <div v-if="previewSearch || previewTypeFilter || previewStatusFilter !== 'all'" class="text-xs text-muted-foreground mb-2">
            {{ filteredGroupedPreview.length }} of {{ groupedPreview.length }} items
          </div>

          <div v-if="filteredGroupedPreview.length === 0" class="text-center py-8 text-muted-foreground text-sm">
            No items match filters.
          </div>

          <div v-else class="overflow-x-auto max-h-[600px] overflow-y-auto relative">
          <UiTable>
            <UiTableHeader class="sticky top-0 z-10 bg-background">
              <UiTableRow>
                <UiTableHead
                  class="w-12 cursor-pointer select-none group"
                  @click="togglePreviewSort('rank')"
                >
                  <span class="inline-flex items-center gap-1">
                    #
                    <ArrowUpIcon v-if="previewSortBy === 'rank' && previewSortDir === 'asc'" class="w-3 h-3" />
                    <ArrowDownIcon v-else-if="previewSortBy === 'rank' && previewSortDir === 'desc'" class="w-3 h-3" />
                    <ArrowUpDownIcon v-else class="w-3 h-3 opacity-0 group-hover:opacity-50 transition-opacity" />
                  </span>
                </UiTableHead>
                <UiTableHead
                  class="cursor-pointer select-none group"
                  @click="togglePreviewSort('score')"
                >
                  <span class="inline-flex items-center gap-1">
                    Score
                    <ArrowUpIcon v-if="previewSortBy === 'score' && previewSortDir === 'asc'" class="w-3 h-3" />
                    <ArrowDownIcon v-else-if="previewSortBy === 'score' && previewSortDir === 'desc'" class="w-3 h-3" />
                    <ArrowUpDownIcon v-else class="w-3 h-3 opacity-0 group-hover:opacity-50 transition-opacity" />
                  </span>
                </UiTableHead>
                <UiTableHead
                  class="cursor-pointer select-none group"
                  @click="togglePreviewSort('title')"
                >
                  <span class="inline-flex items-center gap-1">
                    Title
                    <ArrowUpIcon v-if="previewSortBy === 'title' && previewSortDir === 'asc'" class="w-3 h-3" />
                    <ArrowDownIcon v-else-if="previewSortBy === 'title' && previewSortDir === 'desc'" class="w-3 h-3" />
                    <ArrowUpDownIcon v-else class="w-3 h-3 opacity-0 group-hover:opacity-50 transition-opacity" />
                  </span>
                </UiTableHead>
                <UiTableHead
                  class="cursor-pointer select-none group"
                  @click="togglePreviewSort('type')"
                >
                  <span class="inline-flex items-center gap-1">
                    Type
                    <ArrowUpIcon v-if="previewSortBy === 'type' && previewSortDir === 'asc'" class="w-3 h-3" />
                    <ArrowDownIcon v-else-if="previewSortBy === 'type' && previewSortDir === 'desc'" class="w-3 h-3" />
                    <ArrowUpDownIcon v-else class="w-3 h-3 opacity-0 group-hover:opacity-50 transition-opacity" />
                  </span>
                </UiTableHead>
                <UiTableHead
                  class="text-right cursor-pointer select-none group"
                  @click="togglePreviewSort('size')"
                >
                  <span class="inline-flex items-center gap-1 justify-end">
                    Size
                    <ArrowUpIcon v-if="previewSortBy === 'size' && previewSortDir === 'asc'" class="w-3 h-3" />
                    <ArrowDownIcon v-else-if="previewSortBy === 'size' && previewSortDir === 'desc'" class="w-3 h-3" />
                    <ArrowUpDownIcon v-else class="w-3 h-3 opacity-0 group-hover:opacity-50 transition-opacity" />
                  </span>
                </UiTableHead>
              </UiTableRow>
            </UiTableHeader>
            <UiTableBody>
              <template v-for="(group, groupIdx) in filteredGroupedPreview" :key="group.key">
                <UiTableRow class="cursor-pointer" @click="selectPreviewItem(group.entry); group.seasons.length > 0 && togglePreviewGroup(group.key)">
                  <UiTableCell class="w-12 text-center">
                    <span class="text-xs font-mono tabular-nums text-muted-foreground">{{ groupIdx + 1 }}</span>
                  </UiTableCell>
                  <UiTableCell>
                    <span class="text-xs font-mono tabular-nums font-semibold" :class="group.entry.isProtected ? 'text-emerald-500' : 'text-primary'">
                      {{ group.entry.isProtected ? 'Protected' : group.entry.score.toFixed(2) }}
                    </span>
                  </UiTableCell>
                  <UiTableCell class="font-medium">
                    <div class="flex items-center gap-2">
                      <span class="truncate">{{ group.entry.item.title }}</span>
                      <button v-if="group.seasons.length > 0" class="text-muted-foreground hover:text-foreground transition-colors shrink-0 inline-flex items-center gap-0.5" @click.stop="togglePreviewGroup(group.key)">
                        <ChevronRightIcon class="w-3.5 h-3.5 transition-transform duration-200" :class="{ 'rotate-90': expandedPreviewGroups.has(group.key) }" />
                        <span class="text-xs text-muted-foreground font-normal whitespace-nowrap">({{ group.seasons.length }} season{{ group.seasons.length !== 1 ? 's' : '' }})</span>
                      </button>
                    </div>
                  </UiTableCell>
                  <UiTableCell>
                    <UiBadge variant="secondary" class="capitalize">{{ group.entry.item.type }}</UiBadge>
                  </UiTableCell>
                  <UiTableCell class="text-right font-mono text-xs tabular-nums">{{ formatBytes(group.entry.item.sizeBytes) }}</UiTableCell>
                </UiTableRow>
                <template v-if="expandedPreviewGroups.has(group.key)">
                  <UiTableRow v-for="(season, sIdx) in group.seasons" :key="`${group.key}-s${sIdx}`" class="bg-muted/30 cursor-pointer" @click.stop="selectPreviewItem(season)">
                    <UiTableCell class="w-12" />
                    <UiTableCell>
                      <span class="text-xs font-mono tabular-nums font-semibold" :class="season.isProtected ? 'text-emerald-500' : 'text-primary'">
                        {{ season.isProtected ? 'Protected' : season.score.toFixed(2) }}
                      </span>
                    </UiTableCell>
                    <UiTableCell class="text-muted-foreground pl-8">
                      <span class="inline-flex items-center gap-1.5">
                        <UiSeparator orientation="horizontal" class="w-3" />
                        {{ extractPreviewSeasonLabel(season.item.title) }}
                      </span>
                    </UiTableCell>
                    <UiTableCell>
                      <UiBadge variant="secondary" class="capitalize">{{ season.item.type }}</UiBadge>
                    </UiTableCell>
                    <UiTableCell class="text-right font-mono text-xs tabular-nums text-muted-foreground">{{ formatBytes(season.item.sizeBytes) }}</UiTableCell>
                  </UiTableRow>
                </template>
              </template>
            </UiTableBody>
          </UiTable>
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
import { PlusIcon, XIcon, RefreshCwIcon, LoaderCircleIcon, SaveIcon, CheckIcon, ChevronRightIcon, HardDriveIcon, AlertTriangleIcon, SearchIcon, ShieldCheckIcon, FilterIcon, ArrowUpIcon, ArrowDownIcon, ArrowUpDownIcon } from 'lucide-vue-next'
import { formatBytes } from '~/utils/format'

const api = useApi()
const config = useRuntimeConfig()
const apiBase = `${config.public.apiBaseUrl}/api/v1`
const { addToast } = useToast()

// Disk Groups
const diskGroups = ref<any[]>([])

// Per-disk-group threshold editing state
const thresholdEdits = reactive<Record<number, {
  threshold: number
  target: number
  saving: boolean
  message: string
  success: boolean
}>>({})

function diskUsagePct(dg: any): number {
  if (!dg.totalBytes || dg.totalBytes === 0) return 0
  return (dg.usedBytes / dg.totalBytes) * 100
}

function ensureThresholdEdit(dgId: number, dg: any) {
  if (!thresholdEdits[dgId]) {
    thresholdEdits[dgId] = {
      threshold: dg.thresholdPct,
      target: dg.targetPct,
      saving: false,
      message: '',
      success: false,
    }
  }
}

// Debounce timers for auto-save per disk group
const debounceTimers: Record<number, ReturnType<typeof setTimeout>> = {}

function updateThresholdEdit(dgId: number, field: 'threshold' | 'target', value: number, dg: any) {
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

function thresholdValidation(dgId: number, dg: any): string {
  const edit = thresholdEdits[dgId]
  const t = edit?.threshold ?? dg.thresholdPct
  const g = edit?.target ?? dg.targetPct
  if (t == null || g == null) return 'Both values are required'
  if (t < 1 || t > 99 || g < 1 || g > 99) return 'Values must be between 1 and 99'
  if (t <= g) return 'Threshold must be greater than target'
  return ''
}

async function saveThresholds(dg: any) {
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
        targetPct: edit.target,
      },
    }) as any

    edit.success = true
    edit.message = 'Saved'

    // Update local diskGroups array with canonical values from the API response
    const idx = diskGroups.value.findIndex((g: any) => g.id === dg.id)
    if (idx !== -1 && updated) {
      diskGroups.value[idx] = { ...diskGroups.value[idx], ...updated }
    } else if (idx !== -1) {
      diskGroups.value[idx].thresholdPct = edit.threshold
      diskGroups.value[idx].targetPct = edit.target
    }

    setTimeout(() => { edit.message = ''; edit.success = false }, 2500)
  } catch (err: any) {
    edit.success = false
    edit.message = err.message || 'Failed to save thresholds'
    addToast('Failed to save: ' + (err.message || 'Unknown error'), 'error')
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
  availabilityWeight: 3,
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
  { key: 'availabilityWeight', label: 'Availability (Show Status)', description: 'Ended shows score higher than continuing.' }
]

const presets = [
  { name: 'Balanced', values: { watchHistoryWeight: 8, lastWatchedWeight: 7, fileSizeWeight: 6, ratingWeight: 5, timeInLibraryWeight: 4, availabilityWeight: 3 } },
  { name: 'Space Saver', values: { watchHistoryWeight: 3, lastWatchedWeight: 3, fileSizeWeight: 10, ratingWeight: 2, timeInLibraryWeight: 8, availabilityWeight: 5 } },
  { name: 'Hoarder', values: { watchHistoryWeight: 10, lastWatchedWeight: 10, fileSizeWeight: 2, ratingWeight: 8, timeInLibraryWeight: 2, availabilityWeight: 2 } },
  { name: 'Watch-Based', values: { watchHistoryWeight: 10, lastWatchedWeight: 9, fileSizeWeight: 4, ratingWeight: 3, timeInLibraryWeight: 3, availabilityWeight: 5 } }
]

function isActivePreset(values: Record<string, number>): boolean {
  return Object.entries(values).every(
    ([key, val]) => prefs[key as keyof typeof prefs] === val
  )
}

const presetDescriptions: Record<string, string> = {
  Balanced: 'A general-purpose profile that weighs all factors evenly. Good starting point.',
  'Space Saver': 'Prioritizes freeing disk space. Targets large, old media with low ratings.',
  Hoarder: 'Strongly resists deletion. Only removes media that\'s never been watched and poorly rated.',
  'Watch-Based': 'Focuses on watch history. Unwatched and stale media is removed first.',
}

const activePresetDescription = computed(() => {
  const active = presets.find(p => isActivePreset(p.values))
  return active ? presetDescriptions[active.name] ?? '' : 'Custom configuration — adjust sliders to fine-tune scoring.'
})

// ---------------------------------------------------------------------------
// Custom Rules (Cascading Rule Builder)
// ---------------------------------------------------------------------------
const rules = ref<any[]>([])
const showAddRule = ref(false)
const allIntegrations = ref<any[]>([])

// Operator label mapping for natural-language display
const operatorLabelMap: Record<string, string> = {
  '==': 'is',
  '!=': 'is not',
  'contains': 'contains',
  '!contains': 'does not contain',
  '>': 'more than',
  '>=': 'at least',
  '<': 'less than',
  '<=': 'at most',
}

// Effect label and badge style helpers
const effectLabelMap: Record<string, string> = {
  always_keep: 'Always keep',
  prefer_keep: 'Prefer to keep',
  lean_keep: 'Lean toward keeping',
  lean_remove: 'Lean toward removing',
  prefer_remove: 'Prefer to remove',
  always_remove: 'Always remove',
}

const effectBadgeClassMap: Record<string, string> = {
  always_keep: 'bg-emerald-500 text-white hover:bg-emerald-500',
  prefer_keep: 'bg-teal-400 text-white hover:bg-teal-400',
  lean_keep: 'bg-sky-400 text-white hover:bg-sky-400',
  lean_remove: 'bg-amber-400 text-amber-900 hover:bg-amber-400',
  prefer_remove: 'bg-orange-500 text-white hover:bg-orange-500',
  always_remove: 'bg-red-500 text-white hover:bg-red-500',
}

const effectIconMap: Record<string, string> = {
  always_keep: '🛡️',
  prefer_keep: '🟢',
  lean_keep: '🔵',
  lean_remove: '🟡',
  prefer_remove: '🟠',
  always_remove: '🔴',
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
  availability: 'Show Status',
  seasoncount: 'Season Count',
  episodecount: 'Episode Count',
  playcount: 'Play Count',
  requested: 'Is Requested',
  requestcount: 'Request Count',
  type: 'Media Type',
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
  const svc = allIntegrations.value.find((i: any) => i.id === id)
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
// Returns an array of conflict description strings for the tooltip.
const keepEffects = new Set(['always_keep', 'prefer_keep', 'lean_keep'])
const removeEffects = new Set(['lean_remove', 'prefer_remove', 'always_remove'])

function ruleEffectDirection(rule: any): 'keep' | 'remove' | 'unknown' {
  const eff = rule.effect || legacyEffect(rule.type, rule.intensity)
  if (keepEffects.has(eff)) return 'keep'
  if (removeEffects.has(eff)) return 'remove'
  return 'unknown'
}

function ruleConflicts(rule: any): string[] {
  const direction = ruleEffectDirection(rule)
  if (direction === 'unknown') return []
  const eff = rule.effect || legacyEffect(rule.type, rule.intensity)

  const conflicts: string[] = []
  for (const other of rules.value) {
    if (other.id === rule.id) continue
    const otherDirection = ruleEffectDirection(other)
    if (otherDirection === 'unknown' || otherDirection === direction) continue

    // Check if they could overlap — same integration or one is global
    const sameScope =
      (!rule.integrationId && !other.integrationId) ||
      (!rule.integrationId || !other.integrationId) ||
      (rule.integrationId === other.integrationId)

    if (!sameScope) continue

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
const preview = ref<any[]>([])
const previewLoading = ref(false)
const previewFetchedAt = ref<string>('')
const selectedPreviewItem = ref<any | null>(null)

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

function selectPreviewItem(entry: any) {
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
    createdAt: previewFetchedAt.value || new Date().toISOString(),
  }
}

onMounted(async () => {
  await Promise.all([fetchPreferences(), fetchRules(), fetchPreview(), fetchDiskGroups(), fetchIntegrations()])
})

async function fetchDiskGroups() {
  try {
    diskGroups.value = await api('/api/v1/disk-groups') as any[]
  } catch (e) {
    console.error('Failed to fetch disk groups', e)
  }
}

async function fetchIntegrations() {
  try {
    allIntegrations.value = await api('/api/v1/integrations') as any[]
  } catch (e) {
    console.error('Failed to fetch integrations', e)
  }
}

async function fetchPreferences() {
  try {
    const data = await api('/api/v1/preferences') as any
    if (data?.id) {
      Object.assign(prefs, data)
    }
  } catch (e) {
    console.error('Failed to fetch preferences', e)
  }
}

async function savePreferences() {
  try {
    await api('/api/v1/preferences', { method: 'PUT', body: { ...prefs, id: 1 } })
    addToast('Settings saved', 'success')
  } catch (e) {
    console.error('Failed to save preferences', e)
    addToast('Failed to save preferences', 'error')
  }
}

function applyPreset(values: Record<string, number>) {
  Object.assign(prefs, values)
  // Preset populates sliders but does NOT auto-save; user clicks Save
}

async function fetchRules() {
  try {
    rules.value = await api('/api/v1/protections') as any[]
  } catch (e) {
    console.error('Failed to fetch rules', e)
  }
}

async function addRule(rule: { integrationId: number; field: string; operator: string; value: string; effect: string }) {
  try {
    await api('/api/v1/protections', { method: 'POST', body: rule })
    showAddRule.value = false
    addToast('Rule added', 'success')
    await fetchRules()
    await fetchPreview()
  } catch (e) {
    console.error('Failed to add rule', e)
    addToast('Failed to add rule', 'error')
  }
}

async function deleteRule(id: number) {
  try {
    await api(`/api/v1/protections/${id}`, { method: 'DELETE' })
    addToast('Rule removed', 'success')
    await fetchRules()
    await fetchPreview()
  } catch (e) {
    console.error('Failed to delete rule', e)
    addToast('Failed to delete rule', 'error')
  }
}

async function fetchPreview() {
  previewLoading.value = true
  try {
    preview.value = await api('/api/v1/preview') as any[]
    previewFetchedAt.value = new Date().toISOString()
  } catch (e) {
    console.error('Failed to fetch preview', e)
  } finally {
    previewLoading.value = false
  }
}

function scoreColor(score: number) {
  if (score >= 0.7) return 'bg-primary'
  if (score >= 0.4) return 'bg-primary/70'
  return 'bg-primary/40'
}


// ─── Preview Show/Season Grouping ─────────────────────────────────────────────
interface PreviewGroup {
  key: string
  entry: any
  seasons: any[]
}

const groupedPreview = computed<PreviewGroup[]>(() => {
  const items = preview.value.slice(0, 50)
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
            item: { ...item.item, title: showName, type: 'show' },
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
        const filteredSeasons = group.seasons.filter((s: any) => {
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
