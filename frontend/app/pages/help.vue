<template>
  <div>
    <!-- Header -->
    <div class="mb-8">
      <h1 class="text-3xl font-bold tracking-tight">
        {{ $t('help.title') }}
      </h1>
      <p class="text-muted-foreground mt-1.5">
        {{ $t('help.subtitle') }}
      </p>
    </div>

    <div class="space-y-4">
      <!-- How Scoring Works -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: { type: 'spring', stiffness: 260, damping: 24, delay: 80 },
        }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary
          class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors"
        >
          <ChevronRightIcon
            class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90"
          />
          <h3 class="font-semibold text-primary">
            {{ $t('help.howScoringWorks') }}
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-3">
          <p>
            {{ $t('help.howScoringWorks.intro') }}
          </p>
          <p>
            {{ $t('help.howScoringWorks.behavior') }}
          </p>
        </div>
      </details>

      <!-- Understanding the Sliders -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: { type: 'spring', stiffness: 260, damping: 24, delay: 140 },
        }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary
          class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors"
        >
          <ChevronRightIcon
            class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90"
          />
          <h3 class="font-semibold text-primary">
            {{ $t('help.understandingSliders') }}
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-3">
          <p>
            {{ $t('help.understandingSliders.intro') }}
          </p>
          <ul class="space-y-2 pl-1">
            <li
              v-for="factor in scoringFactors"
              :key="factor.nameKey"
              class="flex items-start gap-2"
            >
              <span class="mt-1 w-1.5 h-1.5 rounded-full bg-primary shrink-0" />
              <span
                ><strong class="text-foreground">{{ $t(factor.nameKey) }}</strong> —
                {{ $t(factor.descKey) }}</span
              >
            </li>
          </ul>
        </div>
      </details>

      <!-- Data Sources -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: { type: 'spring', stiffness: 260, damping: 24, delay: 200 },
        }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary
          class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors"
        >
          <ChevronRightIcon
            class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90"
          />
          <h3 class="font-semibold text-primary">
            {{ $t('help.dataSources') }}
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-4">
          <p>
            {{ $t('help.dataSources.intro') }}
          </p>

          <div>
            <p class="font-medium text-foreground mb-1">
              {{ $t('help.dataSources.ratingsTitle') }}
            </p>
            <p class="mb-2">
              {{ $t('help.dataSources.ratingsDesc') }}
            </p>
            <ul class="space-y-2 pl-1">
              <li
                v-for="source in ratingSources"
                :key="source.nameKey"
                class="flex items-start gap-2"
              >
                <span class="mt-1 w-1.5 h-1.5 rounded-full bg-primary shrink-0" />
                <span
                  ><strong class="text-foreground">{{ $t(source.nameKey) }}</strong> —
                  {{ $t(source.descKey) }}</span
                >
              </li>
            </ul>
          </div>

          <div>
            <p class="font-medium text-foreground mb-1">
              {{ $t('help.dataSources.enrichmentTitle') }}
            </p>
            <p class="mb-2">
              {{ $t('help.dataSources.enrichmentDesc') }}
            </p>
            <ul class="space-y-2 pl-1">
              <li v-for="item in enrichmentSources" :key="item.key" class="flex items-start gap-2">
                <span class="mt-1 w-1.5 h-1.5 rounded-full bg-primary shrink-0" />
                <span>{{ $t(item.key) }}</span>
              </li>
            </ul>
          </div>
        </div>
      </details>

      <!-- Reading a Score Detail -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: { type: 'spring', stiffness: 260, damping: 24, delay: 260 },
        }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary
          class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors"
        >
          <ChevronRightIcon
            class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90"
          />
          <h3 class="font-semibold text-primary">
            {{ $t('help.readingScoreDetail') }}
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-4">
          <p>
            {{ $t('help.readingScoreDetail.intro') }}
          </p>

          <div>
            <p class="font-medium text-foreground mb-1">
              {{ $t('help.readingScoreDetail.rawTitle') }}
            </p>
            <p>
              {{ $t('help.readingScoreDetail.rawDesc') }}
            </p>
            <ul class="space-y-1 pl-4 list-disc mt-2">
              <li v-for="factor in rawScoreExamples" :key="factor.nameKey">
                <strong class="text-foreground">{{ $t(factor.nameKey) }}</strong> —
                {{ $t(factor.descKey) }}
              </li>
            </ul>
          </div>

          <div>
            <p class="font-medium text-foreground mb-1">
              {{ $t('help.readingScoreDetail.weightTitle') }}
            </p>
            <p>
              {{ $t('help.readingScoreDetail.weightDesc') }}
            </p>
            <p class="mt-1">
              {{ $t('help.readingScoreDetail.weightExample') }}
            </p>
          </div>

          <div>
            <p class="font-medium text-foreground mb-1">
              {{ $t('help.readingScoreDetail.contributionTitle') }}
            </p>
            <p>
              {{ $t('help.readingScoreDetail.contributionDesc') }}
            </p>
          </div>
        </div>
      </details>

      <!-- Threshold & Target -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: { type: 'spring', stiffness: 260, damping: 24, delay: 260 },
        }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary
          class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors"
        >
          <ChevronRightIcon
            class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90"
          />
          <h3 class="font-semibold text-primary">
            {{ $t('help.thresholdAndTarget') }}
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-3">
          <p>
            {{ $t('help.thresholdAndTarget.desc') }}
          </p>
          <p>
            {{ $t('help.thresholdAndTarget.example') }}
          </p>
        </div>
      </details>

      <!-- Custom Rules -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: { type: 'spring', stiffness: 260, damping: 24, delay: 320 },
        }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary
          class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors"
        >
          <ChevronRightIcon
            class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90"
          />
          <h3 class="font-semibold text-primary">
            {{ $t('help.customRulesHelp') }}
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-3">
          <p>
            {{ $t('help.customRules.intro') }}
          </p>
          <p class="font-medium text-foreground">
            {{ $t('help.customRules.effectLevels') }}
          </p>
          <ul class="space-y-2 pl-1">
            <li v-for="effect in effectLevels" :key="effect.nameKey" class="flex items-start gap-2">
              <span class="mt-1 w-1.5 h-1.5 rounded-full shrink-0" :class="effect.colorClass" />
              <span
                ><strong class="text-foreground">{{ $t(effect.nameKey) }}</strong> —
                {{ $t(effect.descKey) }}</span
              >
            </li>
          </ul>
          <p class="font-medium text-foreground mt-4">
            {{ $t('help.customRules.conflictTitle') }}
          </p>
          <p>
            {{ $t('help.customRules.conflictDesc') }}
          </p>
          <p>
            {{ $t('help.customRules.scopingDesc') }}
          </p>
        </div>
      </details>

      <!-- Tiebreaker -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: { type: 'spring', stiffness: 260, damping: 24, delay: 350 },
        }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary
          class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors"
        >
          <ChevronRightIcon
            class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90"
          />
          <h3 class="font-semibold text-primary">
            {{ $t('help.scoreTiebreaker') }}
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-3">
          <p>
            {{ $t('help.tiebreaker.intro') }}
          </p>
          <ul class="space-y-1 pl-4 list-disc">
            <li>
              <strong class="text-foreground">{{ $t('help.tiebreaker.largestFirst') }}</strong> —
              {{ $t('help.tiebreaker.largestFirstDesc') }}
            </li>
            <li>
              <strong class="text-foreground">{{ $t('help.tiebreaker.smallestFirst') }}</strong> —
              {{ $t('help.tiebreaker.smallestFirstDesc') }}
            </li>
            <li>
              <strong class="text-foreground">{{ $t('help.tiebreaker.alphabetical') }}</strong> —
              {{ $t('help.tiebreaker.alphabeticalDesc') }}
            </li>
            <li>
              <strong class="text-foreground">{{ $t('help.tiebreaker.oldestFirst') }}</strong> —
              {{ $t('help.tiebreaker.oldestFirstDesc') }}
            </li>
            <li>
              <strong class="text-foreground">{{ $t('help.tiebreaker.newestFirst') }}</strong> —
              {{ $t('help.tiebreaker.newestFirstDesc') }}
            </li>
          </ul>
        </div>
      </details>

      <!-- Reading the Audit Log -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: { type: 'spring', stiffness: 260, damping: 24, delay: 380 },
        }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary
          class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors"
        >
          <ChevronRightIcon
            class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90"
          />
          <h3 class="font-semibold text-primary">
            {{ $t('help.readingAuditLog') }}
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-3">
          <p>
            {{ $t('help.auditLog.intro') }}
          </p>
          <p>{{ $t('help.auditLog.actionsTitle') }}</p>
          <ul class="space-y-1 pl-4 list-disc">
            <li>
              <strong class="text-foreground">{{ $t('help.auditLog.dryRun') }}</strong> —
              {{ $t('help.auditLog.dryRunDesc') }}
            </li>
            <li>
              <strong class="text-foreground">{{ $t('help.auditLog.queued') }}</strong> —
              {{ $t('help.auditLog.queuedDesc') }}
            </li>
            <li>
              <strong class="text-foreground">{{ $t('help.auditLog.deleted') }}</strong> —
              {{ $t('help.auditLog.deletedDesc') }}
            </li>
          </ul>
        </div>
      </details>

      <!-- Execution Modes -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: { type: 'spring', stiffness: 260, damping: 24, delay: 440 },
        }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary
          class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors"
        >
          <ChevronRightIcon
            class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90"
          />
          <h3 class="font-semibold text-primary">
            {{ $t('help.executionModes') }}
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-3">
          <ul class="space-y-2 pl-1">
            <li class="flex items-start gap-2">
              <span class="mt-1 w-1.5 h-1.5 rounded-full bg-primary shrink-0" />
              <span
                ><strong class="text-foreground">{{ $t('help.executionModes.dryRun') }}</strong> —
                {{ $t('help.executionModes.dryRunDesc') }}</span
              >
            </li>
            <li class="flex items-start gap-2">
              <span class="mt-1 w-1.5 h-1.5 rounded-full bg-warning shrink-0" />
              <span
                ><strong class="text-foreground">{{ $t('help.executionModes.approval') }}</strong> —
                {{ $t('help.executionModes.approvalDesc') }}</span
              >
            </li>
            <li class="flex items-start gap-2">
              <span class="mt-1 w-1.5 h-1.5 rounded-full bg-destructive shrink-0" />
              <span
                ><strong class="text-foreground">{{ $t('help.executionModes.auto') }}</strong> —
                {{ $t('help.executionModes.autoDesc') }}</span
              >
            </li>
          </ul>
          <div class="mt-2 rounded-lg border border-border bg-muted/50 p-3">
            <p class="flex items-start gap-2">
              <ShieldIcon class="w-4 h-4 text-warning mt-0.5 shrink-0" />
              <span>
                <strong class="text-foreground">{{
                  $t('help.executionModes.safetyGuardTitle')
                }}</strong>
                —
                {{ $t('help.executionModes.safetyGuardDesc') }}
              </span>
            </p>
          </div>
        </div>
      </details>

      <!-- FAQ -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: { type: 'spring', stiffness: 260, damping: 24, delay: 500 },
        }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary
          class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors"
        >
          <ChevronRightIcon
            class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90"
          />
          <h3 class="font-semibold text-primary">
            {{ $t('help.faq') }}
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-4">
          <div v-for="item in faqItems" :key="item.qKey">
            <p class="font-medium text-foreground mb-1">
              {{ $t(item.qKey) }}
            </p>
            <p>{{ $t(item.aKey) }}</p>
          </div>
        </div>
      </details>

      <!-- About Capacitarr -->
      <details
        v-motion
        :initial="{ opacity: 0, y: 12 }"
        :enter="{
          opacity: 1,
          y: 0,
          transition: { type: 'spring', stiffness: 260, damping: 24, delay: 560 },
        }"
        data-slot="card"
        class="group rounded-xl border border-border bg-card shadow-sm overflow-hidden"
      >
        <summary
          class="flex items-center gap-3 px-5 py-4 cursor-pointer select-none hover:bg-accent transition-colors"
        >
          <ChevronRightIcon
            class="w-4 h-4 text-muted-foreground transition-transform group-open:rotate-90"
          />
          <h3 class="font-semibold text-primary">
            {{ $t('help.aboutCapacitarr') }}
          </h3>
        </summary>
        <div class="px-5 pb-5 text-sm text-muted-foreground leading-relaxed space-y-6">
          <!-- Project Info -->
          <div class="space-y-3">
            <p class="font-medium text-foreground">Project Info</p>
            <div class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2">
              <span class="text-muted-foreground">App</span>
              <span class="text-foreground font-medium">Capacitarr</span>

              <span class="text-muted-foreground">Description</span>
              <span class="text-foreground">Intelligent Media Capacity Management</span>

              <span class="text-muted-foreground">UI Version</span>
              <span class="text-foreground">
                v{{ uiVersion }}
                <template v-if="uiBuildDate">
                  <span class="text-muted-foreground text-xs ml-1">({{ uiBuildDate }})</span>
                </template>
              </span>

              <span class="text-muted-foreground">API Version</span>
              <span class="text-foreground">
                <template v-if="apiVersion">
                  {{ apiVersion }}
                  <template v-if="apiBuildDate">
                    <span class="text-muted-foreground text-xs ml-1">({{ apiBuildDate }})</span>
                  </template>
                </template>
                <template v-else> — </template>
              </span>

              <span class="text-muted-foreground">Source</span>
              <a
                href="https://gitlab.com/starshadow/software/capacitarr"
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex items-center gap-1 text-primary hover:underline"
              >
                GitLab
                <ExternalLinkIcon class="w-3 h-3" />
              </a>

              <span class="text-muted-foreground">Docs</span>
              <a
                href="https://starshadow.gitlab.io/software/capacitarr/"
                target="_blank"
                rel="noopener noreferrer"
                class="inline-flex items-center gap-1 text-primary hover:underline"
              >
                Documentation
                <ExternalLinkIcon class="w-3 h-3" />
              </a>

              <span class="text-muted-foreground">Community</span>
              <div class="flex items-center gap-3">
                <a
                  href="https://discord.gg/fbFkND5qgt"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex items-center gap-1.5 text-primary hover:underline"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="currentColor"
                    class="w-3.5 h-3.5"
                    aria-hidden="true"
                  >
                    <path
                      d="M20.317 4.37a19.791 19.791 0 0 0-4.885-1.515.074.074 0 0 0-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 0 0-5.487 0 12.64 12.64 0 0 0-.617-1.25.077.077 0 0 0-.079-.037A19.736 19.736 0 0 0 3.677 4.37a.07.07 0 0 0-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 0 0 .031.057 19.9 19.9 0 0 0 5.993 3.03.078.078 0 0 0 .084-.028 14.09 14.09 0 0 0 1.226-1.994.076.076 0 0 0-.041-.106 13.107 13.107 0 0 1-1.872-.892.077.077 0 0 1-.008-.128 10.2 10.2 0 0 0 .372-.292.074.074 0 0 1 .077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 0 1 .078.01c.12.098.246.198.373.292a.077.077 0 0 1-.006.127 12.299 12.299 0 0 1-1.873.892.077.077 0 0 0-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 0 0 .084.028 19.839 19.839 0 0 0 6.002-3.03.077.077 0 0 0 .032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 0 0-.031-.03zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.095 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.095 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z"
                    />
                  </svg>
                  Discord
                  <ExternalLinkIcon class="w-3 h-3" />
                </a>
                <a
                  href="https://www.reddit.com/r/capacitarr/"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex items-center gap-1.5 text-primary hover:underline"
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    viewBox="0 0 24 24"
                    fill="currentColor"
                    class="w-3.5 h-3.5"
                    aria-hidden="true"
                  >
                    <path
                      d="M12 0A12 12 0 0 0 0 12a12 12 0 0 0 12 12 12 12 0 0 0 12-12A12 12 0 0 0 12 0zm5.01 4.744c.688 0 1.25.561 1.25 1.249a1.25 1.25 0 0 1-2.498.056l-2.597-.547-.8 3.747c1.824.07 3.48.632 4.674 1.488.308-.309.73-.491 1.207-.491.968 0 1.754.786 1.754 1.754 0 .716-.435 1.333-1.01 1.614a3.111 3.111 0 0 1 .042.52c0 2.694-3.13 4.87-7.004 4.87-3.874 0-7.004-2.176-7.004-4.87 0-.183.015-.366.043-.534A1.748 1.748 0 0 1 4.028 12c0-.968.786-1.754 1.754-1.754.463 0 .898.196 1.207.49 1.207-.883 2.878-1.43 4.744-1.487l.885-4.182a.342.342 0 0 1 .14-.197.35.35 0 0 1 .238-.042l2.906.617a1.214 1.214 0 0 1 1.108-.701zM9.25 12C8.561 12 8 12.562 8 13.25c0 .687.561 1.248 1.25 1.248.687 0 1.248-.561 1.248-1.249 0-.688-.561-1.249-1.249-1.249zm5.5 0c-.687 0-1.248.561-1.248 1.25 0 .687.561 1.248 1.249 1.248.688 0 1.249-.561 1.249-1.249 0-.687-.562-1.249-1.25-1.249zm-5.466 3.99a.327.327 0 0 0-.231.094.33.33 0 0 0 0 .463c.842.842 2.484.913 2.961.913.477 0 2.105-.056 2.961-.913a.361.361 0 0 0 .029-.463.33.33 0 0 0-.464 0c-.547.533-1.684.73-2.512.73-.828 0-1.979-.196-2.512-.73a.326.326 0 0 0-.232-.095z"
                    />
                  </svg>
                  Reddit
                  <ExternalLinkIcon class="w-3 h-3" />
                </a>
              </div>

              <span class="text-muted-foreground">Donate</span>
              <div class="space-y-1">
                <p class="text-xs text-foreground/70">
                  Instead of supporting me, please consider donating to animal rescue:
                </p>
                <div class="flex items-center gap-3">
                  <a
                    href="https://uanimals.org/en/"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="inline-flex items-center gap-1.5 text-primary hover:underline"
                  >
                    <HeartIcon class="w-3.5 h-3.5" />
                    UAnimals
                    <ExternalLinkIcon class="w-3 h-3" />
                  </a>
                  <a
                    href="https://www.aspca.org/ways-to-help"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="inline-flex items-center gap-1.5 text-primary hover:underline"
                  >
                    <PawPrintIcon class="w-3.5 h-3.5" />
                    ASPCA
                    <ExternalLinkIcon class="w-3 h-3" />
                  </a>
                </div>
              </div>

              <span class="text-muted-foreground">Tip Jar</span>
              <div class="flex items-center gap-3">
                <a
                  href="https://github.com/sponsors/ghent"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex items-center gap-1.5 text-primary hover:underline"
                >
                  GitHub Sponsors
                  <ExternalLinkIcon class="w-3 h-3" />
                </a>
                <a
                  href="https://ko-fi.com/ghent"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex items-center gap-1.5 text-primary hover:underline"
                >
                  Ko-fi
                  <ExternalLinkIcon class="w-3 h-3" />
                </a>
                <a
                  href="https://buymeacoffee.com/ghentgames"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="inline-flex items-center gap-1.5 text-primary hover:underline"
                >
                  Buy Me a Coffee
                  <ExternalLinkIcon class="w-3 h-3" />
                </a>
              </div>

              <span class="text-muted-foreground">License</span>
              <span class="text-foreground">PolyForm Noncommercial 1.0.0 + CLA</span>

              <span class="text-muted-foreground">Author</span>
              <span class="text-foreground font-medium">Ghent Starshadow</span>

              <span class="text-muted-foreground">Contributors</span>
              <div class="flex flex-wrap items-center gap-1.5">
                <template v-if="contributors.length > 0">
                  <span v-for="(name, idx) in contributors" :key="name" class="text-foreground"
                    >{{ name
                    }}<span v-if="idx < contributors.length - 1" class="text-muted-foreground/50"
                      >,
                    </span></span
                  >
                </template>
                <span v-else class="text-muted-foreground italic">No contributors yet</span>
              </div>
            </div>
          </div>

          <!-- Tech Stack -->
          <div class="space-y-3">
            <p class="font-medium text-foreground">Tech Stack</p>
            <div class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2">
              <span class="text-muted-foreground">Frontend</span>
              <div class="flex flex-wrap gap-1.5">
                <UiBadge v-for="item in techStack.frontend" :key="item" variant="secondary">
                  {{ item }}
                </UiBadge>
              </div>

              <span class="text-muted-foreground">Backend</span>
              <div class="flex flex-wrap gap-1.5">
                <UiBadge v-for="item in techStack.backend" :key="item" variant="secondary">
                  {{ item }}
                </UiBadge>
              </div>

              <span class="text-muted-foreground">Auth</span>
              <div class="flex flex-wrap gap-1.5">
                <UiBadge v-for="item in techStack.auth" :key="item" variant="secondary">
                  {{ item }}
                </UiBadge>
              </div>

              <span class="text-muted-foreground">Infrastructure</span>
              <div class="flex flex-wrap gap-1.5">
                <UiBadge v-for="item in techStack.infrastructure" :key="item" variant="secondary">
                  {{ item }}
                </UiBadge>
              </div>
            </div>
          </div>

          <!-- Credits -->
          <div class="space-y-3">
            <p class="font-medium text-foreground">Credits &amp; Acknowledgments</p>
            <ul class="space-y-2 pl-1">
              <li v-for="credit in credits" :key="credit.name" class="flex items-start gap-2">
                <span class="mt-1 w-1.5 h-1.5 rounded-full bg-primary shrink-0" />
                <span
                  ><strong class="text-foreground">{{ credit.name }}</strong> —
                  {{ credit.desc }}</span
                >
              </li>
            </ul>
          </div>

          <!-- From the Author -->
          <div class="pt-4 border-t border-border/50">
            <p class="text-xs text-muted-foreground/70 leading-relaxed flex items-start gap-2">
              <img
                src="~/assets/images/flag-ua.svg"
                alt="Ukrainian flag"
                class="w-4 h-4 shrink-0 mt-0.5"
              />
              <span
                >I stand with Ukraine. This project is built with the belief that freedom,
                sovereignty, and self-determination matter — for people and for software.</span
              >
            </p>
          </div>
        </div>
      </details>
    </div>
  </div>
