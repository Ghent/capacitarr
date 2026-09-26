<template>
  <div>
    <div class="mb-8">
      <h1 class="text-3xl font-bold tracking-tight">
        {{ $t('help.title') }}
      </h1>
      <p class="text-muted-foreground mt-1.5">
        {{ $t('help.subtitle') }}
      </p>
    </div>

    <div class="space-y-4">
      <HelpSection
        v-if="allAnnouncements.length > 0"
        :title="$t('announcements.title')"
        :icon="MegaphoneIcon"
        :default-open="true"
        :delay="0"
      >
        <div
          v-for="announcement in allAnnouncements"
          :key="announcement.id"
          class="rounded-lg border overflow-hidden"
          :class="
            announcement.active
              ? 'border-l-4 border-l-primary border-border'
              : 'border-border opacity-70'
          "
        >
          <div class="px-4 py-3">
            <div class="flex items-center gap-2.5 mb-2">
              <UiBadge :variant="announcement.active ? 'default' : 'secondary'" class="text-[10px]">
                {{
                  announcement.active ? $t('announcements.active') : $t('announcements.archived')
                }}
              </UiBadge>
              <UiBadge
                variant="outline"
                class="text-[10px]"
                :class="typeBadgeClass(announcement.type)"
              >
                {{ announcement.type }}
              </UiBadge>
              <span class="text-xs text-muted-foreground">
                {{ formatAnnouncementDate(announcement.date) }}
              </span>
            </div>
            <h3 class="font-semibold text-sm text-foreground mb-1.5">
              {{ announcement.title }}
            </h3>
            <!-- eslint-disable vue/no-v-html -->
            <!-- nosemgrep: javascript.vue.security.audit.xss.templates.avoid-v-html.avoid-v-html — HTML is pre-rendered at build time from developer-authored markdown files in frontend/announcements/, not user input -->
            <div
              class="text-sm text-muted-foreground leading-relaxed prose-sm [&_a]:text-primary [&_a]:underline [&_strong]:text-foreground [&_strong]:font-medium"
              v-html="announcement.body"
            />
            <!-- eslint-enable vue/no-v-html -->
          </div>
        </div>
      </HelpSection>

      <HelpSection :title="$t('help.howScoringWorks')" :delay="80">
        <p>{{ $t('help.howScoringWorks.intro') }}</p>
        <p>{{ $t('help.howScoringWorks.behavior') }}</p>
      </HelpSection>

      <HelpSection :title="$t('help.understandingSliders')" :delay="140">
        <p>{{ $t('help.understandingSliders.intro') }}</p>
        <ul class="space-y-2 pl-1">
          <li v-for="factor in scoringFactors" :key="factor.nameKey" class="flex items-start gap-2">
            <span class="mt-1 w-1.5 h-1.5 rounded-full bg-primary shrink-0" />
            <span
              ><strong class="text-foreground">{{ $t(factor.nameKey) }}</strong> —
              {{ $t(factor.descKey) }}</span
            >
          </li>
        </ul>
      </HelpSection>

      <HelpSection :title="$t('help.dataSources')" :delay="200" content-class="space-y-4">
        <p>{{ $t('help.dataSources.intro') }}</p>
        <div>
          <p class="font-medium text-foreground mb-1">
            {{ $t('help.dataSources.ratingsTitle') }}
          </p>
          <p class="mb-2">{{ $t('help.dataSources.ratingsDesc') }}</p>
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
          <p class="mb-2">{{ $t('help.dataSources.enrichmentDesc') }}</p>
          <ul class="space-y-2 pl-1">
            <li v-for="item in enrichmentSources" :key="item.key" class="flex items-start gap-2">
              <span class="mt-1 w-1.5 h-1.5 rounded-full bg-primary shrink-0" />
              <span>{{ $t(item.key) }}</span>
            </li>
          </ul>
        </div>
      </HelpSection>

      <HelpSection :title="$t('help.readingScoreDetail')" :delay="260" content-class="space-y-4">
        <p>{{ $t('help.readingScoreDetail.intro') }}</p>
        <div>
          <p class="font-medium text-foreground mb-1">
            {{ $t('help.readingScoreDetail.rawTitle') }}
          </p>
          <p>{{ $t('help.readingScoreDetail.rawDesc') }}</p>
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
          <p>{{ $t('help.readingScoreDetail.weightDesc') }}</p>
          <p class="mt-1">{{ $t('help.readingScoreDetail.weightExample') }}</p>
        </div>
        <div>
          <p class="font-medium text-foreground mb-1">
            {{ $t('help.readingScoreDetail.contributionTitle') }}
          </p>
          <p>{{ $t('help.readingScoreDetail.contributionDesc') }}</p>
        </div>
      </HelpSection>

      <HelpSection :title="$t('help.thresholdAndTarget')" :delay="260">
        <p>{{ $t('help.thresholdAndTarget.desc') }}</p>
        <p>{{ $t('help.thresholdAndTarget.example') }}</p>
      </HelpSection>

      <HelpSection :title="$t('help.customRulesHelp')" :delay="320">
        <p>{{ $t('help.customRules.intro') }}</p>
        <p class="font-medium text-foreground">{{ $t('help.customRules.effectLevels') }}</p>
        <ul class="space-y-2 pl-1">
          <li v-for="effect in effectLevels" :key="effect.nameKey" class="flex items-start gap-2">
            <span class="mt-1 w-1.5 h-1.5 rounded-full shrink-0" :class="effect.colorClass" />
            <span
              ><strong class="text-foreground">{{ $t(effect.nameKey) }}</strong> —
              {{ $t(effect.descKey) }}</span
            >
          </li>
        </ul>
        <p class="font-medium text-foreground mt-4">{{ $t('help.customRules.conflictTitle') }}</p>
        <p>{{ $t('help.customRules.conflictDesc') }}</p>
        <p>{{ $t('help.customRules.scopingDesc') }}</p>
      </HelpSection>

      <HelpSection :title="$t('help.scoreTiebreaker')" :delay="350">
        <p>{{ $t('help.tiebreaker.intro') }}</p>
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
      </HelpSection>

      <HelpSection :title="$t('help.readingAuditLog')" :delay="380">
        <p>{{ $t('help.auditLog.intro') }}</p>
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
      </HelpSection>

      <HelpSection :title="$t('help.executionModes')" :delay="440">
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
          <li class="flex items-start gap-2">
            <span class="mt-1 w-1.5 h-1.5 rounded-full bg-warning shrink-0" />
            <span
              ><strong class="text-foreground">{{ $t('help.executionModes.sunset') }}</strong> —
              {{ $t('help.executionModes.sunsetDesc') }}</span
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
      </HelpSection>

      <HelpSection :title="$t('help.faq')" :delay="500" content-class="space-y-4">
        <div v-for="item in faqItems" :key="item.qKey">
          <p class="font-medium text-foreground mb-1">{{ $t(item.qKey) }}</p>
          <p>{{ $t(item.aKey) }}</p>
        </div>
      </HelpSection>

      <HelpCollectionDeletion />

      <HelpSection
        id="show-level-evaluation"
        :title="$t('help.showLevelEvaluation')"
        :delay="580"
        content-class="space-y-4"
      >
        <p>{{ $t('help.showLevelEvaluation.intro') }}</p>
        <div class="space-y-2">
          <p class="font-medium text-foreground">
            {{ $t('help.showLevelEvaluation.howItWorksTitle') }}
          </p>
          <ul class="list-disc pl-5 space-y-1">
            <li>{{ $t('help.showLevelEvaluation.howItWorks1') }}</li>
            <li>{{ $t('help.showLevelEvaluation.howItWorks2') }}</li>
            <li>{{ $t('help.showLevelEvaluation.howItWorks3') }}</li>
          </ul>
        </div>
        <div class="space-y-2">
          <p class="font-medium text-foreground">
            {{ $t('help.showLevelEvaluation.sunsetBehaviorTitle') }}
          </p>
          <div class="rounded-lg border border-border bg-muted/50 p-3">
            <p>{{ $t('help.showLevelEvaluation.sunsetBehaviorDesc') }}</p>
          </div>
        </div>
        <div class="space-y-2">
          <p class="font-medium text-foreground">
            {{ $t('help.showLevelEvaluation.scoringTitle') }}
          </p>
          <ul class="list-disc pl-5 space-y-1">
            <li>{{ $t('help.showLevelEvaluation.scoring1') }}</li>
            <li>{{ $t('help.showLevelEvaluation.scoring2') }}</li>
            <li>{{ $t('help.showLevelEvaluation.scoring3') }}</li>
          </ul>
        </div>
        <p>
          <span class="font-medium text-foreground">{{
            $t('help.showLevelEvaluation.howToEnableTitle')
          }}</span>
          {{ $t('help.showLevelEvaluation.howToEnableDesc') }}
        </p>
      </HelpSection>

      <HelpAbout />
    </div>
  </div>
</template>

<script setup lang="ts">
import { MegaphoneIcon, ShieldIcon } from 'lucide-vue-next';
import { useAnnouncements } from '~/composables/useAnnouncements';

const { allAnnouncements } = useAnnouncements();

function typeBadgeClass(type: string): string {
  switch (type) {
    case 'critical':
      return 'border-destructive/50 text-destructive';
    case 'warning':
      return 'border-warning/50 text-warning';
    default:
      return 'border-primary/50 text-primary';
  }
}

function formatAnnouncementDate(dateStr: string): string {
  return new Date(dateStr + 'T00:00:00').toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  });
}

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
</script>
