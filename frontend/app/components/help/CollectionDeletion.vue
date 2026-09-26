<template>
  <HelpSection
    id="collection-deletion"
    :title="$t('help.collectionDeletion')"
    :delay="540"
    content-class="space-y-4"
  >
    <div class="rounded-lg border border-destructive/50 bg-destructive/5 p-3 text-sm">
      <p class="font-semibold text-destructive">
        {{ $t('help.collectionDeletion.warningTitle') }}
      </p>
      <p class="mt-1">
        {{ $t('help.collectionDeletion.warningBody') }}
      </p>
    </div>

    <p>
      <span class="font-medium text-foreground">{{
        $t('help.collectionDeletion.whatItDoesLabel')
      }}</span>
      {{ $t('help.collectionDeletion.whatItDoes') }}
    </p>

    <div class="space-y-2">
      <p class="font-medium text-foreground">{{ $t('help.collectionDeletion.sourcesTitle') }}</p>
      <ul class="list-disc pl-5 space-y-1">
        <li>
          <span class="text-foreground font-medium">{{
            $t('help.collectionDeletion.radarrLabel')
          }}</span>
          {{ $t('help.collectionDeletion.radarrDesc') }}
          <span class="text-emerald-600 dark:text-emerald-400 font-medium">{{
            $t('help.collectionDeletion.radarrRisk')
          }}</span>
        </li>
        <li>
          <span class="text-foreground font-medium">{{
            $t('help.collectionDeletion.plexLabel')
          }}</span>
          {{ $t('help.collectionDeletion.plexDesc') }}
          <span class="text-amber-600 dark:text-amber-400 font-medium">{{
            $t('help.collectionDeletion.plexRisk')
          }}</span>
        </li>
        <li>
          <span class="text-foreground font-medium">{{
            $t('help.collectionDeletion.jellyfinEmbyLabel')
          }}</span>
          {{ $t('help.collectionDeletion.jellyfinEmbyDesc') }}
          <span class="text-amber-600 dark:text-amber-400 font-medium">{{
            $t('help.collectionDeletion.jellyfinEmbyRisk')
          }}</span>
        </li>
      </ul>
    </div>

    <div class="space-y-2">
      <p class="font-medium text-foreground">
        {{ $t('help.collectionDeletion.multiSourceTitle') }}
      </p>
      <p>
        {{ $t('help.collectionDeletion.multiSourceDesc') }}
      </p>
    </div>

    <div class="space-y-2">
      <p class="font-medium text-foreground">{{ $t('help.collectionDeletion.watchTitle') }}</p>
      <ul class="list-disc pl-5 space-y-1">
        <li>
          <span class="text-foreground font-medium">{{
            $t('help.collectionDeletion.watchMoreLabel')
          }}</span>
          {{ $t('help.collectionDeletion.watchMoreDesc') }}
        </li>
        <li>
          <span class="text-foreground font-medium">{{
            $t('help.collectionDeletion.watchFranchiseLabel')
          }}</span>
          {{ $t('help.collectionDeletion.watchFranchiseDesc') }}
        </li>
        <li>
          <span class="text-foreground font-medium">{{
            $t('help.collectionDeletion.watchCustomLabel')
          }}</span>
          {{ $t('help.collectionDeletion.watchCustomDesc') }}
        </li>
        <li>
          <span class="text-foreground font-medium">{{
            $t('help.collectionDeletion.watchStackLabel')
          }}</span>
          {{ $t('help.collectionDeletion.watchStackDesc') }}
        </li>
        <li>
          <span class="text-foreground font-medium">{{
            $t('help.collectionDeletion.watchPermanentLabel')
          }}</span>
          {{ $t('help.collectionDeletion.watchPermanentDesc') }}
        </li>
      </ul>
    </div>

    <div class="space-y-2">
      <p class="font-medium text-foreground">{{ $t('help.collectionDeletion.safetyTitle') }}</p>
      <ul class="list-disc pl-5 space-y-1">
        <li>{{ $t('help.collectionDeletion.safetyKeep') }}</li>
        <li>{{ $t('help.collectionDeletion.safetySnooze') }}</li>
        <li>{{ $t('help.collectionDeletion.safetyApproval') }}</li>
      </ul>
    </div>

    <div class="space-y-2">
      <p class="font-medium text-foreground">{{ $t('help.collectionDeletion.beforeTitle') }}</p>
      <ul class="list-disc pl-5 space-y-1">
        <li>{{ $t('help.collectionDeletion.beforeDryRun') }}</li>
        <li>{{ $t('help.collectionDeletion.beforeKeep') }}</li>
        <li>{{ $t('help.collectionDeletion.beforeApproval') }}</li>
        <li>{{ $t('help.collectionDeletion.beforeReview') }}</li>
      </ul>
    </div>

    <p>
      <span class="font-medium text-foreground">{{
        $t('help.collectionDeletion.enableLabel')
      }}</span>
      {{ $t('help.collectionDeletion.enableDesc') }}
    </p>
  </HelpSection>
</template>