</template>

<script setup lang="ts">
import {
  ChevronRightIcon,
  ExternalLinkIcon,
  HeartIcon,
  PawPrintIcon,
  ShieldIcon,
} from 'lucide-vue-next';

const { uiVersion, uiBuildDate, apiVersion, apiBuildDate } = useVersion();
const config = useRuntimeConfig();
const contributors = computed(() => (config.public.contributors as string[]) || []);

const scoringFactors = [
  { nameKey: 'help.factor.watchHistory', descKey: 'help.factor.watchHistoryDesc' },
  { nameKey: 'help.factor.lastWatched', descKey: 'help.factor.lastWatchedDesc' },
  { nameKey: 'help.factor.fileSize', descKey: 'help.factor.fileSizeDesc' },
  { nameKey: 'help.factor.rating', descKey: 'help.factor.ratingDesc' },
  { nameKey: 'help.factor.timeInLibrary', descKey: 'help.factor.timeInLibraryDesc' },
  { nameKey: 'help.factor.seriesStatus', descKey: 'help.factor.seriesStatusDesc' },
];

const ratingSources = [
  { nameKey: 'help.dataSources.ratingSonarr', descKey: 'help.dataSources.ratingSonarrDesc' },
  { nameKey: 'help.dataSources.ratingRadarr', descKey: 'help.dataSources.ratingRadarrDesc' },
  { nameKey: 'help.dataSources.ratingLidarr', descKey: 'help.dataSources.ratingLidarrDesc' },
  { nameKey: 'help.dataSources.ratingReadarr', descKey: 'help.dataSources.ratingReadarrDesc' },
  { nameKey: 'help.dataSources.ratingPlex', descKey: 'help.dataSources.ratingPlexDesc' },
];

