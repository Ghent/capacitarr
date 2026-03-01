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
    <div
      v-if="diskGroups.length > 0"
      v-motion
      :initial="{ opacity: 0, y: 12 }"
      :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24 } }"
      class="rounded-xl border border-border bg-card shadow-sm p-6 mb-6"
    >
      <div class="mb-4">
        <h3 class="text-lg font-semibold">Disk Thresholds</h3>
        <p class="text-sm text-muted-foreground">
          Set when cleanup begins (threshold) and when it stops (target) for each disk.
        </p>
      </div>

      <div class="space-y-5">
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
                :class="diskUsagePct(dg) >= (thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct) ? 'bg-red-500' : diskUsagePct(dg) >= (thresholdEdits[dg.id]?.target ?? dg.targetPct) ? 'bg-amber-500' : 'bg-primary'"
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
            <span class="text-2xl font-bold tabular-nums" :class="diskUsagePct(dg) >= (thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct) ? 'text-red-500' : diskUsagePct(dg) >= (thresholdEdits[dg.id]?.target ?? dg.targetPct) ? 'text-amber-500' : 'text-primary'">
              {{ Math.round(diskUsagePct(dg)) }}%
            </span>
          </div>

          <!-- Progress bar with segmented zone background + triangle markers -->
          <div class="relative w-full mt-8 mb-6">
            <!-- Bar container -->
            <div class="relative w-full h-3 rounded-full overflow-hidden">
              <!-- Segmented background zones -->
              <div class="absolute inset-0 flex">
                <!-- Green zone: 0% → target% -->
                <div
                  class="h-full"
                  :style="{ width: (thresholdEdits[dg.id]?.target ?? dg.targetPct) + '%', backgroundColor: 'oklch(0.648 0.2 160 / 0.2)' }"
                />
                <!-- Amber zone: target% → threshold% -->
                <div
                  class="h-full"
                  :style="{ width: ((thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct) - (thresholdEdits[dg.id]?.target ?? dg.targetPct)) + '%', backgroundColor: 'oklch(0.75 0.183 55.934 / 0.2)' }"
                />
                <!-- Red zone: threshold% → 100% -->
                <div
                  class="h-full"
                  :style="{ width: (100 - (thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct)) + '%', backgroundColor: 'oklch(0.577 0.245 27.325 / 0.2)' }"
                />
              </div>
              <!-- Usage fill bar (on top of zones) -->
              <div
                data-slot="progress-bar-fill"
                :data-status="diskUsagePct(dg) >= (thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct) ? 'danger' : diskUsagePct(dg) >= (thresholdEdits[dg.id]?.target ?? dg.targetPct) ? 'warning' : 'ok'"
                class="relative h-full rounded-full transition-all duration-700 ease-out z-10"
                :style="{ width: Math.min(diskUsagePct(dg), 100) + '%', backgroundColor: diskUsagePct(dg) >= (thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct) ? 'oklch(0.637 0.237 25.331)' : diskUsagePct(dg) >= (thresholdEdits[dg.id]?.target ?? dg.targetPct) ? 'oklch(0.769 0.188 70.08)' : 'var(--color-primary)' }"
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
            <div>
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">
                Cleanup Threshold %
              </label>
              <div class="flex items-center gap-2">
                <input
                  :value="thresholdEdits[dg.id]?.threshold ?? dg.thresholdPct"
                  type="number"
                  min="1"
                  max="99"
                  class="w-full px-3 py-2 text-sm rounded-lg border border-input bg-card text-foreground focus:ring-2 focus:ring-red-400/50 focus:border-red-400 outline-none transition-colors"
                  @input="(e: Event) => updateThresholdEdit(dg.id, 'threshold', Number((e.target as HTMLInputElement).value), dg)"
                />
                <span class="w-2 h-2 rounded-full bg-red-400 shrink-0" />
              </div>
              <p class="text-[11px] text-muted-foreground mt-1">Begin cleanup when usage exceeds this %</p>
            </div>
            <div>
              <label class="block text-xs font-medium text-muted-foreground mb-1.5">
                Cleanup Target %
              </label>
              <div class="flex items-center gap-2">
                <input
                  :value="thresholdEdits[dg.id]?.target ?? dg.targetPct"
                  type="number"
                  min="1"
                  max="99"
                  class="w-full px-3 py-2 text-sm rounded-lg border border-input bg-card text-foreground focus:ring-2 focus:ring-emerald-400/50 focus:border-emerald-400 outline-none transition-colors"
                  @input="(e: Event) => updateThresholdEdit(dg.id, 'target', Number((e.target as HTMLInputElement).value), dg)"
                />
                <span class="w-2 h-2 rounded-full bg-emerald-500 shrink-0" />
              </div>
              <p class="text-[11px] text-muted-foreground mt-1">Stop cleanup when usage drops to this %</p>
            </div>
          </div>

          <!-- Validation error -->
          <p v-if="thresholdValidation(dg.id, dg)" class="text-xs text-red-500">
            {{ thresholdValidation(dg.id, dg) }}
          </p>

          <!-- Per-group feedback -->
          <p v-if="thresholdEdits[dg.id]?.message" class="text-xs" :class="thresholdEdits[dg.id]?.success ? 'text-emerald-500' : 'text-red-500'">
            {{ thresholdEdits[dg.id]?.message }}
          </p>

          <!-- Save button — prominent, full-width -->
          <button
            :disabled="!!thresholdValidation(dg.id, dg) || thresholdEdits[dg.id]?.saving"
            class="w-full py-2.5 rounded-lg bg-primary hover:bg-primary/90 text-white text-sm font-medium shadow-sm transition-colors disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2"
            @click="saveThresholds(dg)"
          >
            <component :is="thresholdEdits[dg.id]?.saving ? LoaderCircleIcon : SaveIcon" :class="{ 'animate-spin': thresholdEdits[dg.id]?.saving }" class="w-4 h-4" />
            {{ thresholdEdits[dg.id]?.saving ? 'Saving...' : 'Save Thresholds' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Preference Weights -->
    <div
      v-motion
      :initial="{ opacity: 0, y: 12 }"
      :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24 } }"
      class="rounded-xl border border-border bg-card shadow-sm p-6 mb-6"
    >
      <div class="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-6">
        <div>
          <h3 class="text-lg font-semibold">Preference Weights</h3>
          <p class="text-sm text-muted-foreground">
            Higher weights increase the attribute's influence on deletion score.
          </p>
        </div>
        <button
          class="h-8 px-4 rounded-lg bg-primary hover:bg-primary/90 text-white text-xs font-medium shadow-sm transition-colors"
          @click="savePreferences"
        >
          Save Weights
        </button>
      </div>

      <!-- Preset Chips -->
      <div class="flex flex-wrap gap-2 mb-6">
        <button
          v-for="preset in presets"
          :key="preset.name"
          class="h-7 px-3 rounded-full text-xs font-medium transition-all border"
          :class="isActivePreset(preset.values)
            ? 'bg-primary border-primary text-primary-foreground shadow-sm'
            : 'bg-muted border-input text-foreground hover:border-primary hover:text-primary'"
          @click="applyPreset(preset.values)"
        >
          {{ preset.name }}
        </button>
      </div>

      <!-- Two-Column Slider Grid -->
      <div class="grid grid-cols-1 md:grid-cols-2 gap-x-8 gap-y-5">
        <div v-for="slider in sliders" :key="slider.key" class="space-y-1.5">
          <div class="flex justify-between text-sm">
            <span class="font-medium text-foreground">{{ slider.label }}</span>
            <span class="text-muted-foreground font-mono tabular-nums">{{ prefs[slider.key as keyof typeof prefs] }} / 10</span>
          </div>
          <input
            type="range"
            :value="prefs[slider.key as keyof typeof prefs]"
            min="0"
            max="10"
            class="w-full h-2 bg-muted rounded-full appearance-none cursor-pointer accent-primary"
            @input="(e: Event) => { prefs[slider.key as keyof typeof prefs] = Number((e.target as HTMLInputElement).value) }"
          />
          <p class="text-xs text-muted-foreground">{{ slider.description }}</p>
        </div>
      </div>

      <!-- Execution Mode -->
      <div class="mt-8 pt-6 border-t border-border">
        <h4 class="text-sm font-semibold mb-3">Execution Mode</h4>
        <div class="flex gap-3">
          <button
            v-for="mode in executionModes"
            :key="mode.value"
            data-slot="execution-mode-card"
            :data-active="prefs.executionMode === mode.value"
            class="flex-1 px-4 py-3 rounded-xl border-2 text-left transition-all"
            :class="prefs.executionMode === mode.value
              ? 'border-primary bg-primary/5 shadow-sm ring-1 ring-primary/20'
              : 'border-input hover:border-border'"
            @click="prefs.executionMode = mode.value; savePreferences()"
          >
            <div class="text-sm font-medium" :class="prefs.executionMode === mode.value ? 'text-primary' : ''">
              {{ mode.label }}
            </div>
            <div class="text-xs text-muted-foreground mt-0.5">{{ mode.description }}</div>
          </button>
        </div>
      </div>

      <!-- Tiebreaker -->
      <div class="mt-6 pt-6 border-t border-border">
        <h4 class="text-sm font-semibold mb-1">Score Tiebreaker</h4>
        <p class="text-xs text-muted-foreground mb-3">When items have the same score, how should they be ordered?</p>
        <select
          v-model="prefs.tiebreakerMethod"
          class="h-9 w-full max-w-xs px-3 rounded-lg border border-input bg-card text-sm text-foreground focus:ring-2 focus-visible:ring-ring/50 focus:border-primary outline-none transition-colors"
          @change="savePreferences"
        >
          <option value="size_desc">Largest first (free more space)</option>
          <option value="size_asc">Smallest first</option>
          <option value="name_asc">Alphabetical (A → Z)</option>
          <option value="oldest_first">Oldest in library first</option>
          <option value="newest_first">Newest in library first</option>
        </select>
      </div>
    </div>

    <!-- Custom Rules -->
    <div
      v-motion
      :initial="{ opacity: 0, y: 12 }"
      :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 100 } }"
      class="rounded-xl border border-border bg-card shadow-sm p-6 mb-6"
    >
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-lg font-semibold">Custom Rules</h3>
        <button
          class="h-8 px-3 rounded-lg bg-primary hover:bg-primary/90 text-white text-xs font-medium shadow-sm transition-colors inline-flex items-center gap-1.5"
          @click="showAddRule = !showAddRule"
        >
          <component :is="PlusIcon" class="w-3.5 h-3.5" />
          Add Rule
        </button>
      </div>

      <!-- Add Rule Form -->
      <div v-if="showAddRule" class="mb-4 p-4 rounded-lg border border-input bg-muted space-y-3">
        <div class="grid grid-cols-2 md:grid-cols-5 gap-3">
          <select v-model="newRule.type" class="h-9 px-3 rounded-lg border border-input bg-input text-sm">
            <option value="protect">Protect</option>
            <option value="target">Target</option>
          </select>
          <select v-model="newRule.field" class="h-9 px-3 rounded-lg border border-input bg-input text-sm">
            <option v-for="f in ruleFields" :key="f.field" :value="f.field">{{ f.label }}</option>
          </select>
          <select v-model="newRule.operator" class="h-9 px-3 rounded-lg border border-input bg-input text-sm">
            <option v-for="op in selectedFieldOperators" :key="op" :value="op">{{ op }}</option>
          </select>
          <input v-model="newRule.value" placeholder="Value" class="h-9 px-3 rounded-lg border border-input bg-input text-sm focus:outline-none focus:ring-2 focus-visible:ring-ring/50" />
          <select v-model="newRule.intensity" class="h-9 px-3 rounded-lg border border-input bg-input text-sm">
            <option value="slight">Slight</option>
            <option value="strong">Strong</option>
            <option value="absolute">Absolute</option>
          </select>
        </div>
        <button
          class="h-8 px-4 rounded-lg bg-primary hover:bg-primary/90 text-white text-xs font-medium transition-colors"
          @click="addRule"
        >
          Save Rule
        </button>
      </div>

      <!-- Rules List -->
      <div v-if="rules.length === 0 && !showAddRule" class="text-center py-6 text-muted-foreground text-sm">
        No rules configured. Media will be ranked purely by preference weights.
      </div>
      <div v-else class="space-y-2">
        <div
          v-for="rule in rules"
          :key="rule.id"
          class="flex items-center justify-between px-4 py-2.5 rounded-lg border border-border bg-muted/50"
        >
          <div class="flex items-center gap-2 text-sm">
            <span
              class="inline-flex items-center px-2 py-0.5 rounded text-xs font-medium"
              :class="rule.type === 'protect'
                ? 'bg-emerald-100 dark:bg-emerald-500/15 text-emerald-600 dark:text-emerald-400'
                : 'bg-red-100 dark:bg-red-500/15 text-red-600 dark:text-red-400'"
            >
              {{ rule.type }}
            </span>
            <span class="text-foreground">{{ rule.field }}</span>
            <span class="text-muted-foreground">{{ rule.operator }}</span>
            <span class="font-medium">{{ rule.value }}</span>
            <span class="text-muted-foreground text-xs">({{ rule.intensity }})</span>
          </div>
          <button
            class="text-muted-foreground hover:text-red-500 transition-colors"
            @click="deleteRule(rule.id)"
          >
            <component :is="XIcon" class="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>

    <!-- Live Preview -->
    <div
      v-motion
      :initial="{ opacity: 0, y: 12 }"
      :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 200 } }"
      class="rounded-xl border border-border bg-card shadow-sm p-6"
    >
      <div class="flex items-center justify-between mb-4">
        <h3 class="text-lg font-semibold">Live Preview — What Would Be Deleted</h3>
        <button
          class="h-8 px-3 rounded-lg border border-input text-xs font-medium hover:bg-accent transition-colors inline-flex items-center gap-1.5"
          @click="fetchPreview"
        >
          <component :is="previewLoading ? LoaderCircleIcon : RefreshCwIcon" :class="{ 'animate-spin': previewLoading }" class="w-3.5 h-3.5" />
          Refresh
        </button>
      </div>

      <div v-if="previewLoading" class="flex items-center justify-center py-12">
        <component :is="LoaderCircleIcon" class="w-6 h-6 text-primary animate-spin" />
      </div>

      <div v-else-if="preview.length === 0" class="text-center py-8 text-muted-foreground text-sm">
        No items to evaluate. Connect integrations and ensure media exists.
      </div>

      <div v-else class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="border-b border-border">
              <th class="w-8 px-3 py-2"></th>
              <th class="text-left px-4 py-2 text-xs font-medium uppercase text-muted-foreground">Score</th>
              <th class="text-left px-4 py-2 text-xs font-medium uppercase text-muted-foreground">Title</th>
              <th class="text-left px-4 py-2 text-xs font-medium uppercase text-muted-foreground">Type</th>
              <th class="text-right px-4 py-2 text-xs font-medium uppercase text-muted-foreground">Size</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="group in groupedPreview" :key="group.key">
              <tr class="border-b border-border hover:bg-accent/30 transition-colors cursor-pointer" @click="selectPreviewItem(group.entry); group.seasons.length > 0 && togglePreviewGroup(group.key)">
                <td class="px-3 py-2 w-8">
                  <button v-if="group.seasons.length > 0" class="text-muted-foreground hover:text-foreground transition-colors" @click.stop="togglePreviewGroup(group.key)">
                    <ChevronRightIcon class="w-4 h-4 transition-transform duration-200" :class="{ 'rotate-90': expandedPreviewGroups.has(group.key) }" />
                  </button>
                </td>
                <td class="px-4 py-2">
                  <div class="flex items-center gap-2">
                    <div class="w-16 h-1.5 rounded-full bg-muted overflow-hidden">
                      <div
                        class="h-full rounded-full"
                        :class="group.entry.isProtected ? 'bg-emerald-500' : scoreColor(group.entry.score)"
                        :style="{ width: (group.entry.isProtected ? 0 : group.entry.score * 100) + '%' }"
                      />
                    </div>
                    <span class="text-xs font-mono tabular-nums" :class="group.entry.isProtected ? 'text-emerald-500' : ''">
                      {{ group.entry.isProtected ? 'Protected' : group.entry.score.toFixed(2) }}
                    </span>
                  </div>
                </td>
                <td class="px-4 py-2 font-medium">
                  {{ group.entry.item.title }}
                  <span v-if="group.seasons.length > 0" class="ml-1.5 text-xs text-muted-foreground font-normal">({{ group.seasons.length }} season{{ group.seasons.length !== 1 ? 's' : '' }})</span>
                </td>
                <td class="px-4 py-2">
                  <span class="text-xs px-2 py-0.5 rounded bg-muted capitalize">{{ group.entry.item.type }}</span>
                </td>
                <td class="px-4 py-2 text-right font-mono text-xs tabular-nums">{{ formatBytes(group.entry.item.sizeBytes) }}</td>
              </tr>
              <template v-if="expandedPreviewGroups.has(group.key)">
                <tr v-for="(season, sIdx) in group.seasons" :key="`${group.key}-s${sIdx}`" class="border-b border-border bg-muted/30 transition-colors cursor-pointer" @click.stop="selectPreviewItem(season)">
                  <td class="px-3 py-2 w-8"></td>
                  <td class="px-4 py-2">
                    <div class="flex items-center gap-2">
                      <div class="w-16 h-1.5 rounded-full bg-muted overflow-hidden">
                        <div
                          class="h-full rounded-full"
                          :class="season.isProtected ? 'bg-emerald-500' : scoreColor(season.score)"
                          :style="{ width: (season.isProtected ? 0 : season.score * 100) + '%' }"
                        />
                      </div>
                      <span class="text-xs font-mono tabular-nums" :class="season.isProtected ? 'text-emerald-500' : ''">
                        {{ season.isProtected ? 'Protected' : season.score.toFixed(2) }}
                      </span>
                    </div>
                  </td>
                  <td class="px-4 py-2 text-muted-foreground pl-8">
                    <span class="inline-flex items-center gap-1.5">
                      <span class="w-3 h-px bg-zinc-300 dark:bg-zinc-600 inline-block"></span>
                      {{ extractPreviewSeasonLabel(season.item.title) }}
                    </span>
                  </td>
                  <td class="px-4 py-2">
                    <span class="text-xs px-2 py-0.5 rounded bg-muted text-muted-foreground dark:text-muted-foreground capitalize">{{ season.item.type }}</span>
                  </td>
                    <td class="px-4 py-2 text-right font-mono text-xs tabular-nums text-muted-foreground">{{ formatBytes(season.item.sizeBytes) }}</td>
                </tr>
              </template>
            </template>
          </tbody>
        </table>
      </div>
    </div>

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
import { PlusIcon, XIcon, RefreshCwIcon, LoaderCircleIcon, SaveIcon, ChevronRightIcon, HardDriveIcon } from 'lucide-vue-next'
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

