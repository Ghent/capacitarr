<template>
  <div>
    <!-- Header -->
    <div class="mb-8">
      <h1 class="text-3xl font-bold tracking-tight">
        Help
      </h1>
      <p class="text-muted-foreground mt-1.5">
        Learn how Capacitarr works and how to configure it effectively.
      </p>
    </div>

    <div class="space-y-4">
      <!-- How Scoring Works -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 80 } }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors">
          <ChevronRightIcon class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90" />
          <h3 class="font-semibold text-primary">
            How Scoring Works
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-3">
          <p>
            Capacitarr scores each media item from <strong class="text-foreground">0 to 1</strong> based on weighted factors.
            Higher scores mean the item is a better candidate for removal.
          </p>
          <p>
            When disk usage exceeds your threshold, items are evaluated and the highest-scoring ones
            are removed first — freeing space efficiently while preserving the content you care about most.
          </p>
        </div>
      </details>

      <!-- Understanding the Sliders -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 140 } }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors">
          <ChevronRightIcon class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90" />
          <h3 class="font-semibold text-primary">
            Understanding the Sliders
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-3">
          <p>
            Each factor has a weight from <strong class="text-foreground">0–10</strong>. Higher weight = more influence on the final score.
            The six factors are:
          </p>
          <ul class="space-y-2 pl-1">
            <li
              v-for="factor in scoringFactors"
              :key="factor.name"
              class="flex items-start gap-2"
            >
              <span class="mt-1 w-1.5 h-1.5 rounded-full bg-primary shrink-0" />
              <span><strong class="text-foreground">{{ factor.name }}</strong> — {{ factor.desc }}</span>
            </li>
          </ul>
        </div>
      </details>

      <!-- Reading a Score Detail -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 200 } }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors">
          <ChevronRightIcon class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90" />
          <h3 class="font-semibold text-primary">
            Reading a Score Detail
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-4">
          <p>
            Click any row in the Audit History to open its <strong class="text-foreground">Score Detail</strong> modal. It breaks down exactly
            how the final score was calculated using three columns: <strong class="text-foreground">Raw</strong>, <strong class="text-foreground">Weight</strong>, and <strong class="text-foreground">Contribution</strong>.
          </p>

          <div>
            <p class="font-medium text-foreground mb-1">
              Raw Score (0.0 – 1.0)
            </p>
            <p>
              Represents how strongly this factor suggests the media should be cleaned up.
              <strong class="text-foreground">1.0</strong> = maximum cleanup signal; <strong class="text-foreground">0.0</strong> = minimum cleanup signal.
            </p>
            <ul class="space-y-1 pl-4 list-disc mt-2">
              <li><strong class="text-foreground">Watch History</strong> — 1.0 if never watched by any user, 0.0 if watched by all users</li>
              <li><strong class="text-foreground">Last Watched</strong> — 1.0 if watched very long ago or never, lower if watched recently</li>
              <li><strong class="text-foreground">File Size</strong> — 1.0 for the largest files in your library, scaled relative to the largest item</li>
              <li><strong class="text-foreground">Rating</strong> — 1.0 for lowest-rated content (rating 10/10 → raw 0.0, rating 1/10 → raw 0.9)</li>
              <li><strong class="text-foreground">Time in Library</strong> — 1.0 for content that has been in the library the longest</li>
              <li><strong class="text-foreground">Availability</strong> — 1.0 if available on many streaming services, 0.0 if not available elsewhere</li>
            </ul>
          </div>

          <div>
            <p class="font-medium text-foreground mb-1">
              Weight (0 – 10)
            </p>
            <p>
              Set by you on the <strong class="text-foreground">Scoring Engine</strong> page. Higher weight = more influence on the final score.
              Each factor's contribution = <code class="px-1 py-0.5 rounded bg-muted text-xs">(rawScore × weight) / totalWeightSum</code>.
            </p>
            <p class="mt-1">
              <strong class="text-foreground">Example:</strong> If Watch History has weight 7 and raw score 1.0, and total weights sum to 30,
              its contribution = 7 / 30 = 0.23.
            </p>
          </div>

          <div>
            <p class="font-medium text-foreground mb-1">
              Contribution
            </p>
            <p>
              The actual portion of the final score this factor is responsible for. All contributions sum to the total score.
              These are shown as the colored segments in the stacked bar at the top of the modal.
            </p>
          </div>
        </div>
      </details>

      <!-- Threshold & Target -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 260 } }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors">
          <ChevronRightIcon class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90" />
          <h3 class="font-semibold text-primary">
            Threshold &amp; Target
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-3">
          <p>
            The <strong class="text-foreground">threshold</strong> is the disk usage percentage that triggers cleanup.
            The <strong class="text-foreground">target</strong> is where cleanup stops.
          </p>
          <p>
            <strong class="text-foreground">Example:</strong> threshold 85%, target 75% means cleanup starts at 85% full and
            continues removing items until usage drops to 75%.
          </p>
        </div>
      </details>

      <!-- Custom Rules -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 320 } }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors">
          <ChevronRightIcon class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90" />
          <h3 class="font-semibold text-primary">
            Custom Rules
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-3">
          <p>
            Rules let you protect or target specific content. <strong class="text-foreground">Protect</strong> rules lower an item's score
            (less likely to be removed). <strong class="text-foreground">Target</strong> rules raise it (more likely to be removed).
          </p>
          <p>Intensity levels:</p>
          <ul class="space-y-1 pl-4 list-disc">
            <li><strong class="text-foreground">Slight</strong> — Small adjustment to the score</li>
            <li><strong class="text-foreground">Strong</strong> — Significant adjustment</li>
            <li><strong class="text-foreground">Absolute</strong> — Completely prevents or forces removal</li>
          </ul>
        </div>
      </details>

      <!-- Tiebreaker -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 350 } }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors">
          <ChevronRightIcon class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90" />
          <h3 class="font-semibold text-primary">
            Score Tiebreaker
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-3">
          <p>
            When two or more items have the <strong class="text-foreground">same score</strong>, the tiebreaker determines which is deleted first.
            Configure this on the <strong class="text-foreground">Scoring Engine</strong> page under Preference Weights.
          </p>
          <ul class="space-y-1 pl-4 list-disc">
            <li><strong class="text-foreground">Largest first</strong> — Prefer deleting bigger files to free more space (default)</li>
            <li><strong class="text-foreground">Smallest first</strong> — Prefer deleting smaller files</li>
            <li><strong class="text-foreground">Alphabetical</strong> — Sort tied items A → Z by title</li>
            <li><strong class="text-foreground">Oldest in library</strong> — Items added to the library longest ago are deleted first</li>
            <li><strong class="text-foreground">Newest in library</strong> — Most recently added items are deleted first</li>
          </ul>
        </div>
      </details>

      <!-- Reading the Audit Log -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 380 } }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors">
          <ChevronRightIcon class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90" />
          <h3 class="font-semibold text-primary">
            Reading the Audit Log
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-3">
          <p>
            The audit log shows every item the engine evaluated. Each entry shows the score breakdown —
            hover over the colored bar to see individual factor contributions.
          </p>
          <p>Actions:</p>
          <ul class="space-y-1 pl-4 list-disc">
            <li><strong class="text-foreground">Dry-Run</strong> — Simulated only; no files were deleted</li>
            <li><strong class="text-foreground">Queued for Approval</strong> — Flagged for manual review before deletion</li>
            <li><strong class="text-foreground">Deleted</strong> — Actually removed from disk</li>
          </ul>
        </div>
      </details>

      <!-- Execution Modes -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{ opacity: 1, y: 0, transition: { type: 'spring', stiffness: 260, damping: 24, delay: 440 } }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors">
          <ChevronRightIcon class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90" />
          <h3 class="font-semibold text-primary">
            Execution Modes
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-3">
          <ul class="space-y-2 pl-1">
            <li class="flex items-start gap-2">
              <span class="mt-1 w-1.5 h-1.5 rounded-full bg-primary shrink-0" />
              <span><strong class="text-foreground">Dry-Run</strong> — No files are deleted; the engine only logs what it would do. Safe for testing and tuning your weights.</span>
            </li>
            <li class="flex items-start gap-2">
              <span class="mt-1 w-1.5 h-1.5 rounded-full bg-warning shrink-0" />
              <span><strong class="text-foreground">Approval</strong> — Items are flagged for manual approval before deletion. You review and confirm each removal.</span>
            </li>
            <li class="flex items-start gap-2">
              <span class="mt-1 w-1.5 h-1.5 rounded-full bg-destructive shrink-0" />
              <span><strong class="text-foreground">Auto</strong> — Items are automatically deleted when thresholds are breached. Use with caution.</span>
            </li>
          </ul>
        </div>
      </details>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ChevronRightIcon } from 'lucide-vue-next'

const scoringFactors = [
  { name: 'Watch History', desc: 'Has anyone watched this? Unwatched content scores higher (more likely to be removed).' },
  { name: 'Last Watched', desc: 'How recently was it watched? Content watched long ago scores higher.' },
  { name: 'File Size', desc: 'Larger files score higher, freeing more space per deletion.' },
  { name: 'Rating', desc: 'Lower-rated content scores higher.' },
  { name: 'Time in Library', desc: 'Older library items score higher.' },
  { name: 'Availability', desc: 'Content available on fewer platforms scores higher.' }
]
</script>