const enrichmentSources = [
  { key: 'help.dataSources.enrichmentWatch' },
  { key: 'help.dataSources.enrichmentRequests' },
  { key: 'help.dataSources.enrichmentCollections' },
];

const rawScoreExamples = [
  { nameKey: 'help.factor.watchHistory', descKey: 'help.rawScore.watchHistoryDesc' },
  { nameKey: 'help.factor.lastWatched', descKey: 'help.rawScore.lastWatchedDesc' },
  { nameKey: 'help.factor.fileSize', descKey: 'help.rawScore.fileSizeDesc' },
  { nameKey: 'help.factor.rating', descKey: 'help.rawScore.ratingDesc' },
  { nameKey: 'help.factor.timeInLibrary', descKey: 'help.rawScore.timeInLibraryDesc' },
  { nameKey: 'help.factor.seriesStatus', descKey: 'help.rawScore.seriesStatusDesc' },
];

const effectLevels = [
  {
    nameKey: 'help.effect.alwaysKeep',
    descKey: 'help.effect.alwaysKeepDesc',
    colorClass: 'bg-emerald-500',
  },
  {
    nameKey: 'help.effect.preferKeep',
    descKey: 'help.effect.preferKeepDesc',
    colorClass: 'bg-teal-400',
  },
  {
    nameKey: 'help.effect.leanKeep',
    descKey: 'help.effect.leanKeepDesc',
    colorClass: 'bg-sky-400',
  },
  {
    nameKey: 'help.effect.leanRemove',
    descKey: 'help.effect.leanRemoveDesc',
    colorClass: 'bg-amber-400',
  },
  {
    nameKey: 'help.effect.preferRemove',
    descKey: 'help.effect.preferRemoveDesc',
    colorClass: 'bg-orange-500',
  },
  {
    nameKey: 'help.effect.alwaysRemove',
    descKey: 'help.effect.alwaysRemoveDesc',
    colorClass: 'bg-red-500',
  },
];