function updateThresholdEdit(dgId: number, field: 'threshold' | 'target', value: number, dg: any) {
  ensureThresholdEdit(dgId, dg)
  thresholdEdits[dgId][field] = value
  thresholdEdits[dgId].message = ''
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
  const edit = thresholdEdits[dg.id]
  if (thresholdValidation(dg.id, dg)) return

  edit.saving = true
  edit.message = ''

  try {
    const updated = await api(`/api/v1/disk-groups/${dg.id}`, {
      method: 'PUT',
      body: {
        thresholdPct: edit.threshold,
        targetPct: edit.target,
      },
    }) as any

    edit.success = true
    edit.message = 'Thresholds updated successfully'
    addToast('Thresholds saved', 'success')

    // Update local diskGroups array with canonical values from the API response
    const idx = diskGroups.value.findIndex((g: any) => g.id === dg.id)
    if (idx !== -1 && updated) {
      diskGroups.value[idx] = { ...diskGroups.value[idx], ...updated }
    } else if (idx !== -1) {
      diskGroups.value[idx].thresholdPct = edit.threshold
      diskGroups.value[idx].targetPct = edit.target
    }

    setTimeout(() => { edit.message = '' }, 3000)
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

const executionModes = [
  { value: 'dry-run', label: 'Dry Run', description: 'Log only, no deletions' },
  { value: 'approval', label: 'Approval', description: 'Queue for manual approval' },
  { value: 'auto', label: 'Automatic', description: 'Delete automatically' }
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

// Custom Rules
const rules = ref<any[]>([])
const showAddRule = ref(false)
const newRule = reactive({
  type: 'protect',
  field: 'quality',
  operator: '==',
  value: '',
  intensity: 'absolute'
})

// Dynamic rule fields fetched from API based on configured integrations
const ruleFields = ref<Array<{ field: string; label: string; type: string; operators: string[] }>>([])
const selectedFieldOperators = computed(() => {
  const selected = ruleFields.value.find(f => f.field === newRule.field)
  return selected?.operators ?? ['==', '!=', 'contains', '>', '<', '>=', '<=']
})

async function fetchRuleFields() {
  try {
    ruleFields.value = await api('/api/v1/rule-fields') as any[]
  } catch {
    // Fallback to base fields if API fails
    ruleFields.value = [
      { field: 'title', label: 'Title', type: 'string', operators: ['==', '!=', 'contains'] },
      { field: 'quality', label: 'Quality Profile', type: 'string', operators: ['==', '!=', 'contains'] },
      { field: 'tag', label: 'Tag', type: 'string', operators: ['==', '!=', 'contains'] },
      { field: 'genre', label: 'Genre', type: 'string', operators: ['==', '!=', 'contains'] },
      { field: 'rating', label: 'Rating', type: 'number', operators: ['==', '!=', '>', '>=', '<', '<='] },
      { field: 'monitored', label: 'Monitored', type: 'boolean', operators: ['=='] },
    ]
  }
}

// Preview
const preview = ref<any[]>([])
const previewLoading = ref(false)
const previewFetchedAt = ref<string>('')
const selectedPreviewItem = ref<any | null>(null)

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
  await Promise.all([fetchPreferences(), fetchRules(), fetchPreview(), fetchDiskGroups(), fetchRuleFields()])
})

async function fetchDiskGroups() {
  try {
    diskGroups.value = await api('/api/v1/disk-groups') as any[]
  } catch (e) {
    console.error('Failed to fetch disk groups', e)
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

async function addRule() {
  try {
    await api('/api/v1/protections', { method: 'POST', body: { ...newRule } })
    newRule.value = ''
    showAddRule.value = false
    addToast('Protection rule added', 'success')
    await fetchRules()
    await fetchPreview()
  } catch (e) {
    console.error('Failed to add rule', e)
    addToast('Failed to add protection rule', 'error')
  }
}

async function deleteRule(id: number) {
  try {
    await api(`/api/v1/protections/${id}`, { method: 'DELETE' })
    addToast('Protection rule removed', 'success')
    await fetchRules()
    await fetchPreview()
  } catch (e) {
    console.error('Failed to delete rule', e)
    addToast('Failed to delete protection rule', 'error')
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
  if (score >= 0.7) return 'bg-red-500'
  if (score >= 0.4) return 'bg-amber-500'
  return 'bg-emerald-500'
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
  const showMap = new Map<string, number>()

  for (const item of items) {
    // Detect season items: type === 'season' and title matches "ShowName - Season N"
    if (item.item?.type === 'season' && item.item?.title?.includes(' - Season ')) {
      const showName = item.item.title.split(' - Season ')[0]
      const groupIdx = showMap.get(showName)
      if (groupIdx !== undefined) {
        groups[groupIdx].seasons.push(item)
        continue
      }
    }

    const key = `preview-${item.item?.title}-${item.item?.type}`
    if (item.item?.type === 'show') {
      showMap.set(item.item.title, groups.length)
    }
    groups.push({ key, entry: item, seasons: [] })
  }

  return groups
})

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
