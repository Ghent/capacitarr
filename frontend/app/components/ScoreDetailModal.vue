<template>
  <UiDialog
    :open="visible"
    @update:open="(val: boolean) => { if (!val) $emit('close') }"
  >
    <UiDialogContent class="max-w-lg">
      <!-- Header -->
      <UiDialogHeader>
        <span class="text-[10px] font-medium uppercase tracking-widest text-muted-foreground">Score Detail</span>
        <UiDialogTitle
          class="truncate"
          :title="mediaName"
        >
          {{ mediaName }}
        </UiDialogTitle>
        <div class="flex items-center gap-2 mt-1">
          <UiBadge
            variant="secondary"
            class="capitalize"
          >
            {{ mediaType }}
          </UiBadge>
          <span
            class="text-3xl font-bold tabular-nums tracking-tight"
            :class="scoreColorClass"
          >
            {{ score.toFixed(2) }}
          </span>
        </div>
      </UiDialogHeader>

      <!-- Body -->
      <div class="max-h-[60vh] overflow-y-auto space-y-5">
        <!-- Stacked Bar (reuse ScoreBreakdown in lg mode) -->
        <div v-if="scoreDetails">
          <ScoreBreakdown
            :reason="reasonFromScore"
            :score-details="scoreDetails"
            size="lg"
          />
        </div>

        <!-- Factor Table -->
        <div v-if="weightFactors.length > 0">
          <h3 class="text-xs font-medium uppercase tracking-wider text-muted-foreground mb-2">
            Score Factors
          </h3>
          <div class="rounded-lg border border-border/50 overflow-hidden bg-card/80">
            <UiTable>
              <UiTableHeader>
                <UiTableRow class="bg-primary/5 dark:bg-primary/10">
                  <UiTableHead class="text-xs">
                    Factor
                  </UiTableHead>
                  <UiTableHead class="text-xs text-right">
                    Raw
                  </UiTableHead>
                  <UiTableHead class="text-xs text-right">
                    Weight
                  </UiTableHead>
                  <UiTableHead class="text-xs text-right">
                    Contribution
                  </UiTableHead>
                </UiTableRow>
              </UiTableHeader>
              <UiTableBody>
                <UiTableRow
                  v-for="f in weightFactors"
                  :key="f.name"
                >
                  <UiTableCell class="font-medium">
                    <span class="inline-flex items-center gap-1.5">
                      <span
                        class="w-2 h-2 rounded-full flex-shrink-0"
                        :style="{ backgroundColor: factorColor(f.name) }"
                      />
                      {{ f.name }}
                    </span>
                  </UiTableCell>
                  <UiTableCell class="text-right font-mono tabular-nums text-muted-foreground">
                    {{ f.rawScore.toFixed(2) }}
                  </UiTableCell>
                  <UiTableCell class="text-right font-mono tabular-nums text-muted-foreground">
                    {{ f.weight }}
                  </UiTableCell>
                  <UiTableCell class="text-right font-mono tabular-nums font-semibold">
                    {{ f.contribution.toFixed(3) }}
                  </UiTableCell>
                </UiTableRow>
              </UiTableBody>
            </UiTable>
          </div>
        </div>

        <!-- Custom Rules Section -->
        <div v-if="ruleFactors.length > 0">
          <h3 class="text-xs font-medium uppercase tracking-wider text-muted-foreground mb-2">
            Custom Rules
          </h3>
          <div class="flex flex-wrap gap-1.5">
            <UiBadge
              v-for="f in ruleFactors"
              :key="f.name"
              :variant="f.name.includes('Protect') ? 'outline' : 'destructive'"
            >
              {{ f.name }}
            </UiBadge>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <UiDialogFooter class="flex-row items-center justify-between border-t border-primary/10 dark:border-primary/15 pt-3">
        <div class="flex items-center gap-3">
          <span class="text-sm text-muted-foreground font-mono tabular-nums">
            {{ formatBytes(sizeBytes) }}
          </span>
          <UiBadge
            v-if="action"
            :variant="actionBadgeVariant"
          >
            {{ action }}
          </UiBadge>
        </div>
        <span
          v-if="createdAt"
          class="inline-flex items-center gap-1 text-xs text-muted-foreground"
        >
          <ClockIcon class="w-3 h-3" />
          {{ formatTime(createdAt) }}
        </span>
      </UiDialogFooter>
    </UiDialogContent>
  </UiDialog>
</template>

<script setup lang="ts">
import { ClockIcon } from 'lucide-vue-next'
import { formatBytes, formatTime } from '~/utils/format'

interface ScoreFactor {
  name: string
  rawScore: number
  weight: number
  contribution: number
  type: string
}

interface Props {
  visible: boolean
  mediaName: string
  mediaType: string
  score: number
  scoreDetails: string
  sizeBytes: number
  action: string
  createdAt?: string
}

const props = defineProps<Props>()
defineEmits<{ close: [] }>()

const FACTOR_COLORS: Record<string, string> = {
  'Watch History': '#8b5cf6',
  'Last Watched': '#3b82f6',
  'File Size': '#f59e0b',
  'Rating': '#10b981',
  'Time in Library': '#f97316',
  'Series Status': '#ec4899'
}

function factorColor(name: string): string {
  return FACTOR_COLORS[name] || '#6b7280'
}

// Parse factors
const parsedFactors = computed<ScoreFactor[]>(() => {
  if (!props.scoreDetails) return []
  try {
    const parsed = JSON.parse(props.scoreDetails)
    if (Array.isArray(parsed)) return parsed as ScoreFactor[]
  } catch {
    // ignore
  }
  return []
})

const weightFactors = computed(() => parsedFactors.value.filter(f => f.type === 'weight'))
const ruleFactors = computed(() => parsedFactors.value.filter(f => f.type === 'rule'))

// Build a reason string from score for ScoreBreakdown
const reasonFromScore = computed(() => `Score: ${props.score.toFixed(2)}`)

// Score color class
const scoreColorClass = computed(() => {
  if (props.score >= 0.7) return 'text-destructive'
  if (props.score >= 0.4) return 'text-warning'
  return 'text-success'
})

// Action badge variant
const actionBadgeVariant = computed<'destructive' | 'outline' | 'secondary' | 'default'>(() => {
  if (props.action === 'Deleted') return 'destructive'
  if (props.action === 'Queued for Approval') return 'outline'
  if (props.action === 'Queued for Deletion') return 'outline'
  return 'default'
})
</script>