const faqItems = [
  { qKey: 'help.faq.engineFrequencyQ', aKey: 'help.faq.engineFrequencyA' },
  { qKey: 'help.faq.integrationsQ', aKey: 'help.faq.integrationsA' },
  { qKey: 'help.faq.deleteHappensQ', aKey: 'help.faq.deleteHappensA' },
  { qKey: 'help.faq.notificationsQ', aKey: 'help.faq.notificationsA' },
  { qKey: 'help.faq.languageQ', aKey: 'help.faq.languageA' },
  { qKey: 'help.faq.safeToTestQ', aKey: 'help.faq.safeToTestA' },
];

const techStack = {
  frontend: [
    'Vue 3',
    'Nuxt 4',
    'Tailwind CSS v4',
    'shadcn-vue',
    'ApexCharts',
    'Lucide Icons',
    'SSE (EventSource)',
  ],
  backend: [
    'Go 1.25',
    'Echo HTTP',
    'GORM + SQLite',
    'Service Layer',
    'Event Bus',
    'SSE Broadcaster',
  ],
  auth: ['JWT', 'bcrypt', 'API Key', 'Plex OAuth', 'Proxy Header'],
  infrastructure: ['Docker', 'Alpine Linux'],
};

const credits = [
  { name: 'Maintainerr', desc: 'Inspiration & Plex OAuth reference implementation' },
  { name: 'shadcn-vue', desc: 'Component library' },
  { name: 'Tailwind CSS', desc: 'Utility-first CSS framework' },
  { name: 'Nuxt', desc: 'Vue meta-framework' },
  { name: 'Geist', desc: 'Typography (Vercel)' },
  { name: 'The *arr community', desc: 'Inspiration and ecosystem' },
];
</script>
